# Pitfalls Research

**Domain:** Scrum analytics, board-honesty rules, grooming and story writes added to an existing Go Azure DevOps TUI+CLI (adoBizInt v2.0 Scrum Master)
**Researched:** 2026-10-13
**Confidence:** MEDIUM overall. HIGH for the pitfalls verified directly against this repo and the pinned SDK source in the module cache, including a compile spike against SDK v7.1.0. MEDIUM for Azure DevOps service behavior taken from official Microsoft Learn docs and cross-checked against SDK method signatures. LOW items are marked inline.

**Core-value lens:** every pitfall below is ranked by one question: can it make an agent mistake a broken or mis-scoped call for a true, empty, or complete answer? In v2.0 the failure mode moves up a layer. The API call succeeds, but it is aimed at the wrong team, the wrong field, the wrong state set or the wrong instant. The result is `complete: true` and wrong. v1.0's `Result[T]` envelope cannot catch that. Only explicit `scope` disclosure and per-verb negative tests can.

---

## Critical Pitfalls

### Pitfall 1: Team context silently defaults to the project's default team

**What goes wrong:**
Several APIs have a team as an optional route segment. Without one, they quietly answer for the project's **default team**. This affects `work.GetTeamIterations` (`team` is optional in the route), `QueryByWiql` with `@CurrentIteration` (`QueryByWiqlArgs.Team` is optional), `GetTeamSettings`, `GetTeamFieldValues` and `GetIterationWorkItems`. The call returns 200 with plausible data for a different team. A multi-team rollup where one team's name was missing or mistyped in config reports that team's sprint as the default team's sprint, with `complete: true`.

**Why it happens:**
The Go SDK models `Team *string` as optional on every one of these args. A nil pointer compiles, and the server substitutes the default team instead of returning an error.

**How to avoid:**
- Make team a required, non-pointer parameter on every new `internal/ado` scrum method. Translate to the SDK's `*string` at the last line.
- Validate configured team names against `core.GetTeams(projectId)` at startup, the same way HARD-04 validates projects. An unknown team is a usage/config error (exit 3), not a partial result.
- Put `team` (name and id) in the `scope` block of every scrum verb's envelope.
- Use `core.GetTeams(project)` for TEAM-01 discovery, not `GetAllTeams`. `GetAllTeams` is organization-wide, and its results must then be filtered by `ProjectId`.

**Warning signs:**
Two teams in a rollup report identical iteration ids or identical item counts. A fake test passes with `Team: nil`.

**Phase to address:** Team Foundation (TEAM-01..05), the first scrum phase. Every later phase inherits it.

---

### Pitfall 2: Shared iteration paths pull in other teams' work (iteration is not team scope)

**What goes wrong:**
TEAM-05 says the teams share a sprint cadence. In ADO that usually means they share the **same iteration nodes** (`Project\Sprint 42`). A WIQL filter on `[System.IterationPath] = 'Project\Sprint 42'`, or `@CurrentIteration` with a team route, returns **every team's** items in that sprint. Iteration selects time, and **area path** selects team. The result is points, velocity and nag findings attributed to the wrong team, and a rollup that double-counts when teams' area paths overlap. This happens when a parent team owns `Project\Platform` with include-children and a child team owns `Project\Platform\API`.

**Why it happens:**
In the ADO web UI the sprint view is implicitly area-scoped by team settings, and the REST query is not. The existing `iterationClause` in `internal/ado/client.go:289` filters by iteration only, because v1.0 was never team-scoped.

**How to avoid:**
- Fetch `work.GetTeamFieldValues` per team. These are the team's area paths, each with its own `IncludeChildren`. Apply them as the team filter.
- Apply area membership **client-side** after the iteration query, rather than interpolating area paths into WIQL. This avoids extending the string-built `iterationClause` pattern, which already sits awkwardly with HARD-04 and the org's parameterized-query rule. WIQL has no bind parameters beyond macros.
- In the rollup, detect items claimed by more than one team and emit a finding. Never silently count them twice, and never silently drop the duplicate.
- Disclose the area paths and include-children flags in `scope`.

**Warning signs:**
Sum of per-team points exceeds the project total for the sprint. A team shows items whose AreaPath is outside its team field values.

**Phase to address:** Team Foundation (fetch and cache team field values) and Sprint Data Access (DATA-01/03 apply them).

---

### Pitfall 3: Missing values read as zero (points, remaining work, estimates)

**What goes wrong:**
`internal/ado/workitems.go:260-261` reads StoryPoints with `if points, ok := ...(float64); ok { wi.StoryPoints = int(points) }` into an `int` with `json:"story_points"`. This has two defects for v2.0:
1. **An absent value and 0 are indistinguishable.** NAG-02 ("no story points") cannot tell an unestimated story from a story deliberately pointed at 0. NAG-05 ("remaining work is zero but Active") fires on every task whose RemainingWork was **never set**, because it reads as 0.
2. **Truncation.** `int(points)` turns 0.5 into 0 and 2.5 into 2. Velocity and committed or delivered totals undercount, and a 0.5-point story is flagged as unpointed.

The JSON contract makes this worse. `story_points: 0` is emitted for "unknown", which is exactly the "broken looks like empty" failure, one field down.

**Why it happens:**
Fields in `WorkItem.Fields` are a `map[string]interface{}`. An unset field is simply absent, and the v1 struct had no nullable representation. This is the same root cause as BL-01 part 1, where a blank field became `0`, and this time it is on the read side.

**How to avoid:**
- New sprint-path structs carry `*float64` for StoryPoints/Effort/Size, RemainingWork, CompletedWork and OriginalEstimate, emitted as JSON `null` when absent.
- Do not change the v1 `WorkItem.StoryPoints int` contract in place. Add the nullable fields to the new sprint-scoped type, and record the v1 field as a known lossy shape.
- Nag rules treat `nil` as "unestimated" and never as 0. NAG-05 requires RemainingWork **present and == 0**.
- Add tests with fixture items where the field is absent, 0, and 0.5.

**Warning signs:**
NAG-05 fires on most tasks in a fresh sprint. Velocity is a whole number every sprint.

**Phase to address:** Sprint Data Access (DATA-02), before any NAG or CEREMONY phase consumes the data.

---

### Pitfall 4: Effort field and state names hardcoded to one process template

**What goes wrong:**
The velocity field differs by process: Agile `Microsoft.VSTS.Scheduling.StoryPoints`, Scrum and Basic `Microsoft.VSTS.Scheduling.Effort`, CMMI `Microsoft.VSTS.Scheduling.Size`. The backlog order field differs too: Agile and CMMI use `Microsoft.VSTS.Common.StackRank`, and Scrum uses `Microsoft.VSTS.Common.BacklogPriority`. State names differ across processes: Agile New/Active/Resolved/Closed, Scrum New/Approved/Committed/Done, Basic To Do/Doing/Done, CMMI Proposed/Active/Resolved/Closed. Against a Scrum-process project, a StoryPoints-only reader returns `nil` points for every PBI. NAG-02 then flags the entire backlog, and velocity reports 0 with `complete: true`.

