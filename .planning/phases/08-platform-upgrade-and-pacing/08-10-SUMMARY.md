---
phase: 08-platform-upgrade-and-pacing
plan: 10
subsystem: auth
tags: [transport, host-rule, idn-homograph, unicode-case-folding, escaped-path, org-url, pat-binding, CR-01, WR-01, IN-01]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-02 transport.Allowed and New; 08-03 parseOrgURL, Org(), D-12 Validate and the redirect-hop refusal tests; 08-09 go 1.27.3 toolchain"
provides:
  - "transport.Allowed refuses any non-ASCII host before case folding (asciiLower), on the first hop and on every redirect hop"
  - "dev.azure.com org binding decided on u.EscapedPath() against the org's escaped form; %2F, %5C, %2E, dot and empty segments and opaque URLs refused, never normalised"
  - "parseOrgURL refuses a fragment marker, any non-ASCII byte in the raw value or host, . and .. orgs, and percent-encoded or PathEscape-unstable org segments as the named org_url error"
affects: [08-11, 08-12, phase-09]

# Actuals (#2632)
actuals:
  tokens: 4700
  tasks: 3
  commits: 6

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Decide on the wire form: a security rule inspects u.EscapedPath() (what net/http sends), never the decoded and cleaned u.Path"
    - "ASCII-before-fold: refuse any non-ASCII byte before strings.ToLower on a host, because Unicode folding (U+0130, U+212A) lands on ASCII while net/http dials the IDNA form"
    - "Refuse, never normalise: non-canonical paths are rejected rather than resolved into an allowed form"

key-files:
  created: []
  modified:
    - internal/transport/transport.go
    - internal/transport/transport_test.go
    - internal/config/config.go
    - internal/config/config_test.go
    - .planning/phases/08-platform-upgrade-and-pacing/deferred-items.md

key-decisions:
  - "08-10: user decision 2026-10-21 'Allow non-ASCII names' -- the request rule refuses non-ASCII HOSTS only; a non-ASCII project, repository or team name after the org (Caf%C3%A9) stays allowed. The CR-01 path variant (Kelvin-sign org) is closed by the escaped-path compare: /%E2%84%AAcme never equals /kcme"
  - "08-10: dev.azure.com org binding compares the lowered u.EscapedPath() with '/' + lower(url.PathEscape(org)); %2f, %5c, %2e anywhere and any path path.Clean changes beyond one trailing slash are refused; opaque URLs refused for every host"
  - "08-10: parseOrgURL requires the escaped org segment to equal both the decoded segment and url.PathEscape(segment), so org_url always binds the exact form the host rule compares; a fragment marker, non-ASCII raw value or host, and . or .. orgs are the named org_url error"
  - "08-10: empty-org refusal rests on the early return in Allowed; without it '/' would equal '/' plus the empty org. The plan's draft rationale ('/' never equals '/'+'') was false and was not copied"
  - "08-10: double-encoded dot after the org (/acme/%252e%252e/other/_apis) accepted as a risk in deferred-items.md: only exploitable if ADO double-decodes, via a same-host redirect"

patterns-established:
  - "Wire-form compare: compare escaped request bytes against the escaped expected value, not decoded values against decoded values"
  - "Homograph test rows: every host-rule table carries U+0130 and U+212A rows, raw and percent-encoded"

requirements-completed: []

coverage:
  - id: D1
    description: "A non-ASCII host, including an IDN homograph that folds onto an allowed host (U+0130, U+212A), is refused by Allowed before folding; a first-hop request never reaches the base RoundTripper and returns ErrNotSent; a 307 to the homograph host is refused by the transport on the redirect hop (base called once, hop true, not ErrNotSent)"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestAllowed (homograph host rows, Cyrillic guard row)"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestRefuseHomographNeverReachesBase"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestRefuseRedirectHopIsNotNotSent/redirect_to_an_IDN_homograph_of_the_legacy_org_host"
        status: pass
    human_judgment: false
  - id: D2
    description: "dev.azure.com and *.dev.azure.com org binding decided on the escaped path: encoded traversal, dot and empty segments, encoded backslash, encoded letter (%61cme), the Kelvin-sign org (raw and %E2%84%AAcme) and an opaque URL are refused; every real-v7-SDK test still passes unchanged"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestAllowed (WR-01 rows, Kelvin path rows, opaque URL subtest)"
        status: pass
      - kind: integration
        ref: "go test -count=1 ./internal/ado ./internal/guard ./cmd/adoBizInt"
        status: pass
    human_judgment: false
  - id: D3
    description: "Non-ASCII project names after the org stay allowed (user decision 2026-10-21), plus encoded space, trailing slash and upper-case org on vsrm"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestAllowed (Caf%C3%A9 rows, My%20Project, trailing slash, vsrm ACME)"
        status: pass
    human_judgment: false
  - id: D4
    description: "config.Validate refuses non-ASCII, fragment-marked, dot-segment and percent-encoded or PathEscape-unstable org_url values as the named org_url error; Org() returns empty for them; the built binary exits 3 with org_url on stderr and never echoes the PAT"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestValidateOrgURL (11 new org_url rows)"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestOrg (%E2%84%AAcme and %61cme rows)"
        status: pass
      - kind: other
        ref: "Task 3 verify: built binary with ADO_ORG_URL=https://dev.azure.com/%E2%84%AAcme exits 3, stderr names org_url, no PAT in stdout or stderr"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every legitimate URL the tool sends still passes the stricter rule against the real organization (all sub-clients, vsrm included)"
    requirement: "PLAT-03"
    verification:
      - kind: integration
        ref: "real-v7-SDK tests in internal/ado, internal/guard, cmd/adoBizInt"
        status: pass
    human_judgment: true
    rationale: "Fakes cannot prove what live ADO discovery and sub-client routes look like; the 08-12 smoke re-run against the real org is the gate that would surface a legitimate URL the rule refuses."

