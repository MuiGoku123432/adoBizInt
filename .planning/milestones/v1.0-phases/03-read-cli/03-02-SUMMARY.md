---
phase: 03-read-cli
plan: 02
subsystem: cli
tags: [go, stdlib-flag, generics, help, plain-text-render, tdd]

requires:
  - phase: 03-read-cli
    provides: run() int, readClient seam, dispatch table, exit-code mapper, injectable writers
  - phase: 02-read-correctness
    provides: ado.Result[T] envelope, Failure, FailureKind
provides:
  - "readClient declaring all eight ado.Result[T] aggregate reads"
  - "eight-entry noun-verb dispatch table matching D-02 exactly"
  - "runRead generic command body shared by every read verb"
  - "newFlagSet, the single FlagSet construction site registering --json, --timeout, --project"
  - "three-level usage (root, noun, verb) rendered from the dispatch table"
  - "handleUsage resolving help and usage errors before config.Load"
  - "eight plain-text renderers plus renderFailures, with the stdout/stderr split"
affects: [03-03, 03-04, 03-05]

actuals:
  tokens: 15000
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "one generic runRead body per verb; only the fetch func and renderer differ"
    - "help derived from the dispatch table, so a registered verb is discoverable with no second list"
    - "usage resolved before config is loaded, so --help never needs a credential"
    - "renderers build the whole output in a strings.Builder and issue a single Write"

key-files:
  created:
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/usage.go
    - cmd/adoBizInt/render.go
    - cmd/adoBizInt/usage_test.go
  modified:
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/main.go
    - cmd/adoBizInt/main_test.go
    - cmd/adoBizInt/fakes_test.go

key-decisions:
  - "runRead is generic over the payload type: the eight commands differ only in their fetch function and renderer, so drift between verbs has nowhere to live"
  - "newFlagSet is the only flag.NewFlagSet call site in the package, which is what makes D-20 mechanically checkable rather than a convention"
  - "handleUsage runs before config.Load in run(), closing the config.go:151 trap where --help would fail for lack of a PAT"
  - "Usage and help go to stderr at every level, including when help was explicitly requested, because OUT-03 reserves stdout for parseable output"
  - "renderFailures prints project, operation and kind but not the underlying error text, which result.go records as localisable and server-controlled"
  - "Row fields are flattened to one line and ESC is stripped, so a server-supplied work item title cannot inject an escape sequence or a phantom record"
  - "An empty read writes nothing at all — no header, no count, no placeholder — so a caller counting lines gets zero"

patterns-established:
  - "Generic shared command body (runRead) with per-command closures over the client"
  - "Help tree derived from the single dispatch table, sorted for byte-identical output"
  - "Behaviour tests drive the real dispatch path with a fake client rather than calling renderers directly"

requirements-completed: [CLI-02, CLI-05, OUT-01, OUT-03]

coverage:
  - id: D1
    description: "All eight read verbs dispatch 1:1 to exactly one ado read method; no command merges two envelopes"
    requirement: CLI-02
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchTableMatchesD02"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchEachVerbCallsItsOwnMethod"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchResolvesEveryVerb"
        status: pass
    human_judgment: false
  - id: D2
    description: "--json, --timeout and --project are accepted by every one of the eight subcommands (D-20)"
    requirement: CLI-02
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestFlagsRegisteredOnEverySubcommand"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestProjectListAccumulates"
        status: pass
      - kind: other
        ref: "grep -rn 'flag.NewFlagSet' cmd/adoBizInt/ == 1 site (flags.go:55)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Help works at three levels and succeeds with no config file and no ADO_PAT"
    requirement: CLI-05
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestHelpNeedsNoCredentials"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestHelpIsDeterministic"
        status: pass
      - kind: other
        ref: "ADO_PAT= go run ./cmd/adoBizInt --help | prs --help | prs list --help, all exit 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "A flag parse error prints usage to stderr, writes nothing to stdout, and exits 3 (D-09, D-22)"
    requirement: OUT-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestBadFlagExitsThreeWithCleanStdout"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestUsageErrorsExitThree"
        status: pass
      - kind: other
        ref: "grep -rc 'flag.ExitOnError' cmd/adoBizInt/ == 0 in every file"
        status: pass
    human_judgment: false
  - id: D5
    description: "In human mode rows go to stdout and failure lines go to stderr (D-13)"
    requirement: OUT-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestRenderRowsToStdout"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestRenderFailuresToStderrOnly"
        status: pass
    human_judgment: false
  - id: D6
    description: "Human-mode output contains no ANSI escape sequences, and an empty read emits no placeholder"
    requirement: OUT-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestRenderHasNoANSI"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestRenderEmptyWritesNothing"
        status: pass
      - kind: other
        ref: "grep -rn 'internal/ui|lipgloss|bubble' cmd/adoBizInt/render.go returns nothing"
        status: pass
    human_judgment: false
  - id: D7
    description: "--json still emits exactly one envelope with failures inside it, unchanged by the new renderer"
    requirement: OUT-01
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestRenderNotInvokedUnderJSON"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchEmptyReadEmitsEmptyArrays"
        status: pass
    human_judgment: false
  - id: D8
    description: "Column choice and readability of the eight plain-text row layouts against real Azure DevOps data"
    verification: []
    human_judgment: true
    rationale: "Whether the selected columns are the useful ones for a human scanning real work items, releases and branches is a judgment about data this repo has no fixture for. Correctness of the stream split and escape-freedom is tested; usefulness of the column set is not testable and belongs in the phase UAT."

