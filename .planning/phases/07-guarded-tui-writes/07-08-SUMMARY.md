---
phase: 07-guarded-tui-writes
plan: 08
subsystem: ui
tags: [bubbletea, tui, guard, confirm, quit, tab-switch, audit]

requires:
  - phase: 07-guarded-tui-writes
    provides: "07-04/07-05 WriteBusy on prs, releases and pipelines; 07-06 workitems HasStaged, WriteBusy, ConfirmLeave, ProceedMsg; 07-07 staged resolve on the same leave path"
provides:
  - "app.go leave interception: q, ctrl+c and tab keys 1-6 on Work Items with a staged edit open the S1 confirm modal through ConfirmLeave; the key is held in pendingLeave and replayed on workitems.ProceedMsg"
  - "Quit hold: q and ctrl+c do nothing while the current view's guarded write is busy (preview, open modal, write in flight)"
  - "Six app-level tests in internal/ui/app_test.go, driven by crafted confirm messages"
affects: [phase-7-verification, 08]

actuals:
  tokens: 2904
  tasks: 2
  commits: 3
plan_head_before: 4b33652ea80d40ba391542d1982c10fee37a4475
plan_head_after: ed77bcd20d228d55edadb6dff268cf446135f0fa

tech-stack:
  added: []
  patterns:
    - "Leave interception at the app level: a leave key is stored, the view confirms, and the view's ProceedMsg replays the key through Update"
    - "Quit held on WriteBusy, so the process cannot exit between Mutate and the audit append"

key-files:
  created:
    - internal/ui/app_test.go
  modified:
    - internal/ui/app.go

key-decisions:
  - "The quit hold runs before the staged-edit interception and applies to every write-capable view; tab keys need no separate hold because each view's IsTextInputFocused is already true while busy"
  - "The tab interception requires the target to differ from the current view and no text input focused, so pressing 2 on Work Items with an edit staged does nothing, as it did before"
  - "app tests pass nil projects to NewModel (as 07-06 did) so the work-item refresh a settled write batches reads nothing when the tests flatten it"

patterns-established:
  - "App-level TUI tests: update/press wrappers over Model.Update, crafted confirm.PreviewedMsg/WrittenMsg in place of guard answers, preview and Execute cmds never run"

requirements-completed: [GUARD-03]

coverage:
  - id: D1
    description: "q with a staged work-item edit opens the confirm modal instead of quitting; a second q is held while the modal is open; after y and an executed write, Work Items emits ProceedMsg and the app replays the quit (D-02, GUARD-03)"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/app_test.go#TestQuitWithStagedEditConfirmsFirst"
        status: pass
    human_judgment: false
  - id: D2
    description: "A tab switch with a staged edit confirms first: n discards, ProceedMsg arrives and the view becomes PRs; esc keeps Work Items with the edit still staged (D-02, D-03)"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/app_test.go#TestTabSwitchWithStagedEditDiscardProceeds"
        status: pass
      - kind: unit
        ref: "internal/ui/app_test.go#TestEscKeepsViewAndStagedEdit"
        status: pass
    human_judgment: false
  - id: D3
    description: "An unknown write outcome cancels the quit: no ProceedMsg, no QuitMsg, the view stays on Work Items and shows 'outcome unknown — verify in ADO' (D-08, S2)"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/app_test.go#TestUnknownOutcomeCancelsQuit"
        status: pass
    human_judgment: false
  - id: D4
    description: "q and ctrl+c do nothing while a PR approve flow is busy, so a quit cannot cut a write off from its audit record (Phase 4 D-13, T-07-32)"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/app_test.go#TestQuitHeldWhileWriteBusy"
        status: pass
    human_judgment: false
  - id: D5
    description: "With nothing staged and nothing busy, q quits and 2 switches to Work Items exactly as before"
    verification:
      - kind: unit
        ref: "internal/ui/app_test.go#TestPlainQuitAndTabUnchanged"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && go test -race -count=1 ./... && test -z \"$(gofmt -l .)\""
        status: pass
    human_judgment: false
  - id: D6
    description: "GUARD-06 phase-end gate: both CLI test manifests verify and the inventory check lists only TestWriteRecoverPrecedesMutate"
    verification:
      - kind: other
        ref: "LC_ALL=C shasum -a 256 -c 07-cli-tests.sha256 && shasum -a 256 -c 07-cli-audit-test.sha256 && comm -23 07-cli-test-inventory.txt <(go test ./cmd/adoBizInt/ -list '.*' | grep '^Test' | sort) == TestWriteRecoverPrecedesMutate"
        status: pass
    human_judgment: false
  - id: D7
    description: "In the running TUI: stage a state change on Work Items with n and press 3, and the confirm modal appears before the PRs tab; esc stays on Work Items with the row still staged; q opens the modal again; n exits without writing"
    verification: []
    human_judgment: true
    rationale: "This is the plan's <human-check> and needs a real terminal session against ADO. It is deferred to the end-of-phase UAT"

