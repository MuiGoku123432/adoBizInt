---
phase: quick-260913-uhj
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd/adoBizInt/audit.go
  - cmd/adoBizInt/write.go
  - cmd/adoBizInt/audit_test.go
  - internal/ado/preread.go
  - internal/ado/preread_test.go
autonomous: true
requirements: [CR-01, WR-03]
estimate:
  tokens: 60000
  raw_tokens: 30000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "An executed write whose audit record cannot be persisted exits 5 (exitOutcomeUnknown) with outcome \"unknown\", never exit 0 / \"executed\" (CR-01)."
    - "That path emits the operator-actionable stderr message \"Write may have landed but was NOT recorded in <path>: <err>\", telling the operator what to go check."
    - "A successful audit append is durable: the record is fsync'd before the command reports its outcome."
    - "`GetReleaseApprovalDetail` returns the approval for any id that `releases approvals` lists, regardless of page position and regardless of whether its status is pending, approved, or rejected (WR-03). The contract is mechanism-neutral: paging, status-filter fan-out, or both may be the real fix."
    - "A comment above `GetReleaseApprovalDetail` names which of the two candidate causes (unpaged scan / comma-joined status cast) was real — or states that both were — citing the SDK type that settled it."
    - "An already-approved approval remains visible to the pre-read, so D-03's no-op refusal still fires."
    - "A genuinely absent approval still returns a non-nil error (never (nil, nil)), keeping D-02 and TestPreReadNeverReturnsNilNilStructural green."
  artifacts:
    - cmd/adoBizInt/audit.go
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/audit_test.go
    - internal/ado/preread.go
    - internal/ado/preread_test.go
  key_links:
    - "write.go audit-append branch -> writeExitCodeFor's outcomeUnknown/exitOutcomeUnknown pair (reuse, do not invent a seventh code)."
    - "auditOpen seam var -> audit_test.go injecting an already-closed *os.File so open succeeds and Write fails."
    - "GetReleaseApprovalDetail -> whichever release.GetApprovalsArgs field Task 2's diagnosis implicates (paging: ContinuationToken/Top per the releases.go:144 precedent; status filter: ApprovalStatusValues fan-out per releases.go:243). Mechanism-neutral by design — do not pre-commit."
---

<objective>
Fix the two live defects from `.planning/phases/04-guarded-writes/04-REVIEW.md`:

- **CR-01 (blocker):** an executed write whose audit record fails to persist is reported as exit 0 / `outcome:"executed"`. Make it report `outcome:"unknown"` / exit 5, and make a successful append durable.
- **WR-03:** `releases approve` / `releases reject` are non-functional live — `GetReleaseApprovalDetail` fails to locate approvals that `releases approvals` successfully lists. The cause is NOT settled: it may be the unpaged scan, the comma-joined `release.ApprovalStatus("pending,approved,rejected")` cast at `preread.go:121`, or both. Task 2 diagnoses before fixing. Note the live counter-evidence for the paging theory: `releases.go:241-246` issues an unpaged `GetApprovals` with a single `ApprovalStatusValues.Pending` and returns 56 approvals in one call, so the unpaged call is not truncating at a small page.

Each gets a regression test. Nothing else in 04-REVIEW.md is in scope.

Purpose: SAFE-02 ("every executed write appends an audit record") is currently falsifiable while the CLI reports success — the exact conflation this project exists to prevent. And two of six write verbs do not work at all.
Output: modified `audit.go`, `write.go`, `preread.go` plus tests in `audit_test.go` and `preread_test.go`.
</objective>

