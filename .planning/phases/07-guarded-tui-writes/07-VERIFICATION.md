---
phase: 07-guarded-tui-writes
verified: 2026-10-16T14:01:19Z
status: human_needed
score: 14/15 must-haves verified
covered_files:
  - ".planning/phases/07-guarded-tui-writes/07-01-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-01-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-02-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-02-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-03-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-03-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-04-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-04-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-05-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-05-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-06-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-06-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-07-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-07-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-08-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-08-SUMMARY.md"
  - ".planning/phases/07-guarded-tui-writes/07-09-PLAN.md"
  - ".planning/phases/07-guarded-tui-writes/07-09-SUMMARY.md"
  - "cmd/adoBizInt/actor_test.go"
  - "cmd/adoBizInt/audit.go"
  - "cmd/adoBizInt/audit_test.go"
  - "cmd/adoBizInt/crash_test.go"
  - "cmd/adoBizInt/dispatch.go"
  - "cmd/adoBizInt/exit.go"
  - "cmd/adoBizInt/flags.go"
  - "cmd/adoBizInt/guardnames_test.go"
  - "cmd/adoBizInt/main.go"
  - "cmd/adoBizInt/tuiexit_test.go"
  - "cmd/adoBizInt/write.go"
  - "go.mod"
  - "internal/fetchctx/fetchctx.go"
  - "internal/fetchctx/fetchctx_test.go"
  - "internal/guard/audit.go"
  - "internal/guard/audit_test.go"
  - "internal/guard/guard.go"
  - "internal/guard/guard_test.go"
  - "internal/guard/guardtest/audit.go"
  - "internal/guard/guardtest/fake.go"
  - "internal/guard/ops.go"
  - "internal/guard/structural_test.go"
  - "internal/guard/uibypass_test.go"
  - "internal/ui/app.go"
  - "internal/ui/app_test.go"
  - "internal/ui/branches/branches.go"
  - "internal/ui/confirm/batch.go"
  - "internal/ui/confirm/confirm_test.go"
  - "internal/ui/confirm/flow.go"
  - "internal/ui/confirm/inflight_test.go"
  - "internal/ui/confirm/modal.go"
  - "internal/ui/dashboard/dashboard.go"
  - "internal/ui/pipelines/pipelines.go"
  - "internal/ui/pipelines/write_test.go"
  - "internal/ui/prs/prs.go"
  - "internal/ui/prs/write_test.go"
  - "internal/ui/releases/releases.go"
  - "internal/ui/releases/write_test.go"
  - "internal/ui/workitems/effort.go"
  - "internal/ui/workitems/resolve_test.go"
  - "internal/ui/workitems/staging.go"
  - "internal/ui/workitems/staging_test.go"
  - "internal/ui/workitems/workitems.go"
  - "internal/ui/workitems/workitems_test.go"
covered_digest: "v3:sha256:05b63ae67bab9aa4f7bb89c3a6847638afcae61e74cef3a35cc249851e3bf192"
behavior_unverified: 1 # sub-truth 12f (07-09 truth 6: real-signal exit code and terminal restore). Not one of the 15 headline truths; the headline score is unaffected
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 13/15
  gaps_closed:
    - "A TUI write that is interrupted mid-flight is audited exactly like a CLI write: the process cannot exit between op.Mutate sending the request and the audit append (CR-01, closed by 07-09)"
  gaps_remaining: []
  regressions: []
