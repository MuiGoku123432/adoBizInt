---
phase: 08-platform-upgrade-and-pacing
plan: 11
subsystem: transport
tags: [go, net/http, pacing, rate-limit, retry-after, synctest]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-06 D-01 fail-fast (ErrRateLimited, notSentError reasonRateLimited); 08-10 hardened host rule in transport.go"
provides:
  - "minSendBudget = 2s: a pacer hold is taken only when it ends with at least 2s of the request's deadline left (WR-02)"
  - "maxHold = 5m: no single hold exceeds five minutes, with or without a deadline (WR-03)"
  - "D-08 message rounds the asked wait up and the budget down, inline, overflow-safe (IN-02)"
  - "deferred-items.md records every 08-REVIEW item the gap plans leave alone"
affects: [08-12 live smoke, PLAT-03 verification, phase 9 TUI fetch behaviour under throttling]

# Actuals (#2632) -- chars/4 over the realized diff (git diff feee374..HEAD)
actuals:
  tokens: 6458
  tasks: 2
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Hold budget = min(maxHold, time.Until(deadline) - minSendBudget); refuse when wait > budget, take equality"
    - "Ceiling of a Duration by quotient plus remainder, never by adding (d + 1s - 1ns), which overflows on a saturated wait"

key-files:
  created: []
  modified:
    - internal/transport/transport.go
    - internal/transport/transport_test.go
    - .planning/phases/08-platform-upgrade-and-pacing/deferred-items.md

key-decisions:
  - "D-01 interpretation (refined, not contradicted): a wait fits when the hold ends with at least minSendBudget of the deadline left and is no longer than maxHold; every refusal is still exactly the D-01 fail-fast (ErrRateLimited, rate_limit, D-08 message, failed with no audit for a write under D-02, never retried under D-03), and every accepted wait is honoured to the instructed time (D-04)"
  - "minSendBudget = 2s is the user's decision (user, 2026-10-21: use 2s budget) under the 08-CONTEXT discretion clause 'any cap on a single honoured Retry-After that fits the deadline'. Trade-off: at the 30s default request_timeout waits up to 28s are honoured and every sent request has at least 2s before its deadline; 29s and 30s instructions fail fast as rate_limit; a request_timeout at or below 2s fails every instructed wait fast"
  - "WR-03: cap a single hold at maxHold = 5m rather than reject request_timeout: 0 in Validate. Zero stays the documented large-attachment mode; an instruction above the cap is stored as sent (never clamped, never sent early, D-04) and requests fail fast as rate_limit until it passes or the process restarts"
  - "maxHold supersedes 08-RESEARCH Open Question 5 ('no separate cap: the request deadline is the cap') under the same 08-CONTEXT discretion grant; 08-RESEARCH.md is the research record and is not edited"
  - "The D-08 ceiling uses quotient plus remainder instead of the plan's 'add one second less one nanosecond': the plan's form prints -9223372035s for a saturated wait (far-future Retry-After HTTP-date)"

patterns-established:
  - "Pacer tests pin the decided constants with literals (2s, 5m) so RED fails on an assertion, not on an undefined identifier"

requirements-completed: [PLAT-03]

coverage:
  - id: D1
    description: "A throttled request is held only when the hold leaves at least 2s of its deadline; otherwise it fails fast as rate limited (WR-02)"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastBoundary"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastClientTimeoutIsDeadline"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastWhenNotBeforeMovesPastDeadline"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastNotCounted"
        status: pass
      - kind: integration
        ref: "go test -count=1 -run 'TestFailFastReadClassifiesRateLimit' ./internal/ado; -run TestRunNotSent ./internal/guard; -run TestRateLimitedWriteExitsOne ./cmd/adoBizInt"
        status: pass
    human_judgment: false
  - id: D2
    description: "No single hold exceeds maxHold (5m), with or without a deadline, request_timeout: 0 included (WR-03)"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestHoldCap"
        status: pass
      - kind: unit
        ref: "go test -count=1 ./internal/config (request_timeout: 0 still valid)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The D-08 message never contradicts itself: asked rounds up, budget rounds down, no overflow on a saturated wait (IN-02)"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/transport/transport_test.go#TestFailFastMessageRounding"
        status: pass
      - kind: unit
        ref: "internal/transport/transport_test.go#TestHoldCap/far-future_HTTP-date_refused_with_no_deadline"
        status: pass
    human_judgment: false
  - id: D4
    description: "Waits that fit and unthrottled requests are unchanged (exact gaps, Stats, held-cancel semantics)"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "go test -count=1 -run 'TestPace|TestStats|TestHeldCancelIsNotSent' ./internal/transport"
        status: pass
      - kind: integration
        ref: "go test -race ./... and go test -count=2 ./..."
        status: pass
    human_judgment: false
  - id: D5
    description: "deferred-items.md records IN-03, IN-04, IN-05, IN-06, the notBefore remainder of WR-03 and the transport.go size, each with a reason"
    verification: []
    human_judgment: true
    rationale: "Whether each deferral reason is acceptable is a judgment call for the verifier; only presence of the rows is grep-checked"

