---
phase: 06-core-value-cleanup
plan: 05
subsystem: cli
tags: [go, cli, panic, audit, exit-codes, tdd]
status: complete

requires:
  - phase: 06-02
    provides: TestValidateGuardsCLIAndTUI, the first inline parser.ParseFile in package main
provides:
  - runWrite recovers its own panics from the op.mutate boundary on and exits 5 (exitOutcomeUnknown)
  - a best-effort five-key outcome "unknown" audit record on a post-request panic, never a second record
  - handlePanic(r any, code int) int, which returns the code its caller chose
  - doc comments that no longer call exit 3 the outcome-unknown code
affects: [phase-06 verification, agents branching on exit 3 vs 5, audit.jsonl readers]

actuals:
  tokens: 4366
  tasks: 2
  commits: 4
plan_head_before: 7aa1903ebbc7a628023c5bf9e5e121ef0055f05e

tech-stack:
  added: []
  patterns:
    - "Request-issued boundary expressed by the POSITION of a deferred recover, pinned by a go/ast test"
    - "Local audited flag set before the normal append, so the panic path never double-appends"

key-files:
  created: []
  modified:
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/crash.go
    - cmd/adoBizInt/main.go
    - cmd/adoBizInt/crash_test.go

key-decisions:
  - "The deferred recover in runWrite is the statement immediately before op.mutate and after defer sink.Close(). Its position is the request-issued signal, and LIFO defers mean the sink is still open when it runs (D-12, RESEARCH Pitfall 9)"
  - "handlePanic takes the exit code from its caller: guard passes exitUnknown (3), runWrite passes exitOutcomeUnknown (5). The crash record stays on stdout with its three keys (D-18)"
  - "On a post-request panic the recover appends one outcome unknown record with only the write's command, target and changes, before writing the crash record. A local audited flag set just before the normal append prevents a second record (D-13)"

patterns-established:
  - "Panic exit codes follow the Phase 4 D-22 interrupt rule: once a mutation may be live, the exit is 5, never 3"

requirements-completed: [OUT-04, OUT-05, SAFE-02, SAFE-04]

duration: 3min
completed: 2026-10-12
---

# Phase 6 Plan 05: Write Panic Reports Outcome Unknown Summary

**A panic after a write's request may have left now exits 5 (outcome unknown) instead of 3. The published crash record still goes to stdout, and one best-effort `unknown` audit record is appended to the still-open sink. A panic in the pre-read or in a read still exits 3.**

## Performance

- **Duration:** about 3 minutes
- **Started:** 2026-10-12T21:58:57Z
- **Completed:** 2026-10-12
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- **Boundary placement (D-12).** `runWrite` now has a named result `(code int)`. A deferred recover is registered as the statement directly before `err = op.mutate(...)`. That puts it after `defer sink.Close()`, the no-op return and the dry-run return. A panic in parse, resolve or plan never reaches it and still exits 3 through `guard`. A panic from the mutate onward exits 5. The boundary is structural rather than a flag: `TestWriteRecoverPrecedesMutate` parses `write.go` and checks four things:
  - exactly one `defer sink.Close()`, one recover defer and one `op.mutate` statement
  - the close comes before the recover
  - the recover is at `iMutate-1`
  - the recover calls `handlePanic(..., exitOutcomeUnknown)`
- **Message.** The crash message reads `panic after <command> was issued; outcome unknown: <value>`. The prefix names only the command.
- **handlePanic signature.** It changed from `handlePanic(r any) int` to `handlePanic(r any, code int) int` and returns `code`. `guard` passes `exitUnknown`. The stack still goes to the log only, and only when `loggingReady` is set.
- **Audited guard (D-13).** A local `audited := false` is declared just before the recover defer. It is set to true immediately before the normal `appendAuditRecord` call. If the recover runs and `audited` is still false, it appends `newAuditRecord(writeRecord{Command, Target, Changes, Outcome: outcomeUnknown})` before calling `handlePanic`. A failed append logs the error and writes the operator-facing line `Write may have landed but was NOT recorded in <auditPath>: <err>`, and the exit is still 5. The record has no panic text, so the five keys from D-15 and the no-argv rule from T-04-01 still hold.
- **D-14 comment corrections:**
  - `handlePanic`'s doc now names both caller codes.
  - `guard`'s doc says exit 3 means nothing ran (D-08) and that a post-request write panic exits 5 without ever reaching guard.
  - `run`'s doc says startup failures return exitUnknown because nothing ran.
  - `TestPanicExitsThree`'s doc now says exit 3 (nothing ran).

