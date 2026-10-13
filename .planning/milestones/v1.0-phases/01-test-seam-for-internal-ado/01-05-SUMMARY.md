---
phase: 01-test-seam-for-internal-ado
plan: 05
subsystem: testing
tags: [go, azure-devops, characterization-tests, mutation-testing, error-swallowing]

requires:
  - phase: 01-test-seam-for-internal-ado/01-01
    provides: Package-local ADO seams and fail-fast fake convention
  - phase: 01-test-seam-for-internal-ado/01-02
    provides: Build, Git, and release interfaces plus hand-written fakes
  - phase: 01-test-seam-for-internal-ado/01-03
    provides: Mutation-check protocol and write-path regression suite
  - phase: 01-test-seam-for-internal-ado/01-04
    provides: Work-item and structurally distinct dashboard swallow characterizations
provides:
  - Six mutation-proven aggregate-read characterization tests
  - Exactly eight anchored top-level SwallowsProjectFailure tests across Phase 1
  - Non-vacuous fixtures that bypass both pipeline empty-config guards and release filtering
  - Explicit carry-forward of all three unresolved SEAM probe assumptions
  - Build, vet, race, and 57.2% internal/ado coverage proof
affects: [phase-2-honest-reads, TRUST-01, gsd-verify-work]

actuals:
  tokens: 2941
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - Two-project fixtures return poisoned data with an error so deleting continue becomes observable
    - Aggregate-read characterization names use one anchored top-level gate
    - Config-dependent reads receive explicit non-empty matching filter slices

key-files:
  created:
    - internal/ado/branches_test.go
  modified:
    - internal/ado/pipelines_test.go
    - internal/ado/pullrequests_test.go
    - internal/ado/releases_test.go

key-decisions:
  - "Return poisoned non-nil fixture data with each injected project error so literal continue removal is observably red rather than behaviorally inert."
  - "Set non-empty matching pipelines slices in both pipeline tests and a matching releaseDefinitions slice in both release client literals."
  - "Count only anchored top-level tests with ^=== RUN   TestGet.*SwallowsProjectFailure$ so the work-item trap subtest remains excluded without deletion."
  - "Preserve the dashboard characterization as a structural outlier whose Phase 2 inversion must propagate from getProjectCounts rather than edit only outer loops."

patterns-established:
  - "Poisoned error return: fake calls return both an error and identifiable data, proving continue is the barrier that excludes failed-project output."
  - "Phase gate: enumerate tests independently and count only exact top-level RUN lines."

requirements-completed: [SEAM-04]

coverage:
  - id: D1
    description: "Pipeline runs, pipeline definitions, and branches return only the surviving project's non-empty result while swallowing one project failure."
    requirement: SEAM-04
    verification:
      - kind: unit
        ref: "go test ./internal/ado/ -run 'TestGetPipelineRunsSwallows|TestGetPipelineDefinitionsSwallows|TestGetBranchesSwallows' -v -cover"
        status: pass
      - kind: other
        ref: "three continue-removal mutation checks recorded below"
        status: pass
    human_judgment: false
  - id: D2
    description: "Pull requests, releases, and pending approvals return only the surviving project's non-empty result while swallowing one project failure."
    requirement: SEAM-04
    verification:
      - kind: unit
        ref: "go test ./internal/ado/ -run 'TestGetPullRequestsSwallows|TestGetReleasesSwallows|TestGetPendingApprovalsSwallows' -v -cover"
        status: pass
      - kind: other
        ref: "three continue-removal mutation checks recorded below"
        status: pass
    human_judgment: false
  - id: D3
    description: "Exactly eight top-level aggregate-read characterizations map one-to-one to the eight reads while retaining the GetWorkItems trap subtest."
    requirement: SEAM-04
    verification:
      - kind: integration
        ref: "test 8 -eq $(go test ./internal/ado/ -run SwallowsProjectFailure -v 2>/dev/null | grep -c '^=== RUN   TestGet.*SwallowsProjectFailure$')"
        status: pass
    human_judgment: false
  - id: D4
    description: "The repository builds and vets, the race-enabled suite passes, internal/ado reaches 57.2% coverage, and TUI, command, and dependency files remain unchanged."
    requirement: SEAM-04
    verification:
      - kind: integration
        ref: "gofmt -l internal/ && go build ./... && go vet ./... && go test ./... -race -cover"
        status: pass
    human_judgment: false
  - id: D5
    description: "SEAM-01, SEAM-02, and SEAM-04 interpretation assumptions remain flagged for end-of-phase human judgment."
    requirement: SEAM-04
    verification: []
    human_judgment: true
    rationale: "Requirement wording does not make these three interpretation claims compiler-checkable; they must remain assumptions for /gsd-verify-work."

