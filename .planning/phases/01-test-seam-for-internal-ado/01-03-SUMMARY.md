---
phase: 01-test-seam-for-internal-ado
plan: 03
subsystem: testing
tags: [go, azure-devops, mutation-testing, destructive-writes, race-detector]

requires:
  - phase: 01-test-seam-for-internal-ado/01-01
    provides: Work-item API seam, tracer write test, and mutation-check protocol
  - phase: 01-test-seam-for-internal-ado/01-02
    provides: Git, build, and release interfaces plus fail-fast fakes and cached AuthenticatedUser.Id
provides:
  - Exact SDK argument tests for all seven destructive ADO write methods
  - Three-case GetNextState transition coverage
  - Release approve/reject copy-paste cross-check
  - Mutation evidence for every new write assertion and error-branch guard
affects: [01-04, 01-05, phase-4-guarded-writes]

actuals:
  tokens: 2932
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - Full literal SDK argument structs compared with reflect.DeepEqual
    - Fail-by-name fakes proving destructive calls do not occur on guard branches
    - Source mutation followed by git checkout restoration and green rerun

key-files:
  created:
    - internal/ado/pullrequests_test.go
    - internal/ado/releases_test.go
    - internal/ado/pipelines_test.go
  modified:
    - internal/ado/workitems_test.go

key-decisions:
  - "Represent expected effort patch values as float64 literals so interface{} dynamic types match production values."
  - "Use cached Client.currentUserID as the approval reviewer ID, matching AuthenticatedUser.Id from plan 01-02."
  - "Keep a dedicated release status cross-check in addition to the two exact-call tests so aligned test-oracle drift cannot hide a copy-paste regression."

patterns-established:
  - "Destructive-write contract: capture exactly one SDK argument literal per write and compare the entire value."
  - "Guard proof: leave forbidden fake methods unstubbed so any accidental call fails with its method name."

requirements-completed: [SEAM-03]

coverage:
  - id: D1
    description: "Both work-item writes and GetNextState are pinned, including all four ordered effort paths and three transition outcomes."
    requirement: SEAM-03
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortIssuesExactPatch"
        status: pass
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetNextState"
        status: pass
      - kind: other
        ref: "path, order, and configured-transition mutation checks"
        status: pass
    human_judgment: false
  - id: D2
    description: "Pull request approval and completion issue exact calls, use cached authenticated identity, preserve call order, and stop before writes on invalid inputs."
    requirement: SEAM-03
    verification:
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestApprovePullRequestIssuesExactReviewerCall"
        status: pass
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestCompletePullRequestIssuesFetchThenMerge"
        status: pass
      - kind: other
        ref: "reviewer, merge strategy, empty-ID, and nil-merge mutation checks"
        status: pass
    human_judgment: false
  - id: D3
    description: "Release approve/reject and pipeline queue arguments are exact, with a dedicated release status divergence guard."
    requirement: SEAM-03
    verification:
      - kind: unit
        ref: "internal/ado/releases_test.go#TestApproveAndRejectReleaseSendDifferentStatuses"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestQueuePipelineIssuesExactQueueCall"
        status: pass
      - kind: other
        ref: "release copy-paste and pipeline project mutation checks"
        status: pass
    human_judgment: false
  - id: D4
    description: "The complete repository builds, vets, and passes the race-enabled test suite with no production mutation left behind."
    requirement: SEAM-03
    verification:
      - kind: integration
        ref: "gofmt -l internal/ && go build ./... && go vet ./... && go test ./... -race -cover"
        status: pass
    human_judgment: false

duration: 15 min
completed: 2026-09-04
status: complete
---

# Phase 1 Plan 3: Destructive ADO Write Tests Summary

