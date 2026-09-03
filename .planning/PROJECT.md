# adoBizInt

## What This Is

adoBizInt is a terminal application for working Azure DevOps day to day: work
items, pull requests, pipelines, releases and branches, in one keyboard-driven
TUI. This milestone adds a second front door -- a non-interactive CLI with
structured JSON output -- so an AI agent can drive the same operations the TUI
already performs.

## Core Value

An agent driving this tool must never mistake a broken call for an empty result.

## Requirements

### Validated

<!-- Inferred from the existing codebase. Shipped and in daily use. -->

- Six TUI views over Azure DevOps: dashboard, work items, PRs, pipelines, releases, branches
- 25-method ADO service layer in `internal/ado`, UI-independent and context-first
- PAT auth with YAML file plus environment variable configuration
- Read operations across work items, PRs, pipelines, releases, branches
- Write operations: work item state and effort, PR approve and complete, release approve and reject, pipeline queue

### Active

<!-- This milestone. Agent-accessible CLI over features that already exist. -->

- [ ] Aggregate reads report failures instead of swallowing them
- [ ] Per-invocation timeouts and cancellation
- [ ] A test seam for `internal/ado` so write paths can be verified
- [ ] CLI subcommand surface covering existing read operations
- [ ] Stable JSON output on stdout, diagnostics on stderr
- [ ] Exit codes that distinguish success, partial, auth failure and unknown outcome
- [ ] CLI write verbs guarded by dry-run default and explicit confirmation
- [ ] Audit record for every write
- [ ] Close the attachment, path-containment and nil-dereference findings from CONCERNS.md

### Out of Scope

- **Scrum master features** (sprint CRUD, capacity, velocity, burndown, backlog rank) -- the dormant `work.Client` at `internal/ado/client.go:88` is the seam for these, but they are net-new ADO integration and this milestone deliberately ships the CLI over what already works first
- **Work item creation, comments, linking** -- same reasoning; extends the ADO surface rather than exposing it
- **Native MCP server** -- a CLI with `--json` is composable and dependency-free. Revisit only if the CLI proves insufficient in practice
- **Rewriting the TUI** -- the CLI is additive. `adoBizInt` with no subcommand must keep launching the TUI unchanged
- **Azure DevOps SDK upgrade** -- the pinned `v1.0.0-b5` is a 2020 beta (CONCERNS.md MED5). Real, but not a blocker for this milestone

## Context

Brownfield. 30 Go files, 7,601 lines, mapped in `.planning/codebase/`.

The architecture is unusually well suited to this milestone: `internal/ado`
(2,015 lines) has zero TUI imports, all 25 methods are context-first, and every
return type is a plain struct. Roughly 2,400 lines across `ado`, `config`,
`logging` and `utils/links.go` transfer to a CLI unchanged.

Three findings shape the plan:

1. **Reads cannot fail.** All eight aggregate read functions end in `return x, nil`
   with per-project errors dropped via `continue`. An expired PAT, an ADO outage
   and a genuinely empty backlog produce byte-identical output. A human watching
   a TUI notices. An agent does not. (CONCERNS.md C1)
2. **`internal/ado` is 0.0% covered and has no interface anywhere**, so the seven
   destructive writes -- including PR merge and release approval against
   production -- have no substitutable seam and cannot be tested.
   (CONCERNS.md H1, TESTING.md)
3. **There is no argument parsing anywhere.** `main()` runs one unconditional
   path from config load to `p.Run()`. There is no seam for a CLI to attach to,
   which makes this additive rather than a refactor. (ARCHITECTURE.md)

## Constraints

- **Tech stack**: Go 1.25.4, Charmbracelet Bubbletea. No new dependencies without stated justification
- **Compatibility**: `adoBizInt` with no arguments must launch the existing TUI with no behavior change
- **Security**: The PAT must never be sent to a caller-supplied host, and must never reach stdout or the log file
- **Output contract**: stdout carries only parseable output. All diagnostics go to stderr or the log file
- **Code standards**: single-use code stays inline, rule of three before extracting, no speculative generality, files under 300 lines. See `.planning/codebase/CONVENTIONS.md`

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| CLI with `--json`, not an MCP server | Composable, testable, zero new dependencies. Matches the existing binary | -- Pending |
| Ship the CLI over existing features before adding scrum features | Proves the agent interface against working code. Avoids the speculative generality the standards forbid | -- Pending |
| Fix read error propagation before shipping any read verb | A read verb that renders auth failure as an empty list actively misleads an agent. Correctness precedes surface area | -- Pending |
| Extract interfaces in `internal/ado` despite the no-abstraction rule | Explicitly required to test seven production-mutating write paths. This is the one abstraction the task demands | -- Pending |
| TUI remains the default with no subcommand | Non-breaking for daily use | -- Pending |

---
*Last updated: 2026-09-03 after brownfield onboarding*
