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
| 08-01 T1 | 01 | 1 | PLAT-02 (SC1, SC2) | T-08-01, T-08-02, T-08-03, T-08-SC | One SDK module (v7.1.0), go 1.27.N with no toolchain line, x/sys v0.47.0, govulncheck clean and in CI; a real v7 401 (value and pointer form, GET and PATCH) classifies auth and maps to exitAuth through exitCodeFor and writeExitCodeFor (SC2 clarification, user 2026-10-19); post-swap lock hashes recorded | integration + unit (real SDK, fake RoundTripper) | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... && test "$(go list -m all \| grep -c 'microsoft/azure-devops-go-api')" = 1 && go test ./internal/ado -run 'TestV7.*Auth' -count=1 -v && go test ./cmd/adoBizInt -run TestV7AuthFailureExitsAuth -count=1 -v` plus the 7/7 and 1/1 lock checks | ❌ W0 (`internal/ado/v7_test.go`, `cmd/adoBizInt/v7auth_test.go`, `08-cli-tests.sha256`) | ⬜ pending |
| 08-01 T2 | 01 | 1 | PLAT-02 | — | b5 line references refreshed; PROJECT.md Go constraint and exit-4 label corrected; startup exit 3 recorded as user-accepted | comment/doc check | `go test ./internal/ado -count=1 && grep -c 'client.go:375' internal/ado/result.go` | ✅ | ⬜ pending |
| 08-02 T1 | 02 | 2 | PLAT-03 (SC4) | T-08-13, T-08-34 | Every sub-client built on tr.HTTPClient(); a real v7 git call reaches the base only through the transport; request_timeout visible as a deadline | unit (real SDK, fake RoundTripper) | `go test ./internal/ado -run 'TestNewClientBuildsOnInjectedClient\|TestNewClientSetsTimeout\|TestNewClientZeroTimeoutInstallsNoDeadline\|TestNoRawHTTPInADO' -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-02 T2 | 02 | 2 | PLAT-03 (SC4, D-10a) | T-08-05, T-08-12 | Every discovered LocationUrl, used or not, checked before any sub-client is built; foreign host on an unused area is exit 3 with a named org_url error | unit (real SDK, fake RoundTripper; tripwire) | `go test ./internal/ado -run 'TestNewClientRefusesForeignLocationURL\|TestNewClientAllowsEveryBoundArea' -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-02 T3 | 02 | 2 | PLAT-03 (D-11) | T-08-13 | No GetClientByUrl / typed NewClient / option-less NewClientWithOptions in internal/ado | structural (go/ast) | `go test ./internal/ado -run 'TestSubClientsUseInjectedClient\|TestNoRawHTTPInADO' -count=1 -v` | ❌ W0 (`internal/ado/construct_test.go`) | ⬜ pending |
| 08-03 T1 | 03 | 3 | PLAT-03 (SC4, D-12) | T-08-07 | Non-cloud org_url is exit 3 naming org_url from the real binary; every accepted shape passes the host rule | unit + binary run (isolated HOME) | `go test ./internal/config -run 'TestValidateOrgURL\|TestOrg\|TestValidate' -count=1 -v && go test ./internal/ado -run TestValidatedOrgURLPassesHostRule -count=1 -v` plus the isolated binary run in the task verify | ❌ W0 | ⬜ pending |
| 08-03 T2 | 03 | 3 | PLAT-03 (SC4) | T-08-06, T-08-08, T-08-09, T-08-10, T-08-11 | Host rule table, including an empty org that allows nothing; refusal before base; redirect hop not ErrNotSent; no PAT in logs or errors with a live-capture marker; transport never prints | unit (table, AST) | `go test -race ./internal/transport -run 'TestAllowed\|TestRefuse\|TestRefusalNeverLogsPAT\|TestTransportNeverPrints\|TestHTTPClientTimeout' -count=1 -v` | ❌ W0 (`internal/transport/transport_test.go`) | ⬜ pending |
| 08-04 T1 | 04 | 3 | PLAT-05 | T-08-14, T-08-17 | Fresh build with the go.mod toolchain (GOTOOLCHAIN pin, as CI's go-version-file) plus provenance gate; one read row in fixed-width columns; safe with no credentials; no half-applied parallel edits built | shell (isolated run) | `bash -n scripts/smoke.sh` plus the isolated no-credential run in the task verify | ❌ W0 (`scripts/smoke.sh`) | ⬜ pending |
| 08-04 T2 | 04 | 3 | PLAT-05 | T-08-14, T-08-15, T-08-16 | 8 reads, identity row, 6 `dry-run <noun> <verb>` rows with SKIP; never confirms; never prints the PAT; 08-SMOKE.md checklist, Dry-run waivers section with no template line in waiver-list form, and findings tables that are header and separator only | shell (isolated run with sentinel PAT) | `bash -n scripts/smoke.sh` plus the isolated sentinel-PAT run in the task verify | ❌ W0 | ⬜ pending |
| 08-05 T1 | 05 | 4 | PLAT-03 (SC3) | T-08-35 | Retry-After on a 200 delays the next real v7 request by exactly the instruction | unit (synctest, real SDK) | `go test ./internal/ado -run TestRetryAfterOn200PacesNextRequest -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-05 T2 | 05 | 4 | PLAT-03 (SC3, D-04) | — | HTTP-date, X-RateLimit-Delay, max not sum, no stacking, informational and invalid values ignored | unit (synctest, -race) | `go test -race ./internal/transport -run 'TestPaceRetryAfterHTTPDate\|TestPaceRateLimitDelay\|TestPaceMaxNotSum\|TestPaceSameInstructionDoesNotStack\|TestPaceRemainingIgnored\|TestPaceIgnoresInvalid' -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-05 T3 | 05 | 4 | PLAT-03 (SC3, D-05) | T-08-22, T-08-23, T-08-24, T-08-25, T-08-35 | Shared notBefore under -race with one base call per request; held cancel is not-sent; wait duration and host logged when a wait is taken; Stats; SessionId unchanged | unit (synctest, -race) | `go test -race ./internal/transport -run 'TestPaceConcurrent\|TestHeldCancelIsNotSent\|TestStats\|TestPaceLogs' -count=1 -v && go test ./internal/ado -run TestSessionIDUnchanged -count=1` | ❌ W0 | ⬜ pending |
| 08-06 T1 | 06 | 5 | PLAT-03 (SC3, D-01, D-08) | T-08-19, T-08-21 | An instruction past the deadline fails fast before send and a real v7 read classifies rate_limit with the D-08 text | unit (real SDK, fake RoundTripper) | `go test ./internal/ado -run 'TestFailFastReadClassifiesRateLimit\|TestClassifyFailure' -count=1 -v` | ❌ W0 | ⬜ pending |
| 08-06 T2 | 06 | 5 | PLAT-03 (SC3, D-01) | T-08-19, T-08-21 | Deadline boundary (minus 1ns, equal, plus 1ns); Client.Timeout is a deadline; fail-fast not counted; held, refused and exhausted reads classify as specified | unit (synctest, -race) | `go test -race ./internal/transport -run 'TestFailFast\|TestHeldCancel' -count=1 -v && go test ./internal/ado -run TestClassifyFailure -count=1` | ❌ W0 | ⬜ pending |
| 08-06 T3 | 06 | 5 | PLAT-03 (SC3, D-02) | T-08-20 | Refused-host, fail-fast and held-cancel writes failed and unaudited even with a cancelled parent; redirect hop unknown and audited; 429, refused-host and fail-fast writes run once and exit 1 | unit | `go test ./internal/guard -run 'TestRunNotSent\|TestRunClassifiesOutcomes' -count=1 -v && go test ./cmd/adoBizInt -run TestRateLimitedWriteExitsOne -count=1 -v` | ❌ W0 (`cmd/adoBizInt/ratelimit_test.go`) | ⬜ pending |
| 08-07 T1 | 07 | 6 | PLAT-03 (SC3, D-05..D-07) | T-08-29 | A 429 git read is retried once through the pacer and counted; one stderr summary line on the CLI path only | unit (synctest) + AST | `go test ./internal/ado -run TestReadRetriedOnRateLimitThenSucceeds -count=1 -v && go test ./cmd/adoBizInt -run 'TestThrottleSummary' -count=1 -v` | ❌ W0 (`cmd/adoBizInt/throttle_test.go`) | ⬜ pending |
| 08-07 T2 | 07 | 6 | PLAT-03 (SC3, D-03) | T-08-26, T-08-27, T-08-28 | At most 2 retries on 429 only; fail-fast, ended context and non-429 not retried; no write override (AST); startup reads retry | unit + structural (go/ast) | `go test ./internal/ado -run 'TestRetry\|TestStartupReadsRetry' -count=1 -v` | ❌ W0 (`internal/ado/retry_test.go`) | ⬜ pending |
| 08-08 T1 | 08 | 7 | PLAT-02, PLAT-05 | — | Full gate and Phase 7 lock hashes green on the tree handed to the user | full suite | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... && bash -n scripts/smoke.sh` plus the 7/7 and 1/1 lock checks | ✅ | ⬜ pending |
| 08-08 T2 | 08 | 7 | PLAT-05 (SC5), PLAT-02 (SC1 CI) | T-08-31, T-08-36 | Live reads, six dry runs PASS/NODIFF or user-waived, scratch-item writes, TUI checklist, CI govulncheck green | manual (checkpoint:human-verify) | user runs `scripts/smoke.sh`; see Manual-Only Verifications | n/a | ⬜ pending |
| 08-08 T3 | 08 | 7 | PLAT-05 | T-08-30, T-08-36 | Record has no pending rows, provenance shows v7.1.0, no credential text; exactly 9 read and identity rows and 6 distinct dry-run rows; every FAIL row (and an identity SKIP) named in exactly one findings table; every SKIP dry-run row matched to a user waiver by exact row name or listed under Blocks Phase 9; status blocked exactly when Blocks Phase 9 has a finding, complete otherwise (pre-existing FAILs are the D-15 SC5 exception) | doc check (grep plus awk record gate) | `grep -c 'result: \[pending\]' .planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md` prints 0, then the awk record gate (row counts parsed only inside the `## Script output` section, FAIL classification, SKIP-to-waiver match by name, status consistency) in the task verify | ❌ (filled by 08-08) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Each test file is created by the task that first needs it (tests ship with the code in the same commit; tdd tasks write them first).

