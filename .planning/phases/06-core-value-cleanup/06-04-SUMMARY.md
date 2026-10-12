---
phase: 06-core-value-cleanup
plan: 04
subsystem: cli
tags: [go, cli, help, stderr, scope, tdd]
status: complete

requires:
  - phase: 06-03
    provides: ado.Scope and Result.Scope on the four scoped reads
  - phase: 06-01
    provides: verb help printed from a fresh newFlagSet through usageVerb
provides:
  - renderScope writes one scope line to stderr in human mode, including for zero-row reads
  - a Scope: paragraph in --help for workitems list, dashboard counts, prs list and releases approvals
  - --hide-done help that states the server-side exclusions and the Resolved addition
affects: [06-05, human CLI users, agents reading --help]

actuals:
  tokens: 3533
  tasks: 2
  commits: 4
plan_head_before: b7ce697dfe73af708c61ac9505078c1af18483d4

tech-stack:
  added: []
  patterns:
    - "Single-write, nothing-when-nil stderr renderer (same discipline as renderFailures)"
    - "Per-verb help paragraph map beside verbSummaries (verbScopes)"

key-files:
  created: []
  modified:
    - cmd/adoBizInt/render.go
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/main_test.go
    - cmd/adoBizInt/usage.go
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/usage_test.go

key-decisions:
  - "The scope line goes to stderr between the rows and the failure warnings, only in the human branch; the --json branch and emitJSON are untouched (D-04, Phase 3 D-11)"
  - "The help text gives the recency window as 30 days with the no-dated-iteration fallback, matching GetRecentIterations' cutoff (RESEARCH Pitfall 7)"

patterns-established:
  - "Every scope fact is disclosed in three places in matching terms: the envelope (06-03), a stderr line, and verb help"

requirements-completed: [CLI-06, CLI-05, TRUST-03]

duration: 4min
completed: 2026-10-12
---

# Phase 6 Plan 04: Scope Disclosure in the CLI Summary

**In human mode a scoped read now writes one `scope:` line to stderr, even when it returns zero rows. Each scoped verb's `--help` prints a `Scope:` paragraph, and the `--hide-done` help now says Done, Closed and Removed are already excluded server-side and the flag only adds Resolved.**

## Performance

- **Duration:** about 4 minutes
- **Completed:** 2026-10-12
- **Tasks:** 2 (1 tracer, 1 auto, both TDD)
- **Files modified:** 6

## Accomplishments

### Stderr scope line per verb (D-04)

| Verb | Stderr line |
|------|-------------|
| workitems list | `scope: excludes Done/Closed/Removed; recent iterations only` |
| dashboard counts | `scope: excludes Done/Closed/Removed; recent iterations only; active pull requests only` |
| prs list | `scope: active pull requests only` |
| releases approvals | `scope: pending approvals only` |
| other four reads (null scope) | nothing |

- The line goes to stderr only. It is written after the rows go to stdout and before any `warning:` lines.
- The exit code still comes only from failures and `Complete`.
- Under `--json`, stderr stays empty and the envelope's `scope` object is unchanged.

### Help paragraphs (D-06, D-15)

- **workitems list:** "Scope: Azure DevOps returns only work items whose state is not Done, Closed or Removed, in iterations that start or finish within the last 30 days; a project with no such dated iteration is not limited by iteration. --hide-done additionally drops Resolved items. Under --json the envelope's scope object states the same."
- **dashboard counts:** "Scope: work items are counted only when their state is not Done, Closed or Removed, in iterations that start or finish within the last 30 days (a project with no such dated iteration is not limited by iteration), and only active pull requests are counted. Under --json the envelope's scope object states the same."
- **prs list:** "Scope: Azure DevOps returns only active pull requests; completed and abandoned pull requests are not listed. Under --json the envelope's scope object states the same."
- **releases approvals:** "Scope: Azure DevOps returns only pending approvals. Under --json the envelope's scope object states the same."

The other ten verbs print no `Scope:` paragraph (D-16).

### --hide-done text (D-05)

- **Before:** "drop items in a done or resolved state (off by default)"
- **After:** "also drop Resolved items client-side; Done, Closed and Removed are always excluded by Azure DevOps (off by default)"

The flag's name, default and filtering behaviour are unchanged. `TestFilterFlagsPostFilterItems` still passes, which confirms this.

## Task Commits

1. **Task 1 (tracer): human-mode scope line**
   - `0f88e62` (test, RED)
   - `a8748ab` (feat, GREEN)
2. **Task 2: scoped-verb help and --hide-done text**
   - `f8a5655` (test, RED)
   - `d75e9ba` (feat, GREEN)

## TDD Evidence

### Task 1

**RED** (commit `0f88e62`):
- Rows 1-4 failed with `stderr = "", want "scope: ..."`.
- "scope precedes failures" failed with `stderr = "warning: alpha: get pull requests failed (timeout)\n"`.
- The null-scope row (branches list) and `TestScopeRidesInJSONEnvelopeOnly` passed. They serve as pins.

**GREEN** (commit `a8748ab`):
- `TestScopeLineOnStderrInHumanMode` passes, with all 6 subtests passing.
- `TestScopeRidesInJSONEnvelopeOnly` passes.
- These existing tests still pass unchanged: `TestRenderEmptyWritesNothing`, `TestRenderFailuresToStderrOnly`, `TestRenderNotInvokedUnderJSON`, `TestRenderRowsToStdout` and `TestRenderHasNoANSI`. That is 7 top-level PASS lines.

**Tracer gate:** this run is interactive and `human_verify_mode` is the default. The tracer's `<verify>` is automated only, and it passed on re-run, so there was no checkpoint.

### Task 2

**RED** (commit `f8a5655`):
- `TestScopedVerbHelpStatesScope` failed for workitems list, dashboard counts, prs list and releases approvals: the help did not contain "Scope:".
- The 10 unscoped verbs passed. They serve as pins.
- `TestHideDoneHelp` failed on the old string "drop items in a done or resolved state (off by default)".

**GREEN** (commit `d75e9ba`):
- All 8 targeted tests pass: `TestScopedVerbHelpStatesScope`, `TestHideDoneHelp`, `TestHelpIsDeterministic`, `TestHelpNeedsNoCredentials`, `TestFilterFlagsRegisteredOnSupportedVerbsOnly`, `TestFilterFlagsPostFilterItems`, `TestWriteVerbUsageDocumentsConfirm` and `TestHelpTokenInFlagPositionIsHelp`.

### Module gate

`go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` exits 0 with no FAIL lines.

## Decisions Made

- `renderScope` lives in render.go beside `renderFailures` and uses the same single-write, write-nothing-when-empty discipline. It does not call `sanitize`, because every value is a code constant from internal/ado.
- `workItemScope()` is a small test helper used by both new main_test.go tests. `allReadsFake` and `emptyResult` are untouched, so their scope stays nil.

## Deviations from Plan

None. The plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. The new surface is limited to the stderr line and help text that threats T-06-14 to T-06-16 already cover.

## User Setup Required

None.

## Next Phase Readiness

- Together with 06-03, this satisfies ROADMAP SC3: the fixed scope is disclosed in the envelope, on stderr in human mode, and in help.
- Optional live check for /gsd-verify-work: run `adoBizInt workitems list` against real Azure DevOps and confirm rows on stdout plus one `scope:` line on stderr.
- 06-05 can proceed. It touches disjoint files in the same package.

## Self-Check: PASSED

- All six modified files exist.
- Commits 0f88e62, a8748ab, f8a5655 and d75e9ba are present in `git log`.
