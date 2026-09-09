---
phase: 03-read-cli
verified: 2026-09-09T14:20:00Z
status: human_needed
verdict: PASS-WITH-CONCERNS
score: 8/9 requirements verified in code
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "CLI-01 — `adoBizInt` with no arguments launches the TUI exactly as before"
    test: "Build the binary with a live PAT configured and run `adoBizInt` with zero arguments"
    expected: "The interactive TUI launches, alt-screen, all six views present and behaving as pre-Phase-3"
    why_human: "run() constructs a live ado.Client and calls tea.NewProgram before the TUI branch; no in-process test can reach it. Code inspection confirms the branch guard `if len(os.Args) > 1` is correct, but the launch itself is unexercised."
human_verification:
  - test: "Zero-argument TUI launch (03-01 coverage entry D7)"
    expected: "TUI starts unchanged; no CLI output on stdout"
    why_human: "Requires a live PAT and an interactive terminal"
concerns:
  - id: C-03-A
    severity: warning
    title: "REQUIREMENTS.md traceability rows for Phase 3 still read Pending"
    detail: "Rows 227-228 pack five and four IDs into single cells; requirements.mark-complete matches per-single-ID and returned table_unmatched."
  - id: C-03-B
    severity: warning
    title: "T-03-15 (world-readable log may contain the PAT) has no Phase 5 requirement ID"
    detail: "Tracked in CONCERNS.md MED2 / row 14 but absent from HARD-01..HARD-05, so the roadmap does not carry it."
  - id: C-03-C
    severity: info
    title: "grep-negative acceptance criteria defeated three times by explanatory prose"
    detail: "Recommend recording as a phase learning; criteria of the form `grep returns nothing` are not sound as written."
deferred:
  - truth: "Log file opened 0600 rather than 0644; PAT-redaction regression test"
    addressed_in: "Phase 5"
    evidence: "CONCERNS.md MED2 remediation: 'Open the log 0600 and the directory 0700 ... PAT-redaction test'; Phase 5 goal is 'The security findings that a CLI makes reachable are closed'"
---

# Phase 03: Read CLI Verification Report

**Phase Goal:** An AI agent can query every existing read operation and parse the result
**Verified:** 2026-09-09
**Verdict:** PASS-WITH-CONCERNS
**Status:** human_needed (one open UAT item)

## Build Gate

| Command | Result |
| ------- | ------ |
| `go build ./...` | exit 0, clean |
| `go vet ./...` | exit 0, clean |
| `go test ./...` | all packages ok |
| `go test ./cmd/adoBizInt -cover` | 77.4% of statements |
| `go test ./internal/ado -cover` | 63.4% of statements |

41 top-level tests pass in `cmd/adoBizInt`, covering dispatch, exit codes, flags,
filters, render, timeout, interrupt, panic and usage.

## Requirement Assessment

Each requirement judged against code, not against SUMMARY claims.

