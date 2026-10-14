---
phase: 07-guarded-tui-writes
plan: 04
subsystem: ui
tags: [go, bubbletea, lipgloss, guarded-writes, audit, tui, tdd]

requires:
  - phase: 07-02
    provides: "internal/guard Run, Options, Report, ApprovePR, CompletePR, DefaultAuditPath, ActorTUI; guardtest.Client fake"
  - phase: 07-03
    provides: "Run validates and persists Options.Actor (tui) and user_approved on every audit record"
provides:
  - "internal/ui/confirm: Model (D-01 diff modal), New, WriteMsg, DiscardMsg, StayMsg"
  - "internal/ui/confirm: Batch (ID, Ops, Labels, NoOpLabel, Reports), Preview, Execute, (Batch).Lines; the TUI's only guard.Run call sites, Actor guard.ActorTUI"
  - "internal/ui/confirm: Flow (Start, Update, Busy, Sticky, View), PreviewedMsg, WrittenMsg, Result (Handled, Status, IsErr, Refresh, Proceed, Stay, Kept, Batch)"
  - "internal/ui/prs: approve and complete through confirm.Flow; WriteBusy(); prStatusClient; confirmComplete, actionPending and PRActionMsg removed"
affects: [07-05, 07-06, 07-07, 07-08]

actuals:
  tokens: 9600
  tasks: 2
  commits: 4
plan_head_before: 4740b9e463f1fa5a72ff26c610ec42ecc1f9d06f
plan_head_after: cd607e4ad81aa6329d016bd652744697919eb4a7

tech-stack:
  added: []
  patterns:
    - "A view runs confirm.Flow.Update first in its own Update; when Result.Handled it applies Status/IsErr, re-fetches on Refresh, and returns"
    - "Execute re-runs the pre-read inside guard.Run and confirms only when every fresh FieldChange was shown (approves)"
    - "Per-press client wrapper (prStatusClient) to name pre-read state the guard Report does not carry"

key-files:
  created:
    - internal/ui/confirm/modal.go
    - internal/ui/confirm/batch.go
    - internal/ui/confirm/flow.go
    - internal/ui/confirm/confirm_test.go
    - internal/ui/prs/write_test.go
  modified:
    - internal/ui/prs/prs.go

key-decisions:
  - "The PR view clears the in-flight 'Writing #<id>...' status whenever the flow settles, so the unknown outcome (Status empty, text on the sticky line) does not leave a stale in-flight status under it"
  - "Flow swallows its own messages when they arrive in the wrong state (a second WriteMsg after the modal closed, a stale PreviewedMsg or WrittenMsg): a double y cannot send two writes"
  - "A later op whose audit sink failed at Execute reads '<label> not written: audit log not writable: <path>' (not reachable today: Execute stops after the first op that did not execute)"
  - "a and c carry no separate busy check: the flow swallows every key while busy and Start is a no-op when busy"

patterns-established:
  - "confirm.Batch per keypress: ID is the row, one Op and one past-tense label per write"
  - "Every test that can reach guard.Run sets t.Setenv(\"HOME\", t.TempDir()) and reads $HOME/.config/adoBizInt/audit.jsonl back"

requirements-completed: [GUARD-02, GUARD-04]

