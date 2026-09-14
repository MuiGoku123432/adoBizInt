---
phase: quick-260913-uhj
plan: 01
subsystem: api
tags: [go, audit, exit-codes, azure-devops-sdk, paging, release-approvals]

requires:
  - phase: 04-guarded-writes
    provides: writeOp guard body, write envelope, audit sink, writeExitCodeFor
provides:
  - "Audit-append failure on an executed write downgrades the reported outcome to unknown / exit 5 (CR-01)"
  - "Successful audit appends are fsync'd before the command reports its outcome"
  - "auditOpen test seam for injecting a sink that opens cleanly and fails on Write"
  - "GetReleaseApprovalDetail locates an approval on any page and under any of pending/approved/rejected (WR-03)"
affects: [04-guarded-writes verification, releases approve, releases reject, BL-01 part 2]

actuals:
  tokens: 4700
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Package-level func var as an injection seam for failure modes a pre-flight check cannot cover"
    - "Per-status fan-out plus bounded continuation-token paging over an SDK list endpoint"

key-files:
  created: []
  modified:
    - cmd/adoBizInt/audit.go
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/audit_test.go
    - internal/ado/preread.go
    - internal/ado/preread_test.go

key-decisions:
  - "Audit-append failure reuses the ratified outcomeUnknown / exitOutcomeUnknown pair rather than adding a seventh exit code — the mutation landed and nothing recorded it, which genuinely IS an unknown outcome"
  - "executed is left to fall out of `outcome == outcomeExecuted` at the finishWrite call site rather than being set independently, so the two can never disagree"
  - "The WR-03 failure had TWO real causes, not one: the comma-joined ApprovalStatus cast AND the unpaged 50-item default. Both are fixed."
  - "The audit-failure test uses an already-closed *os.File rather than filesystem permissions — permission-based tests are inert as root (WR-09)"

patterns-established:
  - "Seam vars carry a doc comment stating the exact failure window they exist to reach, and that nothing assigns them from argv"
  - "Paging loops terminate on empty page AND absent/unparseable continuation token AND ctx.Err() AND a hard iteration bound"

requirements-completed: [CR-01, WR-03]

coverage:
  - id: D1
    description: "An executed write whose audit record cannot be persisted exits 5 with outcome unknown and executed false, and emits the operator-actionable stderr line"
    requirement: "CR-01"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestAuditAppendFailureReportsUnknown"
        status: pass
    human_judgment: false
  - id: D2
    description: "A successful audit append is fsync'd before the command reports its outcome"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestExecutedWriteAppendsAuditRecord"
        status: pass
    human_judgment: false
  - id: D3
    description: "GetReleaseApprovalDetail returns the approval for any id regardless of page position and regardless of pending/approved/rejected status"
    requirement: "WR-03"
    verification:
      - kind: unit
        ref: "internal/ado/preread_test.go#TestGetReleaseApprovalDetailPaging"
        status: pass
      - kind: unit
        ref: "internal/ado/preread_test.go#TestGetReleaseApprovalDetail"
        status: pass
    human_judgment: false
  - id: D4
    description: "A genuinely absent approval still returns a non-nil error, never (nil, nil)"
    verification:
      - kind: unit
        ref: "internal/ado/preread_test.go#TestPreReadNotFoundReturnsError"
        status: pass
      - kind: unit
        ref: "internal/ado/preread_test.go#TestPreReadNeverReturnsNilNilStructural"
        status: pass
    human_judgment: false
  - id: D5
    description: "`adoBizInt releases approve <id> --project Kinetic` works live against Azure DevOps on an id that `releases approvals` lists"
    verification: []
    human_judgment: true
    rationale: "Requires a live PAT and a real pending approval in Azure DevOps. The unit tests pin both diagnosed causes, but only a live run proves the REST endpoint accepts the corrected single-enum statusFilter and the continuation-token round trip. Listed as optional in the plan's <verification>."

duration: 14 min
completed: 2026-09-14
status: complete
---

# Quick Task 260913-uhj: Fix CR-01 and WR-03 Summary

**Audit-append failure now downgrades an executed write to exit 5 / `outcome:"unknown"` with a durable fsync'd append, and the release-approval pre-read is fixed for BOTH real causes — an invalid comma-joined status filter and an unpaged 50-item default.**

## Performance

- **Duration:** 14 min
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- **CR-01 closed.** An executed write whose `appendAuditRecord` fails now reports `outcome:"unknown"` / `executed:false` / exit 5 instead of exit 0 / `"executed"`. SAFE-02 is no longer falsifiable while the CLI reports success.
- **Durability.** `appendAuditRecord` now returns `f.Sync()`'s error, so a record reported as written is on disk.
- **WR-03 diagnosed and closed.** Both candidate causes were real; the diagnosis and the SDK types that settled each are recorded in a comment above `GetReleaseApprovalDetail`.
- Two regression tests, each verified to FAIL against the pre-fix code.

## Task Commits