**Exact SDK payload tests now cover all seven destructive ADO writes, GetNextState, guard branches, and release status divergence without network access.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-09-04T22:10:10Z
- **Completed:** 2026-09-04T22:24:55Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Added exact `reflect.DeepEqual` assertions for the remaining six destructive writes, completing coverage of all seven ADO write methods when combined with plan 01-01's tracer.
- Pinned the effort update's four literal ADO paths, `float64` values, and load-bearing operation order; added configured, unknown-type, and unmapped-state coverage for `GetNextState`.
- Verified pull request approval uses cached `AuthenticatedUser.Id`, and proved empty identity plus nil merge commit return errors before any forbidden SDK write.
- Added release approve/reject exact-call tests, a dedicated status-divergence cross-check, and an exact pipeline queue request test.
- Raised `internal/ado` coverage from 2.1% to 17.9%; build, vet, race, and full tests pass.

## Task Commits

Each task was committed atomically:

1. **Task 1: Work item writes -- the four-path effort patch and GetNextState** -- `97a128b` (test)
2. **Task 2: Pull request writes -- approve, complete, and the empty-identity guard** -- `abd6411` (test)
3. **Task 3: Release approve/reject with a swap cross-check, and the pipeline queue** -- `183fab1` (test)

## Files Created/Modified

- `internal/ado/workitems_test.go` -- Adds the ordered four-operation effort patch assertion and three-case `GetNextState` table.
- `internal/ado/pullrequests_test.go` -- Covers exact approval and completion calls plus empty-identity and nil-merge guards.
- `internal/ado/releases_test.go` -- Covers exact approve/reject calls and the status-divergence cross-check.
- `internal/ado/pipelines_test.go` -- Covers the exact pipeline queue payload, including normalized branch and project.

## Mutation Evidence

Every production mutation below was captured while present, made the claimed test fail, was reverted with `git checkout -- <file>`, and was followed by a green rerun. Final production-source status was clean.

### 1. Effort field path literal

```diff
diff --git a/internal/ado/workitems.go b/internal/ado/workitems.go
index 185a94d..20b6b03 100644
--- a/internal/ado/workitems.go
+++ b/internal/ado/workitems.go
@@ -246,7 +246,7 @@ func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, itemID int,
 
 	addOp := webapi.OperationValues.Add
 	statePath := "/fields/System.State"
-	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
+	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate.MUTATED"
 	remainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
 	completedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"
```

```text
=== RUN   TestUpdateWorkItemStateWithEffortIssuesExactPatch
2026/09/04 17:13:19 INFO Work item state updated with effort id=84 state=Closed originalEstimate=8 remaining=3.5 completed=4.5
    workitems_test.go:96: UpdateWorkItem args = {Document:0x2caca2a862a0 Id:0x2caca2a88b38 Project:<nil> ValidateOnly:<nil> BypassRules:<nil> SuppressNotifications:<nil> Expand:<nil>}, want {Document:0x2caca2a86300 Id:0x2caca2a88bb8 Project:<nil> ValidateOnly:<nil> BypassRules:<nil> SuppressNotifications:<nil> Expand:<nil>}
--- FAIL: TestUpdateWorkItemStateWithEffortIssuesExactPatch (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.351s
FAIL
```

### 2. Effort patch operation order

```diff
diff --git a/internal/ado/workitems.go b/internal/ado/workitems.go
index 185a94d..a90e64c 100644
--- a/internal/ado/workitems.go
+++ b/internal/ado/workitems.go
@@ -255,13 +255,13 @@ func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, itemID int,
 	document := []webapi.JsonPatchOperation{
 		{
 			Op:    &addOp,
-			Path:  &originalEstimatePath,
-			Value: originalEstimate,
+			Path:  &remainingPath,
+			Value: remaining,
 		},
 		{
 			Op:    &addOp,
-			Path:  &remainingPath,
-			Value: remaining,
+			Path:  &originalEstimatePath,
+			Value: originalEstimate,
 		},
```

```text
=== RUN   TestUpdateWorkItemStateWithEffortIssuesExactPatch
2026/09/04 17:13:43 INFO Work item state updated with effort id=84 state=Closed originalEstimate=8 remaining=3.5 completed=4.5
    workitems_test.go:96: UpdateWorkItem args = {Document:0x4893df51e288 Id:0x4893df518a88 Project:<nil> ValidateOnly:<nil> BypassRules:<nil> SuppressNotifications:<nil> Expand:<nil>}, want {Document:0x4893df51e2e8 Id:0x4893df518b08 Project:<nil> ValidateOnly:<nil> BypassRules:<nil> SuppressNotifications:<nil> Expand:<nil>}
--- FAIL: TestUpdateWorkItemStateWithEffortIssuesExactPatch (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.379s
FAIL
```