This codebase already disagrees with itself. `workitems.go:176` and `dashboard.go:151` exclude `('Done','Closed','Removed')`, while `doneStates` at `internal/ui/workitems/workitems.go:36` and the CEREMONY "Delivered" definition use Resolved/Closed. That mixes Scrum and Agile vocabularies.

**Why it happens:**
The design session fixed "Points = StoryPoints" and "Delivered = Resolved or Closed" against one known project. Config carries multiple projects, which may run different processes.

**How to avoid:**
- At team-foundation time, resolve per project which process is in use and which effort field applies. The cheapest honest check is `GetWorkItemTypeStates` plus field presence on the requirement-category type. If a configured project's requirement type lacks the configured points field, report a **config failure** naming the field. Never emit nulls that look like unestimated work.
- Derive "done" from **state categories** (`WorkItemStateColor.Category`: Proposed / InProgress / Resolved / Completed / Removed) via `workitemtracking.GetWorkItemTypeStates`, with `config.StateTransitions` as the declared flow for NAG-07. Keep "Delivered = Resolved or Closed" as the documented definition, implemented as "category Resolved or Completed".
- Put the points field and the done-state set used in `scope`.

**Warning signs:**
A project returns zero non-nil points across a sprint. A state appears in data that is in neither the configured transitions nor the done set.

**Phase to address:** Team Foundation (process and field resolution) and Sprint Data Access. NAG-07 depends on it.

---

### Pitfall 5: "Delivered" will not match the ADO velocity chart, and nobody will know why

**What goes wrong:**
ADO's velocity and burndown tooling counts only the **Completed** category. Each work item type has exactly one Completed state: Closed in Agile and CMMI, Done in Scrum and Basic. In Agile, `Resolved` on a Bug is in the Resolved category. In CMMI, Resolved on Requirement, Feature and Epic is in the **In Progress** category (per Microsoft's state-category mapping table). The CEREMONY definition counts Resolved as delivered. `sprint report` will therefore report higher velocity than the ADO Velocity widget, and the first stakeholder who compares them will distrust the tool.

**Why it happens:**
The definition was chosen deliberately to match `doneStates`, but the divergence from ADO's own number was not recorded as a consequence.

**How to avoid:**
Keep the definition, since it is a recorded decision, but make the divergence explicit. Emit `delivered_definition: ["Resolved","Closed"]` (or the category equivalent) in the sprint report's `scope`. Split nothing into extra fields, because CEREMONY-01 is capped at five. If the five-field cap is honored, put the definition in `scope`, not in the payload.

**Warning signs:**
A user says "ADO says 21, the tool says 34".

**Phase to address:** Ceremony Output (CEREMONY-01). Decide in its discuss-phase.

---

### Pitfall 6: Baseline reconstruction aimed at the wrong API (reporting revisions cannot answer "membership at instant T")

**What goes wrong:**
DATA-04 names `workitemtracking.ReadReportingRevisionsPost`. That endpoint:
- filters only by **work item types and fields**, with no filter by ID, iteration or area. Without the `project` route it is organization-wide.
- pages via `continuationToken` (a watermark) until `isLastBatch == true`. The SDK returns `Values *[]interface{}`, untyped, so every row is hand-decoded.
- takes `startDateTime`, which returns only revisions **after** that instant. It therefore cannot give the value of a field **at** the baseline for items whose last change predates it. Getting the baseline from this API means streaming the project's full revision history from the beginning of time.

The typical failures follow from that. The first implementation stops after one page (no `isLastBatch` loop) and silently reconstructs from a fraction of history. It uses `startDateTime = baseline` and misses every item that did not change after the baseline. Or it streams the whole project on every `sprint report` and burns the TSTU budget (Pitfall 9).

**Why it happens:**
"Revisions" sounds like the history API, and its name suggests reporting use. WIQL `ASOF` is less well known.

**How to avoid:**
- **Membership at baseline:** run WIQL with `ASOF '<baseline UTC ISO-8601 with Z>'` against the team's iteration. Point-in-time membership is exactly what ASOF exists for, and it is one cheap query. Pass `TimePrecision: true` so the 48-hour grace window is honored to the hour.
- **Points at baseline** ("re-pointed"): `GetWorkItems` / `GetWorkItemsBatch` with `AsOf` set to the baseline, in batches of 200.
- Use `ReadReportingRevisionsPost` only for "what changed since T" questions. That is **standup** (CEREMONY-02), where `startDateTime = since` is exactly right. Constrain it by `types` and a minimal `fields` list, and loop until `isLastBatch`.
- In the paging loop, treat a missing or empty continuation token with `isLastBatch == false` as a **failure**, not completion. Cap the page count, and mark the result incomplete when the cap is hit.
- Revisit the DATA-04 wording in its discuss-phase. The requirement names the API, but the outcome it wants is membership at baseline.

**Warning signs:**
The baseline set is smaller than the current sprint set, with added=0 every sprint. `sprint report` takes tens of seconds. `X-RateLimit-Remaining` drops sharply per run.

**Phase to address:** Sprint Data Access (DATA-04). Flag for deeper research and spike in that phase.

---

### Pitfall 7: Sprint dates are midnight UTC, so naive comparisons end the sprint a day early

**What goes wrong:**
`TeamIterationAttributes.StartDate/FinishDate` are documented as "Date-only, correct unadjusted at midnight in UTC" (SDK comment and REST docs). `FinishDate` is the **last day of the sprint, inclusive**. In US Central, `2026-10-20T00:00Z` is Oct 19 at 7pm. These defects follow:
- `now.After(finish)` declares the sprint over during its final day, and the evening before it in US time zones. NAG-10 ("PR open past finish") and carryover fire a day early.
- Days remaining (NAG-08) and elapsed working days (NAG-09) are off by one at both ends.
- The baseline is "start + 48h". Computed from a UTC midnight, it lands at 7pm local on day 2, not at end of day 2.
- Dates can be **null**. The REST sample for `$timeframe=current` shows `startDate: null`, for an iteration selected by a team but never dated. Dereferencing panics. Treating null as zero time makes every pace calculation nonsense.

**Why it happens:**
`azuredevops.Time` wraps `time.Time`. It looks like an instant but means a calendar date.

