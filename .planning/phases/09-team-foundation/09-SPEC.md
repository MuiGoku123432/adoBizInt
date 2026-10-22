# Phase 9: Team Foundation -- Specification

Inputs: `09-RESEARCH.md` (Bearing doc `abb4c64c1954d4689483656f37a4d740`), ROADMAP Phase 9, REQUIREMENTS TEAM-01..05, TEAM-07, TEAM-08, PLAT-04. User decisions taken 2026-10-22 are marked **[D-n]**.

## Outcome

Every scrum read targets named, validated teams and each team's current sprint. Nothing falls back to a project's default team or an assumed cadence.

## Scope

In:

- Teams config, validated against ADO for scrum verbs only.
- Four new read verbs: `teams list`, `teams members`, `sprint current`, `teams process`.
- Envelope changes: `team` key on every failure, `findings` array on every result.
- Shape caps, refused before the first call and disclosed in `scope`.

Out:

- TEAM-06 area-path intersection (Phase 10).
- Group expansion, Graph API calls.
- TUI changes (Phase 15).
- Any write.

## 1. Configuration

**[D-1] Map per project.**

```yaml
projects: [ProjA, ProjB]
teams:
  ProjA: [Platform, Payments]
  ProjB: [Mobile]
caps:                    # optional; defaults shown [D-5]
  max_teams: 25
  max_sprint_history: 12
  max_backlog_depth: 500
```

- New `Config.Teams map[string]StringSlice` (`yaml:"teams"`) and `Config.Caps` (three ints).
- Env override `ADO_TEAMS="ProjA/Platform,ProjA/Payments,ProjB/Mobile"`. It splits on the first `/` only. If set, it replaces the YAML map entirely, matching how `ADO_PROJECTS` replaces `projects`.
- `Validate()` (static, no network) adds these checks when `teams` is present:
  - every key is in `projects`
  - no blank team name
  - no duplicate within a project
- Each failure is a `ConfigError` (exit 3). An absent `teams` map is valid here **[D-2]**.
- `ValidateScrum()` (scrum verbs only, no network) checks that every configured project has at least one team and that every cap is > 0. Caps moved here from `Validate()` during build: v1 configs and test literals carry zero caps, and v1 never uses them.

## 2. Team validation (TEAM-01) **[D-2]**

Runs only for `teams members`, `sprint current` and `teams process`. v1 verbs and the TUI never call it and need no teams config.

Steps, before any scrum read:

1. Zero configured teams across all projects: config error `teams: at least one team is required for scrum verbs`, exit 3.
2. A configured project with no `teams` entry: config error naming that project, exit 3.
3. Cap check (section 6) runs here, before any network call.
4. Per configured project, page `core.GetTeams`:
   - `Top=100`, `Skip += len(page)`
   - stop on a page shorter than `Top` or an empty page
   - `Mine` and `ExpandIdentity` unset
5. Any configured name not found (exact, case-insensitive match as ADO treats names): config error `teams: unknown team "ProjA/Foo"`, exit 3.
6. A `GetTeams` call that fails:
   - auth: exit 2. Message names the "Project and Team (Read)" scope (research section 5).
   - other: exit 3 with the error. Validation cannot be partial: it is a precondition, not a read.

The resolved team ID is carried forward. Reads use team IDs, not names.

## 3. Team selection (TEAM-04)

- Scrum verbs run over all configured teams by default.
- `--team P/T` narrows to one. It must name a configured team, or it is a usage error (exit 3). It takes one value only, per the requirement; it is not repeatable.
- Output never contains a synthetic aggregate row. Each item carries `project` and `team`.

## 4. Verbs

All four use `runRead` and the `Result[T]` envelope, and support `--json` like v1 reads.

### `teams list` (discovery, no team config needed)

- Pages `GetTeams` for every configured project.
- Item: `{project, team, team_id, description}`.
- A project whose call fails is a `Failure` with `team: ""`.
- Not subject to `max_teams` (discovery), but each project's paging is bounded by the same termination rule.

### `teams members` (TEAM-02)

- Per selected team: `core.GetTeamMembersWithExtendedProperties`, offset-paged like `GetTeams`.
- Item: `{project, team, id, display_name, unique_name, is_group, is_team_admin, inactive}`. Items are keyed by `id`.
- `is_group = IsContainer == true || descriptor has prefix "vssgp." or "aadgp."`. Groups are listed, never expanded or dropped.
- A failure for one team is a per-team `Failure`. The other teams still return.

### `sprint current` (TEAM-03, TEAM-05)

- Per selected team: `work.GetTeamIterations` with `Timeframe="current"`.
- Item: `{project, team, iteration_id, name, path, start_date, finish_date}`.
  - Dates are `YYYY-MM-DD` civil strings, taken from `.Time.UTC()`.
  - `finish_date` is inclusive.
- These are per-team failures (`kind: "other"`, operation `get current sprint`), never an item:
  - zero iterations
  - more than one iteration
  - nil `Attributes`, `StartDate` or `FinishDate`
  - `FinishDate` before `StartDate`
- Cadence check: when 2+ teams resolve, any whose `(start_date, finish_date)` differs from another's produces one finding (section 5).

