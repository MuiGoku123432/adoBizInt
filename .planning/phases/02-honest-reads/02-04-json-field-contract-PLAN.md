---
phase: 02-honest-reads
plan: 04
type: execute
wave: 4
depends_on: [02-01, 02-02, 02-03]
files_modified:
  - internal/ado/result.go
  - internal/ado/client.go
  - internal/ado/workitems.go
  - internal/ado/pullrequests.go
  - internal/ado/pipelines.go
  - internal/ado/releases.go
  - internal/ado/branches.go
  - internal/ado/dashboard.go
  - internal/ado/json_test.go
  - internal/ado/workitems_test.go
  - internal/ado/pullrequests_test.go
  - internal/ado/pipelines_test.go
  - internal/ado/releases_test.go
  - internal/ado/branches_test.go
  - internal/ado/dashboard_test.go
autonomous: false
requirements: [OUT-02]

estimate:
  tokens: 64000
  raw_tokens: 32000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Eleven exported ado structs carry explicit json tags; ado.Client is excluded because all its fields are unexported and it holds live SDK clients"
    - "Every json field name is lower snake_case, so an initialism-bearing field such as URL renders as a single lower-case word rather than as a letter-split name"
    - "A zero-valued instance of each tagged struct marshals every tagged field, so a consumer sees a stable key set regardless of data"
    - "The embedded WorkItem inside WorkItemDetail inlines its fields into the same JSON object rather than nesting under a key"
    - "The marshalled shape of the pipeline run duration field is a recorded, deliberately chosen contract rather than a Go default"
    - "The result envelope and the failure record carry json tags in the same snake_case convention, so Phase 3 serialises the whole read response with one contract"
    - "No aggregate read test in internal/ado still carries a name asserting the pre-inversion swallowing behavior"
    - statement: "Field ordering in the marshalled object follows struct declaration order, and the embedded struct's fields appear at the position of the embedding"
      verification: backstop
  prohibitions:
    - "Must NOT add a field to `Failure` without updating the exact-key-set test in `internal/ado/json_test.go` in the same change — from Phase 3 onward the tagged record is a machine-readable public contract, and D-04 makes these field names a one-way door"
    - "Must NOT run a repo-wide `go build ./...` / `go test ./...` gate inside this plan's task verifies — plan 02-05 is rewriting `internal/ui/*` in the same wave. Scope every command to `./internal/ado/`; the repo-wide gate is the phase-close step owned by 02-05"
  artifacts:
    - internal/ado/json_test.go
  key_links:
    - "the json tag names on the eleven structs -> Phase 3's --json output contract -> every agent and script parsing it"
    - "the Phase 1 anchored test-name gate -> zero matches -> phase completion signal"
---

<objective>
Give the exported `ado` types stable, lower-cased JSON field names, and close the
phase by retiring the eight characterization test names Phase 1 created to be
retired.

Purpose: OUT-02 is additive and non-breaking today, and becomes a locked public
contract the moment Phase 3 ships `--json`. D-04 fixes the convention as
snake_case. Getting the names wrong now is cheap; getting them wrong after Phase 3
breaks every agent and script parsing the output.

Output: eleven tagged structs, a field-name assertion test covering all of them,
one recorded decision on the duration field's marshalled shape, and a clean run of
the Phase 1 anchored gate.
</objective>

<wave_scoping>
This plan runs in wave 4 **alongside plan 02-05**, which is concurrently rewriting
every package under `internal/ui/`. Go compiles per package, so a repo-wide
`go build ./...` or `go test ./...` here would compile 02-05's half-written view
packages and fail spuriously — and 02-05's repo-wide gates would compile the
`internal/ado` structs this plan is mid-retag on.

Therefore **every verify command and every acceptance criterion in this plan is scoped
to `./internal/ado/`**. This plan owns exactly that package (plus its test files) and
touches no file outside it.

The single repo-wide gate
`go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1`
is the **phase-close step**, and it lives in plan 02-05's final task, which runs only
after this plan's summary exists. Do not add it back here.
</wave_scoping>

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

