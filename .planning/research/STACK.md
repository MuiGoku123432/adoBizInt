# Stack Research

**Domain:** Scrum-master features (team/sprint reads, nag rules, ceremonies, grooming, story management, rate limiting) added to an existing Go Azure DevOps TUI+CLI (adoBizInt v2.0)
**Researched:** 2026-10-13
**Confidence:** HIGH for SDK, API-surface and govulncheck findings (verified by compiling, testing and scanning this repo against both SDK versions). MEDIUM/LOW for Azure DevOps service behavior taken from Microsoft Learn (see Sources).

## Bottom Line

1. **Upgrade the SDK to `github.com/microsoft/azure-devops-go-api/azuredevops/v7` v7.1.0.** It is the newest tagged release (2023-04-17). There is no v8. The upgrade only changes the import path. Every v2.0 API exists in both b5 and v7.1.0. The real reasons to upgrade: v7 adds a way to supply your own `http.Client`, which PLAT-03 needs; it sends 7.1 API versions instead of 5.1; and it supports Markdown comments.
2. **The SDK is not what fails `govulncheck`. The Go toolchain is.** No version of `azure-devops-go-api` has an entry in the Go vulnerability database. Go 1.25 is out of support, and some stdlib fixes exist only in 1.26.9+ and 1.27.2. As specified, PLAT-02 can never pass on `go 1.25.4`. The gate needs a toolchain bump plus `golang.org/x/sys` ≥ v0.44.0.
3. **Add zero new modules.** HTML stripping uses the stdlib `html` package plus the existing `stripHTML` code. Rate limiting is a stdlib `http.RoundTripper`. Concurrency caps use a channel semaphore plus `sync.WaitGroup.Go` (Go 1.25+). The multi-line acceptance-criteria editor is `bubbles/textarea`, already in the module graph.

## Recommended Stack

### Core Technologies (changes only)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/microsoft/azure-devops-go-api/azuredevops/v7` | **v7.1.0** (replaces `azuredevops v1.0.0-b5`) | All ADO calls | Newest tag, published 2023-04-17 (checked on proxy.golang.org; `@latest` returns v7.1.0). The repo has untagged commits up to 2026-06, but they only touch CI or `GetPullRequestProperties`, `GraphGroup.IsDeleted`, none of which we use. So **pin the tag, not a pseudo-version**. A trial upgrade was done in a scratch copy: an import-path `sed` across 24 files plus `go mod tidy`. `go build`, `go vet` and `go test ./...` all pass with **no code changes**. v7 adds `azuredevops.NewClientWithOptions(conn, url, azuredevops.WithHTTPClient(hc))`, which b5 does not have and which PLAT-03 depends on. |
| Go toolchain | **`go 1.27.0` directive + CI on 1.27.2** (from `go 1.25.4`) | Build and vuln gate | Go 1.25 is out of support (current releases are 1.27.2 and 1.26.9). Running `govulncheck` against 1.25.4 reports **26 stdlib vulnerabilities your code calls**. Several are "Fixed in net/http@go1.26.9" and were never backported to 1.25. With v7.1.0, x/sys v0.47.0 and go1.27.2, the result is **"No vulnerabilities found."** The codebase builds and tests cleanly with the directive at 1.27.0. Local dev is already on go1.27.1. |
| `golang.org/x/sys` (indirect) | **v0.47.0** (from v0.38.0) | Vuln fix GO-2026-5024 | Fixed in v0.44.0. v0.47.0 is the newest that still declares `go 1.25.0`; v0.48.0+ require 1.26. This is an indirect dependency via charmbracelet, so it is a bump, not a new dependency. |

