---
phase: 06-core-value-cleanup
plan: 05
type: execute
wave: 2
depends_on: ["06-02"]
files_modified:
  - cmd/adoBizInt/write.go
  - cmd/adoBizInt/crash.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/crash_test.go
autonomous: true
requirements: [OUT-04, OUT-05, SAFE-02, SAFE-04]
coupling_justified: ["06-04: same Go package (cmd/adoBizInt) in the same wave but disjoint files; 06-05 adds only test-local types in crash_test.go and changes handlePanic's signature, which 06-04 never calls, so either order compiles and tests identically"]

estimate:
  tokens: 36000
  raw_tokens: 36000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "A panic inside a write's mutation, after --confirm and the pre-read, exits 5 (exitOutcomeUnknown), not 3 (D-12, OUT-04, SAFE-04, ROADMAP SC4 / audit W1)"
    - "That exit-5 path still emits exactly one JSON document on stdout, the published crash record with top-level keys exactly [error], kind \"panic\", and a message stating the write may have been issued and its outcome is unknown; no write envelope precedes it (D-18, OUT-05, Phase 3 D-26) [edge OUT-05/concurrency]"
    - "A panic before the request is issued (in the pre-read) still exits 3 with the crash record and appends no audit line, and a panic in a read still exits 3 (D-12, RESEARCH Pitfall 12)"
    - "The request-issued boundary is structural: in runWrite the deferred recover is the statement immediately before the op.mutate call and after defer sink.Close(), so it runs while the audit sink is still open, pinned by a go/ast test (D-12, RESEARCH Pitfall 9)"
    - "On a post-request panic the recover appends one best-effort audit record with outcome \"unknown\" and the command, target and changes of the write, carrying exactly the five D-15 keys and no panic message or stack (D-13, SAFE-02)"
    - "When that best-effort append fails, stderr carries \"Write may have landed but was NOT recorded\", the crash record still goes to stdout, and the exit is still 5 (D-13, D-18) [edge SAFE-02/empty]"
    - "A panic after the normal audit append (while emitting the write report) appends no second record: the audit file holds exactly one line, outcome executed, and the exit is 5 (D-13) [edge SAFE-02/adjacency]"
    - "The best-effort record is appended before the crash record is written, and a post-request panic under an already-cancelled parent context exits 5 with one unknown record, mirroring Phase 4 D-22 for interrupts (D-12, D-13, SAFE-04) [edge SAFE-02/ordering, SAFE-02/concurrency, SAFE-04/concurrency]"
    - "The doc comments in cmd/adoBizInt/main.go and cmd/adoBizInt/crash.go no longer call exit 3 the outcome-unknown code; they say 3 means nothing ran and that a post-request write panic exits 5 (D-14)"
  prohibitions:
    - "Must NOT move the crash record off stdout or change its three-key shape (D-18, Phase 3 D-23/D-26)"
    - "Must NOT add the panic message, a stack, argv or a URL to the audit record; the five-key D-15 shape is a one-way contract (T-04-01)"
    - "Must NOT append a second audit record for one write"
    - "Must NOT signal request-issued through a package-level variable or move the recover into guard (RESEARCH Anti-Patterns: the sink is already closed there)"
    - "Must NOT change the read-path panic exit code (3)"
  artifacts:
    - path: "cmd/adoBizInt/write.go"
      provides: "runWrite named result code; deferred recover immediately before op.mutate with best-effort unknown audit and an audited guard"
      contains: "recover()"
    - path: "cmd/adoBizInt/crash.go"
      provides: "handlePanic(r any, code int) int returning the caller's code; corrected doc comment"
      contains: "func handlePanic(r any, code int) int"
    - path: "cmd/adoBizInt/main.go"
      provides: "guard passes exitUnknown; corrected guard and run doc comments"
      contains: "handlePanic(r, exitUnknown)"
    - path: "cmd/adoBizInt/crash_test.go"
      provides: "TestPanicAfterWriteIssuedExitsOutcomeUnknown, TestPanicBeforeWriteIssuedExitsThree, TestWriteRecoverPrecedesMutate, TestPanicAfterWriteIssuedIsAudited"
      contains: "TestWriteRecoverPrecedesMutate"
  key_links:
    - from: "cmd/adoBizInt/write.go runWrite deferred recover"
      to: "cmd/adoBizInt/crash.go handlePanic"
      via: "code = handlePanic(<message naming the issued write>, exitOutcomeUnknown)"
      pattern: "handlePanic\\(.*exitOutcomeUnknown\\)"
    - from: "cmd/adoBizInt/write.go runWrite deferred recover"
      to: "cmd/adoBizInt/audit.go appendAuditRecord"
      via: "when the normal append was not attempted, newAuditRecord with outcomeUnknown is appended to the still-open sink"
      pattern: "Outcome: outcomeUnknown"
    - from: "cmd/adoBizInt/main.go guard"
      to: "cmd/adoBizInt/crash.go handlePanic"
      via: "code = handlePanic(r, exitUnknown)"
      pattern: "handlePanic\\(r, exitUnknown\\)"