- [ ] `internal/ado/v7_test.go` (08-01 T1; extended by 08-02 T1/T2, 08-03 T1, 08-05 T1/T3, 08-06 T1/T2, 08-07 T1): shared `rtFunc`, `jsonResp`, locations fixture (resource areas `e81700f7-3be2-46de-8624-2eb35882fcaa` plus each data route), `uniqueOrgURL`; v7 401 value and pointer forms; NewClient discovery and the every-area startup refusal; pacing, fail-fast and retry end-to-end tests
- [ ] `cmd/adoBizInt/v7auth_test.go` (08-01 T1): SC2 exit-code proof and the cmd-package `rtFunc` twin (reused by 08-06 T3)
- [ ] `.planning/phases/08-platform-upgrade-and-pacing/08-cli-tests.sha256` (08-01 T1): post-swap hashes of the seven Phase-7-locked CLI test files; every later code plan asserts 7/7 plus `07-cli-audit-test.sha256` 1/1
- [ ] `internal/transport/transport_test.go` (08-03 T2; extended by 08-05 T2/T3 and 08-06 T2): host rule table, refusal before send, redirect hop, PAT never logged, never prints, synctest pacing, wait logging, fail-fast boundary, held cancel, stats
- [ ] `internal/ado/construct_test.go` (08-02 T3): D-11 AST assertion (non-vacuous)
- [ ] `internal/ado/client_test.go` rewrite (08-02 T1): injected client Timeout as a request deadline
- [ ] `internal/config/config_test.go` (08-03 T1): `org_url` shape table and `Org()`
- [ ] `internal/guard/guard_test.go` (08-06 T3): `TestRunNotSent` (refused host, fail-fast, held cancel, redirect hop) and a 429 row
- [ ] `cmd/adoBizInt/ratelimit_test.go` (08-06 T3): SC3 write exit 1, Mutate once, for 429, refused host and fail-fast (new file; Phase 7 locks untouched)
- [ ] `internal/ado/retry_test.go` (08-07 T2): retry behaviour plus the no-write-override AST assertion
- [ ] `cmd/adoBizInt/throttle_test.go` (08-07 T1): summary line and CLI-path-only AST check
- [ ] `scripts/smoke.sh` and `08-SMOKE.md` checklist with the Dry-run waivers section (08-04)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| One live read per ADO sub-client and one dry run per write verb (PASS or NODIFF, or a user-written waiver per SKIP) against the real org, recorded | PLAT-05 (SC5) | Fakes cannot observe 7.1 wire behaviour; needs the user's credential and org (no credentials in CI, v1.0 D-17) | `bash -n scripts/smoke.sh`; user runs `scripts/smoke.sh` with a freshly built binary and pastes the result table into `08-SMOKE.md` (08-08) |
| CLI `set-state --confirm` on a scratch item; TUI Task close with effort; TUI attachment download | PLAT-05 (D-13) | Interactive TUI paths and live writes against the real org | Follow the TUI checklist in `08-SMOKE.md` and record outcomes (08-08) |
| govulncheck gate runs and passes in CI | PLAT-02 (SC1) | Requires a pushed CI run | Push `dev`; confirm the CI govulncheck step is green (08-08) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (08-08 T2 is the manual checkpoint, bracketed by automated T1 and T3)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [ ] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
