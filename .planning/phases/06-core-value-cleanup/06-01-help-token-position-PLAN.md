---
phase: 06-core-value-cleanup
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd/adoBizInt/usage.go
  - cmd/adoBizInt/usage_test.go
  - cmd/adoBizInt/writeverbs_test.go
  - cmd/adoBizInt/dispatch.go
  - .planning/BACKLOG.md
autonomous: true
requirements: [CLI-05, CLI-06, OUT-04]
coupling_justified: ["06-02: same Go package (cmd/adoBizInt) but disjoint files, and neither plan adds a package-level symbol the other declares; either execution order compiles and tests identically"]

estimate:
  tokens: 36000
  raw_tokens: 36000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "workitems list --search help runs the read with the search term \"help\": handleUsage reports the arguments unhandled, dispatch calls GetWorkItems exactly once, and only the item whose title matches \"help\" is emitted (D-07, CLI-06, ROADMAP SC1 / audit W2)"
    - "A help token in a flag's value position is that flag's value and never a help request, for -h, -help, --help and the bare word help, in both the separate (--search -h) and joined (--search=-help) spellings (D-07) [edge CLI-05/adjacency]"
    - "A help token in flag position still prints verb help with exit 0, nothing on stdout, and no config or PAT loaded: after another flag's value (--search x -h), after a bool flag (--json -help), with a value attached (--help=x), next to a --project value while no projects are configured, and before or after a write verb's target (set-state 4821 --help, set-state --help 4821) (D-07, D-09, CLI-05) [edge CLI-05/ordering]"
    - "Verb help printed by the new path is byte-identical across runs and still renders typed flag defaults (-timeout duration), proving it is printed from a fresh FlagSet and not from the value-inert probe (D-09, RESEARCH Pitfall 3)"
    - "Write verbs follow the same rule: releases approve 77 --project P --comment -h --confirm sends the comment \"-h\" to ApproveRelease instead of printing help, and pipelines queue 7 --project P --branch -h is not help (D-08, closes BL-02 WR-05)"
    - "The help probe applies no flag value and has no effect beyond stderr: an unconfigured --project next to a value-position help token is not handled by handleUsage, and dispatch then refuses it with exit 3 and \"not a configured project\", so the allow-list is still enforced by the real parse (D-09, threat T-06-02) [edge CLI-05/concurrency]"
    - "Help-token matching is exact and case-sensitive, as stdlib flag defines it: --HELP is not help; handleUsage leaves it unhandled and dispatch rejects it as an unknown flag with exit 3 (D-07) [edge CLI-06/encoding]"
    - "Arguments that contain no help token are not help: workitems list with no further arguments, and workitems list --search with its value missing, are unhandled by handleUsage, and dispatch's existing behaviour for them (a normal read; a parse error exiting 3 with verb usage) is unchanged (D-07) [edge CLI-05/empty]"
    - "workitems list --search help over a read with no matching item exits 0 with nothing on stdout, and the fake proves the read ran exactly once, so an empty answer now always means the read ran (audit W2, OUT-04) [edge CLI-06/empty]"
    - "A stray positional argument on any of the eight read verbs, before or after its flags (workitems list help --json, prs list --json extra), exits 3, names the argument on stderr, writes nothing to stdout and calls no read (D-17, OUT-04) [edge OUT-04/empty]"
    - "adoBizInt help, adoBizInt workitems help, and root, noun and verb --help keep their current output and exit codes, because the bare word help is still recognised in the noun and verb positions (D-07, CLI-05)"
    - ".planning/BACKLOG.md BL-02 lists WR-05 as closed by Phase 6 D-08 rather than open (D-08)"
  prohibitions:
    - "Must NOT decide flag position versus flag value by hand-scanning argv or by tracking flag arity in new code; stdlib flag parsing over the verb's own FlagSet decides it (RESEARCH Don't Hand-Roll)"
    - "Must NOT apply, record or validate any flag value during the pre-config help probe; the --project allow-list is enforced only by the real parse that dispatch runs after config.Load (D-09)"
    - "Must NOT move help detection after config.Load or make --help require a PAT (D-09, Phase 3 CLI-05)"
    - "Must NOT change write-verb argument handling: runWrite's exactly-one-positional check and hoistLeadingPositionals stay as they are; D-17 applies to read verbs only"
  artifacts:
    - path: "cmd/adoBizInt/usage.go"
      provides: "handleUsage decides verb-level help by a silent stdlib parse over a value-inert copy of the verb's FlagSet; private helpProbeValue wrapper"
      contains: "helpProbeValue"
    - path: "cmd/adoBizInt/usage_test.go"
      provides: "TestHelpTokenAsFlagValueIsNotHelp, TestHelpTokenInFlagPositionIsHelp, TestReadRejectsStrayPositional"
      contains: "TestHelpTokenAsFlagValueIsNotHelp"
    - path: "cmd/adoBizInt/writeverbs_test.go"
      provides: "TestReleaseCommentReachesTheMutation row proving --comment -h reaches ApproveRelease and is not help"
      contains: "\"-h\""
    - path: "cmd/adoBizInt/dispatch.go"
      provides: "runRead refuses stray positionals with exit 3 before any read (D-17)"
      contains: "fs.Args()"
    - path: ".planning/BACKLOG.md"
      provides: "WR-05 struck through as closed by D-08"
      contains: "WR-05"
  key_links:
    - from: "cmd/adoBizInt/usage.go handleUsage"
      to: "cmd/adoBizInt/flags.go newFlagSet"
      via: "the probe is newFlagSet(name) with every flag Value wrapped, parsed over hoistLeadingPositionals(args[2:]); only flag.ErrHelp selects help"
      pattern: "errors\\.Is\\(.*flag\\.ErrHelp\\)"
    - from: "cmd/adoBizInt/usage.go handleUsage help branch"
      to: "cmd/adoBizInt/usage.go usageVerb"
      via: "a second, fresh newFlagSet(name) is printed, never the probe"
      pattern: "usageVerb\\("
    - from: "cmd/adoBizInt/dispatch.go runRead"
      to: "exitUnknown"
      via: "a non-empty fs.Args() after a successful Parse returns before any fetch"
      pattern: "len\\(fs\\.Args\\(\\)\\) > 0"
