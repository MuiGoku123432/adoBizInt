---
phase: 06-core-value-cleanup
plan: 02
subsystem: config
tags: [go, config, validation, go-ast, exit-codes]
status: complete

requires:
  - phase: 03
    provides: exit-code contract (exitUnknown = 3 for startup failures)
  - phase: 05
    provides: configuredProjects allow-list captured after Validate (D-09)
provides:
  - config.Validate rejects an empty projects list (D-10) and any blank or whitespace-only entry (D-11) as ConfigError{Field: "projects"}
  - go/ast pin that cfg.Validate() precedes both dispatch and ui.NewModel in run(), returns exitUnknown, and names ADO_PROJECTS
affects: [06-03, 06-04, 06-05, config, cli startup, tui startup]

actuals:
  tokens: 2600
  tasks: 2
  commits: 4
plan_head_before: a54708e02b9fd2904ed9a9ea5a2e169bb7bb0044

tech-stack:
  added: []
  patterns:
    - "Structural run() test in package main: one inline parser.ParseFile on main.go, top-level statement index per call, exactly-once non-vacuity"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/adoBizInt/main.go
    - cmd/adoBizInt/main_test.go

key-decisions:
  - "Validate checks projects after pat and org_url and before fetch_timeout: length first (D-10), then strings.TrimSpace per entry (D-11)"
  - "Project names are only rejected, never trimmed or rewritten; a padded non-blank name validates as written"

patterns-established:
  - "Startup-gate ordering in run() is pinned by go/ast, so a reorder or a comment cannot pass it"

requirements-completed: [TRUST-03, OUT-04]

duration: 3min
completed: 2026-10-12
---

# Phase 6 Plan 02: Reject Empty Projects Summary

**`config.Validate` now fails closed with `ConfigError{Field: "projects"}` on an empty projects list or any TrimSpace-blank name. `run()` validates before both the CLI dispatch and `ui.NewModel`, so both paths exit 3 instead of reporting `complete: true` over zero projects. A go/ast test pins that ordering.**

## Performance

- **Duration:** about 3 min
- **Started:** 2026-10-12T21:46:30Z
- **Completed:** 2026-10-12T21:49:30Z
- **Tasks:** 2 (1 tracer, 1 auto, both TDD)
- **Files modified:** 4

## Accomplishments

- Validate order is now pat, org_url, projects-length (D-10), projects-blank (D-11), fetch_timeout. A config with no PAT still reports `pat` first.
- `TestLoadRejectsBlankProjects` runs every load path through Load and then Validate: YAML sequence `[""]`, `["  "]` and `["A", " "]`; legacy `project: "  "`; YAML scalar `projects: "  "`; `ADO_PROJECTS=" , "`; nothing configured. A valid `["A", "B"]` control row shows the fixture is not vacuous.
- `TestValidateGuardsCLIAndTUI` parses main.go and checks four things: `Validate`, `dispatch` and `ui.NewModel` each appear in exactly one top-level statement of `run`, Validate comes before both, its IfStmt body returns `exitUnknown`, and that body has a string literal containing `ADO_PROJECTS`.
- The startup hint now reads "Please set ADO_PAT, ADO_ORG_URL and ADO_PROJECTS environment variables".

## Task Commits

1. **Task 1 (tracer) RED:** `4893d19` test(06-02): add failing tests for an empty projects list and the run() validate guard (D-10)
2. **Task 1 (tracer) GREEN:** `7d1f9e4` feat(06-02): reject an empty projects list at validation, for the CLI and TUI alike (D-10)
3. **Task 2 RED:** `78339fb` test(06-02): add failing tests for blank project names on every load path (D-11)
4. **Task 2 GREEN:** `929e9e5` feat(06-02): reject blank or whitespace-only project names at validation (D-11)

The tracer feedback gate (interactive, end-of-phase, automated-only verify) re-ran Task 1's `<verify>` plus the full suite. Both were green before expanding to Task 2.

## RED → GREEN Evidence

