---
phase: 05-hardening
plan: 04
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/main_test.go
  - cmd/adoBizInt/filter_test.go
  - cmd/adoBizInt/usage_test.go
  - cmd/adoBizInt/write_test.go
  - cmd/adoBizInt/writeverbs_test.go
  - cmd/adoBizInt/audit_test.go
autonomous: true
requirements: [HARD-04]

estimate:
  tokens: 55000
  raw_tokens: 55000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "`adoBizInt workitems list --project bogus` exits 3 when `bogus` is not in the configured project list. Stdout is empty, stderr names the rejected value in quotes, and the read client is never called (HARD-04, D-09)."
    - "Rejection is fail-closed across the whole invocation. `--project P,bogus`, `--project bogus --project P` and `--project bogus,P` all exit 3 even though `P` is configured (D-09)."
    - "Write verbs get the same check at the same point. `workitems set-state 4821 --project bogus --state Closed` exits 3 before the audit sink is opened, so the audit file is not created, and no write-client method is called (D-09; fail-closed applies equally to writes)."
    - "The check runs while `--project` is parsed, inside the one `newFlagSet` registration site. Every current and future read and write verb therefore inherits it (D-09, the Phase 3 D-20 single-registration precedent). The existing parse-error path in `runRead` and `runWrite` supplies exit 3 with no new branch."
    - "Matching is exact and case-sensitive. `--project p` against a configured `P` exits 3; no case-folding, prefix or fuzzy match (RESEARCH assumption A2)."
    - "A nil or empty configured list rejects every supplied `--project` value (fail closed). `run()` assigns the list from `cfg.Projects` immediately after config validation, beside `readBudget` (D-09)."
    - "An omitted `--project`, or one that is empty or only separators, contributes no value and is not rejected. A read then falls back to the configured list exactly as before (edge-probe HARD-04/empty; stringList semantics unchanged)."
    - "Every pre-existing cmd test still exercises the path it was written for. Each one that passes `--project` declares, through `withConfiguredProjects`, every project value it uses. The arity tests with `--project P --project Q` configure both `P` and `Q`, so they still reach the exactly-one check rather than failing early at parse."
  prohibitions:
    - statement: "An unknown or differently-cased --project must never be silently mapped onto a configured project by case-folding, prefix or fuzzy match. A read or write must never run against a project the caller did not name exactly."
      status: resolved
      verification: test
    - statement: "A rejected --project value must never be written to stdout. Stdout stays parseable-only (OUT-03), and the diagnostic goes to stderr."
      status: resolved
      verification: test
  artifacts:
    - path: "cmd/adoBizInt/flags.go"
      provides: "configuredProjects var and the validating --project registration in newFlagSet"
      contains: "configuredProjects"
    - path: "cmd/adoBizInt/main.go"
      provides: "configuredProjects assigned from cfg.Projects after validation"
      contains: "configuredProjects = "
    - path: "cmd/adoBizInt/main_test.go"
      provides: "withConfiguredProjects save/restore helper"
      contains: "func withConfiguredProjects("
    - path: "cmd/adoBizInt/filter_test.go"
      provides: "TestUnknownProjectRejected table"
      contains: "func TestUnknownProjectRejected("
  key_links:
    - from: "cmd/adoBizInt/main.go"
      to: "cmd/adoBizInt/flags.go"
      via: "run() assigns configuredProjects after cfg.Validate; the --project parse callback reads it"
      pattern: "configuredProjects = \\[\\]string\\(cfg\\.Projects\\)"
    - from: "cmd/adoBizInt/flags.go"
      to: "cmd/adoBizInt/dispatch.go"
      via: "a rejected value makes fs.Parse return an error, which runRead/runWrite already map to exitUnknown (3)"
      pattern: "fs\\.Func\\(\"project\""
---

<objective>
Validate every agent-supplied `--project` value against the configured project list
before any read or write runs. An unknown value exits 3, meaning nothing ran.

Purpose: `--project` shipped in Phase 3, so an agent-supplied string reaches both WIQL
sites today. This plan is the cmd policy layer of HARD-04. Plan 05-05 is the library
layer, where the project name stops entering query text at all.
Output: one validating flag registration, one package variable, one test helper, one
new test, and explicit configured-project declarations in the existing tests.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/05-hardening/05-CONTEXT.md
@.planning/phases/05-hardening/05-RESEARCH.md
@.planning/phases/05-hardening/05-PATTERNS.md
</context>

