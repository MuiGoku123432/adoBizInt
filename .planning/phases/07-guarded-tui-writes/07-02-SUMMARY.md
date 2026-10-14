---
phase: 07-guarded-tui-writes
plan: 02
subsystem: guard
tags: [go, refactor, guarded-writes, audit, ast, tdd]

requires:
  - phase: 07-01
    provides: "internal/fetchctx (NewWithParent) and the CLI test lock manifests plus inventory"
provides:
  - "internal/guard: Run, Op, Options, Report, FieldChange, Target, outcome and actor constants"
  - "internal/guard: DefaultAuditPath, OpenAuditSink (creates 0700 dir / 0600 file, no chmod of existing)"
  - "internal/guard: Client interface, field vocabulary, VoteApproved, Effort, six Op constructors"
  - "internal/guard/guardtest: Client fake for guard and TUI tests"
  - "cmd/adoBizInt runWrite as a thin adapter over guard.Run"
affects: [07-03, 07-04, 07-05, internal/ui, cmd/adoBizInt]

actuals:
  tokens: 26900
  tasks: 2
  commits: 3
plan_head_before: 56f09c2d8dfa283f6eb68d643423b22e1adbbc3e
plan_head_after: ba525d9d046cf3ae8bf9ad481bf138251468c1a2

tech-stack:
  added: []
  patterns:
    - "Shared guard returns a Report and never prints or exits; each surface maps it to its own output"
    - "Panic in Mutate is recovered by the guard and returned as data (Report.Panic), never re-panicked"
    - "AST scan with an in-memory tripwire snippet to prove a structural test is not vacuous"

key-files:
  created:
    - internal/guard/guard.go
    - internal/guard/audit.go
    - internal/guard/ops.go
    - internal/guard/guardtest/fake.go
    - internal/guard/guard_test.go
    - internal/guard/audit_test.go
    - internal/guard/structural_test.go
    - cmd/adoBizInt/guardnames_test.go
  modified:
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/audit.go
    - cmd/adoBizInt/exit.go
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/crash_test.go
    - cmd/adoBizInt/flags.go

key-decisions:
  - "internal/guard is imported as writeguard in package main, because package main already declares func guard (the top-level panic boundary used by locked patleak_test.go)"
  - "Run's no-op branch always returns a non-nil empty Changes slice, even when Plan returns nil, so the CLI envelope keeps [] (P-09)"
  - "The CLI maps Report.Panic to handlePanic(exit 5) and keeps its own deferred recover around report emission, so a panic while printing still exits 5 with no second audit record"
  - "writeExitCodeFor(outcome, err) now only separates auth from other failures; the 5xx, timeout, no-status and cancelled-parent rules live only in guard.classify"

patterns-established:
  - "Surface adapter: build a guard.Op from surface inputs, call guard.Run with Options{Budget, Confirm, Open, Actor}, map the Report"
  - "guardtest.Client: nil func means t.Fatalf by name, the no-mutation assertion; no call counters"

requirements-completed: [GUARD-01, GUARD-06, GUARD-02]

