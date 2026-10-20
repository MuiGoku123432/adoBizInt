---
phase: 08-platform-upgrade-and-pacing
plan: 05
subsystem: api
tags: [go, transport, pacing, retry-after, x-ratelimit, synctest, plat-03, d-04, d-05]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-02: internal/transport (Transport, New, HTTPClient, Allowed, RoundTrip, ErrNotSent, notSentError); 08-03: transport_test.go helpers (rtFunc, emptyResp, basicAuth, closeRecorder) and the v7_test.go NewClient fixtures"
provides:
  - "Header-driven pacer in internal/transport: one mutex-guarded process-wide notBefore, set from Retry-After (integer seconds or http.ParseTime date) and X-RateLimit-Delay (decimal seconds) as a maximum on any response status"
  - "RoundTrip holds an allowed request until notBefore, re-checking after each sleep; a request whose context ends while held returns a notSentError{reason: held} that is ErrNotSent on a first hop and unwraps to ctx.Err()"
  - "(*Transport).Stats() (waits int64, waited time.Duration): holds actually taken and their total (D-05 counters for 08-07)"
  - "Logging to adoBizInt.log: one wait line (wait, host) per hold taken; one rate-limit line (status, resource, retry_after, delay, remaining, limit) only when Retry-After, X-RateLimit-Delay or X-RateLimit-Remaining is present; one line per ignored instruction"
  - "notSentError gains reason (host | held), cause and Unwrap(); still no Timeout method"
  - "Tests: TestRetryAfterOn200PacesNextRequest, TestSessionIDUnchanged (v7_test.go); 11 pacer tests and the captureLog/logLines/send/gapAfter helpers (transport_test.go)"
affects: [08-06, 08-07, 08-08, phase-09]

# Actuals (#2632)
actuals:
  tokens: 7041
  tasks: 3
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pacer tests run inside testing/synctest.Test over a fake base rtFunc: gaps are asserted exactly (7s, 125ms, 5.5s) with zero wall time; never httptest.NewServer inside a bubble"
    - "gapAfter(t, header...) measures the hold of the second of two requests after the first response's headers; send(c, url) returns errors so goroutines can call it"
    - "captureLog(t) points slog.Default at a buffer and restores it; logging.Init must never run in transport tests"
    - "Parse helpers return (time.Duration, error): an absent header is (0, nil), a present but unusable one is (0, err), which observe logs and ignores"

key-files:
  created: []
  modified:
    - internal/transport/transport.go
    - internal/transport/transport_test.go
    - internal/ado/v7_test.go

key-decisions:
  - "Process-wide single notBefore (no per-resource map): ADO budgets per identity; instructions combine as a maximum, never a sum, so the same instruction twice does not stack"
  - "X-TFS-Session is not stripped (RESEARCH Pitfall 1): Go canonicalizes the key so the SDK's raw-key write never runs; TestSessionIDUnchanged pins it and fails if a future SDK canonicalizes its lookup"
  - "The rate-limit header line is logged by a sibling helper logRateLimit(resp.StatusCode, resp.Header) called next to t.observe(resp.Header), because observe's fixed signature carries no status; observe itself logs ignored instructions"
  - "An instruction longer than a time.Duration can hold (Retry-After > 9223372036s, or a non-finite X-RateLimit-Delay) is ignored and logged rather than clamped, matching 'unparseable values are logged and ignored'"
  - "Tasks 2 and 3 ran as TDD with separate test and feat commits; Task 1 is the tracer and its automated verify passed on the committed tree, so execution continued without a mid-flight halt (as in 08-01..08-04)"

patterns-established:
  - "Tripwire per pinned behaviour, restored byte-identically (sha256 checked), never committed: 13 tripwires across the three tasks, each failing the intended test"

