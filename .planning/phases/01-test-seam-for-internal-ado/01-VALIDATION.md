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

> Populated by the planner/executor as tasks are authored. Each write-method
> characterization test must be sanity-checked by temporarily inverting the
> behavior it claims to pin (RESEARCH.md Trap A/B) — a test that stays green
> under mutation is not covering anything.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | SEAM-01, SEAM-02, SEAM-03 | T-01-01, T-01-02 | No PAT or connection in test files; tracer test proven non-vacuous | compile + unit | `go build ./... && go vet ./... && go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch -cover` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | SEAM-03 | T-01-02 | Mutation observed red then reverted clean | mutation | `test -z "$(git status --porcelain internal/ado/workitems.go)" && go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch` | ❌ W0 | ⬜ pending |
| 1-02-01 | 02 | 2 | SEAM-01 | T-01-07 | Exactly 20 methods; no concrete-impl assertion | compile | `go build ./... && go vet ./... && gofmt -l internal/ado/` | ❌ W0 | ⬜ pending |
| 1-02-02 | 02 | 2 | SEAM-02 | T-01-04, T-01-05, T-01-06 | Authenticated-user id cached; no credential in warning log | compile + grep | `go vet ./... && test 0 -eq "$(grep -v '^[[:space:]]*//' internal/ado/pullrequests.go \| grep -c 'location\.')"` | ❌ W0 | ⬜ pending |
| 1-02-03 | 02 | 2 | SEAM-02 | T-01-07 | Fail-by-name guard on all 20 fake methods | compile | `go vet ./... && go test ./internal/ado/... -cover` | ❌ W0 | ⬜ pending |
| 1-03-01 | 03 | 3 | SEAM-03 | T-01-08, T-01-10 | Four field paths verbatim; float-typed effort values; order pinned | unit + mutation | `go test ./internal/ado/ -run 'TestUpdateWorkItem\|TestGetNextState' -v -cover` | ❌ W0 | ⬜ pending |
| 1-03-02 | 03 | 3 | SEAM-03 | T-01-08, T-01-11, T-01-12 | Empty identity issues no SDK call; fake guards intact | unit + mutation | `go test ./internal/ado/ -run 'TestApprovePullRequest\|TestCompletePullRequest' -v -cover` | ❌ W0 | ⬜ pending |
| 1-03-03 | 03 | 3 | SEAM-03 | T-01-08, T-01-09 | Status-swap mutation caught by cross-check | unit + mutation | `go test ./internal/ado/ -run 'TestApproveRelease\|TestRejectRelease\|TestApproveAndReject\|TestQueuePipeline' -v -cover` | ❌ W0 | ⬜ pending |
| 1-04-01 | 04 | 4 | SEAM-04 | T-01-14, T-01-15 | Failure injected at query call, not classification node | unit + mutation | `go test ./internal/ado/ -run 'TestGetWorkItemsSwallows\|TestGetDashboardCountsSwallows' -v -cover` | ❌ W0 | ⬜ pending |
| 1-04-02 | 04 | 4 | SEAM-04 | T-01-14, T-01-16 | Pipelines slice set so the fake is actually reached | unit + mutation | `go test ./internal/ado/ -run 'TestGetPipelineRunsSwallows\|TestGetPipelineDefinitionsSwallows\|TestGetBranchesSwallows' -v -cover` | ❌ W0 | ⬜ pending |
| 1-04-03 | 04 | 4 | SEAM-04 | T-01-14, T-01-17, T-01-18 | Exactly 8 swallow tests; sources reverted clean | unit + gate | `test 8 -eq "$(go test ./internal/ado/ -run SwallowsProjectFailure -v 2>/dev/null \| grep -c '^=== RUN   TestGet')" && go test ./... -race -cover` | ❌ W0 | ⬜ pending |

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
