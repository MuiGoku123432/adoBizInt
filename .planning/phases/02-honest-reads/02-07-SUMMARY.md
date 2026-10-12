---
phase: 02-honest-reads
plan: 07
subsystem: api
tags: [go, azure-devops, failure-envelope, empty-repository, wiql, gap-closure]

requires:
  - phase: 02-honest-reads
    provides: "Result[T] envelope, appendFailure/classifyFailure/statusCodeOf (02-01), per-repository branch-stat failures (02-02), opDashboardIterations precedent (02-03), per-definition failures (02-06)"
provides:
  - "GetBranches suppresses an empty repository only on status 400 AND a message starting with VS403403: (D-06a); every other branch-stats outcome is recorded with the repository named"
  - "Assumption A4 (empty repository answers 404) retired: a VS403403 under 404 is now recorded"
  - "GetWorkItems records a failed GetRecentIterations lookup as opGetWorkItemsIterations ('get work item iterations') on every getProjectWorkItems return path, keeping the degraded items"
affects: [02-verification, phase-03-cli-json, branches-view, workitems-view]

actuals:
  tokens: 4655
  tasks: 2
  commits: 4
plan_head_before: b86e1b6a45075c81da1d70dd0a5602fe8b1223a5

tech-stack:
  added: []
  patterns:
    - "D-06a: message-prefix recognition is allowed only as a conjunction with a structural status, anchored with strings.HasPrefix, and local to one call"
    - "A sub-request degradation that changes the query (not just its row count) is recorded as its own operation label, as in the dashboard"

key-files:
  created: []
  modified:
    - internal/ado/branches.go
    - internal/ado/branches_test.go
    - internal/ado/workitems.go
    - internal/ado/workitems_test.go

key-decisions:
  - "D-06a implemented as: suppress the branch-stats error only when statusCodeOf reports ok with 400 AND strings.HasPrefix(err.Error(), \"VS403403:\"). err.Error() is used directly (no second errors.As) because WrappedError.Error() returns Message verbatim; a wrapped error whose outer text does not start with the code is recorded, which errs toward noise, never a lost failure"
  - "Assumption A4 (404 for an empty repository) retired; a 404 on the branch-stats call is now a recorded Failure"
  - "A failed iteration lookup in the work-item list is recorded as opGetWorkItemsIterations and the over-inclusive items are kept (D-03 keep-what-answered). If both the lookup and QueryByWiql fail, the project yields two Failure entries, iterations first"

patterns-established:
  - "Empty-repository recognition by status plus documented ADO code prefix, never by free text and never outside branches.go"

requirements-completed: [TRUST-01, TRUST-03]

coverage:
  - id: D1
    description: "GetBranches suppresses the live empty-repository shape (400, ArgumentException, errorCode 0, VS403403:) in value and pointer form, returning Complete=true"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/branches_test.go#TestGetBranchesReportsRepoFailure"
        status: pass
    human_judgment: false
  - id: D2
    description: "GetBranches records every other branch-stats outcome with the repository named: other 400 (other), VS403403 under 404 (other), VS403403 on 403 (auth), 401 (auth), VS403403 with no SDK status (other), timeout (timeout), 500 (other)"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/branches_test.go#TestGetBranchesReportsRepoFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/result_test.go#TestClassifyFailure"
        status: pass
    human_judgment: false
  - id: D3
    description: "GetWorkItems records a failed iteration lookup as opGetWorkItemsIterations against its project, keeps the degraded items, and sets Complete=false, including when the query returns nothing"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsReportsIterationFailure"
        status: pass
    human_judgment: false
  - id: D4
    description: "Live branches list against the org reports complete:true with no VS403403 entries in failures"
    requirement: TRUST-03
    verification:
      - kind: manual
        ref: "adoBizInt branches list --json (end-of-phase, /gsd-verify-work)"
        status: pending
    human_judgment: true

duration: 3min
completed: 2026-10-12
status: complete
---

# Phase 2 Plan 07: Empty repositories recognised by 400 plus VS403403, and the work-item list flags a dropped sprint filter