coverage:
  - id: D1
    description: "The guarded-write sequence exists once, in internal/guard Run; the CLI's six write verbs call it and every locked CLI test passes unchanged"
    requirement: GUARD-01
    verification:
      - kind: unit
        ref: "go test -race -count=1 ./cmd/adoBizInt/ (incl. TestPATNeverLogged, TestWriteGuardMatrix, TestPanicAfterWriteIssuedIsAudited, TestPanicBeforeWriteIssuedExitsThree, TestAuditAppendFailureReportsUnknown, TestUnwritableAuditSinkPreventsMutation, TestInterruptedWriteReportsOutcomeUnknownNotNothingRan)"
        status: pass
      - kind: unit
        ref: "internal/guard/structural_test.go#TestRunRecoverPrecedesMutate"
        status: pass
    human_judgment: false
  - id: D2
    description: "CLI test lock intact: both 07-01 manifests verify and the only inventory test missing is TestWriteRecoverPrecedesMutate"
    requirement: GUARD-06
    verification:
      - kind: other
        ref: "LC_ALL=C shasum -a 256 -c 07-cli-tests.sha256 && shasum -a 256 -c 07-cli-audit-test.sha256 && comm -23 07-cli-test-inventory.txt <(go test ./cmd/adoBizInt/ -list '.*' | grep '^Test' | sort) == TestWriteRecoverPrecedesMutate"
        status: pass
    human_judgment: false
  - id: D3
    description: "Guard sequence pinned in internal/guard: sink before pre-read, no-op refusal, unaudited dry run (D-04), one executed record, outcome classification, CR-01 downgrade, panic recorded once as unknown"
    requirement: GUARD-01
    verification:
      - kind: unit
        ref: "internal/guard/guard_test.go#TestRunOpensSinkBeforePreRead|TestRunRefusesNoOpWithoutMutation|TestRunUnconfirmedIsDryRunAndUnaudited|TestRunExecutedAppendsOneRecord|TestRunClassifiesOutcomes|TestRunAuditAppendFailureDowngradesToUnknown|TestRunMutatePanicRecordsUnknownOnce|TestOpNamesArePinned"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every guarded write runs under a bounded context even with no parent deadline and a zero budget"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/guard/guard_test.go#TestRunBoundsContextWithoutParentDeadline"
        status: pass
    human_judgment: false
  - id: D5
    description: "New audit files are 0600 and new audit directories 0700; existing modes are left alone (D-11)"
    verification:
      - kind: unit
        ref: "internal/guard/audit_test.go#TestOpenAuditSinkCreatesOwnerOnlyFileAndDir|TestOpenAuditSinkLeavesExistingModeAlone"
        status: pass
    human_judgment: false
  - id: D6
    description: "internal/guard never writes stdout and never exits: AST scan with tripwire plus a runtime pipe capture"
    requirement: GUARD-01
    verification:
      - kind: unit
        ref: "internal/guard/structural_test.go#TestGuardNeverPrintsOrExits (incl. tripwire); internal/guard/guard_test.go#TestRunNeverWritesStdout"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-10-14
status: complete
---

# Phase 7 Plan 02: Guard extraction Summary

**The guarded-write sequence now lives once, in `internal/guard` Run. Run opens the audit sink 0700/0600 before the pre-read, bounds the context, recovers a Mutate panic as data, and never prints or exits. The CLI's six write verbs are thin adapters over it, and every locked CLI test passes byte-unchanged.**

## Performance

- **Duration:** about 8 min of execution (after reading the context)
- **Started:** 2026-10-14T03:37:19Z
- **Completed:** 2026-10-14T03:45:19Z
- **Tasks:** 2
- **Files modified:** 14 (8 created, 6 modified)

## Final guard API

- `guard.Run(ctx context.Context, op guard.Op, opts guard.Options) (report guard.Report)`
- `guard.Op{Name string; Target guard.Target; Plan func(ctx) ([]guard.FieldChange, error); Mutate func(ctx) error}`
- `guard.Options{Budget time.Duration; Confirm func([]guard.FieldChange) bool; Open func() (*os.File, error); Actor string; UserApproved bool}`. A nil Confirm means a dry run. A nil Open means `OpenAuditSink(DefaultAuditPath())`, evaluated at call time.
- `guard.Report{Changes; Outcome; SinkErr; PreReadErr; MutateErr; AuditErr; Panic any}`
- `OutcomeDryRun/NoOp/Executed/Failed/Unknown`, `ActorTUI/ActorCLIHuman/ActorAgent`
- `FieldChange{Field, From, To}`, `Target{Project, Repo (omitempty), ID}`. The json tags are unchanged.
- `Client`: the eleven methods of the old writeClient, with the same signatures and doc comments. `Effort{OriginalEstimate, Remaining, Completed *float64}`.
- `SetState`, `ApprovePR`, `CompletePR`, `ApproveRelease`, `RejectRelease`, `QueuePipeline`. Approve and reject both go through the unexported `releaseApproval`.
- `DefaultAuditPath()`, `OpenAuditSink(path)`, and the field constants plus `VoteApproved`.
- `guardtest.Client{T, ...Func}` in `sentinovo.ai/bizInt/internal/guard/guardtest`. Its conformance check is `var _ Client = (*guardtest.Client)(nil)` in guard_test.go.

**Line counts:** guard.go 274, audit.go 104, ops.go 308. ops.go is slightly over the 300-line target. Almost all of it is the moved Client doc comments and the six constructors, and there is no real seam to split along. cmd/adoBizInt/write.go went from 642 to 388 lines.