duration: 41 min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 02: Eight Read Verbs, Shared Flags, Three-Level Help Summary

**The tracer's single `prs list` path generalised into eight `noun verb` commands sharing one generic body, one flag registrar and one derived help tree, with a plain-text renderer that puts rows on stdout and failures on stderr.**

## Performance

- **Duration:** 41 min
- **Tasks:** 3
- **Files modified:** 8 (4 created, 4 modified)

## Accomplishments

- `readClient` widened from one method to the eight `Result[T]` aggregate reads, and the dispatch table from one entry to the eight D-02 keys. `GetBranches`' extra `repositories` parameter is absorbed by its own closure — the per-command closure shape from 03-01 paid for itself exactly as designed, with no adapter and no uniform method type.
- `runRead` is generic over the payload type, so the eight commands differ only in a fetch function and a renderer. Behavioural drift between verbs has nowhere to live: there is one parse, one fetch, one emit, one exit-code mapping.
- `newFlagSet` is the only `flag.NewFlagSet` call site in the package. D-20's intent — that `prs list` and `releases approvals` cannot disagree about which flags exist — is now mechanically checkable with a one-line grep rather than upheld by convention.
- `SetOutput(os.Stderr)` is explicit at that one site, closing RESEARCH assumption A2 by construction and pinning it with `TestBadFlagExitsThreeWithCleanStdout` rather than trusting a stdlib default.
- `handleUsage` runs before `config.Load` in `run()`. `config.go:151` aborts validation on an empty PAT, so `adoBizInt --help` would otherwise have failed for the first-time user with no credential configured — verified by running it with `ADO_PAT=` and a fresh `HOME`.
- The help tree is derived from the dispatch table and sorted at both levels, so registering a verb makes it discoverable with no second list to keep in sync, and two runs produce byte-identical output.
- Eight plain-text renderers, no TUI import, with failure lines confined to stderr. Server-supplied text is flattened to one line and ESC-stripped, so a work item title cannot inject an escape sequence or a phantom tab-separated record.

## Task Commits

1. **Task 1: register all eight read verbs** — `5268878` (feat)
2. **Task 2: shared flag registrar and three-level usage** — `8c65dca` (feat)
3. **Task 3 RED: failing renderer tests** — `889e6e2` (test)
4. **Task 3 GREEN: plain-text renderer** — `9b0c8a6` (feat)

## Files Created/Modified

- `cmd/adoBizInt/flags.go` — `globalFlags`, the repeatable/comma-separated `projectList` flag value, and `newFlagSet`, the single FlagSet construction site
- `cmd/adoBizInt/usage.go` — `nounVerbs` derivation, `usageRoot`/`usageNoun`/`usageVerb`, and `handleUsage`, which resolves help and usage errors before config is loaded
- `cmd/adoBizInt/render.go` — eight per-type renderers, `renderFailures`, and the `writeRows`/`row`/`sanitize`/`stamp` helpers
- `cmd/adoBizInt/dispatch.go` — `readClient` widened to eight methods, the eight-entry table, and the generic `runRead` body; the provisional `plainRows` and `printUsage` from 03-01 removed
- `cmd/adoBizInt/main.go` — `handleUsage` called before `config.Load`
- `cmd/adoBizInt/main_test.go` — dispatch-table, per-verb-resolution, 1:1-mapping and the five renderer behaviour tests
- `cmd/adoBizInt/usage_test.go` — flag-registration, help-without-credentials, determinism and usage-error tests
- `cmd/adoBizInt/fakes_test.go` — fake widened to eight closures plus `allReadsFake`

