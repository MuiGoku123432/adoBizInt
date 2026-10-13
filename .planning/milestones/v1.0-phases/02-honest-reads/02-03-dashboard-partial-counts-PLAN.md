---
phase: 02-honest-reads
plan: 03
type: execute
wave: 3
depends_on: [02-01, 02-02]
files_modified:
  - internal/ado/dashboard.go
  - internal/ado/dashboard_test.go
  - internal/ui/dashboard/dashboard.go
autonomous: true
requirements: [TRUST-01, TRUST-03]

estimate:
  tokens: 52000
  raw_tokens: 26000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "GetDashboardCounts returns the sum of the projects that answered, a failure list, and a completeness flag — a partial total is labelled understated rather than zeroed or discarded"
    - "getProjectCounts returns both the counts it did collect and the failures it hit, so a project that answered two of three sub-queries contributes those two to the total"
    - "Each of the three in-place sub-query failures is reported as a distinct failure entry naming the project and which sub-query failed"
    - "A dashboard result is incomplete whenever any project or any sub-query failed, even when the returned total is non-zero"
    - "The degraded iteration lookup is a conscious, recorded outcome rather than a silent fallback"
    - statement: "Failures are appended in configured-project iteration order, and within a project in sub-query evaluation order"
      verification: backstop
  prohibitions:
    - "Must NOT zero or discard the dashboard total when some projects failed — D-03 keeps the sum of the projects that answered and labels it understated"
    - "Must NOT report a partial dashboard total without flagging the result incomplete"
  artifacts: []
  key_links:
    - "getProjectCounts partial return -> the outer summation loop -> Result.Complete"
    - "classifyFailure -> each of the three sub-query failure entries"
---

<objective>
Make the dashboard honest. It is the one read where a partial result is a single
number with no visible seam, and the one read where a mechanical edit changes
nothing observable.

Purpose: `GetDashboardCounts` looks structurally identical to the other seven at
the loop level, and it is not. Its outer `continue` at dashboard.go:51 is dead
code, because `getProjectCounts` returns a nil error unconditionally at :141. Its
three real failures are logged and discarded in place at :102, :116 and :127.
Editing only the outer loop produces a commit that builds, vets, passes, and fixes
nothing — the existing characterization test would still pass unchanged, which is
the warning sign.

Output: `getProjectCounts` returning both its partial counts and its failures, the
outer loop still summing what came back, and a total that is labelled understated
rather than silently wrong.
</objective>

<call_site_scope>
Changing `GetDashboardCounts`'s return type breaks `internal/ui/dashboard`. Task 2
does the **minimum** call-site edit needed to keep `go build ./...` green: change
`CountsMsg` to carry the envelope per D-16 and unwrap the counts where the model
previously received a bare pointer. Nothing else.

The reducer and render work — the auth banner, the understated-total warning line,
and the completeness gate of D-03, D-13 and D-14 — belongs to plan 02-05, which runs
in a later wave. Do not start it here.

Touch only `internal/ui/dashboard/dashboard.go`. The other four view packages were
already compile-fixed by plan 02-02, which ran in the **previous** wave.

**Why this plan is serialized after 02-02 rather than parallel with it.** This plan
edits `internal/ado/dashboard.go`; 02-02 edits `workitems.go`, `pipelines.go`,
`releases.go` and `branches.go`. Those are different *files* but the same Go
*package*. Go compiles per package: Task 1 below changes the `getProjectCounts`
signature, and while that edit is in flight `internal/ado` does not compile, so every
`go build`/`go test` in 02-02 would fail spuriously — and vice versa. There is no safe
interleaving, so 02-02 must land first. `files_modified` overlap being zero is not
sufficient for Go.
</call_site_scope>

