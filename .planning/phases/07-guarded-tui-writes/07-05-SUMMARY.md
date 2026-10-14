---
phase: 07-guarded-tui-writes
plan: 05
subsystem: ui
tags: [go, bubbletea, guarded-writes, audit, tui, tdd, releases, pipelines]

requires:
  - phase: 07-02
    provides: "guard.ApproveRelease, guard.RejectRelease, guard.QueuePipeline ops; guardtest.Client fake"
  - phase: 07-03
    provides: "audit records carry actor and user_approved"
  - phase: 07-04
    provides: "internal/ui/confirm Flow, Batch, Result and the prs.go reference wiring"
provides:
  - "internal/ui/releases: approve and reject through confirm.Flow after the approval modal (D-06); WriteBusy(); approveRelease, rejectRelease and ApprovalResultMsg removed"
  - "internal/ui/pipelines: queue through confirm.Flow after the trigger modal (D-06); WriteBusy(); queuePipeline and TriggerResultMsg removed"
  - "internal/guard/guardtest: AuditRecords(t), the shared audit.jsonl reader for TUI view tests"
affects: [07-06, 07-07, 07-08]

actuals:
  tokens: 7100
  tasks: 2
  commits: 5
plan_head_before: 6e4d3a145c706626dc2c306f12a71704801f762d
plan_head_after: 98d54a62b31678de3fe6f65b8539ded7c38eff4a

tech-stack:
  added: []
  patterns:
    - "Input modal, then confirm: the input modal's submit message only closes it and calls flow.Start; the diff modal's y is the only path to guard.Run"
    - "View tests read the audit file through guardtest.AuditRecords(t) after t.Setenv(\"HOME\", t.TempDir())"

key-files:
  created:
    - internal/ui/releases/write_test.go
    - internal/ui/pipelines/write_test.go
    - internal/guard/guardtest/audit.go
  modified:
    - internal/ui/releases/releases.go
    - internal/ui/pipelines/pipelines.go
    - internal/ui/prs/write_test.go

key-decisions:
  - "Pipeline queue success reads '#<definitionID> queued on <branch>' and no longer shows the build number: the guard reports the requested change, not a second read of the run ADO created (the CLI's rule too, 07-02 ops.go); the refreshed runs table shows the new run"
  - "The audit-file reader moved to guardtest.AuditRecords once releases and pipelines became its second and third users (rule of three); prs/write_test.go now uses it"
  - "The 10-line feed test helper stays per package with a note comment: it is typed on each package's Model, and sharing it would need a new test-support package that imports bubbletea"

patterns-established:
  - "Single-shot views with an input modal call flow.Start from the submit handler, keep the input modal closing first, and never show the two modals together (07-UI-SPEC S5)"

requirements-completed: [GUARD-02, GUARD-04]

coverage:
  - id: D1
    description: "Release approve: approval modal submit, then the pre-read ('Checking approval #42...'), then the diff modal (approval.status pending → approved); only y sends ApproveRelease with the approval's project and comment; one executed record with actor tui and user_approved false that does not contain the comment"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/releases/write_test.go#TestApproveSubmitPreviewsThenConfirms"
        status: pass
    human_judgment: false
  - id: D2
    description: "Release reject uses RejectRelease after the same confirm (pending → rejected); record command 'releases reject'"
    requirement: GUARD-02
    verification:
      - kind: unit
        ref: "internal/ui/releases/write_test.go#TestRejectUsesRejectAfterConfirm"
        status: pass
    human_judgment: false
  - id: D3
    description: "An approval already in the requested state skips the modal and sends nothing: 'No change: #42 already approved' (D-07)"
    verification:
      - kind: unit
        ref: "internal/ui/releases/write_test.go#TestAlreadyApprovedReleaseIsNoChange"
        status: pass
    human_judgment: false
  - id: D4
    description: "Pipeline queue: trigger modal submit, then 'Checking build on main...', then the diff modal (queued_build (none) → main); only y queues; status '#9 queued on main'; one executed record with actor tui and user_approved false"
    requirement: GUARD-04
    verification:
      - kind: unit
        ref: "internal/ui/pipelines/write_test.go#TestQueueSubmitPreviewsThenConfirms"
        status: pass
    human_judgment: false
  - id: D5
    description: "esc on the queue confirm sends nothing, writes no audit record, shows 'Not written: #9', and leaves WriteBusy false (D-04)"
    verification:
      - kind: unit
        ref: "internal/ui/pipelines/write_test.go#TestQueueEscSendsNothing"
        status: pass
    human_judgment: false
  - id: D6
    description: "Real-terminal look: on Releases, submitting a comment opens 'Confirm #<approval id>' with 'approval.status  pending → approved' before anything is sent; on Pipelines, t then submit shows 'queued_build  (none) → <branch>', and esc queues nothing"
    verification: []
    human_judgment: true
    rationale: "The plan's human-check: the two-step modal sequence against live ADO in a real terminal cannot be asserted by unit tests"