<task type="checkpoint:decision" gate="blocking">
  <decision>
    What shape does the pipeline run's duration field take in the JSON output, and
    does any zero-valued timestamp get suppressed?
  </decision>
  <context>
    `PipelineRun.Duration` is declared as a `time.Duration` at
    `internal/ado/pipelines.go:31`. `encoding/json` marshals a `time.Duration` as an
    integer nanosecond count — a one-hour run emits `3600000000000`, not `"1h0m0s"`.

    This is a **one-way door**. D-04 makes these field names and shapes the CLI field
    contract that Phase 3 serialises. Once shipped, every agent and script parsing
    that JSON depends on both the key and its type. Changing an integer to a string
    later is a breaking change for every consumer, and unlike a rename it will not
    even fail loudly — a consumer doing arithmetic on the value silently gets
    nonsense.

    The related question is timestamps. Every `time.Time` field here is a value, not
    a pointer, so a zero timestamp marshals as a sentinel year-one RFC 3339 date
    rather than being omitted. `,omitempty` does not suppress it — a struct is never
    "empty" to `encoding/json` — so omitting a zero timestamp would require a pointer
    field or a custom marshaller, which is a larger change than tagging.

    RESEARCH assumption A3 records that `,omitempty` behavior as standard library
    behavior that was not re-verified this session, and notes it is cheap to confirm
    with a three-line test. That confirmation belongs in this plan's test file
    regardless of which option is chosen.
  </context>
  <options>
    <option id="nanoseconds-and-sentinel-dates">
      <name>Emit duration as integer nanoseconds; accept the sentinel zero date</name>
      <pros>
        Zero custom marshalling code. Matches the Go type exactly, so no information
        is lost and no parsing ambiguity exists. Numerically comparable and sortable
        by a consumer without parsing. This is the RESEARCH recommendation, and it
        keeps this phase's diff to tags only.
      </pros>
      <cons>
        A human reading the raw JSON sees a large opaque integer. A consumer must know
        the unit is nanoseconds, since the key alone will not say so. The sentinel
        year-one date appears for every run that has not finished.
      </cons>
    </option>
    <option id="seconds-integer">
      <name>Emit duration as an integer count of seconds</name>
      <pros>
        Far more readable at a glance and a more conventional unit for a CI run length.
        Still numeric, so still sortable and comparable without parsing.
      </pros>
      <cons>
        Requires a custom marshaller or a derived field, which is more code than
        tagging and puts a conversion in the serialisation path. Loses sub-second
        precision, which is unlikely to matter for a pipeline run but is a real loss.
      </cons>
    </option>
    <option id="duration-string">
      <name>Emit duration as a Go duration string such as "1h0m0s"</name>
      <pros>
        Self-describing and immediately readable by a human. Round-trips through
        `time.ParseDuration` losslessly.
      </pros>
      <cons>
        Requires a custom marshaller. A consumer must parse it before doing any
        arithmetic or sorting, and Go's duration string format is not a standard other
        languages parse natively. Highest friction for the agent consumers this whole
        project exists to serve.
      </cons>
    </option>
  </options>
  <resume-signal>Select: nanoseconds-and-sentinel-dates, seconds-integer, or duration-string. Also state whether any field should become a pointer to allow omission.</resume-signal>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add json tags to the eleven serializable structs and assert every field name</name>
  <files>internal/ado/result.go, internal/ado/client.go, internal/ado/workitems.go, internal/ado/pullrequests.go, internal/ado/pipelines.go, internal/ado/releases.go, internal/ado/branches.go, internal/ado/dashboard.go, internal/ado/json_test.go</files>
  <read_first>
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"The twelve exported `ado`
      types (OUT-02)" (lines 400-441) — the full type list, the per-type field counts,
      the snake_case notes for each, the explicit exclusion of `Client`, and the two
      fields flagged for a deliberate call.
    - `.planning/phases/02-honest-reads/02-CONTEXT.md` D-04.
    - Each of the eleven struct declarations in place: `branches.go:15`,
      `dashboard.go:15`, `pipelines.go:16` and `:36`, `pullrequests.go:16`,
      `releases.go:13`, `:27` and `:40`, `workitems.go:21`, `:34` and `:49`.
    - `internal/ado/client.go:20-36` — the `Client` struct, to confirm every field is
      unexported before excluding it.
    - `internal/config/config.go:38-50` — the aligned-tag formatting style the
      codebase already uses.
    - `internal/ado/result.go` from 02-01 — the envelope and failure record, which
      also need tags.
    - The decision recorded in Task 1 of this plan.
  </read_first>
  <behavior>
    - Marshalling a zero-valued instance of each of the eleven structs produces an
      object whose key set exactly matches the expected snake_case name list for that
      struct, with no extra keys and no missing keys.
    - Marshalling a `WorkItemDetail` produces a single flat object containing both its
      own fields and the embedded work item's fields, not a nested object.
    - The attachment's URL field marshals to a single lower-case word, not to a
      letter-split name; identifier-suffixed fields likewise end in a two-letter key.
    - The pipeline run's duration field marshals to the shape chosen in Task 1.
    - A zero-valued time field marshals to the sentinel year-one RFC 3339 date,
      confirming assumption A3, and the test records that fact explicitly.
    - The result envelope and the failure record each marshal to snake_case keys.
  </behavior>
  <action>
    Add explicit `json:` tags to exactly eleven exported structs in `internal/ado`,
    using the per-type field name lists in RESEARCH §"The twelve exported `ado` types".
    Follow D-04: lower snake_case throughout.

    **Exclude `ado.Client` deliberately.** It is the twelfth exported struct and it is
    not serializable — every field is unexported and it holds live SDK clients and a
    live connection. Eleven types get tags, not twelve. State the exclusion in a
    comment on the `Client` declaration so a future contributor does not read the
    omission as an oversight.

    Handle initialisms explicitly rather than by mechanical word-splitting: an
    identifier-suffixed field becomes a plain two-letter suffix key and the attachment
    address field becomes a single lower-case word. Mechanical camel-to-snake
    conversion produces letter-split garbage for both, and both appear in this type set.

    `WorkItemDetail` embeds `WorkItem`. Leaving the embedded field untagged inlines
    the work item's fields into the same JSON object, which is almost certainly the
    shape a consumer wants — but it is a decision, not a default, so state it
    explicitly in a comment on the embedding and pin it with a test.

    Apply the Task 1 decision to the pipeline run's duration field and to any field
    the decision converts to a pointer.

    Also add tags to the result envelope and the failure record in
    `internal/ado/result.go`, in the same convention. D-01's entire justification is
    that Phase 3 writes its JSON handling once; leaving the envelope untagged would
    give the wrapper Go-cased keys around snake_case payloads, which is worse than
    either convention consistently applied.

    Create `internal/ado/json_test.go` in `package ado`. For each of the eleven
    structs plus the envelope and failure record, marshal a zero value, unmarshal into
    a generic map, and assert the exact key set. Asserting the key set rather than
    individual keys is what catches a *missing* tag, which is the failure mode a
    per-field assertion misses. Include the assumption A3 confirmation as its own
    named test so it is a recorded finding rather than an incidental pass.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt &amp;&amp; go vet ./internal/ado/ &amp;&amp; gofmt -l internal/ado &amp;&amp; go test ./internal/ado/ -run 'TestJSON' -count=1 -v</automated>
  </verify>
  <acceptance_criteria>
    - `grep -c 'json:"' internal/ado/branches.go internal/ado/dashboard.go internal/ado/pipelines.go internal/ado/pullrequests.go internal/ado/releases.go internal/ado/workitems.go` shows a non-zero count in each of the six files.
    - `go test ./internal/ado/ -run 'TestJSONFieldNames' -count=1 -v` passes with one
      subtest per struct, each asserting an exact key set.
    - `go test ./internal/ado/ -count=1 -v` includes a passing subtest asserting a
      `WorkItemDetail` marshals to a flat object containing the embedded work item's keys.
    - `grep -c 'json:"' internal/ado/client.go` returns `0`, and the `Client`
      declaration carries a comment explaining the exclusion.
    - `go test ./internal/ado/ -count=1 -v` includes a passing subtest named for the
      zero-timestamp behavior.
    - `grep -c 'json:"' internal/ado/result.go` is non-zero.
    - `gofmt -l internal/ado` prints nothing.
  </acceptance_criteria>
  <done>Every serializable ado type and the shared envelope have stable snake_case JSON keys, pinned by an exact-key-set test, with the one non-serializable type excluded on the record.</done>
