---
phase: 06-core-value-cleanup
plan: 03
subsystem: ado
tags: [go, json-contract, envelope, scope, wiql, tdd]
status: complete

requires:
  - phase: 02
    provides: ado.Result[T] envelope and newResult (Complete derived from failures)
  - phase: 03
    provides: --json serialises Result[T] as the published agent contract
provides:
  - ado.Scope type and Result.Scope (json:"scope", never omitted; null when unscoped)
  - scope disclosed on workitems list, dashboard counts, prs list and releases approvals
  - tie tests binding each scope value to the request the read actually sends
affects: [06-04, cli --json, agents reading the envelope]

actuals:
  tokens: 6800
  tasks: 3
  commits: 6
plan_head_before: b7969fe9a299581395fa8b998882e0cbb564f916

tech-stack:
  added: []
  patterns:
    - "Assign Scope to the newResult value before each return (local-then-assign); newResult itself is untouched"
    - "Disclosure tie tests: capture the request argument in a fake and compare it to the envelope's scope value"

key-files:
  created: []
  modified:
    - internal/ado/result.go
    - internal/ado/workitems.go
    - internal/ado/client.go
    - internal/ado/dashboard.go
    - internal/ado/pullrequests.go
    - internal/ado/releases.go
    - internal/ado/json_test.go
    - internal/ado/wiql_test.go
    - internal/ado/pullrequests_test.go
    - internal/ado/releases_test.go

key-decisions:
  - "Scope fields carry omitempty (a key's presence is the disclosed fact); Result.Scope does not (null must be emitted, D-02)"
  - "Scope is built inside each read per call, so no two envelopes share a pointer"
  - "The work-item scope fields are duplicated in GetWorkItems and GetDashboardCounts with note: comments (two uses, below the rule of three)"

patterns-established:
  - "Server-side filter disclosure derives status values from the same SDK constant the request sends"

requirements-completed: [TRUST-03, OUT-04, CLI-06]

duration: 3min
completed: 2026-10-12
---

# Phase 6 Plan 03: Scope in Envelope Summary

**Every `ado.Result[T]` now marshals a `scope` key. It is an object naming the fixed server-side filter on workitems list, dashboard counts, prs list and releases approvals, and `null` on the other four reads. Each value is tied by test to the WIQL or status filter the request actually sends.**

## Performance

- **Duration:** about 3 min
- **Started:** 2026-10-12T21:50:19Z
- **Completed:** 2026-10-12T21:53:14Z
- **Tasks:** 3 (1 tracer, 2 auto, all TDD)
- **Files modified:** 10

## Published Contract (one-way, D-01 / D-02 / D-15 / D-16)

| Read | `scope` |
|------|---------|
| workitems list | `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}` |
| dashboard counts | `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent","pr_status":"active"}` |
| prs list | `{"pr_status":"active"}` |
| releases approvals | `{"approval_status":"pending"}` |
| branches list, pipelines runs, pipelines definitions, releases list | `null` |

- Inside `Scope`, every field is `omitempty`, so a key appears only when that filter applies. A fully populated Scope marshals exactly `excluded_states`, `iterations`, `pr_status` and `approval_status`; `Scope{}` marshals `{}`.
- On `Result`, `Scope *Scope` is tagged `json:"scope"` with no omitempty, so the key is never absent.
- `pr_status` and `approval_status` come from `string(git.PullRequestStatusValues.Active)` and `string(release.ApprovalStatusValues.Pending)`, the same constants the requests send. No server-supplied text enters scope.
- Scope is assigned after `newResult`, and nothing reads it. It never changes `complete` or `failures`.

## Accomplishments

