---
phase: 02-honest-reads
plan: 01
subsystem: api
tags: [go, azure-devops, error-classification, timeouts, bubbletea, generics]

requires:
  - phase: 01-test-seam
    provides: package-local ado interfaces and fakes that let a typed SDK error be injected without crossing the REST boundary
provides:
  - "ado.Result[T] — the one envelope shape every aggregate read returns (items, failures, completeness)"
  - "ado.Failure and ado.FailureKind — a closed four-value classification of why a per-project read failed"
  - "ado.classifyFailure and statusCodeOf — structural classification handling both SDK error shapes"
  - "config.RequestTimeout and config.FetchTimeout — per-call and fan-out budgets with defaults and validation"
  - "ado.newConnection — sets connection.Timeout before any sub-client is constructed"
  - "ado.Client.FetchTimeout() — the fan-out budget accessor views read"
  - "styles.RenderAuthBanner / RenderPartialWarning / RenderStaleMarker — shared failure renderers"
  - "GetPullRequests returning the envelope, with the prs view rendering auth, partial, stale and empty distinctly"
affects: [02-02-expand-aggregate-reads, 02-03-dashboard-partial-counts, 02-04-json-field-contract, 02-05-tui-failure-surfacing, phase-03-cli, phase-05-hardening]

actuals:
  tokens: 34000
  tasks: 4
  commits: 6

tech-stack:
  added: []
  patterns:
    - "Generic result envelope shared across aggregate reads, justified by rule-of-three (eight call sites) under the CONVENTIONS rule 5 carve-out"
    - "Dual-target errors.As extraction, because WrappedError.Error() has a value receiver and errors.As will not convert between value and pointer forms"
    - "Structural error classification by HTTP status, never by matching server-controlled error text"
    - "Deadline check at the top of each fan-out iteration, recording unreached projects instead of issuing doomed calls"

key-files:
  created:
    - internal/ado/result.go
    - internal/ado/result_test.go
    - internal/config/config_test.go
    - internal/ado/client_test.go
    - internal/ui/prs/prs_test.go
  modified:
    - internal/ado/fakes_test.go
    - internal/config/config.go
    - internal/ado/client.go
    - internal/ado/pullrequests.go
    - internal/ado/pullrequests_test.go
    - internal/ui/styles/styles.go
    - internal/ui/prs/prs.go

key-decisions:
  - "Expose the fan-out budget via a Client.FetchTimeout() accessor rather than threading a new parameter through six view constructors and app.go (planner-discretion call)"
  - "newConnection leaves connection.Timeout nil for a zero configured budget, because a zero duration means no bound at all for http.Client"
  - "Cancellation is checked before the timeout branch and falls to FailureOther, so Phase 3 CLI-04 interrupts are never reported as server timeouts"
  - "DownloadAttachment's http.DefaultClient is excluded from TRUST-04 in this phase and deferred to Phase 5, where the host allow-list belongs (exclusion by decision, not omission)"
  - "location.NewClient and getConnectionData remain on an unbounded startup context; a recorded scope call, not an oversight"
  - "Routing startup-time auth failures through the classified banner is out of scope for this plan and logged as a follow-up"

patterns-established:
  - "Failure carries exactly project, operation, kind and error; no URL, header, authorization value or PAT may ever be added (T-02-01)"
  - "Failure renderers lead with the classified kind and truncate the server-controlled message before it reaches the terminal (T-02-02)"
  - "newResult derives Complete from the failure slice so no call site hand-sets it"

requirements-completed: [TRUST-01, TRUST-02, TRUST-03, TRUST-04]