---

<objective>
Close audit warning W2 and decision D-17: an agent's request must never be silently misread as a different request.

- W2 (D-07, D-08, D-09): today `handleUsage` treats any help token anywhere after the verb as a help request. `workitems list --search help` prints help, exits 0, writes nothing to stdout and never runs the read, which looks exactly like an honest empty result. The same path makes `releases approve 77 --comment -h --confirm` print help instead of writing (BL-02 WR-05). After this plan a help token means help only in flag position, decided by stdlib `flag` parsing, and help still runs before config is loaded.
- D-17: `runRead` never checks `fs.Args()`. `workitems list help --json` stops parsing at `help`, silently drops `--json`, and emits human output. After this plan any stray positional on a read verb exits 3 and nothing runs.

Purpose: CLI-05 (help works everywhere, with no credentials), CLI-06 (`--search help` is a real filter value) and OUT-04 (exit 0 now means the read ran; a misread request exits 3).
Output: a stdlib help probe in `usage.go`, a stray-positional guard in `runRead`, the tests that pin both, and WR-05 closed in BL-02.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/06-core-value-cleanup/06-CONTEXT.md
@.planning/phases/06-core-value-cleanup/06-RESEARCH.md
@.planning/phases/06-core-value-cleanup/06-PATTERNS.md
@cmd/adoBizInt/usage.go
@cmd/adoBizInt/flags.go
@cmd/adoBizInt/dispatch.go
</context>

<interfaces>
These already exist. Use them as they are and do not redefine them.