- Both returns of `GetWorkItems` (including the no-projects early return) and of `GetDashboardCounts` carry scope. `GetPullRequests` and `GetPendingApprovals` set it on their single return.
- Tie tests: `TestGetWorkItemsScope` and `TestGetDashboardCountsScope` assert the captured WIQL contains `[System.State] NOT IN ('` + the scope's states joined in order + `')`. The PR and approvals tests assert the captured `SearchCriteria.Status` and `StatusFilter` equal the scope's value. The WIQL literal is unchanged and `TestWIQLInterpolationStructural` passes untouched.
- `TestGetWorkItemsScope` also checks that a failed project keeps a non-null scope with `complete:false`, and that two calls get distinct pointers.
- `TestUnscopedReadsReportNullScope` pins `"scope":null` on the four D-16 reads, with `complete:true` to show each fixture answered.
- Comment corrections (D-06): "90 days" became "30 days" in the `GetRecentIterations` doc comment and its two in-body comments (client.go), and above the `GetRecentIterations` calls in workitems.go and dashboard.go. No code in client.go changed.

## Task Commits

1. **Task 1 (tracer) RED:** `b1cb70f` test(06-03): add failing tests for the scope envelope key and the work-item scope (D-01, D-02)
2. **Task 1 (tracer) GREEN:** `67c751e` feat(06-03): publish scope in the read envelope and disclose the work-item filter (D-01, D-02)
3. **Task 2 RED:** `df788ae` test(06-03): add failing scope tests for dashboard counts and prs list (D-01)
4. **Task 2 GREEN:** `8bc156b` feat(06-03): disclose the dashboard and prs list filters in scope (D-01)
5. **Task 3 RED:** `bec235a` test(06-03): add failing approvals scope test and the D-16 null-scope pin (D-15, D-16)
6. **Task 3 GREEN:** `2b06efa` feat(06-03): disclose the pending-only approvals filter in scope (D-15)

Tracer feedback gate (interactive, end-of-phase, automated-only verify): Task 1's `<verify>` and the full module suite were both green before Task 2 started.

## RED → GREEN Evidence

**Task 1 RED, stage 1** (compile failure, committed as `b1cb70f`):
```
internal/ado/json_test.go:174:19: undefined: Scope
internal/ado/json_test.go:474:12: result.Scope undefined (type Result[Branch] has no field or method Scope)
internal/ado/wiql_test.go:491:33: result.Scope undefined (type Result[WorkItem] has no field or method Scope)
FAIL	sentinovo.ai/bizInt/internal/ado [build failed]
```
**Task 1 RED, stage 2** (Scope type and Result field added, no read setting it):
```
--- FAIL: TestGetWorkItemsScope/scope_matches_the_query
--- FAIL: TestGetWorkItemsScope/early_return_carries_the_scope
--- FAIL: TestGetWorkItemsScope/a_failed_project_keeps_the_scope
--- FAIL: TestGetWorkItemsScope/results_do_not_share_a_scope
```
**Task 1 GREEN:** 8/8 top-level tests pass (TestJSONFieldNames, TestNewResultScopeIsNull, TestNewResultEmitsEmptyArraysNotNull, TestGetWorkItemsScope, TestGetWorkItemsUsesProjectMacro, TestWIQLInterpolationStructural, TestGetWorkItemsReportsProjectFailure, TestGetWorkItemsReportsIterationFailure), plus 4/4 TestGetWorkItemsScope subtests.

**Task 2 RED:**
```
pullrequests_test.go:387: Scope is nil, want the PR status filter
pullrequests_test.go:414: scope = null, want {"pr_status":"active"}
wiql_test.go:590: Scope is nil, want the dashboard filters
wiql_test.go:621: scope = null, want {"excluded_states":["Done","Closed","Removed"],"iterations":"recent","pr_status":"active"}
```
**Task 2 GREEN:** 8/8 top-level tests pass, with 2/2 subtests under each of TestGetDashboardCountsScope and TestGetPullRequestsScope.

**Task 3 RED:**
```
releases_test.go:477: Scope is nil, want the approval status filter
--- FAIL: TestGetPendingApprovalsScope
--- PASS: TestUnscopedReadsReportNullScope (4/4 subtests; a D-16 pin, as the plan predicted)
```
**Task 3 GREEN:** 5/5 top-level tests pass and 4/4 TestUnscopedReadsReportNullScope subtests. `go test ./internal/ado -run Scope` runs all 6 scope tests and they pass. Full gate `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` exits 0. The TUI packages compile unchanged.

## Files Created/Modified

- `internal/ado/result.go`: `type Scope` with a doc comment, and `Result.Scope`
- `internal/ado/workitems.go`: per-call scope on both returns; 30-day comment
- `internal/ado/client.go`: three comments changed from 90 to 30 days (comments only)
- `internal/ado/dashboard.go`: per-call scope with `PRStatus` on both returns; 30-day comment
- `internal/ado/pullrequests.go`: `{pr_status: active}` on the return
- `internal/ado/releases.go`: `{approval_status: pending}` on the `GetPendingApprovals` return
- `internal/ado/json_test.go`: scope in the 8 Result key sets, a `Scope` subtest, and `TestNewResultScopeIsNull`
- `internal/ado/wiql_test.go`: `TestGetWorkItemsScope`, `TestGetDashboardCountsScope`
- `internal/ado/pullrequests_test.go`: `TestGetPullRequestsScope`
- `internal/ado/releases_test.go`: `TestGetPendingApprovalsScope`, `TestUnscopedReadsReportNullScope`

## Decisions Made

- `TestGetWorkItemsScope` uses a small subtest-local closure to marshal and nil-check the scope, because its four subtests share that check. Nothing was extracted to package level.
- The approvals scope test builds its own client (no `releaseDefinitions`), because only `GetApprovals` is exercised. `releaseTestClient` is still used for the `releases list` null-scope subtest, as the plan specifies.

## Deviations from Plan

None. The plan was executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None. Live check for /gsd-verify-work (does not block this plan): against the org, `adoBizInt workitems list --json | jq .scope` prints the excluded-states object, and all eight reads keep their current `complete` values.

## Next Phase Readiness

Plan 06-04 can render `Result.Scope` for humans and in help. The key names and values above are the published contract it should describe.

## Self-Check: PASSED

- FOUND: all 10 modified files
- FOUND commits: b1cb70f, 67c751e, df788ae, 8bc156b, bec235a, 2b06efa
- Measured `git rev-list --count b7969fe..HEAD` = 6
