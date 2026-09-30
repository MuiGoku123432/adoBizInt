---
phase: 05-hardening
plan: 04
subsystem: cli
tags: [go, flag, allow-list, fail-closed, exit-codes, wiql]

requires:
  - phase: 03
    provides: "newFlagSet as the single flag registration site (D-20), the --project stringList, and runRead's parse-error -> exitUnknown mapping"
  - phase: 04
    provides: "runWrite's parse-error -> exitUnknown mapping, oneProject arity check, and the auditOpen-before-attempt ordering (D-13)"
provides:
  - "var configuredProjects []string in cmd/adoBizInt/flags.go: the D-09 allow-list; nil or empty rejects every supplied --project"
  - "--project registered with fs.Func; each part must exactly match a configured project, or the parse fails and the verb exits 3"
  - "run() assigns configuredProjects = []string(cfg.Projects) on the line after readBudget = cfg.FetchTimeout"
  - "withConfiguredProjects(t, names...) test helper in cmd/adoBizInt/main_test.go"
  - "TestUnknownProjectRejected: 11-row table in cmd/adoBizInt/filter_test.go"
affects: [05-05 wiql-project-macro, 05-VALIDATION HARD-04 rows, /gsd-secure-phase T-05-14..T-05-18]

actuals:
  tokens: 1940
  tasks: 2
  commits: 3
plan_head_before: 861e84c6bedb2bc0f21de505f63a48ffd6a9a5b8

tech-stack:
  added: []
  patterns:
    - "Validation inside a flag's fs.Func callback, so a rejection travels the existing parse-error path to exit 3 and every verb inherits it"
    - "Package-level policy var assigned in run() only after cfg.Validate, zero value fail-closed, restored in tests by a withX(t, ...) helper"

key-files:
  created: []
  modified:
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/main.go
    - cmd/adoBizInt/main_test.go
    - cmd/adoBizInt/filter_test.go
    - cmd/adoBizInt/usage_test.go
    - cmd/adoBizInt/write_test.go
    - cmd/adoBizInt/writeverbs_test.go
    - cmd/adoBizInt/audit_test.go

key-decisions:
  - "The allow-list check is the --project fs.Func callback in newFlagSet, not a helper called from runRead and runWrite. dispatch.go and write.go are unchanged"
  - "Matching is exact and case-sensitive via slices.Contains. A mutation to strings.EqualFold makes exactly the case-mismatch row fail"
  - "Task 1 was committed as a RED test commit followed by a GREEN feat commit, matching the 05-01 and 05-02 precedent. Only the target test was red at the RED commit"

patterns-established:
  - "An accepted-row assertion treats a nil captured slice as 'the read never ran', because emptyIfNil never passes nil. This stops an empty-slice expectation from passing vacuously"

requirements-completed: [HARD-04]

coverage:
  - id: D1
    description: "An unknown --project exits 3 with empty stdout and the value quoted on stderr, and no client method is called"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestUnknownProjectRejected/unknown_project"
        status: pass
    human_judgment: false
  - id: D2
    description: "Fail-closed across the invocation: P,bogus, bogus --project P, and bogus,P all exit 3"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestUnknownProjectRejected (three bogus-position rows)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Exact, case-sensitive match. p against configured P exits 3, and the row fails if matching is loosened to EqualFold"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestUnknownProjectRejected/case_mismatch_is_not_folded"
        status: pass
      - kind: other
        ref: "Mutation: slices.ContainsFunc + strings.EqualFold in the callback -> only case_mismatch_is_not_folded FAILs; reverted"
        status: pass
    human_judgment: false
  - id: D4
    description: "A nil configured list rejects every supplied value"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestUnknownProjectRejected/nothing_configured_rejects_everything"
        status: pass
    human_judgment: false
  - id: D5
    description: "Write verbs are rejected before auditOpen: set-state and prs approve with --project bogus exit 3, no write-client call, and no audit file created"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestUnknownProjectRejected/write_verb_set-state and /write_verb_prs_approve"
        status: pass
    human_judgment: false
  - id: D6
    description: "A configured project reaches the read as [P]. An empty or separator-only value contributes nothing, and the read gets an empty slice"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestUnknownProjectRejected (three accepted rows)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Every pre-existing cmd test still exercises its original path. The two-project arity rows configure P and Q, so they reach oneProject rather than failing at parse"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "go test -race ./cmd/adoBizInt/... (TestWriteParseErrorsExitUnknown, TestWriteGuardMatrix, TestProjectListAccumulates, TestFlagsRegisteredOnEverySubcommand)"
        status: pass
      - kind: other
        ref: "Throwaway probe (deleted, never committed): P,Q configured + set-state --project P --project Q -> exit 3, stderr 'workitems set-state requires exactly one --project value'"
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-09-30
status: complete
---

