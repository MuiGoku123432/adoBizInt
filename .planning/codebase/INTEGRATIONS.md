# External Integrations

**Analysis Date:** 2026-09-03

## Summary

There is exactly one remote external system: **Azure DevOps Services**. Everything else the application touches is local -- the filesystem under `~/.config/adoBizInt/`, the system clipboard, and the OS default browser.

No database, no message queue, no cache, no object store, no telemetry backend, no secondary API. `go.mod` contains no client library for any of those.

## Remote Systems

### Azure DevOps Services

| Attribute | Value |
|-----------|-------|
| SDK | `github.com/microsoft/azure-devops-go-api/azuredevops` v1.0.0-b5 (`go.mod:10`) |
| Transport | HTTPS to `Config.OrgURL`, e.g. `https://dev.azure.com/{org}` |
| Connection constructor | `azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)` at `internal/ado/client.go:35` |
| Auth | Personal Access Token, HTTP Basic. The SDK precomputes `connection.AuthorizationString` as `Basic base64(:PAT)`. |
| Connection lifetime | One `*azuredevops.Connection`, created once in `NewClient`, stored at `internal/ado/client.go:21`, shared by all sub-clients for process lifetime. |
| API version | Not pinned by this codebase. Whatever the SDK's generated route builders emit. |

**Sub-clients instantiated in `NewClient` (`internal/ado/client.go:34-98`):**

| Sub-client | REST area | Constructed at | Stored at | Used? |
|------------|-----------|----------------|-----------|-------|
| `workitemtracking.Client` | Work Item Tracking | `client.go:39` | `client.go:87` | Yes |
| `git.Client` | Git (repos, refs, PRs) | `client.go:44` | `client.go:89` | Yes |
| `build.Client` | Build (YAML/classic pipelines) | `client.go:49` | `client.go:90` | Yes |
| `work.Client` | Work (boards, sprints, capacity) | `client.go:54` | `client.go:88` | **No -- dead field** |
| `release.Client` | Release Management (classic) | `client.go:59` | `client.go:91` | Yes |
| `location.Client` | Location / Connection Data | `client.go:66` | not stored (local var) | Yes |

**The `work.Client` dead field.** `workClient` is declared at `internal/ado/client.go:23`, constructed at `internal/ado/client.go:54`, and assigned into the struct at `internal/ado/client.go:88`. A repo-wide grep for `workClient` returns exactly those three lines and nothing else. It is never read. Consequences:

- One wasted `work.NewClient` call at every startup. That constructor performs resource-area discovery against the org, so this is a real HTTP round trip, not just an allocation.
- If that call fails, `NewClient` returns an error and `main.go:49` exits the process -- startup can fail because of a client nobody uses.
- The Work API is the surface for iterations/sprints, team capacity, board columns, and backlog ordering. Its presence suggests sprint/board features were scoped but never built. `GetRecentIterations` reaches iteration data through `workitemtracking.GetClassificationNode` instead (`internal/ado/client.go:127`), which is a valid but different route.

**`location.Client` is constructed twice**, in two different places, and never cached:
- `internal/ado/client.go:66` during startup, to resolve `AuthorizedUser.ProviderDisplayName` into `currentUser`.
- `internal/ado/pullrequests.go:154` on every single PR approval, to resolve `AuthenticatedUser.Id` into the reviewer ID.

Note the two calls read *different* fields of the same `GetConnectionData` response: `AuthorizedUser` (startup) vs `AuthenticatedUser` (approve). The user ID needed by `ApprovePullRequest` is available at startup and could be cached on the struct, but is not.

## Complete Method Inventory

`ado.Client` exposes **25** public methods. The briefing supplied to this analysis stated 26; the verified count from `internal/ado/*.go` is 25, and all 25 are listed below. Two additional exported package-level functions exist and are not methods: `ado.NewClient` (`client.go:34`) and `ado.GetNextState` (`workitems.go:510`).

