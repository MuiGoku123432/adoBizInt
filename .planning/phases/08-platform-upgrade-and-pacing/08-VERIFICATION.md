---
phase: 08-platform-upgrade-and-pacing
verified: 2026-10-21T17:22:47Z
status: gaps_found
score: 2/5 roadmap success criteria verified (41/48 plan must-have truths)
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC1: the upgraded build (azuredevops/v7 v7.1.0, Go 1.27, patched golang.org/x/sys) passes govulncheck in CI"
    status: failed
    reason: "govulncheck v1.9.0 run during this verification exits 3 on go1.27.2. GO-2026-6809 (crypto/x509, distrust compromised intermediate certificates) is reachable through ado.Client.DownloadAttachment -> io.Copy -> x509.Certificate.Verify and is fixed in go1.27.3, released 2026-10-19. The next CI run on push would fail at the govulncheck step. The CI-on-push run was also never observed (dev not pushed; user waived it 2026-10-21)."
    artifacts:
      - path: "go.mod"
        issue: "go directive is 1.27.2; the lowest govulncheck-clean 1.27 patch is now 1.27.3 (08-01 must-have: 'the lowest 1.27 patch for which govulncheck v1.9.0 reports no vulnerabilities')"
      - path: ".planning/PROJECT.md"
        issue: "Tech stack line says Go 1.27.2; update with the bump"
    missing:
      - "Raise the go directive to 1.27.3 (no toolchain line), rerun `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` to a clean result, rerun the full suite"
      - "Push dev and record the CI run URL with the govulncheck step green (SC1 CI half)"
  - truth: "SC4: the PAT is only ever sent to hosts validated against the configured organization; a request aimed at any other host is refused before the credential leaves the process"
    status: failed
    reason: "The host rule (transport.Allowed) folds the host with strings.ToLower before comparing. Go folds U+0130 and U+212A to ASCII, while net/http dials the IDNA form of the original host. Reproduced in this verification with a go test -overlay probe (no repo change): Allowed(https://acme.v\\u0130sualstudio.com/x) = true, and the transport handed a request carrying an Authorization header to the base dialer for acme.xn--visualstudio-n4g.com:443, an ordinary non-Microsoft .com domain. A 307 from dev.azure.com/acme to that host was forwarded with the PATCH body; only net/http's own cross-domain redirect policy kept Authorization off it, which is exactly the stdlib behaviour the per-hop rule was written not to rely on. Same defect for {org}.*.visualstudio.com and for the dev.azure.com org-path compare (/\\u212Acme/ matches org kcme). Separately (WR-01), the path check runs on a decoded, cleaned path: https://dev.azure.com/other%2F..%2Facme/_apis is Allowed while the wire carries /other%2F..%2Facme/_apis, and on a same-host redirect net/http forwards Authorization. No first-hop PAT leak is reachable today, because the SDK's normalizeUrl and areaURL lowercase every base URL to ASCII before any request is built, but the rule SC4 asserts does not hold."
    artifacts:
      - path: "internal/transport/transport.go"
        issue: "Allowed (lines 103-119) uses strings.ToLower on host and path, accepts non-ASCII, and validates path.Clean of the decoded path rather than the escaped path sent on the wire"
      - path: "internal/transport/transport_test.go"
        issue: "TestAllowed has no non-ASCII row and no encoded-separator or dot-segment row"
      - path: "internal/config/config.go"
        issue: "parseOrgURL accepts non-ASCII hosts and org segments (same folding pattern)"
    missing:
      - "Refuse any host or path containing a non-ASCII byte before folding (08-REVIEW CR-01 fix)"
      - "Decide on the escaped path; refuse %2F/%5C and any path that path.Clean would change (08-REVIEW WR-01 fix)"
      - "TestAllowed rows, each want false: https://acme.v%C4%B0sualstudio.com/x, https://acme.evil.v\\u0130sualstudio.com/x, https://dev.azure.com/\\u212Acme/x with org kcme, https://dev.azure.com/other%2F..%2Facme/_apis, https://dev.azure.com/other/../acme/_apis"
      - "A redirect-hop test proving a 307 to an IDN homograph host is refused by the transport itself"
      - "parseOrgURL refuses non-ASCII"
  - truth: "SC5: one live read per ADO sub-client and one dry run per write verb pass against the real organization, and the results are recorded before Phase 9 starts"
    status: failed
    reason: "Live reads half is met: re-checked in ~/.config/adoBizInt/adoBizInt.log (167 lines dated 2026-10-21), workitemtracking, git, build, release (vsrm) and location identity all logged failure_count=0 on a v7.1.0/go1.27.2 binary, 0 ERROR, 0 host-rule refusals. Dry-run half is not met: scripts/smoke.sh never ran, 0 of 6 write-verb dry runs executed, no executed write of any kind went over the v7 wire, and the 08-08 Task 3 record gate fails. 08-SMOKE.md frontmatter itself says 'sc5_live_smoke: waived by user, not met'. The user's blanket waiver ('lets just move on') is recorded but is not a verification override, and no VERIFICATION.md override exists."
    artifacts:
      - path: ".planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md"
        issue: "Script output fence holds a 'not run' note; checklist items 1, 2, 3 and 5 not run; CI row not run"
    missing:
      - "Run scripts/smoke.sh with the SMOKE_* targets set (harness verified runnable in this verification), paste the output, and rerun the 08-08 Task 3 record gate"
      - "Run checklist items 1 (CLI set-state --confirm on a scratch item), 2 (TUI two-PATCH Task close), 3 (TUI attachment download under the host rule) and 5 (confirm-modal dismissals)"
      - "OR the user explicitly accepts SC5 as not met via an overrides entry in this file (suggested YAML in the report body); that is a decision for the developer, not the verifier"
