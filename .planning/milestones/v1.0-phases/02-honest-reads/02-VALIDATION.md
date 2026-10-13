---
phase: 2
slug: honest-reads
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-07
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `02-RESEARCH.md` §Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no assertion library, no mock framework) |
| **Config file** | none — `go test` convention |
| **Quick run command** | `go test ./internal/ado/... -count=1` |
| **Full suite command** | `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds (quick), ~45 seconds (full) |

At phase close, eight packages have tests: `internal/ado`, `internal/config`, `internal/utils`,
and all six view packages (`dashboard`, `workitems`, `prs`, `pipelines`, `branches`, `releases`).
At phase start only two did.

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/ado/... -count=1`
- **After every plan wave:** Run `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1`
  — **except inside wave 4.** Plans `02-04` and `02-05` share wave 4 and own different
  Go packages (`internal/ado` and `internal/ui/...` respectively). Go compiles per
  package, so a repo-wide gate run while the sibling plan is mid-edit fails spuriously.
  Within wave 4, each plan's tasks are scoped to the package it owns, and the repo-wide
  gate is `02-05` Task 4 (phase close), which is gated on `02-04-SUMMARY.md` existing.
- **Before `/gsd-verify-work`:** Full suite must be green
- **Phase gate (additional):** `go test ./internal/ado/ -v -run TestGet 2>&1 | grep -cE '^=== RUN   TestGet[A-Za-z]*SwallowsProjectFailure$'` returns `0`
  (the `[A-Za-z]*` + `$` anchoring excludes subtest lines, which plans 02-01..02-03 add
  under these exact parents)
- **Max feedback latency:** 45 seconds

---

## Per-Task Verification Map

| Req | Behavior | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-----|----------|------------|-----------------|-----------|-------------------|-------------|--------|
| TRUST-01 | Each of the 8 aggregate reads returns a failure entry naming project + operation | — | Failure carries project/operation/kind, never a URL or header | unit | `go test ./internal/ado/ -run 'TestGet.*ReportsProjectFailure' -count=1` | ✅ 8 reads inverted | ✅ green (10 tests) |
| TRUST-01 | `GetWorkItems` batch failure (workitems.go:157) reported; project not marked complete | — | N/A | unit | `go test ./internal/ado/ -run TestGetWorkItemsReportsBatchFailure -count=1` | ✅ `workitems_test.go` | ✅ green (2 tests) |
| TRUST-01 | `GetBranches` per-repo failure (branches.go:78) is reported | — | N/A | unit | `go test ./internal/ado/ -run TestGetBranchesReportsRepoFailure -count=1` | ✅ `branches_test.go` | ✅ green (3 tests) |
| TRUST-01 | `getProjectCounts` reports its three in-place failures and still returns collected counts | — | N/A | unit | `go test ./internal/ado/ -run 'TestGet(Project\|Dashboard)Counts.*Partial\|TestGetDashboardCountsReportsProjectFailure' -count=1` | ✅ `dashboard_test.go` | ✅ green (7 tests) |
| TRUST-02 | 401 in a `WrappedError` **value** → kind `auth` | T-02-01 | Value and pointer forms both classify; `errors.As` alone is insufficient | unit | `go test ./internal/ado/ -run TestClassifyFailure -count=1` | ✅ `result_test.go` | ✅ green (16 tests) |
| TRUST-02 | 401 in a `*WrappedError` **pointer** → kind `auth` | T-02-01 | same | unit | same | ✅ `result_test.go` | ✅ green (16 tests) |
| TRUST-02 | 403 → `auth`; 429 → `rate_limit`; deadline → `timeout`; other → `other` | T-02-01 | No silent success on 4xx | unit | same | ✅ `result_test.go` | ✅ green (16 tests) |
| TRUST-02 | An auth failure yields a non-empty `Failures` list even when `Items` is empty | T-02-01 | Empty ≠ failed | unit | `go test ./internal/ado/ -run 'TestGet.*ClassifiesRejectedToken' -count=1` | ✅ 7 reads | ✅ green (7 tests) |
| TRUST-03 | `Complete == false` whenever `len(Failures) > 0`; `true` only on a clean sweep | — | N/A | unit | `go test ./internal/ado/ -run TestResultCompleteness -count=1` | ✅ `result_test.go` | ✅ green (4 tests) |
| TRUST-04 | `connection.Timeout` non-nil and equals `cfg.RequestTimeout` after `NewClient` (construction-order assertion — `NewClient` performs network I/O) | T-02-04 | Unbounded call cannot hang the TUI | unit | `go test ./internal/ado/ -run TestNewClientSetsTimeout -count=1` | ✅ `client_test.go` | ✅ green (1 test) |
| TRUST-04 | Both timeout defaults non-zero after `config.Load()` with no file | T-02-04 | Zero-value duration is not "no timeout" | unit | `go test ./internal/config/ -run TestLoadAppliesTimeoutDefaults -count=1` | ✅ `config_test.go` created | ✅ green (4 tests) |
| TRUST-04 | Expired fetch context yields `timeout`-kind failures for unreached projects (D-12) | T-02-04 | N/A | unit | `go test ./internal/ado/ -run TestFetchTimeoutMarksUnreached -count=1` | ✅ 6 reads | ✅ green (6 tests) |
| OUT-02 | Each of the 11 serializable structs marshals to the D-04 names | — | N/A | unit | `go test ./internal/ado/ -run TestJSONFieldNames -count=1` | ✅ `json_test.go` | ✅ green (17 tests) |
| OUT-02 | `PipelineRun.Duration` marshals to the agreed shape | — | N/A | unit | same | ✅ `json_test.go` | ✅ green (17 tests) |
| D-13/14/15 | Reducer sets banner / warning / stale marker from the envelope | T-02-02 | Server text truncated; kind rendered prominently | unit | `go test ./internal/ui/... -count=1` | ✅ all 6 view packages | ✅ green |