- `func newFlagSet(name string) (*flag.FlagSet, *globalFlags, *filterFlags, *writeFlags)` in cmd/adoBizInt/flags.go:184. It sets `fs.SetOutput(stderrWriter)` (line 186) and `fs.Usage = func() { usageVerb(name, fs) }` (line 280). The `--project` flag is an `fs.Func` (lines 198-215) whose callback rejects every value while `configuredProjects` is empty, which is the state before config loads.
- `func hoistLeadingPositionals(args []string) []string` in cmd/adoBizInt/write.go:597. It moves tokens that precede the first flag to the end, so `set-state 4821 --help` parses its flags.
- `func usageVerb(name string, fs *flag.FlagSet)` in cmd/adoBizInt/usage.go:119, and `helpTokens` at usage.go:11-16, still used for the `args[0]` and `args[1]` checks at lines 148 and 168.
- `const exitOK = 0`, `exitUnknown = 3` in cmd/adoBizInt/dispatch.go:21-29. Use the constants, never bare integers.
- Test helpers: `captureOutput(t)` (main_test.go:55), `withConfiguredProjects(t, names...)` (main_test.go:704; no names models the pre-config state), `wantCommands` (main_test.go:252, the eight read verbs), `allReadsFake(t)` (fakes_test.go:101), `decodeWorkItemIDs(t, stdout)` (filter_test.go), `wrapFake`, `tempAuditSink(t)`.
- stdlib `flag` facts verified in 06-RESEARCH.md "Verification Evidence": with no `h`/`help` flag defined, Parse returns `flag.ErrHelp` for `-h`, `-help`, `--h`, `--help` and `--help=x` in flag position, and never for a token consumed as a non-bool flag's value. `IsBoolFlag` decides whether the next token is consumed.
</interfaces>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): help only in flag position. workitems list --search help runs the read, and a help token in any value position is a value on read and write verbs (W2, D-07, D-08, D-09)</name>
  <reversibility rating="reversible">D-07 changes which argv spellings print help; no output shape, exit-code integer or JSON key moves, and the old scan is restorable in one function.</reversibility>
  <files>cmd/adoBizInt/usage.go, cmd/adoBizInt/usage_test.go, cmd/adoBizInt/writeverbs_test.go, .planning/BACKLOG.md</files>
  <read_first>
    - cmd/adoBizInt/usage.go: the whole file (189 lines). Note helpTokens at 10-16, usageVerb at 112-131 and the args[2:] loop being replaced at 178-188
    - cmd/adoBizInt/flags.go lines 153-283: stringList, newFlagSet, the --project Func at 198-215, SetOutput at 186 and Usage at 280
    - cmd/adoBizInt/write.go lines 133-147 (the Parse plus flag.ErrHelp idiom) and 580-612 (hoistLeadingPositionals and its doc comment)
    - cmd/adoBizInt/usage_test.go: the whole file (236 lines). Note TestHelpNeedsNoCredentials, TestHelpIsDeterministic, TestBadFlagExitsThreeWithCleanStdout and TestHandleUsagePassesValidSubcommandsThrough, which must stay green unchanged
    - cmd/adoBizInt/writeverbs_test.go lines 133-168: TestReleaseCommentReachesTheMutation
    - cmd/adoBizInt/filter_test.go lines 17-75: workItemsFixture, workItemsClient, decodeWorkItemIDs
    - cmd/adoBizInt/main_test.go lines 55-68 (captureOutput) and 700-709 (withConfiguredProjects)
    - .planning/phases/06-core-value-cleanup/06-RESEARCH.md: Pattern 1, Pitfalls 1-3, Verification Evidence
    - .planning/BACKLOG.md lines 73-95: BL-02, the WR-05 line and the strike-through style of the closed branches line
  </read_first>
  <behavior>
    New TestHelpTokenAsFlagValueIsNotHelp in usage_test.go. It calls withConfiguredProjects(t) with no names, the pre-config state. Each row asserts that handleUsage(args) reports handled == false and that stdout and stderr are both empty, because the probe is silent:
    1. workitems list --search help
    2. workitems list --search -h
    3. workitems list --search --help
    4. workitems list --search=-help
    5. workitems list --search -help --json
    6. workitems list --json help (the bare word after flags is not help, D-07)
    7. workitems list help (the bare word in the third position is not help, D-07)
    8. workitems list --HELP (case-sensitive: not help)
    9. workitems list --search (value missing: not help; dispatch reports it later)
    10. workitems list (no further arguments)
    11. releases approve 77 --project P --comment -h --confirm (WR-05)
    12. pipelines queue 7 --project P --branch -h
    13. workitems list --project nope --search help (an unconfigured project next to a value-position help token)

    Subtests of the same test, end to end through dispatch:
    - "search help runs the read": withConfiguredProjects(t, "one"). The fake's GetWorkItemsFunc counts its calls and returns two items, ID 7 titled "help desk outage" and ID 8 titled "Beta task", both Project "one" and State "Active", with Complete true and empty Failures. Args: workitems list --search help --json. Expect handleUsage unhandled, dispatch == exitOK, decodeWorkItemIDs == [7], and exactly 1 call.
    - "search help with no match still runs the read": the same fake returning only ID 8, in human mode (no --json). Expect exitOK, empty stdout, and exactly 1 call.
    - "unconfigured project is still refused": withConfiguredProjects(t, "one"). Args row 13. Expect handleUsage unhandled; then dispatch with a no-closure &fakeReadClient{t: t} returns exitUnknown, stderr contains "not a configured project", and stdout is empty.
    - "--HELP is an unknown flag": dispatch with a no-closure fake on row 8 returns exitUnknown with empty stdout.

    New TestHelpTokenInFlagPositionIsHelp in usage_test.go. It also uses withConfiguredProjects(t) with no names. Each row asserts handled == true, code == exitOK, stdout empty, and that stderr contains "Usage: adoBizInt", "-json" and "-timeout duration", and does not contain "panic calling String":
    1. workitems list --project P --help (RESEARCH Pitfall 1)
    2. workitems list --search x -h
    3. prs list --json -help
    4. prs list --help=x
    5. workitems set-state 4821 --help (RESEARCH Pitfall 2)
    6. workitems set-state --help 4821
    7. releases approve 77 --project P -h
    8. prs list foo --help (the hoisted stray token: help still wins)

    TestReleaseCommentReachesTheMutation in writeverbs_test.go gains the row {"help token as value", releases approve 77 --project P --comment -h --confirm --json, want "-h"}. Every row of that test now also asserts handleUsage(tc.args) reports handled == false before it calls dispatch.

    RED against the current args[2:] scan: value rows 1-7, 11, 12 and 13 report handled == true. The "search help runs the read" subtest fails, and so does the new TestReleaseCommentReachesTheMutation row's handleUsage assertion. Rows 8-10, every flag-position row, and the remaining subtests already pass. They are regression pins, and they must still pass after GREEN.
  </behavior>
  <action>
    RED first:
    1. Add the two tests and the writeverbs_test.go row exactly as the behavior block states.
    2. Put a one-line doc comment on each test naming D-07, D-08 or D-09 and audit W2.
    3. Reuse captureOutput, withConfiguredProjects, decodeWorkItemIDs and wrapFake. Add no new test helper.
    4. Run the tests and record the RED output in the SUMMARY.

    GREEN in cmd/adoBizInt/usage.go, per D-07 and D-09:
    1. Replace the args[2:] loop at lines 178-186 with a stdlib probe:
       - Let name be noun + " " + verb, and build probe from newFlagSet(name).
       - Silence the probe: call probe.SetOutput(io.Discard) and set probe.Usage to an empty func.
       - Walk probe.VisitAll and replace every flag's Value with helpProbeValue wrapping the original Value.
       - Parse hoistLeadingPositionals(args[2:]) with the probe.
       - When errors.Is(err, flag.ErrHelp) holds, build a second, fresh FlagSet with newFlagSet(name), pass it to usageVerb, and return exitOK, true.
       - Every other outcome returns exitOK, false: a nil error, or any non-help parse error such as a missing value or an unknown flag. dispatch then reports a parse error after config loads, exactly as today.
       - Comment above the block: stdlib flag decides position versus value, so a help token consumed as a flag's value is that value (D-07, D-08); the probe runs before config.Load (D-09); it applies no value because the --project allow-list is empty before config loads and would reject every value; usage is printed from a fresh FlagSet because PrintDefaults on wrapped values loses the type names and emits "panic calling String" lines (RESEARCH Pitfall 3); hoistLeadingPositionals is applied so a target written before the flags does not stop the parse (RESEARCH Pitfall 2).
    2. Add the private type helpProbeValue, which embeds flag.Value:
       - Its Set(string) method returns nil and applies nothing.
       - Its IsBoolFlag() method reports true only when the wrapped Value has an IsBoolFlag method that returns true. Forwarding it preserves arity: without it, --json --help would read --help as --json's value.
       - Doc comment: it is single-use and stays private to this file, and it is the only flag.Value wrapper in the package.
    3. Keep helpTokens. It still drives the args[0] and args[1] checks, so adoBizInt help and adoBizInt workitems help keep working (D-07). Rewrite its doc comment: these spellings mean help only as the noun or the verb; inside a verb's own arguments stdlib flag decides.
    4. Rewrite the handleUsage doc comment:
       - Verb-level help is a help flag in flag position, decided by stdlib parsing, so a help token used as a flag's value is the value (D-07, D-08).
       - It still runs before config.Load and cfg.Validate (D-09).
    5. Add errors and io to the imports.

    Then in .planning/BACKLOG.md, under BL-02, strike the WR-05 line through using the same style as the closed branches line, and append "— **closed** by Phase 6 plan 06-01 (D-08): a help token is recognised only in flag position, so --comment -h is a comment value". Change no other BACKLOG line.

    Leave these untouched:
    - flags.go, write.go and dispatch.go. Task 2 owns dispatch.go.
    - usageRoot, usageNoun and usageVerb's printed text.
    - go.mod. No dependency is added.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestHelpTokenAsFlagValueIsNotHelp|TestHelpTokenInFlagPositionIsHelp|TestReleaseCommentReachesTheMutation|TestHelpNeedsNoCredentials|TestHelpIsDeterministic|TestBadFlagExitsThreeWithCleanStdout|TestHandleUsagePassesValidSubcommandsThrough|TestUsageErrorsExitThree)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, or fewer than 8 top-level "--- PASS: Test" lines</fails_when>
  </verify>
  <acceptance_criteria>
    - All 13 TestHelpTokenAsFlagValueIsNotHelp rows report handled == false with empty stdout and stderr, and its four subtests pass:
      - --search help runs the read once and emits only ID 7
      - a no-match search runs the read once with empty stdout and exit 0
      - an unconfigured --project is refused by dispatch with exit 3
      - --HELP is refused by dispatch with exit 3
    - All 8 TestHelpTokenInFlagPositionIsHelp rows report handled == true and exit 0, with stderr containing "-timeout duration" and not containing "panic calling String".
    - TestReleaseCommentReachesTheMutation passes all three rows, and its "-h" row records "-h" as the comment that reached ApproveRelease.
    - TestHelpNeedsNoCredentials, TestHelpIsDeterministic, TestBadFlagExitsThreeWithCleanStdout, TestHandleUsagePassesValidSubcommandsThrough and TestUsageErrorsExitThree pass unchanged.
    - The SUMMARY records RED evidence for the value rows and the end-to-end subtest.
    - .planning/BACKLOG.md shows WR-05 struck through with the D-08 closing note.
  </acceptance_criteria>
  <done>A help token means help only in flag position on every read and write verb, help still needs no config or PAT, and `workitems list --search help` runs the read it names.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: a stray positional on a read verb exits 3 and runs nothing (D-17)</name>
  <reversibility rating="costly">D-17 records this rating. It narrows the argv every read verb accepts, so a script that passed stray words starts failing with exit 3; undoing it is one check, but callers may already depend on the refusal.</reversibility>
  <files>cmd/adoBizInt/dispatch.go, cmd/adoBizInt/usage_test.go</files>
  <read_first>
    - cmd/adoBizInt/dispatch.go lines 214-333: dispatch and runRead. The parse block is at 243-251, and `ff.identity = identity` follows it at 252
    - cmd/adoBizInt/write.go lines 156-171: the write path's arity check and its stderr wording, the analog
    - cmd/adoBizInt/main_test.go lines 163-197 (TestDispatchUsageErrors and its no-closure fake idiom) and 252-261 (wantCommands)
    - cmd/adoBizInt/usage_test.go: as left by Task 1
  </read_first>
  <behavior>
    New TestReadRejectsStrayPositional in usage_test.go. For every key in wantCommands, run two subtests through dispatch with a no-closure &fakeReadClient{t: t}, so any read call fails the test by name:
    - "<key> stray first": the key's two tokens, then "stray", then "--json".
    - "<key> stray last": the key's two tokens, then "--json", then "stray".
    Each subtest expects:
    - dispatch returns exitUnknown
    - stdout is empty
    - stderr contains the verb name and the quoted token "\"stray\""
    One extra subtest drives workitems list help --json and expects exitUnknown, empty stdout, and stderr containing "\"help\"" and "--help".

    RED against the current runRead: every subtest reaches the read and fails with "called with no ...Func set".
  </behavior>
  <action>
    RED first:
    - Add TestReadRejectsStrayPositional as the behavior block states, with a doc comment citing D-17.
    - The comment says a token stdlib could not read as a flag stops the parse, so it and every flag after it were silently dropped. `workitems list help --json` therefore produced human output.
    - Record the RED output in the SUMMARY.

    GREEN in cmd/adoBizInt/dispatch.go runRead, per D-17:
    1. Immediately after the parse block, before ff.identity is assigned, add the refusal. When len(fs.Args()) > 0:
       - write one line to stderrWriter: the verb name, the words "takes no positional arguments; got", the first stray token formatted with %q, and the hint "run 'adoBizInt <name> --help' for its flags"
       - return exitUnknown
    2. Comment: D-17. Nothing ran, so the exit is 3. The token is quoted with %q, so a control character cannot reach the terminal raw. Only the first token is named; that is enough to locate the mistake.

    Leave these untouched:
    - runWrite and its exactly-one-positional rule (D-25). D-17 applies to read verbs only.
    - The help probe from Task 1. A help flag alongside a stray token still prints help, and help wins.
    - usageVerb's text.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestReadRejectsStrayPositional|TestFlagsRegisteredOnEverySubcommand|TestHandleUsagePassesValidSubcommandsThrough|TestDispatchUsageErrors|TestBadFlagExitsThreeWithCleanStdout|TestRenderEmptyWritesNothing|TestDispatchResolvesEveryVerb)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 7 top-level "--- PASS: Test" lines, or fewer than 17 "--- PASS: TestReadRejectsStrayPositional/" subtest lines</fails_when>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"</automated>
    <fails_when>non-zero exit from any stage, any "FAIL" package line, or gofmt listing a file</fails_when>
  </verify>
  <acceptance_criteria>
    - All 17 TestReadRejectsStrayPositional subtests pass: 2 per read verb plus the workitems list help --json row. Each exits 3 with empty stdout, stderr naming the token, and no read call.
    - TestFlagsRegisteredOnEverySubcommand, TestDispatchUsageErrors, TestRenderEmptyWritesNothing and TestDispatchResolvesEveryVerb pass unchanged, so a read with no positional is unaffected.
    - The SUMMARY records RED evidence.
    - The full module gate exits 0.
  </acceptance_criteria>
  <done>A read verb either runs exactly the request it was given or refuses with exit 3. No token is silently dropped together with the flags after it.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| argv (agent or human) to handleUsage | Untrusted tokens decide whether a read or write runs at all, before any config or credential is loaded. |