## Artifacts this phase produces

Phase 05 creates or changes these symbols. None of them exists yet. **This plan owns the 05-04 rows.**

| Plan | Symbol / file | Kind |
|---|---|---|
| 05-01 | `internal/ado/attachments.go`; `DownloadAttachment(ctx, id uuid.UUID, destDir, name string) error`; `attachmentPath(name, destDir string) (string, error)`; `Attachment.ID`; `workItemAPI.GetAttachmentContent`; `fakeWorkItemAPI.GetAttachmentContentFunc` | new / replaced |
| 05-01 | removed: `Client.connection`, `AttachmentDownloadedMsg.Path`, URL-based `DownloadAttachment` | deletions |
| 05-01 | six attachment tests incl. `TestNoRawHTTPInADO` | new tests |
| 05-02 | `internal/ado/nilguard_test.go` with three tests | new tests |
| 05-03 | `.github/workflows/ci.yml` | new file |
| **05-04** | **`var configuredProjects []string` in `cmd/adoBizInt/flags.go`** | **new package var** |
| **05-04** | **`--project` registered with `fs.Func` (was `fs.Var(&gf.projects, ...)`), validating each value against `configuredProjects`** | **changed registration** |
| **05-04** | **`withConfiguredProjects(t *testing.T, names ...string)` in `cmd/adoBizInt/main_test.go`** | **new test helper** |
| **05-04** | **`TestUnknownProjectRejected` in `cmd/adoBizInt/filter_test.go`** | **new test** |
| 05-05 | `func iterationClause(paths []string) string`; `internal/ado/wiql_test.go` with four tests | new |
| 05-06 | `TestInitSetsOwnerOnlyPermissions`; `TestPATNeverLogged` | new tests |

## Planner decisions recorded in this plan

| Item | Decision | Rationale |
|---|---|---|
| Where the check lives | **Inside the `--project` registration in `newFlagSet`, as an `fs.Func` callback.** It is not a helper called from `runRead` and `runWrite`. | The check exists in exactly one place, with no duplicate and no two-use extraction, so the user's rule of three is not in play. `newFlagSet` is already the only `flag.NewFlagSet` site (Phase 3 D-20), so every verb inherits the check structurally, the same reasoning D-03 applies to attachments. It also runs at the earliest moment the values exist. `runRead` and `runWrite` already map a non-help parse error to `exitUnknown`, so neither function changes. RESEARCH and PATTERNS suggested a helper called from two places; this is a planner choice, not a locked decision, and the placement here is the stricter one. |
| D-09 "right after `config.Load`" | The check fires during flag parsing inside `dispatch`, which runs after `ado.NewClient`. | `--project` is parsed per verb, and `run()` never sees the values (RESEARCH Pitfall 5). `NewClient` does identity and location lookups but no project-scoped read, so "before any read runs" holds exactly. The exit code (3) is the D-09 contract. |
| Write verbs | **Included** | Fail-closed applies equally to writes, and exit 3 already means "nothing attempted" (RESEARCH Pitfall 5). The rejection precedes `auditOpen`, so the audit file is never created. |
| Case sensitivity | **Exact match** (RESEARCH A2) | Predictable and fail-closed. An agent's typo exits 3 rather than landing on a near-miss project. |
| Where `configuredProjects` is declared | `cmd/adoBizInt/flags.go`, beside the registration that reads it | Its only reader is the `--project` callback. It mirrors `readBudget`'s write-after-validation contract, and its doc comment says so. |
| Stderr text | The flag package prints `invalid value "<raw>" for flag -project: <reason>`, followed by the verb's usage | This is the same path as every existing flag error. The reason names the offending part with `%q` and lists the configured projects. Stderr is human diagnostics, not the published contract; only the exit code is. |
| Tracer file count (8) | Above the 5-file heuristic. **Accepted and recorded here.** | The fail-closed check turns every existing `--project` test red in the same commit. Two production files carry the change. The other six test files gain only `withConfiguredProjects(t, ...)` lines, so splitting them out would commit a red suite at the tracer. |

## Edge-probe and prohibition-probe disposition (spec-less fallback)

