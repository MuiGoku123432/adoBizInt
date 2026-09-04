---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 1
current_phase_name: Test seam for internal/ado
status: planning
stopped_at: Phase 1 context gathered
last_updated: "2026-09-04T17:26:13.770Z"
last_activity: 2026-09-03
last_activity_desc: Brownfield onboarding complete; codebase mapped, project artifacts created
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-03)

**Core value:** An agent driving this tool must never mistake a broken call for an empty result.
**Current focus:** Phase 1 -- Test seam for internal/ado

## Current Position

Phase: 1 of 5 (Test seam for internal/ado)
Plan: 0 of TBD
Status: Ready to plan
Last activity: 2026-09-03 -- Brownfield onboarding complete; codebase mapped, project artifacts created

Progress: [░░░░░░░░░░] 0%

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

## Accumulated Context

### Decisions

Full log in PROJECT.md Key Decisions.

- Onboarding: CLI with `--json` chosen over a native MCP server -- composable, no new dependency
- Onboarding: Ship the CLI over existing features before adding scrum master features
- Onboarding: Fix read error propagation before shipping any read verb
- Onboarding: Extract interfaces in `internal/ado` despite the no-abstraction standard -- required to test seven production-mutating writes
- Design session: Scrum posture is "agent informs, TUI acts" -- most scrum value lands as CLI read verbs on the v1 foundation
- Design session: Capacity is display-only and never a rule input, because days off are unmaintained and capacity is therefore overstated

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

Last session: 2026-09-04T17:26:13.758Z
Stopped at: Phase 1 context gathered
Resume file: .planning/phases/01-test-seam-for-internal-ado/01-CONTEXT.md
