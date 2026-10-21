---
phase: 08-platform-upgrade-and-pacing
reviewed: 2026-10-21T21:19:53Z
depth: standard
review_kind: re-review after gap closure (08-09..08-11), diff f53cdac..HEAD
files_reviewed: 7
files_reviewed_list:
  - internal/transport/transport.go
  - internal/transport/transport_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/ado/retry_test.go
  - cmd/adoBizInt/v7auth_test.go
  - go.mod
prior_findings:
  CR-01: fixed
  WR-01: fixed (residual accepted in deferred-items.md; see IN-07)
  WR-02: fixed
  WR-03: fixed (notBefore-never-decays remainder accepted in deferred-items.md)
  WR-04: fixed
  WR-05: fixed
  IN-01: fixed
  IN-02: fixed
  IN-03: deferred
  IN-04: deferred (accepted under D-04)
  IN-05: still open (08-12 live check waived; deferred-items.md row is stale, see IN-08)
  IN-06: deferred
findings:
  critical: 0
  warning: 0
  info: 4
  total: 4
status: issues_found
---

# Phase 8: Code Review Report (re-review after gap closure)

**Reviewed:** 2026-10-21T21:19:53Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found (Info only)

## Narrative Findings (AI reviewer)

## Summary

I re-reviewed the gap-closure diff `f53cdac..HEAD` on the seven files in scope, against the prior 08-REVIEW, 08-CONTEXT D-01..D-16, the 08-09/10/11 summaries and deferred-items.md. I also read the v7.1.0 SDK route builder (`client.go:141-153, 281-302`) and `normalizeUrl` (`connection.go:55-57`), because what the host rule sees depends on how the SDK builds URLs. No source file was changed. Probes ran as `go test -overlay` files in the session temp directory and were deleted afterwards.

Gates run:
- `go vet ./...`: clean
- `go test -race -count=2` on transport, config, ado, guard and cmd/adoBizInt: pass
- `go test -count=3 -run TestV7AuthFailureExitsAuth ./cmd/adoBizInt`: pass (this failed before the fix)

**Verdict:** every Critical and Warning from the prior review is closed. I found no new Critical or Warning. The four new items are Info.

### Can the PAT still reach a host or org path other than the configured org?

Not through the URL forms the rule can see. Each case I checked:

- **Non-ASCII hosts.** `asciiLower` (`transport.go:144-147`) refuses them before folding, on the first hop and on redirect hops alike.
- **Org segment.** It is compared on `u.EscapedPath()` (`transport.go:149-161`). That is the same accessor `URL.RequestURI()` uses for the request line, so the rule checks the bytes that go on the wire.
- **Opaque URLs, userinfo and explicit ports** are refused.
- **Look-alike hosts.** `dev.azure.com.`, `evildev.azure.com` and punycode forms such as `acme.xn--visualstudio-n4g.com` are refused.
- **An empty port** (`dev.azure.com:`) is allowed, but it dials 443 on the same host, so nothing leaks.

The one residual is a path that names another org only after the server decodes it a second time or normalises it in a non-standard way. That is the accepted `%252e` risk. The same class is wider than the record says (IN-07).

### Can a legitimate ADO URL be refused?

None of the URLs the tool sends today. I traced `transformRouteTemplate` (route values pass through `url.PathEscape`, which never emits `%2E`) and `normalizeUrl` (BaseUrl is lowercased). Through `Allowed`, all of these pass:
- `Caf%C3%A9` and raw `Café` (sent as `Caf%C3%A9`)
- `A&B`, `50%25`, `web.api`
- a trailing slash
- `vsrm` and `vssps` under the org, upper-case `ACME` on `vsrm`
- `{org}.visualstudio.com` and `{org}.vsrm.visualstudio.com`, upper-case included

`%2F` would be produced only by a wildcard route (`{*path}`) given a nested path, and no current call site passes one (IN-09).

### parseOrgURL

Every real ADO org name passes. Real names use letters, digits and hyphens, either case, with or without a trailing slash, in either the dev.azure.com or the visualstudio.com form; I also checked an underscore label. Refused as intended:
- non-ASCII, raw or percent-encoded, in the org or the host
- `#`, including an empty fragment
- `.` and `..`, raw or encoded
- `%61cme` and `a%2Eb`
- `a!b`
- a trailing space

One inconsistency remains: a dotted org is accepted on dev.azure.com (IN-10).

### Pacer