| Req | Verdict | Evidence |
| --- | ------- | -------- |
| **CLI-01** — no subcommand launches the TUI unchanged | ⚠️ PRESENT, BEHAVIOR UNVERIFIED | `main.go` gates the CLI on `if len(os.Args) > 1`; zero args falls through to `ui.NewModel` + `tea.NewProgram` with the original argument list and the original `return 1` on TUI error. Branch placement is deliberate and correct (it precedes `ui.NewModel` so a read does not pay TUI construction). No automated test reaches it — `run()` builds a live client first. 03-04 Task 3 (TUI render/filter equivalence after the filter promotion) was human-approved this session; the zero-argument launch itself (D7) is still open. |
| **CLI-02** — read subcommands for all six nouns | ✓ VERIFIED | `dispatch.go` `commands` map holds eight `noun verb` keys: `workitems list`, `prs list`, `pipelines runs`, `pipelines definitions`, `releases list`, `releases approvals`, `branches list`, `dashboard counts`. `TestDispatchTableMatchesD02`, `TestDispatchResolvesEveryVerb` and `TestDispatchEachVerbCallsItsOwnMethod` pin the table to the eight `readClient` methods 1:1. |
| **CLI-04** — `--timeout` bounds the invocation; interrupts handled | ✓ VERIFIED | `--timeout` registered in the single `newFlagSet`; `runRead` overrides `readBudget` and derives via `fetchctx.NewWithParent(ctx, budget)`. `main.go` wraps the dispatch in `signal.NotifyContext(..., os.Interrupt, syscall.SIGTERM)`. `runRead` checks `ctx.Err()` *before any stdout write*, so an interrupt yields exactly one document. Five timeout tests plus four interrupt tests pass, including `TestTimeoutEqualToPollIntervalIsAccepted` (proves the D-P6 post-validation assignment) and `TestTimeoutZeroFallsBackToDefaultBudget`. |
| **CLI-05** — every command responds to `--help` | ✓ VERIFIED | `usage.go` provides three levels; `handleUsage` runs *before* `config.Load`, so `--help` needs no PAT. `TestHelpNeedsNoCredentials`, `TestHelpIsDeterministic`, `TestHandleUsagePassesValidSubcommandsThrough` pass. Live smoke: `go run ./cmd/adoBizInt --help` exits 0 with usage on stderr and stdout empty. |
| **CLI-06** — agent filters as flags, promoted out of the views | ✓ VERIFIED | `internal/ado/filters.go` holds six exported predicates (`IsDoneState`, `IsTaskType`, `MatchesIdentity`, `MatchesProject`, `MatchesRepository`, `MatchesSearch`). `doneStates` has exactly one definition repo-wide (grep confirms a single site). All five view packages call through to them (`workitems.go:614,618,628`, `prs.go:504,509,516`, `branches.go:336`, `releases.go:539`, `pipelines.go:513`) — the promotion is real deduplication, not a copy. Every filter flag defaults off (`filterFlags` zero value), which `TestDefaultsApplyNoFilter` pins. `filterSupportByCommand` registers flags only on verbs whose payload supports them, pinned by `TestFilterFlagsRegisteredOnSupportedVerbsOnly`. |
| **OUT-01** — `--json` emits machine-readable JSON on stdout | ✓ VERIFIED | `emitJSON` marshals the raw envelope to a byte slice and does exactly one `Write` — no streaming encoder, so no partial document can precede a crash record. `TestDispatchEmptyReadEmitsEmptyArrays` proves `items`/`failures` serialize as `[]` not `null`, which is the difference between an agent's `.failures \| length` working and erroring on the happy path. |
| **OUT-03** — stdout carries only parseable output | ✓ VERIFIED | `stdoutWriter`/`stderrWriter` are the only sinks; `newFlagSet` calls `fs.SetOutput(stderrWriter)` explicitly rather than trusting the default. Human-mode rows go to stdout, failures to stderr. Pinned by `TestRenderFailuresToStderrOnly`, `TestRenderHasNoANSI`, `TestRenderNotInvokedUnderJSON`, `TestBadFlagExitsThreeWithCleanStdout`, `TestTimeoutUnparseableExitsThreeWithCleanStdout`, `TestLoggingInitFailureKeepsStdoutClean`. That last one closes RESEARCH assumption A1 by construction. |
| **OUT-04** — four distinct exit codes | ✓ VERIFIED | `exit.go` `exitCodeFor` scans `FailureAuth` *before* consulting `Complete`, so a co-occurring timeout cannot mask a rejected credential (D-07). Matches on `FailureKind` only, never on server-controlled error text. Startup failures return `exitUnknown` (3), not `exitPartial` (1) — a read that never ran is a different fact from an incomplete one. `TestExitCodeFor` covers the matrix. |
| **OUT-05** — a panic emits structured JSON | ✓ VERIFIED | `main()` is `os.Exit(guard(run))`. `guard` recovers and calls `handlePanic`, which returns `exitUnknown` and writes `{"error":{kind,message,log_path}}` — nested under `error` so a crash is *not representable* as a normal result envelope. Deliberately carries no stack: Go renders frames with arguments and the client constructor takes the credential-bearing config. `TestPanicExitsThree`, `TestPanicEmitsDistinctErrorObject`, `TestPanicRecordCarriesNoStackOrCredential`, `TestPanicWithFailedLoggingWritesNoStackToStderr`, `TestGuardPassesThroughWhenNothingPanics` all pass. |

