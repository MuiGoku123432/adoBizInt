# Requirements: adoBizInt

**Defined:** 2026-09-03
**Core Value:** An agent driving this tool must never mistake a broken call for an empty result.

## v1 Requirements

Agent-accessible CLI over Azure DevOps operations that already work. Every
requirement below is traceable to a finding in `.planning/codebase/`.

### Read Trustworthiness

Prerequisite for every read verb. Source: CONCERNS.md C1.

- [x] **TRUST-01**: Aggregate read functions return per-project failures alongside results instead of dropping them with `continue`
- [x] **TRUST-02**: HTTP 401 and 403 are fatal and never render as an empty result set
- [x] **TRUST-03**: A partial result is explicitly marked incomplete, so a caller can tell "three of four projects answered" from "four projects, no work"
- [x] **TRUST-04**: Every ADO call carries a deadline; `connection.Timeout` is set rather than left at the SDK default

### Test Seam

Prerequisite for verifying any write guard. Source: CONCERNS.md H1, TESTING.md.

- [ ] **SEAM-01**: `internal/ado` exposes narrow interfaces for the ADO sub-clients it consumes
- [x] **SEAM-02**: `ado.Client` accepts injected sub-clients so tests can substitute fakes
- [x] **SEAM-03**: All seven write methods have test coverage asserting the exact ADO call they issue
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
- [x] **OUT-02**: Exported `ado` structs carry explicit `json:` tags so field names are stable and not Go-cased
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

Scrum master milestone. Scoped in the 2026-09-03 design session, grounded in
what the `work` and `git` clients actually expose. Not in the v1 roadmap.

Posture: **the agent informs, the TUI acts.** Read verbs and synthesis go to
the CLI. Fast keyboard-driven editing goes to the TUI. Two of the three pains
driving this milestone are satisfied by CLI read verbs on the v1 foundation.

### Team Foundation

Every scrum API is `(project, team)` scoped. Current config knows only projects.

- **TEAM-01**: Config carries teams per project; `core.GetAllTeams` provides discovery
- **TEAM-02**: Team membership resolved via `core.GetTeamMembersWithExtendedProperties`
- **TEAM-03**: Current sprint resolved per team via `work.GetTeamIterations` with `Timeframe: "Current"`
- **TEAM-04**: Every scrum verb operates across multiple teams and aggregates by default; single-team is a filter, not the primary mode
- **TEAM-05**: Teams share a sprint cadence (same length, same start and end dates), so the rollup can use one shared time axis. If that stops being true, SPRINT-01 needs redesign.

### Sprint Data Access

The existing read path cannot serve this milestone. Named explicitly so planning
does not assume otherwise.

- **DATA-01**: A sprint-scoped read path that includes completed work. The current WIQL at `internal/ado/workitems.go:104` filters `State NOT IN ('Done','Closed','Removed')`, so velocity, delivered and carryover are all invisible to it
- **DATA-02**: Effort fields are read, not just written. `RemainingWork`, `CompletedWork` and `OriginalEstimate` are written at `workitems.go:249-251` but never selected in the read path, so NAG-05 and hours-based burn have no data source today
- **DATA-03**: Tasks are fetched by iteration in one query per team, not via `GetChildWorkItems` per story, which would be an N+1 across a sprint
- **DATA-04**: Sprint membership at the baseline moment is reconstructed via `workitemtracking.ReadReportingRevisionsPost`

### Board Honesty

The nag list. Rules are hardcoded; only thresholds are configurable. A general
rules engine is the speculative generality the standards forbid.

Readiness and estimation gaps:

- **NAG-01**: Work item in the current sprint with no assignee
- **NAG-02**: Work item in the current sprint with no story points
- **NAG-03**: Story or Feature with empty acceptance criteria. Emptiness is judged after HTML stripping, since the field holds markup
- **NAG-04**: Task with no parent work item
- **NAG-12**: Story with no child task matching the Development convention at `internal/ui/workitems/workitems.go:881`. Without it, hours-based burn silently undercounts that story and biases slippage toward optimism

State lies:

- **NAG-05**: Remaining work is zero but state is still Active. Depends on DATA-02
- **NAG-06**: State is New or Active but a linked pull request is already merged, resolved via `git.GetPullRequestWorkItemRefs`
- **NAG-07**: State is terminal per `config.StateTransitions` for that work item type, but the item was never advanced to its final state. Defined against the configured flow at `internal/config/config.go:53`, not against a hardcoded notion of done

Slippage predictors:

- **NAG-08**: Item still in New with fewer than N days left in the sprint
- **NAG-09**: Burn pace against elapsed working days implies the sprint misses, using `TeamIterationAttributes` dates and `TeamSetting.WorkingDays`
- **NAG-10**: Pull request still open past the sprint finish date

- **NAG-11**: Findings carry team, item, rule, and a suggested action, and are emitted as JSON
- **NAG-13**: Readiness rules NAG-01 through NAG-04 and NAG-12 run against backlog items as well as sprint items, because grooming happens before the sprint. Slippage rules are sprint-scoped only

### Ceremony Output

Synthesis, not raw data. Feeds the existing deck, spreadsheet and debrief
workflows.

