---
phase: 07-guarded-tui-writes
plan: 07
subsystem: ui
tags: [go, bubbletea, guarded-writes, staging, audit, go-ast, structural-test, tdd]

requires:
  - phase: 07-02
    provides: "guard.SetState (effort routing, CR-01 project check, effort-then-state diff), guardtest.Client"
  - phase: 07-04
    provides: "confirm.Batch multi-op Preview/Execute, Lines() dedupe and #<id> prefix, first-part written-text rule"
  - phase: 07-05
    provides: "guardtest.AuditRecords(t)"
  - phase: 07-06
    provides: "stagedEdit, batch(), stage, startConfirm, applyFlow, staging test helpers (press, feed, run, wantNoAudit)"
provides:
  - "internal/ui/workitems/staging.go: effortEdit; stagedEdit.effort; batch() emits state, then (User Story) dev-task close, then effort ops"
  - "workitems: EffortSubmitMsg stages instead of writing; StateUpdateMsg carries only pre-check errors; itemByID"
  - "internal/guard/uibypass_test.go: TestNoUIFileCallsADOMutation (default-deny, two tripwires), TestUIGuardCallsCarryTUIActor"
affects: [07-08]

actuals:
  tokens: 8560
  tasks: 3
  commits: 5
plan_head_before: 61d4aa23bb4b66b0cc7b11bf7f6709ffcdf9a961
plan_head_after: 132420bc95cd44bd280749bbb0eff0b03cb38559

tech-stack:
  added: []
  patterns:
    - "A compound write is one staged edit whose batch holds several guard.SetState ops; one confirm previews them all (Lines dedupes the repeated System.State line) and Execute runs them in order"
    - "Structural no-bypass test: the forbidden set is derived from the ado package's own *Client methods (default-deny), so a new mutation is forbidden without editing the test"

key-files:
  created:
    - internal/ui/workitems/resolve_test.go
    - internal/guard/uibypass_test.go
  modified:
    - internal/ui/workitems/staging.go
    - internal/ui/workitems/workitems.go
    - internal/ui/workitems/workitems_test.go

key-decisions:
  - "TestNoUIFileCallsADOMutation exempts calls through the file's own guard import: guard.ApproveRelease, guard.QueuePipeline and guard.RejectRelease are the op constructors that route writes through the guard and share the ado method names. Any other selector call naming an ado mutation (including m.writer.UpdateWorkItemState) fails"
  - "A User Story resolve stages one batch of three guarded ops (story state, 'task #<id> closed', 'effort update'). The dev task is written under the story's project, so CR-01 refuses a child in another project at preview time. With no dev task found, only the story state is staged"
  - "StateUpdateMsg lost its ItemID and NewState fields: nothing emits a successful StateUpdateMsg any more, so its success branch and those fields were dead"
  - "itemByID replaces three copies of the item-by-ID loop (updateSelectedItemState, getSelectedWorkItem, and the new EffortSubmitMsg handler), which meets the rule of three"

patterns-established:
  - "Resolve test setup: newResolveModel(t, transitions, items, details) points the pre-read at a details map that both write funcs update, so a later op's fresh pre-read sees the earlier write, as ADO would"

requirements-completed: [GUARD-03, GUARD-05, GUARD-02]

