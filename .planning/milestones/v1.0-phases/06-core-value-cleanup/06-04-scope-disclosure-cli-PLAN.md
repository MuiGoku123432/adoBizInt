---
phase: 06-core-value-cleanup
plan: 04
type: execute
wave: 2
depends_on: ["06-01", "06-02", "06-03"]
files_modified:
  - cmd/adoBizInt/render.go
  - cmd/adoBizInt/dispatch.go
  - cmd/adoBizInt/main_test.go
  - cmd/adoBizInt/usage.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/usage_test.go
autonomous: true
requirements: [CLI-06, CLI-05, TRUST-03]
coupling_justified: ["06-05: same Go package (cmd/adoBizInt) in the same wave but disjoint files; 06-04 adds renderScope and verbScopes, 06-05 adds only test-local types in crash_test.go, so neither declares a symbol the other does and either order compiles and tests identically"]

estimate:
  tokens: 30000
  raw_tokens: 30000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "In human mode, a scoped read writes exactly one scope line to stderr and none to stdout: workitems list writes \"scope: excludes Done/Closed/Removed; recent iterations only\" (D-04, ROADMAP SC3 / audit W4)"
    - "The scope line is written even when the read returns zero rows, which is the case W4 exists for: stdout stays empty and stderr carries only the scope line (D-04) [edge TRUST-03/empty]"
    - "dashboard counts, prs list and releases approvals write \"scope: excludes Done/Closed/Removed; recent iterations only; active pull requests only\", \"scope: active pull requests only\" and \"scope: pending approvals only\" respectively; a read whose scope is null (branches list) writes no scope line (D-04, D-15, D-16)"
    - "When a scoped read also has failures, stderr carries the scope line first and the existing warning lines after it, and the exit code is still derived only from failures and complete (D-04, TRUST-03) [edge TRUST-03/adjacency]"
    - "Under --json nothing is written to stderr for scope; the envelope on stdout carries the scope object unchanged and stdout holds exactly one JSON document (D-04, OUT-03, Phase 3 D-11)"
    - "workitems list --help, dashboard counts --help, prs list --help and releases approvals --help each print a Scope: paragraph stating the fixed server-side filter: Done, Closed and Removed excluded and iterations limited to those starting or finishing within the last 30 days (with the no-dated-iteration fallback) for the two work-item verbs, active pull requests for prs list and the dashboard, pending approvals for releases approvals (D-06, D-15)"
    - "No unscoped read verb and no write verb prints a Scope: paragraph, and verb help stays byte-identical across runs (D-06, D-16, CLI-05)"
    - "--hide-done's help text says Done, Closed and Removed are always excluded by Azure DevOps and that the flag additionally drops Resolved items client-side; the flag's behaviour is unchanged (D-05, CLI-06)"
  prohibitions:
    - "Must NOT write the scope line to stdout or under --json (D-04, OUT-03)"
    - "Must NOT wrap ado.Result in a CLI-side type in emitJSON; scope rides in the raw envelope (Phase 3 D-11)"
    - "Must NOT change --hide-done's filtering behaviour or add any widening flag (D-03, D-05)"
    - "Must NOT describe the recency window as 90 days or as an unconditional limit (RESEARCH Pitfall 7)"
  artifacts:
    - path: "cmd/adoBizInt/render.go"
      provides: "renderScope(w io.Writer, s *ado.Scope): one stderr line, nothing for nil"
      contains: "func renderScope"
    - path: "cmd/adoBizInt/dispatch.go"
      provides: "runRead's human branch calls renderScope before renderFailures"
      contains: "renderScope(stderrWriter, result.Scope)"
    - path: "cmd/adoBizInt/main_test.go"
      provides: "TestScopeLineOnStderrInHumanMode, TestScopeRidesInJSONEnvelopeOnly"
      contains: "TestScopeLineOnStderrInHumanMode"
    - path: "cmd/adoBizInt/usage.go"
      provides: "verbScopes map and the Scope: paragraph in usageVerb"
      contains: "verbScopes"
    - path: "cmd/adoBizInt/flags.go"
      provides: "accurate --hide-done usage string"
      contains: "always excluded by Azure DevOps"
    - path: "cmd/adoBizInt/usage_test.go"
      provides: "TestScopedVerbHelpStatesScope, TestHideDoneHelp"
      contains: "TestScopedVerbHelpStatesScope"
  key_links:
    - from: "cmd/adoBizInt/dispatch.go runRead human branch"
      to: "cmd/adoBizInt/render.go renderScope"
      via: "renderScope(stderrWriter, result.Scope) in the non-JSON branch only"
      pattern: "renderScope\\(stderrWriter, result\\.Scope\\)"
    - from: "cmd/adoBizInt/usage.go usageVerb"
      to: "verbScopes"
      via: "a Scope: paragraph printed after the summary when the verb has an entry"
      pattern: "verbScopes\\[name\\]"
