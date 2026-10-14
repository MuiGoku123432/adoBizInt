---
phase: 07-guarded-tui-writes
verified: 2026-10-14T05:24:38Z
status: gaps_found
score: 13/15 must-haves verified
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
  - "cmd/adoBizInt/actor_test.go"
  - "cmd/adoBizInt/audit.go"
  - "cmd/adoBizInt/audit_test.go"
  - "cmd/adoBizInt/crash_test.go"
  - "cmd/adoBizInt/dispatch.go"
  - "cmd/adoBizInt/exit.go"
  - "cmd/adoBizInt/flags.go"
  - "cmd/adoBizInt/guardnames_test.go"
  - "cmd/adoBizInt/main.go"
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
covered_digest: "v3:sha256:719cdd08c506c2ae89649751c264f8bd6cadfc851d225a404143b85cf07d5f59"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "A TUI write that is interrupted mid-flight is audited exactly like a CLI write: the process cannot exit between op.Mutate sending the request and the audit append (phase goal; 07-08 truth 'the process can never exit between a mutation and its audit append'; 07-08 prohibition 'Must NOT terminate the process while a guarded write is executing')"
    status: failed
    reason: "CR-01 is real and not deferred to any later phase. The quit hold covers only the q / ctrl+c key bindings. Bubbletea v1.3.10 turns SIGTERM into QuitMsg, and SIGINT on a non-TTY stdin into InterruptMsg. The event loop handles both before the model sees them (tea.go:402-406). Program.Run deliberately does not wait for command goroutines (tea.go:347-351, 'Don't wait on these goroutines'). main then calls os.Exit(guard(run)), which kills an in-flight confirm.Execute goroutine. The TUI passes context.Background() to guard.Run (confirm/batch.go:36,57), so the guard's classify(err, ctx.Err()) interrupt path (D-22) can never fire for a TUI write. The CLI side differs: main.go:134 wraps dispatch in signal.NotifyContext(os.Interrupt, SIGTERM), so a signal during a CLI write cancels the parent context, Mutate returns, classify returns unknown and the record is appended. A SIGTERM during a TUI write leaves ADO possibly changed and no audit record. That is the Phase 4 D-13 window, and it breaks 'audited exactly like a CLI write' for SOX ITGC evidence. A panic in any other TUI cmd goroutine (recoverFromGoPanic) also kills the process mid-write. SIGHUP is NOT a divergence: the CLI does not handle it either."
    artifacts:
      - path: "internal/ui/confirm/batch.go"
        issue: "guard.Run(context.Background(), ...) at lines 36 and 57: no signal-derived parent, so an interrupt is never classified unknown and audited. No in-flight tracking that process exit could wait on"
      - path: "cmd/adoBizInt/main.go"
        issue: "TUI branch (lines 140-151): tea.NewProgram has no WithContext / signal context, and nothing waits for an in-flight guarded write after p.Run() returns before os.Exit"
      - path: "internal/ui/app.go"
        issue: "Quit hold (lines 93-101) intercepts only the Quit key binding; QuitMsg/InterruptMsg from signals bypass Model.Update"
    missing:
      - "Track in-flight confirm.Execute calls (for example a package-level sync.WaitGroup in confirm with Add/Done around Execute and an exported Wait) and call it in main.go after p.Run() returns, before returning the exit code"
      - "Give the TUI's guard.Run a signal-aware parent context (signal.NotifyContext for os.Interrupt and SIGTERM, also passed via tea.WithContext or threaded to confirm). A SIGTERM during a TUI write then classifies as unknown and is audited, matching CLI D-22"
      - "A test that cancels the parent context mid-Mutate on the TUI path and asserts exactly one 'unknown' audit record with actor tui"
human_verification:
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
    expected: "Human sign-off. Non-authoritative LLM verdict: holds. confirm.Execute runs each op at most once and breaks on the first outcome that is neither executed nor no_op. Flow.settle never calls Start again. The views only re-fetch (read) on Refresh"
    why_human: "unverified-prohibition: judgment tier, so human review is recommended"
---

# Phase 7: Guarded TUI Writes Verification Report

**Phase Goal:** Every write made from the TUI is guarded and audited exactly like a CLI write, through one guard both surfaces share
**Verified:** 2026-10-14T05:24:38Z
**Status:** gaps_found
**Re-verification:** No. This is the initial verification.

