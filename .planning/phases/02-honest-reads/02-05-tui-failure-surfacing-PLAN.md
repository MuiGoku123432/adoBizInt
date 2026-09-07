---
phase: 02-honest-reads
plan: 05
type: execute
wave: 3
depends_on: [02-01, 02-02, 02-03]
files_modified:
  - internal/ui/styles/styles.go
  - internal/ui/dashboard/dashboard.go
  - internal/ui/dashboard/dashboard_test.go
  - internal/ui/workitems/workitems.go
  - internal/ui/workitems/workitems_test.go
  - internal/ui/pipelines/pipelines.go
  - internal/ui/pipelines/pipelines_test.go
  - internal/ui/releases/releases.go
  - internal/ui/releases/releases_test.go
  - internal/ui/branches/branches.go
  - internal/ui/branches/branches_test.go
autonomous: true
requirements: [TRUST-02, TRUST-03]

estimate:
  tokens: 92000
  raw_tokens: 46000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "All eight view message types carry the result envelope rather than a bare slice or a bare counts pointer, so failure and completeness travel with the payload in one shape"
    - "Every one of the six views renders a distinct blocking banner instead of its table when the result carries an auth failure"
    - "Every one of the six views renders its rows plus a persistent warning line naming the failed projects and why when the result is merely incomplete"
    - "The three polling views keep their last good rows on screen when a refresh fails and mark those rows stale rather than implying they are current"
    - "No view presents zero rows or a zero count as an absence of work when the result is incomplete"
    - "The pipelines view no longer attributes an empty result to configuration when completeness is unknown or false"
    - "Only the primary message per view drives the blocking auth banner; the secondary definitions and approvals fetches keep their existing status-line warning"
    - "Server-controlled error text reaching a banner or warning line is truncated before it reaches the terminal"
    - statement: "When a view receives a partial result and then a complete one, the warning line clears rather than persisting across the refresh"
      verification: backstop
  prohibitions:
    - "Must NOT present a zero-row or zero-count result as an absence of work when the result is flagged incomplete"
    - "Must NOT report a partial dashboard total without labelling it understated"
  artifacts:
    - internal/ui/dashboard/dashboard_test.go
    - internal/ui/workitems/workitems_test.go
    - internal/ui/pipelines/pipelines_test.go
    - internal/ui/branches/branches_test.go
  key_links:
    - "ado.Result -> the eight view message types -> each reducer -> the auth banner, warning line, and stale marker"
    - "Result.Complete -> each view's zero-row render branch"
---

<objective>
Make the remaining five views tell the truth. The prs view already does, from the
02-01 tracer; dashboard, workitems, pipelines, releases and branches do not.

Purpose: success criterion 6 of the phase is that the TUI renders failures rather
than silently showing an empty table. Everything upstream of the render is now
honest — the reads carry failures and a completeness flag — but a user still sees
an empty table, and in two views still sees a message actively asserting there is
no work when in fact nothing was successfully fetched. The phase goal is only half
met until this lands.