### Group 1 -- `workitemtracking.Client` (6 methods)

| Method | Defined at | SDK calls issued |
|--------|-----------|------------------|
| `GetRecentIterations(ctx, project)` | `client.go:121` | `GetClassificationNode` (`client.go:127`), `StructureGroup=Iterations`, `Depth=10` |
| `GetWorkItems(ctx, projects)` | `workitems.go:56` | Per project via `getProjectWorkItems`: `QueryByWiql` (`workitems.go:112`) then batched `GetWorkItems` (`workitems.go:151`) |
| `UpdateWorkItemState(ctx, itemID, newState)` | `workitems.go:215` | `UpdateWorkItem` (`workitems.go:228`) -- 1-op JSON Patch on `/fields/System.State` |
| `UpdateWorkItemStateWithEffort(ctx, itemID, newState, orig, rem, done)` | `workitems.go:244` | `UpdateWorkItem` (`workitems.go:278`) -- 4-op JSON Patch: OriginalEstimate, RemainingWork, CompletedWork, then State |
| `GetWorkItemDetail(ctx, id)` | `workitems.go:294` | `GetWorkItem` (`workitems.go:300`) with `Expand=Relations` |
| `GetChildWorkItems(ctx, parentID)` | `workitems.go:412` | `GetWorkItem` (`workitems.go:417`) to read relations, then `GetWorkItems` (`workitems.go:465`) for the child IDs |

**WIQL usage.** Two WIQL queries exist, both assembled by raw string concatenation:
- `internal/ado/workitems.go:104` -- selects Id, Title, Description, StoryPoints, WorkItemType, State, AssignedTo; excludes states Done/Closed/Removed; optionally filters `[System.IterationPath] IN (...)`.
- `internal/ado/dashboard.go:92` -- same predicate, selects Id only, used for counting.

The project name is interpolated directly into the WHERE clause with no escaping at both `workitems.go:104` and `dashboard.go:92`. Iteration paths *are* escaped (single-quote doubling) at `dashboard.go:86` and the equivalent site in `workitems.go`. The project name comes from local config rather than untrusted input, so this is an inconsistency in the code rather than a live exploit path.

**Batching.** Work item detail fetches are chunked at 200 IDs per request (`internal/ado/workitems.go:139-140`), matching the documented Azure DevOps limit. A failed batch is logged and skipped (`workitems.go:158`), so a partial result is returned as if complete. `GetChildWorkItems` does **not** batch (`workitems.go:465`) -- it passes the full child ID slice in one call and will fail if a parent has more than 200 children.

### Group 2 -- `git.Client` (5 methods)

| Method | Defined at | SDK calls issued |
|--------|-----------|------------------|
| `GetBranches(ctx, projects, repositories)` | `branches.go:32` | `GetRepositories` per project (`branches.go:48`), then `GetBranches` per repository (`branches.go:71`) |
| `GetPullRequests(ctx, projects)` | `pullrequests.go:35` | `GetPullRequestsByProject` per project (`pullrequests.go:46`), `Status=Active` |
| `ApprovePullRequest(ctx, project, repositoryID, prID)` | `pullrequests.go:150` | `location.GetConnectionData` (`pullrequests.go:155`), then `CreatePullRequestReviewer` (`pullrequests.go:172`) with `Vote=10` |
| `CompletePullRequest(ctx, project, repositoryID, prID)` | `pullrequests.go:190` | `GetPullRequest` (`pullrequests.go:194`) to read `LastMergeSourceCommit`, then `UpdatePullRequest` (`pullrequests.go:229`) |
| `GetBranchesForPipeline(ctx, project, repositoryID)` | `pipelines.go:319` | `GetRefs` (`pipelines.go:322`) with `Filter="heads/"` |

