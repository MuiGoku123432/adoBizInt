---
phase: 05-hardening
plan: 02
subsystem: api
tags: [robustness, nil-guard, azure-devops-sdk, go-ast, denial-of-service]

requires:
  - phase: 02
    provides: Result envelope with Complete/Failures and the D-07 suppress-not-name precedent for non-call anomalies
provides:
  - "GetPipelineRuns skips a matching build definition with a nil Id (silent continue)"
  - "GetReleases skips a matching release definition with a nil Id (silent continue)"
  - "TestCollectedDefinitionsGuardNameAndID: go/ast pin, exact count 2, on collect-then-dereference-Id loops"
affects: [05-VALIDATION HARD-03 rows, /gsd-secure-phase T-05-08]

actuals:
  tokens: 2374
  tasks: 2
  commits: 5
plan_head_before: 39e457c78915e07713b3a5d15cad8d1831eed023

tech-stack:
  added: []
  patterns:
    - "Guard at the collection point: filter malformed SDK records where they are appended, so later dereferences need no guard"
    - "Structural pin with an exact count rather than a floor, so a new instance of the shape forces review"

key-files:
  created:
    - internal/ado/nilguard_test.go
  modified:
    - internal/ado/pipelines.go
    - internal/ado/releases.go

key-decisions:
  - "Structural selector matches an append argument after the first (not only the second), so it also covers variadic appends; it still finds exactly the two MED4 loops"
  - "Glob-and-parse walk left duplicated with TestNoRawHTTPInADO under the rule of three, marked with a // note: comment"

patterns-established:
  - "Collect-then-dereference-Id loops open with `if v.Name == nil || v.Id == nil { continue }`, pinned by go/ast"

requirements-completed: [HARD-03]

coverage:
  - id: D1
    description: "GetPipelineRuns no longer panics on a matching build definition with a nil Id; returns one row for the well-formed definition, Complete, no Failure"
    requirement: HARD-03
    verification:
      - kind: unit
        ref: "internal/ado/nilguard_test.go#TestGetPipelineRunsSkipsNilID"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineRunsHappyPathRowCountUnchanged"
        status: pass
    human_judgment: false
  - id: D2
    description: "GetReleases no longer panics on a matching release definition with a nil Id; returns one row for the well-formed definition, Complete, no Failure"
    requirement: HARD-03
    verification:
      - kind: unit
        ref: "internal/ado/nilguard_test.go#TestGetReleasesSkipsNilID"
        status: pass
    human_judgment: false
  - id: D3
    description: "Structural pin: exactly two collecting loops in internal/ado, both guarding Name and Id; reverting either guard fails with file:line"
    requirement: HARD-03
    verification:
      - kind: unit
        ref: "internal/ado/nilguard_test.go#TestCollectedDefinitionsGuardNameAndID"
        status: pass
      - kind: other
        ref: "go test -race ./... && go vet ./... && test -z \"$(gofmt -l .)\" && go build ./..."
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-09-30
status: complete
---

# Phase 05 Plan 02: Nil Guards Summary

**MED4 is closed. `GetPipelineRuns` and `GetReleases` now skip any definition the server returned without an `Id`. Each fix widens the existing collection guard by one clause, with a silent `continue`. A go/ast test pins both guards and fails if a third collect-then-dereference loop appears.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-09-30T17:06:00Z
- **Completed:** 2026-09-30T17:09:51Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- A malformed build definition (non-nil `Name`, nil `Id`) no longer panics the pipeline read. Before, it crashed the whole CLI read as outcome-unknown and cost the TUI its tab.
- The same fix is applied to release definitions. The one guard covers both `append` branches in `GetReleases`.
- `TestCollectedDefinitionsGuardNameAndID` walks every non-test file in `internal/ado` and finds exactly two collecting loops, both guarded. When either guard is reverted, the test names the file and line (`pipelines.go:156:3`, `releases.go:103:3`).

## TDD Cycle

