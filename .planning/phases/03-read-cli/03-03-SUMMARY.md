---
phase: 03-read-cli
plan: 03
subsystem: api
tags: [go, filters, refactor, deduplication, tdd, table-driven]

requires:
  - phase: 03-read-cli
    provides: run() int, readClient seam, dispatch table, eight read verbs
  - phase: 02-read-correctness
    provides: ado.Result[T] envelope, WorkItem and PullRequest field shapes
provides:
  - "internal/ado/filters.go with five pure exported predicates"
  - "the single repository-wide doneStates definition, relocated into internal/ado"
  - "IsDoneState, IsTaskType, MatchesIdentity, MatchesProject, MatchesRepository"
  - "empty-slice-means-match-all as a tested rule the CLI can rely on for D-18"
  - "ordered-row equivalence tests for both view packages' buildRows()"
affects: [03-04, 03-05]

actuals:
  tokens: 11000
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "domain predicates live in internal/ado; view packages consume, never redefine"
    - "empty configured slice disables a filter rather than matching nothing"
    - "row-equivalence tests compare table.RowData in order, not table.Row (NewRow stamps a global id)"

key-files:
  created:
    - internal/ado/filters.go
    - internal/ado/filters_test.go
    - internal/ui/workitems/workitems_rows_test.go
    - internal/ui/prs/prs_rows_test.go
  modified:
    - internal/ui/workitems/workitems.go
    - internal/ui/prs/prs.go
    - internal/ado/branches.go

key-decisions:
  - "hide-tasks was promoted alongside the three CONTEXT.md named filters; leaving it inline would have kept workitems filtering in two places, which is exactly what CLI-06 forbids"
  - "MatchesRepository also replaced the inline block inside GetBranches, making the promotion the three-way deduplication the plan's key_links described rather than a two-way one"
  - "matchesAny is unexported and fronted by two named predicates, so a call site cannot filter repositories against a project list without a compile error"
  - "Row equivalence is asserted on ordered []table.RowData, not []table.Row, because table.NewRow stamps each row with a process-global incrementing id that is not part of the rendered output"
  - "The priority column gate is pinned by its own test, independently of the sort, because it sits 36 lines past the sort inside the row loop"

patterns-established:
  - "Predicate doc comments state the behavior, name the rejected alternative and cite the D-number, mirroring branches.go"
  - "RED commits stub the function bodies so the failing test fails on its assertions rather than on the build"

requirements-completed: [CLI-06, CLI-01]

coverage:
  - id: D1
    description: "The five filter rules exist as one pure exported function each in internal/ado, with the empty-input and case-folding edges pinned"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterIsDoneState"
        status: pass
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterIsTaskType"
        status: pass
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesIdentity"
        status: pass
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesProject"
        status: pass
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesRepository"
        status: pass
    human_judgment: false
  - id: D2
    description: "Exactly one doneStates definition exists repository-wide, and it lives in internal/ado"
    requirement: CLI-06
    verification:
      - kind: other
        ref: "grep -rn 'doneStates' --include='*.go' . | grep -v _test.go matches only internal/ado/filters.go"
        status: pass
      - kind: other
        ref: "grep -rn 'doneStates' internal/ui/ returns nothing"
        status: pass
    human_judgment: false
  - id: D3
    description: "An empty project or repository slice disables that filter rather than matching nothing, so a caller who requested no filter receives the unfiltered set (T-03-08)"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesProject/empty_slice_keeps_an_item_a_non-matching_slice_would_drop"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_rows_test.go#TestBuildRowsFilterEquivalence/empty_slices_keep_everything"
        status: pass
    human_judgment: false
  - id: D4
    description: "The promoted by-repo predicate makes the same keep/drop decision as the inline rule it replaced in GetBranches (T-03-09)"
    verification:
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesRepositoryEqualsInlineBranchesRule"
        status: pass
      - kind: unit
        ref: "internal/ui/branches package tests still green after the rewire"
        status: pass
    human_judgment: false
  - id: D5
    description: "buildRows() returns the same ordered rows after the extraction, including the order-bearing priority sort (RESEARCH Pitfall 3, CLI-01)"
    requirement: CLI-01
    verification:
      - kind: unit
        ref: "internal/ui/workitems/workitems_rows_test.go#TestBuildRowsFilterEquivalence"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/workitems_rows_test.go#TestBuildRowsPrioritySortOrder"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_rows_test.go#TestBuildRowsFilterEquivalence"
        status: pass
    human_judgment: false
  - id: D6
    description: "Both surviving assignedToMeFilter gates still read the model flag, so the Priority column still renders a 1-indexed position when the filter is on and an empty string when it is off"
    requirement: CLI-01
    verification:
      - kind: unit
        ref: "internal/ui/workitems/workitems_rows_test.go#TestBuildRowsPriorityColumnGate"
        status: pass
      - kind: other
        ref: "grep -c 'm.assignedToMeFilter && m.priorities != nil' internal/ui/workitems/workitems.go == 2"
        status: pass
    human_judgment: false
  - id: D7
    description: "TUI defaults are unchanged: assigned-to-me and hide-done still default on, hide-tasks still defaults off"
    requirement: CLI-01
    verification:
      - kind: other
        ref: "grep -c 'assignedToMeFilter: *true' == 1 and 'hideDoneFilter: *true' == 1 in workitems.go"
        status: pass
    human_judgment: false
  - id: D8
    description: "The work items and pull requests views behave identically against a live Azure DevOps organisation after the filter promotion"
    verification: []
    human_judgment: true
    rationale: "The row-equivalence tests pin buildRows() against fixtures, but whether the rendered TUI still feels and behaves the same with real data, a real identity and real priorities on disk needs a human with a PAT and an interactive terminal. Belongs in the phase UAT."

