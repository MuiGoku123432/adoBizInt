---
phase: 02-honest-reads
plan: 03
subsystem: api
tags: [go, azure-devops, error-classification, timeouts, generics, dashboard]

requires:
  - phase: 02-honest-reads
    provides: "ado.Result[T], ado.Failure, classifyFailure, appendFailure, newResult and the typed SDK error constructors from plan 02-01"
  - phase: 02-honest-reads
    provides: "the six expanded aggregate reads and the four compile-fixed views from plan 02-02, which had to land first because Go compiles per package"
provides:
  - "GetDashboardCounts returning Result[DashboardCounts], with the aggregate carried as a single-element Items slice"
  - "getProjectCounts returning its partial counts alongside its failures — the one helper in the repo that returns a partial value with its failure list"
  - "Four distinct dashboard operation labels, one per sub-query plus the iteration lookup"
  - "The degraded WIQL iteration filter recorded as a failure instead of a silent substitution"
  - "CountsMsg carrying the envelope (compile fix only)"
affects: [02-04-json-field-contract, 02-05-tui-failure-surfacing, phase-03-cli, phase-05-hardening]

actuals:
  tokens: 29000
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "A helper returning (partial value, failures) with no error return, so a caller cannot discard the data a partially-answering unit did provide"
    - "A single aggregate payload rides a one-element Items slice so the failure list and completeness flag stay the same shape across all eight reads"

key-files:
  created: []
  modified:
    - internal/ado/dashboard.go
    - internal/ado/dashboard_test.go
    - internal/ui/dashboard/dashboard.go

key-decisions:
  - "The dashboard aggregate is carried as a one-element Items slice in Result[DashboardCounts] rather than in a bespoke struct, so Phase 3 writes its JSON failure/completeness handling once (D-01)"
  - "getProjectCounts returns (*DashboardCounts, []Failure) with no error, because an error return would push the caller back onto a continue and discard the counts the project did provide"
  - "The failed iteration lookup at dashboard.go:80-82 is recorded as a failure — a count taken under a silently substituted WIQL filter is not the whole picture (T-02-05)"
  - "The outer loop's log.Error was retained as an aggregate-level 'Incomplete counts for project' rather than deleted with the dead continue, keeping every project-level failure visible in the log"
  - "The dashboard view was left as a compile fix only; it still renders a partial total as if it were whole until 02-05 lands the completeness gate"

patterns-established:
  - "Each of a project's independent sub-queries gets its own operation label, because a partial total is only actionable if the user can tell which sub-query did not answer"
  - "A degradation that changes a query filter is a recorded failure, not a neutral fallback"

requirements-completed: [TRUST-01, TRUST-03]

coverage:
  - id: D1
    description: "getProjectCounts returns the counts it collected alongside its failures: a project that answered two of three sub-queries contributes those two, and each of the three failures carries a distinct operation label"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/dashboard_test.go#TestGetProjectCountsReportsPartialFailures"
        status: pass
    human_judgment: false
  - id: D2
    description: "A non-zero dashboard total is returned alongside a non-empty failure list and Complete == false; the failing project's PRs still count toward the total"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/dashboard_test.go#TestGetDashboardCountsSwallowsProjectFailure"
        status: pass
    human_judgment: false
  - id: D3
    description: "A clean two-project sweep reports complete with an empty failure list and a total equal to the sum of both projects"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/dashboard_test.go#TestGetDashboardCountsReportsCompleteWhenEveryProjectAnswers"
        status: pass
    human_judgment: false
  - id: D4
    description: "The degraded iteration lookup is a recorded outcome: a failed lookup produces a failure with the iterations operation label and an incomplete result, while the counts taken under the degraded filter are still returned"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/dashboard_test.go#TestGetDashboardCountsRecordsDegradedIterationLookup"
        status: pass
    human_judgment: false
  - id: D5
    description: "An already-expired fetch context produces a timeout-kind failure for each unreached project, in configured iteration order, with a zero total and Complete == false"
    verification:
      - kind: unit
        ref: "internal/ado/dashboard_test.go#TestFetchTimeoutMarksUnreachedDashboard"
        status: pass
    human_judgment: false
  - id: D6
    description: "The dashboard view compiles against the envelope but does not yet gate its rendered counts on completeness — a partial total still renders as a plain number"
    verification: []
    human_judgment: true
    rationale: "This is a deliberate, plan-mandated handoff to 02-05, which owns all reducer and render work. Until 02-05 lands, the dashboard's single-number surface remains the phase's most misleading read; whether that interim state is acceptable is a judgment call, not an assertion any test in this plan can make."

