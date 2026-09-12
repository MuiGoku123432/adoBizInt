---
phase: 04
slug: guarded-writes
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: true
created: 2026-09-12
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `04-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`, table-driven, no testify, no mock generator |
| **Config file** | none (stdlib) |
| **Quick run command** | `go test ./internal/ado/... ./cmd/adoBizInt/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |
| **Assertion style** | `reflect.DeepEqual(got, want)` against struct literals |
| **Fake style** | func-field-per-method; nil field → `t.Fatalf("unexpected call to …")` |
| **Injection** | struct literal on unexported `Client` fields, in-package |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/ado/... ./cmd/adoBizInt/...`
- **After every plan wave:** Run `go test ./... && go vet ./... && gofmt -l .`
- **Before `/gsd-verify-work`:** `go test -race ./...` must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Task IDs are assigned by the planner. Rows below are the SC-derived verification
> contract each task must map onto; the planner fills the Task ID / Plan / Wave columns.

| Task ID | Plan | Wave | Requirement | SC | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|----|-----------------|-----------|-------------------|-------------|--------|
| 04-01-T3 | 01 | 1 | CLI-03 / SAFE-01 | SC1 | `--confirm` registered on every write verb and on no read verb (`TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`) | unit (structural) | `go test ./cmd/adoBizInt/... -run TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb` | ✅ `cmd/adoBizInt/write_test.go:392` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-01 | SC1 | Without `--confirm`, no mutating call — mutating fake func left nil (`TestDryRunIssuesNoMutatingCall`) | unit | `go test ./cmd/adoBizInt/... -run TestDryRunIssuesNoMutatingCall` | ✅ `cmd/adoBizInt/write_test.go:95` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-01 | SC2 | Dry-run envelope reports field-level `{field, from, to}` diff (`TestDryRunEnvelopeReportsFieldLevelDiff`) | unit | `go test ./cmd/adoBizInt/... -run TestDryRunEnvelopeReportsFieldLevelDiff` | ✅ `cmd/adoBizInt/write_test.go:107` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-01 | SC2 | Failed pre-read emits no dry-run report (D-02) (`TestFailedPreReadEmitsNoDryRunReport`) | unit | `go test ./cmd/adoBizInt/... -run TestFailedPreReadEmitsNoDryRunReport` | ✅ `cmd/adoBizInt/write_test.go:139` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-01 | SC2 | An already-applied change is refused with exit 4 on BOTH the dry-run and `--confirm` paths (D-03) (`TestRefusedNoOpExitsNoOpCodeOnBothPaths`) | unit | `go test ./cmd/adoBizInt/... -run TestRefusedNoOpExitsNoOpCodeOnBothPaths` | ✅ `cmd/adoBizInt/write_test.go:177` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-02 | SC3 | Executed write appends exactly one audit record with the D-15 field set (`TestExecutedWriteAppendsAuditRecord`) | unit | `go test ./cmd/adoBizInt/... -run TestExecutedWriteAppendsAuditRecord` | ✅ `cmd/adoBizInt/audit_test.go:61` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-02 | SC3 | Audit record omits `argv` / `actor` / credential fields (`TestAuditRecordOmitsArgvAndCredentialFields`) | unit | `go test ./cmd/adoBizInt/... -run TestAuditRecordOmitsArgvAndCredentialFields` | ✅ `cmd/adoBizInt/audit_test.go:102` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-02 | SC3 | Unwritable audit sink prevents the mutation (D-13 ordering) (`TestUnwritableAuditSinkPreventsMutation`) | unit | `go test ./cmd/adoBizInt/... -run TestUnwritableAuditSinkPreventsMutation` | ✅ `cmd/adoBizInt/audit_test.go:135` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-02 | SC3 | Dry run and refused no-op append no audit record (D-14) (`TestDryRunAndNoOpAppendNoAuditRecord`) | unit | `go test ./cmd/adoBizInt/... -run TestDryRunAndNoOpAppendNoAuditRecord` | ✅ `cmd/adoBizInt/audit_test.go:172` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-02 / SAFE-04 | SC3 | Outcome-unknown write IS audited (D-16) (`TestOutcomeUnknownWriteIsAudited`) | unit | `go test ./cmd/adoBizInt/... -run TestOutcomeUnknownWriteIsAudited` | ✅ `cmd/adoBizInt/audit_test.go:197` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-04 | SC5 | success / failed / unknown produce three distinct exit codes (`TestWriteOutcomeClassification`) | unit | `go test ./cmd/adoBizInt/... -run TestWriteOutcomeClassification` | ✅ `cmd/adoBizInt/write_test.go:229` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-04 | SC5 | Mid-write SIGINT → unknown, not exit 3 (D-22) (`TestInterruptedWriteReportsOutcomeUnknownNotNothingRan`) | unit | `go test ./cmd/adoBizInt/... -run TestInterruptedWriteReportsOutcomeUnknownNotNothingRan` | ✅ `cmd/adoBizInt/write_test.go:287` | ✅ green |
| 04-01-T3 | 01 | 1 | SAFE-04 | SC5 | All exit code constants pairwise distinct (`TestExitCodesArePairwiseDistinct`) | unit (structural) | `go test ./cmd/adoBizInt/... -run TestExitCodesArePairwiseDistinct` | ✅ `cmd/adoBizInt/write_test.go:328` | ✅ green |
| 04-01-T3 | 01 | 1 | CLI-03 | SC1 | Usage errors — missing/duplicate target, zero or two `--project`, missing `--state` — exit 3 with no client call (`TestWriteParseErrorsExitUnknown`) | unit | `go test ./cmd/adoBizInt/... -run TestWriteParseErrorsExitUnknown` | ✅ `cmd/adoBizInt/write_test.go:356` | ✅ green |
| 04-02-T1 | 02 | 2 | SAFE-03 | SC4 | Nil effort pointers produce no patch op (`TestUpdateWorkItemStateWithEffortOmitsNilFields`) | unit | `go test ./internal/ado/... -run TestUpdateWorkItemStateWithEffortOmitsNilFields` | ✅ `internal/ado/workitems_test.go:400` | ✅ green |
| 04-02-T3 | 02 | 2 | SAFE-03 | SC4 | `--remaining 0` is a real zero, distinct from omitted (D-18) (`TestExplicitZeroEffortIsDistinctFromOmitted`) | unit | `go test ./cmd/adoBizInt/... -run TestExplicitZeroEffortIsDistinctFromOmitted` | ✅ `cmd/adoBizInt/write_test.go:448` | ✅ green |
| 04-02-T1 | 02 | 2 | SAFE-03 | SC4 | TUI four-operation patch byte-identical after signature change (D-17 regression guard) (`TestUpdateWorkItemStateWithEffortIssuesExactPatch`) | unit | `go test ./internal/ado/... -run TestUpdateWorkItemStateWithEffortIssuesExactPatch` | ✅ `internal/ado/workitems_test.go:341` | ✅ green |
| 04-02-T3 | 02 | 2 | SAFE-03 | SC4 | Effort flags registered on exactly the verbs whose `writeSupport.effort` is true, in both directions (`TestEffortFlagsNotRegisteredOnNonEffortVerbs`) | unit (structural) | `go test ./cmd/adoBizInt/... -run TestEffortFlagsNotRegisteredOnNonEffortVerbs` | ✅ `cmd/adoBizInt/write_test.go:559` | ✅ green |
| 04-03-T3 | 03 | 3 | CLI-03 | SC1 | Every write verb appears in its noun-level `--help`, iterated from `writeSupportByCommand` (`TestEveryWriteVerbAppearsInNounUsage`) | unit (structural) | `go test ./cmd/adoBizInt/... -run TestEveryWriteVerbAppearsInNounUsage` | ✅ `cmd/adoBizInt/usage_test.go:174` | ✅ green |
| 04-04-T1 | 04 | 4 | SAFE-01 / SAFE-02 / SAFE-04 | SC1–SC5 | All five guards asserted for every verb, iterated from `writeSupportByCommand`; a registered verb with no fixture fails with `t.Fatalf` (`TestWriteGuardMatrix`) | unit (structural) | `go test ./cmd/adoBizInt/... -run TestWriteGuardMatrix -v` | ✅ `cmd/adoBizInt/write_test.go:785` | ✅ green |
| 04-04-T1 | 04 | 4 | SAFE-01 | SC6 | Every write test injects the Phase 1 seam; no test opens a socket | unit (structural) | `grep -rn "http\." --include=*_test.go internal/ado cmd \| grep -v ':[0-9]*:[[:space:]]*//'` returns nothing | ✅ by construction — every write guard drives `fakeWriteClient` through `TestWriteGuardMatrix`, which has no transport | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `cmd/adoBizInt/write_test.go` — SC1, SC2, SC5 envelope + exit-code tests, plus the phase-wide `TestWriteGuardMatrix`
- [x] `cmd/adoBizInt/audit_test.go` — SC3, including the D-13 ordering test
- [x] `--confirm` registration + `fs.Visit` presence tests — landed in `cmd/adoBizInt/write_test.go` rather than a new `flags_test.go`, beside the write tests they constrain
- [x] `internal/ado/workitems_test.go` — `TestUpdateWorkItemStateWithEffortIssuesExactPatch` updated for the pointer signature; `TestUpdateWorkItemStateWithEffortOmitsNilFields` added
- [x] `internal/ado/fakes_test.go` — no new by-id interface entry was required (04-03 filtered `GetApprovals` rather than adding one), so no fake method was added
- [x] Shared helper for a temp audit sink — `tempAuditSink` in `cmd/adoBizInt/write_test.go:29`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions | Status |
|----------|-------------|------------|-------------------|--------|
| Real ADO write against a live project | SAFE-01..04 | Requires live credentials and a disposable work item; out of scope for CI | Against a scratch work item: run the verb without `--confirm` (expect diff, no change), then with `--confirm` (expect change + audit line). Confirm the audit file contains exactly one new record. Full seven-step procedure preserved verbatim in `04-04-SUMMARY.md` § "Deferred — Live Azure DevOps verification (UNVERIFIED)". | ⬜ pending |
| TUI effort modal closes a real Task with typed effort (carried from 04-02) | SAFE-03 | No automated test drives the Bubble Tea effort modal end to end | Close one real Task through the TUI effort modal, entering effort values. Confirm the Task closes with the effort typed and that fields left blank were not zeroed. | ⬜ pending |

> **Deferred at plan 04-04 (2026-09-12).** Both rows above were deferred by the user at the
> 04-04 Task 3 checkpoint and have NOT been run. No result may be inferred for them. Phase 4
> is not fully verified against a live Azure DevOps instance until they are. This is why
> `status` remains `draft` and `nyquist_compliant` remains `false`.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