### Supporting Libraries (all already present: stdlib or existing modules)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `html` | Go stdlib | `html.UnescapeString` in the shared HTML stripper; `html.EscapeString` for user/agent text written into HTML fields | NAG-03 empty-AC check, AC display, comment display, GROOM-02 AC write-back |
| stdlib `net/http` | Go stdlib | Custom `RoundTripper` ("pacer") for Retry-After / X-RateLimit; `http.ParseTime` for HTTP-date Retry-After | PLAT-03 |
| stdlib `sync` | Go stdlib (Go 1.25+) | `sync.WaitGroup.Go` + `chan struct{}` semaphore for fan-out caps | TEAM-04 multi-team rollups and per-team/per-project fan-out |
| stdlib `testing/synctest` | Go stdlib (GA in 1.25) | Deterministic tests of backoff/pacing with no real sleeps | Unit-test the pacer with a fake base `RoundTripper` inside the bubble. Not with `httptest.NewServer`, which uses real network I/O. |
| stdlib `encoding/json` | Go stdlib | Re-decode `ReportingWorkItemRevisionsBatch.Values` (`*[]interface{}` in both SDK versions) into a local struct | DATA-04, if reporting revisions are used |
| `github.com/charmbracelet/bubbles/textarea` | v0.21.0 (already required) | Multi-line inline edit of acceptance criteria | GROOM-02. `textinput` is already used for single-line fields. |
| `golang.org/x/vuln/cmd/govulncheck` | **v1.9.0** (needs Go ≥1.26; v1.7.0 is the last that runs on 1.25) | PLAT-02 gate | Run as a tool (`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`), not added to `go.mod` |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `govulncheck` in CI | PLAT-02 gate | Add a step to `.github/workflows/ci.yml`. It reports stdlib vulns for **the toolchain that runs it**, so CI's `go-version` decides pass or fail, not the `go.mod` directive. Prefer `go-version-file: go.mod`, which keeps D-16 ("CI matches go.mod") true automatically. |
| `go test -race` (existing) | Catches the SDK `SessionId` race once fan-out goes concurrent | See the integration notes below. Fakes never touch the SDK, so a race test needs the real `azuredevops.Client` against an `httptest` server. |

## Installation

