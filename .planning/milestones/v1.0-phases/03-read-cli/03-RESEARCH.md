# Phase 3: Read CLI - Research

**Researched:** 2026-09-08
**Domain:** Go CLI surface over an existing bubbletea TUI; stdlib `flag` dispatch, JSON output, exit-code contract, filter extraction
**Confidence:** HIGH — every claim below was read from source this session; file:line and verbatim quotes given

## Summary

Phase 3 is unusually well-prepared. Phase 2 already built the exact envelope the CLI serializes
(`Result[T]` with snake_case tags), all eight aggregate reads are already `ctx`-first, and — the
single biggest finding — **the non-test codebase writes to stdout exactly zero times today**.
OUT-03 is inherited, not built, provided the CLI branches before `tea.NewProgram`.

Three CONTEXT.md claims are wrong or incomplete and change the plan decomposition. First, the
`os.Exit(1)` line numbers in D-08 are off by four lines each. Second, `internal/ado` is 2,517
lines across 9 non-test files, not "2,015 across 7", and `workitems.go` is 591 lines, not 551 —
D-15's recorded cost is worse than recorded. Third, and most consequential for planning: the
free-text search filter D-17 promotes exists in **five** view packages, not two, and each searches
a **different field set**. There is also a **fourth workitems filter (`hideTasksFilter`) that D-17
does not mention at all.**

**Primary recommendation:** Split plan 03-02 (filter promotion) into two plans. Search-filter
promotion is a five-package, five-different-signature problem that is not the same shape as
hide-done/assigned-to-me promotion, and bundling them makes the behavior-equivalence testing
D-16 demands unmanageably wide for one plan.

## User Constraints (from CONTEXT.md)

### Locked Decisions

D-01 noun-verb tree. D-02 six nouns, eight verbs, 1:1 to `ado` read methods. D-03 `dashboard counts`.
D-04 nouns match Go package names, no aliases. D-05 partial exits non-zero. D-06 codes 0/1/2/3.
D-07 auth wins on mixed failure kinds. D-08 the three pre-read `os.Exit(1)` sites become exit 3.
D-09 flag-parse error and unknown subcommand exit 3. D-10 panic and SIGINT exit 3.
D-11 `--json` emits the raw `Result[T]` envelope and nothing else. D-12 human-readable default mode.
D-13 human mode: rows to stdout, failures to stderr. D-14 compact single-line JSON, no `--pretty`.
D-15 promoted filters live in `internal/ado` as pure functions. D-16 the TUI is rewired in this
phase. D-17 promoted set is hide-done, assigned-to-me, by-project, by-repo, free-text search;
priority sort and HTML stripping are NOT promoted. D-18 the CLI defaults to no filtering.
D-19 stdlib `flag`, one `FlagSet` per subcommand, zero new dependencies. D-20 global flags
registered into every FlagSet by a shared function. D-21 `--help` at three levels.
D-22 `ContinueOnError` + replaced `Usage`, exit 3 on flag error. D-23 panic emits structured JSON
on stdout. D-24 no stack trace in the record; log path only. D-25 SIGINT exits 3 like a panic.
D-26 the crash record is a distinct top-level `{"error":{...}}` object, not a `Result[T]`.

### the agent's Discretion

`items:null` vs `[]` (highest priority); unknown first argument behavior; what `--timeout`
overrides; file placement of promoted filters; identity source for `--assigned-to-me`; whether
the CLI path calls `logging.Init`; output ordering determinism; whether human mode reuses lipgloss;
whether D-08's exit-code changes count as TUI behavior change.

### Deferred Ideas (OUT OF SCOPE)

`config.Priorities` sort; HTML description stripping; all write verbs (Phase 4); exit codes 4+;
a fifth usage-error exit code; `--pretty`; invocation metadata in the envelope; a shared
`internal/filter` package; making failed identity fetch fatal; rate-limit backoff; N+1 fan-out;
CI (Phase 5); exporting the Phase 1 fakes.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-01 | No subcommand launches the TUI, no behavior change | Dispatch point verified at `main.go:59`; `main()` is one unconditional path, 69 lines total |
| CLI-02 | Read subcommands for six nouns | All eight `Get*` aggregate reads inventoried below with verified signatures |
| CLI-04 | `--timeout` deadline + interrupt handling | All reads already `ctx`-first; `fetchctx.New` hard-wires `context.Background()` — the blocking seam |
| CLI-05 | `--help` on every command | No `flag` import exists anywhere; three-level usage is entirely new hand-written code |
| CLI-06 | Filters promoted out of view packages | Five view packages hold filters; exact functions and entanglement mapped below |
| OUT-01 | `--json` on stdout | `Result[T]` at `result.go:98-102` is the serialization target, already tagged |
| OUT-03 | stdout carries only parseable output | **Zero stdout writers exist in non-test code.** Inherited, not built |
| OUT-04 | Four-outcome exit codes | `FailureKind` four constants at `result.go:25-36`; mapping is a `switch` over a field |
| OUT-05 | Panic emits structured JSON | No `recover()` exists anywhere in the repo. Entirely new |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Argument parsing / dispatch | `cmd/adoBizInt` | — | Process entry concern; nothing below main should know about `os.Args` |
| Filter predicates | `internal/ado` | — | ADO domain semantics (D-15); consumed by both CLI and TUI |
| JSON serialization | `internal/ado` (types) + `cmd` (encode) | — | Tags already live on the types; the encode call is an output concern |
| Exit-code mapping | `cmd/adoBizInt` | reads `ado.FailureKind` | Process status is not a domain concept; the input to the mapping is |
| Human-readable rendering | `cmd/adoBizInt` | — | New renderer; must NOT reuse `internal/ui` (pulls in bubbletea + lipgloss) |
| Panic recovery | `cmd/adoBizInt` | `internal/logging` | Top-level only; log path from `logging.LogPath()` |
| Context/deadline construction | `cmd/adoBizInt` | `internal/ui/fetchctx` (needs variant) | CLI needs a cancellable parent for SIGINT |