---

# Phase 8: Platform Upgrade and Pacing Verification Report

**Phase Goal:** The tool runs on a supported SDK and toolchain with no reachable vulnerabilities, and slows down when Azure DevOps signals throttling without ever retrying a write
**Verified:** 2026-10-21T17:22:47Z
**Status:** gaps_found
**Re-verification:** No -- initial verification

## Bottom line

- The engineering core is real and well tested: the v7 swap is atomic and complete, the pacer, D-01 fail-fast, read-only retries, never-retried writes and the D-02 not-sent classification all exist, are wired, and are exercised by passing behavioural tests.
- The goal is not achieved as stated. Three of five success criteria fail:
  - "no reachable vulnerabilities" is false today: govulncheck fails the go1.27.2 build on a new stdlib CVE (fix: go 1.27.3).
  - The PAT host rule, the security boundary this phase ships, has a reproduced Unicode-folding bypass (08-REVIEW CR-01 is accurate).
  - The live smoke gate (SC5) was waived, not met: zero dry runs, zero live writes.
- Two of the three gaps are small code changes. SC5 needs either a real run or an explicit developer decision.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | CI runs govulncheck and fails on any reachable vulnerability; the upgraded build (v7.1.0, Go 1.27, patched x/sys) passes it; no v1.0.0-b5 import remains | FAILED | Gate part VERIFIED: ci.yml runs `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` after `go-version-file: go.mod`. No-b5 part VERIFIED: 0 b5 imports, 31 files import `/v7`, `go list -m all` lists only `azuredevops/v7 v7.1.0`, x/sys v0.47.0. Passes-it part FAILED: govulncheck run now exits 3, GO-2026-6809 in crypto/x509@go1.27.2, reachable via `internal/ado/attachments.go:126` DownloadAttachment -> io.Copy -> x509.Certificate.Verify, fixed in go1.27.3 (toolchain published 2026-10-19). CI on push never observed (waived) |
| 2 | A rejected PAT under v7 still exits with the auth-failure code, proven by a test | VERIFIED (warning) | `TestV7AuthFailureExitsAuth` passes at -count=1 (all 4 rows: read/write x JSON/empty body), feeding a real v7 ClientImpl 401 into exitCodeFor and writeExitCodeFor(outcomeFailed, err) -> exitAuth (2). `TestV7ReadAuthFailureClassifiesAuth`, `TestV7WriteAuthFailureClassifiesAuth` pass. Warning WR-05 confirmed: `go test -count=2` fails all 4 rows on the OPTIONS-sequence precondition (SDK global location cache), before the exit-code assertions run. Startup exit 3 for a fully rejected PAT is unchanged by user decision (08-CONTEXT SC2 clarification) |
| 3 | After Retry-After or rate-limit headers, including on HTTP 200, later requests wait as instructed; a rate-limited read retries a bounded number of times within its deadline; a rate-limited write is never retried and exits with the write-failure code | VERIFIED (warnings) | Behavioural tests pass: `TestRetryAfterOn200PacesNextRequest` (real v7 client, 200 + Retry-After 7 -> second send exactly 7s later), `TestPaceRetryAfterHTTPDate`, `TestPaceRateLimitDelay`, `TestPaceMaxNotSum`, `TestPaceConcurrent`, `TestRetryRead` (maxReadRetries = 2), `TestReadRetriedOnRateLimitThenSucceeds`, `TestFailFastBoundary`, `TestFailFastReadClassifiesRateLimit`, `TestRetryWritePassesThroughOnce`, `TestRetryNeverWrapsWrites`, `TestRateLimitedWriteExitsOne` (429, refused host, fail-fast write -> failed, no audit, exit 1). SDK v7 itself does not retry (`client.go:94 // todo: add retry logic`). Warnings WR-02, WR-03 below. Not seen live: ADO did not throttle the TUI sessions |
| 4 | The PAT is only ever sent to hosts validated against the configured organization; a request aimed at any other host is refused before the credential leaves the process | FAILED | ASCII foreign hosts are refused (TestAllowed, TestRefuseNeverReachesBase, TestRefuseRedirectHopIsNotNotSent, TestNewClientRefusesForeignLocationURL all pass; main.go wires `transport.New(cfg.Org(), ...)`). But the rule is bypassable, reproduced here: `Allowed("https://acme.v\u0130sualstudio.com/x") = true`; the transport passed an Authorization-bearing request to the dialer for `acme.xn--visualstudio-n4g.com:443`; a 307 to that host was forwarded with the write body and Authorization was dropped only by net/http's redirect policy. `dev.azure.com/other%2F..%2Facme/_apis` is also Allowed (WR-01). Today's first-hop URLs are ASCII (SDK normalizeUrl + areaURL lowercase), so no live PAT leak path is known, but the stated guarantee does not hold |
| 5 | One live read per ADO sub-client and one dry run per write verb pass against the real organization, and the results are recorded before Phase 9 starts | FAILED | Reads: met through the TUI, re-checked in the log (Dashboard 7,883/7,885, work items, PRs 12, branches 566, pipeline runs 11, definitions 11, releases 5, approvals 61, all failure_count=0; identity logged, 0 "Could not cache current user ID"; 2x "ADO client created successfully" means the D-10a every-area check passed live, vsrm included). Dry runs: 0 of 6. smoke.sh never ran. No live write. 08-SMOKE.md records "waived by user, not met" |

