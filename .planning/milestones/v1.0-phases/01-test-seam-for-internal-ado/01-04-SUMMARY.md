---
phase: 01-test-seam-for-internal-ado
plan: 04
subsystem: testing
tags: [go, azure-devops, characterization-tests, mutation-testing, error-swallowing]

requires:
  - phase: 01-test-seam-for-internal-ado/01-01
    provides: Work-item API seam, fail-fast fake convention, and mutation-check pattern
  - phase: 01-test-seam-for-internal-ado/01-02
    provides: Git and build API seams with fail-fast fakes
  - phase: 01-test-seam-for-internal-ado/01-03
    provides: Seven-step mutation protocol and completed destructive-write coverage
provides:
  - Mutation-proven characterization test for GetWorkItems project-level failure swallowing
  - Executable proof that classification-node failures are warned and discarded rather than failing a project
  - Mutation-proven characterization test for dashboard in-place WIQL swallowing with continued pull request aggregation
  - Phase 2 warning that dashboard inversion requires getProjectCounts propagation rather than loop-only edits
affects: [01-05, phase-2-honest-reads, TRUST-01]

actuals:
  tokens: 2130
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - Two-project characterization fixtures with one injected query failure
    - Mutation proof through captured source diff, observed red, git checkout restoration, and green rerun
    - Unstubbed fail-fast fake methods proving guarded calls remain unreachable

key-files:
  created:
    - internal/ado/dashboard_test.go
  modified:
    - internal/ado/workitems_test.go

key-decisions:
  - "Inject GetWorkItems failure at QueryByWiql; classification-node errors are warned and discarded and therefore do not fail a project."
  - "Assert dashboard pull requests as a non-zero sum of distinct per-project counts so project skipping cannot satisfy the test."
  - "Use continue-to-return propagation as the GetWorkItems mutation because deleting continue alone is behaviorally inert when the failed result is nil."

patterns-established:
  - "Read characterization: drive configured-project fallback with two projects, fail one at the first propagated API error, and assert the surviving aggregate plus nil error."
  - "Structural outlier coverage: prove downstream calls still run for the dashboard project whose WIQL query failed."

requirements-completed: [SEAM-04]

coverage:
  - id: D1
    description: "GetWorkItems omits a project whose WIQL query fails, returns the surviving project's item, and returns a nil error."
    requirement: SEAM-04
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsSwallowsProjectFailure"
        status: pass
      - kind: other
        ref: "mutation check: workitems.go continue replaced by return nil, err"
        status: pass
    human_judgment: false
  - id: D2
    description: "A classification-node failure does not fail GetWorkItems and both projects continue through the query path."
    requirement: SEAM-04
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsSwallowsProjectFailure/classification_node_failure_is_warned_and_discarded"
        status: pass
    human_judgment: false
  - id: D3
    description: "GetDashboardCounts treats one project's WIQL failure as zero work items while retaining distinct pull request contributions from both projects."
    requirement: SEAM-04
    verification:
      - kind: unit
        ref: "internal/ado/dashboard_test.go#TestGetDashboardCountsSwallowsProjectFailure"
        status: pass
      - kind: other
        ref: "mutation check: dashboard.go QueryByWiql error returns nil, err"
        status: pass
    human_judgment: false
  - id: D4
    description: "The repository builds, vets, and passes race-enabled tests without a production mutation or TUI change."
    requirement: SEAM-04
    verification:
      - kind: integration
        ref: "gofmt -l internal/ && go build ./... && go vet ./... && go test ./... -race -cover"
        status: pass
    human_judgment: false

duration: 18 min
completed: 2026-09-04
status: complete
---

# Phase 1 Plan 4: Aggregate Read Characterization Summary

**Mutation-proven tests now distinguish GetWorkItems project skipping from the dashboard's in-place WIQL swallow while documenting the classification-node trap.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-09-04T22:29:26Z
- **Completed:** 2026-09-04T22:47:33Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added `TestGetWorkItemsSwallowsProjectFailure` with a failing-project query, a non-empty surviving result, nil aggregate error, and an executable classification-node trap subtest.
- Added `TestGetDashboardCountsSwallowsProjectFailure` with four surviving work items and distinct non-zero pull request counts of two and three, proving the failing project's pull requests remain in the aggregate.
- Demonstrated both characterization tests red under reachable Phase 2-style propagation mutations, captured both diff hunks, restored production files from Git, and reran green.
- Preserved the TUI and production behavior; full build, vet, race, and test verification passed with `internal/ado` coverage at 33.3%.