## Accomplishments

- `internal/guard` holds the only copy of the ten-step order: audit open, pre-read, no-op refusal, confirm gate, the deferred recover immediately before Mutate, classify, audit append, and the CR-01 downgrade. Every decision reference moved with its comment.
- In `cmd/adoBizInt`, `runWrite` parses arguments, checks arity, builds the Op, calls `writeguard.Run`, and maps the Report. The stderr strings are byte-identical, and logging happens inside the guard with the same messages and keys.
- The audit opener creates the directory 0700 and the file 0600 (D-11). Nothing is chmodded on open.
- Task 2 added 13 guard tests and the shared fake. All 14 named tests pass.

## Task Commits

1. **Task 1 (tracer, TDD).** RED `cac0d84` (test): structural pin, plus a package-doc-only guard.go. GREEN `6b7a1d9` (feat): the guard package and the CLI adapter. The tracer gate re-ran both `<automated>` verifies on the committed tree and they passed.
2. **Task 2.** `ba525d9` (test): guardtest fake, guard_test.go, audit_test.go, and TestGuardNeverPrintsOrExits.

## TDD notes

- **RED (Task 1):** `go test -v ./internal/guard/ -run '^TestRunRecoverPrecedesMutate$'` printed `structural_test.go:33: func Run not found in guard.go` and `--- FAIL: TestRunRecoverPrecedesMutate`, exit 1. Semantic assessment: the target test compiled, ran, and failed on its own planned assertion (Run absent). It was not a build, load, or zero-test failure, so this is a valid RED. The `gsd_run check tdd-red-evidence` classifier has no adapter for `go test` output (TAP, JUnit, swift-testing and unittest only), so the evidence is recorded here instead.
- **GREEN (Task 1):** the guard package plus the adapter. The pin passes and the full CLI suite passes.
- **Task 2 had no RED by design.** Its tests cover behaviour Task 1 already implemented, so they passed on first run. To show they are not vacuous, four throwaway mutations were applied to the committed guard, each was caught, and each file was restored with `git checkout -- <file>`:
  - mode 0755/0644 failed TestOpenAuditSinkCreatesOwnerOnlyFileAndDir
  - a nil Confirm treated as yes failed TestRunUnconfirmedIsDryRunAndUnaudited
  - an injected `fmt.Println` failed both TestGuardNeverPrintsOrExits and TestRunNeverWritesStdout
  - Mutate given the parent context failed TestRunBoundsContextWithoutParentDeadline
- No REFACTOR commit was needed.

## The one sanctioned crash_test.go edit

`TestWriteRecoverPrecedesMutate` was deleted from `cmd/adoBizInt/crash_test.go`, along with its now-unused `go/ast`, `go/parser` and `go/token` imports. Nothing else in the file changed. Its guarantee is re-asserted, and strengthened, by `internal/guard/structural_test.go#TestRunRecoverPrecedesMutate`. The new pin also requires that the recover calls `appendAuditRecord` and never calls `panic`.

## Stack-trace note

`handlePanic` still logs `debug.Stack()` when logging is ready. For a panic inside Mutate, though, that stack is now the CLI-side stack at the point where `runWrite` handles `Report.Panic`, not the stack of the panicking goroutine frame, because the guard recovers the panic and returns it as data. The crash record on stdout, the exit code 5, and the single unknown audit record are unchanged. If Mutate-site stacks are wanted later, the guard would have to capture `debug.Stack()` in its recover and carry it in the Report. That was not done here (YAGNI).

## Gate output

```
cmd/adoBizInt/fakes_test.go: OK
cmd/adoBizInt/filter_test.go: OK
cmd/adoBizInt/main_test.go: OK
cmd/adoBizInt/patleak_test.go: OK
cmd/adoBizInt/usage_test.go: OK
cmd/adoBizInt/write_test.go: OK
cmd/adoBizInt/writeverbs_test.go: OK
cmd/adoBizInt/audit_test.go: OK
comm -23 inventory vs current: TestWriteRecoverPrecedesMutate   (only)
```

Module gate: `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`. All 13 packages report ok, and gofmt is clean.

## Files Created/Modified