## Verified Codebase Facts

### 1. main entrypoint shape — VERIFIED

`cmd/adoBizInt/main.go` is 69 lines, one unconditional path. [VERIFIED: cmd/adoBizInt/main.go:1-69]

Verbatim, the exit sites:

```go
27: 		os.Exit(1)   // after config.Load failure
42: 		os.Exit(1)   // after cfg.Validate failure
49: 		os.Exit(1)   // after ado.NewClient failure
66: 		os.Exit(1)   // after p.Run() failure — TUI path only
```
[VERIFIED: cmd/adoBizInt/main.go:27,42,49,66 — confirmed by `grep -rn "os\.Exit"`, four sites total]

> **CORRECTION the planner must apply:** CONTEXT.md D-08 cites the three pre-read exit sites as
> `main.go:27`, `:38` and `:46`. The actual `os.Exit(1)` calls are at **:27, :42 and :49**.
> Lines 38 and 46 are the `log.Error(...)` calls that precede them. D-08's *intent* is correct and
> unchanged — the three pre-read aborts become exit 3 — but any plan step that cites a line number
> must use 27/42/49.

The dispatch point is confirmed:

```go
59: 	model := ui.NewModel(client, cfg.OrgURL, cfg.Projects, cfg.Repositories, cfg.StateTransitions, cfg.PollInterval, priorities)
60: 	p := tea.NewProgram(model, tea.WithAltScreen())
```
[VERIFIED: cmd/adoBizInt/main.go:59-60]

**What is TRUE now:** no argument handling of any kind. `grep -rn "\"flag\"\|os\.Args"` across all
non-test `.go` files returns **zero results**. [VERIFIED: repo-wide grep, no matches]

**What must change:** a dispatch branch inserted between `config.LoadPriorities` (`:54`) and
`ui.NewModel` (`:59`).

**Ordering constraint the planner must honor:** the branch must land before `:59`, but note that
`ado.NewClient` at `:45` performs network I/O in the constructor and both paths need it. So the
dispatch cannot be the *first* thing in `main()` if the CLI path needs a client — which it does.
The natural shape is: parse args early (before `logging.Init` even, to handle `--help` without
touching the filesystem), then run the shared prelude, then branch at `:59`. Getting `--help` and
unknown-subcommand errors to exit *without* requiring a valid PAT is a real design question the
plan should state: today `cfg.Validate()` at `:37` aborts if `ADO_PAT` is unset, and
`adoBizInt --help` failing because no PAT is configured would be hostile.

### 2. CLI library choice — VERIFIED, D-19 confirmed

`go.mod` has seven direct requires: `bubbles`, `bubbletea`, `lipgloss`, `bubble-table`,
`azure-devops-go-api`, `bubbletea-overlay`, `yaml.v3`. [VERIFIED: go.mod:5-13]

No cobra, no urfave/cli, no pflag — not even transitively in the indirect block.
[VERIFIED: go.mod:15-41]

**Recommendation: stdlib `flag`, as D-19 already locked.** The tradeoff, stated explicitly:

| | stdlib `flag` | cobra |
|---|---|---|
| New dependencies | 0 | ~4 direct + transitive |
| Subcommand tree | hand-rolled dispatch on `os.Args[1]`/`[2]` | built in |
| `-h` per subcommand | **not free** — must wire manually | free |
| Three-level `--help` | ~3 hand-written usage funcs | generated |
| Flag-error exit code | overridable via `ContinueOnError` | overridable |
| Interspersed flags (`prs list --json`) | works — `FlagSet.Parse(os.Args[3:])` | works |

The accepted cost is real: stdlib `flag` prints usage and calls `os.Exit(2)` on error under the
default `ExitOnError`, which D-22 correctly identifies as a lie once 2 means auth failure.
`flag.NewFlagSet(name, flag.ContinueOnError)` plus a replaced `fs.Usage` is the standard fix.
Also note `fs.SetOutput(os.Stderr)` is needed — `ContinueOnError` still writes the error message
to the FlagSet's output, which defaults to `os.Stderr` already but should be set explicitly so a
future change cannot silently redirect it to stdout and violate OUT-03. [ASSUMED — stdlib
behavior from training knowledge; the plan should verify with a one-line test]

### 3. Read operations inventory — VERIFIED

All eight aggregate reads, verbatim signatures:

```go
internal/ado/workitems.go:76:    func (c *Client) GetWorkItems(ctx context.Context, projects []string) (Result[WorkItem], error)
internal/ado/pullrequests.go:44: func (c *Client) GetPullRequests(ctx context.Context, projects []string) (Result[PullRequest], error)
internal/ado/pipelines.go:111:   func (c *Client) GetPipelineRuns(ctx context.Context, projects []string) (Result[PipelineRun], error)
internal/ado/pipelines.go:264:   func (c *Client) GetPipelineDefinitions(ctx context.Context, projects []string) (Result[PipelineDefinition], error)
internal/ado/releases.go:64:     func (c *Client) GetReleases(ctx context.Context, projects []string) (Result[Release], error)
internal/ado/releases.go:222:    func (c *Client) GetPendingApprovals(ctx context.Context, projects []string) (Result[ReleaseApproval], error)
internal/ado/branches.go:52:     func (c *Client) GetBranches(ctx context.Context, projects, repositories []string) (Result[Branch], error)
internal/ado/dashboard.go:44:    func (c *Client) GetDashboardCounts(ctx context.Context, projects []string) (Result[DashboardCounts], error)
```
[VERIFIED: repo-wide `grep -rn "^func (c \*Client) Get" internal/ado/*.go`]

**CONTEXT.md's claim that all eight are `ctx`-first and return the envelope is CONFIRMED.**

**One signature asymmetry the plan must handle:** `GetBranches` is the only read taking a second
slice parameter (`repositories`). Every other read takes `(ctx, projects)`. A generic dispatch
table keyed on noun-verb cannot use one uniform function type across all eight without either a
closure per command or an adapter. This is small but it will surface the moment someone tries to
write `map[string]func(context.Context, []string) (any, error)`.

**Three non-aggregate reads exist and are NOT in scope** (they return bare types, not `Result[T]`):
`GetRecentIterations` (`client.go:170`), `GetBranchesForPipeline` (`pipelines.go:361`),
`GetWorkItemDetail` (`workitems.go:334`), `GetChildWorkItems` (`workitems.go:452`).
[VERIFIED: same grep] D-02's eight-verb list correctly excludes all four.

**Where results are rendered today:** each of the six view packages holds `client *ado.Client`
concretely and renders via its own `buildRows() []table.Row`. Confirmed five `buildRows` methods:
`releases.go:532`, `prs.go:497`, `workitems.go:612`, `pipelines.go:505`, `branches.go:327`.
[VERIFIED: `grep -rn "func .*buildRows" internal/ui`]

### 4. Filtering today — VERIFIED, and D-17 is INCOMPLETE

This is the section where the codebase diverges most from CONTEXT.md.

#### 4a. workitems — four filters, not three

```go
internal/ui/workitems/workitems.go:36-41:
var doneStates = map[string]bool{
	"Done":     true,
	"Closed":   true,
	"Resolved": true,
	"Removed":  true,
}
```
[VERIFIED: internal/ui/workitems/workitems.go:36-41 — verbatim]

Model filter fields:

```go
	assignedToMeFilter bool
	hideDoneFilter     bool
	hideTasksFilter    bool
	currentUser        string
	currentUserEmail   string
	filterFocused      int // 0 = table, 1 = checkbox 1, 2 = checkbox 2, 3 = search
```
[VERIFIED: internal/ui/workitems/workitems.go, model struct — verbatim]

The predicates inside `buildRows()` at `:612`:

```go
		if m.assignedToMeFilter && m.currentUserEmail != "" && !strings.EqualFold(item.AssignedToEmail, m.currentUserEmail) {
		if m.hideDoneFilter && doneStates[item.State] {
		if m.hideTasksFilter && item.Type == "Task" {
```
[VERIFIED: internal/ui/workitems/workitems.go:612+ — verbatim]

> **GAP the planner must close:** `hideTasksFilter` (`item.Type == "Task"`) is a fourth filter
> that CONTEXT.md D-17 never names. It is the same shape as hide-done — a pure predicate over one
> field — and leaving it un-promoted means workitems filtering lives in two places after this
> phase, which is exactly what CLI-06 forbids. The planner should either promote it (recommended,
> it costs one function) or record an explicit decision not to. It cannot be silently skipped.

Defaults, verbatim:

```go
		assignedToMeFilter: true,
		hideDoneFilter:     true,
```
[VERIFIED: internal/ui/workitems/workitems.go, NewModel return literal — verbatim. Note
`hideTasksFilter` is absent from the literal, so it defaults to `false`.]

D-18's claim that the TUI defaults both to `true` is CONFIRMED.

#### 4b. by-project and by-repo — only in `prs`

```go
internal/ui/prs/prs.go:514-526:
		// Filter by configured projects
		if len(validProjects) > 0 && !validProjects[strings.ToLower(item.Project)] {
			continue
		}
		// Filter by configured repositories (if any are defined)
		if len(validRepos) > 0 && !validRepos[strings.ToLower(item.RepositoryName)] {
			continue
		}
```
[VERIFIED: internal/ui/prs/prs.go:497-526 — verbatim]

**Important asymmetry:** by-repo filtering for *branches* is already done at the `ado` layer —
`GetBranches(ctx, projects, repositories)` takes the repo list as a parameter
(`branches.go:52`). So "by-repo" is half-promoted already, and the promoted predicate must be
consistent with whatever `GetBranches` does internally or the CLI will filter twice with two
different rules. The plan needs a step that reads `GetBranches`' internal filtering before
writing the promoted predicate.