---

<objective>
Close the presentation half of audit warning W4. Plan 06-03 put `scope` into the JSON envelope. This plan makes the same fact visible to a human and in help:

- one scope line on stderr in human mode (D-04)
- a `Scope:` paragraph in each scoped verb's `--help` (D-06)
- an accurate `--hide-done` description (D-05)

Today `--hide-done` claims to be "off by default", which implies done items are shown by default. They are not: the server already drops Done, Closed and Removed.

Purpose: CLI-06 (a filter flag's help tells the truth about what it adds), CLI-05 (help states each verb's fixed scope) and TRUST-03 (a human, like an agent, can tell "not in scope" from "does not exist").
Output: `renderScope` and its call site, `verbScopes` in usage, the reworded flag, and their tests.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/06-core-value-cleanup/06-CONTEXT.md
@.planning/phases/06-core-value-cleanup/06-RESEARCH.md
@.planning/phases/06-core-value-cleanup/06-03-SUMMARY.md
@cmd/adoBizInt/render.go
@cmd/adoBizInt/usage.go
</context>

<interfaces>
Produced by 06-03, which must be complete:

- `type Scope struct { ExcludedStates []string; Iterations string; PRStatus string; ApprovalStatus string }` in internal/ado/result.go, with json tags excluded_states, iterations, pr_status and approval_status, each omitempty.
- `Result[T].Scope *Scope`, tagged json:"scope" without omitempty.
- The values per read: workitems list {ExcludedStates: Done, Closed, Removed; Iterations: "recent"}; dashboard counts adds PRStatus "active"; prs list {PRStatus: "active"}; releases approvals {ApprovalStatus: "pending"}; the other four reads leave it nil.

Produced by 06-01, which must be complete:

- handleUsage prints verb help from a fresh newFlagSet(name) through usageVerb.
- runRead refuses stray positionals right after Parse.
- usage_test.go holds the W2 and D-17 tests.

Existing, unchanged:

