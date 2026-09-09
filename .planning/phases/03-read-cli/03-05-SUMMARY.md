---
phase: 03-read-cli
plan: 05
subsystem: cli
tags: [go, context, signals, panic-recovery, crash-record, tdd, stdlib]

requires:
  - phase: 03-read-cli
    provides: run() int owning every exit path, readClient seam, dispatch table, injectable writers, single-marshal-single-write emitJSON
  - phase: 03-read-cli
    provides: --timeout registered on every subcommand via newFlagSet, parsed but unapplied
  - phase: 02-read-correctness
    provides: fetchctx.New, classifyFailure mapping cancellation before the timeout branch
provides:
  - "fetchctx.NewWithParent(parent, budget), the cancellable-ancestor fan-out context constructor"
  - "--timeout applied as a per-invocation fan-out deadline, downstream of config validation"
  - "signal.NotifyContext wiring so SIGINT/SIGTERM cancels an in-flight read"
  - "cmd/adoBizInt/crash.go: the {\"error\":{kind,message,log_path}} crash record and its single-write emitter"
  - "guard(body func() int), the deferred-recover wrapper mapping a panic to exit 3"
  - "loggingReady, gating the stack write on a successful logging init"
affects: [04-write-cli]

actuals:
  tokens: 18000
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "the zero-budget fallback is defined once in fetchctx and every read enters through it"
    - "an interrupt is detected from the parent context's Err, never from a failure kind"
    - "a crash is a distinct top-level key, never a synthetic entry inside the result envelope"
    - "a credential-bearing diagnostic is gated on a sink known to be a file"

key-files:
  created:
    - cmd/adoBizInt/crash.go
    - cmd/adoBizInt/crash_test.go
    - internal/ui/fetchctx/fetchctx_test.go
  modified:
    - internal/ui/fetchctx/fetchctx.go
    - cmd/adoBizInt/main.go
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/main_test.go

key-decisions:
  - "NewWithParent holds the zero-budget fallback and New delegates to it, so an explicit --timeout 0s means the default budget rather than an unbounded read (D-P7)"
  - "The fan-out budget is captured into a package-level readBudget in run() after validation passes; --timeout replaces it inside runRead, which is where the flag is actually parsed"
  - "The interrupt check keys on the parent context's Err rather than the failure kind, because classifyFailure deliberately maps a cancellation to the generic kind"
  - "The interrupt check runs before any stdout write, so a signal landing between the read returning and the output being written yields the crash record instead of the envelope, never both"
  - "guard wraps run rather than the recover living inside run, because run loads config and constructs a live client and is therefore unreachable from a test; coverage is identical since every path in run is inside the call"
  - "The stack write is gated on loggingReady, whose zero value is false, so a panic before initialisation writes no stack anywhere"
  - "The crash record has exactly three fields and a test asserts the count, so a later debuggability edit cannot quietly add a stack"

patterns-established:
  - "A grep-based acceptance criterion constrains prose as well as code; comments near a guarded symbol are worded to avoid spelling it"
  - "Cancellation is driven through an injected already-cancelled context rather than a real signal, keeping the test hermetic"
  - "A credential fixture is closed over by the panicking fake, so a stack leak would be caught by a substring assertion rather than forbidden only by comment"

requirements-completed: [CLI-04, OUT-04, OUT-05]