### 3. GetNextState configured target

```diff
diff --git a/internal/ado/workitems.go b/internal/ado/workitems.go
index 185a94d..ed66088 100644
--- a/internal/ado/workitems.go
+++ b/internal/ado/workitems.go
@@ -509,8 +509,8 @@ func (c *Client) GetChildWorkItems(ctx context.Context, parentID int) ([]WorkIte
 // GetNextState returns the next state for a work item type/current state based on config
 func GetNextState(config map[string]map[string]string, workItemType, currentState string) (string, bool) {
 	if typeMap, ok := config[workItemType]; ok {
-		if nextState, ok := typeMap[currentState]; ok {
-			return nextState, true
+		if _, ok := typeMap[currentState]; ok {
+			return "MUTATED", true
 		}
 	}
 	return "", false
```

```text
=== RUN   TestGetNextState
=== RUN   TestGetNextState/configured_transition
    workitems_test.go:141: GetNextState() = ("MUTATED", true), want ("Closed", true)
=== RUN   TestGetNextState/unknown_work_item_type
=== RUN   TestGetNextState/unmapped_current_state
--- FAIL: TestGetNextState (0.00s)
    --- FAIL: TestGetNextState/configured_transition (0.00s)
    --- PASS: TestGetNextState/unknown_work_item_type (0.00s)
    --- PASS: TestGetNextState/unmapped_current_state (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.348s
FAIL
```

### 4. Pull request reviewer identity

```diff
diff --git a/internal/ado/pullrequests.go b/internal/ado/pullrequests.go
index 3c81ad3..1961700 100644
--- a/internal/ado/pullrequests.go
+++ b/internal/ado/pullrequests.go
@@ -163,7 +163,7 @@ func (c *Client) ApprovePullRequest(ctx context.Context, project, repositoryID s
 		Reviewer:      reviewer,
 		RepositoryId:  &repositoryID,
 		PullRequestId: &prID,
-		ReviewerId:    &userID,
+		ReviewerId:    &repositoryID,
 		Project:       &project,
 	})
```

```text
=== RUN   TestApprovePullRequestIssuesExactReviewerCall
2026/09/04 17:15:02 INFO Pull request approved prID=42
    pullrequests_test.go:44: CreatePullRequestReviewer args = {Reviewer:0x187d2b2521b0 RepositoryId:0x187d2b2386a0 PullRequestId:0x187d2b228a28 ReviewerId:0x187d2b2386a0 Project:0x187d2b238690}, want {Reviewer:0x187d2b2522d0 RepositoryId:0x187d2b2386c0 PullRequestId:0x187d2b228a98 ReviewerId:0x187d2b2386d0 Project:0x187d2b2386b0}
--- FAIL: TestApprovePullRequestIssuesExactReviewerCall (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.347s
FAIL
```

### 5. Pull request empty-identity guard

```diff
diff --git a/internal/ado/pullrequests.go b/internal/ado/pullrequests.go
index 3c81ad3..303827d 100644
--- a/internal/ado/pullrequests.go
+++ b/internal/ado/pullrequests.go
@@ -150,7 +150,7 @@ func (c *Client) ApprovePullRequest(ctx context.Context, project, repositoryID s
 	log := logging.Logger()
 
 	userID := c.currentUserID
-	if userID == "" {
+	if userID == "MUTATED" {
 		return fmt.Errorf("could not determine current user ID")
 	}
```

```text
=== RUN   TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit
    pullrequests.go:162: unexpected call to gitAPI.CreatePullRequestReviewer
--- FAIL: TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.447s
FAIL
```

### 6. Pull request completion merge argument

