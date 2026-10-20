---
phase: 08-platform-upgrade-and-pacing
plan: 06
subsystem: api
tags: [go, transport, pacing, fail-fast, rate-limit, guard, exit-codes, synctest, plat-03, d-01, d-02, d-08]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-05: header-driven pacer (notBefore, hold loop, Stats, notSentError held/host reasons); 08-02: ErrNotSent first-hop rule; 08-01: cmd rtFunc twin and wrappedErrorValue"
provides:
  - "transport.ErrRateLimited and the D-01 fail-fast: before every sleep, hold refuses a wait equal to or longer than the time left before the request deadline, with the D-08 text 'rate limited: ADO asked for Ns, Ms left in budget'"
  - "classifyFailure maps transport.ErrRateLimited to rate_limit before the cancel and deadline branches"
  - "guard.classify checks transport.ErrNotSent first: refused-host, fail-fast and held-then-cancelled writes are failed and unaudited under any parent; redirect hops stay unknown"
  - "Test helpers for 08-07: failFastErr, heldCancelErr, refusedHostErr, primedGitClient (internal/ado/v7_test.go)"
  - "TestRateLimitedWriteExitsOne (cmd/adoBizInt/ratelimit_test.go): 429, refused host and fail-fast writes run once, are failed, exit 1"
affects: [08-07, 08-08, phase-09]

# Actuals (#2632)
actuals:
  tokens: 8054
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "The pacer decides fit before every sleep, not only the first: a hold never outlives the deadline, so http.Client's own Timeout can never replace a not-sent error with its timeoutError"
    - "An already-ended context wins over the fail-fast: it reports its own cause (cancel or deadline), so an interrupted read stays other, not rate_limit"
    - "Not-sent fixtures in other packages are built from a real transport over an rtFunc base, never constructed by hand, so they carry the real notSentError and url.Error wrapping"

key-files:
  created:
    - cmd/adoBizInt/ratelimit_test.go
  modified:
    - internal/transport/transport.go
    - internal/transport/transport_test.go
    - internal/ado/result.go
    - internal/ado/result_test.go
    - internal/ado/v7_test.go
    - internal/guard/guard.go
    - internal/guard/guard_test.go

key-decisions:
  - "Fail-fast is decided inside the hold loop before every sleep (08-05 handoff): a later response that moves notBefore past the deadline fails the held request when its current hold ends, so no hold reaches the client timer"
  - "ctx.Err() is checked before the fail-fast: a cancelled or already-expired context returns the held error wrapping its own cause, so a signal never reads as rate_limit"
  - "The D-08 message prints whole seconds ('ADO asked for 60s'), not Duration.String ('1m0s'), so agents read one unit; the Warn log line keeps full precision (asked, left, host)"
  - "A hold actually taken before a later fail-fast is still counted in Stats; an immediate fail-fast adds nothing"
  - "08-05's 'deadline ends while held' row became 'cancelled while held': under D-01 a deadline can no longer end during a hold"

patterns-established:
  - "Tripwire per pinned behaviour, restored byte-identically (sha256 checked), never committed: 9 tripwires, each failing its target test"