requirements-completed: [PLAT-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Retry-After on an HTTP 200, seen through a real v7 client built by NewClient over the transport, holds the next request by exactly 7s"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestRetryAfterOn200PacesNextRequest"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every explicit header form: HTTP-date (10s), X-RateLimit-Delay at sub-second precision (125ms), max not sum (5.5s), same instruction twice does not stack (5s not 10s), X-RateLimit-Remaining/-Limit/-Resource never change timing, and negative, garbage, past, NaN, infinite and overflowing values are ignored"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceRetryAfterHTTPDate"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceRateLimitDelay"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceMaxNotSum"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceSameInstructionDoesNotStack"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceRemainingIgnored"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceIgnoresInvalid (7 subtests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Concurrency and held requests: eight concurrent requests all wait for notBefore, each reaches the base exactly once, no data race under -race; a request whose context is already cancelled or whose deadline ends while held is not sent, closes its body, and is both ErrNotSent and the context error"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "go test -race internal/transport#TestPaceConcurrent"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestHeldCancelIsNotSent (2 subtests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-05 counters and log: Stats() counts only holds actually taken; each hold logs one wait line with wait and host; rate-limit headers log one bounded line, never for the cost header alone and never with the URL, query or Authorization"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestStats"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceLogsWaitTaken"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestPaceLogsRateLimitHeadersOnly"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestRefusalNeverLogsPAT, #TestTransportNeverPrints (unchanged, still pass)"
        status: pass
    human_judgment: false
  - id: D5
    description: "SessionId regression pin: a real v7 call whose responses carry X-Tfs-Session leaves azuredevops.SessionId unchanged"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestSessionIDUnchanged"
        status: pass
    human_judgment: false
  - id: D6
    description: "Phase 7 locks preserved"
    verification:
      - kind: other
        ref: "shasum -a 256 -c 08-cli-tests.sha256 -> 7 OK; 07-cli-audit-test.sha256 -> 1 OK"
        status: pass
    human_judgment: false

# Metrics
duration: 11min
completed: 2026-10-20
status: complete
---

# Phase 8 Plan 05: Header-Driven Pacer Summary

**The transport now holds every request until ADO's last instruction has passed: Retry-After (seconds or HTTP-date) and X-RateLimit-Delay (sub-second), on any status including 200, combined as a maximum in one mutex-guarded process-wide notBefore. It counts and logs each wait it takes (Stats, adoBizInt.log), logs rate-limit headers only when they carry something, and a request whose context ends while held is reported not-sent and wraps the context error.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-10-20T16:19:25Z
- **Completed:** 2026-10-20T16:30:59Z
- **Tasks:** 3
- **Files modified:** 3 (757 insertions, 17 deletions)

## Accomplishments

- `RoundTrip` order is: host rule, then `hold`, then the base exactly once, then `logRateLimit` and `t.observe(resp.Header)` on every response. The transport never reads, replays or re-sends a body. Its only body access is `closeBody` on the two not-sent paths.
- `hold` loops: it computes the wait under the mutex, sleeps on a timer or the request context, and re-checks, because another response may move `notBefore` during the sleep. Each completed sleep adds to `waits` and `waitedNanos` (atomics) and logs `wait=<duration> host=<host>`.
- `observe` parses `Retry-After` with `strconv.Atoi`, then `http.ParseTime`, and `X-RateLimit-Delay` with `strconv.ParseFloat` rounded to the nanosecond. It takes `max(retry, delay)` and moves `notBefore` only forward.
- `notSentError` now has `reason` (`host` or `held`), `cause` and `Unwrap()`. A held first-hop request satisfies both `errors.Is(err, ErrNotSent)` and `errors.Is(err, context.Canceled)` or `context.DeadlineExceeded`. A held redirect hop is not `ErrNotSent` (08-02 hop rule unchanged).
- First end-to-end pacing proof through a real v7 SDK client: `NewClient` over `transport.New` in a synctest bubble shows the second `GetRepositories` sent exactly 7s after the first.

## Task Commits

1. **Task 1: Retry-After on an HTTP 200 paces the next request, end to end through a real v7 client** - `d88c280` (feat, tracer)
2. **Task 2: Every explicit header form** - `f05f215` (test, RED) and `cb7f2ac` (feat, GREEN)
3. **Task 3: Shared notBefore under concurrency, held release, wait logging and Stats, SessionId pin** - `08c0d33` (test, RED) and `deb973f` (feat, GREEN)

**Plan metadata:** recorded in the `docs(08-05)` summary and completion commits

## Files Created/Modified

- `internal/transport/transport.go` - `mu`, `notBefore`, `waits`, `waitedNanos`; `hold`, `untilNotBefore`, `observe`, `retryAfter`, `rateLimitDelay`, `maxWaitSeconds`, `logRateLimit`, `closeBody`, `Stats`; `notSentReason` and the extended `notSentError`. The package doc now describes pacing.
- `internal/transport/transport_test.go` - `pacedURL`, `send`, `gapAfter`, `captureLog`, `logLines`; 11 pacer tests.
- `internal/ado/v7_test.go` - `TestRetryAfterOn200PacesNextRequest`, `TestSessionIDUnchanged`; imports `testing/synctest`.

## Tripwire and Gate Runs

All tripwires below were uncommitted edits, and each file's sha256 was checked identical after restoring.

| Run | Change | Result |
|-----|--------|--------|
| Task 1 tripwire | `t.observe(resp.Header)` removed | `TestRetryAfterOn200PacesNextRequest` FAIL (gap 0s) |
| Tracer gate | Task 1 `<verify>` on the committed tree | PASS (7s gap, race suite, locks 7/1) |
| Task 2 RED | tests before HTTP-date/delay parsing | HTTPDate, RateLimitDelay, MaxNotSum and the overflow row FAIL; others pass (max and invalid rows already held from Task 1) |
| Task 2 tripwires | `retry + delay`; additive stacking of notBefore; acting on `X-RateLimit-Remaining: 0` | MaxNotSum, SameInstructionDoesNotStack, RemainingIgnored each FAIL |
| Task 3 RED | tests before Stats/logging (compile RED: `tr.Stats undefined`); runtime RED with a temporary `Stats` stub | TestStats, TestPaceLogsRateLimitHeadersOnly, TestPaceLogsWaitTaken FAIL; Concurrent and HeldCancel pass (Task 1 behaviour pinned) |
| Task 3 tripwires | mutex removed; hold removed; base called twice; held body not closed; held cause dropped; wait not counted; cost header logged; URL logged; wait line dropped | each FAILs its target test (`race detected` for the mutex removal) |
| SessionId tripwire | fixture writes the raw non-canonical `X-TFS-Session` key | `TestSessionIDUnchanged` FAIL (SessionId overwritten) |

## Decisions Made

- One process-wide `notBefore`, combined as a maximum (Claude's Discretion, RESEARCH Pattern 4).
- `X-TFS-Session` is left alone, and `TestSessionIDUnchanged` is the regression pin (Pitfall 1).
- `logRateLimit` sits beside `observe`, because the planned `observe(h http.Header)` signature cannot log `status`.
- Overflowing and non-finite instructions are ignored and logged rather than clamped. With no cap, a legitimate huge wait is still obeyed until the request's context ends. 08-06's deadline fail-fast turns that into an immediate `rate_limit`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Overflowing Retry-After wrapped into a 55660h hold**
- **Found during:** Task 2 RED (an added `TestPaceIgnoresInvalid` row)
- **Issue:** Task 1's `time.Duration(secs) * time.Second` overflows for `Retry-After: 99999999999999`, wraps, and held the next request for 55660h.
- **Fix:** `maxWaitSeconds = math.MaxInt64 / int64(time.Second)`; larger integer values and non-finite or oversized `X-RateLimit-Delay` values are rejected as unusable (logged, ignored).
- **Files modified:** internal/transport/transport.go, internal/transport/transport_test.go
- **Verification:** `TestPaceIgnoresInvalid/overflowing_Retry-After`, `/NaN_X-RateLimit-Delay`, `/infinite_X-RateLimit-Delay` pass
- **Committed in:** f05f215 (rows), cb7f2ac (fix)

**2. [Rule 2 - Missing critical] Extra invalid-value and held-deadline rows**
- **Found during:** Tasks 2 and 3
- **Issue:** `strconv.ParseFloat` accepts `NaN` and `+Inf`, whose conversion to `time.Duration` is undefined. The planned held test covered an already-cancelled context only, not a deadline that ends during the hold.
- **Fix:** Added the NaN, +Inf and overflow rows to `TestPaceIgnoresInvalid`, and a `deadline ends while held` subtest (held exactly 10s, wraps `context.DeadlineExceeded`, body closed) to `TestHeldCancelIsNotSent`.
- **Files modified:** internal/transport/transport_test.go
- **Committed in:** f05f215, 08c0d33

### Plan-text adjustments

**3. [Interface] The rate-limit line is logged outside `observe`**
- The plan puts the status/resource/retry_after/delay/remaining/limit line in `observe(h http.Header)`, but that signature carries no status. `logRateLimit(resp.StatusCode, resp.Header)` is called on the line before `t.observe(resp.Header)`, so the key_link pattern and the logged keys both hold. `observe` still logs each ignored instruction.

**4. [Test design] TestPaceConcurrent answers each concurrent request with Retry-After 1**
- With no instruction on the eight answers, `notBefore` would only be read concurrently and `-race` could not catch a missing mutex. Each answer now writes `notBefore` while other requests read it. The timing assertion uses the Retry-After 4 lower bound, which holds under any interleaving. The mutex-removal tripwire is reported as `race detected`.

### Process

**5. [TDD] Tasks 2 and 3 landed as test and feat commit pairs** (`f05f215`/`cb7f2ac`, `08c0d33`/`deb973f`), not the plan's single feat commits. The Task 3 RED commit does not compile (`Stats` undefined). Runtime RED was shown with an uncommitted stub.

**6. [Tracer gate] No mid-flight human-verify halt.** Auto mode is not active, but the tracer `<verify>` is fully automated and passed on the committed tree. Same handling as 08-01..08-04.

**7. [Phase 7 gate] Not checked here.** Per the user decision of 2026-10-20, it applies only to 08-08.

---

**Total deviations:** 7 (1 bug fix, 1 missing-critical test addition, 2 plan-text adjustments, 3 process). **Impact:** none on scope. Every acceptance criterion of the three tasks passes as written.

## Issues Encountered

- `internal/transport/transport.go` is now 314 lines and holds two responsibilities: the host rule and the pacer. CONVENTIONS treats crossing 150 lines as a split signal, not a defect. The plan's artifacts pin `Stats` and `t.observe(resp.Header)` to this file, so it was not split. A `pace.go` split is a candidate when 08-06 adds the fail-fast.
- `TestRefusalNeverLogsPAT` keeps its inline slog capture. The new `captureLog` helper serves the two new log tests. The 08-03 test was left byte-identical rather than refactored.
- The plan estimated 66000 tokens; the realized diff is about 7000 (chars/4 over added lines).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 08-06 adds `ErrRateLimited` and the D-01 fail-fast. One fact for it: once `http.Client.Timeout` fires, `Client.do` replaces any RoundTrip error with its own `timeoutError` (`net/http/client.go:740-746`), which drops `ErrNotSent`. A request still held when the client timer fires therefore reads as a timeout, and a write would read `unknown`. The fail-fast has to decide before the hold, and again in the `hold` loop when `notBefore` moves later, so no hold outlives the deadline.
- 08-07 reads `(*Transport).Stats()` after dispatch for the stderr summary line. The transport never writes stderr (`TestTransportNeverPrints` covers the new code).
- PLAT-03 stays Pending in REQUIREMENTS.md: `requirements.ready-ids` reports 0/1 because later Phase 8 plans also declare it.
- Plan-level verification: `go build ./... && go vet ./... && go test -race ./...` (rc 0, 16 packages ok), gofmt clean, 11 pacer tests pass under `-race` with no DATA RACE, locks 7/7 and 1/1.

## Self-Check: PASSED

- FOUND: internal/transport/transport.go, internal/transport/transport_test.go, internal/ado/v7_test.go
- FOUND commits: d88c280, f05f215, cb7f2ac, 08c0d33, deb973f (no file deletions since 84f3e26)
- must_haves: `func (t *Transport) Stats()` (1), `TestPaceConcurrent` and `TestRetryAfterOn200PacesNextRequest` present, `t.observe(resp.Header)` (1), `synctest.Test(` in v7_test.go (1)
- Prohibition: the only `Body` use in transport.go is `closeBody`; TestPaceConcurrent asserts one base call per request
- Criteria greps: `X-TFS-Session|X-Tfs-Session` in transport.go 0, `Timeout()` 0, `http.ParseTime` 2, `ParseFloat` 1, `wait=2s` in transport_test.go 1
- TDD gate: `test(08-05)` f05f215 precedes `feat(08-05)` cb7f2ac; `test(08-05)` 08c0d33 precedes `feat(08-05)` deb973f
- No stub markers (TODO, FIXME, placeholder) in modified files

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-20*
