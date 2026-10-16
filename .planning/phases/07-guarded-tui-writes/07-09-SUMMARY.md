---
phase: 07-guarded-tui-writes
plan: 09
subsystem: guard
tags: [go, bubbletea, signal, sync-waitgroup, audit, sox, cr-01, gap-closure]

requires:
  - phase: 07-guarded-tui-writes
    provides: "internal/guard.Run with classify(err, ctx.Err()) (07-02), confirm Preview/Execute (07-04), quit hold (07-08)"
provides:
  - "confirm.SetParent / confirm.Wait: a signal-aware parent for every TUI guard.Run and an in-flight WaitGroup around Execute"
  - "main's TUI branch registers signal.NotifyContext(os.Interrupt, SIGTERM), defers its stop, and calls confirm.Wait() after p.Run() before the error branch"
  - "TestExecuteInterruptedMidMutateIsAuditedUnknown, TestCancelledParentSendsNothing, TestTUIWaitsForInflightWriteBeforeExit"
  - "CR-01 recorded fixed (Source 07-09) in 07-REVIEW-DISPOSITION.md"
affects: [phase-07-reverification, guard, tui-exit, phase-13-agent-tiers]

actuals:
  tokens: 3846
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Process-level write parent set once via an exported setter, not threaded through constructors"
    - "WaitGroup Add/deferred Done inside the work function itself, never at the cmd-dispatch site"
    - "AST pin of run's top-level statements for exit wiring (rule-numbered failures)"

key-files:
  created:
    - internal/ui/confirm/inflight_test.go
    - cmd/adoBizInt/tuiexit_test.go
  modified:
    - internal/ui/confirm/batch.go
    - cmd/adoBizInt/main.go
    - .planning/phases/07-guarded-tui-writes/07-REVIEW-DISPOSITION.md

key-decisions:
  - "07-09: TUI guarded writes run under main's signal.NotifyContext via confirm.SetParent; confirm.Execute counts itself in an inflight WaitGroup and main calls confirm.Wait() after p.Run() and before its error branch (CR-01)"
  - "07-09: the signal context is not handed to the bubbletea program (G-03): its signal goroutine's blocking send would hang shutdown with a raw terminal"
  - "07-09: GUARD-01/02/04 left unchecked in REQUIREMENTS.md; commit 20b8d1c reverted them pending verification, so re-verification, not the gap executor, flips them"

patterns-established:
  - "Exit-path wiring in main is pinned structurally over run's top-level AST, with a recorded vacuity check"

requirements-completed: [GUARD-01, GUARD-02, GUARD-04]

coverage:
  - id: D1
    description: "A TUI write whose parent is cancelled mid-Mutate is classified unknown and leaves exactly one audit record (actor tui, user_approved false); Wait holds until that record is on disk"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/confirm/inflight_test.go#TestExecuteInterruptedMidMutateIsAuditedUnknown"
        status: pass
    human_judgment: false
  - id: D2
    description: "A write that starts after the parent was cancelled is refused at its pre-read, sends nothing and writes no audit record; writeParent reaches both Preview and Execute"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/confirm/inflight_test.go#TestCancelledParentSendsNothing"
        status: pass
    human_judgment: false
  - id: D3
    description: "main's TUI branch registers NotifyContext(os.Interrupt, SIGTERM), defers its stop, installs it with confirm.SetParent before p.Run, and calls confirm.Wait() after p.Run with no return in between"
    requirement: GUARD-01
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/tuiexit_test.go#TestTUIWaitsForInflightWriteBeforeExit"
        status: pass
    human_judgment: false
  - id: D4
    description: "CLI write path and locked CLI tests unchanged (GUARD-06 gate)"
    requirement: GUARD-04
    verification:
      - kind: other
        ref: "shasum -a 256 -c 07-cli-tests.sha256 and 07-cli-audit-test.sha256; comm -23 inventory check"
        status: pass
    human_judgment: false
  - id: D5
    description: "A real SIGTERM sent to a live TUI mid-write against ADO leaves one unknown record and a restored terminal"
    verification: []
    human_judgment: true
    rationale: "Needs a live terminal, a live ADO write and an OS signal; the unit tests stand the signal in with a context cancel and the AST pin proves the wiring, but no test drives bubbletea's real signal goroutine"

duration: 6min
completed: 2026-10-16
status: complete
---

# Phase 7 Plan 09: Interrupted TUI Writes Summary

**TUI guarded writes now run under main's SIGINT/SIGTERM NotifyContext through confirm.SetParent, and main calls confirm.Wait() on an in-flight WaitGroup after p.Run(), so an interrupted TUI write is classified unknown and audited before exit, as on the CLI (CR-01)**

## Performance

