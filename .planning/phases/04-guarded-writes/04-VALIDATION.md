---
phase: 04
slug: guarded-writes
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| TBD | 01 | 1 | CLI-03 / SAFE-01 | SC1 | `--confirm` registered on every write verb and on no read verb (`TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | SAFE-01 | SC1 | Without `--confirm`, no mutating call — mutating fake func left nil (`TestDryRunIssuesNoMutatingCall`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | SAFE-02 | SC2 | Dry-run envelope reports field-level `{field, from, to}` diff (`TestDryRunEnvelopeReportsFieldLevelDiff`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | SAFE-02 | SC2 | Failed pre-read emits no dry-run report (D-02) (`TestFailedPreReadEmitsNoDryRunReport`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-02 | SC3 | Executed write appends exactly one audit record with the D-15 field set (`TestExecutedWriteAppendsAuditRecord`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-02 | SC3 | Audit record omits `argv` / `actor` / credential fields (`TestAuditRecordOmitsArgvAndCredentialFields`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-02 | SC3 | Unwritable audit sink prevents the mutation (D-13 ordering) (`TestUnwritableAuditSinkPreventsMutation`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-02 | SC3 | Dry run and refused no-op append no audit record (D-14) (`TestDryRunAndNoOpAppendNoAuditRecord`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-02 / SAFE-04 | SC3 | Outcome-unknown write IS audited (D-16) (`TestOutcomeUnknownWriteIsAudited`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 04 | 2 | SAFE-03 | SC4 | Nil effort pointers produce no patch op (`TestUpdateWorkItemStateWithEffortOmitsNilFields`) | unit | `go test ./internal/ado/...` | ❌ W0 | ⬜ pending |
| TBD | 04 | 2 | SAFE-03 | SC4 | `--remaining 0` is a real zero, distinct from omitted (D-18) (`TestExplicitZeroEffortIsDistinctFromOmitted`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 04 | 2 | SAFE-03 | SC4 | TUI four-operation patch byte-identical after signature change (D-17 regression guard) | unit | `go test ./internal/ado/...` | ✅ `internal/ado/workitems_test.go:341` | ⬜ pending |
| TBD | 03 | 2 | SAFE-04 | SC5 | success / failed / unknown produce three distinct exit codes (`TestWriteOutcomeClassification`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-04 | SC5 | Mid-write SIGINT → unknown, not exit 3 (D-22) (`TestInterruptedWriteReportsOutcomeUnknownNotNothingRan`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 03 | 2 | SAFE-04 | SC5 | All exit code constants pairwise distinct (`TestExitCodesArePairwiseDistinct`) | unit | `go test ./cmd/adoBizInt/...` | ❌ W0 | ⬜ pending |
| TBD | 02 | 2 | SAFE-01 | SC6 | Every write test injects the Phase 1 seam; no test opens a socket | unit (structural) | `grep -rn "http\." --include=*_test.go internal/ado cmd \| grep -v '^[^:]*:[0-9]*:\s*//'` returns no new matches — structural, additionally satisfied by construction: every write test drives `runWrite` through `fakeWriteClient` / `TestWriteGuardMatrix`, which have no transport | partial | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/adoBizInt/write_test.go` — SC1, SC2, SC5 envelope + exit-code tests
- [ ] `cmd/adoBizInt/audit_test.go` — SC3, including the D-13 ordering test
- [ ] `cmd/adoBizInt/flags_test.go` — extend with `--confirm` registration + `fs.Visit` presence tests
- [ ] `internal/ado/workitems_test.go` — update `TestUpdateWorkItemStateWithEffortIssuesExactPatch` for the pointer signature; add the nil-omission test
- [ ] `internal/ado/fakes_test.go` — add fake methods for any new by-id read interface entries
- [ ] Shared helper for a temp audit sink (`t.TempDir()` + path) so it is not re-invented per test

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real ADO write against a live project | SAFE-01..04 | Requires live credentials and a disposable work item; out of scope for CI | Against a scratch work item: run the verb without `--confirm` (expect diff, no change), then with `--confirm` (expect change + audit line). Confirm the audit file contains exactly one new record. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