## Decisions Made

Beyond the plan's stated decisions, three judgment calls were made during execution:

- **Help goes to stderr, including when explicitly requested.** The plan does not name a stream for help output. OUT-03 reserves stdout for parseable output, and a caller piping `adoBizInt prs list --json` into a parser should never have to distinguish a document from a help page. 03-01's `printUsage` already wrote to stderr; that habit is now uniform across all three levels.
- **Verb-level `--help` is intercepted in `handleUsage`, not left to the FlagSet.** Letting `fs.Parse` return `flag.ErrHelp` would work, but only after `config.Load` and `cfg.Validate` had already run — which is exactly the failure D-21 and the plan's ordering requirement exist to prevent. `runRead` still treats `ErrHelp` as exit 0 as a backstop.
- **`renderFailures` omits the underlying error text.** `result.go:131-134` records the server message as localisable and server-controlled. Project, operation and kind are the three actionable fields; the full text remains available under `--json` and in the log file, so nothing is lost and the diagnostic channel stays terse and stable.

## Deviations from Plan

### Adjusted acceptance criterion

**1. [Scope] `grep -rn 'lipgloss\|bubble' cmd/adoBizInt/` cannot return nothing**
- **Found during:** Task 3 verification
- **Issue:** `cmd/adoBizInt/main.go` imports `bubbletea` for the zero-argument TUI path. CLI-01 forbids altering that path, so this criterion is unsatisfiable as literally written.
- **Resolution:** The criterion was applied to `render.go`, the file it exists to constrain: `grep -rn 'internal/ui\|lipgloss\|bubble' cmd/adoBizInt/render.go` returns nothing. Two prose comments that mentioned those names were reworded so the guard is a mechanical check rather than one defeated by its own explanation.
- **Files modified:** cmd/adoBizInt/render.go, cmd/adoBizInt/main_test.go
- **Committed in:** `9b0c8a6`

---

**Total deviations:** 1 (criterion scoped to the file it constrains).
**Impact on plan:** None on behaviour. The renderer imports nothing from the TUI packages, which is what the criterion was protecting.

## Issues Encountered

None. Task 1 and Task 2 passed verification on the first run after implementation; Task 3's RED phase failed as intended and went green on the first GREEN commit.

## TDD Gate Compliance

Task 3 carried `tdd="true"` and completed both gates: RED at `889e6e2` (`test(03-02)`, renderers stubbed as no-ops so the assertions rather than the build failed), GREEN at `9b0c8a6` (`feat(03-02)`). No REFACTOR commit was needed.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Ready for 03-03, 03-04 and 03-05. Two flags are deliberately parsed-but-unapplied and are hard dependencies on later plans, which is the one honesty debt this plan carries:

- **`--project`** is collected into `globalFlags.projects` and discarded at the `_ = gf.projects` line in `runRead`. 03-04 Task 2 must pass it as the `projects` argument of each read. Until then the CLI accepts a flag it ignores.
- **`--timeout`** is parsed into `globalFlags.timeout` and never turned into a deadline. 03-05 owns applying it, after `cfg.Validate()` so it cannot trip `config.go:159`.

**Carried to phase UAT:** whether the eight row layouts pick the useful columns for a human reading real Azure DevOps data (coverage entry D8).

## Self-Check: PASSED

- All four created files exist on disk.
- All four commit hashes resolve in `git log`.
- `go build ./... && go vet ./... && go test ./...` all green.
- `grep -rc 'flag.ExitOnError' cmd/adoBizInt/` returns 0 for every file; exactly one `flag.NewFlagSet` site.
- `ADO_PAT= go run ./cmd/adoBizInt --help`, `prs --help` and `prs list --help` each exit 0.

---
*Phase: 03-read-cli*
*Completed: 2026-09-09*