---

<objective>
Close audit warning W1: `handlePanic` always returns exit 3 ("nothing ran"). A panic after a write's request has left therefore reports that nothing ran, and leaves no audit record, although the mutation may be live in Azure DevOps. Phase 4 D-22 already treats an interrupt in that window as exit 5. A panic must be treated the same way.

After this plan:
- runWrite recovers its own panics from the moment the mutation is issued, exits 5 (D-12) and makes a best-effort `unknown` audit append to the still-open sink (D-13).
- The crash record stays on stdout (D-18).
- A panic before the request leaves still exits 3.
- The stale comments that call 3 "outcome unknown" are corrected (D-14).

Purpose: OUT-04 and SAFE-04 (a write that may have landed reports outcome-unknown), OUT-05 (the panic still emits the structured record) and SAFE-02 (every write that may have landed leaves a trace).
Output: a deferred recover at the mutate boundary in `runWrite`, `handlePanic(r, code)`, the corrected comments, and the behavioral and structural tests.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/06-core-value-cleanup/06-CONTEXT.md
@.planning/phases/06-core-value-cleanup/06-RESEARCH.md
@.planning/phases/06-core-value-cleanup/06-02-SUMMARY.md
@cmd/adoBizInt/write.go
@cmd/adoBizInt/crash.go
</context>

<interfaces>
These already exist. Use them as they are and do not redefine them.

- `func runWrite(ctx context.Context, op writeOp, args []string) int` in cmd/adoBizInt/write.go:137. Its order: parse, resolve, `writeCtx, cancel := fetchctx.NewWithParent(...)` plus `defer cancel()`, `sink, err := auditOpen()` plus `defer sink.Close()` (~line 202), plan (the pre-read), the no-op and dry-run returns, then `err = op.mutate(writeCtx, target, wf)` (~line 230). After that come `outcome, code := writeExitCodeFor(err, ctx.Err())`, the audit append inside `if outcome == outcomeExecuted || outcome == outcomeUnknown` (~244-267), and `return finishWrite(...)`.
- `func handlePanic(r any) int` in crash.go:87-99. It logs the stack only when loggingReady, calls writeCrashRecord(crashKindPanic, fmt.Sprint(r)), and returns exitUnknown. writeCrashRecord (crash.go:55-68) writes one document to stdoutWriter.
- `func guard(body func() int) (code int)` in main.go:36-43. It is the only other handlePanic caller. Its doc comment is at main.go:26-35, and run's doc comment at 62-64. After 06-02, line 94's hint names ADO_PROJECTS; leave it.
- `newAuditRecord(writeRecord)` and `appendAuditRecord(sink, record)` in audit.go. The record has exactly the five keys timestamp, command, target, changes and outcome.
- `outcomeUnknown = "unknown"` and `outcomeExecuted = "executed"` in exit.go:36-40. `exitUnknown = 3` and `exitOutcomeUnknown = 5` in dispatch.go:21-29.
- Test helpers:
  - tempAuditSink(t), preReadFake(t, state) and setStateArgs(extra...) in write_test.go:31-65
  - assertAuditLineCount(t, path, n), decodeAuditRecord(t, line) and failingAuditSink(t) in audit_test.go
  - decodeCrash(t, raw) and keysOf in crash_test.go
  - captureOutput(t) and withConfiguredProjects(t, names...) in main_test.go
