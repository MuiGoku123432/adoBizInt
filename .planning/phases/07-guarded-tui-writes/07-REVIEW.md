---
phase: 07-guarded-tui-writes
reviewed: 2026-10-14T00:00:00Z
depth: standard
files_reviewed: 41
files_reviewed_list:
  - cmd/adoBizInt/actor_test.go
  - cmd/adoBizInt/audit_test.go
  - cmd/adoBizInt/audit.go
  - cmd/adoBizInt/crash_test.go
  - cmd/adoBizInt/dispatch.go
  - cmd/adoBizInt/exit.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/guardnames_test.go
  - cmd/adoBizInt/write.go
  - go.mod
  - internal/fetchctx/fetchctx_test.go
  - internal/fetchctx/fetchctx.go
  - internal/guard/audit_test.go
  - internal/guard/audit.go
  - internal/guard/guard_test.go
  - internal/guard/guard.go
  - internal/guard/guardtest/audit.go
  - internal/guard/guardtest/fake.go
  - internal/guard/ops.go
  - internal/guard/structural_test.go
  - internal/guard/uibypass_test.go
  - internal/ui/app_test.go
  - internal/ui/app.go
  - internal/ui/branches/branches.go
  - internal/ui/confirm/batch.go
  - internal/ui/confirm/confirm_test.go
  - internal/ui/confirm/flow.go
  - internal/ui/confirm/modal.go
  - internal/ui/dashboard/dashboard.go
  - internal/ui/pipelines/pipelines.go
  - internal/ui/pipelines/write_test.go
  - internal/ui/prs/prs.go
  - internal/ui/prs/write_test.go
  - internal/ui/releases/releases.go
  - internal/ui/releases/write_test.go
  - internal/ui/workitems/resolve_test.go
  - internal/ui/workitems/staging_test.go
  - internal/ui/workitems/staging.go
  - internal/ui/workitems/workitems_test.go
  - internal/ui/workitems/workitems.go
findings:
  critical: 1
  warning: 8
  info: 6
  total: 15
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-10-14
**Depth:** standard
**Files Reviewed:** 41 (40 Go files + go.mod)
**Status:** issues_found

## Summary

The shared guard (`internal/guard`) holds its documented invariants. The sink opens before the pre-read and the mutation. Mutate runs under a recover that never re-panics. Only executed and unknown outcomes are audited. A failed audit append downgrades the outcome to unknown. The guard never writes stdout and never calls `os.Exit`. New audit files are created 0600 and new directories 0700. Audit records carry no argv, URL, headers or PAT. `actor` is restricted to three constants, and no flag, environment variable or config value can set it.

Every TUI write goes through `confirm/batch.go` → `guard.Run` with `ActorTUI` and a bounded budget. Blank effort stays nil all the way to the client. The execute-time subset check (`approves`) refuses any change the modal did not show.

`go vet ./...` and `go test -race ./internal/guard/... ./internal/ui/... ./internal/fetchctx/... ./cmd/...` pass. The locked CLI test checksums in `07-cli-tests.sha256` verify (GUARD-06).

Main concerns:
1. **The quit hold is not the only way the TUI process can end mid-write.** If the process ends this way, ADO can change and leave no audit record, which is the D-13 window this project exists to close (CR-01).
2. **The confirm modal can hide the value being approved.** Real work-item IDs push story-resolve lines past the 60-rune cap (WR-01).
3. **The `cli-human` vs `agent` actor label comes from a TTY check.** An agent can satisfy that check, which matters because the label is SOX segregation-of-duties evidence (WR-02).
4. **Some async-ordering and staging holes are left in the TUI state machine** (WR-04, WR-05).

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: A TUI write can be killed between mutate and audit append; the quit hold covers only the q / ctrl+c keys

