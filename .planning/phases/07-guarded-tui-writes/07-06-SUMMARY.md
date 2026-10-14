---
phase: 07-guarded-tui-writes
plan: 06
subsystem: ui
tags: [go, bubbletea, bubble-table, guarded-writes, staging, audit, tui, tdd]

requires:
  - phase: 07-02
    provides: "guard.SetState (CR-01 project check, System.State diff, no-op), guardtest.Client"
  - phase: 07-03
    provides: "audit records carry actor and user_approved"
  - phase: 07-04
    provides: "confirm.Flow / Batch / Result (Proceed, Stay, Kept, Refresh), PreviewedMsg, WrittenMsg, WriteMsg, DiscardMsg, StayMsg"
  - phase: 07-05
    provides: "guardtest.AuditRecords(t)"
provides:
  - "internal/ui/workitems/staging.go: stagedEdit + batch(), ProceedMsg, highlightedID, (*Model).stage, (*Model).startConfirm, (Model).applyFlow, HasStaged, WriteBusy, ConfirmLeave"
  - "workitems: n stages the next state; row moves, enter and n on another row confirm first (D-02); y writes through guard.SetState with the row's real project"
  - "workitems: Actions column 18 wide; staged row reads 'staged → <state>' in WarningStyle"
  - "workitems: confirm overlay drawn above the detail and effort modals; sticky unknown line above the status line"
affects: [07-07, 07-08]

actuals:
  tokens: 7300
  tasks: 3
  commits: 9
plan_head_before: 99e8a3387c3b4aadfc4db9acc53975e294dff8e5
plan_head_after: 09fd198f62d345390fae196e8dbaa3ac724e7ba5

tech-stack:
  added: []
  patterns:
    - "Held navigation: a key that would move off the staged item is stored in pendingKey, the confirm starts, and applyFlow replays the key through m.Update on Proceed (dropped on Stay)"
    - "Table moves are applied to a copy first; the copy is committed only when the highlighted ID is unchanged or nothing is staged"
    - "Every settle except Kept drops the staged edit, so a failing preview can never trap the user"

key-files:
  created:
    - internal/ui/workitems/staging.go
    - internal/ui/workitems/staging_test.go
  modified:
    - internal/ui/workitems/workitems.go

key-decisions:
  - "applyFlow does not set loading on Refresh: the work-items view replaces the whole table with 'Loading work items...' while loading, and the old StateUpdateMsg success path re-fetched without setting it either"
  - "applyFlow replaces the status on every settle (the 07-04 prs rule), so an unknown outcome does not leave 'Writing #<id>...' under the sticky line; esc then overrides it with '#<id> still staged — move on to confirm'"
  - "The confirm modal is drawn above the detail and effort modals, because the flow takes keys before either; ConfirmLeave from inside a detail would otherwise swallow keys behind an invisible modal"
  - "Replayed keys are batched after the flow cmd and the refresh: tea.Batch(flowCmd, fetchWorkItems, replayedCmd, ProceedMsg cmd), in that order; the replay runs synchronously inside applyFlow, so the highlight has already moved when Update returns"

patterns-established:
  - "Staging test setup: New(&ado.Client{}, url, nil projects, ...) so the refresh cmd is safe to run and yields an empty WorkItemsMsg"

requirements-completed: [GUARD-03, GUARD-02]

