---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 01
current_phase_name: test-seam-for-internal-ado
status: executing
stopped_at: Completed 01-03-PLAN.md
last_updated: "2026-09-04T22:26:29.287Z"
last_activity: 2026-09-04
last_activity_desc: Phase 01 execution started
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 5
  completed_plans: 3
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-03)

**Core value:** An agent driving this tool must never mistake a broken call for an empty result.
**Current focus:** Phase 01 — test-seam-for-internal-ado

## Current Position

Phase: 01 (test-seam-for-internal-ado) — EXECUTING
Plan: 4 of 5
Status: Ready to execute
Last activity: 2026-09-04 — Phase 01 execution started

Progress: [██████░░░░] 60%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: --
- Total execution time: 0.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: none
- Trend: --

**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01 | 4 min | 2 tasks | 4 files |
| Phase 01 P02 | 9 min | 3 tasks | 4 files |
| Phase 01 P03 | 15 min | 3 tasks | 4 files |

## Accumulated Context

### Decisions

Full log in PROJECT.md Key Decisions.

- Onboarding: CLI with `--json` chosen over a native MCP server -- composable, no new dependency
- Onboarding: Ship the CLI over existing features before adding scrum master features
- Onboarding: Fix read error propagation before shipping any read verb
- Onboarding: Extract interfaces in `internal/ado` despite the no-abstraction standard -- required to test seven production-mutating writes
- Design session: Scrum posture is "agent informs, TUI acts" -- most scrum value lands as CLI read verbs on the v1 foundation
- Design session: Capacity is display-only and never a rule input, because days off are unmaintained and capacity is therefore overstated
- [Phase 01]: Use the assignable unexported Client.workitemClient field as the injection seam; add no constructor or adapter. — In-package tests can assign the field directly, preserving the public API and avoiding constructor network I/O.
- [Phase 01]: Rely on NewClient's SDK-interface assignment as compile-time conformance proof; add no ClientImpl assertion. — The SDK constructor returns an interface, so the production assignment is the correct compile-time relationship.
- [Phase 01]: Require each fake method to fail by interface and method name when its function field is unset. — Unexpected ADO calls must fail at the call site instead of returning misleading zero values.
- [Phase 01]: Cache AuthenticatedUser.Id rather than AuthorizedUser.Id for pull request approval under delegated access. — This preserves the identity used by the prior approval implementation.
- [Phase 01]: Keep identity resolution non-fatal during NewClient construction while warning when approval identity is unavailable. — Read-only TUI startup remains compatible while the lost approval capability becomes visible.
- [Phase 01]: Keep workClient concrete and intentionally dormant until v2 scrum requirements consume it. — It has no current call sites, so an empty interface would add meaningless abstraction.
- [Phase 01]: Represent expected effort patch values as float64 literals so interface{} dynamic types match production values. — reflect.DeepEqual compares dynamic types as well as values.
- [Phase 01]: Use cached Client.currentUserID as the approval reviewer ID. — It preserves AuthenticatedUser.Id semantics from plan 01-02.
- [Phase 01]: Keep a dedicated release status cross-check in addition to exact-call tests. — It detects aligned test-oracle drift that independent literals can miss.

### Pending Todos

None yet.

### Blockers/Concerns

Carried from `.planning/codebase/CONCERNS.md`. All are addressed by the roadmap.

- **C1 (critical)**: All eight aggregate reads swallow per-project errors. An expired PAT is indistinguishable from an empty backlog. Phase 2.
- **C2 (critical)**: Seven destructive writes have no dry-run, no idempotency, no audit. The only confirmation lives in the TUI at `internal/ui/prs/prs.go:69` and a CLI bypasses it. Phase 4.
- **C3 (critical)**: `DownloadAttachment` sends the PAT to a caller-supplied URL with no allow-list or timeout (`internal/ado/workitems.go:520`). Phase 5.
- **H1 (high)**: `internal/ado` is 0.0% covered with zero interface seam. Phase 1.
- **H2 (high)**: 22 `context.Background()` sites, no timeouts, `connection.Timeout` never set. Phase 2.

Affecting the v2 scrum milestone, not v1:

- **Days off unmaintained in ADO.** ADO capacity is `capacityPerDay x workingDays - daysOff`, so every capacity figure is overstated. Constrains v2 design: capacity is display-only, and no nag rule may read it. Revisit if teams start maintaining days off.

### Environment Notes

- Gortex has this repo tracked and indexed (31 files, 2352 nodes, 14808 edges) with churn enrichment. The Gortex MCP tool surface was loaded before tracking, so it reports `repo_not_tracked` until the session reconnects. The `gortex call ...` CLI path works now.
- `gsd` CLI is not on PATH. GSD is reachable through its MCP server.

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Scrum master | Sprint CRUD, capacity, velocity, burndown, backlog rank | v2 | 2026-09-03 |
| Story management | Work item creation, comments, links | v2 | 2026-09-03 |
| Platform | Native MCP server | v2 | 2026-09-03 |
| Platform | Azure DevOps SDK upgrade off 2020 beta | v2 | 2026-09-03 |

## Session Continuity

Last session: 2026-09-04T22:26:29.282Z
Stopped at: Completed 01-03-PLAN.md
Resume file: None
