# Phase 2: Honest reads - Research

**Researched:** 2026-09-07
**Domain:** Go error propagation, Azure DevOps SDK error classification, Bubble Tea message contracts
**Confidence:** HIGH (all findings verified against repo source and vendored SDK source this session)

## Summary

This phase is almost entirely a *local* engineering problem, not a research problem. Every fact the planner needs is in the repo or in the vendored SDK at `~/go/pkg/mod/github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5`. There is no external library to add, no ecosystem pattern to import, and no package legitimacy audit to run — this phase installs nothing.

Three findings materially change how the plan should be written:

1. **The SDK returns its error type in two different Go shapes.** `azuredevops.Client.UnwrapError` returns `*WrappedError` on two paths and a bare `WrappedError` **value** on two others. A classifier that does `errors.As(err, &wrappedErrPtr)` only will silently mis-classify roughly half the real failures — including the text/plain path, which is exactly what an HTML sign-in page from a rejected PAT tends to hit. This is the single highest-risk detail in the phase.
2. **`connection.Timeout` must be set before the first sub-client is constructed**, because `azuredevops.NewClient` copies it into `http.Client.Timeout` at construction and `Connection` caches constructed clients. Setting it after `NewClient` in `internal/ado/client.go` is a silent no-op.
3. **The eight aggregate reads are not eight identical shapes.** Six are a flat `for project { ... continue }`. `GetBranches` has a *nested* per-repo swallow. `GetDashboardCounts` swallows nothing at the loop — its helper `getProjectCounts` returns `nil` error unconditionally, so its outer `continue` at `dashboard.go:51` is dead code today. A plan that treats all eight as one mechanical edit will produce a green build and an unchanged dashboard.

**Primary recommendation:** Land a single generic `Result[T]` envelope plus a non-generic `Failure` struct in a new `internal/ado/result.go`, wire the classifier in the same file, then invert the eight reads one file at a time so each Phase 1 characterization test flips in its own commit. Do the `json:` tags (02-03) first and standalone — it is additive, zero-risk, and unblocks nothing else, so it should not sit behind the envelope work.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Error classification (401/403/timeout/429) | `internal/ado` | — | Only this package sees the SDK error; classifying it anywhere else means re-exporting SDK types |
| Partial-result aggregation | `internal/ado` | — | The fan-out loop is the only place that knows which projects were attempted |
| Deadline creation (fetch-level) | `internal/ui/*` fetch commands | `cmd/adoBizInt/main.go` (Phase 3) | The caller owns the budget; `ado` methods are already `ctx`-first and should not create their own |
| Per-call HTTP deadline | `internal/ado/client.go` (`NewClient`) | — | `connection.Timeout` is applied at sub-client construction and cannot be set later |
| Timeout config keys | `internal/config` | — | `PollInterval` precedent already lives here |
| Failure rendering (banner / warning / stale) | `internal/ui/<view>` | `internal/ui/styles` | Each view owns its `View()`; styles owns the visual vocabulary |
| JSON field names | `internal/ado` struct tags | — | OUT-02 is a tag-only change on the domain structs |

**Planner note:** the fetch-level deadline (D-09) belongs at the Bubble Tea `tea.Cmd` closures listed under "Call sites" below, *not* inside the `ado` methods. Creating a deadline inside `GetWorkItems` would double-bound the Phase 3 CLI `--timeout` and make the two budgets invisible to each other.

## Project Constraints (from AGENTS.md)

No `AGENTS.md`, `.opencode/AGENTS.md`, or `CLAUDE.md` exists at the repo root. [VERIFIED: `ls` of repo root this session — only `cmd/`, `internal/`, `go.mod`, `go.sum`, `adoBizInt`, `systemDesign.pdf`, `.planning/`]

The governing document is `.planning/codebase/CONVENTIONS.md`. Directives relevant to this phase:

- **Rule 5** — no interface or abstraction purely for testability unless that *is* the task. The `Result[T]` envelope is not an interface, but it is the codebase's first generic type and needs the same explicit justification the Phase 1 interfaces received. [CITED: 02-CONTEXT.md D-01]
- **Rule of three** — eight call sites is well past it. Both the envelope (D-01) and the shared failure-rendering helper (open question) clear this bar.
- **300-line file standard** — `internal/ado` is 2,015 lines across 7 files; `workitems.go` is 551. [VERIFIED: `wc -l internal/ado/*.go` this session — workitems.go 551, pipelines.go 397, releases.go 319, client.go 251, pullrequests.go 244, dashboard.go 142, branches.go 127, interfaces.go 47]. New types must not be appended to `workitems.go`.
- **Import grouping** — three blank-line-separated groups: stdlib, third-party, module-local. [CITED: CONVENTIONS.md §Import grouping]
- **Receiver naming** — `(c *Client)` in `internal/ado`, `(m Model)` value receiver for Bubble Tea views, `(m *Model)` when mutating. [CITED: CONVENTIONS.md §Receiver naming]
- **Toolchain** — `gofmt -l .` must stay empty; `go vet ./...` must stay clean. No linter beyond `go vet`, no CI. Verification is by hand until Phase 5 HARD-05. [CITED: CONVENTIONS.md §Toolchain Baseline]

## Package Legitimacy Audit

**Not applicable.** This phase installs no external packages. Every type it needs is in the Go standard library (`context`, `errors`, `net`, `time`) or already a direct dependency (`github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5`, `gopkg.in/yaml.v3`). [VERIFIED: `head -5 go.mod` + `grep azure-devops-go-api go.mod` → `github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5` at go.mod:10]

No `[SLOP]` or `[SUS]` verdicts to report; no `checkpoint:human-verify` install gates are required.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `errors` | go 1.25.4 | `errors.As` / `errors.Is` for SDK error classification | The only correct way to reach a wrapped concrete error type |
| stdlib `context` | go 1.25.4 | `context.WithTimeout` for the fetch-level budget (D-09) | Every `ado` method is already `ctx`-first |
| stdlib `net` | go 1.25.4 | `net.Error.Timeout()` for transport-level timeout detection | Works regardless of whether the deadline came from `ctx` or `http.Client.Timeout` |
| `azuredevops` | v1.0.0-b5 | `WrappedError.StatusCode`, `Connection.Timeout` | Already a direct dependency; both fields exist and are unused |
| Go generics | go 1.25.4 | `Result[T]` envelope (D-01) | Language feature, not a dependency |

[VERIFIED: go.mod:1-3 declares `module sentinovo.ai/bizInt` / `go 1.25.4`. Generics require go >= 1.18.]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Generic `Result[T]` | Eight concrete `XxxResult` structs | No generics in a codebase that has none, but eight near-identical types and eight JSON shapes for Phase 3 to handle. Rejected by D-01. |
| Generic `Result[T]` | Embedded non-generic `ReadOutcome` + concrete `Items []T` per read | Keeps the codebase generic-free while sharing the failure/completeness half. Still eight types. **This is the open question the planner must close** — D-01 fixes the *contract*, not the Go mechanism. |
| Multi-return `(items, failures, complete, err)` | — | Four return values on eight methods, and Go has no way to stop a caller ignoring the middle two. Contradicts D-01. |
| `errors.Join` for the failure list | — | Loses the structured project/operation/kind fields D-02 and D-06 require. Only usable as a supplementary flattened `error`. |