## Task Commits

Each task was committed atomically:

1. **Task 1: GetWorkItems -- canonical swallow and classification-node trap** -- `4ce430c` (test)
2. **Task 2: GetDashboardCounts -- in-place query swallow and continued PR aggregation** -- `a0626fe` (test)

## Files Created/Modified

- `internal/ado/workitems_test.go` -- Adds the canonical project-failure characterization and classification-node trap subtest.
- `internal/ado/dashboard_test.go` -- Adds the dashboard structural-outlier characterization with a fail-fast build fake.

## Mutation Evidence

Both transient production mutations were captured while present, made the relevant test fail, were reverted with `git checkout -- <file>`, and were followed by a green rerun and clean production-file status.

### 1. GetWorkItems project-level swallow

A literal deletion of `continue` was tried first and correctly exposed a flaw in the planned mutation: `items` is nil on the propagated query error, so `append(allItems, items...)` is a no-op and observable behavior does not change. The non-vacuous mutation therefore replaces the swallow with the Phase 2 inversion, `return nil, err`. This removes the `continue`, makes the query error reach the caller, and turns the characterization test red.

```diff
diff --git a/internal/ado/workitems.go b/internal/ado/workitems.go
index 185a94d..8ba986d 100644
--- a/internal/ado/workitems.go
+++ b/internal/ado/workitems.go
@@ -74,7 +74,7 @@ func (c *Client) GetWorkItems(ctx context.Context, projects []string) ([]WorkIte
 		items, err := c.getProjectWorkItems(ctx, project)
 		if err != nil {
 			log.Error("Failed to get work items for project", "project", project, "error", err)
-			continue
+			return nil, err
 		}
 		allItems = append(allItems, items...)
 	}
```

Observed failing output with that mutation present:

```text
=== RUN   TestGetWorkItemsSwallowsProjectFailure
=== RUN   TestGetWorkItemsSwallowsProjectFailure/query_failure_skips_only_the_failed_project
2026/09/04 17:40:20 INFO No iterations found for project project=project-query-fails
2026/09/04 17:40:20 INFO Executing WIQL query project=project-query-fails query="SELECT [System.Id], [System.Title], [System.Description], [Microsoft.VSTS.Scheduling.StoryPoints], [System.WorkItemType], [System.State], [System.AssignedTo] FROM WorkItems WHERE [System.TeamProject] = 'project-query-fails' AND [System.State] NOT IN ('Done', 'Closed', 'Removed') ORDER BY [System.Id] DESC"
2026/09/04 17:40:20 ERROR Failed to get work items for project project=project-query-fails error="query failed"
    workitems_test.go:62: GetWorkItems() error = query failed, want nil
--- FAIL: TestGetWorkItemsSwallowsProjectFailure (0.00s)
    --- FAIL: TestGetWorkItemsSwallowsProjectFailure/query_failure_skips_only_the_failed_project (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.511s
FAIL
```

### 2. GetDashboardCounts in-place swallow

The mutation was applied at the reachable `QueryByWiql` error site inside `getProjectCounts`. The unreachable outer `continue` at `dashboard.go:49-51` was not changed: `getProjectCounts` returns nil errors in production, so that branch cannot prove the current swallow.

```diff
diff --git a/internal/ado/dashboard.go b/internal/ado/dashboard.go
index 9492ba4..b01edcc 100644
--- a/internal/ado/dashboard.go
+++ b/internal/ado/dashboard.go
@@ -100,6 +100,7 @@ func (c *Client) getProjectCounts(ctx context.Context, project string) (*Dashboa
 	workItems, err := c.workitemClient.QueryByWiql(ctx, query)
 	if err != nil {
 		log.Error("Failed to query work items", "project", project, "error", err)
+		return nil, err
 	} else if workItems != nil && workItems.WorkItems != nil {
 		counts.WorkItems = len(*workItems.WorkItems)
 		log.Info("Dashboard work item count", "project", project, "count", counts.WorkItems)
```

Observed failing output with that mutation present:

