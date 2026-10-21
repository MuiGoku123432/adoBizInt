---
phase: 08-platform-upgrade-and-pacing
verified: 2026-10-21T21:29:15Z
status: passed
score: 5/5 roadmap success criteria (3 verified outright; SC1 and SC5 PASSED via override for the parts only the user can prove); plan must-have truths 64/74 verified outright, 74/74 with overrides
behavior_unverified: 0
overrides_applied: 2
overrides:
  - must_have: "CI runs `govulncheck` and fails on any reachable vulnerability; the upgraded build (`azuredevops/v7` v7.1.0, Go 1.27, patched `golang.org/x/sys`) passes it, and no `v1.0.0-b5` import remains"
    reason: "Override covers ONLY the 'in CI' half. Proven in this re-verification: go.mod declares go 1.27.3 with no toolchain line and no other module change since the gap plans; go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... scanned 28 modules and the go1.27.3 standard library and printed 'No vulnerabilities found.' (exit 0; GO-2026-6809 closed); .github/workflows/ci.yml runs that exact command after actions/setup-go with go-version-file: go.mod; 0 v1.0.0-b5 imports, 31 files import azuredevops/v7, go list -m all shows only azuredevops/v7 v7.1.0 and golang.org/x/sys v0.47.0; a built binary reports go1.27.3, v7.1.0, x/sys v0.47.0. Not proven: a CI run on push. dev was never pushed (local origin/dev ref dated 2026-10-12, 152 commits behind HEAD), so no CI run URL or conclusion exists. User, 2026-10-21, declining the 08-12 push and CI check, verbatim: 'I am willing to move to phase 9 as is'. Earlier waiver on the same item (08-08), verbatim: 'lets just move on'."
    accepted_by: "user"
    accepted_at: "2026-10-21"
  - must_have: "One live read per ADO sub-client and one dry run per write verb pass against the real organization, and the results are recorded before Phase 9 starts"
    reason: "Override covers the dry-run and live-write half, and the re-run of reads on the final build. Proven: two TUI sessions on 2026-10-21 on the azuredevops/v7 v7.1.0, go1.27.2 build logged failure_count=0 for every workitemtracking, git, build and release (vsrm host) read, identity logged at construction, 0 'Could not cache current user ID', 0 ERROR, 0 host-rule refusals, 2 'ADO client created successfully' (re-checked in ~/.config/adoBizInt/adoBizInt.log, 168 lines dated 2026-10-21, log last written before 08-09 started). Not proven: those reads predate 08-09 (go1.27.3), 08-10 (hardened host rule) and 08-11 (pacer send budget and hold cap), so no live read has run on the final build; 0 of 6 write-verb dry runs on any v7 build; scripts/smoke.sh never run against the org; no live write on v7 (audit.jsonl mtime 2026-09-12); checklist 1 (CLI set-state --confirm), 2 (TUI two-PATCH close), 3 (TUI attachment download under the hardened host rule) and 5 (confirm-modal dismissals) not run; the 08-08 record gate fails. 08-SMOKE.md records all of this as waived, not met. User, 2026-10-21, declining the 08-12 re-run, verbatim: 'I am willing to move to phase 9 as is'. Earlier waiver (08-08), verbatim: 'lets just move on'."
    accepted_by: "user"
    accepted_at: "2026-10-21"
re_verification:
  previous_status: gaps_found
  previous_score: 2/5
  gaps_closed:
    - "SC1 (code half): go directive raised to 1.27.3 by 08-09; govulncheck v1.9.0 clean, GO-2026-6809 closed. CI-on-push half accepted by override"
    - "SC4: transport.Allowed refuses non-ASCII hosts before folding (CR-01) and decides the dev.azure.com org on the escaped wire path, refusing encoded separators, encoded dots, dot and empty segments and opaque URLs (WR-01); parseOrgURL refuses non-ASCII, fragment markers, dot-segment and non-canonical orgs (IN-01). 08-10"
    - "SC5: accepted by override; dry runs and live writes still not run"
    - "Warnings WR-02 (2s send budget after a hold), WR-03 (5m hold cap), WR-04 (two-sided read/write classification), WR-05 (SC2 proof repeatable under -count) and IN-02 (message rounding) fixed in 08-09 and 08-11"
  gaps_remaining: []
  regressions: []
---

# Phase 8: Platform Upgrade and Pacing Verification Report