# Phase 05 Plan 04: Project Allow-List Summary

**Every `--project` value is now checked against `cfg.Projects` while flags are parsed. The check is an `fs.Func` callback in the single `newFlagSet` registration. An unknown, differently-cased, or unconfigured value fails the parse, and every read and write verb exits 3 before any client call or audit-sink open. An 11-row table test pins the fail-closed edges.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-09-30T17:16:18Z
- **Completed:** 2026-09-30T17:22:05Z
- **Tasks:** 2
- **Files modified:** 8 (2 production, 6 test)

## Accomplishments
- HARD-04 cmd policy layer (T-05-14). An agent-supplied `--project` can no longer reach a WIQL query or a write target unless it exactly matches a configured project. The rejection happens inside `fs.Parse`. `runRead` and `runWrite` already map a non-help parse error to `exitUnknown` (3), so neither function changed, and any future verb gets the check automatically.
- Fail-closed in every direction. The whole value is rejected on the first bad part, so no part is ever appended to `gf.projects`. The position of the bad value does not matter, and nil or empty `configuredProjects` rejects everything.
- Stderr names the bad part with `%q` and lists the configured names, or says none are configured. The flag package adds its own `invalid value "<raw>" for flag -project:` prefix and the verb usage. Stdout stays empty (OUT-03, T-05-17).
- An omitted, empty, or separator-only `--project` behaves as before. It contributes no value, and a read falls back to the configured list.

## Task Commits

1. **Task 1: An unknown --project exits 3 before any read or write is attempted (tracer)**: `00bcf68` (test, RED) and `f24c3fe` (feat, GREEN)
2. **Task 2: Pin the fail-closed edges**: `520ee3a` (test)

## TDD and Tracer Gates

- **RED:** At `00bcf68`, the single-row `TestUnknownProjectRejected` failed because the read ran: `readClient.GetWorkItems called with no GetWorkItemsFunc set`. It was the only failing test in the suite. `gsd-tools check tdd-red-evidence` returned `RED_EVIDENCE_OK` (`target_test_failed`) on a TAP record built from `go test -json`.
- **GREEN:** At `f24c3fe`, the target test and the whole `go test -race ./cmd/adoBizInt/...` suite pass.
- **Tracer:** Task 1 is a tracer. `human_verify_mode` was end-of-phase, auto mode was off, and the `<verify>` block is automated-only. I re-ran the verify end-to-end with `-count=1`, it passed, and I moved on to Task 2 without a checkpoint.

## Verification Results

| Check | Result |
|---|---|
| `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"` (`-count=1`) | exit 0, no FAIL lines |
| `go test ./cmd/adoBizInt -run TestUnknownProjectRejected -v` | 11 `--- PASS: TestUnknownProjectRejected/` subtests |
| `newFlagSet` registers `project` via `fs.Func`; `flag.NewFlagSet` appears only in `newFlagSet` | PASS (`flags.go:185`, `flags.go:198`) |
| `configuredProjects` is assigned on the line after `readBudget = cfg.FetchTimeout` | PASS (`main.go:105-106`) |
| `git log --format= --name-only --grep='(05-04)'` lists neither `dispatch.go` nor `write.go` | PASS |
| The diff to the five pre-existing test files adds only `withConfiguredProjects(` lines | PASS (27 added lines, no removals) |
| EqualFold mutation | only `case_mismatch_is_not_folded` fails; reverted, no diff left |

