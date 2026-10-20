---
phase: 08-platform-upgrade-and-pacing
plan: 03
subsystem: api
tags: [go, transport, host-trust, pat, org-url, plat-03, d-09, d-10b, d-12]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-02: internal/transport (New, HTTPClient, Allowed, RoundTrip, ErrNotSent, notSentError), config.parseOrgURL and Config.Org, the v7_test.go fixtures"
provides:
  - "config.Validate rejects any org_url outside https://dev.azure.com/{org} and https://{org}.visualstudio.com with ConfigError org_url, after the missing-PAT check (D-12); the binary exits 3 before any request"
  - "TestValidateOrgURL (20 rows) and TestOrg in internal/config/config_test.go"
  - "TestValidatedOrgURLPassesHostRule in internal/ado/v7_test.go: every accepted shape, normalized by the SDK, passes the host rule"
  - "internal/transport/transport_test.go: rtFunc twin, TestAllowed (24 rows), TestRefuseNeverReachesBase, TestRefuseRedirectHopIsNotNotSent, TestRefusalNeverLogsPAT, TestTransportNeverPrints (with forbiddenOutput and tripwire), TestHTTPClientTimeout"
affects: [08-05, 08-06, 08-08, phase-09]

# Actuals (#2632)
actuals:
  tokens: 5587
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/transport/transport_test.go is the transport package's test home; 08-05 and 08-06 extend it (rtFunc, mustNotSend, emptyResp, basicAuth helpers)"
    - "The refusal-before-send body-close check runs through http.Client: the client marks the body closed after a send error and never closes it itself, so only RoundTrip can have closed it"
    - "TestTransportNeverPrints globs every non-test file in the package and requires transport.go among them, so a file 08-05 adds is covered automatically"

key-files:
  created:
    - internal/transport/transport_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/ado/v7_test.go

key-decisions:
  - "No defect found in Allowed or RoundTrip: every TestAllowed row and refusal test passed on the first run, so transport.go is unchanged"
  - "The D-12 error text is the parse error verbatim, as the plan specifies; on stderr it reads 'Invalid config: org_url: org_url must be https://dev.azure.com/{org} or https://{org}.visualstudio.com' (field name appears twice, cosmetic only)"
  - "Redirect-hop tests use POST with a *strings.Reader body, so the client follows the 307 with the body as it would for a write; a body without GetBody would stop the redirect before the hop"
  - "Tracer feedback gate: the tracer verify is fully automated and passed on the committed tree, so execution continued to Task 2 without a mid-flight human-verify halt, as in 08-01 and 08-02"
  - "PLAT-03 stays Pending in REQUIREMENTS.md: requirements.ready-ids reports 0/1 because later Phase 8 plans also declare it"

patterns-established:
  - "Tripwire per pinned behaviour: weaken transport.go one rule at a time, confirm the specific test fails, restore byte-identically (sha256 checked), never commit"