**Score: 8/9 verified in code, 1 present-but-behavior-unverified.**

## Success Criteria (ROADMAP)

| # | Criterion | Status |
| - | --------- | ------ |
| 1 | No-argument launch starts the TUI as before | ⚠️ present, human check open |
| 2 | Read subcommands for all six nouns | ✓ |
| 3 | `--json` parseable, stdout pure | ✓ |
| 4 | Exit codes distinguish success/partial/auth/unknown | ✓ |
| 5 | `--timeout` bounds the invocation, interrupts handled | ✓ |
| 6 | Filters as flags, promoted out of view packages | ✓ |
| 7 | Panic emits structured JSON | ✓ |

## Known Issues — Judged

### 1. REQUIREMENTS.md traceability rows stale — WARNING, does not block

The checkbox list at lines 31-44 is correct: all nine Phase 3 IDs are `[x]`. Only
the *summary* traceability table at lines 227-228 is stale, and it is stale for a
mechanical reason: the rows pack five and four IDs into one cell
(`| CLI-01, CLI-02, CLI-04, CLI-05, CLI-06 | Phase 3 | Pending |`), and
`requirements.mark-complete` matches per-single-ID, so it correctly reported
`table_unmatched` rather than guessing.

**Judgment:** Phase 3 traceability is substantively satisfied. The authoritative
per-requirement checkboxes are marked and the code backs every one of them. The
table is a derived view that failed to update, not a missing deliverable. This is
a documentation defect, not a phase gap.

**Recommended fix:** Split rows 227-228 into one row per ID, matching the shape
already used for TRUST-01..TRUST-04 and OUT-02 at lines 221-225 — those rows are
per-ID and they updated correctly in Phase 2. That makes the table
machine-updatable from now on and removes the class of failure. Do it as a
one-line docs commit; do not defer it, because Phase 4 and Phase 5 will hit the
identical failure on their own packed rows (`SAFE-01 .. SAFE-04`,
`HARD-01 .. HARD-05`).

### 2. T-03-15 (stack to a 0644 log that may contain the PAT) — ACCEPTABLE to leave open, with one required action

Confirmed in code: `internal/logging/logging.go:24` opens the log
`O_CREATE|O_WRONLY|O_APPEND, 0644` in a `0755` directory. `handlePanic` writes
`debug.Stack()` there, and Go renders frames with arguments — the client
constructor takes the config value carrying the PAT, so the exposure is real.

**Judgment: acceptable for phase closure.** Three reasons, and they are about the
delta this phase introduced, not about the absolute risk:

- The pre-existing state was already a 0644 log. Phase 3 did not create the
  world-readable credential-bearing artifact; it added one more writer to it.
- The exposure requires local filesystem access on the operator's own machine
  *and* an actual panic. It is not remotely reachable and not agent-reachable.
- The countermeasure this phase *did* take is the load-bearing one: the crash
  record on stdout carries no stack at all, and the stack write is gated on
  `loggingReady` so a failed init never dumps credential-bearing frames to the
  terminal. The exposed surface — stdout, stderr, the agent's view — is clean.
  Only the private log file is affected.

**Required action, and this is the actual finding:** the deferral is currently
*not tracked where the roadmap will act on it*. It appears in `03-05-PLAN.md`,
`03-05-SUMMARY.md`, and `CONCERNS.md` MED2 (row 14), but **HARD-01 through
HARD-05 do not include it**. Phase 5's plans will be written from the HARD-\*
requirement list, so as filed this item will not be picked up. Add a `HARD-06`
covering "log opened 0600, directory 0700, and a regression test asserting the
PAT string never appears in captured log output" — the remediation text already
exists verbatim in CONCERNS.md:437. Without that, "track it as Phase 5 hardening"
is an intention, not a plan.

### 3. Open UAT items

- **03-01 coverage entry D7 — zero-argument TUI launch:** OPEN. Requires a live
  PAT and an interactive terminal. Code inspection is favorable (the branch guard
  is correct and the TUI path is byte-identical to pre-Phase-3, including the
  `return 1` on `p.Run()` error), but the launch is unexercised. This is the one
  item holding the phase at `human_needed` rather than `passed`.