**How to avoid:**
- Convert iteration attributes to **civil dates** (y-m-d) immediately: take `.UTC().Date()`, never `.Local()`. Do all sprint math on dates. Define end-of-sprint as the end of `FinishDate` in a configured team time zone, and disclose that zone in `scope`.
- Null dates are a per-team failure ("iteration has no dates"), never a zero time.
- Table-driven tests in at least UTC, America/Chicago and Asia/Kolkata, run with `TZ` set, around the last day of a sprint.

**Warning signs:**
Results change depending on the machine's TZ. NAG-10 fires for PRs that are still in-sprint.

**Phase to address:** Team Foundation (TEAM-03 resolves the iteration) and Board Honesty (NAG-08/09/10).

---

### Pitfall 8: Working-day math that ignores `WorkingDays` format and team days off

**What goes wrong:**
`TeamSetting.WorkingDays` is `*[]string` in both SDK b5 and v7 (`"monday"`, …), not `time.Weekday`. Code that compares those strings case-sensitively, or assumes Mon–Fri, miscounts. Team days off (`work.GetTeamDaysOff`, holidays) are a separate call. The project decision says individual days off are not maintained, and team holidays may not be either. NAG-09 then predicts on-track or miss from inflated working days with no disclosure.

**How to avoid:**
- Parse WorkingDays case-insensitively. If a value is unknown, fail the team rather than default it.
- NAG-09 uses `WorkingDays` and team days off only, never capacity (SPRINT-03, Key Decision). It discloses `working_days_source` and whether any team days off were found.
- Burn pace needs remaining-work history. NAG-09 compares the **current** remaining work against linear elapsed working days. It does not try to reconstruct daily burndown, because that would be the reporting-revisions firehose again.

**Phase to address:** Board Honesty (NAG-09).

---

### Pitfall 9: Rate limiting surfaces as slowness and timeouts, not 429s, and the SDK hides the headers

**What goes wrong:**
Per the Microsoft rate-limit docs (updated 2026-07), each identity has a budget of 200 TSTUs per sliding 5-minute window. Over it, requests are first **delayed** while still returning **HTTP 200**, with delays from milliseconds up to 30 seconds and `Retry-After` / `X-RateLimit-Delay` headers. Only sustained abuse gets **429** with `TF400733`. Work item queries cost more as the org grows. With the pinned SDK:
- `azuredevops.Client.SendRequest` has `// todo: add retry logic` and no retry or `Retry-After` handling. Response headers on successful calls are never exposed to callers.
- The b5 `NewClient` builds `&http.Client{}` with a nil Transport, so it uses `http.DefaultTransport`. The only hook is `TlsConfig`, so there is no supported way to wrap a RoundTripper.
- v1.0 classifies 429 as `rate_limit`, but a delayed 200 that exceeds `RequestTimeout` is classified as `timeout`. The agent sees "timeout", retries harder, and deepens the throttling.

TEAM-04's multi-team default multiplies calls: teams × (iterations + settings + field values + WIQL + ⌈items/200⌉ batches + members + PR lookups). Run concurrently, that hits the delay regime in one invocation.

**How to avoid:**
- **Budget first, retry second.** Cap fan-out concurrency per invocation with a plain buffered-channel semaphore. That needs no new dependency, whereas `errgroup.SetLimit` would add `golang.org/x/sync`. Make the cap configurable with a conservative default (4–6). Note that `FetchTimeout` bounds the whole fan-out, so the cap and the timeout must be set together.
- **Honor `Retry-After` on 429 only for reads**, with a bounded number of attempts and a deadline check. A 429 (`TF400733`, "request was blocked") means the request was **not executed**, so retrying a write after a 429 is safe. A **timeout** on a write is not safe to retry, and must stay outcome-unknown under SAFE-04.
- For header access, the clean path is SDK v7 (Pitfall 13). v7 has `NewClientWithOptions(conn, url, WithHTTPClient(c))`, and the area clients expose `ClientImpl{Client: azuredevops.Client}`, so a custom `http.Client` with a header-reading RoundTripper can be injected per client. Do **not** mutate `http.DefaultTransport` globally in b5.
- When a throttle is observed, record `X-RateLimit-Resource` and the delay in the failure or stderr diagnostics. Mark the result incomplete if the cap or deadline cut the fan-out short. Never drop the unfinished teams silently.

**Warning signs:**
Rollups that pass for 2 teams time out for 6. Timeouts cluster late in a run. A banner or email from ADO about delayed requests.

**Phase to address:** Platform: rate limits and fan-out caps (PLAT-03). Order it **before or with** the first multi-team fan-out phase, not at the end of the milestone.

---

### Pitfall 10: Batch reads that drop items silently (`errorPolicy=Omit`, `$expand` vs `fields`, 200-ID cap)

**What goes wrong:**
- `GetWorkItems` accepts at most 200 IDs per call. The existing loop at `workitems.go:210-235` handles that and records batch failures.
- With the default `errorPolicy=Fail`, one deleted or permission-hidden ID fails the **whole batch** of 200. Switching to `Omit` to "fix" it returns `nil` entries for the bad IDs. Code that skips nils then reports fewer items with no failure, which is a silent shrink.
- `$expand=Relations` (needed for parent and PR links) cannot be combined with an explicit `fields` list. The service rejects the request. This is MEDIUM confidence, widely reported but not quoted in current docs. Developers then drop `fields`, fetch every field for every item, and multiply payload and TSTU cost.

**How to avoid:**
- If `Omit` is used, compare requested and returned IDs, and record the missing ones as a failure with their count. They are not "not found".
- Use the **`System.Parent`** field for NAG-04 ("task with no parent") and for DATA-03's parent-child grouping. It is a plain field, so `fields`-scoped batches still work, and relations are unnecessary.
- Fetch relations only where truly needed (NAG-06 PR artifact links), as a separate pass over the sprint's items.

**Phase to address:** Sprint Data Access (DATA-01/03) and Board Honesty (NAG-04/06).

---

### Pitfall 11: WIQL caps and truncation (20,000 results, `$top`, 32K text)

**What goes wrong:**
- Above **20,000** results, QueryByWiql fails with `VS402337`. This is loud, and good, but v1.0 classifies it as `FailureOther`, which reads as transient.
- Setting `$top` (`QueryByWiqlArgs.Top`) to avoid that **truncates silently**. The response carries no "more available" flag.
- WIQL text is capped at 32K characters. Building `IN (...)` lists of IDs or paths (as `iterationClause` does) breaks on large sets.
- NAG-13 runs readiness rules against the **backlog**. Read without a state filter, because DATA-01 removed it to see completed work, an older project's backlog plus completed history crosses 20,000 quickly.