coverage:
  - id: D1
    description: "A Task resolve from the effort modal is staged and sends nothing, is confirmed once with a modal listing 'System.State  Active → Closed', 'RemainingWork  3 → 0' and 'CompletedWork  (none) → 2' and no OriginalEstimate line, and y sends UpdateWorkItemState(P, 5, Closed) and then UpdateWorkItemStateWithEffort(P, 5, Closed, nil, ptr(0), ptr(2)). It leaves two tui audit records (state, then effort only), sets the status '#5 closed; effort update' and moves the highlight to #6"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/workitems/resolve_test.go#TestTaskResolveIsTwoGuardedWrites"
        status: pass
    human_judgment: false
  - id: D2
    description: "A blank effort field stays nil from the modal's keys to the client argument, and no change for it appears in the diff or the audit record (BL-01 end to end)"
    requirement: GUARD-05
    verification:
      - kind: unit
        ref: "internal/ui/workitems/resolve_test.go#TestTaskResolveIsTwoGuardedWrites"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/resolve_test.go#TestUserStoryResolveClosesDevTaskThenRecordsEffort"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-09: when the effort write fails after the state write, the status reads '#5 closed; effort update FAILED: …' as an error, the item is re-fetched, the highlight stays, and there is exactly one state call, one effort call, no compensating write, and one audit record"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/workitems/resolve_test.go#TestResolveEffortFailureReportsBoth"
        status: pass
    human_judgment: false
  - id: D4
    description: "A User Story resolve writes the story state, closes dev task #11, then records its effort, as three guarded writes with three tui audit records. A dev task in another project is refused at preview with nothing sent, nothing staged and no audit. With no dev task found, only the story state is written"
    requirement: GUARD-03
    verification:
      - kind: unit
        ref: "internal/ui/workitems/resolve_test.go#TestUserStoryResolveClosesDevTaskThenRecordsEffort"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/resolve_test.go#TestUserStoryResolveWithoutDevTaskWritesStateOnly"
        status: pass
    human_judgment: false
  - id: D5
    description: "No internal/ui file calls an *ado.Client mutation directly (default-deny set derived from internal/ado, with the known-mutations and snippet tripwires), and every guard.Run under internal/ui is in confirm/batch.go with Actor: guard.ActorTUI"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/guard/uibypass_test.go#TestNoUIFileCallsADOMutation"
        status: pass
      - kind: unit
        ref: "internal/guard/uibypass_test.go#TestUIGuardCallsCarryTUIActor"
        status: pass
    human_judgment: false
  - id: D6
    description: "In the running TUI, closing a Task with Original Estimate blank leaves Original Estimate unchanged in ADO History, and audit.jsonl gains two lines with \"actor\":\"tui\""
    verification: []
    human_judgment: true
    rationale: "This needs live ADO History and a real terminal (the plan's <human-check>). It is deferred to the end-of-phase UAT"

duration: 6min
completed: 2026-10-14
status: complete
---

# Phase 7 Plan 07: Guarded Compound Resolve and the GUARD-02 No-Bypass Test Summary