**Phase Goal:** The tool runs on a supported SDK and toolchain with no reachable vulnerabilities, and slows down when Azure DevOps signals throttling without ever retrying a write
**Verified:** 2026-10-21T21:29:15Z
**Status:** passed (with 2 developer overrides)
**Re-verification:** Yes -- after gap closure 08-09 to 08-12. Previous result: gaps_found, 2/5 (SC1, SC4, SC5 failed).

## Bottom line

- Every criterion that can be proven in code now passes my own checks:
  - SC1 code half: govulncheck v1.9.0 is clean on go 1.27.3, re-run here.
  - SC2: the v7 401 proof passes under `-count=3`.
  - SC3: the pacing and retry behaviour tests pass.
  - SC4: my own probe of the host rule passes 49 of 49 rows, plus first-hop and redirect-hop wire probes.
- The full suite passes under `go test -race -count=2 ./...`.
- Two items are accepted by the user's explicit override, not proven:
  - SC1: no CI run on a pushed branch has been observed.
  - SC5: 0 of 6 dry runs and 0 live writes. The live reads that did run were on the go1.27.2 build, before the host-rule and pacer hardening.
- No defect found. The four Info items in the re-review (IN-07 to IN-10) do not break SC4. Two of them (IN-09, IN-05) are Phase 9 risks; see the Phase 9 risks section.

## What changed since the previous verification

