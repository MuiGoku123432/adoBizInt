---
phase: "05"
slug: "hardening"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: true) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-30"
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, Go 1.25.4) |
| **Config file** | none |
| **Quick run command** | `go test -race ./internal/ado ./internal/logging ./cmd/adoBizInt ./internal/ui/workitems` |
| **Full suite command** | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run the quick run command
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite green locally plus one green GitHub Actions run on the phase branch
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

Task IDs are filled in by the planner; requirement rows come from 05-RESEARCH.md § Validation Architecture.

| Task | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01 T1 | HARD-01 | Download fetches by attachment ID via `GetAttachmentContent` | unit (fake, end-to-end at `internal/ado`) | `go test -race ./internal/ado -run TestDownloadAttachmentFetchesByID` | ✅ yes | ✅ green |
| 05-01 T3 | HARD-01 | No `net/http` import / `AuthorizationString` read / `*azuredevops.Connection` field in `internal/ado` | structural (go/ast) | `go test ./internal/ado -run TestNoRawHTTPInADO` | ✅ yes | ✅ green |
| 05-01 T3 | HARD-01 | GUID parsed from relation URL; unparsable or nil GUID not appended | unit | `go test ./internal/ado -run TestGetWorkItemDetailAttachmentID` | ✅ yes | ✅ green |
| 05-01 T1 | HARD-01 | `Attachment` JSON keys `id,name,url,size` | unit | `go test ./internal/ado -run TestJSONFieldNames` | ✅ yes | ✅ green |
| 05-01 T2 | HARD-02 | Hostile-name table: error or direct child of destDir | unit | `go test ./internal/ado -run TestAttachmentPath` | ✅ yes | ✅ green |
| 05-01 T2 | HARD-02 | Rejected name → no fetch call, destDir left empty | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentRefusesBeforeFetch` | ✅ yes | ✅ green |
| 05-01 T2 | HARD-01 | Mid-body failure removes the partial file and closes the body | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentRemovesPartialFile` | ✅ yes | ✅ green |
| 05-02 T2 | HARD-03 | Collecting loops guard `Name` and `Id` (exactly 2) | structural | `go test ./internal/ado -run TestCollectedDefinitionsGuardNameAndID` | ✅ yes | ✅ green |
| 05-02 T1, T2 | HARD-03 | nil-`Id` definition skipped, no panic | unit (fake) | `go test -race ./internal/ado -run 'TestGetPipelineRunsSkipsNilID\|TestGetReleasesSkipsNilID'` | ✅ yes | ✅ green |
| 05-05 T2 | HARD-04 | No project interpolation in any `FROM WorkItems` chain; `@project` used (D-10 revised) | structural | `go test ./internal/ado -run TestWIQLInterpolationStructural` | ✅ yes | ✅ green |
| 05-05 T1, T2 | HARD-04 | Captured query at both sites uses `@project`; logs carry no query text | unit (fake) | `go test ./internal/ado -run 'TestGetWorkItemsUsesProjectMacro\|TestDashboardUsesProjectMacro'` | ✅ yes | ✅ green |
| 05-05 T1 | HARD-04 | Iteration paths escaped through `iterationClause`, order kept, empty → "" | unit | `go test ./internal/ado -run TestIterationClauseEscapesQuotes` | ✅ yes | ✅ green |
| 05-04 T1, T2 | HARD-04 | Unknown `--project` → exit 3, no fake call (read and write) | unit (cmd fake) | `go test ./cmd/adoBizInt -run TestUnknownProjectRejected` | ✅ yes | ✅ green |
| 05-04 T1 | HARD-04 | Existing `--project` tests pass with `withConfiguredProjects` | regression | `go test -race ./cmd/adoBizInt` | ✅ yes | ✅ green |
| 05-03 T1, T2 | HARD-05 | CI runs the four gates on push/PR; gofmt step fails on an unformatted file | manual-only (user decision 2026-10-12) | none — see Manual-Only: green CI run 36798351937 on be36596 (2026-10-01) + `ci.yml` inspection | n/a | ✅ green (manual) |
| 05-06 T1 | HARD-06 | Log dir 0700 / file 0600 after `Init`, fresh, pre-existing and repeated | unit | `go test ./internal/logging -run TestInitSetsOwnerOnlyPermissions` | ✅ yes | ✅ green |
| 05-06 T2 | HARD-06 | Sentinel PAT (raw, Basic header, base64) absent from logs across read, write, panic; capture non-empty per path | unit | `go test ./cmd/adoBizInt -run TestPATNeverLogged` | ✅ yes | ✅ green |
| REVIEW-FIX WR-01 | HARD-01/02 | Failed download never clobbers prior copy; second success takes next free name | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentNeverClobbers` | ✅ yes | ✅ green |
| REVIEW-FIX WR-02 | HARD-02 | Download dir 0700 / file 0600 (skipped on Windows) | unit | `go test ./internal/ado -run TestDownloadAttachmentOwnerOnlyModes` | ✅ yes | ✅ green |
| REVIEW-FIX WR-03 | HARD-01 | Closing the detail view cancels the in-flight download | unit | `go test -race ./internal/ui/workitems -run TestCloseDetailCancelsDownload` | ✅ yes | ✅ green |
| REVIEW-FIX WR-04 | HARD-02 | Attachment line never renders raw control bytes | unit | `go test ./internal/ui/workitems -run TestDetailNeverRendersRawControlBytes` | ✅ yes | ✅ green |
| REVIEW-FIX WR-06 | HARD-04 | Structural WIQL check reports split-literal, WorkItemLinks, variable-source and `+=` bypasses; accepts parameterized shape | unit | `go test ./internal/ado -run TestWIQLCheckCatchesBypasses` | ✅ yes | ✅ green |
| REVIEW-FIX WR-05 | HARD-06 | Loaded config never renders the PAT via slog text/JSON or fmt verbs | unit | `go test ./internal/config -run TestLoadedConfigNeverRendersPAT` | ✅ yes | ✅ green |
| REVIEW-FIX CR-01 | HARD-04 | set-state refuses a work item in another project (dry-run and `--confirm`) | unit (cmd fake) | `go test ./cmd/adoBizInt -run TestSetStateRefusesWorkItemInAnotherProject` | ✅ yes | ✅ green |
| REVIEW-FIX CR-01 | HARD-04 | Detail pre-read reports the owning project | unit (fake) | `go test ./internal/ado -run TestGetWorkItemDetailReportsOwningProject` | ✅ yes | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/ado/attachments_test.go` — HARD-01/02
- [x] `internal/ado/nilguard_test.go` — HARD-03
- [x] `internal/ado/wiql_test.go` — HARD-04
- [x] `internal/logging/logging_test.go` — HARD-06 permissions
- [x] `cmd/adoBizInt/patleak_test.go` — HARD-06 PAT absence
- [x] `withConfiguredProjects` helper in `cmd/adoBizInt/main_test.go`
- [x] `GetAttachmentContentFunc` on `fakeWorkItemAPI` in `internal/ado/fakes_test.go`
- [x] `.github/workflows/ci.yml`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Evidence / Status |
|----------|-------------|------------|-------------------|
| Workflow triggers and goes green on GitHub; `ci.yml` keeps push/PR triggers and build, vet, `test -race`, gofmt gates | HARD-05 | Only a real push proves the trigger; workflow shape left manual by user decision 2026-10-12 | **Passed** — CI run 36798351937 on be36596 (push, 2026-10-01) concluded success, all four gates green, no secrets used (05-UAT.md test 1); `ci.yml` inspected 2026-10-12 |
| `@project` returns the same results as the old interpolated query | HARD-04 | Macro needs the live org | **Passed** — 05-UAT.md test 2: base-commit vs HEAD binary for `workitems list` and `dashboard counts` |
| Attachment download works end to end by ID through the TUI | HARD-01 | Needs a real attachment | **Deferred — verification debt** (05-UAT.md test 3 skipped). Covered by fakes in `internal/ado` and `internal/ui/workitems` tests |
| Live `set-state --confirm` with project in the update route | HARD-04 (CR-01) | Mutates a live work item | **Deferred — verification debt** (05-UAT.md test 4 skipped). Covered by fakes: TestSetStateRefusesWorkItemInAnotherProject, TestUpdateWorkItemState*IssuesExactPatch |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-12

---

## Validation Audit 2026-10-12

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 0 |
| Escalated | 1 |

- HARD-05 workflow shape (no regression test for `ci.yml` triggers/gates) → manual-only by user decision; evidence: green CI run 36798351937 on be36596 (2026-10-01) and `ci.yml` inspection.
- All 17 original map rows and 8 review-fix rows verified green on HEAD f52bd53 with `go test -race -count=1`.