duration: 4min
completed: 2026-10-14
status: complete
---

# Phase 7 Plan 05: Release approve/reject and pipeline queue through the guarded confirm flow Summary

**Release approve, release reject and pipeline queue now go input modal, then guard pre-read, then the `Confirm #<id>` diff modal, then `y`, then `guard.Run` with actor `tui`. Each view reuses 07-04's `confirm.Flow` exactly as prs.go wires it. The direct-send helpers and their result messages are gone.**

## Performance

- **Duration:** about 4 min of execution, after reading the context
- **Started:** 2026-10-14T04:09:45Z
- **Completed:** 2026-10-14T04:14:06Z
- **Tasks:** 2 (Task 1 tracer, Task 2 auto; both TDD)
- **Files:** 6 (3 created, 3 modified)

## Accomplishments

- **Releases.** `ApprovalSubmitMsg` closes the approval modal and starts the flow with `confirm.Batch{ID: ApprovalID, Ops: [guard.ApproveRelease | guard.RejectRelease], Labels: ["approved" | "rejected"]}`. The status reads `Checking approval #<id>...`. The comment goes to the guard op, which sends it only to ADO. The test checks that the comment never appears in the audit file.
- **Pipelines.** `TriggerSubmitMsg` closes the trigger modal and starts the flow with `confirm.Batch{ID: DefinitionID, Ops: [guard.QueuePipeline], Labels: ["queued on <branch>"]}`. The status reads `Checking <PipelineName> on <Branch>...`.
- **Both views:**
  - The flow runs first in Update.
  - A refresh re-fetches the view's own data: releases plus approvals, or pipeline runs.
  - While the flow is busy, the tick only re-arms and `IsTextInputFocused` returns true, so tabs are held.
  - `WriteBusy()` is exported for 07-08.
  - The flow modal is drawn over the table, and the sticky unknown-outcome line sits above the status line.

## Removed status text (and why)

- Releases: `Processing <action>...` and `Successfully <action>d approval #<id>` are gone. They are replaced by `Checking approval #<id>...` and `#<id> approved` / `#<id> rejected`.
- Pipelines: `Queuing <name> on <branch>...` and `Queued <name>: Build #<buildNumber>` are gone. They are replaced by `Checking <name> on <branch>...` and `#<definitionID> queued on <branch>`.
- **The build number is no longer shown.** The guard reports the change that was requested (`queued_build (none) → <branch>`), not a second read of the run ADO created. This matches the CLI's rule (07-02 ops.go). The refreshed runs table shows the new run and its number.

## Line counts

| File | Lines |
|------|-------|
| internal/ui/releases/releases.go | 704 (was 691) |
| internal/ui/pipelines/pipelines.go | 681 (was 665) |
| internal/ui/releases/write_test.go | 164 |
| internal/ui/pipelines/write_test.go | 112 |
| internal/guard/guardtest/audit.go | 36 |
| internal/ui/prs/write_test.go | 311 (was 336) |

Both view files were already over the 300-line target before this plan. Here they grow by 13 and 16 lines: the deleted direct-send helpers and result handlers offset most of the flow wiring.

## Task Commits

1. **Shared test helper.** `1e986d9` (refactor): `guardtest.AuditRecords`, which prs/write_test.go now uses.
2. **Task 1 (tracer, TDD):**
   - RED `d7e49a4` (test): the three release tests, plus the releases `writer` field and a `WriteBusy` stub so they compile.
   - GREEN `e34e67d` (feat): approve and reject go through confirm.Flow.
   - Tracer gate (interactive, end-of-phase, `<automated>`-only verify): both verifies were re-run on the committed tree and passed. The run then continued to Task 2.
3. **Task 2 (TDD):**
   - RED `662b629` (test): the two pipeline tests, plus the pipelines `writer` field and a `WriteBusy` stub.
   - GREEN `98d54a6` (feat): queue goes through confirm.Flow.

No REFACTOR commit was needed.

## TDD notes