| Previous gap | Fix | Evidence in this re-verification |
|--------------|-----|----------------------------------|
| SC1: govulncheck exit 3 on go1.27.2 (GO-2026-6809) | 08-09 `b9b3cfe`: `go 1.27.3`, go.sum untouched | `govulncheck@v1.9.0 -show verbose ./...`: "scanned ... the go1.27.3 standard library", "No vulnerabilities found.", exit 0. `git diff 172cec8..HEAD -- go.mod` shows only the go line |
| SC1: CI on push never observed | Not fixed; user declined | Override 1 |
| SC4: Unicode-folding host bypass (CR-01) | 08-10 `172bcc7`: `asciiLower` before fold | Probe: `acme.v\u0130sualstudio.com`, `acme.v%C4%B0sualstudio.com`, `\u212Acme.visualstudio.com` (org kcme), punycode form, Cyrillic `d\u0435v.azure.com` all refused. The first hop to the homograph never reached the base RoundTripper (ErrNotSent). A 307 to it was refused by the transport with the base called once |
| SC4: decoded-path compare (WR-01) | 08-10 `0e40c3c`: compare on `u.EscapedPath()` | Probe: `other%2F..%2Facme`, `other/../acme`, `acme/../other`, `acme/%2e%2e/other`, `acme%2F`, `acme%5C`, `%61cme`, `//acme`, `/./acme`, `acme;x` all refused, first hop and redirect hop |
| SC4: org_url accepted non-ASCII (CR-01 config half, IN-01) | 08-10 `3b87cf2` | Probe: 25 parseOrgURL rows as intended. Built go1.27.3 binary exits 3 naming org_url for 4 bad org_urls, with empty stdout and the PAT absent from stdout and stderr |
| SC5: dry runs 0/6, no live write | Not fixed; user declined the 08-12 re-run | Override 2 |
| WR-05: SC2 proof failed under `-count=2` | 08-09 `c5ff75a`: per-invocation org | `-count=3`: 3/3 PASS for the parent test and both classify tests |
| WR-02, WR-03, WR-04, IN-02 | 08-09 `fea5a9c`, 08-11 `01a8311`, `9872d78` | TestFailFastBoundary, TestHoldCap, TestFailFastMessageRounding, TestRetryNeverWrapsWrites PASS |

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | CI runs govulncheck and fails on any reachable vulnerability; the upgraded build (v7.1.0, Go 1.27, patched x/sys) passes it; no v1.0.0-b5 import remains | VERIFIED (code half) + PASSED (override, CI-on-push half) | **Gate:** ci.yml runs `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` after `setup-go` with `go-version-file: go.mod`. A govulncheck finding exits non-zero; the previous verification saw exit 3 on go1.27.2. **Passes:** re-run here on go1.27.3, "No vulnerabilities found.", exit 0. **No b5:** 0 imports; 31 files import `/v7`; `go list -m all` lists only `azuredevops/v7 v7.1.0` and `x/sys v0.47.0`. **Toolchain:** `go version -m` on a fresh build shows go1.27.3. **Not observed:** CI on push. Override 1 |
| 2 | A rejected PAT under v7 still exits with the auth-failure code, proven by a test | VERIFIED | `TestV7AuthFailureExitsAuth` (`cmd/adoBizInt/v7auth_test.go:50`): a real v7 ClientImpl gets a 401 in the JSON-body and empty-body shapes, on the read and write paths. The test asserts the exact OPTIONS-then-data-route sequence, and that both `exitCodeFor` and `writeExitCodeFor(outcomeFailed, err)` return `exitAuth` (2, `dispatch.go:25`). `-count=3`: PASS 3/3, as are `TestV7ReadAuthFailureClassifiesAuth` and `TestV7WriteAuthFailureClassifiesAuth` |
| 3 | After Retry-After or rate-limit headers, including on HTTP 200, later requests wait as instructed; a rate-limited read retries a bounded number of times within its deadline; a rate-limited write is never retried and exits with the write-failure code | VERIFIED | Behavioural tests, run by name and passing: `TestRetryAfterOn200PacesNextRequest` (real v7 client, 200 + Retry-After 7, next send exactly 7s later), the 8 `TestPace*` tests, `TestRetryRead` (`maxReadRetries = 2`; `retryRead` stops on `ctx.Err()`), `TestReadRetriedOnRateLimitThenSucceeds`, `TestFailFastBoundary` (2s send budget), `TestHoldCap` (5m cap), `TestFailFastReadClassifiesRateLimit`, `TestRetryWritePassesThroughOnce`, `TestRetryNeverWrapsWrites` (16 reads, 5 writes, each method in exactly one list), `TestRateLimitedWriteExitsOne` (429, refused-host and fail-fast rows: Mutate runs once, outcome failed, no audit, exit 1 = `exitPartial`), `TestRunNotSent`. The transport calls base once per request (no replay). ADO has never throttled a live v7 run, so this rests on tests, which is what the criterion asks |
| 4 | The PAT is only ever sent to hosts validated against the configured organization; a request aimed at any other host is refused before the credential leaves the process | VERIFIED | **Rule:** `transport.Allowed` (`transport.go:137-167`): https only, no userinfo, no port, no opaque part; non-ASCII host refused before folding; dev.azure.com and *.dev.azure.com bound to `/{PathEscape(org)}` on the escaped path; non-canonical paths refused, not normalised. **Wiring:** `RoundTrip` checks every request and redirect hop before `base` (`:193`); `checkAreaHosts` checks every discovered LocationUrl at startup (`client.go:219`); `main.go:115` binds `cfg.Org()`. Every SDK client is built with `NewClientWithOptions(..., WithHTTPClient(tr.HTTPClient()))`; only transport.go and main.go import net/http. **My probe** (overlay test, no repo change): 49/49 Allowed rows as SC4 requires, 4 first-hop refusals never reached base (ErrNotSent), 6 redirect-hop refusals with base called once (not ErrNotSent), legitimate same-org redirect still works. Repo tests `TestAllowed`, `TestRefuseNeverReachesBase`, `TestRefuseHomographNeverReachesBase`, `TestRefuseRedirectHopIsNotNotSent` and `TestNewClientRefusesForeignLocationURL` pass. Residuals assessed below; none breaks SC4 |
| 5 | One live read per ADO sub-client and one dry run per write verb pass against the real organization, and the results are recorded before Phase 9 starts | PASSED (override) | **Reads:** observed 2026-10-21 on the go1.27.2 build (log re-checked: 11 x failure_count=0, identity logged, 0 refusals, 0 ERROR); not repeated on the go1.27.3, hardened build. **Dry runs:** 0/6. **Live writes:** none on v7. **Record:** 08-SMOKE.md records both as waived, not met, honestly. Override 2 |

**Score:** 5/5 (3 VERIFIED, 2 PASSED via override; 0 present but behavior-unverified).

### Plan must-have truths (74 across 12 plans)

