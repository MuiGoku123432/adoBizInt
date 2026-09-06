---
phase: 01-test-seam-for-internal-ado
plan: 06
subsystem: testing
tags: [go, azure-devops, identity, mutation-testing, exact-call-contracts]

requires:
  - phase: 01-test-seam-for-internal-ado/01-02
    provides: Narrow ADO interfaces, successful eager identity caching, and package-local fake seams
  - phase: 01-test-seam-for-internal-ado/01-03
    provides: Exact write-call tests including the ordered pull request completion flow
  - phase: 01-test-seam-for-internal-ado/01-05
    provides: Eight aggregate-read characterizations and anchored phase gate
provides:
  - Cache-first pull request approval with nil-safe, per-attempt identity lookup recovery
  - AuthenticatedUser.Id-only reviewer selection with exact 1/0, 2/1, and 3/2 provider/Git counts
  - Whole-value GetPullRequestArgs assertion for the fetch half of pull request completion
  - Mutation evidence proving the fetch assertion detects a wrong project argument
  - Green build, vet, race, coverage, and eight-read regression gates
affects: [phase-2-honest-reads, phase-4-guarded-writes, SEAM-02, SEAM-03]

actuals:
  tokens: 2680
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - Package-private exact-signature function provider retained from an SDK method value
    - Cache-first identity lookup with uncached per-attempt fallback
    - Complete SDK argument structs compared with reflect.DeepEqual
    - Trap-backed semantic source mutation restored from the Task 1 commit

key-files:
  created: []
  modified:
    - internal/ado/client.go
    - internal/ado/pullrequests.go
    - internal/ado/pullrequests_test.go

key-decisions:
  - "Keep successful construction-time identity caching, but retain locationClient.GetConnectionData as a package-private provider for empty-cache approval retries."
  - "Do not memoize fallback identity results; every approval from an initially empty cache performs a fresh lookup."
  - "Use only AuthenticatedUser.Id for reviewer writes; AuthorizedUser remains display-only."
  - "Assert the complete initial GetPullRequestArgs value and prove non-vacuity by mutating only its Project identifier."

patterns-established:
  - "Identity recovery: cache hits bypass the provider; empty-cache attempts retry independently and cannot reach Git without an authenticated ID."
  - "Multi-call write contract: assert every call's whole SDK argument plus call order, then mutation-test the previously unasserted call."

requirements-completed: [SEAM-02, SEAM-03]

coverage:
  - id: D1
    description: "Pull request approval preserves cache-hit behavior and safely retries identity lookup on every empty-cache attempt."
    requirement: SEAM-02
    verification:
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestApprovePullRequestIssuesExactReviewerCall"
        status: pass
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestApprovePullRequestRetriesIdentityLookupAfterTransientFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit"
        status: pass
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestApprovePullRequestWithoutAuthenticatedUserIDDoesNotCallGit"
        status: pass
    human_judgment: false
  - id: D2
    description: "NewClient retains the location provider method value, performs eager lookup through it, and preserves non-fatal construction semantics."
    requirement: SEAM-02
    verification:
      - kind: other
        ref: "01-06 Task 1 structural NewClient identity-provider wiring gate"
        status: pass
      - kind: integration
        ref: "go build ./... && go vet ./..."
        status: pass
    human_judgment: false
  - id: D3
    description: "CompletePullRequest's fetch and update requests are whole-value asserted in get-then-update order."
    requirement: SEAM-03
    verification:
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestCompletePullRequestIssuesFetchThenMerge"
        status: pass
      - kind: other
        ref: "semantic mutation: fetch Project &project -> &repositoryID"
        status: pass
    human_judgment: false
  - id: D4
    description: "The repository retains all eight aggregate characterizations and passes formatting, build, vet, race, and coverage gates."
    requirement: SEAM-03
    verification:
      - kind: integration
        ref: "go test ./internal/ado/ -count=1 -run '^TestGet.*SwallowsProjectFailure$' -v (8 top-level tests)"
        status: pass
      - kind: integration
        ref: "go build ./... && go vet ./... && go test ./... -count=1 -race -cover"
        status: pass
    human_judgment: false