## Files Created/Modified
- `cmd/adoBizInt/flags.go`: `configuredProjects` var with a D-09 doc comment, and the validating `fs.Func("project", ...)` registration. Adds the `fmt` and `slices` imports.
- `cmd/adoBizInt/main.go`: `configuredProjects = []string(cfg.Projects)` after `readBudget`.
- `cmd/adoBizInt/main_test.go`: `withConfiguredProjects`, placed below `withReadBudget` and built the same way.
- `cmd/adoBizInt/filter_test.go`: `TestUnknownProjectRejected` table (8 rejected rows, 3 accepted rows). `TestFilterFlagsProjectNarrowsTheRead` configures `one`.
- `cmd/adoBizInt/write_test.go`: 11 tests configure `P`. `TestWriteParseErrorsExitUnknown` and `TestWriteGuardMatrix` configure `P` and `Q`.
- `cmd/adoBizInt/writeverbs_test.go`: 5 tests configure `P`.
- `cmd/adoBizInt/audit_test.go`: 6 tests configure `P`.
- `cmd/adoBizInt/usage_test.go`: `TestFlagsRegisteredOnEverySubcommand` and `TestProjectListAccumulates` configure `alpha`, `beta` and `gamma`.

## Decisions Made
- The callback splits its input with the existing `stringList.Set`, so the comma and whitespace rules are still defined in one place. `Set` never returns an error, so its result is discarded explicitly with a comment rather than checked by a dead branch.
- Some tests exited 3 before this plan and would keep passing even if parsing now failed first: `TestWriteParseErrorsExitUnknown`, `TestWriteRepoArityIsExactlyOne`, `TestUnwritableAuditSinkPreventsMutation`, `TestDryRunAndNoOpAppendNoAuditRecord` and `TestFailedPreReadEmitsNoDryRunReport`. I gave them the helper too. I chose the target tests by scanning for `--project`, `setStateArgs(` and `writeGuardFixtures()`, not just by looking at which tests failed. That keeps each test on the path it was written for.

## Deviations from Plan

Minor additions within the plan's own truths. No auto-fix rules were triggered.

- **Separator-only row added.** Truth 7 covers "empty or only separators". The `<behavior>` list had only the `""` row, so I added a `" , "` row. That makes 11 subtests rather than the minimum 10.
- **Non-vacuity guard on accepted rows.** `slices.Equal(nil, []string{})` is true, so the empty-value rows would have passed even if the read never ran. The assertion now also fails when the captured slice is nil. This is safe because `emptyIfNil` never passes nil.
- **Audit not-exist asserted on every rejected row**, not only the set-state row. This is a stronger form of the same check.

**Total deviations:** 0 auto-fixed. **Impact:** none on scope. The additions only tighten assertions.

## Issues Encountered
- zsh does not word-split an unquoted `$CMD`, so the first RED-evidence capture ran nothing. The checker returned `INVALID_RED zero_tests_discovered`. I re-ran it with a literal command, and it returned `RED_EVIDENCE_OK`.
- Flagged, out of scope: a duplicated valid value such as `--project P,P` passes validation, and both copies reach the read. De-duplication was already missing before this plan (edge-probe adjacency row).

## Threat Flags

None. The only new surface is the rejection message on stderr. It quotes the rejected part with `%q` (T-05-16) and lists the operator's configured project names (T-05-18, accepted).

## User Setup Required

None. No external service configuration required.

## Next Phase Readiness
- Ready for 05-05 (wiql-project-macro), the library layer of HARD-04. HARD-04 is shared with 05-05, so it should read Complete only after 05-05 lands.
- No human-check items for this plan.

## Self-Check: PASSED
- All 8 modified files exist on disk.
- Commits `00bcf68`, `f24c3fe` and `520ee3a` are present in `git log`.
- `git rev-list --count 861e84c..HEAD` = 3 before the SUMMARY commit.

---
*Phase: 05-hardening*
*Completed: 2026-09-30*