- `renderFailures(w io.Writer, failures []ado.Failure)` in render.go:94-103. It is the single-write, nothing-when-empty analog.
- runRead's human branch in dispatch.go (~lines 323-330, shifted by 06-01's D-17 lines) calls render(stdoutWriter, result.Items) and then renderFailures(stderrWriter, result.Failures).
- `verbSummaries` in usage.go:46-62. It is the map shape to copy.
- usageVerb (usage.go:119-131) prints the summary, then the write notice, then Flags.
- `allReadsFake(t)` and `emptyResult[T]()` in fakes_test.go:101-136. emptyResult leaves Scope nil, and TestRenderEmptyWritesNothing depends on that, so do not change either.
- `captureOutput`, `twoFailures()` and `renderCases()` in main_test.go. `filterSupportByCommand` in flags.go gates --hide-done to workitems list.
</interfaces>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): a human running workitems list sees the server-side scope on stderr, even with zero rows (W4, D-04)</name>
  <files>cmd/adoBizInt/render.go, cmd/adoBizInt/dispatch.go, cmd/adoBizInt/main_test.go</files>
  <read_first>
    - cmd/adoBizInt/render.go: the whole file (154 lines). Note renderFailures at 84-103
    - cmd/adoBizInt/dispatch.go: runRead's emit section (the `if gf.asJSON` block and its else branch)
    - cmd/adoBizInt/main_test.go lines 590-680: TestRenderFailuresToStderrOnly, TestRenderEmptyWritesNothing, TestRenderNotInvokedUnderJSON, and the renderCases and twoFailures helpers they use
    - cmd/adoBizInt/fakes_test.go lines 95-140: allReadsFake and emptyResult
    - internal/ado/result.go: the Scope type from 06-03
  </read_first>
  <behavior>
    New TestScopeLineOnStderrInHumanMode in main_test.go, a table run in human mode (no --json). Each row:
    - starts from allReadsFake(t) and overrides one closure to return a result with the given Scope, empty Failures and Complete true
    - asserts the exit code is exitOK
    - asserts stderr equals the expected string exactly
    - asserts stdout contains no "scope:" text

    Rows:
    1. workitems list with zero items and Scope{ExcludedStates: Done, Closed, Removed; Iterations "recent"}. Stdout is empty. Stderr is exactly "scope: excludes Done/Closed/Removed; recent iterations only\n".
    2. dashboard counts with one DashboardCounts item and Scope{the same plus PRStatus "active"}. Stderr is exactly "scope: excludes Done/Closed/Removed; recent iterations only; active pull requests only\n".
    3. prs list with zero items and Scope{PRStatus "active"}. Stderr is exactly "scope: active pull requests only\n".
    4. releases approvals with zero items and Scope{ApprovalStatus "pending"}. Stderr is exactly "scope: pending approvals only\n".
    5. branches list with a nil Scope (allReadsFake unchanged). Stderr is empty.

    One more subtest, "scope precedes failures":
    - prs list returns Scope{PRStatus "active"} plus one FailureTimeout failure for project "alpha", with Complete false.
    - Expect exitPartial.
    - Stderr's first line is "scope: active pull requests only", and the second line starts with "warning: alpha:".

    New TestScopeRidesInJSONEnvelopeOnly in main_test.go:
    - workitems list --json with row 1's scope.
    - Expect exitOK and an empty stderr.
    - Stdout holds exactly one newline-terminated document.
    - Decoding its scope field as json.RawMessage gives exactly {"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}.

    RED: rows 1-4 and "scope precedes failures" fail because nothing writes a scope line. Row 5 and the JSON test already pass and act as pins.
  </behavior>
  <action>
    RED first:
    - Add both tests to main_test.go as the behavior block states, each with a doc comment citing D-04 and audit W4.
    - Override closures on allReadsFake(t). Do not modify allReadsFake or emptyResult; TestRenderEmptyWritesNothing relies on their nil scope.
    - Record the RED output in the SUMMARY.

    GREEN in cmd/adoBizInt/render.go, per D-04, add renderScope(w io.Writer, s *ado.Scope):
    1. It returns without writing when s is nil.
    2. Otherwise it collects the parts in this order:
       - "excludes " + ExcludedStates joined with "/" (when non-empty)
       - Iterations + " iterations only" (when non-empty)
       - PRStatus + " pull requests only" (when non-empty)
       - ApprovalStatus + " approvals only" (when non-empty)
    3. It writes "scope: " + the parts joined with "; " + newline in a single io.WriteString, matching renderFailures' single-write discipline.
    4. Doc comment:
       - D-04: one line on stderr, never stdout.
       - It prints even for zero rows, because an empty result under an undisclosed filter is exactly W4.
       - Values are code constants from internal/ado, so sanitize is not needed.
       - Under --json the envelope carries the scope instead.

    GREEN in cmd/adoBizInt/dispatch.go runRead's human (else) branch:
    - Call renderScope(stderrWriter, result.Scope) after render and before renderFailures.
    - Extend that branch's comment: the scope line is the human counterpart of the envelope's scope (D-04).
    - Do not touch the --json branch or emitJSON. Scope already rides in the raw envelope, and Phase 3 D-11 forbids a wrapper.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestScopeLineOnStderrInHumanMode|TestScopeRidesInJSONEnvelopeOnly|TestRenderEmptyWritesNothing|TestRenderFailuresToStderrOnly|TestRenderNotInvokedUnderJSON|TestRenderRowsToStdout|TestRenderHasNoANSI)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 7 top-level "--- PASS: Test" lines, or fewer than 6 "--- PASS: TestScopeLineOnStderrInHumanMode/" subtest lines</fails_when>
  </verify>
  <acceptance_criteria>
    - TestScopeLineOnStderrInHumanMode passes all 6 subtests:
      - the four exact scope lines, including the zero-row workitems row with empty stdout
      - no line for a null scope
      - the scope line before the failure warning
    - TestScopeRidesInJSONEnvelopeOnly passes: empty stderr, one stdout document, and the exact scope object.
    - TestRenderEmptyWritesNothing, TestRenderFailuresToStderrOnly, TestRenderNotInvokedUnderJSON, TestRenderRowsToStdout and TestRenderHasNoANSI pass unchanged.
    - The SUMMARY records RED evidence.
  </acceptance_criteria>
  <done>A human running a scoped read in the default mode is told on stderr which items the server left out, including when nothing came back. Stdout and the --json envelope are unchanged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: each scoped verb's --help states its fixed scope, and --hide-done says what it actually adds (D-05, D-06)</name>
  <files>cmd/adoBizInt/usage.go, cmd/adoBizInt/flags.go, cmd/adoBizInt/usage_test.go</files>
  <read_first>
    - cmd/adoBizInt/usage.go: as left by 06-01. Note verbSummaries at ~46-62 and usageVerb at ~112-131
    - cmd/adoBizInt/flags.go lines 217-238: the filter flag registrations, with --hide-done at 227-230
    - internal/ado/filters.go lines 1-20: doneStates is Done, Closed, Resolved and Removed
    - internal/ado/client.go: GetRecentIterations' 30-day cutoff and the include rule (start or finish after the cutoff; undated iterations never included; no qualifying iteration means no iteration clause)
    - cmd/adoBizInt/usage_test.go: as left by 06-01, for the captureOutput plus handleUsage idiom of TestHelpNeedsNoCredentials
  </read_first>
  <behavior>
    New TestScopedVerbHelpStatesScope in usage_test.go. For each verb, call handleUsage([noun, verb, "--help"]) and capture stderr:
    - workitems list: stderr contains "Scope:", "Done", "Closed", "Removed", "30 days", "not limited by iteration" and "Resolved"
    - dashboard counts: stderr contains "Scope:", "Done", "Closed", "Removed", "30 days" and "active pull requests"
    - prs list: stderr contains "Scope:" and "active pull requests"
    - releases approvals: stderr contains "Scope:" and "pending approvals"
    - For every other key in commandKeys(), the four unscoped reads and the six write verbs: stderr does not contain "Scope:"

    New TestHideDoneHelp in usage_test.go:
    - newFlagSet("workitems list").Lookup("hide-done").Usage contains "Resolved", "Done", "Closed", "Removed", "always excluded by Azure DevOps" and "off by default".

    TestHelpIsDeterministic and TestFilterFlagsRegisteredOnSupportedVerbsOnly stay green unchanged. So does TestFilterFlagsPostFilterItems, which proves --hide-done's behaviour did not change.

    RED: every scoped-verb assertion fails because no Scope: paragraph exists. TestHideDoneHelp fails on the current "drop items in a done or resolved state (off by default)" text. The unscoped rows already pass and act as pins.
  </behavior>
  <action>
    RED first:
    - Add both tests as the behavior block states, with doc comments citing D-05, D-06, D-15 and D-16.
    - Record the RED output in the SUMMARY.

    GREEN in cmd/adoBizInt/usage.go, per D-06:
    1. Add verbScopes, a map from the verb key to a help paragraph, beside verbSummaries and in the same shape. It has exactly four entries.
       - "workitems list": "Scope: Azure DevOps returns only work items whose state is not Done, Closed or Removed, in iterations that start or finish within the last 30 days; a project with no such dated iteration is not limited by iteration. --hide-done additionally drops Resolved items. Under --json the envelope's scope object states the same."
       - "dashboard counts": "Scope: work items are counted only when their state is not Done, Closed or Removed, in iterations that start or finish within the last 30 days (a project with no such dated iteration is not limited by iteration), and only active pull requests are counted. Under --json the envelope's scope object states the same."
       - "prs list": "Scope: Azure DevOps returns only active pull requests; completed and abandoned pull requests are not listed. Under --json the envelope's scope object states the same."
       - "releases approvals": "Scope: Azure DevOps returns only pending approvals. Under --json the envelope's scope object states the same."
    2. Doc comment on verbScopes:
       - It is the help-side counterpart of the scope 06-03 put in the envelope (D-01, D-06).
       - Only reads with a built-in server-side filter appear (D-02, D-15, D-16).
       - The window is 30 days, because that is client.go's cutoff (RESEARCH Pitfall 7).
    3. In usageVerb, after the summary paragraph and before the write notice, print the scope paragraph the same way the summary is printed, when verbScopes has an entry for name.

    GREEN in cmd/adoBizInt/flags.go, per D-05:
    - Change only the --hide-done usage string to "also drop Resolved items client-side; Done, Closed and Removed are always excluded by Azure DevOps (off by default)".
    - Leave the flag name, default and filtering behaviour as they are.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestScopedVerbHelpStatesScope|TestHideDoneHelp|TestHelpIsDeterministic|TestHelpNeedsNoCredentials|TestFilterFlagsRegisteredOnSupportedVerbsOnly|TestFilterFlagsPostFilterItems|TestWriteVerbUsageDocumentsConfirm|TestHelpTokenInFlagPositionIsHelp)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, or fewer than 8 top-level "--- PASS: Test" lines</fails_when>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"</automated>
    <fails_when>non-zero exit from any stage, any "FAIL" package line, or gofmt listing a file</fails_when>
  </verify>
  <acceptance_criteria>
    - TestScopedVerbHelpStatesScope passes:
      - each of the four scoped verbs' help contains its listed tokens, including "30 days" for both work-item verbs
      - none of the other ten verbs prints "Scope:"
    - TestHideDoneHelp passes on the reworded usage string.
    - TestHelpIsDeterministic, TestHelpNeedsNoCredentials, TestFilterFlagsRegisteredOnSupportedVerbsOnly, TestFilterFlagsPostFilterItems, TestWriteVerbUsageDocumentsConfirm and TestHelpTokenInFlagPositionIsHelp pass unchanged.
    - The SUMMARY records RED evidence.
    - The full module gate exits 0.
  </acceptance_criteria>
  <done>Help states each scoped verb's fixed server-side filter in the same terms as the envelope, and --hide-done no longer implies that done items appear by default.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| runRead output to stdout (parseable) and stderr (diagnostic) | OUT-03 makes stdout parse-only. A human or agent reads stderr for context. |