coverage:
  - id: D1
    description: "--timeout bounds a CLI invocation and is applied after config validation, so a value below poll_interval is accepted"
    requirement: CLI-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestTimeoutOverridesConfiguredBudgetAfterValidation"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestTimeoutEqualToPollIntervalIsAccepted"
        status: pass
    human_judgment: false
  - id: D2
    description: "An omitted --timeout uses the configured fetch budget; --timeout 0s falls back to the fetchctx default rather than meaning unbounded"
    requirement: CLI-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestTimeoutOmittedUsesConfiguredBudget"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestTimeoutZeroFallsBackToDefaultBudget"
        status: pass
      - kind: unit
        ref: "internal/ui/fetchctx/fetchctx_test.go#TestNewWithParentZeroBudgetStillHasDeadline"
        status: pass
    human_judgment: false
  - id: D3
    description: "An unparseable --timeout exits 3 with usage on stderr and nothing on stdout"
    requirement: OUT-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestTimeoutUnparseableExitsThreeWithCleanStdout"
        status: pass
    human_judgment: false
  - id: D4
    description: "The zero-budget default is defined in exactly one place and no CLI file builds its own timeout"
    requirement: CLI-04
    verification:
      - kind: other
        ref: "grep -c 'context.Background()' internal/ui/fetchctx/fetchctx.go == 1"
        status: pass
      - kind: other
        ref: "grep -rc 'context.WithTimeout' cmd/adoBizInt/ --include='*.go' == 0 for every file"
        status: pass
      - kind: unit
        ref: "internal/ui/fetchctx/fetchctx_test.go#TestNewIsUnchanged"
        status: pass
    human_judgment: false
  - id: D5
    description: "Cancelling the signal parent cancels the derived read context"
    requirement: OUT-05
    verification:
      - kind: unit
        ref: "internal/ui/fetchctx/fetchctx_test.go#TestNewWithParentPropagatesParentCancellation"
        status: pass
      - kind: other
        ref: "grep -c 'signal.NotifyContext' cmd/adoBizInt/main.go == 1; grep -rc 'signal.Notify(' cmd/adoBizInt/ == 0 for every file"
        status: pass
    human_judgment: false
  - id: D6
    description: "An interrupted read exits 3 and emits a single top-level error object with kind interrupt naming the log path, never a result envelope"
    requirement: OUT-05
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestInterruptExitsThree"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestInterruptEmitsDistinctErrorObject"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestInterruptRecordShape"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestInterruptRecordNotEmittedOnCleanRead"
        status: pass
    human_judgment: false
  - id: D7
    description: "A panic anywhere inside run is recovered, exits 3, and emits one {\"error\":{\"kind\":\"panic\",...}} document on stdout"
    requirement: OUT-05
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestPanicExitsThree"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestPanicEmitsDistinctErrorObject"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestGuardPassesThroughWhenNothingPanics"
        status: pass
      - kind: other
        ref: "grep -rho 'os\\.Exit' cmd/adoBizInt/ --include='*.go' | wc -l == 1; grep -c 'recover()' cmd/adoBizInt/main.go == 1"
        status: pass
    human_judgment: false
  - id: D8
    description: "The crash record carries no stack and no credential material, on both the logging-succeeded and logging-failed branches"
    requirement: OUT-05
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestPanicRecordCarriesNoStackOrCredential"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/crash_test.go#TestPanicWithFailedLoggingWritesNoStackToStderr"
        status: pass
      - kind: other
        ref: "grep -n 'debug.Stack' cmd/adoBizInt/crash.go -- single site, passed only to logging.Logger()"
        status: pass
    human_judgment: false
  - id: D9
    description: "A real SIGINT delivered to a running invocation against a live Azure DevOps organisation cancels the fan-out and produces the interrupt record"
    verification: []
    human_judgment: true
    rationale: "The automated tests drive cancellation through an injected already-cancelled context, which proves the dispatch path's response but not the OS signal delivery wiring. Confirming that Ctrl-C during a real multi-project fan-out produces exit 3 and one interrupt document needs a human at a terminal with a live PAT. Belongs in the phase UAT."

duration: 29 min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 05: Deadline, Interrupt and Crash Record Summary

**`--timeout` applied downstream of the TUI-oriented validation rule, `signal.NotifyContext` cancelling an in-flight fan-out, and a distinct `{"error":{"kind":...}}` document that makes a panic or an interrupt legible to an agent instead of looking like an empty result.**

## Performance

- **Duration:** 29 min
- **Tasks:** 3 (all TDD)
- **Files modified:** 8 (3 created, 5 modified)

## Accomplishments

- **The last honesty debt of the phase is paid.** 03-02 shipped `--timeout` parsed and discarded and said so; 03-04 restated it as 03-05's to settle. The flag now bounds the read, and the help text no longer says "applied in a later plan".
- **The ordering is proven, not asserted.** `TestTimeoutOverridesConfiguredBudgetAfterValidation` first checks that a 5s fetch budget written into the config *does* fail validation, then checks the same value passed as `--timeout` succeeds. Without the first half the test would pass just as well if the validation rule had been quietly deleted.
- **`NewWithParent` holds the zero-budget fallback and `New` delegates.** The alternative — a CLI-local `context.WithTimeout` — would have put a second copy of "an unset budget is not an unbounded read" in the tree, which is the exact failure `fetchctx` was extracted to remove. `--timeout 0s` therefore means the default budget, never no deadline.
- **The interrupt is detected from the parent's `Err()`, not the failure kind.** `classifyFailure` maps a cancellation to the generic kind deliberately, so a user-initiated abort is never reported as a server timeout — which means the kind carries no signal information at all and only the parent context can answer the question.
- **The interrupt check sits before any stdout write.** That is what discharges the plan's backstop truth: a signal landing between the read returning and the output being written produces the crash record *instead of* the envelope, so there is exactly one top-level document either way, never two.
- **The crash record has exactly three fields, and a test asserts the count.** `TestPanicRecordCarriesNoStackOrCredential` fails if a fourth key appears, so a later "just add the stack for debuggability" edit is caught rather than merely discouraged by a comment.
- **The residual disclosure path named in T-03-14 is closed by construction.** `logging.Logger()` falls back to `slog.Default()`, which writes to **stderr**, and 03-01 keeps `logging.Init()` non-fatal — so that branch is live, not theoretical. The stack write is gated on `loggingReady`, and a forced-init-failure test captures the process stderr through a pipe and asserts neither the credential fixture nor a stack marker reaches it.
- **The credential fixture is closed over by the panicking fake** rather than merely named in a comment. A Go stack renders function arguments, so if the handler ever put a stack on stdout the substring assertion would catch the actual disclosure.