duration: 11 min
completed: 2026-09-04
status: complete
---

# Phase 1 Plan 5: Aggregate Read Phase Gate Summary

**Six mutation-proven characterizations complete the eight-read TRUST-01 baseline, with both pipeline short circuits defeated and the dashboard outlier preserved.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-09-04T22:51:54Z
- **Completed:** 2026-09-04T23:03:08Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Added non-empty surviving-result characterizations for pipeline runs, pipeline definitions, branches, pull requests, releases, and pending approvals.
- Proved all six tests are non-vacuous by deleting each production `continue`, capturing the exact diff, observing the corresponding test fail on poisoned failed-project data, restoring from Git, and rerunning green.
- Confirmed exactly eight top-level `TestGet*SwallowsProjectFailure` tests with the anchored pattern while retaining the intentionally excluded `GetWorkItems` trap subtest.
- Passed formatting, full build, vet, race, and coverage checks; `internal/ado` reports 57.2% coverage, and no `internal/ui/`, `cmd/`, `go.mod`, or `go.sum` path changed during Phase 1.

## Task Commits

Each task was committed atomically:

1. **Task 1: Pipelines and branches** -- `2a78da0` (test)
2. **Task 2: Pull requests and releases** -- `dd02d89` (test)
3. **Task 3: Phase-wide gate** -- `07634cc` (test, evidence-only empty commit)

## Files Created/Modified

- `internal/ado/branches_test.go` -- Pins repository-fetch failure swallowing with a poisoned failed-project branch.
- `internal/ado/pipelines_test.go` -- Pins both build-backed reads with non-empty matching `pipelines` configuration.
- `internal/ado/pullrequests_test.go` -- Pins project pull-request failure swallowing with a non-empty surviving result.
- `internal/ado/releases_test.go` -- Pins release-definition and pending-approval failure swallowing with explicit release configuration.

## Eight-Read Mapping

| Aggregate read | Characterization test | Source swallow |
|---|---|---|
| Work items | `TestGetWorkItemsSwallowsProjectFailure` | Project-loop `continue` after propagated WIQL failure |
| Pull requests | `TestGetPullRequestsSwallowsProjectFailure` | Project-loop `continue` after `GetPullRequestsByProject` failure |
| Pipeline runs | `TestGetPipelineRunsSwallowsProjectFailure` | Project-loop `continue` after `GetDefinitions` failure |
| Pipeline definitions | `TestGetPipelineDefinitionsSwallowsProjectFailure` | Project-loop `continue` after `GetDefinitions` failure |
| Releases | `TestGetReleasesSwallowsProjectFailure` | Project-loop `continue` after `GetReleaseDefinitions` failure |
| Pending approvals | `TestGetPendingApprovalsSwallowsProjectFailure` | Project-loop `continue` after `GetApprovals` failure |
| Branches | `TestGetBranchesSwallowsProjectFailure` | Project-loop `continue` after `GetRepositories` failure |
| Dashboard counts | `TestGetDashboardCountsSwallowsProjectFailure` | In-place swallowing inside `getProjectCounts`, not an outer-loop-only edit |

The phase gate used exactly:

```sh
go test ./internal/ado/ -run SwallowsProjectFailure -v 2>/dev/null \
  | grep -c '^=== RUN   TestGet.*SwallowsProjectFailure$'
```

It returned `8`. The trailing `$` deliberately excludes `TestGetWorkItemsSwallowsProjectFailure/classification_node_failure_is_warned_and_discarded`; that trap subtest remains present.

## Mutation Evidence

Every transient source mutation below deleted the exact `continue` claimed by its test. Each fake deliberately returned poisoned non-nil data together with the error, making fallthrough observable. Each mutation produced the shown failure, was restored exactly with `git checkout -- <file>`, and was followed by a green rerun.

### 1. GetPipelineRuns project failure

```diff
diff --git a/internal/ado/pipelines.go b/internal/ado/pipelines.go
index 7534a18..1fb1953 100644
--- a/internal/ado/pipelines.go
+++ b/internal/ado/pipelines.go
@@ -111,7 +111,6 @@ func (c *Client) GetPipelineRuns(ctx context.Context, projects []string) ([]Pipe
 		})
 		if err != nil {
 			log.Error("Failed to get pipeline definitions", "project", project, "error", err)
-			continue
 		}
```

