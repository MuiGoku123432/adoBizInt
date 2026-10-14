---
phase: 07-guarded-tui-writes
plan: 03
subsystem: audit
tags: [go, audit, guarded-writes, sox, actor, tty, tdd]

requires:
  - phase: 07-guarded-tui-writes (07-02)
    provides: internal/guard Run, Options.Actor / Options.UserApproved, the Actor* constants, newAuditRecord on both append paths
provides:
  - "Every audit record carries actor (tui | cli-human | agent) and user_approved (JSON bool, always present), appended after outcome"
  - "guard.Run refuses any actor outside the three labels before opening the sink"
  - "cliActor(): cli-human when stdin is an interactive terminal, otherwise agent; stdinIsTerminal test seam"
  - "GUARD-06 approved exception recorded in REQUIREMENTS.md and ROADMAP Phase 7 SC5"
affects: [07-04, phase-13-agent-tiers, audit-consumers]

actuals:
  tokens: 5500
  tasks: 3
  commits: 3
plan_head_before: 27a530e85e2f357738ec09ae0997ac323110f3be
plan_head_after: 264666941851ff4951f18213a24d8f4e1bba570c

tech-stack:
  added: ["github.com/charmbracelet/x/term v0.2.2 promoted from indirect to direct require (already in go.sum, go.sum unchanged)"]
  patterns:
    - "Actor is a fixed surface label validated by the guard, never free text and never caller-selectable"
    - "Package-level func seam (stdinIsTerminal) mirroring auditOpen: swapped by tests only, never assigned from argv/env/config"

key-files:
  created:
    - cmd/adoBizInt/actor_test.go
  modified:
    - internal/guard/audit.go
    - internal/guard/guard.go
    - internal/guard/guard_test.go
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/audit_test.go
    - cmd/adoBizInt/crash_test.go
    - go.mod
    - .planning/phases/07-guarded-tui-writes/07-cli-audit-test.sha256
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md

key-decisions:
  - "CLI actor label is tty: cli-human when stdin is an interactive terminal, otherwise agent (user decision at the 07-03 checkpoint, 2026-10-13)"
  - "Audit keys ratified: actor (string) and user_approved (JSON bool, always present, no omitempty), appended after outcome; older five-key lines are never migrated"
  - "Run refuses an actor outside tui/cli-human/agent with the same shape as an unopenable sink (Report.SinkErr), before anything is opened, read or sent"
  - "go mod tidy was not used for the dependency move: it also promoted atotto/clipboard and google/uuid, so go.mod was reverted and only the x/term line was moved by hand"

patterns-established:
  - "One-way-door record changes go through a blocking-human checkpoint and an exception record that names each amended test by exact Go name"

requirements-completed: [GUARD-04, GUARD-06]

coverage:
  - id: D1
    description: "Every audit record carries actor and user_approved on the executed path and the panic path"
    requirement: GUARD-04
    verification:
      - kind: unit
        ref: "internal/guard/guard_test.go#TestAuditRecordCarriesActorAndUserApproved"
        status: pass
      - kind: integration
        ref: "cmd/adoBizInt/audit_test.go#TestExecutedWriteAppendsAuditRecord"
        status: pass
      - kind: integration
        ref: "cmd/adoBizInt/crash_test.go#TestPanicAfterWriteIssuedIsAudited"
        status: pass
    human_judgment: false
  - id: D2
    description: "guard.Run refuses an unknown, empty or identity-shaped actor before opening the sink"
    requirement: GUARD-04
    verification:
      - kind: unit
        ref: "internal/guard/guard_test.go#TestRunRefusesUnknownActor"
        status: pass
    human_judgment: false
  - id: D3
    description: "CLI labels writes cli-human on an interactive stdin and agent otherwise; no write verb has an actor flag"
    requirement: GUARD-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/actor_test.go#TestCLIActorLabel"
        status: pass
      - kind: integration
        ref: "cmd/adoBizInt/actor_test.go#TestExecutedCLIWriteRecordsActor"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/actor_test.go#TestActorIsNotAFlag"
        status: pass
    human_judgment: false
  - id: D4
    description: "GUARD-06 holds except for the approved exception: seven-file manifest verifies, regenerated audit_test.go lock verifies, inventory differs from baseline only by TestWriteRecoverPrecedesMutate, and the exception is recorded in REQUIREMENTS.md and ROADMAP SC5"
    requirement: GUARD-06
    verification:
      - kind: other
        ref: "shasum -a 256 -c 07-cli-tests.sha256 && shasum -a 256 -c 07-cli-audit-test.sha256 && comm inventory check && exception-record awk/grep check (Task 2 verify)"
        status: pass
    human_judgment: false

