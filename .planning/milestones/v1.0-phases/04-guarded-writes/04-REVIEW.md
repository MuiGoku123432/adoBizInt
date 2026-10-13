---
phase: 04-guarded-writes
reviewed: 2026-09-13T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - cmd/adoBizInt/audit.go
  - cmd/adoBizInt/audit_test.go
  - cmd/adoBizInt/dispatch.go
  - cmd/adoBizInt/exit.go
  - cmd/adoBizInt/fakes_test.go
  - cmd/adoBizInt/filter_test.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/main_test.go
  - cmd/adoBizInt/usage.go
  - cmd/adoBizInt/usage_test.go
  - cmd/adoBizInt/write.go
  - cmd/adoBizInt/write_test.go
  - cmd/adoBizInt/writeverbs_test.go
  - internal/ado/json_test.go
  - internal/ado/preread.go
  - internal/ado/preread_test.go
  - internal/ado/result.go
  - internal/ado/workitems.go
  - internal/ado/workitems_test.go
  - internal/ui/workitems/effort.go
  - internal/ui/workitems/effort_test.go
  - internal/ui/workitems/workitems.go
  - internal/ui/workitems/workitems_test.go
findings:
  critical: 1
  warning: 9
  info: 0
  total: 10
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-09-13
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

`go build`, `go vet`, and `go test ./...` are all clean. No `AGENTS.md` exists in the repo,
so review was held to the conventions stated in the source comments themselves.

The four invariants I was asked to attack hardest hold:

- **D-13 ordering holds for all six verbs.** `openAuditSink` is `write.go:196`; `op.plan` is
  `:207`; `op.mutate` is `:230`. Because `runWrite` is one shared body, there is no per-verb
  copy that could have drifted, and `TestWriteGuardMatrix` guard 5 re-proves it per verb with
  an observable mutating closure. This is the strongest part of the phase.
- **The effort nil guard is on the `append`, not the value.** `workitems.go:311-333` gates each
  `document = append(...)` on `ptr != nil` and dereferences (`Value: *originalEstimate`) inside
  the branch. A nil `*float64` cannot reach `JsonPatchOperation.Value`, so `value:null` cannot
  be emitted. The TUI path (`effort.go:200-212`) now propagates `(nil, nil)` for blank input.
- **`ClassifyFailure` does not conflate cancel with timeout.** `result.go:145-155` checks
  `context.Canceled` first and falls through to `FailureOther`; `writeExitCodeFor` consults the
  parent `ctx.Err()` before the kind. 5xx and status-less transport failures both route to
  outcome-unknown (`exit.go:81-91`).
- **Registry/dispatch drift is closed.** `TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`
  plus `TestWriteGuardMatrix`'s bidirectional fixture/`writeSupportByCommand` check make a new
  verb in one map without the other a test failure.
- **The new pre-reads do not repeat `GetWorkItemDetail`'s `(nil, nil)`.** All three return an
  error for a missing target, and `TestPreReadNeverReturnsNilNilStructural` enforces it via an
  AST walk rather than a grep — correctly immune to the comment-defeats-grep incident class.

What is wrong is concentrated in what happens *after* the guard fires, in two pre-read
correctness gaps, and in test coverage for the argument hoister.

Per instruction, the four documented trade-offs (file length, exit-1 overload, empty
`audit.jsonl` on dry run, TUI bypass) are not reported as findings.

## Critical Issues

### CR-01: An audit append failure after an executed write is swallowed and the command exits 0

**File:** `cmd/adoBizInt/write.go:244-261`
**Issue:**
```go
if auditErr := appendAuditRecord(sink, record); auditErr != nil {
    log.Error("Failed to append audit record", ...)
    fmt.Fprintf(stderrWriter, "Failed to append audit record: %v\n", auditErr)
}
// falls through
return finishWrite(name, target, changes, outcome == outcomeExecuted, outcome, code, gf.asJSON)
```
The append error is logged to stderr and then discarded. `code` is still `exitOK`, and the
stdout envelope still carries `"executed": true, "outcome": "executed"`. The result is a
mutation that landed in ADO with **no audit record**, reported to the caller as a complete
success on the parseable channel.