<execution_context>
@/Users/e1001547-mbp-it/.config/opencode/gsd-core/workflows/execute-plan.md
@/Users/e1001547-mbp-it/.config/opencode/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/02-honest-reads/02-CONTEXT.md
@.planning/phases/02-honest-reads/02-RESEARCH.md
@.planning/phases/02-honest-reads/02-PATTERNS.md
@.planning/phases/02-honest-reads/02-VALIDATION.md
@.planning/phases/02-honest-reads/02-01-SUMMARY.md
@.planning/codebase/CONVENTIONS.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Make getProjectCounts return its partial counts alongside its failures</name>
  <files>internal/ado/dashboard.go, internal/ado/dashboard_test.go</files>
  <read_first>
    - `internal/ado/dashboard.go` in full — `GetDashboardCounts` at :23, the dead
      outer `continue` at :48-52, the summation at :59-61, the sole return at :70,
      `getProjectCounts` at :74, the degraded iteration lookup at :80-82, the three
      in-place swallows at **:102**, **:116** and **:127**, and the unconditional
      `return counts, nil` at :141.
    - `internal/ado/result.go` from 02-01 — the envelope, the failure helper, and the
      finaliser.
    - `internal/ado/pullrequests.go` as it stands after 02-01 — the outer-loop template.
    - `internal/ado/dashboard_test.go:17-19` — the comment that already records this
      exact structural trap, and :71-76 — the three-fake wiring, including the bare
      build fake that asserts a call is never made.
    - `.planning/phases/02-honest-reads/02-PATTERNS.md` §"internal/ado/dashboard.go"
      — the three `if err != nil { log } else if <nil-guard> { assign }` chains, and
      the note that removing the `else` changes the assignment guard too.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pitfall 4" (lines 588-594).
  </read_first>
  <behavior>
    - A project whose work-item query fails but whose PR and build queries succeed
      returns the PR and pipeline counts it collected, plus one failure naming the
      project and the work-item query.
    - A project whose work-item query succeeds and whose PR query fails returns the
      work-item count it collected, plus one failure naming the PR fetch.
    - A project where all three sub-queries fail returns zero counts and three
      distinct failure entries.
    - A project where all three sub-queries succeed returns its counts and no failures.
    - The three failure entries are distinguishable from one another by their
      operation label.
  </behavior>
  <action>
    Change `getProjectCounts` at `dashboard.go:74` so it returns both the counts it
    collected and the failures it hit. This is the signature D-03 requires and the one
    place in the repo with no analog — no existing helper returns a partial value
    alongside its failures, so specify it deliberately rather than pattern-matching.

    Do **not** convert it to a plain error return. Returning an error would push the
    outer loop back onto its `continue` and discard the counts the project did
    provide, degrading "the sum of the projects that answered" into "the sum of the
    projects that answered perfectly". D-03 explicitly keeps the good data.

    At each of the three swallow sites — **:102** (the work-item WIQL query), **:116**
    (the pull-request fetch) and **:127** (the build count, inside the pipelines
    guard) — record a failure carrying the project, an operation label that identifies
    which sub-query failed, and the classified kind from 02-01's classifier.

    Handle the `else if` structure carefully. Each site is an
    `if err != nil { log } else if <nil-guard> { assign }` chain, so the nil guard is
    the `else` of the error branch. Adding the failure record inside the existing
    error branch preserves that relationship; restructuring the chain changes which
    values get assigned. Keep the existing `log.Error` calls where they are.

    Leave `return counts, nil` at :141 replaced by the new two-value return, and make
    sure the counts value is returned even when all three sub-queries failed — a
    zero-count struct with three failures is the correct answer, not a nil.

    Rewrite the existing characterization test in place, keeping its current name for
    the end-of-phase rename sweep, and add cases for each bullet in the behavior block.
    <!-- planner-discipline-allow: SwallowsProjectFailure -->
    Reuse the three-fake wiring already in that file. If the dashboard characterization
    test still passes unchanged after this task, the task was not done — that is the
    documented warning sign for this specific read.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./internal/ado/ && gofmt -l internal/ado && go test ./internal/ado/ -count=1 -v -run 'Dashboard|ProjectCounts'</automated>
  </verify>
  <acceptance_criteria>
    - `getProjectCounts` returns two values, and the counts value is non-nil on every
      return path including the all-failed path.
    - `go test ./internal/ado/ -count=1 -v -run 'Dashboard|ProjectCounts'` includes a
      passing case asserting that a project with a failed work-item query still
      contributes its PR and pipeline counts.
    - `go test ./internal/ado/ -count=1 -v` includes a passing case asserting three
      distinct failure entries with three distinct operation labels when all three
      sub-queries fail.
    - `git diff internal/ado/dashboard_test.go` is non-empty — the characterization
      test changed.
    - `grep -c 'log.Error' internal/ado/dashboard.go` is unchanged from before this task.
    - `gofmt -l internal/ado` prints nothing.
  </acceptance_criteria>
  <done>A project that answered two of three sub-queries contributes those two to the total and reports the third as a named failure.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Return the dashboard envelope and decide the degraded iteration lookup</name>
  <files>internal/ado/dashboard.go, internal/ado/dashboard_test.go</files>
  <read_first>
    - `internal/ado/dashboard.go` as it stands after Task 1.
    - `internal/ado/dashboard.go:80-82` — the iteration lookup logged at `Warn` and
      discarded with an explicit fallback comment about using a date filter only.
    - `internal/ado/result.go` from 02-01.
    - `.planning/phases/02-honest-reads/02-PATTERNS.md` §"Planner Notes" item 3.
    - `.planning/phases/02-honest-reads/02-CONTEXT.md` D-03 and D-12.
  </read_first>
  <behavior>
    - Two configured projects, one of which fails entirely: the surviving project's
      counts are the returned total, the failure is listed, and the result is incomplete.
    - Two configured projects, both of which answer fully: the total is their sum and
      the result is complete with no failures.
    - A project whose iteration lookup degrades produces an outcome the test can
      observe — either a recorded failure or an explicitly asserted absence of one —
      rather than the current silent difference in the query filter.
    - An already-expired fetch context produces a timeout-kind failure for each
      unreached project and an incomplete result.
  </behavior>
  <action>
    Change `GetDashboardCounts` at `dashboard.go:23` to return the shared envelope
    from 02-01 carrying the summed counts, the collected failures, and the derived
    completeness flag.

    Keep the summation at :59-61 running for every project that returned counts,
    including projects that also returned failures. Collect each project's failures
    into the envelope's failure list. The outer `continue` at :51 is dead today
    because `getProjectCounts` never returned an error; now that it returns failures
    instead, that branch should disappear rather than be preserved as decoration —
    but the loop must still visit every configured project, because D-08 forbids
    short-circuiting.

    Note the envelope's items field: the dashboard's payload is a single aggregate
    counts value, not a slice. Decide and record how it is carried — either as a
    one-element items slice or by giving the counts their own field alongside the
    shared failure and completeness fields. Whichever is chosen, the failure list and
    the completeness flag must be the same shape as the other seven reads, because
    D-01's whole purpose is that Phase 3 writes its JSON handling once.

    Add the already-expired-context guard from the `pullrequests.go` template for D-12.

    **Decide the fourth swallow.** `dashboard.go:80-82` logs a failed iteration lookup
    at `Warn` and falls back to a date-only filter. This is not covered by the CONTEXT
    inventory and it is not a neutral fallback: it changes the WIQL filter, and
    therefore it changes the count that gets summed into a number the user reads as
    authoritative. Recommendation: record it as a failure, because a count produced
    under a different filter than intended is exactly the kind of quiet inaccuracy
    this phase exists to remove, and D-03's contract is that a partial total is
    labelled rather than silently understated.

    If you choose instead to leave it as a silent fallback, say so explicitly in the
    plan summary with the reasoning, and add a test asserting the absence of a failure
    so the choice is pinned rather than assumed. Either way it stops being an
    unrecorded behavior.

    Extend the dashboard test file to cover every bullet in the behavior block.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && gofmt -l . && go test ./internal/ado/ -count=1 -v -run 'Dashboard'</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./...` succeeds, proving the dashboard view was updated to acknowledge
      the new return shape.
    - `go test ./internal/ado/ -run 'TestGetDashboardCounts' -count=1 -v` includes a
      passing case asserting a non-zero total alongside a non-empty failure list and an
      incomplete result.
    - `go test ./internal/ado/ -run 'TestGetDashboardCounts' -count=1 -v` includes a
      passing case asserting a clean two-project sweep reports complete with an empty
      failure list and a total equal to the sum of both projects.
    - `go test ./internal/ado/ -count=1 -v` includes a passing case that pins the
      chosen behavior for the degraded iteration lookup, either asserting a failure is
      recorded or asserting none is.
    - `go test ./internal/ado/ -run TestFetchTimeoutMarksUnreached -count=1 -v` passes
      with a `GetDashboardCounts` case asserting an already-expired fetch context
      produces a timeout-kind failure for each unreached project and `Complete == false`
      (D-12, `02-VALIDATION.md:57`).
    - `grep -vn '^\s*//' internal/ado/dashboard.go | grep -c 'Skip projects with errors'` returns `0`.
    - `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>The dashboard total is the sum of the projects that answered, explicitly flagged incomplete when any of them did not, and the fourth swallow is a recorded decision instead of an unrecorded behavior.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| ado client -> Azure DevOps REST API | Three sub-queries per project cross here; responses are server-controlled |
