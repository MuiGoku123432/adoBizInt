---
phase: 02-honest-reads
plan: 05
subsystem: ui
tags: [go, bubbletea, lipgloss, tui, error-surfacing, timeouts]

requires:
  - phase: 02-honest-reads
    provides: "ado.Result[T], ado.Failure, FailureKind and the classifier from plan 02-01, plus the three banner/warning/stale renderers in internal/ui/styles"
  - phase: 02-honest-reads
    provides: "the six expanded aggregate reads from 02-02 and the dashboard envelope from 02-03, both of which left their views compile-fixed but not honest"
  - phase: 02-honest-reads
    provides: "the JSON field contract from 02-04, whose landing was the precondition for this plan's repo-wide gate"
provides:
  - "All six view packages render a blocking auth banner, a persistent partial warning, and a completeness-gated zero-row message"
  - "internal/ui/fetchctx — the single derived-deadline helper every read fetch in the TUI now uses"
  - "styles.SummarizeFailures, styles.FirstAuthFailure, styles.AuthBannerFor and styles.RenderUnderstatedWarning"
  - "Five new view test files, taking the TUI from one tested view package to six"
  - "02-VALIDATION.md carrying real per-row statuses and a recorded approval"
affects: [phase-03-cli, phase-04-writes, phase-05-hardening]

actuals:
  tokens: 71000
  tasks: 4
  commits: 7

tech-stack:
  added: []
  patterns:
    - "A per-view applyResult(envelope) reducer helper that folds Items, Failures and Complete into render state in one place, so the auth-versus-partial asymmetry cannot drift between views"
    - "Only the primary message per view drives the blocking banner; secondary fetches keep a status-line warning"
    - "A read fetch derives its deadline through a shared helper, so no view file mentions context.Background() on a read path"

key-files:
  created:
    - internal/ui/fetchctx/fetchctx.go
    - internal/ui/dashboard/dashboard_test.go
    - internal/ui/workitems/workitems_test.go
    - internal/ui/pipelines/pipelines_test.go
    - internal/ui/branches/branches_test.go
  modified:
    - internal/ui/styles/styles.go
    - internal/ui/dashboard/dashboard.go
    - internal/ui/workitems/workitems.go
    - internal/ui/pipelines/pipelines.go
    - internal/ui/releases/releases.go
    - internal/ui/releases/releases_test.go
    - internal/ui/branches/branches.go
    - internal/ui/prs/prs.go
    - .planning/phases/02-honest-reads/02-VALIDATION.md

key-decisions:
  - "D-15's stale marker was applied to the three polling views only (prs, pipelines, releases) and deliberately omitted from workitems and branches, which have no refresh timer"
  - "The dashboard warning states 'totals understated' in the model field rather than in the renderer, so the claim is assertable by a test rather than only visible on screen"
  - "A failed iteration lookup is worded as 'counted unfiltered by iteration', separately from projects that 'did not answer', because it skews the total upward rather than downward"
  - "A shared internal/ui/fetchctx package replaced the six-line inline deadline block, which is what actually lowered the context.Background() counts the acceptance criteria measure"
  - "workitems.New was made nil-client tolerant so its reducer can be exercised without a live client"
  - "Two validation-map commands that matched zero tests were corrected to the real test names rather than marked green against a pattern that ran nothing"

patterns-established:
  - "An auth failure replaces the data surface; a non-auth failure annotates it. The asymmetry is uniform across all six views."
  - "Only a completed read may assert an absence of work, and only a completed read may blame the user's configuration for it."

requirements-completed: [TRUST-02, TRUST-03]

