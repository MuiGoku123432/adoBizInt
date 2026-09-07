---
phase: 02-honest-reads
plan: 02
type: execute
wave: 2
depends_on: [02-01]
files_modified:
  - internal/ado/workitems.go
  - internal/ado/workitems_test.go
  - internal/ado/pipelines.go
  - internal/ado/pipelines_test.go
  - internal/ado/releases.go
  - internal/ado/releases_test.go
  - internal/ado/branches.go
  - internal/ado/branches_test.go
  - internal/ui/workitems/workitems.go
  - internal/ui/pipelines/pipelines.go
  - internal/ui/releases/releases.go
  - internal/ui/branches/branches.go
autonomous: true
requirements: [TRUST-01, TRUST-02, TRUST-03, TRUST-04]

estimate:
  tokens: 84000
  raw_tokens: 42000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "GetWorkItems, GetPipelineRuns, GetPipelineDefinitions, GetReleases, GetPendingApprovals and GetBranches each return the shared result envelope and each reports a per-project failure naming the project and the operation"
    - "A GetWorkItems batch fetch that fails inside getProjectWorkItems is reported as a failure, so a project that answered partially is never marked complete"
    - "A GetBranches per-repository failure is reported and names the repository as part of the operation, not only the project"
    - "An empty repository is not reported as a branch failure, because it is distinguished by status code from a real fetch error"
    - "An injected 401 on any of these six reads produces an auth-kind failure and a non-empty failure list even when the items slice is empty"
    - "When the caller's fetch context has already expired, each project not reached receives a timeout-kind failure and the result is incomplete"
    - "The data-filter continue statements in pipelines.go and releases.go are byte-for-byte unchanged"
    - statement: "Failures are appended in configured-project iteration order, and within GetBranches per-repository failures follow their owning project's entry"
      verification: backstop
  prohibitions:
    - "Must NOT short-circuit or skip the remaining configured projects once an auth failure has been observed — D-08 rejected this explicitly after the cost was stated"
  artifacts: []
  key_links:
    - "ado.Result / ado.Failure from 02-01 -> each of the six expanded reads"
    - "classifyFailure -> the auth kind -> the failure list even when items is empty"
---

<objective>
Expand the proven tracer shape across the six remaining simple fan-out reads:
work items (including the batch swallow), both pipeline reads, both release reads,
and branches (including the nested per-repository swallow).

Purpose: 02-01 proved the envelope, the classifier, and the deadline behavior on
one read. These six follow the same template mechanically — but the template only
holds if each edit targets the exact error line from the research inventory. Ten
of the twenty-six `continue` statements in this package are error swallows; the
other sixteen are nil guards and name-pattern filters, and converting those
corrupts the filtering logic.

Output: six aggregate reads returning the shared envelope, six characterization
tests inverted, and new coverage for the two structurally distinct swallows
(the work-item batch and the per-repository branch fetch).
</objective>

<call_site_scope>
Changing these six return types breaks four view packages. Each task in this plan
does the **minimum** call-site edit needed to keep `go build ./...` green: change the
view's message type to carry the envelope per D-16, and unwrap the items where the
model previously received a bare slice. Nothing else.

The reducer and render work — the auth banner, the persistent warning line, the stale
marker, and the completeness-gated zero-row messages of D-13 through D-15 — belongs to
plan 02-05, which runs in the next wave. Do not start it here. A partial rendering
implementation split across two waves is harder to review than a compile fix followed
by a complete one, and 02-05 owns the shared-renderer decisions.