## Task Commits

1. **Task 1 RED: failing `fetchctx.NewWithParent` tests** — `a3759f9` (test)
2. **Task 1 GREEN: `--timeout` through `NewWithParent`** — `1555f13` (feat)
3. **Task 2 RED: failing interrupt-record tests** — `c985cf0` (test)
4. **Task 2 GREEN: SIGINT wiring and interrupt record** — `3899227` (feat)
5. **Task 3 RED: failing panic-recovery tests** — `98f4cb2` (test)
6. **Task 3 GREEN: `guard` and `handlePanic`** — `99465d2` (feat)

## Files Created/Modified

- `internal/ui/fetchctx/fetchctx.go` — `NewWithParent` added; `New` delegates; package doc reworded so the one-site grep stays mechanical
- `internal/ui/fetchctx/fetchctx_test.go` — budget application, the zero/negative fallback, parent-cancellation propagation, and the `New`-unchanged guard
- `cmd/adoBizInt/crash.go` — `crashRecord`, `crashEnvelope`, `writeCrashRecord`, `loggingReady`, `handlePanic`
- `cmd/adoBizInt/crash_test.go` — four interrupt tests and five panic tests, including the forced-logging-failure stderr capture
- `cmd/adoBizInt/main.go` — `guard` wrapping `run`; `signal.NotifyContext` supplying the read's parent; `readBudget` captured after validation; `initLogging` records its outcome
- `cmd/adoBizInt/dispatch.go` — `readBudget`; the `--timeout` override; the read context derived via `NewWithParent`; the pre-write interrupt branch
- `cmd/adoBizInt/flags.go` — `--timeout` help text and the `defaultTimeout` comment corrected now that the flag is applied
- `cmd/adoBizInt/main_test.go` — five `--timeout` behaviour tests and the deadline-capturing fake helper

## Decisions Made

Beyond the plan's D-P5..D-P7, three judgment calls were made during execution:

- **`readBudget` is a package-level variable rather than an assignment to `cfg.FetchTimeout`.** The plan describes assigning the override into the config value, but the flag is parsed inside `runRead`, per subcommand, and `cfg` does not reach there. Pre-scanning `os.Args` in `run()` to find `--timeout` would duplicate flag parsing and diverge from the FlagSet on the first edge case. `run()` therefore captures the validated `cfg.FetchTimeout` into `readBudget` after validation, and `runRead` replaces it when the flag was given. The stated truth — the override is applied after validation and never measured against the `poll_interval` rule — holds exactly, and is what the ordering test asserts.
- **`guard(body func() int)` wraps `run` instead of the `defer recover()` living inside `run`.** `run` loads configuration and constructs a live Azure DevOps client, so no test can reach a panic raised inside it; a recover buried there would be shipped untested, which for a handler whose whole job is to fire on the unexpected path is the wrong trade. `guard` lives in `main.go` (so the one-`recover()` criterion still guards the entry point), `main()` remains a single `os.Exit` statement, and every path inside `run` is inside the call — coverage is identical and the handler is exercised by five real tests.
- **`loggingReady` is a separate flag rather than a `logging.Logger() != slog.Default()` comparison.** The logging package caches its logger in a package variable and returns the default on a nil, so the comparison would be an inference about another package's internal state. Recording the init result at the one call site is a fact rather than a deduction, and its zero value is the safe branch.

## Deviations from Plan

### Adjusted acceptance criterion

**1. [Scope] `grep -rn 'context.Background()' internal/ui/ | grep -v fetchctx | grep -v _test.go` cannot return nothing**

- **Found during:** Task 1 verification
- **Issue:** Thirteen pre-existing sites remain across `releases.go`, `prs.go`, `workitems.go`, `detail.go`, `pipelines.go` and `trigger_modal.go`. All are write call sites (approve, reject, complete, queue, update state, download attachment) plus two bare-type detail reads. The `fetchctx` package doc states explicitly that write call sites "deliberately do not use this: they keep their own contexts and are Phase 4's scope", so the criterion as literally written is unsatisfiable without doing Phase 4's work.
- **Resolution:** The criterion was applied to its actual subject — the fan-out read paths. `grep -rn 'fetchctx.New' internal/ui/` returns exactly eight sites, one per view read, confirming every `Result[T]` fan-out read enters through the package. No new `context.Background()` site was introduced by this plan.
- **Files modified:** none
- **Verification:** eight `fetchctx.New` call sites; the thirteen remaining sites are byte-identical to their pre-plan state.