**Installation:** none. `go build ./...` unchanged.

**Version verification:** `github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5` is a 2020 beta, confirmed present in the module cache and unchanged. Upgrading is PLAT-02, explicitly deferred out of v1. [CITED: REQUIREMENTS.md §Platform PLAT-02; ROADMAP.md §Deferred]

## Architecture Patterns

### System Architecture Diagram

```
config.yaml / env
   │  request_timeout, fetch_timeout   (NEW, D-10/D-11)
   ▼
config.Load()  ──────────────────────────────────────────┐
   │                                                     │
   ▼                                                     ▼
ado.NewClient(cfg)                          internal/ui/<view> tea.Cmd
   │ set connection.Timeout = cfg.RequestTimeout    │
   │ ── MUST precede sub-client construction ──     │ ctx, cancel :=
   ▼                                                │   context.WithTimeout(
azuredevops.Connection                              │     ctx, cfg.FetchTimeout)
   │ (caches *azuredevops.Client per resource area) │
   ▼                                                ▼
workitemtracking / git / build / release  ◄──── ado.GetXxx(ctx, projects)
   │                                                │
   │  per-project fan-out                           │
   ▼                                                │
 SDK call ──► error?                                │
   │            │                                   │
   │            ▼                                   │
   │       classifyFailure(err)                     │
   │        ├─ 401/403        → kind "auth"         │
   │        ├─ ctx/net timeout→ kind "timeout"      │
   │        ├─ 429            → kind "rate_limit"   │
   │        └─ everything else→ kind "other"        │
   │            │                                   │
   │            ▼                                   │
   │       append to Failures; DO NOT short-circuit │  (D-08)
   │            │                                   │
   └────────────┴──────────────────► Result[T]{ Items, Failures, Complete }
                                                    │
                                                    ▼
                                          XxxMsg{ Result, Err }   (D-16)
                                                    │
                                                    ▼
                                          Model.Update() reducer
                              ┌─────────────────────┼─────────────────────┐
                              ▼                     ▼                     ▼
                    any Failure.Kind==auth   Complete==false      refresh failed
                    → blocking banner        → rows + persistent  after good load
                      replaces table (D-14)    warning line (D-13)→ keep last rows,
                                                                    mark stale (D-15)
```

### Recommended Project Structure

```
internal/ado/
├── result.go        # NEW — Result[T], Failure, FailureKind, classifyFailure
├── result_test.go   # NEW — classifier table test (value + pointer WrappedError)
├── client.go        # connection.Timeout wiring
├── workitems.go     # 551 lines — do NOT add types here
├── pullrequests.go
├── pipelines.go
├── releases.go
├── branches.go
└── dashboard.go
internal/config/config.go   # RequestTimeout, FetchTimeout
internal/ui/styles/styles.go # NEW: WarnStyle / StaleStyle if shared helpers chosen
```

A new `result.go` is the right home: it is the only file all seven other `ado` files import from, and putting it in the 551-line `workitems.go` would deepen the exact CONVENTIONS violation this project is trying to stop widening.

### Pattern 1: Classifying an SDK error — handle BOTH value and pointer

**What:** `azuredevops.Client.UnwrapError` returns four different things depending on the response shape. Two are `*WrappedError`, two are a bare `WrappedError` value.

**When to use:** every failure path in every aggregate read.

**Verbatim source of truth** — `azuredevops@v1.0.0-b5/client.go`, the four return statements inside `UnwrapError`:

```go
// client.go:364-369  — empty body
	if response.ContentLength == 0 {
		message := "Request returned status: " + response.Status
		return &WrappedError{
			Message:    &message,
			StatusCode: &response.StatusCode,
		}
	}

// client.go:386-389  — text/plain body  ◄── VALUE, not pointer
	if ok && len(contentType) > 0 && strings.Index(contentType[0], MediaTypeTextPlain) >= 0 {
		message := string(body)
		statusCode := response.StatusCode
		return WrappedError{Message: &message, StatusCode: &statusCode}
	}

// client.go:400-407  — improper-error JSON envelope
		var wrappedImproperError WrappedImproperError
		err = json.Unmarshal(body, &wrappedImproperError)
		if err == nil && wrappedImproperError.Value != nil && wrappedImproperError.Value.Message != nil {
			return &WrappedError{
				Message:    wrappedImproperError.Value.Message,
				StatusCode: &response.StatusCode,
			}
		}

// client.go:411  — normal JSON error body  ◄── VALUE, not pointer
	return wrappedError
```

[VERIFIED: `/Users/e1001547-mbp-it/go/pkg/mod/github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5/client.go:362-412`]

The type itself:

```go
// models.go:128-149
// WrappedError
type WrappedError struct {
	ExceptionId      *string                 `json:"$id,omitempty"`
	InnerError       *WrappedError           `json:"innerException,omitempty"`
	Message          *string                 `json:"message,omitempty"`
	TypeName         *string                 `json:"typeName,omitempty"`
	TypeKey          *string                 `json:"typeKey,omitempty"`
	ErrorCode        *int                    `json:"errorCode,omitempty"`
	EventId          *int                    `json:"eventId,omitempty"`
	CustomProperties *map[string]interface{} `json:"customProperties,omitempty"`
	StatusCode       *int
}

func (e WrappedError) Error() string {
	if e.Message == nil {
		if e.StatusCode != nil {
			return "REST call returned status code " + strconv.Itoa(*e.StatusCode)
		}
		return ""
	}
	return *e.Message
}
```

[VERIFIED: `azuredevops@v1.0.0-b5/models.go:128-149`]

Because `Error()` has a **value receiver**, both `WrappedError` and `*WrappedError` satisfy `error`, and `errors.As` will not convert between them. A status-code extractor must try both targets:

```go
// Pattern the classifier must follow. Not code to copy verbatim — the plan
// decides naming — but the two-target structure is mandatory.
func statusCodeOf(err error) (int, bool) {
	var we azuredevops.WrappedError          // value form  (client.go:389, :411)
	if errors.As(err, &we) && we.StatusCode != nil {
		return *we.StatusCode, true
	}
	var wep *azuredevops.WrappedError        // pointer form (client.go:366, :403)
	if errors.As(err, &wep) && wep != nil && wep.StatusCode != nil {
		return *wep.StatusCode, true
	}
	return 0, false
}
```

`StatusCode` is set on **every** path, including the two that build the value from an unmarshalled body (`client.go:394` assigns `wrappedError.StatusCode = &response.StatusCode` before the `Message == nil` branch). So a status code is always available when the error came from a real HTTP response. [VERIFIED: `azuredevops@v1.0.0-b5/client.go:392-394`]

### Pattern 2: Detecting a timeout regardless of which deadline fired

**What:** the deadline can come from `context.WithTimeout` (fetch budget, D-09) or from `http.Client.Timeout` (per-call budget via `connection.Timeout`). These produce different errors.

**Verified experimentally this session** with a `httptest` server and `&http.Client{Timeout: 100ms}`:

```
err: Get "http://127.0.0.1:61332": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
errors.Is(err, context.DeadlineExceeded): true
errors.As(err, &netErr) && netErr.Timeout(): true
```

[VERIFIED: executed `/tmp/tochk/m.go` this session on go1.27.1]