This contradicts the phase guarantee "append exactly one JSONL audit record per executed or
outcome-unknown write", and it directly contradicts this repo's own stated contract in
`audit.go:90-92`: *"Unlike writeCrashRecord, a marshal or write failure here is returned rather
than swallowed, because D-13 makes this sink fatal."* `appendAuditRecord` does return the error;
`runWrite` is the one that makes it non-fatal.

`openAuditSink` proving the file *openable* does not prove it *writable* — a full filesystem,
a quota breach, an `EIO`, or a concurrently-revoked mount all fail at `f.Write`, which is
exactly the window D-13 exists to close. And `f.Write` is never followed by `Sync`, so a
short/partial write is also possible in principle.

An agent branching on exit code cannot distinguish "written and audited" from "written and
unrecorded". Given the phase's entire subject is that a write's disposition is never
misreported, this is the phase's own core failure mode.

**Fix:** make the append failure change the reported outcome and exit code, and flush.
```go
// audit.go
func appendAuditRecord(f *os.File, r auditRecord) error {
    payload, err := json.Marshal(r)
    if err != nil {
        return err
    }
    payload = append(payload, '\n')
    if _, err := f.Write(payload); err != nil {
        return err
    }
    return f.Sync()
}

// write.go, replacing the swallow
if auditErr := appendAuditRecord(sink, record); auditErr != nil {
    log.Error("Failed to append audit record", "command", name, "error", auditErr)
    fmt.Fprintf(stderrWriter, "Write may have landed but was NOT recorded in %s: %v\n",
        auditPath, auditErr)
    // The mutation's disposition is no longer establishable from the trail.
    outcome, code = outcomeUnknown, exitOutcomeUnknown
}
```
Add a test (see WR-08) that forces the write to fail — e.g. close `sink` before the mutate
returns, or inject the sink through a seam — and asserts exit 5.

## Warnings

### WR-01: `hoistLeadingPositionals` is load-bearing for every write verb and has zero tests

**File:** `cmd/adoBizInt/write.go:571-586`
**Issue:** This function rewrites the argv of all six mutating verbs and there is no test for it
anywhere (`grep -in hoist cmd/adoBizInt/*_test.go` returns nothing). `TestWriteGuardMatrix`'s
fixtures all use positional-first form (`{"prs","approve","4821","--project","P",...}`), so only
the branch where `firstFlag > 0` is exercised, and only with well-formed input.

Concrete mis-parses reachable today:

- **Flag-then-positional does not work, despite the doc claiming both spellings parse
  identically.** `prs approve --project P 4821 --repo R` has `firstFlag == 0`, so nothing is
  hoisted; stdlib `flag` stops at `4821`, `fs.Args()` is `["4821","--repo","R"]`, and the arity
  check at `write.go:163` rejects it with "requires exactly one id". The doc comment at
  `:557-558` asserts the two orderings are equivalent. They are not.
- **`--` is classified as a flag.** `len("--") > 1 && a[0] == '-'` is true, so `--` terminates
  the hoist scan and is then consumed by `flag` as the argument terminator, dumping every
  subsequent flag into `fs.Args()`.

Both currently fail loudly (exit 3, no mutation), which is why this is a WARNING and not a
BLOCKER — but that safety is incidental, not asserted. There is no test pinning "a dropped
`--confirm` can never coexist with a passing arity check", and that property is the only thing
standing between this parser and an unconfirmed mutation.

**Fix:** add a table-driven test over `hoistLeadingPositionals` covering: leading positional,
trailing positional, flag-value that looks like a flag (`--comment -x`), `--` terminator,
repeated `--project`, negative float value (`--remaining -5`), and empty input. Then either
document flag-then-positional as unsupported in the comment, or handle it by scanning for the
first token that is not a flag and not a flag's value.

### WR-02: `prs approve`'s no-op guard silently disables itself when the identity is unresolved

**File:** `internal/ado/preread.go:97-104`, used by `cmd/adoBizInt/write.go:327`
**Issue:**
```go
if pr.Reviewers != nil && c.currentUserID != "" {
```
When `currentUserID` is empty, `detail.MyVote` stays at its zero value. `prsApproveOp.plan`
then sees `0 != voteApproved` and always emits a change, so an **already-approved** PR is not
refused as a no-op — the vote is re-cast and an audit record claiming a real change is written.