requirements-completed: [PLAT-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "A real v7 read whose throttle instruction (Retry-After 60) does not fit its 1s deadline is refused before send and classifies rate_limit with 'ADO asked for' and 'left in budget' in its text"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestFailFastReadClassifiesRateLimit"
        status: pass
    human_judgment: false
  - id: D2
    description: "Boundary probe: with Retry-After 5, 5s-1ns and exactly 5s left fail fast without calling the base; 5s+1ns waits exactly 5s and sends"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastBoundary (minus 1ns, equal, plus 1ns)"
        status: pass
    human_judgment: false
  - id: D3
    description: "http.Client.Timeout alone is a deadline for the fail-fast (45s asked, 10s left); a fail-fast adds nothing to Stats and logs one Warn line with asked/left/host and no wait line; a hold overtaken by a later Retry-After fails fast when it ends, keeping ErrNotSent instead of the client's timeout error"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastClientTimeoutIsDeadline"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastNotCounted"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastWhenNotBeforeMovesPastDeadline"
        status: pass
    human_judgment: false
  - id: D4
    description: "Read classifications: held-then-cancelled is other, refused host is other and names evil.example, 'rate limited after 2 retries' wrapping a 429 is rate_limit; held errors never satisfy ErrRateLimited; an expired deadline or a cancel wins over the fail-fast"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/result_test.go#TestClassifyFailure (4 new rows)"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestHeldCancelIsNotSent (4 rows)"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-02 in guard.classify: refused-host, D-01 fail-fast and held-then-cancelled writes are failed with zero audit records even under a cancelled parent; a redirect-hop refusal stays unknown with one audit record; a write 429 is failed and unaudited"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/guard/guard_test.go#TestRunNotSent (5 rows)"
        status: pass
      - kind: unit
        ref: "internal/guard/guard_test.go#TestRunClassifiesOutcomes/429"
        status: pass
      - kind: unit
        ref: "internal/guard/structural_test.go#TestGuardNeverPrintsOrExits"
        status: pass
    human_judgment: false
  - id: D6
    description: "SC3 write half at the CLI: a 429, a refused host and a D-01 fail-fast on a write mutation each run Mutate exactly once, are failed with no audit record, and writeExitCodeFor maps them to exit 1, never 4"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/ratelimit_test.go#TestRateLimitedWriteExitsOne (429, refused_host, fail-fast)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Phase 7 locks preserved and full suite green"
    verification:
      - kind: other
        ref: "shasum -a 256 -c 08-cli-tests.sha256 -> 7 OK; 07-cli-audit-test.sha256 -> 1 OK; go build/vet/test -race ./... rc 0 (16 packages); gofmt clean"
        status: pass
    human_judgment: false

# Metrics
duration: 13min
completed: 2026-10-20
status: complete
---

# Phase 8 Plan 06: Deadline Fail-Fast and Never-Sent Writes Summary

**When ADO asks for a wait that does not fit the request's deadline, the transport now refuses the request at once with `rate limited: ADO asked for 60s, 1s left in budget`, which classifies `rate_limit`, not `timeout`. It decides before every sleep, so no hold can outlive the deadline and lose `ErrNotSent` to `http.Client`'s own timeout error. `guard.classify` checks `transport.ErrNotSent` before `parentErr`, so a write refused by the host rule, refused by the pacer, or held and then cancelled is `failed`, unaudited and exits 1, while a redirect-hop refusal stays `unknown` and audited.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-10-20T16:33:22Z
- **Completed:** 2026-10-20T16:46:30Z
- **Tasks:** 3
- **Files modified:** 8 (1 created, 7 modified; 814 insertions, 36 deletions)

## Accomplishments

- `hold` now runs, on every loop pass: no wait, return; context already ended, return the held error wrapping `ctx.Err()`; wait `>=` time left before `ctx.Deadline()`, close the body, log one Warn line (`asked`, `left`, `host`) and return `notSentError{reason: rate_limited}`; otherwise sleep and re-check. The deadline already includes `http.Client.Timeout`, which `setRequestCancel` installs on the request context for a custom RoundTripper.
- `notSentError.Is` matches `ErrRateLimited` for the fail-fast on any hop, and `ErrNotSent` on a first hop only (unchanged). The fail-fast wraps nothing and still has no `Timeout` method.
- `classifyFailure` maps `transport.ErrRateLimited` to `rate_limit` first. The belt-and-braces deadline comment now names `http.Client.Timeout` on the client `internal/transport` builds instead of the old SDK connection field.
- `guard.classify`'s first statement is the `ErrNotSent` check. Its doc comment states the new precedence (not sent, then `parentErr`, then kind) and why. The call site and the audit rule are unchanged, so failed stays unaudited (Phase 4 D-14/D-16).
- The 08-05 handoff is proven fixed: `TestFailFastWhenNotBeforeMovesPastDeadline` holds a request 5s under a 10s `Client.Timeout`, lets a slow response push `notBefore` 60s out at 2s, and gets the fail-fast at exactly 5s. With the decision made only before the first sleep (tripwire), the same test gets `context deadline exceeded (Client.Timeout exceeded while awaiting headers)` at 10s, with `ErrNotSent` lost.

## Task Commits

1. **Task 1: Fail fast past the deadline as rate_limit (tracer)** - `1394800` (feat)
2. **Task 2: Fail-fast boundary, deadline sources, read classifications** - `4332d2e` (test)
3. **Task 3: Never-sent writes are failed** - `a8945ee` (test, RED) and `17154c1` (fix, GREEN)

**Plan metadata:** recorded in the `docs(08-06)` summary and completion commits

## Files Created/Modified

- `internal/transport/transport.go` - `ErrRateLimited`; `reasonRateLimited`; `wait`/`left` fields; fail-fast and `ctx.Err()` checks in `hold`; D-08 `Error()` text; `wholeSeconds`; extended `Is`; package and `RoundTrip` docs.
- `internal/transport/transport_test.go` - `assertFailFast`; `TestFailFastBoundary`, `TestFailFastClientTimeoutIsDeadline`, `TestFailFastNotCounted`, `TestFailFastWhenNotBeforeMovesPastDeadline`; `TestHeldCancelIsNotSent` gains the not-`ErrRateLimited` assertion and two rows.
- `internal/ado/result.go` - the `ErrRateLimited` branch and the rewritten deadline comment; imports `internal/transport`.
- `internal/ado/result_test.go` - `wantText` column and four rows.
- `internal/ado/v7_test.go` - `primedGitClient`, `unsentRepositoriesErr`, `failFastErr`, `heldCancelErr`, `refusedHostErr`, `TestFailFastReadClassifiesRateLimit`.
- `internal/guard/guard.go` - `ErrNotSent` first in `classify`; doc comment; imports `errors` and `internal/transport`.
- `internal/guard/guard_test.go` - `429` row; `rtFunc` twin, `emptyResponse`, `getError`, `primedClient`; `TestRunNotSent`.
- `cmd/adoBizInt/ratelimit_test.go` - `TestRateLimitedWriteExitsOne`, `failFastWriteErr`.

## Tripwire and Gate Runs

Every tripwire was an uncommitted edit, restored and checked sha256-identical.

| Run | Change | Result |
|-----|--------|--------|
| Task 1 tripwire | `ErrRateLimited` branch removed from `classifyFailure` | `TestFailFastReadClassifiesRateLimit` FAIL (`other`) |
| Task 1 tripwire | fail-fast disabled | same test FAIL after 1s real time (`timeout`) |
| Tracer gate | Task 1 `<verify>` on the committed tree | PASS (17 rows, race suite, locks 7/1) |
| Task 2 tripwires | `>` instead of `>=`; no `ctx.Err()` check; decide before the first sleep only; fail-fast counted in Stats | `TestFailFastBoundary/equal`; `TestHeldCancelIsNotSent/deadline_already_passed` and `/cancelled_under_a_short_deadline`; `TestFailFastWhenNotBeforeMovesPastDeadline` (error replaced by the client timeout, held 10s); `TestFailFastNotCounted` each FAIL |
| Task 3 RED | tests before the guard change | `TestRunNotSent` 4 rows FAIL (unknown, 1 audit line); `TestRateLimitedWriteExitsOne/refused_host` and `/fail-fast` FAIL (exit 5, audited) |
| Task 3 tripwire | `ErrNotSent` check moved after `parentErr` | the three cancelled-parent rows FAIL |

## Decisions Made

- Decide the fit before every sleep, not only the first (08-05 handoff).
- An ended context reports its own cause ahead of the fail-fast, so a signal stays `other` and an expired deadline stays `timeout`.
- The D-08 text uses whole seconds. `Duration.Round(time.Second).String()` would print `1m0s` for the 60s fixture.
- A hold already taken before a later fail-fast stays counted. The time was spent waiting.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 08-05's "deadline ends while held" row contradicts D-01**
- **Found during:** Task 1
- **Issue:** The row primed Retry-After 60 and sent a request with a 10s deadline, expecting a 10s hold that wraps `context.DeadlineExceeded`. Under D-01 that request fails fast at once, so the row would fail, and the plan requires every 08-05 test to pass.
- **Fix:** The row became "cancelled while held" (`time.AfterFunc(10s, cancel)`, held exactly 10s, wraps `context.Canceled`). The deadline case moved to the `TestFailFast` tests.
- **Files modified:** internal/transport/transport_test.go
- **Committed in:** 1394800

**2. [Rule 2 - Missing critical] Cancel and expired deadline must win over the fail-fast**
- **Found during:** Task 1 design
- **Issue:** In the CLI every request carries a fetchctx deadline. Without an ordering rule, a signal-cancelled read with a pending wait longer than its remaining deadline would classify `rate_limit` instead of `other`.
- **Fix:** `hold` checks `ctx.Err()` before the fit. Two `TestHeldCancelIsNotSent` rows pin it: "deadline already passed" (wraps `DeadlineExceeded`, held 0) and "cancelled under a short deadline" (wraps `Canceled`, held 0).
- **Files modified:** internal/transport/transport.go, internal/transport/transport_test.go
- **Committed in:** 1394800, 4332d2e

**3. [Rule 2 - Missing critical] Test for the 08-05 handoff**
- **Found during:** Task 2
- **Issue:** The plan's tests cover an immediate fail-fast only. The handoff risk is a hold that fitted until a later response moved `notBefore`.
- **Fix:** Added `TestFailFastWhenNotBeforeMovesPastDeadline` under `Client.Timeout`. Its tripwire reproduces the lost `ErrNotSent`.
- **Files modified:** internal/transport/transport_test.go
- **Committed in:** 4332d2e

### Plan-text adjustments

**4. [Test helpers] `primedGitClient` and `unsentRepositoriesErr` shared by `failFastErr` and `heldCancelErr`**
- Both helpers need the same primed real-v7 client and the same "not sent, one request" assertions. The helpers also assert `ErrNotSent` (and `Canceled` for the held form), so a classification row cannot pass on a different error. `refusedHostErr` asserts `ErrNotSent` too. The interface signatures `failFastErr(t) error`, `refusedHostErr(t) error` and `heldCancelErr(t) error` are as planned.

**5. [RED count] Four guard rows were RED, not three**
- The plan expected the three cancelled-parent rows to fail first. "refused host, background parent" also returned `unknown`, through the P-13 no-status rule. All four pass after the fix.

**6. [Extra assertions] TestRunNotSent and TestRateLimitedWriteExitsOne check more than planned**
- `TestRunNotSent` asserts that each fixture is (or, for the hop, is not) `ErrNotSent` before running it, and that Mutate ran exactly once. `TestRateLimitedWriteExitsOne` also asserts zero audit records, which must_haves truth 5 requires.

### Process

**7. [TDD] Task 3 landed as a test/fix pair** (`a8945ee` RED, `17154c1` GREEN) instead of the plan's single fix commit. Task 2's tests pinned behaviour Task 1 had already built, so they were green on first run. Their tripwires above show they are not vacuous.

**8. [Tracer gate] No mid-flight human-verify halt.** Auto mode is off, but `human_verify_mode` defaults to end-of-phase and the tracer `<verify>` is fully automated and passed on the committed tree. Same handling as 08-01..08-05.

**9. [Phase 7 gate] Not checked here.** Per the user decision of 2026-10-20, it applies only to 08-08. The Phase 7 hash locks were checked after every task: 7/7 and 1/1 OK. No locked file was edited, and the new CLI test is in a new file.

---

**Total deviations:** 9 (1 bug fix, 2 missing-critical additions, 3 plan-text adjustments, 3 process). **Impact:** none on scope. Every acceptance criterion of the three tasks passes as written.

## Issues Encountered

- `internal/transport/transport.go` is now 366 lines, over the 300-line CONVENTIONS guideline (it was 314 after 08-05). The plan's artifacts pin `ErrRateLimited` to this file, so it was not split. A `pace.go` split (hold, observe, parsers, logRateLimit) is logged in `deferred-items.md` as a candidate.
- The plan estimated 76000 tokens; the realized diff is about 8000 (chars/4 over added lines).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 08-07 can reuse `failFastErr(t)` (one base call, `ErrRateLimited`, no status) to prove the read retry never retries a fail-fast. A fail-fast carries no HTTP status, so `StatusCodeOf(err) == 429` already excludes it. `heldCancelErr` and `refusedHostErr` are available for its non-retry rows.
- The exit-code contract is unchanged. Exit 1 on these write paths now also means "never sent, safe to retry later" (D-02, reversibility costly).
- PLAT-03 stays Pending until 08-07 and 08-08, which also declare it, finish (shared-ID gate).

## Self-Check: PASSED

- FOUND: all 8 key files; commits 1394800, 4332d2e, a8945ee, 17154c1; no file deletions since ac1e45e
- Artifacts: `ErrRateLimited` in transport.go (5), `transport.ErrRateLimited` in result.go (1), `transport.ErrNotSent` in guard.go (2), `TestRateLimitedWriteExitsOne` in ratelimit_test.go (2)
- Criteria: `connection.Timeout` in result.go 0; `ADO asked for` in transport.go 3; `Timeout()` in transport.go 0; the first statement of `classify` is the `ErrNotSent` check (awk check prints 1)
- Plan verification: `go build ./... && go vet ./... && go test -race ./...` rc 0 (16 packages), gofmt clean, `TestFailFast*` 7 PASS lines, `TestRunNotSent` and `TestRateLimitedWriteExitsOne` pass, locks 7/7 and 1/1
- No stub markers (TODO, FIXME, placeholder) in modified files

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-20*
