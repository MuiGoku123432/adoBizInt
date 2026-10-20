---
phase: 08-platform-upgrade-and-pacing
plan: 07
subsystem: api
tags: [go, retry, rate-limit, decorator, go-ast, synctest, stderr, plat-03, d-03, d-05, d-06, d-07, d-08]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-05: header-driven pacer and (*transport.Transport).Stats(); 08-06: D-01 fail-fast (ErrRateLimited, no HTTP status), failFastErr test helper, write 429/not-sent classified failed"
provides:
  - "internal/ado/retry.go: maxReadRetries = 2, generic retryRead, and four read-only decorators (retryingWorkItems, retryingGit, retryingBuild, retryingRelease) with 16 read overrides and no write override"
  - "NewClient wraps the four narrow-interface clients, startup resource-area discovery and the identity GetConnectionData (also used by the PR-approval identity fallback) in the read retry, all sharing one *atomic.Int64"
  - "(*ado.Client).FetchRetries() int64: retries attempted, read by the CLI after dispatch"
  - "cmd/adoBizInt/throttle.go: renderThrottleSummary, one stderr line 'throttled by ADO: N wait(s), Ds total, R retry/retries' on the CLI path only"
  - "Structural guard TestRetryNeverWrapsWrites (go/ast) and behavioural TestRetryWritePassesThroughOnce: no write can be retried"
affects: [08-08, phase-09]

# Actuals (#2632)
actuals:
  tokens: 8236
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Read-only retry as an embedding decorator: embed the narrow interface, override reads only; writes and any later-added method fall through un-retried (safe default)"
    - "Exhaustiveness by AST: decorator method set must equal embedded interface methods minus the five writes, scanned across every package file, with non-vacuity floors (4 types, 16 overrides, every write present in some interface)"
    - "Stderr counters render whole seconds, never Duration.String, matching the D-08 message unit"

key-files:
  created:
    - internal/ado/retry.go
    - internal/ado/retry_test.go
    - cmd/adoBizInt/throttle.go
    - cmd/adoBizInt/throttle_test.go
  modified:
    - internal/ado/client.go
    - internal/ado/v7_test.go
    - internal/ado/wiql_test.go
    - cmd/adoBizInt/main.go

key-decisions:
  - "retryRead returns the call's own value and error unchanged on every non-retry path, so off the raw-429 path a decorated client is indistinguishable from the bare SDK client"
  - "The throttle total prints whole seconds ('90s total', never '1m30s'), the unit 08-06 chose for the D-08 message, so an agent regex on '(\\d+)s total' cannot misread a run over a minute"
  - "checkWIQL (TestWIQLInterpolationStructural) no longer counts a method that forwards its own parameters unchanged to the same-named method: the decorator's QueryByWiql pass-through builds no query, and its callers are the two counted sinks"
  - "getConnectionData became a closure over the location client's GetConnectionData wrapped in retryRead, so the PR-approval identity fallback retries a 429 too"

patterns-established:
  - "Tripwire per pinned behaviour, restored byte-identically (sha256 checked), never committed: 6 tripwires across the two tasks, each failing its target test"