Both checks pass for the `http.Client.Timeout` case on a modern toolchain. The repo targets `go 1.25.4` (go.mod:3) and the local toolchain is go1.27.1, so the `errors.Is(err, context.DeadlineExceeded)` behaviour is confirmed for the toolchain in use but **not** independently confirmed for 1.25.4 specifically. Use the belt-and-braces form so the plan does not depend on that:

```go
var ne net.Error
isTimeout := errors.Is(err, context.DeadlineExceeded) ||
	(errors.As(err, &ne) && ne.Timeout())
```

Note ordering: `context.Canceled` is **not** a timeout. If Phase 3 later adds interrupt handling (CLI-04), a cancelled context must not be reported as `timeout` kind. Check `context.Canceled` explicitly or it will fall through to `other`, which is acceptable but should be a conscious choice.

### Pattern 3: Timeout must be set before sub-client construction

```go
// azuredevops@v1.0.0-b5/client.go:53-73
func NewClient(connection *Connection, baseUrl string) *Client {
	var client *http.Client
	...
	if connection.Timeout != nil {
		client.Timeout = *connection.Timeout
	}
	return &Client{ ... }
}
```

```go
// azuredevops@v1.0.0-b5/connection.go:36-48
type Connection struct {
	AuthorizationString     string
	BaseUrl                 string
	UserAgent               string
	SuppressFedAuthRedirect bool
	ForceMsaPassThrough     bool
	Timeout                 *time.Duration
	TlsConfig               *tls.Config
	clientCache             map[string]Client
	clientCacheLock         sync.RWMutex
	resourceAreaCache       map[uuid.UUID]ResourceAreaInfo
	resourceAreaCacheLock   sync.RWMutex
}
```

[VERIFIED: `azuredevops@v1.0.0-b5/client.go:53-73` and `connection.go:36-48`]

`Timeout` is a `*time.Duration`, read **once** at `azuredevops.NewClient` time, and the resulting `Client` is cached on the `Connection` (`clientCache`). The insertion point is therefore between line 39 and line 43 of `internal/ado/client.go`:

```go
// internal/ado/client.go:38-43 (current)
func NewClient(cfg *config.Config) (*Client, error) {
	connection := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)

	ctx := context.Background()

	workitemClient, err := workitemtracking.NewClient(ctx, connection)
```

[VERIFIED: internal/ado/client.go:38-43]

Setting `connection.Timeout` after any of the five `NewClient` calls at `client.go:43, 48, 53, 58, 63` — or after `location.NewClient` at `:70` — is a silent no-op for the already-constructed clients.

### Pattern 4: The existing UI message + reducer contract (D-16 extends this)

Every view already carries `Err error` beside its payload and branches on it:

```go
// internal/ui/prs/prs.go:154-162
	case PRsMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.err = nil
			m.items = msg.Items
			m.table = m.table.WithRows(m.buildRows())
		}
```

```go
// internal/ui/prs/prs.go:380-382
	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
```

[VERIFIED: internal/ui/prs/prs.go:154-162 and :380-382]

D-15's "keep last good rows, mark stale" is a direct modification of the `else` arm above — today `m.items` is simply not touched on error, so the rows *already* persist. What is missing is the *stale marker*, not the retention. That is a smaller change than the CONTEXT wording implies and the planner should size it accordingly.

`m.err` is a plain `error` field in all six views. D-14's blocking auth banner and D-13's warning line need a richer field — most likely the `Result` envelope itself, or a derived `(authFailed bool, warnLine string)` pair.

### Anti-Patterns to Avoid

- **Classifying by string-matching `err.Error()`.** `WrappedError.Error()` returns the server message, which is localised and can be an HTML fragment. D-06 exists precisely to stop this. Use `StatusCode`.
- **Short-circuiting the project loop on the first `auth` failure.** Explicitly rejected under D-08 after the cost was stated. Do not reintroduce it as an "obvious optimization".
- **Zeroing `DashboardCounts` on partial failure.** D-03 keeps the sum of projects that answered and labels it understated.
- **Creating a `context.WithTimeout` inside the `ado` methods.** The caller owns the budget; a method-internal deadline is invisible to Phase 3's `--timeout`.
- **Appending `Result[T]` to `workitems.go`.** 551 lines against a 300-line standard.
- **Deleting `continue` alone.** Phase 1 recorded this: for `GetWorkItems` the failed result slice is `nil`, so removing `continue` without changing the return is behaviourally inert. [CITED: STATE.md Decisions, Phase 01 — "Use continue-to-return propagation for the GetWorkItems mutation proof"]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Reaching the SDK error's status code | A string parser over `err.Error()` | `errors.As` against both `WrappedError` and `*WrappedError` | Messages are server-controlled and localised; `StatusCode` is always populated (client.go:394) |
| Timeout detection | Comparing elapsed time against the budget | `errors.Is(err, context.DeadlineExceeded)` + `net.Error.Timeout()` | Both deadline sources produce different concrete errors; the stdlib already unifies them |
| Per-call HTTP deadline | A custom `http.RoundTripper` or a `time.AfterFunc` wrapper | `connection.Timeout` | The SDK already plumbs it into `http.Client.Timeout` (client.go:62-63); the code simply never sets it |
| Duration parsing from YAML | A string field plus `time.ParseDuration` | `time.Duration` field with a `yaml:` tag | `gopkg.in/yaml.v3` parses `30s` / `2m` into `time.Duration` natively — `PollInterval` at config.go:46 already proves it |
| Aggregating multiple errors into one | A custom joiner | Structured `[]Failure` (D-02) | `errors.Join` flattens away the project/operation/kind fields D-06 requires |

**Key insight:** the entire phase is stdlib plus fields the SDK already exposes and the codebase already ignores. Any new dependency in this phase's plan is a red flag and should be challenged under CONVENTIONS rule 6.

## Runtime State Inventory

This is a behaviour-change phase, not a rename or migration. Categories assessed explicitly:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **None.** The tool has no datastore. No database, cache, or persisted collection references any of the changed types. | none |
| Live service config | **None from this phase.** Azure DevOps holds no configuration owned by this tool. The tool is a pure client. | none |
| OS-registered state | **None.** No scheduler entries, no daemons, no service units. Single binary invoked interactively. | none |
| Secrets/env vars | `ADO_PAT`, `ADO_ORG_URL`, `ADO_PROJECTS`, `ADO_PROJECT` are read at `internal/config/config.go:123-141`. **None are renamed by this phase.** The two new keys (`request_timeout`, `fetch_timeout`) are YAML-only under D-11 — no env override is specified, matching `PollInterval` which also has no env override. [VERIFIED: internal/config/config.go:122-142 — `loadFromEnv` handles exactly those four names and nothing else] | none; planner should note the deliberate absence of env overrides |
| Build artifacts | The committed binary `adoBizInt` at repo root is stale after any change. `.gitignore` ignores the compiled binary. | rebuild; no action beyond normal `go build` |

**Nothing found in four of five categories** — verified by reading `config.go` and by the absence of any persistence dependency in `go.mod`.

The one genuine runtime consideration is a **user-facing config file**: `~/.config/adoBizInt/config.yaml` (`internal/config/config.go:113`). Existing user config files will not have `request_timeout` or `fetch_timeout`. `Load()` must apply defaults for both, exactly as it does for `PollInterval` at `config.go:81-83`, or every existing install gets a zero-value `time.Duration` and therefore an *unbounded* client — the opposite of TRUST-04. [VERIFIED: internal/config/config.go:80-83 and :113]

