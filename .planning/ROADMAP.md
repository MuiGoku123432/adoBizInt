# Roadmap: adoBizInt

## Milestones

- ✅ **v1.0 Agent CLI** — Phases 1-6 (shipped 2026-10-12) — [archive](milestones/v1.0-ROADMAP.md)
- 🚧 **v2.0 Scrum Master** — Phases 7-16 (in progress)

## Phases

<details>
<summary>✅ v1.0 Agent CLI (Phases 1-6) — SHIPPED 2026-10-12</summary>

- [x] Phase 1: Test seam for internal/ado (6/6 plans) — completed 2026-10-12
- [x] Phase 2: Honest reads (7/7 plans) — completed 2026-10-12
- [x] Phase 3: Read CLI (5/5 plans) — verified 2026-09-09
- [x] Phase 4: Guarded writes (4/4 plans) — completed 2026-10-12
- [x] Phase 5: Hardening (6/6 plans) — completed 2026-10-12
- [x] Phase 6: Core-value cleanup (5/5 plans) — completed 2026-10-12

</details>

### 🚧 v2.0 Scrum Master (In Progress)

**Milestone Goal:** Make sprint health visible and actionable across teams. The agent informs through CLI read verbs; the TUI acts through a Sprint tab and grooming mode.

Ten phases add a scrum layer to the v1.0 agent CLI. The order follows one rule:
nothing is built on a path that would later have to be rebuilt. Phase 7 puts TUI
writes on the CLI guard, so every later write lands on one guard. Phase 8 moves to
SDK v7 and Go 1.27 before any new code is written against the 2020 beta, and
installs rate-limit pacing ahead of the first multi-team fan-out. Phases 9-12
build the read side in dependency order: teams, then the sprint snapshot, then the
rules, then ceremony synthesis. Phases 13-14 add agent tiers before the first new
agent-reachable write. Phases 15-16 add the TUI Sprint tab and grooming mode last,
because they consume every earlier phase.

The v2 failure mode sits one layer above v1's: calls that succeed but answer for the
wrong target (default team, shared iteration path, wrong points field, UTC date
shift, absent value read as 0). Every scrum phase therefore discloses its targeting
in `scope`, and every scrum verb carries negative tests for no teams, no current
sprint, an empty sprint, and one failing team.

**Decisions in effect (user, 2026-10-13):** human-tier verbs are allowed with
`--user-approved`; a tier refusal exits 6; audit records gain an `actor` key;
acceptance criteria are append-only; Delivered = Resolved + Closed; the process
template is resolved per project; working days come from team settings only.

- [ ] **Phase 7: Guarded TUI Writes** - TUI writes go through the same guard and audit path as the CLI (BL-01)
- [ ] **Phase 8: Platform Upgrade and Pacing** - SDK v7, Go 1.27 and govulncheck in CI, a rate-limit pacing transport, and a live smoke run
- [ ] **Phase 9: Team Foundation** - Validated teams per project, membership, current sprint per team, process fields, and request-shape caps
- [ ] **Phase 10: Sprint Data Access** - Team-scoped sprint snapshot with completed work, nullable effort, and as-of baseline, exposed as `sprint items`
- [ ] **Phase 11: Board Honesty** - Hardcoded nag rules with configurable thresholds, emitted as actionable JSON findings
- [ ] **Phase 12: Ceremony Output** - `sprint report`, `standup` and `planning` as reproducible cross-team JSON
- [ ] **Phase 13: Agent Authorization** - Write verbs tiered agent or human, enforced once at dispatch, refusal exits 6
- [ ] **Phase 14: Story Management** - Create, update, comment and link work items within an allow-listed field set
- [ ] **Phase 15: Sprint Tab** - TUI cross-team sprint rollup with team drill-down and navigable risk findings
- [ ] **Phase 16: Grooming Mode** - Keyboard-only grooming with guarded inline edits and rank-safe backlog reorder

## Phase Details

### Phase 7: Guarded TUI Writes

