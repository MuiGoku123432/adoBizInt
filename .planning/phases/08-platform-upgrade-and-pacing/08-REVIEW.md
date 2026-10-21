---
phase: 08-platform-upgrade-and-pacing
reviewed: 2026-10-21T17:15:00Z
depth: standard
files_reviewed: 34
files_reviewed_list:
  - internal/transport/transport.go
  - internal/ado/retry.go
  - internal/ado/client.go
  - internal/ado/result.go
  - internal/ado/interfaces.go
  - internal/config/config.go
  - internal/guard/guard.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/throttle.go
  - scripts/smoke.sh
  - .github/workflows/ci.yml
  - go.mod
  - internal/ado/attachments.go
  - internal/ado/branches.go
  - internal/ado/dashboard.go
  - internal/ado/pipelines.go
  - internal/ado/preread.go
  - internal/ado/pullrequests.go
  - internal/ado/releases.go
  - internal/ado/workitems.go
  - internal/transport/transport_test.go
  - internal/ado/retry_test.go
  - internal/ado/v7_test.go
  - internal/ado/client_test.go
  - internal/ado/construct_test.go
  - internal/ado/result_test.go
  - internal/guard/guard_test.go
  - internal/config/config_test.go
  - cmd/adoBizInt/ratelimit_test.go
  - cmd/adoBizInt/throttle_test.go
  - cmd/adoBizInt/v7auth_test.go
  - cmd/adoBizInt/patleak_test.go
  - cmd/adoBizInt/write_test.go
  - internal/ui/workitems/resolve_test.go
findings:
  critical: 1
  warning: 5
  info: 6
  total: 12
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-10-21T17:15:00Z
**Depth:** standard
**Files Reviewed:** 34
**Status:** issues_found

## Narrative Findings (AI reviewer)

## Summary

I reviewed the v1 to v7 SDK swap, the org-bound transport (host rule, redirect refusal, pacer, D-01 fail-fast), the read-only retry decorators, the D-02 never-sent write classification, the org_url validator, the CLI throttle summary, the smoke script and CI. `go vet ./...` and `go test -race -count=1 ./...` pass. I also checked the v7.1.0 SDK source in the module cache and ran throwaway probes against a copy of `transport.go` outside the repo. No source files were changed.

What holds up:
- Writes are never retried. The decorators embed the interface and override only reads.
- The ErrNotSent-before-parentErr ordering in `guard.classify` is correct for every current `Mutate`. Each one issues at most one write, and any read inside a `Mutate` runs before that write. So I found no path where a write that was actually sent gets reported `failed` and left out of the audit.
- The pacer mutex and the atomic counters are race-free.
- Header parsing cannot overflow `time.Duration`.

What does not hold up:
1. The host rule is the security boundary this phase exists to ship, and it can be bypassed through Unicode case folding (CR-01).
2. The rule checks a decoded, cleaned path, but the wire carries the raw escaped path (WR-01).
3. A wait the pacer judges to "fit" can use up the whole per-call budget. The request then goes out with almost no time left and times out, which is exactly the outcome D-01 was written to prevent (WR-02).
4. A server-supplied wait has no upper bound when the request has no deadline (WR-03).
5. Two test-quality defects. One test pushes developers toward retrying writes (WR-04). The SC2 proof test fails under `-count=2` (WR-05).

The live smoke (PLAT-05) was waived, so none of the transport behaviour below has been seen on the wire.

## Critical Issues

### CR-01: Host rule bypass through Unicode case folding sends requests to an attacker-registrable domain

**File:** `internal/transport/transport.go:110-118` (also `:112` for the path compare, and `internal/config/config.go:229` for the same pattern)
**Issue:** `Allowed` decides with `strings.ToLower(u.Hostname())`. Go's `unicode.ToLower` folds U+0130 (`İ`) to ASCII `i` and U+212A (Kelvin sign) to ASCII `k`. `net/http`, however, dials the IDNA/punycode form of the original host. A probe against a copy of this exact file gave:

```
host="acme.vİsualstudio.com"  lower="acme.visualstudio.com"  Allowed=true  dialed="acme.xn--visualstudio-n4g.com:443"
```

`xn--visualstudio-n4g.com` is an ordinary `.com` domain, not Microsoft's. The `{org}.*.visualstudio.com` branch is open to the same trick (`acme.evil.vİsualstudio.com`). The org-path compare in the dev.azure.com branch is open too (`/\u212Acme/...` matches org `kcme`).