duration: 3min
completed: 2026-10-13
status: complete
---

# Phase 7 Plan 03: Audit actor and user_approved Summary

**Each audit record now has two more keys: `actor`, which is one of tui, cli-human or agent, and `user_approved`, a boolean that is always written. The guard refuses to run a write whose actor is not one of those three labels. The CLI sets its label from a terminal check on stdin.**

## Performance

- **Duration:** about 3 min for Tasks 2-3 (the Task 1 decision was taken earlier by the user)
- **Started:** 2026-10-14T03:54:02Z
- **Completed:** 2026-10-14T03:56:56Z
- **Tasks:** 3 (1 decision checkpoint, 2 executed)
- **Files modified:** 11

## Task 1 decision (recorded verbatim from the checkpoint resume, 2026-10-13)

- CLI actor option: **tty**. `cliActor()` returns `cli-human` when stdin is an interactive terminal, otherwise `agent`. This explicitly confirms promoting `github.com/charmbracelet/x/term v0.2.2` from indirect to a direct require in go.mod (already in go.sum; go.sum must not change).
- **keys ok**. This ratifies:
  - `actor` (string: tui | cli-human | agent) and `user_approved` (JSON boolean, always present, no omitempty), appended after `outcome`.
  - The three locked-test amendments: the key lists in TestExecutedWriteAppendsAuditRecord and TestPanicAfterWriteIssuedIsAudited gain the two keys, and TestAuditRecordOmitsArgvAndCredentialFields drops "actor" from its forbidden list. The other forbidden keys are unchanged and the doc sentence is reworded.
  - The 07-02 relocation of TestWriteRecoverPrecedesMutate to internal/guard as TestRunRecoverPrecedesMutate.
  - Recording those four as the "Approved Phase 7 exception" in REQUIREMENTS.md GUARD-06 and ROADMAP Phase 7 SC5, dated 2026-10-13.

Final key names are as proposed: `actor` and `user_approved`.

## Accomplishments

- `auditRecord` has two new fields after `Outcome`: `Actor` (`json:"actor"`) and `UserApproved` (`json:"user_approved"`). Neither uses omitempty. `newAuditRecord` takes `actor` and `userApproved`, and Run passes `opts.Actor` and `opts.UserApproved` on both the normal append and the panic-path append.
- `Run` checks `opts.Actor` first, before the opener is resolved. An unknown actor is logged as "Audit actor invalid" and Run returns `Report{SinkErr: ...}`, with a message naming the rejected value and the three allowed labels. Nothing is opened, read or sent.
- `cmd/adoBizInt/write.go` adds `var stdinIsTerminal = func() bool { return term.IsTerminal(os.Stdin.Fd()) }` and `func cliActor() string`. `runWrite` passes `Actor: cliActor(), UserApproved: false`, with a comment that `--user-approved` arrives in Phase 13 (AGENT-03).
- Five new tests and three approved amendments to locked tests. Locked CLI test files are otherwise byte-identical.

## Task Commits

1. **Task 1: Decision** — no commit (user decision only)
2. **Task 2 (tracer, TDD)**
   - RED `5c034a4` (test): the three approved amendments, the regenerated `07-cli-audit-test.sha256`, and the GUARD-06 exception in REQUIREMENTS.md and ROADMAP.md. These are in one commit, so no commit has amended locked tests without a recorded exception.
   - GREEN `618fd2c` (feat): the guard and audit changes, cliActor, and the go.mod move.
3. **Task 3: tests** — `2646669` (test)