| Plans | Verified outright | Override-covered | Notes |
|-------|-------------------|------------------|-------|
| 08-01 to 08-07 | 39 | 0 | The previously failed 08-01 "lowest govulncheck-clean 1.27 patch" truth and the two 08-03 host-rule truths now pass |
| 08-08 | 5 | 4 | Full smoke table, live set-state and Task close, CI on pushed dev, provenance and table: subsumed by SC5 and SC1 overrides |
| 08-09 | 7 | 0 | 1.27.3 clean; full gate; CI and smoke.sh follow go.mod (`go-version-file: go.mod`, `GOTOOLCHAIN=go${want_go}`); PROJECT.md says Go 1.27.3; SC2 under `-count=3`; `-count=2 ./...`; two-sided classification |
| 08-10 | 5 | 0 | Non-ASCII host refusal; homograph first-hop and redirect-hop; escaped-path binding; legitimate URLs pass (Caf%C3%A9, My%20Project, trailing slash, vsrm, upper case); org_url refusals at startup, exit 3 |
| 08-11 | 7 | 0 | `minSendBudget = 2 * time.Second`, `maxHold = 5 * time.Minute`, boundary and cap tests, rounding, unchanged pacing tests, `request_timeout: 0` still valid, deferrals recorded |
| 08-12 | 1 | 6 | "Executor never runs git push" verified: the local origin/dev ref is still dated 2026-10-12 and the user confirms no push. The other six are the waived re-run (provenance, script table, checklist a/b/c/e, refused-host classification, CI row, record gate): subsumed by the SC5 and SC1 overrides |
| **Total** | **64** | **10** | 74/74 with overrides |

### Prohibitions (gap plans; earlier plans regression-checked)

