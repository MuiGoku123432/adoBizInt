---
phase: 04-guarded-writes
verified: 2026-10-12T14:58:23Z
status: passed
score: 6/6 roadmap success criteria verified (plus 23/23 plan truths, 17/17 prohibitions)
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/04-guarded-writes/04-01-PLAN.md
  - .planning/phases/04-guarded-writes/04-01-SUMMARY.md
  - .planning/phases/04-guarded-writes/04-02-PLAN.md
  - .planning/phases/04-guarded-writes/04-02-SUMMARY.md
  - .planning/phases/04-guarded-writes/04-03-PLAN.md
  - .planning/phases/04-guarded-writes/04-03-SUMMARY.md
  - .planning/phases/04-guarded-writes/04-04-PLAN.md
  - .planning/phases/04-guarded-writes/04-04-SUMMARY.md
  - cmd/adoBizInt/audit.go
  - cmd/adoBizInt/audit_test.go
  - cmd/adoBizInt/dispatch.go
  - cmd/adoBizInt/exit.go
  - cmd/adoBizInt/fakes_test.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/usage.go
  - cmd/adoBizInt/write.go
  - cmd/adoBizInt/write_test.go
  - cmd/adoBizInt/writeverbs_test.go
  - internal/ado/interfaces.go
  - internal/ado/preread.go
  - internal/ado/preread_test.go
  - internal/ado/workitems.go
  - internal/ado/workitems_test.go
  - internal/ui/workitems/effort.go
  - internal/ui/workitems/workitems.go
covered_digest: "v1:sha256:4ea8ada980faa4bd0cd20ecb68f3af665a1018054052869286d52a1e53be8ea7"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 4: Guarded writes Verification Report

**Phase Goal:** An agent can mutate Azure DevOps without a mistake being silent or irreversible-by-surprise
**Verified:** 2026-10-12T14:58:23Z, against HEAD `ccba07b` on `dev`. That HEAD includes the later Phase 5 edits to the same files: the CR-01 project-ownership check in `setStateOp.plan` and the `--project` allow-list in `newFlagSet`.
**Status:** passed (with non-blocking warnings, listed below)
**Re-verification:** No. This is the first verification.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every write verb performs a dry run unless an explicit confirmation flag is passed | VERIFIED | One shared body handles every write: `runWrite` (write.go:137-277). It takes the `!wf.confirm` dry-run branch at :226 before `op.mutate` at :230. `--confirm` is registered only for verbs in `writeSupportByCommand` (flags.go:113-125, 255-259). `TestWriteGuardMatrix` iterates that map and fails on any verb that has no fixture. Its "dry run reports a field-level diff and mutates nothing" subtest passes for all 6 verbs, and in that subtest every mutating fake func is nil, so any mutating call would hit `t.Fatalf` (fakes_test.go:174-180). Live evidence: step 1 of the 2026-09-12 run on work item 1198503 (04-04-SUMMARY, STATE.md:192). |
| 2 | A dry run reports exactly what would change and makes no ADO call that mutates state | VERIFIED | A by-id pre-read drives the diff: `op.plan` at :207 runs before the no-op and confirm branches. A failed pre-read is fatal and puts nothing on stdout (D-02, the matrix subtest "failed pre-read is fatal and emits no report", 6/6 verbs). A missing target never returns `(nil, nil)` (`TestPreReadNeverReturnsNilNilStructural`, `TestPreReadNotFoundReturnsError`, plus the explicit nil check at write.go:309). The diff order is fixed (`diffWorkItem`, write.go:505-523). `changes` is always `[]`, never null. Live evidence: `System.State: New -> Active`, then a re-run that still showed `from: New`. One caveat is warning WR-02 below. |
| 3 | Every executed write appends an audit record identifying what changed | VERIFIED (scope: the CLI write surface) | `auditOpen()` at write.go:196 runs before `op.plan` (:207) and `op.mutate` (:230) (D-13). Executed and unknown outcomes are appended at :244-267. On HEAD, an append failure lowers the result to `unknown`/exit 5 instead of reporting success (CR-01 from the Phase 4 review, quick task 260913-uhj). `appendAuditRecord` fsyncs (audit.go:108-119). Tests: `TestExecutedWriteAppendsAuditRecord`, `TestOutcomeUnknownWriteIsAudited`, `TestAuditAppendFailureReportsUnknown`, `TestUnwritableAuditSinkPreventsMutation`, and the matrix guards "audits exactly once" and "unwritable audit sink prevents the mutation" for 6/6 verbs. Live: exactly 1 new line, five keys, no leak of argv, token or URL (steps 3-5). TUI writes are outside the Phase 4 boundary (04-CONTEXT "Phase Boundary"; D-17); see the scope note. |
| 4 | Omitting an effort field leaves it unchanged rather than zeroing it | VERIFIED | `UpdateWorkItemStateWithEffort` checks each pointer for nil before it `append`s that field's operation (workitems.go:321-347). `Op` stays `Add`, and the order is OE, RW, CW, State. On the CLI, `promoteEffort` uses `fs.Visit` to detect which effort flags were set (flags.go:297-311), so `--remaining 0` sends a real zero. TUI: `EffortSubmitMsg` uses `*float64` and `parseEffort("")` returns `(nil, nil)` (effort.go:27-29, 203-212); this is the quick task 260913-nd2 fix. Tests: `TestUpdateWorkItemStateWithEffortOmitsNilFields`, `TestUpdateWorkItemStateWithEffortIssuesExactPatch`, `TestExplicitZeroEffortIsDistinctFromOmitted`, `TestEffortModalSubmitNullability`, `TestUpdateWorkItemStateWithEffortCallSitesForwardPointers`. Live: the TUI modal re-test passed on 2026-09-13 (BACKLOG BL-01 part 1, STATE.md:191-192). |
| 5 | A write that times out reports outcome-unknown, distinct from both success and failure | VERIFIED | `writeExitCodeFor` (exit.go:62-97) checks the parent ctx first, then FailureTimeout. A missing HTTP status maps to unknown, and so does 5xx. This means a real-world timeout of any shape ends in exit 5: it either classifies as Timeout or carries no status. There is no crash record and no exit 3 (write.go:232-237). Tests: `TestWriteOutcomeClassification` (success, 403, 409, 503, deadline, no-status, plus pairwise-distinct exit codes and outcomes), `TestInterruptedWriteReportsOutcomeUnknownNotNothingRan`, and the matrix "deadline exceeded is outcome-unknown and audited" subtest (6/6). The published help lists exit 4 and exit 5. |
| 6 | All of the above are covered by tests using the Phase 1 seam | VERIFIED | Every test above runs in process against `fakeReadClient`/`fakeWriteClient` (cmd) or the `-API` fakes (internal/ado), with no network. Full gate is green: `go build`, `go vet`, `go test -race -count=1 ./...`, and `gofmt -l .` printed nothing. |

