---
phase: 08-platform-upgrade-and-pacing
plan: 02
subsystem: api
tags: [go, azuredevops-v7, transport, host-trust, pat, plat-03, d-09, d-10a, d-11]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-01: azuredevops/v7 v7.1.0 on Go 1.27.2 and the real-SDK test harness in internal/ado/v7_test.go (rtFunc, jsonResp, v7Locations, withLocations, uniqueOrgURL)"
provides:
  - "internal/transport: Transport (http.RoundTripper), New, HTTPClient, Allowed (the one D-09 host rule), ErrNotSent, unexported notSentError"
  - "config.parseOrgURL (unexported, D-12 shape parse) and (*Config).Org() returning the lowercase {org}"
  - "ado.NewClient(cfg, tr): discovery on the injected client, the D-10a every-area host sweep (checkAreaHosts), explicit NewClientWithOptions construction of wit/git/build/release/work/location (areaURL)"
  - "main builds transport.New(cfg.Org(), cfg.RequestTimeout, http.DefaultTransport) and passes it to ado.NewClient"
  - "TestSubClientsUseInjectedClient (D-11 go/ast structural test) and the NewClient fake-base fixture (v7Area, usedAreas, resourceAreasFixture, newClientBase) in v7_test.go"
affects: [08-03, 08-05, 08-06, 08-07, 08-08, phase-09]

# Actuals (#2632)
actuals:
  tokens: 8291
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Every SDK sub-client is built as &X.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, url, opt)} with opt := azuredevops.WithHTTPClient(tr.HTTPClient()); typed X.NewClient, location.NewClient and GetClientByUrl are forbidden structurally"
    - "internal/ado reaches *http.Client only by type inference from tr.HTTPClient(); it never imports net/http"
    - "NewClient tests drive the real constructor over newClientBase, which records method, host, path, Authorization presence and the request-context deadline of every request"
    - "Discovered LocationUrls are normalized as the SDK does (ToLower(TrimRight(u, \"/\"))) before both the host check and sub-client construction"

key-files:
  created:
    - internal/transport/transport.go
    - internal/ado/construct_test.go
    - .planning/phases/08-platform-upgrade-and-pacing/deferred-items.md
  modified:
    - internal/ado/client.go
    - internal/ado/client_test.go
    - internal/ado/v7_test.go
    - internal/ado/interfaces.go
    - internal/config/config.go
    - cmd/adoBizInt/main.go

key-decisions:
  - "D-10a implemented over every discovered area, not only the five used ones (user confirmation 2026-10-19); an entry with no id or no location is skipped because it names no host"
  - "transport.New lowercases org and Allowed fails closed on an empty org, so a misparsed org_url can never admit the root of dev.azure.com"
  - "notSentError carries the org as well as host and hop, because its Error() must name the organization; the plan's interface listed host and hop only"
  - "TestNewClientSetsTimeout bounds the deadline to [start+45s, end+45s]: the literal 'no more than 45s after the call started' cannot hold, since http.Client stamps the deadline at send time, after the call starts"
  - "Task 2 ran as TDD with separate test and feat commits (RED 51e7c6d, GREEN d7469f5) instead of the plan's single feat commit"
  - "Tracer feedback gate: the tracer verify is fully automated and passed end-to-end after commit, so execution continued without a mid-flight human-verify halt (human_verify_mode default end-of-phase, as in 08-01)"

patterns-established:
  - "D-11 structural test: subClientFindings resolves SDK imports per file (the v7 root's local name is azuredevops), treats each top-level declaration as one scope for the WithHTTPClient-bound identifier, and is non-vacuous at 7 constructions"
  - "Tripwire before commit: temporarily weaken the implementation, confirm the pinning test fails, restore byte-identically (sha256 checked), never commit the tripwire"