**2. [Rule 3 - Blocking] The `fetchctx` package doc defeated its own one-site grep**

- **Found during:** Task 1 verification
- **Issue:** `grep -c 'context.Background()' internal/ui/fetchctx/fetchctx.go` must return 1. The package doc explained the invariant by spelling the constructor, so the count was 2 — the comment describing the guard was breaking it.
- **Fix:** The doc was reworded to say "names the background context", with a sentence recording *why* the wording avoids the literal so a future editor does not helpfully restore it.
- **Files modified:** internal/ui/fetchctx/fetchctx.go
- **Verification:** the grep returns 1; `go test ./internal/ui/fetchctx/...` green.
- **Committed in:** `1555f13`

This is the **third** time in this phase a grep-based guard has been defeated by prose (03-02's `lipgloss|bubble`, 03-04's `cfg.`, and now this one). The pattern is settled enough to state as a rule: a criterion of the form "grep returns nothing" or "grep returns exactly N" constrains the comments as much as the code, and any comment written next to the guarded symbol must be worded with the grep in mind.

---

**Total deviations:** 2 (1 criterion scoped to its actual subject, 1 Rule 3 blocking fix).
**Impact on plan:** None on behaviour. Neither changed what any code does.

## Issues Encountered

None. Each task's RED phase failed as intended — Tasks 1 and 3 on the undefined new symbol, Task 2 on its assertions with the build intact — and each went green on the first GREEN commit.

## TDD Gate Compliance

All three tasks carried `tdd="true"` and all three completed both gates:

| Task | RED | GREEN | REFACTOR | Status |
|------|-----|-------|----------|--------|
| 1 | `a3759f9` | `1555f13` | — | Pass |
| 2 | `c985cf0` | `3899227` | — | Pass |
| 3 | `98f4cb2` | `99465d2` | — | Pass |

No REFACTOR commit was needed for any task. Task 2's RED is the strongest of the three: `crash_test.go` compiled against the existing dispatch path and failed on the JSON assertions rather than on a missing symbol, so the tests were proven to be exercising the real code before the implementation existed.

## Threat Flags

None. No new network endpoint, auth path, file access pattern or schema change at a trust boundary was introduced beyond those the plan's threat register already covers.

**Carried forward, not closed:** T-03-15 remains **accepted, not mitigated**. The full stack is written to `~/.config/adoBizInt/adoBizInt.log`, which `logging.go:24` opens with mode `0644`. That stack may render the PAT, so the log is a world-readable credential-bearing artifact. Tightening the mode to `0600` is a Phase 5 hardening item and must be tracked there; this plan added no control over it.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Phase 03 is complete. Every read verb is bounded by a deadline, cancellable by a signal, and incapable of reporting a crash as an empty result. The three artefacts Phase 04 (write CLI) inherits are `guard`, which already wraps every future write path; the crash record, which write commands should reuse rather than re-invent; and `fetchctx.NewWithParent`, which the package doc still scopes to reads and which Phase 04 will need to extend or mirror for write contexts.

**Carried to phase UAT:** real SIGINT delivery during a live fan-out (coverage entry D9). The dispatch path's response to cancellation is tested; the OS signal wiring itself needs a human pressing Ctrl-C against a real organisation.

## Self-Check: PASSED

- All three created files exist on disk.
- All six task commit hashes resolve in `git log`.
- `go build ./... && go vet ./... && go test ./...` all green.
- `grep -rho 'os\.Exit' cmd/adoBizInt/ --include='*.go' | wc -l` returns 1.
- `grep -c 'recover()' cmd/adoBizInt/main.go` returns 1.
- `grep -c 'context.Background()' internal/ui/fetchctx/fetchctx.go` returns 1.
- `grep -c 'signal.NotifyContext' cmd/adoBizInt/main.go` returns 1; `signal.Notify(` returns 0 in every file.
- `grep -rc 'context.WithTimeout' cmd/adoBizInt/ --include='*.go'` returns 0 for every file.
- The single `debug.Stack()` site in `crash.go` passes its result only to `logging.Logger()`.
- The build artifact `./adoBizInt` was not produced or committed; `git status` is clean apart from the untracked `.gsd/` runtime directory.

---
*Phase: 03-read-cli*
*Completed: 2026-09-09*