- **RED (Task 1):** `TestGetPipelineRunsSkipsNilID` panicked at `pipelines.go:171` (`defID := *def.Id`). `check tdd-red-evidence` returned `RED_EVIDENCE_OK`.
- **GREEN (Task 1):** `if d.Name == nil {` became `if d.Name == nil || d.Id == nil {`. This test and `TestGetPipelineRunsHappyPathRowCountUnchanged` both pass.
- **Tracer gate:** interactive run, end-of-phase mode, automated-only `<verify>`. I re-ran the verify, it passed, and I moved on to Task 2.
- **RED (Task 2):** `TestGetReleasesSkipsNilID` panicked at `releases.go:125`. `TestCollectedDefinitionsGuardNameAndID` flagged `releases.go:103:3` and found a count of exactly 2. Both returned `RED_EVIDENCE_OK`.
- **GREEN (Task 2):** `if rd.Name == nil {` became `if rd.Name == nil || rd.Id == nil {`. All three tests pass.
- **REFACTOR:** none to production code. One follow-up test commit adds a `// note:` comment for the duplicated file walk.

## Task Commits

1. **Task 1: A nil-Id build definition no longer panics GetPipelineRuns**
   - `2fcd14b` (test, RED)
   - `4a98caa` (fix, GREEN)
2. **Task 2: Guard GetReleases and pin the collection-guard shape structurally**
   - `676b581` (test, RED)
   - `845183b` (fix, GREEN)
   - `bcd3326` (test, rule-of-three note)

## Files Created/Modified
- `internal/ado/nilguard_test.go`: two behavioural nil-`Id` tests and the go/ast structural pin, plus three private AST helpers (`appendsIdent`, `derefsIDAfter`, `guardsNameAndID`).
- `internal/ado/pipelines.go`: collection guard in `GetPipelineRuns` (1 line changed).
- `internal/ado/releases.go`: collection guard in `GetReleases` (1 line changed).

## Decisions Made
- **What counts as a "collecting loop".** It is a range loop whose value identifier appears as any bare-ident argument after the first in an `append` call, inside a function with a `*X.Id` dereference after the loop. This matches the plan's `<behavior>` definition. It still finds exactly the two MED4 loops. `GetPipelineDefinitions` and `branches.go:84` do not match, as the plan requires.
- **The count is exact.** The test requires exactly 2 collecting loops, not "at least 2" (D-13).
- **Scope held to D-13.** No other dereference in `internal/ado` was touched. `handlePanic` stays the backstop.

## Deviations from Plan

None. The plan was executed as written.

The only addition is the `// note:` comment in commit `bcd3326`. The user's engineering standards require it when the same code appears twice. It is a comment only, and it stays inside a file the plan already lists.

## Issues Encountered
- A Go test binary aborts when a test panics. In the Task 2 RED phase, the releases panic stopped the structural test from running in the same invocation, so I ran each RED test on its own to record its evidence.
- `check tdd-red-evidence` only parses node TAP output. I converted the `go test -json` pass/fail events one-to-one into TAP (`ok`/`not ok` lines plus `# tests/pass/fail` counts) and gave that to the checker. The conversion keeps every test result as it was.

## Scope Verification (acceptance criteria)
- `git log --format= --name-only --grep='(05-02)'` lists only `internal/ado/nilguard_test.go`, `internal/ado/pipelines.go` and `internal/ado/releases.go`. PASS.
- `git log --numstat --format= --grep='(05-02)' -- internal/ado/pipelines.go internal/ado/releases.go` shows `1	1` for each file. PASS.
- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"` exits 0. PASS.
- `go test ./internal/ado -run 'SkipsNilID|GuardNameAndID' -v` shows 3 passing tests. PASS.

## User Setup Required

None. No external service configuration is required.

## Next Phase Readiness
- The HARD-03 rows in 05-VALIDATION.md each have a passing named test.
- ROADMAP Phase 5 SC3 holds. Plans 05-03 through 05-06 are not affected.
- No human-check items.

---
*Phase: 05-hardening*
*Completed: 2026-09-30*

## Self-Check: PASSED
- FOUND: internal/ado/nilguard_test.go, internal/ado/pipelines.go, internal/ado/releases.go
- FOUND commits: 2fcd14b, 4a98caa, 676b581, 845183b, bcd3326