## Code Inventory

### The eight aggregate reads and their swallow sites

All `continue` locations below are from a `grep -n continue internal/ado/*.go` executed this session. The *shape* of each read was confirmed by reading the surrounding source.

| # | Read | File:line of signature | Swallow `continue` | Structural note |
|---|------|------------------------|--------------------|-----------------|
| 1 | `GetWorkItems` | workitems.go:56 | **:77** (per-project), **:157** (per-batch) | Two independent swallows. :157 is inside `getProjectWorkItems` batching — a project can answer *partially*. D-02 requires this be reported, not marked complete. |
| 2 | `GetPullRequests` | pullrequests.go:34 | **:54** | Simplest case. Good tracer candidate. |
| 3 | `GetPipelineRuns` | pipelines.go:91 | **:114** | Also has inner `continue`s at :118, :126, :170 that are *data filters*, not error swallows — do not touch them. |
| 4 | `GetPipelineDefinitions` | pipelines.go:231 | **:253** | Inner `continue`s at :257, :271, :276 are filters. |
| 5 | `GetReleases` | releases.go:56 | **:73** | Inner `continue`s at :77, :85, :132 are filters. |
| 6 | `GetPendingApprovals` | releases.go:201 | **:220** | Inner `continue` at :224 is a filter. |
| 7 | `GetBranches` | branches.go:32 | **:51** (per-project repos), **:78** (per-repo branch stats) | **Nested fan-out.** The :78 site is already logged at `Warn` with the comment "Empty repositories have no branch stats to report" — a legitimate empty case is being conflated with a real failure. Classifying it will surface noise unless empty-repo is distinguished. |
| 8 | `GetDashboardCounts` | dashboard.go:23 | **:51** — **currently unreachable** | `getProjectCounts` (dashboard.go:74) returns `counts, nil` at :141 unconditionally. Its three failures at **:102**, **:116**, **:127** are logged and discarded in place. The outer `continue` is dead code. |

[VERIFIED: `grep -n continue internal/ado/*.go` this session; signature and return lines read directly in workitems.go, pullrequests.go, branches.go, dashboard.go, releases.go, pipelines.go]

**Filter-vs-failure discrimination is the trap in this table.** Twenty-six `continue` statements exist across the six files; only **ten** are error swallows. A plan task phrased as "replace `continue` with failure recording" will corrupt the filtering logic in `pipelines.go` and `releases.go`. Every task must name the exact line.

**`GetDashboardCounts` is a two-level change**, and its `getProjectCounts` returns `*DashboardCounts` — a pointer whose partial value is still meaningful under D-03. Propagating an error from `getProjectCounts` must *not* discard the counts it did collect, or D-03's "sum of the projects that answered" degrades to "sum of the projects that answered perfectly".

### The twelve exported `ado` types (OUT-02)

```
internal/ado/branches.go:15    type Branch struct
internal/ado/client.go:20      type Client struct
internal/ado/dashboard.go:15   type DashboardCounts struct
internal/ado/pipelines.go:16   type PipelineRun struct
internal/ado/pipelines.go:36   type PipelineDefinition struct
internal/ado/pullrequests.go:16 type PullRequest struct
internal/ado/releases.go:13    type Release struct
internal/ado/releases.go:27    type ReleaseEnvironment struct
internal/ado/releases.go:40    type ReleaseApproval struct
internal/ado/workitems.go:21   type WorkItem struct
internal/ado/workitems.go:34   type WorkItemDetail struct
internal/ado/workitems.go:49   type Attachment struct
```

[VERIFIED: `grep -n "^type [A-Z].* struct" internal/ado/*.go` this session]

**`Client` is not a serializable type** — all its fields are unexported and it holds live SDK clients. Eleven types get tags, not twelve. The plan must exclude `Client` explicitly or a naive "all exported structs" task will be ambiguous.

Field counts, all fields exported and all currently untagged [VERIFIED by reading each struct this session]:

| Type | Fields | Snake_case notes (D-04) |
|------|--------|-------------------------|
| `WorkItem` | 10 | `story_points`, `assigned_to`, `assigned_to_email` |
| `WorkItemDetail` | 10 + embedded `WorkItem` | **Embedded struct.** Untagged embedding inlines `WorkItem`'s fields into the JSON object — that is almost certainly the desired shape, but it is a decision, not a default. Fields: `acceptance_criteria`, `repro_steps`, `created_date`, `modified_date`, `created_by`, `modified_by`, `iteration_path`, `area_path` |
| `Attachment` | 3 | `name`, `url`, `size` — note `URL` → `url`, not `u_r_l` |
| `PullRequest` | 15 | `repository_id`, `repository_name`, `source_branch`, `target_branch`, `created_by`, `created_by_id`, `creation_date`, `is_draft`, `reviewer_count`, `approval_status` |
| `PipelineRun` | 17 | `build_number`, `pipeline_name`, `pipeline_id`, `source_branch`, `source_version`, `requested_by`, `requested_for`, `start_time`, `finish_time`, `queue_time`, `duration`, `repository_id` |
| `PipelineDefinition` | 7 | `repository_id`, `default_branch`, `queue_status` |
| `Release` | 10 | `definition_id`, `definition_name`, `created_on`, `modified_on`, `pending_approvals` |
| `ReleaseEnvironment` | 9 | `deployment_status`, `release_id`, `release_name`, `definition_id`, `definition_name` |
| `ReleaseApproval` | 11 | `release_id`, `release_name`, `environment_id`, `environment_name`, `approval_type`, `approver_id`, `created_on` |
| `Branch` | 10 | `repository_id`, `repository_name`, `default_branch`, `is_default`, `ahead_count`, `behind_count`, `last_commit_date`, `last_commit_by`, `commit_id` |
| `DashboardCounts` | 3 | `work_items`, `prs`, `pipelines` |

Two fields need an explicit call the planner should not leave to the executor:

- `PipelineRun.Duration` is a `time.Duration`. `encoding/json` marshals it as an **integer nanosecond count** — `3600000000000`, not `"1h0m0s"`. Under D-04 this becomes a locked CLI field contract in Phase 3. Decide now whether `duration` is nanoseconds or a string. [VERIFIED: internal/ado/pipelines.go:31 declares `Duration time.Duration`]
- Every `time.Time` field marshals as RFC 3339. Zero values become `"0001-01-01T00:00:00Z"` rather than being omitted, since none of these are pointers. Whether to add `,omitempty` is a contract decision — note that `,omitempty` does **not** suppress a zero `time.Time` anyway (it is a struct, never "empty" to `encoding/json`), so omitting a zero timestamp requires a pointer or a custom marshaller. Recommendation: do neither in this phase, and accept the sentinel date. [ASSUMED — standard `encoding/json` behaviour, not re-verified this session]

### The 22 `context.Background()` sites (TRUST-04)