duration: 22 min
completed: 2026-09-07
status: complete
---

# Phase 02 Plan 03: Dashboard Partial Counts Summary

**`getProjectCounts` now returns the counts it collected alongside the failures it hit, so a project that answered two of three sub-queries contributes those two to a total that is explicitly labelled understated rather than silently wrong — and the fourth swallow, a silently substituted WIQL filter, is now a recorded failure.**

## Performance

- **Duration:** 22 min
- **Tasks:** 2 (both TDD: RED then GREEN)
- **Files modified:** 3 (0 created, 3 modified)

## Accomplishments

- `getProjectCounts` returns `(*DashboardCounts, []Failure)` and never an error. This is the one signature in the repo with no analog: no other helper returns a partial value alongside its failure list. The error return was deliberately rejected because it would have pushed the outer loop back onto a `continue` and degraded "the sum of the projects that answered" into "the sum of the projects that answered perfectly" (D-03).
- Each of the three in-place swallows records a failure with its own operation label, so a user reading a single understated number can tell whether the work-item query, the PR fetch or the build count is the one that did not answer. All three `if err != nil { log } else if <nil-guard> { assign }` chains are structurally unchanged — the failure record was added inside the existing error branch, so no assignment guard moved.
- `GetDashboardCounts` returns `Result[DashboardCounts]`. The dead outer `continue` is gone; the loop still visits every configured project (D-08), and the summation runs for every project that returned counts including ones that also returned failures.
- The D-12 already-expired-context guard marks each unreached project as a timeout instead of issuing three doomed sub-queries per project.
- **The fourth swallow is now a decision on the record**, with a test pinning it. See below.
- The characterization test changed, which is the documented signal that this specific read was actually inverted rather than mechanically edited.

## Task Commits

1. **Task 1 RED: failing tests for partial project counts** — `23203fa` (test)
2. **Task 1 GREEN: partial counts alongside failures** — `c38c005` (feat)
3. **Task 2 RED: failing tests for the dashboard envelope** — `bdf4d46` (test)
4. **Task 2 GREEN: envelope return and the degraded filter decision** — `dc475a9` (feat)

## Files Created/Modified

- `internal/ado/dashboard.go` — four operation labels, the two-value `getProjectCounts`, three sub-query failure records, the iteration-lookup failure record, the envelope return, the D-12 guard, and the removal of the dead `continue`
- `internal/ado/dashboard_test.go` — the characterization test inverted in place under its existing name, plus a five-case table for `getProjectCounts`, a clean-sweep case, the degraded-filter case and the D-12 case; adds the `dashboardStubs` fixture that can fail exactly one of a project's four calls
- `internal/ui/dashboard/dashboard.go` — `CountsMsg` carries the envelope per D-16 and the reducer unwraps `Items[0]`; compile fix only

## The Decision on the Degraded Iteration Lookup (`dashboard.go:80-82`)

**Decision: it is recorded as a failure**, with operation label `get dashboard iterations`, and pinned by `TestGetDashboardCountsRecordsDegradedIterationLookup`.

The plan's recommendation was followed, and the reasoning holds up on inspection: this site is not a neutral fallback. When `GetRecentIterations` fails, the WIQL query below silently loses its `[System.IterationPath] IN (...)` clause and runs unfiltered by iteration. The work-item count that gets summed into the dashboard total was therefore taken under a **different query than the one intended**, and is then presented with exactly the same authority as a correct one. That is the specific kind of quiet inaccuracy this phase exists to remove, and it is the failure mode that D-03's "labelled rather than silently understated" contract targets.