A second probe sent a 307 from `dev.azure.com/acme` to `https://acme.v%C4%B0sualstudio.com/y`. The transport passed the hop to the base transport along with the PATCH body. Only Go's own redirect header stripping kept `Authorization` off that request, because it compares IDNA-ASCII hosts. The transport doc and the comment on `TestRefuseRedirectHopIsNotNotSent` say the per-hop check exists precisely so the code does not rely on that stdlib behaviour.

How reachable is a PAT leak today?
- First-hop URLs are safe for now. Every one is built from `strings.ToLower`'d base URLs, which fold the same way and come out as plain ASCII.
- Any future first-hop URL built from server data without that normalisation would attach the PAT and pass the rule. Examples: following `_links`, or an attachment URL.
- Redirect hops already leak request bodies (write payloads, approval comments) to the foreign domain.

The TestAllowed table has no non-ASCII row.

**Fix:** refuse any non-ASCII host or path before folding, and add test rows:
```go
// asciiLower lowercases s, or reports false when s holds any non-ASCII byte:
// unicode case folding maps U+0130 and U+212A to ASCII while net/http dials
// the IDNA form of the original, so a folded match is not the dialed host.
func asciiLower(s string) (string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return "", false
		}
	}
	return strings.ToLower(s), true
}

host, ok := asciiLower(u.Hostname())
if !ok {
	return false
}
...
p, ok := asciiLower(path.Clean("/" + u.Path))
if !ok {
	return false
}
```
Test rows to add, each `want: false`:
- `https://acme.v%C4%B0sualstudio.com/x`
- `https://acme.evil.v\u0130sualstudio.com/x`
- `https://dev.azure.com/\u212Acme/x` with org `kcme`

`parseOrgURL` should refuse non-ASCII for the same reason.

## Warnings

### WR-01: Allowed validates a decoded, cleaned path that is not the path sent on the wire

**File:** `internal/transport/transport.go:112-113`
**Issue:** The org check runs on `path.Clean("/" + u.Path)`. `u.Path` is percent-decoded, and the dot segments are then resolved. Go sends `u.EscapedPath()` unmodified. A probe gave:

```
https://dev.azure.com/other/../acme/_apis      Allowed=true  wire="/other/../acme/_apis"
https://dev.azure.com/other%2F..%2Facme/_apis  Allowed=true  wire="/other%2F..%2Facme/_apis"
```

In the second case the first path segment the server receives is `other%2F..%2Facme`, not `acme`. How ADO interprets that depends on how its front end decodes `%2F` and resolves dot segments. Because the host is unchanged, Go also forwards `Authorization` on a same-host redirect to such a URL. That is the exact "another organization on the same host" case D-09 exists to refuse. The rule should decide on the bytes it lets out, not on a normalised view of them.

**Fix:** reject non-canonical paths instead of normalising them, and compare against the escaped form:
```go
raw := "/" + strings.TrimPrefix(u.EscapedPath(), "/")
lower := strings.ToLower(raw)
if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
	return false // an encoded separator hides a segment boundary
}
if c := path.Clean(raw); c != raw && c+"/" != raw {
	return false // dot segments or empty segments: refuse, do not resolve
}
org := "/" + strings.ToLower(url.PathEscape(t.org))
return lower == org || strings.HasPrefix(lower, org+"/")
```
Add the two URLs above to TestAllowed with `want: false`.

### WR-02: A hold that "fits" consumes the per-call budget, so the request is sent with almost no time left

**File:** `internal/transport/transport.go:180-189` (pinned by `transport_test.go:971`, the "plus 1ns" row)
**Issue:** `http.Client.Timeout` (the configured `request_timeout`, default 30s) is installed as the request-context deadline before `RoundTrip`, so the hold and the actual HTTP exchange share one budget. `hold` refuses only when `wait >= left`. With `Retry-After: 29` and the default 30s budget, the request is held 29s and then sent with about 1s left.

A probe used a 2.1s budget, a 2s wait and a 300ms server. The request was held, sent, and failed with `Client.Timeout exceeded`: `ErrNotSent=false`, `ErrRateLimited=false`, `DeadlineExceeded=true`. The consequences:
- A read classifies as `timeout`, not `rate_limit`. D-01 says "Waiting into a timeout hides the real cause and pushes agents to retry harder (Pitfall 9)", and that is what happens here.
- A write classifies as `unknown`: audited, exit 5. A clean `failed` (not sent) was available.