behavior_unverified_items:
  - truth: "12f (07-09 truth 6): bubbletea keeps its own SIGINT/SIGTERM handling and the TUI exit codes are unchanged; a SIGTERM still ends p.Run through QuitMsg and run returns exitOK once confirm.Wait is done"
    test: "Start the TUI against a test org, stage and confirm a write (press y), and send SIGTERM to the process (kill -TERM <pid>) while 'Writing #<id>...' is shown. Repeat once with no write in flight"
    expected: "The process exits 0 after the write settles. The terminal is restored (not left raw, cursor visible). ~/.config/adoBizInt/audit.jsonl gains exactly one line for that write, with \"actor\":\"tui\" and outcome unknown (or executed if the PATCH finished before the signal). With no write in flight the exit is immediate and nothing is appended"
    why_human: "No test drives bubbletea's real signal goroutine or an OS signal. The unit tests stand the signal in with a context cancel, and the AST pin plus a read of bubbletea v1.3.10 tea.go (handleSignals L273-312, eventLoop L402-406, Run L716-745) prove the wiring and the expected path, but not the live process"
human_verification:
  - test: "Live SIGTERM during a real TUI write (07-09 coverage D5, sub-truth 12f). Start the TUI, confirm a write with y, then kill -TERM <pid> while 'Writing #<id>...' is shown"
    expected: "Exactly one new audit line for that write with \"actor\":\"tui\" and outcome unknown (or executed if it finished first). The process exits 0 and the terminal is restored. The ADO item should be checked by hand, as the unknown outcome asks"
    why_human: "Needs a live terminal, a live ADO write and an OS signal; no automated test reaches bubbletea's signal goroutine"
  - test: "PRs tab against a test PR you can approve: press a, then n; then press c on an active PR"
    expected: "Rounded 'Confirm #<id>' modal shows 'reviewer.vote  0 → 10' and '[y] write  [n] discard  [esc] stay'. n changes nothing in ADO. c shows 'status  active → completed' with no old 'Merge PR ... [y] confirm' status prompt. At 80x24 the 66-column modal shows title and key hint unclipped"
    why_human: "Live ADO and real-terminal rendering (07-04 deferred human-check; 07-UI-SPEC backstop)"
  - test: "Releases tab: open a pending approval, type a comment, submit. Pipelines tab: press t, pick a definition and branch, submit, then esc"
    expected: "A second modal 'Confirm #<approval id>' shows 'approval.status  pending → approved' before anything is sent. The pipeline confirm shows 'queued_build  (none) → <branch>'. esc queues nothing in ADO"
    why_human: "Live ADO round trip (07-05 deferred human-check)"
  - test: "Work Items: press n on an Active bug, then down, esc, down, n. Repeat at 80x24"
    expected: "Actions cell shows the whole 'staged → Resolved' in amber. down opens 'Confirm #<id>' with 'System.State  Active → Resolved'. esc keeps the item staged. n discards and moves. At 80x24 the 18-column Actions column does not clip the row or status hint"
    why_human: "Real-terminal layout and live ADO (07-06 deferred human-check; 07-UI-SPEC backstop)"
  - test: "Close an Active Task with n: leave Original Estimate blank, set Remaining 0 and Completed > 0, submit, move down, press y"
    expected: "One confirm modal lists the state and the two effort changes, with no Original Estimate line. In ADO History, Original Estimate is unchanged. ~/.config/adoBizInt/audit.jsonl gains two lines with \"actor\":\"tui\""
    why_human: "Only real ADO History can prove the blank field was not zeroed server-side (07-07 deferred human-check)"
  - test: "Stage a state change on Work Items, press 3, then esc, then q, then n"
    expected: "The confirm modal appears before the PRs tab. esc stays on Work Items with the row still staged. q shows the modal again. n exits the app without writing"
    why_human: "Real-terminal end-to-end flow (07-08 deferred human-check)"
  - test: "On an 80-column terminal, trigger a long server error (e.g. a 409 on PR complete) and an unknown outcome"
    expected: "The sticky line and status line stay readable and are not clipped by the help bar"
    why_human: "07-UI-SPEC backstop: narrow-terminal readability of server-controlled text"
  - test: "Decide the reading of ROADMAP SC1 'one audit record per changed field' versus SC2 'two audit records' for a resolve"
    expected: "Confirm that bundling a resolve's effort fields into ONE record is acceptable. Today Remaining and Completed share one record with changes[] holding both, and each changed field appears in exactly one record. Phase 16 SC3 reuses the 'one audit record per changed field' wording"
    why_human: "The roadmap criteria contradict each other for a multi-field effort write, and the code follows SC2 (TestTaskResolveIsTwoGuardedWrites pins 2 records for 3 changed fields). This is a spec decision, not a code check"
  - test: "Judgment-tier prohibition (07-04): 'Must NOT retry a write automatically after a failed or unknown outcome'"
    expected: "Human sign-off. Non-authoritative LLM verdict: holds. confirm.Execute runs each op at most once and breaks on the first outcome that is neither executed nor no_op. Execute has one call site (flow.go:107), and WriteMsg clears the modal so a second y is ignored. Flow.settle never calls Start again. 07-09 adds test-tier evidence for the interrupted case: TestExecuteInterruptedMidMutateIsAuditedUnknown asserts Mutate ran exactly once"
    why_human: "unverified-prohibition: judgment tier, so human review is recommended"