**Goal**: Every write made from the TUI is guarded and audited exactly like a CLI write, through one guard both surfaces share
**Depends on**: Nothing (first phase of v2.0)
**Requirements**: GUARD-01, GUARD-02, GUARD-03, GUARD-04, GUARD-05, GUARD-06
**Success Criteria** (what must be TRUE):
  1. A state or effort change made in the TUI shows the pre-read diff, refuses a no-op without sending a request, and is confirmed once per item when the user moves on, leaving one audit record per changed field
  2. A TUI resolve (state plus effort) runs as two sequential guarded writes with two audit records, and leaving the effort modal's field blank leaves that field unchanged rather than zeroing it, proven by an end-to-end test that drives the modal through the guard
  3. A structural test fails if any `internal/ui` file calls an `ado` mutation method directly, and every existing TUI write runs with the real project and a bounded context
  4. Every audit record, from the CLI or the TUI, carries an `actor` key (`tui`, `cli-human` or `agent`) and a field recording whether `--user-approved` was supplied
  5. The existing CLI write tests pass unchanged, and the shared guard never writes to stdout and never exits the process. Approved Phase 7 exception (07-03 checkpoint): TestExecutedWriteAppendsAuditRecord and TestPanicAfterWriteIssuedIsAudited expect the two GUARD-04 keys `actor` and `user_approved`, TestAuditRecordOmitsArgvAndCredentialFields no longer forbids "actor", and TestWriteRecoverPrecedesMutate is relocated to internal/guard as TestRunRecoverPrecedesMutate; see REQUIREMENTS.md GUARD-06 for the detail

**Plans**: 5/8 plans executed
Plans:
**Wave 1**
- [x] 07-01-PLAN.md — Lock the CLI write tests (sha256 + inventory) and move fetchctx to internal/fetchctx

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 07-02-PLAN.md — Extract internal/guard (Run, audit sink 0600/0700, ops) and make the CLI a thin adapter

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 07-03-PLAN.md — Persist `actor` and `user_approved` on every audit record (decision checkpoint: CLI actor label)
- [x] 07-04-PLAN.md — Shared confirm flow (diff modal, preview/execute) and PR approve/complete through the guard

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 07-05-PLAN.md — Release approve/reject and pipeline queue: input step, then diff confirm, then guarded write
- [ ] 07-06-PLAN.md — Work items: stage state changes and confirm on move-on (row, other item), leave API

**Wave 5** *(blocked on Wave 4 completion)*
- [ ] 07-07-PLAN.md — Compound resolve as sequential guarded writes, blank-effort end-to-end test, no-bypass structural test
- [ ] 07-08-PLAN.md — App-level D-02: tab switch and quit confirm staged edits; quit held while a write is busy

**UI hint**: yes
**Research**: Skip. Internal refactor, protected by the existing CLI write tests
**Compliance**: Supports SOX ITGC audit evidence for TUI-made changes. Do not weaken the TUI path to confirm-only.
**Notes**: The `--user-approved` flag itself arrives in Phase 13; the audit field exists from this phase. `audit.jsonl` is still created 0644 (T-05-26, BL-02, out of scope), and v2.0 starts writing agent-supplied text to it: decide in discuss-phase whether to pull that one fix in or record it as an accepted risk.

### Phase 8: Platform Upgrade and Pacing

**Goal**: The tool runs on a supported SDK and toolchain with no reachable vulnerabilities, and slows down when Azure DevOps signals throttling without ever retrying a write
**Depends on**: Phase 7
**Requirements**: PLAT-02, PLAT-03, PLAT-05
**Success Criteria** (what must be TRUE):
  1. CI runs `govulncheck` and fails on any reachable vulnerability; the upgraded build (`azuredevops/v7` v7.1.0, Go 1.27, patched `golang.org/x/sys`) passes it, and no `v1.0.0-b5` import remains
  2. A rejected PAT under v7 still exits with the auth-failure code, proven by a test
  3. After a response carrying `Retry-After` or rate-limit headers, including an HTTP 200, later requests wait as instructed; a rate-limited read retries a bounded number of times within its deadline, and a rate-limited write is never retried and exits with the write-failure code
  4. The PAT is only ever sent to hosts validated against the configured organization; a request aimed at any other host is refused before the credential leaves the process
  5. One live read per ADO sub-client and one dry run per write verb pass against the real organization, and the results are recorded before Phase 9 starts

**Plans**: TBD
**Research**: Yes. Live behaviour after the 5.1-preview to 7.1 API version jump (PR complete, vsrm approvals, comments); request-body replay via `GetBody` on POST; `LocationUrl` host rules
**Notes**: Kept after Phase 7 so the live smoke run also exercises the new TUI guard path. PLAT-02 lands as one change before the PLAT-03 pacer, because `http.Client` injection needs v7. Update the "Go 1.25.4" constraint in PROJECT.md. v7 does not fix offline startup (`g-01-1`); carry it forward. The PLAT-05 smoke run overlaps the deferred v1.0 live checks: record what it finds without reopening v1.0.

