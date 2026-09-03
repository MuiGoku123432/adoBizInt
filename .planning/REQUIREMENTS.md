# Requirements: adoBizInt

**Defined:** 2026-09-03
**Core Value:** An agent driving this tool must never mistake a broken call for an empty result.

## v1 Requirements

Agent-accessible CLI over Azure DevOps operations that already work. Every
requirement below is traceable to a finding in `.planning/codebase/`.

### Read Trustworthiness

Prerequisite for every read verb. Source: CONCERNS.md C1.

- [ ] **TRUST-01**: Aggregate read functions return per-project failures alongside results instead of dropping them with `continue`
- [ ] **TRUST-02**: HTTP 401 and 403 are fatal and never render as an empty result set
- [ ] **TRUST-03**: A partial result is explicitly marked incomplete, so a caller can tell "three of four projects answered" from "four projects, no work"
- [ ] **TRUST-04**: Every ADO call carries a deadline; `connection.Timeout` is set rather than left at the SDK default

### Test Seam

Prerequisite for verifying any write guard. Source: CONCERNS.md H1, TESTING.md.

- [ ] **SEAM-01**: `internal/ado` exposes narrow interfaces for the ADO sub-clients it consumes
- [ ] **SEAM-02**: `ado.Client` accepts injected sub-clients so tests can substitute fakes
- [ ] **SEAM-03**: All seven write methods have test coverage asserting the exact ADO call they issue
- [ ] **SEAM-04**: The aggregate read failure paths from TRUST-01 are covered by tests

### CLI Surface

- [ ] **CLI-01**: `adoBizInt` with no subcommand launches the existing TUI with no behavior change
- [ ] **CLI-02**: Read subcommands exist for work items, PRs, pipelines, releases, branches and dashboard
- [ ] **CLI-03**: Write subcommands exist for the seven existing mutations
- [ ] **CLI-04**: `--timeout` sets a per-invocation deadline; the process handles interrupt signals
- [ ] **CLI-05**: Every command responds to `--help` with its flags and arguments
- [ ] **CLI-06**: Filtering an agent would want (assigned-to-me, hide-done, by project, by repo) is available as flags, promoted out of the view packages where it currently lives

### Structured Output

- [ ] **OUT-01**: `--json` emits machine-readable JSON on stdout
- [ ] **OUT-02**: Exported `ado` structs carry explicit `json:` tags so field names are stable and not Go-cased
- [ ] **OUT-03**: stdout carries only parseable output; all logging and diagnostics go to stderr or the log file
- [ ] **OUT-04**: Exit codes distinguish success, partial result, authentication failure, and outcome-unknown
- [ ] **OUT-05**: A panic emits structured JSON rather than dying silently and looking like an empty result

### Write Safety

Source: CONCERNS.md C2.

- [ ] **SAFE-01**: Write verbs are dry-run by default and require an explicit confirmation flag to execute
- [ ] **SAFE-02**: Every executed write appends an audit record identifying what changed
- [ ] **SAFE-03**: Effort updates accept omitted fields as "leave unchanged" rather than blind-overwriting with zero
- [ ] **SAFE-04**: A write that times out reports outcome-unknown rather than success or failure

### Hardening

- [ ] **HARD-01**: Attachment download takes an attachment identifier, not a caller-supplied URL, so the PAT cannot be sent to an arbitrary host
- [ ] **HARD-02**: Attachment writes are contained to the intended directory
- [ ] **HARD-03**: Nil-dereference guards on the ADO SDK pointer fields identified in CONCERNS.md MED4
- [ ] **HARD-04**: WIQL project interpolation is escaped and validated against the configured project list
- [ ] **HARD-05**: CI runs build, vet, test with race detection, and gofmt on every push

## v2 Requirements

Tracked, deliberately not in this roadmap.

### Scrum Master

- **SCRUM-01**: Sprint and iteration CRUD via the dormant `work.Client`
- **SCRUM-02**: Team capacity planning
- **SCRUM-03**: Velocity and burndown
- **SCRUM-04**: Backlog rank ordering against ADO rank, not the local priorities file

### Story Management

- **STORY-01**: Create work items
- **STORY-02**: Read and post comments
- **STORY-03**: Create and remove work item links

### Platform

- **PLAT-01**: Native MCP server, only if the CLI proves insufficient
- **PLAT-02**: Azure DevOps SDK upgrade off the 2020 beta, gated on `govulncheck`
- **PLAT-03**: Rate limit backoff and request fan-out caps

## Out of Scope

| Feature | Reason |
|---------|--------|
| Scrum master features in v1 | Net-new ADO integration. Ship the agent interface over working code first |
| Work item creation and comments in v1 | Extends the ADO surface rather than exposing it |
| Native MCP server | CLI with `--json` is composable and adds no dependency |
| TUI rewrite | The CLI is additive; the TUI must not change behavior |
| Windows browse verb in the CLI | `cmd /c start` injection risk (CONCERNS.md H4). The TUI keeps it; the CLI omits it |
| SDK upgrade | Real debt, not a blocker for this milestone |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SEAM-01 .. SEAM-04 | Phase 1 | Pending |
| TRUST-01 .. TRUST-04 | Phase 2 | Pending |
| OUT-02 | Phase 2 | Pending |
| CLI-01, CLI-02, CLI-04, CLI-05, CLI-06 | Phase 3 | Pending |
| OUT-01, OUT-03, OUT-04, OUT-05 | Phase 3 | Pending |
| CLI-03 | Phase 4 | Pending |
| SAFE-01 .. SAFE-04 | Phase 4 | Pending |
| HARD-01 .. HARD-05 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 27 total
- Covered by phases: 27
- Uncovered: 0