#### 4c. Free-text search — FIVE packages, five different field sets

| Package | Line | Fields searched |
|---|---|---|
| `workitems` | `workitems.go:612+` | `ID` (as string), `Title` |
| `prs` | `prs.go:525-536` | `ID`, `Title`, `RepositoryName`, `CreatedBy` |
| `branches` | `branches.go:332-339` | `Name`, `RepositoryName`, `LastCommitBy` |
| `releases` | `releases.go:538-545` | `Name`, `DefinitionName` |
| `pipelines` | `pipelines.go:509-521` | `PipelineName`, `SourceBranch`, `RequestedBy`, `BuildNumber` |

[VERIFIED: all five read this session — `internal/ui/workitems/workitems.go:612+`,
`internal/ui/prs/prs.go:497-540`, `internal/ui/branches/branches.go:327-340`,
`internal/ui/releases/releases.go:532-546`, `internal/ui/pipelines/pipelines.go:505-522`]

Every one opens with the identical line:

```go
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
```
[VERIFIED: verbatim, identical in all five files at the lines above]

> **CORRECTION the planner must apply:** CONTEXT.md D-17 says search is "currently in the `prs`
> and `branches` views" and D-16's risk note scopes the rewire to "three of them (`workitems`,
> `prs`, `branches`) — the ones with filters — not all six." **This is wrong. Five of the six
> views have a search filter.** Promoting search touches `releases` and `pipelines` too.

**The entanglement — this is the real work of 03-02:**

The filters are not separable predicates today. In every package the filter is a `continue` inside
the loop that builds `[]table.Row`, reading directly off `m` (the bubbletea Model). Specifically
they are entangled with:

1. **`m.searchInput` — a `textinput.Model` from `bubbles`.** The search term's source of truth is
   a TUI widget's `.Value()`. A promoted `MatchesSearch(item, query string) bool` needs the query
   as a plain string, so each view must change to read `.Value()` once and pass it down. That is
   mechanical but it is five files.
2. **`m.currentUser` / `m.currentUserEmail`** — cached identity on the Model, sourced from the
   client (`client.go:161` `CurrentUser()`, `:165` `CurrentUserEmail()`). Promoted
   assigned-to-me needs identity as a parameter, not a Model field.
3. **`m.projects` / `m.repositories`** — config slices copied onto the Model, converted to a
   `map[string]bool` lookup *inside* `buildRows` on every call (`prs.go:501-511`). The promoted
   version should take the slice and do its own normalization.
4. **The priority sort, which is interleaved between filtering and row-building**
   (`workitems.go` — the `if m.assignedToMeFilter && m.priorities != nil` block sits *between*
   the filter loop and the row loop). D-17 correctly leaves the sort in place, but note it is
   **gated on `assignedToMeFilter`** — so extracting the filter without care changes when the sort
   runs. This is the single highest-risk edit in the phase for CLI-01's no-behavior-change rule.

**Recommended decomposition change:** split 03-02 into `03-02a` (hide-done, hide-tasks,
assigned-to-me, by-project, by-repo — three packages) and `03-02b` (free-text search — five
packages, five field sets). They have different blast radii and different test shapes, and D-16
demands a behavior-equivalence test per promoted filter, which is 9+ tests if bundled.

#### 4d. File placement

`internal/ado` non-test files, actual line counts:

```
internal/ado/branches.go       163
internal/ado/client.go         285
internal/ado/dashboard.go      195
internal/ado/interfaces.go      47
internal/ado/pipelines.go      439
internal/ado/pullrequests.go   261
internal/ado/releases.go       349
internal/ado/result.go         187
internal/ado/workitems.go      591
                             -----
                              2517  across 9 files
```
[VERIFIED: `wc -l internal/ado/*.go`, non-test files]

> **CORRECTION:** CONTEXT.md D-15 records "2,015 lines across 7 files" and "`workitems.go` alone
> is 551". The real figures are **2,517 across 9** and **`workitems.go` is 591**. D-15's recorded
> cost is understated by ~25%. The decision itself does not change, but the planner should carry
> the accurate number, and a new `filters.go` making it 10 files is worth stating out loud.

### 5. Output discipline — VERIFIED, and better than expected

```
$ grep -rn "fmt\.Print\|os\.Stdout\|println(" --include="*.go" . | grep -v "_test.go"
(no output)
```
[VERIFIED: repo-wide grep this session, zero matches in non-test code]

**Nothing in the non-test codebase writes to stdout today.** Every diagnostic in `main.go` uses
`fmt.Fprintf(os.Stderr, ...)` — four sites at `:18`, `:26`, `:39-41`, `:48`, `:65`.
[VERIFIED: cmd/adoBizInt/main.go]

Logging goes to a file, verbatim:

```go
internal/logging/logging.go:23-31:
	logPath := filepath.Join(logDir, "adoBizInt.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	logger = slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
```
[VERIFIED: internal/logging/logging.go:23-31 — verbatim]

And the fallback path:

```go
internal/logging/logging.go:36-42:
func Logger() *slog.Logger {
	if logger == nil {
		// Fallback to stderr if not initialized
		return slog.Default()
	}
	return logger
}
```
[VERIFIED: internal/logging/logging.go:36-42 — verbatim]

`slog.Default()` writes to **stderr**, so even the `logging.Init` failure path does not violate
OUT-03. [ASSUMED — stdlib `slog` default handler target; worth a one-line assertion in the plan
rather than trusting it]

**The only stdout writer that will exist is bubbletea itself.** `tea.NewProgram(model, tea.WithAltScreen())`
at `main.go:60` writes the full-screen TUI to stdout. Since the CLI dispatch branches before
`:59`, bubbletea is never constructed on the CLI path and OUT-03 holds structurally.

**What must change:** essentially nothing defensive. The plan should add a **regression test** that
asserts stdout is byte-exact JSON for a `--json` invocation — not because something violates it
today, but because the property is currently accidental and one `fmt.Println` in a future phase
breaks it silently.

**One risk:** D-12's human-readable renderer is the first deliberate stdout writer in the repo.
If it reuses `internal/ui/styles` (lipgloss), ANSI escape codes land in redirected stdout. The
discretion item flagging this is correct and the answer should be plain text.

### 6. Phase 2 seam — VERIFIED, concrete type names

The four failure kinds, verbatim:

```go
internal/ado/result.go:25-36:
const (
	// FailureAuth means the credential was rejected — the read cannot succeed
	// until the operator acts.
	FailureAuth FailureKind = "auth"
	// FailureTimeout means a deadline elapsed before the server answered.
	FailureTimeout FailureKind = "timeout"
	// FailureRateLimit means the server asked the caller to slow down.
	FailureRateLimit FailureKind = "rate_limit"
	// FailureOther covers everything else, including transport errors and any
	// server status this classifier does not recognise.
	FailureOther FailureKind = "other"
)
```
[VERIFIED: internal/ado/result.go:25-36 — verbatim]

The envelope, verbatim:

```go
internal/ado/result.go:98-102:
type Result[T any] struct {
	Items    []T       `json:"items"`
	Failures []Failure `json:"failures"`
	Complete bool      `json:"complete"`
}
```
[VERIFIED: internal/ado/result.go:98-102 — verbatim]

The `Failure` record, verbatim:

```go
internal/ado/result.go:61-66:
type Failure struct {
	Project   string      `json:"project"`   // the configured project whose read failed
	Operation string      `json:"operation"` // short label for the call being made, e.g. "get pull requests"
	Kind      FailureKind `json:"kind"`      // "auth", "timeout", "rate_limit", "other"
	Err       error       `json:"-"`         // the underlying error, preserved for errors.Is / errors.As
}
```
[VERIFIED: internal/ado/result.go:61-66 — verbatim]

The cancellation-before-timeout ordering D-25 depends on, verbatim with its comment:

```go
internal/ado/result.go:140-145:
	// Cancellation is checked before the timeout branch and deliberately falls
	// through to FailureOther. Phase 3's CLI-04 interrupt handling depends on a
	// user-initiated cancel not being reported as a server timeout.
	if errors.Is(err, context.Canceled) {
		return FailureOther
	}
```
[VERIFIED: internal/ado/result.go:140-145 — verbatim. Note: this is line 143 for the `errors.Is`,
not `:135` as CONTEXT.md cites; `:135` is the `func classifyFailure` declaration line.]

**Exit-code mapping is mechanical.** D-06/D-07 map onto existing types with no new vocabulary:
scan `result.Failures` for any `Kind == FailureAuth` → exit 2; else if `!result.Complete` → exit 1;
else exit 0. There is no parallel taxonomy to invent.

**There is a contract test the plan must not break:**

```go
internal/ado/json_test.go:62:  func TestJSONFieldNames(t *testing.T)
internal/ado/json_test.go:321: func TestJSONFailureErrIsFlattenedString(t *testing.T)
```
[VERIFIED: `grep -n "func Test" internal/ado/json_test.go`]

And `result.go:47-50` states verbatim: *"From Phase 3 onward these four field names are a
machine-readable public contract. Adding a fifth field REQUIRES updating the exact-key-set
assertion in json_test.go in the same change."* [VERIFIED: internal/ado/result.go:47-50]

#### The `items: null` hazard — CONFIRMED, and it is worse than CONTEXT.md states

```go
internal/ado/result.go:106-112:
func newResult[T any](items []T, failures []Failure) Result[T] {
	return Result[T]{
		Items:    items,
		Failures: failures,
		Complete: len(failures) == 0,
	}
}
```
[VERIFIED: internal/ado/result.go:106-112 — verbatim]

No pre-allocation. A nil `items` marshals as `null`, and `jq '.items | length'` errors rather than
returning 0. **Both `items` and `failures` are affected**, and `failures` is nil on *every
successful read* — so the common case emits `"failures":null`. An agent checking
`.failures | length == 0` to decide "clean read" gets an error on the happy path.

This is the highest-priority discretion item and CONTEXT.md is right to flag it. It is a two-line
fix in `newResult` plus two assertions. **It must ship in the first plan that emits JSON, not the
last.**

### 7. Context/timeout plumbing — VERIFIED