- **Hold budget** (`transport.go:247-257`): `min(maxHold, time.Until(deadline) - minSendBudget)`. A wait is refused only when it is strictly greater than the budget.
- **Lock usage:** `notBefore` is read and written only under `mu`, and the counters are atomic. Race-clean under `-race -count=2`.
- **Rounding** (`transport.go:419-423`): ceiling by quotient plus remainder cannot overflow; a saturated wait prints `9223372037s` (TestHoldCap). Because a refusal requires `wait > budget`, ceil(wait) > floor(budget) always holds, so the message can never read as a wait that fits.
- **Overflow in the budget subtraction:** `time.Until(deadline) - minSendBudget` wraps only for a deadline about 292 years in the past. Any stdlib context has already reported `ctx.Err()` by then (`transport.go:243`), so this cannot be reached.
- **D-01:** refusals are still the typed fail-fast. `ErrRateLimited` is set and `DeadlineExceeded` is not wrapped.
- **D-02:** a refusal on a first hop is `ErrNotSent`; on a redirect hop it is not.
- **D-04:** a wait that is accepted is held until the instructed time and never shortened. `minSendBudget = 2s` is the user's decision. `maxHold` falls under the 08-CONTEXT discretion clause.

## Prior Findings: Status

| ID | Status | Evidence |
|----|--------|----------|
| CR-01 | **Fixed** | `transport.go:144-147, 174-181`: `asciiLower` refuses any non-ASCII host byte before folding. `config.go:232-234, 244-246`: raw value and decoded host must be ASCII. Pinned by TestAllowed homograph rows (`transport_test.go:111-113`), TestRefuseHomographNeverReachesBase (`:217`), TestRefuseRedirectHopIsNotNotSent IDN row (`:272-277`), TestValidateOrgURL Kelvin and dotted-I rows (`config_test.go:263-266`). The Kelvin path variant is closed by the escaped-path compare: `/%E2%84%AAcme` is not `/kcme` (`transport_test.go:135-136`). |
| WR-01 | **Fixed** (residual accepted) | `transport.go:149-161` decides on `EscapedPath()`, refuses `%2f`, `%5c`, `%2e`, dot and empty segments, and compares to `"/" + PathEscape(org)`. Both of the prior probe URLs are refused (`transport_test.go:125-126`). The `%252e` residual is accepted in deferred-items.md:15. The same residual also covers other forms (IN-07). |
| WR-02 | **Fixed** | `transport.go:65, 247-251`: a held request reaches the base with at least 2s left. TestFailFastBoundary (`transport_test.go:1055`) asserts the 7s boundary and the deadline remaining at the base. A 29s Retry-After under the 30s default now fails fast as rate_limit. |
| WR-03 | **Fixed** (remainder accepted) | `transport.go:72, 247`: `maxHold = 5m` bounds every single hold, including on deadline-less startup. TestHoldCap (`transport_test.go:1296`) covers 300s held, 301s refused, a year-9999 HTTP-date refused, and 400s refused under a 10m budget. The notBefore-never-decays remainder is accepted in deferred-items.md:29. |
| WR-04 | **Fixed** | `retry_test.go:37-54` adds `readMethods`. `:258-267` makes every method exactly one of read or write, and the unclassified message never suggests adding an override. `:345` makes the missing-override check fire only for classified reads. Stale-entry checks cover both lists. |
| WR-05 | **Fixed** | `v7auth_test.go:41, 91`: an atomic per-invocation org suffix. `go test -count=3 -run TestV7AuthFailureExitsAuth` passes (verified in this review). |
| IN-01 | **Fixed** | `config.go:232` refuses any `#`. `:251` refuses `.` and `..`. `:256-260` requires the escaped org to equal the decoded org and `PathEscape(org)`. Rows at `config_test.go:269-277`. |
| IN-02 | **Fixed** | `transport.go:419-423`: the asked wait rounds up and the budget rounds down. TestFailFastMessageRounding (`transport_test.go:1388`) pins `5s, 4s` and `1s, 0s`. |
| IN-03 | **Deferred** | deferred-items.md:25. retry.go is not in this diff. |
| IN-04 | **Deferred** (accepted under D-04) | deferred-items.md:26. |
| IN-05 | **Still open** | deferred-items.md:28 says "Not deferred: checked live by 08-12 checklist item c". 08-12-SUMMARY:135 records item c as "Not run. WAIVED". The live attachment download under the hardened rule has never run. BACKLOG BL-02 carries it. See IN-08. |
| IN-06 | **Deferred** | deferred-items.md:27. client.go is not in this diff. |

## Info

### IN-07: The accepted double-encoding risk is wider than the deferred-items record describes

**File:** `internal/transport/transport.go:153-161`; `.planning/phases/08-platform-upgrade-and-pacing/deferred-items.md:15`
**Issue:** The record accepts only `%252e`. A probe against `Allowed` (org `acme`) shows several other forms of the same weakness pass. Each relies on the server decoding a second time or normalising beyond RFC 3986:

```
allowed=true  /acme/x%252f..%252f..%252fother/_apis      (double-encoded slash)
allowed=true  /acme/%c0%ae%c0%ae/other/_apis             (overlong UTF-8 dot)
allowed=true  /acme/%EF%BC%8E%EF%BC%8E/other/_apis        (fullwidth full stop)
allowed=true  /acme/..;/other/_apis                       (path-parameter traversal)
```

None of these can come from the SDK. As with `%252e`, each needs a same-host redirect or a server-supplied URL. The last one does not even need double decoding, only a front end that strips `;` parameters before resolving dot segments. The reasoning behind the accepted risk still holds. The record just understates what was accepted.
**Fix:** Widen the deferred-items entry to the whole class ("any path that names another org only after a second decode or a non-RFC-3986 normalisation"). Alternatively, refuse the cheap subset without refusing a literal `%` in names:
```go
for _, bad := range []string{"%252e", "%252f", "%255c", "%c0%ae", "%c0%af", "%e0%80%ae"} {
	if strings.Contains(p, bad) {
		return false
	}
}
```
A segment that equals `..` once its `;` parameter is removed (`..;x`) can be refused the same way.

### IN-08: deferred-items.md still says IN-05 was "checked live", but that check was waived

**File:** `.planning/phases/08-platform-upgrade-and-pacing/deferred-items.md:28`
**Issue:** The gap-closure table marks IN-05 as "Not deferred: checked live by 08-12 checklist item c". The user then waived 08-12 in full (08-12-SUMMARY:39, :135, :159). BACKLOG BL-02 has the correct state, but anyone reading deferred-items.md alone would take the attachment-host question as answered. It is not: no attachment download has run through the hardened host rule.
**Fix:** Change the row to "Waived 2026-10-21 (08-12 not run); open in BACKLOG BL-02, first Phase 9 live run".

### IN-09: A path refusal is reported as a host refusal, and wildcard routes with nested paths will hit it

**File:** `internal/transport/transport.go:154-159, 193-197, 427`; SDK `azuredevops/v7@v7.1.0/client.go:283, 292`
**Issue:** WR-01 lets `Allowed` refuse a URL whose host is bound to the org but whose path is not in canonical form. `RoundTrip` reports every refusal the same way:
- log line: "Refused to send credentials to a host outside the organization"
- error: `refused to send credentials to host "dev.azure.com": not bound to organization "acme"`

For a path refusal, both are wrong about the cause.

One SDK-generated case reaches this. `transformRouteTemplate` turns `{*path}` into `{path}` and applies `url.PathEscape`, so a nested classification path comes out as `%2F`. The probe `.../acme/proj/_apis/wit/classificationnodes/Areas/Team%20A%2FSub` gives `allowed=false`. Nothing reaches it today: `GetRecentIterations` (`internal/ado/client.go:280-284`) passes no `Path`, and project, repository and team names cannot contain `/` or `\`. A Phase 9 read that passes an area or iteration path would fail with a message pointing at the host.
**Fix:** Let the rule report which check refused the URL, for example `allowedReason(u) (ok bool, why string)` with `host` or `path`, and name the path problem in the error and the log line. At minimum, add a line to the `Allowed` doc saying an SDK wildcard route with a nested path is refused by design, and add a TestAllowed row that pins that case.

### IN-10: A dotted org on dev.azure.com widens the legacy host prefix across a label boundary

**File:** `internal/config/config.go:251, 264`; `internal/transport/transport.go:163-166`
**Issue:** `parseOrgURL` refuses a dotted label in the visualstudio.com form (`:264`) but accepts a dotted segment in the dev.azure.com form (`:251`). The probe shows `https://dev.azure.com/a.b` gives org `a.b`, and `https://dev.azure.com/...` gives org `...`. For org `a.b`, `Allowed` then admits `a.b.visualstudio.com` and `a.b.*.visualstudio.com`. In the legacy namespace those are service hosts of org `a`.

ADO org names cannot contain dots, so a working configuration never gets here: discovery for a non-existent org fails first. This is an asymmetry, not a live leak.

Separately, the doc comment at `config.go:218-220` ("structural rather than a character class") no longer fits the code. The PathEscape-stability check at `:258` now refuses `a!b`, `a'b` and similar names.
**Fix:**
```go
if !ok || segment == "" || strings.ContainsAny(segment, "/.") {
	return "", errOrgURLShape
}
```
This also makes the separate `.` and `..` checks redundant. Update the doc comment to say an org must also be PathEscape-stable and dot-free.

---

_Reviewed: 2026-10-21T21:19:53Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
