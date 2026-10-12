---
phase: 06-core-value-cleanup
plan: 02
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/config/config.go
  - internal/config/config_test.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/main_test.go
autonomous: true
requirements: [TRUST-03, OUT-04]
coupling_justified: ["06-01: same Go package (cmd/adoBizInt) but disjoint files, and neither plan adds a package-level symbol the other declares; either execution order compiles and tests identically"]

estimate:
  tokens: 26000
  raw_tokens: 26000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "config.Validate returns *ConfigError with Field \"projects\" when Projects is nil or an empty slice, so no read can report complete:true over zero projects (D-10, TRUST-03, ROADMAP SC2 / audit W3) [edge OUT-04/empty]"
    - "config.Validate returns *ConfigError with Field \"projects\" when any entry is empty or whitespace-only, including a blank entry next to a valid one ([\"A\", \" \"]) (D-11) [edge OUT-04/empty]"
    - "Whitespace is judged by strings.TrimSpace, so space, tab and other Unicode whitespace (NBSP included) count as blank, while a name with interior spaces (\"My Project\") passes unchanged and a padded non-blank name is accepted as written, not rewritten (D-11) [edge OUT-04/encoding]"
    - "Every load path that can produce a blank or empty list is rejected after Load: a YAML sequence [\"\"] or [\"  \"], the legacy project: \"  \" key, a YAML scalar projects: \"  \", and ADO_PROJECTS set to \" , \" (D-10, D-11)"
    - "The projects check runs after the pat and org_url checks, so a config missing its PAT still reports Field \"pat\" first and the existing TestValidate rows keep their expected fields (D-10)"
    - "In run(), the cfg.Validate() check precedes both the CLI dispatch call and ui.NewModel, and its failure branch returns exitUnknown, so the CLI exits 3 (nothing ran) and the TUI refuses to start, both on one rule (D-10), pinned structurally with go/ast"
    - "The startup hint printed on a validation failure names ADO_PROJECTS alongside ADO_PAT and ADO_ORG_URL, so a projects error does not tell the user to set only variables they already set (D-10)"
  prohibitions:
    - "Must NOT trim, rewrite, de-duplicate or drop configured project names; Validate only rejects (D-11)"
    - "Must NOT add a config key, env var or flag to bypass the rule (D-10 applies to CLI and TUI alike)"
    - "Must NOT change Load's precedence between the YAML file, ADO_PROJECTS and the legacy project key"
  artifacts:
    - path: "internal/config/config.go"
      provides: "Validate rejects an empty projects list and any blank entry with ConfigError{Field: \"projects\"}"
      contains: "Field: \"projects\""
    - path: "internal/config/config_test.go"
      provides: "TestValidate projects rows and fixed base() fixture; TestLoadRejectsBlankProjects over every load path"
      contains: "TestLoadRejectsBlankProjects"
    - path: "cmd/adoBizInt/main.go"
      provides: "validation hint naming ADO_PROJECTS"
      contains: "ADO_PROJECTS"
    - path: "cmd/adoBizInt/main_test.go"
      provides: "TestValidateGuardsCLIAndTUI (go/ast over run); Projects added to the TestTimeoutOverridesConfiguredBudgetAfterValidation fixture"
      contains: "TestValidateGuardsCLIAndTUI"
  key_links:
    - from: "cmd/adoBizInt/main.go run"
      to: "internal/config/config.go Validate"
      via: "if err := cfg.Validate(); err != nil returns exitUnknown before dispatch and before ui.NewModel"
      pattern: "cfg\\.Validate\\(\\)"
    - from: "internal/config/config.go Validate"
      to: "ConfigError"
      via: "two checks after org_url: len(c.Projects) == 0, then strings.TrimSpace(p) == \"\" per entry"
      pattern: "strings\\.TrimSpace\\(p\\) == \"\""
---