## Goal Achievement

The shared guard exists and both surfaces use it. Every one of the seven TUI write call sites now goes through `confirm/batch.go` → `guard.Run` with `ActorTUI`, the real project and a bounded context. On the normal path, TUI audit records match CLI records: the same `Run` body, the same 7-key record and the same sink. The goal is missed on one path. A TUI write interrupted by a signal is not audited the way the CLI audits it (CR-01).

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: A TUI state/effort change shows the pre-read diff, refuses a no-op without a request, is confirmed once per item on move-on, and leaves one audit record per changed field | ? UNCERTAIN | Diff, no-op and confirm-once are verified: `TestStagedStateConfirmsOnRowMove` and `TestNoOpSkipsModalAndMoves` pass, and `n` only stages (`staging.go:83`). A state change yields 1 record for 1 field. A resolve's effort record bundles several fields: the e2e test pins `[RemainingWork, CompletedWork]` in one record. That matches SC2's "two audit records", but not a literal per-field reading. Spec decision for the human (see Human Verification) |
| 2 | SC2: A TUI resolve is two sequential guarded writes with two audit records. A blank effort field stays unchanged, proven by an e2e test driving the modal through the guard | ✓ VERIFIED | `TestTaskResolveIsTwoGuardedWrites` (run, PASS) types into the effort modal with tab-skipped blank OE, confirms via modal `y`, and asserts `UpdateWorkItemState` then `UpdateWorkItemStateWithEffort` with `oe == nil`, plus 2 audit records with actor `tui`. `EffortSubmitMsg` carries the `*float64` directly into `guard.Effort` (`workitems.go:352-356`) |
| 3 | SC3: A structural test fails if any `internal/ui` file calls an `ado` mutation directly. Every TUI write runs with the real project and a bounded context | ✓ VERIFIED | `TestNoUIFileCallsADOMutation` derives the forbidden set default-deny from `*ado.Client` and has 2 tripwires (PASS). A grep finds no `.Approve*/.Complete*/.Reject*/.QueuePipeline/.UpdateWorkItemState*` calls on a client under `internal/ui`, only `guard.*` op constructors. Projects come from `pr.Project`, `m.approval.Project`, `def.Project` and `s.item.Project`. The bound is `fetchctx.NewWithParent(ctx, opts.Budget)` in `guard.go:113`, proven by `TestRunBoundsContextWithoutParentDeadline`. Caveat WR-07: method-value and aliased-import forms are not caught |
| 4 | SC4: Every audit record from the CLI or the TUI carries `actor` (`tui`/`cli-human`/`agent`) and `user_approved` | ✓ VERIFIED | `auditRecord` has `Actor` and `UserApproved` with no omitempty (`audit.go:66-74`). `Run` refuses any other actor before opening the sink (`guard.go:121-126`, `TestRunRefusesUnknownActor`). CLI: `Actor: cliActor()` from a stdin TTY check, with no flag or env (`TestActorIsNotAFlag`, `TestExecutedCLIWriteRecordsActor`). TUI: `TestUIGuardCallsCarryTUIActor` |
| 5 | SC5/GUARD-06: The existing CLI write tests pass unchanged, within the approved Phase 7 exception. The shared guard never writes stdout and never exits | ✓ VERIFIED | `shasum -a 256 -c` gives OK on both manifests (7 files, and audit_test.go as re-pinned in 07-03). `git diff 83f9918 HEAD` on the CLI tests shows exactly the three approved assertion edits plus the removal of `TestWriteRecoverPrecedesMutate`, which now lives in `internal/guard/structural_test.go` as `TestRunRecoverPrecedesMutate`. The inventory diff is only `-TestWriteRecoverPrecedesMutate`, plus 3 new tests in `actor_test.go`. `TestGuardNeverPrintsOrExits` (AST scan plus tripwire) PASS |
| 6 | GUARD-01: The guarded sequence exists once, in `internal/guard.Run`, and both surfaces call it | ✓ VERIFIED | `guard.go:106-235` (actor check → open sink → Plan → no-op → confirm → recover → Mutate → classify → append). CLI `runWrite` calls `writeguard.Run(ctx, gop, ...)` (`write.go:166`) and carries no copy of the sequence. TUI calls it only from `confirm/batch.go` |
| 7 | The audit sink opens before any pre-read or mutation. An unopenable sink blocks the write on both surfaces (D-10/D-13) | ✓ VERIFIED | `TestRunOpensSinkBeforePreRead` and `prs/TestUnwritableSinkBlocksWrite` PASS. Status reads `write blocked: audit log not writable: <path>` (`flow.go:145`) |
| 8 | Declined or discarded edits send nothing and write no audit record (D-04) | ✓ VERIFIED | `TestRunUnconfirmedIsDryRunAndUnaudited`, `TestDiscardSendsNothing`, `TestDiscardDropsStagedAndMoves` and `TestStayKeepsStagedAndPosition` PASS |
| 9 | New audit files are 0600 and new directories 0700; existing modes are untouched (D-11) | ✓ VERIFIED | `audit.go:39-43`, `TestOpenAuditSinkCreatesOwnerOnlyFileAndDir`, `TestOpenAuditSinkLeavesExistingModeAlone` |
| 10 | PR approve/complete, release approve/reject and pipeline queue go input → pre-read → diff confirm → guarded write (D-05, D-06) | ✓ VERIFIED | `TestApproveRunsThroughGuard`, `TestCompleteRunsThroughGuardAfterConfirm`, `TestApproveSubmitPreviewsThenConfirms`, `TestRejectUsesRejectAfterConfirm` and `TestQueueSubmitPreviewsThenConfirms` PASS. `confirmComplete`/`PRActionMsg` are gone from prs.go |
| 11 | Tab switch and quit with staged edits confirm first. esc cancels the leave. A failed or unknown outcome cancels it. With nothing staged and nothing busy, behaviour is unchanged (D-02, D-03) | ✓ VERIFIED | `app.go:84-110` with `TestQuitWithStagedEditConfirmsFirst`, `TestTabSwitchWithStagedEditDiscardProceeds`, `TestEscKeepsViewAndStagedEdit`, `TestUnknownOutcomeCancelsQuit` and `TestPlainQuitAndTabUnchanged` PASS |
| 12 | A TUI write interrupted mid-flight is audited exactly like a CLI write. The process cannot exit between the mutation and its audit append | ✗ FAILED | CR-01 (see Gaps). The key-bound quit is held (`TestQuitHeldWhileWriteBusy`). SIGTERM goes to `QuitMsg`, which bypasses the model, and `p.Run()` does not wait for command goroutines, so `os.Exit` kills `Execute`. The TUI parent context is `context.Background()`, so there is no D-22 unknown classification. The CLI handles SIGINT/SIGTERM via `signal.NotifyContext` (`main.go:134`) |
| 13 | At execution the guard re-reads and only writes changes the modal showed | ✓ VERIFIED | `approves()` subset check (`batch.go:73-82`), `TestExecuteRefusesChangedDiff` PASS |
| 14 | If the effort write fails after the state write succeeds, both results are reported, the item is re-fetched and no compensating write is sent (D-09) | ✓ VERIFIED | `TestResolveEffortFailureReportsBoth` asserts exactly two calls, "no compensation". `TestUserStoryResolveClosesDevTaskThenRecordsEffort` covers story → dev task → effort under the story's project, with the guard's project check at `ops.go:116` |
| 15 | Only `y` confirms a write; enter, space and other keys do nothing | ✓ VERIFIED | `TestModalOnlyYWrites` PASS. Caveat WR-04: the decision is delivered asynchronously while the modal stays open, so pressing n then y quickly can still write |