coverage:
  - id: D1
    description: "Each of the six views renders a blocking auth banner and suppresses its data surface when the result carries an auth-kind failure"
    requirement: TRUST-02
    verification:
      - kind: unit
        ref: "internal/ui/prs/prs_test.go#TestUpdateAuthFailureBlocksTheTable"
        status: pass
      - kind: unit
        ref: "internal/ui/pipelines/pipelines_test.go#TestUpdateAuthFailureBlocksTheTable"
        status: pass
      - kind: unit
        ref: "internal/ui/releases/releases_test.go#TestUpdateAuthFailureBlocksTheTable"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/workitems_test.go#TestUpdateAuthFailureBlocksTheTable"
        status: pass
      - kind: unit
        ref: "internal/ui/branches/branches_test.go#TestUpdateAuthFailureBlocksTheTable"
        status: pass
      - kind: unit
        ref: "internal/ui/dashboard/dashboard_test.go#TestUpdateAuthFailureBlocksTheCounts"
        status: pass
    human_judgment: false
  - id: D2
    description: "A non-auth failure keeps the rows on screen and raises a persistent warning naming the failed projects and the failure kind"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ui/pipelines/pipelines_test.go#TestUpdateNonAuthFailureKeepsRowsAndWarns"
        status: pass
      - kind: unit
        ref: "internal/ui/releases/releases_test.go#TestUpdateNonAuthFailureKeepsRowsAndWarns"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/workitems_test.go#TestUpdateNonAuthFailureKeepsRowsAndWarns"
        status: pass
      - kind: unit
        ref: "internal/ui/branches/branches_test.go#TestUpdateNonAuthFailureKeepsRowsAndWarns"
        status: pass
    human_judgment: false
  - id: D3
    description: "No view presents zero rows (or a zero count) as an absence of work when the result is incomplete; the pipelines and releases views in particular no longer blame config.yaml"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ui/pipelines/pipelines_test.go#TestZeroRunsDistinguishesEmptyFromIncomplete"
        status: pass
      - kind: unit
        ref: "internal/ui/releases/releases_test.go#TestZeroReleasesDistinguishesEmptyFromIncomplete"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/workitems_test.go#TestViewDistinguishesEmptyFromIncomplete"
        status: pass
      - kind: unit
        ref: "internal/ui/branches/branches_test.go#TestViewDistinguishesEmptyFromIncomplete"
        status: pass
      - kind: unit
        ref: "internal/ui/dashboard/dashboard_test.go#TestZeroCountsDistinguishEmptyFromIncomplete"
        status: pass
    human_judgment: false
  - id: D4
    description: "The dashboard total is labelled understated rather than silently partial, and a degraded iteration lookup is worded distinctly from a project that did not answer"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ui/dashboard/dashboard_test.go#TestUpdatePartialFailureRendersCountsAndSaysUnderstated"
        status: pass
      - kind: unit
        ref: "internal/ui/dashboard/dashboard_test.go#TestUpdateDegradedIterationFilterIsWordedDistinctly"
        status: pass
    human_judgment: false
  - id: D5
    description: "The three polling views keep their last good rows and mark them stale when a refresh fails"
    verification:
      - kind: unit
        ref: "internal/ui/pipelines/pipelines_test.go#TestUpdateTransportErrorMarksPreviousRowsStale"
        status: pass
      - kind: unit
        ref: "internal/ui/releases/releases_test.go#TestUpdateTransportErrorMarksPreviousRowsStale"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_test.go#TestUpdateTransportErrorMarksPreviousRowsStale"
        status: pass
    human_judgment: false
  - id: D6
    description: "A secondary fetch (pipeline definitions, release approvals) failing auth warns on the status line and never blanks a table the primary fetch populated"
    verification:
      - kind: unit
        ref: "internal/ui/pipelines/pipelines_test.go#TestSecondaryDefinitionsAuthFailureDoesNotBlock"
        status: pass
      - kind: unit
        ref: "internal/ui/releases/releases_test.go#TestSecondaryApprovalsAuthFailureDoesNotBlock"
        status: pass
    human_judgment: false
  - id: D7
    description: "Every read fetch in the six views runs under a derived deadline; every write call site is unchanged"
    verification:
      - kind: other
        ref: "grep -c 'context.Background()' per view file, compared against the pre-task counts (pipelines 3->1, releases 4->2, workitems 6->5, prs 3->2, branches 1->0, dashboard 1->0)"
        status: pass
      - kind: other
        ref: "git diff of internal/ui/workitems/workitems.go showing all five write-path context.Background() call sites untouched"
        status: pass
    human_judgment: false
  - id: D8
    description: "The whole tree builds, vets, formats and tests clean in one command, and 02-VALIDATION.md records what was actually verified"
    verification:
      - kind: integration
        ref: "go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1"
        status: pass
      - kind: other
        ref: "go test ./internal/ado/ -v -run TestGet | grep -cE '^=== RUN   TestGet[A-Za-z]*SwallowsProjectFailure$' returns 0"
        status: pass
    human_judgment: false
  - id: D9
    description: "The rendered TUI actually shows the failure banner legibly against a real rejected PAT"
    verification: []
    human_judgment: true
    rationale: "Every view has a View()-substring assertion covering the same ground at the string level, but a substring assertion is not a visual check. Nobody has run the tool against an invalid PAT since these six views changed, so legibility, layout and colour contrast of the banner in a real terminal are unverified. 02-VALIDATION.md keeps this row open and explicitly does not claim it."

duration: 41 min
completed: 2026-09-07
status: complete
---

# Phase 02 Plan 05: TUI Failure Surfacing Summary

**All six view packages now distinguish a rejected token from an empty backlog: an auth failure replaces the data surface with a banner, a non-auth partial keeps the rows and names what did not answer, and no view asserts an absence of work — or blames the user's config.yaml for one — unless the read actually completed.**

