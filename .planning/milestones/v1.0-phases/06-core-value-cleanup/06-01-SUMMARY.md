---
phase: 06-core-value-cleanup
plan: 01
subsystem: cli
tags: [go, stdlib-flag, help, argv, exit-codes]

requires:
  - phase: 03
    provides: handleUsage pre-config help (CLI-05), exit-code contract
  - phase: 05
    provides: --project allow-list in newFlagSet (D-09)
provides:
  - verb-level help decided by a silent stdlib parse over a value-inert FlagSet probe
  - runRead refuses stray positionals with exit 3 before any read (D-17)
affects: [06-02, 06-04, cli help, read verbs]

actuals:
  tokens: 3943
  tasks: 2
  commits: 4
plan_head_before: ada2cb1e3d1981453ace662392b7ccf837e4fdbb

tech-stack:
  added: []
  patterns:
    - "Help probe: newFlagSet copy with every Value wrapped (Set no-op, IsBoolFlag forwarded), parsed over hoistLeadingPositionals(args); only flag.ErrHelp means help; usage printed from a fresh FlagSet"

key-files:
  created: []
  modified:
    - cmd/adoBizInt/usage.go
    - cmd/adoBizInt/usage_test.go
    - cmd/adoBizInt/writeverbs_test.go
    - cmd/adoBizInt/dispatch.go
    - .planning/BACKLOG.md

key-decisions:
  - "Verb help is decided by stdlib flag parsing over a value-inert probe of the verb's own FlagSet; a help token consumed as a flag value is that value (D-07, D-08)"
  - "Read verbs refuse any stray positional with exit 3 and name the first token with %q (D-17); write-verb arity is unchanged"

patterns-established:
  - "helpProbeValue is the only flag.Value wrapper in the package and stays private to usage.go"

requirements-completed: [CLI-05, CLI-06, OUT-04]

coverage:
  - id: D1
    description: "A help token in a flag-value position is the value: workitems list --search help runs the read once; --comment -h reaches ApproveRelease; an unconfigured --project is still refused by the real parse"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestHelpTokenAsFlagValueIsNotHelp"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/writeverbs_test.go#TestReleaseCommentReachesTheMutation"
        status: pass
    human_judgment: false
  - id: D2
    description: "A help token in flag position still prints verb help with exit 0, no stdout, no config/PAT, and typed defaults (-timeout duration)"
    requirement: CLI-05
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestHelpTokenInFlagPositionIsHelp"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestHelpNeedsNoCredentials"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestHelpIsDeterministic"
        status: pass
    human_judgment: false
  - id: D3
    description: "A stray positional on any of the eight read verbs exits 3, names the token on stderr, writes nothing to stdout and calls no read"
    requirement: OUT-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestReadRejectsStrayPositional"
        status: pass
    human_judgment: false

duration: 3min
completed: 2026-10-12
status: complete
---

# Phase 6 Plan 01: Help Token Position Summary

