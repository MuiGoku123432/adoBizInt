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
    description: "Live Azure DevOps write confirmation against a scratch work item (preview / --confirm / repeat / audit inspection / required-flag rejection)"
    requirement: "SAFE-02"
    verification:
      - ref: "Live run 2026-09-12, Windstream/Kinetic work item 1198503, steps 1-6"
        status: pass
    human_judgment: true
    rationale: "Run against live ADO on 2026-09-12. Steps 1-6 all passed. See '## Live Azure DevOps verification — RUN 2026-09-12'."
  - id: D4
    description: "Carried from 04-02: TUI effort modal preserves effort fields left blank"
    requirement: "SAFE-03"
    verification:
      - ref: "Live run 2026-09-12: Task closed via Bubble Tea effort modal with Original Estimate blank"
        status: fail
    human_judgment: true
    rationale: "FAILED. Blank effort field is silently overwritten with 0 (effort.go:193-199 + workitems.go:362-374). SAFE-03 holds on the CLI path only. The TUI write path bypasses runWrite entirely — no dry run, no --confirm, no audit record. Remediation tracked in .planning/BACKLOG.md; not a Phase 4 correction because D-17 locked TUI behaviour."

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

## Live Azure DevOps verification — RUN 2026-09-12

**Superseded the deferral below. Steps 1-6 PASSED. Step 7 FAILED.**

Target: `dev.azure.com/Windstream`, project `Kinetic`, work item `1198503`
(restored to its original `New` state after the run; two `executed` audit records reflect
the write and the restore).

| # | Check | Result |
|---|-------|--------|
| 1 | Dry run reports the diff, mutates nothing | ✅ `System.State: New -> Active`, `executed:false`, `outcome:dry_run`, exit 0. Re-run showed `from` still `New`, proving no mutation. |
| 2 | Audit baseline | ✅ `audit.jsonl` did not exist; 0 lines |
| 3 | `--confirm` executes | ✅ `executed:true`, `outcome:executed`, exit 0, exactly 1 new audit line |
| 4 | Repeat refused as no-op | ✅ exit 4, `outcome:no_op`, `changes:[]`, no new audit line — on BOTH the `--confirm` and dry-run paths |
| 5 | Audit record shape | ✅ keys exactly `["changes","command","outcome","target","timestamp"]`; leak scan for `argv`/token/PAT/bearer/URL/actor returned 0 hits |
| 6 | `pipelines queue` without `--branch` | ✅ exit 3, error on stderr, no ADO call attempted (used a nonexistent definition id AND project — still failed at the flag guard first) |
| 7 | TUI effort modal preserves blank fields | ❌ **FAILED — blank field silently overwritten with 0** |

### Step 7 failure — SAFE-03 is only half satisfied

Predicted from source before running, then confirmed empirically.

- `internal/ui/workitems/effort.go:193-199` — `parseFloat("")` returns `0, nil`. A blank input
  becomes `0.0`, not "absent".
- `internal/ui/workitems/workitems.go:362-374` — all three effort values are then always passed
  as non-nil pointers. The in-source comment states it plainly: *"All three are ALWAYS non-nil
  here — TUI behaviour is unchanged."*
- `EffortSubmitMsg` still carries plain `float64`, not `*float64`, so the modal has no
  representation for "leave unchanged".

Observed: a Task closed through the modal with Original Estimate left blank had its Original
Estimate overwritten with 0. (`Completed` cannot be tested this way — `effort.go:144` rejects
a value <= 0, so it can never be blank.)

**The TUI write path bypasses every Phase 4 guard.** It does not route through `runWrite`, so
there is no dry run, no `--confirm`, and no audit record. The observed data loss left no trace
in `audit.jsonl`. That makes this a gap against SAFE-01 and SAFE-02 for that path as well, not
only SAFE-03.

| Path | SAFE-03 |
|------|---------|
| CLI (`--original-estimate` / `--remaining` / `--completed`) | ✅ omitted -> no patch op; `--remaining 0` is a real zero |
| TUI effort modal | ❌ blank -> explicit `value: 0` -> field overwritten |

Not a regression from this phase: D-17 locked "preserve TUI behaviour exactly", so 04-02
deliberately plumbed the nullable signature to the boundary and stopped. Remediation is a NEW
decision — tracked in `.planning/BACKLOG.md`, not a correction within Phase 4.

### Three findings from the CLI run that no fake produced

1. **ADO refused the first write, and the refusal path was correct.** Initial target `Closed`
   hit a Kinetic project rule: `TF401320: Rule Error for field Completed Work. Error code:
   Required, InvalidEmpty.` The CLI reported `outcome:"failed"`, `executed:false`, exit 1
   (P-08), and wrote **no** audit record. The "ADO refused a write" branch was exercised by a
   real rule engine — it was not one of the seven steps.
2. **A dry run creates `audit.jsonl` as a side effect** (empty, 0 records). D-14 holds, but the
   D-13 pre-flight open runs on dry runs too, so a preview is not filesystem-inert. Defensible
   — you learn the sink is unwritable before typing `--confirm` — but undocumented, and not what
   "reports what would change and does nothing" implies.
3. **Exit 1 is ambiguous in practice.** It means both "a read did not reach every configured
   project" and "Azure DevOps refused a write". The envelope disambiguates via
   `outcome:"failed"`, but the exit code alone does not, which weakens the SAFE-04 claim that
   outcomes are distinguishable. Flagged as a known cost at the P-01 ratification; confirmed
   live.

## Superseded — original deferral record (UNVERIFIED at time of writing)

**Superseded by the run above on 2026-09-12. Retained for history.**

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