<objective>
Close audit warning W3: a configuration with no projects, or with a blank project name, passes validation today. Every read then fans out over nothing and reports `complete: true` with no items, so a misconfiguration looks like an honest empty answer. The TUI shows only empty views, which is the same lie.

After this plan `config.Validate` rejects an empty list (D-10) and any blank or whitespace-only entry (D-11) as `ConfigError{Field: "projects"}`. `run()` already validates before both the CLI and the TUI branch, so both refuse with exit 3. A go/ast test pins that ordering, and the startup hint names `ADO_PROJECTS`.

Purpose: TRUST-03 (no `complete: true` over zero projects) and OUT-04 (a misconfiguration exits 3, not 0).
Output: two checks in `Validate`, tests over `Validate` and every `Load` path, a structural test of `run()`, and a corrected hint.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/06-core-value-cleanup/06-CONTEXT.md
@.planning/phases/06-core-value-cleanup/06-RESEARCH.md
@internal/config/config.go
@cmd/adoBizInt/main.go
</context>

<interfaces>
These already exist. Use them as they are and do not redefine them.

- `func (c *Config) Validate() error` in internal/config/config.go:171-184. It checks pat, then org_url, then fetch_timeout against poll_interval.
- `type ConfigError struct { Field, Message string }` in config.go:186-193. Error() renders "Field: Message", and run() prints it as "Invalid config: projects: ...".
- `type StringSlice []string` in config.go:15-38. A YAML sequence decodes verbatim with no trim; a YAML scalar is split on commas, trimmed, and empty parts dropped.
- `func Load() (*Config, error)` in config.go:103-132. ADO_PROJECTS (lines 156-164) trims and drops empty parts. The legacy `project` key becomes `Projects = []string{cfg.Project}` when Projects is empty (lines 121-124).
- `writeConfig(t, body)` in config_test.go:18-32. It points HOME at a temp dir and writes config.yaml there.
- `run()` in cmd/adoBizInt/main.go:65-150. cfg.Validate() is at line 91, dispatch(...) at 135 inside `if len(os.Args) > 1`, and ui.NewModel(...) at 138. The hint lines are 94-95.
- `internal/ado/nilguard_test.go:139-159` parsePackageFiles is the repo's go/ast parse analog. It lives in another package and cannot be imported; copy only its parser.ParseFile call, inline.
</interfaces>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): an empty projects list is a config error, and run() refuses it for the CLI and the TUI alike (W3, D-10)</name>
  <reversibility rating="reversible">D-10 tightens a validation rule inside one function; undoing it is deleting one check, and no published output shape changes (exit 3 for a config error already exists).</reversibility>
  <files>internal/config/config.go, internal/config/config_test.go, cmd/adoBizInt/main.go, cmd/adoBizInt/main_test.go</files>
  <read_first>
    - internal/config/config.go: the whole file (193 lines)
    - internal/config/config_test.go: the whole file (205 lines). Note the base() fixture at 97-104 and the row shape at 106-138
    - cmd/adoBizInt/main.go: the whole file (150 lines). Note the hint at 91-97, dispatch at 135 and ui.NewModel at 138
    - cmd/adoBizInt/main_test.go lines 728-770: TestTimeoutOverridesConfiguredBudgetAfterValidation, whose fixture at 735-740 has no Projects and breaks the moment the rule lands (RESEARCH Pitfall 5)
    - internal/ado/nilguard_test.go lines 136-159: the parser.ParseFile analog
    - .planning/phases/06-core-value-cleanup/06-RESEARCH.md: Pattern 2 and Pitfall 5
  </read_first>
  <behavior>
    TestValidate in config_test.go:
    - base() gains Projects: StringSlice{"P"}, so the "valid config" row stays valid.
    - New rows, each with wantField "projects":
      - "nil projects": the mutate sets Projects to nil
      - "empty projects slice": the mutate sets Projects to StringSlice{}
    - Existing rows keep their fields. A new row "missing pat with no projects" sets PAT "" and Projects nil and expects "pat", which pins the check order.

    New TestValidateGuardsCLIAndTUI in main_test.go, a go/ast test:
    - Parse main.go with parser.ParseFile, find the FuncDecl named "run", and walk its Body.List.
    - Find the index of the statement whose subtree contains a call to a selector named Validate (the IfStmt guarding cfg.Validate()).
    - Find the index of the statement containing a call to the identifier "dispatch".
    - Find the index of the statement containing a call to the selector ui.NewModel.
    - Each of the three must be found exactly once (non-vacuity), and the Validate index must be lower than both of the others.
    - The Validate IfStmt's body must contain a ReturnStmt whose result is the identifier exitUnknown.
    - That body must contain a string BasicLit whose value contains "ADO_PROJECTS". It is an AST literal, so a comment cannot satisfy it.

    TestTimeoutOverridesConfiguredBudgetAfterValidation: its cfg fixture gains Projects: config.StringSlice{"P"}. No assertion changes.

    RED:
    - The two new TestValidate rows fail, because Validate returns nil today.
    - TestValidateGuardsCLIAndTUI fails on the ADO_PROJECTS literal.
    - The ordering assertions already hold. They are pins.
  </behavior>
  <action>
    RED first, per D-10:
    1. Add the TestValidate rows and the base() Projects field as the behavior block states.
    2. Add TestValidateGuardsCLIAndTUI to main_test.go, importing go/ast, go/parser and go/token.
       - The parse is inline: one parser.ParseFile call on "main.go" with parser.SkipObjectResolution. No helper is extracted, because this is the first use in package main.
       - Its doc comment cites D-10: one Validate covers the CLI and the TUI, because run() calls it before both branches. It also states that a comment can neither satisfy nor defeat an AST assertion (STATE.md F-03-3 precedent).
    3. Add Projects to the TestTimeoutOverridesConfiguredBudgetAfterValidation fixture in the same commit, or that test fails for an unrelated reason.
    4. Record the RED output in the SUMMARY.

    GREEN, in internal/config/config.go Validate, right after the org_url check:
    - When len(c.Projects) == 0, return &ConfigError{Field: "projects", Message: "at least one project is required"}.
    - Comment: D-10. An empty list makes every read fan out over nothing and report complete:true. It applies to the CLI and the TUI, because run() validates before both.

    In cmd/adoBizInt/main.go:
    - Change the hint at line 94 to name ADO_PAT, ADO_ORG_URL and ADO_PROJECTS.
    - Keep line 95, the config-file hint, as it is.
    - Keep `return exitUnknown`.
    - Change nothing else in run(), guard or initLogging. Plan 06-05 edits guard and the run doc comment.

    Leave these untouched:
    - Load, loadFromFile, loadFromEnv and StringSlice
    - the fetch_timeout rule
    - go.mod
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./internal/config -run '^TestValidate$' && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestValidateGuardsCLIAndTUI|TestTimeoutOverridesConfiguredBudgetAfterValidation)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in either output, fewer than 8 "--- PASS: TestValidate/" subtest lines, or no "--- PASS: TestValidateGuardsCLIAndTUI" line</fails_when>
  </verify>
  <acceptance_criteria>
    - TestValidate passes all 8 rows: the 5 existing rows, nil projects, the empty slice, and missing pat with no projects. The last reports Field "pat", which proves the order.
    - TestValidateGuardsCLIAndTUI passes:
      - Validate, dispatch and ui.NewModel are each found exactly once in run
      - Validate precedes both
      - its failure branch returns exitUnknown and carries a string literal containing ADO_PROJECTS
    - TestTimeoutOverridesConfiguredBudgetAfterValidation passes.
    - The SUMMARY records RED evidence.
  </acceptance_criteria>
  <done>A config with no projects is rejected at startup with exit 3 for the CLI and the TUI alike, and the hint tells the operator which variable to set.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: a blank or whitespace-only project name is a config error on every load path (D-11)</name>
  <files>internal/config/config.go, internal/config/config_test.go</files>
  <read_first>
    - internal/config/config.go: as left by Task 1. Note StringSlice.UnmarshalYAML at 17-38, the legacy project fallback at 121-124 and loadFromEnv at 149-169
    - internal/config/config_test.go: as left by Task 1. Note writeConfig at 18-32 and TestLoadParsesTimeoutDurationsFromYAML at 81-94 for the Load call shape
  </read_first>
  <behavior>
    TestValidate gains these rows (wantField "projects" unless stated):
    - "blank project entry": Projects = StringSlice{""}
    - "whitespace project entry": Projects = StringSlice{" \t "}
    - "non-breaking-space project entry": Projects = StringSlice{" "}
    - "blank entry beside a valid one": Projects = StringSlice{"A", " "}
    - "interior space is a valid name": Projects = StringSlice{"My Project"}, wantField ""
    - "padded name is accepted as written": Projects = StringSlice{" P "}, wantField ""

    New TestLoadRejectsBlankProjects, a table over Load. Every row:
    - calls writeConfig(t, body), with a body that always sets pat: fixture-pat and org_url: https://dev.azure.com/fixture
    - sets t.Setenv("ADO_PAT", ""), t.Setenv("ADO_ORG_URL", "") and t.Setenv("ADO_PROJECT", "")
    - sets ADO_PROJECTS to "" unless the row says otherwise
    - calls Load (it must succeed), then Validate, and expects *ConfigError with Field "projects"

    Rows:
    1. "yaml sequence with an empty string": projects: [""]
    2. "yaml sequence with spaces": projects: ["  "]
    3. "yaml sequence with a blank beside a valid name": projects: ["A", " "]
    4. "legacy project key of spaces": project: "  "
    5. "yaml scalar of spaces": projects: "  " (the scalar path drops it, which leaves the list empty)
    6. "env list of separators only": no projects key, and ADO_PROJECTS set to " , "
    7. "no projects anywhere": no projects key and no project key

    A final row "valid yaml sequence", projects: ["A", "B"], expects Validate to return nil. This proves the fixture is not vacuous.

    RED against Task 1:
    - Rows 1-4 pass Validate today, because the length check does not see blanks.
    - Rows 5-7 already fail through Task 1's length check and act as pins.
  </behavior>
  <action>
    RED first, per D-11:
    - Add the TestValidate rows and TestLoadRejectsBlankProjects as the behavior block states.
    - Give TestLoadRejectsBlankProjects a doc comment. It says the YAML sequence and the legacy key do not trim, so D-11's blank check is the only guard on those paths. The env and scalar paths already trim and drop, which turns their blanks into an empty list.
    - Reuse writeConfig. Add no helper.
    - Record the RED output in the SUMMARY.

    GREEN, in internal/config/config.go Validate, directly after Task 1's length check:
    - Loop over c.Projects. When strings.TrimSpace(p) is empty, return &ConfigError{Field: "projects", Message: "project names must not be blank"}.
    - strings is already imported.
    - Comment: D-11. A blank name would reach the read as a project with no name. Names are validated, never trimmed or rewritten, so what the operator configured is what the read uses.

    Leave these untouched:
    - Load, loadFromFile, loadFromEnv and StringSlice
    - cmd/adoBizInt, which Task 1 finished
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./internal/config -run '^(TestValidate|TestLoadRejectsBlankProjects|TestLoadAppliesTimeoutDefaults|TestLoadParsesTimeoutDurationsFromYAML|TestLoadedConfigNeverRendersPAT)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 5 top-level "--- PASS: Test" lines, fewer than 14 "--- PASS: TestValidate/" subtest lines, or fewer than 8 "--- PASS: TestLoadRejectsBlankProjects/" subtest lines</fails_when>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"</automated>
    <fails_when>non-zero exit from any stage, any "FAIL" package line, or gofmt listing a file</fails_when>
  </verify>
  <acceptance_criteria>
    - TestValidate passes all 14 rows:
      - blank, whitespace, NBSP and blank-beside-valid each report Field "projects"
      - an interior space and a padded name both validate
    - TestLoadRejectsBlankProjects passes all 8 rows:
      - seven load paths end in Field "projects"
      - the valid sequence validates
    - The existing config tests pass unchanged.
    - The SUMMARY records RED evidence for rows 1-4.
    - The full module gate exits 0.
  </acceptance_criteria>
  <done>No load path can produce a projects list that validates while being empty or containing a blank name. The names that do validate reach the read exactly as configured.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| config.yaml and ADO_* environment to config.Load | Operator-controlled input decides which projects every read fans out over. |