```bash
# SDK upgrade (mechanical; verified to compile and pass tests unchanged)
grep -rl 'azure-devops-go-api/azuredevops' --include='*.go' . \
  | xargs sed -i '' 's#github.com/microsoft/azure-devops-go-api/azuredevops#github.com/microsoft/azure-devops-go-api/azuredevops/v7#g'
go mod edit -droprequire github.com/microsoft/azure-devops-go-api/azuredevops
go get github.com/microsoft/azure-devops-go-api/azuredevops/v7@v7.1.0

# Vuln-gate prerequisites
go mod edit -go=1.27.0
go get golang.org/x/sys@v0.47.0
go mod tidy

# Gate (CI and local)
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

No `go get` for any new module. Package names are unchanged: `azuredevops`, `work`, `core`, and so on, so `attachments_test.go:462`'s `pkg.Name == "azuredevops"` check still holds.

## API Surface Verification (pinned b5 vs v7.1.0)

Checked by grepping `func (client *ClientImpl) X(` and the `Send(... apiVersion)` literal in both module-cache copies.

| API (requirement) | b5 | v7.1.0 | b5 api-version | v7 api-version |
|---|---|---|---|---|
| `core.GetAllTeams` (TEAM-01) | yes | yes | **5.1-preview.3** | 7.1-preview.3 |
| `core.GetTeams` (recommended for TEAM-01) | yes | yes | 5.1 (released) | 7.1-preview.3 |
| `core.GetTeamMembersWithExtendedProperties` (TEAM-02) | yes | yes | 5.1 | 7.1-preview.2 |
| `work.GetTeamIterations` + `Timeframe` (TEAM-03) | yes | yes | 5.1 | 7.1-preview.1 |
| `work.GetTeamSettings` → `WorkingDays *[]string` (NAG-09) | yes | yes | 5.1 | 7.1-preview.1 |
| `work.ReorderBacklogWorkItems` (GROOM-03) | yes | yes | **5.1-preview.1** | 7.1-preview.1 |
| `work.ReorderIterationWorkItems` (sprint-scoped order) | yes | yes | — | — |
| `workitemtracking.ReadReportingRevisionsPost` (DATA-04) | yes | yes | 5.1 | 7.1-preview.2 |
| `workitemtracking.CreateWorkItem` (STORY-01) | yes | yes | 5.1 | 7.1-preview.3 |
| `workitemtracking.GetComments` / `AddComment` (STORY-02) | yes | yes | **5.1-preview.3** | 7.1-preview.4 |
| `workitemtracking.AddWorkItemComment` (with `Format: Markdown\|Html`) | **no** | yes | — | 7.1-preview.4 |
| `workitemtracking.UpdateWorkItem` (STORY-03 links via `/relations`) | yes | yes | 5.1 | 7.1-preview.3 |
| `workitemtracking.GetWorkItemsBatch` with `AsOf` | yes | yes | 5.1 | 7.1-preview.1 |
| `git.GetPullRequestWorkItemRefs` (NAG-06) | yes | yes | 5.1 | 7.1-preview.1 |
| Capacity (SPRINT-03, display only) | `GetCapacitiesWithIdentityRef` | **renamed** `GetCapacitiesWithIdentityRefAndTotals` → `*TeamCapacity` | — | — |
| `azuredevops.NewClientWithOptions` / `WithHTTPClient` (PLAT-03) | **no** | yes | — | — |

Argument structs are identical in both versions for every v2.0 call: `ReadReportingRevisionsPostArgs`, `CreateWorkItemArgs`, `GetCommentsArgs`, `AddCommentArgs`, `GetTeamIterationsArgs`, `ReorderBacklogWorkItemsArgs`, `GetTeamMembersWithExtendedPropertiesArgs`, `GetAllTeamsArgs`, `UpdateWorkItemArgs`.

**Breaking changes b5 → v7.1.0 that matter here:**
- Import path gains `/v7`. This is mechanical, across 24 import sites.
- `work.GetCapacitiesWithIdentityRef` is renamed to `GetCapacitiesWithIdentityRefAndTotals` with a different return type. It is unused today, so build against the v7 name.
- `workitemtracking.GetField/GetFields/CreateField/DeleteField` are renamed to `GetWorkItemField(s)/…` returning `WorkItemField2`. They are unused.
- Every call moves from `5.1` / `5.1-preview` to `7.1-preview.N`. This is behavioral rather than compile-time. Re-verify the six views against live ADO after the swap. Because the build and test suite pass, the risk is limited to JSON shape drift in responses.
- The doc comments in `internal/ado/result.go` cite b5 line numbers (`client.go:366, :389, :403, :411`). In v7 those lines are 374/398/403/414. The value-vs-pointer `WrappedError` behavior they describe is **unchanged** (diffed). Update the line references.

**Why the b5 API versions are a liability:** Microsoft's policy is that a `-preview` version "can be deactivated after 12 weeks" once the API is released. Three of the APIs v2.0 needs (`GetAllTeams`, comments, `ReorderBacklogWorkItems`) are 5.1-preview in b5. This is a reason to land PLAT-02 early, not late.

## Integration Points

### 1. Rate limiting (PLAT-03): use a RoundTripper, not service-layer header reads

**Why a transport is mandatory:** the SDK throws away response headers. `Client.Send` returns `nil, err` on non-2xx, and `UnwrapError` keeps only the status code and message, so `Retry-After` is unreachable from `internal/ado`. Azure DevOps also sends `Retry-After` and `X-RateLimit-*` **on HTTP 200 responses before throttling starts**, and its docs say to honor them. Only a transport ever sees those headers.

**How to inject it (v7 only):** `Connection` exposes no hook for an HTTP client, and `GetClientByUrl` always calls `NewClient`. Build the sub-clients yourself:

```go
hc := &http.Client{Timeout: cfg.RequestTimeout, Transport: &pacer{base: http.DefaultTransport}}
root := azuredevops.NewClientWithOptions(conn, conn.BaseUrl, azuredevops.WithHTTPClient(hc))
areas, err := root.GetResourceAreas(ctx)            // one discovery call (empty list on-prem -> use BaseUrl)
// urlFor(id) = matching *ResourceAreaInfo.LocationUrl, else conn.BaseUrl
work := &work.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, urlFor(work.ResourceAreaId), azuredevops.WithHTTPClient(hc))}
```

This compiled and vetted against v7.1.0 in a scratch prototype covering `work` and `core`. Requirements for it:
- **`hc.Timeout` must be set explicitly.** `WithHTTPClient` bypasses `connection.Timeout`. That silently voids the per-call budget that `newConnection` (`internal/ado/client.go`) exists to install, which is the same trap its doc comment warns about.
- **Validate `LocationUrl` before sending the PAT to it.** Accept the OrgURL host, `*.dev.azure.com` and `*.visualstudio.com`. The SDK trusts it blindly, and PROJECT.md says the PAT must never go to a caller-supplied host.
- Apply `normalizeUrl`-equivalent trimming. `NewClientWithOptions` does not normalize; `GetClientByUrl` did.
- `core.NewClient` is not built today. TEAM-01/02 add a `core` sub-client and a narrow `coreAPI` interface, matching the existing seam pattern.

**What the pacer does (stdlib only, about 50 lines):**
- Keeps a mutex-guarded `notBefore time.Time`. Before each request it waits until `notBefore`, aborting on `req.Context().Done()`.
- After each response it parses `Retry-After` (integer seconds, falling back to `http.ParseTime`), caps it at a configurable maximum, and advances `notBefore`. It logs `X-RateLimit-Resource/Remaining/Limit/Delay` to the slog file, never stdout.
- **Deletes the `X-TFS-Session` response header.** Both b5 and v7 assign the package global `azuredevops.SessionId` from that header in `Send` without a lock. The codebase is fully sequential today, so this race is latent. TEAM-04 fan-out makes it real, and CI runs `go test -race`.
- **Does not retry inside the transport.** Read-only calls such as WIQL, `workitemsbatch` and reporting revisions are POSTs, so the transport cannot tell reads from writes. Retry lives in the service layer, read paths only. On `classifyFailure(err) == FailureRateLimit`, which already exists, re-issue the call up to N times; the pacer supplies the wait. Writes are never auto-retried. A 429 on a write is a server refusal with a known outcome, so it maps to exit 4, not 5.

### 2. Fan-out caps (PLAT-03, TEAM-04)

Use `sem := make(chan struct{}, cfg.MaxConcurrency)` plus `var wg sync.WaitGroup; wg.Go(func(){ sem <- struct{}{}; defer func(){ <-sem }(); ... })`. Default to a small N (4). Throttling is measured in TSTU consumption per user over a 5-minute window, not in concurrent connections, so the cap smooths bursts and the pacer handles the server's signal. Collect `Failure`s per goroutine into a mutex-guarded slice so the `Result[T]` completeness contract survives concurrency. Do **not** add `golang.org/x/sync/errgroup`. Its `SetLimit` is convenient, but errgroup's cancel-on-first-error is the opposite of the partial-result semantics the `Result[T]` envelope requires.

### 3. HTML stripping (NAG-03, AC display, comments)

A `stripHTML` already exists: `internal/ui/workitems/workitems.go:816`, with 4 call sites in the TUI. Under the reuse rule, **move it to `internal/utils/text.go` as `StripHTML`** rather than writing a second one. The nag/ceremony code cannot import a UI package. Extend it in place:
- Apply `html.UnescapeString` **after** tag removal. Without it, `&nbsp;`, which the ADO rich-text editor emits for "blank" AC, counts as content, and NAG-03 silently misses empty AC. Unescaping after stripping also means an escaped `&lt;b&gt;` survives as literal text instead of being eaten as a tag.
- Replace the `ReplaceAll("  "," ")` loop with `strings.Join(strings.Fields(s), " ")`. `strings.Fields` uses `unicode.IsSpace`, which covers U+00A0 NBSP; the current ASCII-only collapse does not.
- NAG-03 then becomes `utils.StripHTML(ac) == ""`.
- **Decision for the plan:** whether image-only AC (`<img>` with no text) counts as empty. The stripper says yes; record it as rule semantics.
- `golang.org/x/net/html` would be a new module and is not justified. ADO stores sanitized editor output, `<` in text arrives as `&lt;`, and the question asked is only whether visible text exists.

Writing HTML fields: `html.EscapeString` any user- or agent-supplied text, and turn `\n` into `<br>`, before patching `System.Description` or `Microsoft.VSTS.Common.AcceptanceCriteria`. For comments, prefer v7 `AddWorkItemComment` with `Format: Markdown`, which avoids building HTML at all. **Grooming risk:** round-tripping existing AC through strip, edit and write destroys its rich formatting (lists, bold). GROOM-02 must either edit only empty or plain AC or warn before overwriting.

### 4. Sprint data and baseline (DATA-01..04)

- Current sprint: use `work.GetTeamIterations{Timeframe: "current"}`. Dates arrive as `azuredevops.Time` at **UTC midnight**. Do working-day math (NAG-09, `WorkingDays` such as `"monday"`) in UTC; converting to local time shifts sprint boundaries by a day west of UTC.
- **For DATA-04, prefer WIQL `ASOF` plus `workitemsbatch` `AsOf` over `ReadReportingRevisionsPost`.** Reporting revisions cannot be filtered by iteration server-side. It streams every revision in the project since `StartDateTime`, paged through `ContinuationToken`/`IsLastBatch`, with untyped `Values`, which is TSTU-heavy and the wrong shape for this question. `… WHERE [System.IterationPath] = '<path>' ASOF '<start+48h>Z'` returns baseline membership in one query per team. `WorkItemBatchGetRequest.AsOf`, present in b5 and v7, returns StoryPoints as of that moment. Scope change (added/removed/re-pointed) and carryover all fall out of set differences between two ASOF snapshots. Keep `ReadReportingRevisionsPost` only if a requirement needs the full revision timeline.
- WIQL literals must keep going through the existing quote-doubling at `internal/ado/client.go:295`. WIQL has no parameter binding, so team names and iteration paths containing `'` are an injection vector. Use ISO-8601 dates with a `Z` suffix; WIQL otherwise interprets dates in the locale and timezone of the PAT user.
- Keep the existing 200-ID `workitemsbatch` chunking (`workitems.go:211`) for the new sprint-scoped read.