<context>
@.planning/phases/04-guarded-writes/04-REVIEW.md
@cmd/adoBizInt/audit.go
@cmd/adoBizInt/write.go
@cmd/adoBizInt/exit.go
@internal/ado/preread.go
@internal/ado/releases.go
</context>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: Audit-append failure downgrades the reported outcome to unknown</name>
  <files>cmd/adoBizInt/audit.go, cmd/adoBizInt/write.go, cmd/adoBizInt/audit_test.go</files>
  <read_first>
    - cmd/adoBizInt/audit.go (all 100 lines — note `appendAuditRecord`'s doc comment stating the contract the call site breaks)
    - cmd/adoBizInt/write.go lines 190-262 (sink open at 196, audit branch at 244-252, return at 261)
    - cmd/adoBizInt/exit.go lines 31-97 (`outcomeUnknown`, `writeExitCodeFor` semantics)
    - cmd/adoBizInt/dispatch.go line 28 (`exitOutcomeUnknown = 5`)
    - cmd/adoBizInt/write_test.go lines 30-60 (`tempAuditSink`, `preReadFake`, `setStateArgs`)
    - cmd/adoBizInt/audit_test.go lines 1-70 and 190-216 (assertion helpers and the existing timed-out-write test)
  </read_first>
  <behavior>
    - An executed write whose `appendAuditRecord` returns an error exits `exitOutcomeUnknown` (5), not `exitOK`.
    - That same run reports `outcome:"unknown"` in the envelope, not `"executed"`, and reports `executed:false`.
    - The stderr line is upgraded to the operator-actionable wording prescribed by 04-REVIEW.md's CR-01 fix block: `Write may have landed but was NOT recorded in <path>: <err>`.
    - An executed write whose append succeeds is unchanged: exit 0, `outcome:"executed"`, one audit line on disk.
    - A write already classified `outcomeUnknown` whose append also fails stays exit 5 / `"unknown"` (idempotent downgrade, no double-penalty).
  </behavior>
  <action>
Introduce a test seam and make the audit failure load-bearing.

In `audit.go`: add a package-level `var auditOpen = openAuditSink` immediately below `openAuditSink`, documenting that it exists ONLY so a test can inject a sink that opens cleanly and fails on `Write` — the ENOSPC/EIO/quota window that D-13's pre-flight open provably does not cover. Nothing ever assigns it from argv, mirroring the existing `auditPath` var rationale.

In `audit.go`: after the successful `f.Write(payload)` in `appendAuditRecord`, call `f.Sync()` and return its error. Explain in the doc comment that without the sync the durability claim is untrue — a buffered line lost to a crash is exactly as invisible as one never written, and this file's entire purpose is to survive the moment something went wrong.

In `write.go` line 196: call `auditOpen()` instead of `openAuditSink()`. Do not move the call — its POSITION is D-13 and the surrounding comment says so.

In `write.go` lines 244-252: when `appendAuditRecord` returns an error, after logging, replace the existing `fmt.Fprintf` stderr text with 04-REVIEW.md's prescribed wording — `Write may have landed but was NOT recorded in %s: %v` (audit path, err) — because the whole point of the downgrade is to tell the operator what to go check, and "Failed to append audit record" does not. Update any existing test asserting on the old text. Then reassign `outcome = outcomeUnknown` and `code = exitOutcomeUnknown`. Comment the reason: the mutation landed in ADO and nothing recorded it, which genuinely IS an unknown outcome — this reuses already-ratified D-10/D-23 vocabulary rather than adding a seventh exit code, and `finishWrite`'s `executed` argument at line 261 then falls to false on its own because it is computed as `outcome == outcomeExecuted`.

Then write `TestAuditAppendFailureReportsUnknown` in `audit_test.go`, table-driven over the two mutate results, asserting exit code and the `outcome` and `executed` fields of the decoded envelope in both rows. Row 1 (nil mutate error → would have been executed) is the load-bearing CR-01 regression proof: it is the only row that fails against the current broken code. Row 2 (a 503 wrapped error → already `outcomeUnknown` via `writeExitCodeFor` at `exit.go:87-91`) pins IDEMPOTENCE of the downgrade only — the reassignment is a no-op there and the row passes both before and after the fix; keep it, but do not mistake it for coverage. Build the failing sink by opening a real file under `t.TempDir()`, closing it, and returning that `*os.File` from a swapped `auditOpen` restored via `t.Cleanup` — `Write` on a closed file returns `os.ErrClosed`. Do NOT use file permissions: permission-based tests are meaningless as root, which is WR-09's standing complaint.
  </action>
  <verify>
    <automated>go test ./cmd/adoBizInt/ -v</automated>
  </verify>
  <acceptance_criteria>
    - `TestAuditAppendFailureReportsUnknown` passes and fails if either the `outcome = outcomeUnknown` or the `code = exitOutcomeUnknown` reassignment is reverted (verify by temporarily reverting one, then restoring).
    - The test asserts the decoded envelope's `outcome` field equals `"unknown"` and the returned int equals `exitOutcomeUnknown` — not merely that stderr contains a message.
    - The test also asserts the decoded envelope's `write.executed` is false, so a future refactor that passes `executed` independently of `outcome` cannot reintroduce the `executed:true, outcome:"unknown"` envelope this fix exists to prevent.
    - `TestExecutedWriteAppendsAuditRecord` and every other existing test in `cmd/adoBizInt` still passes (the task's `<automated>` runs the whole package, so a regression in `TestWriteGuardMatrix` or `TestUnwritableAuditSinkPreventsMutation` surfaces here, not in Task 3).
    - Note in the SUMMARY that `04-03-SUMMARY.md` / `04-03-PLAN.md` carry a manual criterion phrased as "the `openAuditSink(` line number is less than every mutate-call line number"; that literal criterion is SUPERSEDED because the call site is now `auditOpen(`. The D-13 guard itself is behavioural (`TestUnwritableAuditSinkPreventsMutation`), so nothing actually breaks.
    - `openAuditSink` is still called before `op.mutate` in `write.go` (via `auditOpen`); the line order 196 → 230 is unchanged.
  </acceptance_criteria>
  <done>An executed write with an unwritable sink exits 5 with `outcome:"unknown"`, a successful append is fsync'd, and the regression test pins both.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Diagnose and fix the release-approval pre-read lookup</name>
  <files>internal/ado/preread.go</files>
  <read_first>
    - internal/ado/preread.go lines 110-153 (`GetReleaseApprovalDetail`)
    - internal/ado/releases.go lines 136-150 (the `Top` precedent on the releases list path) and lines 240-250 (the working `GetApprovals` call using `release.ApprovalStatusValues.Pending`)
    - the SDK's `release.GetApprovalsArgs` struct and the `release.ApprovalStatus` type in the module cache (find via `go doc github.com/microsoft/azure-devops-go-api/azuredevops/release.GetApprovalsArgs`) — you MUST confirm which paging fields exist (`Top`, `ContinuationToken`) and whether `ApprovalStatus` is a plain string alias before writing code
    - internal/ado/interfaces.go line 42-43 (the `releaseAPI` seam — do not widen it unless the fix genuinely requires a new method)
  </read_first>
  <behavior>
    - An approval present on the SECOND page of `GetApprovals` is found and returned.
    - An approval with status `approved` is found and returned (the D-03 no-op case must keep working).
    - An approval that exists on no page returns `(nil, non-nil error)` naming only the kind and the id.
    - Paging terminates: an empty page, or a response with no continuation token, ends the loop rather than spinning.
  </behavior>
  <action>
DIAGNOSE FIRST, do not assume. There are two candidate causes and the review explicitly refuses to pick one for you:

1. **No paging.** The call passes no `Top` and no `ContinuationToken`, then linearly scans `resp.Value`. With 56 pending approvals alone — and 04-03 having widened the filter to include every historical approved/rejected approval — the target is almost never on the first page.
2. **Invalid status filter.** `release.ApprovalStatus("pending,approved,rejected")` is a comma-joined string cast to a typed enum. If the endpoint rejects or ignores a comma list, the response is empty or unfiltered.

Establish which is real — it may be either or BOTH — by reading the SDK's `GetApprovalsArgs` and `ApprovalStatus` definitions (paging fields present? is the type a string alias the REST API documents as accepting a comma list?) and comparing against the call at `releases.go:243` that is known to work live with a single `ApprovalStatusValues.Pending` and returns 56 approvals in one unpaged call — evidence that weighs AGAINST the paging theory and toward the status-filter cast. Record your finding in a comment above the function.

Then fix accordingly — paging, a corrected status filter, or both:

- If paging is the cause: loop over pages using whichever mechanism `GetApprovalsArgs` actually exposes (`ContinuationToken` threaded from the response, or a `Top` plus continuation, following the `releases.go:144` local precedent for setting `Top`). Break on an empty `Value`, on a nil/absent continuation token, and on `ctx.Err()`. Bound the loop so a server that never stops handing back tokens cannot spin forever.
- If the status filter is the cause: use whatever the SDK/REST contract actually supports. If a single call cannot cover pending + approved + rejected, issue one call per status and merge, rather than narrowing back to `pending` only — narrowing would silently break the D-03 no-op case, which is exactly what the review forbids.

Preserve every existing property of this function: it returns an error (never `(nil, nil)`) for a missing approval, the error text names only the kind and the id with no URL/headers/body (D-15, T-04-14), and it logs through `logging.Logger()` on both the found and not-found paths. Do not touch `GetWorkItemDetail`.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test ./internal/ado/ -run 'TestGetReleaseApprovalDetail|TestPreRead' -v</automated>
  </verify>
  <acceptance_criteria>
    - A comment above `GetReleaseApprovalDetail` states which of the two candidate causes was found real — or that both were — and cites the SDK type that settled it.
    - `TestGetReleaseApprovalDetail` (existing) still passes, including its assertion that `StatusFilter` is not left at the endpoint's pending-only default.
    - `TestPreReadNotFoundReturnsError` and `TestPreReadNeverReturnsNilNilStructural` still pass.
    - If a paging loop was added, it has an explicit termination condition on empty page AND on absent continuation token AND a hard iteration bound.
  </acceptance_criteria>
  <done>`GetReleaseApprovalDetail` locates an approval regardless of its page position and regardless of whether it is pending, approved, or rejected.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Regression test pinning the paged / widened approval lookup</name>
  <files>internal/ado/preread_test.go</files>
  <read_first>
    - internal/ado/preread_test.go lines 88-116 (the existing `TestGetReleaseApprovalDetail`) and lines 148-200 (the not-found table)
    - internal/ado/fakes_test.go lines 203-234 (`fakeReleaseAPI`, func-field-per-method with nil-func `t.Fatalf`)
    - your Task 2 implementation, so the fake drives the mechanism you actually chose
  </read_first>
  <behavior>
    - Table row "target on a later page": the fake returns page 1 without the id (plus a continuation token if that is the mechanism), page 2 with it; the detail is returned.
    - Table row "already approved": an approval with status `approved` is returned, not filtered away — this is the D-03 no-op input.
    - Table row "absent from every page": the fake exhausts its pages; the call returns a nil detail AND a non-nil error.
    - The fake counts its invocations so the "later page" row fails loudly if the implementation regresses to a single unpaged call.
  </behavior>
  <action>
Add `TestGetReleaseApprovalDetailPaging` to `preread_test.go`, table-driven with Go's stdlib `testing` only — no testify, no mock generators, matching the file's existing style.

Drive `fakeReleaseAPI.GetApprovalsFunc` from a per-case slice of canned pages plus a call counter captured in the closure. Each case declares its pages, the id to look up, and whether an error is expected. Assert on the returned detail's `ID`/`Status` for the success rows, and on `err != nil` together with `got == nil` for the absent row. For the multi-page row, additionally assert the call counter is greater than one, so a regression back to a single unpaged `GetApprovals` fails the test rather than passing by luck.

Build `release.ReleaseApproval` values with addressable locals for the pointer fields, the way the existing test does at lines 89-104 — note the loop-variable aliasing trap if you construct pages inside a range.

If Task 2's fix turned out to be a per-status fan-out rather than paging, shape the fake to assert the set of `StatusFilter` values requested covers pending, approved, and rejected, and keep the "already approved is found" row unchanged. The behavioural assertions above are the contract; the mechanism assertion follows whatever Task 2 built.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test ./... &amp;&amp; go vet ./... &amp;&amp; gofmt -l .</automated>
  </verify>
  <acceptance_criteria>
    - `TestGetReleaseApprovalDetailPaging` passes, and fails if `GetReleaseApprovalDetail` is reverted to the pre-fix call at `preread.go:123` — whichever mechanism Task 2 identified (single unpaged call, comma-joined status cast, or both).
    - The absent-approval row asserts both `err != nil` and a nil detail, preserving the D-02 (nil, nil) prohibition behaviourally.
    - No acceptance criterion in this plan is a negative grep — every assertion is a Go test (STATE.md:150 records three incidents of grep-negative criteria defeated by a comment containing the forbidden token).
    - `gofmt -l .` prints nothing; `go vet ./...` is clean; the full `go test ./...` suite is green.
  </acceptance_criteria>
  <done>The WR-03 fix is pinned by a test that fails on regression, and the whole repo builds, vets, formats, and tests clean.</done>
</task>

</tasks>

<verification>
- `go build ./... && go test ./... && go vet ./... && gofmt -l .` all clean.
- Manual live confirmation (optional, requires a PAT): `adoBizInt releases approve <id> --project Kinetic` on an id listed by `adoBizInt releases approvals` no longer errors with "was not found" — it prints a dry-run diff.
</verification>

<success_criteria>
- CR-01: audit-append failure on an executed write yields exit 5 / `outcome:"unknown"`, proven by test.
- CR-01: successful appends are fsync'd.
- WR-03: `GetReleaseApprovalDetail` returns the approval for any id that `releases approvals` lists — any page position, any of pending/approved/rejected — proven by test, with the real cause recorded in a comment.
- No out-of-scope change: WR-01, WR-02, WR-07, WR-08 (beyond CR-01's own branch), WR-09, BL-01 part 2, and the `write.go` split are untouched.
</success_criteria>

<output>
Create `.planning/quick/260913-uhj-fix-cr-01-audit-append-failure-reported-/260913-uhj-SUMMARY.md` when done.
</output>