| Plan | Prohibition | Disposition | Evidence |
|------|-------------|-------------|----------|
| 08-09 | No toolchain directive, no module change | VERIFIED | go.mod diff is the go line only; go.sum unchanged; `go mod tidy -diff` clean |
| 08-09 | No edit to locked files | VERIFIED | 08-cli-tests.sha256 7/7 OK, 07-cli-audit-test.sha256 1/1 OK |
| 08-09 | SC2 proof not weakened | VERIFIED | OPTIONS-then-data-route assertion and both exitAuth checks present on all 4 rows |
| 08-10 | Authorization never reaches a non-org host or path on any hop | VERIFIED (residual accepted) | Tests plus my probe. Residual: paths that name another org only after a second decode or a non-RFC-3986 normalisation (IN-07). These are not SDK-generated and need a same-host redirect plus non-standard server behaviour |
| 08-10 | Never normalise into an allowed form | VERIFIED | `path.Clean` used only as an equality test; non-canonical paths refused |
| 08-10 | Non-ASCII names after the org not refused | VERIFIED | `Caf%C3%A9` rows allowed on dev.azure.com and the legacy host |
| 08-11 | Never send before the instructed time | VERIFIED | `observe` stores the instruction as sent; a wait over the cap is refused, never clamped (TestHoldCap 301s, year 9999) |
| 08-11 | No retry or replay in the transport | VERIFIED | `RoundTrip` calls base once |
| 08-11 | Exit codes, envelope, Failure schema unchanged | VERIFIED | `dispatch.go` constants 0 to 5 unchanged; lock hashes OK |
| 08-12 | Executor never pushes | VERIFIED (judgment) | origin/dev ref 2026-10-12, 152 commits behind HEAD; user confirms no push |
| 08-12 | No drafted waiver | VERIFIED | 0 `- waiver:` lines; "No waivers recorded." |
| 08-12 | No credential in 08-SMOKE.md | VERIFIED | No Authorization or Basic-token line |
| 08-01 to 08-07 | Previously verified prohibitions | VERIFIED (regression) | Full suite green; the 08-03 prohibition flagged last time is now enforced (above) |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `go.mod` | v7 v7.1.0, go 1.27.N clean, x/sys v0.47.0, no toolchain line | VERIFIED | `go 1.27.3` |
| `.github/workflows/ci.yml` | go-version-file + govulncheck@v1.9.0 | VERIFIED | Unchanged; SHA-pinned actions |
| `internal/transport/transport.go` | Hardened Allowed, asciiLower, minSendBudget, maxHold | VERIFIED | 440 lines (over the 300-line convention; recorded in deferred-items.md) |
| `internal/transport/transport_test.go` | CR-01, WR-01, budget, cap, rounding tests | VERIFIED | Contains `other%2F..%2Facme`, `TestHoldCap`, `TestRefuseHomographNeverReachesBase` |
| `internal/config/config.go` | parseOrgURL hardening | VERIFIED | `isASCII`, fragment, dot-segment and PathEscape-stability checks |
| `cmd/adoBizInt/v7auth_test.go` | Repeatable SC2 proof | VERIFIED | `v7AuthSeq atomic.Int64` |
| `internal/ado/retry_test.go` | Two-sided classification | VERIFIED | `readMethods` (16), `writeMethods` (5) |
| `deferred-items.md` | Deferrals with reasons | VERIFIED with stale row | The IN-05 row says "checked live by 08-12 item c"; that check was waived (IN-08) |
| `08-SMOKE.md` | Recorded smoke | PASSED (override) | Honest waived record |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `transport.RoundTrip` | `Allowed` | `if !t.Allowed(req.URL)` before hold and base | WIRED |
| `ado.checkAreaHosts` | `Allowed` | `tr.Allowed(u)` on every LocationUrl | WIRED |
| `main.go` | `transport.New` | `transport.New(cfg.Org(), cfg.RequestTimeout, http.DefaultTransport)` | WIRED |
| `ado.NewClient` | `tr.HTTPClient()` | `WithHTTPClient` on root, 5 area clients and location | WIRED |
| `transport.hold` | `notSentError` | `reason: reasonRateLimited`, wait and floored budget | WIRED |
| `ErrRateLimited` | `ado.classifyFailure` | `errors.Is` (`result.go:183`) | WIRED |
| `ErrNotSent` | `guard.classify` | `errors.Is` first (`guard.go:263`) | WIRED |
| `ci.yml` | `go.mod` | `go-version-file: go.mod` | WIRED |
| `scripts/smoke.sh` | `go.mod` | `want_go` and `GOTOOLCHAIN=go${want_go}` | WIRED |
| `retry_test.go` | `interfaces.go` | go/ast parse; `readMethods[`/`writeMethods[` | WIRED |
| `main.go` | `throttle.go` | `renderThrottleSummary(..., client.FetchRetries())` | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| Throttle summary line | waits, waited, retries | `tr.Stats()` atomics in `hold`; `retryRead` counter | Yes | FLOWING |
| Pacer `notBefore` | Retry-After, X-RateLimit-Delay | `observe(resp.Header)` on every response | Yes | FLOWING |
| D-08 rate_limit message | wait, left | `notSentError` fields set in `hold`, rounded in `Error()` | Yes | FLOWING |
| Host-rule org | `cfg.Org()` | `parseOrgURL(cfg.OrgURL)` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Toolchain | `go version` | go1.27.3 darwin/arm64 | PASS |
| Build, vet, fmt, tidy | `go build ./...`, `go vet ./...`, `gofmt -l .`, `go mod tidy -diff` | OK, OK, empty, no diff | PASS |
| govulncheck | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` (and `-show verbose`) | "No vulnerabilities found.", exit 0, go1.27.3 stdlib + 28 modules | PASS |
| Full suite (once) | `go test -race -count=2 ./...` | exit 0, every package ok | PASS |
| SC2 repeatable | `go test -count=3 -run 'TestV7AuthFailureExitsAuth\|TestV7ReadAuthFailureClassifiesAuth\|TestV7WriteAuthFailureClassifiesAuth' ./cmd/adoBizInt ./internal/ado` | 3/3 PASS each | PASS |
| SC3 named tests | `go test -run '...'` across internal/ado, internal/transport, internal/guard, cmd/adoBizInt | all PASS | PASS |
| SC4 rule probe | `go test -overlay` probe calling `Allowed` (49 rows) | 49/49 as SC4 requires | PASS |
| SC4 wire probe | overlay probe through `HTTPClient().Do` with a recording base | 4 first hops: base 0, ErrNotSent; 6 redirect hops: base 1, refused, not ErrNotSent; same-org redirect: base 2, 200 | PASS |
| org_url probe | overlay probe on `parseOrgURL` (25 rows) | 25/25 | PASS |
| D-12 at startup | built binary, isolated HOME, 4 bad org_urls | exit 3, empty stdout, "org_url" on stderr, PAT absent | PASS |
| Binary provenance | `go version -m` on the built binary | go1.27.3, azuredevops/v7 v7.1.0, x/sys v0.47.0 | PASS |
| Locks | `shasum -a 256 -c` both lock files | 8/8 OK | PASS |
| Live read log | grep of `~/.config/adoBizInt/adoBizInt.log` for 2026-10-21 | 11 x failure_count=0; 0 refusals; 0 ERROR; 0 identity-cache warnings | PASS (go1.27.2 build only) |

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` exists and no plan declares one. The host-rule and org_url probes above are verifier-written `go test -overlay` files in the session temp directory; no repository file was changed.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| PLAT-02 | 08-01, 08-08, 08-09, 08-12 | v7 v7.1.0, Go 1.27, patched x/sys in one change; CI govulncheck fails on reachable vulns; v7 401 classifies auth | SATISFIED (CI observation by override) | Atomic swap (08-01), go 1.27.3 clean, CI gate configured, SC2 test repeatable. REQUIREMENTS.md: Pending. Recommendation: mark Complete with a note that the CI run on push is accepted by override |
| PLAT-03 | 08-02, 08-03, 08-05, 08-06, 08-07, 08-09, 08-10, 08-11 | Pacing on any status; bounded read retries; writes never retried; PAT only to org-validated hosts | SATISFIED | All code truths verified, host-rule bypass closed. REQUIREMENTS.md: Complete, now correct (it was premature at the previous verification) |
| PLAT-05 | 08-04, 08-08, 08-12 | One live read per sub-client and one dry run per write verb against the real org | ACCEPTED BY OVERRIDE, NOT MET | Reads on go1.27.2 only; dry runs 0/6; harness exists and fails closed. REQUIREMENTS.md: Pending. Recommendation: if marked Complete, annotate "accepted by user override 2026-10-21; dry runs and live writes not run (BACKLOG BL-02)". Plain Complete would overstate it |