**How to avoid:**
- Backlog-scope queries exclude Completed and Removed categories and filter by the team's area paths (Pitfall 2). Sprint-scope queries include all states but are bounded by iteration.
- If `Top` is ever set, request `Top+1`. When `Top+1` items come back, mark the result `complete: false` with a truncation failure.
- Map `VS402337` to an explicit, non-retryable failure that names the 20,000 limit.
- Never build ID lists into WIQL text. Fetch by ID via `GetWorkItems` batches instead.

**Phase to address:** Sprint Data Access, and Board Honesty for the NAG-13 backlog scope.

---

### Pitfall 12: An empty sprint looks the same as a missing sprint

**What goes wrong:**
`GetTeamIterations($timeframe=current)` returns an **empty list** when no iteration is current. This happens between sprints, when the team has no iterations selected, or when the backlog iteration is misconfigured. A scrum verb that loops over "current iterations" emits zero findings and zero points with `complete: true`. An agent reads that as "the board is healthy". This is the v1.0 W3 class of bug ("an empty projects list reports complete: true"; pipelines with none configured) recurring in every new verb.

**How to avoid:**
- Every scrum verb distinguishes "no current sprint for team X" (a per-team failure with a specific kind, or a finding) from "sprint with zero items" (complete, empty).
- Add a negative test per verb: no teams configured, team with no current iteration, iteration with null dates, iteration with zero items. Each asserts a distinct `complete` / `failures` / exit code. Lock them with the same exact-shape style as `json_test.go`.
- Make "zero configured teams" a usage/config error (exit 3), not an empty success. Do not repeat the W3 / pipelines mistake.

**Phase to address:** Team Foundation (TEAM-03), then repeated as an acceptance criterion in every verb phase.

---

### Pitfall 13: The SDK upgrade compiles cleanly, which hides what actually changes

**What goes wrong:**
**Spike result, HIGH confidence, verified 2026-10-13.** Rewriting every import from `azure-devops-go-api/azuredevops` (v1.0.0-b5) to `azure-devops-go-api/azuredevops/v7` at v7.1.0, in a scratch copy, gives a clean `go build`, `go vet` and **full `go test ./...` with zero failures**. The source footprint is 24 files across 8 sub-packages. The danger is that this looks done when it is not:
- **Every API version on the wire changes.** QueryByWiql goes from 5.1 to 7.1-preview.2, CreateWorkItem from 5.1 to 7.1-preview.3, Comments from 5.1-preview.3 to 7.1-preview.4, ReadReportingRevisions from 5.1 to 7.1-preview.2, and ReorderBacklogWorkItems from 5.1-preview.1 to 7.1-preview.1. All tests run against SDK-typed fakes, so **passing tests prove nothing about wire behavior**.
- **Half-migration is a core-value bug.** If both b5 and v7 are in `go.mod` during an incremental migration, `azuredevops.WrappedError` from b5 and from v7 are **different types**. `classifyFailure`'s `errors.As` in `internal/ado/result.go` silently stops matching, so every 401/403 is classified `other` instead of `auth`. Auth failure then renders as a partial result instead of being fatal, which is exactly TRUST-02 regressing.
- v7.1.0 was tagged **2023-04-17** and no newer release exists, so the "upgrade" is still to a dormant SDK. `govulncheck` is not installed locally, and CI must install it for the PLAT-02 gate to mean anything.
- v7 does not fix the mandatory resource-area discovery (the `g-01-1` offline-startup issue). `NewClient` still calls `GetClientByResourceAreaId`.
- `go mod tidy` in the spike also promoted `google/uuid` from `// indirect` to direct, which closes BL-02 IN-01 as a side effect, and moved `atotto/clipboard`. Review the go.mod diff as part of the change.

**How to avoid:**
- Swap **atomically in one commit**, with no dual-version window. Add a test that builds a v7 `WrappedError` with `StatusCode 401` and asserts `FailureAuth`.
- Make the live smoke test the gate, not CI: one read per sub-client, plus one dry-run of each write verb against a real org.
- Pin `govulncheck` in CI (`go install golang.org/x/vuln/cmd/govulncheck@<version>`) and fail the build on findings.
- Do it **early**. v7 is what unlocks markdown comments (Pitfall 15) and an injectable `http.Client` (Pitfall 9).

**Warning signs:**
A failure in v2.0 output shows `kind: "other"` with an HTTP 401 message.

**Phase to address:** Platform: SDK upgrade (PLAT-02). Recommend it as the first platform phase, right after BL-01.

---

### Pitfall 14: BL-01 cannot reuse `runWrite`, because the guards live in `package main`, coupled to stdout, flags and exit codes

**What goes wrong:**
The guarded path is `runWrite(ctx, op writeOp, args []string) (code int)` in `cmd/adoBizInt/write.go`. It parses flags, writes to `stderrWriter` and stdout, opens the audit sink and returns exit codes. The TUI cannot import `package main`. The likely shortcuts both fail:
- Calling the CLI as a subprocess from the TUI is slow and loses the alt-screen.
- Copying the guard logic into the TUI creates a second implementation that drifts, against the code standards. And anything the guard prints to **stdout** while Bubble Tea owns the terminal corrupts the TUI screen.

The BACKLOG also lists the TUI effort modal as "the only write surface with no automated coverage".

**How to avoid:**
- Extract the **pre-read → audit-append → send → outcome-classify** sequence into an importable package. It must take an `io.Writer` for diagnostics and return a typed outcome, not an exit code. The CLI keeps flag parsing and exit-code mapping. The TUI maps the outcome to a status line.
- Lock the CLI's behavior with the existing write tests **before** extracting, so the refactor is proven byte-identical (the D-17 precedent).
- The TUI's equivalent of `--confirm` is a modal confirmation step showing the same dry-run diff. Decide this explicitly in discuss-phase, per BACKLOG.
- Every new TUI write in this milestone must go through the extracted path from day one: GROOM-02 inline edits and GROOM-03 reorder. Otherwise BL-01 is reintroduced in the same milestone that closes it.

**Phase to address:** BL-01, the first phase of v2.0, before Grooming Mode exists.

---

## Moderate Pitfalls

### Pitfall 15: Comments are HTML in the pinned SDK, and the Comments API is still preview

**What goes wrong:**
SDK b5 `AddComment` sends `5.1-preview.3` with only `CommentCreate{Text}`, and there is no format parameter, so the text is stored as **HTML**. Agent text containing `<T>`, `a < b`, or a pasted stack trace loses content or renders as markup. Reading a comment back returns HTML that the agent must strip. Even at REST 7.1 the Comments API is `7.1-preview.4`. Preview versions can change shape and eventually get retired.
v7 adds `AddWorkItemComment` with `Format *CommentFormat` (markdown | html). It keeps the old `AddComment` too, so callers can still pick the wrong one.

