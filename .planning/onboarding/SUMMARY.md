# Onboarding Summary: adoBizInt

**Onboarded:** 2026-09-03
**Type:** Brownfield -- existing Go codebase, 30 files, 7,601 lines

## What This Project Is

A Charmbracelet Bubbletea terminal UI for Azure DevOps covering work items,
pull requests, pipelines, releases and branches. This milestone adds a
non-interactive CLI with JSON output so an AI agent can drive the same
operations.

## What Was Learned

| Document | Covers |
|----------|--------|
| `codebase/STACK.md` | Go 1.25.4, five TUI libraries, config precedence, 12 notable absences including no CLI framework |
| `codebase/INTEGRATIONS.md` | Azure DevOps as the sole remote system, six sub-clients, all 25 public methods mapped to SDK calls, three N+1 fan-outs |
| `codebase/ARCHITECTURE.md` | `cmd -> ui -> ado` layering, Elm message loop, and a CLI-suitability assessment quantifying 2,400 of 7,601 lines as directly reusable |
| `codebase/STRUCTURE.md` | Annotated tree, 16-package inventory, acyclic import graph, eight "where to add code" recipes |
| `codebase/CONVENTIONS.md` | Observed Go conventions plus the adopted engineering standards, with an 18-row gap table |
| `codebase/TESTING.md` | Coverage per package, both existing test files dissected, zero interfaces means no test seam |
| `codebase/CONCERNS.md` | 19 ranked findings, four compounding failure chains, and a gated remediation sequence |

## The Three Findings That Shaped the Roadmap

1. **Reads cannot fail.** All eight aggregate read functions end in `return x, nil`
   with per-project errors dropped. An expired PAT, an outage and an empty
   backlog are byte-identical. A human notices; an agent does not.
2. **`internal/ado` is 0.0% covered with zero interfaces**, so seven writes --
   including PR merge and release approval against production -- cannot be
   tested or safely guarded.
3. **No argument parsing exists anywhere**, so a CLI is genuinely additive
   rather than a refactor. That is the good news.

## Scope Decision

Milestone one ships an agent-accessible CLI over operations that already work.
Scrum master features (sprint CRUD, capacity, velocity, backlog rank) are
deferred to v2 despite being the eventual goal, because they are net-new Azure
DevOps integration and the agent interface should be proven against working
code first. The dormant `work.Client` at `internal/ado/client.go:88` is the
seam those features will use.

## Roadmap

Five phases. Correctness precedes surface area because an agent can only trust
the exit code and the JSON.

1. Test seam for `internal/ado`
2. Honest reads -- failures surface, auth errors fatal, deadlines everywhere
3. Read CLI -- subcommands, JSON, exit codes
4. Guarded writes -- dry-run default, explicit confirmation, audit record
5. Hardening -- attachment and path fixes, nil guards, WIQL escaping, CI

## Tooling State

- **Gortex**: tracked and indexed (31 files, 2352 nodes, 14808 edges) with churn enrichment. The MCP tool surface loaded before tracking and reports `repo_not_tracked` until the session reconnects; `gortex call ...` works now.
- **GSD**: reachable via MCP. The `gsd` CLI is not on PATH.

## Next Command

```
/gsd-plan-phase 1
```
