---
phase: 02-honest-reads
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/ado/result.go
  - internal/ado/result_test.go
  - internal/ado/fakes_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/ado/client.go
  - internal/ado/client_test.go
  - internal/ado/pullrequests.go
  - internal/ado/pullrequests_test.go
  - internal/ui/styles/styles.go
  - internal/ui/prs/prs.go
  - internal/ui/prs/prs_test.go
autonomous: false
requirements: [TRUST-01, TRUST-02, TRUST-03, TRUST-04]
user_setup:
  - service: azure-devops
    why: "Assumption A1 — the HTTP status code Azure DevOps actually returns for a rejected PAT was never observed, and TRUST-02's classifier is built on it"
    dashboard_config:
      - task: "Create or obtain a deliberately invalid / expired / revoked PAT for the configured org"
        location: "Azure DevOps -> User settings -> Personal access tokens"

estimate:
  tokens: 96000
  raw_tokens: 48000
  tasks: 4
  confidence: low

must_haves:
  truths:
    - "GetPullRequests returns items, a failure list, and a completeness flag; a project whose PR fetch failed appears in the failure list while the surviving project's PRs are still returned"
    - "Every failure entry names project and operation and never contains a request URL or an HTTP header"
    - "401 and 403 both classify to kind auth; 429 classifies to rate_limit; a deadline classifies to timeout; every other status classifies to other, and no status code maps to two kinds"
    - "A 401 carried in a WrappedError value classifies identically to a 401 carried in a *WrappedError pointer"
    - "A nil error produces no failure; an error carrying no StatusCode classifies to kind other, never auth"
    - "Complete is false whenever the failure list is non-empty, and true only when zero failures occurred across every project and every sub-request"
    - "A config file that omits request_timeout and fetch_timeout still yields non-zero durations after config.Load()"
    - "config.Validate() returns a ConfigError when fetch_timeout is less than or equal to poll_interval"
    - "connection.Timeout is assigned to cfg.RequestTimeout before the first azuredevops sub-client constructor runs in NewClient"
    - "The prs view renders a blocking banner instead of the table when the result carries an auth failure, and renders the rows plus a persistent warning line when the result is merely incomplete"
    - "The prs view does not present zero rows as an absence of work when the result is incomplete"
    - statement: "Failures are appended in configured-project iteration order, and failures of equal kind retain insertion order"
      verification: backstop
  prohibitions:
    - "Must NOT short-circuit or skip the remaining configured projects once an auth failure has been observed — D-08 rejected this explicitly after the cost was stated"
    - "Must NOT present a zero-row result as an absence of work when the result is flagged incomplete"
  artifacts:
    - internal/ado/result.go
    - internal/ado/result_test.go
    - internal/config/config_test.go
    - internal/ado/client_test.go
    - internal/ui/prs/prs_test.go
  key_links:
    - "config.Load() default literal -> cfg.RequestTimeout -> connection.Timeout -> every azuredevops sub-client's http.Client.Timeout"
    - "classifyFailure -> Failure.Kind -> PRsMsg envelope -> prs reducer -> auth banner vs warning line"
    - "Result.Complete -> the prs empty-state render branch"
---

<objective>
Prove the entire honest-read vertical on one path: pull requests. One shared result
envelope, one classifier that survives the SDK's two error shapes, both timeout
budgets wired end to end, and the prs view rendering the outcome truthfully.

Purpose: `GetPullRequests` is the simplest of the eight fan-outs (02-PATTERNS.md
names it the tracer). Proving the whole stack here — types, classifier, config,
connection wiring, message envelope, reducer, render — costs one commit and
de-risks the six expansion reads and five expansion views that follow. If the
envelope shape is wrong, this plan finds it before seven other files depend on it.