**How to avoid:**
- On b5, HTML-escape agent text (`html.EscapeString`, stdlib) before sending, and say so in the audit record.
- On v7, use `AddWorkItemComment` with `Format: markdown`.
- Page `GetComments` via `continuationToken` until it is null. Check `count` against `totalCount`, and mark the result incomplete on mismatch.

**Phase to address:** Story Management (STORY-02), ideally after PLAT-02.

---

### Pitfall 16: Acceptance Criteria emptiness and AC writes (HTML, entities, and the markdown-format switch)

**What goes wrong:**
- **NAG-03 false negatives.** The existing `stripHTML` at `internal/ui/workitems/workitems.go:816` removes tags but **does not decode entities**. An AC of `<div>&nbsp;</div>` or `<p><br></p>`, which is common after someone clears the field in the web editor, strips to `&nbsp;`, a non-empty string. The story then passes the readiness check with no criteria.
- **Duplication.** NAG-03 needs the same stripping outside the UI package. Copying the function makes a second implementation, which the standards forbid.
- **Markdown fields.** Markdown in large text fields became GA in July 2025. A work item can be switched per field via the patch path `/multilineFieldsFormat/<field>` = `Markdown`, and **cannot be switched back**. Neither SDK version models `multilineFieldsFormat` on `WorkItem`, so the client cannot tell which format a given item's AC is in. If GROOM-02 writes HTML into a markdown-format AC, the item shows literal tags. If it writes plain text into an HTML AC, the text is interpreted as markup. This is MEDIUM confidence, from Microsoft DevBlogs.

**How to avoid:**
- Move `stripHTML` to a shared location, since it now has a second consumer, and extend it with `html.UnescapeString` (stdlib `html`, no new dependency). Treat NBSP and zero-width characters as whitespace. Add tests for `<div>&nbsp;</div>`, `<p><br></p>`, `<div></div>` and markdown-only content.
- GROOM-02 AC edit: HTML-escape user text and wrap it in `<div>`, preserving the HTML default. Never set `multilineFieldsFormat`, because it is irreversible. Disclose this in the edit confirmation.

**Phase to address:** Board Honesty (NAG-03) and Grooming Mode (GROOM-02).

---

### Pitfall 17: Backlog reorder against the wrong rank semantics

**What goes wrong:**
`ReorderBacklogWorkItems` is team-routed: `PATCH {project}/{team}/_apis/work/workitemsorder`. It takes `ids`, `previousId`, `nextId`, `parentId` and `iterationPath`, where 0 means start or end, and `iterationPath` applies only on an iteration backlog. Common mistakes:
- Reading order from the wrong field. Agile and CMMI use `StackRank`, while Scrum uses `BacklogPriority`. Both are background-maintained doubles with gaps, so equal or nil values are normal, and items never ranked show nil.
- Sending `previousId`/`nextId` from a **stale local list**. If someone reordered in the web UI meanwhile, the item lands somewhere unintended.
- Omitting `parentId` for items under a parent on a hierarchical backlog.
- Treating reorder as reversible without recording the prior neighbours.

**How to avoid:**
- Resolve the rank field per process (Pitfall 4). Re-read neighbours' ranks immediately before the PATCH, as the guarded path's pre-read. Abort if the pre-read order differs from what the user saw.
- Audit-record the previous and next neighbours before and after, so a reorder can be undone by hand.
- Sort with nil rank last, and break ties by ID, to match ADO's display.
- Reorder is a write. It goes through the extracted BL-01 guard path. Decide in AGENT tiers whether the agent tier gets it (AGENT-02 does not list it, so default human-only).

**Phase to address:** Grooming Mode (GROOM-03).

---

### Pitfall 18: Link removal by relation index races concurrent edits

**What goes wrong:**
STORY-03 "remove link" is a JSON Patch `remove` on `/relations/<index>`. If any relation was added or removed between the read and the PATCH, by a person, a PR auto-link or a rule, the index points at a **different link**. The wrong link is deleted and the server reports success.

**How to avoid:**
- Read the item with relations, locate the relation by `(rel, url)`, and send `{"op":"test","path":"/rev","value":<rev>}` **before** the `remove`. The server then rejects the patch if the item changed.
- Treat a test-op failure as "nothing written, re-read and retry by human". For create-link, check for an existing identical relation first, because ADO rejects duplicates with an error that should not be classified as unknown outcome.

**Phase to address:** Story Management (STORY-03).

---

### Pitfall 19: Created work items land outside the team's view

**What goes wrong:**
`CreateWorkItem` without `System.AreaPath` and `System.IterationPath` defaults both to the **project root**. A task the agent creates for NAG-12 ("missing Development task") is then outside the team's area. It is invisible on the team's taskboard and excluded from the team's sprint read (Pitfall 2), so NAG-12 keeps firing, and the agent may create another one. That retry is non-idempotent.

**How to avoid:**
- Child tasks inherit the parent story's AreaPath and IterationPath, read in the pre-read, plus a `System.LinkTypes.Hierarchy-Reverse` relation to the parent in the **same** create patch.
- Use `validateOnly=true` (an existing `CreateWorkItemArgs.ValidateOnly`) as the server-side dry run. It is better than a client-only preview.
- Before creating, check for an existing child whose title matches the Development convention (`findDevelopmentTask`). Creation is not retryable after a timeout (SAFE-04 outcome-unknown), so a re-run must detect the earlier success.

**Phase to address:** Story Management (STORY-01) and Agent Authorization (AGENT-02 permits task creation).

---

### Pitfall 20: Agent tiers enforced by something the agent controls

**What goes wrong:**
AGENT-04 says "enforced in the CLI surface". If the tier is chosen by a flag, an environment variable or a config key, the same agent that is constrained can set it, because it has shell and file access. AGENT-03's human-only list is then a convention, the very thing AGENT-04 rejects. A second trap is enforcing by **verb name** while a generic field-update path exists. A "story points" verb built on the same PATCH helper as state changes can be made to carry `System.State`.

**How to avoid:**
- Enforce **by field allow-list at the write layer**, not by verb. Agent-tier writes may touch only `StoryPoints/Effort/Size`, `AssignedTo`, `IterationPath`, comments, creation and links. Any patch op on another path is rejected before audit and send.
- Be honest in the design about what a CLI boundary can stop. It stops a **cooperating** agent from using a human-only verb. It does not stop an adversarial process with the same PAT, because ADO PAT scopes are coarse (`vso.work_write` covers state changes). A true boundary is a separate identity or PAT with narrower ADO permissions. Record this as a stated limitation, or as an open question for the AGENT discuss-phase.
- Make human-only verbs require something an agent cannot pass by flag. For example, require an interactive TTY confirmation, and refuse when stdin is not a terminal. Decide this explicitly. It is a UX trade-off for human scripting.

**Phase to address:** Agent Authorization (AGENT-01..04). It must land **before** Story Management exposes creation and links to the agent.