- **03-04 Task 3 — TUI behavioral equivalence after the filter promotion:**
  CLOSED. Human-approved this session; the user built the binary and verified.
  Treated as passed. Note this is a *different* check from D7: it covers whether
  the promoted predicates changed TUI rendering, not whether the zero-argument
  entry point still reaches the TUI.

### 4. `grep returns nothing` acceptance criteria — INFO, record as a learning

Three separate incidents this phase where an acceptance criterion of the form
"`grep -r 'X'` returns nothing" was satisfied-then-defeated by a *comment*
explaining why X is forbidden. All three were reworded.

**Judgment: yes, this is a systemic quality risk and worth recording.** Three
occurrences in one phase is a pattern, not bad luck, and the failure mode is
adversarial in exactly the wrong direction — the criterion gets *weaker* as the
codebase gets better documented, because this project's house style is heavy
explanatory comments (see `crash.go`, `flags.go`, `exit.go` — every non-obvious
decision carries a paragraph). A grep-negative criterion and a
comment-your-reasoning convention are in permanent conflict here.

**Recommended learning:** grep-negative acceptance criteria must exclude comment
lines, or better, assert on a structural property instead of textual absence. The
phase already contains the superior pattern: `newFlagSet` is deliberately the
only `flag.NewFlagSet` call site *and* `TestFlagsRegisteredOnEverySubcommand`
asserts the resulting behavior rather than the absence of a string. Prefer that
shape. Where a textual check is genuinely the only option, scope it with
`--include` and strip comments first.

### 5. 03-05 deviation: `guard(body func() int)` instead of `defer recover()` inside `run()` — ACCEPTED, verified

Verified both claims:

- **Exactly one `os.Exit` repo-wide in non-test code.**
  `grep -rn "os\.Exit" --include="*.go" .` excluding tests returns a single hit:
  `cmd/adoBizInt/main.go:23`. This is the load-bearing invariant — a surviving
  process exit inside `run()` would skip the deferred recover for that path and
  silently disable OUT-05.
- **Panic recovery is tested through the real handler.** `TestPanicExitsThree`
  and `TestGuardPassesThroughWhenNothingPanics` drive a real panic through the
  real `guard`, plus three tests on the record's shape and its credential
  hygiene.

**Judgment: the deviation is an improvement, not a compromise.** The stated
reason is correct — `run()` calls `config.Load` and `ado.NewClient`, so no test
can enter it without live credentials, which would have left the recover path as
untested dead code. Extracting `guard` makes the handler reachable from a test
while covering strictly the same set of paths, since every path in `run` executes
inside the `guard` call. The plan text described an implementation; the executor
delivered the behavior with better test reachability. Accepted.

## Anti-Pattern Scan

No `TBD`, `FIXME` or `XXX` markers in files modified by this phase. `go vet` clean.
No stub returns, no orphaned artifacts, no hollow props: every one of the eight
dispatch entries is proven to call its own distinct client method by
`TestDispatchEachVerbCallsItsOwnMethod`.

## Verdict

**PASS-WITH-CONCERNS.**

The phase goal — "an AI agent can query every existing read operation and parse
the result" — is achieved. Eight read verbs dispatch to eight distinct methods,
`--json` emits one parseable document on a stdout no other writer can touch, four
exit codes discriminate the outcomes an agent must distinguish, and both the
panic and interrupt paths produce a structured record rather than an ambiguous
empty result. That last property is the project's stated core value, and it holds
under every failure mode this phase introduced.

Three items travel with it:

1. One open UAT check (zero-argument TUI launch) — needs a human with a live PAT.
2. Split the packed Phase 3 traceability rows in REQUIREMENTS.md so the summary
   table stops lying, and so Phases 4 and 5 do not inherit the same failure.
3. File the 0644 log finding as HARD-06 before Phase 5 planning starts, or it
   will not be picked up.

None of the three blocks proceeding to Phase 4. Item 3 must be filed before
Phase 5 is planned.

---

_Verified: 2026-09-09_
_Verifier: gsd-verifier_