**`handleUsage` now asks stdlib `flag` whether a help token is in flag position (value-inert probe over the verb's own FlagSet), so `workitems list --search help` runs the search and `--comment -h` is a comment; `runRead` refuses stray positionals with exit 3.**

## Performance

- **Duration:** 3 min (measured from the start marker, after context loading)
- **Started:** 2026-10-12T21:42:08Z
- **Completed:** 2026-10-12T21:44:40Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- **Probe design (W2, D-07, D-08, D-09).** `handleUsage` builds `newFlagSet(noun+" "+verb)`, silences it (`SetOutput(io.Discard)`, empty `Usage`), wraps every flag's `Value` in `helpProbeValue`, and parses `hoistLeadingPositionals(args[2:])`. Only `errors.Is(err, flag.ErrHelp)` selects help. Every other outcome, including a nil error or a parse error, is unhandled, and dispatch reports it after config loads.
  - `helpProbeValue.Set` applies nothing, so the empty pre-config `--project` allow-list cannot reject anything during the probe. The real parse in dispatch still enforces it (T-06-02).
  - `helpProbeValue.IsBoolFlag` forwards the wrapped flag's arity, so `--json -help` still reads `-help` as a flag.
  - Help is printed from a second, fresh `newFlagSet(name)`, because `PrintDefaults` over wrapped values loses type names and prints "panic calling String" (RESEARCH Pitfall 3).
  - Hoisting the positionals means `set-state 4821 --help` and `prs list foo --help` still reach the help flag (Pitfall 2).
  - `helpTokens` still drives the noun and verb positions, so `adoBizInt help` and `adoBizInt workitems help` are unchanged.
- **D-17 refusal.** `runRead` checks `len(fs.Args()) > 0` right after a successful Parse. It writes `<verb> takes no positional arguments; got "<tok>"; run 'adoBizInt <verb> --help' for its flags` to stderr and returns `exitUnknown` before any fetch. Write verbs are untouched.
- **BL-02 WR-05** is struck through in `.planning/BACKLOG.md` as closed by D-08.

## Task Commits

1. **Task 1 (tracer): help only in flag position**
   - `300c3ba` (test, RED)
   - `11a7ac8` (feat, GREEN; includes the BACKLOG WR-05 closure)
2. **Task 2: stray positional on a read verb exits 3**
   - `0bf4887` (test, RED)
   - `5445d34` (feat, GREEN)

## RED → GREEN Evidence

**Task 1 RED** (`go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestHelpTokenAsFlagValueIsNotHelp|TestHelpTokenInFlagPositionIsHelp|TestReleaseCommentReachesTheMutation)$'`, exit 1). Failing assertions against the old `args[2:]` scan:
- TestHelpTokenAsFlagValueIsNotHelp: value rows `--search help`, `--search -h`, `--search --help`, `--search -help --json`, `--json help`, `list help`, `releases approve … --comment -h --confirm`, `pipelines queue … --branch -h`, and `--project nope --search help` failed with "reported handled". The subtests "search help runs the read", "search help with no match still runs the read" and "unconfigured project is still refused" also failed.
- TestReleaseCommentReachesTheMutation/help_token_as_value failed on the `handleUsage` assertion.
- TestHelpTokenInFlagPositionIsHelp/prs_list_--help=x failed: the old exact-token scan did not recognise `--help=x`.
- Passed as regression pins: `--HELP`, `--search` with its value missing, bare `workitems list`, and `--search=-help`, which the old scan never matched. The `--HELP is an unknown flag` subtest and the other 7 flag-position rows also passed.

**Task 1 GREEN:** the plan's verify command passed with exit 0, 8 top-level `--- PASS: Test` lines and 0 FAIL. The tracer verify was re-run after the commit and passed again before expanding.

**Task 2 RED** (`go test … -run '^TestReadRejectsStrayPositional$'`): all 17 subtests failed with `readClient.<Method> called with no <Method>Func set`, so every stray token reached the read.

**Task 2 GREEN:** the plan's verify command passed with exit 0. It produced 7 top-level PASS lines and 17 `TestReadRejectsStrayPositional/` PASS lines.

**Module gate:** `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` passed (every package ok).

## Files Created/Modified

- `cmd/adoBizInt/usage.go`: stdlib help probe in `handleUsage`, private `helpProbeValue`, and rewritten doc comments for `helpTokens` and `handleUsage`.
- `cmd/adoBizInt/usage_test.go`: TestHelpTokenAsFlagValueIsNotHelp (13 rows and 4 subtests), TestHelpTokenInFlagPositionIsHelp (8 rows), TestReadRejectsStrayPositional (17 subtests).
- `cmd/adoBizInt/writeverbs_test.go`: `--comment -h` row; every row now asserts `handleUsage` leaves the request unhandled.
- `cmd/adoBizInt/dispatch.go`: D-17 stray-positional refusal in `runRead`.
- `.planning/BACKLOG.md`: BL-02 WR-05 closed by D-08.

## Decisions Made

None beyond the plan. D-07, D-08, D-09 and D-17 were implemented as specified.

## Deviations from Plan

None in the code. The plan's RED predictions were off in two places, and the tests themselves are exactly as specified:
- `workitems list --search=-help` (value row 4) already passed at RED. The old scan compared whole tokens, so it never matched `--search=-help`. It stays as a regression pin.
- `prs list --help=x` (flag-position row 4) failed at RED. The old scan did not recognise `--help=x`, and the stdlib probe fixes it. This means the new path widens help to the `--help=x` spelling, as the plan's must-haves require.
- The RED evidence was recorded from `go test -v` output. `gsd check tdd-red-evidence` was not run because it parses node TAP output, and this plan is `type: execute` with task-level TDD, not a `type: tdd` plan.

## Issues Encountered

None.

## User Setup Required

None. No external service configuration is required. An optional live check for /gsd-verify-work: `adoBizInt workitems list --search help --json` against the org should return an envelope, not help text.

## Next Phase Readiness

Ready for 06-02. It touches the same package but different files, and declares no shared symbols.

## Self-Check: PASSED

- All five modified files exist on disk.
- Commits 300c3ba, 11a7ac8, 0bf4887 and 5445d34 are present in `git log`.
- Acceptance greps passed: `helpProbeValue` is in usage.go, `len(fs.Args()) > 0` is in dispatch.go, and WR-05 is struck through in BACKLOG.md.
- The stub scan of the modified source found no TODO, FIXME or placeholder.

---
*Phase: 06-core-value-cleanup*
*Completed: 2026-10-12*
