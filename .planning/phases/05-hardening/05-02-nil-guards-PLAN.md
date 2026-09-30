---
phase: 05-hardening
plan: 02
type: execute
wave: 1
depends_on: []
coupling_justified: ["05-01: both edit disjoint files in package internal/ado; 05-01 adds no range loop that appends its range value, and 05-02 adds no net/http import, so each plan's package-wide go/ast test passes in either order"]
files_modified:
  - internal/ado/pipelines.go
  - internal/ado/releases.go
  - internal/ado/nilguard_test.go
autonomous: true
requirements: [HARD-03]

estimate:
  tokens: 30000
  raw_tokens: 30000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "`GetPipelineRuns`, given a matching build definition that has a non-nil `Name` and a nil `Id`, does not panic. It returns one row per remaining matching definition, with `Complete == true` and no `Failure` recorded for the skipped definition (HARD-03, D-13, D-14)."
    - "`GetReleases`, given a matching release definition that has a non-nil `Name` and a nil `Id`, does not panic. It returns one row per remaining matching definition, with `Complete == true` and no `Failure` recorded for the skipped definition (HARD-03, D-13, D-14)."
    - "The nil-`Id` guard is a silent `continue` in the same `if` as the existing `Name == nil` guard. It adds no `Failure`, no log line and no exit-code change (D-14, matching the Phase 2 D-07 suppression precedent)."
    - "Exactly two range loops in `internal/ado` append their range value to a slice in a function that later dereferences `.Id`. Both begin with `if v.Name == nil || v.Id == nil { continue }`, and `TestCollectedDefinitionsGuardNameAndID` fails when a third such loop appears or either guard loses a clause (D-13)."
    - "Only the two MED4 sites change. No repo-wide sweep of the other pointer dereferences in `internal/ado` is made (D-13 scope), and the `handlePanic` recover in `cmd/adoBizInt/crash.go` stays the backstop for everything else."
  prohibitions: []
  artifacts:
    - path: "internal/ado/nilguard_test.go"
      provides: "Behavioural nil-Id tests for both reads plus the structural collection-guard pin"
      contains: "TestCollectedDefinitionsGuardNameAndID"
    - path: "internal/ado/pipelines.go"
      provides: "Collection guard in GetPipelineRuns"
      contains: "d.Name == nil || d.Id == nil"
    - path: "internal/ado/releases.go"
      provides: "Collection guard in GetReleases"
      contains: "rd.Name == nil || rd.Id == nil"
  key_links:
    - from: "internal/ado/pipelines.go"
      to: "internal/ado/pipelines.go"
      via: "the guard in the definitions loop is what makes the later defID := *def.Id dereference safe"
      pattern: "d\\.Name == nil \\|\\| d\\.Id == nil"
    - from: "internal/ado/releases.go"
      to: "internal/ado/releases.go"
      via: "the guard in the release-definitions loop is what makes the later defID := *def.Id dereference safe"
      pattern: "rd\\.Name == nil \\|\\| rd\\.Id == nil"
---

<objective>
Close CONCERNS.md MED4. The only two places in `internal/ado` where a definition is
collected into a slice and its `Id` is later dereferenced unguarded now skip a
definition that the server returned without an `Id`. A structural test keeps any future
collecting loop from repeating the shape.

Purpose: a malformed definition from the server currently panics the whole read. The
CLI's crash handler reports that panic as outcome-unknown, and the TUI loses the tab.
Output: two one-clause guards and `internal/ado/nilguard_test.go`.
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

Phase 05 creates or changes these symbols. None of them exists yet. **This plan owns the 05-02 row.**

| Plan | Symbol / file | Kind |
|---|---|---|
| 05-01 | `internal/ado/attachments.go`; `DownloadAttachment(ctx, id uuid.UUID, destDir, name string) error`; `attachmentPath(name, destDir string) (string, error)`; `Attachment.ID`; `workItemAPI.GetAttachmentContent`; `fakeWorkItemAPI.GetAttachmentContentFunc` | new / replaced |
| 05-01 | removed: `Client.connection`, `AttachmentDownloadedMsg.Path`, URL-based `DownloadAttachment` | deletions |
| 05-01 | `TestDownloadAttachmentFetchesByID`, `TestAttachmentPath`, `TestDownloadAttachmentRefusesBeforeFetch`, `TestDownloadAttachmentRemovesPartialFile`, `TestGetWorkItemDetailAttachmentID`, `TestNoRawHTTPInADO` | new tests |
| **05-02** | **`internal/ado/nilguard_test.go`: `TestGetPipelineRunsSkipsNilID`, `TestGetReleasesSkipsNilID`, `TestCollectedDefinitionsGuardNameAndID`** | **new tests (no new production symbol)** |
| 05-03 | `.github/workflows/ci.yml` (workflow `ci`, job `check`) | new file |
| 05-04 | `var configuredProjects []string` (`cmd/adoBizInt/flags.go`); `--project` via `fs.Func`; `withConfiguredProjects`; `TestUnknownProjectRejected` | new |
| 05-05 | `func iterationClause(paths []string) string` (`internal/ado/client.go`); `internal/ado/wiql_test.go` with four tests | new |
| 05-06 | `TestInitSetsOwnerOnlyPermissions` (`internal/logging/logging_test.go`); `TestPATNeverLogged` (`cmd/adoBizInt/patleak_test.go`) | new tests |