# Metrics
duration: 11min
completed: 2026-10-21
status: complete
---

# Phase 8 Plan 10: Host Rule Homograph and Escaped-Path Hardening Summary

**transport.Allowed now refuses IDN homograph hosts before case folding and binds dev.azure.com requests to the org on the escaped wire path (refusing encoded, dot and empty segments rather than normalising), while parseOrgURL rejects any org_url whose org would fold or encode differently -- closing SC4's CR-01, WR-01 and IN-01 with non-ASCII project names still allowed.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-10-21T20:07:27Z
- **Completed:** 2026-10-21T20:18:03Z
- **Tasks:** 3
- **Files modified:** 4 source files, plus deferred-items.md

## Accomplishments

- CR-01 closed at the rule: `asciiLower` refuses any non-ASCII host byte before `strings.ToLower`, so `acme.v\u0130sualstudio.com` (dialed as `acme.xn--visualstudio-n4g.com`) and `\u212Acme.visualstudio.com` are refused on the first hop and on a redirect hop, without relying on net/http's redirect header stripping.
- WR-01 and the CR-01 path variant closed: the dev.azure.com branch compares `u.EscapedPath()` with the org's escaped form. The verifier's probes `other%2F..%2Facme/_apis` and `other/../acme/_apis` are refused, as are `%61cme`, the Kelvin org on the wire as `/%E2%84%AAcme`, and opaque URLs.
- IN-01 and the config half of CR-01 closed: `parseOrgURL` refuses `#` anywhere, non-ASCII raw or host, `.`/`..` orgs and percent-encoded or PathEscape-unstable org segments, all as the named org_url error at startup (exit 3).
- User decision honoured: `Caf%C3%A9` after the org passes on both dev.azure.com and the legacy host, pinned by want-true rows.
- No legitimate URL shape had to be admitted: every real-v7-SDK test in internal/ado, internal/guard and cmd/adoBizInt passed unchanged against the stricter rule.

## Task Commits

Each task landed as a RED/GREEN pair:

1. **Task 1: Non-ASCII host refusal (tracer)** - `fc9691c` (test, RED), `172bcc7` (fix, GREEN)
2. **Task 2: Escaped-path org binding (WR-01)** - `712491f` (test, RED), `0e40c3c` (fix, GREEN)
3. **Task 3: org_url hardening (D-12, IN-01)** - `4b3b912` (test, RED), `3b87cf2` (fix, GREEN)

**Plan metadata:** this SUMMARY commit, followed by the state and roadmap commit.

## TDD Record

| Gate | Result |
|------|--------|
| Task 1 RED | 3 homograph TestAllowed rows FAIL (true, want false); TestRefuseHomographNeverReachesBase FAIL (base called); redirect homograph row FAIL (base called 10 times, "stopped after 10 redirects"). Cyrillic and both Caf%C3%A9 guard rows PASS |
| Task 1 GREEN | All TestAllowed and TestRefuse* PASS; `go test -race ./...` PASS |
| Task 2 RED | 8 TestAllowed rows FAIL (other%2F..%2Facme, other/../acme, %5C, `//`, `/./`, %61cme, both Kelvin rows) plus the opaque subtest. The `%2e%2e` and `..%2F` rows after the org were already refused by path.Clean and act as guard rows |
| Task 2 GREEN | TestAllowed PASS; internal/ado, internal/guard, cmd/adoBizInt PASS unchanged; race PASS; locks 7 OK |
| Task 3 RED | All 11 new TestValidateOrgURL rows FAIL (Validate nil); TestOrg returns "kcme" and "acme", want "" |
| Task 3 GREEN | TestValidateOrgURL and TestOrg PASS; race PASS; built binary exits 3 naming org_url, no PAT echoed |

## Files Created/Modified