The four view files touched for the compile fix are `internal/ui/workitems/workitems.go`,
`internal/ui/pipelines/pipelines.go`, `internal/ui/releases/releases.go`, and
`internal/ui/branches/branches.go`. Do not touch `internal/ui/dashboard/dashboard.go`
— that belongs to plan 02-03, which runs in parallel with this one.
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
  <name>Task 1: Invert GetWorkItems, including the batch swallow inside getProjectWorkItems</name>
  <files>internal/ado/workitems.go, internal/ado/workitems_test.go</files>
  <read_first>
    - `internal/ado/pullrequests.go` as it stands after 02-01 — this is the template
      to copy, and it is the only correct reference for the accumulator, the failure
      append, the retained `continue`, the extended terminal log line, and the
      already-expired-context guard.
    - `internal/ado/result.go` from 02-01 — the envelope, the failure helper, and the
      finaliser that derives completeness.
    - `internal/ado/workitems.go` — `GetWorkItems` at :56, the per-project swallow at
      :75-78, the sole `return` at :83, `getProjectWorkItems` at :86, and the batch
      skip at :155-158.
    - `internal/ado/workitems_test.go` — the existing characterization test.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Code Inventory" rows 1 and
      the filter-versus-failure warning immediately below the table (lines 383-398).
  </read_first>
  <behavior>
    - Two configured projects, one whose WIQL query fails: the surviving project's
      work items are returned, one failure entry names the failing project and the
      query operation, and the result is incomplete.
    - A project whose WIQL query succeeds but whose subsequent batch fetch fails:
      the work items that were retrieved are returned, a failure entry names that
      project and the batch operation, and the result is incomplete. The project is
      NOT reported as complete just because its query succeeded.
    - An injected 401 on the WIQL query produces an auth-kind failure alongside a
      possibly-empty items slice.
    - A clean sweep across both projects yields a complete result with no failures.
  </behavior>
  <action>
    Change `GetWorkItems` at `workitems.go:56` to return the shared envelope from
    02-01 instead of a bare slice, following the `pullrequests.go` template exactly.

    Record a failure at **two** sites and no others in this file:

    - **workitems.go:77** — the per-project swallow. Record project, an operation
      label for the WIQL query, and the classified kind. Keep the `continue`.
    - **workitems.go:157** — the batch swallow inside `getProjectWorkItems`. This is
      the reason D-02 exists: a project can answer *partially*, and reporting only
      the outer failure would mark that project complete when it is not. Propagate
      these batch failures out of `getProjectWorkItems` alongside the items it did
      collect, so the caller can attribute them.

    Do not touch any other `continue` in this file. If a `continue` is guarding a nil
    pointer or filtering by a name pattern, it stays exactly as it is.

    Keep both existing `log.Error` calls where they are and extend the terminal
    `log.Info` with a failure count rather than adding a second log statement.

    Add the already-expired-context guard from the `pullrequests.go` template so an
    exhausted fetch budget produces timeout-kind failures for unreached projects
    (D-12) rather than a burst of doomed calls.

    Rewrite the existing characterization test in place to assert the propagating
    behavior. Keep its current name; the rename sweep happens at the end of the phase.
    <!-- planner-discipline-allow: SwallowsProjectFailure -->
    Add a new test covering the batch-failure case at :157, which has no existing
    coverage, using the typed-error fake helpers added in 02-01.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./internal/ado/ && gofmt -l internal/ado && go test ./internal/ado/ -count=1 -v -run 'WorkItems'</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./...` succeeds, proving every consumer of `GetWorkItems` was updated.
    - `go test ./internal/ado/ -run 'TestGetWorkItems' -count=1 -v` includes a passing
      case asserting a surviving project's items alongside exactly one failure entry.
    - `go test ./internal/ado/ -count=1 -v` includes a passing test covering a batch
      fetch failure that asserts the result is incomplete while items are still returned.
    - `grep -c 'continue' internal/ado/workitems.go` is unchanged from before this task.
    - `go test ./internal/ado/ -count=1 -v` includes a case injecting a typed 401 and
      asserting the auth kind on a `GetWorkItems` failure entry.
    - `gofmt -l internal/ado` prints nothing.
  </acceptance_criteria>
  <done>Work items report both their per-project and their per-batch failures, so a partially-answering project is never presented as complete.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Invert both pipeline reads and both release reads</name>
  <files>internal/ado/pipelines.go, internal/ado/pipelines_test.go, internal/ado/releases.go, internal/ado/releases_test.go</files>
  <read_first>
    - `internal/ado/pullrequests.go` as it stands after 02-01 — the template.
    - `internal/ado/result.go` from 02-01.
    - `internal/ado/pipelines.go` — `GetPipelineRuns` at :91 with its error swallow at
      **:114**, its sole return at :227, and its data-filter `continue` statements at
      :118, :126 and :170; `GetPipelineDefinitions` at :231 with its error swallow at
      **:253**, its return at :315, and its filters at :257, :271 and :276.
    - `internal/ado/releases.go` — `GetReleases` at :56 with its error swallow at
      **:73**, its return at :197, and its filters at :77, :85 and :132;
      `GetPendingApprovals` at :201 with its error swallow at **:220**, its return at
      :268, and its filter at :224.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pitfall 3" (lines 581-587)
      and the Code Inventory table rows 3 through 6.
    - The two existing characterization tests in each of `pipelines_test.go` and
      `releases_test.go`.
  </read_first>
  <behavior>
    - Each of the four reads, given two projects where one fails, returns the
      surviving project's rows plus exactly one failure entry naming that project and
      the operation, and reports the result incomplete.
    - Each of the four reads, given an injected 401, produces an auth-kind failure and
      a non-empty failure list even when the items slice is empty.
    - Each of the four reads, given a clean sweep, reports complete with no failures.
    - The N+1 inner fan-outs continue to visit every element they visit today — the
      row counts returned on a fully successful fetch are unchanged.
  </behavior>
  <action>
    Change all four reads to return the shared envelope, applying the
    `pullrequests.go` template unchanged.

    Record a failure at exactly four lines across these two files:
    `pipelines.go:114`, `pipelines.go:253`, `releases.go:73`, `releases.go:220`.

    **Every other `continue` in these two files must remain byte-for-byte identical.**
    Seven of them are data filters — nil-pointer guards, name-pattern matches, and
    allow-list checks — and converting one to a failure record both fills the failure
    list with phantom entries and inverts the filtering the views depend on. If you
    find yourself editing a `continue` whose line number is not one of the four above,
    stop and re-read the inventory.

    Preserve the existing `log.Error` placement in each; extend each terminal
    `log.Info` with a failure count.

    Add the already-expired-context guard to each of the four outer loops. These four
    reads carry the N+1 inner fan-outs recorded in CONCERNS.md MED3, so they are the
    reads most likely to exhaust the fan-out budget mid-sweep — D-12's behavior
    matters most here.

    Rewrite the four existing characterization tests in place, keeping their current
    names for the end-of-phase rename sweep, and add one typed-401 case per read.
    <!-- planner-discipline-allow: SwallowsProjectFailure -->
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./internal/ado/ && gofmt -l internal/ado && go test ./internal/ado/ -count=1 -v -run 'Pipeline|Release|Approval'</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./...` succeeds with all four reads returning the envelope.
    - `go test ./internal/ado/ -count=1 -v -run 'Pipeline|Release|Approval'` passes and
      shows, for each of the four reads, a case asserting a surviving project's rows
      alongside exactly one failure entry.
    - `go test ./internal/ado/ -count=1 -v` includes a typed-401 case for each of the
      four reads asserting the auth kind.
    - `grep -c 'continue' internal/ado/pipelines.go` and
      `grep -c 'continue' internal/ado/releases.go` are both unchanged from before
      this task.
    - `git diff internal/ado/pipelines.go internal/ado/releases.go` shows no change on
      any line within three lines of the filter sites at pipelines.go:118, :126, :170,
      :257, :271, :276 and releases.go:77, :85, :132, :224 other than line-number shift.
    - `gofmt -l internal/ado` prints nothing.
  </acceptance_criteria>
  <done>All four pipeline and release reads report their per-project failures, and every data filter in those files is untouched.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Invert GetBranches including the nested per-repository swallow, without turning empty repos into failures</name>
  <files>internal/ado/branches.go, internal/ado/branches_test.go</files>
  <read_first>
    - `internal/ado/pullrequests.go` as it stands after 02-01 — the outer-loop template.
    - `internal/ado/result.go` from 02-01, in particular the classifier's status-code
      extraction.
    - `internal/ado/branches.go` — `GetBranches` at :32, the per-project repository
      swallow at **:49-52**, the nested per-repository branch-stats swallow at
      **:77-78** with its existing `Warn` level and its explanatory comment, and the
      sole return at :126.
    - `internal/ado/branches_test.go:12-59` — the tightest characterization test in
      the package and the shape to preserve.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pitfall 6" (lines 602-608)
      and assumption A4 in the Assumptions Log (line 636).
  </read_first>
  <behavior>
    - Two configured projects, one whose repository listing fails: the surviving
      project's branches are returned, one failure entry names the failing project and
      the repository-listing operation, and the result is incomplete.
    - A project that lists repositories successfully but whose per-repository branch
      fetch fails with a non-404 error: the branches from the other repositories are
      returned, a failure entry names the project and identifies the repository in the
      operation label, and the result is incomplete.
    - A per-repository branch fetch that fails with a 404-family status is treated as
      an ordinary empty repository: no failure entry is recorded, and the result stays
      complete if nothing else failed.
    - An injected 401 on the repository listing produces an auth-kind failure.
  </behavior>
  <action>
    Change `GetBranches` at `branches.go:32` to return the shared envelope, applying
    the `pullrequests.go` template to the outer loop and recording a failure at
    **branches.go:51** with the project and a repository-listing operation label.

    The nested swallow at **branches.go:78** needs a judgement the other seven reads
    do not. Its existing comment records that empty repositories have no branch stats
    to report, so the SDK is signalling "this repository has no branches" as an error.
    Classifying that as a failure would put a permanent warning line on the branches
    view naming every empty repository in the org — noise that trains the user to
    ignore the exact channel D-13 depends on.

    Distinguish by status code, which is the mechanism D-06 exists to provide:
    reuse the classifier's status-code extraction, and when the status is in the
    404 family treat it as an ordinary empty repository — keep the existing `Warn` log
    and record no failure. For any other status, record a failure whose operation
    label identifies the repository as well as the project, so a user reading the
    warning line can tell which repository is broken.

    D-07 fixed the kind set at four and explicitly deferred a not-found kind, so do
    not add a fifth kind here — suppress the empty-repository case rather than naming
    it. Leave a comment recording that this is where a not-found kind would have
    earned its keep, and that the deferred-ideas list in `02-CONTEXT.md` is where it
    would come back from.

    Note that assumption A4 (that an empty repository yields a 404-family response) is
    inferred from the existing log comment, not observed. If the suppression turns out
    to be wrong in practice, the failure it produces is noise rather than a lost
    failure, which is the safe direction for it to be wrong in. Record that in the
    plan summary.

    Rewrite the existing characterization test in place, keeping its current name for
    the end-of-phase rename sweep.
    <!-- planner-discipline-allow: SwallowsProjectFailure -->
    Add new tests for the per-repository failure at :78 — which has no coverage today
    — including one case injecting a 404-family status asserting no failure is
    recorded and one injecting a 500 asserting a failure is.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./internal/ado/ && gofmt -l internal/ado && go test ./internal/ado/ -count=1 -v -run 'Branch'</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./...` succeeds with `GetBranches` returning the envelope.
    - `go test ./internal/ado/ -run 'TestGetBranches' -count=1 -v` includes a passing
      case asserting a surviving project's branches alongside exactly one failure entry.
    - `go test ./internal/ado/ -count=1 -v` includes a passing case injecting a
      404-family per-repository error and asserting the failure list is empty and the
      result is complete.
    - `go test ./internal/ado/ -count=1 -v` includes a passing case injecting a
      500 per-repository error and asserting exactly one failure whose operation label
      contains the repository name.
    - `grep -c 'FailureKind' internal/ado/result.go` shows no fifth kind was added:
      the constant count is unchanged from 02-01.
    - `gofmt -l internal/ado` prints nothing.
  </acceptance_criteria>
  <done>Branches report both their per-project and their per-repository failures, and an ordinary empty repository is still an ordinary empty repository.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| ado client -> Azure DevOps REST API | The PAT crosses here on every call in six more reads; responses are server-controlled |
| `Failure.Operation` label -> TUI render | The operation label now embeds a repository name sourced from the ADO response |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-01 | Information Disclosure | the failure entries recorded by all six expanded reads | high | mitigate | Every failure records only project, operation label, kind, and the underlying error, reusing the 02-01 `Failure` struct unchanged. No read in this plan may add a field or embed a request URL or header into the operation label. |
| T-02-02 | Information Disclosure / Spoofing | `GetBranches` operation label embedding a server-sourced repository name | medium | mitigate | The repository name is server-controlled text entering the operation label. Render it through the same truncating renderers added in 02-01; do not introduce a second, untruncated display path for it. |
| T-02-03 | Denial of Service | the six expanded fan-out loops under D-08 | medium | **accept** | Each of these six reads now records a failure and continues, so a rejected token still costs one round trip per configured project per refresh — and the four N+1 reads here multiply that by their inner fan-outs (CONCERNS.md MED3). Accepted per D-08; `fetch_timeout` bounds the duration but not the request count. PLAT-03 owns the fix. Surfaced, not absorbed. |
| T-02-04 | Denial of Service (self-inflicted) | the four N+1 fan-outs in pipelines.go, releases.go and branches.go | high | mitigate | Each outer loop gains the already-expired-context guard from 02-01, so an exhausted fan-out budget stops issuing calls rather than running for many multiples of it (D-09, D-12). Transport-level bounding comes from `connection.Timeout`, set in 02-01. |
| T-02-SC | Tampering | go module installs | n/a | mitigate | This plan adds zero dependencies; `go.mod` and `go.sum` must be unchanged. No package legitimacy checkpoint required. |
</threat_model>

<artifacts_this_phase_produces>
This plan creates **no new exported symbols**. It changes the return type of six
existing functions and adds test cases to four existing test files.

| Kind | Symbol / path |
|------|---------------|
| signature change | `ado.GetWorkItems` now returns the 02-01 envelope |
| signature change | `ado.GetPipelineRuns` now returns the 02-01 envelope |
| signature change | `ado.GetPipelineDefinitions` now returns the 02-01 envelope |
| signature change | `ado.GetReleases` now returns the 02-01 envelope |
| signature change | `ado.GetPendingApprovals` now returns the 02-01 envelope |
| signature change | `ado.GetBranches` now returns the 02-01 envelope |
| signature change | `ado.getProjectWorkItems` now propagates its batch failures |

All types, kinds, helpers, and config keys referenced here are created by 02-01.
</artifacts_this_phase_produces>

<verification>
- `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` is green.
- `grep -c 'continue'` on `workitems.go`, `pipelines.go` and `releases.go` is unchanged
  from before this plan — every retained `continue` is either a preserved data filter
  or a preserved D-08 non-short-circuit.
- Every one of the six reads has at least one test asserting a surviving project's rows
  alongside a failure entry, and one asserting an auth-kind failure from a typed 401.
- `git diff --stat go.mod go.sum` is empty.
</verification>

<success_criteria>
- Six more aggregate reads carry failures and a completeness flag.
- The two structurally distinct swallows — the work-item batch and the per-repository
  branch fetch — are reported, not just the six easy outer loops.
- Not one data filter was converted into a failure record.
- An ordinary empty repository still produces no warning.
</success_criteria>

<output>
Create `.planning/phases/02-honest-reads/02-02-SUMMARY.md` when done. It must record
the 404-family suppression decision for empty repositories, and note that assumption
A4 remains inferred rather than observed.
</output>