**Score:** 6/6 roadmap truths verified (0 behavior-unverified). Each behavior-dependent truth (D-13 ordering, timeout or interrupt leading to unknown, no-op refusal on both paths) has a named passing test that exercises it.

### Plan must_haves (merged)

| Plan | Truths | Prohibitions | Result |
|------|--------|--------------|--------|
| 04-01 | 10 | 6 | All hold on HEAD. `runWrite` calls neither `emptyIfNil` nor `writeCrashRecord`. `auditOpen(` (:196) comes before the mutate call (:230); the literal criterion name `openAuditSink(` is superseded by the seam var, and the behaviour is still pinned by a test. The envelope has a single top-level key, `write`. There is no TTY or stdin prompt anywhere in cmd/ (checked by grep for isatty, IsTerminal, os.Stdin and Scanln: 0 hits). |
| 04-02 | 4 | 5 | All hold. No `float64`+`omitempty` in code (the only hits are comments). Only `OperationValues.Add` is used. effort.go did change, but through the later quick task 260913-nd2 (an explicit new decision under BL-01), not inside 04-02. |
| 04-03 | 5 | 4 | All hold. Six verbs drive the seven mutations (dispatch.go:194-211), and `writeSupportByCommand` has the same keys (`TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`). `pipelines queue` without `--branch` exits 3 (matrix "missing branch" plus live step 6). |
| 04-04 | 3 | 2 | All hold. The matrix ranges over `writeSupportByCommand` and calls `t.Fatalf` on a missing fixture. The live-ADO truth was met on 2026-09-12 (steps 1-6 passed). |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/adoBizInt/write.go` | Guarded write body plus 6 verb ops | VERIFIED | 612 lines, substantive, called from 6 dispatch entries |
| `cmd/adoBizInt/audit.go` | JSONL sink, pre-flight open, fsync'd append | VERIFIED | Called by `runWrite` via `auditOpen` |
| `cmd/adoBizInt/exit.go` | Outcome vocabulary plus `writeExitCodeFor` | VERIFIED | Called at write.go:237 |
| `cmd/adoBizInt/flags.go` | `writeSupportByCommand`, write flags, `promoteEffort` | VERIFIED | Consumed by `newFlagSet` and `runWrite` |
| `internal/ado/preread.go` | 3 by-id pre-reads, never `(nil, nil)` | VERIFIED | Called from 4 op `plan` closures |
| `internal/ado/workitems.go` | Nullable effort patch builder | VERIFIED | Called by CLI set-state and both TUI call sites |
| `cmd/adoBizInt/write_test.go` | `TestWriteGuardMatrix` | VERIFIED | Passes, 6 verbs × 7-8 subtests |
| `04-VALIDATION.md` | Map complete | VERIFIED (doc) | No TBD rows. `status: draft` / `nyquist_compliant: false` remains until `/gsd-validate-phase 4` runs; that is a bookkeeping state, not a goal gap |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `commands["workitems set-state"]` | `runWrite` → `GetWorkItemDetail` → diff → `auditOpen` → `UpdateWorkItemState[WithEffort]` → `writeExitCodeFor` | dispatch.go:194, write.go:196/207/230/237 | WIRED |
| `commands[prs approve / prs complete / releases approve / releases reject / pipelines queue]` | each op's pre-read → mutate | dispatch.go:197-211, write.go:340-478 | WIRED |
| `--remaining` | `fs.Visit` → `*float64` → `UpdateWorkItemStateWithEffort` → conditional append | flags.go:297, write.go:152/331, workitems.go:329 | WIRED |
| `writeSupportByCommand` | the `commands` map and the guard matrix | write_test.go:436-466, 855-866 | WIRED (bidirectional drift check) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| write envelope `changes[].from` | current field values | by-id SDK pre-reads (`GetWorkItem`, `GetPullRequest`, `GetApprovals` paged, `GetDefinition`) | Yes. Confirmed live on 1198503 and approval 21611 | FLOWING |
| audit line | `writeRecord` | the same `changes` and `target` as the envelope | Yes. Live five-key record | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full gate | `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` | all ok, gofmt empty | PASS |
| Guard matrix | `go test -count=1 -v -run TestWriteGuardMatrix ./cmd/adoBizInt/` | 6 verbs, all subtests PASS | PASS |
| Phase 4 named tests (13 cmd, 7 ado, 3 ui) | `go test -run '...'` | all PASS | PASS |
| Help advertises the dry-run default and exit codes 4/5 | `adoBizInt workitems set-state --help` (no ADO call) | "previews by default ... Exits 4 ... 5"; exit 0 | PASS |

### Probe Execution

Step 7c: SKIPPED. No `scripts/*/tests/probe-*.sh` exist, and no Phase 4 plan declares a probe.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CLI-03 | 04-01, 04-03 | Write subcommands exist for the seven existing mutations | SATISFIED | 6 verbs drive 7 `ado` mutations (set-state covers both state methods) |
| SAFE-01 | 04-01, 04-03, 04-04 | Dry-run by default, explicit confirmation to execute | SATISFIED | SC1 |
| SAFE-02 | 04-01, 04-03, 04-04 | Every executed write appends an audit record | SATISFIED (CLI surface) | SC3. CR-01 closed |
| SAFE-03 | 04-02 | Omitted effort fields are left unchanged | SATISFIED (CLI and TUI) | SC4 |
| SAFE-04 | 04-01, 04-04 | A timeout reports outcome-unknown | SATISFIED in code. Still unchecked in REQUIREMENTS.md | SC5. **Bookkeeping:** REQUIREMENTS.md:53 is still `[ ]` and :244 still reads `Pending`. Both should be ticked when the phase is marked complete |

All 5 IDs from ROADMAP and the plan frontmatter are accounted for. No orphaned requirement maps to Phase 4.

### Scope note: TUI write path (BL-01 part 2)

TUI-initiated writes still skip dry-run, `--confirm` and the audit record. This is a known item, deliberately deferred and tracked in `.planning/BACKLOG.md` BL-01 part 2. The 04-CONTEXT Phase Boundary limits the guards to the CLI write subcommands, and the only TUI change in scope was the nullable-effort call sites (D-17). The phase goal is also worded around *an agent* mutating ADO, and agents go through the CLI. I therefore read SC1 and SC3 as applying to the CLI write surface and did not count this as a Phase 4 gap. If you want that reading on record, add an `overrides:` entry for SC3.

### Anti-Patterns Found

No TBD, FIXME or XXX markers and no TODO, HACK or PLACEHOLDER markers in any file this phase touched. Several 04-REVIEW findings are **still open on HEAD and not tracked in BACKLOG.md**. Only CR-01 and WR-03 (quick task 260913-uhj) and WR-08 (covered by CR-01's test) were closed, and WR-04 is mitigated in practice. None of the open ones lets a mutation happen silently. Each fails loudly or toward safety, so they are warnings, not blockers:

| File | Line | Finding | Severity | Impact |
|------|------|---------|----------|--------|
| internal/ado/preread.go | 97-104 | WR-02: `prs approve` no-op guard turns itself off when `currentUserID` is empty (identity resolution is non-fatal) | Warning | In that state the dry run reports `reviewer.vote: 0 -> 10` even for a PR that is already approved, and `--confirm` re-casts the vote and writes an audit "change". This weakens SC2's "exactly" in a degraded state. No test covers it |
| cmd/adoBizInt/usage.go | 179-185 | WR-05: help-token scan matches flag *values* | Warning | `releases approve 77 --project P --comment -h --confirm` prints usage, exits 0, writes nothing, and leaves stdout empty. It fails safe (no write), but exit 0 for a `--confirm` invocation is misleading |
| cmd/adoBizInt/write.go | 561-564 | WR-06: `finishWrite` returns 3 ("nothing ran") when stdout emit fails after a mutation | Warning | Reachable through EPIPE (`\| head`). The audit record is already on disk, but the exit code contradicts D-22 |
| cmd/adoBizInt/write.go | 597-612 | WR-01: `hoistLeadingPositionals` has no tests, and the doc claims the flag-first spelling works when it does not | Warning | Mis-parses currently fail as exit 3. That safety is a side effect, and no test asserts it |
| cmd/adoBizInt/audit_test.go | — | WR-07: credential-hygiene test checks key names on one verb only | Info | Test strength only |
| cmd/adoBizInt/{audit,write}_test.go | — | WR-09: D-13 permission-based tests do not skip as root | Info | They pass here (non-root). The CR-01 test already avoids permission-based setup |
| cmd/adoBizInt/audit.go | 77-80 | WR-04: audit file created 0644 in MkdirAll(0755) | Info | Mitigated: `logging.Init` forces `~/.config/adoBizInt` to 0700 on every run (logging.go:26-31). The comment at audit.go:15-17 claiming HARD-06 covers this file too is inaccurate |
| cmd/adoBizInt/write.go | — | 612 lines, over the 300-line convention | Info | Known, deliberate trade-off (04-04-SUMMARY) |

### Human Verification Required

None outstanding for Phase 4. The live evidence is already on record:
- CLI steps 1-6 passed on 2026-09-12 against work item 1198503 (04-04-SUMMARY, STATE.md:192).
- The TUI effort-modal step failed on 2026-09-12, was fixed by 260913-nd2, and passed on re-test on 2026-09-13.
- `releases approve`/`reject` were confirmed by a live A/B test on approval 21611 (260913-uhj-VERIFICATION.md:61).

Cross-reference only, not re-requested: a live `set-state` run with the project in the PATCH route has not been done since Phase 5 changed the set-state route (CR-01). It is owned by 05-UAT item 4, which the user explicitly deferred to the v2 smoke test. Its failure modes are loud: a case-mismatched project gives exit 3, and a bad route gives a 4xx, which is `failed`/exit 1. Neither threatens a Phase 4 guarantee.

### Gaps Summary

No gaps. The six roadmap success criteria hold on HEAD after the Phase 5 edits, each backed by a named passing test, and the live checks recorded for 2026-09-12 and 2026-09-13 confirm the CLI and TUI effort behaviour. Recommended follow-ups (non-blocking):
1. Tick SAFE-04 in REQUIREMENTS.md (:53 and :244).
2. Capture the open 04-REVIEW warnings WR-01, WR-02, WR-05 and WR-06 in BACKLOG.md so they are not lost. WR-02 and WR-06 bear most directly on the goal's "never silent" promise.

---

_Verified: 2026-10-12T14:58:23Z_
_Verifier: Claude (gsd-verifier)_