</task>

<task type="auto">
  <name>Task 3: Retire the eight characterization test names and run the phase gate</name>
  <files>internal/ado/workitems_test.go, internal/ado/pullrequests_test.go, internal/ado/pipelines_test.go, internal/ado/releases_test.go, internal/ado/branches_test.go, internal/ado/dashboard_test.go</files>
  <read_first>
    - All six `internal/ado/*_test.go` files as they stand after 02-02 and 02-03.
    - `.planning/phases/02-honest-reads/02-RESEARCH.md` §"Open Questions" item 4
      (lines 653-654) — the recommendation to keep the original names until one final
      sweep, so the anchored Phase 1 gate acts as a countdown.
    - `.planning/phases/02-honest-reads/02-VALIDATION.md` §"Sampling Rate" — the phase
      gate command and its expected zero result.
    - `.planning/phases/01-test-seam-for-internal-ado/01-CONTEXT.md` D-14 through D-16
      — why these eight tests exist and that Phase 2 was always meant to retire them.
    - `internal/ui/releases/releases_test.go:18-20` — the regression-comment style that
      states the old broken behavior.
  </read_first>
  <action>
    Phase 1 created eight characterization tests whose names assert the old swallowing
    behavior, and created an anchored grep gate specifically so all eight could be
    found together. Plans 02-01 through 02-03 rewrote their bodies to assert the new
    propagating behavior but deliberately left the names alone, so the gate's match
    count served as a countdown through the phase.

    Rename all eight in one sweep so the names describe what the tests now assert.
    <!-- planner-discipline-allow: SwallowsProjectFailure -->
    Replace the `SwallowsProjectFailure` suffix with `ReportsProjectFailure` on each,
    keeping each test's `TestGet` prefix and read name intact so the validation map's
    `TestGet.*ReportsProjectFailure` run pattern matches all of them.

    Update the doc comment on each test at the same time. Each currently says it pins
    pre-TRUST-01 behavior and that Phase 2 will invert it; that sentence is now false.
    Rewrite each in the regression-comment style used in the releases view test:
    state that the project failure used to be dropped so a caller could not tell a
    failed read from an empty one, and state what is asserted instead now.

    Change nothing else. This task is a rename and a comment rewrite. If a test body
    needs to change to make it pass, that is a defect from an earlier plan and belongs
    in that plan, not here.

    Then run the anchored gate from `02-VALIDATION.md`, scoped to `./internal/ado/`,
    and confirm the countdown has reached zero. Do **not** run the repo-wide suite
    here — see `<wave_scoping>`; that is 02-05's phase-close step.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt &amp;&amp; go vet ./internal/ado/ &amp;&amp; gofmt -l internal/ado &amp;&amp; go test ./internal/ado/ -count=1 &amp;&amp; go test ./internal/ado/ -v -run 'TestGet.*ReportsProjectFailure' -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/ado/ -v -run TestGet 2>&amp;1 | grep -cE '^=== RUN   TestGet[A-Za-z]*SwallowsProjectFailure$'` returns `0`.
    - `go test ./internal/ado/ -v -run 'TestGet.*ReportsProjectFailure' -count=1 2>&amp;1 | grep -cE '^=== RUN   TestGet[A-Za-z]*ReportsProjectFailure$'` returns `8`.
      The `[A-Za-z]*` + `$` anchoring is load-bearing: plans 02-01 through 02-03 add
      subtests under these exact eight parents, and an unanchored `^=== RUN   TestGet`
      count would include every `.../subtest_name` line and over-count.
    - `git diff --stat internal/ado/workitems.go internal/ado/pullrequests.go internal/ado/pipelines.go internal/ado/releases.go internal/ado/branches.go internal/ado/dashboard.go` is empty for this task — no production file changed.
    - Every renamed test carries a rewritten doc comment describing the old dropped-failure
      behavior rather than describing itself as pinning pre-TRUST-01 behavior.
    - `go vet ./internal/ado/ &amp;&amp; gofmt -l internal/ado &amp;&amp; go test ./internal/ado/ -count=1` is green.
      (The repo-wide gate is 02-05's phase-close step, not this task's.)
  </acceptance_criteria>
  <done>The Phase 1 countdown gate reads zero, eight tests are named for what they now assert, and the full suite is green.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `ado` structs -> Phase 3 `--json` on stdout | Field names and value types become a public contract consumed by agents and scripts |