| argv to runRead / runWrite FlagSet | The real parse applies values, including the --project allow-list (D-09, Phase 5). |
| CLI stdout/stderr and exit code to the calling agent | The agent branches on exit 0 and an empty stdout as "the read ran and found nothing". |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-06-01 | Spoofing (result integrity) | handleUsage help detection (audit W2) | medium | mitigate | Help is decided by stdlib parsing over the verb's own FlagSet, so a token consumed as a value is never help. TestHelpTokenAsFlagValueIsNotHelp pins 13 value-position rows and proves end to end that `--search help` runs the read once. An exit 0 with empty stdout can therefore no longer come from a read that never ran. |
| T-06-02 | Elevation of Privilege (access control) | pre-config help probe vs the --project allow-list | medium | mitigate | helpProbeValue.Set applies nothing, so the probe neither records nor validates a value, and an unhandled request always reaches the real parse in dispatch, which enforces the allow-list. The "unconfigured project is still refused" subtest proves `--project nope --search help` exits 3 with "not a configured project". |
| T-06-03 | Tampering (request integrity) | runRead stray positionals (D-17) | medium | mitigate | A non-empty fs.Args() after Parse returns exit 3 before any fetch, so `help --json` can no longer silently drop --json. TestReadRejectsStrayPositional covers all eight read verbs in both token orders with a no-closure fake. |
| T-06-04 | Information Disclosure | D-17 diagnostic echoes an argv token | low | accept | The token goes to stderr only and is %q-quoted, so control bytes are escaped. It is the caller's own input echoed to the caller's own terminal. Nothing reaches stdout, the log or the audit file. |
| T-06-05 | Denial of Service | probe parse on hostile argv | low | accept | The probe is one in-memory stdlib parse over the same tokens dispatch already parses, with no I/O and no allocation beyond the FlagSet. |
| T-06-SC | Tampering | dependency installs | low | accept | Not applicable: no package install, and go.mod is untouched. |
</threat_model>

