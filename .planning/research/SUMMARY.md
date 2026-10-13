# Project Research Summary

**Project:** adoBizInt, milestone v2.0 Scrum Master
**Domain:** Scrum-master tooling (team and sprint reads, board-honesty rules, ceremony synthesis, grooming, agent write tiers, rate limits, SDK upgrade) added to an existing Go Azure DevOps TUI and agent CLI
**Researched:** 2026-10-13
**Confidence:** MEDIUM overall. Stack and codebase integration are HIGH because the SDK upgrade and govulncheck were trialled in a scratch copy. ADO service behaviour is MEDIUM or LOW: it comes from official Microsoft Learn pages that have not yet been confirmed with live calls.

## Executive Summary

v2.0 adds a synthesis layer to a working v1.0 foundation, which provides the `Result[T]` envelope, guarded and audited CLI writes, and narrow `internal/ado` interfaces. The work splits cleanly. Fetch one sprint snapshot per team through `internal/ado`. Evaluate it with pure functions in a new `internal/scrum` package. Send every write through one guard body, moved out of `cmd/` so the TUI can call it too. All four researchers converged on this shape, and none of it needs a new module. Rate pacing is a stdlib `RoundTripper`, HTML stripping extends the existing `stripHTML`, and the AC editor is `bubbles/textarea`, which is already in the module graph.

The main risk is a new kind of failure, one layer above the one v1.0 fixed. v1.0 stopped broken calls from looking empty. In v2.0 the calls succeed but can be aimed at the wrong target:
- the project's default team, when `Team` is nil
- every team's items, when teams share an iteration path
- `StoryPoints`, on a Scrum-process project
- a sprint end shifted by the midnight-UTC date format
- an absent value read as `0`
- a "Delivered" set that includes `Removed`

Each of these returns `complete: true` and a wrong answer, and the envelope cannot catch it. The mitigations:
- Team is a required, non-pointer parameter.
- Team area paths scope every read, applied the same way at every instant.
- Every scrum verb has a `scope` block that states the team, area paths, points field, done set and as-of instants.
- Every verb has negative tests for no teams, no current sprint, an empty sprint, and one team failing.

Several requirements as written name the wrong API or cannot be tested:
- DATA-04 names `ReadReportingRevisionsPost`, which cannot answer "membership at instant T". WIQL `ASOF` plus batch `AsOf` can.
- NAG-06 names an API that maps PRs to work items, the opposite of the direction the rule needs.
- NAG-07 contradicts itself.
- TEAM-01 names an org-wide preview API.
- AGENT-02/03 leave `prs approve`, reorder and unlink unclassified, and do not say how the CLI tells an agent caller from a human.
- PLAT-02's govulncheck gate cannot pass on Go 1.25 whatever the SDK version.

REQUIREMENTS.md needs about 28 amendments and 9 user decisions, all listed below, before it is final.

## Key Findings

### Recommended Stack

No new modules. Two version bumps and one toolchain bump. Details: [STACK.md](STACK.md).

**Core technologies:**
- **`azure-devops-go-api/azuredevops/v7` v7.1.0** (replaces `v1.0.0-b5`). This is the newest tag; there is no v8. Pin the tag, not a pseudo-version.
  - Trial: an import-path rewrite across 24 files builds, vets and passes `go test ./...` with no code changes.
  - Why upgrade:
    - `NewClientWithOptions(WithHTTPClient)`, the only clean way to install a rate-limit transport.
    - 7.1 API versions instead of the 5.1-preview versions, which are subject to deactivation (GetAllTeams, Comments, ReorderBacklogWorkItems).
    - `AddWorkItemComment` with `Format: Markdown`.
- **Go toolchain 1.27** (`go 1.27.0` directive, CI on 1.27.2, `go-version-file: go.mod`). **This, not the SDK, is what fixes govulncheck.** The SDK has no entries in the Go vulnerability database. On Go 1.25.4 govulncheck reports 26 stdlib vulnerabilities the code calls, and some are fixed only in 1.26.9+ and 1.27.2. With Go 1.27.2, x/sys v0.47.0 and v7.1.0 the result is "No vulnerabilities found" (trial).
- **`golang.org/x/sys` v0.47.0** (indirect dependency, from v0.38.0). Fixes GO-2026-5024. It is the newest release that still declares `go 1.25.0`.
- **govulncheck v1.9.0**, run via `go run ...@v1.9.0` in CI and not added to `go.mod`. It needs Go 1.26 or later to run.
- **Stdlib only for the new mechanics:**
  - `html.UnescapeString` and `html.EscapeString`
  - a pacer built on `net/http`'s `RoundTripper`
  - `testing/synctest` for deterministic pacer tests
  - `bubbles/textarea` (already present) for AC edits