duration: 26 min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 03: Filter Promotion into internal/ado Summary

**Five filtering rules that lived inline across three files are now one pure exported function each in `internal/ado/filters.go`, with ordered-row equivalence tests proving the TUI renders exactly what it rendered before.**

## Performance

- **Duration:** 26 min
- **Tasks:** 2
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments

- `doneStates` has exactly one definition in the repository and it lives in `internal/ado`. `grep -rn 'doneStates' internal/ui/` returns nothing — the duplication CLI-06 exists to remove is gone rather than relocated.
- Five predicates — `IsDoneState`, `IsTaskType`, `MatchesIdentity`, `MatchesProject`, `MatchesRepository` — are pure functions with no model, no client and no view dependency, so the CLI in 03-04 consumes the same rule the TUI does with nothing to keep in sync.
- The promotion turned out to be a genuine **three-way** deduplication: `MatchesRepository` also replaced the inline block inside `GetBranches`, not just the one in the pull requests view.
- The empty-slice-means-match-all rule is preserved and tested at both levels. This is what makes D-18's "the CLI defaults to no filtering" free — the CLI passes an empty slice — and it is the mitigation for T-03-08, over-filtering presented as a complete read.
- The empty-identity no-op survived the extraction into `MatchesIdentity`. Phase 1 D-12 leaves identity resolution non-fatal, so an empty cache is legitimate; a match-nothing rule would have rendered an empty backlog indistinguishable from a real one.
- Both `assignedToMeFilter && priorities != nil` gates survived and are pinned separately. The one at the row loop is 36 lines past the sort and decides whether the Priority column renders a number; an order-only test would not have caught its loss.
- First test files in `internal/ui/workitems` and `internal/ui/prs` covering `buildRows()` at all — the highest-risk refactor in this phase now has a regression net that did not previously exist.

## Task Commits

1. **Task 1 RED: failing predicate table** — `989f2ee` (test)
2. **Task 1 GREEN: five promoted predicates** — `453f48d` (feat)
3. **Task 2: rewire workitems, prs and branches** — `d84b63a` (refactor)

## Files Created/Modified

- `internal/ado/filters.go` — the five predicates, the relocated `doneStates` map, and the unexported `matchesAny` shared by the two set-membership predicates
- `internal/ado/filters_test.go` — table-driven coverage of every behavior row plus the inline-rule equivalence check against `branches.go`
- `internal/ui/workitems/workitems.go` — `buildRows()` calls the promoted predicates; the local `doneStates` declaration deleted; the `StateUpdateMsg` handler's done check rewired too
- `internal/ui/workitems/workitems_rows_test.go` — ordered-row equivalence across all four filter axes, the priority sort, the priority column gate and the empty-identity case
- `internal/ui/prs/prs.go` — `buildRows()` calls `MatchesProject`/`MatchesRepository`; the two hand-built lookup sets removed
- `internal/ui/prs/prs_rows_test.go` — ordered-row equivalence across the project and repository axes including both empty-slice cases
- `internal/ado/branches.go` — the inline repository filter and its lookup set replaced by `MatchesRepository`