| config.Validate to run() CLI and TUI branches | The single gate both entry points pass through before any Azure DevOps call. |
| CLI exit code and stderr to the calling agent | An agent treats exit 0 with complete:true as the whole answer. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-06-06 | Spoofing (result integrity) | config.Validate with an empty or blank projects list (audit W3) | medium | mitigate | Validate fails closed with ConfigError{Field: "projects"} on a nil or empty list and on any TrimSpace-blank entry. TestValidate and TestLoadRejectsBlankProjects cover every load path, and run() returns exitUnknown (3) before any read, so a misconfiguration can no longer surface as complete:true over nothing. |
| T-06-07 | Elevation of Privilege (bypass) | TUI entry path skipping validation | medium | mitigate | TestValidateGuardsCLIAndTUI asserts with go/ast that the Validate statement precedes both the dispatch call and ui.NewModel in run(), and that its failure branch returns exitUnknown. A later reorder fails the test, and a comment cannot satisfy it. |
| T-06-08 | Information Disclosure | ConfigError text on stderr and in the log | low | accept | The message names only the field and a fixed reason. It never echoes a project value or the PAT. The existing Config LogValue/String redaction is unchanged, and TestLoadedConfigNeverRendersPAT stays green. |
| T-06-SC | Tampering | dependency installs | low | accept | Not applicable: no package install, and go.mod is untouched. |
</threat_model>

