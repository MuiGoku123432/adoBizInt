# Requirements: adoBizInt

**Defined:** 2026-10-13
**Core Value:** An agent driving this tool must never mistake a broken call for an empty result.

## v2.0 Requirements

Milestone v2.0 Scrum Master. Scope: the 2026-09-03 scrum design, amended by
`.planning/research/SUMMARY.md` (2026-10-13), plus BL-01 first and PLAT-02/03.

Posture: **the agent informs, the TUI acts.** Read verbs and synthesis go to the
CLI. Fast keyboard-driven editing goes to the TUI.

The v2 failure mode is one layer above v1's: calls that succeed but answer for the
wrong target (default team, shared iteration path, wrong points field, UTC date
shift, absent value read as 0). Every scrum read therefore discloses its targeting
in `scope`, and every scrum verb has negative tests for no teams, no current sprint,
an empty sprint, and one failing team.

### Guarded TUI Writes (BL-01)

- [ ] **GUARD-01**: The guarded-write sequence (audit open, pre-read and diff, no-op refusal, confirm gate, mutate under recover, outcome classification, audit append) lives in one package both the CLI and the TUI call. It never writes stdout and never exits the process
- [ ] **GUARD-02**: All existing TUI write call sites go through that guard with the real project and a bounded context. A structural test fails if any `internal/ui` file calls an `ado` mutation method directly
- [ ] **GUARD-03**: The TUI confirms once per item: edits to one item are staged and confirmed once when the user moves on, with one audit record per changed field. The compound resolve (state plus effort) is two sequential guarded writes
- [ ] **GUARD-04**: Every audit record carries an `actor` key (`tui`, `cli-human` or `agent`) and records whether `--user-approved` was supplied (AGENT-03)
- [ ] **GUARD-05**: The TUI effort modal carries "leave unchanged" as absent, not zero. An end-to-end test drives the modal through the guard
- [ ] **GUARD-06**: Extraction is behaviour-preserving for the CLI: the existing CLI write tests pass unchanged

### Platform

- [ ] **PLAT-02**: The Azure DevOps SDK moves from `v1.0.0-b5` to `azuredevops/v7` v7.1.0 in one change, with the Go toolchain raised to 1.27 and `golang.org/x/sys` to a version with no known vulnerabilities. CI runs `govulncheck` and fails on any reachable vulnerability. A test proves a v7 401 still classifies as an auth failure
- [ ] **PLAT-03**: A pacing HTTP transport honours `Retry-After` and rate-limit headers on any response, including HTTP 200. Reads retry on a rate-limit failure a bounded number of times within their deadline; writes are never retried automatically. The PAT is only sent to hosts validated against the configured organization
- [ ] **PLAT-04**: Request-shape caps (maximum teams, maximum sprints of history, backlog depth) are validated before the first call and disclosed in `scope`
- [ ] **PLAT-05**: After the upgrade, one live read per ADO sub-client and one dry run per write verb pass against the real organization before later phases build on v7

### Team Foundation

Every scrum API is `(project, team)` scoped. Current config knows only projects.

- [ ] **TEAM-01**: Config carries teams per project as an allow-list, validated at startup against `core.GetTeams` (paged). An unknown team or zero configured teams is a config error (exit 3). A `teams list` verb supports discovery
- [ ] **TEAM-02**: Team membership is resolved with identities keyed by id; group members are flagged rather than silently expanded or dropped
- [ ] **TEAM-03**: The current sprint is resolved per team. No current iteration, or an iteration with null dates, is a per-team failure, never an empty sprint. Sprint dates are civil dates stored as UTC midnight, and the finish date is inclusive
- [ ] **TEAM-04**: Every scrum verb operates across all configured teams and aggregates by default; `--team P/T` filters to one. Rollups never emit a synthetic "all teams" row
- [ ] **TEAM-05**: Shared sprint cadence across teams is checked at runtime, and a mismatch is reported as a finding rather than assumed away
- [ ] **TEAM-06**: Team membership of work items is the team's iteration intersected with the team's area paths (with their include-children flags), applied identically at every point in time. Items claimed by two teams produce an overlap finding
- [ ] **TEAM-07**: The process template is resolved per project: the points field (StoryPoints, Effort or Size), the done-category states and the backlog rank field. A project whose fields cannot be resolved fails with a named config failure
- [ ] **TEAM-08**: Read failures carry an always-present `team` key (empty for project-scoped reads); the JSON contract test is updated in the same change

### Sprint Data Access

- [ ] **DATA-01**: A sprint-scoped read includes completed work. It filters on the team's iteration id (formatted as an integer, never string-built) with no state filter
- [ ] **DATA-02**: Story points, RemainingWork, CompletedWork and OriginalEstimate are read as nullable numbers and serialise as `null` when absent. The v1 `story_points` integer field is unchanged
- [ ] **DATA-03**: Tasks are fetched by iteration in one query per team and grouped by parent, not fetched per story. Batch reads compare requested ids with returned ids and report any dropped
- [ ] **DATA-04**: Sprint membership and field values at a past instant (the scope baseline, the sprint close) are reconstructed with WIQL `ASOF` and as-of batch reads at UTC instants
- [ ] **DATA-05**: A `sprint items` verb exposes the per-team sprint snapshot so live data can be checked before rules are built on it
- [ ] **DATA-06**: Acceptance-criteria emptiness is judged after stripping tags, decoding entities (including `&nbsp;`) and collapsing whitespace