- Go facts:
  - Deferred calls run last-in first-out, so a recover deferred after `defer sink.Close()` runs while the sink is still open.
  - At function-body level, `outcome, code := ...` assigns to a named result `code` rather than shadowing it.
  - recover() returns nil for runtime.Goexit, so a fake's t.Fatalf is not swallowed.
- TestValidateGuardsCLIAndTUI (main_test.go, added by 06-02) is the first inline parser.ParseFile use in package main. The structural test here is the second.
</interfaces>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): a panic after the write's request is issued exits 5 with the crash record, and the boundary is pinned structurally (W1, D-12, D-14, D-18)</name>
  <reversibility rating="reversible">D-12 moves one panic path from exit 3 to exit 5 inside an existing published vocabulary (Phase 4 D-22 already defines 5); undoing it is deleting one deferred function.</reversibility>
  <files>cmd/adoBizInt/write.go, cmd/adoBizInt/crash.go, cmd/adoBizInt/main.go, cmd/adoBizInt/crash_test.go</files>
  <read_first>
    - cmd/adoBizInt/write.go lines 133-277: runWrite, every step and its comments
    - cmd/adoBizInt/crash.go: the whole file (99 lines)
    - cmd/adoBizInt/main.go lines 18-70: main, guard and its doc comment, initLogging, and run's doc comment
    - cmd/adoBizInt/crash_test.go: the whole file (330 lines). Note decodeCrash, panickingClient, TestPanicExitsThree (its doc comment at ~181-183 also calls 3 "the outcome-unknown code") and TestPanicEmitsDistinctErrorObject
    - cmd/adoBizInt/write_test.go lines 25-70: tempAuditSink, preReadFake, setStateArgs
    - cmd/adoBizInt/main_test.go: TestValidateGuardsCLIAndTUI from 06-02, the first inline parser.ParseFile use
    - .planning/phases/06-core-value-cleanup/06-RESEARCH.md: Pattern 4, Pitfalls 9-12, Anti-Patterns
  </read_first>
  <behavior>
    New TestPanicAfterWriteIssuedExitsOutcomeUnknown in crash_test.go:
    - Setup: withConfiguredProjects(t, "P"), tempAuditSink(t) and captureOutput(t). Start from preReadFake(t, "Active") and set UpdateWorkItemStateFunc to panic("deliberate panic mid-write").
    - Call code := guard(func() int { return dispatch(context.Background(), c, setStateArgs("--confirm")) }).
    - Expect:
      - code == exitOutcomeUnknown
      - decodeCrash(stdout) has top-level keys exactly [error]
      - decoding as crashEnvelope gives Kind "panic", a Message containing both "deliberate panic mid-write" and "outcome unknown", and a non-empty LogPath
      - stdout holds exactly one newline

    New TestPanicBeforeWriteIssuedExitsThree in crash_test.go:
    - Same setup, but GetWorkItemDetailFunc panics with "deliberate panic in pre-read", and UpdateWorkItemStateFunc stays nil.
    - Expect:
      - code == exitUnknown
      - the crash record's top-level keys are exactly [error], with Kind "panic"
      - assertAuditLineCount(t, sink, 0)

    New TestWriteRecoverPrecedesMutate in crash_test.go, a go/ast test:
    - Parse write.go with parser.ParseFile and find the FuncDecl runWrite.
    - Walk its Body.List and find three indices:
      - iClose: the DeferStmt whose call is the selector sink.Close
      - iRecover: the DeferStmt whose call is a FuncLit containing a call to the identifier recover
      - iMutate: the statement containing a call to the selector op.mutate
    - Each must be found exactly once (non-vacuity), with iClose < iRecover and iRecover == iMutate - 1.
    - The FuncLit must also contain a call to handlePanic whose second argument is the identifier exitOutcomeUnknown.

    TestPanicExitsThree, TestPanicEmitsDistinctErrorObject, TestPanicRecordCarriesNoStackOrCredential, TestPanicWithFailedLoggingWritesNoStackToStderr, TestGuardPassesThroughWhenNothingPanics and TestPATNeverLogged (patleak_test.go) pass unchanged. Read panics still exit 3.

    RED: TestPanicAfterWriteIssuedExitsOutcomeUnknown returns 3 today, and TestWriteRecoverPrecedesMutate finds no recover defer. TestPanicBeforeWriteIssuedExitsThree already passes and is the D-12 lower-boundary pin.
  </behavior>
  <action>
    RED first:
    1. Add the three tests as the behavior block states. Doc comments cite D-12 and D-18, and Phase 4 D-22 as the interrupt precedent.
    2. TestWriteRecoverPrecedesMutate parses with one inline parser.ParseFile on "write.go", adding the go/ast, go/parser and go/token imports. Leave a note: comment saying this is the second inline parse in package main, after TestValidateGuardsCLIAndTUI in main_test.go, so a helper is extracted only on a third use (rule of three).
    3. Record the RED output in the SUMMARY.

    GREEN in cmd/adoBizInt/crash.go, per D-14 and D-18:
    1. Change the signature to handlePanic(r any, code int) int. It returns code instead of exitUnknown. The stack-to-log-only logic and writeCrashRecord stay as they are, and the record stays on stdout (D-18).
    2. Rewrite the doc comment's first sentence: it turns a recovered value into the published crash record and returns the exit code its caller chose:
       - exitUnknown (3, nothing ran) from guard
       - exitOutcomeUnknown (5) from runWrite once a write's request was issued (D-12)

    GREEN in cmd/adoBizInt/main.go, per D-14:
    1. In guard, call handlePanic(r, exitUnknown).
    2. Rewrite guard's doc comment so it says a panic becomes a structured crash record and exit 3 (nothing ran, D-08). Add that a panic inside a write after its request was issued never reaches guard, because runWrite recovers it first and exits 5 (D-12).
    3. Rewrite run's doc comment so startup failures return exitUnknown because nothing ran, rather than describing the outcome as unknown.
    4. Leave 06-02's hint text and everything else in run unchanged.

    GREEN in cmd/adoBizInt/write.go runWrite, per D-12:
    1. Give runWrite a named result: (code int).
       - The existing `outcome, code := writeExitCodeFor(...)` line sits at function-body level, so it already assigns the named result. Keep it at that level and do not move it into a nested block.
    2. Register the deferred recover as the statement immediately before `err = op.mutate(writeCtx, target, wf)`. It must come after `defer sink.Close()`, the dry-run return and the no-op return.
       - It recovers.
       - It returns when the recovered value is nil.
       - Otherwise it sets code to handlePanic of the string "panic after <name> was issued; outcome unknown: " followed by the recovered value rendered with %v, and exitOutcomeUnknown.
    3. Comment the recover:
       - D-12: its POSITION is the request-issued signal. A panic in resolve or plan never reaches it and exits 3 through guard.
       - It is registered after defer sink.Close(), and defers run last-in first-out, so the sink is still open when it runs. Task 2 relies on that.
       - Treating "mutate entered" as "request issued" over-reports unknown, which is the safe direction (RESEARCH A4, Phase 4 D-22).

    Also fix TestPanicExitsThree's doc comment in crash_test.go so it says exit 3 (nothing ran) instead of "the outcome-unknown code". This is D-14 wording in a file this task already edits.

    Leave these untouched:
    - crashRecord and crashEnvelope
    - writeCrashRecord
    - the audit path. Task 2 adds the best-effort append.
    - dispatch.go and exit.go
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestPanicAfterWriteIssuedExitsOutcomeUnknown|TestPanicBeforeWriteIssuedExitsThree|TestWriteRecoverPrecedesMutate|TestPanicExitsThree|TestPanicEmitsDistinctErrorObject|TestPanicRecordCarriesNoStackOrCredential|TestPanicWithFailedLoggingWritesNoStackToStderr|TestGuardPassesThroughWhenNothingPanics|TestPATNeverLogged|TestInterruptedWriteReportsOutcomeUnknownNotNothingRan)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, or fewer than 10 top-level "--- PASS: Test" lines</fails_when>
  </verify>
  <acceptance_criteria>
    - TestPanicAfterWriteIssuedExitsOutcomeUnknown passes: exit 5, one stdout document with keys exactly [error], kind panic, and a message naming the panic and "outcome unknown".
    - TestPanicBeforeWriteIssuedExitsThree passes: exit 3 and zero audit lines.
    - TestWriteRecoverPrecedesMutate passes:
      - one recover defer, after defer sink.Close() and at the index immediately before the op.mutate statement
      - it calls handlePanic with exitOutcomeUnknown
    - TestPanicExitsThree, TestPanicEmitsDistinctErrorObject, TestPanicRecordCarriesNoStackOrCredential, TestPanicWithFailedLoggingWritesNoStackToStderr, TestGuardPassesThroughWhenNothingPanics, TestPATNeverLogged and TestInterruptedWriteReportsOutcomeUnknownNotNothingRan pass unchanged.
    - The SUMMARY records RED evidence.
  </acceptance_criteria>
  <done>A panic after a write's request leaves is reported as outcome unknown (exit 5) with the published crash record on stdout, while every panic before that point, and every read panic, still reports that nothing ran.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: a post-request panic leaves a best-effort unknown audit record, never two records for one write (D-13, SAFE-02)</name>
  <files>cmd/adoBizInt/write.go, cmd/adoBizInt/crash_test.go</files>
  <read_first>
    - cmd/adoBizInt/write.go: runWrite as left by Task 1, especially the recover defer and the normal audit append block (~lines 244-267)
    - cmd/adoBizInt/audit.go: auditRecord's five keys, newAuditRecord, appendAuditRecord
    - cmd/adoBizInt/audit_test.go: assertAuditLineCount, decodeAuditRecord, TestOutcomeUnknownWriteIsAudited, failingAuditSink, and TestAuditAppendFailureReportsUnknown's stderr wording
    - cmd/adoBizInt/crash_test.go: as left by Task 1
  </read_first>
  <behavior>
    New table-driven TestPanicAfterWriteIssuedIsAudited in crash_test.go. Every row:
    - uses withConfiguredProjects(t, "P"), tempAuditSink(t), captureOutput(t), preReadFake(t, "Active") and setStateArgs("--confirm")
    - drives guard(dispatch)

    Rows:
    1. "mutate panics":
       - UpdateWorkItemStateFunc panics.
       - Expect exit 5 and exactly 1 audit line.
       - decodeAuditRecord shows keys exactly timestamp, command, target, changes and outcome, with outcome "unknown", command "workitems set-state", target.id 4821, target.project "P", and one change on System.State from Active to Closed.
       - No audit value contains "deliberate panic".
       - Stdout is the crash record.
    2. "append fails, still unknown":
       - failingAuditSink(t) after tempAuditSink(t), and UpdateWorkItemStateFunc panics.
       - Expect exit 5.
       - Stderr contains "Write may have landed but was NOT recorded".
       - Stdout is exactly one crash document with keys [error] and kind "panic".
    3. "panic after the normal append writes no second record":
       - UpdateWorkItemStateFunc returns nil.
       - stdoutWriter is replaced, after captureOutput and restored through t.Cleanup, by a test-local writer that panics on its first Write and forwards every later Write to the captured buffer.
       - Expect exit 5 and exactly 1 audit line, with outcome "executed".
       - The buffer holds the crash record.
    4. "cancelled parent still unknown":
       - The parent context is cancelled before dispatch, and UpdateWorkItemStateFunc panics. preReadFake ignores ctx, so the pre-read still answers.
       - Expect exit 5 and exactly 1 audit line with outcome "unknown".

    RED against Task 1:
    - Rows 1 and 4 find 0 audit lines.
    - Row 2 finds no "NOT recorded" text on stderr.
    - Row 3 already passes, with one executed line, and pins the audited guard: once the best-effort append exists, deleting the guard makes row 3 find 2 lines.
  </behavior>
  <action>
    RED first:
    - Add TestPanicAfterWriteIssuedIsAudited as the behavior block states. Its doc comment cites D-13 and SAFE-02: every write that may have landed leaves a trace, and never two.
    - The panicking writer is a small type declared in crash_test.go and used only by this test.
    - Record the RED output in the SUMMARY.

    GREEN in cmd/adoBizInt/write.go runWrite, per D-13:
    1. Declare a local bool `audited`, initialised false, immediately before Task 1's recover defer. The defer must stay the statement directly before op.mutate, because TestWriteRecoverPrecedesMutate pins that.
    2. In the normal path, set audited = true immediately before the existing appendAuditRecord call inside the executed-or-unknown block, so a panic anywhere after that point cannot cause a second append.
    3. In the recover, before handlePanic, when audited is false:
       - build a record with newAuditRecord(writeRecord{Command: name, Target: target, Changes: changes, Outcome: outcomeUnknown})
       - call appendAuditRecord(sink, record)
       - on failure, log "Failed to append audit record" with the command and error, and write the same operator-actionable stderr line the normal path writes ("Write may have landed but was NOT recorded in <auditPath>: <err>")
       - in every case, continue to handlePanic with exitOutcomeUnknown
    4. The append comes before handlePanic, so the trace is persisted before the crash record is written.
    5. Add a note: comment on the block. The newAuditRecord, appendAuditRecord and stderr-line sequence now appears twice in runWrite (the normal path and this recover), below the rule of three, so it is not extracted.
    6. Comment the block:
       - D-13 best effort. The record carries no panic text, so the five-key D-15 shape and the T-04-01 no-argv rule hold.
       - A refused write that later panics is recorded as unknown, which over-reports in the safe direction.
    7. Keep the whole addition to roughly 20 lines (write.go is already over the 300-line target).

    Leave these untouched:
    - the normal-path audit logic and its CR-01 downgrade
    - finishWrite
    - audit.go
    - the crash record
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./cmd/adoBizInt -run '^(TestPanicAfterWriteIssuedIsAudited|TestPanicAfterWriteIssuedExitsOutcomeUnknown|TestPanicBeforeWriteIssuedExitsThree|TestWriteRecoverPrecedesMutate|TestExecutedWriteAppendsAuditRecord|TestOutcomeUnknownWriteIsAudited|TestAuditAppendFailureReportsUnknown|TestDryRunAndNoOpAppendNoAuditRecord|TestAuditRecordOmitsArgvAndCredentialFields|TestWriteGuardMatrix)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 10 top-level "--- PASS: Test" lines, or fewer than 4 "--- PASS: TestPanicAfterWriteIssuedIsAudited/" subtest lines</fails_when>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"</automated>
    <fails_when>non-zero exit from any stage, any "FAIL" package line, or gofmt listing a file</fails_when>
  </verify>
  <acceptance_criteria>
    - TestPanicAfterWriteIssuedIsAudited passes all 4 rows:
      - a mutate panic leaves exactly one five-key unknown record with the write's command, target and changes, and no panic text
      - a failing sink still exits 5, with the NOT-recorded stderr line and the crash record on stdout
      - a panic after the normal append leaves exactly one executed record
      - a cancelled parent plus a panic exits 5 with one unknown record
    - Task 1's three tests and the existing audit tests pass unchanged: TestExecutedWriteAppendsAuditRecord, TestOutcomeUnknownWriteIsAudited, TestAuditAppendFailureReportsUnknown, TestDryRunAndNoOpAppendNoAuditRecord, TestAuditRecordOmitsArgvAndCredentialFields and TestWriteGuardMatrix.
    - The SUMMARY records RED evidence for rows 1, 2 and 4.
    - The full module gate exits 0.
  </acceptance_criteria>
  <done>Every write whose request may have landed leaves exactly one audit record, even when the process panics. When the record cannot be written, the operator is told on stderr and the exit is still 5.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| runWrite to Azure DevOps mutation | After op.mutate is entered, ADO state may have changed whatever happens locally. |