| Azure DevOps response values -> marshalled JSON | Server-controlled strings become field values in machine-parsed output |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-01 | Information Disclosure | the tagged `Failure` record in `internal/ado/result.go` | high | mitigate | Tagging the failure record makes its fields machine-readable, which raises the cost of the struct ever gaining a URL, header, or token field. Task 2 tags exactly the four fields 02-01 declared and adds none; the exact-key-set test in `json_test.go` fails loudly if a fifth field is ever added without review. |
| T-02-02 | Information Disclosure | server-controlled error text serialised into the failure record's error field | medium | mitigate | The truncation applied for terminal rendering in 02-01 does not apply to JSON. Record in the plan summary that the machine-readable failure error is intentionally untruncated for fidelity, and that Phase 3's OUT-03 stdout/stderr split is what keeps it out of parseable output when it should be. |
| T-02-06 | Tampering / contract integrity | the eleven structs' field names and the duration value type | high | mitigate | These become a one-way public contract at Phase 3. Task 1 forces the duration shape to be an explicit blocking decision, and the exact-key-set test in Task 2 makes any later accidental rename a failing test rather than a silent break. |
| T-02-SC | Tampering | go module installs | n/a | mitigate | This plan adds zero dependencies; `go.mod` and `go.sum` must be unchanged. No package legitimacy checkpoint required. |
</threat_model>