| --help text to the agent reading it | An agent that reads help rather than source learns each verb's semantics from this text alone. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-06-14 | Tampering (output channel integrity) | scope line on the parseable channel | medium | mitigate | renderScope is called only from the human branch, with stderrWriter. TestScopeLineOnStderrInHumanMode asserts stdout carries no scope text, and TestScopeRidesInJSONEnvelopeOnly asserts an empty stderr and a single stdout document under --json. |
| T-06-15 | Spoofing (misleading documentation) | --hide-done and verb help overstating or understating scope | medium | mitigate | TestScopedVerbHelpStatesScope pins the excluded states, the "30 days" window and its no-dated-iteration fallback, and the active and pending filters. TestHideDoneHelp pins that Done, Closed and Removed are always excluded and that Resolved is the flag's addition. |
| T-06-16 | Information Disclosure | scope line content | low | accept | The line is built from internal/ado code constants only. No server text, credential or URL can reach it. |
| T-06-SC | Tampering | dependency installs | low | accept | Not applicable: no package install, and go.mod is untouched. |
</threat_model>

<verification>
- Module gate (proven command): `cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`. It fails on a non-zero exit, a "FAIL" package line, or gofmt listing a file.
- 06-VALIDATION.md W4 CLI row is green: `go test ./cmd/adoBizInt -run 'Scope|TestHideDoneHelp' -count=1`.
- Optional live confirmation, a human step for /gsd-verify-work that does not block this plan: `adoBizInt workitems list` prints rows on stdout and one "scope:" line on stderr. `adoBizInt workitems list --help` shows the Scope: paragraph.
</verification>

