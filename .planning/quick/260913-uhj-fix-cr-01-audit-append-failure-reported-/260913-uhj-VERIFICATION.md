---
task: 260913-uhj
verified: 2026-09-14T10:15:00Z
status: passed
score: 9/9 checks verified
behavior_unverified: 0
commits: [1a6d73f, 6afc26a, 5ef869c]
base: bea1a7f
---

# Quick Task 260913-uhj Verification Report

**Goals:** CR-01 (audit-append failure must not report success) and WR-03 (`releases approve`/`reject` cannot find a listed approval).
**Status:** PASSED

## Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Audit-append failure downgrades to unknown | VERIFIED | `write.go:265-266` sets `outcome = outcomeUnknown; code = exitOutcomeUnknown` inside the `auditErr != nil` branch |
| 2 | `executed` falls to false without a new set | VERIFIED | `write.go:277` returns `finishWrite(..., outcome == outcomeExecuted, outcome, code, ...)`. `grep executed` on write.go shows no independent assignment; only the struct field at :72 and the `finishWrite` param at :537/:542 |
| 3 | CR-01 test has teeth (mutation-proved) | VERIFIED | Deleted the two-line reassignment and re-ran: `exit = 0, want 5` — FAIL. Restored, tree clean. See below |
| 4 | Test uses the closed-file seam, not permissions | VERIFIED | `audit_test.go` `failingAuditSink` injects `auditOpen` returning an already-`Close()`d `*os.File`; runtime error is `file already closed`. No chmod anywhere |
| 5 | `f.Sync()` added; durability claim real | VERIFIED | `audit.go` `appendAuditRecord` now `Write` → return on error → `return f.Sync()`. Sync error is returned as the append error, so a failed fsync also triggers the CR-01 downgrade |
| 6 | WR-03 diagnosis comment exists and matches SDK | VERIFIED | Comment above `GetReleaseApprovalDetail` names both causes; verified against SDK v1.0.0-b5 (see table below) |
| 7 | D-03 no-op survives (status set not narrowed) | VERIFIED | `preread.go` fans out over Pending/Approved/Rejected; `preread_test.go` has an "already approved is still found" case plus a final block asserting all three statuses were requested |
| 8 | `(nil, nil)` prohibition holds | VERIFIED | `TestPreReadNeverReturnsNilNilStructural` PASS (uncached) |
| 9 | Paging loop bounded | VERIFIED | `maxPagesPerStatus = 50` hard bound, plus breaks on empty page, unparseable/absent token, and `ctx.Err()`. A server echoing one token forever terminates at 50 pages × 3 statuses = 150 calls max |

**Score:** 9/9

## CR-01 Mutation Proof

Removed only `outcome = outcomeUnknown` / `code = exitOutcomeUnknown`:

```
--- FAIL: TestAuditAppendFailureReportsUnknown/executed_write_is_downgraded_to_unknown
    audit_test.go:271: exit = 0, want 5
```

Row 1 is genuinely load-bearing. Row 2 (503 already-unknown) passes with or without the fix — the test's own comment says exactly that, which is honest. Working tree restored clean after the experiment.

The test asserts three independent things: exit code `5`, decoded `outcome == "unknown"`, and `Executed == false` — with an explicit comment that the `executed` assertion is deliberately independent of `outcome` so a future refactor cannot reintroduce `executed:true / outcome:"unknown"`. Stderr text is a fourth, secondary assertion, not the load-bearing one.

## WR-03 SDK Claim Audit

Module: `github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5/release`

| Claim in comment | SDK evidence | Verdict |
|---|---|---|
| `ApprovalStatus` is a plain string alias | `models.go:178: type ApprovalStatus string` | Correct |
| Forwarded verbatim, no comma handling | `client.go:286: queryParams.Add("statusFilter", string(*args.StatusFilter))` | Correct |
| Contrast: `ReleaseIdsFilter` IS joined with ',' | `client.go:288-295` loops and joins | Correct |
| `Top` defaults to 50 | `client.go:335: // (optional) Number of approvals to get. Default is 50.` | Correct |
| Response token never read | request `ContinuationToken *int` (`:338`), response `ContinuationToken string` (`:349`), set from header at `:318`; old code passed neither and read neither | Correct |

The comment's claim that BOTH causes were real is supported. The `*int` request field vs `string` response field mismatch also justifies the `strconv.Atoi` step and its break-on-error.

## Live Confirmation (recorded from orchestrator, not re-run)

WR-03 A/B on approval `21611` (project Kinetic): pre-fix binary from `bea1a7f` → exit 3 "was not found"; post-fix → exit 0 correct dry-run envelope. `releases reject` and a second approval in another project also resolve. **WR-03 is confirmed fixed against live Azure DevOps.**

## Toolchain

```
go build ./...   BUILD_OK
go vet ./...     VET_OK
gofmt -l .       (no output)
go test -count=1 ./...   all ok, 0 failures
```

No `TODO`/`FIXME`/`XXX`/`HACK`/`PLACEHOLDER` markers in any of the five changed files.

## Scope

`git diff --stat bea1a7f..HEAD -- . ':!.planning'` touches exactly five files: `audit.go`, `audit_test.go`, `write.go`, `preread.go`, `preread_test.go` (+365/-29). WR-01, WR-02, WR-07, WR-09, BL-01 part 2 and a `write.go` split were **not** attempted. Scope is clean.

## Honest Assessment: CR-01 has no live confirmation

Correct — there is no live run against a genuinely unwritable sink. I judge that **acceptable, and the closed-file seam sufficient**, for these reasons:

- The failure mode under test is "open succeeds, `Write` returns an error." A closed descriptor reproduces that at the same call site with the same `error` shape the real ENOSPC/EIO/quota window produces. The code under test cannot distinguish them — it branches on `auditErr != nil`, nothing more.
- The mutation experiment proves the assertion is load-bearing, which is stronger evidence than a one-off live run would be: a live run proves it worked once; the mutation proves the test will fail if it stops working.
- The residual gap is narrow and not about CR-01's logic: it is whether a real ENOSPC surfaces as an error from `Write`/`Sync` rather than being silently swallowed by the OS. `f.Sync()` materially shrinks even that gap, since delayed-allocation ENOSPC typically surfaces at fsync, not at write.

Recommend no follow-up work on this point.

---

_Verifier: gsd-verifier_