### 5. Teams (TEAM-01..03)

**Use `core.GetTeams{ProjectId}` rather than `core.GetAllTeams`.** It is project-scoped, which matches the config model. It is a released 5.1 API in b5, whereas `GetAllTeams` is 5.1-preview. It does not need the org-wide visibility that `GetAllTeams` (`Mine`/`Top`/`Skip` across the org) implies. Page both `GetTeams` and `GetTeamMembersWithExtendedProperties` with `Top`/`Skip`.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| v7.1.0 (tagged) | Untagged pseudo-version from `main` (2024-10) | Never for this project. The extra commits fix only `GetPullRequestProperties` and add `GraphGroup.IsDeleted`. |
| v7.1.0 | v6.0.1 (2022) | Only if the org were Azure DevOps Server 2020, which supports up to 6.0. Services and Server 2022 accept 7.x; the SDK negotiates down if the server's max is lower. |
| Transport pacer via `NewClientWithOptions` | Replace `http.DefaultTransport` globally at startup (works on b5 too) | Only as a stopgap if PLAT-03 must ship before PLAT-02. It mutates process-global state, which is invisible and test-hostile. |
| Service-layer retry on `FailureRateLimit` | Retry inside the `RoundTripper` | Never. The transport cannot tell read POSTs from write POSTs, and a transport-level retry of a write breaks the audit and unknown-outcome contract. |
| WIQL `ASOF` + batch `AsOf` | `ReadReportingRevisionsPost` | When a full field-change timeline is required, not just snapshots. |
| stdlib semaphore + `WaitGroup.Go` | `golang.org/x/sync/errgroup` | If cancel-on-first-error were wanted. It is not, because of partial results. |
| stdlib `StripHTML` | `golang.org/x/net/html` tokenizer, or bluemonday | If real HTML-to-text rendering (lists, tables) became a requirement. It is not. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Staying on `azuredevops v1.0.0-b5` | No `WithHTTPClient` hook, so Retry-After on 200s is invisible. It calls 5.1-preview endpoints that are subject to deactivation. Last published 2020-08. | v7.1.0 |
| Treating PLAT-02 as "SDK upgrade fixes govulncheck" | The SDK has zero vuln-DB entries. All findings are stdlib (toolchain) plus x/sys. Upgrading only the SDK leaves the gate red. | Toolchain to 1.27.x, x/sys to v0.47.0, then the SDK |
| `golang.org/x/time/rate` | ADO publishes no fixed request rate to configure. It signals dynamically through headers. A token bucket adds a dependency and a guessed number. | The header-driven pacer plus a concurrency cap |
| Retry or backoff libraries (`hashicorp/go-retryablehttp`, `cenkalti/backoff`) | New dependencies. go-retryablehttp retries writes by default. Neither understands ADO's 200-with-Retry-After behavior. | About 50-line stdlib pacer plus service-layer read retry |
| `charm.land/bubbletea/v2` or `bubbles/v2` migration | v2 is available (bubbletea v2.1.0, bubbles v2.2.1), but migrating violates "TUI behavior unchanged" and is unrelated to scrum features | Stay on bubbletea v1.3.10 / bubbles v0.21.0 |
| `core.GetAllTeams` as the primary discovery call | Org-wide, and preview in b5 | `core.GetTeams` per configured project |
| Capacity APIs as rule inputs | Out of scope per SPRINT-03, and the method was renamed in v7 | `GetCapacitiesWithIdentityRefAndTotals`, display only |