<verification>
- Module gate (proven command): `cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`. It fails on a non-zero exit, a "FAIL" package line, or gofmt listing a file.
- 06-VALIDATION.md W3 rows are green: `go test ./internal/config -run 'TestValidate|TestLoadRejectsBlankProjects' -count=1` and `go test ./cmd/adoBizInt -run TestValidateGuardsCLIAndTUI -count=1`.
- Manual-only row, a human step for /gsd-verify-work that does not block this plan: with `ADO_PROJECTS=` and no projects in config.yaml, `./adoBizInt` (TUI) and `./adoBizInt prs list --json` both print "Invalid config: projects: ..." and exit 3.
</verification>

<success_criteria>
- ROADMAP SC2 holds: a configuration with no projects, or a blank project name, is rejected at startup with exit 3, so no read can report complete:true over zero projects (D-10, D-11).
- The TUI refuses to start on the same rule (D-10).
- No change to Load precedence, project-name values, or any read's behaviour on a valid config.
</success_criteria>

## Artifacts this phase produces

| Artifact | Kind | Purpose |
|----------|------|---------|
| `internal/config/config.go` | modified | Two `projects` checks in `Validate` (D-10, D-11) |
| `internal/config/config_test.go` | modified | `TestValidate` rows, `base()` fixture, `TestLoadRejectsBlankProjects` |
| `cmd/adoBizInt/main.go` | modified | Validation hint names `ADO_PROJECTS` |
| `cmd/adoBizInt/main_test.go` | modified | `TestValidateGuardsCLIAndTUI`; Projects added to the timeout fixture |
| `.planning/phases/06-core-value-cleanup/06-02-SUMMARY.md` | new | Execution record with RED evidence |

<output>
Create `.planning/phases/06-core-value-cleanup/06-02-SUMMARY.md` when done. Record the two Validate checks and their order, the load paths covered, the structural test, and the RED-then-GREEN evidence.
</output>
