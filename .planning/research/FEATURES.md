# Feature Research

**Domain:** Scrum-master tooling on Azure DevOps (sprint reports, standup digests, board-hygiene rules, grooming, agent write authorization), added to an existing Go TUI + agent CLI
**Researched:** 2026-10-13
**Confidence:** MEDIUM overall. SDK and codebase claims are HIGH (verified by reading `azuredevops@v1.0.0-b5`, `azuredevops/v7@v7.1.0` and this repo's source). ADO behavioural claims come from Microsoft Learn pages; the GSD classify-confidence seam tags all web sources LOW, so treat them as "official doc, not independently re-verified".

Scope: only the v2.0 additions (BL-01, TEAM, DATA, NAG, CEREMONY, SPRINT, GROOM, AGENT, STORY, PLAT-02/03). The design-session decisions (definitions table, five-field sprint report, capacity display-only, hardcoded rules, agent informs / TUI acts) are treated as fixed. This file validates them and names the gaps.

---

## Headline Findings

1. **"Delivered" contradicts the code it cites.** The definitions table says Delivered = Resolved or Closed, "matching `doneStates`". `doneStates` (now at `internal/ado/filters.go:9`, not `workitems.go:36`) also contains `Done` and `Removed`. Reusing `IsDoneState` for the sprint report would count removed items as delivered. Delivered needs its own predicate. ADO's own velocity widget also excludes Resolved-category states from "Completed", so the report will not reconcile with the ADO velocity chart unless that is documented.
2. **DATA-04 names the wrong API.** `ReadReportingRevisionsPost` is a forward stream of full-snapshot revisions from a start time, with no "as of" parameter. Rebuilding membership at the baseline from it means replaying every revision in the project from before the sprint. WIQL `ASOF '<instant>'` on a flat query, plus `WorkItemBatchGetRequest.AsOf` (present in the b5 SDK), gives membership and field values (points, state) at the baseline and close moments in two calls. The reporting-revisions stream is the right tool for `standup` (what changed since T), not for the baseline.
3. **Team scope is iteration plus area, and the requirements only model iteration.** An ADO team owns area paths (`work.GetTeamFieldValues`). Teams on a shared cadence (TEAM-05) usually share iteration nodes. If you filter by iteration path alone, every team's sprint contains every other team's items, and the multi-team rollup double-counts. `work.GetIterationWorkItems(project, team, iterationId)` returns team-scoped parent/child links in one call, which also satisfies DATA-03.
4. **NAG-06 names an API that runs the wrong way.** `git.GetPullRequestWorkItemRefs` maps PR to work items. The rule needs work item to PR, which is an `ArtifactLink` relation (`vstfs:///Git/PullRequestId/...`) on the work item. Each linked PR then needs its own status lookup. "Merged" also needs a target-branch rule: a PR merged into a feature branch is not delivery.
5. **AGENT tiers leave existing and new verbs unclassified.** `prs approve` (an existing v1 verb) and `STORY-03` link removal appear in neither AGENT-02 nor AGENT-03. Nothing says how the CLI tells an agent caller from a human one. If the tier comes from a flag, an env var or the config file, the agent can set it itself. That would be the same "flag used as designed" failure the section exists to prevent.
6. **BL-01 and GROOM pull against each other.** BL-01 adds a confirmation step to TUI writes. GROOM-01/02 exist to be faster than the web UI. A confirmation on every inline edit defeats grooming. Decide this once in the BL-01 discuss-phase: audit always, one confirmation per item on advance, not per field.

---

## Feature Landscape

### Table Stakes (users expect these; missing = tool not trusted)

| Feature | Req IDs | Why Expected | Complexity | Notes |
|---------|---------|--------------|------------|-------|
| Guarded, audited TUI writes | BL-01 | Every grooming edit is a new TUI write. Shipping five new unaudited write paths on top of the seven existing ones grows the debt BL-01 names | MEDIUM | Seven existing TUI call sites (`workitems.go:266,335,350,359`, `prs.go:361,384`, `releases.go:206,214`, `pipelines.go:205`) all use `context.Background()` with no deadline. They must reuse `cmd/adoBizInt` `runWrite` semantics (audit before send, outcome classification). Moving the guard below `cmd/` is the one extraction this needs |
| Team config + allow-list | TEAM-01 | Every scrum API is `(project, team)` scoped | LOW | Treat configured teams as an allow-list like HARD-04 projects. Unknown team means exit 3. Discovery (`core.GetAllTeams`) belongs in a read verb (`teams list`) that helps fill config, not in implicit auto-population |
| Current sprint per team | TEAM-03 | Every ceremony verb starts here | LOW | `GetTeamIterations(timeframe=current)` returns none between sprints or on gap days. That must be a reported condition, not an empty result (core value). `TeamIterationAttributes` dates are "date-only, midnight UTC". Finish is the last working day, inclusive |
| Team area scoping | (gap, not in reqs) | Without it, the rollup double-counts on shared iterations | LOW | `GetTeamFieldValues` or `GetIterationWorkItems` with team. Add to TEAM or DATA |
| Sprint read including done work | DATA-01 | Velocity, delivered and carryover are invisible today | MEDIUM | New path. Do not edit `workitems list`'s WIQL (`workitems.go:176`), whose `scope` disclosure is pinned. Read `scope` must disclose iteration, team areas and as-of instant |
| Effort fields read | DATA-02 | NAG-05 and NAG-09 have no data otherwise | LOW | Use `*float64` for all three. Nil must stay distinct from 0, or NAG-05 fires on unestimated tasks |
| Story points as nullable float | (gap) | NAG-02 "no story points" cannot tell missing from 0. `WorkItem.StoryPoints` is `int` (`workitems.go:23`), so 0.5 truncates to 0 and is reported as unestimated | LOW | Change the sprint read model to `*float64`. Leave the v1 `story_points` JSON contract alone |
| One query per team for tasks | DATA-03 | N+1 over a sprint hits rate limits (PLAT-03) | LOW | `GetIterationWorkItems` (links) then `GetWorkItemsBatch`. Batch is capped at 200 ids per call, so chunk |
| Baseline reconstruction | DATA-04 | Committed, scope change and re-pointed all need the line at start+48h | MEDIUM with ASOF / HIGH with reporting revisions | See Requirement Issues: switch to WIQL ASOF |
| Readiness rules | NAG-01..04, NAG-12 | The core board-honesty value | LOW each | NAG-03 needs `stripHTML` (currently private in `internal/ui/workitems/detail.go`). The second caller moves it to a shared spot. Also strip `&nbsp;` and empty `<div><br></div>` |
| State-lie rules | NAG-05..07 | Stale boards are pain #1 | LOW (05), HIGH (06), ? (07) | NAG-07 is not testable as worded (see below) |
| Slippage rules | NAG-08..10 | Early warning before review day | MEDIUM | Working-day math via `TeamSetting.WorkingDays`. Team days off (`GetTeamDaysOff`) are team-level holidays, not personal PTO. Ask whether they are excluded with capacity or used for working-day math |
| JSON findings | NAG-11 | Agent consumes them | LOW | Add a stable finding key `rule:item` so an agent can diff runs |
| `sprint report` | CEREMONY-01 | No structured report exists today. This is the highest-priority verb | HIGH | Five fields over two as-of instants. Depends on DATA-01..04 and TEAM-03 |
| `standup` | CEREMONY-02 | Manual prep is pain #2 | MEDIUM | "Moved" is undefined (see Issues). Reporting-revisions stream since T, filtered to team sprint IDs |
| `planning` | CEREMONY-03 | Ready vs not-ready with gaps named | MEDIUM | Needs backlog order (`GetBacklogLevelWorkItems`), not only gap detection |
| Cross-team rollup, drill to one team | TEAM-04, CEREMONY-05, SPRINT-01/02 | Multi-team is the primary mode | MEDIUM | Dedupe by item id in rollups. Overlapping area paths ("include sub-areas") put one item in two teams |
| Sprint tab | SPRINT-01..05 | TUI half of the posture | MEDIUM | Tab 7 plus a secondary key. Risk list jumps to the existing detail view |
| Grooming mode | GROOM-01..04 | The one place a terminal beats the web UI | HIGH | Inline edits, HTML acceptance criteria and reorder semantics all carry traps (see Issues) |
| Agent tiers enforced in the CLI | AGENT-01..04 | Confirmation flags do not bound intent | MEDIUM | Needs a caller-identity mechanism the agent cannot set itself |
| Create / comment / link | STORY-01..03 | NAG-12 remediation path and agent write surface | MEDIUM | Comments API is preview-versioned in both SDKs (`5.1-preview` in b5). Link removal is a by-index JSON Patch (`/relations/N`), which is racy |
| Backoff + fan-out caps | PLAT-03 | Multi-team fan-out multiplies calls | LOW (caps) / MEDIUM (header-aware backoff) | Neither SDK exposes response headers through typed clients. See dependency on PLAT-02 |

### Differentiators (where this tool beats the ADO web UI and Microsoft's MCP server)

Microsoft's own Azure DevOps MCP server already exposes iteration, capacity and work-item reads to agents, with toolset filtering and a binary read-only header. Raw sprint reads are commodity. This tool's edge is synthesis that can be trusted, and write authorization finer than read-only/read-write.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Reproducible sprint report | Every field is computed as of fixed instants (baseline = start+grace, close = end of finish day), so running the report next week gives the same numbers. ADO's own charts drift as items are edited later | MEDIUM | Disclose both instants in `scope`. Matches the core value: a reader can tell a recomputed number from a moved one |
| Scope change as three numbers | Added / removed / re-pointed kept separate. Jira and ADO fold these into a single "scope" line or an asterisk | MEDIUM | "Removed" must cover both moved out of the iteration and state set to Removed while still in it |
| "Insufficient data" verdicts | NAG-09 refuses to give a pace verdict when RemainingWork is nil on more than X% of sprint tasks, the same reasoning that made capacity display-only | LOW | Without it, NAG-09 repeats the capacity failure mode with hours |
| Machine-actionable remediation | NAG-11 `suggested_action` names the CLI verb the agent tier allows (e.g. NAG-12 leads to `workitems create --type Task --parent N`) | LOW | Only for rules whose fix is agent-tier. State-lie fixes stay human |
| Standup shows silent members | TEAM-02 membership lets standup list members with zero movement. A plain query cannot show absence | LOW | This is the real justification for TEAM-02. State it in the requirement |
| Field-level agent tiers | Points/assignee/sprint/comments allowed, state/effort/merge/approve denied. MCP offers only read-only or not | MEDIUM | Also restrict *fields on create*: an agent creating a task with `State=Closed` or effort values would bypass AGENT-03 |
| Agent-attributed audit | Audit record carries `actor: agent|human` and tier. In ADO History, agent writes made with the user's PAT are indistinguishable from the user's own | LOW | Optional comment prefix for agent-posted comments |
| Keyboard grooming queue | One-keystroke advance through readiness gaps, no page loads | HIGH | Already the stated rationale |

### Anti-Features (seem good, create problems)

The design session already excluded Kanban, charts, retros, staleness rules, capacity rules, blocked status and averaged velocity. The items below are new candidates that will come up during planning.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Readiness rules NAG-01 and NAG-12 on the whole backlog | NAG-13 says readiness runs in backlog scope | Backlog items are normally unassigned and have no tasks until sprint planning. Every backlog item fails NAG-01 and NAG-12, which is the noise the staleness rules were cut for | Run NAG-02, NAG-03 and NAG-04 on the backlog. Run NAG-01 and NAG-12 only on sprint items and on items already assigned to the next iteration |
| Free-text acceptance-criteria rewrite in the TUI | GROOM-02 lists AC as an inline edit | AC is HTML. Editing the stripped text and writing it back destroys lists, tables, links and images | Edit only when the field is empty after stripping, or append. Refuse (or warn and require a second keystroke) to overwrite non-trivial markup |
| Reusing `IsDoneState` for Delivered | It's the "single definition of done" | Includes Removed and Done, so the sprint report overstates delivery | Separate `delivered` predicate over Resolved and Closed, defined beside `doneStates` with a test pinning the difference |
| Title-matching "Development" tasks as truth | NAG-12 cites the existing heuristic (`findDevelopmentTask`, now `workitems.go:940`) | It matches titles ("Dev", "Development"). ADO's own field for this is `Microsoft.VSTS.Common.Activity = Development` | Keep the title heuristic (it's the team's convention) but check Activity first. Record which matched in the finding |
| Configurable tier in the config file / env var | Easy to implement | The agent has shell access and can edit both. That makes the tier a flag again | Default every CLI invocation to agent tier. Human-only verbs require an interactive TTY confirmation (typed item id) that a non-interactive agent shell cannot answer. Document that this holds against cooperative agents only. A PAT cannot separate comments from state changes (both are `vso.work_write`) |
| Per-keystroke confirmation in grooming | BL-01 literal reading | Destroys the grooming speed advantage | Stage edits per item, confirm once on advance, audit every field |
| Hardcoding `User Story` / `StoryPoints` / Agile states | Matches current defaults (`DefaultStateTransitions`) | Scrum-process projects use `Product Backlog Item`, `Effort` and `Done`. Points and Delivered silently read as zero | Assert the process template per project at startup and fail loudly on non-Agile. Do not abstract it (no speculative generality) |
| Proactive throttling on `X-RateLimit-Remaining` | Docs recommend it | Typed SDK clients never surface response headers in b5 or v7.1.0 | Fan-out semaphore plus retry on 429 (the status is visible in the SDK's `WrappedError`). Header-aware pacing only after PLAT-02 with a custom `http.Client` |

---

## Requirement Issues (ambiguous, untestable, conflicting)

Ordered by blast radius. Each one needs a decision in REQUIREMENTS.md before its phase plans.

| ID | Problem | Type | Recommended resolution |
|----|---------|------|------------------------|
| Definitions: Delivered | Cites `doneStates` "matching" Resolved or Closed. `doneStates` = {Done, Closed, Resolved, Removed}. Also diverges from ADO velocity (Resolved excluded) | Conflict | New predicate. State in the requirement that numbers intentionally differ from ADO's velocity widget |
| Definitions: Committed | One of the five report fields but missing from the definitions table. Count or points? Items present at baseline, or at start? | Ambiguous | Committed = points (and count) of requirement-level items in the team's sprint as of the baseline instant |
| Definitions: Delivered timing | "Delivered" is undefined as of when. Items closed the day after finish are "Completed Late" in ADO | Ambiguous | Delivered = in sprint *and* in a delivered state as of close instant. Optionally emit late-delivered count later, if demand proves it |
| CEREMONY-01 "per-sprint points" vs Velocity | Velocity = raw points delivered per sprint. "Per-sprint points" reads as the same thing, which would make it a duplicate of Delivered | Ambiguous | Either define per-sprint points as the velocity series over the last N sprints (N caller-supplied) or drop it as duplicate |
| Carryover for the current sprint | Requires sprint N+1 membership, which does not exist until N closes | Untestable mid-sprint | Report carryover only for closed sprints. For the current sprint emit `null` with reason, not 0 |
| DATA-04 API | `ReadReportingRevisionsPost` has no as-of. Reconstruction means a full project replay | Wrong tool | WIQL `ASOF` (flat queries only; ASOF is incompatible with tree queries) plus batch get with `AsOf`. Move reporting revisions to CEREMONY-02 |
| TEAM-05 shared cadence | Stated as an assumption with no check. If false, SPRINT-01 silently misaligns | Untestable as an assumption | Make it a runtime check: compare current-iteration dates across teams and emit a failure/finding on mismatch. Better: plot SPRINT-01 on an *elapsed working-day fraction* axis per team, which removes the dependency entirely |
| Team scope (missing) | No requirement for team area paths | Gap | Add TEAM-06: sprint and backlog membership = team iteration ∩ team field values; rollups dedupe by item id |
| NAG-06 | API direction wrong. "Merged" is undefined for non-default branches. Each linked PR is a separate lookup | Ambiguous + cost | Read ArtifactLink relations from the sprint batch (expand Relations), then one PR read per linked PR, capped by PLAT-03. Merged = status completed into the repo's default branch |
| NAG-07 | "State is terminal per StateTransitions … but never advanced to its final state" contradicts itself: a terminal state *is* final | Untestable | Pick one: (a) item in a non-final done-category state (e.g. Resolved) at sprint close; or (b) parent story Active while every child task is terminal. (b) is the common board-hygiene rule and needs no history |
| NAG-09 | "Burn pace implies the sprint misses": no model given. Hours depend on RemainingWork being maintained, the same weakness that made capacity display-only | Ambiguous | Linear ideal-line comparison: remaining-hours fraction vs remaining working-day fraction, with a tolerance threshold. Return an "insufficient data" finding below a RemainingWork coverage threshold |
| NAG-10 | Which PRs? Every PR in team repos, or PRs linked to sprint items? | Ambiguous | PRs linked to sprint items (shares NAG-06's link read). Team-repo mapping does not exist in config |
| NAG-13 | Backlog scope undefined (whole backlog? top N?). NAG-01 and NAG-12 on backlog = noise | Ambiguous + conflict with the anti-noise principle | Backlog = team backlog at the requirement level, not done, via `GetBacklogLevelWorkItems`, capped at top N (threshold). Exclude NAG-01 and NAG-12 per Anti-Features |
| CEREMONY-02 | "What moved" is undefined (state change? remaining burned? any edit?). "Grouped by person": assignee now, or whoever made the change? Default for "since"? | Ambiguous | Moved = state transition or RemainingWork decrease since T, grouped by current assignee, plus members with nothing moved. `--since` is required (no default) so a Monday standup is explicit |
| CEREMONY-03 | "Ready to pull" with no ordering or cut line | Ambiguous | Backlog order, each item tagged ready/not-ready with gaps, plus the raw velocity series for the caller to judge. No capacity input (consistent with SPRINT-03) |
| SPRINT-01 "pace" | Undefined. Points or hours? Same as NAG-09? | Ambiguous | Pace = the NAG-09 computation, shown; risk count = NAG findings count. One definition, two surfaces |
| GROOM-03 vs GROOM-04 | The grooming queue is filtered to gap items, while reorder works on the full backlog. "Move up one" in a filtered list has no defined meaning for `ReorderOperation{PreviousId, NextId}` | Conflict | Reorder only in a full-backlog view (gap markers inline). The gap queue is edit-only. Sprint-backlog reorder also needs `IterationPath` set |
| GROOM-02 acceptance criteria | HTML round-trip destroys formatting | Data-loss risk | See Anti-Features |
| BL-01 vs GROOM-01 | Confirmation on every edit vs speed | Conflict | Decide in the BL-01 discuss-phase: per-item staged confirm, audit per field |
| AGENT-02/03 coverage | `prs approve` and link removal unclassified. Create-with-fields could set State or effort | Gap | Default-deny: any verb not on the AGENT-02 list is human-only. Restrict create to an allow-listed field set (title, type, parent, description, AC, points, assignee, iteration) |
| AGENT-02 sprint assignment | An agent moving items between iterations changes CEREMONY-01 scope numbers | Integrity | Allowed, but audited with actor. Scope-change output can attribute moves to agent actors if demand appears (do not build now) |
| AGENT-04 caller identity | No mechanism defined to tell agent from human | Untestable | Non-interactive invocation = agent tier. Human-only verbs require TTY-interactive confirmation. Test: running a human-only verb with stdin not a TTY plus `--confirm` exits with a dedicated code and writes nothing |
| PLAT-03 vs PLAT-02 | Header-aware backoff is impossible through b5's typed clients. v7.1.0 adds `NewClientWithOptions(WithHTTPClient)` but typed `NewClient` doesn't use it, so you would construct `ClientImpl{Client: ...}` by hand | Hidden dependency | Ship caps + 429 retry first (SDK-independent). Header-aware pacing after PLAT-02 |
| Stale line references | Reqs cite `workitems.go:36` (now `internal/ado/filters.go:9`), `:881` (now `:940`), `workitems.go:104` (now `:176`), `:249-251` (now around `:341`), `config.go:53` (`StateTransitions` field now `:52`, defaults `:76`) | Hygiene | Replace line numbers with symbol names (`doneStates`, `findDevelopmentTask`, `getProjectWorkItems`, `Config.StateTransitions`) |

---

## Feature Dependencies

```
BL-01 (TUI writes through guard/audit)
    └──required by──> GROOM-02, GROOM-03 (new TUI writes)

PLAT-03 caps + 429 retry ──required by──> TEAM-04 fan-out (teams × projects × batches)
PLAT-02 SDK v7 ──enables──> PLAT-03 header-aware pacing (optional)

TEAM-01 config/allow-list
    └──> TEAM-03 current iteration ──> TEAM-06* team area scope
              └──> DATA-01/02/03 sprint read (GetIterationWorkItems + batch, nullable effort/points)
                        ├──> DATA-04 baseline (WIQL ASOF + batch AsOf)
                        │        └──> CEREMONY-01 sprint report
                        ├──> NAG-01..05, 07, 08, 12 (no history needed)
                        ├──> NAG-06, NAG-10 (relations + PR status)
                        ├──> NAG-09 (working days + RemainingWork coverage)
                        └──> CEREMONY-02 standup (reporting revisions since T)
TEAM-02 membership ──enhances──> CEREMONY-02 (silent members), SPRINT-02 (per-person load)

NAG-13 backlog scope (GetBacklogLevelWorkItems, team-scoped)
    ├──> CEREMONY-03 planning
    └──> GROOM-04 grooming queue ──> GROOM-01/02
                 GROOM-03 reorder ──conflicts──> GROOM-04 filtered queue (resolve: separate views)

NAG-11 findings ──> SPRINT-04 navigable risks, SPRINT-01 risk count
AGENT-01..04 tiers ──required by──> STORY-01..03 shipping on the CLI
STORY-01 create ──enables──> NAG-12 remediation (agent creates the Development task)

* TEAM-06 is the recommended new requirement for area-path scoping.
```

### Dependency Notes

- **AGENT tiers before STORY verbs.** STORY adds the first agent-tier writes. Shipping them before the tier gate means they ship as v1-style "confirm = allowed" writes, which is what AGENT exists to prevent.
- **BL-01 before GROOM.** Each grooming edit is a new TUI write. Adding them before BL-01 grows the unaudited surface from seven paths to twelve.
- **DATA before NAG and CEREMONY.** Every rule and verb reads through the sprint path. NAG-05, NAG-09 and points-based fields read wrong until effort and points are nullable floats.
- **Team area scoping before any rollup.** Without it, TEAM-04 rollups double-count on shared iterations. That error is invisible because the numbers still look plausible.
- **Existing code reused:** `ado.Result[T]` envelope with `failures`/`scope`, the `runWrite` guard/audit path, `Config.StateTransitions`, `findDevelopmentTask`, `stripHTML`, `iterationClause` (WIQL quoting), the dormant `work.Client` at `internal/ado/client.go:30`, the existing detail view for SPRINT-04 jumps.

---

## MVP Definition

### Launch With (v2.0 core: the two CLI pains first)

- [ ] BL-01: TUI writes through guard/audit. Prerequisite for any new TUI write; already scheduled first
- [ ] TEAM-01/03 plus team area scoping (new TEAM-06): nothing is correct without `(project, team)` scope
- [ ] DATA-01/02/03 with nullable points and effort: data source for every rule
- [ ] NAG-01..05, 08, 11, 12 (sprint scope): cheap, no history, answer pain #1 (stale board)
- [ ] DATA-04 via ASOF, then CEREMONY-01 `sprint report`: highest-priority ceremony output
- [ ] CEREMONY-02 `standup`: pain #2
- [ ] PLAT-03 fan-out caps + 429 retry: multi-team fan-out needs it from the first rollup
- [ ] AGENT-01..04 then STORY-01/02: agent remediation for NAG-12

### Add After Core Works (same milestone, later phases)

- [ ] SPRINT-01..05 Sprint tab: once findings and report numbers are trusted
- [ ] NAG-06, NAG-10 PR linkage: costlier (per-PR lookups). Add once caps exist
- [ ] NAG-07 (after the wording is fixed), NAG-09 (with the insufficient-data guard)
- [ ] NAG-13 backlog scope, CEREMONY-03 `planning`, GROOM-01..04: backlog order and grooming share `GetBacklogLevelWorkItems`
- [ ] STORY-03 links (after the tier decision for removal)
- [ ] TEAM-02 membership (silent members in standup, per-person load in SPRINT-02)

### Future Consideration

- [ ] Header-aware rate pacing: after PLAT-02 makes a custom `http.Client` possible
- [ ] Late-delivered count, agent-attributed scope moves: only on demonstrated need, per the "fields earn their way in" rule
- [ ] PLAT-01 MCP: explicitly out of scope

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| BL-01 guarded TUI writes | HIGH | MEDIUM | P1 |
| TEAM-01/03 + area scoping | HIGH | LOW | P1 |
| DATA-01..03 (nullable fields) | HIGH | MEDIUM | P1 |
| Readiness + simple state rules (NAG-01..05, 08, 11, 12) | HIGH | LOW | P1 |
| DATA-04 via ASOF + `sprint report` | HIGH | MEDIUM | P1 |
| `standup` | HIGH | MEDIUM | P1 |
| PLAT-03 caps + 429 retry | MEDIUM | LOW | P1 |
| AGENT tiers | HIGH | MEDIUM | P1 |
| STORY-01/02 create + comment | MEDIUM | MEDIUM | P1 |
| Sprint tab | MEDIUM | MEDIUM | P2 |
| NAG-06/10 PR linkage | MEDIUM | HIGH | P2 |
| NAG-07, NAG-09 | MEDIUM | MEDIUM | P2 (after rewording) |
| NAG-13 + `planning` | MEDIUM | MEDIUM | P2 |
| Grooming mode | HIGH | HIGH | P2 (after BL-01) |
| STORY-03 links | LOW | MEDIUM | P2 |
| TEAM-02 membership | MEDIUM | LOW | P2 |
| PLAT-02 SDK upgrade | MEDIUM (security) | MEDIUM | P2 (gated on govulncheck) |
| Header-aware pacing | LOW | MEDIUM | P3 |

---

## Competitor Feature Analysis

| Feature | Jira Software | ADO web (Boards + Analytics widgets) | Microsoft Azure DevOps MCP server | Our Approach |
|---------|---------------|-------------------------------------|-----------------------------------|--------------|
| Sprint report | Completed / not completed / removed. Items added after start marked `*`. "Committed" = estimates present at start | Velocity: Planned / Completed / Completed Late / Incomplete. Resolved excluded from Completed | Raw iteration reads only | Five fields as of fixed instants. Scope change as three numbers. JSON only, no charts |
| Scope change | Asterisk per issue; one burndown "scope change" line | Burndown scope line | None | Added / removed / re-pointed, never merged |
| Standup digest | Marketplace bots (e.g. Geekbot-style) collect answers; no movement-derived digest | None built in | None | Derived from revisions since T, grouped by person, silent members listed |
| Board hygiene | Marketplace apps; no built-in nag list | Board "health" via queries the team builds by hand | None | 13 hardcoded rules, thresholds configurable, JSON findings with remediation verb |
| Grooming | Backlog drag-and-drop, page per issue detail | Backlog drag-and-drop, page load per item | Create/update tools | Keyboard single-item queue driven by readiness gaps |
| Agent write control | API token scopes | PAT scopes (`vso.work_write` covers all work-item writes) | Toolset filter + binary read-only header | Field-level tiers enforced in the CLI, default-deny, actor recorded in audit |

---

## Compliance Note

AGENT-03 keeps release approval and PR merge human-only, and BL-01 puts every TUI write in the audit log. Together these support change-management segregation of duties and audit-evidence expectations (SOX ITGC). Keep both requirements intact. Do not weaken AGENT-03 to "confirm-gated" for convenience. Standup and per-person load output carries employee names and work-item text only. Work-item descriptions and comments may contain customer details, so agent-posted comments (STORY-02) should not copy content between work items wholesale. Flag any phase that does so for CPNI review.

---

## Sources

Seam confidence for every web source below is LOW (per `classify-confidence`). They are official Microsoft or Atlassian documentation, cross-checked where possible against SDK source.

- Codebase (HIGH, direct inspection): `internal/ado/filters.go`, `internal/ado/workitems.go`, `internal/ado/client.go`, `internal/config/config.go`, `internal/ui/workitems/workitems.go`, `cmd/adoBizInt/dispatch.go`
- SDK source (HIGH, direct inspection): `github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5` (work, workitemtracking, core, git clients; `ReadReportingRevisionsPostArgs`, `WorkItemBatchGetRequest.AsOf`, `ReorderOperation`, `TeamIterationAttributes`, `GetIterationWorkItems`, `GetBacklogLevelWorkItems`, `GetWorkItemTypeStates`) and `azuredevops/v7@v7.1.0` (`Connection`, `NewClient`, `NewClientWithOptions`)
- [WIQL syntax reference, ASOF clause](https://learn.microsoft.com/en-us/azure/devops/boards/queries/wiql-syntax)
- [Azure DevOps rate and usage limits](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits)
- [Team velocity definitions (Planned / Completed / Completed Late / Incomplete)](https://learn.microsoft.com/en-us/azure/DevOps/report/dashboards/team-velocity?view=azure-devops)
- [Jira: committed bar in velocity chart](https://support.atlassian.com/jira/kb/the-grey-commited-bar-is-not-showing-for-sprint-via-velocity-chart/)
- [Azure DevOps remote MCP server (toolsets, X-MCP-Readonly)](https://learn.microsoft.com/azure/devops/mcp-server/remote-mcp-server)
- [microsoft/azure-devops-mcp](https://github.com/microsoft/azure-devops-mcp)
- [azure-devops-go-api releases (v7.1.0 latest)](https://github.com/microsoft/azure-devops-go-api/releases)
- [Link type reference (ArtifactLink)](https://learn.microsoft.com/ro-ro/azure/devops/boards/queries/link-type-reference?view=azure-devops)

---
*Feature research for: Azure DevOps scrum-master capabilities (adoBizInt v2.0)*
*Researched: 2026-10-13*