```text
=== RUN   TestGetDashboardCountsSwallowsProjectFailure
2026/09/04 17:44:30 INFO No iterations found for project project=project-query-fails
2026/09/04 17:44:30 INFO Executing dashboard WIQL query project=project-query-fails query="SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = 'project-query-fails' AND [System.State] NOT IN ('Done', 'Closed', 'Removed')"
2026/09/04 17:44:30 ERROR Failed to query work items project=project-query-fails error="query failed"
2026/09/04 17:44:30 ERROR Failed to get counts for project project=project-query-fails error="query failed"
2026/09/04 17:44:30 INFO No iterations found for project project=project-query-succeeds
2026/09/04 17:44:30 INFO Executing dashboard WIQL query project=project-query-succeeds query="SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = 'project-query-succeeds' AND [System.State] NOT IN ('Done', 'Closed', 'Removed')"
2026/09/04 17:44:30 INFO Dashboard work item count project=project-query-succeeds count=4
2026/09/04 17:44:30 INFO Dashboard counts fetched total_work_items=4 total_prs=3 total_pipelines=0
    dashboard_test.go:88: GetDashboardCounts().PRs = 3, want 5 from both projects
--- FAIL: TestGetDashboardCountsSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.482s
FAIL
```

## Verification

- `gofmt -l internal/` -- passed with no output.
- `go build ./...` -- passed.
- `go vet ./...` -- passed.
- `go test ./internal/ado/ -run SwallowsProjectFailure -v` -- passed with exactly two top-level characterization tests and both work-item subtests.
- `go test ./... -race -cover` -- passed; `internal/ado` coverage is 33.3%.
- `git diff --exit-code -- internal/ado/workitems.go internal/ado/dashboard.go` -- passed; no deliberate mutation remains.
- Scope check against the pre-plan commit -- only `internal/ado/workitems_test.go` and `internal/ado/dashboard_test.go` changed; no `internal/ui/`, `cmd/`, `go.mod`, or `go.sum` file changed.
- Gortex post-edit contract verification found no caller or implementor violations for either test symbol.

## Decisions Made

- Injected both failures at `QueryByWiql`. A classification-node error is warned and discarded in `getProjectWorkItems`, so it cannot exercise the project-level swallow.
- Used distinct positive PR counts for both dashboard projects and asserted their sum. This makes the failing project's continued PR contribution observable and distinguishes in-place swallowing from project skipping.
- Left the dashboard's outer `continue` untouched because it is unreachable under production `getProjectCounts` behavior.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Plan bug] Replaced a behaviorally inert work-item mutation with the actual inversion**
- **Found during:** Task 1 (GetWorkItems -- canonical swallow and classification-node trap)
- **Issue:** Deleting `continue` alone left the test green because the failed `getProjectWorkItems` call returns a nil slice and appending nil changes nothing.
- **Fix:** Restored the source, then replaced `continue` with `return nil, err`, which is the Phase 2 behavior the characterization test must reject today.
- **Files modified:** `internal/ado/workitems.go` transiently; fully restored before commit.
- **Verification:** The corrected mutation produced the recorded `FAIL`, the green rerun passed, and production source is clean.
- **Committed in:** `4ce430c` (test change; no mutation committed)

---

**Total deviations:** 1 auto-fixed (1 plan bug).
**Impact on plan:** The correction made the non-vacuity proof meaningful without changing scope or production behavior.

## Issues Encountered

- Context7 was unavailable, so the pinned Azure DevOps SDK fixture types were verified using `go doc` from the resolved local module.
- The first work-item mutation attempt exposed an impossible assertion in the plan rather than a test defect. Its correction and evidence are documented above.

## Known Stubs

None.

## Threat Surface

No production endpoint, authentication path, file access, schema, or network surface was added. Both tests terminate at package-local fakes, use fixture project names only, and contain no credentials or organization URLs.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- The two work-item-query reads are pinned for Phase 2's TRUST-01 inversion.
- Plan 01-05 can apply the same two-project pattern to the remaining six aggregate reads and enforce the eight-test phase gate.
- Phase 1 remains in progress; plan 01-05 remains.

## Self-Check: PASSED

- Created `internal/ado/dashboard_test.go` and modified `internal/ado/workitems_test.go`; both exist.
- Task commits `4ce430c` and `a0626fe` exist.
- All task acceptance criteria and plan-level formatting, build, vet, race, coverage, and test commands passed.
- Both production mutation targets were restored, and no deliberate production mutation remains.
- No stubs, skipped tests, unrun verification, or unexpected source changes remain.

---
*Phase: 01-test-seam-for-internal-ado*
*Completed: 2026-09-04*