```text
=== RUN   TestGetPipelineRunsSwallowsProjectFailure
2026/09/04 17:56:51 ERROR Failed to get pipeline definitions project=project-definitions-fail error="definitions failed"
2026/09/04 17:56:51 INFO Pipeline count project=project-definitions-fail configured_pipelines=[pipeline-one] matching=1 total=1
2026/09/04 17:56:51 INFO Pipeline count project=project-definitions-succeed configured_pipelines=[pipeline-one] matching=1 total=1
2026/09/04 17:56:51 INFO Pipeline runs fetched total_count=2 configured_pipelines=[pipeline-one]
    pipelines_test.go:83: GetPipelineRuns() returned 2 runs, want 1 surviving run: [{ID:0 BuildNumber: PipelineName:pipeline-one PipelineID:17 Project:project-definitions-fail Status:neverRun Result: SourceBranch: SourceVersion: RequestedBy: RequestedFor: StartTime:0001-01-01 00:00:00 +0000 UTC FinishTime:0001-01-01 00:00:00 +0000 UTC QueueTime:0001-01-01 00:00:00 +0000 UTC Duration:0s RepositoryID:} {ID:0 BuildNumber: PipelineName:pipeline-one PipelineID:17 Project:project-definitions-succeed Status:neverRun Result: SourceBranch: SourceVersion: RequestedBy: RequestedFor: StartTime:0001-01-01 00:00:00 +0000 UTC FinishTime:0001-01-01 00:00:00 +0000 UTC QueueTime:0001-01-01 00:00:00 +0000 UTC Duration:0s RepositoryID:}]
--- FAIL: TestGetPipelineRunsSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.496s
FAIL
```

### 2. GetPipelineDefinitions project failure

```diff
diff --git a/internal/ado/pipelines.go b/internal/ado/pipelines.go
index 7534a18..2106f16 100644
--- a/internal/ado/pipelines.go
+++ b/internal/ado/pipelines.go
@@ -250,7 +250,6 @@ func (c *Client) GetPipelineDefinitions(ctx context.Context, projects []string)
 		})
 		if err != nil {
 			log.Error("Failed to get pipeline definitions", "project", project, "error", err)
-			continue
 		}
```

```text
=== RUN   TestGetPipelineDefinitionsSwallowsProjectFailure
2026/09/04 17:57:22 ERROR Failed to get pipeline definitions project=project-definitions-fail error="definitions failed"
2026/09/04 17:57:22 INFO Pipeline definitions fetched total_count=2
    pipelines_test.go:126: GetPipelineDefinitions() returned 2 definitions, want 1 surviving definition: [{ID:23 Name:pipeline-one Path: Project:project-definitions-fail RepositoryID: DefaultBranch: QueueStatus:} {ID:23 Name:pipeline-one Path: Project:project-definitions-succeed RepositoryID: DefaultBranch: QueueStatus:}]
--- FAIL: TestGetPipelineDefinitionsSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.354s
FAIL
```

### 3. GetBranches project failure

```diff
diff --git a/internal/ado/branches.go b/internal/ado/branches.go
index c38507d..4dad923 100644
--- a/internal/ado/branches.go
+++ b/internal/ado/branches.go
@@ -48,7 +48,6 @@ func (c *Client) GetBranches(ctx context.Context, projects, repositories []strin
 		repos, err := c.gitClient.GetRepositories(ctx, git.GetRepositoriesArgs{Project: &project})
 		if err != nil {
 			log.Error("Failed to get repositories", "project", project, "error", err)
-			continue
 		}
```

```text
=== RUN   TestGetBranchesSwallowsProjectFailure
2026/09/04 17:57:43 ERROR Failed to get repositories project=project-repositories-fail error="repositories failed"
2026/09/04 17:57:43 INFO Branches fetched total_count=2
    branches_test.go:54: GetBranches() returned 2 branches, want 1 surviving branch: [{Name:poisoned-failed-project-branch Project:project-repositories-fail RepositoryID:01000000-0000-0000-0000-000000000000 RepositoryName:repository-one DefaultBranch:main IsDefault:false AheadCount:0 BehindCount:0 LastCommitDate:0001-01-01 00:00:00 +0000 UTC LastCommitBy: CommitID:} {Name:main Project:project-repositories-succeed RepositoryID:01000000-0000-0000-0000-000000000000 RepositoryName:repository-one DefaultBranch:main IsDefault:true AheadCount:0 BehindCount:0 LastCommitDate:0001-01-01 00:00:00 +0000 UTC LastCommitBy: CommitID:}]
--- FAIL: TestGetBranchesSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.399s
FAIL
```

### 4. GetPullRequests project failure