requirements-completed: [PLAT-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Org-bound transport end to end: org_url -> Config.Org -> transport.New -> ado.NewClient -> real v7 git sub-client -> base RoundTripper, with the PAT header arriving only through the injected client"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestNewClientBuildsOnInjectedClient"
        status: pass
    human_judgment: false
  - id: D2
    description: "The per-call budget lives on the injected http.Client and reaches the wire as a request-context deadline; a zero budget installs none"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/client_test.go#TestNewClientSetsTimeout"
        status: pass
      - kind: unit
        ref: "internal/ado/client_test.go#TestNewClientZeroTimeoutInstallsNoDeadline"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-10a: every discovered LocationUrl, used or unused, is checked before any sub-client is built; a foreign one returns *config.ConfigError{Field: org_url} naming area and host and nothing past discovery is sent; every org-bound shape still passes"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestNewClientRefusesForeignLocationURL (6 subtests)"
        status: pass
      - kind: unit
        ref: "internal/ado/v7_test.go#TestNewClientAllowsEveryBoundArea"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-11: no GetClientByUrl, GetClientByResourceAreaId, SDK NewClient or option-less NewClientWithOptions in internal/ado non-test files; internal/ado does not import net/http and TestNoRawHTTPInADO is unchanged"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/construct_test.go#TestSubClientsUseInjectedClient"
        status: pass
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestNoRawHTTPInADO (file diff since 81058bd: 0 lines)"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-10b per-request refusal in Transport.RoundTrip (closes the body, never reaches the base, first-hop ErrNotSent, hop not ErrNotSent, no Timeout method)"
    requirement: "PLAT-03"
    verification:
      - kind: other
        ref: "uncommitted probe internal/transport/probe_test.go#TestProbeRefusal, run once and deleted"
        status: pass
    human_judgment: true
    rationale: "No committed test pins RoundTrip refusal yet; 08-03 Task 1 owns the hardening table, redirect-hop, logging and no-print tests. The verifier should confirm 08-03 lands them"
  - id: D6
    description: "Phase 7 locks preserved"
    verification:
      - kind: other
        ref: "shasum -a 256 -c 08-cli-tests.sha256 -> 7 OK; 07-cli-audit-test.sha256 -> 1 OK; TestValidateGuardsCLIAndTUI and TestTUIWaitsForInflightWriteBeforeExit pass"
        status: pass
    human_judgment: false

# Metrics
duration: 11min
completed: 2026-10-20
status: complete
---

# Phase 8 Plan 02: Org-Bound Transport and Explicit Sub-Client Construction Summary

**New internal/transport package binds the PAT to the configured org (D-09 host rule on every request), ado.NewClient builds all six SDK sub-clients with NewClientWithOptions on the injected client after checking every discovered LocationUrl (D-10a), and a go/ast test forbids any construction path that bypasses the transport (D-11)**

## Performance

- **Duration:** 11 min
- **Started:** 2026-10-20T15:42:15Z
- **Completed:** 2026-10-20T15:53:30Z
- **Tasks:** 3
- **Files modified:** 8 source files (801 insertions, 73 deletions) plus deferred-items.md

## Accomplishments

- `internal/transport` (127 lines, stdlib plus `internal/logging` only): `Allowed` is the single D-09 rule (https only, no userinfo or port, dev.azure.com and `*.dev.azure.com` path-bound to `/{org}`, `{org}.visualstudio.com` and `{org}.*.visualstudio.com` any path, empty org fails closed); `RoundTrip` refuses before the base RoundTripper, closes the body, logs host and hop only, and returns a `notSentError` that is `ErrNotSent` on the first hop only and has no `Timeout` method
- `ado.NewClient(cfg, tr)`: one `WithHTTPClient(tr.HTTPClient())` option for the discovery root and all six sub-clients; `newConnection` and every `X.NewClient(ctx, connection)` are gone; the `core` client is not built (D-16)
- D-10a sweep `checkAreaHosts` runs immediately after `GetResourceAreas` and before the first `areaURL` lookup (line 81 vs 85), over every returned area
- `Config.Org()` / `parseOrgURL` give the transport its `{org}`; `main` builds the transport and passes it in, and the existing error branch maps the D-10a `ConfigError` to exit 3
- `NewClient` is now unit-tested end to end over a fake base for the first time (previously "cannot run as a unit test")

## Task Commits

1. **Task 1: End-to-end org-bound transport through ado.NewClient to a real v7 sub-client** - `d8d77e2` (feat, tracer)
2. **Task 2: Startup check of every discovered LocationUrl (D-10a)** - `51e7c6d` (test, RED) and `d7469f5` (feat, GREEN)
3. **Task 3: D-11 structural test that every SDK sub-client uses the injected client** - `2d12bd0` (test)

**Plan metadata:** recorded in the `docs(08-02): complete org-bound transport plan` commit

## Files Created/Modified

- `internal/transport/transport.go` - Transport, New, HTTPClient, Allowed, RoundTrip, ErrNotSent, notSentError
- `internal/config/config.go` - `errOrgURLShape`, `parseOrgURL`, `(*Config).Org()`; Validate untouched (D-12 wiring is 08-03)
- `internal/ado/client.go` - NewClient rewrite, `areaURL`, `checkAreaHosts`; imports `transport`, `net/url`, `uuid`, `fmt`
- `internal/ado/client_test.go` - TestNewClientSetsTimeout, TestNewClientZeroTimeoutInstallsNoDeadline over the real NewClient
- `internal/ado/v7_test.go` - v7Area, usedAreas, mixedCase, resourceAreasFixture, v7Request, newClientBase; TestNewClientBuildsOnInjectedClient, TestNewClientRefusesForeignLocationURL, TestNewClientAllowsEveryBoundArea
- `internal/ado/construct_test.go` - TestSubClientsUseInjectedClient and `subClientFindings`
- `internal/ado/interfaces.go` - drift-check comment reworded for `&X.ClientImpl` values (method sets unchanged)
- `cmd/adoBizInt/main.go` - `tr := transport.New(cfg.Org(), cfg.RequestTimeout, http.DefaultTransport)`; `ado.NewClient(cfg, tr)`
- `.planning/phases/08-platform-upgrade-and-pacing/deferred-items.md` - stale `newConnection` mention inside the locked TestNoRawHTTPInADO body

## Tripwire and Gate Runs

| Run | Change (never committed) | Result | Restored |
|-----|--------------------------|--------|----------|
| Task 2 RED | Tests written before `checkAreaHosts` existed | All 6 refusal rows FAIL (NewClient returned a client, nil error); allow test PASS | n/a (committed as `51e7c6d`) |
| Task 2 tripwire | `checkAreaHosts` skips every area whose id is not one of the five used ids | `foreign_host_on_unused_area`, `cross-org_path_on_unused_area`, `http_scheme_on_unused_area` and `unparsable_LocationUrl_on_unused_area` FAIL; `foreign_release_host`, `cross-org_git` PASS | sha256 `2284644e...` identical before and after; all 6 PASS |
| Task 3 tripwire | git sub-client built as `NewClientWithOptions(conn, gitURL)` | FAIL: `client.go:107:40: NewClientWithOptions without an azuredevops.WithHTTPClient option builds a default http.Client` | sha256 identical; PASS |
| Tracer gate | Task 1 `<verify>` re-run on the committed tree | build, vet, 4 targeted tests, 7/7 and 1/1 hashes: TRACER_VERIFY_PASS | n/a |
| D-10b probe | Throwaway `internal/transport/probe_test.go` | PASS: 4 foreign/cross-org/root/http URLs refused before base with body closed and `errors.Is(ErrNotSent)`; hop refusal not `ErrNotSent`; through `http.Client` the `*url.Error` reaches `ErrNotSent` and reports no timeout; 4 org-bound shapes allowed; empty org refuses the dev.azure.com root | file deleted |

## Decisions Made

- Every-area D-10a sweep as confirmed by the user; RESEARCH Open Question 3's used-areas narrowing not adopted.
- `transport.New` lowercases `org`; `Allowed` fails closed on an empty org.
- `notSentError` has an `org` field (needed by `Error()`).
- The tracer test builds the transport from `cfg.Org()` rather than a literal org, so `Config.Org` is on the proven path.

## Deviations from Plan

### Auto-fixed / Adjusted

**1. [Rule 1 - Bug in spec text] Timeout assertion bound**
- **Found during:** Task 1 (client_test.go)
- **Issue:** "deadline no more than 45s after the call started" is false by construction: http.Client sets the deadline when it sends the request, which is after the call starts.
- **Fix:** Assert the deadline lies in `[start+45s, end+45s]`, which is both satisfiable and stricter (it also fails for any budget other than 45s).
- **Files modified:** internal/ado/client_test.go
- **Committed in:** d8d77e2

**2. [Rule 2 - Missing critical] `notSentError.org` field**
- **Found during:** Task 1
- **Issue:** The interfaces block lists `host` and `hop` only, but `Error()` must name the organization.
- **Fix:** Added an unexported `org` field set by RoundTrip.
- **Files modified:** internal/transport/transport.go
- **Committed in:** d8d77e2

### Process

**3. [TDD] Task 2 split into RED and GREEN commits**
- The plan names one commit (`feat(08-02): refuse startup when any discovered location is foreign`); as a `tdd="true"` task it landed as `51e7c6d` (test) then `d7469f5` (feat, with the planned message).

**4. [Tracer gate] No mid-flight human-verify halt**
- Auto mode is not active, but the tracer's `<verify>` is fully automated and passed on the committed tree; `human_verify_mode` is at its end-of-phase default. Same handling as 08-01.

---

**Total deviations:** 4 (1 spec bug, 1 missing critical, 2 process). **Impact:** none on scope; every acceptance criterion of all three tasks passes.

## Issues Encountered

- `internal/ado/attachments_test.go:434` still names the deleted `newConnection` in a comment inside `TestNoRawHTTPInADO`. The body must stay byte-identical (D-11), so it is logged in `deferred-items.md` rather than fixed.
- Plan estimate was 82000 tokens; the realized diff is about 8300 (chars/4 over added lines). Recorded as is.

## User Setup Required

None - no external service configuration required. Note for 08-08: `org_url` must already be `https://dev.azure.com/{org}` or `https://{org}.visualstudio.com` (RESEARCH A4); any other value now yields an empty `Org()`, so the transport refuses every request and startup exits 3. 08-03 turns that into a named `Validate` error.

## Next Phase Readiness

- 08-03 can add the per-request hardening table, redirect-hop, logging and no-print tests against `Transport` and wire `parseOrgURL` into `Validate` (D-12); no committed test pins `RoundTrip` refusal until then (coverage D5).
- 08-05 and 08-06 extend `Transport` (pacer, `notBefore`) and `notSentError` (reasons) under the names fixed here.
- Plan-level verification: `go build ./... && go vet ./... && go test -race ./...` (15 packages ok), gofmt clean, all `TestNewClient*`, `TestSubClientsUseInjectedClient`, `TestNoRawHTTPInADO` pass, locks 7/7 and 1/1.

## Self-Check: PASSED

- FOUND: internal/transport/transport.go, internal/ado/construct_test.go, internal/ado/client.go, internal/ado/v7_test.go, internal/ado/client_test.go, internal/config/config.go, cmd/adoBizInt/main.go, internal/ado/interfaces.go, deferred-items.md
- FOUND commits: d8d77e2, 51e7c6d, d7469f5, 2d12bd0
- key_links: `transport.New(cfg.Org()` in main.go (1), `WithHTTPClient(tr.HTTPClient())` in client.go (1), `tr.Allowed(` in client.go (1), `t.base.RoundTrip` in transport.go (1); `NewClientWithOptions` in client.go (8 incl. doc comment); `func (c *Config) Org() string` (1)
- TDD gate: `test(08-02)` 51e7c6d precedes `feat(08-02)` d7469f5
- No stub markers (TODO, FIXME, placeholder) in created or modified source files

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-20*