```diff
diff --git a/internal/ado/pullrequests.go b/internal/ado/pullrequests.go
index 3c81ad3..fd33e92 100644
--- a/internal/ado/pullrequests.go
+++ b/internal/ado/pullrequests.go
@@ -200,7 +200,7 @@ func (c *Client) CompletePullRequest(ctx context.Context, project, repositoryID
 
 	// Set up completion options
 	deleteSourceBranch := false // Keep source branch as per requirements
-	mergeStrategy := git.GitPullRequestMergeStrategyValues.NoFastForward
+	mergeStrategy := git.GitPullRequestMergeStrategyValues.Squash
 	completedStatus := git.PullRequestStatusValues.Completed
```

```text
=== RUN   TestCompletePullRequestIssuesFetchThenMerge
2026/09/04 17:15:47 INFO Pull request completed prID=42
    pullrequests_test.go:114: UpdatePullRequest args = {GitPullRequestToUpdate:0x6f83d097a480 RepositoryId:0x6f83d091a740 PullRequestId:0x6f83d0910a28 Project:0x6f83d091a730}, want {GitPullRequestToUpdate:0x6f83d097a6c0 RepositoryId:0x6f83d091a7a0 PullRequestId:0x6f83d0910a90 Project:0x6f83d091a790}
--- FAIL: TestCompletePullRequestIssuesFetchThenMerge (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.371s
FAIL
```

### 7. Pull request nil-merge guard

```diff
diff --git a/internal/ado/pullrequests.go b/internal/ado/pullrequests.go
index 3c81ad3..ad14b8b 100644
--- a/internal/ado/pullrequests.go
+++ b/internal/ado/pullrequests.go
@@ -195,7 +195,7 @@ func (c *Client) CompletePullRequest(ctx context.Context, project, repositoryID
 	if pr.LastMergeSourceCommit != nil && pr.LastMergeSourceCommit.CommitId != nil {
 		lastCommit = *pr.LastMergeSourceCommit.CommitId
 	} else {
-		return fmt.Errorf("could not determine last merge source commit")
+		lastCommit = "MUTATED"
 	}
```

```text
=== RUN   TestCompletePullRequestWithNilMergeCommitReturnsError
    pullrequests.go:219: unexpected call to gitAPI.UpdatePullRequest
--- FAIL: TestCompletePullRequestWithNilMergeCommitReturnsError (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.396s
FAIL
```

### 8. Release status copy-paste regression

This mutation reproduces the dangerous aligned-oracle scenario: `ApproveRelease` is changed to send `Rejected`, and the approve test's expected literal is changed in the same way. Both individual exact-call tests passed; only the independent cross-check failed.

```diff
diff --git a/internal/ado/releases.go b/internal/ado/releases.go
index f5d4230..9d2fa3c 100644
--- a/internal/ado/releases.go
+++ b/internal/ado/releases.go
@@ -272,7 +272,7 @@ func (c *Client) GetPendingApprovals(ctx context.Context, projects []string) ([]
 func (c *Client) ApproveRelease(ctx context.Context, project string, approvalID int, comment string) error {
 	log := logging.Logger()
 
-	status := release.ApprovalStatusValues.Approved
+	status := release.ApprovalStatusValues.Rejected
 	approval := &release.ReleaseApproval{
 		Status:   &status,
 		Comments: &comment,
```

```text
=== RUN   TestApproveReleaseIssuesExactApproval
2026/09/04 17:24:37 INFO Release approved approvalID=42
--- PASS: TestApproveReleaseIssuesExactApproval (0.00s)
=== RUN   TestRejectReleaseIssuesExactApproval
2026/09/04 17:24:37 INFO Release rejected approvalID=42
--- PASS: TestRejectReleaseIssuesExactApproval (0.00s)
PASS
ok  	sentinovo.ai/bizInt/internal/ado	0.404s
```

```text
=== RUN   TestApproveAndRejectReleaseSendDifferentStatuses
2026/09/04 17:24:37 INFO Release approved approvalID=42
2026/09/04 17:24:37 INFO Release rejected approvalID=42
    releases_test.go:100: copy-paste swap guard: statuses = [rejected rejected], want two different values
--- FAIL: TestApproveAndRejectReleaseSendDifferentStatuses (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.405s
FAIL
```

### 9. Pipeline queue project argument