**Score:** 2/5 roadmap success criteria verified (0 present, behavior-unverified). Plan-level must-have truths: 41/48 verified.

### Plan must-have truths that failed (beyond the roadmap rows above)

| Plan | Truth | Status | Evidence |
|------|-------|--------|----------|
| 08-01 | go.mod at the lowest 1.27 patch for which govulncheck v1.9.0 reports no vulnerabilities | FAILED | 1.27.2 now has a reachable finding; 1.27.3 is the lowest clean patch |
| 08-03 | Org binding: host and path compared case-insensitively ... every other host refused | FAILED | Unicode folding and decoded-path checks admit non-org targets (probe) |
| 08-03 | A request to a host not bound to the org returns an error before the base RoundTripper is called | FAILED | IDN homograph host reached the base RoundTripper and the dialer (probe) |
| 08-08 | Full pasted smoke table with 9 read/identity rows and 6 dry-run rows | FAILED | Script not run; record gate fails |
| 08-08 | CLI set-state --confirm and TUI Task close recorded with observed outcome | FAILED | Not run; audit.jsonl last modified 2026-09-12 |
| 08-08 | CI on the pushed dev branch shows govulncheck passing | FAILED | Not run (and would now fail, see SC1) |
| 08-08 | Provenance, full table and non-pending checklist results | FAILED | Provenance recorded from executor-run `go version -m`; table absent |