## Task Commits

1. **Task 1 (tracer): post-request panic exits 5, boundary pinned**
   - `13abe29` (test, RED)
   - `9691301` (feat, GREEN)
2. **Task 2: best-effort unknown audit record**
   - `dae42dc` (test, RED)
   - `9190e40` (feat, GREEN)

## TDD Gate Compliance

**Task 1 RED** (commit `13abe29`):

```
--- FAIL: TestPanicAfterWriteIssuedExitsOutcomeUnknown
    crash_test.go:352: post-request panic = 3, want 5 (stderr: "")
--- PASS: TestPanicBeforeWriteIssuedExitsThree      (D-12 lower-boundary pin, passes by design)
--- FAIL: TestWriteRecoverPrecedesMutate
    crash_test.go:480: defer sink.Close() at 16, deferred recover at -1, op.mutate at 21; each must be found
```

**Task 1 GREEN** (commit `9691301`): all 10 tests named in the plan's verify command pass. The tracer gate re-ran that verify end to end, and it passed before Task 2 started.

**Task 2 RED** (commit `dae42dc`):

```
--- FAIL: TestPanicAfterWriteIssuedIsAudited/mutate_panics
    crash_test.go:581: audit file holds 0 records, want 1
--- FAIL: TestPanicAfterWriteIssuedIsAudited/append_fails,_still_unknown
    crash_test.go:575: stderr = "", want it to contain "Write may have landed but was NOT recorded"
--- PASS: TestPanicAfterWriteIssuedIsAudited/panic_after_the_normal_append_writes_no_second_record
--- FAIL: TestPanicAfterWriteIssuedIsAudited/cancelled_parent_still_unknown
    crash_test.go:581: audit file holds 0 records, want 1
```

**Task 2 GREEN** (commit `9190e40`):
- All 4 rows pass, along with the 9 other tests in the verify command.
- Mutation check on the audited guard: replacing `if !audited` with `if true || !audited` made row 3 fail with "audit file holds 2 records, want 1" (one executed line and one unknown line). That confirms row 3 pins the guard. The original code was restored before the commit.

**Full gate:** `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` exits 0.

## Files Created/Modified

- `cmd/adoBizInt/write.go`: named result; the recover defer immediately before `op.mutate`; the `audited` flag and the best-effort unknown append
- `cmd/adoBizInt/crash.go`: `handlePanic(r any, code int) int` and a corrected doc comment
- `cmd/adoBizInt/main.go`: `guard` calls `handlePanic(r, exitUnknown)`; corrected guard and run doc comments
- `cmd/adoBizInt/crash_test.go`: four new tests, the test-local `panicOnFirstWrite` writer, and a corrected doc comment on `TestPanicExitsThree`

## Decisions Made

All three are listed under key-decisions in the frontmatter. One more point: the best-effort append duplicates the normal path's append-and-report sequence. It has two uses, below the rule of three, so it stays inline with a `note:` comment.

## Deviations from Plan

None. The plan was executed as written.

## Issues Encountered

None.

## Known Stubs

None.

## Next Phase Readiness

All five Phase 6 plans have landed. Phase verification is next and is run by the orchestrator. Phase 6 is not marked complete here.

## Self-Check: PASSED

- All four modified files exist.
- Commits `13abe29`, `9691301`, `dae42dc` and `9190e40` are present in `git log`.
