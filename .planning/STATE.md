---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 01
current_phase_name: test-seam-for-internal-ado
status: complete
stopped_at: Completed 01-06-PLAN.md
last_updated: "2026-09-06T05:42:53.464Z"
last_activity: 2026-09-06
last_activity_desc: Phase 01 execution started
progress:
  total_phases: 1
  completed_phases: 1
  total_plans: 6
  completed_plans: 6
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-03)

**Core value:** An agent driving this tool must never mistake a broken call for an empty result.
**Current focus:** Phase 01 — test-seam-for-internal-ado

## Current Position

Phase: 01 (test-seam-for-internal-ado) — COMPLETE
Plan: 6 of 6
Status: Phase complete
Last activity: 2026-09-06 — Completed gap-closure plan 01-06

Progress: [██████████] 100%

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
| Phase 01 P04 | 18 min | 2 tasks | 2 files |
| Phase 01 P05 | 11 min | 3 tasks | 4 files |
| Phase 01 P06 | 9 min | 2 tasks | 3 files |

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
- [Phase 01]: Inject aggregate work-item failures at QueryByWiql, because classification-node failures are warned and discarded. — This is the first error path that actually fails GetWorkItems at project scope.
- [Phase 01]: Assert distinct positive pull request counts from both dashboard projects. — The two-project sum distinguishes in-place query swallowing from project skipping.
- [Phase 01]: Use continue-to-return propagation for the GetWorkItems mutation proof. — Deleting continue alone is behaviorally inert because the failed result slice is nil.
- [Phase 01]: Use poisoned non-nil fixture data with injected project errors so deleting continue produces an observable failed-project result. — A nil error result can make literal continue removal behaviorally inert; poisoned data proves the characterization is non-vacuous.
- [Phase 01]: Use the anchored top-level-only pattern ^=== RUN   TestGet.*SwallowsProjectFailure$ for the eight-test phase gate. — The anchor excludes the intentional GetWorkItems trap subtest without deleting it and remains stable if more subtests are added.
- [Phase 01]: Preserve dashboard inversion as a getProjectCounts propagation change rather than an outer-loop-only edit. — Dashboard errors are swallowed in place and its outer continue is unreachable until the helper propagates.
- [Phase 01]: Keep successful eager identity caching and retry through the retained location provider whenever the approval cache is empty. — This restores transient-failure recovery without adding public API or shared mutable state.
- [Phase 01]: Do not memoize fallback identity results; each empty-cache approval performs a fresh lookup. — The 1/0, 2/1, and 3/2 provider/Git contract preserves per-attempt retry semantics.

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

Last session: 2026-09-06T05:42:53.459Z
Stopped at: Completed 01-06-PLAN.md
Resume file: None