- `internal/transport/transport.go` - `asciiLower`; Allowed refuses non-ASCII hosts and opaque URLs and decides the dev.azure.com org on `u.EscapedPath()`. Now 407 lines (366 before).
- `internal/transport/transport_test.go` - CR-01 and WR-01 TestAllowed rows, Caf%C3%A9 guards, opaque subtest, TestRefuseHomographNeverReachesBase, homograph redirect row.
- `internal/config/config.go` - `isASCII`; parseOrgURL fragment-marker, non-ASCII, dot-segment and escaped-form checks. Now 303 lines.
- `internal/config/config_test.go` - 11 TestValidateOrgURL rows, 2 TestOrg rows.
- `.planning/phases/08-platform-upgrade-and-pacing/deferred-items.md` - double-encoded dot accepted risk; transport.go line count.

## Decisions Made

- **"Allow non-ASCII names" (user, 2026-10-21):** the request rule refuses non-ASCII hosts only. The CR-01 path variant is closed by the escaped-path compare, not by refusing non-ASCII paths: a Kelvin-sign org segment is `/%E2%84%AAcme` on the wire and never equals `/kcme`, while `Caf%C3%A9` after the org is canonical ASCII and passes.
- **Path check inlined in Allowed**, not split into a helper: it has one caller (user rule against single-use helpers). `asciiLower` stays a helper because host and path both use it.
- **Empty-org rationale corrected:** the doc comment says the early return is what refuses everything, because without it `"/"` would equal `"/"` plus the empty org. The plan draft's claim that `"/"` never equals `"/"+""` is false and was not copied (plan-checker note).
- **config helper `isASCII`** lives in config.go rather than importing transport, which sits above config in the dependency order.

## Deviations from Plan

### Auto-fixed Issues

None. No bugs, missing functionality or blockers came up outside the planned work.

### Process notes

**1. [TDD] Each task landed as a test/fix pair**
- **Plan:** one `fix(08-10)` commit per task.
- **Done:** a `test(08-10)` RED commit with the failing rows, then the GREEN commit with the plan's exact `fix(08-10)` subject. This follows the executor TDD protocol for `tdd="true"` tasks and the 08-06 precedent. Each RED was confirmed to fail for the right reason first (see TDD Record).

**2. [Tracer gate] Run as an automated re-verify**
- Auto mode is not set (`_auto_chain_active` false, `auto_advance` unset), which would normally stop for a `checkpoint:human-verify` after the tracer. `human_verify_mode` is at its end-of-phase default, and the orchestrator asked for the whole plan, so the tracer's `<verify>` was re-run after commit `172bcc7` (PASS) before expanding. Nothing in the tracer needs a human to check it: it is a unit-tested rule.

**3. [Doc accuracy] Empty-org comment** -- see Decisions; the plan's parenthetical rationale was not reproduced.

---

**Total deviations:** 0 auto-fixed; 3 process notes.
**Impact on plan:** None on scope. Every planned row, test and check is present and green.

## Issues Encountered

- A double-encoded dot after the org (`/acme/%252e%252e/other/_apis`) passes the new rule; confirmed with a throwaway overlay probe (no repo change). Recorded in deferred-items.md as an accepted risk: it only names another org if ADO's front end decodes twice, and only via a same-host redirect or a server-supplied URL.

## Verification

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go vet ./...`, `gofmt -l .` | PASS, no files |
| `go test -race ./...` | PASS |
| `go test -count=2 ./...` | PASS |
| `go test -count=1 ./internal/ado ./internal/guard ./cmd/adoBizInt` | PASS (real v7 clients, startup area check) |
| 08-cli-tests.sha256 lock check | 7 OK |
| Task 1, 2, 3 `<verify>` blocks | PASS (all greps and the built-binary exit-3 check) |

## Threat Model Coverage

| Threat | Status |
|--------|--------|
| T-08-44 (homograph host) | Mitigated: asciiLower before fold; first-hop and redirect-hop tests |
| T-08-45 (path compare on dev.azure.com) | Mitigated: escaped-path compare; both verifier probe URLs refused. Residual: double-encoded dot, accepted in deferred-items.md |
| T-08-46 (org_url tampering) | Mitigated: 11 org_url rows; built binary exits 3 without echoing the PAT |
| T-08-47 (legitimate URL refused) | Mitigated in tests (Caf%C3%A9 rows, every real-SDK test); live confirmation is 08-12 |

## User Setup Required

None. No external service configuration required.

## Next Phase Readiness

- Ready for 08-11, which also edits transport.go and transport_test.go. The working tree has no uncommitted Go changes.
- PLAT-03 stays open in REQUIREMENTS.md: 08-11 also declares it.
- The 08-12 live smoke re-run is the gate that would surface a legitimate URL the stricter rule refuses (D5, human judgment).

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-21*

## Self-Check: PASSED

- All 4 modified source files and this SUMMARY exist on disk
- All 6 task commits found: fc9691c, 172bcc7, 712491f, 0e40c3c, 4b3b912, 3b87cf2
- Plan-level verification re-run green: build, vet, race, count=2, locks 7 OK