coverage:
  - id: D1
    description: "n stages the next state and sends nothing; the staged row's Actions cell reads 'staged → Resolved'; a row move opens 'Confirm #1' with 'System.State  Active → Resolved' while the highlight stays; y writes UpdateWorkItemState(\"P\", 1, \"Resolved\") once, moves to #2, and appends one tui audit record"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/workitems/staging_test.go#TestStagedStateConfirmsOnRowMove"
        status: pass
    human_judgment: false
  - id: D2
    description: "The write carries the row's own project (not \"\") and runs through guard.Run with the client's fetch budget"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/workitems/staging_test.go#TestStagedStateConfirmsOnRowMove"
        status: pass
    human_judgment: false
  - id: D3
    description: "n on the modal discards and moves; esc keeps the edit staged and stays; a no-op pre-read skips the modal and moves; none of them send or audit (D-03, D-04, D-07)"
    verification:
      - kind: unit
        ref: "internal/ui/workitems/staging_test.go#TestDiscardDropsStagedAndMoves|TestStayKeepsStagedAndPosition|TestNoOpSkipsModalAndMoves"
        status: pass
    human_judgment: false
  - id: D4
    description: "Unknown outcome stays on the item with the sticky line, drops the edit and re-fetches; an unwritable sink blocks, drops the edit and leaves the next move free (D-08, D-10, T-07-26)"
    verification:
      - kind: unit
        ref: "internal/ui/workitems/staging_test.go#TestUnknownOutcomeStaysSticky|TestUnwritableSinkDropsStagedAndStays"
        status: pass
    human_judgment: false
  - id: D5
    description: "enter (and n) on another row confirms the staged item first, then replays; IsTextInputFocused is true while the flow is busy"
    verification:
      - kind: unit
        ref: "internal/ui/workitems/staging_test.go#TestEnterOnAnotherItemConfirmsFirst"
        status: pass
    human_judgment: false
  - id: D6
    description: "Leave API for 07-08: ConfirmLeave yields ProceedMsg after y or n, never after esc, at once with nothing staged; WriteBusy follows the flow; the confirm draws over an open detail modal"
    verification:
      - kind: unit
        ref: "internal/ui/workitems/staging_test.go#TestConfirmLeaveProceedsAfterWrite|TestConfirmLeaveEscDoesNotProceed|TestConfirmLeaveDiscardProceeds|TestConfirmLeaveWithNothingStagedProceedsImmediately|TestConfirmLeaveShowsOverDetail"
        status: pass
    human_judgment: false
  - id: D7
    description: "In the running TUI the amber 'staged → Resolved' marker renders whole, and at 80x24 the 18-column Actions column clips neither rows nor the status hint"
    verification: []
    human_judgment: true
    rationale: "Terminal rendering and colour at a real size are not asserted by any test (plan <human-check>, 07-UI-SPEC backstop); deferred to end-of-phase UAT"

duration: 5min
completed: 2026-10-14
status: complete
---

# Phase 7 Plan 06: Staged Work-Item State Changes Summary

**Pressing n on a work item now only stages the next state. It is written once, through guard.SetState with the row's own project, and only after the user answers y in the confirm modal that opens when they move to another row, open or stage another item, or (from 07-08) leave the view.**

## Performance

- **Duration:** about 5 min
- **Started:** 2026-10-14T04:18:20Z
- **Completed:** 2026-10-14T04:23:00Z
- **Tasks:** 3 of 3
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- The plain state cycle (formerly `client.UpdateWorkItemState(context.Background(), "", id, nextState)`) is gone. `n` calls `m.stage(*item, nextState)`, and the write happens in `stagedEdit.batch` as `guard.SetState(w, s.item.Project, s.item.ID, s.state, guard.Effort{})`.
- The view holds navigation:
  - A table key that would change the highlighted ID is applied to a copy of the table.
  - While an edit is staged, the copy is not committed. The key goes into `pendingKey` and the confirm starts instead.
  - `enter` and `n` on a row other than the staged item are held the same way.
- `applyFlow` applies the settle rules:
  - Kept (esc) keeps the edit. Every other settle drops it.
  - Proceed replays the held key. Stay forgets it.
  - Refresh re-fetches.
  - An executed write to a done state removes the item from priorities. This is what the old StateUpdateMsg success branch did.
- Leave API for 07-08: `HasStaged`, `WriteBusy`, `ConfirmLeave`, and `ProceedMsg`.
- 12 staging tests, all passing.

## Staged-row marker

- The text is `staged → <state>`, for example `staged → Resolved`. It is rendered with `styles.WarningStyle.Render(...)` in the Actions cell and replaces `[n] Next`.
- The Actions column is `table.NewColumn(columnKeyActions, "Actions", 18)` (C2).
- Rows with nothing staged are unchanged, and the existing row-equivalence tests pass.
- The status hint is `#<id> staged: <state> — move to another row to confirm`. After esc it reads `#<id> still staged — move on to confirm`, and while the pre-read runs it reads `Checking #<id>...`.

## How replayed keys are batched

`applyFlow` builds `cmds = [flowCmd]`. It then appends `m.fetchWorkItems()` when `Refresh` is set. On `Proceed` with a held key, it calls `m.Update(*pendingKey)` synchronously and appends the cmd that call returns. When the confirm came from `ConfirmLeave`, it appends a cmd that yields `ProceedMsg`. It returns `tea.Batch(cmds...)`.

The highlight has already moved by the time `Update(WrittenMsg)` returns. `pendingKey` and `leaving` are cleared on every settle.

## Line counts

| File | Lines |
|------|-------|
| internal/ui/workitems/workitems.go | 1001 (was 979; cap 1060) |
| internal/ui/workitems/staging.go | 140 (cap < 300) |
| internal/ui/workitems/staging_test.go | 467 (test file, 12 tests and helpers) |