## Edge-probe and prohibition-probe disposition (spec-less fallback)

| Probe item | Disposition |
|---|---|
| HARD-03 / unclassified | **Flagged assumption.** The probe could not classify HARD-03. I reviewed it manually. The meaningful edge is "one malformed definition beside well-formed ones", and truths 1 and 2 author it as a fixture with a nil-`Id` definition next to a normal one. The probe row stays `unresolved` per the fallback rules. |
| Prohibition recall | Zero kept. A nil-dereference panic is canon robustness (denial of service), covered by `/gsd-secure-phase`. The D-14 suppression rule is a truth, not a values constraint. |

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: A nil-Id build definition no longer panics GetPipelineRuns</name>
  <files>internal/ado/nilguard_test.go, internal/ado/pipelines.go</files>
  <read_first>
    - internal/ado/pipelines.go:111-215 (`GetPipelineRuns`: the collecting loop at :156-164 and the dereference `defID := *def.Id` at :171)
    - internal/ado/pipelines_test.go:148-199 (`pipelineTestClient` closures and the row-count assertion style to copy)
    - internal/ado/branches.go:84 (in-repo precedent for the combined `Name == nil || Id == nil` guard)
    - internal/ado/client.go:283 (`strPtr`)
  </read_first>
  <behavior>
    - One project, `pipelines: []string{"pipeline-one"}`. `GetDefinitions` returns `{Id: &17, Name: "pipeline-one"}` and `{Name: "pipeline-one"}` (nil `Id`). `GetBuilds` returns an empty response.
    - Before the fix, `GetPipelineRuns` panics on the nil dereference, and the test must be seen failing this way first.
    - After the fix, it returns exactly 1 row with `PipelineID == 17`, `Complete == true` and zero failures.
  </behavior>
  <action>
    Create `internal/ado/nilguard_test.go` with `TestGetPipelineRunsSkipsNilID`.

    Build the `&Client{...}` inline in the test. It is single-use, and `pipelineTestClient`'s hook parameterises errors rather than definitions, so do not widen or fork it. Copy its `GetDefinitionsFunc`, `GetBuildsFunc` and `GetDefinitionFunc` closure shapes from `pipelines_test.go:151-181`, with the fixture in `<behavior>`. Run the test and confirm it fails with the nil-pointer panic.

    Then change the collection guard at `internal/ado/pipelines.go:157` from `if d.Name == nil {` to `if d.Name == nil || d.Id == nil {`. Keep the silent `continue` body (D-14). Do not add a `Failure`, a log line, or a second `if`.

    Do not touch any other dereference in the file. `GetPipelineDefinitions` at :311-327 already guards `Id` inline and is out of scope (D-13).
  </action>
  <verify>
    <automated>go test -race ./internal/ado -run 'TestGetPipelineRunsSkipsNilID|TestGetPipelineRunsHappyPathRowCountUnchanged' -v</automated>
    <fails_when>non-zero exit, or output contains `panic:` or `no tests to run`, or the -v output lacks `--- PASS: TestGetPipelineRunsSkipsNilID`</fails_when>
  </verify>
  <acceptance_criteria>
    - `TestGetPipelineRunsSkipsNilID` passes under `-race` and asserts exactly one row whose `PipelineID` is 17.
    - Reverting the guard to `if d.Name == nil {` makes the test fail with a nil-pointer panic. Revert after confirming.
    - `TestGetPipelineRunsHappyPathRowCountUnchanged` passes unchanged.
  </acceptance_criteria>
  <done>A malformed build definition is skipped rather than crashing the pipeline read. Committed.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Guard GetReleases and pin the collection-guard shape structurally</name>
  <files>internal/ado/releases.go, internal/ado/nilguard_test.go</files>
  <read_first>
    - internal/ado/releases.go:64-175 (`GetReleases`: the collecting loop at :103-118, which appends `rd` in two branches, and the dereference `defID := *def.Id` at :125)
    - internal/ado/releases_test.go:199-240 (`releaseTestClient` closures to copy)
    - internal/ado/preread_test.go:272-330 (go/ast scaffolding, the `seen` non-vacuity check, and the doc-comment rationale)
    - internal/ado/pipelines.go:311-327 (`GetPipelineDefinitions`: the negative control, whose loop appends a freshly built struct, not the range value)
    - .planning/phases/05-hardening/05-RESEARCH.md § Pattern 4 (prototype output: exactly `pipelines.go:156` and `releases.go:103`)
  </read_first>
  <behavior>
    - `TestGetReleasesSkipsNilID`: one project, `releaseDefinitions: []string{"release-definition-one"}`. `GetReleaseDefinitions` returns `{Id: &31, Name: "release-definition-one"}` and `{Name: "release-definition-one"}` (nil `Id`). `GetReleases` returns an empty response. The test first fails with a panic, then passes with exactly 1 row whose `DefinitionID` is 31, `Complete == true` and zero failures.
    - `TestCollectedDefinitionsGuardNameAndID`: over all non-test files in the package, a collecting loop is an `*ast.RangeStmt` whose value is an identifier `v` that appears as a bare `*ast.Ident` argument after the first argument of an `append` call anywhere in the loop body, inside a function that contains a `*X.Id` dereference (`*ast.StarExpr` over a selector named `Id`) positioned after the loop.
      - Every collecting loop's first body statement is an `*ast.IfStmt` whose body is exactly one `continue`. Its condition, flattened over `||`, contains both `v.Name == nil` and `v.Id == nil`.
      - Exactly 2 collecting loops are found.
  </behavior>
  <action>
    Add `TestGetReleasesSkipsNilID` to `internal/ado/nilguard_test.go`. Build the client inline, copying the closure shapes from `releases_test.go:202-236` with the `<behavior>` fixture, and confirm it fails. Then change `internal/ado/releases.go:104` from `if rd.Name == nil {` to `if rd.Name == nil || rd.Id == nil {` with the silent `continue` unchanged (D-14).

    Add `TestCollectedDefinitionsGuardNameAndID` in the same file:
    - Enumerate files with `filepath.Glob("*.go")`, skipping `_test.go` names. Do not use the deprecated `parser.ParseDir`.
    - Parse each with `parser.SkipObjectResolution`.
    - For each `*ast.FuncDecl`, find the collecting loops as defined in `<behavior>`. Require each one's guard exactly as described, and report violations with `fset.Position`.
    - Count the loops found, and fail unless the count is exactly 2. An exact count rather than "at least two" is deliberate: a third collecting loop forces a review (D-13).
    - Write a doc comment that states the rule, why it is scoped to "collect, then dereference `.Id`" rather than to every dereference (D-13), and the go/ast-over-grep rationale copied from `preread_test.go:283-288`.

    The `GetPipelineDefinitions` loop and the `branches.go:84` loop must not match: neither appends its range value. If either matches, the selector is wrong. Fix the test, not the production code.
  </action>
  <verify>
    <automated>go test -race ./internal/ado -run 'TestGetReleasesSkipsNilID|TestGetPipelineRunsSkipsNilID|TestCollectedDefinitionsGuardNameAndID' -v && go test -race ./... && go vet ./...</automated>
    <fails_when>non-zero exit, or output contains `panic:` or `no tests to run`, or the -v output lacks `--- PASS: TestCollectedDefinitionsGuardNameAndID`</fails_when>
  </verify>
  <acceptance_criteria>
    - All three tests in `internal/ado/nilguard_test.go` pass under `-race`.
    - Reverting either guard makes `TestCollectedDefinitionsGuardNameAndID` fail and name that file and line. Revert after confirming.
    - `git log --format= --name-only --grep='(05-02)'` lists only `internal/ado/pipelines.go`, `internal/ado/releases.go` and `internal/ado/nilguard_test.go` (plus the plan's own `.planning/` docs), and `git log --numstat --format= --grep='(05-02)' -- internal/ado/pipelines.go internal/ado/releases.go` shows exactly `1	1` for each file (D-13 scope). Both commands are scoped by commit message rather than by a revision range, because wave-1 plans commit in parallel.
    - `go test -race ./...` and `go vet ./...` exit 0, and `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>Both MED4 sites are guarded, and the shape that produced them cannot recur unnoticed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| ADO server → `GetPipelineRuns` / `GetReleases` | Definition records are server-supplied, and every SDK field is a pointer that may be nil |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-05-08 | Denial of Service | `*def.Id` dereference in both reads (MED4) | medium | mitigate | Combined `Name == nil \|\| Id == nil` guard at the collection point (D-13), with a silent `continue` (D-14). Pinned behaviourally by the two nil-`Id` tests and structurally by `TestCollectedDefinitionsGuardNameAndID` (exact count 2). |
| T-05-09 | Denial of Service | Other unguarded SDK dereferences not in MED4 | low | accept | D-13 rejects a repo-wide sweep because a flow-insensitive checker produces mostly false positives. The `handlePanic` recover in `cmd/adoBizInt/crash.go` turns any remaining panic into a structured outcome-unknown record. |
| T-05-SC | Tampering | npm/pip/cargo installs | high | mitigate | Not applicable. This plan installs nothing. |
</threat_model>

<verification>
- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- `go test ./internal/ado -run 'SkipsNilID|GuardNameAndID' -v` shows three passing tests.
</verification>

<success_criteria>
- ROADMAP Phase 5 SC3 holds: both MED4 nil-dereference sites are guarded.
- The 05-VALIDATION.md HARD-03 rows each have a green named test.
</success_criteria>

<output>
Create `.planning/phases/05-hardening/05-02-SUMMARY.md` when done
</output>
