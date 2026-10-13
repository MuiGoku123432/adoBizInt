---
phase: 07-guarded-tui-writes
plan: 01
subsystem: infra
tags: [go, refactor, fetchctx, test-lock, sha256]

requires:
  - phase: 03
    provides: fetchctx.NewWithParent (CLI --timeout path)
provides:
  - "package internal/fetchctx (New, NewWithParent) importable by non-UI packages such as internal/guard"
  - "07-cli-tests.sha256: phase-long byte lock on seven CLI test files (GUARD-06)"
  - "07-cli-audit-test.sha256: lock on cmd/adoBizInt/audit_test.go, valid until plan 07-03"
  - "07-cli-test-inventory.txt: 85 top-level CLI test names, LC_ALL=C sorted, for comm checks"
affects: [07-02, 07-03, internal/guard]

actuals:
  tokens: 2500
  tasks: 2
  commits: 2
plan_head_before: 4c108f4e12d33cb2a78fb40c4f0f078ded9eb429
plan_head_after: 83f9918da5a3032d7c3dd7b1400fc79a6958ff41

tech-stack:
  added: []
  patterns:
    - "Test lock via committed sha256 manifests plus a sorted test-name inventory, checked by later plans with shasum -c and comm"

key-files:
  created:
    - .planning/phases/07-guarded-tui-writes/07-cli-tests.sha256
    - .planning/phases/07-guarded-tui-writes/07-cli-audit-test.sha256
    - .planning/phases/07-guarded-tui-writes/07-cli-test-inventory.txt
  modified:
    - internal/fetchctx/fetchctx.go (moved from internal/ui/fetchctx, doc comment only)
    - internal/fetchctx/fetchctx_test.go (moved, unchanged)
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/write.go
    - internal/ui/dashboard/dashboard.go
    - internal/ui/releases/releases.go
    - internal/ui/prs/prs.go
    - internal/ui/pipelines/pipelines.go
    - internal/ui/workitems/workitems.go
    - internal/ui/branches/branches.go

key-decisions:
  - "fetchctx relocated to internal/fetchctx so internal/guard can import it without depending on internal/ui (GUARD-01 prerequisite)"
  - "CLI test lock taken at commit 059a4d1: seven files locked for the whole phase, audit_test.go locked until 07-03, crash_test.go deliberately unlocked (07-02 relocates TestWriteRecoverPrecedesMutate)"

patterns-established:
  - "Phase test lock: sha256 manifests with repo-root-relative paths plus an LC_ALL=C sorted inventory from go test -list"

requirements-completed: [GUARD-06, GUARD-01]

coverage:
  - id: D1
    description: "fetchctx lives at internal/fetchctx; old internal/ui/fetchctx is gone; all 8 importers use the new path; suite green"
    requirement: GUARD-01
    verification:
      - kind: unit
        ref: "go build ./... && go vet ./... && go test -race -count=1 ./... && test -z \"$(gofmt -l .)\""
        status: pass
      - kind: unit
        ref: "internal/fetchctx/fetchctx_test.go#TestNewWithParentAppliesBudget|TestNewWithParentZeroBudgetStillHasDeadline|TestNewWithParentPropagatesParentCancellation|TestNewIsUnchanged"
        status: pass
    human_judgment: false
  - id: D2
    description: "CLI write tests locked: two sha256 manifests check OK and the inventory holds 85 sorted names including the four required tests"
    requirement: GUARD-06
    verification:
      - kind: other
        ref: "shasum -a 256 -c 07-cli-tests.sha256 && shasum -a 256 -c 07-cli-audit-test.sha256 && grep -qx TestWriteRecoverPrecedesMutate|TestExecutedWriteAppendsAuditRecord 07-cli-test-inventory.txt"
        status: pass
    human_judgment: false

duration: 2min
completed: 2026-10-13
status: complete
---

# Phase 7 Plan 01: fetchctx relocation and CLI test lock Summary