### Board Honesty

Rules are hardcoded; only thresholds are configurable. A rule whose input could not be
read is skipped and recorded as a failure, never evaluated against empty input.

Readiness and estimation gaps:

- [ ] **NAG-01**: Work item in the current sprint with no assignee
- [ ] **NAG-02**: Work item in the current sprint with no story points (null, not zero)
- [ ] **NAG-03**: Story or Feature with empty acceptance criteria, per DATA-06
- [ ] **NAG-04**: Task with no parent work item
- [ ] **NAG-12**: Story with no Development child task, identified by the Activity field first and the title convention second; the finding records which matched

State lies:

- [ ] **NAG-05**: Remaining work is zero but state is still Active
- [ ] **NAG-06**: State is New or Active but a linked pull request was completed into the repository's default branch. Linked PRs are read from the item's artifact links, capped, with an incomplete marker when the cap is hit
- [ ] **NAG-07**: Parent story is still Active while every child task is in a terminal state per the project's configured state flow

Slippage predictors:

- [ ] **NAG-08**: Item still in New with fewer than N working days left in the sprint
- [ ] **NAG-09**: Remaining-hours fraction exceeds remaining-working-day fraction by more than a tolerance. Working days come from team settings only (not days off, not capacity). Below a RemainingWork coverage threshold the verdict is "insufficient data"
- [ ] **NAG-10**: Pull request linked to a sprint item still open past the sprint finish date

Output and scope:

- [ ] **NAG-11**: Findings carry team, item, rule, a stable key (`rule:item`) and a suggested action naming the CLI verb that would fix it, and are emitted as JSON
- [ ] **NAG-13**: NAG-02, NAG-03 and NAG-04 also run against the top `backlog_depth` not-done backlog items, with the depth disclosed. NAG-01 and NAG-12 run on sprint items only. Slippage rules are sprint-scoped only

### Ceremony Output

Synthesis, not raw data. Feeds the existing deck, spreadsheet and debrief workflows.

- [ ] **CEREMONY-01**: `sprint report` emits five fields and no more: committed, delivered, scope change, carryover, and the velocity series. Both instants used (baseline and close) and the delivered definition are disclosed in `scope`. Carryover for a sprint still in progress is `null` with a reason
- [ ] **CEREMONY-02**: `standup` emits, grouped by current assignee, items with a state transition or a RemainingWork decrease since a required `--since` time, read from revision history, and lists members with no movement. Blocked status is excluded: it has no data source
- [ ] **CEREMONY-03**: `planning` emits the backlog in rank order with each item marked ready or not ready, its readiness gaps named, and the raw velocity series. No capacity
- [ ] **CEREMONY-04**: All ceremony verbs emit JSON suitable for a deck or spreadsheet pipeline
- [ ] **CEREMONY-05**: All ceremony verbs roll up across teams and can drill to one

Definitions:

| Term | Definition |
|------|------------|
| Committed | Points and count of requirement-level items in the team's sprint as of the scope baseline |
| Delivered | In the sprint and in state Resolved or Closed as of the close instant. Done and Removed are excluded. Deliberately differs from ADO's velocity widget (Completed category only); the definition is disclosed in `scope` |
| Close instant | End of the sprint's inclusive finish date |
| Hours | Sum of `RemainingWork` across all child tasks, not only the Development task. Stories with no Development task are flagged via NAG-12 rather than silently undercounted |
| Points | The project's resolved points field (TEAM-07) on the requirement-level item |
| Scope baseline | Iteration start date plus a 48 hour grace window, so planning-day churn settles before the line is drawn. Window is a configurable threshold |
| Scope change | Three separate numbers: items added, items removed, items re-pointed. Never merged into one figure |
| Carryover | Item present in sprint N, not delivered by its close, present again in sprint N+1 |
| Velocity | Raw points delivered per sprint over the last N sprints (default 6, maximum 12), emitted as a series. No averaging |

### Sprint View (TUI)

One Sprint tab with sub-views, rather than several new top-level tabs.

- [ ] **SPRINT-01**: Default sub-view is a cross-team rollup: points, pace and risk count per team, on an elapsed-working-day-fraction axis so teams with different cadences remain comparable
- [ ] **SPRINT-02**: Drill into one team for per-person load
- [ ] **SPRINT-03**: Capacity is display-only and labelled unreliable. It is never an input to any rule
- [ ] **SPRINT-04**: Risk findings are navigable and jump to the work item
- [ ] **SPRINT-05**: Sub-views are switched by a secondary key within the tab; grooming mode launches from here rather than owning its own tab