The comment at `:95-96` correctly notes that Phase 1 D-12 makes identity resolution non-fatal,
so an empty `currentUserID` is a supported production state. The guarantee "refuse an
already-applied change as a no-op (exit 4)" is therefore silently unenforceable for this verb,
with no signal to the caller. `TestGetPullRequestDetailIgnoresOtherReviewersVotes` covers the
wrong-reviewer case but not the empty-identity case.

**Fix:** make the unresolvable state explicit rather than indistinguishable from "no vote".
Add `MyVoteKnown bool` (or make `MyVote *int`) to `PullRequestDetail`, set it only when
`currentUserID != ""`, and have `prsApproveOp.plan` return an error when it is false — a guard
that cannot run must fail the pre-read under D-02, not quietly pass.

### WR-03: `GetReleaseApprovalDetail` does not paginate, so an approval past page 1 reads as "not found"

**File:** `internal/ado/preread.go:118-153`
**Issue:** `c.releaseClient.GetApprovals` returns a paged response; only `resp.Value` is scanned
and `resp.ContinuationToken` is ignored. In a project with more pending/approved/rejected
approvals than one page, a perfectly valid `--project P 77` returns
`fmt.Errorf("release approval %d was not found", 77)`, which D-02 makes fatal — `releases
approve` and `releases reject` become unusable against that project with a message that
actively misinforms ("was not found" when it exists).

This is aggravated by the widened `statusFilter` at `:121`, which deliberately enlarges the
result set — increasing the chance of crossing a page boundary — without a corresponding
pagination loop. `TestGetReleaseApprovalDetail` uses a single-page fake, so nothing covers this.

**Fix:** loop on the continuation token until the id is found or the pages are exhausted, and
add a fake returning two pages with the target on the second.
```go
var token string
for {
    resp, err := c.releaseClient.GetApprovals(ctx, release.GetApprovalsArgs{
        Project: &project, StatusFilter: &statusFilter, ContinuationToken: &token,
    })
    // ... scan resp.Value; if found return
    if resp == nil || resp.ContinuationToken == "" { break }
    token = resp.ContinuationToken
}
```

### WR-04: The audit trail is created world-readable (0644) in a world-executable directory (0755)

**File:** `cmd/adoBizInt/audit.go:77-80`
**Issue:** `os.MkdirAll(..., 0755)` and `os.OpenFile(..., 0644)`. Every local user can read the
full mutation history — project names, work item ids, states, effort values, PR ids. The comment
at `:15-17` defers this to "Phase 5 HARD-06's 0600/0700 permission work", but the permission
mode is an argument on these two lines and costs nothing to get right now; deferring it means
records written between now and Phase 5 are created with the loose mode permanently, since D-12
makes the file append-only and it will not be recreated.

**Fix:** `os.MkdirAll(filepath.Dir(auditPath), 0700)` and `os.OpenFile(auditPath, ..., 0600)`.
Add an assertion on `fi.Mode().Perm()` in `TestExecutedWriteAppendsAuditRecord`.

### WR-05: `handleUsage` scans flag *values* for help tokens, so `--comment -h` aborts a write with exit 0

**File:** `cmd/adoBizInt/usage.go:177-186`
**Issue:** The loop walks `args[2:]` unconditionally and matches any token in `helpTokens`. It
cannot distinguish a flag name from a flag value, so
`adoBizInt releases approve 77 --project P --comment "-h" --confirm` prints usage to stderr and
returns `exitOK` with no mutation and no envelope on stdout. A caller that passed `--confirm`
receives exit 0 — the code that means "the command completed and nothing failed" — for a write
that never happened, with nothing on the parseable channel to say so.

This was pre-existing on the read path where it was harmless; extending `newFlagSet` to write
verbs makes it reachable for mutations. `--comment` is free text, so `-h` is plausible content.

**Fix:** stop the scan at the first `--` and skip the value token following any registered
non-boolean flag, or move help detection into the per-verb `FlagSet` (which already returns
`flag.ErrHelp` and is handled at `write.go:143`) and delete the ad-hoc scan for verbs that have
a FlagSet.

### WR-06: `finishWrite` returns exit 3 ("nothing ran") when encoding fails after a completed mutation