**Merge semantics are hardcoded** in `CompletePullRequest`: `MergeStrategy=NoFastForward` (`pullrequests.go:213`), `DeleteSourceBranch=false` (`pullrequests.go:212`), `Status=Completed` (`pullrequests.go:214`). None of these are configurable. The `LastMergeSourceCommit` round trip is required by the API as an optimistic-concurrency token; if the PR was updated between the read and the write, ADO rejects the merge.

**Approve vote value** is the literal `10` (`pullrequests.go:167`), which is ADO's "Approved". The other vote values (5 approve-with-suggestions, 0 no-vote, -5 waiting, -10 rejected) are not exposed.

### Group 3 -- `build.Client` (3 methods)

| Method | Defined at | SDK calls issued |
|--------|-----------|------------------|
| `GetPipelineRuns(ctx, projects)` | `pipelines.go:91` | `GetDefinitions` per project (`pipelines.go:109`), then `GetBuilds` per matching definition (`pipelines.go:160`) with `Top=1`, `QueryOrder=StartTimeDescending` |
| `GetPipelineDefinitions(ctx, projects)` | `pipelines.go:231` | `GetDefinitions` per project (`pipelines.go:248`), then `GetDefinition` per matching definition (`pipelines.go:295`) to resolve repository details |
| `QueuePipeline(ctx, project, definitionID, branch)` | `pipelines.go:349` | `QueueBuild` (`pipelines.go:364`); normalizes the branch to `refs/heads/...` at `pipelines.go:353-355` |

Both read methods short-circuit and return empty when `c.pipelines` is empty (`pipelines.go:95`, `pipelines.go:235`). Pipeline monitoring is opt-in via the `pipelines` config key.

### Group 4 -- `release.Client` (4 methods)

| Method | Defined at | SDK calls issued |
|--------|-----------|------------------|
| `GetReleases(ctx, projects)` | `releases.go:56` | `GetReleaseDefinitions` per project (`releases.go:68`), then `GetReleases` per matching definition (`releases.go:121`) with `Top=1`, `QueryOrder=Descending`, `Expand=Environments` |
| `GetPendingApprovals(ctx, projects)` | `releases.go:201` | `GetApprovals` per project (`releases.go:214`) |
| `ApproveRelease(ctx, project, approvalID, comment)` | `releases.go:272` | `UpdateReleaseApproval` (`releases.go:281`) with `Status=Approved` |
| `RejectRelease(ctx, project, approvalID, comment)` | `releases.go:297` | `UpdateReleaseApproval` (`releases.go:306`) with `Status=Rejected` |

Unlike pipelines, releases do **not** short-circuit on empty config -- when `c.releaseDefinitions` is empty, all definitions are included (`releases.go:96-99`). `releaseDefinitions` falls back to `cfg.Pipelines` when `release_definitions` is unset (`client.go:80-83`).

This targets **classic Release Management**, not YAML multi-stage pipelines. Organizations that have migrated fully to YAML will get empty results from all four of these methods.

### Group 5 -- Composite, multiple sub-clients (1 method)

| Method | Defined at | SDK calls issued |
|--------|-----------|------------------|
| `GetDashboardCounts(ctx, projects)` | `dashboard.go:23` | Per project via `getProjectCounts` (`dashboard.go:74`): `GetRecentIterations` -> `GetClassificationNode`; `QueryByWiql` (`dashboard.go:100`); `git.GetPullRequestsByProject` (`dashboard.go:109`); `build.GetBuilds` (`dashboard.go:123`, only when `c.pipelines` is non-empty) |

The build count here fetches **all** builds for the project and filters client-side (`dashboard.go:130-136`) with no `Top` cap. This is the heaviest single request the application makes on a busy project.

### Group 6 -- Raw HTTP, bypasses the SDK (1 method)

| Method | Defined at | Behavior |
|--------|-----------|----------|
| `DownloadAttachment(ctx, url, destPath)` | `workitems.go:520` | Builds a bare `http.NewRequestWithContext` GET (`workitems.go:521`), sets `Authorization` from `c.connection.AuthorizationString` (`workitems.go:526`), and executes on `http.DefaultClient` (`workitems.go:528`). |