```diff
diff --git a/internal/ado/pipelines.go b/internal/ado/pipelines.go
index 7534a18..e2ac905 100644
--- a/internal/ado/pipelines.go
+++ b/internal/ado/pipelines.go
@@ -363,7 +363,7 @@ func (c *Client) QueuePipeline(ctx context.Context, project string, definitionID
 
 	queuedBuild, err := c.buildClient.QueueBuild(ctx, build.QueueBuildArgs{
 		Build:   &buildToQueue,
-		Project: &project,
+		Project: &sourceBranch,
 	})
```

```text
=== RUN   TestQueuePipelineIssuesExactQueueCall
2026/09/04 17:18:15 INFO Pipeline queued project=project-one definitionID=17 branch=main buildID=99
    pipelines_test.go:42: QueueBuild args = {Build:0x4d8d62276180 Project:0x4d8d621f0720 IgnoreWarnings:<nil> CheckInTicket:<nil> SourceBuildId:<nil>}, want {Build:0x4d8d62276480 Project:0x4d8d621f0750 IgnoreWarnings:<nil> CheckInTicket:<nil> SourceBuildId:<nil>}
--- FAIL: TestQueuePipelineIssuesExactQueueCall (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.399s
FAIL
```

## Verification

- `gofmt -l internal/` -- passed with no output.
- `go build ./...` -- passed.
- `go vet ./...` -- passed.
- `go test ./... -race -cover` -- passed; `internal/ado` coverage is 17.9%, up from 2.1% in plan 01-02.
- Targeted seven-write and `GetNextState` suite -- passed and named every write method plus all three transition subtests.
- New-test network scan -- zero `http.` or `NewPatConnection` matches in all four files.
- Fake integrity -- `internal/ado/fakes_test.go` still contains 20 `Fatalf` guards.
- Production restoration -- `git diff --exit-code` is clean for `workitems.go`, `pullrequests.go`, `releases.go`, and `pipelines.go`.

## Decisions Made

- Used explicit floating-point literals (`8.0`, `3.5`, `4.5`) for effort values because `JsonPatchOperation.Value` is `interface{}` and `reflect.DeepEqual` includes the dynamic type.
- Kept exact release argument tests and added the cross-check. The cross-check protects against a production and test literal being copy-pasted to the same wrong status.
- Proved guard branches through unstubbed fake methods instead of counters, preserving the fail-by-name convention from plan 01-02.

## Deviations from Plan

None -- plan executed exactly as written.

## Issues Encountered

- Context7 was unavailable, so version-specific Azure DevOps SDK structs were confirmed with `go doc` from the pinned module before writing literals.
- A literal swap of Approved and Rejected makes individual exact-call tests fail, not survive. To exercise the exact aligned-oracle risk described by the plan, the mutation changed `ApproveRelease` to `Rejected` and transiently aligned only that test oracle. Both individual tests then passed while the cross-check failed. Both source and test mutation were restored with `git checkout`.

## Known Stubs

None.

## Threat Surface

No production endpoint, auth path, file access, schema, or network surface was added. All new tests terminate at in-process fakes and contain no PAT, organization URL, HTTP client, or connection constructor.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- All seven destructive write baselines are pinned for Phase 4's dry-run, confirmation, and audit guards.
- The expanded fake suite is ready for plan 01-04's work-item read characterization tests.
- Phase 1 remains in progress; plans 01-04 and 01-05 remain.

## Self-Check: PASSED

- Created files exist: `internal/ado/pullrequests_test.go`, `internal/ado/releases_test.go`, and `internal/ado/pipelines_test.go`; modified `internal/ado/workitems_test.go` exists.
- Task commits exist: `97a128b`, `abd6411`, and `183fab1`.
- All task acceptance criteria and plan-level formatting, build, vet, race, coverage, and test commands passed.
- Every recorded production mutation was restored; `git status --porcelain internal/ado/` and production-file `git diff --exit-code` were empty.
- No stubs, skipped tests, unrun verification, or unexpected production changes remain.

---
*Phase: 01-test-seam-for-internal-ado*
*Completed: 2026-09-04*
