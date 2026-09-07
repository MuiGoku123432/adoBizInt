---
phase: 2
slug: honest-reads
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
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

Two packages currently have tests: `internal/ado` and `internal/ui/releases`. Fourteen have none.

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
| TRUST-01 | Each of the 8 aggregate reads returns a failure entry naming project + operation | — | Failure carries project/operation/kind, never a URL or header | unit | `go test ./internal/ado/ -run 'TestGet.*ReportsProjectFailure' -count=1` | ✅ (invert the 8 existing) | ⬜ pending |
| TRUST-01 | `GetWorkItems` batch failure (workitems.go:157) reported; project not marked complete | — | N/A | unit | `go test ./internal/ado/ -run TestGetWorkItemsReportsBatchFailure` | ❌ W0 | ⬜ pending |
| TRUST-01 | `GetBranches` per-repo failure (branches.go:78) is reported | — | N/A | unit | `go test ./internal/ado/ -run TestGetBranchesReportsRepoFailure` | ❌ W0 | ⬜ pending |
| TRUST-01 | `getProjectCounts` reports its three in-place failures and still returns collected counts | — | N/A | unit | `go test ./internal/ado/ -run TestGetDashboardCountsReportsPartial` | ✅ (invert `dashboard_test.go`) | ⬜ pending |
| TRUST-02 | 401 in a `WrappedError` **value** → kind `auth` | T-02-01 | Value and pointer forms both classify; `errors.As` alone is insufficient | unit | `go test ./internal/ado/ -run TestClassifyFailure` | ❌ W0 | ⬜ pending |
| TRUST-02 | 401 in a `*WrappedError` **pointer** → kind `auth` | T-02-01 | same | unit | same | ❌ W0 | ⬜ pending |
| TRUST-02 | 403 → `auth`; 429 → `rate_limit`; deadline → `timeout`; other → `other` | T-02-01 | No silent success on 4xx | unit | same | ❌ W0 | ⬜ pending |
| TRUST-02 | An auth failure yields a non-empty `Failures` list even when `Items` is empty | T-02-01 | Empty ≠ failed | unit | `go test ./internal/ado/ -run 'TestGet.*AuthFailure'` | ❌ W0 | ⬜ pending |
| TRUST-03 | `Complete == false` whenever `len(Failures) > 0`; `true` only on a clean sweep | — | N/A | unit | `go test ./internal/ado/ -run TestResultCompleteness` | ❌ W0 | ⬜ pending |
| TRUST-04 | `connection.Timeout` non-nil and equals `cfg.RequestTimeout` after `NewClient` (construction-order assertion — `NewClient` performs network I/O) | T-02-04 | Unbounded call cannot hang the TUI | unit | `go test ./internal/ado/ -run TestNewClientSetsTimeout` | ❌ W0 | ⬜ pending |
| TRUST-04 | Both timeout defaults non-zero after `config.Load()` with no file | T-02-04 | Zero-value duration is not "no timeout" | unit | `go test ./internal/config/ -run TestLoadAppliesTimeoutDefaults` | ❌ W0 (package has no test file) | ⬜ pending |
| TRUST-04 | Expired fetch context yields `timeout`-kind failures for unreached projects (D-12) | T-02-04 | N/A | unit | `go test ./internal/ado/ -run TestFetchTimeoutMarksUnreached` | ❌ W0 | ⬜ pending |
| OUT-02 | Each of the 11 serializable structs marshals to the D-04 names | — | N/A | unit | `go test ./internal/ado/ -run TestJSONFieldNames` | ❌ W0 | ⬜ pending |
| OUT-02 | `PipelineRun.Duration` marshals to the agreed shape | — | N/A | unit | same | ❌ W0 | ⬜ pending |
| D-13/14/15 | Reducer sets banner / warning / stale marker from the envelope | T-02-02 | Server text truncated; kind rendered prominently | unit | `go test ./internal/ui/... -count=1` | ❌ W0 (5 view packages lack tests) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/ado/result_test.go` — classifier table: value + pointer `WrappedError`, 401, 403, 429, deadline, canceled, nil-StatusCode. Covers TRUST-02.
- [ ] `internal/config/config_test.go` — package has **no** test file today. Covers TRUST-04 defaults.
- [ ] `internal/ado/json_test.go` — field-name assertions for the 11 serializable structs. Covers OUT-02.
- [ ] Fake extensions in `internal/ado/fakes_test.go` — existing fakes fail loudly when a function field is unset (deliberate Phase 1 property). Injecting *typed* SDK errors rather than `errors.New` needs new helper constructors.
- [ ] Test files for `internal/ui/dashboard`, `workitems`, `prs`, `pipelines`, `branches` — five view packages with no tests. Covers D-13/D-14/D-15 at the reducer level.
- [ ] Framework install: none needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ADO returns 401 (not 302/203) for a rejected PAT on these endpoints (assumption A1, LOW confidence, load-bearing for TRUST-02) | TRUST-02 | Requires a real invalid PAT against a live org; cannot be faked | Run the tool with a deliberately invalid/expired PAT. Record the observed status code and response body shape. Confirm `SuppressFedAuthRedirect: true` forces 401. Finalize the classifier only after this. |
| Rendered TUI output actually shows the failure banner | D-13 | `View()` returns a lipgloss-styled string; asserting on it is brittle | Visual check against an invalid PAT |

---

## Validation Sign-Off

> **Owner:** plan `02-05`, **Task 4 (phase close)**. That task runs the repo-wide gate,
> sets every Status cell in the map above from the real result of running that row's
> command, ticks the boxes below, and flips the frontmatter to
> `status: validated` / `nyquist_compliant: true` / `wave_0_complete: true` — but only
> if every box below is genuinely ticked. If any is not, the frontmatter stays as-is and
> the reason is recorded in `02-05-SUMMARY.md`.

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 45s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