coverage:
  - id: D1
    description: "GetPullRequests returns items, a failure list and a completeness flag; a project whose fetch failed appears in the failure list while the surviving project's PRs are still returned"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestGetPullRequestsSwallowsProjectFailure"
        status: pass
    human_judgment: false
  - id: D2
    description: "401 and 403 classify to auth, 429 to rate_limit, a deadline to timeout, everything else to other; no status maps to two kinds, and the value and pointer SDK error shapes classify identically"
    requirement: TRUST-02
    verification:
      - kind: unit
        ref: "internal/ado/result_test.go#TestClassifyFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/result_test.go#TestClassifyFailureIsSingleValued"
        status: pass
    human_judgment: false
  - id: D3
    description: "The classifier's auth branch fires against the real Azure DevOps service for a rejected PAT"
    requirement: TRUST-02
    verification:
      - kind: manual_procedural
        ref: "Task 4 checkpoint — live probe against the real org with an invalid PAT"
        status: pass
    human_judgment: true
    rationale: "Assumption A1 was unobservable from any stub; only the real service could reveal the status code and error shape it actually returns."
  - id: D4
    description: "Complete is false whenever the failure list is non-empty and true only when zero failures occurred"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/result_test.go#TestResultCompleteness"
        status: pass
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestGetPullRequestsReportsCompleteWhenEveryProjectAnswers"
        status: pass
    human_judgment: false
  - id: D5
    description: "Both timeout budgets default non-zero, parse from YAML, and Validate() rejects a fetch budget at or below the poll interval"
    requirement: TRUST-04
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestLoadAppliesTimeoutDefaults"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestValidate"
        status: pass
    human_judgment: false
  - id: D6
    description: "connection.Timeout is assigned from cfg.RequestTimeout before the first azuredevops sub-client constructor runs"
    requirement: TRUST-04
    verification:
      - kind: unit
        ref: "internal/ado/client_test.go#TestNewClientSetsTimeout"
        status: pass
    human_judgment: false
  - id: D7
    description: "A caller's already-elapsed deadline records a timeout-kind failure for each unreached project rather than issuing doomed calls"
    requirement: TRUST-04
    verification:
      - kind: unit
        ref: "internal/ado/pullrequests_test.go#TestFetchTimeoutMarksUnreached"
        status: pass
    human_judgment: false
  - id: D8
    description: "The prs view blocks the table on an auth failure, keeps rows plus a persistent warning when merely incomplete, marks stale rows, and never presents zero rows as an absence of work when incomplete"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ui/prs/prs_test.go#TestUpdateAuthFailureBlocksTheTable"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_test.go#TestUpdateNonAuthFailureKeepsRowsAndWarns"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_test.go#TestViewDistinguishesEmptyFromIncomplete"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_test.go#TestUpdateTransportErrorMarksPreviousRowsStale"
        status: pass
    human_judgment: false

duration: 41 min
completed: 2026-09-07
status: complete
---

# Phase 02 Plan 01: Tracer Honest Read Summary

**A shared `Result[T]` envelope, a status-based failure classifier that survives both SDK error shapes, both timeout budgets wired from YAML through `connection.Timeout` to a per-call-site fan-out deadline, and a pull-request view that tells a rejected token from an empty backlog.**

## Performance

- **Duration:** 41 min
- **Tasks:** 4 (3 implementation, 1 human-verify checkpoint)
- **Files modified:** 12 (5 created, 7 modified)

## Accomplishments

- `internal/ado/result.go` declares the one contract shape all eight aggregate reads will return: items, a failure list naming project and operation, and a completeness flag derived from that list rather than hand-set at each call site.
- `classifyFailure` maps failures structurally by HTTP status, never by matching the server-controlled error text, and `statusCodeOf` tries both the value and pointer `WrappedError` forms the SDK returns.
- Two new YAML config keys (`request_timeout` 30s, `fetch_timeout` 2m) with load-bearing defaults and a validation rule that the fan-out budget must exceed the poll interval.
- `newConnection` installs `connection.Timeout` before any sub-client is constructed, which is the only point in the file where the assignment is not a silent no-op.
- `GetPullRequests` returns the envelope, still visits every configured project on failure, and marks unreached projects as timeouts when the caller's deadline has already elapsed.
- The prs view now blocks the table on an auth failure, keeps rows with a persistent warning when merely incomplete, marks stale rows after a failed refresh, and only claims "No open pull requests found" when the read actually completed.

## Task Commits

1. **Task 1 RED: failing tests for envelope and classifier** — `cb8a198` (test)
2. **Task 1 GREEN: result envelope, failure record, classifier** — `7b66dbf` (feat)
3. **Task 2 RED: failing tests for timeout keys and connection wiring** — `f80dca4` (test)
4. **Task 2 GREEN: timeout config keys and connection timeout** — `15d00de` (feat)
5. **Task 3 RED: failing tests for the end-to-end honest read** — `8c5983a` (test)
6. **Task 3 GREEN: honest pull-request read through every layer** — `f5aab0b` (feat)