**Good news:** all eight reads already accept `context.Context` as the first parameter. No `ado`
signature changes needed. [VERIFIED: the eight signatures in §3 above]

**The blocker** is `fetchctx`, verbatim:

```go
internal/ui/fetchctx/fetchctx.go:28-33:
func New(budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultBudget
	}
	return context.WithTimeout(context.Background(), budget)
}
```
[VERIFIED: internal/ui/fetchctx/fetchctx.go:28-33 — verbatim]

`context.Background()` is hard-wired. The package doc explains why, verbatim:

```
internal/ui/fetchctx/fetchctx.go:6-9:
// which is well past the rule of three. Extracting it also means no view file
// mentions context.Background() on a read path at all, so the absence of an
// unbounded read context is visible by inspection rather than by reading six
// closures (D-09, threat T-02-04).
```
[VERIFIED: internal/ui/fetchctx/fetchctx.go:6-9 — verbatim]

**The CLI cannot use `fetchctx.New` as-is** because SIGINT handling requires a cancellable parent
(`signal.NotifyContext(ctx, os.Interrupt)`), and `New` gives no way to supply one.

Two options, both acceptable:
- **(a)** Add `fetchctx.NewWithParent(parent context.Context, budget time.Duration)` and make
  `New` call it with `context.Background()`. Preserves the package's stated invariant (no view
  file mentions `Background`) and gives the CLI what it needs. **Recommended.**
- **(b)** The CLI constructs its own `context.WithTimeout` in `cmd/`. Simpler, but duplicates the
  zero-budget-default logic (`defaultBudget = 60 * time.Second`, `fetchctx.go:23`) that exists
  precisely to prevent an unbounded read.

Option (b) reintroduces the failure `fetchctx` was written to remove. Prefer (a).

**The two budgets `--timeout` might map to**, verbatim:

```go
internal/config/config.go:46-48:
	PollInterval       time.Duration                `yaml:"poll_interval"`
	RequestTimeout     time.Duration                `yaml:"request_timeout"` // Bounds a single HTTP call to Azure DevOps
	FetchTimeout       time.Duration                `yaml:"fetch_timeout"`   // Bounds a whole multi-project fan-out
```
[VERIFIED: internal/config/config.go:46-48 — verbatim]

Defaults, verbatim:

```go
internal/config/config.go:86-88:
		PollInterval:   30 * time.Second,
		RequestTimeout: 30 * time.Second,
		FetchTimeout:   2 * time.Minute,
```
[VERIFIED: internal/config/config.go:86-88 — verbatim]

**Recommendation for the open discretion item:** `--timeout` should map to `FetchTimeout` (the
whole-invocation budget) only. That is what "per-invocation deadline" means in CLI-04, it is the
one an agent actually cares about, and `RequestTimeout` is baked into the `connection.Timeout` at
client construction (`main.go:45`) which happens before flag values could apply it cleanly.

**A validation constraint the plan must check:**

```go
internal/config/config.go:159:
	if c.FetchTimeout <= c.PollInterval {
```
[VERIFIED: internal/config/config.go:159]

`Validate()` rejects a `FetchTimeout` at or below `PollInterval` (default 30s). If `--timeout`
writes into `cfg.FetchTimeout` *before* `cfg.Validate()` runs, then `adoBizInt prs list --timeout 10s`
fails validation for a reason that makes no sense on a CLI path — `PollInterval` is a TUI refresh
concept with no meaning for a one-shot read. The plan must either apply `--timeout` after
validation, or exempt the CLI path from that particular check. **This is a concrete, easy-to-miss
bug and should be an explicit acceptance criterion.**

### 8. Panic-to-JSON — nothing exists today

```
$ grep -rn "recover()" --include="*.go" .
(no output)
$ grep -rn "os/signal\|signal.Notify" --include="*.go" .
(no output)
```
[VERIFIED: repo-wide greps this session, zero matches including test files]

Both the panic handler and signal handling are entirely new code.

**The Go idiom and its sharp edge:**

```go
func main() {
    os.Exit(run())   // run() owns all defers; main() only exits
}

func run() (code int) {
    defer func() {
        if r := recover(); r != nil {
            // emit {"error":{"kind":"panic",...}} to stdout
            code = 3
        }
    }()
    ...
}
```

**`os.Exit` does not run deferred functions.** [ASSUMED — well-established Go semantics, but the
plan should treat it as a hard constraint regardless of provenance because getting it wrong
silently disables the entire panic handler.] This means:

- **Every one of the four existing `os.Exit(1)` calls at `main.go:27,42,49,66` must become a
  `return 3` (or `return 1`) out of `run()`.** If any `os.Exit` survives inside the recovered
  scope, the deferred handler never fires for that path. D-08 already renumbers three of them;
  the plan must additionally *restructure* them from `os.Exit` to `return`. CONTEXT.md D-08 talks
  about renumbering but not about restructuring — **this is a missing implementation detail.**
- The fourth site (`:66`, TUI run failure) is on the TUI path. CLI-01 forbids behavior change, and
  it currently exits 1. It should keep exiting 1 unless the planner decides otherwise — but it
  still has to become `return 1` rather than `os.Exit(1)` if it sits inside `run()`.