| runWrite to audit.jsonl | The append-only trail an operator uses to learn what changed (SAFE-02). |
| crash record on stdout to the calling agent | A panic must be a distinct, parseable document. The exit code tells the agent whether anything may have run. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-06-17 | Repudiation | post-request panic leaves no audit trail (audit W1, SAFE-02) | high | mitigate | The deferred recover, registered after defer sink.Close() and so running while the sink is open, appends one newAuditRecord with outcome unknown when the normal append was not attempted. TestPanicAfterWriteIssuedIsAudited rows 1 and 4 prove exactly one record. Row 2 proves a failed append is reported on stderr and still exits 5. |
| T-06-18 | Spoofing (outcome integrity) | a write that may have landed reported as exit 3 "nothing ran" | high | mitigate | handlePanic takes the caller's code, and runWrite passes exitOutcomeUnknown (5) from the mutate boundary on. TestWriteRecoverPrecedesMutate pins the boundary with go/ast, and TestPanicBeforeWriteIssuedExitsThree pins that a pre-request panic stays 3. |
| T-06-19 | Information Disclosure | panic message, stack or argv reaching audit.jsonl | medium | mitigate | The best-effort record is built from writeRecord fields only (command, target, changes, outcome). Row 1 asserts exactly the five D-15 keys and no panic text. The stack still goes only to the log, when loggingReady. |
| T-06-20 | Information Disclosure | crash record on stdout | medium | mitigate | The record shape is unchanged: three keys, no stack (D-18, Phase 3 D-26). TestPanicRecordCarriesNoStackOrCredential and TestPATNeverLogged stay green. The added message prefix names only the command. |
| T-06-21 | Tampering (audit integrity) | duplicate record for one write | medium | mitigate | The audited local is set immediately before the normal append. Row 3 (a panic while emitting the report after an executed append) asserts exactly one executed record. |
| T-06-22 | Denial of Service | recover swallowing test or runtime termination | low | accept | recover returns nil for runtime.Goexit, so a fake's t.Fatalf still stops the test. The recover is scoped to runWrite after mutate starts, and a non-nil value always produces the crash record and exit 5. |
| T-06-SC | Tampering | dependency installs | low | accept | Not applicable: no package install, and go.mod is untouched. |
</threat_model>