---

# Phase 7: Guarded TUI Writes Verification Report

**Phase Goal:** Every write made from the TUI is guarded and audited exactly like a CLI write, through one guard both surfaces share
**Verified:** 2026-10-16T14:01:19Z
**Status:** human_needed
**Re-verification:** Yes. This follows gap closure plan 07-09 (CR-01). The previous report was gaps_found at 13/15.

## Goal Achievement

The goal is met in code. CR-01, the one blocking gap, is closed. A TUI guarded write now runs under the same `signal.NotifyContext(os.Interrupt, SIGTERM)` parent the CLI uses. When a signal arrives mid-write, the guard's existing `classify(err, ctx.Err())` path records the write `unknown` with actor `tui`. `main` then calls `confirm.Wait()` after `p.Run()` and before the error branch, so the process cannot exit between Mutate and the audit append. I checked this against the production code, the bubbletea v1.3.10 source and three passing tests. I did not take the SUMMARY's word for it.

The status is human_needed, not passed. The reasons:
- 8 human items from the prior report are still outstanding: live ADO and terminal checks, the SC1 spec reading and a judgment-tier prohibition.
- 1 new live-signal item (07-09 coverage D5).
- No gaps remain.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: A TUI state/effort change shows the pre-read diff, refuses a no-op without a request, is confirmed once per item on move-on, and leaves one audit record per changed field | UNCERTAIN | Unchanged from the prior report: no source in scope changed. Diff, no-op and confirm-once are verified (`TestStagedStateConfirmsOnRowMove`, `TestNoOpSkipsModalAndMoves` pass in the full suite). One effort record bundles several fields. That matches SC2, but not a literal per-field reading. Spec decision for the human |
| 2 | SC2: A TUI resolve is two sequential guarded writes with two audit records, and a blank effort field stays unchanged, proven by an e2e modal-through-guard test | VERIFIED (regression) | `TestTaskResolveIsTwoGuardedWrites` passes in the full `-race` suite. `workitems.go` and `effort.go` are unchanged since the prior report |
| 3 | SC3: A structural test fails on a direct `ado` mutation from `internal/ui`. Every TUI write runs with the real project and a bounded context | VERIFIED (regression) | `TestNoUIFileCallsADOMutation` passes. The bound is still `fetchctx.NewWithParent(ctx, opts.Budget)` (`guard.go:113`), and the parent is now `writeParent`, which has no deadline. `TestRunBoundsContextWithoutParentDeadline` passes. The other `context.Background()` uses left in `internal/ui` are all reads: `detail.go:80,164`, `workitems.go:264` and `trigger_modal.go:175`. Caveat WR-07 is still open |
| 4 | SC4: Every CLI or TUI audit record carries `actor` and `user_approved` | VERIFIED | Prior evidence still holds. New: the interrupted TUI record carries `actor: tui` and `user_approved: false` (`inflight_test.go:89`, PASS) |
| 5 | SC5/GUARD-06: The CLI write tests pass unchanged (approved exception). The shared guard never prints or exits | VERIFIED | `shasum -a 256 -c` gives 8/8 OK. `comm -23` on the inventory prints only `TestWriteRecoverPrecedesMutate`. The `main.go` diff since `18e898c` leaves the CLI branch (L129-139) untouched. `tuiexit_test.go` is the only new CLI-package test file. `TestGuardNeverPrintsOrExits` passes |
| 6 | GUARD-01: the guarded sequence exists once (`guard.Run`) and both surfaces call it | VERIFIED | Non-test `guard.Run(` call sites: `write.go:166` (CLI) and `batch.go:66,96` (TUI). Nothing else |
| 7 | The sink opens before any pre-read or mutation, and an unopenable sink blocks the write (D-10/D-13) | VERIFIED (regression) | `guard.go:136` precedes `Plan` at L146. Tests pass |
| 8 | Declined or discarded edits send nothing and are not audited (D-04) | VERIFIED (regression) | Tests pass in the full suite |
| 9 | Audit files are created 0600 and directories 0700; existing modes are untouched (D-11) | VERIFIED (regression) | `audit.go` is unchanged. Tests pass |
| 10 | PR, release and pipeline writes go input → pre-read → diff confirm → guarded write | VERIFIED (regression) | Views unchanged. Tests pass |
| 11 | Tab switch and quit with staged edits confirm first, esc cancels, and failed or unknown cancels (D-02, D-03) | VERIFIED (regression) | `app.go` is unchanged. Tests pass |
| 12 | A TUI write interrupted mid-flight is audited exactly like a CLI write. The process cannot exit between the mutation and its audit append | VERIFIED | **Previously FAILED (CR-01), now closed.** See sub-truths 12a-12g below. The invariant is exercised by a passing behavioral test (`TestExecuteInterruptedMidMutateIsAuditedUnknown`). `main`'s ordering is straight-line code, read directly at `main.go:155-164`. bubbletea v1.3.10 has no `os.Exit` anywhere in `tea.go`, so every exit path returns through `p.Run()` to `confirm.Wait()` |
| 13 | At execution the guard re-reads and only writes changes the modal showed | VERIFIED (regression) | `approves()` is unchanged (`batch.go:112-121`). `TestExecuteRefusesChangedDiff` passes |
| 14 | An effort failure after a state success reports both and sends no compensating write (D-09) | VERIFIED (regression) | Tests pass |
| 15 | Only `y` confirms a write | VERIFIED (regression) | `TestModalOnlyYWrites` passes. WR-04 is still open |