**File:** `cmd/adoBizInt/write.go:531-540`
**Issue:**
```go
if err := emitJSON(writeEnvelope{Write: record}); err != nil {
    fmt.Fprintf(stderrWriter, "Error encoding write report: %v\n", err)
    return exitUnknown   // == 3
}
```
`finishWrite` is shared by the dry-run and post-mutation paths. On the post-mutation path this
discards the real `code` and returns 3, which `dispatch.go:25` and the published help text at
`usage.go:89` both define as *"nothing ran"*. That is precisely the assertion `exit.go:56-61`
argues must never be made about a write that was issued.

`json.Marshal` of this struct cannot fail, but `emitJSON` also writes to stdout, so `EPIPE`
against a closed pipe (`... | head`) reaches this branch. The audit record is already on disk
by then, so the truth survives — hence WARNING rather than BLOCKER.

**Fix:** preserve the write's own code, escalating only when the outcome was genuinely
established and can no longer be reported.
```go
if err := emitJSON(writeEnvelope{Write: record}); err != nil {
    fmt.Fprintf(stderrWriter, "Error encoding write report: %v\n", err)
    if executed || outcome == outcomeUnknown {
        return exitOutcomeUnknown // the write ran; its report did not reach the caller
    }
    return exitUnknown
}
```

### WR-07: The credential-hygiene test asserts key names on a fixture that never carried a credential

**File:** `cmd/adoBizInt/audit_test.go:102-129`
**Issue:** `TestAuditRecordOmitsArgvAndCredentialFields` runs one `workitems set-state` with
fake inputs (`"Active"`, no comment, no secret anywhere in scope) and then substring-scans the
line for `"argv"`, `"token"`, `"pat"`, etc. as **JSON key names**.

`auditRecord` is a closed five-field struct whose only free-text content is `Command` (a
compile-time literal) and `changes[].from/.to`. The test therefore cannot fail for the case it
is named after: a PAT or a URL reaching the file as a *value* — e.g. a state string, a comment,
or an error message flattened into a change — carries no key called `"token"` and passes
cleanly. It is a weaker sibling of the AST test in `preread_test.go`, which the same repo
history says is the standard here.

It also only ever exercises one of six verbs, so `releases approve --comment <secret>` — the one
verb with caller-controlled free text on a write — is not covered at all.

**Fix:** (a) run the assertion across the whole guard matrix, not one verb; (b) add a positive
case that plants a sentinel secret in every caller-controlled input (`--comment`, `--state`,
`--branch`, `--repo`) and asserts the sentinel does not appear anywhere in the persisted line;
(c) consider an AST/reflect assertion that `auditRecord`'s field set is exactly the five keys
D-15 names, so a sixth field is a test failure rather than a silent widening.

### WR-08: No test covers the audit-append failure branch

**File:** `cmd/adoBizInt/write.go:248-252`, `cmd/adoBizInt/audit_test.go`
**Issue:** Every audit test drives `openAuditSink` success or `openAuditSink` failure. Nothing
drives *open succeeds, write fails*, which is the branch CR-01 is about. Because that branch is
untested, CR-01's swallow was free to be introduced and will be free to survive a fix
regression. This is the coverage gap that made the BLOCKER possible.

**Fix:** introduce a seam (e.g. `appendAuditRecord(w io.Writer, r auditRecord)`, or a
package-level `openAuditSinkFn` var) so a test can supply a writer that returns an error, and
assert exit 5 plus the stderr warning.

### WR-09: The D-13 guard tests pass vacuously when run as root

**File:** `cmd/adoBizInt/audit_test.go:135-145`, `cmd/adoBizInt/write_test.go:910-940`
**Issue:** Both rely on `os.MkdirAll(dir, 0500)` making the directory unwritable. Root ignores
the mode, so `openAuditSink` succeeds, the mutation is issued, and the tests fail rather than
skip — or, if the assertion is ever loosened, pass silently. Given these two tests are the *only*
executable proof of the phase's single most load-bearing ordering invariant, they should refuse
to run in an environment where they cannot mean anything (CI containers commonly run as root).

**Fix:** guard both with an explicit check:
```go
if os.Geteuid() == 0 {
    t.Skip("root bypasses directory permissions; this test cannot prove D-13 as root")
}
```
A skip is honest; a silent pass is the failure mode this whole phase exists to prevent.

---

_Reviewed: 2026-09-13_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