| Probe item | Disposition |
|---|---|
| HARD-04 / adjacency (two values exactly equal, or equal except case) | resolved (explicit). Exact, case-sensitive match (truth 5). Duplicate valid values each validate. De-duplicating them is pre-existing pass-through behaviour, **flagged, and out of scope** for this plan. |
| HARD-04 / empty | resolved (explicit). An empty or separator-only value contributes nothing and is not rejected. A nil configured list rejects every supplied value (truths 6 and 7). |
| HARD-04 / ordering | resolved (explicit). Rejection does not depend on position: a bogus value before, after or between valid ones exits 3 (truth 2). |
| Prohibition recall | Two kept (see `must_haves.prohibitions`). Injection itself is canon, covered by `/gsd-secure-phase`, and plan 05-05 removes the interpolation. |

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: An unknown --project exits 3 before any read or write is attempted</name>
  <reversibility rating="one-way">D-09: exit 3 for an unknown project joins the published exit-code vocabulary agents branch on (Phase 3 D-06/D-07/D-08). The user decided this in 05-CONTEXT.md, so no checkpoint re-asks it.</reversibility>
  <files>cmd/adoBizInt/flags.go, cmd/adoBizInt/main.go, cmd/adoBizInt/main_test.go, cmd/adoBizInt/filter_test.go, cmd/adoBizInt/usage_test.go, cmd/adoBizInt/write_test.go, cmd/adoBizInt/writeverbs_test.go, cmd/adoBizInt/audit_test.go</files>
  <read_first>
    - cmd/adoBizInt/flags.go (whole file: `globalFlags` at :12-16, `stringList` and its `Set` at :141-153, `newFlagSet` at :168-245 with the `--project` registration at :177-178, and the imports at :3-7)
    - cmd/adoBizInt/main.go:62-110 (`run`: `config.Load`, `cfg.Validate`, and `readBudget = cfg.FetchTimeout` at :105)
    - cmd/adoBizInt/dispatch.go:38-49 (`readBudget` doc-comment style) and :242-252 (runRead's parse-error branch returning `exitUnknown`)
    - cmd/adoBizInt/write.go:136-150 (runWrite's parse-error branch) and :111-121 (`oneProject`, whose arity check must stay reachable)
    - cmd/adoBizInt/main_test.go:691-699 (`withReadBudget`, the helper shape to copy)
    - cmd/adoBizInt/filter_test.go:146-180 (`TestFilterFlagsProjectNarrowsTheRead` and the `workItemsClient` fixture)
    - cmd/adoBizInt/write_test.go:354-390 (`TestWriteParseErrorsExitUnknown`, including the "two projects" row at :363) and :940-960 (the guard matrix's "two projects" row at :953)
    - .planning/phases/05-hardening/05-RESEARCH.md § Pitfall 5 and § Pitfall 6
  </read_first>
  <behavior>
    - With `withConfiguredProjects(t, "P")`, `dispatch(ctx, &fakeReadClient{t: t}, []string{"workitems", "list", "--project", "bogus", "--json"})` returns 3. The captured stdout has length 0, and the captured stderr contains `"bogus"` including the quotes.
    - With `withConfiguredProjects(t, "one")`, the existing `TestFilterFlagsProjectNarrowsTheRead` still passes exactly as before.
    - Every existing cmd test passes after gaining the configured projects it uses.
  </behavior>
  <action>
    Write the single-row `TestUnknownProjectRejected` in `cmd/adoBizInt/filter_test.go` first and see it fail: the read runs, so exit is not 3. Then implement.

    **`cmd/adoBizInt/flags.go`**:
    - Declare `var configuredProjects []string` beside `globalFlags`. Its doc comment, in the `readBudget` style at `dispatch.go:38-49`, must say:
      - it is the D-09 allow-list;
      - `run()` assigns it from `cfg.Projects` only after config validation;
      - nil or empty rejects every supplied `--project` value, so the zero value is the fail-closed one;
      - tests set it through `withConfiguredProjects`;
      - matching is exact and case-sensitive.
    - In `newFlagSet`, replace `fs.Var(&gf.projects, "project", ...)` with `fs.Func("project", <the same usage string>, callback)`. The callback does four things:
      1. Splits its value into a local `stringList` by calling that type's existing `Set`, so the comma and whitespace rules stay defined in one place.
      2. Checks each part for exact membership in `configuredProjects` using `slices.Contains`.
      3. On the first miss, returns an error such as `fmt.Errorf("not a configured project: %q (configured: %s)", part, ...)`. Join the configured names with `, `, and say that no projects are configured when the list is empty.
      4. Appends the parts to `gf.projects` only after every part has passed.
    - Add `fmt` and `slices` to the imports. `gf.projects` keeps its `stringList` type, so `emptyIfNil(gf.projects)` in `dispatch.go` and `oneProject` in `write.go` are untouched.

    **`cmd/adoBizInt/main.go`**: immediately after `readBudget = cfg.FetchTimeout` at :105, assign `configuredProjects = []string(cfg.Projects)` with a one-line comment citing D-09.

    **`cmd/adoBizInt/main_test.go`**: add `withConfiguredProjects(t *testing.T, names ...string)` directly below `withReadBudget`. Copy its shape exactly: `t.Helper()`, save the previous value, assign, and restore in `t.Cleanup`. Its doc comment says it stands in for the assignment `run()` makes after config validation.

    **The existing tests**: run `go test ./cmd/adoBizInt/...`. Every test that now fails with exit 3 passes a `--project` value. Add one `withConfiguredProjects(t, ...)` call at the top of that test function, or of the subtest if the values differ per subtest. List **every** project value the test passes. For example, `TestWriteParseErrorsExitUnknown` and the guard matrix pass `P` and `Q`. They must configure both, so the "two projects" rows still reach `oneProject`'s exactly-one check rather than failing early at parse, which would make them pass for the wrong reason.

    RESEARCH Pitfall 6 counted about 30 sites across `write_test.go`, `writeverbs_test.go`, `filter_test.go`, `usage_test.go` and `audit_test.go`. `TestProjectListAccumulates` calls `newFlagSet` and `Parse` directly, so it needs `alpha`, `beta` and `gamma`. Do not add the call inside `captureOutput`, `tempAuditSink`, `setStateArgs` or a `TestMain`. The allow-list must be visible in each test that depends on it. Change no assertion in any existing test.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test -race ./cmd/adoBizInt/... && go test ./cmd/adoBizInt -run 'TestUnknownProjectRejected|TestFilterFlagsProjectNarrowsTheRead|TestWriteParseErrorsExitUnknown|TestProjectListAccumulates' -v</automated>
    <fails_when>non-zero exit, or `no tests to run`, or the -v output lacks `--- PASS: TestUnknownProjectRejected` or `--- PASS: TestWriteParseErrorsExitUnknown`</fails_when>
  </verify>
  <acceptance_criteria>
    - `go test -race ./cmd/adoBizInt/...` exits 0.
    - `newFlagSet` registers `project` through `fs.Func`, and `flag.NewFlagSet` still appears only inside `newFlagSet`. `TestFlagsRegisteredOnEverySubcommand` passes.
    - `run()` assigns `configuredProjects` on the line after `readBudget = cfg.FetchTimeout`.
    - `cmd/adoBizInt/dispatch.go` and `cmd/adoBizInt/write.go` are unchanged by this plan: `git log --format= --name-only --grep='(05-04)'` does not list either. This is scoped by commit message rather than by a revision range, because wave-1 plans commit in parallel.
    - No existing test's assertions changed. The diff to the five pre-existing test files adds only `withConfiguredProjects(` lines.
    - `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>An unknown project is rejected with exit 3 before anything runs, and the whole cmd suite is green. Committed.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Pin the fail-closed edges: mixed values, case, empty list, empty value, and write verbs</name>
  <files>cmd/adoBizInt/filter_test.go</files>
  <read_first>
    - cmd/adoBizInt/filter_test.go (`TestUnknownProjectRejected` as written in Task 1; the `workItemsClient` fixture used by `TestFilterFlagsProjectNarrowsTheRead`)
    - cmd/adoBizInt/write_test.go:20-65 (`tempAuditSink`, `preReadFake`, `setStateArgs`)
    - cmd/adoBizInt/main_test.go:55-80 (`captureOutput`, `emptyReadClient`)
  </read_first>
  <behavior>
    Rows where the fake has no closures set, so any client call fails the row by name. Each asserts exit 3, empty stdout, and the offending value quoted in stderr:
    - `workitems list --project bogus` (configured `P`)
    - `workitems list --project P,bogus`
    - `workitems list --project bogus --project P`
    - `workitems list --project bogus,P`
    - `workitems list --project p` (configured `P`: case mismatch)
    - `workitems list --project P` with `withConfiguredProjects(t)` (nothing configured)
    - `workitems set-state 4821 --project bogus --state Closed --json` with `tempAuditSink(t)`. It also asserts `os.Stat` of the audit path reports not-exist.
    - `prs approve 4821 --project bogus --repo R`

    Rows that reach the read (fixture `workItemsClient`), each asserting exit 0:
    - `workitems list --project P --json` with `P` configured: the read receives `[P]`.
    - `workitems list --project "" --json` with `P` configured: the read receives an empty slice (the empty-value edge).
  </behavior>
  <action>
    Expand `TestUnknownProjectRejected` into a table in the `write_test.go:357-387` style: a named-case map or slice, with `t.Run` per row. Each row carries its own configured list, args, want-exit, and the stderr fragment it expects.

    Rejected rows use `&fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}` with every closure nil. The fakes' own `t.Fatalf` is the "nothing ran" assertion; add no call counter.

    The two positive rows use the existing `workItemsClient` fixture and assert the projects slice it received. The write row also calls `tempAuditSink(t)` and asserts the audit file does not exist afterwards, which proves the rejection preceded `auditOpen` (D-13 ordering unaffected).

    Add a doc comment on the test citing D-09, the fail-closed rule, and exact-match assumption A2.
  </action>
  <verify>
    <automated>go test -race ./cmd/adoBizInt -run 'TestUnknownProjectRejected' -v</automated>
    <fails_when>non-zero exit, or `no tests to run`, or fewer than 10 `--- PASS: TestUnknownProjectRejected/` subtest lines</fails_when>
  </verify>
  <acceptance_criteria>
    - `TestUnknownProjectRejected` has at least 10 subtests covering every row in `<behavior>`, and all pass under `-race`.
    - The case-mismatch row asserts exit 3.
    - The write row asserts that the audit file does not exist after the rejection.
    - Temporarily making the callback compare with `strings.EqualFold` makes the case-mismatch row fail. Revert after confirming.
    - `go test -race ./cmd/adoBizInt/...` exits 0.
  </acceptance_criteria>
  <done>Every fail-closed edge of D-09 is a named, green subtest.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Agent argv → `newFlagSet` | `--project` values are caller-controlled strings |
| `cmd` → `internal/ado` reads and writes | Project names flow into WIQL routes and mutation targets |
| `cmd` → stdout / stderr | Stdout is the parseable contract (OUT-03); stderr is diagnostics |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-05-14 | Tampering / Information Disclosure | Agent-supplied `--project` reaching both WIQL sites and every write target (H5, live since Phase 3) | high | mitigate | Exact-match allow-list against `cfg.Projects` at parse time, fail-closed, exit 3 (D-09). Pinned by `TestUnknownProjectRejected`. Plan 05-05 also removes the project from query text, so the library does not depend on this check. |
| T-05-15 | Elevation of Privilege | Case-folded or fuzzy match redirecting a write to a near-miss project | medium | mitigate | Case-sensitive `slices.Contains`. The case-mismatch row fails if matching is loosened. |
| T-05-16 | Tampering | Terminal-escape bytes in an echoed project value | low | mitigate | The flag package quotes the raw value with `%q`, and the callback's reason formats the part with `%q`. |
| T-05-17 | Information Disclosure | Rejection text or configured names on stdout breaking OUT-03 | low | mitigate | Flag errors go to `fs.SetOutput(stderrWriter)`. Every rejected row asserts empty stdout. |
| T-05-18 | Information Disclosure | Stderr lists configured project names | low | accept | The operator's own project names on the operator's own terminal. They help an agent self-correct and contain no credential. |
| T-05-SC | Tampering | npm/pip/cargo installs | high | mitigate | Not applicable. Only stdlib `slices` and `fmt` are added. |
</threat_model>

<verification>
- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- `go test ./cmd/adoBizInt -run TestUnknownProjectRejected -v` shows at least 10 passing subtests.
</verification>

<success_criteria>
- The validation half of ROADMAP Phase 5 SC4 holds: `--project` is validated against the configured project list.
- The 05-VALIDATION.md HARD-04 rows "Unknown `--project` → exit 3" and "Existing `--project` tests pass with `withConfiguredProjects`" are green.
</success_criteria>

<output>
Create `.planning/phases/05-hardening/05-04-SUMMARY.md` when done
</output>