**Resolving a work item from the effort modal no longer writes anything straight away. It is staged and confirmed once, then written as ordered guard.SetState ops: the state first, then the effort (and for a User Story, the development task's close in between). Each op leaves its own tui audit record. A blank effort field stays nil from the keystroke to the client. A go/ast test, whose forbidden set is derived from \*ado.Client, now fails if any internal/ui file calls an ado mutation directly.**

## Performance

- **Duration:** about 6 min
- **Started:** 2026-10-14T04:26:22Z
- **Completed:** 2026-10-14T04:32:30Z
- **Tasks:** 3 of 3
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- The last three direct TUI write call sites, all in `workitems.go`'s `EffortSubmitMsg` handler, are gone. That was the Task close+effort and the User Story's dev-task close+effort and story state. The handler now calls `m.stage(*item, effortMsg.NewState, &effortEdit{...})`, forwarding `effortMsg.OriginalEstimate`, `.Remaining` and `.Completed` unchanged.
- `stagedEdit.batch` emits, in order:
  - the item's state op, labelled with the lower-case state
  - for a User Story only, `guard.SetState(w, story.Project, taskID, "Closed", guard.Effort{})`, labelled `task #<id> closed`
  - the effort op, labelled `effort update`
- The effort op re-sends the state. By the time it runs, its fresh pre-read already shows the new state, so its diff and audit record carry only the effort fields. That gives one audit record per staged change.
- D-09 needs no extra code. `confirm.Execute` stops at the first op that does not land, and the written text reads `#5 closed; effort update FAILED: <error>`. There is no rollback path.
- **Direct mutation calls under internal/ui: 0.** Every write in the TUI is a `guard.Op` run by `confirm/batch.go`.

## Retired test and its replacement

- **Retired:** `TestUpdateWorkItemStateWithEffortCallSitesForwardPointers` (workitems_test.go). It asserted exactly two direct `UpdateWorkItemStateWithEffort` call sites, and GUARD-02 forbids both, so it could not be satisfied any more. The `go/ast`, `go/parser` and `go/token` imports went with it.
- **Replacement:** `TestTaskResolveIsTwoGuardedWrites` (resolve_test.go). It drives the effort modal's keys (blank, tab, `0`, tab, `2`, enter) through the confirm modal and the guard into the fake client. It asserts that the client's `originalEstimate` argument is nil and that the audit record has no OriginalEstimate change. The rationale is written as a comment at the top of resolve_test.go.

## Derived forbidden set

The test derives the set as follows. It collects the exported methods on `*Client` in internal/ado's non-test files. It then drops names starting with Get, Current, Fetch or Download, and drops Projects, Pipelines and ReleaseDefinitions. Today the set is:

`ApprovePullRequest, ApproveRelease, CompletePullRequest, QueuePipeline, RejectRelease, UpdateWorkItemState, UpdateWorkItemStateWithEffort`

All seven are asserted by the tripwire. The snippet tripwire has two real calls, a comment, a string, and a `guard.ApproveRelease(...)` call, and the scanner reports exactly 2 findings.

## Line counts

| File | Lines | Change since 07-06-SUMMARY |
|------|-------|----------------------------|
| internal/ui/workitems/workitems.go | 968 | was 1001, so -33 (cap 1060) |
| internal/ui/workitems/staging.go | 174 | was 140, so +34 (cap < 300) |
| internal/ui/workitems/resolve_test.go | 389 | new (4 tests and helpers) |
| internal/guard/uibypass_test.go | 235 | new (2 tests and helpers) |
| internal/ui/workitems/workitems_test.go | 121 | was 187 (retired test removed) |

## Task Commits

1. **Task 1 (tracer, TDD):**
   - RED `50190bb` (test): `TestTaskResolveIsTwoGuardedWrites`; the old pointer-forwarding test is retired.
   - GREEN `69a1ab7` (feat): `effortEdit`, the effort op in `batch`, the Task branch stages, and `itemByID`.
   - Tracer gate: the run was interactive with `human_verify_mode` at its default `end-of-phase`, and the verify was `<automated>`-only. Both verifies were re-run on the committed tree and passed, so the run continued to Task 2.
2. **Task 2 (TDD):**
   - RED `79ca9b5` (test): three tests, including the cross-project subtest.
   - GREEN `d9abfae` (feat): the dev-task ops in `batch`, the User Story branch stages, and the `StateUpdateMsg` success branch is removed.
3. **Task 3 (TDD):** `132420b` (test): `uibypass_test.go`. There is no feat commit, because no production code needed to change (see TDD notes).

## TDD notes

The `tdd-red-evidence` classifier has no `go test` adapter. As in 07-02 and 07-04 through 07-06, the evidence and its semantic assessment are recorded here.

- **RED, Task 1:** `go test -race -count=1 -v ./internal/ui/workitems/ -run '^TestTaskResolveIsTwoGuardedWrites$'` exited 1 with `effort submit: cmd = true, calls = []; staging must send nothing`. The target ran and failed on its planned assertion, because the old handler returned the direct-write closure. This is a valid RED.
- **RED, Task 2:**
  - `TestUserStoryResolveClosesDevTaskThenRecordsEffort` and `TestUserStoryResolveWithoutDevTaskWritesStateOnly` both failed with `effort submit: cmd = true, HasStaged = false`. That is the planned assertion, so both are valid.
  - `TestResolveEffortFailureReportsBoth` passed at RED. Task 1's two-op batch already produces the D-09 behaviour through `confirm.Execute`, so the test pins that behaviour; it does not drive new code.
- **RED, Task 3:** the tests pass on the current tree by design, because Tasks 1 and 2 removed the call sites. To show the tests are not vacuous, two temporary violations were added, the tests were run, and both files were restored with `git checkout -- <file>`:
  - With the pre-plan `workitems.go` in place (`git show 61d4aa2:...`), `TestNoUIFileCallsADOMutation` reported exactly the three removed calls: `workitems.go:361` UpdateWorkItemStateWithEffort, `:376` UpdateWorkItemState and `:385` UpdateWorkItemStateWithEffort.
  - With `Actor: guard.ActorCLI` in `confirm/batch.go`, `TestUIGuardCallsCarryTUIActor` reported `batch.go:36:13: guard.Run options lack Actor: guard.ActorTUI`.
- **GREEN:** the four resolve tests give 4 top-level `--- PASS: Test` lines, plus the subtest PASS. The two structural tests give 2.

## Gate output

- `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`: all 14 test packages are ok, and gofmt is clean.
- `shasum -a 256 -c 07-cli-tests.sha256`: all seven locked CLI test files report OK. `07-cli-audit-test.sha256`: `audit_test.go` reports OK.
- `grep -n UpdateWorkItemState internal/ui/workitems/workitems.go internal/ui/workitems/staging.go` finds nothing.

## Files Created/Modified

- `internal/ui/workitems/staging.go`: `effortEdit`, `stagedEdit.effort`, the multi-op `batch`, and `stage(item, state, effort)`.
- `internal/ui/workitems/workitems.go`:
  - `EffortSubmitMsg` stages for both the Task and User Story branches.
  - The `StateUpdateMsg` handler and type are reduced to pre-check errors.
  - `itemByID` is new.
- `internal/ui/workitems/workitems_test.go`: the retired test and its AST imports are removed.
- `internal/ui/workitems/resolve_test.go`: the four resolve tests, plus `newResolveModel`, `newTaskResolveModel`, `newStoryResolveModel`, `submitEffort`, `previewTaskResolve`, `stageStoryResolve` and `auditChanges`.
- `internal/guard/uibypass_test.go`: the two structural tests, plus `readOnlyClientMethod`, `adoMutations`, `parseUIFiles` and `mutationCalls`.

## Decisions Made

See key-decisions in the frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The plan's scanner flagged the guard's own op constructors**
- **Found during:** Task 3
- **Issue:** The plan says to report "any call whose Fun is a selector whose Sel.Name is in the forbidden set". On the clean tree that reported `pipelines.go:240 QueuePipeline` and `releases.go:239/243 ApproveRelease/RejectRelease`. Those are `guard.QueuePipeline(...)`, `guard.ApproveRelease(...)` and `guard.RejectRelease(...)`, the very op constructors that route writes through the guard. The test could never pass, and "fix the production code" was not possible, because the code was already correct.
- **Fix:** `mutationCalls` skips a selector whose X is the identifier the file imports `sentinovo.ai/bizInt/internal/guard` as, alias included. Every other receiver is still reported, including `m.writer.UpdateWorkItemState` (a guard.Client method call would bypass guard.Run). The snippet tripwire now includes a `guard.ApproveRelease(...)` call and still requires exactly 2 findings.
- **Files modified:** internal/guard/uibypass_test.go
- **Verification:** the clean tree passes. The pre-plan `workitems.go` still yields its three real findings, and no false positive was reported for pipelines or releases.
- **Committed in:** 132420b

**2. [Rule 2 - Missing critical] EffortSubmitMsg for an item no longer loaded**
- **Found during:** Task 1
- **Issue:** The handler now has to find the item by ID. A refresh between `n` and submit could drop it.
- **Fix:** The handler shows `Error: work item #<id> not found`, stages nothing and sends nothing.
- **Committed in:** 69a1ab7

**3. [Scope - minor] itemByID and the StateUpdateMsg fields**
- `itemByID` was extracted because the new handler made the item-by-ID loop the third copy (rule of three). It is used by `updateSelectedItemState` and `getSelectedWorkItem`.
- The now-unwritten `ItemID` and `NewState` fields were removed from `StateUpdateMsg`.

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical), plus the minor item 3. **Impact:** without the scanner fix, the GUARD-02 test could not pass on correct code. There was no scope creep.