```diff
diff --git a/internal/ado/pullrequests.go b/internal/ado/pullrequests.go
index 3c81ad3..28cb31d 100644
--- a/internal/ado/pullrequests.go
+++ b/internal/ado/pullrequests.go
@@ -50,7 +50,6 @@ func (c *Client) GetPullRequests(ctx context.Context, projects []string) ([]Pull
 		})
 		if err != nil {
 			log.Error("Failed to get pull requests", "project", project, "error", err)
-			continue
 		}
```

```text
=== RUN   TestGetPullRequestsSwallowsProjectFailure
2026/09/04 18:00:45 ERROR Failed to get pull requests project=project-pull-requests-fail error="pull requests failed"
2026/09/04 18:00:45 INFO Pull requests fetched total_count=2
    pullrequests_test.go:169: GetPullRequests() returned 2 pull requests, want 1 surviving pull request: [{ID:42 Title:poisoned failed-project pull request Description: Project:project-pull-requests-fail RepositoryID: RepositoryName: SourceBranch: TargetBranch: CreatedBy: CreatedByID: CreationDate:0001-01-01 00:00:00 +0000 UTC Status: IsDraft:false ReviewerCount:0 ApprovalStatus:} {ID:42 Title:surviving pull request Description: Project:project-pull-requests-succeed RepositoryID: RepositoryName: SourceBranch: TargetBranch: CreatedBy: CreatedByID: CreationDate:0001-01-01 00:00:00 +0000 UTC Status: IsDraft:false ReviewerCount:0 ApprovalStatus:}]
--- FAIL: TestGetPullRequestsSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.448s
FAIL
```

### 5. GetReleases project failure

```diff
diff --git a/internal/ado/releases.go b/internal/ado/releases.go
index f5d4230..d10c1cd 100644
--- a/internal/ado/releases.go
+++ b/internal/ado/releases.go
@@ -70,7 +70,6 @@ func (c *Client) GetReleases(ctx context.Context, projects []string) ([]Release,
 		})
 		if err != nil {
 			log.Error("Failed to get release definitions", "project", project, "error", err)
-			continue
 		}
```

```text
=== RUN   TestGetReleasesSwallowsProjectFailure
2026/09/04 18:01:13 ERROR Failed to get release definitions project=project-release-definitions-fail error="release definitions failed"
2026/09/04 18:01:13 INFO Release definition count project=project-release-definitions-fail configured_definitions=[release-definition-one] matching=1 total=1
2026/09/04 18:01:13 INFO Release definition count project=project-release-definitions-succeed configured_definitions=[release-definition-one] matching=1 total=1
2026/09/04 18:01:13 INFO Releases fetched total_count=2 configured_definitions=[release-definition-one]
    releases_test.go:141: GetReleases() returned 2 releases, want 1 surviving release: [{ID:0 Name: DefinitionID:31 DefinitionName:release-definition-one Project:project-release-definitions-fail Status:noReleases CreatedOn:0001-01-01 00:00:00 +0000 UTC ModifiedOn:0001-01-01 00:00:00 +0000 UTC Environments:[] PendingApprovals:0} {ID:0 Name: DefinitionID:31 DefinitionName:release-definition-one Project:project-release-definitions-succeed Status:noReleases CreatedOn:0001-01-01 00:00:00 +0000 UTC ModifiedOn:0001-01-01 00:00:00 +0000 UTC Environments:[] PendingApprovals:0}]
--- FAIL: TestGetReleasesSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.391s
FAIL
```

### 6. GetPendingApprovals project failure

```diff
diff --git a/internal/ado/releases.go b/internal/ado/releases.go
index f5d4230..6b1dd9e 100644
--- a/internal/ado/releases.go
+++ b/internal/ado/releases.go
@@ -217,7 +217,6 @@ func (c *Client) GetPendingApprovals(ctx context.Context, projects []string) ([]
 		})
 		if err != nil {
 			log.Error("Failed to get approvals", "project", project, "error", err)
-			continue
 		}
```

```text
=== RUN   TestGetPendingApprovalsSwallowsProjectFailure
2026/09/04 18:01:36 ERROR Failed to get approvals project=project-approvals-fail error="approvals failed"
2026/09/04 18:01:36 INFO Pending approvals fetched total_count=2
    releases_test.go:177: GetPendingApprovals() returned 2 approvals, want 1 surviving approval: [{ID:47 ReleaseID:0 ReleaseName: EnvironmentID:0 EnvironmentName: Project:project-approvals-fail Status: ApprovalType: Approver: ApproverID: CreatedOn:0001-01-01 00:00:00 +0000 UTC Comments:} {ID:47 ReleaseID:0 ReleaseName: EnvironmentID:0 EnvironmentName: Project:project-approvals-succeed Status: ApprovalType: Approver: ApproverID: CreatedOn:0001-01-01 00:00:00 +0000 UTC Comments:}]
--- FAIL: TestGetPendingApprovalsSwallowsProjectFailure (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.404s
FAIL
```