# Metrics
duration: 13min
completed: 2026-10-21
status: complete
---

# Phase 8 Plan 11: Pacer Send Budget and Hold Cap Summary

**Pacer hold budget = min(5m, deadline left less 2s): a throttled request is either held and sent with at least 2s to complete, or refused at once as rate_limit; no header can hold any request longer than 5 minutes; the D-08 message rounds so it never reads as a wait that fits**

## Performance

- **Duration:** about 13 min
- **Started:** 2026-10-21T20:22:38Z
- **Completed:** 2026-10-21T20:35Z
- **Tasks:** 2 (tracer + expansion), TDD RED/GREEN each
- **Files modified:** 3

## Accomplishments

- WR-02 closed: `hold` refuses a wait longer than `time.Until(deadline) - minSendBudget`. A 29s Retry-After under the 30s default no longer goes out with 1s left and times out (a read now reads rate_limit, a never-sent write reads failed instead of unknown).
- WR-03 closed: `maxHold = 5 * time.Minute` bounds every single hold, including the deadline-less startup context under `request_timeout: 0`. Retry-After 301, or a year-9999 HTTP-date, is refused at once and never sent; config.Validate is unchanged.
- IN-02 closed: `notSentError.Error` rounds the asked wait up and the budget down inline (`wholeSeconds` deleted), so `ADO asked for 5s, 4s left in budget` replaces `5s, 5s`.
- The Warn line is now `Refused to hold a request for ADO rate-limit pacing` with `asked`, `budget` and `host` (no URL, Pitfall 15).
- ErrRateLimited, RoundTrip, hold, reasonRateLimited, the notSentError left field, New and the package doc all describe the hold budget and name both minSendBudget and maxHold.
- deferred-items.md gained a gap-closure table: transport.go at 440 lines, IN-03, IN-04 (accepted under D-04), IN-05 (checked live by 08-12 item c), IN-06, and the notBefore-never-decays remainder of WR-03.

## Task Commits

1. **Task 1: send budget after a hold (tracer, WR-02)** - `81d98e2` (test, RED), `01a8311` (fix, GREEN)
2. **Task 2: hold cap, D-08 rounding, deferrals (WR-03, IN-02)** - `4e81db6` (test, RED), `9872d78` (fix, GREEN)

**Plan metadata:** see the docs(08-11) commit that adds this file

## Files Created/Modified

- `internal/transport/transport.go` - `minSendBudget`, `maxHold`, hold budget in `hold`, inline rounding in `notSentError.Error`, updated doc comments (407 to 440 lines)
- `internal/transport/transport_test.go` - TestFailFastBoundary rewritten to the 7s boundary with an at-base deadline check; three tests renumbered; new TestHoldCap and TestFailFastMessageRounding
- `.planning/phases/08-platform-upgrade-and-pacing/deferred-items.md` - "From the 08 gap closure (08-09 to 08-11)" table

## Decisions Made

- **D-01 interpretation:** "the wait fits" now means the hold ends with at least minSendBudget of the deadline left and is no longer than maxHold. Refusals are still exactly the D-01 fail-fast; accepted waits are still honoured to the instructed time (D-04).
- **minSendBudget = 2s (user, 2026-10-21: use 2s budget)** under the 08-CONTEXT Claude's Discretion clause "any cap on a single honoured Retry-After that fits the deadline". At the 30s default request_timeout, waits up to 28s are honoured and every sent request has at least 2s; 29s and 30s instructions fail fast as rate_limit.
- **WR-03 via maxHold, not Validate:** request_timeout: 0 stays valid (internal/ado/attachments.go large-download mode). An instruction above the cap is stored as ADO sent it, so the tool never sends early; requests fail fast until it passes or the process restarts (recorded in deferred-items.md).
- **maxHold supersedes 08-RESEARCH Open Question 5** ("no separate cap") under the same discretion grant; 08-RESEARCH.md is not edited.
- **Tests pin literals:** the RED tests compare against `2*time.Second` and `5*time.Minute`, not the constants, so RED failed on assertions rather than on undefined identifiers, and the tests pin the decided sizes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Overflow-safe ceiling in the D-08 message**
- **Found during:** Task 2 (rounding)
- **Issue:** The plan's ceiling, `(e.wait + time.Second - time.Nanosecond) / time.Second`, overflows when the wait is near `math.MaxInt64` ns. That is reachable: `retryAfter` saturates a far-future HTTP-date to the maximum Duration, and the new cap refuses it, so the message is built. A probe printed `-9223372035s` with the plan's form and `9223372037s` with the remainder form.
- **Fix:** `asked := e.wait / time.Second; if e.wait%time.Second != 0 { asked++ }`, still inline in `notSentError.Error`, no helper. The old `wholeSeconds` used `Duration.Round`, which saturates, so this keeps the old safety.
- **Files modified:** internal/transport/transport.go, internal/transport/transport_test.go
- **Verification:** new row `TestHoldCap/far-future_HTTP-date_refused_with_no_deadline` asserts `ADO asked for 9223372037s, 300s left in budget`
- **Committed in:** `4e81db6` (test), `9872d78` (fix)