Orphaned requirements: none. REQUIREMENTS.md maps exactly PLAT-02, PLAT-03 and PLAT-05 to Phase 8, and each is claimed by at least one plan.

### Re-review Info items (08-REVIEW.md, IN-07 to IN-10) against SC4

I reproduced each with my own probe.

| ID | What I observed | Breaks SC4? | Why |
|----|-----------------|-------------|-----|
| IN-07 | `/acme/%252e%252e/other`, `/acme/x%252f..%252f..%252fother` and `/acme/..;/other` (org acme) are allowed | No | Every one stays on dev.azure.com and under `/acme/` under single RFC 3986 decoding. Each names another org only if ADO's front end decodes twice or strips `;` parameters before resolving dot segments. The SDK builds every route with one `url.PathEscape`, so these only arrive through a same-host redirect from ADO itself or a server-supplied URL; no code path follows server links. deferred-items.md records only `%252e`; the accepted class is wider. Widen the record, or refuse the cheap subset (review suggestion) |
| IN-08 | deferred-items.md row IN-05 says "checked live by 08-12 checklist item c" | No (doc accuracy) | Item c was waived. BACKLOG BL-02 has the true state. Correct the row so the attachment-host question does not read as answered |
| IN-09 | `.../classificationnodes/Areas/Team%20A%2FSub` is refused, and reported as a refused host | No: over-refusal is fail-safe | A refusal never leaks the credential. Today no call site passes a wildcard `Path` (`GetRecentIterations` passes none, `client.go:280`). It is a Phase 9/10 functional risk and a misleading message ("refused to send credentials to host dev.azure.com") |
| IN-10 | `https://dev.azure.com/a.b` parses to org `a.b`, which then admits `a.b.visualstudio.com` and `a.b.vsrm.visualstudio.com` | No | ADO org names cannot contain dots, so this config never works: the first request goes to the configured `dev.azure.com/a.b` and discovery fails there. Any admitted host is still a Microsoft-owned `*.visualstudio.com` host, reached only through an ADO redirect. A typo'd org_url naming another real org exposes the PAT just as much, by design. Cheap fix: refuse `.` in the dev.azure.com org segment |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| 7 files changed by 08-09 to 08-11 | - | TBD / FIXME / XXX | none | - |
| same | - | TODO / HACK / PLACEHOLDER / "not yet implemented" | none | - |
| `internal/transport/transport.go` | 193-197, 427 | A path refusal is logged and returned as a host refusal | Info | IN-09; misleading diagnosis in Phase 9 |
| `internal/transport/transport.go` | whole file | 440 lines vs 300-line convention | Info | Recorded in deferred-items.md (pace.go split) |
| `internal/ado/retry_test.go` | whole file | 407 lines | Info | Noted in 08-09 SUMMARY |
| `.planning/phases/08-platform-upgrade-and-pacing/deferred-items.md` | 15, 28 | Accepted-risk record narrower than the accepted class; stale IN-05 row | Info | IN-07, IN-08 |