- **CEREMONY-01**: `sprint report` emits five fields and no more: committed, delivered, scope change, carryover, and per-sprint points. Highest priority of the three. Deliberately narrow -- no structured sprint report exists today, so the format is being defined rather than matched, and fields should be pulled in by demonstrated need rather than because they are computable
- **CEREMONY-02**: `standup` emits what moved and what did not, grouped by person, since a caller-supplied time. Blocked status is excluded: it is not tracked in ADO and has no data source
- **CEREMONY-03**: `planning` emits what is ready to pull and what is not, with readiness gaps named
- **CEREMONY-04**: All ceremony verbs emit JSON suitable for a deck or spreadsheet pipeline
- **CEREMONY-05**: All ceremony verbs roll up across teams and can drill to one

Definitions fixed in the design session:

| Term | Definition |
|------|------------|
| Delivered | State is Resolved or Closed, matching `doneStates` at `internal/ui/workitems/workitems.go:36` |
| Hours | Sum of `RemainingWork` across all child tasks, not only the Development task. Stories with no Development task are flagged via NAG-12 rather than silently undercounted |
| Points | `Microsoft.VSTS.Scheduling.StoryPoints` on the story |
| Scope baseline | Iteration start date plus a 48 hour grace window, so planning-day churn settles before the line is drawn. Window is a configurable threshold |
| Scope change | Three separate numbers: items added, items removed, items re-pointed. Never merged into one figure |
| Carryover | Item present in sprint N, not delivered by its close, present again in sprint N+1 |
| Velocity | Raw points delivered per sprint, emitted as a series. No averaging -- the consumer decides the window |

### Sprint View (TUI)

One Sprint tab with sub-views, rather than several new top-level tabs. Tabs 1
through 6 are already bound in `internal/keymap/keymap.go`.

- **SPRINT-01**: Default sub-view is a cross-team rollup: points, pace, risk count per team, on a shared time axis per TEAM-05
- **SPRINT-02**: Drill into one team for per-person load
- **SPRINT-03**: Capacity is display-only and labelled unreliable. It is never an input to any NAG rule, because days off are not maintained and capacity is therefore systematically overstated.
- **SPRINT-04**: Risk findings are navigable and jump to the work item
- **SPRINT-05**: Sub-views are switched by a secondary key within the tab; grooming mode launches from here rather than owning its own tab

### Grooming Mode (TUI)

The one place a terminal beats the ADO web UI, which requires a page load per item.

- **GROOM-01**: Full-screen single-item view, keyboard only, advance to next
- **GROOM-02**: Inline edit of story points, assignee, sprint and acceptance criteria
- **GROOM-03**: Backlog reorder via `work.ReorderBacklogWorkItems`, against real ADO rank
- **GROOM-04**: The grooming queue is driven by the readiness gaps NAG-01 through NAG-04 and NAG-12, running in backlog scope per NAG-13

### Agent Authorization

Distinct from the v1 write guards. SAFE-01 dry-run and `--confirm` prevent
accidents; they do not constrain an agent that intends to write, because passing
the flag is the flag's designed use. Authorization is a capability boundary, not
a flag.

- **AGENT-01**: Write operations are tiered by blast radius and reversibility
- **AGENT-02**: The agent tier permits comments, story points, assignee, sprint assignment, and work item creation including tasks under a story
- **AGENT-03**: Work item state changes, effort updates, pull request merge, release approval and rejection, and pipeline queueing are human-only and unavailable to the agent at any confirmation level
- **AGENT-04**: The boundary is enforced in the CLI surface, not by documentation or convention

Agent-side task creation is the remediation path for NAG-12: the rule flags a
story missing its Development task, and the agent can create it.

### Story Management

- **STORY-01**: Create work items via `workitemtracking.CreateWorkItem`
- **STORY-02**: Read and post comments via `GetComments` and `AddComment`
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

### Excluded from the v2 scrum milestone

Decided in the 2026-09-03 design session. Recorded to prevent re-adding.

| Feature | Reason |
|---------|--------|
| Kanban board with columns | Terminals are poor at 2D drag-and-drop. The ADO web board is better and already exists |
| Rendered burndown or velocity charts | A sparkline is not why the tool gets opened. Emit numbers as JSON and let the deck pipeline draw them |
| Retrospective tooling | Separate marketplace API, and retros are a conversation rather than a data problem |
| Staleness rules (Active N days, remaining unchanged, carried over N sprints) | Explicitly rejected as noise that would not be acted on |
| Capacity contradiction rules (over-allocation, work across PTO) | Days off are not maintained, so capacity is systematically overstated. These rules would produce confident wrong answers |
| Capacity as an input to any rule | Same reason. Capacity is display-only under SPRINT-03 |
| Blocked status in standup | Not tracked in ADO. No tags are read and no blocked concept exists anywhere in the codebase. Cut rather than invented |
| Averaged velocity | Emit raw per-sprint points and let the consumer choose a window. Averaging in the tool bakes in a decision the deck pipeline should make |
| Agent access to state changes, PR merge, release approval, pipeline queue | Irreversible or high-noise. Restricted by capability boundary under AGENT-03, not by confirmation flag |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SEAM-01 .. SEAM-04 | Phase 1 | Pending |
| TRUST-01 .. TRUST-04 | Phase 2 | Pending |
| OUT-02 | Phase 2 | Complete |
| CLI-01, CLI-02, CLI-04, CLI-05, CLI-06 | Phase 3 | Pending |
| OUT-01, OUT-03, OUT-04, OUT-05 | Phase 3 | Pending |
| CLI-03 | Phase 4 | Pending |
| SAFE-01 .. SAFE-04 | Phase 4 | Pending |
| HARD-01 .. HARD-05 | Phase 5 | Pending |

**Coverage:**

- v1 requirements: 27 total
- Covered by phases: 27
- Uncovered: 0