**Task 1 RED** (before the length check and the hint change):
```
=== RUN   TestValidate/nil_projects
    config_test.go:174: Validate() error = <nil> (<nil>), want *ConfigError
=== RUN   TestValidate/empty_projects_slice
    config_test.go:174: Validate() error = <nil> (<nil>), want *ConfigError
--- FAIL: TestValidate (0.00s)
--- PASS: TestTimeoutOverridesConfiguredBudgetAfterValidation (0.00s)
    main_test.go:869: the Validate failure hint does not name ADO_PROJECTS
--- FAIL: TestValidateGuardsCLIAndTUI (0.00s)
```
The ordering assertions passed in RED, as the plan expected, so they act as pins. "missing pat with no projects" passed as well, confirming the check order.

**Task 1 GREEN:** 8/8 `TestValidate/` subtests pass. `TestValidateGuardsCLIAndTUI` and `TestTimeoutOverridesConfiguredBudgetAfterValidation` pass.

**Task 2 RED** (after Task 1, before the TrimSpace loop):
```
--- FAIL: TestValidate/blank_project_entry
--- FAIL: TestValidate/whitespace_project_entry
--- FAIL: TestValidate/non-breaking-space_project_entry
--- FAIL: TestValidate/blank_entry_beside_a_valid_one
--- PASS: TestValidate/interior_space_is_a_valid_name
--- PASS: TestValidate/padded_name_is_accepted_as_written
--- FAIL: TestLoadRejectsBlankProjects/yaml_sequence_with_an_empty_string        Projects = [""]
--- FAIL: TestLoadRejectsBlankProjects/yaml_sequence_with_spaces                 Projects = ["  "]
--- FAIL: TestLoadRejectsBlankProjects/yaml_sequence_with_a_blank_beside_a_valid_name  Projects = ["A" " "]
--- FAIL: TestLoadRejectsBlankProjects/legacy_project_key_of_spaces             Projects = ["  "]
--- PASS: TestLoadRejectsBlankProjects/yaml_scalar_of_spaces
--- PASS: TestLoadRejectsBlankProjects/env_list_of_separators_only
--- PASS: TestLoadRejectsBlankProjects/no_projects_anywhere
--- PASS: TestLoadRejectsBlankProjects/valid_yaml_sequence
```
Rows 1-4 failed, as predicted. Rows 5-7 were already rejected by D-10's length check and act as pins.

**Task 2 GREEN:** 5 top-level tests pass, along with 14/14 `TestValidate/` and 8/8 `TestLoadRejectsBlankProjects/` subtests. Full gate `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` exits 0.

## Files Created/Modified

- `internal/config/config.go`: two projects checks in Validate after org_url, each with a D-10/D-11 comment
- `internal/config/config_test.go`: `base()` gains `Projects: StringSlice{"P"}`, 9 new TestValidate rows, and new `TestLoadRejectsBlankProjects`
- `cmd/adoBizInt/main.go`: the validation hint names ADO_PROJECTS (one line)
- `cmd/adoBizInt/main_test.go`: new `TestValidateGuardsCLIAndTUI` (go/ast, go/parser, go/token imports), and Projects added to the timeout-ordering fixture

## Decisions Made

- The NBSP row uses the `" "` escape rather than a literal non-breaking space, so a reviewer can see the character and an editor cannot silently normalise it. The behaviour under test is unchanged.
- The test isolates every Load row by blanking ADO_PAT, ADO_ORG_URL and ADO_PROJECT, setting ADO_PROJECTS per row, and pointing HOME at a temp dir through `writeConfig`. No test reads the real `~/.config/adoBizInt` or a real PAT.

## Deviations from Plan

None. The plan was executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None. Manual check for /gsd-verify-work (does not block this plan): with `ADO_PROJECTS=` and no projects in config.yaml, both `./adoBizInt` and `./adoBizInt prs list --json` should print "Invalid config: projects: ..." and exit 3.

## Next Phase Readiness

Plans 06-03, 06-04 and 06-05 can rely on a validated config always having at least one non-blank project. Plan 06-05 still owns `guard` and the `run` doc comment, which this plan did not touch.

## Self-Check: PASSED

- FOUND: internal/config/config.go, internal/config/config_test.go, cmd/adoBizInt/main.go, cmd/adoBizInt/main_test.go
- FOUND commits: 4893d19, 7d1f9e4, 78339fb, 929e9e5
- Measured `git rev-list --count a54708e..HEAD` = 4