coverage:
  - id: D1
    description: "PR approve runs pre-read, the diff modal, y, and guard.Run with the PR's real project and repository, and appends one executed audit record"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/prs/write_test.go#TestApproveRunsThroughGuard"
        status: pass
    human_judgment: false
  - id: D2
    description: "PR complete uses the same modal (status active → completed); the status-line merge prompt is gone"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/prs/write_test.go#TestCompleteRunsThroughGuardAfterConfirm"
        status: pass
    human_judgment: false
  - id: D3
    description: "Only y writes; n and esc send nothing and write no audit record (D-04); enter, space and Y do nothing"
    verification:
      - kind: unit
        ref: "internal/ui/confirm/confirm_test.go#TestModalOnlyYWrites; internal/ui/prs/write_test.go#TestDiscardSendsNothing"
        status: pass
    human_judgment: false
  - id: D4
    description: "No-op pre-reads skip the modal: 'No change: #7 already approved' and 'No change: #7 already abandoned' (D-07)"
    verification:
      - kind: unit
        ref: "internal/ui/prs/write_test.go#TestAlreadyApprovedIsNoChange|TestAbandonedPRCompleteIsNoChange"
        status: pass
    human_judgment: false
  - id: D5
    description: "Unwritable audit sink blocks the write before any pre-read; the next press previews again (D-10)"
    verification:
      - kind: unit
        ref: "internal/ui/prs/write_test.go#TestUnwritableSinkBlocksWrite"
        status: pass
    human_judgment: false
  - id: D6
    description: "Unknown outcome is a sticky line '#7: outcome unknown — verify in ADO' that survives refreshes until a keypress, and the list is re-fetched (D-08, C3)"
    verification:
      - kind: unit
        ref: "internal/ui/prs/write_test.go#TestUnknownOutcomeIsSticky; internal/ui/confirm/confirm_test.go#TestFlowWrittenTextReportsEachStep"
        status: pass
    human_judgment: false
  - id: D7
    description: "Execute refuses a fresh diff the user did not see (T-07-15)"
    verification:
      - kind: unit
        ref: "internal/ui/confirm/confirm_test.go#TestExecuteRefusesChangedDiff"
        status: pass
    human_judgment: false
  - id: D8
    description: "Auto-refresh skipped and tab switching held while a preview, modal or write is in flight"
    verification:
      - kind: unit
        ref: "internal/ui/prs/write_test.go#TestTickDoesNotRefreshWhileConfirmOpen|TestTabHeldWhileWriteBusy"
        status: pass
    human_judgment: false
  - id: D9
    description: "Modal geometry: Width(64), 60 content columns, a 60-rune line on one row, 66 columns with the border (C1); control runes dropped"
    verification:
      - kind: unit
        ref: "internal/ui/confirm/confirm_test.go#TestModalFitsSixtyRuneLineOnOneRow|TestModalDropsControlRunes|TestBatchLinesDedupeAndPrefix"
        status: pass
    human_judgment: false
  - id: D10
    description: "Real-terminal look of the modal and status lines at 80x24 against a live PR (the plan's human-check)"
    verification: []
    human_judgment: true
    rationale: "Visual fit and readability in a real 80x24 terminal against live ADO cannot be asserted by unit tests; 07-UI-SPEC lists it as a backstop"

duration: 8min
completed: 2026-10-14
status: complete
---

# Phase 7 Plan 04: Shared TUI confirm flow, proven on pull requests Summary

**`internal/ui/confirm` adds the D-01 diff modal (only `y` writes), a Batch runner that holds the TUI's only `guard.Run` calls (actor `tui`), and a Flow state machine for the D-07/D-08/D-10 outcomes and the C3 first-part status rule. PR approve and complete now go through it, with the PR's real project and repository. The status-line merge prompt is gone.**

## Performance

- **Duration:** about 8 min of execution, after reading the context
- **Started:** 2026-10-14T04:00:05Z
- **Completed:** 2026-10-14T04:08:00Z
- **Tasks:** 2 (Task 1 tracer, Task 2 auto; both TDD)
- **Files:** 6 (5 created, 1 modified)

## Final confirm API (later plans depend on it)

- `confirm.New(title string, lines []string) Model`: drops control runes from each line, then truncates it to 60 runes. `(Model).Update(tea.Msg) (Model, tea.Cmd)` maps `y`, `n` and `esc` to `WriteMsg{}`, `DiscardMsg{}` and `StayMsg{}`, and every other key to nil. `(Model).View() string` renders a rounded border in colour 62, with `Padding(1, 2)`, `Width(64)` and no Height. The title is bold, colour 12, `Width(60)` and centred. The key hint uses `styles.HelpStyle`.
- `confirm.Batch{ID int; Ops []guard.Op; Labels []string; NoOpLabel func() string; Reports []guard.Report}`
  - `Preview(b, budget) Batch`: dry-runs each op with `Options{Budget, Actor: guard.ActorTUI}` and stops at the first SinkErr or PreReadErr.
  - `Execute(b, budget) Batch`: runs each op with `Options{Budget, Actor: guard.ActorTUI, Confirm: approves(previewChanges)}` and stops at the first outcome that is neither executed nor no_op.
  - `(Batch).Lines() []string`: formats each change as `<Field>  <From> → <To>`, with `(none)` for an empty From and a `#<id> ` prefix when the change targets a different item. Duplicates are dropped by (target ID, change).