- **RED, Task 1:** `go test -race -count=1 -v ./internal/ui/releases/ -run '^(TestApproveSubmitPreviewsThenConfirms|TestRejectUsesRejectAfterConfirm|TestAlreadyApprovedReleaseIsNoChange)$'` exited 1. All three targets compiled, ran, and failed on their first planned assertion: `status after submit = "Processing approve..." / "Processing reject...", want it to contain "Checking approval #42"`. The old code sent straight from the input submit, which is exactly what D-06 forbids. That makes this a valid RED.
- **RED, Task 2:** the same command for the two pipeline tests exited 1. Both failed with `status after submit = "Queuing build on main...", want it to contain "Checking build on main"`, which is valid RED for the same reason.
- The `tdd-red-evidence` classifier has no `go test` adapter (it reads only TAP, JUnit, swift-testing and unittest). As in 07-02 and 07-04, the evidence and its semantic assessment are recorded here instead.
- **GREEN:** all five named tests pass, with 3 and 2 top-level `--- PASS: Test` lines.

## Gate output

- `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`: every package is ok, and gofmt is clean.
- `shasum -a 256 -c .planning/phases/07-guarded-tui-writes/07-cli-tests.sha256`: all seven locked CLI test files report OK.
- releases.go calls no client ApproveRelease or RejectRelease; it uses `guard.ApproveRelease(` and `guard.RejectRelease(`. pipelines.go calls no client QueuePipeline; it uses `guard.QueuePipeline(`.
- `guard.Run(` under `internal/ui` still appears only in `internal/ui/confirm/batch.go`, at two sites.

## Files Created/Modified

- `internal/ui/releases/releases.go`:
  - adds the `writer` and `flow` fields; the flow runs first in Update
  - the approval submit starts the flow
  - the tick rule also covers a busy flow; sticky line, overlay, `IsTextInputFocused` and `WriteBusy` follow the flow
  - approveRelease, rejectRelease and ApprovalResultMsg are removed
- `internal/ui/pipelines/pipelines.go`: the same wiring for the trigger submit; queuePipeline and TriggerResultMsg are removed
- `internal/ui/releases/write_test.go`: three release write tests
- `internal/ui/pipelines/write_test.go`: two pipeline write tests
- `internal/guard/guardtest/audit.go`: `AuditRecords(t)`
- `internal/ui/prs/write_test.go`: its local `auditLines` is replaced by `guardtest.AuditRecords`

## Decisions Made

See key-decisions in the frontmatter:
- The build number is no longer shown on queue.
- The audit reader is shared through guardtest.
- `feed` stays per package.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - CLAUDE.md rule of three] The audit-file reader was extracted into guardtest**
- **Found during:** Task 1, while writing the release tests
- **Issue:** Releases and pipelines both needed the same audit.jsonl reader that prs/write_test.go already had, which would have made three copies. The user's CLAUDE.md requires extraction at the third occurrence.
- **Fix:** Added `guardtest.AuditRecords(t)` in a new file `internal/guard/guardtest/audit.go`. It builds the path from HOME because guardtest must not import guard. prs/write_test.go now calls it instead of its own `auditLines`. Neither file is in the plan's `files_modified`; both are test-only.
- **Files modified:** internal/guard/guardtest/audit.go, internal/ui/prs/write_test.go
- **Verification:** prs tests pass before and after; the full module gate is green.
- **Committed in:** 1e986d9

---

**Total deviations:** 1 auto-fixed (rule-of-three extraction). **Impact:** test-only. No production behaviour changed beyond the plan, and there was no scope creep.

## Issues Encountered

None.

## Requirements

`requirements-completed` lists GUARD-02 and GUARD-04, copied from the plan frontmatter. As the orchestrator instructed, `requirements.mark-complete` was not run: the REQUIREMENTS.md checkboxes and traceability rows stay Pending until phase verification. GUARD-02 still needs the work-item call sites (07-06, 07-07) and the structural test.

## Known Stubs

None. The `WriteBusy` stubs that returned false existed only in the RED commits. Both now return `m.flow.Busy()`.

## User Setup Required

None. No external service configuration is required.

## Next Phase Readiness

- 07-06 and 07-07 (work items) can follow the same wiring. View tests can read audit records with `guardtest.AuditRecords(t)`.
- 07-08 can use `releases.Model.WriteBusy()` and `pipelines.Model.WriteBusy()` for its quit hold, alongside `prs.Model.WriteBusy()`.
- Human check, deferred to end-of-phase UAT (D6): the Releases and Pipelines two-step modal sequence against live ADO at 80x24.

## Self-Check: PASSED

- FOUND: internal/ui/releases/releases.go, internal/ui/releases/write_test.go, internal/ui/pipelines/pipelines.go, internal/ui/pipelines/write_test.go, internal/guard/guardtest/audit.go, internal/ui/prs/write_test.go
- FOUND commits: 1e986d9, d7e49a4, e34e67d, 662b629, 98d54a6 (`git rev-list --count 6e4d3a1..HEAD` = 5)

---
*Phase: 07-guarded-tui-writes*
*Completed: 2026-10-14*