**Score:** 14/15 headline truths verified. 1 is UNCERTAIN (the SC1 spec reading) and 0 are FAILED. Separately, sub-truth 12f is present but behavior-unverified.

#### Truth 12 sub-truths (07-09 must_haves)

| # | 07-09 truth | Status | Evidence |
|---|-------------|--------|----------|
| 12a | A parent cancelled while Mutate is in flight produces `unknown`, exactly one record, actor `tui` and `user_approved` false | VERIFIED | `TestExecuteInterruptedMidMutateIsAuditedUnknown` passes under `-race` (run by me). It asserts 1 record, `MutateErr` is `context.Canceled`, `AuditErr` is nil, and Mutate ran exactly once. `guard.go:200` reads `classify(err, ctx.Err())` with `ctx` as the parent, and `classify` returns unknown first when `parentErr != nil` (L252-255) |
| 12b | `confirm.Wait` holds while an Execute is between Mutate and the append, and returns once the record is on disk | VERIFIED | The same test: `waited` stays open for 100 ms with 0 records on disk, then closes within 5 s of the cancel, with 1 record on disk. `batch.go:87-88` makes `inflight.Add(1)` and `defer inflight.Done()` the first two statements of `Execute` |
| 12c | main's TUI branch: NotifyContext(os.Interrupt, SIGTERM), deferred stop, SetParent before Run, then Wait after Run with no return between | VERIFIED | Read directly: `main.go:155` NotifyContext, `:156` `defer stopTUISignals()`, `:157` `confirm.SetParent(tuiSignalCtx)`, `:160` `_, err = p.Run()`, `:163` a bare top-level `confirm.Wait()`, `:164` `if err != nil`. `TestTUIWaitsForInflightWriteBeforeExit` passes. Caveat WR-01 (07-09-REVIEW) is a test-strength issue, not a code issue: see Anti-Patterns |
| 12d | Neither TUI `guard.Run` uses the root background context; a write that starts after the signal sends nothing and is not audited | VERIFIED | Non-comment `guard.Run(context.Background()` count in `batch.go` is 0, and `guard.Run(writeParent` count is 2. `TestCancelledParentSendsNothing` passes: Preview and Execute both return `PreReadErr` of `context.Canceled`, Mutate is never called, and there are 0 records |
| 12e | The wait always ends | VERIFIED | Each `guard.Run` is bounded by `fetchctx.NewWithParent` (`TestRunBoundsContextWithoutParentDeadline` passes). A cancelled parent ends Mutate at once (12a: Wait returned within 5 s on a 1-minute budget). IN-01 (07-09-REVIEW): audit file I/O has no context bound and the bound depends on Plan and Mutate honoring ctx. Neither is a hang in practice |
| 12f | bubbletea keeps its own signal handling and TUI exit codes are unchanged; the signal context is not handed to the program | PRESENT_BEHAVIOR_UNVERIFIED | Static parts verified. `main.go:142` is `tea.NewProgram(model, tea.WithAltScreen())`, unchanged, and `WithContext` appears 0 times in non-test code. Source trace: SIGTERM leads to `QuitMsg` (`tea.go:298-302`), which leads to `eventLoop` returning `model, nil` (L402-403), then `killed=false` and a graceful `shutdown` that calls `restoreTerminalState` (L722-745), so the exit is `exitOK`. No test drives a real signal. Routed to the live-signal human item |
| 12g | The CLI path and the locked CLI tests are unchanged | VERIFIED | Same evidence as truth 5 |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/ui/confirm/batch.go` | inflight WaitGroup, `writeParent`, `SetParent`, `Wait`; both `guard.Run` calls take `writeParent` | VERIFIED | 153 lines. `gsd-tools verify.artifacts` passes. Wired: `SetParent` and `Wait` are called from `main.go`, and `Execute` is called from `flow.go:107` |
| `internal/ui/confirm/inflight_test.go` | cancel-mid-Mutate test, cancelled-parent test | VERIFIED | 167 lines. Both tests pass under `-race` |
| `cmd/adoBizInt/main.go` | TUI signal context to `SetParent`; `Wait` after `p.Run()` before the error branch | VERIFIED | See 12c |
| `cmd/adoBizInt/tuiexit_test.go` | AST pin of the exit wiring | VERIFIED (weak pin, WR-01/WR-02) | Passes, and is non-vacuous for the documented mutation (moving Wait below the error branch fails rule 2, which I reproduced). It does not catch every regression: see Anti-Patterns |
| `07-REVIEW-DISPOSITION.md` | CR-01 `fixed`, Source `07-09`; the other 14 unchanged | VERIFIED | Row `| CR-01 | critical | fixed | 07-09 |`, `open: 14`, `total: 15`. The other 14 entries are `open` with Source `-` |
| Prior artifacts (`internal/guard/*`, `internal/fetchctx`, `confirm/{modal,flow}.go`, views, `app.go`, `write.go`, `actor_test.go`, locks) | as in the previous report | VERIFIED (regression) | None changed since `698bd4a`. `git diff --stat 698bd4a HEAD` outside `.planning` lists only the four 07-09 files |

### Key Link Verification

`gsd-tools verify.key-links` cannot evaluate these links: the `from:` fields are prose, not file paths, as in the previous report. I checked each one by hand:

| From | To | Via | Status |
|------|----|-----|--------|
| `main.go` TUI branch | `confirm.SetParent` | `confirm\.SetParent\(`: 1 hit (L157), argument `tuiSignalCtx` from the NotifyContext at L155 | WIRED |
| `main.go` after `p.Run()` | `confirm.Wait` | `confirm\.Wait\(\)`: 1 non-comment hit (L163), between L160 and L164 | WIRED |
| `batch.go` Preview/Execute | `guard.Run` | `guard\.Run\(writeParent`: 2 hits (L66, L96), with Execute inside `inflight.Add/Done` | WIRED |
| `guard.Run` | `appendAuditRecord` | `classify\(err, ctx\.Err\(\)\)`: 1 hit (`guard.go:200`); unknown is appended at L207-213 | WIRED |
| Prior links (`write.go` to `guard.Run`, the fetchctx bound, `OpenAuditSink`, the actor, the views to `Flow.Start`, staging to `SetState`, `app.go` leave/replay) | | | WIRED (unchanged files) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| TUI guard parent | `writeParent` | `signal.NotifyContext` in `main.go:155`, through `SetParent` | Yes: the OS signal cancels it | FLOWING |
| Interrupted record `outcome` | `classify(err, ctx.Err())` | parent `ctx.Err()` | Yes | FLOWING |
| Process exit hold | `inflight` counter | `Execute` Add/Done, then `Wait` in `main` | Yes | FLOWING |
| Modal lines, actor, effort pointers | as in the previous report | | Yes | FLOWING (unchanged) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet | `go build ./... && go vet ./...` | exit 0 | PASS |
| gofmt | `gofmt -l .` | empty | PASS |
| Full suite, run once | `go test -race -count=1 ./...` | every package `ok` | PASS |
| Interrupted write audited unknown; sends nothing after cancel | `go test -race -count=1 -v -run '^(TestExecuteInterruptedMidMutateIsAuditedUnknown\|TestCancelledParentSendsNothing)$' ./internal/ui/confirm/` | 2 PASS | PASS |
| Exit wiring pin plus locked CLI/TUI split | `go test -race -count=1 -v -run '^(TestTUIWaitsForInflightWriteBeforeExit\|TestValidateGuardsCLIAndTUI)$' ./cmd/adoBizInt/` | 2 PASS | PASS |
| All 41 phase tests named in this report exist | `go test -list '.*'` enumeration | 41/41 found | PASS |
| CLI lock (GUARD-06) | `shasum -a 256 -c 07-cli-tests.sha256 07-cli-audit-test.sha256` | 8/8 OK | PASS |
| Inventory | `comm -23 07-cli-test-inventory.txt <(go test -list ...)` | `TestWriteRecoverPrecedesMutate` only | PASS |
| Pin vacuity (control) | scratch copy, Wait moved below the error branch | FAIL on rule 2 | PASS (pin bites) |
| Pin strength (WR-01) | scratch copy, variant A (Wait inside `if err != nil`) and variant B (`go confirm.Wait()`) | both `ok` | WARNING (pin misses both) |
| Real SIGTERM to a live TUI mid-write | none possible without a TTY and live ADO | n/a | SKIP: human item |

The scratch checks ran on copies under the OS temp dir. The parsed `main.go` was build-tagged out, so nothing in the repo was edited.

### Probe Execution

Step 7c: SKIPPED. No phase plan or summary declares a probe, and there is no `scripts/` directory.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| GUARD-01 | 07-01, 07-02, 07-09 | One shared guarded sequence; no stdout, no exit | SATISFIED | Truths 5 and 6. 07-09 keeps the guard unchanged and adds only the TUI exit wiring |
| GUARD-02 | 07-02, 04, 05, 06, 07, 09 | All TUI writes go through the guard with the real project and a bounded context; structural test | SATISFIED | Truths 3 and 12. The interrupted-write concurrency edge is now resolved. Caveat WR-07 is still open |
| GUARD-03 | 07-06, 07, 08 | Stage per item, confirm once on move-on, one record per changed field; compound resolve is two guarded writes | NEEDS HUMAN | Staging, confirm-once and the two-write resolve are verified. The "per changed field" reading is UNCERTAIN (truth 1) |
| GUARD-04 | 07-03, 04, 05, 09 | `actor` and `user_approved` on every record | SATISFIED | Truth 4, now including the interrupted TUI record |
| GUARD-05 | 07-07 | Blank effort is absent, not zero; e2e test | SATISFIED | Truth 2 |
| GUARD-06 | 07-01, 02, 03 | CLI tests pass unchanged (approved exception) | SATISFIED | Truth 5 |

No orphaned requirements: REQUIREMENTS.md maps only GUARD-01..06 to Phase 7, and every one is claimed by at least one plan. REQUIREMENTS.md still shows GUARD-01 as "Gaps Found" and GUARD-02..06 as "Pending". I did not change that. The orchestrator owns those updates.

### Prohibitions (ADR-550)

| Plan | Prohibition | Tier | Disposition |
|------|-------------|------|-------------|
| 07-09 | Must NOT let the process exit while a TUI write is between Mutate and its append (key quit, SIGTERM, non-TTY SIGINT, recovered panic) | test | Enforced. `TestExecuteInterruptedMidMutateIsAuditedUnknown` (Wait hold) and `TestTUIWaitsForInflightWriteBeforeExit` (wiring). WARNING: the wiring pin misses variants A, B and C (WR-01), so it is a partial regression guard |
| 07-09 | Must NOT retry, re-send or compensate an interrupted TUI write | test | Enforced: `mutates.Load() == 1` at `inflight_test.go:112` |
| 07-09 | Must NOT edit the locked CLI tests or the CLI dispatch signal handling | test | Enforced: shasum 8/8 OK; the CLI branch is not in the diff |
| 07-08 | Must NOT terminate the process while a guarded write is executing | test | **Now enforced** for q/ctrl+c (`TestQuitHeldWhileWriteBusy`) and for SIGTERM, non-TTY SIGINT and recovered panics (07-09 tests). Residuals R1-R3 are accepted in the 07-09 threat model |
| 07-04 | No automatic retry after a failed or unknown outcome | judgment | unverified-prohibition: human review recommended. Non-authoritative verdict: holds (see the human item) |
| 07-01..07-08 | All other prohibitions in the previous report | test | Unchanged and still enforced. Their files are unchanged and their tests pass |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| 4 files changed in 07-09 | n/a | TBD/FIXME/XXX/TODO/HACK/placeholder | n/a | None found |
| `cmd/adoBizInt/tuiexit_test.go` | 73-96, 103-111 | `only` matches any statement whose subtree holds the call, not a bare call (07-09-REVIEW WR-01) | Warning | **Reproduced by me in scratch copies.** Variant A (`confirm.Wait()` moved inside `if err != nil`) and variant B (`go confirm.Wait()`) both pass the test while reopening CR-01. The production code is correct today. The risk is a silent future regression |
| `cmd/adoBizInt/tuiexit_test.go` | n/a | Design G-03 (signal context NOT handed to bubbletea) has no lasting pin (07-09-REVIEW WR-02) | Warning | A later `tea.WithContext(tuiSignalCtx)` would hang shutdown with a raw terminal and turn SIGTERM exit 0 into 1, and no test would fail. Today the grep count is 0 |
| `internal/ui/confirm/batch.go` | 30-36 | `Wait` doc says "cannot hang", which overstates the bound (07-09-REVIEW IN-01) | Info | Audit file I/O has no context bound. Doc only |
| 07-REVIEW WR-01..WR-08, IN-01..IN-06 | various | Carried forward. `open` in 07-REVIEW-DISPOSITION.md, out of gap scope by user decision | Warning/Info | Non-blocking. Most consequential: WR-01 (modal truncation hides the approved value), WR-04 (a second confirm decision is not final), WR-05 (silent re-stage across items), WR-07 (the GUARD-02 test misses method values and aliased imports), WR-02 (the `cli-human` label is a TTY heuristic and needs a compliance caveat before Phase 13) |

Accepted residuals from the 07-09 threat model are recorded, not gaps:
- **R1:** an Execute that starts after `Wait` returns. Whether it reaches Mutate before `os.Exit` depends on timing. To get there, a full pre-read round trip has to finish between `Wait` and the deferred `stopTUISignals`.
- **WaitGroup reuse:** the edge noted in the 07-09 SUMMARY, which needs two Executes in flight. Unreachable today: there is one Execute call site and the busy, tab and quit holds allow one write at a time.
- **R2:** a signal absorbed in the startup window. The TUI then refuses every write at its pre-read.
- **R3:** SIGKILL, SIGHUP and power loss. Same exposure as the CLI.
- **R4:** a silent wait of up to a write's budget after a recovered panic.

None of these lets ADO change with no record on a path the CLI covers.

### Human Verification Required

The frontmatter `human_verification` lists 9 items:
1. **Live SIGTERM during a real TUI write.** New. It also covers sub-truth 12f.
2. **PRs tab confirm modal.** Live ADO, plus 80x24 rendering.
3. **Releases and Pipelines confirm.** Live ADO.
4. **Work Items staging.** Live ADO, plus 80x24 layout.
5. **Task resolve with blank Original Estimate.** Check ADO History.
6. **Tab and quit with staged edits.** Real terminal.
7. **Narrow-terminal error and unknown text.**
8. **SC1 "one audit record per changed field" vs SC2.** Spec decision.
9. **Judgment-tier no-retry prohibition.** Sign-off.

No new evidence resolves items 2-9. The source behind them is unchanged, and each needs live ADO, a real terminal or a human decision.

### Gaps Summary

**No blocking gaps remain. CR-01 is closed.** The previous report's single failed truth is now satisfied in the code:
- `confirm/batch.go` gives both TUI `guard.Run` calls a signal-aware parent (`writeParent`, installed from `main`'s `signal.NotifyContext(os.Interrupt, SIGTERM)`).
- `Execute` counts itself in an `inflight` WaitGroup.
- `main` waits on that WaitGroup after `p.Run()` and before the error branch.

A SIGTERM, a non-TTY SIGINT or a recovered panic during a TUI write therefore ends with the write classified (`unknown` if the signal fired) and appended with actor `tui` before the process exits. That is the same D-22 path the CLI takes.

**Do 07-09-REVIEW WR-01 and WR-02 weaken the proof? Not for goal achievement today.** They are regression-guard gaps, not defects in the code. I verified the wiring by reading `main.go` directly. The behavioral half (Wait holds until the record is on disk, and cancellation yields `unknown`) is proven by a passing test that does not rely on the AST pin. I reproduced WR-01: the pin passes when `confirm.Wait()` is moved into the error branch or made `go confirm.Wait()`. A future edit can therefore silently reopen CR-01. WR-02 means the G-03 hang hazard has no lasting guard. Both should be fixed before code that depends on this guarantee ships: Phase 13 agent writes and SOX evidence. The fix is small and is given in 07-09-REVIEW.md. Both are WARNING, not BLOCKER.

**Remaining work is human, not code.** Nine human items are listed above, including one spec decision (SC1 vs SC2) that also governs Phase 16 SC3. The 07-REVIEW warnings WR-01..WR-08 and IN-01..IN-06 stay open and non-blocking, per the user's "CR-01 only" scope. WR-01 (modal truncation) and WR-04 (decision not final) both weaken the "y means write what I saw" guard and are the best candidates for the next triage.

---

_Verified: 2026-10-16T14:01:19Z_
_Verifier: the agent (gsd-verifier)_