`workitems.go` grew by 22 lines net. The new fields, flow-first dispatch, key interception, and render hooks add about 48 lines. The refactor that routes `openDetailModal` and `getSelectedWorkItem` through `highlightedID` removes 26.

## Task Commits

1. **Task 1 (tracer, TDD):**
   - RED `7b537a9` (test): the tracer test, the `writer` field, `highlightedID`, and a `HasStaged` stub.
   - GREEN `44d87eb` (feat): staging, the flow wiring, the 18-wide Actions column, and the marker.
   - REFACTOR `4d5eed1`: `openDetailModal` and `getSelectedWorkItem` use `highlightedID`.
   - Tracer gate (interactive, end-of-phase, `<automated>`-only verify): all three verifies were re-run on the committed tree and passed, so the run continued to Task 2.
2. **Task 2 (TDD):**
   - RED `e5f2f57` (test): six tests.
   - GREEN `c96492f` (feat): `enter`/`n` interception and `IsTextInputFocused` follows the flow.
3. **Task 3 (TDD):**
   - RED `2f66c79` (test): four leave tests, the `ProceedMsg` type, and stubs.
   - GREEN `f2876b0` (feat): `ConfirmLeave`, `WriteBusy`, and the `leaving` field.
4. **Overlay fix (Rule 1):**
   - RED `4b415ca` (test): `TestConfirmLeaveShowsOverDetail`.
   - Fix `09fd198`: the confirm is drawn above the detail and effort modals.

## TDD notes

- **RED, Task 1:** `go test -race -count=1 -v ./internal/ui/workitems/ -run '^TestStagedStateConfirmsOnRowMove$'` exited 1. The target ran and failed on its first planned assertion, `n returned a cmd; staging must send nothing`, because the old code returned the direct-write closure. This is a valid RED.
- **RED, Task 2:** `TestEnterOnAnotherItemConfirmsFirst` failed with `enter opened #2's detail before the staged #1 was confirmed`, which is valid.
  - The first attempt crashed with a nil-pointer panic: the test fed the detail's fetch cmd before checking `showDetail`. That is fixture-crash INVALID_RED, so the check was moved ahead of the feed and the run repeated.
  - The other five Task 2 tests passed at RED. Task 1 had already implemented the settle rules the plan assigns to it ("Drop staged on every settle except Kept (Task 1 rule)"). They pin those rules.
- **RED, Task 3:** all four failed on planned assertions: `cmd = nil, want one yielding confirm.PreviewedMsg`, and `yielded [], want one ProceedMsg`.
- **RED, overlay fix:** the test failed with `View() hides the confirm modal behind the detail modal`.
- The `tdd-red-evidence` classifier has no `go test` adapter, so, as in 07-02, 07-04 and 07-05, the evidence and its semantic assessment are recorded here.
- **GREEN:** the 7 Task 1–2 tests and the 4 Task 3 tests each give the required count of `--- PASS: Test` lines. All 12 staging tests pass.

## Gate output

- `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`: all 14 test packages are ok, and gofmt is clean.
- `shasum -a 256 -c .planning/phases/07-guarded-tui-writes/07-cli-tests.sha256`: all seven locked CLI test files report OK.
- `grep -v '^\s*//' internal/ui/workitems/workitems.go | grep -c 'table.NewColumn(columnKeyActions, "Actions", 18)'` returns 1.
- One `UpdateWorkItemState(` call remains in `workitems.go`: the User Story effort path, which 07-07 converts. `TestUpdateWorkItemStateWithEffortCallSitesForwardPointers` still finds its two effort call sites.

## Files Created/Modified

- `internal/ui/workitems/staging.go`: stagedEdit, batch, highlightedID, HasStaged, stage, startConfirm, applyFlow, ProceedMsg, WriteBusy, ConfirmLeave
- `internal/ui/workitems/workitems.go`:
  - adds the `writer`, `flow`, `staged`, `pendingKey` and `leaving` fields; the flow runs first in Update
  - `updateSelectedItemState` reads the from-state from the staged edit and stages instead of writing
  - `enter`/`n` and table-move interception
  - the Actions column is 18 wide and shows the staged marker
  - the confirm overlay sits above all modals; the sticky line and a busy `IsTextInputFocused` are new
  - `highlightedID` is reused
- `internal/ui/workitems/staging_test.go`: 12 tests and their helpers (`newStagingModel`, `press`, `feed`, `run`, `stageAndPreview`, `stageAndLeave`, `wantNoAudit`, `actionsCell`, `proceedCount`)

## Decisions Made

