---
phase: quick-260913-nd2
plan: 01
subsystem: ui
tags: [bubbletea, tui, go, azure-devops, nullable]

requires:
  - phase: 04-02
    provides: "ado.UpdateWorkItemStateWithEffort nullable *float64 signature that omits patch ops for nil"
provides:
  - "Nullable effort values end to end through the Bubble Tea TUI: a blank field leaves the work item value unchanged"
  - "parseEffort returning (*float64, error) with (nil, nil) for blank/whitespace input"
  - "Executable guards pinning blank-means-absent and explicit-zero-means-zero"
  - "go/ast structural guard against re-introducing address-of-local at either call site"
affects: [BL-01 part 2, TUI write routing, effort modal]

actuals:
  tokens: 9800
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "go/ast structural assertions instead of text-negative grep criteria"
    - "Nullable domain values as *float64 where nil means absent and a pointer to 0 means a deliberate zero"

key-files:
  created:
    - internal/ui/workitems/effort_test.go
  modified:
    - internal/ui/workitems/effort.go
    - internal/ui/workitems/workitems.go
    - internal/ui/workitems/workitems_test.go

key-decisions:
  - "Tasks 1 and 2 committed together: Task 1 alone does not compile because it widens EffortSubmitMsg while the call sites still take addresses of locals. Splitting would have landed a non-building commit."
  - "A blank Completed field is rejected by the existing greater-than-zero rule (nil is treated as invalid, not as leave-unchanged), keeping Completed required."
  - "An explicitly typed 0 remains a real zero write via a non-nil pointer, mirroring CLI decision D-18."

patterns-established:
  - "Structural AST guards: comments are not AST nodes, so prose cannot defeat the assertion the way it defeats a text search."

requirements-completed: [BL-01-part1, SAFE-03]

coverage:
  - id: D1
    description: "A blank Original Estimate or Remaining field emits a nil pointer, so no patch operation reaches Azure DevOps and the stored value is unchanged."
    requirement: BL-01-part1
    verification:
      - kind: unit
        ref: "internal/ui/workitems/effort_test.go#TestEffortModalSubmitNullability/blank_original_estimate_and_remaining_are_absent"
        status: pass
      - kind: unit
        ref: "internal/ui/workitems/effort_test.go#TestEffortModalSubmitNullability/whitespace-only_original_estimate_is_absent"
        status: pass
    human_judgment: false
  - id: D2
    description: "An explicitly typed 0 still produces a non-nil pointer to 0.0 and remains a real zero write (D-18)."
    requirement: BL-01-part1
    verification:
      - kind: unit
        ref: "internal/ui/workitems/effort_test.go#TestEffortModalSubmitNullability/explicit_zero_original_estimate_is_a_real_zero_write"
        status: pass
    human_judgment: false
  - id: D3
    description: "A blank Completed field is still rejected by the greater-than-zero rule and emits no message."
    requirement: BL-01-part1
    verification:
      - kind: unit
        ref: "internal/ui/workitems/effort_test.go#TestEffortModalSubmitNullability/blank_completed_is_rejected"
        status: pass
    human_judgment: false
  - id: D4
    description: "Both UpdateWorkItemStateWithEffort call sites forward the message pointers directly; neither takes the address of a local copy."
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "internal/ui/workitems/workitems_test.go#TestUpdateWorkItemStateWithEffortCallSitesForwardPointers"
        status: pass
    human_judgment: false
  - id: D5
    description: "internal/ado is unmodified and its D-17 regression guard still passes unchanged."
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortIssuesExactPatch"
        status: pass
      - kind: other
        ref: "git diff --stat internal/ado/ (empty)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Closing a real Task through the TUI with Original Estimate blank leaves the stored estimate unchanged in Azure DevOps."
    requirement: BL-01-part1
    verification: []
    human_judgment: true
    rationale: "The original bug was confirmed against a live work item; only a live round-trip against Azure DevOps proves the end-to-end fix. No test in this repo issues a real API call."

duration: 12 min
completed: 2026-09-13
status: complete
---

# Quick 260913-nd2 Plan 01: Fix BL-01 TUI Effort Modal Silently Zeroing Summary

**Blank effort fields in the Bubble Tea modal now travel as nil `*float64` instead of 0, so a blank Original Estimate no longer overwrites the stored Azure DevOps value, while an explicitly typed `0` remains a real zero write.**

## Performance

- **Duration:** ~12 min
- **Tasks:** 3
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments

- `EffortSubmitMsg.OriginalEstimate/.Remaining/.Completed` widened from `float64` to `*float64`, with nil documented as "user named no value, leave unchanged".
- `parseFloat` replaced by `parseEffort(string) (*float64, error)` returning `(nil, nil)` for blank or whitespace-only input.
- Both `UpdateWorkItemStateWithEffort` call sites in `workitems.go` now forward `effortMsg` selectors directly; the three local copies and their now-false aliasing/non-nil comments are gone.
- `Completed` remains required: a nil Completed is rejected by the existing greater-than-zero rule.
- New `effort_test.go` table test covers blank, whitespace, explicit zero, all-supplied, blank-completed and invalid-input paths.
- New `go/ast` structural guard in `workitems_test.go` fails if either call site regresses to an address-of expression. Verified red against a compiling revert, then restored.

## Task Commits

1. **Task 1 + Task 2: nullable message type and call-site forwarding** - `b2f2ae2` (fix)
2. **Task 3: blank-means-absent and call-site guards** - `266b88b` (test)

## Files Created/Modified

- `internal/ui/workitems/effort.go` - Nullable `EffortSubmitMsg` fields; `parseEffort` replaces `parseFloat`; nil Completed rejected.
- `internal/ui/workitems/workitems.go` - Both call sites forward message pointers; stale comments replaced.
- `internal/ui/workitems/effort_test.go` - New table test pinning nullability semantics at the modal level.
- `internal/ui/workitems/workitems_test.go` - Appended go/ast call-site structural guard.

## Decisions Made

- **Tasks 1 and 2 committed as one commit.** Task 1's own acceptance criterion requires `go build ./...` to succeed, but widening `EffortSubmitMsg` without fixing the call sites is a compile error (`**float64` vs `*float64`). The plan's Task 1 `<done>` acknowledges this ("call sites are fixed in Task 2"), so the two were committed atomically rather than landing a broken commit.
- **Blank Completed treated as invalid, not absent.** Per the plan, Completed stays a required product rule; the nil check sits alongside the existing `*completed <= 0` check and reuses the same message.
- **AST guard asserts positively (selector on `effortMsg`) as well as negatively (no `token.AND` unary).** The revert experiment produced a `*ast.StarExpr`, which the negative-only check would have missed — the positive shape assertion caught it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 1 and Task 2 merged into a single commit**
- **Found during:** Task 1
- **Issue:** Widening `EffortSubmitMsg` to `*float64` breaks compilation at both `workitems.go` call sites, which Task 2 fixes. Task 1's acceptance criteria require a green `go build ./...`.
- **Fix:** Applied Task 1 and Task 2 edits, then committed once as `b2f2ae2` with both changes described.
- **Files modified:** `internal/ui/workitems/effort.go`, `internal/ui/workitems/workitems.go`
- **Verification:** `go build ./... && go vet ./... && gofmt -l .` all clean before commit.
- **Committed in:** `b2f2ae2`

**2. [Rule 1 - Bug] AST guard initially proved by a build failure, not a test failure**
- **Found during:** Task 3 acceptance verification
- **Issue:** The first revert experiment (`&originalEstimate` with no matching local declaration) failed to compile, so the guard was never exercised — a build error is not proof the test has teeth.
- **Fix:** Re-ran the experiment with a compiling revert (`*&originalEstimate` plus the local declaration). The guard went red with `call site 1 arg 4 is *ast.StarExpr, want *ast.SelectorExpr on effortMsg`, then the revert was restored and the suite returned green.
- **Files modified:** none (temporary, reverted)
- **Verification:** `go test ./internal/ui/workitems/ -run CallSites` red during the experiment, `ok` after restore.
- **Committed in:** n/a (verification only)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 verification correctness)
**Impact on plan:** No scope creep. `internal/ado/` untouched as required; the D-17 guard passes byte-identical.

## Issues Encountered

None beyond the two deviations above.

## Verification Results

- `go build ./...` — clean
- `go test ./...` — all packages pass
- `go vet ./...` — clean
- `gofmt -l .` — no output
- `git diff --stat internal/ado/` — empty
- `TestUpdateWorkItemStateWithEffortIssuesExactPatch` and `TestUpdateWorkItemStateWithEffortOmitsNilFields` — both PASS, file unmodified

## User Setup Required

None.

## Next Phase Readiness

- BL-01 part 1 is closed. Part 2 (routing TUI writes through `runWrite` for dry-run / `--confirm` / audit records) remains open in `.planning/BACKLOG.md` and was not touched.
- Recommended: a live round-trip against a real Task (coverage item D6) to confirm the end-to-end behaviour that reproduced the original bug.

## Self-Check: PASSED

- `internal/ui/workitems/effort_test.go` exists on disk.
- Commits `b2f2ae2` and `266b88b` both present in `git log`.
- All task acceptance criteria re-run and passing.

---
*Quick task: 260913-nd2*
*Completed: 2026-09-13*