All other plan truths (08-01 x7, 08-02 x5, 08-03 x4, 08-04 x5, 08-05 x6, 08-06 x5, 08-07 x7, 08-08 x2) are VERIFIED by passing tests, code reading and the spot-checks below.

### Prohibitions (must_haves.prohibitions, all test-tier)

| Plan | Prohibition | Disposition | Evidence |
|------|-------------|-------------|----------|
| 08-01 | MUST NOT change exit-code contract 0-5 or envelope schema | VERIFIED | 08-cli-tests.sha256 7/7 OK, 07-cli-audit-test.sha256 1/1 OK, dispatch.go constants unchanged |
| 08-02 | MUST NOT build any sub-client or send past discovery on a foreign LocationUrl | VERIFIED | TestNewClientRefusesForeignLocationURL passes; checkAreaHosts runs before areaURL |
| 08-03 | MUST NOT let Authorization reach any host or path not bound to the org, including a same-host other-org redirect | FLAGGED (enforcement incomplete) | Wired for ASCII; IDN homograph hosts pass the rule (Authorization withheld on redirect only by net/http); `%2F..%2F` paths pass the rule on the same host, where net/http does forward Authorization on redirect. Tied to the SC4 gap |
| 08-04 | MUST NOT execute writes from the script or run in CI | VERIFIED | smoke.sh never passes the confirm flag; no CI reference to smoke |
| 08-04 | MUST NOT print the PAT, env, config or headers | VERIFIED | Spot run in an isolated HOME: exit 1, no credential in output |
| 08-05 | MUST NOT re-send, replay or read a request body in the transport | VERIFIED | RoundTrip calls base once; TestPaceConcurrent asserts once per request |
| 08-06 | MUST NOT report a write failed when part of it may have reached ADO | VERIFIED | notSentError.Is matches ErrNotSent on first hop only; guard checks it first; TestRunNotSent, TestRefuseRedirectHopIsNotNotSent |
| 08-07 | MUST NOT automatically re-send a write | VERIFIED | Decorators override reads only; TestRetryWritePassesThroughOnce, TestRetryNeverWrapsWrites (see WR-04) |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `go.mod` | v7 v7.1.0, go 1.27.N, x/sys v0.47.0, no toolchain line | PARTIAL | All present; 1.27.2 is no longer govulncheck-clean |
| `.github/workflows/ci.yml` | go-version-file + govulncheck@v1.9.0 | VERIFIED | Both present, SHA-pinned actions, no credentials |
| `internal/transport/transport.go` | Transport, New, HTTPClient, Allowed, ErrNotSent, ErrRateLimited, pacer, Stats | VERIFIED with defect | Substantive (366 lines) and wired; Allowed has the CR-01/WR-01 holes |
| `internal/ado/client.go` | NewClient(cfg, tr), checkAreaHosts, NewClientWithOptions + WithHTTPClient | VERIFIED | Every sub-client, location included, on tr.HTTPClient(); retry decorators wrap the four narrow clients |
| `internal/ado/retry.go` | retryRead, maxReadRetries = 2, four decorators | VERIFIED | 16 read overrides, 0 write overrides |
| `internal/ado/result.go` | ErrRateLimited branch in classifyFailure | VERIFIED | Line 183 |
| `internal/guard/guard.go` | ErrNotSent before parentErr | VERIFIED | Line 263, first statement of classify |
| `internal/config/config.go` | parseOrgURL, Org(), D-12 in Validate | VERIFIED | Built binary exits 3 with org_url on stderr for http, foreign-host and extra-path URLs |
| `cmd/adoBizInt/throttle.go` | renderThrottleSummary | VERIFIED | Called only in the CLI branch of run() |
| `cmd/adoBizInt/v7auth_test.go` | SC2 proof | VERIFIED (fragile) | Passes once per process; fails -count=2 |
| `cmd/adoBizInt/ratelimit_test.go` | SC3 write proof | VERIFIED | Three rows, exit 1, no audit |
| `scripts/smoke.sh` | Re-runnable live smoke | VERIFIED (never run live) | 251 lines; provenance check works; fails closed with no config |
| `08-SMOKE.md` | Recorded live smoke result | PARTIAL | Honest record of a waived run |
| `08-cli-tests.sha256` | Post-swap hashes of 7 locked files | VERIFIED | 7/7 OK |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| cmd/adoBizInt/main.go | internal/transport | `transport.New(cfg.Org(), cfg.RequestTimeout, http.DefaultTransport)` -> `ado.NewClient(cfg, tr)` | WIRED |
| internal/ado/client.go | transport.HTTPClient | `azuredevops.WithHTTPClient(tr.HTTPClient())` on every NewClientWithOptions | WIRED |
| internal/ado/client.go | transport.Allowed | checkAreaHosts on every discovered LocationUrl before any sub-client | WIRED |
| transport.RoundTrip | base RoundTripper | Allowed -> hold -> base.RoundTrip -> observe | WIRED (Allowed bypassable) |
| transport.ErrRateLimited | ado.classifyFailure | errors.Is through *url.Error | WIRED |
| transport.ErrNotSent | guard.classify | errors.Is as first statement | WIRED |
| client.go | retry.go | retryingWorkItems/Git/Build/Release share one *atomic.Int64 | WIRED |
| main.go | throttle.go | `renderThrottleSummary(stderrWriter, waits, waited, client.FetchRetries())` after dispatch, CLI branch only | WIRED |
| ci.yml | go.mod | `go-version-file: go.mod` | WIRED |
| v7auth_test.go | exit.go | exitCodeFor and writeExitCodeFor(outcomeFailed, err) | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| throttle summary line | waits, waited, retries | tr.Stats() atomics incremented in hold; client.FetchRetries() incremented in retryRead | Yes | FLOWING |
| pacer notBefore | Retry-After / X-RateLimit-Delay | observe(resp.Header) on every response | Yes | FLOWING |
| D-08 rate_limit message | wait, left | notSentError fields set in hold | Yes | FLOWING (IN-02 rounding can print "5s, 5s") |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet | `go build ./... && go vet ./...` | OK | PASS |
| Full suite (run once) | `go test -race -count=1 ./...` | all packages ok | PASS |
| gofmt gate | `gofmt -l .` | empty | PASS |
| govulncheck gate | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | exit 3, GO-2026-6809 crypto/x509@go1.27.2, fixed go1.27.3 | FAIL |
| No b5 import | `rg` for b5 / non-v7 SDK imports; `go list -m all` | none; only v7 v7.1.0 | PASS |
| SC2 proof | `go test -count=1 -run TestV7AuthFailureExitsAuth ./cmd/adoBizInt` | 4/4 PASS | PASS |
| SC2 proof re-run | `go test -count=2 -run TestV7AuthFailureExitsAuth ./cmd/adoBizInt` | 4/4 FAIL on OPTIONS precondition | FAIL (WR-05) |
| SC3 named tests | `go test -run 'TestRetryAfterOn200PacesNextRequest|TestReadRetriedOnRateLimitThenSucceeds|TestFailFastReadClassifiesRateLimit|TestRetryRead$|TestRetryWritePassesThroughOnce|TestRetryNeverWrapsWrites' ./internal/ado` | PASS | PASS |
| CR-01 host bypass | `go test -overlay` probe calling Allowed and HTTPClient().Do | Allowed=true; dialed acme.xn--visualstudio-n4g.com:443 | FAIL |
| CR-01 redirect | overlay probe: 307 dev.azure.com/acme -> acme.v%C4%B0sualstudio.com | second PATCH sent with body, Authorization stripped by net/http only | FAIL |
| WR-01 path | overlay probe | `/other%2F..%2Facme/_apis` Allowed=true, wire path unchanged | FAIL (warning-class) |
| D-12 at startup | built binary, isolated HOME, 3 bad org_urls | exit 3, empty stdout, org_url on stderr, no PAT | PASS |
| smoke.sh fails closed | `bash scripts/smoke.sh` in isolated HOME | exit 1, provenance v7.1.0/go1.27.2 shown, every row exit 3, no PAT | PASS |
| Phase 7 locks | `shasum -a 256 -c` both lock files | 8/8 OK | PASS |
| Atomic swap | `git show --stat 81058bd` | go.mod, go.sum, ci.yml, both v7 tests in one commit | PASS |

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` exist and no plan declares one. The phase's "probe" truths are Go tests, run above. The CR-01/WR-01 probes are verifier-written overlay tests; no repository file was changed.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PLAT-02 | 08-01, 08-08 | v7 v7.1.0, Go 1.27, patched x/sys in one change; CI govulncheck fails on reachable vulns; v7 401 classifies auth | BLOCKED | Swap, x/sys, CI gate and 401 test delivered; build is not govulncheck-clean today; CI never observed. REQUIREMENTS.md: Pending (consistent) |
| PLAT-03 | 08-02, 08-03, 08-05, 08-06, 08-07 | Pacing transport on any status; bounded read retries; writes never retried; PAT only to hosts validated against the org | BLOCKED (partial) | Pacing, retries and never-retried writes verified; the host-validation clause has a reproduced bypass. REQUIREMENTS.md marks PLAT-03 `[x]` Complete -- that is premature and should revert to Pending until CR-01 is fixed |
| PLAT-05 | 08-04, 08-08 | One live read per sub-client and one dry run per write verb pass against the real org | BLOCKED | Reads met via TUI; dry runs 0/6. REQUIREMENTS.md: Pending (consistent) |

Orphaned requirements: none. REQUIREMENTS.md maps exactly PLAT-02, PLAT-03 and PLAT-05 to Phase 8, and every one is claimed by at least one plan.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (45 Go/yml/sh files changed in phase) | - | TBD / FIXME / XXX | none found | - |
| (same) | - | TODO / HACK / PLACEHOLDER / "not implemented" | none found | - |
| internal/transport/transport.go | 110-113 | Security decision on strings.ToLower of an untrusted, possibly non-ASCII host and on a decoded path | Blocker | SC4 (CR-01, WR-01) |
| internal/transport/transport.go | 180-189 | Hold "fits" when wait < left by any margin; hold and send share one budget | Warning | WR-02: a throttled read can surface as timeout, a never-sent write as unknown -- the outcome D-01 exists to prevent. Pinned as correct by TestFailFastBoundary "plus 1ns" |
| internal/transport/transport.go | 248-283 | No cap on a server-supplied wait; notBefore never decays | Warning | WR-03: with request_timeout 0 (accepted by Validate) startup can sleep for any server-chosen time with nothing on stderr; in a long TUI session one huge value fails every request as rate_limit until restart |
| internal/ado/retry_test.go | 25-31, 288-297 | Unlisted interface methods are assumed to be reads that must be retried | Warning | WR-04: a future write added without updating writeMethods fails the test in a way that invites a retrying override |
| cmd/adoBizInt/v7auth_test.go | 78-112 | Per-name org plus exact OPTIONS assertion against a process-global cache | Warning | WR-05: SC2 proof fails under -count=2 |
| internal/ado/retry.go | 52-66 | 429 with no pacing header retried back to back | Info | IN-04 |
| internal/transport/transport.go | 344, 352 | Rounding can print "asked for 5s, 5s left" | Info | IN-02 |
| internal/transport/transport.go | whole file | 366 lines vs 300-line convention | Info | Logged in deferred-items.md |

### Human Verification Required

Not the routing status (gaps take precedence), but these remain open and only a human against the real org can close them:

1. **Live smoke run** -- run `scripts/smoke.sh` with SMOKE_* targets; expect 9 read/identity rows PASS and 6 dry-run rows PASS or NODIFF; why human: needs the real org and a PAT, never in CI.
2. **Live writes on v7** -- checklist 1 (CLI set-state --confirm with project in the PATCH route) and 2 (TUI two-PATCH Task close, 07-REVIEW IN-01); expect executed outcomes and audit records; why human: no write has crossed the v7 wire.
3. **TUI attachment download under the host rule** -- checklist 3; expect the file saved and no refusal; why human: 08-RESEARCH A3 and 08-REVIEW IN-05 (an off-org redirect would now be refused).
4. **CI on push** -- after the go 1.27.3 bump, push dev and confirm the govulncheck step is green.

### Override suggestion (developer decision, not applied)

SC5 failed because it was waived, not because the code is wrong. If the developer accepts entering Phase 9 with zero dry runs and zero writes proven on v7, add this to the frontmatter and re-run verification. The verifier does not recommend it: PITFALLS 13 names the live smoke as the only gate that sees the 5.1 to 7.1 API-version change, and Phase 9 builds on it.

```yaml
overrides:
  - must_have: "One live read per ADO sub-client and one dry run per write verb pass against the real organization, and the results are recorded before Phase 9 starts"
    reason: "Live reads passed for every sub-client with a call site via two TUI sessions on the v7.1.0/go1.27.2 build (08-SMOKE.md); dry runs and live writes deferred, carried in BACKLOG.md BL-02"
    accepted_by: "{developer}"
    accepted_at: "{ISO timestamp}"