```
internal/ado/client.go:41            ← construction only; leave as-is or bound separately
internal/ui/releases/releases.go:145 GetReleases          ◄ D-09 fetch deadline
internal/ui/releases/releases.go:154 GetPendingApprovals  ◄
internal/ui/releases/releases.go:162 ApproveRelease         (write — Phase 4)
internal/ui/releases/releases.go:170 RejectRelease          (write — Phase 4)
internal/ui/prs/prs.go:130           GetPullRequests      ◄
internal/ui/prs/prs.go:308           ApprovePullRequest     (write — Phase 4)
internal/ui/prs/prs.go:331           CompletePullRequest    (write — Phase 4)
internal/ui/workitems/detail.go:75   GetWorkItemDetail      (single-item read)
internal/ui/workitems/detail.go:191  DownloadAttachment     (open question / Phase 5)
internal/ui/workitems/workitems.go:136 GetWorkItems       ◄
internal/ui/workitems/workitems.go:203 GetChildWorkItems    (single-item read)
internal/ui/workitems/workitems.go:217 UpdateWorkItemState  (write — Phase 4)
internal/ui/workitems/workitems.go:285 UpdateWorkItemStateWithEffort (write)
internal/ui/workitems/workitems.go:298 UpdateWorkItemState  (write)
internal/ui/workitems/workitems.go:306 UpdateWorkItemStateWithEffort (write)
internal/ui/pipelines/trigger_modal.go:175 QueuePipeline    (write — Phase 4)
internal/ui/pipelines/pipelines.go:144 GetPipelineRuns    ◄
internal/ui/pipelines/pipelines.go:153 GetPipelineDefinitions ◄
internal/ui/pipelines/pipelines.go:161 QueuePipeline        (write — Phase 4)
internal/ui/dashboard/dashboard.go:97 GetDashboardCounts  ◄
internal/ui/branches/branches.go:98   GetBranches         ◄
```

[VERIFIED: `grep -rn "context.Background()" cmd internal --include="*.go" | grep -v _test.go` this session — 22 sites]

**Eight sites carry the aggregate reads** (marked ◄) and are D-09's fetch-deadline targets. TRUST-04 says "every ADO call carries a deadline". `connection.Timeout` satisfies that for *all* 22 at the transport layer, so the fetch-level `context.WithTimeout` only needs to cover the eight fan-outs. The planner should state that split explicitly, otherwise a reviewer will read TRUST-04 as requiring 22 edits and mark the phase incomplete.

**`internal/ado/client.go:41` is a genuine gap.** `NewClient` calls six SDK constructors, each of which performs a network round trip, on `context.Background()` with no deadline. Once `connection.Timeout` is set, five of those are bounded — but `location.NewClient` at `:70` returns no error and `getConnectionData` at `:77` is a real call on an unbounded context. This is startup, not refresh, so it is lower risk, but it is inside TRUST-04's literal scope and the plan should either fix it or record why not.

### Phase 1 test seam (how this phase must use it)

Four unexported interfaces exist in `internal/ado/interfaces.go`:

```go
// interfaces.go:17-23
type workItemAPI interface {
	UpdateWorkItem(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	GetWorkItems(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
	GetWorkItem(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiql(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetClassificationNode(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
}
```

`gitAPI` (7 methods, :25-33), `buildAPI` (4 methods, :35-40), `releaseAPI` (4 methods, :42-47). [VERIFIED: internal/ado/interfaces.go:17-47]

The header comment states the constraint the plan must not violate:

```go
// interfaces.go:12-16
// These package-local interfaces make destructive write methods testable without
// crossing the Azure DevOps REST boundary (SEAM-01). Each interface declares only
// methods this package calls; adding an uncalled method violates ROADMAP SC4.
// The SDK constructors return interfaces, so their assignments in NewClient are
// already compile-time drift checks. Do not add concrete ClientImpl assertions.
```

[VERIFIED: internal/ado/interfaces.go:12-16]

Injection is by direct struct literal — tests write `&Client{workitemClient: fake, projects: []string{"P"}}`. The `Client` struct fields at `client.go:20-36` are all unexported, so this works only from inside `package ado`. Every new test in this phase must live in `package ado`, not `package ado_test`. [VERIFIED: internal/ado/client.go:20-36; CITED: 02-CONTEXT.md §Reusable Assets]

**Testing an auth failure needs no new infrastructure.** A fake returns `azuredevops.WrappedError{StatusCode: ptr(401)}` and the classifier must produce kind `auth`. Testing the *value vs pointer* trap requires two cases in the same table — that test is the single most valuable new test in the phase.

Existing test files, with the eight characterization tests this phase inverts: `workitems_test.go` (253), `pullrequests_test.go` (293), `pipelines_test.go` (131), `releases_test.go` (182), `branches_test.go` (59), `dashboard_test.go` (90), `fakes_test.go` (218). [VERIFIED: `wc -l internal/ado/*_test.go` this session]

`dashboard_test.go:17-19` carries a comment recording exactly the structural trap named above:

```
// happens per call inside getProjectCounts, not at the aggregate loop's continue.
// per-project loop continues will find seven sites and silently miss this one.
```

[VERIFIED: `grep -n continue internal/ado/dashboard_test.go` this session]

### The eight message types (D-16)

| View | Message type | File:line | Reducer | Renders `m.err` |
|------|--------------|-----------|---------|-----------------|
| dashboard | `CountsMsg{Counts *ado.DashboardCounts; Err error}` | dashboard.go:17-20 | :128-136 | :234 |
| workitems | `WorkItemsMsg{...; Err error}` | workitems.go:44-47 | :358 | :682 |
| prs | `PRsMsg{Items; Err error}` | prs.go:33-36 | :154-162 | :380 |
| pipelines | `PipelinesMsg{...; Err error}` | pipelines.go:34-37 | :225 | — |
| pipelines | `DefinitionsMsg{...; Err error}` | pipelines.go:40-43 | :238 | — |
| releases | `ReleasesMsg{...; Err error}` | releases.go:37-40 | :237-240 | — |
| releases | `ApprovalsMsg{...; Err error}` | releases.go:43-45 | :246 | — |
| branches | `BranchesMsg{Items; Err error}` | branches.go:32-35 | :114-116 | :230 |

[VERIFIED: `grep -n "Msg struct\|m.err\|statusMsg\|statusErr" internal/ui/*/*.go` this session]

Two of these are already treated as *secondary* data and downgraded to a status-line warning rather than a banner:

```go
// internal/ui/pipelines/pipelines.go:238-241
	case DefinitionsMsg:
		if msg.Err != nil {
			m.statusMsg = "Warning: Could not load pipeline definitions - trigger may not work"
			m.statusErr = true
```

```go
// internal/ui/releases/releases.go:246-249
	case ApprovalsMsg:
		if msg.Err != nil {
			m.statusMsg = "Warning: Could not load pending approvals"
			m.statusErr = true
```

[VERIFIED: internal/ui/pipelines/pipelines.go:238-241; internal/ui/releases/releases.go:246-249]

D-14's blocking auth banner conflicts with this for `DefinitionsMsg` and `ApprovalsMsg`: an auth failure on the *secondary* fetch would blank a table that the *primary* fetch populated fine. Under a rejected PAT both fetches fail, so it is moot in practice — but the plan should state which message drives the blocking banner rather than letting two reducers race to set it. **Recommendation: only the primary message per view drives the auth banner** (`PipelinesMsg`, `ReleasesMsg`); the secondary keeps its warning line.