duration: 9 min
completed: 2026-09-06
status: complete
---

# Phase 1 Plan 6: Approval Identity Recovery and Exact PR Fetch Summary

**Cache-first approval now recovers from transient identity lookup failures, while the PR completion test mutation-proves the complete initial fetch contract.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-09-06T05:32:22Z
- **Completed:** 2026-09-06T05:41:57Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Added a package-private `getConnectionData` provider wired from the same location client used for eager construction-time identity lookup.
- Preserved cache-hit bypass and nil-provider safety while proving empty-cache attempts call the provider and Git in exact sequences `1/0`, `2/1`, and `3/2`.
- Locked reviewer selection to `AuthenticatedUser.Id`; an ID-less response returns the exact missing-ID error before any Git call.
- Captured and deep-compared the complete initial `git.GetPullRequestArgs` while retaining ordered `get`, `update` behavior and the existing exact update assertion.
- Mutation-proved that changing only the fetch `Project` field makes the targeted test fail, then restored from the Task 1 commit and reran green.

## Task Commits

Each task was committed atomically through the required TDD sequence for Task 1 and a separate Task 2 commit:

1. **Task 1 RED: Add failing approval identity tests** -- `9542a8e` (test)
2. **Task 1 GREEN: Restore approval identity retries** -- `b63289a` (feat)
3. **Task 2: Assert pull request fetch arguments** -- `56ea96d` (test)

## Files Created/Modified

- `internal/ado/client.go` -- Adds and wires the exact-signature package-private identity provider while preserving eager caching and non-fatal startup.
- `internal/ado/pullrequests.go` -- Adds cache-first, nil-safe, per-attempt identity fallback using only `AuthenticatedUser.Id`.
- `internal/ado/pullrequests_test.go` -- Adds approval recovery and guard tests, plus the complete fetch argument assertion.

## Mutation Evidence

The semantic selector scoped itself to `CompletePullRequest`, found one typed fetch literal and one typed update literal, verified both original `Project` fields referenced `&project`, and changed only the fetch literal's identifier:

```diff
diff --git a/internal/ado/pullrequests.go b/internal/ado/pullrequests.go
index 953c9ee..4172b19 100644
--- a/internal/ado/pullrequests.go
+++ b/internal/ado/pullrequests.go
@@ -195,7 +195,7 @@ func (c *Client) CompletePullRequest(ctx context.Context, project, repositoryID
 	pr, err := c.gitClient.GetPullRequest(ctx, git.GetPullRequestArgs{
 		RepositoryId:  &repositoryID,
 		PullRequestId: &prID,
-		Project:       &project,
+		Project:       &repositoryID,
 	})
 	if err != nil {
 		return fmt.Errorf("failed to get PR details: %w", err)
```

Observed targeted RED output:

```text
=== RUN   TestCompletePullRequestIssuesFetchThenMerge
2026/09/06 00:39:07 INFO Pull request completed prID=42
    pullrequests_test.go:207: GetPullRequest args = {RepositoryId:0x1f50deea7c0 PullRequestId:0x1f50decead8 Project:0x1f50deea7c0 MaxCommentLength:<nil> Skip:<nil> Top:<nil> IncludeCommits:<nil> IncludeWorkItemRefs:<nil>}, want {RepositoryId:0x1f50deea820 PullRequestId:0x1f50deceb28 Project:0x1f50deea810 MaxCommentLength:<nil> Skip:<nil> Top:<nil> IncludeCommits:<nil> IncludeWorkItemRefs:<nil>}
--- FAIL: TestCompletePullRequestIssuesFetchThenMerge (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.364s
FAIL
```

