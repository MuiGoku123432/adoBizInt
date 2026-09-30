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

| Task | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01 T1 | HARD-01 | Download fetches by attachment ID via `GetAttachmentContent` | unit (fake, end-to-end at `internal/ado`) | `go test -race ./internal/ado -run TestDownloadAttachmentFetchesByID` | ❌ W0 | ⬜ pending |
| 05-01 T3 | HARD-01 | No `net/http` import / `AuthorizationString` read / `*azuredevops.Connection` field in `internal/ado` | structural (go/ast) | `go test ./internal/ado -run TestNoRawHTTPInADO` | ❌ W0 | ⬜ pending |
| 05-01 T3 | HARD-01 | GUID parsed from relation URL; unparsable or nil GUID not appended | unit | `go test ./internal/ado -run TestGetWorkItemDetailAttachmentID` | ❌ W0 | ⬜ pending |
| 05-01 T1 | HARD-01 | `Attachment` JSON keys `id,name,url,size` | unit | `go test ./internal/ado -run TestJSONFieldNames` | ✅ update | ⬜ pending |
| 05-01 T2 | HARD-02 | Hostile-name table: error or direct child of destDir | unit | `go test ./internal/ado -run TestAttachmentPath` | ❌ W0 | ⬜ pending |
| 05-01 T2 | HARD-02 | Rejected name → no fetch call, destDir left empty | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentRefusesBeforeFetch` | ❌ W0 | ⬜ pending |
| 05-01 T2 | HARD-01 | Mid-body failure removes the partial file and closes the body | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentRemovesPartialFile` | ❌ W0 | ⬜ pending |
| 05-02 T2 | HARD-03 | Collecting loops guard `Name` and `Id` (exactly 2) | structural | `go test ./internal/ado -run TestCollectedDefinitionsGuardNameAndID` | ❌ W0 | ⬜ pending |
| 05-02 T1, T2 | HARD-03 | nil-`Id` definition skipped, no panic | unit (fake) | `go test -race ./internal/ado -run 'TestGetPipelineRunsSkipsNilID\|TestGetReleasesSkipsNilID'` | ❌ W0 | ⬜ pending |
| 05-05 T2 | HARD-04 | No project interpolation in any `FROM WorkItems` chain; `@project` used (D-10 revised) | structural | `go test ./internal/ado -run TestWIQLInterpolationStructural` | ❌ W0 | ⬜ pending |
| 05-05 T1, T2 | HARD-04 | Captured query at both sites uses `@project`; logs carry no query text | unit (fake) | `go test ./internal/ado -run 'TestGetWorkItemsUsesProjectMacro\|TestDashboardUsesProjectMacro'` | ❌ W0 | ⬜ pending |
| 05-05 T1 | HARD-04 | Iteration paths escaped through `iterationClause`, order kept, empty → "" | unit | `go test ./internal/ado -run TestIterationClauseEscapesQuotes` | ❌ W0 | ⬜ pending |
| 05-04 T1, T2 | HARD-04 | Unknown `--project` → exit 3, no fake call (read and write) | unit (cmd fake) | `go test ./cmd/adoBizInt -run TestUnknownProjectRejected` | ❌ W0 | ⬜ pending |
| 05-04 T1 | HARD-04 | Existing `--project` tests pass with `withConfiguredProjects` | regression | `go test -race ./cmd/adoBizInt` | ✅ update | ⬜ pending |
| 05-03 T1, T2 | HARD-05 | CI runs the four gates on push/PR; gofmt step fails on an unformatted file | CI + structural (YAML parse) | Ruby structural check in 05-03 T1 + gofmt-step proof in 05-03 T2 + observed green run (phase-blocking human-check on 05-03 T2) | ❌ W0 | ⬜ pending |
| 05-06 T1 | HARD-06 | Log dir 0700 / file 0600 after `Init`, fresh, pre-existing and repeated | unit | `go test ./internal/logging -run TestInit` | ❌ W0 | ⬜ pending |
| 05-06 T2 | HARD-06 | Sentinel PAT (raw, Basic header, base64) absent from logs across read, write, panic; capture non-empty per path | unit | `go test ./cmd/adoBizInt -run TestPATNeverLogged` | ❌ W0 | ⬜ pending |

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
| Workflow triggers and goes green on GitHub | HARD-05 | Only a real push proves the trigger | **PHASE-BLOCKING:** Phase 5 is not complete without one green run on the phase branch. Covered by the `<human-check>` on 05-03 T2. Push the phase branch after the wave-2 plans land; confirm a green run. Optionally push an unformatted file, confirm red, revert (the optional part is not blocking). |
| `@project` returns the same results as the old interpolated query | HARD-04 | Macro not yet exercised against the live org | **PHASE-BLOCKING (D-10 revised):** Phase 5 is not complete until this passes. One `<human-check>` on 05-05 T2 covers both `workitems list` and `dashboard counts`. Compare a base-commit binary against HEAD for one configured project. |
| Attachment download works end to end by ID | HARD-01 | Needs a real attachment | `<human-check>` on 05-01 T3. Download one known attachment through the TUI; confirm file contents and location. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