See key-decisions in the frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The confirm modal was hidden behind an open detail or effort modal**
- **Found during:** self-review after Task 3
- **Issue:** `View()` returned the detail or effort composite before reaching `m.flow.View`. The flow takes keys before either modal, so a confirm started by `ConfirmLeave` while a detail was open would have swallowed keys behind a modal the user could not see.
- **Fix:** `View()` composes the table, then the effort or detail modal, and always wraps the result in `m.flow.View(...)`.
- **Files modified:** internal/ui/workitems/workitems.go, internal/ui/workitems/staging_test.go (`TestConfirmLeaveShowsOverDetail`)
- **Committed in:** 4b415ca, 09fd198

**2. [Rule 1 - Bug] Refresh does not set `loading`**
- **Found during:** Task 1
- **Issue:** The plan says "When Refresh: set loading". In this view, `loading` replaces the whole table with `Loading work items...`, so the user would lose the row they had just moved to until the fetch returned. The old StateUpdateMsg success path re-fetched without setting `loading`.
- **Fix:** applyFlow appends `m.fetchWorkItems()` and leaves `loading` alone.
- **Committed in:** 44d87eb

**3. [Rule 1 - Bug] The status is replaced on every settle**
- **Found during:** Task 1
- **Issue:** This is the same issue 07-04 fixed in prs. Setting the status only when `res.Status` is non-empty would leave `Writing #1...` under the sticky unknown line.
- **Fix:** The status is replaced whenever the flow settles.
- **Committed in:** 44d87eb

**4. [Rule 3 - Blocking] The test model gets no projects**
- **Found during:** Task 2
- **Issue:** The plan's setup passes `[]string{"P"}`. Tasks 2 and 3 must run the batched cmds to find `ProceedMsg` and the refresh, and with a project, `fetchWorkItems` would call a zero `ado.Client`.
- **Fix:** `newStagingModel` passes nil projects, so the refresh returns an empty `WorkItemsMsg` at once. The items still carry `Project: "P"`, and that is what the write and the assertion use.
- **Committed in:** e5f2f57 (helper introduced in 7b537a9)

**5. [Scope - minor] Small additions outside the task split**
- `HasStaged` moved to Task 1, because the tracer test needs it.
- `openDetailModal` and `getSelectedWorkItem` now use `highlightedID`, so there is no second copy of the row-ID parse.
- `TestEnterOnAnotherItemConfirmsFirst` also asserts the busy `IsTextInputFocused`, which Task 2 adds.

---

**Total deviations:** 4 auto-fixed (3 bugs, 1 blocking), plus the minor additions in item 5. **Impact:** all were needed for correct display or for testability. There was no scope creep. 12 tests were written, not 11.

## Issues Encountered

The first Task 2 RED run panicked inside the detail fetch (see TDD notes). Reordering the assertion fixed it.

## Requirements

`requirements-completed` lists GUARD-03 and GUARD-02, copied from the plan. As the orchestrator instructed, `requirements.mark-complete` was not run, and the REQUIREMENTS.md checkboxes and traceability rows stay Pending. GUARD-03 still needs the compound resolve (07-07) and the tab and quit wiring (07-08).

## Threat model

All four mitigations are in place and tested:
- T-07-23: the confirm, discard and enter tests.
- T-07-24: zero audit lines on discard, stay, no-op and sink failure.
- T-07-25: the tracer asserts project `"P"`.
- T-07-26: the sink test proves the next move is free.

No new network, auth or file surface was added.

## User Setup Required

None.

## Next Phase Readiness

- 07-07 can build its compound resolve as a `stagedEdit` with more ops. It should extend `batch()`, not fork it. The two effort call sites in `EffortSubmitMsg` are still there for it to convert.
- For 07-08, app.go should:
  - call `m.workitems.ConfirmLeave()` on a tab switch or quit when `HasStaged()` is true
  - replay the navigation when `workitems.ProceedMsg` arrives
  - hold `q`/`ctrl+c` while `WriteBusy()` is true
  - rely on `IsTextInputFocused()`, which already holds tab keys while the flow is busy
- Human check, deferred to end-of-phase UAT: on Work Items, `n` on an Active bug should show the whole amber `staged → Resolved`. `down` should open `Confirm #<id>`. `esc` should keep the edit staged, and `down` then `n` should discard it and move the cursor. At 80x24, nothing should clip.

## Self-Check: PASSED

- FOUND: internal/ui/workitems/staging.go, staging_test.go, workitems.go
- FOUND commits: 7b537a9, 44d87eb, 4d5eed1, e5f2f57, c96492f, 2f66c79, f2876b0, 4b415ca, 09fd198 (`git rev-list --count 99e8a33..HEAD` = 9)

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-14*