### Phase 9: Team Foundation

**Goal**: Every scrum read is aimed at a named, validated team and that team's current sprint, never at a project's default team or an assumed cadence
**Depends on**: Phase 8
**Requirements**: TEAM-01, TEAM-02, TEAM-03, TEAM-04, TEAM-05, TEAM-07, TEAM-08, PLAT-04
**Success Criteria** (what must be TRUE):
  1. `teams list` discovers each configured project's teams (paged), and configuring an unknown team or zero teams stops the tool at startup with exit 3 and a named config error
  2. Each configured team's current sprint resolves to civil start and inclusive finish dates that stay correct under `TZ=America/Chicago` and `TZ=Asia/Kolkata`; a team with no current iteration or null dates is a per-team failure, never an empty sprint, and every read failure carries a `team` key (empty for project-scoped reads)
  3. A scrum read covers all configured teams by default, `--team P/T` narrows it to one, output never contains a synthetic "all teams" row, and teams whose sprint cadences differ produce a mismatch finding
  4. Team membership lists identities keyed by id with group members flagged, and each project's points field, done-category states and backlog rank field are resolved, or the project fails with a named config failure
  5. A request exceeding a shape cap (maximum teams, sprints of history, backlog depth) is refused before the first call, and the caps in force appear in `scope`

**Plans**: TBD
**Research**: Light checks only: `GetTeams` paging, group-member handling, and PAT scopes for the new team calls
**Notes**: TEAM-08 is a deliberate v2 JSON contract change; update the contract test in the same change. TEAM-06 (area-path intersection) is in Phase 10, where work items are first read, so one filter serves both the live set and the baseline.

### Phase 10: Sprint Data Access

**Goal**: A user or agent can inspect each team's sprint exactly as Azure DevOps holds it, now and at the baseline, including completed work, before any rule is built on it
**Depends on**: Phase 9
**Requirements**: TEAM-06, DATA-01, DATA-02, DATA-03, DATA-04, DATA-05, DATA-06
**Success Criteria** (what must be TRUE):
  1. `sprint items` returns each team's sprint including completed and closed items, selected by the team's iteration id and intersected with the team's area paths (honouring include-children), with the team, area paths and points field disclosed in `scope`
  2. Story points, RemainingWork, CompletedWork and OriginalEstimate appear as `null` when absent and keep fractional values when present, while the v1 `story_points` integer field is unchanged
  3. Tasks arrive in one query per team grouped under their parent, and any id requested but not returned by a batch read is reported as dropped
  4. Sprint membership and field values at the scope baseline and at sprint close are reconstructed at UTC instants with the same team filter as the live set, and an item claimed by two teams produces an overlap finding
  5. Acceptance criteria holding only tags, entities such as `&nbsp;`, or whitespace are judged empty

**Plans**: TBD
**Research**: Yes, spike first. `System.IterationId` combined with WIQL `ASOF`, tested live (fallback: the server-sourced iteration path through the existing escaper, recorded as a deviation); whether `$expand` and `fields` are mutually exclusive; reading PR link facts without N+1; what `$timeframe=current` returns between sprints
**Notes**: The iteration id is formatted as an integer with `strconv`, never string-built. Add `IsDelivered` (Resolved + Closed) beside `IsDoneState` rather than reusing `doneStates`.

### Phase 11: Board Honesty

**Goal**: An agent can ask where the board is lying or incomplete and get JSON findings it can act on, with any rule that could not run named rather than silently passed
**Depends on**: Phase 10
**Requirements**: NAG-01, NAG-02, NAG-03, NAG-04, NAG-05, NAG-06, NAG-07, NAG-08, NAG-09, NAG-10, NAG-11, NAG-12, NAG-13
**Success Criteria** (what must be TRUE):
  1. `sprint nag` reports readiness gaps (no assignee, points absent rather than zero, empty acceptance criteria, task with no parent, story with no Development task with the matching method recorded) as JSON findings carrying team, item, rule, a stable `rule:item` key and a suggested CLI verb that would fix it
  2. State-lie findings appear for an Active item with zero remaining work, a New or Active item whose linked pull request was completed into the default branch (PR reads capped, with an incomplete marker when the cap is hit), and an Active parent whose child tasks are all terminal
  3. Slippage findings appear for items still New with fewer than N working days left, for remaining hours running ahead of remaining working days beyond tolerance (or "insufficient data" below the RemainingWork coverage threshold, with working days taken from team settings and that source disclosed), and for linked pull requests still open past the sprint finish
  4. NAG-02, NAG-03 and NAG-04 also run over the top `backlog_depth` not-done backlog items with the depth disclosed, while NAG-01, NAG-12 and the slippage rules stay sprint-scoped
  5. A rule whose input could not be read is skipped and listed as a failure rather than evaluated against empty input, and thresholds change through config while the rule set does not