- `internal/guard/guard.go`: Run, types, outcome and actor constants, classify
- `internal/guard/audit.go`: DefaultAuditPath, OpenAuditSink (0700/0600), the five-key auditRecord, newAuditRecord, appendAuditRecord
- `internal/guard/ops.go`: Client, vocabulary, Effort, six constructors, diffWorkItem, appendFloatChange
- `internal/guard/guardtest/fake.go`: shared fail-by-name fake
- `internal/guard/guard_test.go`, `audit_test.go`, `structural_test.go`: 14 tests
- `cmd/adoBizInt/write.go`: aliases `fieldChange`/`writeTarget`, `writeOp{name, build}`, the runWrite adapter, per-verb builders
- `cmd/adoBizInt/audit.go`: `auditPath`, `openAuditSink`, `auditOpen` over the guard
- `cmd/adoBizInt/exit.go`: outcome aliases and `writeExitCodeFor(outcome, err) int`
- `cmd/adoBizInt/dispatch.go`: writeClient removed; `client` embeds `writeguard.Client`; the releases entries use the new builders
- `cmd/adoBizInt/guardnames_test.go`: test-only vocabulary aliases
- `cmd/adoBizInt/crash_test.go`: the relocated test removed
- `cmd/adoBizInt/flags.go`: dead `hasEffort` removed

## Decisions Made

See key-decisions in the frontmatter. The import alias is the only decision the plan did not anticipate.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] internal/guard imported as `writeguard` in package main**
- **Found during:** Task 1
- **Issue:** package main declares `func guard` (main.go:38), the top-level panic boundary, and the locked `patleak_test.go` calls it. Go forbids an import name that collides with a package-level declaration, so `import "…/internal/guard"` would not compile in cmd.
- **Fix:** imported it as `writeguard` in write.go, audit.go, exit.go, dispatch.go and guardnames_test.go. The plan's key_link patterns (`guard\.Run\(ctx`, `guard\.OpenAuditSink\(auditPath\)`) still match as substrings.
- **Verification:** build, vet, the full CLI suite and the manifests all pass.
- **Committed in:** 6b7a1d9

**2. [Rule 1 - Cleanup caused by the move] removed the dead `writeFlags.hasEffort`**
- **Found during:** Task 1
- **Issue:** its only caller, setStateOp's mutate, moved into `guard.SetState`, which checks the Effort pointers directly. That left the method unused. flags.go was not in files_modified.
- **Fix:** deleted the 4-line method. No test referenced it.
- **Committed in:** 6b7a1d9

**3. [Rule 2 - Correctness] the no-op Report always carries a non-nil empty slice**
- **Found during:** Task 1
- **Issue:** the interfaces contract says "Changes set to the empty slice". Passing Plan's slice through would emit `null` if a future Op's Plan returned nil.
- **Fix:** `Report{Changes: []FieldChange{}, Outcome: OutcomeNoOp}`. TestRunRefusesNoOpWithoutMutation covers both the nil and the empty subcase.
- **Committed in:** 6b7a1d9

---

**Total deviations:** 3 auto-fixed (1 blocking, 1 cleanup, 1 correctness).
**Impact on plan:** none on behaviour or scope. The CLI output and the audit format are unchanged.

## Issues Encountered

None.

## Known Stubs

None. `Options.Actor` and `Options.UserApproved` are carried but not yet read by Run. That is intentional per the plan: plan 07-03 persists them and makes Run refuse an empty actor. Every Options literal in the new tests already sets `Actor: ActorTUI`.

## User Setup Required

None.

## Next Phase Readiness

- 07-03 can extend `auditRecord`, `newAuditRecord` and `Run` (actor and user_approved). `audit_test.go` in cmd is still locked until then.
- 07-04 can build Ops with the six constructors and test with `guardtest.Client`.
- In package main, the guard must keep being imported under an alias (`writeguard`).

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-14*

## Self-Check: PASSED

- FOUND: internal/guard/{guard,audit,ops}.go, internal/guard/guardtest/fake.go, internal/guard/{guard,audit,structural}_test.go, cmd/adoBizInt/guardnames_test.go
- FOUND commits (ancestors of HEAD): cac0d84, 6b7a1d9, ba525d9
- Module gate, both manifests and the inventory comm check re-run green after the final code commit