## Decisions Made

- **`hide-tasks` was promoted.** CONTEXT.md's D-17 names three workitems filters; there are four. Leaving `hideTasksFilter` inline would have left workitems filtering in two places after a plan whose whole purpose is removing that split. Recorded in the plan's `<source_corrections>` and executed as written.
- **Row comparison is on `table.RowData`, not `table.Row`.** `table.NewRow` stamps each row with a process-global incrementing id. A `reflect.DeepEqual` on `[]table.Row` therefore fails for structurally identical rows and passes only by accident of allocation order. The projection preserves order and compares every rendered column, which is what the criterion was protecting.
- **`matchesAny` stays unexported.** A single exported generic set-membership helper would let a call site filter repositories against a project list with no compile error. Two named predicates cost one line of indirection and make that mistake visible at the call site.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `doneStates` had a second consumer outside `buildRows()`**
- **Found during:** Task 2
- **Issue:** The plan's `read_first` scoped Task 2 to `buildRows()`, but `doneStates` is also read at `workitems.go:434` in the `StateUpdateMsg` handler, which auto-removes an item from the priority list when it transitions to a done state. Deleting the local map broke the build there.
- **Fix:** Rewired that call site to `ado.IsDoneState(msg.NewState)` as well.
- **Files modified:** internal/ui/workitems/workitems.go
- **Verification:** `go build ./...` green; `grep -rn 'doneStates' internal/ui/` returns nothing.
- **Committed in:** `d84b63a`

**2. [Rule 2 - Missing Critical] The inline repository filter in `GetBranches` was left as a second copy**
- **Found during:** Task 2
- **Issue:** The plan's `files_modified` does not list `internal/ado/branches.go`, but its `key_links` states the promoted by-repo predicate is a **three-way** deduplication. Leaving `branches.go:92` inline would have kept a second live copy of the promoted rule, contradicting the plan's own stated intent and the CLI-06 requirement.
- **Fix:** Replaced the `validRepos` map build and the membership test with a call to `MatchesRepository`.
- **Files modified:** internal/ado/branches.go
- **Verification:** `TestFilterMatchesRepositoryEqualsInlineBranchesRule` asserts the decisions match for every combination of five repository names and five configured sets; `go test ./internal/ui/branches/ ./internal/ado/` green.
- **Committed in:** `d84b63a`

---

**Total deviations:** 2 auto-fixed (both Rule 2 — missing critical).
**Impact on plan:** Both extend the promotion to call sites the plan intended to cover but did not enumerate. No scope creep: no new behavior, no new API surface beyond the five planned predicates.

## Issues Encountered

- The first draft of the row-equivalence tests compared `[]table.Row` directly and failed on every case with structurally identical data. `table.NewRow` assigns a monotonically increasing global id, so no literal expectation can ever equal a freshly built row. Resolved by projecting to `[]table.RowData`, which preserves order and every rendered column while dropping the field that is not output.

## TDD Gate Compliance

Task 1 carried `tdd="true"` and completed both gates: RED at `989f2ee` (`test(03-03)`, predicate bodies stubbed to return `false` so the assertions rather than the build failed), GREEN at `453f48d` (`feat(03-03)`). No REFACTOR commit was needed. Task 2 is not a TDD task; its tests were written alongside the rewire as behavior-equivalence pins.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Ready for 03-04, which promotes the remaining search predicate and wires `--project` through to the reads. The five predicates are exported, pure and tested, so the CLI filter flags have a single implementation to call. Note for 03-04: the search predicate is still inline in both `buildRows()` bodies by design — this plan deliberately left it there.

**Carried to phase UAT:** live-organisation behavioral equivalence of the two rewired views (coverage entry D8).

## Self-Check: PASSED

- All four created files exist on disk.
- All three commit hashes resolve in `git log`.
- `go build ./... && go vet ./... && go test ./...` all green.
- `grep -rn 'doneStates' --include='*.go' . | grep -v _test.go` matches only `internal/ado/filters.go`.
- `grep -c 'm.assignedToMeFilter && m.priorities != nil' internal/ui/workitems/workitems.go` returns 2; both TUI defaults still return 1.

---
*Phase: 03-read-cli*
*Completed: 2026-09-09*