**Plans**: TBD
**Research**: Yes. NAG-06, NAG-07 and NAG-09 semantics as amended; `BugsBehavior` (whether bugs count as requirement-level items)
**Notes**: NAG-06 and NAG-10 (PR linkage) can land as a later plan within the phase.

### Phase 12: Ceremony Output

**Goal**: Sprint review, standup and planning prep each come from one command as reproducible JSON across all configured teams
**Depends on**: Phase 10, Phase 11
**Requirements**: CEREMONY-01, CEREMONY-02, CEREMONY-03, CEREMONY-04, CEREMONY-05
**Success Criteria** (what must be TRUE):
  1. `sprint report` emits exactly five fields: committed, delivered, scope change (added, removed and re-pointed as three separate numbers), carryover, and the raw velocity series (default 6 sprints, maximum 12, no average); `scope` discloses the baseline and close instants and the Resolved + Closed delivered definition, and carryover for a sprint still in progress is `null` with a reason
  2. `standup --since T` groups items with a state transition or a RemainingWork decrease since T by current assignee, read from revision history, and lists members with no movement; running it without `--since` is a usage error
  3. `planning` lists the backlog in ADO rank order with each item marked ready or not ready, its readiness gaps named, and the raw velocity series, with no capacity figure
  4. Every ceremony verb emits JSON a deck or spreadsheet pipeline can consume, rolls up across all configured teams by default, drills to one team with `--team`, and has negative tests for no teams, no current sprint, an empty sprint and one failing team

**Plans**: TBD
**Research**: Yes. Decoding and paging reporting revisions (loop until `isLastBatch`; an empty continuation token is a failure) and their TSTU cost
**Compliance**: CPNI review. `standup` emits names and work item text grouped by person.
**Notes**: `sprint report` needs the Phase 10 baseline; `planning` needs the Phase 11 readiness rules. Ship `sprint report` first.

### Phase 13: Agent Authorization

**Goal**: An agent can run high-blast-radius write verbs only when the user has explicitly authorized that command, and a tier refusal is distinguishable from every other failure
**Depends on**: Phase 7
**Requirements**: AGENT-01, AGENT-02, AGENT-03, AGENT-04
**Success Criteria** (what must be TRUE):
  1. Every write verb in the command table has a tier (`agent` or `human`), a table test fails on any verb without one, and a newly added verb is `human` unless declared otherwise
  2. Run non-interactively with `--confirm` but without `--user-approved`, every human-tier verb (state change, effort update, PR approve, PR complete, release approve and reject, pipeline queue) is refused with exit code 6 before any request; with `--user-approved`, or from an interactive terminal, it proceeds
  3. The agent tier (comments, story points, assignee, sprint assignment, work item creation including tasks under a story, link add) is recorded in the table and requires only `--confirm`
  4. The tier is checked once at dispatch, an approved human-tier write's audit record shows the actor and that `--user-approved` was supplied, and the documentation states that tiers constrain a cooperative agent rather than forming a security boundary

**Plans**: TBD
**Research**: Skip. Dispatch-table data plus a TTY check
**Compliance**: SOX. `--user-approved` lets an agent run PR complete and approve and release approve and reject, which touches change-management and segregation-of-duties controls. Confirm the change-management policy permits agent-executed merges and approvals before authorizing them.
**Notes**: Technically needs only the Phase 7 guard and audit `actor` key; scheduled after Phase 12 to avoid churn with the Phase 8 import swap and the new verbs of Phases 9-12. Must complete before Phase 14. Retrofitting tiers onto the v1 write verbs breaks existing non-interactive scripts that pass only `--confirm`; record this as a decision. Exit code 6 is a one-way door.

### Phase 14: Story Management