- **Duration:** 6 min
- **Started:** 2026-10-16T13:34:39Z
- **Completed:** 2026-10-16T13:40:53Z
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- `internal/ui/confirm/batch.go`: package-level `inflight sync.WaitGroup` and `writeParent` (initialised to the background context), exported `SetParent(ctx)` and `Wait()`. `Preview` and `Execute` both call `guard.Run(writeParent, ...)`. `Execute` opens with `inflight.Add(1)` / `defer inflight.Done()`. `Preview` is not counted (dry run, nothing sent, nothing audited).
- `cmd/adoBizInt/main.go` TUI branch: `tuiSignalCtx, stopTUISignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`, `defer stopTUISignals()`, `confirm.SetParent(tuiSignalCtx)`, then `_, err = p.Run()`, `confirm.Wait()`, and the unchanged error branch. The CLI branch is byte-identical. The program is not given the signal context (no program-context option anywhere in main.go).
- Three new tests, all passing under `-race`: the cancel-mid-Mutate audit test, the cancelled-parent sends-nothing test, and the AST pin of run's exit wiring.
- 07-REVIEW-DISPOSITION.md: CR-01 is now `fixed` with Source `07-09`; `open: 14`, `total: 15`. WR-01..WR-08 and IN-01..IN-06 remain `open` with Source `-` (out of this gap's scope by user decision).

## Task Commits

1. **Task 1 (tracer): interrupted TUI write audited unknown, main waits** -- RED `fb57667` (test), GREEN `bd3fa31` (feat)
2. **Task 2: pin exit wiring, late write sends nothing, gates, record CR-01 fixed** -- `e150326` (test)

**Plan metadata:** recorded in the docs(07-09) commit that adds this file.

RED evidence (Task 1, before batch.go/main.go changes):

```
internal/ui/confirm/inflight_test.go:23:2: undefined: SetParent
internal/ui/confirm/inflight_test.go:28:3: undefined: Wait
FAIL	sentinovo.ai/bizInt/internal/ui/confirm [build failed]
```

## Verification Evidence

### Vacuity check of TestTUIWaitsForInflightWriteBeforeExit

`confirm.Wait()` temporarily moved below the `if err != nil { ... return 1 }` branch in main.go (never committed):

```
=== RUN   TestTUIWaitsForInflightWriteBeforeExit
    tuiexit_test.go:109: rule 2: statement 23 between p.Run (22) and confirm.Wait (24) holds a return, so that path exits without waiting for an in-flight write
--- FAIL: TestTUIWaitsForInflightWriteBeforeExit (0.00s)
FAIL	sentinovo.ai/bizInt/cmd/adoBizInt
```

main.go restored from a copy of its Task 1 content; `git diff cmd/adoBizInt/main.go` empty; the test then passes:

```
--- PASS: TestTUIWaitsForInflightWriteBeforeExit (0.00s)
ok  	sentinovo.ai/bizInt/cmd/adoBizInt
```

Extra (not required by the plan, not committed): reverting only `Preview`, then only `Execute`, to the root context each fails TestCancelledParentSendsNothing (`Preview reports = [... PreReadErr:<nil> ...]` and `Mutate called after the parent was cancelled` respectively), so the test proves writeParent reaches both calls.

### Module gate

`go build ./... && go vet ./... && go test -race -count=1 ./...`: every package `ok`. `gofmt -l .` prints nothing.

### GUARD-06 gate

```
cmd/adoBizInt/fakes_test.go: OK
cmd/adoBizInt/filter_test.go: OK
cmd/adoBizInt/main_test.go: OK
cmd/adoBizInt/patleak_test.go: OK
cmd/adoBizInt/usage_test.go: OK
cmd/adoBizInt/write_test.go: OK
cmd/adoBizInt/writeverbs_test.go: OK
cmd/adoBizInt/audit_test.go: OK
comm -23 output:
TestWriteRecoverPrecedesMutate
```

`cmd/adoBizInt/tuiexit_test.go` is the only new or changed `_test.go` file under cmd/adoBizInt/. The locked TestValidateGuardsCLIAndTUI still passes (the new TUI statements call none of Validate, dispatch, ui.NewModel).

### Task 1 acceptance greps (comment lines excluded)

- batch.go: `guard.Run(context.Background()` 0, `guard.Run(writeParent` 2, `inflight.Add(1)`, `defer inflight.Done()`, `func SetParent(ctx context.Context)`, `func Wait()` each 1.
- main.go: `confirm.SetParent(tuiSignalCtx)` (line 99 of the comment-stripped file), `_, err = p.Run()` (102), `confirm.Wait()` (103), `if err != nil {` (104); `confirm.Wait()` count 1; `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` count 2; `WithContext` count 0.

### Disposition checks

All six greps from the Task 2 verify block pass (CR-01 row, 14 `| open | - |` rows, CR-01 frontmatter `disposition: fixed`, 14 `disposition: open`, `open: 14`, `total: 15`).

## Files Created/Modified

- `internal/ui/confirm/batch.go` -- inflight WaitGroup, writeParent, SetParent, Wait; both guard.Run calls take writeParent
- `internal/ui/confirm/inflight_test.go` -- TestExecuteInterruptedMidMutateIsAuditedUnknown, TestCancelledParentSendsNothing
- `cmd/adoBizInt/main.go` -- TUI NotifyContext, deferred stop, confirm.SetParent before p.Run, confirm.Wait() before the error branch; confirm import
- `cmd/adoBizInt/tuiexit_test.go` -- TestTUIWaitsForInflightWriteBeforeExit (five numbered rules, each lookup fails with t.Fatalf when nothing matches)
- `.planning/phases/07-guarded-tui-writes/07-REVIEW-DISPOSITION.md` -- CR-01 fixed, Source 07-09, open 14

## Decisions Made

- Followed design decisions G-01..G-06 as written: Add/Done inside Execute (G-01), package-level parent via SetParent (G-02), signal context not given to the bubbletea program (G-03), Wait before the error branch (G-04), NotifyContext registered just before p.Run with a deferred stop (G-05), Preview uses writeParent but is not counted (G-06).
- REQUIREMENTS.md is left as is. Commit 20b8d1c reverted GUARD-01..04 from Complete because verification found this gap. Flipping GUARD-01, GUARD-02 and GUARD-04 back is left to phase re-verification, so the gap executor does not repeat the premature marking. `requirements-completed` above lists the plan's IDs for traceability only.

## Deviations from Plan

### Notes (no code change)

**1. Plan wording correction: edge-disposition row 1 ("at most one TUI write is in flight at a time") is inaccurate**
- **Found during:** Task 1 (reading flow.go and app.go)
- **Issue:** The busy gate is per-Flow, and each write-capable tab embeds its own Flow. Writes on different tabs (for example PRs and Releases) can each have an Execute in flight at the same time, and q on a non-busy tab can end the program while another tab is still writing (the 07-08 quit hold checks only the current view's WriteBusy()).
- **Disposition:** No design change. The WaitGroup counts every concurrent Execute, so confirm.Wait() after p.Run() covers any number of in-flight writes, including the q-on-another-tab case, which this plan therefore also closes. Recorded here rather than by editing the plan.

**2. Tracer feedback gate ran in its automated form**
- **Found during:** after Task 1
- **Issue:** Auto mode is off (`_auto_chain_active: false`, no `auto_advance`), which would mean a `checkpoint:human-verify` after the tracer. `workflow.human_verify_mode` is unset, so the default `end-of-phase` mode applies (no mid-flight human-verify halts), the tracer's `<verify>` is fully automated, and the orchestrator asked for both tasks.
- **Disposition:** Re-ran the tracer verify end to end (exit 0), then expanded to Task 2. The live-signal check is listed as coverage D5 (human_judgment) for end-of-phase UAT.

---

**Total deviations:** 0 auto-fixed code deviations; 2 recorded notes.
**Impact on plan:** None on the design or the code. Note 1 widens what the fix is known to cover.

## Issues Encountered

- **Residual R1 also has a WaitGroup-reuse edge (observation, accepted with R1).** In Go's sync.WaitGroup, if main is blocked in Wait on an in-flight Execute, that Execute's Done resets the counter, and a late Execute's `inflight.Add(1)` lands before the woken waiter re-checks the state, Wait panics with "sync: WaitGroup is reused before previous Wait has returned". run's `guard` recover turns that into a crash record and exit 3. The audit guarantee still holds: every counted Execute has appended its record, and the late one has only just started (no pre-read done, nothing sent). This needs two concurrent writes plus a third starting inside a nanosecond-scale window after p.Run returned. It sits inside the accepted R1 window (T-07-38), so it is recorded, not fixed.

## Known Residuals (from the plan, unchanged)

R1 late start after Wait (timing argument, T-07-38), R2 startup signal window (T-07-40), R3 SIGKILL/SIGHUP/power loss at parity with the CLI (T-07-41), R4 silent wait after a recovered panic (T-07-39).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- CR-01, the single failed must-have in 07-VERIFICATION.md (`gaps[0]`), is closed in code and tests. Phase 7 is ready for re-verification; GUARD-01/02/04 status in REQUIREMENTS.md should be restored there.
- Still open in 07-REVIEW-DISPOSITION.md and out of this plan's scope: WR-01..WR-08, IN-01..IN-06, plus the eight human_verification items in 07-VERIFICATION.md.

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-16*

## Self-Check: PASSED

- FOUND: internal/ui/confirm/batch.go, internal/ui/confirm/inflight_test.go, cmd/adoBizInt/main.go, cmd/adoBizInt/tuiexit_test.go, .planning/phases/07-guarded-tui-writes/07-REVIEW-DISPOSITION.md
- FOUND commits: fb57667, bd3fa31, e150326
- Plan-level verification re-run before this file: module gate, gofmt, GUARD-06 manifests (8/8 OK), comm -23 inventory, disposition greps all pass
