---
phase: 1
slug: test-seam-for-internal-ado
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-04
validated: 2026-09-06
---

# Phase 1 -- Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, no new deps) |
| **Config file** | none -- stdlib toolchain |
| **Quick run command** | `go test ./internal/ado/...` |
| **Full suite command** | `go build ./... && go vet ./... && go test ./... -count=1 -race -cover` |
| **Observed quick runtime** | 0.43 seconds |
| **Observed full runtime** | 4.05 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/ado/...`
- **After every plan wave:** Run `go build ./... && go vet ./... && go test ./... -count=1 -race -cover`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds
- **Audit observation:** Slowest targeted/full command was 4.05 seconds; all commands were one-shot and contained no watch flag.

---

## Requirement-to-Task/Test Map

| Requirement | Tasks | Automated evidence | Status |
|-------------|-------|--------------------|--------|
| SEAM-01 | 1-01-01, 1-02-01 | Four package-local interfaces exactly match the consumed 5/7/4/4 method sets; production constructor assignments compile. | green |
| SEAM-02 | 1-01-01, 1-02-01, 1-02-02, 1-02-03, 1-06-01 | Struct-literal fake injection drives production behavior; 20 fail-fast fake methods compile; the exact four approval tests pass; the `NewClient` provider-wiring structural gate passes. | green |
| SEAM-03 | 1-01-01, 1-01-02, 1-03-01, 1-03-02, 1-03-03, 1-06-02 | All seven writes have behavioral exact-call tests; `CompletePullRequest` separately whole-compares fetch and update literals in order; guarded fetch mutation evidence is recorded and source restoration is clean. | green |
| SEAM-04 | 1-04-01, 1-04-02, 1-05-01, 1-05-02, 1-05-03, 1-06-02 | Eight non-vacuous aggregate characterizations pass; D-15 exact top-level count and D-16 exact test-to-file/header map pass; all eight mutation records are present. | green |

---

## Per-Task Verification Map

> All 15 tasks have an explicit one-shot `<automated>` command in their PLAN. Historical mutation tasks are backed by the recorded source diff hunk and observed red output in the corresponding SUMMARY, followed by clean restoration and green reruns.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | SEAM-01, SEAM-02, SEAM-03 | T-01-01, T-01-02 | No PAT or connection in tests; exact state patch is behavioral | compile + unit | `gofmt -l internal/ado/ && go build ./... && go vet ./... && go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch -v -cover` | ✅ | ✅ green |
| 1-01-02 | 01 | 1 | SEAM-01, SEAM-02, SEAM-03 | T-01-02 | State-path mutation produced the named failure and was restored cleanly | mutation + unit | `test -z "$(git status --porcelain --untracked-files=no internal/ado/workitems.go)" && go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch && go build ./... && go vet ./...` | ✅ | ✅ green |
| 1-02-01 | 02 | 2 | SEAM-01 | T-01-07 | Exactly four interfaces and 20 methods in the 5/7/4/4 split; no concrete implementation assertion | structural + compile | `gofmt -l internal/ado/ && go build ./... && go vet ./... && go test ./internal/ado/... -cover` plus interface parser gate | ✅ | ✅ green |
| 1-02-02 | 02 | 2 | SEAM-02 | T-01-04, T-01-05, T-01-06 | Authenticated-user ID is cached and approval recovery remains injectable; warning/error logging excludes credentials | structural + unit | Historical command superseded by `go test ./internal/ado/ -run 'TestApprovePullRequest' -v -cover` plus the Wave 6 `NewClient` provider-wiring gate | ✅ | ✅ green (superseded gate) |
| 1-02-03 | 02 | 2 | SEAM-02 | T-01-07 | All 20 fake methods retain fail-by-name guards | compile + structural | `go vet ./... && go test ./internal/ado/... -cover` plus 20-method/20-`Fatalf` parser gate | ✅ | ✅ green |
| 1-03-01 | 03 | 3 | SEAM-03 | T-01-08, T-01-10 | Four exact field paths, float effort values, operation order, and transition outcomes are pinned | unit + mutation | `go test ./internal/ado/ -run 'TestUpdateWorkItem\|TestGetNextState' -v -cover` | ✅ | ✅ green |
| 1-03-02 | 03 | 3 | SEAM-03 | T-01-08, T-01-11, T-01-12 | Approval/complete exact calls and no-Git guard branches are exercised | unit + mutation | `go test ./internal/ado/ -run 'TestApprovePullRequest\|TestCompletePullRequest' -v -cover` | ✅ | ✅ green |
| 1-03-03 | 03 | 3 | SEAM-03 | T-01-08, T-01-09 | Approve/reject divergence and exact queue request are behavioral | unit + mutation | `go test ./internal/ado/ -run 'TestApproveRelease\|TestRejectRelease\|TestApproveAndReject\|TestQueuePipeline' -v -cover` | ✅ | ✅ green |
| 1-04-01 | 04 | 4 | SEAM-04 | T-01-14, T-01-15 | Query failure is swallowed for one project; classification-node trap is independently exercised | integration + mutation | `go test ./internal/ado/ -run 'TestGetWorkItemsSwallows' -v -cover` | ✅ | ✅ green |
| 1-04-02 | 04 | 4 | SEAM-04 | T-01-14, T-01-19, T-01-20 | Dashboard pins in-place per-call swallowing and both projects' PR contribution | integration + mutation | `go test ./internal/ado/ -run 'TestGetDashboardCountsSwallows' -v -cover` | ✅ | ✅ green |
| 1-05-01 | 05 | 5 | SEAM-04 | T-01-14, T-01-16 | Both pipeline fixtures bypass empty-config short circuit; branch swallow is pinned | integration + mutation | `go test ./internal/ado/ -run 'TestGetPipelineRunsSwallows\|TestGetPipelineDefinitionsSwallows\|TestGetBranchesSwallows' -v -cover` | ✅ | ✅ green |
| 1-05-02 | 05 | 5 | SEAM-04 | T-01-14, T-01-17 | Pull request/release/approval survivors are non-empty and release filters are active | integration + mutation | `go test ./internal/ado/ -run 'TestGetPullRequestsSwallows\|TestGetReleasesSwallows\|TestGetPendingApprovalsSwallows' -v -cover` | ✅ | ✅ green |
| 1-05-03 | 05 | 5 | SEAM-04 | T-01-18, T-01-21 | Exactly eight top-level swallow tests; full race/coverage suite green | gate | `test 8 -eq "$(go test ./internal/ado/ -run SwallowsProjectFailure -v 2>/dev/null \| grep -c '^=== RUN   TestGet.*SwallowsProjectFailure$')" && go test ./... -race -cover` | ✅ | ✅ green |
| 1-06-01 | 06 | 6 | SEAM-02 | T-01-22, T-01-23, T-01-24, T-01-25, T-01-27 | Exact four approval declarations/RUN/PASS records; cache bypass, nil/ID-less guards, and retry counts 1/0, 2/1, 3/2; provider wiring structurally proven | unit + structural integration | Task 1 command from `01-06-PLAN.md` | ✅ | ✅ green |
| 1-06-02 | 06 | 6 | SEAM-03, SEAM-04 (D-15, D-16) | T-01-26 | Whole fetch/update assertions and order pass; guarded mutation evidence proves fetch mismatch; exact eight-name/file/header aggregate contract passes | unit + structural + guarded mutation + gate | Task 2 command from `01-06-PLAN.md`, plus audit reruns of its non-mutating gates | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Historical Guarded Mutation Evidence

| Scope | Evidence |
|-------|----------|
| 01-01 | 1 mutation hunk and named failing output in `01-01-SUMMARY.md` |
| 01-03 | 9 mutation hunks and corresponding failing output in `01-03-SUMMARY.md` |
| 01-04 | 2 mutation hunks and corresponding failing output in `01-04-SUMMARY.md` |
| 01-05 | 6 mutation hunks and corresponding failing output in `01-05-SUMMARY.md` |
| 01-06 | Fetch-only `Project: &project` to `Project: &repositoryID` hunk, targeted `GetPullRequest args` failure, restoration from `b63289a`, and restored green output in `01-06-SUMMARY.md` |

---

## Wave 0 Requirements

- [x] `internal/ado/interfaces.go` -- four narrow unexported interfaces, 20 methods total
- [x] `internal/ado/fakes_test.go` -- four hand-written fail-fast fakes, 20 guarded methods
- [x] `internal/ado/workitems_test.go`, `pullrequests_test.go`, `releases_test.go`, `pipelines_test.go`, `branches_test.go`, `dashboard_test.go`

*No framework install needed -- `go test` is stdlib.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| TUI launches against a live organization | SEAM-02 | Live credentials and an interactive terminal are outside the automated phase audit | Optional smoke check only. Build and unchanged caller wiring are automated; approval retry behavior is covered without network through the injected provider. |
| Aggregate ordering backstop from the original verification report | SEAM-04 | The two reads and required order were never specified | Not counted as a Nyquist gap. Define the reads and expected ordering before adding a meaningful held-out test. |

---

## Validation Sign-Off

- [x] All 15 tasks have an `<automated>` verify command
- [x] Every referenced behavioral test exists and is discoverable by `go test -list`
- [x] Sampling continuity: no task lacks automated evidence
- [x] Wave 0 covers all previously missing references
- [x] No watch-mode flags in any of the 15 task commands
- [x] Feedback latency is bounded below 30 seconds (observed maximum: 4.05 seconds)
- [x] Wave 6 exact four approval declaration/RUN/PASS gate passes
- [x] Wave 6 `NewClient` provider-wiring structural gate passes
- [x] Wave 6 whole-fetch and preserved whole-update structural gate passes
- [x] Guarded mutation evidence contains source hunks, named red output, restoration, and green reruns
- [x] D-15/D-16 exact eight-test count, file mapping, and adjacent headers pass
- [x] Build, vet, and full `-race -cover` suite pass
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated on 2026-09-06

---

## Validation Audit 2026-09-06

| Metric | Count |
|--------|-------|
| Plans audited | 6 |
| Tasks audited | 15 |
| Requirements mapped | 4 |
| Required test references checked | 21 |
| Missing test references | 0 |
| Watch commands | 0 |
| Gaps found | 0 |
| Resolved | 15 |
| Escalated | 0 |

### Observed Results

- All 13 original tasks and both Wave 6 gap tasks have explicit automated evidence.
- All current targeted behavior groups passed. The historical 01-02 Task 2 grep (`location.` must be absent from `pullrequests.go`) is superseded by Wave 6's required provider type and exact structural wiring gate; running that obsolete grep alone exits 1 because `ApprovePullRequest` intentionally consumes `location.GetConnectionDataArgs` for retry.
- Exact four approval tests: 4 declarations, 4 top-level RUN records, 4 PASS records.
- Whole-fetch structural gate: passed; `TestCompletePullRequestIssuesFetchThenMerge` whole-compares `git.GetPullRequestArgs`, preserves whole `git.UpdatePullRequestArgs`, and asserts `get` then `update`.
- Guarded fetch mutation: recorded semantic fetch-only hunk, named assertion failure, commit-based restoration, clean source comparison, and restored green result.
- D-15/D-16: exactly 8 top-level aggregate tests, exact expected file mapping, no duplicates, and all adjacent headers include `TRUST-01` and `Phase 2`.
- `go build ./...`: passed.
- `go vet ./...`: passed.
- `go test ./... -count=1 -race -cover`: passed; `internal/ado` coverage was 57.4%.
- Fast package/structural gate observed at 0.43 seconds; full build/vet/race/coverage gate observed at 4.05 seconds.
- No implementation or test file was modified during this audit.