Output: `internal/ado/result.go` with `Result[T]`, `Failure`, `FailureKind` and
`classifyFailure`; two new config duration keys with defaults and validation;
`connection.Timeout` set at the only point where it is not a no-op; `GetPullRequests`
returning the envelope; and the prs view rendering auth, partial, and stale states.
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
@.planning/codebase/CONVENTIONS.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create the result envelope, failure record, and status classifier</name>
  <files>internal/ado/result.go, internal/ado/result_test.go, internal/ado/fakes_test.go</files>
  <read_first>
    - `internal/ado/interfaces.go` (all 47 lines) — the precedent for a small
      support-only file in this package and the header-comment style to match.
    - `internal/ado/dashboard.go:14-19` — the doc-comment + flat-struct style every
      `ado` type uses.
    - `internal/ado/releases.go:19` — the inline enumerated-value comment style to
      apply to the kind field.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pattern 1" and §"Pattern 2"
      (lines 147-260) — the verbatim SDK source showing `UnwrapError` returning a
      pointer at client.go:366 and :403 and a bare value at :389 and :411, plus the
      two-target extractor structure and the belt-and-braces timeout detection.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pitfall 1" and §"Pitfall 7"
      (lines 567-573, 609-613).
    - `internal/ado/fakes_test.go` (all of it) — the `t *testing.T` + one `XxxFunc`
      field per method + `Fatalf`-on-unstubbed fake pattern; the compile-time drift
      assertion block at the top must be preserved.
    - `internal/ado/branches_test.go:12-59` — the characterization-test shape and the
      `Method() field = %v, want %v` assertion-message convention.
  </read_first>
  <behavior>
    - A `nil` error yields no classification result / no failure.
    - An error that is a bare `azuredevops.WrappedError` **value** with StatusCode 401
      classifies to the auth kind.
    - An error that is an `*azuredevops.WrappedError` **pointer** with StatusCode 401
      classifies to the auth kind. Both forms must be separate rows in the table test —
      this is the single highest-value test in the phase.
    - StatusCode 403 classifies to the auth kind.
    - StatusCode 429 classifies to the rate-limit kind.
    - An error wrapping `context.DeadlineExceeded` classifies to the timeout kind.
    - An error satisfying `net.Error` with `Timeout() == true` classifies to the
      timeout kind even when it does not wrap `context.DeadlineExceeded`.
    - An error wrapping `context.Canceled` classifies to the other kind, not timeout.
    - A `WrappedError` with a nil StatusCode classifies to the other kind, not auth.
    - A wrapped-with-%w `*WrappedError` still reaches the auth kind through the
      wrapping layer.
    - A result constructed with zero failures reports complete; a result constructed
      with one or more failures reports incomplete.
  </behavior>
  <action>
    Create `internal/ado/result.go` as a new file. Do not append these types to
    `workitems.go` — it is 551 lines against a 300-line standard and is the exact
    violation this project is trying to stop widening.

    Open the file with a header comment in the style of `interfaces.go:12-16` that
    states, in its own words, why a generic shared envelope is justified here:
    D-01 fixes one contract shape across eight aggregate reads, which is well past
    the rule of three, and it reuses the same CONVENTIONS rule 5 carve-out that
    justified the Phase 1 interfaces. Cite D-01 and SEAM-01's precedent by name.

    Declare, per D-01, D-02, D-06 and D-07:

    - A `FailureKind` string type with four exported constants covering the auth,
      timeout, rate-limit and other classes. Their string values are the four names
      fixed by D-07. Annotate the legal value set inline in the field comment, in
      the `releases.go:19` style.
    - A `Failure` struct carrying exactly four fields: the project name, the
      operation name (a short label such as the SDK call being made), the
      classified kind, and the underlying error. **Security constraint, per threat
      T-02-01: this struct must not gain a field holding a request URL, a request
      header, an authorization value, or the PAT.** State that constraint as a
      comment on the struct so a later contributor does not add one.
    - A generic `Result[T any]` struct carrying the items slice, the failure slice,
      and a boolean completeness flag, per D-01 and TRUST-03.
    - A small constructor or finaliser that derives the completeness flag from the
      failure slice rather than leaving it to eight call sites to remember. Complete
      is true only when the failure slice is empty (TRUST-03).
    - An unexported `classifyFailure` (or equivalently named) function taking an
      error and returning a `FailureKind`. Implement it with the two-target
      `errors.As` structure from RESEARCH Pattern 1 — one target of the bare value
      type and one of the pointer type — because `WrappedError.Error()` has a value
      receiver and `errors.As` will not convert between the two forms. Then apply
      the RESEARCH Pattern 2 belt-and-braces timeout detection combining
      `errors.Is` against the deadline sentinel with a `net.Error` timeout check.
      Check the cancellation sentinel explicitly before the timeout check and let it
      fall to the other kind; leave a comment recording that Phase 3's CLI-04
      interrupt handling depends on that ordering.
    - A helper that appends a `Failure` given a project, an operation label and an
      error, so the seven expansion reads have one call site shape to copy.

    Do NOT classify by string-matching the error text. `WrappedError.Error()`
    returns the server-controlled, localisable message and can be an HTML fragment;
    D-06 exists to stop exactly that.

    Add no new module dependency — this is stdlib plus the already-vendored SDK
    (CONVENTIONS rule 6).

    Create `internal/ado/result_test.go` in `package ado` (Client's fields are
    unexported, so `package ado_test` cannot construct the seam). Write the
    classifier as a table-driven test with one row per bullet in the behavior block
    above. Follow the `branches_test.go` assertion-message convention.

    Extend `internal/ado/fakes_test.go` with small constructor helpers that build a
    typed SDK error in each of the two shapes given a status code, so the expansion
    plans can inject a real `WrappedError` rather than a bare `errors.New`. Keep the
    existing compile-time drift assertion block unchanged and do not add any method
    to the four interfaces.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./internal/ado/ && gofmt -l internal/ado && go test ./internal/ado/ -run 'TestClassifyFailure|TestResultCompleteness' -count=1 -v</automated>
  </verify>
  <acceptance_criteria>
    - `internal/ado/result.go` exists and `internal/ado/workitems.go` line count is unchanged.
    - `go test ./internal/ado/ -run TestClassifyFailure -count=1 -v` shows a subtest
      for a `WrappedError` value carrying 401 and a separate subtest for a
      `*WrappedError` pointer carrying 401, and both assert the auth kind.
    - `go test ./internal/ado/ -run TestClassifyFailure -count=1 -v` shows subtests
      asserting 403 to auth, 429 to rate-limit, a deadline error to timeout, a
      cancellation error to the other kind, and a nil-StatusCode error to the other kind.
    - `grep -n 'errors.As' internal/ado/result.go` returns at least two lines.
    - The `Failure` struct declares exactly four fields, and `go doc` / source
      inspection shows none of them is named for a URL, a header, a token, or a PAT.
    - `git diff --stat go.mod go.sum` is empty.
    - `gofmt -l internal/ado` prints nothing.
  </acceptance_criteria>
  <done>The shared envelope, the four failure kinds, and a classifier that handles both SDK error shapes exist and are covered by a table test whose rows include the value/pointer pair.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add the two timeout config keys and set connection.Timeout before any sub-client is built</name>
  <files>internal/config/config.go, internal/config/config_test.go, internal/ado/client.go, internal/ado/client_test.go</files>
  <read_first>
    - `internal/config/config.go` in full — the `Config` struct at :38-50, the
      `Load()` default literal at :80-83, `loadFromFile` at :107-119, `loadFromEnv`
      at :122-142, and `Validate()` with its `ConfigError` at :144-152.
    - `internal/ado/client.go:38-70` — `NewClient`, the `NewPatConnection` call at
      :39, and the six sub-client constructors at :43, :48, :53, :58, :63, :70.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pattern 3" (lines 262-309)
      — the SDK source showing `azuredevops.NewClient` copies `connection.Timeout`
      into `http.Client.Timeout` at construction and caches the result on the
      `Connection`.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Pitfall 2" and §"Pitfall 5"
      (lines 574-580, 595-601).
    - `.planning/phases/02-honest-reads/02-PATTERNS.md` §"internal/config/config.go"
      — the aligned `yaml:` tag style and the defaults-before-loadFromFile ordering.
    - `internal/ado/releases.go:117-119` — the take-the-address-of-a-local idiom for
      producing a pointer to a non-string value. `strPtr` at `client.go:234` is
      string-only and must not be widened.
  </read_first>
  <behavior>
    - `config.Load()` with no config file present yields a non-zero request timeout
      and a non-zero fetch timeout.
    - `config.Load()` with a config file that sets only `poll_interval` still yields
      non-zero values for both new keys.
    - A YAML file setting `request_timeout: 45s` parses into a 45-second duration
      without any string parsing in application code.
    - `Validate()` returns a `ConfigError` naming the fetch-timeout field when the
      fetch timeout is less than or equal to the poll interval.
    - `Validate()` returns nil when the fetch timeout exceeds the poll interval and
      the existing required fields are set.
    - The connection constructed for a config carrying a request timeout has a
      non-nil `Timeout` equal to that duration.
  </behavior>
  <action>
    In `internal/config/config.go`, add two `time.Duration` fields to the `Config`
    struct beside `PollInterval`, per D-10 and D-11: one bounding a single HTTP call
    and one bounding a whole fan-out. Their `yaml:` tags are `request_timeout` and
    `fetch_timeout`. Keep the tag column alignment `gofmt` produces.

    Add a default for **both** keys to the `&Config{...}` literal inside `Load()` at
    :81-83, beside the existing 30-second poll interval. Per RESEARCH Open Question 3
    use 30 seconds for the per-call budget and 2 minutes for the fan-out budget.
    Omitting either default is a one-line mistake with two opposite catastrophic
    outcomes — a zero duration means *no bound at all* for `http.Client`, and
    *instant expiry* for `context.WithTimeout` (Pitfall 5). Both defaults are
    therefore load-bearing, not cosmetic.

    Do **not** add these keys to `loadFromEnv`. That function handles exactly four
    names today and `PollInterval` is deliberately absent from it; match that
    precedent (D-11 specifies YAML only).

    Extend `Validate()` with a check that the fan-out budget strictly exceeds the
    poll interval, returning the existing `ConfigError` type with the fetch-timeout
    field name. Without this, a fan-out slower than the poll interval is cancelled by
    its own deadline exactly as the next tick fires, producing a view that fails
    permanently and looks like an outage.

    Create `internal/config/config_test.go` — this package has no test file today.
    `Load()` reads `$HOME/.config/adoBizInt/config.yaml` unconditionally with no
    injection seam, so isolate it with `t.Setenv("HOME", t.TempDir())` before each
    `Load()` call rather than adding a seam the package does not otherwise need
    (CONVENTIONS rule 5). Cover every bullet in the behavior block.

    In `internal/ado/client.go`, extract the connection construction into a small
    unexported helper that takes the config and returns the `*azuredevops.Connection`
    with its `Timeout` already set from the per-call budget. `Timeout` is a
    `*time.Duration`, so take the address of a local — do not widen `strPtr`.
    Call that helper as the first statement of `NewClient`, replacing the direct
    `NewPatConnection` call at :39.

    The ordering is the whole point: `azuredevops.NewClient` reads
    `connection.Timeout` once at construction, copies it into the HTTP client, and
    the `Connection` caches the constructed client. Assigning `Timeout` after any of
    the six constructors at :43, :48, :53, :58, :63 or :70 compiles, vets, passes
    every test, and bounds nothing.

    Create `internal/ado/client_test.go` in `package ado` asserting the helper's
    output directly. Test the helper, not `NewClient` — `NewClient` performs six
    network round trips and cannot run in a unit test.

    Record explicitly in the plan summary that `NewClient`'s own
    `context.Background()` at :41 is now bounded at the transport layer by
    `connection.Timeout` for the five error-returning constructors, and that
    `location.NewClient` at :70 plus `getConnectionData` at :77 remain on an
    unbounded context at startup — a deliberate, recorded scope call, not an
    oversight (RESEARCH Open Question 7).

    Also record explicitly that `DownloadAttachment`'s `http.DefaultClient` is
    **excluded** from TRUST-04 in this phase and deferred to Phase 5, where the host
    allow-list and path containment that belong with it live (HARD-01..HARD-03,
    RESEARCH Open Question 6). This is an exclusion by decision, not by omission.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./internal/config/ ./internal/ado/ && gofmt -l internal/config internal/ado && go test ./internal/config/ ./internal/ado/ -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/config/ -run TestLoadAppliesTimeoutDefaults -count=1 -v` passes
      and asserts both durations are non-zero with an absent config file.
    - `go test ./internal/config/ -run TestValidate -count=1 -v` includes a case where
      the fan-out budget equals the poll interval and asserts a `ConfigError` is returned.
    - `go test ./internal/ado/ -run TestNewConnection -count=1 -v` passes and asserts
      the returned connection's `Timeout` is non-nil and equal to the configured
      per-call budget.
    - `grep -n 'request_timeout\|fetch_timeout' internal/config/config.go` returns exactly
      two lines, both inside the `Config` struct declaration.
    - `grep -vn '^\s*//' internal/config/config.go | grep -c 'ADO_REQUEST_TIMEOUT\|ADO_FETCH_TIMEOUT'` returns `0`.
    - In `internal/ado/client.go`, the line assigning the connection timeout has a
      lower line number than every `NewClient(` call in the file.
    - `gofmt -l internal/config internal/ado` prints nothing.
  </acceptance_criteria>
  <done>Both budgets are configurable with safe defaults and a validated relationship to the poll interval, and every SDK sub-client inherits the per-call deadline because the timeout is set before any of them is constructed.</done>
</task>

<task type="tracer" tdd="true">
  <name>Task 3: End-to-end honest pull-request read — ado through message through reducer through render</name>
  <files>internal/ado/pullrequests.go, internal/ado/pullrequests_test.go, internal/ui/styles/styles.go, internal/ui/prs/prs.go, internal/ui/prs/prs_test.go</files>
  <read_first>
    - `internal/ado/pullrequests.go` in full — `GetPullRequests` at :34, the swallow
      at :52-55, the sole `return allPRs, nil` at :65, the three-group import block
      at :3-13, and the `log := logging.Logger()` first statement at :36.
    - `internal/ado/result.go` as written in Task 1.
    - `internal/ado/pullrequests_test.go:253-293` — the existing characterization test
      this task inverts.
    - `internal/ado/branches_test.go:12-59` — the two-project failing/surviving
      const pair and the poisoned-value technique that makes a leak detectable.
    - `internal/ui/prs/prs.go` in full — the message type at :32-36, the fetch command
      at :126-133 with its capture-into-locals closure discipline, the reducer arm at
      :154-162, the error banner at :379-383, and the empty-state branch at :387-391.
    - `internal/ui/styles/styles.go:5-69` — the single package-level `var (...)` block
      and the existing `ErrorStyle`, `WarningStyle`, `MutedStyle` vocabulary.
    - `internal/ui/releases/releases_test.go:9-20` — the `New(nil, ...)` nil-client
      test-model helper and the `// Regression:` comment convention.
    - `internal/utils/text.go:6` — the existing `Truncate` helper.
    - `.planning/phases/02-honest-reads/02-PATTERNS.md` §"internal/ui/prs/prs.go"
      and §"Planner Notes".
  </read_first>
  <behavior>
    - `GetPullRequests` against two configured projects where one project's fetch
      returns an error yields the surviving project's PRs in the items slice, exactly
      one failure entry naming the failing project, and an incomplete result.
    - That failure entry names the operation it was performing.
    - `GetPullRequests` where the failing project returns a 401 in a `WrappedError`
      value yields a failure of the auth kind, and the items slice may be empty while
      the failure slice is not — empty is distinguishable from failed.
    - `GetPullRequests` where every project answers yields a complete result with an
      empty failure slice.
    - `GetPullRequests` given a context that has already exceeded its deadline yields
      a timeout-kind failure for each project it did not reach and an incomplete result.
    - The prs reducer, given a message whose envelope carries an auth failure, sets
      state that causes the view to render a blocking banner and suppress the table.
    - The prs reducer, given a message whose envelope carries a non-auth failure and
      a non-empty items slice, retains the rows and sets a persistent warning naming
      the failed project and the failure kind.
    - The prs reducer, given a transport-level error after a prior successful load,
      leaves the previously loaded rows in place and marks them stale.
    - The prs view, given a complete envelope with zero items, renders the
      no-work-found message; given an incomplete envelope with zero items, it does not.
  </behavior>
  <action>
    This is the tracer slice. Wire ONE aggregate read through every layer this phase
    touches, end to end, at production quality. Do not touch the other seven reads or
    the other five views — those are the expansion plans.

    **ado layer.** Change `GetPullRequests` to return `Result[PullRequest]` instead of
    `[]PullRequest`. Give the existing `var allPRs []PullRequest` accumulator a
    failure-slice sibling. At the error path at **pullrequests.go:52-55** specifically
    — not at any other `continue` in the package — record a failure carrying the
    project, an operation label for the PR fetch, and the classified kind from Task 1's
    classifier, and then keep the `continue`. The `continue` stays: D-08 rejected
    short-circuiting explicitly after the cost was stated, so every configured project
    is still called on every refresh even once an auth failure is known.

    Leave the existing `log.Error` at :53 exactly where it is — the failure entry
    carries the same project/error pair already being logged rather than introducing a
    second vocabulary. Extend the terminal `log.Info` at :64 with a failure count
    rather than adding a second log statement.

    Add the deadline behavior required by D-12: before issuing each project's call,
    check whether the caller's context is already done; if it is, record a
    timeout-kind failure for that project and every project after it rather than
    issuing a doomed call. Construct the result through Task 1's finaliser so
    completeness is derived, never hand-set.

    Do NOT create a `context.WithTimeout` inside this method. The caller owns the
    budget; a method-internal deadline would be invisible to Phase 3's `--timeout`.

    Rewrite the characterization test at `pullrequests_test.go:253-293` in place,
    asserting the new propagating behavior. Keep its existing name for now — the
    anchored Phase 1 gate that counts those names is the phase's completion signal and
    the rename happens in one sweep at the end of the phase.
    <!-- planner-discipline-allow: SwallowsProjectFailure -->
    Add the further cases from the behavior block as new tests, using the Task 1 fake
    helpers to inject a typed 401 rather than a bare `errors.New`.

    **styles layer.** Add to the existing `var (...)` block in
    `internal/ui/styles/styles.go` whatever small set of rendering helpers the failure
    display needs — at minimum a blocking auth banner renderer, a partial-result
    warning-line renderer, and a stale marker. Six views need identical treatment, so
    extraction here clears the rule of three and is compliance rather than speculative
    generality; state that in a comment. Reuse the existing `ErrorStyle`,
    `WarningStyle` and `MutedStyle` — a stale marker needs no new colour.

    **Security constraint on the renderers, per threat T-02-02:** the underlying error
    text is server-controlled and for a rejected PAT can be an HTML sign-in fragment.
    Render the classified kind prominently and the raw message secondarily, and pass
    the raw message through the existing `internal/utils.Truncate` before it reaches
    the terminal. Do not render an unbounded server string.

    **ui/prs layer.** Per D-16, change `PRsMsg` to carry the envelope in place of its
    bare items slice, keeping its existing `Err error` field for genuine transport
    failure. In `fetchPRs` at :126-133, capture the fan-out budget into a local
    alongside the existing `projects` and `client` locals — never capture the receiver
    inside the closure — and replace `context.Background()` with a
    `context.WithTimeout` derived from it, deferring its cancel inside the closure.
    Expose the configured fan-out budget from `ado.Client` via a small accessor rather
    than threading a new parameter through six view constructors and `app.go`; record
    that as a planner-discretion call in the summary.

    In the reducer arm at :154-162, implement D-13, D-14 and D-15 against the
    envelope. Note that D-15's "keep the last good rows" is **already the current
    behavior** — `m.items` is simply untouched on the error branch — so what this task
    adds is the stale *marker*, not the retention. Size the change accordingly.
    Per D-14, an auth-kind failure is not partial data: set state that replaces the
    table with a blocking banner. Per D-13, a non-auth failure keeps the rows and adds
    a persistent warning line naming the failed projects and why — the status line is
    already used for action feedback and is too easy to miss across a 30-second
    refresh, so this must not go there.

    In `View()`, gate the auth banner so it replaces the block at :387-391 rather than
    sitting above it. Then change the zero-items branch at :389: the message it renders
    today asserts an absence of work, which is precisely the empty-versus-failed
    confusion this phase exists to remove. Render it only when the envelope reports the
    result complete; otherwise render the failure state.

    Create `internal/ui/prs/prs_test.go` following the `releases_test.go` nil-client
    helper (`prs.New` takes five arguments). Drive `m.Update(...)` with constructed
    messages — no existing test drives a Bubble Tea reducer, so this is new ground —
    and assert on model fields plus, where unavoidable, on substrings of `View()`.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && gofmt -l . && go test ./internal/ado/ ./internal/ui/prs/ -count=1 -v -run 'PullRequest|PRs'</automated>
  </verify>
  <acceptance_criteria>
    - `go build ./...` succeeds with `GetPullRequests` returning the envelope, which
      means every consumer was updated to acknowledge failure.
    - `go test ./internal/ado/ -run 'TestGetPullRequests' -count=1 -v` includes a case
      asserting one surviving project's PRs are returned alongside exactly one failure
      entry, and a case asserting an injected 401 produces an auth-kind failure with a
      non-empty failure slice.
    - `go test ./internal/ui/prs/ -count=1 -v` passes and includes a case driving the
      reducer with an auth-carrying envelope and a case driving it with an incomplete
      zero-item envelope.
    - `grep -c 'context.Background()' internal/ui/prs/prs.go` is strictly lower than it
      was before this task, and `grep -n 'context.WithTimeout' internal/ui/prs/prs.go`
      returns at least one line inside `fetchPRs`.
    - `grep -n 'utils.Truncate' internal/ui/styles/styles.go` returns at least one line.
    - `grep -c 'continue' internal/ado/pullrequests.go` is unchanged from before this
      task — the loop still visits every configured project.
    - `git diff --stat internal/ado/workitems.go internal/ado/pipelines.go internal/ado/releases.go internal/ado/branches.go internal/ado/dashboard.go` is empty.
    - `gofmt -l .` prints nothing and `go vet ./...` is clean.
  </acceptance_criteria>
  <done>A caller of `GetPullRequests`, and a user looking at the prs tab, can both tell "one of two projects answered" from "two projects, no open PRs" — and a rejected token shows as a rejected token rather than as an empty table.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking-human">
  <what-built>
    The failure classifier from Task 1 maps HTTP 401 and 403 to the auth kind. That
    mapping is built on assumption A1 in `02-RESEARCH.md`, which is recorded at LOW
    confidence: `NewPatConnection` sets `SuppressFedAuthRedirect: true`, which is the
    mechanism that should force a 401 rather than a sign-in redirect, but the status
    code Azure DevOps actually returns for an expired, revoked, or
    insufficient-scope PAT on these endpoints was never observed.

    If the real response is a 203 or a 302 rather than a 401, the auth banner never
    fires, TRUST-02 fails in production, and every automated test in this plan still
    passes. This cannot be faked with a stub — the whole point is observing what the
    server actually returns.
  </what-built>
  <how-to-verify>
    1. Take a copy of your working config and replace the PAT with a deliberately
       invalid value, or generate a PAT and revoke it in
       Azure DevOps -> User settings -> Personal access tokens.
    2. Run the built binary against that config and open the Pull Requests tab.
    3. Confirm the view shows a blocking banner stating the token was rejected, and
       does **not** show an empty table or a no-PRs-found message.
    4. In the log file, find the failure recorded for the PR fetch and read the
       underlying error. Record the HTTP status code it carries.
    5. If the code is 401 or 403, the classifier is correct as written.
       If it is anything else (203, 302, or a redirect body), record the observed
       code and body shape, and add it to the classifier's auth branch and to the
       `result_test.go` table before this plan is considered done.
    6. Confirm the banner does not display an unbounded HTML fragment — the message
       must be truncated and the classified kind must be the prominent element.
  </how-to-verify>
  <resume-signal>Reply with the observed status code (e.g. "401") and either "approved" if the classifier already handles it, or a description of the amendment needed.</resume-signal>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| ado client -> Azure DevOps REST API | The PAT crosses here on every call; responses are server-controlled and untrusted |
| ado `Failure` -> TUI render | Server-controlled error text crosses into the terminal, where control sequences and markup can spoof UI |
| `~/.config/adoBizInt/config.yaml` -> `config.Load()` | User-controlled durations become HTTP and context deadlines |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-01 | Information Disclosure | `ado.Failure` struct (`internal/ado/result.go`) | high | mitigate | `Failure` carries exactly project, operation, kind, and the underlying error. No field holds a request URL, header, authorization value, or PAT. Constraint is stated as a source comment on the struct and asserted in Task 1 acceptance criteria. |
| T-02-02 | Information Disclosure / Spoofing | failure renderers in `internal/ui/styles/styles.go`; prs banner | medium | mitigate | Raw `WrappedError.Error()` text is server-controlled and can be an HTML sign-in fragment. Render the classified `kind` prominently; pass the raw message through the existing `internal/utils.Truncate` before it reaches the terminal. |
| T-02-03 | Denial of Service | the project fan-out loop under D-08 | medium | **accept** | D-08 rejected short-circuiting after the cost was stated: on a four-project config with a rejected token, each 30-second refresh spends four failing round trips instead of one, against an SDK with no retry, backoff, or 429 handling (`azuredevops/client.go:84` carries a literal to-do for retry logic). `fetch_timeout` (D-09) is the only mitigation this phase adds and it bounds duration, not request count. PLAT-03 owns the real fix. Recorded here rather than absorbed silently. |
| T-02-04 | Denial of Service (self-inflicted) | `internal/ado/client.go` `NewClient`; the eight fetch call sites | high | mitigate | Task 2 sets `connection.Timeout` before any sub-client is constructed, bounding every transport call; Task 3 adds a `context.WithTimeout` fan-out deadline at the prs call site. Residual: `location.NewClient` and `getConnectionData` at startup remain unbounded by context — recorded as a scope decision. |
| T-02-SC | Tampering | npm/pip/cargo/go module installs | n/a | mitigate | This plan adds **zero** new module dependencies (`go.mod` and `go.sum` diff must be empty — see Task 1 acceptance criteria). `02-RESEARCH.md` §"Don't Hand-Roll" records that the entire phase is stdlib plus SDK fields already vendored, and that any new dependency is a red flag under CONVENTIONS rule 6. No package legitimacy checkpoint is required because no package is installed. |

**Prohibition breadcrumb (§B stage 2):** the candidate "must NOT include the PAT,
authorization header, or request URL in any `Failure` or rendered banner" was dropped
from `must_haves.prohibitions` as canon security and is carried instead by T-02-01 and
T-02-02 above.
</threat_model>

<artifacts_this_phase_produces>
Symbols created by **this plan** (not pre-existing; excluded from drift verification):

| Kind | Symbol / path |
|------|---------------|
| file | `internal/ado/result.go` |
| file | `internal/ado/result_test.go` |
| file | `internal/config/config_test.go` |
| file | `internal/ado/client_test.go` |
| file | `internal/ui/prs/prs_test.go` |
| type | `ado.Result[T]` (generic envelope: items, failures, completeness flag) |
| type | `ado.Failure` (project, operation, kind, error) |
| type | `ado.FailureKind` (string) |
| consts | the four `FailureKind` values fixed by D-07 (auth, timeout, rate_limit, other) |
| func | `ado.classifyFailure` (unexported; error -> FailureKind) |
| func | the result finaliser / constructor deriving completeness from the failure slice |
| func | the failure-append helper (project, operation, error) |
| func | the unexported connection constructor in `internal/ado/client.go` that sets `Timeout` |
| method | the `ado.Client` accessor exposing the configured fan-out budget |
| config key | `request_timeout` (`config.Config`, `time.Duration`, default 30s) |
| config key | `fetch_timeout` (`config.Config`, `time.Duration`, default 2m) |
| struct field | the two new `time.Duration` fields on `config.Config` |
| style helpers | the auth-banner, partial-warning, and stale-marker renderers in `internal/ui/styles/styles.go` |
| message shape | `prs.PRsMsg` now carries the envelope in place of a bare items slice |
| test helpers | typed-SDK-error constructors added to `internal/ado/fakes_test.go` |
</artifacts_this_phase_produces>

<verification>
- `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` is green.
- `go test ./internal/ado/ -run TestClassifyFailure -count=1 -v` shows both the value
  and the pointer `WrappedError` rows passing.
- `go test ./internal/config/ -count=1` passes in a package that had no test file before.
- The connection timeout assignment in `internal/ado/client.go` precedes every
  sub-client constructor in that file.
- `git diff --stat go.mod go.sum` is empty.
- The Task 4 human checkpoint is answered with an observed status code.
</verification>

<success_criteria>
- One shared result envelope exists and one aggregate read returns it end to end.
- The classifier handles both SDK error shapes and is proven to by a table test.
- Both timeout budgets are configurable, defaulted, validated, and actually wired.
- The prs view distinguishes rejected-token, partial, stale, and genuinely-empty.
- No new module dependency was added.
</success_criteria>

<output>
Create `.planning/phases/02-honest-reads/02-01-SUMMARY.md` when done. It must record:
the observed status code from Task 4; the planner-discretion call on exposing the
fan-out budget via a `Client` accessor; the recorded exclusion of `DownloadAttachment`
from TRUST-04; and the recorded residual that `location.NewClient` and
`getConnectionData` remain on an unbounded startup context.
</output>