**Goal**: An agent can create, update, comment on and link work items through the guard, and cannot reach state or effort through any of them
**Depends on**: Phase 13
**Requirements**: STORY-01, STORY-02, STORY-03, STORY-04, AGENT-05
**Success Criteria** (what must be TRUE):
  1. `workitems create` creates a work item, a child inherits area and iteration from its parent, the dry run uses the server's validate-only mode, and creating a Development task where one already exists is a no-op
  2. Comments on a work item can be read (paged) and posted in Markdown
  3. A link can be added with `--confirm` alone, while removing one is human-tier and checks the item revision first, aborting if it changed
  4. `workitems update` sets points, assignee and iteration and appends acceptance criteria below the existing text, and any attempt to set state, effort or another field outside the allow-list through create or update is refused before a request is sent

**Plans**: TBD
**Research**: Yes. PAT scopes for create, comment and link; the Markdown comment format; `/multilineFieldsFormat` on acceptance criteria (never set it; it cannot be undone)
**Compliance**: CPNI review. Comments read and post work item free text.
**Notes**: Tiers (Phase 13) must exist before any new agent-reachable write. Produces the write methods grooming reuses in Phase 16.

### Phase 15: Sprint Tab

**Goal**: A user can see every team's sprint health at a glance in the TUI and drill into a team, a person, or a risky item
**Depends on**: Phase 10, Phase 11
**Requirements**: SPRINT-01, SPRINT-02, SPRINT-03, SPRINT-04, SPRINT-05
**Success Criteria** (what must be TRUE):
  1. A Sprint tab shows points, pace and risk count per team on an elapsed-working-day-fraction axis, so teams with different cadences line up
  2. Drilling into one team shows per-person load
  3. Capacity appears only when labelled unreliable and changes no finding, pace figure or risk count
  4. Selecting a risk finding jumps to that work item
  5. A secondary key switches sub-views within the single Sprint tab, and no new top-level tab is added

**Plans**: TBD
**UI hint**: yes
**Research**: Skip. Existing Bubble Tea patterns
**Compliance**: CPNI review. Per-person load shows names against work.
**Notes**: Can run in parallel with Phases 13-14 (disjoint files). Grooming launches from this tab (SPRINT-05); the launch itself is verified in Phase 16. No auto-poll more often than every 5 minutes.

### Phase 16: Grooming Mode

**Goal**: A user can groom the backlog from the keyboard one item at a time, with every edit guarded and the backlog's real order respected
**Depends on**: Phase 7, Phase 9, Phase 11, Phase 14, Phase 15
**Requirements**: GROOM-01, GROOM-02, GROOM-03, GROOM-04, GROOM-05, GROOM-06
**Success Criteria** (what must be TRUE):
  1. Launched from the Sprint tab, grooming shows one item full screen and advances to the next using only the keyboard
  2. Points, assignee (chosen from the team's resolved members) and sprint can be edited inline, and acceptance-criteria text is appended below existing content without rewriting it
  3. Every grooming edit goes through the guard: edits to one item are confirmed once on advancing and leave one audit record per changed field with actor `tui`
  4. In the full-backlog view an item can be moved against real ADO rank: neighbours are re-read first, the move aborts if they changed, and the before and after neighbours are audited; the gap queue (items with NAG-02, NAG-03, NAG-04 or NAG-12 gaps) allows edits but no reordering
  5. Every list view labels whether it shows ADO rank or the personal `priorities.yaml` order, and the personal order keeps working

**Plans**: TBD
**UI hint**: yes
**Research**: Yes. Reorder semantics (`parentId`, the iteration backlog, the process-aware rank field); how the server treats HTML appended to acceptance criteria stored as Markdown
**Notes**: Uses the Phase 7 guard, Phase 9 team members and iterations, Phase 11 gap rules, Phase 14 write methods, and the Phase 15 launch point.

## Progress

**Execution Order:**
Phases execute in numeric order: 7 → 8 → 9 → 10 → 11 → 12 → 13 → 14 → 15 → 16. Phase 15 may run in parallel with Phases 13-14.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 7. Guarded TUI Writes | v2.0 | 5/8 | In Progress | - |
| 8. Platform Upgrade and Pacing | v2.0 | 0/TBD | Not started | - |
| 9. Team Foundation | v2.0 | 0/TBD | Not started | - |
| 10. Sprint Data Access | v2.0 | 0/TBD | Not started | - |
| 11. Board Honesty | v2.0 | 0/TBD | Not started | - |
| 12. Ceremony Output | v2.0 | 0/TBD | Not started | - |
| 13. Agent Authorization | v2.0 | 0/TBD | Not started | - |
| 14. Story Management | v2.0 | 0/TBD | Not started | - |
| 15. Sprint Tab | v2.0 | 0/TBD | Not started | - |
| 16. Grooming Mode | v2.0 | 0/TBD | Not started | - |