Also note `pipelines.go:232-233` currently sets a status-line message when a *successful* fetch returns zero rows:

```go
	m.statusMsg = "No pipeline runs found. Check configured pipeline names in config.yaml"
```

This is the exact "empty vs failed" confusion the phase exists to remove, and it becomes actively misleading once completeness is known. It should be gated on `Complete == true`.

## Common Pitfalls

### Pitfall 1: Classifying only `*WrappedError`

**What goes wrong:** 401s that arrive with a `text/plain` body, and every 4xx/5xx with a normal JSON error body, are returned as a `WrappedError` **value**. `errors.As(err, &ptr)` misses both. They fall through to kind `other`, the auth banner never fires, and TRUST-02 fails while every test that injects a `*WrappedError` passes.
**Why it happens:** `UnwrapError` mixes pointer and value returns across four paths (client.go:366, :389, :403, :411).
**How to avoid:** check both targets, and put both in the classifier's test table.
**Warning signs:** an auth test that passes but a real expired PAT still renders as an empty table.

### Pitfall 2: Setting `connection.Timeout` after `NewClient`

**What goes wrong:** silent no-op. `go build` passes, `go vet` passes, every test passes, and no HTTP call is bounded.
**Why it happens:** `Timeout` looks like a mutable field on a live object, but it is read once per sub-client at construction and the constructed clients are cached on the `Connection`.
**How to avoid:** set it on line 40 of `internal/ado/client.go`, immediately after `NewPatConnection`.
**Warning signs:** a hung refresh that never returns despite `request_timeout` being configured.

### Pitfall 3: Treating all 26 `continue` statements as swallows

**What goes wrong:** sixteen of them are legitimate data filters (nil pointer guards, name-pattern filters, repository allow-list checks). Converting them to failure records produces a failure list full of phantom entries and, worse, can invert filter logic.
**Why it happens:** a plan task phrased as "remove `continue`" instead of "change the error path at line N".
**How to avoid:** every task names the exact line from the inventory table above.
**Warning signs:** the failure list has more entries than there are configured projects.

### Pitfall 4: Assuming `GetDashboardCounts` is an outer-loop edit

**What goes wrong:** `dashboard.go:51`'s `continue` is unreachable because `getProjectCounts` returns `nil` error at `:141`. Editing only the outer loop changes nothing observable, and `dashboard_test.go` will still pass in its characterization form.
**Why it happens:** the dashboard looks structurally identical to the other seven at the loop level.
**How to avoid:** the dashboard task changes `getProjectCounts` (dashboard.go:74) to report its three in-place failures at `:102`, `:116`, `:127` **while still returning the counts it did collect** (D-03).
**Warning signs:** dashboard characterization test unchanged after the "inversion" commit.

### Pitfall 5: A zero-value duration in existing user config

**What goes wrong:** users with an existing `~/.config/adoBizInt/config.yaml` get `RequestTimeout == 0` and `FetchTimeout == 0`. A zero `http.Client.Timeout` means *no timeout*; a zero `context.WithTimeout` deadline expires **immediately**, so every fetch fails instantly with `context deadline exceeded`.
**Why it happens:** `Load()` applies defaults into the struct *before* `loadFromFile()` (config.go:81-90), so a file that omits the key correctly leaves the default intact — but only if the default was set in the literal at `:81-83`. Forgetting one of the two keys there is a one-line omission with two opposite catastrophic behaviours.
**How to avoid:** add both to the `&Config{...}` literal at `config.go:81-83` beside `PollInterval: 30 * time.Second`. Add a config test asserting both are non-zero after a `Load()` with an absent file.
**Warning signs:** "context deadline exceeded" on every project immediately after upgrade.

### Pitfall 6: Empty repositories reported as branch failures

**What goes wrong:** `branches.go:78` swallows a `GetBranches` error whose documented cause is "Empty repositories have no branch stats to report". Classifying it as a `Failure` turns an ordinary empty repo into a permanent warning line on the branches view.
**Why it happens:** the SDK signals "no branches" as an error rather than an empty slice.
**How to avoid:** classify by status code — an empty repo yields a 404-family response, distinct from a 401/403/timeout. Consider suppressing that specific case, or accept it and note the noise. This is the one place where the `not_found` kind rejected under D-07's deferred list would have earned its keep; the planner should decide consciously.
**Warning signs:** every refresh of the branches view shows a warning naming repos that are simply empty.

### Pitfall 7: `context.Canceled` classified as `timeout`

**What goes wrong:** when Phase 3 adds interrupt handling (CLI-04), Ctrl-C produces `context.Canceled`, not `DeadlineExceeded`. If the classifier checks `ctx.Err() != nil` rather than the specific sentinel, a user-initiated cancel is reported as a timeout and mapped to the wrong exit code.
**How to avoid:** check `context.DeadlineExceeded` specifically; let `context.Canceled` fall to `other` (or add an explicit case).
**Warning signs:** none in this phase — this bites in Phase 3. Worth a comment in the classifier.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `if err != nil { return err }` sentinel comparison | `errors.Is` / `errors.As` with `%w` wrapping | Go 1.13 | Mandatory for reaching `WrappedError` through any wrapping layer |
| Ad-hoc "multi-error" slices | `errors.Join` | Go 1.20 | Available but **wrong here** — it discards the structured fields D-02 requires |
| Concrete-type duplication for shared shapes | Type parameters | Go 1.18 | Makes `Result[T]` viable; go.mod:3 declares go 1.25.4 |
| `Timeout()` on `net.Error` | `errors.Is(err, context.DeadlineExceeded)` also true for `http.Client.Timeout` | recent toolchains | Verified true on go1.27.1 this session; use both checks for safety |

**Deprecated/outdated:**

- `github.com/pkg/errors` — superseded by stdlib `%w`. Not a dependency here and must not become one.
- `azure-devops-go-api v1.0.0-b5` is a 2020 beta. It has no retry, no backoff, and no 429 handling (`client.go:84` carries the literal comment `// todo: add retry logic`). [VERIFIED: azuredevops@v1.0.0-b5/client.go:84]. Upgrading is PLAT-02, deferred.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Azure DevOps returns HTTP 401 (not a 203 or a 302 sign-in redirect) for a rejected PAT on these endpoints. `NewPatConnection` sets `SuppressFedAuthRedirect: true` (connection.go:31-32, verified), which is the mechanism that forces 401 over a redirect — but the actual response code for an *expired* vs *revoked* vs *insufficient-scope* PAT was not observed. | Pattern 1, Pitfall 1 | The auth classifier never fires; TRUST-02 fails in production while passing every test. **Recommend a `checkpoint:human-verify` task in the plan: run the TUI against a deliberately invalid PAT and record the observed status code before finalising the classifier.** |
| A2 | Azure DevOps signals throttling as HTTP 429. The `rate_limit` kind (D-07) assumes this. ADO also uses 200-with-`Retry-After` and `X-RateLimit-*` headers in some tiers, which the SDK does not surface at all. | Pattern 1 | `rate_limit` never fires and throttling classifies as `other`. Low harm — D-07 only makes it *visible*, and PLAT-03 owns actual handling. |
| A3 | `encoding/json` marshals a zero `time.Time` as `"0001-01-01T00:00:00Z"` and `,omitempty` does not suppress it. Standard library behaviour, not re-verified this session. | OUT-02 inventory | A wrong assumption changes the Phase 3 JSON contract, which D-04 makes one-way. Cheap to verify with a three-line test. |
| A4 | An empty Azure DevOps repository causes `git.GetBranches` to return a 404-family error rather than an empty slice. Inferred from the existing `Warn`-level log and its comment at `branches.go:77`. | Pitfall 6 | The branches view gets a permanent warning line for every empty repo. Worth confirming against a real empty repo during execution. |
| A5 | `errors.Is(err, context.DeadlineExceeded)` holds for `http.Client.Timeout` on go1.25.4 specifically. Verified on go1.27.1 this session; the repo's declared toolchain is 1.25.4. | Pattern 2 | Mitigated by the recommended belt-and-braces `net.Error.Timeout()` check, which is stable across all versions. |