- `confirm.Flow`: `Start(b, budget) (Flow, tea.Cmd)`, `Update(msg) (Flow, tea.Cmd, Result)`, `Busy() bool`, `Sticky() string`, `View(background string) string` (which wraps `modal.Center`).
- `confirm.PreviewedMsg{Batch}`, `confirm.WrittenMsg{Batch}`, `confirm.Result{Handled, Status, IsErr, Refresh, Proceed, Stay, Kept, Batch}`.
- How a view embeds it: run `m.flow, cmd, res = m.flow.Update(msg)` first. When `res.Handled`, set the status from `res` and, if `res.Refresh`, batch a re-fetch with `cmd`, then return. Render with `m.flow.View(background)` and draw `m.flow.Sticky()` in `ErrorStyle` above the status line. Report `m.flow.Busy()` from `IsTextInputFocused` and `WriteBusy`, and skip the tick's fetch while it is busy.

## Line counts

| File | Lines |
|------|-------|
| internal/ui/confirm/modal.go | 96 |
| internal/ui/confirm/batch.go | 114 |
| internal/ui/confirm/flow.go | 282 |
| internal/ui/confirm/confirm_test.go | 199 |
| internal/ui/prs/write_test.go | 336 |
| internal/ui/prs/prs.go | 567 (was 583) |

`prs.go` was already over the 300-line target before this plan, and this plan made it 16 lines shorter. `write_test.go` is a test file with nine tests and their shared helpers.

## Wording beyond the D-01/D-07/D-08/D-10 literals

All of these strings come from the 07-UI-SPEC Copywriting Contract or from the plan:

- `Checking PR #<id>...`
- `Writing #<id>...`
- `Discarded: nothing written for #<id>`
- `Not written: #<id>`
- `#<id>: could not read before writing: <err>`
- `#<id>: write FAILED: <err>`
- `#<id>: not written — changed since preview; press the action key again to re-check`
- `<label> not attempted`
- `(not recorded in audit log)`

One string is new: `<label> not written: audit log not writable: <path>`, used when an op after the first hits a SinkErr at Execute. Today nothing can produce it, because Execute stops after the first op that does not execute or no-op.

The old `Merge cancelled` and `PR #<id> <action>d successfully` strings are gone. A successful approve now reads `#7 approved`.

## Task Commits

1. **Task 1 (tracer, TDD).**
   - RED `c51b8c0` (test): TestApproveRunsThroughGuard, plus a confirm package containing only message types and the prs `writer` field.
   - GREEN `7fe04ad` (feat): the confirm package, and `a`/`c` routed through the flow.
   - The tracer gate re-ran both `<automated>` verifies on the committed tree. Both passed.
2. **Task 2 (TDD).**
   - RED `f8171ec` (test): the eight remaining PR tests, the six confirm tests, and a `WriteBusy` stub returning false.
   - GREEN `cd607e4` (feat): field removal, tick and tab hold, sticky render, and prStatusClient.

## TDD notes

- **RED, Task 1:** `go test -race -count=1 -v ./internal/ui/prs/ -run '^TestApproveRunsThroughGuard$'` printed `write_test.go:99: status after a = "Approving PR #7...", want it to contain "Checking PR #7"` and `--- FAIL`, then exited 1. The target test compiled, ran, and failed on its own assertion: the old code took the direct-approve path. That makes it a valid RED. The `tdd-red-evidence` classifier has no `go test` adapter (it reads only TAP, JUnit, swift-testing and unittest), so the evidence is recorded here instead, as 07-02 did.
- **RED, Task 2:** four target tests failed on their planned assertions:
  - TestAbandonedPRCompleteIsNoChange: `already completed`, want `already abandoned`
  - TestUnknownOutcomeIsSticky: `View() lacks the sticky unknown line`
  - TestTickDoesNotRefreshWhileConfirmOpen: `loading = true`
  - TestTabHeldWhileWriteBusy: `IsTextInputFocused false, WriteBusy false`

  The other Task 2 tests cover behaviour Task 1 already built, so they passed in RED. To show they are not vacuous, six throwaway mutations were applied. Each one was caught, and each file was restored with `git checkout -- <file>`:
  - enter also writes: caught by TestModalOnlyYWrites
  - `approves` always true: caught by TestExecuteRefusesChangedDiff, because Mutate fatals
  - no control-rune drop: caught by TestModalDropsControlRunes
  - `Width(56)`: caught by TestModalFitsSixtyRuneLineOnOneRow
  - first-part rule disabled: caught by TestFlowWrittenTextReportsEachStep
  - Lines dedupe disabled: caught by TestBatchLinesDedupeAndPrefix
- **GREEN:** all 15 named tests pass, with 15 top-level `--- PASS: Test` lines. No REFACTOR commit was needed.