**Score:** 13/15 truths verified (0 present, behavior-unverified; 1 FAILED, 1 UNCERTAIN)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/fetchctx/fetchctx.go` | relocated New/NewWithParent | ✓ VERIFIED | `internal/ui/fetchctx` is gone. 6 UI views use `fetchctx.New(budget)` |
| `internal/guard/guard.go` | Run, Options, Report, actor constants | ✓ VERIFIED | 286 lines, used by the CLI and `confirm/batch.go` |
| `internal/guard/audit.go` | 0700/0600 opener, 7-key record | ✓ VERIFIED | |
| `internal/guard/ops.go` | Client, SetState, ApprovePR, CompletePR, release ops, QueuePipeline | ✓ VERIFIED | |
| `internal/guard/guardtest/fake.go` | fail-by-name fake | ✓ VERIFIED | |
| `internal/guard/structural_test.go` | relocated D-12 pin, no-print/exit scan | ✓ VERIFIED | |
| `internal/guard/uibypass_test.go` | GUARD-02 structural test plus actor test | ✓ VERIFIED (WR-07 gaps) | |
| `internal/ui/confirm/{modal,batch,flow}.go` | diff modal, Preview/Execute, Flow | ✓ VERIFIED | |
| `internal/ui/workitems/staging.go` | stagedEdit, effortEdit, leave API | ✓ VERIFIED | |
| `internal/ui/app.go` | pendingLeave, quit hold | ✓ VERIFIED (key path only; see gap) | |
| `cmd/adoBizInt/write.go` | thin adapter, cliActor | ✓ VERIFIED | |
| `cmd/adoBizInt/actor_test.go` | actor tests | ✓ VERIFIED | |
| `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` | "Approved Phase 7 exception" | ✓ VERIFIED | Both name the 3 assertions and the 1 relocated test by exact Go name |
| 07-cli-tests.sha256 / 07-cli-audit-test.sha256 / inventory | locks | ✓ VERIFIED | Both manifests check OK. The inventory has 85 names |

`gsd-tools verify.artifacts`: every plan passes, except that 07-06 reports "Missing pattern: flow confirm.Flow". This is a false negative caused by column-aligned whitespace: `workitems.go:95` reads `flow       confirm.Flow`.

### Key Link Verification

`gsd-tools verify.key-links` could not evaluate these links: every plan's `from:` is a prose descriptor, not a file path. I checked each pattern by hand:

| From | To | Via | Status |
|------|----|-----|--------|
| write.go runWrite | guard.Run | `guard\.Run\(ctx` | WIRED |
| guard.Run | fetchctx.NewWithParent | `fetchctx\.NewWithParent\(ctx, opts\.Budget\)` | WIRED |
| audit.go openAuditSink | guard.OpenAuditSink | `guard\.OpenAuditSink\(auditPath\)` | WIRED |
| write.go runWrite | Options.Actor | `Actor:\s+cliActor\(\)` | WIRED |
| guard.Run | newAuditRecord | normal and panic paths (2 hits) | WIRED |
| prs / releases / pipelines | confirm Flow.Start | `flow\.Start\(` | WIRED (1 each) |
| confirm batch.go | guard.Run | `guard\.Run\(context\.Background\(\)` | WIRED (2). This is also the CR-01 root: no signal parent |
| staging.go batch | guard.SetState | `guard\.SetState\(w, s\.item\.Project` | WIRED (3) |
| workitems.go key branch | startConfirm | `pendingKey` | WIRED |
| workitems.go EffortSubmitMsg | stage | `effortMsg\.OriginalEstimate` | WIRED |
| app.go | workitems.ConfirmLeave / ProceedMsg replay | `ConfirmLeave\(\)`, `pendingLeave` | WIRED |
| 07-01 write.go → fetchctx.NewWithParent | | | SUPERSEDED by the 07-02 link: the guard now derives the context |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| confirm modal lines | `Batch.Lines()` | `guard.Run` dry-run `Report.Changes` from live `Plan` pre-read | Yes | ✓ FLOWING |
| audit record `actor` | `opts.Actor` | `guard.ActorTUI` constant (TUI) / `cliActor()` TTY check (CLI) | Yes | ✓ FLOWING |
| effort pointers | `EffortSubmitMsg.*` → `guard.Effort` → `UpdateWorkItemStateWithEffort` | effort modal `parseEffort` | Yes, nil preserved | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet | `go build ./... && go vet ./...` | exit 0 | ✓ PASS |
| Full suite (run once) | `go test -count=1 ./...` | all packages ok | ✓ PASS |
| 28 named phase tests under the race detector | `go test -race -run '<named>' ./internal/guard/ ./internal/ui/... ./cmd/adoBizInt/` | all PASS | ✓ PASS |
| CLI lock | `shasum -a 256 -c 07-cli-tests.sha256 07-cli-audit-test.sha256` | 8/8 OK | ✓ PASS |
| Signal-interrupted TUI write is audited | no test exists; source traced in bubbletea v1.3.10 `tea.go:286-305,347-351,402-406` | process exits without waiting | ✗ FAIL (CR-01) |

### Probe Execution

Step 7c: SKIPPED. The phase declares no probes and there is no `scripts/` directory.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| GUARD-01 | 07-01, 07-02 | One shared guarded sequence; no stdout, no exit | ✓ SATISFIED | Truths 5, 6 |
| GUARD-02 | 07-02, 04, 05, 06, 07 | All TUI writes through the guard with the real project and a bounded context; structural test | ✓ SATISFIED | Truth 3 (WR-07 caveat) |
| GUARD-03 | 07-06, 07, 08 | Stage per item, confirm once on move-on, one record per changed field; compound resolve is two guarded writes | ? NEEDS HUMAN | Staging and confirm work (truths 1, 2, 11). The "per changed field" reading is UNCERTAIN (truth 1). WR-05 is a silent re-stage edge |
| GUARD-04 | 07-03, 04, 05 | `actor` and `user_approved` on every record | ✓ SATISFIED | Truth 4 (WR-02: `cli-human` means "stdin is a TTY") |
| GUARD-05 | 07-07 | Blank effort is absent, not zero; e2e test | ✓ SATISFIED | Truth 2 |
| GUARD-06 | 07-01, 02, 03 | CLI tests pass unchanged (approved exception) | ✓ SATISFIED | Truth 5, checked against the recorded exception |

No orphaned requirements: REQUIREMENTS.md maps only GUARD-01..06 to Phase 7, and each one is claimed by at least one plan. REQUIREMENTS.md still shows GUARD-02..06 unchecked/Pending. That is deliberate (commit 4740b9e), pending this verification.

### Prohibitions (ADR-550)

| Plan | Prohibition | Tier | Disposition |
|------|-------------|------|-------------|
| 07-01 | No edits to cmd/adoBizInt/*_test.go in this plan | test | ✓ Enforced: commits 059a4d1 and 83f9918 touch no CLI test file, and the manifest was created there |
| 07-02 | No actor/user_approved in this plan | test | ✓ Enforced: the 6b7a1d9 record had 5 keys and the locked tests passed |
| 07-02 | Guard never re-panics | test | ✓ `TestRunMutatePanicRecordsUnknownOnce`, `TestRunRecoverPrecedesMutate` |
| 07-03 | Actor not settable by flag/env/config | test | ✓ `TestActorIsNotAFlag`. No `Getenv` in cmd/ or guard/ |
| 07-03 | No identity/email/PAT/URL in actor | test | ✓ `TestRunRefusesUnknownActor` (closed set) |
| 07-04 | Only y confirms | test | ✓ `TestModalOnlyYWrites` (WR-04 race noted) |
| 07-04 | No mutation when the fresh diff differs | test | ✓ `TestExecuteRefusesChangedDiff` |
| 07-04 | No automatic retry after failed/unknown | judgment | ⚠ unverified-prohibition: human review recommended. Non-authoritative verdict: holds |
| 07-05 | Never send straight from the input modal | test | ✓ `TestApproveSubmitPreviewsThenConfirms`, `TestQueueSubmitPreviewsThenConfirms` |
| 07-06 | No write of a staged edit without y | test | ✓ discard/stay tests |
| 07-06 | No staged edit left after a failed preview | test | ✓ `TestUnwritableSinkDropsStagedAndStays` |
| 07-07 | No compensating write | test | ✓ `TestResolveEffortFailureReportsBoth` |
| 07-07 | Blank effort never becomes a pointer to zero | test | ✓ `TestTaskResolveIsTwoGuardedWrites` |
| 07-08 | Must NOT terminate the process while a guarded write is executing | test | ✗ Enforced only for q/ctrl+c (`TestQuitHeldWhileWriteBusy`). Violated on SIGTERM / non-TTY SIGINT / foreign goroutine panic, which is the CR-01 gap |
| 07-08 | No silent drop of staged edits on tab/quit | test | ✓ app tests. WR-05 is a separate in-view silent re-stage |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (40 phase-modified files) | — | TBD/FIXME/XXX/TODO/HACK | — | None found |
| internal/ui/confirm/batch.go | 36, 57 | `context.Background()` parent for guarded writes | 🛑 Blocker | CR-01: interrupt is never classified and audited on the TUI path |
| internal/ui/confirm/flow.go | 88-93 | Modal stays open until the decision message arrives | ⚠️ Warning | WR-04: n then y quickly can write; the first decision is not final |
| internal/ui/confirm/modal.go | 36 | 60-rune truncation cuts the `→ <To>` tail on 5–7 digit IDs | ⚠️ Warning | WR-01: the user can approve a value not shown on screen |
| internal/ui/workitems/workitems.go | 258-366 | Late `ChildTasksLoadedMsg` / `EffortSubmitMsg` replaces another item's staged edit | ⚠️ Warning | WR-05: silent re-stage, which breaks the D-02 spirit |
| internal/guard/uibypass_test.go | 115-129, 185-192 | Method values and aliased guard import not scanned | ⚠️ Warning | WR-07: GUARD-02 test can be bypassed |
| cmd/adoBizInt/write.go | 96-112 | `cli-human` = stdin is a TTY | ⚠️ Warning | WR-02: an agent under a pty is labelled human (SOX SoD evidence). This was a checkpoint decision; it needs the compliance caveat documented before Phase 13 |
| internal/guard/audit.go | 20-23 | `os.UserHomeDir` error discarded | ⚠️ Warning | WR-03: an unset HOME relocates the audit log to CWD (pre-existing, now shared) |
| internal/guard/ops.go | 92-133 | Empty state: diff omits it, Mutate sends `""` | ⚠️ Warning | WR-06: no current caller does this |
| prs/releases/pipelines/workitems | various | Flow-result and sticky render copied 3–4× | ℹ️ Info | WR-08: rule-of-three violation |

### Human Verification Required

Listed in frontmatter `human_verification` (8 items): five deferred live-terminal/ADO checks harvested from the 07-04..07-08 `<human-check>` blocks, one 07-UI-SPEC narrow-terminal backstop, the SC1 per-field audit reading, and the judgment-tier no-retry prohibition. These do not change the status: gaps_found takes precedence.

### Gaps Summary

**One blocking gap (CR-01): the TUI's audit guarantee is weaker than the CLI's under process interruption.** The shared guard is correct, and on every normal path a TUI write is audited the same way a CLI write is. The CLI also survives SIGINT/SIGTERM mid-write: `signal.NotifyContext` cancels the parent, `classify` sees `ctx.Err()`, and the write is recorded `unknown` (Phase 4 D-22). The TUI has no equivalent. Bubbletea converts SIGTERM into a `QuitMsg` that never reaches `Model.Update`, so the quit hold cannot see it. `Program.Run` returns without waiting for command goroutines, and `main` calls `os.Exit` while `confirm.Execute` may sit between a sent PATCH and `appendAuditRecord`. Because the TUI passes `context.Background()`, the guard's interrupt classification is unreachable from the TUI. The result: a change can land in ADO with no audit line. That is the D-13 window and the SOX ITGC evidence gap this phase exists to close. It directly falsifies the 07-08 must-have "the process can never exit between a mutation and its audit append". No later roadmap phase addresses it, so it is not deferred.

The fix is small. Add an in-flight WaitGroup around `confirm.Execute` and wait on it in `main` after `p.Run()`. Pass a signal-derived parent into the TUI's `guard.Run`. Add a test that cancels the parent mid-Mutate and asserts one `unknown` record with actor `tui`.

The developer can instead accept this as residual risk, for example by treating SIGTERM to an interactive TUI as out of scope. To do that, add an override:

```yaml
overrides:
  - must_have: "A TUI write that is interrupted mid-flight is audited exactly like a CLI write: the process cannot exit between op.Mutate sending the request and the audit append"
    reason: "<why SIGTERM / non-TTY SIGINT during an interactive TUI write is an accepted SOX residual risk>"
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

**Non-blocking concerns**, all from 07-REVIEW, all confirmed in code and still `open` in 07-REVIEW-DISPOSITION.md:
- WR-01: modal truncation hides the approved value on real IDs.
- WR-04: the first confirm decision is not final.
- WR-05: silent re-stage across items.
- WR-07: the structural test misses method values and aliased imports.
- WR-02: the `cli-human` heuristic needs a compliance caveat before Phase 13 builds on it.
- WR-03 and WR-06: smaller items.

None of these falsifies a roadmap success criterion on its own. WR-01 and WR-04 both weaken the "y means write what I saw" accident guard and should be triaged with the CR-01 fix.

---

_Verified: 2026-10-14T05:24:38Z_
_Verifier: Claude (gsd-verifier)_
