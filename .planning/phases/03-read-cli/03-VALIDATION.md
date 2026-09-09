---
phase: 03
slug: read-cli
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
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

> Reconciled against the five-plan decomposition. Plan, wave and task columns match
> the PLAN.md files; the requirement→behavior mapping comes from research.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-T1 | 03-01 | 1 | CLI-01 | T-03-02 | no args → TUI, unchanged | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| 03-01-T1 | 03-01 | 1 | OUT-01 | T-03-04 | `--json` stdout parses as one document | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| 03-01-T2 | 03-01 | 1 | OUT-04 | T-03-03 | four codes, auth-wins precedence | unit | `go test ./cmd/... -run TestExit` | ❌ W0 | ⬜ pending |
| 03-01-T3 | 03-01 | 1 | OUT-03 | T-03-01 | stdout is byte-exact JSON, nothing else | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| 03-01-T3 | 03-01 | 1 | — | — | `items`/`failures` are `[]` not `null` | unit | `go test ./internal/ado/...` | ❌ W0 | ⬜ pending |
| 03-01-T3 | 03-01 | 1 | OUT-03 | T-03-01 | `logging.Init` failure warns on stderr, stdout stays pure (closes A1) | unit | `go test ./cmd/...` | ❌ W0 | ⬜ pending |
| 03-02-T1 | 03-02 | 2 | CLI-02 | T-03-02 | eight verbs dispatch to eight methods | unit | `go test ./cmd/... -run TestDispatch` | ❌ W0 | ⬜ pending |
| 03-02-T2 | 03-02 | 2 | CLI-05 | T-03-05 | three `--help` levels; `--help` works with no PAT | unit | `go test ./cmd/... -run 'TestFlags\|TestUsage\|TestHelp'` | ❌ W0 | ⬜ pending |
| 03-02-T3 | 03-02 | 2 | OUT-03 | T-03-07 | human rows on stdout, failures on stderr, no ANSI | unit | `go test ./cmd/... -run TestRender` | ❌ W0 | ⬜ pending |
| 03-03-T1 | 03-03 | 2 | CLI-06 | T-03-08, T-03-09 | five promoted predicates; empty set means match-all | unit | `go test ./internal/ado/... -run TestFilter` | ❌ W0 | ⬜ pending |
| 03-03-T2 | 03-03 | 2 | CLI-01 | T-03-08 | `buildRows()` ordered-row equivalence; both priority gates survive | unit | `go test ./internal/ui/... ./internal/ado/...` | ❌ W0 | ⬜ pending |
| 03-04-T1 | 03-04 | 3 | CLI-06 | T-03-13 | one search matcher, five views, original field sets | unit | `go test ./internal/ui/... ./internal/ado/...` | ❌ W0 | ⬜ pending |
| 03-04-T2 | 03-04 | 3 | CLI-06 | T-03-11, T-03-12 | filter flags default off; `--project`/`--repo` narrow the read | unit | `go test ./cmd/... -run 'TestFilterFlags\|TestDefaults'` | ❌ W0 | ⬜ pending |
| 03-04-T3 | 03-04 | 3 | CLI-01 | — | TUI renders and filters identically post-refactor | manual | checkpoint:human-verify | n/a | ✅ passed (human, 2026-09-09) |
| 03-05-T1 | 03-05 | 4 | CLI-04 | T-03-17 | `--timeout` bounds the read, survives a sub-`poll_interval` value | unit | `go test ./internal/ui/fetchctx/... ./cmd/...` | ❌ W0 | ⬜ pending |
| 03-05-T2 | 03-05 | 4 | CLI-04 | T-03-16 | SIGINT cancels; interrupt record, exit 3 | unit | `go test ./cmd/... -run TestInterrupt` | ❌ W0 | ⬜ pending |
| 03-05-T3 | 03-05 | 4 | OUT-05 | T-03-14, T-03-18 | panic → `{"error":{...}}` + exit 3, no stack, no PAT | unit | `go test ./cmd/... -run TestPanic` | ❌ W0 | ⬜ pending |

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

| Behavior | Requirement | Why Manual | Test Instructions | Status |
|----------|-------------|------------|-------------------|--------|
| Zero-argument TUI launch (coverage entry D7) | CLI-01 | The TUI path needs a live PAT, a real ADO organisation and an interactive terminal; no test constructs a bubbletea program against a real client | Run `adoBizInt` with no args, confirm the TUI opens | ✅ passed (human, 2026-09-09) |
| TUI renders identically to pre-phase | CLI-01 | Terminal rendering has no assertion harness in this repo | Run `adoBizInt` with no args, confirm dashboard, tab navigation, and each view render unchanged | ✅ passed (human, 2026-09-09) |
| SIGINT during a live network fetch | CLI-04 | Requires a real in-flight ADO request to interrupt | Run a read subcommand against a slow query, press Ctrl-C, confirm exit code and no partial JSON on stdout | ⬜ not required — closed by `TestInterrupt*` against the real signal-cancelled parent context |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-09 — all manual verifications closed by human UAT