## Open Questions

1. **Generic `Result[T]` vs embedded non-generic outcome**
   - What we know: D-01 fixes the *contract* (items + failures + completeness, one shape). Go 1.25.4 supports generics. The codebase has zero generic types today.
   - What's unclear: whether the plan takes the generic or the embedded-struct route. CONVENTIONS rule 5 and rule of three both point at "shared", not specifically at "generic".
   - Recommendation: **generic.** Eight concrete types means Phase 3 writes eight JSON marshalling paths; one generic type means one. The rule-5 carve-out that justified the Phase 1 interfaces applies verbatim, and the plan should cite it in the same words.

2. **Where `Result`, `Failure`, and the classifier live**
   - Recommendation: **new file `internal/ado/result.go`.** `workitems.go` is 551 lines against a 300-line standard; appending there compounds a known violation. A new ~120-line file is the only option that leaves the package closer to the standard than it started.

3. **Default values for `request_timeout` and `fetch_timeout`**
   - What we know: `PollInterval` defaults to 30s (config.go:82). The N+1 fan-outs at `pipelines.go`, `releases.go`, `branches.go` mean a four-project fetch can issue dozens of sequential calls.
   - Recommendation: `request_timeout: 30s`, `fetch_timeout: 2m` — the values CONTEXT D-11 already uses as its illustrative parse examples. Critically, `fetch_timeout` must **exceed** `PollInterval` (30s) or a slow fetch is cancelled by its own deadline just as the next tick fires, producing a permanently-failing view. Consider a `Validate()` check for `FetchTimeout > PollInterval`.

4. **How the eight characterization tests are inverted**
   - Recommendation: **rewrite in place, preserving the `Test*SwallowsProjectFailure` names until the final commit, then rename to `Test*ReportsProjectFailure` in one sweep.** This keeps the Phase 1 anchored gate `^=== RUN   TestGet.*SwallowsProjectFailure$` meaningful as a countdown during the phase — it should go from 8 matches to 0, and that transition is the phase's cleanest completion signal.

5. **Shared vs per-view failure rendering**
   - What we know: six views need the same warning line, stale marker, and auth banner. Well past the rule of three.
   - Recommendation: **shared helpers in `internal/ui/styles`** (or a small sibling package), returning rendered strings. Do not attempt a shared Model — the six views have genuinely divergent state and a shared model would be the speculative generality CONVENTIONS forbids.

6. **`DownloadAttachment`'s `http.DefaultClient` (workitems.go ~:528)**
   - Recommendation: **defer to Phase 5.** TRUST-04's subject is "every ADO call"; `DownloadAttachment` sends the PAT to a caller-supplied URL and is C3/HARD-01. Adding a timeout there without the host allow-list bundles an unrelated risk into this phase's diff and makes the Phase 5 review harder. State the deferral explicitly in the plan so it is a decision, not an oversight.

7. **`internal/ado/client.go:41` startup context**
   - What we know: six SDK constructors plus `getConnectionData` run on an unbounded `context.Background()`. `connection.Timeout` bounds five of them at the transport layer once set.
   - Recommendation: bound `NewClient` with `context.WithTimeout(ctx, cfg.RequestTimeout)` for symmetry, or record explicitly why startup is exempt. Do not leave it unmentioned — a TRUST-04 reviewer will find it.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | everything | ✓ | go1.27.1 (go.mod declares 1.25.4) | — |
| `gofmt` | CONVENTIONS baseline | ✓ | bundled | — |
| `go vet` | CONVENTIONS baseline | ✓ | bundled | — |
| `azuredevops` SDK v1.0.0-b5 | classification, timeout | ✓ | in module cache | — |
| Live Azure DevOps + valid PAT | manual UAT of D-13/D-14/D-15 | unknown | — | Fakes cover all four failure kinds; **A1's status-code confirmation cannot be faked** |
| golangci-lint / CI | — | ✗ | — | Not required until Phase 5 HARD-05 |

**Missing dependencies with no fallback:**

- **A deliberately-invalid PAT against the real org**, needed to close assumption A1. There is no substitute — the whole point is observing what the server actually returns. This should be a `checkpoint:human-verify` task.

**Missing dependencies with fallback:**

- CI. Verification stays manual (`go build ./... && go vet ./... && gofmt -l . && go test ./...`), matching the current baseline.

## Validation Architecture

`.planning/config.json` was not found in the repo, so `workflow.nyquist_validation` is absent and is treated as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no assertion library, no mock framework) |
| Config file | none — `go test` convention |
| Quick run command | `go test ./internal/ado/... -count=1` |
| Full suite command | `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` |

Two packages currently have tests: `internal/ado` and `internal/ui/releases`. Fourteen have none. [CITED: CONVENTIONS.md §Toolchain Baseline]

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| TRUST-01 | Each of the 8 reads returns a failure entry naming project + operation | unit | `go test ./internal/ado/ -run 'TestGet.*ReportsProjectFailure' -count=1` | ✅ (invert the 8 existing) |
| TRUST-01 | `GetWorkItems` batch failure (workitems.go:157) is reported, project not marked complete | unit | `go test ./internal/ado/ -run TestGetWorkItemsReportsBatchFailure` | ❌ Wave 0 |
| TRUST-01 | `GetBranches` per-repo failure (branches.go:78) is reported | unit | `go test ./internal/ado/ -run TestGetBranchesReportsRepoFailure` | ❌ Wave 0 |
| TRUST-01 | `getProjectCounts` reports its three in-place failures and still returns collected counts | unit | `go test ./internal/ado/ -run TestGetDashboardCountsReportsPartial` | ✅ (invert `dashboard_test.go`) |
| TRUST-02 | 401 in a `WrappedError` **value** → kind `auth` | unit | `go test ./internal/ado/ -run TestClassifyFailure` | ❌ Wave 0 |
| TRUST-02 | 401 in a `*WrappedError` **pointer** → kind `auth` | unit | same | ❌ Wave 0 |
| TRUST-02 | 403 → kind `auth`; 429 → `rate_limit`; deadline → `timeout`; other → `other` | unit | same | ❌ Wave 0 |
| TRUST-02 | An auth failure yields a non-empty `Failures` list even when `Items` is empty | unit | `go test ./internal/ado/ -run 'TestGet.*AuthFailure'` | ❌ Wave 0 |
| TRUST-03 | `Complete == false` whenever `len(Failures) > 0`; `true` only on a clean sweep | unit | `go test ./internal/ado/ -run TestResultCompleteness` | ❌ Wave 0 |
| TRUST-04 | `connection.Timeout` is non-nil and equals `cfg.RequestTimeout` after `NewClient` | unit | `go test ./internal/ado/ -run TestNewClientSetsTimeout` — **note: `NewClient` performs network I/O, so this may need to be a construction-order assertion rather than a live call** | ❌ Wave 0 |
| TRUST-04 | Both timeout defaults are non-zero after `config.Load()` with no file | unit | `go test ./internal/config/ -run TestLoadAppliesTimeoutDefaults` | ❌ Wave 0 (package has **no** test file today) |
| TRUST-04 | An expired fetch context yields `timeout`-kind failures for unreached projects (D-12) | unit | `go test ./internal/ado/ -run TestFetchTimeoutMarksUnreached` | ❌ Wave 0 |
| OUT-02 | Each of the 11 serializable structs marshals to the D-04 snake_case names | unit | `go test ./internal/ado/ -run TestJSONFieldNames` | ❌ Wave 0 |
| OUT-02 | `PipelineRun.Duration` marshals to the agreed shape | unit | same | ❌ Wave 0 |
| TUI D-13/14/15 | Reducer sets banner / warning / stale from the envelope | unit | `go test ./internal/ui/... -count=1` — **`internal/ui/releases` is the only view package with a test file today; the other five need one created** | ❌ Wave 0 |
| TUI D-13/14/15 | Rendered output actually shows the banner | manual | visual check against an invalid PAT | manual-only — `View()` returns a lipgloss-styled string; asserting on it is brittle |