## Files Created/Modified

- `internal/ado/result.go` — the shared envelope, the four failure kinds, the classifier and the dual-target status extractor
- `internal/ado/result_test.go` — table test whose rows include the value/pointer 401 pair
- `internal/ado/fakes_test.go` — typed SDK error constructors in both shapes, for the expansion plans to reuse
- `internal/config/config.go` — two duration fields, both defaults, the fetch-vs-poll validation rule
- `internal/config/config_test.go` — first test file in this package; isolates `Load()` by redirecting `HOME`
- `internal/ado/client.go` — `newConnection` helper, `fetchTimeout` field, `FetchTimeout()` accessor
- `internal/ado/client_test.go` — asserts the helper, not `NewClient`, which performs six network round trips
- `internal/ado/pullrequests.go` — envelope return, failure recording, D-12 deadline guard
- `internal/ado/pullrequests_test.go` — the swallow characterization inverted in place, plus auth, complete and timeout cases
- `internal/ui/styles/styles.go` — three shared failure renderers, truncating the server message
- `internal/ui/prs/prs.go` — envelope-carrying message, bounded fetch, `applyResult` reducer, honest render branches
- `internal/ui/prs/prs_test.go` — first test to drive a Bubble Tea reducer in this codebase

## Verified Evidence: Assumption A1 (Task 4 checkpoint)

A1 was recorded at LOW confidence: the HTTP status Azure DevOps actually returns for a rejected PAT had never been observed, and TRUST-02's entire auth branch is built on it. If the real answer had been a 203 or a 302, the auth banner would never fire in production and every automated test in this plan would still pass.

Observed against the real org with an invalid PAT:

| Probe | Result |
|-------|--------|
| Raw `curl` with a malformed Basic header | HTTP **302** to the vssps sign-in page |
| Real SDK path (`git.NewClient` + `NewPatConnection`) | `azuredevops.WrappedError`, bare **value** form |
| Error text | `TF400813: The user '...' is not authorized to access this resource.` |
| `statusCodeOf(err)` | `401, true` |
| `errors.As(err, &value)` / `errors.As(err, &pointer)` | `true` / `false` |
| `classifyFailure(err)` | `auth` |

**Status code: 401. The classifier is correct as written; no amendment was required.**

Two findings worth carrying forward:

1. The 302 is a red herring. It occurs only when the Basic credential is unparseable, not when a well-formed PAT is rejected. A classifier built on the `curl` observation alone would have been wrong.
2. **The dual-target `statusCodeOf` was load-bearing, not defensive.** The real error arrives as a bare value, so a pointer-only `errors.As` would have returned `found=false` and dropped the rejected token to `FailureOther` — silently defeating TRUST-02 with every test green. This was the single highest-value test in the plan and it earned its place.

The probe test was temporary: it was written, run against the real service, and deleted. No probe file remains and the working tree is clean.

## Decisions Made

- **Fan-out budget exposed via `Client.FetchTimeout()`** rather than threading a new parameter through six view constructors and `app.go`. Planner-discretion call: the value already travels with the client, and the alternative touches five files this plan is otherwise forbidden to modify.
- **`newConnection` leaves `Timeout` nil for a zero budget** instead of installing a zero duration, which `http.Client` reads as no bound at all.
- **Cancellation is checked before the timeout branch** and deliberately falls to `FailureOther`, so Phase 3's CLI-04 interrupt handling never reports a user-initiated cancel as a server timeout.
- **`DownloadAttachment`'s `http.DefaultClient` is excluded from TRUST-04 in this phase** and deferred to Phase 5, where the host allow-list and path containment that belong with it live (HARD-01..HARD-03). This is an exclusion by decision, not by omission.
- **`location.NewClient` at `client.go:70` and `getConnectionData` at `:77` remain on an unbounded startup context.** The five error-returning constructors are now bounded at the transport layer by `connection.Timeout`; the startup identity lookup is not. A recorded scope call.

## Deviations from Plan

### 1. [Rule 3 - Blocking] Task 3 acceptance criterion is mechanically unsatisfiable as written