**GetBranches now suppresses only the live empty-repository shape (HTTP 400 with a message starting `VS403403:`) and records everything else on the branch-stats call. GetWorkItems records a failed iteration lookup as `get work item iterations`, so a list taken without its sprint filter reports `complete:false`.**

## Performance

- **Duration:** about 3 min
- **Started:** 2026-10-12T15:42:06Z
- **Completed:** 2026-10-12T15:45Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Gap 2 (D-06a) closed. `emptyRepositoryStatus = 400` and `emptyRepositoryCode = "VS403403:"` replace the single 404 constant. The per-repository error branch records the failure unless `statusCodeOf` reports 400 **and** `strings.HasPrefix(err.Error(), emptyRepositoryCode)` holds. On the live org, `branches list` should stop reporting `complete:false` and exiting 1 because of empty repositories.
- Assumption A4 is retired. A VS403403 message under 404 is now recorded, and the doc comment cites the 2026-10-12 capture.
- Gap 3 (D-02, T-02-05) closed. `getProjectWorkItems` declares `failures` at the top, records `opGetWorkItemsIterations` when `GetRecentIterations` fails, and returns the slice on all four return paths: the query error, both empty-result returns, and the final return. The caller's local is renamed from `batchFailures` to `projectFailures`.
- Classification is unchanged. `result.go`, `classifyFailure`, `statusCodeOf`, the WIQL text, `iterationClause`, `GetRecentIterations` and `dashboard.go` were not touched.

## Task Commits

Each TDD task produced a RED test commit and then a GREEN fix commit:

1. **Task 1 (tracer): D-06a empty-repository recognition in GetBranches**: `07546af` (test, RED) and `a40606a` (fix, GREEN)
2. **Task 2: iteration-lookup failure recorded in GetWorkItems**: `15e3e1c` (test, RED) and `0c67c57` (fix, GREEN)

No REFACTOR commit was needed.

Tracer feedback gate (Task 1): the plan carries no `gate` attribute, auto mode is off, and `human_verify_mode` defaults to end-of-phase with an automated-only `<verify>`. The verify was re-run after GREEN and passed (5 top-level PASS lines and 9 subtest PASS lines), so expansion to Task 2 continued without a checkpoint.

## RED → GREEN Evidence

RED runs used the task's own test names. Each RED run exited 1 on assertions about the planned behavior. None failed from a compile error. For Task 2, the `opGetWorkItemsIterations` constant was added inert in the RED commit so the test compiled.

| Test | RED (before fix; exit 1) | GREEN |
|------|--------------------------|-------|
| TestGetBranchesReportsRepoFailure/live_VS403403_capture_is_an_empty_repository | `Failures = [{... Operation:get branch stats for repository-one Kind:other Err:VS403403: ...}], want none` | PASS |
| TestGetBranchesReportsRepoFailure/VS403403_pointer_form_is_an_empty_repository | same: recorded as kind other | PASS |
| TestGetBranchesReportsRepoFailure/VS403403_under_404_is_recorded | `Failures = [], want exactly one per-repository entry` (suppressed by the old 404 rule) | PASS |
| TestGetBranchesReportsRepoFailure (other 6 rows) | passed already: other 400, 403 auth, 401 auth, no status, timeout, 500 | PASS |
| TestGetWorkItemsReportsIterationFailure/one_project's_lookup_fails | `Failures = [], want exactly one iteration entry` (so Complete was true) | PASS |
| TestGetWorkItemsReportsIterationFailure/degradation_is_reported_even_when_the_query_returns_nothing | `Failures = [], want exactly one "get work item iterations" entry` | PASS |

`gsd-tools check tdd-red-evidence` parses node:test TAP output and cannot read Go `testing` output. The plan is `type: execute`, so the plan-level gate does not apply. The table above is the RED record, as in 02-06.