After cleanup restored `internal/ado/pullrequests.go` from Task 1 commit `b63289a`, the source diff was clean and the targeted test returned green:

```text
=== RUN   TestCompletePullRequestIssuesFetchThenMerge
2026/09/06 00:39:07 INFO Pull request completed prID=42
--- PASS: TestCompletePullRequestIssuesFetchThenMerge (0.00s)
PASS
ok  	sentinovo.ai/bizInt/internal/ado	0.170s
```

## Verification

- Exact four approval test declarations, RUN records, and PASS records -- passed with ADO credential variables removed.
- `NewClient` structural gate -- passed for exact provider field/local signatures, method-value assignment, eager provider invocation, no receiver-call bypass, return wiring, and ordering.
- `TestCompletePullRequestIssuesFetchThenMerge` structural and behavioral gates -- passed for full fetch capture/literal comparison, order, and full update comparison.
- Trap-backed semantic mutation -- produced the required `GetPullRequest args` failure, restored from `b63289a`, left a clean production diff, and reran green.
- Aggregate gate -- all eight top-level `TestGet*SwallowsProjectFailure` tests passed; exact file mapping and adjacent `TRUST-01`/`Phase 2` headers passed.
- Protected artifacts -- `internal/ado/interfaces.go`, `internal/ado/fakes_test.go`, `go.mod`, and `go.sum` remain identical to `7eb081a`.
- Credential/logging gate -- no new PAT, authorization string, or connection-data logging was introduced.
- `go build ./...` -- passed.
- `go vet ./...` -- passed.
- `go test ./... -count=1 -race -cover` -- passed; `internal/ado` coverage is 57.4%.

## Decisions Made

- Retain successful eager caching and use the same location client's method value only when the cache is empty.
- Keep fallback success local to one approval attempt rather than mutating shared client state; this preserves deterministic per-attempt retries.
- Treat nil provider, provider error, and ID-less provider response as pre-Git failure paths.
- Preserve full-struct assertions for both calls in the fetch-then-merge sequence rather than checking selected fields.

## Deviations from Plan

None -- plan executed exactly as written.

## Issues Encountered

- The first local mutation-run wrapper used an incorrect full Task 1 SHA and exited before mutation. No source was changed. The run was repeated with the verified commit SHA and completed the required trap-backed red/restore/green proof.
- GSD state helpers calculate completion from on-disk summaries, so final phase state and roadmap completion were finalized together with this summary.

## Known Stubs

None.

## Threat Surface

No unplanned security-relevant surface was introduced. The planned identity trust boundary now rejects nil and ID-less provider results before Git, uses only `AuthenticatedUser.Id`, logs no connection data or credentials, and adds no public API or network endpoint.

## Explicit Non-Claims

- `NewClient` identity failure was not directly unit-tested because the constructor still creates live SDK clients; its exact provider wiring was verified structurally and through build/vet.
- The unrelated, under-specified aggregate-ordering backstop remains outside this surgical gap closure. This plan only re-established the eight existing aggregate tests and their source/header mapping.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- Phase 1's two verification gaps are closed with executable evidence, and all six plans are complete.
- Phase 2 can invert the eight explicitly mapped aggregate failure characterizations without revisiting the test seam.
- No transient mutation, dependency change, credential, or unrelated source change remains.

## Self-Check: PASSED

- Modified files exist: `internal/ado/client.go`, `internal/ado/pullrequests.go`, and `internal/ado/pullrequests_test.go`.
- Task commits exist: `9542a8e`, `b63289a`, and `56ea96d`.
- Every task acceptance criterion and plan-level formatting, targeted, build, vet, race, coverage, aggregate, structural, and restoration gate passed.
- `internal/ado/pullrequests.go` is clean against Task 1 commit `b63289a`; the deliberate fetch mutation was never committed.
- Only preserved orchestration artifacts remain outside the plan's committed source changes.

---
*Phase: 01-test-seam-for-internal-ado*
*Completed: 2026-09-06*