- **Found during:** Task 3 acceptance verification
- **Issue:** The criterion requires `grep -c 'context.Background()' internal/ui/prs/prs.go` to be strictly lower after the task. It is 3, unchanged.
- **Why it cannot be satisfied:** The fetch path's occurrence is now wrapped in `context.WithTimeout` at `prs.go:143`, but the literal must remain — a context has to be rooted somewhere. The other two occurrences are inside the approve and complete action commands, which this plan explicitly forbids touching.
- **Resolution:** The behavioral intent — the fetch fan-out is now bounded — is met and independently proven by `grep -n 'context.WithTimeout' internal/ui/prs/prs.go` returning a line inside `fetchPRs`, and by `TestFetchTimeoutMarksUnreached` passing. No code was changed to chase the grep.
- **Note for the expansion plans:** the two action-command `context.Background()` sites are unbounded and belong to whichever plan takes on PR write paths.

### 2. [Logged, not fixed] Startup-path auth failure bypasses the classified banner

- **Found during:** Task 4 checkpoint verification
- **Issue:** A *fully* rejected PAT fails inside `ado.NewClient` — the SDK performs a resource-area lookup during `workitemtracking.NewClient` and `git.NewClient` — and surfaces as a fatal `Failed to create ADO client` startup error from `cmd/adoBizInt/main.go`. It never reaches the honest-read path, so for the total auth-failure case the classified auth banner is unreachable.
- **Why this is not a TRUST-02 failure:** the `Result` envelope covers the case TRUST-02 actually targets — a PAT valid for one project but not another, where the fan-out records a per-project `auth` failure and the view stays honest. The total-failure case fails loudly at startup rather than misleadingly, which is not the C1 failure mode this phase exists to remove.
- **Resolution:** logged, not fixed. Routing startup auth failures through the same classified banner is explicitly out of scope for plan 02-01 and is captured as a follow-up item below.

---

**Total deviations:** 1 acceptance-criterion deviation (documented, no code change), 1 discovered gap (logged, deferred by decision)
**Impact on plan:** No scope creep. Both deviations are recorded rather than absorbed silently.

## Known Stubs

None. Every symbol this plan created is wired to a real call site and covered by a passing test.

## Follow-up Items

| Item | Why deferred | Suggested owner |
|------|--------------|-----------------|
| Route startup-time auth failures through the classified auth banner instead of a fatal `Failed to create ADO client` | Out of scope for 02-01; touches `cmd/adoBizInt/main.go` and `ado.NewClient`, neither of which this plan may modify | Phase 02 expansion or Phase 03 CLI |
| Bound the two `context.Background()` sites in the prs approve/complete action commands | This plan forbids touching PR write paths | Whichever plan takes on write-path timeouts |
| `DownloadAttachment` still uses `http.DefaultClient` with no host allow-list or timeout | Deferred by decision to Phase 5 where HARD-01..HARD-03 live | Phase 05 |
| `location.NewClient` and `getConnectionData` remain on an unbounded startup context | Recorded scope call | Phase 05 |

## Issues Encountered

None beyond the two deviations above. All three RED phases failed as expected (compile failures against undeclared symbols), all three GREEN phases passed on first run.

## User Setup Required

None persisting. The Task 4 checkpoint required a deliberately invalid PAT for one-off verification; no configuration change is retained.

## Next Phase Readiness

The envelope shape is proven end to end and is safe for the six expansion reads and five expansion views to depend on. Specifically de-risked for the expansion plans:

- `newResult` and `appendFailure` give one call-site shape to copy; no expansion read needs to reason about completeness.
- The typed SDK error constructors in `fakes_test.go` let every expansion test inject a real `WrappedError` in either shape rather than a bare `errors.New`.
- The three style renderers are in place, so the expansion views add render branches rather than styling decisions.
- The A1 status code is now observed fact rather than a low-confidence assumption, so the expansion plans inherit a verified auth branch.

**Blocker for the phase completion gate:** the eight `TestGet.*SwallowsProjectFailure` characterization names are still in place by design — the rename sweep happens once at the end of the phase, and that anchored count remains the phase's completion signal. One of the eight (`TestGetPullRequestsSwallowsProjectFailure`) now asserts inverted behavior while keeping its old name.

---
*Phase: 02-honest-reads*
*Completed: 2026-09-07*

## Self-Check: PASSED

All five created files verified present on disk. All seven commit hashes verified present in git history.
