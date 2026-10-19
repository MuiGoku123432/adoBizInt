---
phase: 8
slug: platform-upgrade-and-pacing
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: true) (#2117)
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-10-19
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing` + `testing/synctest` on go1.27.2; `go/ast` for structural tests) |
| **Config file** | none (CI: `.github/workflows/ci.yml`) |
| **Quick run command** | `go test ./internal/transport/... ./internal/ado/... ./internal/guard/... ./internal/config/... ./cmd/adoBizInt/...` |
| **Full suite command** | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` |
| **Estimated runtime** | ~30 seconds (about 2s per package under `-race`; synctest tests add no wall time; govulncheck dominates) |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched packages (quick run command above)
- **After every plan wave:** Run `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- **Before `/gsd-verify-work`:** Full suite (including govulncheck) must be green, a green CI run on the pushed `dev` branch, and `08-SMOKE.md` filled
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01 T1 | 01 | 1 | PLAT-02 (SC1, SC2) | T-08-01, T-08-02, T-08-03, T-08-SC | One SDK module (v7.1.0), go 1.27.N with no toolchain line, x/sys v0.47.0, govulncheck clean and in CI; a real v7 401 (value and pointer form, GET and PATCH) classifies auth | integration + unit (real SDK, fake RoundTripper) | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... && test "$(go list -m all \| grep -c 'microsoft/azure-devops-go-api')" = 1 && go test ./internal/ado -run 'TestV7.*Auth' -count=1 -v` | ❌ W0 (`internal/ado/v7_test.go`) | ⬜ pending |
| 08-01 T2 | 01 | 1 | PLAT-02 | — | b5 line references refreshed; PROJECT.md Go constraint and exit-4 label corrected | comment/doc check | `go test ./internal/ado -count=1 && grep -c 'client.go:375' internal/ado/result.go` | ✅ | ⬜ pending |
| 08-02 T1 | 02 | 2 | PLAT-03 (SC4) | T-08-05, T-08-13 | Every sub-client built on tr.HTTPClient(); a real v7 git call reaches the base only through the transport; request_timeout visible as a deadline | unit (real SDK, fake RoundTripper) | `go test ./internal/ado -run 'TestNewClientBuildsOnInjectedClient\|TestNewClientSetsTimeout\|TestNewClientZeroTimeoutInstallsNoDeadline\|TestNoRawHTTPInADO' -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-02 T2 | 02 | 2 | PLAT-03 (SC4) | T-08-06, T-08-07, T-08-08, T-08-09, T-08-10, T-08-11 | Host rule table; refusal before base; redirect hop not ErrNotSent; no PAT in logs or errors; transport never prints; org_url shape (D-12); startup LocationUrl refusal | unit (table, AST) | `go test ./internal/transport ./internal/config ./internal/ado -run 'TestAllowed\|TestRefuse\|TestRefusalNeverLogsPAT\|TestTransportNeverPrints\|TestHTTPClientTimeout\|TestValidateOrgURL\|TestOrg\|TestValidate\|TestNewClientRefusesForeignLocationURL\|TestValidatedOrgURLPassesHostRule' -count=1 -v` | ❌ W0 (`internal/transport/transport_test.go`) | ⬜ pending |
| 08-02 T3 | 02 | 2 | PLAT-03 (D-11) | T-08-13 | No GetClientByUrl / typed NewClient / option-less NewClientWithOptions in internal/ado | structural (go/ast) | `go test ./internal/ado -run 'TestSubClientsUseInjectedClient\|TestNoRawHTTPInADO' -count=1 -v` | ❌ W0 (`internal/ado/construct_test.go`) | ⬜ pending |
| 08-03 T1 | 03 | 2 | PLAT-05 | T-08-14, T-08-17 | Fresh build plus provenance gate; one read row; safe with no credentials | shell (isolated run) | `bash -n scripts/smoke.sh` plus the isolated no-credential run in the task verify | ❌ W0 (`scripts/smoke.sh`) | ⬜ pending |
| 08-03 T2 | 03 | 2 | PLAT-05 | T-08-14, T-08-15, T-08-16 | 8 reads, identity row, 6 dry runs with SKIP; never confirms; never prints the PAT; 08-SMOKE.md checklist | shell (isolated run with sentinel PAT) | `bash -n scripts/smoke.sh` plus the isolated sentinel-PAT run in the task verify | ❌ W0 | ⬜ pending |
| 08-04 T1 | 04 | 3 | PLAT-03 (SC3) | T-08-19 | Retry-After on a 200 delays the next real v7 request by exactly the instruction | unit (synctest, real SDK) | `go test ./internal/ado -run TestRetryAfterOn200PacesNextRequest -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-04 T2 | 04 | 3 | PLAT-03 (SC3) | T-08-19, T-08-21, T-08-22, T-08-24, T-08-25 | HTTP-date, X-RateLimit-Delay, max not sum, no stacking, Remaining ignored, deadline boundary fail-fast, held cancel, stats, logging; fail-fast classifies rate_limit | unit (synctest, -race) | `go test -race ./internal/transport -run 'TestPace\|TestFailFast\|TestHeldCancel\|TestStats' -count=1 -v && go test ./internal/ado -run 'TestClassifyFailure\|TestSessionIDUnchanged' -count=1` | ❌ W0 | ⬜ pending |
| 08-04 T3 | 04 | 3 | PLAT-03 (SC3, D-02) | T-08-20 | Not-sent write failed and unaudited even with a cancelled parent; redirect hop unknown and audited; 429 write failed, Mutate once, exit 1 | unit | `go test ./internal/guard -run 'TestRunNotSent\|TestRunClassifiesOutcomes' -count=1 -v && go test ./cmd/adoBizInt -run TestRateLimitedWriteExitsOne -count=1` | ❌ W0 (`cmd/adoBizInt/ratelimit_test.go`) | ⬜ pending |
| 08-05 T1 | 05 | 4 | PLAT-03 (SC3, D-05..D-07) | T-08-29 | A 429 git read is retried once through the pacer and counted; one stderr summary line on the CLI path only | unit (synctest) + AST | `go test ./internal/ado -run TestReadRetriedOnRateLimitThenSucceeds -count=1 -v && go test ./cmd/adoBizInt -run 'TestThrottleSummary' -count=1 -v` | ❌ W0 (`cmd/adoBizInt/throttle_test.go`) | ⬜ pending |
| 08-05 T2 | 05 | 4 | PLAT-03 (SC3, D-03) | T-08-26, T-08-27, T-08-28 | At most 2 retries on 429 only; fail-fast, ended context and non-429 not retried; no write override (AST); startup reads retry | unit + structural (go/ast) | `go test ./internal/ado -run 'TestRetry\|TestStartupReadsRetry' -count=1 -v` | ❌ W0 (`internal/ado/retry_test.go`) | ⬜ pending |
| 08-06 T1 | 06 | 5 | PLAT-02, PLAT-05 | — | Full gate green on the tree handed to the user | full suite | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... && bash -n scripts/smoke.sh` | ✅ | ⬜ pending |
| 08-06 T2 | 06 | 5 | PLAT-05 (SC5), PLAT-02 (SC1 CI) | T-08-31 | Live reads, dry runs, scratch-item writes, TUI checklist, CI govulncheck green | manual (checkpoint:human-verify) | user runs `scripts/smoke.sh`; see Manual-Only Verifications | n/a | ⬜ pending |
| 08-06 T3 | 06 | 5 | PLAT-05 | T-08-30 | Record has no pending rows, provenance shows v7.1.0, no credential text | doc check | `grep -c 'result: \[pending\]' .planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md` prints 0 (full command in the task verify) | ❌ (filled by 08-06) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Each test file is created by the task that first needs it (tests ship with the code in the same commit; tdd tasks write them first).

- [ ] `internal/ado/v7_test.go` (08-01 T1; extended by 08-02 T1/T2, 08-04 T1/T2, 08-05 T1): shared `rtFunc`, `jsonResp`, locations fixture (resource areas `e81700f7-3be2-46de-8624-2eb35882fcaa` plus each data route), `uniqueOrgURL`; v7 401 value and pointer forms; NewClient discovery and startup refusal; pacing and retry end-to-end tests
- [ ] `internal/transport/transport_test.go` (08-02 T2; extended by 08-04 T2): host rule table, refusal before send, redirect hop, PAT never logged, never prints, synctest pacing, fail-fast boundary, held cancel, stats, logging
- [ ] `internal/ado/construct_test.go` (08-02 T3): D-11 AST assertion (non-vacuous)
- [ ] `internal/ado/client_test.go` rewrite (08-02 T1): injected client Timeout as a request deadline
- [ ] `internal/config/config_test.go` (08-02 T2): `org_url` shape table and `Org()`
- [ ] `internal/guard/guard_test.go` (08-04 T3): `TestRunNotSent` and a 429 row
- [ ] `cmd/adoBizInt/ratelimit_test.go` (08-04 T3): SC3 write exit 1, Mutate once (new file; Phase 7 locks untouched)
- [ ] `internal/ado/retry_test.go` (08-05 T2): retry behaviour plus the no-write-override AST assertion
- [ ] `cmd/adoBizInt/throttle_test.go` (08-05 T1): summary line and CLI-path-only AST check
- [ ] `scripts/smoke.sh` and `08-SMOKE.md` checklist (08-03)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| One live read per ADO sub-client and one dry run per write verb against the real org, recorded | PLAT-05 (SC5) | Fakes cannot observe 7.1 wire behaviour; needs the user's credential and org (no credentials in CI, v1.0 D-17) | `bash -n scripts/smoke.sh`; user runs `scripts/smoke.sh` with a freshly built binary and pastes the result table into `08-SMOKE.md` |
| CLI `set-state --confirm` on a scratch item; TUI Task close with effort; TUI attachment download | PLAT-05 (D-13) | Interactive TUI paths and live writes against the real org | Follow the TUI checklist in `08-SMOKE.md` and record outcomes |
| govulncheck gate runs and passes in CI | PLAT-02 (SC1) | Requires a pushed CI run | Push `dev`; confirm the CI govulncheck step is green |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (08-06 T2 is the manual checkpoint, bracketed by automated T1 and T3)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [ ] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
