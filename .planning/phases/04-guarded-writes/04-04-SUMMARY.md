---
phase: 04-guarded-writes
plan: 04
subsystem: testing
tags: [go, testing, table-driven, guarded-writes, audit, validation-map]

requires:
  - phase: 04-01
    provides: runWrite's ten-step guard order, the write envelope, writeExitCodeFor, the audit sink, fakeWriteClient
  - phase: 04-02
    provides: nullable effort signature and the fs.Visit set-flag discipline
  - phase: 04-03
    provides: writeOp, the five additional write verbs, writeSupportByCommand as the dispatcher's own registry
provides:
  - "TestWriteGuardMatrix: one table over writeSupportByCommand asserting all five guards for every write verb, with a t.Fatalf drift guard on any registered verb lacking a fixture"
  - "a completed 04-VALIDATION.md Per-Task Verification Map with no TBD entries"
affects: [05-hardening, any future write verb]

actuals:
  tokens: 21000
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Coverage guard by iterating the production registry, not a literal verb list — a new verb cannot be registered without a fixture"
    - "nil fake func field as the no-mutation assertion, in preference to a call counter"

key-files:
  created: []
  modified:
    - cmd/adoBizInt/write_test.go
    - cmd/adoBizInt/writeverbs_test.go
    - .planning/phases/04-guarded-writes/04-VALIDATION.md

key-decisions:
  - "The guard matrix ranges over writeSupportByCommand rather than a literal slice, so a seventh verb cannot be added without failing the suite"
  - "The validation map's Requirement column was corrected to REQUIREMENTS.md:50-53 (audit rows are SAFE-02, effort rows SAFE-03); the plans' mapping was authoritative"
  - "Task 3's live-ADO confirmation is DEFERRED, not performed — recorded unverified rather than assumed"

patterns-established:
  - "Registry-iterating coverage guard: the same map the dispatcher consults is the test's source of truth"

requirements-completed: []

coverage:
  - id: D1
    description: "All five write guards asserted for every one of the six write verbs from one enumerable table"
    requirement: "SAFE-01"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestWriteGuardMatrix"
        status: pass
    human_judgment: false
  - id: D2
    description: "04-VALIDATION.md Per-Task Verification Map completed — no TBD, requirements reconciled against REQUIREMENTS.md"
    verification:
      - kind: other
        ref: "grep -c TBD .planning/phases/04-guarded-writes/04-VALIDATION.md => 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "Live Azure DevOps write confirmation against a scratch work item (preview / --confirm / repeat / audit inspection / required-flag rejection), plus the carried 04-02 TUI effort-modal close"
    requirement: "SAFE-02"
    verification: []
    human_judgment: true
    rationale: "Requires live credentials and a disposable work item; DEFERRED by the user and NOT performed. No result may be inferred. See '## Deferred — Live Azure DevOps verification (UNVERIFIED)'."

duration: 38 min
completed: 2026-09-12
status: complete
---

# Phase 04 Plan 04: Phase-wide guard matrix and validation map Summary

**One table over `writeSupportByCommand` asserts all five write guards for all six verbs and fails on any registered verb without a fixture; the Phase 4 validation map now names only tests that exist — the live-ADO confirmation is deferred and remains unverified.**

## Performance

- **Duration:** 38 min (across the original executor and this continuation)
- **Tasks:** 2 of 3 completed; 1 deferred
- **Files modified:** 3

## Accomplishments

- `TestWriteGuardMatrix` ranges over `writeSupportByCommand` — the dispatcher's own registry — and asserts, per verb: no mutation without `--confirm`, field-level diff, refused no-op (exit 4, no audit line), audited execution (exactly one line), outcome-unknown classification (exit 5, audited), and unwritable-sink-prevents-mutation. A registered key with no fixture fails with `t.Fatalf`, so drift is impossible to ship silently.
- The Per-Task Verification Map has no `TBD` entries; Task ID / Plan / Wave are filled, rows were reassigned to the plans that actually own the tests, and the `Requirement` column was corrected against `.planning/REQUIREMENTS.md:50-53` (audit rows → SAFE-02, effort rows → SAFE-03). SC6's Automated Command is now the scoped comment-stripped grep that can actually observe its claim.