| dashboard total -> user's judgement | A single number with no visible seam; the completeness flag is the only signal that it is understated |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-01 | Information Disclosure | the three sub-query failure entries in `getProjectCounts` | high | mitigate | Each entry carries only project, operation label, kind, and the underlying error, reusing 02-01's `Failure` struct unchanged. The operation labels are literal sub-query names chosen in source, not values derived from a request URL. |
| T-02-03 | Denial of Service | the dashboard fan-out under D-08 | medium | **accept** | Each project issues three sub-queries and the build query is uncapped (CONCERNS.md MED3). Under a rejected token, every project's three queries still run on every 30-second refresh because D-08 rejected short-circuiting. `fetch_timeout` bounds the duration only. PLAT-03 owns the fix. Surfaced, not absorbed. |
| T-02-04 | Denial of Service (self-inflicted) | the dashboard outer loop | high | mitigate | The already-expired-context guard from 02-01 stops the sweep when the fan-out budget is exhausted (D-12). Transport-level bounding comes from `connection.Timeout`, set in 02-01. |
| T-02-05 | Repudiation / integrity of reported state | the summed total under a degraded WIQL filter (`dashboard.go:80-82`) | medium | mitigate | A count produced under a silently substituted filter is presented with the same authority as a correct one. Task 2 forces the degradation to become either a recorded failure or an explicitly pinned and justified absence, so the number's provenance is no longer unrecorded. |
| T-02-SC | Tampering | go module installs | n/a | mitigate | This plan adds zero dependencies; `go.mod` and `go.sum` must be unchanged. No package legitimacy checkpoint required. |
</threat_model>

<artifacts_this_phase_produces>
This plan creates **no new exported symbols**.

| Kind | Symbol / path |
|------|---------------|
| signature change | `ado.GetDashboardCounts` now returns the 02-01 envelope |
| signature change | `ado.getProjectCounts` now returns partial counts alongside its failures |

All types, kinds, helpers, and config keys referenced here are created by 02-01.
</artifacts_this_phase_produces>

<verification>
- `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` is green.
- The dashboard characterization test file has a non-empty diff — an unchanged one is
  the documented signature of a dashboard "inversion" that did nothing.
- A test pins the chosen behavior for the degraded iteration lookup.
- `git diff --stat go.mod go.sum` is empty.
</verification>

<success_criteria>
- Three separate sub-query failures per project are each reported and named.
- The total keeps the data from healthy projects and is flagged incomplete.
- The dead outer `continue` is gone without the loop skipping any project.
- The fourth swallow is a decision on the record.
</success_criteria>

<output>
Create `.planning/phases/02-honest-reads/02-03-SUMMARY.md` when done. It must record
the decision made on the degraded iteration lookup at `dashboard.go:80-82`, and how
the aggregate counts value is carried inside the shared envelope shape.
</output>
