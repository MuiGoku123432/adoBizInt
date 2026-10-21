# adoBizInt

## What This Is

adoBizInt is a terminal application for working Azure DevOps day to day: work
items, pull requests, pipelines, releases and branches. It has two front doors
over one UI-independent service layer: a keyboard-driven TUI for humans, and a
non-interactive `noun verb` CLI with structured JSON output so an AI agent can
drive the same operations.

## Core Value

An agent driving this tool must never mistake a broken call for an empty result.

## Current State

**Shipped:** v1.0 Agent CLI (2026-10-12). See `.planning/MILESTONES.md`.

- Eight read verbs and six guarded write verbs on the CLI; `adoBizInt` with no subcommand still launches the TUI unchanged
- Every read returns a `Result[T]` envelope with `complete`, `failures` and `scope`; exit codes 0-5 distinguish success (0), partial or write failure (1), auth failure (2), usage/config/startup error (3), no-op (4) and unknown outcome (5)
- Writes default to dry run, require `--confirm`, and append a JSONL audit record before the request is sent
- Closed as `override_closeout`: Phases 2, 4 and 5 verifications are stale against later code, and live-ADO confirmation of the write path is deferred

**v2.0 in progress:** Phase 8 complete (2026-10-21) — the tool runs on azuredevops/v7 v7.1.0 and Go 1.27.3 behind an org-bound pacing transport. Passed with two user overrides: no CI-on-push run and no live dry runs or writes on v7 yet (BACKLOG BL-02). Phase 7 verification is still pending human UAT.

## Current Milestone: v2.0 Scrum Master

**Goal:** Make sprint health visible and actionable across teams -- the agent informs through CLI read verbs, the TUI acts through a Sprint tab and grooming mode.

**Target features:**
- BL-01 first: TUI writes route through the same guard and audit path as the CLI
- Team foundation: teams per project, membership, current sprint per team, multi-team rollup
- Sprint data access: sprint-scoped reads including completed work and effort fields, baseline reconstruction
- Board honesty: hardcoded nag rules with configurable thresholds, JSON findings
- Ceremony output: `sprint report`, `standup`, `planning`
- Sprint tab and grooming mode in the TUI
- Agent authorization tiers enforced in the CLI
- Story management: create, comment, link
- Platform: rate-limit backoff and fan-out caps; SDK upgrade off the 2020 beta, gated on `govulncheck`

## Requirements

### Validated

<!-- Pre-existing: inferred from the codebase at onboarding. -->

- ✓ Six TUI views over Azure DevOps: dashboard, work items, PRs, pipelines, releases, branches — pre-v1.0
- ✓ 25-method ADO service layer in `internal/ado`, UI-independent and context-first — pre-v1.0
- ✓ PAT auth with YAML file plus environment variable configuration — pre-v1.0
- ✓ Read operations across work items, PRs, pipelines, releases, branches — pre-v1.0
- ✓ Write operations: work item state and effort, PR approve and complete, release approve and reject, pipeline queue — pre-v1.0

<!-- Shipped in v1.0 Agent CLI. -->

- ✓ Aggregate reads report failures instead of swallowing them — v1.0
- ✓ Per-invocation timeouts and cancellation — v1.0
- ✓ A test seam for `internal/ado` so write paths can be verified — v1.0
- ✓ CLI subcommand surface covering existing read operations — v1.0
- ✓ Stable JSON output on stdout, diagnostics on stderr — v1.0
- ✓ Exit codes that distinguish success, partial, auth failure and unknown outcome — v1.0
- ✓ CLI write verbs guarded by dry-run default and explicit confirmation — v1.0 (live-ADO confirmation deferred)
- ✓ Audit record for every write — v1.0 (CLI only; the TUI write path is BL-01)
- ✓ Close the attachment, path-containment and nil-dereference findings from CONCERNS.md — v1.0

<!-- Validated in v2.0. -->

- ✓ Azure DevOps SDK upgrade off the 2020 beta: azuredevops/v7 v7.1.0 on Go 1.27.3, govulncheck gate in CI — Validated in Phase 8: Platform Upgrade and Pacing (CI-on-push run accepted by user override)
- ✓ Rate-limit backoff: header-driven pacing on any status, deadline fail-fast, bounded read-only retry, writes never retried, PAT bound to the configured org — Validated in Phase 8: Platform Upgrade and Pacing (live smoke and live writes waived by user override, tracked in BL-02)

### Active

<!-- v2.0 Scrum Master. Full REQ-IDs in REQUIREMENTS.md. -->