*Status: ⬜ not yet run · ✅ green · ❌ red · ⚠️ flaky · test counts are `=== RUN Test` lines, subtests included*

---

## Wave 0 Requirements

- [x] `internal/ado/result_test.go` — classifier table: value + pointer `WrappedError`, 401, 403, 429, deadline, canceled, nil-StatusCode. Covers TRUST-02.
- [x] `internal/config/config_test.go` — package has **no** test file today. Covers TRUST-04 defaults.
- [x] `internal/ado/json_test.go` — field-name assertions for the 11 serializable structs. Covers OUT-02.
- [x] Fake extensions in `internal/ado/fakes_test.go` — existing fakes fail loudly when a function field is unset (deliberate Phase 1 property). Injecting *typed* SDK errors rather than `errors.New` needs new helper constructors.
- [x] Test files for `internal/ui/dashboard`, `workitems`, `prs`, `pipelines`, `branches` — five view packages with no tests. Covers D-13/D-14/D-15 at the reducer level.
- [x] Framework install: none needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ADO returns 401 (not 302/203) for a rejected PAT on these endpoints (assumption A1, LOW confidence, load-bearing for TRUST-02) | TRUST-02 | Requires a real invalid PAT against a live org; cannot be faked | Run the tool with a deliberately invalid/expired PAT. Record the observed status code and response body shape. Confirm `SuppressFedAuthRedirect: true` forces 401. Finalize the classifier only after this. |
| Rendered TUI output actually shows the failure banner | D-13 | `View()` returns a lipgloss-styled string; asserting on it is brittle | Visual check against an invalid PAT |

**Status at phase close:**

- **A1 (401 for a rejected PAT): DONE.** Verified empirically against the live service
  during 02-01; a rejected PAT yields HTTP 401 and classifies as `auth`. The classifier
  was finalized only after that observation, as this table required.
- **Rendered TUI banner: NOT DONE, and deliberately not claimed.** No human has run the
  tool against an invalid PAT since these six views changed. Each view now has a
  `View()`-substring assertion covering the same ground at the string level
  (`TestUpdateAuthFailureBlocksThe*` in all six packages), which is why the automated
  rows above are green — but a substring assertion is not a visual check, and this row
  stays manual and open. It is the correct subject for `/gsd-verify-work`.

---

## Validation Sign-Off

> **Owner:** plan `02-05`, **Task 4 (phase close)**. That task runs the repo-wide gate,
> sets every Status cell in the map above from the real result of running that row's
> command, ticks the boxes below, and flips the frontmatter to
> `status: validated` / `nyquist_compliant: true` / `wave_0_complete: true` — but only
> if every box below is genuinely ticked. If any is not, the frontmatter stays as-is and
> the reason is recorded in `02-05-SUMMARY.md`.

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 45s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved at phase close by plan `02-05` Task 4, 2026-09-07.

Repo-wide gate `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` is
fully green. The Phase 1 countdown gate returns `0`. Every Status cell above was set from
the real result of running that row's command.

Two rows carried a command that matched **zero** tests as originally seeded — the
`getProjectCounts` row and the `TestGet.*AuthFailure` row. Both behaviors are genuinely
covered, by tests under different names (`TestGetProjectCountsReportsPartialFailures` and
`TestGet*ClassifiesRejectedToken`). The commands were corrected to the real names rather
than the rows being marked green against a pattern that ran nothing; `go test` reports
`ok` for a pattern matching no tests, so a passing exit code on those two commands would
have proved nothing.