## Issues Encountered

None beyond the deviations above.

## Requirements

`requirements-completed` lists GUARD-03, GUARD-05 and GUARD-02, copied from the plan. As the orchestrator instructed, `requirements.mark-complete` was not run, and the REQUIREMENTS.md GUARD checkboxes and traceability rows stay Pending. GUARD-03 still needs 07-08's tab and quit wiring.

## Threat model

- T-07-27: TestTaskResolveIsTwoGuardedWrites (nil originalEstimate at the client, no OriginalEstimate audit change).
- T-07-28: TestResolveEffortFailureReportsBoth (exactly one state call and one effort call).
- T-07-29: TestNoUIFileCallsADOMutation with both tripwires.
- T-07-30: the "dev task in another project" subtest.
- T-07-31: TestUIGuardCallsCarryTUIActor.

No new network, auth, file or schema surface was added. There are no threat flags.

## Known Stubs

None.

## User Setup Required

None.

## Next Phase Readiness

- 07-08 can wire app.go to `ConfirmLeave`, `ProceedMsg` and `WriteBusy` as 07-06 described. A staged resolve uses the same leave path as a staged state cycle, so nothing new is needed for it.
- Human check, deferred to end-of-phase UAT:
  1. In the running TUI, close an Active Task with `n`. Leave Original Estimate blank, set Remaining to 0 and Completed to a positive number, and submit.
  2. Move down. One confirm modal should list the state and the two effort changes, with no Original Estimate line.
  3. Press `y`. ADO History should show Original Estimate unchanged, and `~/.config/adoBizInt/audit.jsonl` should gain two `"actor":"tui"` lines.

## Self-Check: PASSED

- FOUND: internal/ui/workitems/resolve_test.go, internal/guard/uibypass_test.go, internal/ui/workitems/staging.go, internal/ui/workitems/workitems.go
- FOUND commits: 50190bb, 69a1ab7, 79ca9b5, d9abfae, 132420b (`git rev-list --count 61d4aa2..132420b` = 5)
- `TestUpdateWorkItemStateWithEffortCallSitesForwardPointers` exists nowhere except the retirement note in resolve_test.go

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-14*