Note the direction of the error: an unfiltered query returns *more* work items, so this degradation makes the total look **overstated**, not understated. That is arguably worse than a missing count, because there is no seam at all — the number simply grows. Recording it makes the result incomplete, which is the only signal available.

**Cost of the choice, stated honestly:** any org whose projects have no iterations configured, or where the classification-node API is flaky, will now see the dashboard reported as incomplete on every refresh. If 02-05's warning line proves noisy in practice, this and the A4 empty-repository inference from 02-02 are the two places to look first.

## How the Aggregate Is Carried Inside the Envelope

`Result[DashboardCounts]` with **exactly one element in `Items`**, holding the summed `DashboardCounts`.

The alternative considered was a bespoke `DashboardResult` struct with its own `Counts`, `Failures` and `Complete` fields. It was rejected because it would create a second envelope shape, and D-01's entire purpose is that Phase 3 writes its failure/completeness JSON handling once for all eight reads. A one-element slice is slightly awkward at the call site; a second shape is a permanent tax on every downstream consumer.

The invariant is that `Items` always has exactly one element on every path, including the no-projects-configured path and the all-projects-failed path — the summed value exists even when it is zero. `dashboardTotal` in the test file asserts this and is the documented unwrap shape for 02-05 and Phase 3.

## Decisions Made

- **The outer `log.Error` was retained, not deleted with the dead `continue`.** It became `"Incomplete counts for project"` with a `failure_count`. Deleting it would have made a project-level failure invisible in the log while the per-sub-query errors remained, which is a strictly worse log. This also happens to keep the `log.Error` count at 4, satisfying the Task 1 acceptance criterion without any code being shaped to chase a grep.
- **Four operation labels rather than one.** The dashboard is the only read issuing three independent sub-queries per project. A single shared label would make the failure list ambiguous in exactly the read where the payload has the least visible structure.
- **The D-12 guard uses `opDashboardWorkItems` as its label.** The project was never reached, so no sub-query is the true culprit; the first sub-query's label is used to avoid inventing a fifth label for a case that names no specific call. Consistent with how the other seven reads label their unreached projects.

## Deviations from Plan

None. Both tasks executed as written, and every acceptance criterion in the plan was satisfiable as specified — notably the `grep -c 'log.Error'` criterion, which the two preceding plans' equivalent criteria were not.

## Known Stubs

**One, and it is the highest-stakes one in the phase.**

| File | What | Why |
|------|------|-----|
| `internal/ui/dashboard/dashboard.go` (reducer, `CountsMsg` branch) | The view receives `Failures` and `Complete` and reads neither. A partial or overstated total renders as a plain number with no warning. | Plan-mandated scope boundary: `<call_site_scope>` explicitly assigns all reducer and render work to 02-05 and says "Do not start it here." A comment at the site records the constraint and names the owning plan. |

This is not an oversight and it is not resolvable inside this plan without colliding with 02-05, which owns the same file. But it should be stated plainly: **until 02-05 lands, the dashboard is the one view where the data layer is now honest and the presentation layer is not.** The four views compile-fixed by 02-02 are in the same interim state, but their surfaces are tables, where an empty row set is at least visibly different from a populated one. The dashboard reduces everything to three integers, so its interim state is the most misleading of the six. 02-05 should treat this view as its highest priority.

## Issues Encountered

None. Both RED phases failed as expected (compile failures against undeclared symbols and the old return type), and both GREEN phases passed on first run.

## User Setup Required

None.

## Next Phase Readiness

All eight aggregate reads now return the envelope. The `internal/ado` layer of this phase is complete.

Remaining for the phase:

- **02-04** owns the JSON field contract on the exported `ado` structs (D-04).
- **02-05** owns all reducer and render work across all six views, including the dashboard stub above.
- The `TestGet.*SwallowsProjectFailure` characterization names remain in place across all eight reads by design; the rename sweep is the phase's completion signal, and all eight now assert inverted behavior under their old names.

---
*Phase: 02-honest-reads*
*Completed: 2026-09-07*

## Self-Check: PASSED

All 3 modified files verified present on disk. All 4 commit hashes verified present in git history. Full suite green: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test ./... -count=1`.
