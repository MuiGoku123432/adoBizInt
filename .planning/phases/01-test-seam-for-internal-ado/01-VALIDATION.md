---
phase: 1
slug: test-seam-for-internal-ado
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-04
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, no new deps) |
| **Config file** | none — stdlib toolchain |
| **Quick run command** | `go test ./internal/ado/...` |
| **Full suite command** | `go build ./... && go vet ./... && go test ./... -cover` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/ado/...`
- **After every plan wave:** Run `go build ./... && go vet ./... && go test ./... -cover`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Populated by the planner/executor as tasks are authored. Each characterization
> test must be sanity-checked by temporarily mutating the behavior it claims to
> pin (RESEARCH.md Trap A/B) — a test that stays green under mutation is not
> covering anything. **The mutation target is not uniform:** for seven of the
> eight aggregate reads it is removing the per-project `continue`; for
> `GetDashboardCounts` the `continue` is unreachable and the target is instead
> making `getProjectCounts` propagate. See RESEARCH §"The Eight Aggregate Reads"
> revision-1 CORRECTION. Every mutation record must include the mutated diff hunk
> alongside the failure output — green plus clean `git status` is identical
> whether the mutation ran or not.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | SEAM-01, SEAM-02, SEAM-03 | T-01-01, T-01-02 | No PAT or connection in test files; tracer test proven non-vacuous | compile + unit | `go build ./... && go vet ./... && go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch -cover` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | SEAM-03 | T-01-02 | Mutation observed red then reverted clean; hunk recorded | mutation | `test -z "$(git status --porcelain internal/ado/workitems.go)" && go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch` | ❌ W0 | ⬜ pending |
| 1-02-01 | 02 | 2 | SEAM-01 | T-01-07 | Exactly 20 methods; no concrete-impl assertion | compile | `go build ./... && go vet ./... && gofmt -l internal/ado/` | ❌ W0 | ⬜ pending |
| 1-02-02 | 02 | 2 | SEAM-02 | T-01-04, T-01-05, T-01-06 | Authenticated-user id cached; no credential in warning log | compile + grep | `go vet ./... && test 0 -eq "$(grep -v '^[[:space:]]*//' internal/ado/pullrequests.go \| grep -c 'location\.')"` | ❌ W0 | ⬜ pending |
| 1-02-03 | 02 | 2 | SEAM-02 | T-01-07 | Fail-by-name guard on all 20 fake methods | compile | `go vet ./... && go test ./internal/ado/... -cover` | ❌ W0 | ⬜ pending |
| 1-03-01 | 03 | 3 | SEAM-03 | T-01-08, T-01-10 | Four field paths verbatim; float-typed effort values; order pinned | unit + mutation | `go test ./internal/ado/ -run 'TestUpdateWorkItem\|TestGetNextState' -v -cover` | ❌ W0 | ⬜ pending |
| 1-03-02 | 03 | 3 | SEAM-03 | T-01-08, T-01-11, T-01-12 | Empty identity issues no SDK call; fake guards intact | unit + mutation | `go test ./internal/ado/ -run 'TestApprovePullRequest\|TestCompletePullRequest' -v -cover` | ❌ W0 | ⬜ pending |
| 1-03-03 | 03 | 3 | SEAM-03 | T-01-08, T-01-09 | Status-swap mutation caught by cross-check | unit + mutation | `go test ./internal/ado/ -run 'TestApproveRelease\|TestRejectRelease\|TestApproveAndReject\|TestQueuePipeline' -v -cover` | ❌ W0 | ⬜ pending |
| 1-04-01 | 04 | 4 | SEAM-04 | T-01-14, T-01-15 | Failure injected at query call, not classification node; mutation = remove `continue` at `workitems.go:77` | unit + mutation | `go test ./internal/ado/ -run 'TestGetWorkItemsSwallows' -v -cover` | ❌ W0 | ⬜ pending |
| 1-04-02 | 04 | 4 | SEAM-04 | T-01-14, T-01-19, T-01-20 | Pins in-place per-call swallow, not project skipping; mutation = make `getProjectCounts` propagate, NOT remove the unreachable `continue` | unit + mutation | `go test ./internal/ado/ -run 'TestGetDashboardCountsSwallows' -v -cover` | ❌ W0 | ⬜ pending |
| 1-05-01 | 05 | 5 | SEAM-04 | T-01-14, T-01-16 | Pipelines slice set on BOTH pipeline literals so the fake is actually reached | unit + mutation | `go test ./internal/ado/ -run 'TestGetPipelineRunsSwallows\|TestGetPipelineDefinitionsSwallows\|TestGetBranchesSwallows' -v -cover` | ❌ W0 | ⬜ pending |
| 1-05-02 | 05 | 5 | SEAM-04 | T-01-14, T-01-17 | Release definitions slice set so filtering cannot empty the assertion | unit + mutation | `go test ./internal/ado/ -run 'TestGetPullRequestsSwallows\|TestGetReleasesSwallows\|TestGetPendingApprovalsSwallows' -v -cover` | ❌ W0 | ⬜ pending |
| 1-05-03 | 05 | 5 | SEAM-04 | T-01-18, T-01-21 | Exactly 8 swallow tests (TOP-LEVEL only — `$`-anchor excludes mandated subtests); sources reverted clean; all three probe items carried forward | gate | `test 8 -eq "$(go test ./internal/ado/ -run SwallowsProjectFailure -v 2>/dev/null \| grep -c '^=== RUN   TestGet.*SwallowsProjectFailure$')" && go test ./... -race -cover` | ❌ W0 | ⬜ pending |
| 1-06-01 | 06 | 6 | SEAM-02 | T-01-22, T-01-23, T-01-24, T-01-25, T-01-27 | Four approval tests: cache hit bypasses provider; nil provider returns exact missing-ID error without panic or Git; separate ID-less provider returns the same error without Git; three-attempt retry proves zero Git after failure, provider count 3, Git count 2, and two exact successful reviewer calls | unit + integration | `go test ./internal/ado/ -count=1 -run '^(TestApprovePullRequestIssuesExactReviewerCall\|TestApprovePullRequestRetriesIdentityLookupAfterTransientFailure\|TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit\|TestApprovePullRequestWithoutAuthenticatedUserIDDoesNotCallGit)$' -v` | ✅ existing | ⬜ pending |
| 1-06-02 | 06 | 6 | SEAM-03, SEAM-04 (D-15, D-16) | T-01-26 | Complete fetch args are a whole literal; order and exact update stay pinned; one production fetch-arg mutation is observed red and restored exactly; all eight aggregate tests remain present, correctly located/headered, and green | unit + mutation + aggregate contract gate | `Run the executable Task 2 automated command from 01-06-PLAN.md: targeted fetch test; all eight aggregate tests; exact anchored count of 8; exact static name-to-file and adjacent TRUST-01/Phase 2 header checks; build; vet; full race/coverage suite` | ✅ existing | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/ado/interfaces.go` — the four narrow unexported interfaces (blocks everything else)
- [ ] `internal/ado/fakes_test.go` — hand-written fakes implementing the narrow interfaces
- [ ] `internal/ado/workitems_test.go`, `pullrequests_test.go`, `releases_test.go`, `pipelines_test.go`, `branches_test.go`, `dashboard_test.go`

*No framework install needed — `go test` is stdlib.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| TUI runs with no behavior change | SEAM-02 | Requires live ADO credentials and a terminal | Build and launch the TUI against a real org; confirm existing flows behave identically |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