### `teams process` (TEAM-07)

- Per selected project: `work.GetBacklogConfigurations` for the first configured team of that project.
- Item: `{project, points_field, rank_field, done_states: {<work item type>: [states]}}`.
  - `points_field = TypeFields["Effort"]` and `rank_field = TypeFields["Order"]`.
  - `done_states`: the states with category `Completed`, for the requirement-backlog work item types.
- These are per-project failures (`team` = the team queried), with operation `resolve process fields` and a message naming the missing piece:
  - missing `Effort` or `Order`
  - zero done states for any requirement type
- **[D-7]** The exit code follows v1 partial-read rules (exit 1), not exit 3. TEAM-07's "named config failure" is honored by the message, because this is discovered at read time, not from config.

The resolution lives in `internal/ado` so Phases 10-12 reuse it in-process. Verbs that need it later call it once per project per run.

## 5. Envelope changes (TEAM-08) **[D-4]**

### `Failure` gains `team`

- Five keys: `project, team, operation, kind, err`. Always present.
- `""` for project-scoped reads, which includes every v1 read and `teams list`.
- `appendFailure` gains a `team` parameter. Existing call sites pass `""`.
- `json_test.go` exact-key assertion is updated in the same change.

### `Result` gains `findings`

- Always present, `[]` when empty, pre-allocated in `newResult` like `failures`.
- Finding: `{kind, project, teams, detail}`.
  - `kind` is a closed set; Phase 9 defines only `cadence_mismatch`.
  - `teams` is `["P/T", ...]`.
  - `detail` is code-generated text, never server text.
- Findings never affect `complete`.
- This changes the output of all v1 reads (one new key, always `[]`). It is a deliberate v2 contract change. Contract tests are updated in the same change.

## 6. Shape caps (PLAT-04) **[D-5]**

| Cap | Default | Enforced in Phase 9 by |
|---|---|---|
| `max_teams` | 25 | `teams members`, `sprint current`, `teams process` (count of selected teams) |
| `max_sprint_history` | 12 | validated and disclosed only; enforced by Phase 10 verbs |
| `max_backlog_depth` | 500 | validated and disclosed only; enforced by Phase 16 |

- Over the cap: usage error naming the cap and the count, exit 3, no network call made.
- `Scope` gains `caps: {max_teams, max_sprint_history, max_backlog_depth}` on scrum verbs only (v1 scope rules unchanged). Values are the caps in force.

## 7. Seams and tests

- Add only the called methods to package-local interfaces in `interfaces.go` (SEAM-01 rule):
  - new `coreAPI`: `GetTeams`, `GetTeamMembersWithExtendedProperties`
  - new `workAPI`: `GetTeamIterations`, `GetBacklogConfigurations`
- Fakes go in `fakes_test.go`.
- No new dependencies.

## Acceptance criteria

| ID | Req | Check |
|---|---|---|
| AC1 | TEAM-01 | Config tests: unknown project key, blank or duplicate team, bad cap give `ConfigError`. Scrum verb with zero teams, or a name absent from paged fake `GetTeams` (teams split across 3 pages), gives exit 3 with the named error. |
| AC2 | TEAM-01 | `teams list` against a 3-page fake returns all teams. With no `teams` config, v1 verbs still exit 0. |
| AC3 | TEAM-02 | `teams members` fake with a user, `IsContainer` group and `aadgp.` group: both groups have `is_group: true`, and all three are present and keyed by id. |
| AC4 | TEAM-03 | `sprint current` gives the same `start_date`/`finish_date` strings under `TZ=America/Chicago` and `TZ=Asia/Kolkata`. Zero iterations, two iterations or null dates each give a per-team failure and no item. |
| AC5 | TEAM-04 | Default covers all configured teams. `--team P/T` returns only that team. An unconfigured `--team` exits 3. No item lacks `team`. |
| AC6 | TEAM-05 | Two teams with different dates give one `cadence_mismatch` finding and `complete: true`. Equal dates give `findings: []`. |
| AC7 | TEAM-07 | `teams process` resolves points, rank and done states for Scrum-, Agile- and CMMI-shaped fakes. A missing `Order` gives a named per-project failure. |
| AC8 | TEAM-08 | Exact-key tests: Failure = 5 keys, `team` always present (empty on v1 reads). Result includes `findings`. |
| AC9 | PLAT-04 | 26 selected teams with the default cap exits 3 and the fake records zero calls. Scrum verb `scope.caps` shows the values in force. |
| AC10 | all | CI gates pass: build, vet, `test -race`, gofmt, govulncheck. |

## Resolved decisions (user, 2026-10-22)

- **[D-6]** Process fields come from the first configured team per project; no cross-team comparison.
- **[D-7]** Unresolvable process fields are a named per-project failure, exit 1 (partial), not exit 3.
- **[D-8]** `scripts/smoke.sh` gains the four reads, gated on `SMOKE_TEAM`. Human-run only, never in CI.

## Open (verify on live smoke)

- **Unverified (from research):**
  - the shape of the 403 when `vso.project` is missing
  - server-side `$top` cap
  - whether ADO can return more than one current iteration (handled as a failure regardless)