## Performance

- **Duration:** 41 min
- **Tasks:** 4 (three TDD, one phase-close)
- **Files modified:** 13 (5 created, 8 modified)

## Accomplishments

- **The dashboard stub 02-03 flagged as the phase's highest-stakes gap is closed.** It received `Failures` and `Complete` and read neither; three integers with no visible seam rendered a partial sum with exactly the authority of a correct one. It now blocks on auth and states in words that the totals are understated.
- **Both pipelines misdirection sites are gated, not just one.** The reducer's status-line message (`:232`) and the `View()` render branch (`:397`) both told the user to check their `config.yaml` when nothing had been fetched. Gating only the reducer would have left the text the user actually reads intact. The same applies to the releases render at `:426`.
- **The auth-versus-partial asymmetry is uniform across all six views** and is enforced by a per-view `applyResult` helper rather than by six hand-written reducer arms that could drift.
- Every read fetch in the TUI now runs under a derived deadline. Every write call site is untouched — verified by diff, not by assertion.
- **The TUI went from one tested view package to six.**

## Task Commits

1. **Task 1 RED: pipelines and releases failure surfacing** — `dabc5d6` (test)
2. **Task 1 GREEN: pipelines and releases** — `fff468c` (feat)
3. **Task 2 RED: workitems and branches failure surfacing** — `26eff79` (test)
4. **Task 2 GREEN: workitems and branches** — `836bac8` (feat)
5. **Task 3 RED: dashboard understated total** — `7a5db53` (test)
6. **Task 3 GREEN: dashboard** — `5fd7032` (feat)
7. **Task 4: validation map sign-off** — `4a386cc` (docs)

## The Scope Call on D-15's Stale Marker

**The stale marker was applied to the three polling views only — prs, pipelines and releases — and deliberately omitted from workitems and branches.**

D-15 names the polling views specifically, and the reason holds on inspection: staleness is a claim about elapsed time since the last successful read. A polling view has a timer that makes that claim meaningful — rows on screen are asserted to be no more than one poll interval old, and a failed refresh breaks that assertion. Workitems and branches refresh only when the user presses `r`, so their rows are *always* as old as the last manual refresh. A marker there would either fire constantly or assert a staleness the view has no timer to measure. The omission is recorded in a comment at each model's field block rather than left silent.

## The Wording Chosen for the Dashboard's Understated-Total Warning

The rendered line is:

```
totals understated; did not answer: project-two (timeout: get dashboard pull request count)
```

and, when an iteration lookup degraded:

```
totals understated; counted unfiltered by iteration: project-one (other: get dashboard iterations)
```

Three deliberate choices:

1. **"totals understated" leads.** D-03's justification is that a labelled understatement beats a silent one, and the label is the part that makes it true. Naming the failed sub-query alone leaves the user to work out the effect on the number — which is precisely the inference this phase exists to remove.
2. **The phrase lives in the model field, not the renderer.** `RenderUnderstatedWarning` was reduced to styling and truncation only. This is what makes the claim assertable by a test rather than merely visible on screen; the acceptance criterion asks the warning to *state* the understatement, and a test can only check a value it can reach.
3. **The degraded iteration lookup is worded separately.** Per 02-03, a failed lookup drops the WIQL `IN (...)` clause, so the work-item count comes back **larger** than intended. Wording it identically to a project that went silent would be actively wrong about the direction of the error, and would train the user to ignore a line that means something different each time. `summarizeCountFailures` partitions on the operation label.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extracted `internal/ui/fetchctx` to satisfy the context-count criteria honestly**

- **Found during:** Task 1 (verifying acceptance criteria)
- **Issue:** The criteria require `grep -c 'context.Background()'` to be *strictly lower* after the change. Following 02-01's inline pattern literally (`ctx := context.Background()` then `context.WithTimeout(ctx, budget)`) leaves the count unchanged, because `WithTimeout` needs a parent and `Background()` is that parent. The criterion was unsatisfiable without either grep-gaming or a structural change.
- **Fix:** Extracted the identical six-line block from all six read fetches into `internal/ui/fetchctx.New(budget)`. No view file now mentions `context.Background()` on a read path, so the absence of an unbounded read context is visible by inspection. Counts dropped genuinely: pipelines 3→1, releases 4→2, workitems 6→5, prs 3→2 (remainders are all write call sites).
- **Files modified:** `internal/ui/fetchctx/fetchctx.go` (new), all six view files
- **Verification:** `go build ./... && go test ./... -count=1` green; per-file grep counts recorded above
- **Committed in:** `fff468c`, `836bac8`, `5fd7032`

**2. [Rule 3 - Blocking] `workitems.New` made tolerant of a nil client**