Output: eight message types carrying the envelope, six reducers implementing the
asymmetry between auth failure and partial data, five new view test files, and
every zero-row message gated on completeness.
</objective>

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
@.planning/phases/02-honest-reads/02-02-SUMMARY.md
@.planning/phases/02-honest-reads/02-03-SUMMARY.md
@.planning/codebase/CONVENTIONS.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Carry the envelope on the three polling views and implement banner, warning line, and stale marker</name>
  <files>internal/ui/styles/styles.go, internal/ui/pipelines/pipelines.go, internal/ui/pipelines/pipelines_test.go, internal/ui/releases/releases.go, internal/ui/releases/releases_test.go</files>
  <read_first>
    - `internal/ui/prs/prs.go` as it stands after 02-01 — this is the reference
      implementation for all five remaining views: the envelope-carrying message, the
      fetch command with its fan-out deadline and capture-into-locals closure, the
      reducer arms for auth versus partial versus stale, and the completeness-gated
      zero-row branch.
    - `internal/ui/styles/styles.go` after 02-01 — the banner, warning-line, and
      stale-marker renderers added there.
    - `internal/ui/pipelines/pipelines.go` — `PipelinesMsg` at :34-37, `DefinitionsMsg`
      at :40-43, the fetch commands at :144 and :153, the reducer arms at :225 and
      :238-241, and the zero-row status-line message at **:232-233**.
    - `internal/ui/releases/releases.go` — `ReleasesMsg` at :37-40, `ApprovalsMsg` at
      :43-45, the fetch commands at :145 and :154, and the reducer arms at :237-240
      and :246-249.
    - `internal/ui/releases/releases_test.go` in full — the only existing view test,
      including its nil-client `newTestModel` helper and its regression-comment style.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"The eight message types
      (D-16)" (lines 520-563), especially the recommendation that only the primary
      message per view drives the blocking banner.
    - `.planning/phases/02-honest-reads/02-CONTEXT.md` D-13 through D-16.
  </read_first>
  <behavior>
    - The pipelines reducer, given a primary message whose envelope carries an
      auth-kind failure, sets state that causes the view to render a blocking banner
      and suppress the table.
    - The pipelines reducer, given a primary message whose envelope carries a non-auth
      failure alongside rows, retains the rows and sets a persistent warning naming
      the failed projects and the failure kind.
    - The pipelines reducer, given a transport-level error after a prior successful
      load, retains the previously loaded rows and marks them stale.
    - The pipelines view, given a complete envelope with zero rows, renders its
      no-runs-found message; given an incomplete envelope with zero rows, it does not,
      and in particular does not attribute the emptiness to configuration.
    - The releases reducer exhibits the same four behaviors for its primary message.
    - The secondary definitions and approvals messages set the existing status-line
      warning and do not trigger a blocking banner, even when their envelope carries
      an auth failure.
    - A view that received a partial result and then a complete one no longer shows
      the warning line.
  </behavior>
  <action>
    Apply the prs implementation from 02-01 to the pipelines and releases views. These
    two are the most involved because each carries a primary and a secondary message,
    and pipelines carries the worst instance of the empty-versus-failed confusion.

    Per D-16, change all four message types — the two primary and the two secondary —
    to carry the envelope in place of their bare slices, keeping the existing
    `Err error` field for genuine transport failure.

    In each of the four fetch commands, capture the fan-out budget into a local
    alongside the existing captured fields and replace `context.Background()` with a
    `context.WithTimeout` derived from it, deferring cancel inside the closure. Never
    capture the receiver.

    In the two primary reducer arms, implement the D-13/D-14/D-15 asymmetry exactly as
    prs does. The asymmetry is the feature and must not be flattened: an auth failure
    gets a blocking banner that replaces the table, because a warning line above a
    nearly-empty table invites a user to keep trusting it; a non-auth partial gets the
    rows plus a persistent warning line, because the user can keep working with real
    data as long as they can see what is missing. The warning must not go on the status
    line — that channel already carries action feedback and is easy to miss across a
    thirty-second refresh.

    For the two secondary reducer arms, keep the existing status-line warning behavior.
    Only the primary message per view drives the blocking banner. Without this rule an
    auth failure on a secondary fetch would blank a table the primary fetch populated
    correctly, and two reducers would race to set the banner state.

    Fix `pipelines.go:232-233` specifically. It currently sets a status-line message on
    a successful zero-row fetch that names the configured pipeline list as the likely
    cause. That is the empty-versus-failed confusion in its worst form — it does not
    merely fail to report a failure, it actively misdirects the user toward their own
    configuration. Gate it on the envelope reporting the result complete.

    Add a stale marker to both views. Note that retention of the last good rows is
    already the existing behavior in these reducers, so what this adds is the marker,
    not the retention.

    Extend `internal/ui/releases/releases_test.go` and create
    `internal/ui/pipelines/pipelines_test.go`, both following the existing nil-client
    `newTestModel` helper pattern — the reducer and row-building paths never
    dereference the client, so a nil client is fine. Constructor arity differs per
    package. Drive `Update` with constructed messages and assert on model fields, and
    on `View()` substrings only where a model field cannot express the assertion.

    If any renderer added in 02-01 turns out to be missing a case these two views need,
    extend it in `internal/ui/styles/styles.go` rather than writing per-view rendering.
    Six views need identical treatment, which is well past the rule of three, so the
    shared helper is compliance rather than speculative generality. Do not attempt a
    shared Model — the six views have genuinely divergent state and a shared model
    would be exactly the speculative generality the standards forbid.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt &amp;&amp; go build ./... &amp;&amp; go vet ./internal/ui/... &amp;&amp; gofmt -l internal/ui &amp;&amp; go test ./internal/ui/pipelines/ ./internal/ui/releases/ -count=1 -v</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/ui/pipelines/ -count=1 -v` passes and includes a case driving
      the primary reducer with an auth-carrying envelope asserting the banner state,
      and a case with an incomplete zero-row envelope asserting the configuration-blaming
      message is not produced.
    - `go test ./internal/ui/releases/ -count=1 -v` passes and includes the same two
      cases for its primary message.
    - `go test ./internal/ui/pipelines/ ./internal/ui/releases/ -count=1 -v` includes a
      case per view driving the secondary message with an auth-carrying envelope and
      asserting the blocking banner state is NOT set.
    - `grep -c 'context.Background()' internal/ui/pipelines/pipelines.go` and
      `grep -c 'context.Background()' internal/ui/releases/releases.go` are both
      strictly lower than before this task.
    - `grep -c 'context.WithTimeout' internal/ui/pipelines/pipelines.go` is at least 2
      and the same for `internal/ui/releases/releases.go`.
    - `gofmt -l internal/ui` prints nothing.
  </acceptance_criteria>
  <done>The two dual-message polling views distinguish rejected token, partial, stale, and genuinely empty — and the pipelines view no longer blames the user's configuration for a failure.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Carry the envelope on the workitems and branches views</name>
  <files>internal/ui/workitems/workitems.go, internal/ui/workitems/workitems_test.go, internal/ui/branches/branches.go, internal/ui/branches/branches_test.go</files>
  <read_first>
    - `internal/ui/prs/prs.go` as it stands after 02-01 — the reference implementation.
    - `internal/ui/pipelines/pipelines.go` as it stands after Task 1 of this plan.
    - `internal/ui/workitems/workitems.go` — `WorkItemsMsg` at :44-47, the fetch at
      :136, the reducer arm at :358, and the error render at :682.
    - `internal/ui/branches/branches.go` — `BranchesMsg` at :32-35, the fetch at :98,
      the reducer arm at :114-116, and the error render at :230.
    - `internal/ui/releases/releases_test.go:9-20` — the nil-client test-model helper
      and the regression-comment style.
    - `.planning/phases/02-honest-reads/02-CONTEXT.md` D-13, D-14, D-16.
  </read_first>
  <behavior>
    - The workitems reducer, given a message whose envelope carries an auth-kind
      failure, sets state that causes the view to render a blocking banner and
      suppress the table.
    - The workitems reducer, given a non-auth failure alongside rows, retains the rows
      and sets a persistent warning naming the failed projects and the failure kind.
    - The workitems view does not present zero rows as an absence of work when the
      envelope reports the result incomplete.
    - The branches reducer and view exhibit the same three behaviors.
    - A branches failure whose operation label identifies a repository renders that
      repository name in the warning line, so a user can tell which repository is broken.
  </behavior>
  <action>
    Apply the same treatment to the workitems and branches views. Both carry a single
    message type, so neither has the primary-versus-secondary question that Task 1 had.

    Per D-16, change both message types to carry the envelope. In both fetch commands,
    capture the fan-out budget into a local and replace `context.Background()` with a
    `context.WithTimeout`, deferring cancel inside the closure.

    Implement D-13 and D-14 in both reducers using the shared renderers, keeping the
    asymmetry intact: a blocking banner that replaces the table for an auth failure,
    and rows plus a persistent warning line for a non-auth partial.

    Gate each view's zero-row message on the envelope reporting the result complete.

    Neither of these is a polling view, so D-15's stale marker does not apply to them —
    D-15 names the three polling views specifically. Record that scope call rather than
    silently omitting it.

    Branches carries an extra detail from 02-02: its per-repository failures record a
    repository in the operation label. Make sure the warning line surfaces that label,
    not only the project, or a user with many repositories cannot act on it.

    Create `internal/ui/workitems/workitems_test.go` and
    `internal/ui/branches/branches_test.go` following the nil-client helper pattern.
    Note the workitems package has the largest constructor and the most model state, so
    build the helper carefully and keep the assertions to the reducer's outputs.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt &amp;&amp; go build ./... &amp;&amp; go vet ./internal/ui/... &amp;&amp; gofmt -l internal/ui &amp;&amp; go test ./internal/ui/workitems/ ./internal/ui/branches/ -count=1 -v</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/ui/workitems/ -count=1 -v` passes and includes a case driving
      the reducer with an auth-carrying envelope and a case with an incomplete zero-row
      envelope.
    - `go test ./internal/ui/branches/ -count=1 -v` passes and includes the same two
      cases, plus a case asserting a per-repository failure's operation label reaches
      the warning line.
    - `grep -c 'context.Background()' internal/ui/workitems/workitems.go` is strictly
      lower than before this task, and `grep -c 'context.WithTimeout' internal/ui/workitems/workitems.go` is at least 1.
    - `grep -c 'context.WithTimeout' internal/ui/branches/branches.go` is at least 1.
    - The write call sites in `internal/ui/workitems/workitems.go` at the four update
      methods are unchanged — this plan touches read fetches only, and writes are Phase 4.
    - `gofmt -l internal/ui` prints nothing.
  </acceptance_criteria>
  <done>Work items and branches show a rejected token as a rejected token, name the projects and repositories that failed, and stop presenting an unfetched view as an empty one.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Carry the envelope on the dashboard view and label the understated total</name>
  <files>internal/ui/dashboard/dashboard.go, internal/ui/dashboard/dashboard_test.go</files>
  <read_first>
    - `internal/ui/prs/prs.go` as it stands after 02-01.
    - `internal/ui/dashboard/dashboard.go` — `CountsMsg` at :17-20, the fetch at :97,
      the reducer arm at :128-136, and the error render at :234.
    - `internal/ado/dashboard.go` as it stands after 02-03, in particular how the
      aggregate counts value is carried inside the envelope.
    - `.planning/phases/02-honest-reads/02-03-SUMMARY.md` — the recorded decision on the
      degraded iteration lookup, which determines whether the dashboard will routinely
      carry a failure the user must be able to interpret.
    - `.planning/phases/02-honest-reads/02-CONTEXT.md` D-03, D-13, D-14, D-16, and
      §Specific Ideas — the reasoning that a labelled understatement beats both a
      silent understatement and a discarded truth.
  </read_first>
  <behavior>
    - The dashboard reducer, given a message whose envelope carries an auth-kind
      failure, sets state that causes the view to render a blocking banner and suppress
      the counts.
    - The dashboard reducer, given a non-auth failure alongside non-zero counts, renders
      the counts and a persistent warning line stating the totals are understated and
      naming which projects and sub-queries did not answer.
    - The dashboard does not render a zero count as an authoritative zero when the
      envelope reports the result incomplete.
    - A complete result renders the counts with no warning line.
  </behavior>
  <action>
    Apply the treatment to the dashboard view. This is the view where the honesty
    problem is sharpest: the payload is a small set of totals with no visible seam, so
    a partial sum looks exactly like a correct one.

    Per D-16, change `CountsMsg` to carry the envelope in place of its bare counts
    pointer. In the fetch command, capture the fan-out budget into a local and replace
    `context.Background()` with a `context.WithTimeout`, deferring cancel in the closure.

    In the reducer, implement D-14's blocking banner for an auth failure and D-13's
    persistent warning line for a non-auth partial, using the shared renderers.

    The warning line here must do one thing the other five do not: it must say that the
    totals shown are understated, not merely that something failed. D-03's entire
    justification is that a labelled understatement beats a silent one, and the label is
    the part that makes it true. Naming the failed projects and sub-queries alone leaves
    the user to infer the effect on the number.

    Depending on the decision recorded in 02-03, a degraded iteration lookup may produce
    a routine failure on healthy configurations. If so, make sure the warning wording
    distinguishes a filter degradation from a project that did not answer at all, or the
    warning becomes background noise the user learns to ignore.

    Create `internal/ui/dashboard/dashboard_test.go` following the nil-client helper
    pattern. Cover every bullet in the behavior block.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt &amp;&amp; go build ./... &amp;&amp; go vet ./... &amp;&amp; gofmt -l . &amp;&amp; go test ./... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/ui/dashboard/ -count=1 -v` passes and includes a case driving
      the reducer with an auth-carrying envelope asserting the banner state.
    - `go test ./internal/ui/dashboard/ -count=1 -v` includes a case with non-zero
      counts and a non-auth failure asserting both that the counts are rendered and that
      the warning states the totals are understated.
    - `go test ./internal/ui/dashboard/ -count=1 -v` includes a case with a complete
      envelope asserting no warning line is produced.
    - `grep -c 'context.WithTimeout' internal/ui/dashboard/dashboard.go` is at least 1.
    - `go build ./... &amp;&amp; go vet ./... &amp;&amp; gofmt -l . &amp;&amp; go test ./... -count=1` is fully green.
    - `ls internal/ui/dashboard/dashboard_test.go internal/ui/workitems/workitems_test.go internal/ui/prs/prs_test.go internal/ui/pipelines/pipelines_test.go internal/ui/branches/branches_test.go internal/ui/releases/releases_test.go` lists six files.
  </acceptance_criteria>
  <done>The dashboard total is shown with its understatement stated rather than implied, and all six view packages have reducer tests.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `ado.Failure` error text -> lipgloss render -> terminal | Server-controlled text, potentially an HTML sign-in fragment or containing control sequences, reaches a terminal that interprets escapes |
| aggregate counts -> user's judgement | A small set of totals with no visible seam; the warning line is the only signal they are understated |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-02 | Information Disclosure / Spoofing | the banner and warning-line renderers now consumed by five more views | medium | mitigate | `WrappedError.Error()` returns the server message verbatim, which for a rejected PAT can be an HTML sign-in fragment. All five views render through the shared 02-01 renderers, which put the classified `kind` first and pass the raw message through `internal/utils.Truncate`. No view may build its own untruncated display path — the acceptance criteria require the shared helpers. |
| T-02-01 | Information Disclosure | the branches warning line surfacing a repository-bearing operation label | medium | mitigate | The label is server-sourced repository text and is rendered through the same truncating helper. The `Failure` struct still carries no URL, header, or token, so there is nothing further to leak through this path. |
| T-02-03 | Denial of Service | the six fetch commands under D-08 | medium | **accept** | Each view refreshes on its own timer and, under a rejected token, still issues one round trip per configured project per refresh — the polling views every thirty seconds. D-08 rejected short-circuiting after the cost was stated. The `context.WithTimeout` added here bounds each fetch's duration but not the number of requests. PLAT-03 owns the fix. Surfaced, not absorbed. |
| T-02-04 | Denial of Service (self-inflicted) | the eight fetch call sites still on `context.Background()` before this plan | high | mitigate | Every read fetch command in the five remaining views gains a fan-out deadline derived from the configured budget, closing the D-09 half of TRUST-04 that 02-01 opened. Write call sites keep `context.Background()` and are explicitly Phase 4's scope. |
| T-02-07 | Repudiation / integrity of reported state | the dashboard totals under a partial result | medium | mitigate | A partial sum is visually indistinguishable from a correct one. Task 3 requires the warning to state the totals are understated rather than merely naming what failed, so the number's reliability is asserted in words rather than left to inference. |
| T-02-SC | Tampering | go module installs | n/a | mitigate | This plan adds zero dependencies; `go.mod` and `go.sum` must be unchanged. No package legitimacy checkpoint required. |
</threat_model>

<artifacts_this_phase_produces>
| Kind | Symbol / path |
|------|---------------|
| file | `internal/ui/dashboard/dashboard_test.go` |
| file | `internal/ui/workitems/workitems_test.go` |
| file | `internal/ui/pipelines/pipelines_test.go` |
| file | `internal/ui/branches/branches_test.go` |
| message shape | `dashboard.CountsMsg` now carries the 02-01 envelope in place of a bare counts pointer |
| message shape | `workitems.WorkItemsMsg` now carries the envelope |
| message shape | `pipelines.PipelinesMsg` and `pipelines.DefinitionsMsg` now carry the envelope |
| message shape | `releases.ReleasesMsg` and `releases.ApprovalsMsg` now carry the envelope |
| message shape | `branches.BranchesMsg` now carries the envelope |
| model fields | per-view state backing the auth banner, the persistent warning line, and the stale marker |
| style helpers | any extension to the 02-01 renderers in `internal/ui/styles/styles.go` needed by the five expansion views |

`prs.PRsMsg` and the base renderers are created by 02-01, not by this plan.
</artifacts_this_phase_produces>

<verification>
- `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` is green.
- All six view packages have a test file, up from one before this phase.
- Every read fetch command in the six views uses a derived deadline rather than an
  unbounded background context; every write call site is unchanged.
- Each of the six views has a test asserting the auth banner and a test asserting the
  completeness gate on its zero-row message.
- `git diff --stat go.mod go.sum` is empty.
</verification>

<success_criteria>
- A user with a rejected token sees that stated, in every view, rather than an empty table.
- A user with three of four projects answering sees the three, and sees which one did not.
- A user with a genuinely empty backlog still sees the ordinary no-work message.
- No view attributes a failure to the user's configuration.
</success_criteria>

<output>
Create `.planning/phases/02-honest-reads/02-05-SUMMARY.md` when done. It must record
the scope call that D-15's stale marker applies to the three polling views only, and
the wording chosen for the dashboard's understated-total warning.
</output>
