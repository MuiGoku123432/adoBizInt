# Phase 9: Team Foundation -- Research

Scope: the light checks the roadmap asks for (GetTeams paging, group members, PAT scopes), plus the iteration and process-field APIs Phase 9 needs. SDK = `github.com/microsoft/azure-devops-go-api/azuredevops/v7@v7.1.0` (module cache). No live calls were made.

No Phase 9 call (`GetTeams`, `GetTeamMembers*`, `GetTeamIterations`, `GetBacklogConfigurations`, `GetProcessConfiguration`) exists in `internal/` yet.

## 1. `core.GetTeams` paging (TEAM-01)

Observed (`core/client.go:662-705`):

- Args: `ProjectId` (required), `Mine`, `Top`, `Skip`, `ExpandIdentity`. Sent as `$top` / `$skip` query params, API version `7.1-preview.3`.
- Returns `*[]WebApiTeam` via `UnmarshalCollectionBody`. No continuation token; the SDK exposes no total count.
- Paging is offset-based only. Termination rule: loop `$skip += len(page)` until a page returns fewer than `$top` items (or zero).

Recommendation:

- Always set `Top` explicitly (e.g. 100). Do not rely on a server default; ADO REST docs state a default of 100 when `$top` is omitted, which silently truncates large projects.
- Leave `Mine` unset (validation must see all teams the PAT can read, not only the user's).
- Leave `ExpandIdentity` unset; not needed for name validation.
- Validate configured team names against the full paged set, so an unknown name fails with exit 3, not a false negative from page 1.

Unknown: whether the server caps `$top` below the requested value. Guard by terminating on `len(page) == 0` as well as `< top`, and cap total pages via the PLAT-04 shape cap.

## 2. Team members and groups (TEAM-02)

Observed:

- `core.GetTeamMembersWithExtendedProperties` (`core/client.go` ~620-646): args `ProjectId`, `TeamId`, `Top`, `Skip`; same offset paging, version `7.1-preview.2`. Returns `[]webapi.TeamMember`.
- `webapi.TeamMember` (`webapi/models.go:248`): `Identity *IdentityRef`, `IsTeamAdmin *bool`.
- `webapi.IdentityRef` (`webapi/models.go` ~95-125): `Id`, `Descriptor`, `DisplayName`, `UniqueName`, `Inactive`, `IsContainer`, `IsAadIdentity`.
  - `IsContainer` marks a group. The SDK comment calls it deprecated in favor of the descriptor subject type.
  - Descriptor prefixes carry the subject type: groups are `vssgp.` (ADO group) or `aadgp.` (Entra group); users are `aad.`, `msa.`, `svc.` etc.

Recommendation:

- Key members by `Identity.Id`.
- Flag a member as a group when `IsContainer == true` OR the descriptor starts with `vssgp.` / `aadgp.`. Using both covers the deprecation.
- Never expand groups (expansion needs Graph APIs and `vso.graph`; out of scope per TEAM-02).
- Carry `Inactive` through; do not drop inactive members silently.

## 3. Current sprint per team (TEAM-03)

Observed:

- `work.GetTeamIterations` (`work/client.go` ~1175-1200): `$timeframe` param; pass `"current"`. Team context = project + team.
- `work.TeamSettingsIteration` (`work/models.go:808`): `Id`, `Name`, `Path`, `Attributes`.
- `work.TeamIterationAttributes` (`work/models.go:730`): `StartDate`, `FinishDate` (`*azuredevops.Time`), `TimeFrame`. SDK comment: "Date-only, correct unadjusted at midnight in UTC."

Recommendation:

- Zero results, or `Attributes`/`StartDate`/`FinishDate` nil, is a per-team failure (TEAM-03).
- Convert to a civil date by reading `.Time.UTC()` year/month/day. Never call `.Local()`; that is what breaks under `TZ=America/Chicago` (day shifts back) and stays hidden under `Asia/Kolkata`. Test both TZ values.
- Finish date is inclusive: the last working day is `FinishDate`, not `FinishDate - 1`.
- More than one "current" iteration: unknown whether ADO can return it. Treat it as a per-team failure, not a pick.

## 4. Process fields (TEAM-07)

Observed:

- `work.GetBacklogConfigurations` (team-scoped) returns `BacklogConfiguration` (`work/models.go:31`):
  - `BacklogFields.TypeFields map[string]string`: field type to reference name. Keys include `Order` (backlog rank field), `Effort` (points field), `Activity`, etc.
  - `WorkItemTypeMappedStates []WorkItemTypeStateInfo`: per type, `States map[state]category`. Done category is `"Completed"`.
  - `RequirementBacklog`: the requirement-level work item types.
- `work.GetProcessConfiguration` (project-scoped) also exposes `TypeFields map[string]WorkItemFieldReference`.

Recommendation:

- Use `GetBacklogConfigurations` for one configured team per project: it gives points field (`TypeFields["Effort"]` resolves to `Microsoft.VSTS.Scheduling.StoryPoints`, `.Effort` or `.Size` by process), rank field (`TypeFields["Order"]`), and done states (states whose category is `Completed`, for requirement-backlog types).
- Missing `Effort` or `Order`, or zero `Completed` states, is a named config failure for that project.
- `GetProcessConfiguration` is a fallback only; do not call both.

Unknown: whether two teams in one project can return different backlog configurations. They share a process, so the field names should match; confirm on the live smoke.

## 5. PAT scopes (R3)

Source: Microsoft Learn, OAuth scopes table (https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/oauth?view=azure-devops#scopes). PATs use the same scope names. Per-endpoint mapping below is from the REST reference `scopes` header for each endpoint; not verified against a live PAT.

| Call | Scope |
|---|---|
| `core.GetTeams` | `vso.project` (Project and team, read) |
| `core.GetTeamMembersWithExtendedProperties` | `vso.project` |
| `work.GetTeamIterations` | `vso.work` (Work items, read) |
| `work.GetBacklogConfigurations` / `GetProcessConfiguration` | `vso.work` |

Implications:

- New requirement: the PAT needs **Project and Team (Read)** in addition to Work Items (Read). Existing PATs scoped only to the v1 reads may lack it. Startup validation should map the resulting 401/403 to exit 2 and name the missing scope.
- Do not require `vso.graph` or `vso.identity`; group expansion is out of scope.

Unknowns to check in the Phase 9 live smoke:

- Confirm the 403 shape when `vso.project` is missing (for the error mapping).
- Confirm `$top` behavior on a project with more than 100 teams, if one exists.