---

### Pitfall 21: Standup "what moved" computed from `ChangedDate`

**What goes wrong:**
`System.ChangedDate` is bumped by comments, link additions, PR auto-links, rule-driven field updates and bulk edits. "Changed since T" is therefore not "moved since T", and standup reports noise as progress. WIQL date comparisons without `timePrecision=true` are **day-granular**, and date literals without a zone are interpreted in the client's local time zone (WIQL reference), so "since 9am" silently means "since midnight, some zone".

**How to avoid:**
- Use WIQL to narrow candidates, with `ChangedDate >= since`, `TimePrecision: true` and an ISO-8601 literal with `Z`. Then use `ReadReportingRevisionsPost` with `startDateTime = since` and `fields = [State, RemainingWork, CompletedWork, AssignedTo, IterationPath]` to compute actual moves (a state transition, or a remaining-work delta).
- "Did not move" is the sprint set minus the moved set, grouped by **current** assignee. Disclose that reassignment during the window is attributed to the new owner.

**Phase to address:** Ceremony Output (CEREMONY-02).

---

### Pitfall 22: Team membership returns groups, and identity matching by display name

**What goes wrong:**
`GetTeamMembersWithExtendedProperties` pages via `Top`/`Skip` and returns `TeamMember` entries whose identity can be a **group** (`IsContainer`). Expansion is not automatic. A team whose membership is an AAD group shows **zero** people for per-person load (SPRINT-02), with `complete: true`. Separately, `AssignedTo` on work items is an identity object, and in b5 the field arrives as a map. Matching people by display name breaks on duplicate names and renames.

**How to avoid:**
- Page until a short page comes back. Flag container members in the output, as "membership includes group X, not expanded". Do not silently drop them.
- Key people by identity id or `uniqueName`, never by display name. GROOM-02 assignee edits pick from resolved team members only, never from a free-text name.

**Phase to address:** Team Foundation (TEAM-02).

---

### Pitfall 23: The `Failure` contract has no team field

**What goes wrong:**
`Failure` is locked to four JSON keys (`project`, `operation`, `kind`, `error`) by an exact-key-set test. Scrum failures are per **team**. Squeezing `"Project/Team"` into `project` breaks consumers that match project names. Adding `team` silently is exactly what the contract test exists to prevent.

**How to avoid:**
Make a deliberate contract change in the Team Foundation phase: add `team` (omitempty), update `json_test.go` in the same change, and note it as a v2 output-contract change. Add per-verb `scope` fields (team, area paths, iteration id and dates, baseline instant, points field, done set) the same way.

**Phase to address:** Team Foundation.

---

## Minor Pitfalls

- **Iteration path formats differ by API.** `TeamSettingsIteration.Path` and `System.IterationPath` are `Project\Sprint 5`. Classification-node APIs return `\Project\Iteration\Sprint 5`. Mixing them makes every WIQL comparison match nothing, which is complete and empty. Prevention: normalize at one boundary, and test with nested iteration paths (`Project\2026\Q4\Sprint 5`).
- **Bugs behavior per team.** `TeamSetting.BugsBehavior` (Off / AsRequirements / AsTasks) changes whether Bugs count as backlog items, and therefore whether they carry points or act as tasks. Disclose it in `scope`. A readiness rule that ignores it nags bugs for missing points when they are tracked as tasks.
- **The 183-day hidden-item rule.** ADO boards hide completed items whose ChangedDate is older than 183 days, but queries do not. Velocity series that reach back far enough are correct via WIQL, but will not match what users see on the board. This is not a bug. Note it in docs.
- **`@CurrentIteration` offsets.** `@CurrentIteration - 1` with the team route gives the previous sprint without interpolating a path, which is useful for velocity series and carryover (N to N+1). It is **only** valid with team context (Pitfall 1).
- **PR-merged detection (NAG-06) as N+1.** Calling `GetPullRequestWorkItemRefs` per PR across all repositories is a fan-out explosion. Instead, read `ArtifactLink` relations (`vstfs:///Git/PullRequestId/...`) on the sprint's items, then fetch only those PRs by ID, with a cap and an incomplete marker.
- **Velocity series fetch.** Emitting N sprints of raw velocity means N ASOF or iteration queries per team. Cap N (config), and say in `scope` how many sprints were requested and returned.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Extend `iterationClause` string-quoting for area paths and team names | One-line WIQL changes | Widens the HARD-04 deviation from the org's parameterized-query rule; 32K-char cap on large lists | Never for user-supplied values. Server-sourced paths only, with a recorded deviation. Prefer macros plus client-side filtering |
| Reuse v1 `WorkItem` (int points, no effort) for sprint verbs | No new type | Pitfall 3: unknown values become 0 in every rule and report | Never |
| Hardcode StoryPoints and Resolved/Closed | Matches the one known project | Silent nulls and zeros on any Scrum/Basic/CMMI project | Only if config validation hard-fails non-matching projects at startup |
| Retry everything on any error | Fewer flaky runs | Duplicate creates and comments; hides auth failures; deepens throttling | Never. Retry reads on 429 only, honoring Retry-After |
| Copy `stripHTML` into the nag package | Avoids touching the UI package | Two diverging HTML strippers; entity bug fixed in one only | Never. Move it, it now has two consumers |
| Mutate `http.DefaultTransport` to read rate-limit headers on b5 | Header access without the SDK upgrade | Affects every HTTP client in the process; brittle | Never. Do PLAT-02 and use v7 `WithHTTPClient` |
| Stream reporting revisions for the baseline | Matches the DATA-04 wording | TSTU burn, slow reports, partial history on early exit | Never for the baseline. Fine for standup with `startDateTime` |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `work.GetTeamIterations` | `Team` nil gives the default team; empty list read as an empty sprint | Required team; empty is a per-team "no current sprint" failure |
| `QueryByWiql` + `@CurrentIteration` | Macro without team context | Set `QueryByWiqlArgs.Team`; disclose team in scope |
| `QueryByWiql` ASOF / dates | Local-zone or day-precision literals | ISO-8601 with `Z`, `TimePrecision: true` |
| `GetWorkItems` | `errorPolicy=Omit` drops IDs silently; `$expand` with `fields` rejected | Compare requested and returned IDs; use `System.Parent` instead of relations |
| `ReadReportingRevisionsPost` | One page only; `startDateTime` used for baseline; untyped `Values` | Loop until `isLastBatch`; use only for since-T deltas; strict decode with failure on unknown shape |
| `GetComments` / `AddComment` | HTML vs markdown; one page only | Escape on b5, `Format: markdown` on v7; page via continuationToken; check `totalCount` |
| `ReorderBacklogWorkItems` | Stale neighbours; wrong rank field per process | Pre-read neighbours; process-aware rank field; audit before and after |
| `CreateWorkItem` | Defaults to project root area and iteration | Inherit from parent; `validateOnly` for dry run |
| JSON Patch relations | `remove /relations/N` without a rev test | `test /rev` op first |
| Rate limits | Expecting 429; ignoring delayed 200s | Concurrency cap; Retry-After on 429 for reads; header capture via v7 http.Client |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-story `GetChildWorkItems` (N+1) | Sprint read takes one call per story | DATA-03: one iteration query per team, group by `System.Parent` | ~30+ stories per sprint × teams |
| Unbounded team fan-out | Timeouts late in run; ADO delay banner | Semaphore cap; `FetchTimeout` sized to cap × calls | ~4+ teams concurrently |
| Reporting-revisions firehose | Multi-second reports; TSTU drop | WIQL ASOF and `GetWorkItems AsOf` for baseline | Any project with years of history |
| Per-PR work-item-ref lookups | Hundreds of calls for NAG-06 | Artifact links on sprint items, then PR by ID | ~50+ PRs per sprint window |
| Velocity series of N sprints | Linear growth in calls per team | Cap N; `@CurrentIteration - k` with team route | N > 6 with several teams |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Agent tier selectable by flag, env or config the agent can edit | AGENT-03 human-only writes reachable by the agent | Field allow-list at the write layer; TTY requirement for human-only verbs; document the PAT-scope limit (Pitfall 20) |
| Agent-supplied comment or AC text sent as raw HTML | Markup injection into work items; content loss | `html.EscapeString` on b5; markdown format on v7 |
| Logging WIQL text containing area paths or team names | Low, but D-11 already forbids logging query text | Keep D-11 for every new query |
| Rate-limit diagnostics echoing request URLs or headers into `Failure` | T-02-01 violation (PAT or URL leak) | Record only `X-RateLimit-Resource` and delay; never the request |
| Audit record for creates and comments containing full agent text | Audit file is 0644 (BL-02) and may hold sensitive text | Fix audit file mode with BL-01; consider hashing long bodies |
| CPNI / customer data in work item text echoed to agent JSON | Regulatory exposure if items contain customer details | No new risk from structure, but standup and comments surface free text. Keep output to fields the verb needs; flag for review if items carry customer data |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Sprint report delivered ≠ ADO velocity widget | Loss of trust in the tool | Disclose the delivered definition in `scope` (Pitfall 5) |
| Nag findings with no team or rule context | Agent cannot act | NAG-11: team, item, rule, suggested action, and the threshold used |
| Grooming edit that succeeds locally but writes stale data | Silent overwrite of a teammate's change | Pre-read the rev; `test /rev` op; show the diff in the confirm modal |
| Capacity shown without the unreliable label | Users reason from overstated capacity | SPRINT-03: label it in the view and in JSON |
| Rollup hides teams that failed | Looks like fewer teams, not a broken one | Failed teams render as rows with the failure kind |