- **Found during:** Task 2
- **Issue:** `New` called `client.CurrentUser()` and `client.CurrentUserEmail()` unconditionally, so `New(nil, ...)` panicked. Every other view tolerates a nil client, which is what the plan's mandated nil-client test-helper pattern depends on.
- **Fix:** Guarded both reads behind a nil check, leaving the fields at their zero value. The constructor is now total.
- **Files modified:** `internal/ui/workitems/workitems.go`
- **Verification:** `go test ./internal/ui/workitems/ -count=1` green
- **Committed in:** `836bac8`

**3. [Rule 1 - Bug] `internal/ui/prs/prs.go` touched despite being 02-01's output**

- **Found during:** Task 1
- **Issue:** Not a bug in prs, but a consistency problem: leaving prs on the inline deadline block while the other five used the shared helper would have left two patterns for one job in the same package tree.
- **Fix:** prs's read fetch now calls `fetchctx.New`. Its write call sites and every other line are unchanged; behaviour is identical.
- **Files modified:** `internal/ui/prs/prs.go`
- **Verification:** `go test ./internal/ui/prs/ -count=1` green — 02-01's five tests still pass unmodified
- **Committed in:** `fff468c`

**4. [Rule 1 - Bug] Two validation-map commands matched zero tests**

- **Found during:** Task 4
- **Issue:** The `getProjectCounts` row and the `TestGet.*AuthFailure` row carried seeded patterns that match no test in the tree. `go test` exits `ok` for a pattern matching nothing, so marking either green would have recorded a pass that proved nothing.
- **Fix:** Corrected both commands to the real test names (`TestGetProjectCountsReportsPartialFailures` / `TestGet*ClassifiesRejectedToken`), re-ran them, confirmed 7 tests each, and recorded the correction in the Approval block.
- **Files modified:** `.planning/phases/02-honest-reads/02-VALIDATION.md`
- **Committed in:** `4a386cc`

---

**Total deviations:** 4 auto-fixed (3 blocking, 1 correctness)
**Impact on plan:** No scope creep. Deviations 1 and 2 were required to satisfy stated acceptance criteria; 3 is a one-line consistency change inside this plan's own scope; 4 prevented a false green in the phase's sign-off artifact.

## Validation Sign-Off — Every Box Ticked, With One Caveat Stated

`02-VALIDATION.md` is now `status: validated`, `nyquist_compliant: true`, `wave_0_complete: true`. Every sign-off box was genuinely tickable, so the frontmatter was flipped as the plan permits.

**The caveat, which is recorded in the file and not hidden:** the manual-only row *"Rendered TUI output actually shows the failure banner"* is **not** done and is explicitly not claimed. All six views have a `View()`-substring assertion covering the same ground, which is why the automated rows are green — but a substring assertion is not a visual check. Nobody has run this tool against an invalid PAT since these six views changed, so the banner's legibility, layout and colour contrast in a real terminal are unverified. That row is the correct subject for `/gsd-verify-work`. The sign-off boxes concern automated-verify coverage and feedback latency, which are satisfied; the manual row is tracked separately by design.

The other manual row, assumption A1 (a rejected PAT yields 401), **is** done — verified empirically during 02-01 before the classifier was finalized.

## Issues Encountered

One, and it was informative. The dashboard's `TestUpdatePartialFailureRendersCountsAndSaysUnderstated` failed on first GREEN run because "understated" lived only in the renderer, not in the model field the test could reach. Rather than weaken the test to assert on `View()` output, the phrase was moved into the model field and the renderer reduced to styling. The test was right: a claim the code makes only at render time is a claim no test can hold it to.

## Known Stubs

None. The dashboard stub 02-03 recorded is closed.

## User Setup Required

None.

## Next Phase Readiness

**Phase 2 is complete.** All eight aggregate reads carry the envelope, all six views render it honestly, the whole tree is green in one command, and the validation map records what was actually verified.

Carried forward, neither a blocker:

- **Warning-line noise is the open empirical question.** Two sources could make the partial warning routine on healthy configurations: 02-03's degraded iteration lookup (any org whose projects have no iterations configured) and 02-02's assumption A4 (404 suppression for empty repositories, inferred from a log comment rather than observed). If the line proves noisy in practice, those are the two places to look, in that order.
- **T-02-03 remains accepted, not fixed.** Under a rejected token each view still issues one round trip per configured project per refresh — every thirty seconds for the polling views. `fetchctx` bounds each fetch's *duration* but not the *number* of requests. PLAT-03 owns the fix. Surfaced, not absorbed.

---
*Phase: 02-honest-reads*
*Completed: 2026-09-07*

## Self-Check: PASSED

All 5 created files verified present on disk. All 7 commit hashes verified present in git history. Repo-wide gate re-run clean: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test ./... -count=1`.