## Task Commits

1. **Task 1: One guard matrix over every write verb** — `9117431` (test)
2. **Task 2: Complete the validation map** — `389d2f5` (docs)
3. **Task 3: Live Azure DevOps confirmation** — DEFERRED, no commit, no result

## Files Created/Modified

- `cmd/adoBizInt/write_test.go` — `TestWriteGuardMatrix` plus its per-verb fixture table
- `cmd/adoBizInt/writeverbs_test.go` — narrower per-verb assertions subsumed by the matrix consolidated away
- `.planning/phases/04-guarded-writes/04-VALIDATION.md` — map completed; manual-only row left pending

## Deferred — Live Azure DevOps verification (UNVERIFIED)

**Status: NOT RUN. NOT APPROVED. No step below has an observed result.**

The user elected to defer this checkpoint rather than run it now. Nothing in this SUMMARY,
in `04-VALIDATION.md`, or in the phase state asserts that any of these steps passed. The
phase's single manual-only verification is therefore still outstanding, and Phase 4 should
not be treated as fully verified against a live Azure DevOps instance until it is run.

Procedure preserved verbatim so a later session can run it without re-deriving it.
Pick a scratch work item, note its current State, then:

1. `adoBizInt workitems set-state <id> --project <P> --state <different-state>` — expect a diff showing `System.State: <current> -> <new>`, exit 0, and the work item **unchanged** in the ADO web UI.
2. `wc -l ~/.config/adoBizInt/audit.jsonl` — note the count (the file may not exist yet; that is fine).
3. Re-run step 1 with `--confirm` appended — expect exit 0, `executed`, the State changed in the web UI, and **exactly one** new audit line.
4. Re-run step 3 unchanged — expect exit 4, an already-in-the-requested-state outcome, **no** new audit line, no further change.
5. `tail -1 ~/.config/adoBizInt/audit.jsonl` — expect keys `timestamp`, `command`, `target`, `changes`, `outcome` and nothing else; no `argv`, no token, no URL.
6. `adoBizInt pipelines queue <definition-id> --project <P>` with no `--branch` — expect exit 3, an error on stderr, nothing queued.
7. Carried from 04-02 (its only open coverage item): close one real Task through the Bubble Tea TUI effort modal, entering effort values. No automated test drives that modal end to end. Confirm the Task closes with the effort typed, and that fields left blank were not zeroed.

## Decisions Made

- The matrix iterates the production registry rather than a literal verb list. A hand-maintained list is exactly the artifact that drifts; T-04-17 is only mitigated if the test reads the same map the dispatcher does.
- `nil` fake func fields over call counters for the no-mutation assertions: a counter asserts equality with zero after the fact, a nil field makes the call impossible to make silently and names the offending method in the failure.
- Where the validation map disagreed with the plans about requirement ownership, the plans (and `REQUIREMENTS.md:50-53`) won.

## Deviations from Plan

None in Tasks 1 and 2 — both executed as written.

Task 3 was not executed. This is a user-directed deferral of a `gate="blocking"`
human-verify checkpoint, recorded as unverified rather than assumed.

## Issues Encountered

- **`cmd/adoBizInt/write.go` is ~470 lines, past this project's 300-line convention.** 04-03 deliberately deferred the split as a considered trade: `runWrite`'s ten-step guard order is the phase's central invariant, and splitting it mid-phase risked reordering guards for a line-count win. The debt is real and carried forward, not forgotten.
- The phase's single manual-only verification is outstanding (above).

## User Setup Required

None.

## Next Phase Readiness

- `go build ./...`, `go test ./...`, `go vet ./...` and `gofmt -l .` are all green.
- Phase 4's automated contract is complete and self-guarding against new verbs.
- **Blocker for declaring Phase 4 verified:** the live-ADO confirmation and the carried 04-02 TUI effort-modal check have not been performed. `04-VALIDATION.md` keeps `status: draft` and `nyquist_compliant: false`, and its Manual-Only Verifications row stays `⬜ pending`.

---
*Phase: 04-guarded-writes*
*Completed: 2026-09-12*