## Stack Patterns by Variant

**If PLAT-02 lands before TEAM-04 / PLAT-03 (recommended):**
- Build the pacer on `NewClientWithOptions` from day one, and add the `core` client in the same constructor rewrite.
- One rewrite of `ado.NewClient` instead of two.

**If the org turns out to be Azure DevOps Server 2020, not Services:**
- The 7.1 requests negotiate down to 6.0 automatically. `GetResourceAreas` returns an empty list, so use `BaseUrl` for every area.
- Rate-limit headers are a Services feature, so the pacer becomes a no-op. That is harmless.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `azuredevops/v7@v7.1.0` (`go 1.12`, requires `google/uuid v1.1.1`) | Go 1.25–1.27 | Same uuid pin as b5, so no transitive churn |
| `golang.org/x/sys@v0.47.0` | `go` ≥ 1.25.0 | v0.48.0+ require 1.26 |
| `govulncheck@v1.9.0` / `@v1.8.0` | Go ≥ 1.26 to run | v1.7.0 is the last that runs on 1.25 |
| `sync.WaitGroup.Go`, `testing/synctest` GA | Go ≥ 1.25 | Already satisfied |
| `bubbles@v0.21.0/textarea` | bubbletea v1.3.10 | Already in the module cache, so no new requirement |

