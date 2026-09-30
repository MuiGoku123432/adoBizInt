---
phase: "05"
slug: "hardening"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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

| Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|-----------------|-----------|-------------------|-------------|--------|
| HARD-01 | Download fetches by attachment ID via `GetAttachmentContent` | unit (fake) | `go test -race ./internal/ado -run TestDownloadAttachmentFetchesByID` | ❌ W0 | ⬜ pending |
| HARD-01 | No `net/http` import / `AuthorizationString` read in `internal/ado` | structural (go/ast) | `go test ./internal/ado -run TestNoRawHTTPInADO` | ❌ W0 | ⬜ pending |
| HARD-01 | GUID parsed from relation URL; unparsable GUID not appended | unit | `go test ./internal/ado -run TestGetWorkItemDetailAttachmentID` | ❌ W0 | ⬜ pending |
| HARD-01 | `Attachment` JSON keys `id,name,url,size` | unit | `go test ./internal/ado -run TestJSONFieldNames` | ✅ update | ⬜ pending |
| HARD-02 | Hostile-name table: error or direct child of destDir | unit | `go test ./internal/ado -run TestAttachmentPath` | ❌ W0 | ⬜ pending |
| HARD-02 | Rejected name → no fetch call | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentRefusesBeforeFetch` | ❌ W0 | ⬜ pending |
| HARD-03 | Collecting loops guard `Name` and `Id` | structural | `go test ./internal/ado -run TestCollectedDefinitionsGuardNameAndID` | ❌ W0 | ⬜ pending |
| HARD-03 | nil-`Id` definition skipped, no panic | unit (fake) | `go test -race ./internal/ado -run 'TestGetPipelineRunsSkipsNilID\|TestGetReleasesSkipsNilID'` | ❌ W0 | ⬜ pending |
| HARD-04 | No project interpolation in any `FROM WorkItems` chain; `@project` used (D-10 revised) | structural | `go test ./internal/ado -run TestWIQLInterpolationStructural` | ❌ W0 | ⬜ pending |
| HARD-04 | Captured query at both sites uses `@project`; logs carry no query text | unit (fake) | `go test ./internal/ado -run 'TestGetWorkItemsUsesProjectMacro\|TestDashboardUsesProjectMacro'` | ❌ W0 | ⬜ pending |
| HARD-04 | Unknown `--project` → exit 3, no fake call (read and write) | unit (cmd fake) | `go test ./cmd/adoBizInt -run TestUnknownProjectRejected` | ❌ W0 | ⬜ pending |
| HARD-04 | Existing `--project` tests pass with `withConfiguredProjects` | regression | `go test -race ./cmd/adoBizInt` | ✅ update | ⬜ pending |
| HARD-05 | CI runs the four gates on push/PR | CI | `test -f .github/workflows/ci.yml` + observed green run | ❌ W0 | ⬜ pending |
| HARD-06 | Log dir 0700 / file 0600 after `Init`, fresh and pre-existing | unit | `go test ./internal/logging -run TestInit` | ❌ W0 | ⬜ pending |
| HARD-06 | Sentinel PAT (raw + Basic form) absent from logs across read, write, panic; capture non-empty | unit | `go test ./cmd/adoBizInt -run TestPATNeverLogged` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/ado/attachments_test.go` — HARD-01/02
- [ ] `internal/ado/nilguard_test.go` — HARD-03
- [ ] `internal/ado/wiql_test.go` — HARD-04
- [ ] `internal/logging/logging_test.go` — HARD-06 permissions
- [ ] `cmd/adoBizInt/patleak_test.go` — HARD-06 PAT absence
- [ ] `withConfiguredProjects` helper in `cmd/adoBizInt/main_test.go`
- [ ] `GetAttachmentContentFunc` on `fakeWorkItemAPI` in `internal/ado/fakes_test.go`
- [ ] `.github/workflows/ci.yml`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Workflow triggers and goes green on GitHub | HARD-05 | Only a real push proves the trigger | Push the phase branch; confirm a green run. Optionally push an unformatted file, confirm red, revert. |
| `@project` returns the same results as the old interpolated query | HARD-04 | Macro not yet exercised against the live org | Run `workitems list` and `dashboard` for one configured project before and after; compare counts. |
| Attachment download works end to end by ID | HARD-01 | Needs a real attachment | Download one known attachment via the TUI/CLI; confirm file contents and location. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