### Human Verification Required

None outside the overrides. Every live item still open was declined by the user under "I am willing to move to phase 9 as is" and is covered by Override 1 or 2:

- push dev and observe the govulncheck step green in CI;
- `scripts/smoke.sh` on the go1.27.3 build (8 reads, identity, 6 dry runs);
- checklist 1 (CLI set-state, then the confirm flag), 2 (TUI two-PATCH Task close), 3 (TUI attachment download under the hardened host rule) and 5 (confirm-modal dismissals).

They stay open in BACKLOG.md BL-02. They are not routed to UAT by this verification.

### Phase 9 risks

1. **Nothing hardened in 08-10 or 08-11 has met real ADO.**
   - No live read has run on the go1.27.3 build. The stricter host rule's first live contact will be Phase 9's first run.
   - Offline, the rule admits `dev.azure.com/Windstream`, `vsrm.dev.azure.com/Windstream` and `vssps.dev.azure.com/Windstream`.
   - Make the first Phase 9 live run double as the smoke: `scripts/smoke.sh` plus checklist 3. Treat any refused host or path as a host-rule regression.
2. **No write has crossed the v7 wire** (audit.jsonl mtime 2026-09-12).
   - Not exercised live: the 5.1 to 7.1 API-version change for PATCH, PR complete, vsrm approvals and pipeline queue.
   - Not exercised live: the TUI two-PATCH close (07-REVIEW IN-01).
   - Phase 9 is read-only per its roadmap. The first later phase that writes inherits this unproven path.
3. **IN-09 wildcard routes.** Any Phase 9/10 read that passes a nested area or iteration `Path`, such as classification nodes for include-children area intersection (TEAM-06), will be refused, with a message blaming the host. Fix before that read lands: report which check refused (host or path), and decide whether `%2F` in a known wildcard route value is admissible.
4. **New sub-clients must stay on the transport.** Phase 9 needs teams (`core`) and team settings and iterations (`work`, currently dormant with no live read). A `core.NewClient(ctx, conn)` or `GetClientByUrl` call would build a default http.Client that bypasses the host rule and the pacer. Build it with `NewClientWithOptions(conn, areaURL, WithHTTPClient(tr.HTTPClient()))`. `checkAreaHosts` already covers the core area's LocationUrl.
5. **IN-05 attachment host.** An attachment served from a host outside the org is now refused as a foreign redirect hop. It has never run under the hardened rule, and the deferred-items row wrongly says it was checked.
6. **The pacer refuses waits it used to honour.** At the 30s default `request_timeout`, a Retry-After of 29s or 30s now fails fast as `rate_limit` ("ADO asked for 29s, 28s left in budget") instead of being held. An instruction over 5m fails every request until it passes or the process restarts, because `notBefore` never decays (accepted under D-04). Long Phase 9 fan-out reads under throttling will surface this as rate_limit failures, not delays.
7. **IN-10 dotted org.** A one-line hardening; there is no live risk.

### Context the developer should weigh

- Phase 7 is still `human_needed` (07-VERIFICATION.md, 14/15, 9 pending UAT rows). The user overrode the Phase 7 precondition for 08-01, 08-08 and 08-12. This is not a Phase 8 gap. SC5 was also meant to exercise the Phase 7 TUI guard path live, and that has not happened either.
- 08-REVIEW.md's re-review claims hold up: CR-01, WR-01 to WR-05, IN-01 and IN-02 are closed in code and tests. Its IN-07 to IN-10 probes reproduce exactly.
- The 08-12 SUMMARY is accurate and does not overclaim. It reports its own failing gates, the waiver and the stale-text corrections.

### Gaps Summary

No open gaps. All three previous gaps are resolved:

- SC1: the code half is fixed and re-verified; the CI-on-push half is accepted by override.
- SC4: fixed and re-verified by an independent probe.
- SC5: accepted by override. The dry runs and live writes were not run.

The phase goal holds in code:
- supported SDK and toolchain with no reachable vulnerabilities, per govulncheck on go1.27.3;
- the tool slows down on throttling signals, any status included;
- writes are never retried;
- the PAT is bound to the organization.

What the overrides leave unproven is live behaviour: CI on push, writes on v7, and the hardened build against the real org.

---

_Verified: 2026-10-21T21:29:15Z_
_Verifier: the agent (gsd-verifier)_