## "Looks Done But Isn't" Checklist

- [ ] **Team foundation:** a test where `Team` is nil fails to compile or is rejected. An unknown configured team exits 3, not 0.
- [ ] **Team scoping:** two teams sharing an iteration path produce disjoint item sets. An overlapping area produces a duplicate finding.
- [ ] **Sprint read:** includes Completed-category items. Points and effort are `null` when absent, and 0.5 survives the round trip.
- [ ] **Process awareness:** a Scrum-process fixture (Effort, Done, BacklogPriority) produces correct points and done counts, or a named config failure.
- [ ] **Baseline:** uses ASOF at start + grace window, in UTC with time precision. A test item that changed before the baseline and never after is in the baseline set.
- [ ] **Dates:** tests pass under `TZ=America/Chicago` and `TZ=Asia/Kolkata`. The last day of the sprint counts as in-sprint. Null dates produce a failure.
- [ ] **Every new verb:** negative tests for no teams, no current sprint, an empty sprint, one team failing, and a 429 mid-fan-out. Each has a distinct `complete` / `failures` / exit code.
- [ ] **Reporting revisions (standup):** loops until `isLastBatch`. An empty token with `isLastBatch=false` is a failure.
- [ ] **Rate limits:** a fake returning 429 with `Retry-After` is retried for reads only. A write timeout is still outcome-unknown.
- [ ] **SDK upgrade:** a single `azure-devops-go-api` module in `go.mod`. A test asserts a v7 401 classifies as `auth`. govulncheck runs in CI. A live smoke test covers each sub-client.
- [ ] **BL-01:** the TUI effort modal, grooming edits and reorder all produce `audit.jsonl` records. CLI write tests are unchanged byte-for-byte.
- [ ] **NAG-03:** `<div>&nbsp;</div>` is flagged as empty.
- [ ] **Agent tier:** a patch containing `/fields/System.State` is rejected on the agent path even when sent through the story-points verb.
- [ ] **STORY-03:** link removal sends a `test /rev` op. Removing by stale index fails safely.
- [ ] **STORY-01:** an agent-created task inherits the parent's area and iteration, and appears in the team's next sprint read.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Wrong-team or default-team data already emitted to decks | MEDIUM | Re-run with team validation. Reports are JSON snapshots, so regenerate the affected sprints. Add the `scope.team` check to the deck pipeline |
| Points truncated or zeroed in emitted velocity series | LOW | Data in ADO is intact. Fix the reader and regenerate the series |
| Baseline built from partial revision history | LOW | Switch to ASOF and recompute. Nothing was written |
| Duplicate tasks or comments from a retried write | MEDIUM | The audit log lists each send. Delete duplicates by hand in ADO, and add the existence check before create |
| Wrong link removed by a stale index | MEDIUM | Work item History shows the removed relation. Re-add it by hand, and add the `test /rev` op |
| Half-migrated SDK misclassifying auth | LOW if caught in CI, HIGH if shipped | Revert to a single module version. Add the v7 `WrappedError` classification test |
| TUI writes outside audit (BL-01 regression) | HIGH | No local record exists. ADO History is the only source of truth, the same open question as BL-01 today |
| AC written in the wrong format (markdown switch) | HIGH | The markdown switch is irreversible per field. Never set `multilineFieldsFormat` from this tool |

## Pitfall-to-Phase Mapping