TestFailFastBoundary's "plus 1ns, wantSent: true" row pins this behaviour as correct.

**Fix:** require headroom for the send itself. One option is a fixed minimum:
```go
// minSendBudget is the least time a request must still have once its hold ends;
// a hold that leaves less is refused as rate limited (D-01), because sending
// into a near-expired deadline turns a throttle into a timeout or unknown write.
const minSendBudget = 5 * time.Second

if left := time.Until(deadline); wait+minSendBudget > left {
	// refuse as reasonRateLimited, as today
}
```
The other option is to start the per-call budget after the hold, which means moving `requestTimeout` out of `http.Client.Timeout` and into a per-request context created after `hold` returns. Update the boundary test either way.

### WR-03: No upper bound on a server-supplied wait; an unbounded silent hang when there is no deadline

**File:** `internal/transport/transport.go:169-197, 248-266`; `internal/ado/client.go:78-85, 136`; `internal/config/config.go:173-207`
**Issue:** `retryAfter` accepts up to about 292 years, from integer seconds or an HTTP-date. `hold` only fails fast when the context has a deadline. `NewClient` runs discovery and identity on `context.Background()`. `request_timeout: 0s` is accepted by `Validate`, and `HTTPClient()` documents zero as "no per-call bound". With that configuration, one startup response carrying `Retry-After: 86400` (or `X-RateLimit-Delay`) makes the process sleep for a day before the TUI renders or the CLI dispatches, with nothing on stderr.

Even with a deadline, `notBefore` is process-wide and never decays. In a long-lived TUI session, one oversized value makes every later request fail fast as `rate_limit` until restart. 08-CONTEXT lists "any cap on a single honoured Retry-After" as Claude's Discretion; none was applied.

**Fix:** cap a single instruction and treat anything above the cap as a fail-fast, independent of the deadline. Also reject a non-positive `request_timeout` in `Validate`, or give startup a bounded context:
```go
// maxHonouredWait bounds one server instruction. A longer one is refused as
// rate limited rather than held, so no header can park the process.
const maxHonouredWait = 5 * time.Minute

if wait > maxHonouredWait {
	closeBody(req)
	return &notSentError{reason: reasonRateLimited, host: host, org: t.org, hop: hop, wait: wait}
}
```

### WR-04: TestRetryNeverWrapsWrites pushes developers to retry any write it does not already know about

**File:** `internal/ado/retry_test.go:25-31, 288-297`
**Issue:** retry.go's design note says "forgetting an override costs a retry, never a duplicate write". The test inverts that. It classifies methods with a hard-coded `writeMethods` map and requires every other interface method to have a retrying override. Suppose a later phase adds a write to an interface, such as `CreateWorkItem` or `AddComment`, and does not also add it to this map. The test then fails with "does not override the read workItemAPI.AddComment, so it is never retried", and the obvious way to make it pass is to add a retrying override for a write. That is a double-mutation hazard under one audit record (T-08-26).

**Fix:** classify both ways explicitly and fail on any method in neither list:
```go
var readMethods = map[string]bool{"GetBranches": true, /* ...every read... */}
for m := range ifaces[iface] {
	switch {
	case writeMethods[m] && readMethods[m]:
		t.Errorf("%s is listed as both read and write", m)
	case !writeMethods[m] && !readMethods[m]:
		t.Errorf("%s.%s is unclassified: add it to readMethods or writeMethods before deciding whether it may retry", iface, m)
	case readMethods[m] && !got[m]:
		t.Errorf("%s does not override the read %s.%s", decorator, iface, m)
	}
}
```

### WR-05: The SC2 exit-code proof fails when run more than once in a process

**File:** `cmd/adoBizInt/v7auth_test.go:78-83, 109-112`
**Issue:** The per-subtest org comes from `t.Name()`, and the test asserts exactly `[OPTIONS ..., data request]`. The SDK's `apiResourceLocationCache` is a process global keyed by base URL. On a second iteration the cache is already populated, so OPTIONS is never sent. Verified:

```
go test -count=2 -run TestV7AuthFailureExitsAuth ./cmd/adoBizInt
--- FAIL: .../read_json_body: requests = ["GET /.../p/_apis/git/repositories"], want ["OPTIONS ...", "GET ..."]
```

All four rows fail. `-count=N` is the standard flake hunt, so the test that proves SC2 fails the moment anyone runs it. The `internal/ado` tests that use `uniqueOrgURL` pass under `-count=2` only because they do not assert OPTIONS.

**Fix:** make the org unique per invocation, not per name. One option is a package-level `atomic.Int64` suffix: `org.WriteString(strconv.FormatInt(seq.Add(1), 10))`. The other is to assert only that the last request is the data route.

## Info

### IN-01: parseOrgURL accepts a trailing `#` and dot-segment org names, contrary to its doc comment

**File:** `internal/config/config.go:225-236`
**Issue:**
- `url.Parse("https://dev.azure.com/example#")` leaves `Fragment` empty, so the `u.Fragment != ""` check passes. The SDK then concatenates `.../example#/_apis`, so every request goes to `/example`, with the route sitting in the fragment. Startup then fails with a confusing discovery error rather than the named org_url error. The comment says a fragment "is refused".
- `https://dev.azure.com/.` and `/..` validate, as orgs `.` and `..`. They fail closed later, because `Allowed` can never match `/.`, but the error is the wrong one.

**Fix:** reject `strings.ContainsAny(raw, "#")` alongside `ForceQuery`, and reject `segment == "." || segment == ".."`.

### IN-02: The D-08 fail-fast message can contradict itself after rounding

**File:** `internal/transport/transport.go:344, 352-354`
**Issue:** `wholeSeconds` rounds to the nearest second. A wait of 5s with 4.6s left prints "ADO asked for 5s, 5s left in budget", which reads as a wait that fits being refused. Sub-second values print "0s, 0s".
**Fix:** round the wait up and the time left down, for example `ceil` for `wait` and `floor` for `left`, so the printed pair always shows wait > left.

### IN-03: The retry log line names neither the operation nor the resource

**File:** `internal/ado/retry.go:65`
**Issue:** D-05 asks the log to carry the resource and the retry count for a throttled run. "Retrying rate-limited read retry=1 max=2" gives no way to tell which of the dozens of fan-out reads was throttled. The transport line carries the host and `X-RateLimit-Resource`, but nothing connects the two lines.
**Fix:** pass an operation label into `retryRead`, or log the WrappedError's `TypeKey` and status, alongside `retry`.

### IN-04: A 429 with no pacing header is re-issued immediately, twice

**File:** `internal/ado/retry.go:52-66`
**Issue:** If a 429 carries neither `Retry-After` nor `X-RateLimit-Delay`, which can happen when a proxy or front end rather than ADO itself returns it, `notBefore` does not move. The two retries then go out back to back, hammering a throttled endpoint. This follows D-04 ("no invented threshold") to the letter, but it is worth a conscious decision.
**Fix:** either document this as accepted, or apply a small floor (for example 1s) to a retry whose 429 carried no instruction.

### IN-05: Foreign-host redirect refusal also blocks credential-free hops; the attachment download path is unverified live

**File:** `internal/transport/transport.go:131-137`; `internal/ado/attachments.go:102`
**Issue:** Go already strips `Authorization` on a cross-domain redirect. So refusing a hop to, say, a pre-signed blob-storage URL protects no credential, yet it breaks the download. 08-RESEARCH A3 assumes attachment content is served from the org host, and checklist item 3 of 08-SMOKE.md (the TUI attachment download) was waived. If ADO redirects attachment content off-org, TUI downloads will fail as "refused to send credentials" after this phase.
**Fix:** run checklist item 3 before Phase 9. If it is refused, decide explicitly whether a hop that carries no `Authorization` may leave the org.

### IN-06: areaURL still documents and implements the on-prem empty-list fallback that D-12 removed

**File:** `internal/ado/client.go:176-187`
**Issue:** D-12 makes org_url cloud-only, but an empty `ResourceAreas` answer still silently builds every sub-client, release included, on `org_url`. For a cloud org that is a malformed answer, not the on-prem case. The result is a confusing `LocationIdNotRegistered` error later, instead of a clear startup error now.
**Fix:** treat an empty area list as a startup error naming the org, or at least update the comment so the fallback is not described as a supported mode.

---

_Reviewed: 2026-10-21T17:15:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