<verification>
- Module gate (proven command): `cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`. It fails on a non-zero exit, a "FAIL" package line, or gofmt listing a file.
- 06-VALIDATION.md rows for W2, D-08 and D-17 are green: `go test ./cmd/adoBizInt -run 'TestHelpTokenAsFlagValueIsNotHelp|TestHelpTokenInFlagPositionIsHelp|TestReleaseCommentReachesTheMutation|TestReadRejectsStrayPositional' -count=1`.
- Optional live confirmation, a human step for /gsd-verify-work that does not block this plan: `adoBizInt workitems list --search help --json` against the org returns an envelope rather than help text.
</verification>

<success_criteria>
- ROADMAP SC1 holds: a help token used as a flag value is treated as the value, and only a help token in flag position prints help (D-07, D-08), still with no config or PAT (D-09).
- D-17 holds: a stray positional on a read verb exits 3 with nothing run.
- No change to help text, exit-code integers, write-verb argument handling, or go.mod.
</success_criteria>

## Artifacts this phase produces

| Artifact | Kind | Purpose |
|----------|------|---------|
| `cmd/adoBizInt/usage.go` | modified | stdlib help probe in `handleUsage`; private `helpProbeValue` |
| `cmd/adoBizInt/usage_test.go` | modified | `TestHelpTokenAsFlagValueIsNotHelp`, `TestHelpTokenInFlagPositionIsHelp`, `TestReadRejectsStrayPositional` |
| `cmd/adoBizInt/writeverbs_test.go` | modified | `--comment -h` row in `TestReleaseCommentReachesTheMutation` |
| `cmd/adoBizInt/dispatch.go` | modified | stray-positional refusal in `runRead` (D-17) |
| `.planning/BACKLOG.md` | modified | WR-05 closed by D-08 |
| `.planning/phases/06-core-value-cleanup/06-01-SUMMARY.md` | new | Execution record with RED evidence |

<output>
Create `.planning/phases/06-core-value-cleanup/06-01-SUMMARY.md` when done. Record the probe design (wrapped Set, forwarded IsBoolFlag, fresh FlagSet for printing, hoisted positionals), the D-17 refusal, the RED-then-GREEN evidence, and the BL-02 WR-05 closure.
</output>