## Dashboard Inversion Warning

`GetDashboardCounts` remains structurally different from the other seven reads. Its production helper `getProjectCounts` swallows errors per call and always returns a nil error, so its outer-loop `continue` is unreachable. Phase 2 must make `getProjectCounts` propagate its internal failures; an outer-loop-only edit or a search limited to the seven reachable `continue` statements will silently miss the dashboard.

## Unresolved Probe Assumptions

These are flagged assumptions, not verified criteria. `/gsd-verify-work` must evaluate all three.

| Requirement | Item | Assumption made | Why it is not verified |
|---|---|---|---|
| SEAM-01 | unclassified | "Narrow" is operationalized as: exactly the 20 methods in the verified call-site inventory, no more. | The requirement gives no independent definition of narrow. The count is a proxy for intent, not a proof of it. |
| SEAM-02 | unclassified | "Accepts injected sub-clients" is satisfied by assignable unexported struct fields rather than a constructor parameter (D-10). | This is an interpretation of requirement wording. The compiler cannot check intent, and REQUIREMENTS.md is deliberately unamended so the interpretation stays visible. |
| SEAM-04 | unclassified | "The failure paths from TRUST-01" is satisfied by characterization tests pinning the pre-TRUST-01 behavior, since those paths do not exist until Phase 2 (D-14). | The requirement names paths that do not yet exist. Whether pinning the inverse counts as coverage is a judgment the user accepted in discussion. |

## Verification

- `gofmt -l internal/` -- passed with no output.
- `go build ./...` -- passed.
- `go vet ./...` -- passed.
- Anchored top-level gate -- passed with exactly eight matches for `^=== RUN   TestGet.*SwallowsProjectFailure$`.
- `go test ./... -race -cover` -- passed; `internal/ado` reports 57.2% coverage.
- `go test ./internal/ado/ -list '^TestGet.*SwallowsProjectFailure$'` -- listed the eight mapped tests exactly once.
- Production restoration -- `git status --porcelain internal/ado/` passed with no output after all task commits.
- Phase scope -- no file under `internal/ui/` or `cmd/` changed from the Phase 1 base, and neither `go.mod` nor `go.sum` changed.

## Decisions Made

- Returned poisoned data with errors in each fake. This is deliberate: deleting `continue` must expose failed-project output and make each mutation proof non-vacuous.
- Configured both pipeline client literals with `[]string{"pipeline-one"}` because both production methods otherwise return before calling the fake.
- Preserved the work-item trap subtest and excluded it only through the anchored top-level count.
- Kept the dashboard warning explicit because its inversion cannot be implemented by changing only the seven project-loop `continue` statements.

## Deviations from Plan

None -- plan executed exactly as written.

## Issues Encountered

- Context7 was unavailable, so version-specific Azure DevOps SDK response and fixture types were checked with `go doc` against the pinned local `v1.0.0-b5` module.
- The shell's `status` variable is read-only under zsh; the first pipeline-runs mutation command still produced the required failing output, and source restoration plus green rerun completed normally using a different variable name thereafter.

## Known Stubs

None.

## Threat Surface

No production endpoint, authentication path, file access, schema, or network surface was added. Tests use fixture-only project names, package-local fakes, and no credentials or organization URLs. Every transient production mutation was restored before task commits.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- Phase 1's eight aggregate reads are each pinned by a greppable characterization for Phase 2's TRUST-01 inversion.
- The dashboard's distinct propagation requirement and all three unresolved probe assumptions are explicitly carried forward.
- Phase 1 implementation is complete and ready for end-of-phase verification.

## Self-Check: PASSED

- Created `internal/ado/branches_test.go`; modified `internal/ado/pipelines_test.go`, `internal/ado/pullrequests_test.go`, and `internal/ado/releases_test.go`; all exist.
- Task commits `2a78da0`, `dd02d89`, and `07634cc` exist.
- Every recorded mutation failed its target test, was restored with `git checkout --`, and was followed by a green rerun.
- All task acceptance criteria and plan-level formatting, build, vet, anchored-count, race, coverage, scope, and restoration checks passed.
- No stubs, skipped tests, unrun verification, or unexpected source changes remain.

---
*Phase: 01-test-seam-for-internal-ado*
*Completed: 2026-09-04*