### Grooming Mode (TUI)

- [ ] **GROOM-01**: Full-screen single-item view, keyboard only, advance to next
- [ ] **GROOM-02**: Inline edit of story points, assignee (chosen from resolved team members), and sprint. Acceptance criteria are append-only: new text is added below existing content, which is never rewritten
- [ ] **GROOM-03**: Backlog reorder against real ADO rank, available only in the full-backlog view. Neighbours are re-read before the move, the move aborts if they changed, and the before and after neighbours are audited
- [ ] **GROOM-04**: The grooming queue is driven by the readiness gaps NAG-02, NAG-03, NAG-04 and NAG-12 and is edit-only (no reorder)
- [ ] **GROOM-05**: The local `priorities.yaml` order is kept as a personal order. Every list view labels which order it shows (ADO rank or personal)
- [ ] **GROOM-06**: All grooming writes go through the guard (GUARD-01..03)

### Agent Authorization

Write verbs are tiered by blast radius and reversibility. The user can explicitly
authorize an agent to use any verb on a per-command basis. This is a cooperative
convention enforced in the CLI, not a security boundary: an agent able to run the
CLI can supply the flag itself.

- [ ] **AGENT-01**: Every write verb has a tier recorded in the command table: `agent` or `human`. A verb with no tier fails a table test. New verbs default to `human`
- [ ] **AGENT-02**: The `agent` tier is: comments, story points, assignee, sprint assignment, work item creation (including tasks under a story), and link add. These require only `--confirm`
- [ ] **AGENT-03**: The `human` tier is: state changes, effort updates, PR approve, PR complete, release approve and reject, pipeline queue, backlog reorder, and link removal. These require `--confirm` plus either an interactive terminal or `--user-approved`, which an agent passes only when the user has explicitly authorized that command
- [ ] **AGENT-04**: The tier is checked once at dispatch. A refusal exits with a dedicated code 6, distinct from usage errors (3)
- [ ] **AGENT-05**: Work item creation and updates accept only an allow-listed set of fields (title, type, parent, description, acceptance criteria, points, assignee, iteration), so state or effort cannot be set through a create or update

### Story Management

- [ ] **STORY-01**: Create work items. A child inherits area and iteration from its parent. Dry run uses the server's validate-only mode. Creating a Development task where one already exists is a no-op
- [ ] **STORY-02**: Read comments (paged) and post comments in Markdown
- [ ] **STORY-03**: Add and remove work item links. Removal verifies the item revision first
- [ ] **STORY-04**: A `workitems update` verb sets points, assignee, iteration and appends acceptance criteria, within AGENT-05's allow-list

## Future Requirements

Deferred. Tracked but not in this roadmap.

- **PLAT-01**: Native MCP server, only if the CLI proves insufficient
- Late-delivered count in the sprint report
- Attribution of scope moves to agent actors
- Concurrent fan-out across teams (serial plus caps until a measured all-teams read exceeds the fetch timeout)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Kanban board with columns | Terminals are poor at 2D drag-and-drop. The ADO web board is better and already exists |
| Rendered burndown or velocity charts | Emit numbers as JSON and let the deck pipeline draw them |
| Retrospective tooling | Separate marketplace API, and retros are a conversation rather than a data problem |
| Staleness rules (Active N days, remaining unchanged, carried over N sprints) | Rejected as noise that would not be acted on |
| Capacity contradiction rules and capacity as a rule input | Days off are not maintained, so capacity is systematically overstated |
| Team days off in working-day math | Decided 2026-10-13: working days come from team settings only; the source is disclosed in `scope` |
| Blocked status in standup | Not tracked in ADO; no data source |
| Averaged velocity | Emit raw per-sprint points; the consumer chooses a window |
| Rewriting or overwriting existing acceptance criteria | ADO stores AC as rich HTML; a terminal editor would destroy formatting. Append only |
| Native MCP server (PLAT-01) | The CLI with `--json` remains the agent interface |
| BL-02 review warnings and the deferred v1.0 live checks | Left in `.planning/BACKLOG.md`. PLAT-05's smoke run overlaps the live checks; findings are recorded without reopening v1.0 |
| Bubble Tea v2 migration | Would break "TUI unchanged" |
| Windows browse verb in the CLI | `cmd /c start` injection risk (CONCERNS.md H4) |

## Compliance Notes

- Agent use of `human`-tier verbs (PR complete and approve, release approve and reject) under `--user-approved` touches SOX change-management and segregation-of-duties controls. Every such use is audited with `actor` and the approval flag (GUARD-04). Confirm the change-management policy permits agent-executed merges and approvals before authorizing them.
- Standup, per-person load and comments expose work item free text and names. Phases that copy work item text into output are flagged for CPNI review.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|

**Coverage:**
- v2.0 requirements: 62 total
- Mapped to phases: 0 (filled by roadmap)
- Unmapped: 62 ⚠️

---
*Requirements defined: 2026-10-13*
*Last updated: 2026-10-13 after research and scoping decisions*