This is the only place the ADO SDK is circumvented. Notes:

- It reuses the SDK's precomputed Basic credential rather than re-deriving it, which is correct.
- `http.DefaultClient` has **no timeout**. A hung ADO endpoint blocks until the caller's context is cancelled. The `ctx` is honored by the request, so cancellation works, but there is no independent deadline.
- The URL is passed in from the caller (relation URLs read out of the work item's `Relations` expand). There is no allow-list check that the URL points at the configured org, so a crafted work item relation could direct the request -- and the `Authorization` header -- to an arbitrary host.
- Only `200` is accepted (`workitems.go:534`); anything else returns `download failed: <status>`.
- The destination directory is created at mode `0755` and the file inherits the default umask via `os.Create` (`workitems.go:539-543`).

### Group 7 -- Pure accessors, zero network I/O (5 methods)

| Method | Defined at | Returns | Called from `internal/ui/`? |
|--------|-----------|---------|------------------------------|
| `Projects()` | `client.go:100` | `c.projects` | **No -- no call sites anywhere** |
| `Pipelines()` | `client.go:104` | `c.pipelines` | Yes |
| `ReleaseDefinitions()` | `client.go:108` | `c.releaseDefinitions` | **No -- no call sites anywhere** |
| `CurrentUser()` | `client.go:112` | `c.currentUser` | Yes |
| `CurrentUserEmail()` | `client.go:116` | `c.currentUserEmail` | Yes |

`Projects()` and `ReleaseDefinitions()` are dead exported API alongside the dead `workClient` field. The UI receives the project list directly through `ui.NewModel(...)` (`cmd/adoBizInt/main.go:59`) rather than asking the client for it.

## Authentication Model

| Aspect | Detail |
|--------|--------|
| Scheme | Personal Access Token over HTTP Basic |
| Source of truth | `Config.PAT`, from `~/.config/adoBizInt/config.yaml` key `pat` (`internal/config/config.go:39`) or env `ADO_PAT` (`internal/config/config.go:123`) |
| Validation | Presence only -- `Validate()` checks non-empty (`internal/config/config.go:145`). The token is never test-called before the TUI launches. |
| Identity resolution | `location.GetConnectionData` at startup (`client.go:70`) populates `currentUser` from `AuthorizedUser.ProviderDisplayName`. Failures are silently swallowed (`client.go:71` only proceeds on `err == nil`), leaving `currentUser` empty. |
| Email | Never resolved from the API. `currentUserEmail` is taken verbatim from the `user_email` config key (`client.go:68`). If unset, "Assigned to Me" filtering has nothing to match on. |
| Redaction | `main.go:31-35` logs org URL and projects and deliberately omits the PAT. The PAT does not appear in `~/.config/adoBizInt/adoBizInt.log`. |
| Rotation / expiry | Not handled. An expired PAT surfaces as per-call 401s that are logged and skipped, producing silently empty views rather than an auth error. |
| Storage | Plaintext in `~/.config/adoBizInt/config.yaml`. No keychain, no credential helper, no encryption. The config file is read but never created by the app, so its permissions are whatever the user set. |

**Required PAT scopes** are *inferred* from the API surface above and have not been verified against a live token: Work Items (read & write), Code (read & write, for PR approve/complete), Build (read & execute, for queueing), Release (read, write, execute & manage, for approvals). Treat this row as a starting hypothesis, not a verified fact.

## Rate Limiting and Request Volume

**There is no rate limiting, retry, or backoff anywhere in the codebase.** A repo-wide grep across `internal/` for `retry`, `backoff`, `ratelimit`, `429`, and `throttl` returns zero matches. Azure DevOps enforces per-org TSTU rate limits and returns `429` with a `Retry-After` header; this application treats a `429` identically to any other error -- log it and move on with partial data.

**There is also no concurrency.** Zero matches for `go func`, `sync.`, or `errgroup` in `internal/`. Every multi-project fetch is a sequential `for` loop. This bounds the burst rate but makes wall-clock latency scale linearly with project and definition count.

**Request volume per refresh cycle**, where `P` = configured projects, `D` = matching pipeline definitions, `R` = matching release definitions, `W` = work items returned, `Repo` = repositories:

| Method | Requests |
|--------|----------|
| `GetWorkItems` | `P x (1 classification node + 1 WIQL + ceil(W/200) detail batches)` |
| `GetPullRequests` | `P` |
| `GetPipelineRuns` | `P x (1 + D)` -- classic N+1 |
| `GetPipelineDefinitions` | `P x (1 + D)` -- classic N+1 |
| `GetReleases` | `P x (1 + R)` -- classic N+1 |
| `GetPendingApprovals` | `P` |
| `GetBranches` | `P x (1 + Repo)` -- classic N+1 |
| `GetDashboardCounts` | `P x (1 classification node + 1 WIQL + 1 PR + 1 unbounded builds)` |

The N+1 fan-outs in `GetPipelineRuns` (`pipelines.go:137-165`), `GetReleases` (`releases.go:103-130`), and `GetBranches` (`branches.go:57-71`) are the dominant cost. Each inner call fetches `Top=1`.

**Polling cadence.** Three views auto-refresh on a `tea.Tick`, defaulting to 30 seconds:

| View | Tick site |
|------|-----------|
| Pull requests | `internal/ui/prs/prs.go:121` |
| Pipelines | `internal/ui/pipelines/pipelines.go:135` |
| Releases | `internal/ui/releases/releases.go:136` |

Work items, branches, and the dashboard have **no** auto-refresh -- they load on demand only. The interval is set once from `Config.PollInterval` (`internal/config/config.go:46`, default 30s at `internal/config/config.go:82`) and each view independently re-defaults to 30s if the value is <= 0.

## Error Handling Patterns

Three distinct patterns, applied inconsistently:

**1. Log-and-continue (dominant in read paths).** Per-project and per-definition failures are logged and the loop advances, so `GetX` returns `nil` error with partial data. The caller cannot distinguish "no results" from "half the requests failed."

Sites: `branches.go:50-51`, `branches.go:77-78`, `dashboard.go:50-51`, `dashboard.go:102`, `dashboard.go:116`, `dashboard.go:127`, `pipelines.go:112-114`, `pipelines.go:168-171`, `releases.go:71-73`, `workitems.go:158` (batch skip), `client.go:71` (identity resolution).

**2. Propagate (dominant in write paths).** Mutations return the error unwrapped after logging: `ApprovePullRequest` (`pullrequests.go:182`), `CompletePullRequest` (`pullrequests.go:238`), `ApproveRelease` (`releases.go:289`), `RejectRelease` (`releases.go:314`), `UpdateWorkItemState` (`workitems.go:235`), `UpdateWorkItemStateWithEffort` (`workitems.go:285`), `QueuePipeline` (`pipelines.go:370`), `GetChildWorkItems` (`workitems.go:471`).

**3. Wrap with `%w` (rare).** Only three sites: `pullrequests.go:157`, `pullrequests.go:200`, and the sentinel errors at `pullrequests.go:164`, `pullrequests.go:208`, `pipelines.go:374`. There are no typed sentinel errors and no `errors.Is`/`errors.As` handling anywhere.

**Startup is the only place errors are fatal.** `main.go:23-50` exits on config load failure, validation failure, or `NewClient` failure. Once the TUI is running, no ADO error terminates the process.

**No error classification.** `401`, `403`, `404`, `429`, and `500` all take the same path. There is no way for the user to tell an expired PAT from a missing project from a rate-limit block -- all three render as an empty table.

## Local System Integrations

### Filesystem -- `~/.config/adoBizInt/`

| Path | Access | Site |
|------|--------|------|
| `config.yaml` | Read only. Never written or created by the app. | `internal/config/config.go:113-119` |
| `priorities.yaml` | Read (`priorities.go:31`) and write (`priorities.go:70`, mode `0644`). Directory created at `0755` (`priorities.go:59`). Guarded by a dirty flag (`priorities.go:49`). | `internal/config/priorities.go` |
| `adoBizInt.log` | Append-only write, mode `0644`. Directory created at `0755`. Debug level, unbounded growth, no rotation. | `internal/logging/logging.go:18-31` |
| Attachment destinations | Arbitrary caller-supplied path. Parent directories created at `0755`. | `internal/ado/workitems.go:539-543` |

### System Clipboard

`utils.CopyToClipboard` (`internal/utils/links.go:44-46`) delegates to `github.com/atotto/clipboard`. On darwin this shells to `pbcopy`; on linux it requires `xclip` or `xsel` to be installed, and returns an error if neither is present. Write-only -- the app never reads the clipboard.

### Default Browser

`utils.OpenInBrowser` (`internal/utils/links.go:49-62`) shells out per platform:

| GOOS | Command | Site |
|------|---------|------|
| darwin | `open <url>` | `links.go:53` |
| linux | `xdg-open <url>` | `links.go:55` |
| windows | `cmd /c start <url>` | `links.go:57` |
| other | returns `unsupported OS: <goos>` | `links.go:59` |

Uses `cmd.Start()` (`links.go:61`), non-blocking, so the TUI does not stall. The child process is never reaped -- no `Wait()` call -- which leaks a zombie per invocation on Unix.

**URL construction** is pure string formatting in `internal/utils/links.go`, independent of the SDK. Six builders: work item (`links.go:13`), PR (`links.go:18`), branch (`links.go:23`), PR-create (`links.go:28`), pipeline (`links.go:34`), release (`links.go:39`). Only the branch-name and ref parameters are `url.QueryEscape`d (`links.go:24`, `links.go:30`); `orgURL`, `project`, and `repoName` are interpolated raw. Project or repository names containing `#`, `?`, or spaces will produce malformed URLs.

**The `cmd /c start` path on Windows is a command-injection surface.** The URL is passed through `cmd.exe`, and these URLs embed unescaped project and repository names sourced from the ADO API.

## Webhooks and Callbacks

**Incoming:** None. Nothing listens on any port. There is no `net/http.Server` in the codebase.

**Outgoing:** None. All ADO interaction is client-initiated polling or user-initiated action.

## Integration Gaps

| Gap | Evidence |
|-----|----------|
| `work.Client` constructed but never used -- one wasted startup round trip, and a startup failure mode for a feature that does not exist | `client.go:23,54,88` |
| No retry or `429` handling against a rate-limited API | zero grep matches in `internal/` |
| No HTTP timeouts on the raw attachment download | `workitems.go:528` uses `http.DefaultClient` |
| Attachment URL is not validated against the configured org before the `Authorization` header is attached | `workitems.go:520-528` |
| Partial-failure results are indistinguishable from empty results | log-and-continue sites listed above |
| N+1 request fan-out in three read paths | `pipelines.go:137`, `releases.go:103`, `branches.go:57` |
| Unbounded `GetBuilds` in the dashboard, filtered client-side | `dashboard.go:123-136` |
| `GetChildWorkItems` does not batch and will fail above 200 children | `workitems.go:465` |
| `location.Client` rebuilt on every PR approval instead of caching the user ID from startup | `pullrequests.go:154` vs `client.go:66` |
| Project name interpolated into WIQL unescaped while iteration paths are escaped | `workitems.go:104`, `dashboard.go:92` vs `dashboard.go:86` |
| PAT stored in plaintext with no permission check on the config file | `internal/config/config.go:113` |
| The entire `internal/ado/` package is untested; `*ado.Client` is a concrete struct with no interface seam | no `internal/ado/*_test.go` exists |

---

*Integration audit: 2026-09-03*