- [ ] TUI writes pass through the same guard and audit path as CLI writes (BL-01)
- [ ] Team-scoped configuration, membership and current-sprint resolution
- [ ] Sprint-scoped read path that includes completed work and effort fields
- [ ] Board-honesty nag rules emitted as JSON findings
- [ ] Ceremony verbs: sprint report, standup, planning
- [ ] Sprint tab with cross-team rollup and per-team drill-down
- [ ] Keyboard-only grooming mode with inline edits and backlog reorder
- [ ] Agent write tiers enforced in the CLI surface
- [ ] Work item creation, comments and links
- [ ] Fan-out caps (PLAT-04, Phase 9)

### Out of Scope

- **Rewriting the TUI** -- the CLI is additive. `adoBizInt` with no subcommand must keep launching the TUI unchanged
- **Native MCP server (PLAT-01)** -- deferred again from v2.0; only if the CLI proves insufficient
- **BL-02 review warnings and the deferred v1.0 live checks** -- left in `.planning/BACKLOG.md`, not v2.0 scope

## Context

Brownfield. At v1.0 close: 84 Go files, about 11,700 lines of production code and
13,200 lines of tests. The codebase map in `.planning/codebase/` predates v1.0.

The `internal/ado` layer has zero TUI imports, and now sits behind narrow
interfaces with fail-fast fakes. The dormant `work.Client` at
`internal/ado/client.go` remains the seam for sprint and capacity reads.

Known issues carried forward (`.planning/BACKLOG.md`):

- BL-01: TUI writes bypass the dry-run, confirmation and audit guards the CLI enforces
- BL-02: `prs approve` cannot detect an existing approval when the identity is unresolved; a stdout write failure after a mutation exits 3; `audit.jsonl` is created 0644; the log file is never rotated
- Pipeline reads report `complete: true` when no pipelines are configured (same class as audit W3)
- The `prs list` page size cap under no limit is unverified
- Offline startup fails because the SDK's resource-area discovery is mandatory (debug session `g-01-1`, diagnosed, deferred)

## Constraints

- **Tech stack**: Go 1.27.3, azuredevops/v7 SDK, Charmbracelet Bubbletea. No new dependencies without stated justification
- **Compatibility**: `adoBizInt` with no arguments must launch the existing TUI with no behavior change
- **Security**: The PAT must never be sent to a caller-supplied host, and must never reach stdout or the log file
- **Output contract**: stdout carries only parseable output. All diagnostics go to stderr or the log file
- **Code standards**: single-use code stays inline, rule of three before extracting, no speculative generality, files under 300 lines. See `.planning/codebase/CONVENTIONS.md`

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| CLI with `--json`, not an MCP server | Composable, testable, zero new dependencies. Matches the existing binary | ✓ Good |
| Ship the CLI over existing features before adding scrum features | Proves the agent interface against working code. Avoids the speculative generality the standards forbid | ✓ Good |
| Fix read error propagation before shipping any read verb | A read verb that renders auth failure as an empty list actively misleads an agent. Correctness precedes surface area | ✓ Good |
| Extract interfaces in `internal/ado` despite the no-abstraction rule | Explicitly required to test seven production-mutating write paths. This is the one abstraction the task demands | ✓ Good |
| TUI remains the default with no subcommand | Non-breaking for daily use | ✓ Good |
| Writes guarded by dry-run default, `--confirm` and a pre-flight audit record | An unverified `--dry-run` is worse than none; the audit must exist before the request leaves | ✓ Good (CLI); ⚠️ Revisit for the TUI path (BL-01) |
| Every read discloses its server-side `scope` | An undisclosed filter makes a filtered result look like a complete one (audit W4) | ✓ Good |
| Scrum posture: the agent informs, the TUI acts | Two of the three driving pains (stale board data, manual ceremony prep) are CLI read verbs on the v1 foundation. Only navigation speed needs new TUI work | -- Pending |
| Capacity is display-only, never a rule input | Days off are not maintained, so ADO capacity is systematically overstated. Reasoning from it would produce confident wrong answers -- the C1 failure mode one layer up | -- Pending |
| Nag rules hardcoded, thresholds configurable | A general rules engine is speculative generality. The rule set is the feature, not the extensibility | -- Pending |
| Agent writes bounded by capability tier, not by confirmation flag | Dry-run and `--confirm` prevent accidents, not intent. An agent passing `--confirm` is using the flag as designed. Authorization needs a boundary | -- Pending |
| Hours summed across all child tasks, with missing Development task flagged | Summing only the Development task would silently undercount and bias slippage toward optimism. Flagging makes the gap visible instead | -- Pending |
| Sprint report starts at five fields | No structured report exists today, so the format is being defined rather than matched. Fields earn their way in through use | -- Pending |

## Design Sessions

| Date | Topic | Output |
|------|-------|--------|
| 2026-09-03 | Scrum master milestone scope, features and TUI shape | REQUIREMENTS.md v2 (archived in `milestones/v1.0-REQUIREMENTS.md`): TEAM, DATA, NAG, CEREMONY, SPRINT, GROOM, AGENT, STORY |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-10-21 after Phase 8 completion*