### Sampling Rate

- **Per task commit:** `go test ./internal/ado/... -count=1`
- **Per wave merge:** `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1`
- **Phase gate:** full suite green, **plus** `go test ./internal/ado/ -v -run TestGet 2>&1 | grep -c '^=== RUN   TestGet.*SwallowsProjectFailure$'` returns `0`

### Wave 0 Gaps

- [ ] `internal/ado/result_test.go` — classifier table covering value + pointer `WrappedError`, 401, 403, 429, deadline, canceled, nil-StatusCode. Covers TRUST-02.
- [ ] `internal/config/config_test.go` — **the `internal/config` package has no test file at all.** Covers TRUST-04 defaults.
- [ ] `internal/ado/json_test.go` — field-name assertions for the 11 serializable structs. Covers OUT-02.
- [ ] Fake extensions in `internal/ado/fakes_test.go` — the existing fakes fail loudly when a function field is unset (a deliberate Phase 1 property). Injecting *typed* SDK errors rather than `errors.New` may need new helper constructors.
- [ ] Test files for `internal/ui/dashboard`, `workitems`, `prs`, `pipelines`, `branches` — five view packages with no tests. Covers D-13/D-14/D-15 at the reducer level.
- [ ] Framework install: none needed.

## Security Domain

`.planning/config.json` is absent, so `security_enforcement` is treated as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | **yes** — indirectly | This phase does not authenticate; it *classifies* auth failure. The control is that a 401/403 is never silently rendered as success (TRUST-02). |
| V3 Session Management | no | PAT is stateless; no session. |
| V4 Access Control | no | Access control is Azure DevOps-side. Phase 4 AGENT-01..04 owns the client-side capability boundary. |
| V5 Input Validation | no | No new user input in this phase. WIQL interpolation is HARD-04, Phase 5. |
| V6 Cryptography | no | TLS via `http.Client` defaults. `Connection.TlsConfig` is left nil, which correctly means system defaults. |
| V7 Error Handling & Logging | **yes** | The central category for this phase. |
| V14 Configuration | **yes** — two new config keys | Zero-value duration handling (Pitfall 5). |

### Known Threat Patterns for Go + Azure DevOps PAT

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| **PAT leaked into a failure message.** `WrappedError.Message` is server-controlled, and if a failure entry ever wraps a request URL, the `Authorization` header is not in it — but `internal/ado/workitems.go`'s attachment path takes a caller-supplied URL. | Information Disclosure | `Failure` must carry `project`, `operation`, `kind`, and the underlying `error` — **never a request URL or header**. Add this as an explicit constraint on the `Failure` struct definition. |
| **Failure list surfaced to a TUI banner that renders raw server text.** `WrappedError.Error()` returns `*e.Message` verbatim, which for a rejected PAT can be an HTML sign-in fragment. | Information Disclosure / UI spoofing | Truncate and/or render the *kind* prominently and the raw message secondarily. `internal/utils.Truncate` already exists and is used at `releases.go:340`. |
| **Denial of service against the org via D-08's uncapped retry-on-every-refresh.** Four projects × failing calls every 30s, with zero backoff in the SDK (`client.go:84` `// todo: add retry logic`). | Denial of Service | **Accepted risk, explicitly.** D-08 rejected short-circuiting after the cost was stated. The `fetch_timeout` bound (D-09) is the only mitigation this phase adds. PLAT-03 owns the real fix. The plan must surface this, not absorb it. |
| **Unbounded call with no deadline hanging the TUI indefinitely.** 22 `context.Background()` sites, `connection.Timeout` never set. | Denial of Service (self-inflicted) | TRUST-04 — this phase's fix. |
| **Timing-based PAT validity oracle.** Not applicable — this is a single-user local tool. | — | — |

## Sources

### Primary (HIGH confidence)

- `internal/ado/*.go`, `internal/ui/*/*.go`, `internal/config/config.go`, `go.mod` — read directly this session; every file:line citation above was opened or grepped in this session.
- `~/go/pkg/mod/github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5/{client.go,connection.go,models.go}` — vendored SDK source, read directly. The value-vs-pointer `WrappedError` finding, `Connection.Timeout` semantics, and the `// todo: add retry logic` comment all come from here.
- Local Go experiment at `/tmp/tochk/m.go` — executed this session to confirm `errors.Is(err, context.DeadlineExceeded)` and `net.Error.Timeout()` both hold for `http.Client.Timeout`.

### Secondary (MEDIUM confidence)

- `.planning/phases/02-honest-reads/02-CONTEXT.md` — D-01 through D-16, canonical refs, discretion list.
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`, `.planning/codebase/CONVENTIONS.md` (§Toolchain, §Import grouping, §Receiver naming).

### Tertiary (LOW confidence)

- Assumptions A1–A5 in the Assumptions Log. A1 in particular is the one that should not reach implementation unconfirmed.

## Metadata

**Confidence breakdown:**

- Standard stack: **HIGH** — stdlib only, no new dependency, versions read from `go.mod`.
- Code inventory (file:line): **HIGH** — every line number came from a grep or a read in this session, not from CONTEXT.md restated.
- SDK error classification: **HIGH** — read from vendored source; the value/pointer split is quoted verbatim.
- Timeout plumbing: **HIGH** — SDK source plus a local experiment.
- Auth status code (A1): **LOW** — the one genuinely unverified fact, and it is load-bearing for TRUST-02.
- TUI integration points: **HIGH** — all eight message types and their reducers located by grep and confirmed by read.
- Pitfalls: **HIGH** for 1–5 and 7 (derived from read source), **MEDIUM** for 6 (A4).

**Research date:** 2026-09-07
**Valid until:** 2026-10-07 — nothing here is fast-moving. The SDK is a pinned 2020 beta and the repo is the source of truth for the rest.