requirements-completed: [PLAT-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "A git read built by NewClient that gets a 429 with Retry-After: 1 is re-issued once, the retry waits exactly 1s in the pacer, the read succeeds, and FetchRetries() is 1"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/v7_test.go#TestReadRetriedOnRateLimitThenSucceeds"
        status: pass
    human_judgment: false
  - id: D2
    description: "retryRead rules: 429 then success (2 calls, 1 retry); two 429s then success (3, 2); three 429s (3 calls, 2 retries, 'rate limited after 2 retries', still rate_limit); D-01 fail-fast, status 500 and a 429 under a cancelled context each return unchanged on the first call"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/retry_test.go#TestRetryRead (6 rows)"
        status: pass
    human_judgment: false
  - id: D3
    description: "No write is ever retried: each of UpdateWorkItem, CreatePullRequestReviewer, UpdatePullRequest, QueueBuild, UpdateReleaseApproval returning 429 through its decorator reaches the client exactly once with 0 retries counted; the AST test proves the 16 reads are overridden and no write is"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/retry_test.go#TestRetryWritePassesThroughOnce (5 rows)"
        status: pass
      - kind: unit
        ref: "internal/ado/retry_test.go#TestRetryNeverWrapsWrites"
        status: pass
    human_judgment: false
  - id: D4
    description: "Startup reads retry: resource-area discovery and identity each answer 429 once, NewClient succeeds, currentUserID is cached, FetchRetries() is 2"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "internal/ado/retry_test.go#TestStartupReadsRetry"
        status: pass
    human_judgment: false
  - id: D5
    description: "CLI stderr summary: nothing when neither paced nor retried, otherwise one line with singular nouns for 1 and whole seconds; rendered only in the run() statement that calls dispatch, after dispatch"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/throttle_test.go#TestThrottleSummary (6 rows)"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/throttle_test.go#TestThrottleSummaryOnlyOnCLIPath"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestValidateGuardsCLIAndTUI, cmd/adoBizInt/tuiexit_test.go#TestTUIWaitsForInflightWriteBeforeExit (unchanged, pass)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Phase 7 locks preserved, full suite green, gofmt and vet clean"
    verification:
      - kind: other
        ref: "shasum -a 256 -c 08-cli-tests.sha256 -> 7 OK; 07-cli-audit-test.sha256 -> 1 OK; go build/vet/test -race ./... rc 0 (16 packages ok); gofmt -l . empty"
        status: pass
    human_judgment: false
  - id: D7
    description: "The stderr line reads well to an operator and an agent in a real throttled run against the org"
    verification: []
    human_judgment: true
    rationale: "No live 429 can be produced on demand; the line format is unit-tested, but its usefulness in a real throttled run is judged during the 08-08 smoke run or later operation"

# Metrics
duration: 11min
completed: 2026-10-20
status: complete
---

# Phase 8 Plan 07: Read-Only Retry and Throttle Summary

**A read that gets HTTP 429 is now re-issued at most twice, from the service layer, through decorators that embed each narrow interface and override only its 16 reads; the five writes fall through untouched and a go/ast test keeps it that way. Startup discovery and identity retry under the same rule. When a CLI run was paced or retried, one line such as `throttled by ADO: 4 waits, 31s total, 1 retry` goes to stderr after dispatch; stdout, the JSON envelope and the TUI are unchanged.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-10-20T16:48:44Z
- **Completed:** 2026-10-20T16:59:06Z
- **Tasks:** 2
- **Files modified:** 8 (4 created, 4 modified; 773 insertions, 10 deletions)

## Accomplishments

- `retryRead` loops: call; success returns; any error that is not a raw 429 (including the D-01 fail-fast, which has no status), or any failure once `ctx` has ended, returns unchanged; after `maxReadRetries` (2) retries the 429 is wrapped as `rate limited after 2 retries: ...`, which still classifies `rate_limit` (D-08); otherwise it counts the retry, logs `retry` and `max` to adoBizInt.log and calls again. The re-issued SDK call builds a fresh body, so nothing is replayed.
- The counting rule is on `retryRead`'s doc comment: the counter records retries attempted, so a retry the pacer then refuses under D-01 is still counted.
- `NewClient` creates one `*atomic.Int64`, wraps `workitemtracking`, `git`, `build` and `release` in their decorators, wraps `root.GetResourceAreas` in `retryRead`, and makes `getConnectionData` a retrying closure. The dormant `work` client stays unwrapped (D-16). Tests that assign fakes straight to `c.gitClient` and the other fields are unaffected.
- `main.run`'s CLI branch now reads `code := dispatch(...)`, then `tr.Stats()`, then `renderThrottleSummary(stderrWriter, waits, waited, client.FetchRetries())`, then `return code`, all inside the one `if len(os.Args) > 1` statement. It prints on failed runs too (Open Question 9). The TUI path cannot reach it (D-07).
- The log shows the D-05 detail in order for one retried read: the transport's rate-limit header line (`status=429 retry_after=1`), `Retrying rate-limited read retry=1 max=2`, then the pacer's `wait=1s host=dev.azure.com`.

## Task Commits

1. **Task 1: One rate-limited git read retried end to end, counted, and summarized on stderr (tracer)** - `635c9c6` (feat)
2. **Task 2: Exhaustive read-only decorators, startup read retries, no-write-override test** - `06b2dc4` (test, RED) and `81d6007` (feat, GREEN)

**Plan metadata:** recorded in the `docs(08-07)` summary and completion commits

## Files Created/Modified

- `internal/ado/retry.go` - file doc (why retries live here, why decorators embed); `maxReadRetries`; `retryRead`; the four decorators and 16 value-receiver read overrides.
- `internal/ado/retry_test.go` - `writeMethods`; `TestRetryRead`, `TestRetryWritePassesThroughOnce`, `TestRetryNeverWrapsWrites`, `TestStartupReadsRetry`.
- `internal/ado/client.go` - `readRetries` field; shared counter; decorator wiring; retrying discovery and identity; `FetchRetries`; `NewClient` doc paragraph on D-03.
- `internal/ado/v7_test.go` - `tf400733` 429 body; `TestReadRetriedOnRateLimitThenSucceeds`.
- `internal/ado/wiql_test.go` - `forwardsOwnArgs`; the sink counter skips a decorator's pure pass-through; doc line.
- `cmd/adoBizInt/throttle.go` - `renderThrottleSummary`.
- `cmd/adoBizInt/throttle_test.go` - `TestThrottleSummary`, `TestThrottleSummaryOnlyOnCLIPath`.
- `cmd/adoBizInt/main.go` - the CLI branch renders the summary after dispatch.

## Tripwire and Gate Runs

Every tripwire was an uncommitted edit, restored and checked sha256-identical.

| Run | Change | Result |
|-----|--------|--------|
| Task 1 tripwire | `maxReadRetries = 0` | `TestReadRetriedOnRateLimitThenSucceeds` FAIL (`rate limited after 0 retries`) |
| Tracer gate | Task 1 targeted `<verify>` on the committed tree | PASS (race suite, locks 7/1 also passed before the commit) |
| Task 2 RED (compile) | tests before the three decorator types | `undefined: retryingWorkItems`, `retryingBuild`, `retryingRelease` |
| Task 2 RED (runtime) | the three struct types declared, no overrides, no wiring | `TestRetryNeverWrapsWrites` FAIL (11 missing reads, 4 types / 5 overrides); `TestStartupReadsRetry` FAIL (NewClient returns the 429) |
| Task 2 tripwire 1 (plan) | `UpdatePullRequest` override added to `retryingGit` | `TestRetryNeverWrapsWrites` FAIL (`retryingGit overrides the write UpdatePullRequest`); `TestRetryWritePassesThroughOnce/UpdatePullRequest` FAIL (3 calls, 2 retries). Passed again after removal |
| Task 2 tripwire 2 | discovery not wrapped | `TestStartupReadsRetry` FAIL (NewClient returns the 429) |
| Task 2 tripwire 3 | identity not wrapped | `TestStartupReadsRetry` FAIL (1 connectionData request, `currentUserID` empty) |
| Task 2 tripwire 4 | `ctx.Err()` check removed | `TestRetryRead/429_under_a_cancelled_context` FAIL (second attempt) |
| Task 2 tripwire 5 | decorator passes a copy of `args` to `QueryByWiql` | `TestWIQLInterpolationStructural` FAIL (3 sinks) |

## Decisions Made

- `retryRead` passes the call's own result through on every non-retry path.
- The throttle total is whole seconds, matching 08-06's D-08 unit.
- `checkWIQL` exempts a pure same-name forward of the method's own parameters, and nothing else.
- The approval-time identity fallback retries, through the same `getConnectionData` closure.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TestWIQLInterpolationStructural counted the decorator as a third WIQL sink**
- **Found during:** Task 2 (`go test -race ./...`)
- **Issue:** `checkWIQL` counts every `QueryByWiql` call and requires exactly 2. `retryingWorkItems.QueryByWiql` forwards `r.workItemAPI.QueryByWiql(ctx, args)`, so the count became 3 and the suite failed.
- **Fix:** `forwardsOwnArgs(fn, call)` excludes a call inside a method that passes that method's own parameters, unchanged and in order, to a method of the same name. Such a forward builds no query; its callers are the two counted sinks. Any other `QueryByWiql` call, including one in a decorator that passes anything else, still counts (tripwire 5).
- **Files modified:** internal/ado/wiql_test.go
- **Verification:** `TestWIQL*` pass; tripwire 5 fails as intended
- **Committed in:** 81d6007

**2. [Rule 2 - Consistency] Throttle total in whole seconds, not `Duration.Round(time.Second)`**
- **Found during:** Task 1
- **Issue:** The plan's `<waited.Round(time.Second)>` prints `1m30s` for a 90s wait. D-06 says agents parse this line, and 08-06 already chose whole seconds for the D-08 text for this reason. A regex on `(\d+)s total` would read `1m30s` as 30.
- **Fix:** Prints `%ds` of the rounded duration. All five planned rows produce byte-identical output; a sixth row pins `90s total`.
- **Files modified:** cmd/adoBizInt/throttle.go, cmd/adoBizInt/throttle_test.go
- **Committed in:** 635c9c6

### Plan-text adjustments

**3. [Extra assertions]** `TestRetryRead` has a sixth row ("two 429s then success": 3 calls, 2 retries, success) to pin the boundary between the last retry and exhaustion. `TestThrottleSummaryOnlyOnCLIPath` also asserts that the summary call comes after the dispatch call. `TestRetryNeverWrapsWrites` scans every package file for methods on `retrying*` types (not only retry.go), flags methods not in the embedded interface, and fails if any of the five writes is missing from interfaces.go.

**4. [gofmt]** Two overrides (`GetClassificationNode`, `GetReleaseDefinitions`) are a single `retryRead` call that gofmt splits over three lines. The `^func \(r retrying...` greps still print 16 and 0.

### Process

**5. [TDD] Task 2 landed as a test/feat pair** (`06b2dc4` RED, `81d6007` GREEN) instead of the plan's single feat commit. The RED commit does not compile, as in 08-05. Runtime RED was shown at an intermediate step (struct types only), which became part of the GREEN commit.

**6. [Tracer gate] No mid-flight human-verify halt.** Auto mode is off, but `human_verify_mode` defaults to end-of-phase and the tracer `<verify>` is fully automated and passed on the committed tree. Same handling as 08-01..08-06.

**7. [Phase 7 gate] Not checked here.** Per the user decision of 2026-10-20, it applies only to 08-08. The Phase 7 hash locks were checked after each task: 7/7 and 1/1 OK. No locked file was edited; the new CLI tests are in a new file.

---

**Total deviations:** 7 (1 blocking fix, 1 consistency fix, 2 plan-text adjustments, 3 process). **Impact:** none on scope. Every acceptance criterion of both tasks passes as written.

## Issues Encountered

- `internal/ado/client.go` is 409 lines (380 before this plan), over the 300-line CONVENTIONS guideline. The growth here is wiring the plan pins to `NewClient`. Not split.
- The plan estimated 85000 tokens; the realized diff is about 8200 (chars/4 over added lines).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ROADMAP SC3 is complete in code: rate-limited reads retry a bounded number of times within their deadline, writes never retry, and CLI callers see one throttle line.
- PLAT-03 is ready to mark complete: `requirements.ready-ids` reports 1/1, since 08-07 is the last Phase 8 plan that declares it.
- 08-08 (live smoke, Phase 7 gate) can watch for the `throttled by ADO:` line and the `Retrying rate-limited read` log entry. A clean smoke run will most likely show neither.
- Phase 9: any read added to a narrow interface fails `TestRetryNeverWrapsWrites` until its decorator overrides it. A new interface has no decorator requirement; its reads are simply not retried.

## Self-Check: PASSED

- FOUND: all 8 key files; commits 635c9c6, 06b2dc4, 81d6007; no file deletions since 74287f9
- Artifacts: `maxReadRetries = 2` in retry.go (1); `TestRetryNeverWrapsWrites` in retry_test.go; `throttled by ADO` in throttle.go; `retryingGit{` in client.go (1); `renderThrottleSummary(stderrWriter` in main.go (1); `retries attempted` in retry.go (2)
- Criteria: read overrides grep 16, write overrides grep 0
- Plan verification: `go build ./... && go vet ./... && go test -race ./...` rc 0 (16 packages ok), gofmt clean, `TestRetry*`, `TestStartupReadsRetry`, `TestThrottleSummary*` pass, locks 7/7 and 1/1
- No stub markers (TODO, FIXME, placeholder) in created files

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-20*