<artifacts_this_phase_produces>
| Kind | Symbol / path |
|------|---------------|
| file | `internal/ado/json_test.go` |
| struct tags | `json:` tags on `Branch`, `DashboardCounts`, `PipelineRun`, `PipelineDefinition`, `PullRequest`, `Release`, `ReleaseEnvironment`, `ReleaseApproval`, `WorkItem`, `WorkItemDetail`, `Attachment` |
| struct tags | `json:` tags on the 02-01 `Result` envelope and `Failure` record |
| exclusion | `ado.Client` is explicitly untagged and carries a comment saying why |
| renamed tests | eight `TestGet*ReportsProjectFailure` functions replacing the Phase 1 characterization names |

The `Result`, `Failure`, and `FailureKind` symbols themselves are created by 02-01,
not by this plan.
</artifacts_this_phase_produces>

<verification>
- `go vet ./internal/ado/ && gofmt -l internal/ado && go test ./internal/ado/ -count=1` is green.
  (The repo-wide gate is deliberately **not** run here — see `<wave_scoping>`. It is
  plan 02-05's phase-close task.)
- `go test ./internal/ado/ -run TestJSONFieldNames -count=1 -v` passes with a subtest
  per tagged struct plus the envelope and the failure record.
- The Phase 1 anchored gate returns zero matches and the anchored replacement pattern
  returns eight.
- `git diff --stat go.mod go.sum` is empty.
- `git diff --stat -- . ':!internal/ado'` is empty — this plan touched nothing outside
  the package it owns for the wave.
- The Task 1 decision is recorded in the plan summary with its rationale.
</verification>

<success_criteria>
- Eleven structs tagged, one deliberately excluded, both facts pinned by tests.
- The duration contract was chosen by a human before it was written down.
- The Phase 1 countdown gate reads zero, which is the phase's cleanest completion signal.
</success_criteria>

<output>
Create `.planning/phases/02-honest-reads/02-04-SUMMARY.md` when done. It must record
the duration-shape decision from Task 1 and its rationale, the confirmation or
refutation of assumption A3, and the note that the JSON failure error text is
intentionally untruncated.
</output>
