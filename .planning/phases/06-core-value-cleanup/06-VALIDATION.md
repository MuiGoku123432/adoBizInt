---
phase: "06"
slug: "core-value-cleanup"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-10-12"
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source: 06-RESEARCH.md § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) |
| **Config file** | none; CI at `.github/workflows/ci.yml` |
| **Quick run command** | `go test -count=1 ./cmd/adoBizInt/ ./internal/config/ ./internal/ado/` |
| **Full suite command** | `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** quick run command
- **After every plan wave:** full suite command
- **Before `/gsd-verify-work`:** full suite green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

Task IDs are filled in by the planner. Test names are research proposals.

| Requirement | Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|-----------|-------------------|-------------|--------|
| CLI-05, CLI-06, OUT-04 (W2) | Help token as a flag value is the value; read/write runs | unit | `go test ./cmd/adoBizInt -run 'TestHelpTokenAsFlagValueIsNotHelp\|TestReleaseCommentReachesTheMutation' -count=1` | ❌ W0 / ✅ extend | ⬜ pending |
| CLI-05 (W2, D-09) | Help in flag position works with no config, incl. `--project P --help`, `set-state 4821 --help` | unit | `go test ./cmd/adoBizInt -run 'TestHelpTokenInFlagPositionIsHelp\|TestHelpNeedsNoCredentials\|TestHelpIsDeterministic' -count=1` | ❌ W0 / ✅ | ⬜ pending |
| CLI-06 (D-08) | WR-05: `--comment -h` reaches the mutation | unit | `go test ./cmd/adoBizInt -run TestReleaseCommentReachesTheMutation -count=1` | ✅ extend | ⬜ pending |
| OUT-04 (D-17) | Stray positional on a read verb exits 3, nothing runs | unit | `go test ./cmd/adoBizInt -run TestReadRejectsStrayPositional -count=1` | ❌ W0 | ⬜ pending |
| TRUST-03, OUT-04 (W3) | Empty, nil, blank, whitespace projects → `ConfigError{Field:"projects"}` | unit | `go test ./internal/config -run 'TestValidate\|TestLoadRejectsBlankProjects' -count=1` | ✅ extend / ❌ W0 | ⬜ pending |
| TRUST-03 (W3, TUI) | `Validate` precedes both `dispatch` and `ui.NewModel` in `run()` | structural | `go test ./cmd/adoBizInt -run TestValidateGuardsCLIAndTUI -count=1` | ❌ W0 | ⬜ pending |
| OUT-04, CLI-06 (W4) | `scope` key on every `Result[T]`; null when unscoped | unit (contract) | `go test ./internal/ado -run 'TestJSONFieldNames\|TestNewResultScopeIsNull' -count=1` | ✅ update / ❌ W0 | ⬜ pending |
| TRUST-03 (W4, D-15/D-16) | Exact scope per read; scope ↔ WIQL/SearchCriteria tie; `complete` unchanged; early returns | unit | `go test ./internal/ado -run 'Scope' -count=1` | ❌ W0 | ⬜ pending |
| CLI-06 (W4, D-04/05/06) | Human stderr scope line, never under `--json`; verb help states scope; `--hide-done` text | unit | `go test ./cmd/adoBizInt -run 'Scope\|TestHideDoneHelp' -count=1` | ❌ W0 | ⬜ pending |
| OUT-04, OUT-05, SAFE-02, SAFE-04 (W1) | Panic in mutate → exit 5, crash record on stdout, one `unknown` audit line; failing sink → 5; pre-mutate panic → 3; no double record | unit | `go test ./cmd/adoBizInt -run 'TestPanic(After\|Before)Write' -count=1` | ❌ W0 | ⬜ pending |
| SAFE-02 (W1) | Recover defer after `defer sink.Close()` and immediately before `op.mutate` | structural | `go test ./cmd/adoBizInt -run TestWriteRecoverPrecedesMutate -count=1` | ❌ W0 | ⬜ pending |
| OUT-05 (regression) | Read panic still exits 3 | unit | `go test ./cmd/adoBizInt -run 'TestPanicExitsThree\|TestPATNeverLogged' -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] W2 tests in `cmd/adoBizInt/usage_test.go`; `-h` row in `writeverbs_test.go`
- [ ] D-17 stray-positional test in `cmd/adoBizInt`
- [ ] W3 rows in `internal/config/config_test.go`; fix the `main_test.go:735` fixture
- [ ] W4 contract updates in `internal/ado/json_test.go`; scope tests beside `wiql_test.go` / `pullrequests_test.go`; CLI scope-line and help tests
- [ ] W1 tests in `cmd/adoBizInt`; structural test for the recover position

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `workitems list --search help --json` returns items and `scope` against the live org | CLI-06 | Live confirmation of automated coverage | Run read-only against the Windstream org; expect items and a `scope` object |
| TUI refuses to start with no projects | TRUST-03 | Interactive | `ADO_PROJECTS=` with no projects in config; expect "Invalid config: projects: …" and exit 3 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