Phase names follow the v2.0 requirement groups. The suggested order puts BL-01 and the platform work before the first multi-team fan-out.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 14 BL-01 guards in `package main` | BL-01 (first) | Extracted guard package; CLI tests byte-identical; TUI modal writes produce audit records |
| 13 SDK upgrade hidden behavior changes | Platform: SDK upgrade (second) | Single module; v7 auth-classification test; govulncheck in CI; live smoke test per sub-client |
| 9 Rate limiting, fan-out | Platform: rate limits (before or with Team Foundation fan-out) | 429/Retry-After fake test; concurrency cap test; incomplete marker when cap or deadline cuts the run |
| 1 Default-team substitution | Team Foundation | Required team param; unknown-team exit 3; `scope.team` |
| 2 Shared iteration paths | Team Foundation + Sprint Data | Disjoint per-team sets in tests; overlap finding |
| 4 Process field and state differences | Team Foundation | Scrum fixture test; config failure on missing field |
| 12 Empty vs missing sprint | Team Foundation, then every verb | Negative-test matrix per verb |
| 22 Membership groups and identity | Team Foundation | Group member flagged; paging test |
| 23 Failure contract lacks team | Team Foundation | `json_test.go` updated in the same change |
| 3 Missing values read as zero | Sprint Data Access | Nullable fields; absent / 0 / 0.5 fixtures |
| 6 Baseline via the wrong API | Sprint Data Access (spike recommended) | ASOF-based baseline test; no full-history stream |
| 10 Batch silent drops | Sprint Data Access | Requested-vs-returned ID check |
| 11 WIQL caps | Sprint Data Access + Board Honesty (NAG-13) | Top+1 truncation test; VS402337 mapping |
| 7 UTC-midnight dates | Team Foundation + Board Honesty | Multi-TZ tests; null-date failure |
| 8 Working days | Board Honesty (NAG-09) | WorkingDays parse test; disclosure in scope |
| 16 AC HTML emptiness | Board Honesty (NAG-03) | Entity-only AC flagged |
| 5 Delivered ≠ ADO velocity | Ceremony Output | `scope.delivered_definition` present |
| 21 Standup from ChangedDate | Ceremony Output | Revision-based move detection test |
| 16 AC writes, 17 reorder | Grooming Mode | Rev-checked writes; neighbour pre-read; guarded path |
| 20 Agent tier bypass | Agent Authorization (before Story Management) | Field allow-list test; human-only verb refuses non-TTY (if adopted) |
| 15 Comments format, 18 link races, 19 create defaults | Story Management | Escape or markdown test; `test /rev` op; inherited area and iteration |

## Sources

**Verified locally (HIGH):**
- Pinned SDK source in the module cache: `azure-devops-go-api/azuredevops@v1.0.0-b5`. Covers API version strings per method, `ReportingWorkItemRevisionsBatch.Values *[]interface{}`, `TeamIterationAttributes` date comment, `WorkingDays *[]string`, `SendRequest // todo: add retry logic`, `NewClient` transport construction, `AddCommentArgs` without format, `GetAllTeamsArgs`, `ReorderOperation`.
- `azure-devops-go-api/azuredevops/v7@v7.1.0` (tagged 2023-04-17). Covers `NewClientWithOptions` / `WithHTTPClient`, `AddWorkItemCommentArgs.Format`, 7.1-preview API versions, and no `multilineFieldsFormat` on `WorkItem`.
- Spike, 2026-10-13: import rewrite to v7.1.0 in a scratch copy. `go build`, `go vet` and `go test ./...` all pass with 0 failures. 24 source files import the SDK.
- Repo code: `internal/ado/workitems.go:176,210-261`, `internal/ado/client.go:289` (`iterationClause`), `internal/ado/result.go` (`classifyFailure`, 4-key `Failure`), `internal/ui/workitems/workitems.go:816` (`stripHTML`), `cmd/adoBizInt/write.go:137` (`runWrite`).

**Official docs (MEDIUM, cross-checked against SDK signatures):**
- [Rate and usage limits, Azure DevOps](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits) (updated 2026-07): 200 TSTU per 5 min, delayed 200s, 429 TF400733, X-RateLimit-* headers.
- [WIQL syntax reference](https://learn.microsoft.com/en-us/azure/devops/boards/queries/wiql-syntax): 32K limit, ASOF, macros, `@CurrentIteration('[Project]\Team')`, date-literal time zones.
- [Read Reporting Revisions Post (7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/reporting-work-item-revisions/read-reporting-revisions-post?view=azure-devops-rest-7.1): filter fields, continuationToken and startDateTime, isLastBatch.
- [Comments: Get Comments (7.1-preview.4)](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/get-comments?view=azure-devops-rest-7.1): paging, markdown/html format.
- [Work Items: List (7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/list?view=azure-devops-rest-7.1): 200-ID max, errorPolicy Fail/Omit, asOf.
- [Iterations: List (7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/work/iterations/list?view=azure-devops-rest-7.1): optional team, only `current` timeframe, null dates in sample.
- [Reorder Backlog Work Items (7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/work/workitemsorder/reorder-backlog-work-items?view=azure-devops-rest-7.1).
- [Workflow and state categories](https://learn.microsoft.com/en-us/azure/devops/boards/work-items/workflow-and-state-categories): category mapping per process, one Completed state per type, 183-day hiding.
- [Query by numeric fields](https://learn.microsoft.com/en-us/azure/devops/boards/queries/query-numeric) and [Team velocity](https://learn.microsoft.com/en-us/azure/DevOps/report/dashboards/team-velocity?view=azure-devops): StoryPoints / Effort / Size per process.
- [Query by rank and picklist value](https://learn.microsoft.com/el-gr/azure/devops/boards/queries/planning-ranking-priorities?view=azure-devops): StackRank vs BacklogPriority.
- [Markdown support arrives for work items (DevBlogs, 2025-07-07)](https://devblogs.microsoft.com/devops/markdown-support-arrives-for-work-items/): `/multilineFieldsFormat/<field>`, irreversible.

**Community (LOW to MEDIUM):**
- VS402337 20,000-result limit: [Microsoft Q&A](https://learn.microsoft.com/en-us/answers/a/12251493), [ZappySys guide](https://zappysys.com/api/integration-hub/azure-devops-connector/help/odbc/sql-examples/read-more-than-20000-work-items-union-all).
- `$expand` and `fields` mutual exclusion on GetWorkItems: widely reported by API wrappers. The exact error text was not found in current docs (MEDIUM).
- `@CurrentIteration` via REST needing team context: [Query by date or current iteration](https://learn.microsoft.com/en-za/azure/devops/boards/queries/query-by-date-or-current-iteration?view=azure-devops).

**Not verified (LOW, research in phase):**
- What `$timeframe=current` returns in a gap between sprints (empty vs nearest). Treat empty as a possibility regardless.
- Whether `GetTeams` and `GetTeamMembersWithExtendedProperties` default `$top` is 100. Page defensively either way.
- Server behavior when HTML is written to a markdown-format AC field.

---
*Pitfalls research for: scrum analytics and grooming on an Azure DevOps Go client (adoBizInt v2.0)*
*Researched: 2026-10-13*
