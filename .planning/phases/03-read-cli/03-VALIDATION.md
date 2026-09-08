---
phase: 03
slug: read-cli
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-08
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `03-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`, table-driven |
| **Config file** | none — `go test ./...` |
| **Quick run command** | `go test ./internal/ado/... ./cmd/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

No testify, no mock generator, no gomock in `go.mod`. [VERIFIED: go.mod:5-41]

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/ado/... ./cmd/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

> Task IDs are filled in by the planner. Requirement→behavior mapping below is
> authoritative and comes from research.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | 03-01 | 1 | CLI-01 | — | no args → TUI, unchanged | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-01 | 1 | CLI-02 | — | eight verbs dispatch to eight methods | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-01 | 1 | CLI-05 | — | three `--help` levels | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-02 | 2 | CLI-06 | — | promoted filters behave identically pre/post | unit | `go test ./internal/ado/... ./internal/ui/...` | ❌ W0 | ⬜ pending |
| TBD | 03-03 | 3 | OUT-01 | — | `--json` stdout parses | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-03 | 3 | OUT-03 | — | stdout is byte-exact JSON, nothing else | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-03 | 3 | — | — | `items`/`failures` are `[]` not `null` | unit | `go test ./internal/ado/...` | ❌ W0 | ⬜ pending |
| TBD | 03-04 | 4 | OUT-04 | — | four codes, auth-wins precedence | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-04 | 4 | OUT-05 | — | panic → `{"error":{...}}` + exit 3 | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| TBD | 03-04 | 4 | CLI-04 | — | `--timeout` bounds; SIGINT cancels | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/adoBizInt/main_test.go` — no test file exists for `cmd/` at all
- [ ] `internal/ado/filters_test.go` — for the promoted filter predicates
- [ ] Behavior-equivalence harness for `buildRows()` across the five view packages
      (workitems, prs, branches, releases, pipelines) — no test files exist in
      `internal/ui/*` for row building today
- [ ] A `run() int`-shaped entry point so exit codes are testable without spawning
      a process. **Blocking prerequisite for OUT-04 and OUT-05** — `os.Exit` skips
      deferred functions, so the panic handler is dead code until this lands.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| TUI renders identically to pre-phase | CLI-01 | Terminal rendering has no assertion harness in this repo | Run `adoBizInt` with no args, confirm dashboard, tab navigation, and each view render unchanged |
| SIGINT during a live network fetch | CLI-04 | Requires a real in-flight ADO request to interrupt | Run a read subcommand against a slow query, press Ctrl-C, confirm exit code and no partial JSON on stdout |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