## Sources

- **Local verification (HIGH, primary evidence):**
  - Module cache diff of `azuredevops@v1.0.0-b5` vs `azuredevops/v7@v7.1.0`: `client.go`, `connection.go`, `errors.go`, `versionnegotiation.go`, `Client` interfaces of `core`, `work`, `workitemtracking` and `git`, and arg/model structs.
  - Trial upgrade in a scratch copy of the repo: build, vet and `go test ./...` all pass with the `go 1.27.0` directive and x/sys v0.47.0.
  - `govulncheck@v1.9.0` runs: GOTOOLCHAIN go1.25.4 gives 26 stdlib findings plus x/sys; go1.27.1 gives 8 stdlib findings; go1.27.2 with v7.1.0 and x/sys v0.47.0 gives "No vulnerabilities found". b5 with x/sys v0.47.0 on go1.27.2 is also clean, which confirms the SDK is not the source.
- **proxy.golang.org / vuln.go.dev / go.dev/dl (HIGH):**
  - `azuredevops` latest is v1.0.0-b5 (2020-08-23); `/v6` latest v6.0.1; `/v7` latest v7.1.0 (2023-04-17).
  - No vuln-DB entries for `azure-devops-go-api`.
  - Current Go releases are 1.27.2 and 1.26.9; latest 1.25 is go1.25.14.
  - `go` directives of x/sys and x/vuln, read from their `.mod` files.
- **GitHub API, microsoft/azure-devops-go-api (HIGH):** tags list, plus the content of post-v7.1.0 commits that touch `azuredevops/`.
- **Microsoft Learn: Rate and usage limits** (https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits, updated 2026-07-31). Covers header names and semantics and the advice to honor Retry-After on 200s. Rated LOW by the classify-confidence seam (webfetch provider), though it is the official source. The pacer design tolerates absent headers.
- **Microsoft Learn: REST API versioning** (https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rest-api-versioning, updated 2026-07-23). Source for the 12-week deactivation of preview versions. Seam: LOW.
- **Microsoft Learn: WIQL syntax** (https://learn.microsoft.com/en-us/azure/devops/boards/queries/wiql-syntax, updated 2026-07-23). Covers ASOF, ISO-8601 `Z` dates, the 32K limit, and ASOF being incompatible with tree queries. Seam: LOW.
- **Unverified, flag for phase research:**
  - The PAT scopes needed per new call (expected `vso.work_write` for create, comment and reorder, and `vso.project` for teams).
  - Whether `/multilineFieldsFormat/<field>` = `Markdown` is accepted on AC writes at 7.1-preview.3.

---
*Stack research for: adoBizInt v2.0 Scrum Master*
*Researched: 2026-10-13*