**File:** `internal/ui/app.go:93-101`, `internal/ui/confirm/batch.go:36,57`, `cmd/adoBizInt/main.go:141-151` (launch site, outside this phase's file list)
**Issue:** The quit hold (`quit && m.currentViewWriteBusy()`) only intercepts the Quit **key binding**. Other paths end the process while `Execute` is in flight on its cmd goroutine:
- **SIGTERM.** Bubbletea v1.3.10's `handleSignals` turns SIGTERM into `QuitMsg`. The event loop handles `QuitMsg` before the model sees it (`tea.go:402-403`).
- **SIGINT when stdin is not a TTY.** It becomes `InterruptMsg` and returns `ErrInterrupted`.
- **SIGHUP.** Closing the terminal window ends the process by default.
- **A panic in any other cmd goroutine,** for example a read fetch. `recoverFromGoPanic` kills the program.

`main` returns as soon as `p.Run()` returns, so the process exits and takes the write goroutine with it. If that happens after `op.Mutate` sent the request and before `appendAuditRecord`, ADO has changed and the audit log has no record.

The CLI explicitly handles the interrupt-mid-write case (D-22: signal context → `classify` → unknown → audited). The TUI passes `context.Background()` as the parent, so it gets none of that. Both the phase invariant "no write can race a quit" and SOX audit-evidence completeness fail on this path.

**Fix:** Make process exit wait for an in-flight guarded write, bounded by its budget, and let the guard see the signal as a parent cancellation. For example:

```go
// internal/ui/confirm/batch.go
var inflight sync.WaitGroup

// Wait blocks until every in-flight Execute has returned (its audit append done).
func Wait() { inflight.Wait() }

func Execute(b Batch, budget time.Duration) Batch {
	inflight.Add(1)
	defer inflight.Done()
	...
}

// cmd/adoBizInt/main.go
p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(sigCtx))
_, err := p.Run()
confirm.Wait() // never exit with a mutation in flight and unaudited
```

Also pass a signal-derived parent context into `guard.Run` from the TUI rather than `context.Background()`, so a SIGTERM during a write classifies as `unknown` and is audited, matching the CLI.

## Warnings

### WR-01: The confirm modal truncates realistic diff lines and hides the proposed value the user is approving

**File:** `internal/ui/confirm/modal.go:33-38`, `internal/ui/confirm/batch.go:106-109`
**Issue:** Lines are cut at 60 runes with `...`. UI-SPEC sized that cap against a 2-digit ID (`#11 ...CompletedWork  (none) → 4`, 55 runes). Real ADO IDs are 5-7 digits:
- `#12345 Microsoft.VSTS.Scheduling.OriginalEstimate  (none) → 4` is 61 runes.
- `#123456 Microsoft.VSTS.Scheduling.CompletedWork  12.5 → 14.75` is 61 runes.

Both render with the `→ <value>` tail replaced by `...`. On every User Story resolve whose dev task has a 5+ digit ID, `y` approves an effort write whose target value is not on screen. That undermines the "y means write what I saw" contract (D-01, T-07-15). `approves()` compares full values, so the write is still what was previewed, but the human never saw it.

**Fix:** Do not truncate the `To` side. Either:
- shorten the field name for display (for example strip the `Microsoft.VSTS.Scheduling.` prefix, keeping the full name in the audit record), or
- truncate the `From`/field segment and keep `→ <To>` intact, or
- wrap long lines instead of cutting them.

Add a test using a 6-digit task ID.

### WR-02: The CLI actor label is a stdin-TTY heuristic, so an agent under a pty is recorded as `cli-human`

**File:** `cmd/adoBizInt/write.go:96-112`
**Issue:** `cliActor()` returns `cli-human` whenever `term.IsTerminal(os.Stdin.Fd())`. No flag, environment variable or config value sets it. But any automated caller that allocates a pty is labelled human: `script -q /dev/null adoBizInt ...`, `expect`, tmux `send-keys`, many agent harnesses.

The comment says the dangerous direction is "an agent-run write recorded as a human action", and this is that direction, reachable with one wrapper command. The label is persisted as SOX ITGC segregation-of-duties evidence, so a reviewer will read `cli-human` as a stronger claim than the code makes.

**Fix:** At minimum, document in the audit record contract and in REQUIREMENTS/AGENT-03 that `cli-human` means "stdin was a terminal", not "a human acted". Consider requiring stdin, stdout and stderr all to be TTYs. Consider recording the heuristic's basis (for example `actor_basis: "stdin_tty"`) so evidence consumers do not over-read it. Raise with compliance before Phase 13 builds its tier gate on the same definition.

### WR-03: An unresolved home directory silently relocates the audit log to the current working directory

**File:** `internal/guard/audit.go:20-23`
**Issue:** `home, _ := os.UserHomeDir()` discards the error. When HOME is unset (systemd units, some CI runners, `env -i`), `filepath.Join("", ".config", "adoBizInt", "audit.jsonl")` is the relative path `.config/adoBizInt/audit.jsonl`. `OpenAuditSink` then creates it under whatever directory the process was launched from, which could be a repo working tree where it can be committed or lost.

The sink "opens" successfully, so D-13's fail-closed check passes while the evidence is scattered. This behaviour predates the phase but moved into the shared guard, so it now also governs every TUI write.

**Fix:** Fail closed:

```go
func DefaultAuditPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot resolve home directory for audit log: %w", err)
	}
	return filepath.Join(home, ".config", "adoBizInt", "audit.jsonl"), nil
}
```

Then have `Run`'s default opener return that error as `SinkErr`, which produces a refusal on both surfaces.

### WR-04: Confirm decisions are delivered asynchronously while the modal stays open, so a second key can produce a conflicting decision

**File:** `internal/ui/confirm/flow.go:88-93`, `internal/ui/confirm/modal.go:48-55`
**Issue:** Pressing `y`/`n`/`esc` returns a `tea.Cmd` that later yields `WriteMsg`/`DiscardMsg`/`StayMsg`. `f.modal` stays non-nil until that message is processed, so any key pressed in the meantime is fed to the modal again. A later key can produce a second, contradictory decision cmd. Bubbletea runs each cmd in its own goroutine, so delivery order is not guaranteed.

On the PRs view, `y` is also the copy-link key. Pressing `n` and then `y` quickly (key repeat, pasted input, a loaded machine) queues both `DiscardMsg` and `WriteMsg`. If `WriteMsg` is delivered first, the write executes. For an accident guard (D-04) whose only job is to make sure only a deliberate `y` writes, the first decision should be final.

**Fix:** Decide synchronously in `Flow.Update` and close the modal on the first decision key:

```go
case tea.KeyMsg:
	if f.modal != nil {
		switch msg.String() {
		case "y":
			f.modal = nil
			pending, budget := f.pending, f.budget
			return f, func() tea.Msg { return WrittenMsg{Batch: Execute(pending, budget)} },
				Result{Handled: true, Status: fmt.Sprintf("Writing #%d...", pending.ID)}
		case "n": /* settle Proceed */
		case "esc": /* settle Stay+Kept */
		}
		return f, nil, Result{Handled: true}
	}
```

Alternatively set a `decided` flag on the first decision key and swallow keys until the decision message arrives.

### WR-05: An in-flight User Story child fetch can silently replace a staged edit on a different item

**File:** `internal/ui/workitems/workitems.go:258-272`, `307-339`, `343-366`; `internal/ui/workitems/staging.go:83-88`
**Issue:** Pressing `n` on an Active User Story starts an async `GetChildWorkItems` cmd, and no busy state is set. Meanwhile the user can move to another row and press `n`, which stages item Y. When `ChildTasksLoadedMsg` arrives, the effort modal opens for story X even though the highlight is on Y. Submitting it calls `m.stage(*X, ...)`, and `stage` "replaces any staged edit", so Y's edit is dropped with no confirm and no status.

This breaks D-02 ("Nothing can stay staged silently"). Nothing is written, so the failure is in the safe direction, but the user believes Y is staged.

**Fix:** In the `EffortSubmitMsg` handler, and when opening the modal on `ChildTasksLoadedMsg`, refuse or confirm-first when `m.staged != nil && m.staged.item.ID != effortMsg.ItemID`. Either route through `startConfirm` with the effort submit held, or drop the late `ChildTasksLoadedMsg` when the highlight has moved off `ParentID`.

### WR-06: `guard.SetState` with an empty state shows no state change in the diff but sends `System.State = ""`

**File:** `internal/guard/ops.go:92-133`, `283`
**Issue:** `diffWorkItem` treats `state == ""` as "state not requested" and omits the line. `Mutate` still always sends the state: `UpdateWorkItemState(ctx, project, id, "")`, or `UpdateWorkItemStateWithEffort(..., "", ...)`, which appends `/fields/System.State` with value `""`.

The preview, the confirm modal and the audit record would then all describe an effort-only write while the patch tries to blank the state. Today the CLI rejects an empty `--state` and the TUI always passes one, but `SetState` is now an exported shared API, and the diff code openly invites the empty-state usage.

**Fix:** Refuse in `Plan` so nothing is attempted:

```go
if state == "" {
	return nil, fmt.Errorf("work item %d: a target state is required", id)
}
```

Or make `Mutate` omit the state operation when `state == ""`. Either way, the diff and the patch must describe the same document.

### WR-07: The GUARD-02 structural test misses method values and an aliased guard import

**File:** `internal/guard/uibypass_test.go:115-129`, `185-192`
**Issue:** `mutationCalls` only flags a `CallExpr` whose `Fun` is a selector naming a mutation. Taking a method value bypasses it, for example `approve := m.writer.ApprovePullRequest; approve(ctx, ...)`, or passing `c.RejectRelease` as an argument. That second pattern is exactly the one `guard/ops.go:197-208` itself uses.

`TestUIGuardCallsCarryTUIActor` hard-codes the identifier `guard`, so a `guard.Run` reached through an aliased import (`g "…/internal/guard"`) is neither counted nor checked for `ActorTUI`. The test exists so that "a structural test fails if any internal/ui file calls an ado mutation directly" (GUARD-02), and both gaps let a direct mutation through.

**Fix:**
- In `mutationCalls`, inspect every `*ast.SelectorExpr`, not only call funs, that names a forbidden method and is not qualified by the guard package import.
- In the actor test, resolve the guard import's local name the same way `mutationCalls` does.
- Add both cases to the tripwire snippet.

### WR-08: Flow-result handling and the D-08 sticky line are copy-pasted across four views (rule of three)

**File:** `internal/ui/prs/prs.go:184-199`, `internal/ui/releases/releases.go:206-221`, `internal/ui/pipelines/pipelines.go:205-220`, plus the sticky render at `prs.go:431-436`, `releases.go:513-518`, `pipelines.go:489-494`, `workitems.go:768-773`
**Issue:** The "flow sees every message first → apply Status/IsErr on settle → refetch on Refresh" block is identical in three views. The sticky-line render block, which is the D-08 contract, appears four times. `confirm.Flow`'s doc says it exists "so the D-07, D-08 and D-10 handling exists once", but its application to the view does not. The project and user conventions require extraction at three or more copies. A future fix to sticky or status semantics will have to be made in four places and will drift.

**Fix:** Add one small helper in `confirm`, for example `func (f Flow) StickyLine() string` that returns the rendered line plus newline or `""`. Optionally add a `Result.Apply(statusMsg *string, statusErr *bool) bool` that applies Status/IsErr and reports whether to refresh. Each view then keeps only its own refetch call.

## Info

### IN-01: A Task resolve is now two separate PATCHes, state first and then effort

**File:** `internal/ui/workitems/staging.go:47-68`
**Issue:** Before this phase, a Task close sent effort and state in one patch (`UpdateWorkItemStateWithEffort`'s doc says it was "Used for Tasks transitioning to Closed which require effort tracking"). GUARD-03/D-09 locks the two-write order. However:
- A process rule that requires RemainingWork on Closed will now fail op 0.
- A rule that rewrites effort on close will make the effort op refuse as "changed since preview".

**Fix:** Confirm both cases against the live org in PLAT-05. Consider noting the risk in the D-09 rationale.

### IN-02: The audit record can under-describe the patch that was sent

**File:** `internal/guard/ops.go:121-131`, `295-307`
**Issue:** An effort value equal to the current value, and the re-sent `System.State` on the effort op, are both omitted from `changes` but are still in the JSON-Patch. ADO treats them as no-ops, so nothing changed that was not recorded. Still, the record is a diff, not the request.
**Fix:** Document "changes = effective diff, not request payload" in the audit record contract.

### IN-03: `SinkErr` is overloaded for an invalid actor

**File:** `internal/guard/guard.go:121-126`
**Issue:** An actor outside the three labels returns `Report{SinkErr: ...}`. Both surfaces then print "audit sink/log not writable: <path>", which would send an operator to debug file permissions.
**Fix:** Add a distinct `Report.ActorErr`, or make the surfaces print `SinkErr`'s text rather than a fixed sink message.

### IN-04: `NoOpLabel` plus `prStatusClient` is a single-use side channel

**File:** `internal/ui/confirm/batch.go:16-24`, `internal/ui/prs/prs.go:533-549`
**Issue:** An optional hook on the shared `Batch` and a wrapper client that writes through a shared `*string` from the preview and execute cmd goroutines exist only for the "already abandoned" wording. The current read happens after the channel hand-off, so it is race-free, but any later reader of `NoOpLabel` after `Execute` starts would race.
**Fix:** Consider carrying the pre-read state in `guard.Report` (for example a `NoOpReason` string set by `Plan`) and drop the wrapper.

### IN-05: Typing `q` in a text input with an edit staged starts the write confirm

**File:** `internal/ui/app.go:93-110`
**Issue:** The Quit binding (`q`) is matched before text-input focus is checked. With an item staged and the search box or effort modal focused, typing `q` starts `ConfirmLeave`, and the confirm modal opens over the effort modal. This comes from the older behaviour where `q` quits even while typing, and the new path is safer than an outright quit, but it is surprising mid-edit.
**Fix:** When `isCurrentViewTextInputFocused()` and the current view is not write-busy, match only `ctrl+c` as quit.

### IN-06: A merged comment line in the recover block

**File:** `internal/guard/guard.go:181`
**Issue:** "...the T-04-01 no-argv rule hold. A refused write that later panics is recorded unknown, which" is one run-on line, a leftover from the move.
**Fix:** Reflow the comment.

---

_Reviewed: 2026-10-14_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