**D-23's atomicity constraint is real and correctly identified.** `json.NewEncoder(os.Stdout).Encode(v)`
streams — a panic mid-encode leaves a partial document on stdout followed by the crash record,
which is unparseable. The fix is `json.Marshal` to a `[]byte` first, then one `os.Stdout.Write`.
The plan should state this as a rule, not a preference.

**D-24's security reasoning is sound and verifiable.** Go's panic output includes function
arguments, and `ado.NewClient(cfg)` (`main.go:45`) takes a `config.Config` carrying the PAT. The
`Failure` doc comment already codifies this class of constraint at `result.go:41-45`. Keeping the
stack off stdout is consistent with an existing, documented invariant — not a new rule.

**SIGINT:** `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` gives a
cancellable parent. When it fires, the in-flight read returns `context.Canceled`, which
`classifyFailure` maps to `FailureOther` by the explicit branch at `result.go:143` — exactly as
D-25 assumes. The CLI must then distinguish "interrupt" from "generic other failure" at the
process level, which it can do by checking whether the signal context's `Err()` is non-nil rather
than by inspecting the failure kind.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal → cancellable context | Manual `signal.Notify` + goroutine + channel | `signal.NotifyContext` | stdlib since 1.16; handles the goroutine, the cleanup, and the double-signal case |
| Failure→exit-code mapping | String matching on error text | `switch` on `Failure.Kind` | `classifyFailure` already did the structural work; `result.go:131-134` explicitly warns that error text is server-controlled and localisable |
| JSON field naming | A DTO layer in `cmd/` | The existing tags on `Result[T]` and payload types | Phase 2 OUT-02 is already complete and `json_test.go` guards it |
| Duration flag parsing | Custom `time.ParseDuration` wrapper | `flag.DurationVar` | stdlib; matches the `time.Duration` config fields exactly |
| Test fakes for CLI paths | New mocks | `internal/ado/fakes_test.go` + struct-literal injection | Phase 1 built this seam; no testify or mock generator is in `go.mod` |

## Common Pitfalls

### Pitfall 1: `os.Exit` inside the recovered scope
**What goes wrong:** the panic handler silently never runs for the config/validate/client paths.
**Why it happens:** the existing code uses `os.Exit(1)` in four places and the natural edit is to
change the integer, not the control flow.
**How to avoid:** move all of `main()`'s body into `run() int`; `main()` contains only
`os.Exit(run())`. Grep for `os.Exit` after the change — exactly one should remain.
**Warning sign:** a test that panics deliberately and gets no JSON on stdout.

### Pitfall 2: `--timeout` tripping `Validate()`'s `FetchTimeout > PollInterval` check
**What goes wrong:** `--timeout 10s` errors with a message about poll interval.
**Why it happens:** `config.go:159` couples a TUI-refresh concept to the fetch budget.
**How to avoid:** apply the flag override after `Validate()`, or skip that check on the CLI path.

### Pitfall 3: extracting the workitems filter changes when the priority sort runs
**What goes wrong:** TUI row order changes — a CLI-01 violation.
**Why it happens:** the sort block is gated on `m.assignedToMeFilter` and sits *between* the
filter loop and the row loop in `buildRows()`.
**How to avoid:** the behavior-equivalence test D-16 demands must assert row *order*, not just
row set.

### Pitfall 4: `"failures":null` on every successful read
**What goes wrong:** an agent's `.failures | length` errors on the happy path.
**Why it happens:** `newResult` (`result.go:106-112`) passes the caller's possibly-nil slices straight through.
**How to avoid:** pre-allocate both in `newResult`; assert both in a test before any verb ships.

### Pitfall 5: promoting search as one function
**What goes wrong:** five call sites need five different field sets; a single
`MatchesSearch(item any, q string)` becomes a type switch.
**How to avoid:** one small typed predicate per payload type, or a
`func(T) []string` field-extractor passed to a shared matcher.

## Project Constraints

No `AGENTS.md` or `.opencode/AGENTS.md` exists in this repo. [VERIFIED: `ls` returned nothing for
both paths] Constraints therefore come from `.planning/PROJECT.md` and `.planning/codebase/CONVENTIONS.md`
as cited in CONTEXT.md: no new dependencies without justification (satisfied by D-19), no
abstraction unless the current task requires it, files under 300 lines (already violated by five
files in `internal/ado`), TUI behavior unchanged.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, table-driven |
| Config file | none — `go test ./...` |
| Quick run | `go test ./internal/ado/... ./cmd/...` |
| Full suite | `go test ./...` |

No testify, no mock generator, no gomock in `go.mod`. [VERIFIED: go.mod:5-41, no test deps]