duration: 3min
completed: 2026-10-14
status: complete
---

# Phase 7 Plan 08: Leave and Quit Confirm at the App Level Summary

**Quitting or switching tabs with a staged work-item edit now opens the same confirm modal first. The app quits or switches only after y writes or n discards. Quit does nothing while any view's guarded write is busy, so a write can no longer be cut off from its audit record.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-10-14T04:35:39Z
- **Completed:** 2026-10-14T04:38:15Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `internal/ui/app.go`: before the existing quit and tab handling, the KeyMsg branch now:
  1. Computes `quit` and `tab`. `tab` means a tab key whose target differs from the current view, with no text input focused.
  2. Returns `m, nil` for a quit while `currentViewWriteBusy()` is true.
  3. On Work Items with `HasStaged()`, stores a copy of the key in `pendingLeave`, calls `m.workitems.ConfirmLeave()` and returns its cmd.
- A new `case workitems.ProceedMsg:` clears `pendingLeave` and replays the stored key through `Update`. The replay uses the existing quit and tab paths, so a tab switch still returns the target view's `Init` cmd. With nothing pending it returns `m, nil`.
- Private helpers: `tabTarget(msg)` maps keys 1-6 to a `View`. `currentViewWriteBusy()` calls `WriteBusy` on workitems, prs, pipelines and releases, and returns false for dashboard and branches.
- `internal/ui/app_test.go`: six tests covering the five must-have truths.
- No new copy was added. Per 07-UI-SPEC S4, the hold shows nothing of its own. The user sees the existing `Checking …`/`Writing …` status or the open modal's key hints.

## Task Commits

1. **Task 1 (tracer): q with a staged edit confirms first, then quits**
   - RED `a7b0561` (test)
   - GREEN `2e37e9d` (feat)
2. **Task 2: tab discard, esc, unknown outcome, busy hold, plain paths**: `ed77bcd` (test). No app.go change was needed (see TDD notes).

**Plan metadata:** recorded in the docs commit that adds this file.

## Files Created/Modified

- `internal/ui/app.go` (292 lines, was 224): `pendingLeave` field, leave interception, `workitems.ProceedMsg` replay, `tabTarget`, `currentViewWriteBusy`
- `internal/ui/app_test.go` (258 lines, new):
  - Tests: `TestQuitWithStagedEditConfirmsFirst`, `TestTabSwitchWithStagedEditDiscardProceeds`, `TestEscKeepsViewAndStagedEdit`, `TestUnknownOutcomeCancelsQuit`, `TestQuitHeldWhileWriteBusy`, `TestPlainQuitAndTabUnchanged`
  - Helpers: `update`, `press`, `run`, `yields`, `stagedApp`, `batch`, `leaveAndPreview`, `answer`

## Decisions Made

- **Hold order.** The quit hold is checked before the staged-edit interception. While Work Items is confirming, a second q is therefore held rather than restarting ConfirmLeave, which would be a no-op while busy anyway. Tab keys need no hold of their own, because each view's `IsTextInputFocused()` is already true while busy (07-04).
- **Tab to the current view.** A tab key that names the current view is not a leave. Pressing 2 on Work Items with an edit staged does nothing, as before.
- **Tests use nil projects.** As in 07-06, `NewModel` gets nil projects, so the work-item refresh batched with ProceedMsg reads nothing and returns at once when a test flattens the cmd. The plan's setup text named `[]string{"P"}`. That would have run `GetWorkItems` against the zero `ado.Client`. The rows still carry `Project: "P"`.

## TDD Notes

- **RED, Task 1:** `go test -race -count=1 -v ./internal/ui/ -run '^TestQuitWithStagedEditConfirmsFirst$'` exited 1. The output was `app_test.go:103: q with a staged edit: cmd nil = false, WriteBusy = false; want the preview started instead of a quit`.
  - Semantic assessment: the target test ran and failed on its first planned assertion, for the intended reason. The old app returned `tea.Quit` for q and started no confirm. This is a valid RED.