<verification>
- Module gate (proven command): `cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`. It fails on a non-zero exit, a "FAIL" package line, or gofmt listing a file.
- 06-VALIDATION.md W1 rows are green: `go test ./cmd/adoBizInt -run 'TestPanic(After|Before)Write' -count=1`, `go test ./cmd/adoBizInt -run TestWriteRecoverPrecedesMutate -count=1`, and the regression row `go test ./cmd/adoBizInt -run 'TestPanicExitsThree|TestPATNeverLogged' -count=1`.
</verification>

<success_criteria>
- ROADMAP SC4 holds: a panic after a write's request was issued exits 5 (outcome unknown) with a best-effort audit record, not exit 3 (D-12, D-13).
- The crash record stays on stdout with its published shape (D-18). The stale "outcome-unknown" wording for exit 3 is gone (D-14).
- Read-path panics and pre-request write panics still exit 3.
</success_criteria>

## Artifacts this phase produces

| Artifact | Kind | Purpose |
|----------|------|---------|
| `cmd/adoBizInt/write.go` | modified | Named result; deferred recover at the mutate boundary; best-effort unknown audit with an `audited` guard |
| `cmd/adoBizInt/crash.go` | modified | `handlePanic(r any, code int) int`; corrected doc comment (D-14) |
| `cmd/adoBizInt/main.go` | modified | `guard` passes `exitUnknown`; corrected guard and run doc comments (D-14) |
| `cmd/adoBizInt/crash_test.go` | modified | `TestPanicAfterWriteIssuedExitsOutcomeUnknown`, `TestPanicBeforeWriteIssuedExitsThree`, `TestWriteRecoverPrecedesMutate`, `TestPanicAfterWriteIssuedIsAudited` |
| `.planning/phases/06-core-value-cleanup/06-05-SUMMARY.md` | new | Execution record with RED evidence |

<output>
Create `.planning/phases/06-core-value-cleanup/06-05-SUMMARY.md` when done. Record the boundary placement and why it is structural, the handlePanic signature change, the audited guard, the D-14 comment corrections, and the RED-then-GREEN evidence.
</output>