Verify gates:
- Task 1: `go test -race -count=1 -v ./internal/ado -run '^(TestGetBranchesReportsRepoFailure|TestGetBranchesReportsProjectFailure|TestGetBranchesClassifiesRejectedToken|TestFetchTimeoutMarksUnreachedBranches|TestClassifyFailure)$'` produced 5 top-level PASS lines, 9 `TestGetBranchesReportsRepoFailure/` PASS lines, and no FAIL.
- Task 2: the six-test run produced 6 top-level PASS lines and no FAIL. No `classification_node_failure_is_warned_and_discarded` line remains.
- Module gate: `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` exits 0, with every package ok.

Verifier probe scenarios, now pinned as tests:
- VS403403 at status 400 is suppressed (rows 1 and 2, `complete:true`).
- VS403403 at 404 is recorded (row 4).
- A classification-node failure in GetWorkItems gives `failures=1 complete=false` (Task 2, subtest 1).

## Files Created/Modified

- `internal/ado/branches.go`: replaces the 404 const with the D-06a two-constant block and a rewritten doc comment; adds the conjunctive suppression in the per-repository error branch.
- `internal/ado/branches_test.go`: `TestGetBranchesReportsRepoFailure` is now a 9-row table. Two rows use full `azuredevops.WrappedError` literals (TypeKey `ArgumentException`, ErrorCode 0) to mirror the live capture. Adds the `azuredevops` import.
- `internal/ado/workitems.go`: adds `opGetWorkItemsIterations` and records it from the iteration error branch. Failures are carried on every return path, and the doc comments are updated.
- `internal/ado/workitems_test.go`: removes the silent-discard subtest and adds `TestGetWorkItemsReportsIterationFailure` (2 subtests).

## Decisions Made

- **D-06a as implemented.** Suppression requires both `statusCodeOf(err)` ok with 400 and the `VS403403:` prefix on `err.Error()`. No second `errors.As` extraction was added: the SDK's `WrappedError.Error()` returns Message verbatim, and `statusCodeOf` already requires a WrappedError in the chain. A wrapped error whose outer text does not begin with the code is recorded, which is the safe direction.
- **A4 retired.** A 404 on the branch-stats call is now a recorded `other` failure.
- **Degraded work items kept.** The iteration Failure sits alongside the over-inclusive items (D-03 precedent). The workitems view already renders the warning as `could not read: <project> (other: get work item iterations)`.
- **Two entries when both calls fail.** If the iteration lookup and QueryByWiql both fail for a project, it yields the iteration Failure followed by the `get work items` Failure. These are two distinct failed sub-requests (D-02).

## Deviations from Plan

None. The plan executed exactly as written.

The two full `WrappedError` literals in the branch test share a small local closure (`liveError`) inside the test function. This avoids spelling out four pointer fields twice and is not shared outside that test.

## Issues Encountered

None.

## User Setup Required

None. No external service configuration is required.

## Human-check items (end-of-phase)

- **Live confirmation (for /gsd-verify-work, not run here, no live ADO calls made):** with a working PAT, `adoBizInt branches list --json` against the Windstream org should report `"complete": true` and no VS403403 entries in `failures`, unless an unrelated failure is present.

## Next Phase Readiness

- Gaps 2 and 3 are closed in code and tests. Gap 1 was closed by 02-06. Phase 2 is ready for orchestrator re-verification.
- REQUIREMENTS.md TRUST statuses were deliberately left unchanged. The orchestrator handles them after re-verification.
- No blockers.

## Self-Check: PASSED

- FOUND: internal/ado/branches.go, internal/ado/branches_test.go, internal/ado/workitems.go, internal/ado/workitems_test.go
- FOUND commits: 07546af, a40606a, 15e3e1c, 0c67c57 (`git rev-list --count b86e1b6..HEAD` = 4 before this SUMMARY)
- Key-link patterns present: `strings.HasPrefix(err.Error(), emptyRepositoryCode)` in branches.go and `appendFailure(failures, project, opGetWorkItemsIterations, err)` in workitems.go
- Module gate exits 0

---
*Phase: 02-honest-reads*
*Completed: 2026-10-12*