- **GREEN, Task 1:** the same command printed `--- PASS: TestQuitWithStagedEditConfirmsFirst`. `go build ./...` and `go vet ./internal/ui/` were clean.
- **Tracer gate:** auto mode is off and `human_verify_mode` is `end-of-phase`. Task 1's `<verify>` is automated only, so it was re-run, it passed, and expansion continued with no checkpoint.
- **Task 2 (unexpected GREEN, investigated):** all five new tests passed on their first run. Task 1's action already implemented the full interception (quit, tab, hold and replay), so nothing in Task 2 needed new code. To show the tests are not vacuous, they were run against the pre-plan `app.go` (`git show 4b33652:internal/ui/app.go`), and then the current file was restored with no diff:
  - `TestTabSwitchWithStagedEditDiscardProceeds`, `TestEscKeepsViewAndStagedEdit` and `TestUnknownOutcomeCancelsQuit` failed with `setup: View() after 3|q and the preview lacks "Confirm #1"`. The old app switched tabs or quit before any confirm opened, which is the D-02 defect these tests pin.
  - `TestQuitHeldWhileWriteBusy` failed with `q quit while the PR write was busy` and `ctrl+c quit while the PR write was busy`.
  - `TestPlainQuitAndTabUnchanged` passed, as intended: it guards the unchanged path.
- The `tdd-red-evidence` classifier has no `go test` adapter. As in 07-02 and 07-04 through 07-07, the evidence and its semantic assessment are recorded here.

## TDD Gate Compliance

- RED `a7b0561` comes before GREEN `2e37e9d` for Task 1.
- Task 2 has a `test(07-08)` commit and no `feat` commit, because no implementation change was needed. This is recorded above, not hidden.

## Gate Output

- `go build ./... && go vet ./... && go test -race -count=1 ./...`: all 14 test packages are ok. `gofmt -l .` is empty.
- The six app tests give six `--- PASS: Test` lines.
- GUARD-06 phase-end gate:

```
cmd/adoBizInt/fakes_test.go: OK
cmd/adoBizInt/filter_test.go: OK
cmd/adoBizInt/main_test.go: OK
cmd/adoBizInt/patleak_test.go: OK
cmd/adoBizInt/usage_test.go: OK
cmd/adoBizInt/write_test.go: OK
cmd/adoBizInt/writeverbs_test.go: OK
cmd/adoBizInt/audit_test.go: OK
comm -23 inventory vs current: TestWriteRecoverPrecedesMutate   (only)
```

- `cmd/adoBizInt` and `internal/ui/keymap` are untouched. Running `adoBizInt` with no arguments still builds `ui.NewModel` the same way. The only change in TUI behaviour is the guarded leave and the quit hold.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] App tests construct NewModel with nil projects instead of `[]string{"P"}`**
- **Found during:** Task 1, while writing the test
- **Issue:** The tests must flatten the cmd batched after a settled write to find `ProceedMsg`. That batch includes the work-item refresh. With a project configured, the refresh calls `GetWorkItems` on the zero `ado.Client`, which has no transport.
- **Fix:** Pass nil projects. `GetWorkItems` then returns an empty result at once. The rows and the crafted batch keep `Project: "P"`. 07-06 made the same change in `newStagingModel`.
- **Files modified:** internal/ui/app_test.go
- **Verification:** All six tests pass under `-race`.
- **Committed in:** a7b0561, ed77bcd

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Test fixture only. No production behaviour differs from the plan.

## Issues Encountered

None.

## Requirements

`requirements-completed` lists GUARD-03, copied from the plan. As the orchestrator instructed, `requirements.mark-complete` was not run, so the REQUIREMENTS.md GUARD checkboxes and traceability rows stay Pending for phase verification. This plan finishes the last GUARD-03 trigger (tab switch and quit).

## Threat Model

- T-07-32 (quit mid-write) is mitigated: `currentViewWriteBusy` holds quit, as `TestQuitHeldWhileWriteBusy` shows. A staged-edit quit fires only after the write settles (`TestQuitWithStagedEditConfirmsFirst`).
- T-07-33 (staged edits lost on leave) is mitigated by ConfirmLeave routing (`TestTabSwitchWithStagedEditDiscardProceeds`, `TestEscKeepsViewAndStagedEdit`).
- T-07-34 (unknown outcome hidden) is mitigated (`TestUnknownOutcomeCancelsQuit`).
- T-07-35 (ctrl+c held during a hung write) is accepted as planned. The hold lasts at most the guard's bounded budget.
- No new network, auth, file or schema surface was added.

## Known Stubs

None.

## User Setup Required

None. No external service configuration is required.

## Next Phase Readiness

- This was the final plan of Phase 7. D-02 now covers every focus change: row move, another item, tab switch and quit.
- Phase 7 is ready for verification. The end-of-phase UAT should include the D7 human check above, plus the deferred human checks from 07-04 through 07-07.

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-14*