## Gate output

- `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`: all 15 packages report ok, and gofmt is clean.
- `shasum -a 256 -c .planning/phases/07-guarded-tui-writes/07-cli-tests.sha256`: all seven locked CLI test files report OK.
- `internal/ui/prs/prs.go` has no ApprovePullRequest or CompletePullRequest call, and no confirmComplete, actionPending or PRActionMsg.
- `guard.Run(` under `internal/ui` appears only in `internal/ui/confirm/batch.go`, at two sites, and both set `Actor: guard.ActorTUI`.

## Files Created/Modified

- `internal/ui/confirm/modal.go`: the D-01 modal, its key messages, and the private `dropControl` helper
- `internal/ui/confirm/batch.go`: Batch, Preview, Execute, approves, Lines
- `internal/ui/confirm/flow.go`: Flow, PreviewedMsg, WrittenMsg, Result, the status text (`written`, `writtenText`), and `clean`
- `internal/ui/confirm/confirm_test.go`: six modal, batch and flow tests
- `internal/ui/prs/prs.go`:
  - adds the `writer` and `flow` fields and runs the flow first in Update
  - `a`/`c` start the flow; prStatusClient, WriteBusy, and the sticky render are new
  - the tick and IsTextInputFocused follow the flow
  - approveSelectedPR, completeSelectedPR, PRActionMsg, confirmComplete and actionPending are removed
- `internal/ui/prs/write_test.go`: nine PR write tests and their helpers

## Decisions Made

See key-decisions in the frontmatter. In short:
- A settled flow always replaces the in-flight status.
- Flow messages that arrive in the wrong state are swallowed.
- The later-op sink wording is new.
- The `a`/`c` keys have no separate busy check.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A stale "Writing #<id>..." status stayed under the sticky unknown line**
- **Found during:** Task 1 (wiring the flow into prs.go)
- **Issue:** The plan applies Result.Status only when it is non-empty. An unknown outcome deliberately leaves Status empty and puts its text on the sticky line, so the status line would have kept showing `Writing #7...` after the write had settled.
- **Fix:** prs.go also replaces the status whenever the flow settles (`res.Proceed || res.Stay`). In the unknown case the status line is cleared and the sticky line carries the message.
- **Files modified:** internal/ui/prs/prs.go
- **Verification:** TestUnknownOutcomeIsSticky passes. All other status assertions are unchanged.
- **Committed in:** 7fe04ad

**2. [Rule 3 - Blocking] The merge key gate was removed in Task 1, not Task 2**
- **Found during:** Task 1
- **Issue:** The `confirmComplete` key gate called `completeSelectedPR`, which Task 1 deletes, so the package would not compile with it in place. Because `c` now opens the flow directly, the gate could never trigger anyway.
- **Fix:** The gate block was deleted in Task 1. The `confirmComplete` and `actionPending` fields, `PRActionMsg`, and the tick condition were removed in Task 2 as planned.
- **Files modified:** internal/ui/prs/prs.go
- **Committed in:** 7fe04ad

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking). **Impact:** both were needed for correct status text and to compile. There was no scope creep.

## Issues Encountered

None.

## Requirements

`requirements-completed` lists GUARD-02 and GUARD-04, copied from the plan frontmatter. As the orchestrator instructed, `requirements.mark-complete` was not run: the REQUIREMENTS.md checkboxes and traceability rows stay Pending until phase verification. GUARD-02 still needs the other TUI call sites (07-05, 07-06) and the structural test.

## User Setup Required

None. No external service configuration is required.

## Next Phase Readiness

- 07-05 and 07-06 (releases, pipelines, work items) can embed `confirm.Flow` as described in "Final confirm API". For a staged edit, `Result.Kept` (esc) and `Result.Proceed` are ready for the D-02/D-03 navigation handling.
- 07-08 can use `prs.Model.WriteBusy()` for its quit hold.
- Human check, deferred to end-of-phase UAT: on the PRs tab against a test PR, `a` should show `Confirm #<id>` with `reviewer.vote  0 → 10`, and `c` should show `status  active → completed`. At 80x24, the 66-column modal should show its title and key hint without clipping.

## Self-Check: PASSED

- FOUND: internal/ui/confirm/modal.go, batch.go, flow.go, confirm_test.go; internal/ui/prs/write_test.go, prs.go
- FOUND commits: c51b8c0, 7fe04ad, f8171ec, cd607e4 (`git rev-list --count 4740b9e..HEAD` = 4)

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-14*