**2. [Tracer gate] Run as an automated re-verify**
- Auto mode is off (`_auto_chain_active` false, `auto_advance` unset), which would normally stop for a `checkpoint:human-verify` after the tracer. `human_verify_mode` is at its end-of-phase default and the tracer's verify is fully automated, so, as in 08-09 and 08-10, the tracer's verify was re-run after `01a8311` (PASS) before Task 2.

**3. [Format] deferred-items.md section as a table**
- The plan asked for "a table in the file's existing format". The file's top section is a table and its later sections are bullets; the new section uses the table's columns (Found in, File, Item, Why deferred).

---

**Total deviations:** 1 auto-fixed bug (Rule 1), 2 process notes
**Impact on plan:** The Rule 1 fix keeps the plan's intent (asked rounds up, inline, no helper) and removes an overflow the plan's formula would have introduced. No scope creep.

## Issues Encountered

- The Task 2 RED commit `4e81db6` contains a row (far-future HTTP-date, no deadline) that does not fail before the fix: it hangs, because the pre-fix pacer holds for the saturated wait. That hang is the WR-03 defect itself (confirmed with `-timeout 15s`). It means `go test ./internal/transport` at exactly `4e81db6` hangs until the test timeout; every commit after it is green. RED for the other rows was observed with `-run 'TestHoldCap/(300|301|400)|TestFailFastMessageRounding'`.

## TDD Gate Compliance

- Task 1: RED `81d98e2` (boundary rows 5s+1ns and 7s-1ns, "8s left", "3s left" and `budget=0s` failed on assertions), GREEN `01a8311`.
- Task 2: RED `4e81db6` (301s and 400s rows sent instead of refused; messages read "5s, 5s" and "0s, 0s"), GREEN `9872d78`.
- No refactor commits were needed.

## Verification

| Check | Result |
|-------|--------|
| Task 1 verify command (incl. grep and awk doc checks) | PASS |
| Task 2 verify command (incl. gofmt, vet, `! grep func wholeSeconds`, awk for both constants, deferred-items greps) | PASS |
| TestFailFastReadClassifiesRateLimit, TestRunNotSent, TestRateLimitedWriteExitsOne | PASS unchanged |
| TestPaceIgnoresInvalid, TestHeldCancelIsNotSent, internal/config | PASS unchanged |
| `go build ./...` | PASS |
| `go test -race ./...` | PASS |
| `go test -count=2 ./...` | PASS |

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SC3 is tighter: no throttled request is sent without at least 2s to complete, and no single instruction can hold the process indefinitely. SC3 and SC4 code gaps from 08-REVIEW are all closed (CR-01, WR-01 in 08-10; WR-02, WR-03 here; WR-04, WR-05 in 08-09), so PLAT-03 is marked complete; this is the last plan declaring it.
- Ready for 08-12: the live smoke (PLAT-05), including checklist item c for IN-05 (attachment download host).
- Behaviour to watch live: an ADO delay of 29s or 30s under the 30s default now fails fast as rate_limit with `ADO asked for 29s, 28s left in budget` instead of timing out.

## Self-Check: PASSED

- FOUND: internal/transport/transport.go, internal/transport/transport_test.go, .planning/phases/08-platform-upgrade-and-pacing/deferred-items.md
- FOUND commits: 81d98e2, 01a8311, 4e81db6, 9872d78
- `minSendBudget = 2 * time.Second` and `maxHold = 5 * time.Minute` present; `TestHoldCap` present; no TODO, FIXME or placeholder in transport.go

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-21*