```

No override is appropriate for SC1 or SC4: both are fixable code defects (one go.mod line; a few lines in Allowed plus test rows), which is exactly the "implementation is simply incomplete -- fix it instead" case.

### Context the developer should weigh

- Phase 7 is still `human_needed` (07-VERIFICATION.md, 9 pending UAT rows). The 08-01 and 08-08 preconditions were overridden by the user. Phase 8 does not inherit a Phase 7 gap directly, but SC5 was meant to exercise the Phase 7 TUI guard path live and did not.
- 08-REVIEW.md's claims were checked independently: CR-01 reproduced (host and redirect), WR-01 reproduced, WR-02 and WR-03 confirmed by code reading and the pinned boundary test, WR-05 reproduced. The review's reachability note is also correct: today's first-hop URLs are ASCII, so this is a broken boundary, not an observed leak.
- The 08-08 SUMMARY is accurate and does not overclaim. The govulncheck regression is new since its Task 1 run (go1.27.3 shipped 2026-10-19).

### Gaps Summary

Three gaps, two root causes plus one decision:

1. **Toolchain (SC1, PLAT-02):** the build fails the phase's own govulncheck gate today. One-line fix: `go 1.27.3`. Then observe CI green on push.
2. **Host rule (SC4, PLAT-03):** `transport.Allowed` decides on a Unicode-folded host and a decoded path. Refuse non-ASCII, decide on the escaped path, add the five TestAllowed rows and a redirect-hop test. Revert PLAT-03 to Pending in REQUIREMENTS.md until then.
3. **Live smoke (SC5, PLAT-05):** run it, or record an explicit override. Running it is cheap now that the harness is proven to work, and it is the only check that covers writes on v7.

Warnings worth fixing in the same gap plan: WR-02 (send headroom after a hold), WR-03 (cap a single instruction, reject request_timeout 0 or bound startup), WR-05 (unique org per invocation), WR-04 (classify interface methods both ways).

---

_Verified: 2026-10-21T17:22:47Z_
_Verifier: the agent (gsd-verifier)_
