---
phase: 8
slug: platform-upgrade-and-pacing
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-19
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing` + `testing/synctest` on go1.27.2; `go/ast` for structural tests) |
| **Config file** | none (CI: `.github/workflows/ci.yml`) |
| **Quick run command** | `go test ./internal/transport/... ./internal/ado/... ./internal/guard/... ./internal/config/... ./cmd/adoBizInt/...` |
| **Full suite command** | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` |
| **Estimated runtime** | ~30 seconds (about 2s per package under `-race`; synctest tests add no wall time; govulncheck dominates) |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched packages (quick run command above)
- **After every plan wave:** Run `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- **Before `/gsd-verify-work`:** Full suite (including govulncheck) must be green, a green CI run on the pushed `dev` branch, and `08-SMOKE.md` filled
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 8-xx-xx | TBD | TBD | PLAT-02 / PLAT-03 / PLAT-05 | — | Populated once PLAN.md files exist (see RESEARCH.md `## Validation Architecture` test map) | — | — | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/transport/transport_test.go` — host rule table, synctest pacing, fail-fast, hold-cancel, redirect hop, refusal-before-send, PAT-never-logged
- [ ] `internal/ado/v7_test.go` — shared `rtFunc`, JSON response helper, location-list fixture (resource-areas location `e81700f7-3be2-46de-8624-2eb35882fcaa` plus each data route), unique org per test; 401 value and pointer forms; `NewClient` discovery and startup refusal
- [ ] `internal/ado/retry_test.go` — read-retry decorator behaviour plus the no-write-override AST assertion
- [ ] `internal/ado/construct_test.go` — D-11 AST assertion (non-vacuous)
- [ ] `internal/guard` — new test function for not-sent and redirect-hop outcomes; 429 row in the outcome table
- [ ] `internal/config/config_test.go` — `org_url` shape table
- [ ] `cmd/adoBizInt/throttle_test.go` — throttle summary line
- [ ] `scripts/smoke.sh` and the TUI checklist (in `08-SMOKE.md` or beside the script)
- [ ] Rewrite `internal/ado/client_test.go` `TestNewClientSetsTimeout` to assert the injected client's `Timeout`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| One live read per ADO sub-client and one dry run per write verb against the real org, recorded | PLAT-05 (SC5) | Fakes cannot observe 7.1 wire behaviour; needs the user's credential and org (no credentials in CI, v1.0 D-17) | `bash -n scripts/smoke.sh`; user runs `scripts/smoke.sh` with a freshly built binary and pastes the result table into `08-SMOKE.md` |
| CLI `set-state --confirm` on a scratch item; TUI Task close with effort; TUI attachment download | PLAT-05 (D-13) | Interactive TUI paths and live writes against the real org | Follow the TUI checklist in `08-SMOKE.md` and record outcomes |
| govulncheck gate runs and passes in CI | PLAT-02 (SC1) | Requires a pushed CI run | Push `dev`; confirm the CI govulncheck step is green |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