## Exact diff of the three approved amendments

```diff
--- cmd/adoBizInt/audit_test.go (TestExecutedWriteAppendsAuditRecord)
-	wantKeys := []string{"changes", "command", "outcome", "target", "timestamp"}
+	wantKeys := []string{"actor", "changes", "command", "outcome", "target", "timestamp", "user_approved"}
--- cmd/adoBizInt/audit_test.go (TestAuditRecordOmitsArgvAndCredentialFields doc + forbidden list)
-// crash records. Actor identity is omitted separately (D-15): there is one
-// identity, the PAT's, and its resolution is non-fatal.
+// crash records. Actor is a fixed surface label (tui, cli-human or agent,
+// GUARD-04), not an identity; user identity is still never recorded.
-	forbidden := []string{"argv", "actor", "url", "headers", "token", "pat", "authorization"}
+	forbidden := []string{"argv", "url", "headers", "token", "pat", "authorization"}
--- cmd/adoBizInt/crash_test.go (TestPanicAfterWriteIssuedIsAudited)
-			if want := []string{"changes", "command", "outcome", "target", "timestamp"}; !reflect.DeepEqual(keys, want) {
+			if want := []string{"actor", "changes", "command", "outcome", "target", "timestamp", "user_approved"}; !reflect.DeepEqual(keys, want) {
```

The regenerated lock is `0ea2263b06e680663b03a5fb513fc8a899446f052b5f89c4d87e58001a63da05  cmd/adoBizInt/audit_test.go`.

## go.mod diff

```diff
 	github.com/charmbracelet/lipgloss v1.1.0
+	github.com/charmbracelet/x/term v0.2.2
 	github.com/evertras/bubble-table v0.19.2
...
-	github.com/charmbracelet/x/term v0.2.2 // indirect
```

go.sum: no diff. `go mod verify` passes.

## Exception text added

REQUIREMENTS.md (sub-bullet under the unchanged `- [ ] **GUARD-06**` line):

> - Approved Phase 7 exception (07-03 checkpoint, 2026-10-13): three locked key-set assertions were amended for GUARD-04 and one test was relocated. TestExecutedWriteAppendsAuditRecord: expected audit keys gain `actor` and `user_approved`. TestPanicAfterWriteIssuedIsAudited: expected audit keys gain the same two keys. TestAuditRecordOmitsArgvAndCredentialFields: "actor" leaves the forbidden-key list, because actor is now a fixed surface label rather than an identity; argv, url, headers, token, pat and authorization stay forbidden. TestWriteRecoverPrecedesMutate moved from cmd/adoBizInt/crash_test.go to internal/guard/structural_test.go as TestRunRecoverPrecedesMutate (plan 07-02, GUARD-01), because the recover moved into the shared guard. Every other CLI test file is byte-identical, enforced by `.planning/phases/07-guarded-tui-writes/07-cli-tests.sha256`.

ROADMAP.md (appended to Phase 7 success criterion 5, same line):

> Approved Phase 7 exception (07-03 checkpoint): TestExecutedWriteAppendsAuditRecord and TestPanicAfterWriteIssuedIsAudited expect the two GUARD-04 keys `actor` and `user_approved`, TestAuditRecordOmitsArgvAndCredentialFields no longer forbids "actor", and TestWriteRecoverPrecedesMutate is relocated to internal/guard as TestRunRecoverPrecedesMutate; see REQUIREMENTS.md GUARD-06 for the detail

## TDD notes

- **RED (Task 2):** `go test -count=1 -v ./cmd/adoBizInt/ -run '^(TestExecutedWriteAppendsAuditRecord|TestPanicAfterWriteIssuedIsAudited|TestAuditRecordOmitsArgvAndCredentialFields)$'` exited 1.
  - TestExecutedWriteAppendsAuditRecord failed with `audit_test.go:84: audit record keys = [changes command outcome target timestamp], want [actor changes command outcome target timestamp user_approved]`.
  - TestPanicAfterWriteIssuedIsAudited failed with the same key mismatch at `crash_test.go:499` on its three record-writing rows. The "append fails" row writes no line, so it does not check keys.
  - TestAuditRecordOmitsArgvAndCredentialFields passed, as expected. Its amendment only relaxes the test, so it cannot go red.
  - Semantic assessment: both target tests compiled, ran, and failed on their own key-set assertion. There was no build, load or zero-test fault, so this is a valid RED.
  - The `gsd_run check tdd-red-evidence` classifier has no adapter for `go test` output, so this evidence is recorded here, following the 07-02 precedent.