### Phase Requirements → Test Map
| Req | Behavior | Type | Command | Exists? |
|---|---|---|---|---|
| CLI-01 | no args → TUI, unchanged | unit | `go test ./cmd/...` | Wave 0 |
| CLI-02 | eight verbs dispatch to eight methods | unit | `go test ./cmd/...` | Wave 0 |
| CLI-04 | `--timeout` bounds; SIGINT cancels | unit | `go test ./cmd/...` | Wave 0 |
| CLI-05 | three `--help` levels | unit | `go test ./cmd/...` | Wave 0 |
| CLI-06 | promoted filters behave identically pre/post | unit | `go test ./internal/ado/... ./internal/ui/...` | Wave 0 |
| OUT-01 | `--json` stdout parses | unit | `go test ./cmd/...` | Wave 0 |
| OUT-03 | stdout is byte-exact JSON, nothing else | unit | `go test ./cmd/...` | Wave 0 |
| OUT-04 | four codes, auth-wins precedence | unit | `go test ./cmd/...` | Wave 0 |
| OUT-05 | panic → `{"error":{...}}` + exit 3 | unit | `go test ./cmd/...` | Wave 0 |
| — | `items`/`failures` are `[]` not `null` | unit | `go test ./internal/ado/...` | Wave 0 |

### Wave 0 Gaps
- [ ] `cmd/adoBizInt/main_test.go` — no test file exists for `cmd/` at all
- [ ] `internal/ado/filters_test.go` — for the promoted predicates
- [ ] Behavior-equivalence harness for `buildRows()` across 3-5 view packages (no test files exist in `internal/ui/*` for row building)
- [ ] A `run() int`-shaped entry point so exit codes are testable without spawning a process

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A1 | `slog.Default()` writes to stderr | §5 | OUT-03 violated on the `logging.Init`-failure path. One-line test settles it. |
| A2 | stdlib `flag` with `ContinueOnError` writes errors to its output (default stderr), not stdout | §2 | OUT-03 violated on flag-error path. Call `fs.SetOutput(os.Stderr)` explicitly regardless. |
| A3 | `os.Exit` does not run deferred functions | §8 | The entire panic handler is dead code. Treat as hard constraint. |
| A4 | Go panic output includes function arguments (D-24's premise) | §8 | D-24's security rationale weakens, but the decision to exclude the stack is still correct on other grounds. |

## Open Questions (RESOLVED)

All four were closed during planning. Resolutions and their landing sites:

1. **Should `--help` work without a valid PAT?** `cfg.Validate()` at `main.go:37` aborts when
   `ADO_PAT` is unset. Today that is fine (TUI needs it). A CLI where `--help` fails for lack of
   credentials is hostile. Recommendation: parse args and handle `--help`/unknown-subcommand
   *before* `config.Load`.
   **RESOLVED — yes.** Plan 03-02 Task 2 handles `--help`, `-h` and the bare-noun case before
   `config.Load` and `cfg.Validate` in `run()`, with an acceptance criterion asserting `--help`
   succeeds with `ADO_PAT` unset and no config file present.
2. **Does `GetBranches` already filter by repository internally, and with what matching rule?**
   The promoted by-repo predicate must not double-filter with different semantics.
   `branches.go:52` takes the slice; its body was not read this session.
   **RESOLVED — yes, and the rule is byte-identical to `prs.go`'s.** `branches.go:92` does
   lowercase exact-match set membership, skipped when the set is empty. Recorded in plan 03-03
   `<source_corrections>`; one promoted predicate serves both. Plan 03-04 Task 2 consequently has
   `branches list` pass `--repo` into `GetBranches`' `repositories` argument rather than
   post-filtering.
3. **Is `hideTasksFilter` promoted or explicitly deferred?** Not addressed by D-17. Must be
   decided, not skipped.
   **RESOLVED — promoted.** Plan 03-03 `<source_corrections>` records the planner decision: it is
   the same shape as hide-done, costs one function, and leaving it inline would keep workitems
   filtering in two places, which is what CLI-06 forbids.
4. **What is the uniform dispatch signature given `GetBranches`' extra parameter?**
   **RESOLVED — there is none, by design.** Plan 03-01 D-P2 makes each dispatch entry its own
   closure, so `GetBranches`' third parameter is absorbed at the call site with no adapter and no
   uniform method type.

## Sources

### Primary (HIGH confidence)
- `cmd/adoBizInt/main.go` (whole file, 69 lines) — read this session
- `internal/ado/result.go` (whole file, 187 lines) — read this session
- `internal/ui/fetchctx/fetchctx.go` (whole file, 33 lines) — read this session
- `internal/logging/logging.go:1-48` — read this session
- `go.mod` (whole file) — read this session
- `internal/ui/{workitems,prs,branches,releases,pipelines}` `buildRows()` regions — read this session
- `internal/config/config.go` (grepped regions :41-48, :84-102, :137-159)
- Repo-wide greps: stdout writers, `os.Exit`, `flag`/`os.Args`, `signal`, `recover`, `buildRows`, `func (c *Client) Get`, `wc -l internal/ado/*.go`

### Tertiary (LOW confidence)
- Go stdlib semantics for `slog.Default()`, `flag.ContinueOnError` output, `os.Exit` vs defer, and panic argument printing — training knowledge, logged as A1-A4

## Metadata

**Confidence breakdown:**
- Codebase facts: HIGH — every claim quoted verbatim from a file opened this session
- CLI library choice: HIGH — `go.mod` read directly; D-19 confirmed
- Filter inventory: HIGH — all five `buildRows()` read; three CONTEXT.md errors found and corrected
- Go idiom claims (§8, A1-A4): LOW — training knowledge, flagged

**Research date:** 2026-09-08
**Valid until:** 30 days (stable codebase, no fast-moving external deps)