**The bounded-context helper now lives at internal/fetchctx, so internal/guard can use it without importing the UI tree. The seven CLI test files, plus audit_test.go and the 85-test inventory, are byte-locked at commit 059a4d1 so later plans can prove the guard extraction left them unchanged.**

## Performance

- **Duration:** about 2 min
- **Started:** 2026-10-13T20:03:27Z
- **Completed:** 2026-10-13T20:05Z
- **Tasks:** 2
- **Files modified:** 13 (10 code via one mechanical rename, 3 lock files)

## Accomplishments

- `git mv internal/ui/fetchctx internal/fetchctx`. History follows the move: `git log --follow` shows commits 1555f13 and fff468c from before it. `New`, `NewWithParent` and `defaultBudget` are byte-identical.
- The import path is rewritten in all eight importers: the CLI files `dispatch.go` and `write.go`, and the TUI views dashboard, releases, prs, pipelines, workitems and branches. gofmt re-sorted the two CLI import groups.
- The package doc comment now says the package bounds reads (CLI and TUI) and guarded writes. The old claim that writes keep their own contexts is gone. The paragraph explaining why the comment never spells the root-context constructor is kept, and `context.Background` still appears exactly once in the file.
- Lock files were committed at **059a4d1**, the HEAD when the lock was taken:
  - `07-cli-tests.sha256`: 7 lines covering fakes, filter, main, patleak, usage, write and writeverbs `_test.go`. All OK.
  - `07-cli-audit-test.sha256`: 1 line for `audit_test.go`. OK.
  - `07-cli-test-inventory.txt`: **85** names, all matching `^Test`, `LC_ALL=C` sorted. It contains TestWriteRecoverPrecedesMutate, TestExecutedWriteAppendsAuditRecord, TestAuditRecordOmitsArgvAndCredentialFields and TestPanicAfterWriteIssuedIsAudited.
- **No test file changed.** `git diff 4c108f4..HEAD -- 'cmd/adoBizInt/*_test.go'` is empty.

## Task Commits

1. **Task 1 (tracer): relocate fetchctx to internal/fetchctx.** Commit `059a4d1` (refactor). The tracer gate re-ran verify on the committed tree and it passed.
2. **Task 2: lock the CLI write tests.** Commit `83f9918` (test).

## Files Created/Modified

- `internal/fetchctx/fetchctx.go`: moved; only the doc comment changed
- `internal/fetchctx/fetchctx_test.go`: moved, unchanged
- 8 importers: one import line each
- `.planning/phases/07-guarded-tui-writes/07-cli-tests.sha256`, `07-cli-audit-test.sha256`, `07-cli-test-inventory.txt`: new lock files

## Decisions Made

- I added one sentence to the doc comment saying why the package sits outside internal/ui: non-UI importers such as the CLI and the guard. It replaces the dropped sentence about writes.

## Deviations from Plan

None in the plan's scope. One process note: the first `git add` for Task 1 failed on a stale pathspec (`internal/ui/fetchctx`, already gone after `git mv`). The resulting commit 6fafc60 held only the two renames, so it did not compile. Before doing anything else, I amended that unpushed commit to include all 10 files. It is now `059a4d1`, and no non-compiling commit remains in history.

## Issues Encountered

None.

## Known Stubs

None.

## Next Phase Readiness

Ready for 07-02. `internal/guard` can import `sentinovo.ai/bizInt/internal/fetchctx`. Plans 07-02 and 07-03 should run `shasum -a 256 -c` on both manifests and `comm` against the inventory, using `LC_ALL=C`.

## Self-Check: PASSED

- FOUND: internal/fetchctx/fetchctx.go, internal/fetchctx/fetchctx_test.go, all three lock files; internal/ui/fetchctx absent
- FOUND commits: 059a4d1, 83f9918 (ancestors of HEAD)
- Module gate and lock gate re-run green after the final commit
