# Phase 10: Sprint Data Access -- Research (live spike)

Spike: `scripts/spike-phase10.sh`, read-only. It ran on 2026-10-22 against org `Windstream`, team `Kinetic/Pricing Engine`, current sprint `Kinetic\S2W 2026\Sprint 218` (2026-10-13 to 2026-10-26). It used API version 7.1 and the user's PAT. The script prints only counts, status codes, field reference names and dates.

One script bug was found and fixed during the run. The URL-encoding loop dropped the last path segment, so the first Q1 run compared against the parent node and wrongly showed a mismatch. The results below are from the fixed script.

## Headline: an iteration is shared across teams; the area path is what makes a sprint a team's

| Query (Sprint 218, IterationId 11351) | Items |
|---|---|
| Iteration only, whole project | 556 live |
| Iteration AND `[System.AreaPath] UNDER <team area>` | **5** live, **2** ASOF 2026-10-14T12:00Z |

- Kinetic has 87 teams on shared sprint iterations.
- TEAM-06 (iteration ∩ area paths) is not an edge case: without it, `sprint items` overstates a team's sprint about 110 times.
- The area filter must be applied in the WIQL itself, both live and in ASOF queries. Fetching the iteration set and filtering client-side would read 100+ times more items.

## Q1: WIQL `[System.IterationId] = N`, live and ASOF -- works (DATA-01, DATA-04)

- The integer iteration id comes from `GET {project}/_apis/wit/classificationnodes/Iterations/{relative path}`, field `id`.
- Path mapping: a team iteration path `Kinetic\S2W 2026\Sprint 218` maps to node path `\Kinetic\Iteration\S2W 2026\Sprint 218`. The relative path is the team path without its first segment.
- Every segment must be encoded, including the last one. The spike's own bug showed what happens otherwise: a dropped segment silently resolves to the parent node, whose id matches a different and larger item set.
- `[System.IterationId] = 11351` returned exactly the same set as `[System.IterationPath] = '…'`:
  - live: 556 = 556
  - `ASOF '2026-10-14T12:00:00Z'`: 345 = 345
- The ASOF response echoes `asOf` as the instant requested. A live query echoes the server's current time.
- `[System.IterationPath] UNDER` gave the same 556, because sprint leaves have no children.
- The query has no state filter, and completed work is included. States seen across 200 items: Active, Closed, New, ON Hold, Pending, Removed, Resolved, UAT.
  - States are process-customised (for example "ON Hold" and "UAT").
  - Done classification must use the TEAM-07 resolved done states, never a hard-coded list.
- **Recommendation:** filter on `[System.IterationId] = <int>` formatted with `strconv` (as the roadmap requires). The iteration-path fallback is not needed.

## Q2: `workitemsbatch` (DATA-02, DATA-03, DATA-04)

| Request | Result |
|---|---|
| `fields` + `$expand: Relations` | **400** `ConflictingParametersException: The expand parameter can not be used with the fields parameter.` |
| `$expand: Relations` only | 200, 20/20 returned, all fields plus relations |
| `fields` + `asOf` | 200, 20/20 |
| 20 real ids + 1 missing, `errorPolicy: Omit` | 200, **count 20, no null entry**: the missing id is silently dropped |
| Same, default error policy | **404** `TF401232 … does not exist, or you do not have permissions`: the whole batch fails |

Implications:

- `$expand` and `fields` are mutually exclusive. Read fields with `fields` (works with `asOf`), and read relations in a separate `$expand: Relations` call. Or use `$expand: Relations` alone and accept all fields.
- Use `errorPolicy: Omit`, and compare the requested ids with the returned ids to report dropped items (DATA-03). With Omit, ADO signals nothing itself. Without Omit, one deleted or unreadable item fails the whole batch.
- The batch limit is 200 ids per call (documented API limit; not exercised).
- Numeric fields: `StoryPoints` and `RemainingWork` were absent on 30 of 40 values and JSON numbers on 10.
  - Absent means the key is missing, not 0.
  - This confirms DATA-02's `null` when absent; JSON numbers keep fractions.

## Q3: pull-request links and parents in one call -- yes, no N+1 (DATA-03)

- A single `$expand: Relations` batch of 20 items returned relation names including `Parent` (16), `Child` (9), `Pull Request` (8), `Fixed in Commit` (8), `Related` (62), and attachments (by file name).
- PR link: `rel = "ArtifactLink"`, `attributes.name = "Pull Request"`, url `vstfs:///Git/PullRequestId/<projectGuid>%2F<repoGuid>%2F<prId>`. The project id, repo id and PR id are all parseable from the url.
- Parent link: `rel = "System.LinkTypes.Hierarchy-Reverse"` (16 found). Tasks can be grouped under their parent from relations, or from `System.Parent` in a fields read.
- **Still unknown:** PR state (active, completed, abandoned) is not on the link. It needs a git call per distinct PR (`GetPullRequestById`). That is N per PR, not per work item. Phase 10 only needs link facts; defer PR status to the phase that uses it.
- Attachment relations carry user file names. Never render relation names to the terminal without filtering on `rel`.

## Q4: iterations and `$timeframe=current` (TEAM-03, Phase 9 open question)

- 8 team iterations: 1 `current`, 7 `past`, none with null dates.
- `$timeframe=current` returned exactly 1.
- Date strings arrive as `"2026-07-07T00:00:00Z"`: UTC midnight, matching Phase 9's civil-date reading.
- **Still unknown:** behaviour between sprints. Today is inside a sprint, and this team's iterations have no gap to test. Phase 9 already treats zero or several current iterations as a per-team failure, which is the safe answer whatever ADO does.

## Q5: team area paths (TEAM-06)

- `GET {project}/{team}/_apis/work/teamsettings/teamfieldvalues`: `field.referenceName = System.AreaPath`, 1 value, `includeChildren = true`, and `defaultValue` is among the values.
- So the team field is the area path, and its values carry per-value `includeChildren`.
- The WIQL clause per value is `UNDER` when includeChildren is true and `=` when false, OR'ed across the values.
- An org could configure a non-area team field. Treat any `field.referenceName` other than `System.AreaPath` as a named per-team failure until seen live.

## Phase 9 live facts gathered on the way (waived smoke, partially closed)

- `teams list --project Kinetic` ran live: **87 teams, complete, no failures**.
  - Under 100, so paging past one page is still unverified live.
  - The project and token can read teams, so Project and Team (Read) is in place.
- A user-facing pitfall: the team is `Pricing Engine`, not `Kinetic Pricing Engine`. A wrong name gives `404 TeamNotFoundException`. Phase 9's validation turns that into `unknown team "Kinetic/…"` (exit 3), which is the right behaviour. `teams list` is the way to find the name.

## Open for the spec

1. Area-path filtering in WIQL: `UNDER` vs `=` per value, multiple values OR'ed, and the overlap finding when two teams' areas both claim an item (TEAM-06).
2. Two batch reads, fields (with `asOf` when historical) and relations (live only), or one `$expand: Relations` read. Note that relations `asOf` was not tested.
3. Baseline instant definition: sprint start date at 00:00 UTC, or a configurable offset? The spike used start + 1 day 12:00Z only to test ASOF.
4. Sprint close instant: finish date + 1 day 00:00 UTC (the finish is inclusive).