requirements-completed: [PLAT-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "D-12: Validate refuses every non-cloud org_url with ConfigError org_url after the missing-PAT check, and accepts the five cloud forms"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestValidateOrgURL (20 subtests)"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestOrg (3 subtests)"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestValidate (pre-existing 14 rows unchanged)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The built binary with ADO_ORG_URL=https://tfs.corp.local/tfs/DefaultCollection in an isolated HOME exits 3, names org_url on stderr, never reaches ado.NewClient, and no stream or log holds the sentinel PAT"
    requirement: "PLAT-03"
    verification:
      - kind: e2e
        ref: "Task 1 <verify> command (go build + env -i run): rc=3, stderr contains org_url, sentinel absent from stdout, stderr and adoBizInt.log, no 'Failed to create ADO client' log line"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every org_url D-12 accepts yields an SDK-normalized base URL the host rule allows"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestValidatedOrgURLPassesHostRule (5 subtests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-09 org binding table: 8 allowed shapes, 15 refused look-alike/traversal/port/userinfo/scheme/cross-org shapes, and an empty org refusing the dev.azure.com root"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestAllowed (24 subtests, 25 PASS lines)"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-10b: a first-hop refusal never reaches the base, closes the body, satisfies ErrNotSent and names the host; a redirect-hop refusal (foreign host and same-host cross-org) is refused after exactly one base call and is not ErrNotSent"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestRefuseNeverReachesBase"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestRefuseRedirectHopIsNotNotSent (2 subtests)"
        status: pass
    human_judgment: false
  - id: D6
    description: "No raw, Basic-header or base64 PAT form in the refusal log or errors, with the capture proven live by host=evil.example; the package never writes stdout or stderr or exits; HTTPClient carries the request budget"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestRefusalNeverLogsPAT"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestTransportNeverPrints (incl. tripwire, 11 forms)"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestHTTPClientTimeout"
        status: pass
    human_judgment: false
  - id: D7
    description: "Phase 7 locks preserved"
    verification:
      - kind: other
        ref: "shasum -a 256 -c 08-cli-tests.sha256 -> 7 OK; 07-cli-audit-test.sha256 -> 1 OK"
        status: pass
    human_judgment: false

# Metrics
duration: 7min
completed: 2026-10-20
status: complete
---

# Phase 8 Plan 03: Org Binding Hardening and Cloud org_url Summary

**`config.Validate` now refuses any org_url outside the two cloud shapes (the binary exits 3 naming org_url before any request), and a new `internal/transport/transport_test.go` pins the host rule with a 24-row table, refusal before send on first hops, non-not-sent refusal on redirect hops including same-host cross-org, PAT-free logs with a live-capture marker, and a no-print AST check**

## Performance

- **Duration:** 7 min
- **Started:** 2026-10-20T15:57:32Z
- **Completed:** 2026-10-20T16:04:26Z
- **Tasks:** 2
- **Files modified:** 4 (579 insertions, 0 deletions)

## Accomplishments

- D-12 wired: one branch in `Validate`, directly after the empty org_url check, returns `&ConfigError{Field: "org_url", Message: <parse error>}`; `c.OrgURL` is never rewritten and `LogValue`/`String` are untouched
- The real binary, run with an on-prem collection URL in an isolated HOME, exits 3 with the org_url error on stderr and never reaches `ado.NewClient`; the sentinel PAT appears in no stream and not in the log
- Every accepted shape, after `NewPatConnection` lowercases and trims it, is allowed by `transport.New(cfg.Org(), 0, nil).Allowed`, so a validated config cannot have every request refused
- First committed tests of `RoundTrip` refusal (08-02 coverage D5 was an uncommitted probe): body closed, base never called, `ErrNotSent` on first hops only, and Go's same-host Authorization forwarding on a cross-org redirect is refused per hop
- `TestRefusalNeverLogsPAT` checks three PAT forms in both the log buffer and every error string, and fails with "capture saw nothing" if the refusal line with `host=evil.example` is absent

## Task Commits

1. **Task 1: A non-cloud org_url stops the binary at startup with exit 3 naming org_url (D-12)** - `d891f33` (feat, tracer)
2. **Task 2: Host-rule hardening table, refusal before send, redirect hops, no PAT in logs, never prints** - `aa8a77e` (test)

**Plan metadata:** recorded in the `docs(08-03): complete org binding hardening plan` commit

## Files Created/Modified

- `internal/config/config.go` - D-12 branch in `Validate` calling `parseOrgURL(c.OrgURL)`
- `internal/config/config_test.go` - `TestValidateOrgURL` (base() and assertion block copied verbatim from `TestValidate`), `TestOrg`
- `internal/ado/v7_test.go` - `TestValidatedOrgURLPassesHostRule`; imports `net/url`
- `internal/transport/transport_test.go` - new: `rtFunc` twin, `mustNotSend`, `emptyResp`, `basicAuth`, `closeRecorder`, `forbiddenOutput` copy with `os.Stderr`, and the six tests

## Tripwire and Gate Runs

| Run | Change (never committed) | Result |
|-----|--------------------------|--------|
| Task 1 tripwire | Validate branch disabled (`false && err != nil`) | 14 rejected TestValidateOrgURL rows FAIL; restored byte-identically |
| Tracer gate | Task 1 `<verify>` re-run on the committed tree | TRACER_VERIFY_PASS |
| path.Clean removed (`u.Path` used raw) | `TestAllowed/path_traversal_to_another_org` FAIL only |
| Body close removed | `TestRefuseNeverReachesBase` FAIL |
| `notSentError.Is` ignores hop | both `TestRefuseRedirectHopIsNotNotSent` rows FAIL |
| Authorization added to the refusal log line | `TestRefusalNeverLogsPAT` FAIL |
| Refusal log line removed / host attribute dropped | `TestRefusalNeverLogsPAT` FAIL with "capture saw nothing" |
| dev.azure.com path binding removed | 4 cross-org TestAllowed rows and the same-host redirect row FAIL |
| `fmt.Fprintln(os.Stderr, ...)` added to RoundTrip | `TestTransportNeverPrints` FAIL |
| `HTTPClient` Timeout forced to 0 | `TestHTTPClientTimeout` FAIL |

`transport.go` sha256 `03578b1d...` was identical before and after all transport tripwires.

## Decisions Made

- No defect in `Allowed` or `RoundTrip`; `transport.go` unchanged.
- D-12 message is the parse error verbatim, per the plan, so stderr names `org_url` twice. Cosmetic; changing it would mean a new message constant the plan did not ask for.
- Redirect tests use POST with a `*strings.Reader` body so `GetBody` exists and the 307 is followed with the body, matching the write case T-08-11 covers.
- `TestTransportNeverPrints` globs all non-test files and requires `transport.go` among them (non-vacuity), instead of parsing `transport.go` alone, so files 08-05 adds are covered.

## Deviations from Plan

### Acceptance criterion text

**1. [Criterion counts a pre-existing match] `grep -c 'parseOrgURL(c.OrgURL)' internal/config/config.go` prints 2, not 1**
- **Found during:** Task 1 acceptance check
- **Issue:** 08-02's `(*Config).Org()` already contains `parseOrgURL(c.OrgURL)` (line 252). The new Validate call is line 184.
- **Resolution:** Verified the intent instead: the call appears exactly once inside `Validate` (`awk` over the function body prints 1). Rewriting `Org()` to dodge the grep would be gaming the criterion.
- **Files modified:** none

### Process

**2. [Tracer gate] No mid-flight human-verify halt after the tracer**
- Auto mode is not active; the tracer `<verify>` is fully automated and passed end-to-end on the committed tree, and `human_verify_mode` is at its end-of-phase default. Same handling as 08-01 and 08-02.

**3. [Phase 7 gate] Not checked here**
- Per the user decision of 2026-10-20, the Phase 7 verification gate applies only to 08-08.

---

**Total deviations:** 3 (1 criterion-text mismatch, 2 process). No auto-fixes were needed. **Impact:** none on scope; every other acceptance criterion passes as written.

## Issues Encountered

- Plan estimate was 60000 tokens; the realized diff is about 5600 (chars/4 over added lines).
- Two first-attempt tripwires were badly built (an unused-import compile error, and a Warn-to-Debug change the Debug-level capture still sees); both were redone as above and failed the intended test.

## User Setup Required

None - no external service configuration required. For 08-08: an `org_url` that is not `https://dev.azure.com/{org}` or `https://{org}.visualstudio.com` now stops startup with `Invalid config: org_url: ...` and exit 3.

## Next Phase Readiness

- 08-04 precondition: `git status --porcelain -- '*.go' go.mod go.sum` is empty.
- 08-05 and 08-06 extend `internal/transport/transport_test.go`; its helpers (`rtFunc`, `mustNotSend`, `emptyResp`, `basicAuth`) are ready for pacer tests.
- Plan-level verification: `go build ./... && go vet ./... && go test -race ./...` all ok, gofmt clean, locks 7/7 and 1/1.

## Self-Check: PASSED

- FOUND: internal/transport/transport_test.go, internal/config/config.go, internal/config/config_test.go, internal/ado/v7_test.go
- FOUND commits: d891f33, aa8a77e
- key_links: `parseOrgURL(c.OrgURL)` inside Validate (1); `.Allowed(` in transport_test.go (TestAllowed); `TestAllowed`, `TestValidateOrgURL`, `parseOrgURL(c.OrgURL)` present per must_haves.artifacts
- No stub markers (TODO, FIXME, placeholder) in created or modified files

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-20*