<success_criteria>
- ROADMAP SC3 holds in full together with 06-03: the fixed scope of workitems list and prs list (and dashboard counts, releases approvals) is disclosed in help (D-06), on stderr in human mode (D-04) and in the JSON envelope.
- --hide-done's help is accurate (D-05), and its behaviour is unchanged.
</success_criteria>

## Artifacts this phase produces

| Artifact | Kind | Purpose |
|----------|------|---------|
| `cmd/adoBizInt/render.go` | modified | `renderScope` (D-04) |
| `cmd/adoBizInt/dispatch.go` | modified | `renderScope` call in runRead's human branch |
| `cmd/adoBizInt/main_test.go` | modified | `TestScopeLineOnStderrInHumanMode`, `TestScopeRidesInJSONEnvelopeOnly` |
| `cmd/adoBizInt/usage.go` | modified | `verbScopes` and the `Scope:` help paragraph (D-06) |
| `cmd/adoBizInt/flags.go` | modified | Accurate `--hide-done` usage (D-05) |
| `cmd/adoBizInt/usage_test.go` | modified | `TestScopedVerbHelpStatesScope`, `TestHideDoneHelp` |
| `.planning/phases/06-core-value-cleanup/06-04-SUMMARY.md` | new | Execution record with RED evidence |

<output>
Create `.planning/phases/06-core-value-cleanup/06-04-SUMMARY.md` when done. Record the exact stderr wording per verb, the help paragraphs, the --hide-done text, and the RED-then-GREEN evidence.
</output>