**Explicitly rejected:**
- `golang.org/x/time/rate`: ADO publishes no fixed rate to configure.
- `go-retryablehttp` and `cenkalti/backoff`: they retry writes.
- `errgroup`: cancel-on-first-error conflicts with partial results.
- `x/net/html` and bluemonday: not needed.
- Migrating to bubbletea v2: breaks "TUI unchanged".
- Swapping the global `http.DefaultTransport`.

### Expected Features

Details: [FEATURES.md](FEATURES.md). The design-session decisions hold, subject to the amendments below: the definitions table, the five-field report, display-only capacity, hardcoded rules, and "agent informs, TUI acts".

**Must have (table stakes):**
- BL-01 guarded and audited TUI writes. There are 9 existing call sites, all on `context.Background()`, and grooming adds 5 more.
- Team config as an allow-list, current sprint per team, and **team area-path scoping**. Area scoping is missing from the requirements, and without it rollups double-count.
- A sprint read that includes done work, with nullable points and effort (`*float64`).
- Baseline via ASOF, then `sprint report`, the highest-priority ceremony.
- The cheap readiness and state rules (NAG-01..05, 08, 11 and 12) and `standup`.
- Agent tiers enforced in the CLI before any new agent-reachable write ships, then create and comment.
- Fan-out caps and 429 handling from the first multi-team rollup.

