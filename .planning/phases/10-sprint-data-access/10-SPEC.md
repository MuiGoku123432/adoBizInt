# Phase 10: Sprint Data Access -- Specification

Inputs:

- `10-RESEARCH.md` (Bearing doc `8833a7a2…`, live spike 2026-10-22)
- `09-SPEC.md` (team selection, process fields, envelope)
- ROADMAP Phase 10
- REQUIREMENTS TEAM-06, DATA-01..06

User decisions taken 2026-10-22 are marked **[D-n]**.

## Outcome

`sprint items` shows each selected team's current sprint exactly as Azure DevOps holds it, at three points:

- live
- at the scope baseline
- at sprint close

It covers completed work and only that team's items. Phases 11-12 build rules on this read; nothing here judges the data.

## Scope

In:

- The `sprint items` verb.
- Team membership filter (iteration ∩ area paths).
- Nullable numerics, tasks grouped under parents, dropped-id reporting.
- `ASOF` reconstruction, overlap finding, acceptance-criteria emptiness, scope disclosure.

Out:

- PR status (active or completed): only link ids are read.
- Sprints other than the current one (`max_sprint_history` use is Phase 12).
- Rules and nags (Phase 11).
- TUI (Phase 15).

## 1. Team membership filter (TEAM-06)

Per selected team (Phase 9 `selectTeams`):

1. **Iteration id.** Take the team's current sprint from Phase 9 `currentSprint`. Resolve its integer id from the project's iteration tree, read once per project with `GetClassificationNode` (depth 10), and matched on the team-style path (`\P\Iteration\X` maps to `P\X`).
   - Build deviation (2026-10-23): this replaces a by-path node lookup. The SDK escapes a path route value as one segment (`/` becomes `%2F`), which the spike did not verify the server accepts. The tree walk cannot drop the last segment either.
   - The id is formatted with `strconv.Itoa`, never string-built. The query is built from three reviewed escapers (`iterationIDClause`, `areaClause`, `asOfClause`) added to the WIQL structural test's allow-list.
2. **Area paths.** Read `teamsettings/teamfieldvalues`.
   - `field.referenceName` must be `System.AreaPath`. Any other team field is a named per-team failure.
   - Each value becomes `[System.AreaPath] UNDER '<v>'` when `includeChildren` is true, and `= '<v>'` when false.
   - Values are OR'ed and quoted with the existing `''` escaper.
3. **WIQL.** One query per team (DATA-03), with no state filter (DATA-01):
   ```
   SELECT [System.Id] FROM WorkItems
   WHERE [System.TeamProject] = @project
     AND [System.IterationId] = <id>
     AND (<area clause>)
   [ASOF '<instant>']
   ```
   This set includes tasks. The same clause is used live and historically (TEAM-06 "applied identically at every point in time").
   - **Known limit:** team area settings are read as they are now. ADO keeps no history of team settings, so a historical snapshot uses today's area paths. This is disclosed in `scope`.
4. **Overlap.** An item returned for two or more selected teams produces one `team_overlap` finding naming the item id and the teams. A new `FindingKind` joins the closed set. The item still appears under each team.

## 2. Snapshots and instants (DATA-04)

`--at live|baseline|close`, default `live`.

| `--at` | Instant (UTC) | Notes |
|---|---|---|
| `live` | none (no `ASOF`) | relations read; `pull_request_ids` filled |
| `baseline` | sprint start date 00:00:00Z **[D-1]** | |
| `close` | day after finish date, 00:00:00Z **[D-2]**, of the **last closed sprint** | finish is inclusive; no closed sprint yet is a per-team failure |

- Historical reads use WIQL `ASOF` plus batch `asOf` at the same instant.
- Relations are not read historically (untested in the spike), so `pull_request_ids` is `null` there.
- Build correction (2026-10-23, review finding): the current sprint has by definition not closed, so `--at close` reads each team's most recently closed sprint (iterations listed in full, since `$timeframe` supports only `current`). `live` and `baseline` read the current sprint.
- `scope.as_of` is the snapshot requested (`live`, `baseline`, `close`); `scope.teams[].as_of` is the instant each team was read at, with `iteration_path`, because teams' sprints can differ.

## 3. Item reads (DATA-02, DATA-03)

There are two batch calls per ≤200 ids, using the existing `workItemAPI.GetWorkItems` (no new SDK method) **[D-3]**:

1. **Fields.**
   - `Fields` = Id, WorkItemType, Title, State, AssignedTo, Parent, AreaPath, IterationPath, AcceptanceCriteria, the project's points field (TEAM-07), RemainingWork, CompletedWork, OriginalEstimate.
   - `AsOf` is set when historical.
   - `ErrorPolicy = Omit`.
2. **Relations** (live only). `Expand = Relations`, no `Fields`, because the two are mutually exclusive (research Q2 400). Same `ErrorPolicy = Omit`.

**Dropped ids.** Compare the requested ids with the returned ids after each call. An item missing from the relations call keeps `pull_request_ids: null` (unknown, not none) and is reported the same way. A failed batch call returns the items already read with the failure. ADO silently omits missing or unreadable ids under `Omit`. Any difference is a per-team `Failure` with operation `get sprint items` and a code-generated message `"N requested ids not returned: [ids]"`. The read still returns what it got, with `complete: false`.

## 4. Item shape

`SprintItem`, one per work item:

| key | type | source |
|---|---|---|
| `project`, `team` | string | selection |
| `id` | int | |
| `type`, `title`, `state` | string | |
| `state_category` | string | the process category of the state (`Proposed`, `InProgress`, `Resolved`, `Completed`, `Removed`), from backlog configuration |
| `done` | bool | state is in the project's TEAM-07 done states |
| `assigned_to` | string | display name, "" when unassigned |
| `parent_id` | int or null | `System.Parent` |
| `points` | number or null | the project's resolved points field, fraction kept |
| `remaining_work`, `completed_work`, `original_estimate` | number or null | |
| `area_path`, `iteration_path` | string | |
| `acceptance_criteria_empty` | bool | DATA-06 (section 5) |
| `pull_request_ids` | int[] or null | live: ids parsed from `ArtifactLink` urls `vstfs:///Git/PullRequestId/<proj>%2F<repo>%2F<id>`; `[]` when none; `null` when historical |
| `tasks` | SprintItem[] | children of type Task (`ado.IsTaskType`) whose parent is in this team's set; `[]` otherwise |

Rules:

- Grouping (DATA-03): Tasks whose parent is in the set appear only under `tasks`. A task whose parent is outside the set stays top level with its `parent_id`, so nothing is silently dropped.
- Absent numeric fields are `null`, never 0 (research Q2: absent means the key is missing).
- The v1 `WorkItem.story_points` int is untouched (DATA-02).
- `state_category` and `done` come from the project's process. They are never a fixed list: the live states include custom ones such as "ON Hold" and "UAT".
  - This replaces the roadmap note's fixed `IsDelivered (Resolved + Closed)`. `doneStates` and `IsDoneState` (v1) are untouched.
  - **Live finding (2026-10-23, Kinetic Sprint 217):** the `Resolved` state maps to category `InProgress`, not `Resolved`. So "delivered" cannot be `state_category in {Resolved, Completed}`. Phase 10 exposes `state` and `state_category` only; Phase 11/12 must decide "delivered" with this in mind.
- `ProcessFields` (Phase 9) gains `state_categories: {type: {state: category}}` for requirement and task types. That adds a key, so the contract test is updated.

## 5. Acceptance-criteria emptiness (DATA-06)

`acceptance_criteria_empty` is true when, after all of the following, the text is empty:

- removing HTML tags
- decoding entities with `html.UnescapeString` (so `&nbsp;` becomes U+00A0)
- trimming all Unicode whitespace (`unicode.IsSpace`, which covers U+00A0)

An absent field is empty. This is a pure function, unit-tested on `<div><br></div>`, `&nbsp;`, `<p>&nbsp; </p>`, and real text.

## 6. Verb and envelope

- `sprint items` is a scrum verb. It uses `runScrum`, `--team`, `--project`, `--json` and `--timeout`, plus `--at`.
- Order:
  1. `selectTeams`
  2. per team: current sprint
  3. area paths
  4. process fields (once per project)
  5. WIQL
  6. batches
- `scope` on this verb carries:
  - `caps` (Phase 9)
  - `as_of`: `"live"` or the instant, per invocation
  - `teams`: per team `{team, iteration_id, area_paths: [{path, include_children}], points_field}`
  - `area_paths_current: true` (the known limit in section 1.3)

  New `Scope` keys; the exact-key test is updated.
- Per-team failures are `Failure` with the `team` set, and other teams still return. Operations:
  - `resolve sprint iteration`
  - `get team area paths`
  - `query sprint items`
  - `get sprint items`
- Human mode: one row per item (`project team id type state points title`), with tasks indented under their parent.
- Exit codes: as for every read.

## 7. Seams

- `coreAPI`/`workAPI` gain only the called methods:
  - `work.GetTeamFieldValues`
  - `workitemtracking.GetClassificationNode` (already on `workItemAPI`)
- WIQL uses the existing `workItemAPI.QueryByWiql` with a team context.
- Each new read method gets a retry decorator and an entry in the `retry_test` classification.

## Acceptance criteria

| ID | Req | Check |
|---|---|---|
| AC1 | TEAM-06 | The WIQL text for a team with two area values (one with children, one without) is exactly `… IterationId = 11351 AND ([System.AreaPath] UNDER 'A' OR [System.AreaPath] = 'B')`. A non-AreaPath team field gives a named per-team failure. |
| AC2 | TEAM-06 | An item returned for two selected teams gives one `team_overlap` finding and appears under both teams. |
| AC3 | DATA-01 | The query has no state clause. Closed and Removed items from the fake appear. The iteration id goes through `strconv`. |
| AC4 | DATA-02 | A missing points or effort field serialises as `null`. `2.5` stays `2.5`. `WorkItem.story_points` is still int (key test). |
| AC5 | DATA-03 | One WIQL per team. Tasks are nested under parents in the set, and an orphan task stays top level. A batch returning 19 of 20 ids gives a per-team failure naming the dropped id, with `complete: false`. |
| AC6 | DATA-04 | `--at baseline` sends `ASOF '<start>T00:00:00Z'` and batch `AsOf` at the same instant, and skips relations (`pull_request_ids: null`). `--at close` uses finish+1 day. A close in the future is a per-team failure. |
| AC7 | DATA-05 | `sprint items --json` has scope keys `caps`, `as_of`, `teams`, `area_paths_current`. Every item carries `team`. `--team` narrows. |
| AC8 | DATA-06 | The emptiness function's table passes, including `&nbsp;` and tag-only HTML. |
| AC9 | live | Live `pull_request_ids` are parsed from ArtifactLink urls. A non-PR ArtifactLink and attachments are ignored. |
| AC10 | all | CI gates pass. `smoke.sh` gains `sprint items` gated on `SMOKE_TEAM`. |

## Open (verify live when convenient)

- `$timeframe=current` between sprints (Phase 9 treats it as a failure either way).
- Relations with `asOf`: if it works, historical `pull_request_ids` could be filled later.
- Team field other than area path: not seen in this org.