1. **Task 1: Audit-append failure downgrades the reported outcome** — `1a6d73f` (fix)
2. **Task 2: Diagnose and fix the release-approval pre-read lookup** — `6afc26a` (fix)
3. **Task 3: Regression test pinning the paged / fanned-out lookup** — `5ef869c` (test)

## Files Created/Modified

- `cmd/adoBizInt/audit.go` — added the `auditOpen` seam var; `appendAuditRecord` now fsyncs and returns the sync error
- `cmd/adoBizInt/write.go` — call site switched to `auditOpen()`; audit-failure branch reassigns `outcome`/`code` and emits the operator-actionable stderr line
- `cmd/adoBizInt/audit_test.go` — `failingAuditSink` helper and `TestAuditAppendFailureReportsUnknown`
- `internal/ado/preread.go` — `GetReleaseApprovalDetail` rewritten as a per-status fan-out with bounded continuation-token paging, plus the WR-03 diagnosis comment
- `internal/ado/preread_test.go` — `TestGetReleaseApprovalDetailPaging`

## Decisions Made

### WR-03: the diagnosis (Task 2)

The plan explicitly refused to pre-commit to a cause. **Both were real.**

**Cause 1 — invalid status filter.** `release.ApprovalStatus` is a plain `type ApprovalStatus string` alias, and `release.ApprovalStatusValues` enumerates only single tokens (`undefined`/`pending`/`approved`/`rejected`/`reassigned`/`canceled`/`skipped`). The SDK forwards it verbatim at `release/client.go:285-287` as `queryParams.Add("statusFilter", string(*args.StatusFilter))` — no comma handling anywhere on that path. Contrast `ReleaseIdsFilter` at `client.go:288-295`, which the SDK *explicitly* joins with `,` precisely because that parameter does accept a list. The previous `release.ApprovalStatus("pending,approved,rejected")` cast therefore put a comma list where the endpoint documents one enum token. **The SDK type that settled it: `release.GetApprovalsArgs.StatusFilter *ApprovalStatus` together with the `approvalStatusValuesType` enum.**

**Cause 2 — no paging.** `GetApprovalsArgs.Top` is documented as *"Number of approvals to get. Default is 50"*, `ContinuationToken *int` exists, and `GetApprovalsResponseValue.ContinuationToken string` was never read. A single unpaged call therefore saw at most the first 50 approvals of a project.

The live counter-evidence cited in the plan — `releases.go:241-246` returning 56 approvals from an unpaged call — does **not** acquit paging. That 56 is the sum across the `targetProjects` loop, not one response; each individual response was still capped at 50.

The fix addresses both: one call per status, each paged to exhaustion, bounded on empty page, absent/unparseable continuation token, `ctx.Err()`, and a hard 50-page cap. The status set was **not** narrowed back to pending only — that would have silently broken the D-03 no-op case.

### CR-01 design notes

- Reuses `outcomeUnknown` / `exitOutcomeUnknown`; no seventh exit code invented.
- `executed` is not set independently — `finishWrite` computes it as `outcome == outcomeExecuted`, so it falls to false automatically. The test asserts it separately anyway, so a future refactor that decouples them fails loudly.
- The downgrade is idempotent: a run already classified unknown by `writeExitCodeFor` is unchanged.
- The `auditOpen` seam reaches the ENOSPC/EIO/quota window that D-13's pre-flight open provably cannot cover — D-13 proves the sink was writable *at open time*, not that the append succeeds.

### Superseded acceptance criterion (noted per plan)

`04-03-SUMMARY.md` / `04-03-PLAN.md` carry a manual criterion phrased as *"the `openAuditSink(` line number is less than every mutate-call line number"*. That **literal** criterion is now SUPERSEDED: the call site is `auditOpen(` (write.go:196). Nothing actually breaks — the D-13 guard is behavioural, pinned by `TestUnwritableAuditSinkPreventsMutation`, which still passes. The line order 196 → 230 is unchanged.

## Deviations from Plan

None - plan executed exactly as written.

The plan left the WR-03 mechanism deliberately open; selecting "both causes" is the diagnosis the plan asked for, not a deviation.

## Issues Encountered

None.

## Regression Proofs

Both new tests were verified to fail against the pre-fix code, then restored:

- `TestAuditAppendFailureReportsUnknown` with the `outcome`/`code` reassignment removed: `exit = 0, want 5`.
- `TestGetReleaseApprovalDetailPaging` against `HEAD~1:internal/ado/preread.go`: `StatusFilter "pending,approved,rejected" is a comma list`.

## Verification

`go build ./...` ✅ · `go test ./...` ✅ all packages · `go vet ./...` ✅ · `gofmt -l .` ✅ prints nothing.

Optional live confirmation (D5) is unrun — requires a PAT and a real pending approval.

## User Setup Required

None.

## Self-Check: PASSED

- All five modified files exist on disk.
- Commits `1a6d73f`, `6afc26a`, `5ef869c` present in `git log`.
- All task acceptance criteria re-run and passing.

---
*Quick task: 260913-uhj*
*Completed: 2026-09-14*