- **GREEN:** `618fd2c`. The Task 2 verifies all pass: build, vet, and race tests for cmd/adoBizInt and internal/guard; both manifests; the inventory check; and the exception-record check.
- **Tracer gate:** Task 2 has no `blocking-human` gate, and human_verify_mode is the default end-of-phase with only automated verifies. All three verifies were re-run on the committed tree and passed, so execution continued.
- **Task 3 had no RED by design.** It pins behaviour that Task 2 already built, so its tests passed on the first run. To show they are not vacuous, four throwaway mutations were applied and each was restored with `git checkout -- <file>`:
  1. `cliActor` forced to cli-human: TestCLIActorLabel and TestExecutedCLIWriteRecordsActor failed.
  2. Actor validation in Run disabled: TestRunRefusesUnknownActor failed.
  3. `omitempty` added to `user_approved`: TestAuditRecordCarriesActorAndUserApproved and TestExecutedCLIWriteRecordsActor failed.
  4. An `actor` flag registered in newFlagSet: TestActorIsNotAFlag failed.
- TestCLIActorLabel uses the tty form. It swaps `stdinIsTerminal` to true and then to false, restoring it via t.Cleanup, and asserts `cli-human` and then `agent`.

## Decisions Made

- The user chose the tty option and "keys ok" (recorded verbatim above).
- The actor refusal reuses `Report.SinkErr` instead of adding a new Report field. It has the same meaning as an unopenable sink: no compliant record can be written, so nothing starts. The existing CLI SinkErr branch already maps it to exit "unknown" with nothing run. The CLI cannot reach it, because cliActor only returns valid labels.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `go mod tidy` changed more than the one permitted line**
- **Found during:** Task 2 (GREEN)
- **Issue:** tidy also promoted `github.com/atotto/clipboard` and `github.com/google/uuid` to direct requires. Both were already out of step with tidy before this plan.
- **Fix:** As the plan prescribes, go.mod and go.sum were reverted, and only the `github.com/charmbracelet/x/term v0.2.2` line was moved into the direct block by hand.
- **Files modified:** go.mod
- **Verification:** `git diff go.sum` is empty; `go mod verify` passes; build and tests are green.
- **Committed in:** 618fd2c

**2. [Naming] The plan's interfaces call the write-verb map `writeVerbFlags`; the real map is `writeSupportByCommand`**
- **Found during:** Task 3
- **Fix:** TestActorIsNotAFlag iterates `writeSupportByCommand` and asserts it has exactly six entries before checking each verb's FlagSet.
- **Committed in:** 2646669

---

**Total deviations:** 2. Both are minor: one was prescribed by the plan's own fallback, and one is a naming correction.
**Impact on plan:** None on scope.

## Issues Encountered

- internal/guard/guard_test.go is now about 420 lines, over the 300-line target. The plan put both guard tests there to reuse its Run helpers, so it was not split. Splitting it is a candidate for a later cleanup.

## Known Stubs

None. `UserApproved: false` in the CLI is intentional: Phase 13 (AGENT-03) adds `--user-approved`, and the comment at the call site says so.

## Next Phase Readiness

- 07-04 (TUI writes) can set `Options.Actor = guard.ActorTUI` and will get compliant seven-key records. Any other value is refused.
- Phase 13 can reuse `stdinIsTerminal` and `cliActor` for its tier gate.

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-13*

## Self-Check: PASSED

- Files found: cmd/adoBizInt/actor_test.go, internal/guard/audit.go, internal/guard/guard.go, cmd/adoBizInt/write.go, this SUMMARY
- Commits found in HEAD ancestry: 5c034a4, 618fd2c, 2646669