**Should have (differentiators against the ADO web UI and Microsoft's ADO MCP server):**
- A reproducible sprint report: every field is computed at a fixed instant, and both instants are disclosed in `scope`.
- Scope change as three numbers: added, removed, re-pointed.
- "Insufficient data" verdicts, for example NAG-09 when RemainingWork coverage is low.
- `suggested_action` names the CLI verb the agent tier may run. For NAG-12 that is `workitems create --parent`.
- Standup lists silent members. This is the real reason TEAM-02 exists.
- Field-level agent tiers with default-deny, and the actor recorded in the audit log.
- A keyboard-driven grooming queue.

**Defer:**
- Later in v2.0, after the core: Sprint tab, NAG-06/10 PR linkage, NAG-07/09 (after rewording), NAG-13 with `planning` and grooming, STORY-03, TEAM-02.
- After v2.0: late-delivered count, attributing scope moves to agent actors, PLAT-01 MCP (out of scope).

### Architecture Approach

The pattern is "fetch once, evaluate purely". Details: [ARCHITECTURE.md](ARCHITECTURE.md).
- **Reads.** `ado.GetSprintSnapshot(ctx, teams, opts)` returns `Result[SprintSnapshot]` with one item per team. Each sub-read records its own `Failure`. Pure `scrum.Nag/Report/Standup/Planning(snapshot, thresholds, now)` functions then map it through `mapResult`, which copies `Failures`, `Complete` and `Scope` verbatim. Four verbs use `mapResult`, which justifies extracting it.
- **One read path for both surfaces.** The CLI and the TUI Sprint tab call the same two functions.
- **Writes.** Both surfaces call `guard.Run(ctx, op, confirm) Report`. The guard never writes stdout and never exits; each surface maps the outcome to an exit code or a status line.
- **Tiers.** A tier is table data on the command map, checked once in `dispatch()`.

**Major components:**
1. **`internal/guard`** (new; moved out of `cmd/adoBizInt/write.go`, `audit.go` and `exit.go`). It handles audit open, plan and diff, no-op refusal, the confirm gate, the mutate step under recover, outcome classification and audit append. `internal/ui/fetchctx` moves to `internal/fetchctx`.
2. **`internal/ado` additions.**
   - `teams.go`: teams, members, iterations, settings, team field values.
   - `sprint.go`: snapshot and ASOF baseline. Split `baseline.go` out if the file passes 300 lines.
   - `stories.go`: create, comment, link, field update, reorder.
   - `transport.go`: the pacer.
   - Narrow `workAPI` and `coreAPI` interfaces.
   - `findDevelopmentTask` and `stripHTML` are promoted, and a new `IsDelivered` sits beside `IsDoneState`.
3. **`internal/scrum`** (new, pure): `nag.go`, `report.go`, `standup.go`, `planning.go`. Table-tested with no fakes, with `now` injected.
4. **`internal/config/scrum.go`** (new): `Teams` per project, a thresholds block, and validation.
5. **`internal/ui/sprint`** (new) plus `groom/`: Tab 7 with rollup, team drill, findings navigation, and grooming launched from the tab.
6. **`cmd/adoBizInt`.**
   - The command map entries become `command{run, tier}`.
   - New files: `tier.go`, and `sprint.go` and `story.go` for the new verbs.
   - `write.go` shrinks to parse, resolve, `guard.Run`, render and exit mapping.

### Critical Pitfalls

Details: [PITFALLS.md](PITFALLS.md). Ranked by "can it produce a confident wrong answer?"

1. **Wrong team, silently.** Several APIs fall back to the project's default team when `Team` is nil, and shared iteration paths pull in every team's items.
   - Make team a required, non-pointer parameter.
   - Validate configured teams against `core.GetTeams`; an unknown team exits 3.
   - Filter area paths client-side using `GetTeamFieldValues`, with the same filter at every instant.
   - Emit an overlap finding for items claimed by two teams.
   - Disclose the team and area paths in `scope`.
2. **Missing values read as zero, and the process template is hardcoded.** `int(points)` cannot tell absent from 0 and truncates 0.5 to 0. A reader that only knows StoryPoints returns nil on Scrum and Basic projects, which use Effort, Done and BacklogPriority.
   - Use `*float64` for points and effort on the new sprint type, and leave the v1 `story_points` contract alone.
   - Assert the process and fields per project at startup, and fail with a named config failure.
3. **Baseline built on the wrong API.** `ReadReportingRevisionsPost` has no as-of parameter and no iteration filter, so it means replaying the project's full history.
   - Use WIQL `ASOF '<UTC Z>'` with `TimePrecision: true`, plus `GetWorkItemsBatch{AsOf}`.
   - Keep reporting revisions for `standup` only: loop until `isLastBatch`, and treat an empty continuation token as a failure.
4. **Midnight-UTC dates and empty sprints.** Sprint dates are civil dates and `FinishDate` is inclusive, so calling `.Local()` ends the sprint a day early west of UTC. Null dates and "no current iteration" both come back as successful 200 responses.
   - Do civil-date math in UTC.
   - Treat null dates and an empty current iteration as per-team failures.
   - Run tests under `TZ=America/Chicago` and `TZ=Asia/Kolkata`.
5. **Guard bypass and tier bypass.** A new TUI write added without the guard reintroduces BL-01. A tier the agent can set through a flag, env var or config file is not a boundary.
   - Add a structural test that no `internal/ui` file calls an ado mutation method.
   - Enforce a field allow-list at the write layer.
   - Gate human-only verbs on an interactive TTY.
   - Document honestly that this only constrains a cooperative agent.

Also high-impact:
- **The SDK upgrade compiles cleanly, but every wire API version changes.** A half-migration with b5 and v7 both present silently reclassifies 401s as `other`. Swap in one commit, add a test that a v7 401 classifies as `auth`, and make a live smoke run the gate.
- **Rate limiting shows up first as delayed 200 responses, not 429s,** so it gets misread as timeouts.

## Implications for Roadmap

### Consolidated Build Order

All four researchers agree on this dependency chain:

**BL-01 → PLAT-02/03 transport → TEAM → DATA → NAG → CEREMONY → AGENT → STORY → Sprint tab → GROOM**

The hard edges:
- BL-01 before any new TUI write.
- AGENT before any new agent-reachable write.
- PLAT-02 before PLAT-03 header pacing, because the `http.Client` injection needs v7.
- TEAM before DATA, and DATA before NAG and CEREMONY.
- NAG before `planning` and GROOM-04.
- STORY write methods before GROOM edits and reorder.

### Phase 1: Guarded TUI Writes (BL-01)
**Rationale:** The user scheduled this first. Every later write (STORY, tiers, GROOM edits and reorder) must land on one guard. Building those writes first would mean writing each one twice.
**Delivers:**
- `internal/guard`, and `internal/fetchctx` moved out of `internal/ui`.
- All 9 TUI write call sites go through `guard.Run`, with the real project and bounded contexts.
- The compound resolve becomes two sequential guarded writes.
- A structural no-bypass test and an end-to-end modal test.
**Addresses:** BL-01. Decisions 4 and 9 must be made here.
**Avoids:** Pitfall 14. Lock the CLI write tests first and prove the extraction is byte-identical, as in D-17. The guard never prints to stdout.

### Phase 2: Platform: Toolchain, SDK v7, Transport (PLAT-02 + PLAT-03 transport)
**Rationale:** Now is the cheapest time. The trial was mechanical, and no new code has been written yet against b5 types or a third (`core`) client. One constructor rewrite serves both PLAT-02 and PLAT-03, and the live smoke also exercises the new TUI guard path.
**Delivers:**
- Toolchain and dependencies: the Go 1.27 directive, x/sys v0.47.0, a single-commit import swap to v7.1.0, and a govulncheck step in CI.
- Explicit sub-client construction through `NewClientWithOptions`:
  - set `hc.Timeout` explicitly
  - validate the `LocationUrl` host before sending the PAT
  - normalise URLs
  - add the `core` client
- The pacer `RoundTripper`.
- A test that a v7 401 classifies as `auth`.
- A live smoke run: one read per sub-client, plus a dry run of each write.
- Updated line references in `result.go`.
**Uses:** STACK.md core items 1–3. **Avoids:** Pitfalls 9 and 13.

### Phase 3: Team Foundation (TEAM-01..05, new TEAM-06, fan-out shape caps)
**Rationale:** Every scrum API is `(project, team)` scoped, so nothing later can be keyed without this.
**Delivers:**
- `config.Teams` as an allow-list.
- A `teams list` discovery verb using `core.GetTeams`, paged.
- Team membership, with group members flagged and identities keyed by id.
- Current iteration per team, plus `GetTeamSettings` (working days) and `GetTeamFieldValues` (area scope).
- Process and field validation.
- `Failure.Team`, with the contract test updated in the same change.
- A `TeamRef` type threaded end to end, and a `--team P/T` flag.
- Shape caps validated before the first call: team ceiling, `--sprints` maximum, and `backlog_depth` disclosed in `scope`.
- A cadence-mismatch check instead of an assumption.
**Avoids:** Pitfalls 1, 2, 4, 12, 22 and 23.

### Phase 4: Sprint Data Access (DATA-01..04)
**Rationale:** Data comes before rules. Shipping `sprint items` on its own lets live data be checked before anything is built on it.
**Delivers:**
- `GetSprintSnapshot`:
  - WIQL filtered on `[System.IterationId]`, with no state filter
  - a client-side area filter
  - 200-ID batches, with requested IDs compared against returned IDs
  - grouping by `System.Parent`, with no `$expand`
  - nullable points and effort
  - an ASOF baseline at start + grace
- `IsDelivered`, separate from `IsDoneState`.
- `stripHTML` promoted and extended: entity unescape and NBSP handling.
- `findDevelopmentTask` promoted.
- The `sprint items` verb.
**Avoids:** Pitfalls 3, 6, 10 and 11. **Spike first.**

### Phase 5: Board Honesty (NAG-01..13)
**Rationale:** These are pure functions over Phase 4's output, and `planning` and GROOM-04 reuse the readiness rules.
**Delivers:**
- `scrum/nag.go`, the `sprint nag` verb, `mapResult` and the thresholds.
- Stable finding keys of the form `rule:item`, and `suggested_action`.
- Backlog scope through `GetBacklogLevelWorkItems`, with depth disclosed.
- NAG-06/10 as a later plan within this phase.
**Avoids:** Pitfalls 7, 8 and 16. A rule whose input is missing is skipped and recorded, never run against empty input.

### Phase 6: Ceremony Output (CEREMONY-01..05)
**Rationale:** The report needs the Phase 4 baseline, and planning needs the Phase 5 rules. The report ships first.
**Delivers:**
- `sprint report`: five fields, with the delivered definition and both instants in `scope`.
- `standup`: reporting revisions since a required `--since`, plus silent members.
- `planning`: backlog order with ready/not-ready and gaps.
**Avoids:** Pitfalls 5 and 21. No synthetic "ALL" rows.

### Phase 7: Agent Authorization (AGENT-01..04)
**Rationale:** This must land before the first new agent-reachable write. It retrofits tiers onto the v1 verbs, which breaks non-interactive scripts; record that as a decision.
**Delivers:**
- `command{run, tier}` and the gate in `dispatch()`.
- `tier.go`: TTY check plus a typed target id.
- A field allow-list at the write layer.
- A table test that every write verb has a non-read tier.
- PAT-scope guidance for an agent profile.
**Avoids:** Pitfall 20.

### Phase 8: Story Management (STORY-01..03 + `workitems update`)
**Rationale:** Needs the guard (Phase 1) and tiers (Phase 7). It produces the write methods GROOM reuses.
**Delivers:**
- `create`:
  - inherits area and iteration from the parent
  - uses `validateOnly` as the dry run
  - is a no-op when the Development task already exists
  - accepts only an allow-listed set of fields
- `comment` via `AddWorkItemComment` with Markdown, with paged reads.
- `link` and `unlink`, sending `test /rev` before `remove`.
- `update` for points, assignee, iteration and AC.
- `UpdateWorkItemFields` and `ReorderBacklogItem`.

Phases 7 and 8 may merge if tiers come first. Never invert them.
**Avoids:** Pitfalls 15, 18 and 19.

### Phase 9: Sprint Tab (SPRINT-01..05)
**Rationale:** Needs the snapshot (Phase 4) and findings (Phase 5). The files are separate from Phases 7 and 8, so it can run in parallel with them.
**Delivers:**
- Tab 7, with a rollup on an elapsed-working-day-fraction axis. This removes the dependency on TEAM-05.
- A team drill showing per-person load.
- Capacity, labelled unreliable.
- Navigable findings through a root `JumpToWorkItemMsg`.
- A secondary key for sub-views.
- No auto-poll more often than every 5 minutes.

### Phase 10: Grooming Mode (GROOM-01..04)
**Rationale:** Depends on Phases 1, 3, 5, 8 and 9.
**Delivers:**
- A full-screen groom model with inline edits:
  - points
  - assignee, chosen only from resolved team members
  - sprint
  - AC, under the policy chosen in decision 6
- Reorder only in the full-backlog view: neighbours are pre-read, the move aborts if they have drifted, and before/after neighbours are audited.
- A gap queue that is edit-only.
**Avoids:** Pitfall 17, and Architecture anti-patterns 6 and 7.

### Phase Ordering Rationale

- **PLAT-02 moves to Phase 2.** Three APIs that v2.0 needs are 5.1-preview in b5 and could be deactivated. The transport injection is only clean on v7. Doing it early avoids writing against b5 twice. If it slips, Phases 3 onward still work on b5, since every method exists there, but the pacer would need a stopgap that swaps the global transport.
- **PLAT-03 is split.** The transport goes in Phase 2 and the shape caps in Phase 3, before the first fan-out. Throttling starts with the first multi-team rollup, so neither part can wait until the end.
- **TEAM-06 goes in Phase 3,** so the same area filter covers both the current set and the baseline set.

### Researcher Disagreements Resolved by This Synthesis

These are recommendations; the user can override them in discuss-phase.

| Topic | Positions | Resolution |
|---|---|---|
| Concurrency | STACK: semaphore plus `WaitGroup.Go`, N=4. PITFALLS: semaphore, 4–6. ARCHITECTURE: serial loops plus shape caps, no semaphore | **Serial plus shape caps in v2.0.** This follows YAGNI and avoids the global `azuredevops.SessionId` race. Add a semaphore only if a measured all-teams read exceeds `FetchTimeout`. If one is added, the pacer must delete `X-TFS-Session`, and `-race` must cover a test against the real client |
| Where 429 retry lives | ARCHITECTURE: the transport retries 429 for any method within the deadline. STACK and PITFALLS: no retry in the transport; reads retry in the service layer | **The transport paces but never retries:** Retry-After on a 200 or a 429 sets `notBefore`. Reads retry in the service layer on `FailureRateLimit`, up to N times. Writes are never retried automatically; a 429 on a write is a known refusal and exits 4. That keeps one mutation per audit record |
| Team scoping mechanism | FEATURES: `GetIterationWorkItems`, scoped server-side. ARCHITECTURE and PITFALLS: WIQL plus a client-side area filter | **WIQL plus a client-side area filter at both instants.** `GetIterationWorkItems` cannot use ASOF, and mixing the two mechanisms creates phantom scope change |
| Iteration predicate | STACK: `IterationPath` with quote-doubling. ARCHITECTURE: integer `IterationId` | **`[System.IterationId] = <int>`, formatted with `strconv`.** This is the closest WIQL gets to the org's parameterized-query rule. If the spike fails, fall back to the server-sourced path through the existing escaper and record a deviation |
| `Failure.Team` | ARCHITECTURE: always emitted. PITFALLS: `omitempty` | **Always emitted,** as `""` for project-scoped reads. This keeps a stable key set (the D-P3 lesson). It is a deliberate v2 contract change |

### Requirement Amendments Recommended

| # | Req | Amendment |
|---|---|---|
| 1 | **DATA-04** | Reconstruct baseline membership and values with WIQL `ASOF` plus `GetWorkItemsBatch{AsOf}` at start + grace (UTC, `TimePrecision`). Move `ReadReportingRevisionsPost` to CEREMONY-02 |
| 2 | **TEAM-06 (new)** | Sprint and backlog membership = team iteration ∩ team field values (area paths with their IncludeChildren flags). Use the same filter at every instant. Rollups dedupe by item id and emit an overlap finding |
| 3 | **Definitions: Delivered** | New `IsDelivered` predicate covering Resolved and Closed. It excludes Removed and Done; do not reuse `doneStates`. State the divergence from the ADO velocity widget in `scope` (decision 8) |
| 4 | **Definitions: Committed (new)** | Points and count of requirement-level items in the team's sprint as of the baseline instant |
| 5 | **Definitions: Delivered timing** | Item is in the sprint *and* delivered as of the close instant (end of `FinishDate` in the team's time zone) |
| 6 | **DATA-02 + points** | StoryPoints/Effort, RemainingWork, CompletedWork and OriginalEstimate are `*float64` and serialise as JSON `null` when absent on the new type. v1 `WorkItem.StoryPoints int` is unchanged and recorded as a known lossy field |
| 7 | **TEAM-01** | Discover teams with `core.GetTeams(project)`, paged, not with `GetAllTeams`, which is org-wide and 5.1-preview. Configured teams are an allow-list validated at startup. An unknown team exits 3. Zero configured teams is a config error |
| 8 | **TEAM-03** | No current iteration, or null dates, is a per-team failure, never an empty success. Dates are civil dates stored as UTC midnight. `FinishDate` is inclusive |
| 9 | **TEAM-05** | Replace the assumption with a runtime check that emits a finding on mismatch. SPRINT-01 uses an elapsed-working-day-fraction axis |
| 10 | **Process template (new, TEAM)** | Validate the points field, done states and rank field per project at startup, and fail loudly (decision 1) |
| 11 | **NAG-06** | Read the `ArtifactLink` PR relations on sprint items, then fetch only those PRs by id, capped, with an incomplete marker. Merged = completed into the repo's default branch. Drop `GetPullRequestWorkItemRefs` |
| 12 | **NAG-07** | Rewrite it, because it contradicts itself. Recommended: parent story still Active while every child task is terminal. Alternative: item still in a non-final done-category state (such as Resolved) at sprint close |
| 13 | **NAG-09** | Linear model: remaining-hours fraction compared with remaining-working-day fraction, with a tolerance. Below a RemainingWork-coverage threshold, report "insufficient data". Uses `WorkingDays` (parsed case-insensitively), never capacity |
| 14 | **NAG-10** | Limit to PRs linked to sprint items, sharing NAG-06's read |
| 15 | **NAG-13** | Backlog = the top `backlog_depth` not-done team backlog items at requirement level, with the depth disclosed. Only NAG-02, 03 and 04 run there. NAG-01 and NAG-12 run on sprint and next-iteration items only |
| 16 | **NAG-03** | "Empty" means empty after stripping tags, unescaping entities (`&nbsp;`) and collapsing Unicode whitespace. Record whether AC containing only an image counts as empty |
| 17 | **NAG-12** | Check `Microsoft.VSTS.Common.Activity = Development` first, then the title heuristic. Record which one matched |
| 18 | **CEREMONY-01** | Define "per-sprint points" as the velocity series over the last N sprints (default 6, max 12, set in config), or drop it as a duplicate of Delivered. Carryover for the current sprint is `null` with a reason |
| 19 | **CEREMONY-02** | Moved = a state transition or a RemainingWork decrease since T, read from reporting revisions, not `ChangedDate`. Group by current assignee, and list silent members. `--since` is required |
| 20 | **CEREMONY-03** | Backlog order, each item marked ready or not ready with its gaps, plus the raw velocity series. No capacity |
| 21 | **GROOM-03/04** | Reorder only in the full-backlog view; the gap queue is edit-only. Pre-read neighbours, use the process-aware rank field, and audit the before and after neighbours |
| 22 | **AGENT-02/03 coverage** | Default-deny: any verb not on AGENT-02 is human-only. **`prs approve` → human. Reorder → human. Link add → agent (amend AGENT-02). Unlink → human.** Create accepts only title, type, parent, description, AC, points, assignee and iteration |
| 23 | **AGENT-04 mechanism** | Tier is set per verb in the command table and checked in `dispatch()`. Human-only verbs need an interactive TTY and a typed target id, in addition to `--confirm`. The write layer enforces a field allow-list. Document the cooperative-agent limit and the separate-PAT pattern (decision 3) |
| 24 | **Audit record** | Add actor or source attribution: tui, cli-human or agent (decision 4) |
| 25 | **STORY-01/02/03** | Create inherits area and iteration from the parent and supports `validateOnly`. Comments use v7 `AddWorkItemComment` with Markdown, paged. Unlink sends `test /rev` before `remove /relations/N` |
| 26 | **PLAT-02** | Upgrade the SDK to v7.1.0 **and the toolchain to Go 1.27 with x/sys ≥ v0.44.0**, gated on govulncheck v1.9.0 in CI and a live smoke per sub-client. Update the "Go 1.25.4" constraint in PROJECT.md |
| 27 | **PLAT-03** | Split in two: (a) a pacer transport driven by response headers, plus read-only service-layer retry; (b) request-shape caps validated before the first call (team ceiling, `--sprints`, `backlog_depth`), all disclosed in `scope` |
| 28 | **Hygiene** | Replace stale line references with symbol names (`doneStates`, `findDevelopmentTask`, `getProjectWorkItems`, `Config.StateTransitions`, `stripHTML`) |

### Open Decisions (user must resolve before requirements are final)

| # | Decision | Options | Recommendation | Blocks |
|---|---|---|---|---|
| 1 | **Process template** of the configured projects | (a) Agile only, asserted at startup. (b) Resolve per project from state categories | **(a)** if every project is Agile. *Which process do your projects use?* | Phase 3 |
| 2 | **Team days off** in working-day math | (a) `WorkingDays` only. (b) Also subtract `GetTeamDaysOff` | **(b)** if the team maintains its holidays, otherwise (a). Either way, disclose `working_days_source`. Never use personal PTO | Phase 5 |
| 3 | **Strength of the tier boundary** | (a) Cooperative: TTY gate plus field allow-list. (b) Also a separate agent PAT with Work Items R/W only | **(a) in code, with (b) documented as configuration.** A PAT cannot separate state changes from comments. Also confirm that v1 write verbs become TTY-only | Phase 7 |
| 4 | **Audit source field** | (a) Add a 6th key, `actor`/`source`. (b) Encode it in `command`, e.g. `tui:workitems set-state` | **(a).** Must be decided in Phase 1, before the TUI starts appending | Phase 1 |
| 5 | **Exit code for a tier refusal** | (a) Reuse 3. (b) A new dedicated code, e.g. 6 | **(b).** "Not allowed" should be distinguishable from "malformed". Exit codes are a one-way door | Phase 7 |
| 6 | **Overwriting existing AC** (GROOM-02) | (a) Allow editing only when the AC is empty after stripping. (b) Append only. (c) Overwrite after a warning and a second keystroke | **(a).** In every case, write escaped HTML and never set `multilineFieldsFormat`, which cannot be undone | Phase 10 |
| 7 | **Two backlog orders** | (a) `priorities.yaml` stays as a personal order. (b) The groom view shows only ADO rank, labelled. (c) Retire `priorities.yaml` | **(a)+(b).** Label the source of each order | Phase 10 |
| 8 | **Delivered vs the ADO velocity widget** | (a) Resolved+Closed, with the divergence disclosed in `scope`. (b) Match ADO: Completed only (Closed/Done) | **(a),** with `delivered_definition` in `scope`. Choose (b) if stakeholders reconcile against the widget | Phase 6 |
| 9 | **TUI confirm granularity** | (a) Confirm every write. (b) Stage edits per item, confirm once when advancing, and write one audit record per field | **(b)** | Phase 1 |

### Research Flags

Phases likely to need `/gsd-plan-phase --research-phase <N>`:
- **Phase 2:**
  - Live behaviour after the move from 5.1 to 7.1-preview (PR complete, vsrm approvals, comments)
  - Replaying the request body via `GetBody` on POSTs
  - The `LocationUrl` host rules
- **Phase 4 (spike required):**
  - `IterationId` combined with `ASOF`, tested live
  - Whether `$expand` and `fields` are mutually exclusive
  - Fetching PR facts without an N+1
  - What `$timeframe=current` returns between sprints
- **Phase 5:** NAG-06, 07 and 09 semantics after the amendments; `BugsBehavior`.
- **Phase 6:** Decoding and paging reporting revisions, and their TSTU cost.
- **Phase 8:** PAT scopes; the Markdown comment format; `/multilineFieldsFormat` on AC.
- **Phase 10:** Reorder semantics (`parentId`, the iteration backlog); writing HTML into AC stored as Markdown.

Phases with standard patterns (research can be skipped):
- **Phase 1:** internal refactor, protected by the CLI write tests.
- **Phase 3:** light checks only, on `GetTeams` paging and group members.
- **Phase 7:** dispatch table plus a TTY check.
- **Phase 9:** existing Bubble Tea patterns.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | The upgrade, toolchain bump and govulncheck run were trialled. APIs were diffed in the module cache. Wire behaviour at 7.1-preview is the remaining risk |
| Features | MEDIUM | Codebase and SDK findings are HIGH. ADO behaviour comes from docs not yet confirmed live. The requirement issues are logical findings |
| Architecture | HIGH for integration, MEDIUM for the new API patterns | Integration was read at HEAD `dc25a3c`. ASOF with IterationId, and the PR-facts approach, have not been tested live |
| Pitfalls | MEDIUM to HIGH | Pitfalls verified against the repo are HIGH. Those about service behaviour are MEDIUM or LOW |

**Overall confidence:** MEDIUM. The plan is well grounded, but the numbers it produces depend on live checks in Phases 2 and 4.

### Gaps to Address

- **ASOF with `IterationId`, live:** spike at the start of Phase 4, with the fallback described above.
- **v1.0 deferred live checks are out of scope,** but the Phase 2 smoke run overlaps them. Record what it finds without reopening v1.0.
- **Audit file is created 0644** (BL-02, out of scope). v2.0 writes agent text to `audit.jsonl`. Either pull this one item into Phase 1, or accept the exposure and record it as a known risk.
- **PAT scopes per new call:** confirm in Phases 3 and 8.
- **Offline startup (`g-01-1`):** v7 does not fix it. Carry it forward.
- **Compliance:** AGENT-03 and BL-01 support SOX ITGC segregation of duties and audit evidence, so do not weaken them to confirm-gated. Standup, per-person load and STORY-02 expose free text, so flag any phase that copies work-item text for CPNI review.

## Sources

### Primary (HIGH confidence)
- Repository at HEAD `dc25a3c`: `cmd/adoBizInt/{dispatch,write,audit,exit}.go`, `internal/ado/{client,interfaces,result,workitems,filters}.go`, `internal/config/config.go`, `internal/ui/workitems/workitems.go`, `internal/keymap/keymap.go`
- SDK source in the module cache: `azuredevops@v1.0.0-b5` and `azuredevops/v7@v7.1.0`
- Trial upgrade and govulncheck runs, 2026-10-13
- proxy.golang.org, vuln.go.dev, go.dev/dl

### Secondary (MEDIUM confidence; official docs not yet confirmed live)
- Microsoft Learn: rate and usage limits; WIQL syntax; REST API versioning
- Microsoft Learn: REST 7.1 pages for iterations, work items list, reporting revisions, comments, reorder, and iteration work items
- Microsoft Learn: state categories; team velocity; points field per process; StackRank vs BacklogPriority
- Microsoft DevBlogs, 2025-07-07: Markdown in work items

### Tertiary (LOW confidence; needs validation)
- `$expand` and `fields` being mutually exclusive
- The VS402337 limit of 20,000 results
- What `$timeframe=current` returns between sprints, and default page sizes
- How the server treats HTML written into AC stored as Markdown

---
*Research completed: 2026-10-13*
*Ready for roadmap: yes, once the 9 open decisions are resolved and the amendments are applied to REQUIREMENTS.md*
