# Phase 2: Honest reads - Pattern Map

**Mapped:** 2026-09-07
**Files analyzed:** 20 (3 new, 17 modified)
**Analogs found:** 18 / 20

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/ado/result.go` **(NEW)** | model + utility | transform | `internal/ado/dashboard.go` (type + method in one domain file); `internal/ado/interfaces.go` (small non-domain support file) | partial — no existing shared-type file |
| `internal/ado/result_test.go` **(NEW)** | test | transform | `internal/ado/branches_test.go` | role-match |
| `internal/ado/pullrequests.go` | service | request-response | *self* (`GetPullRequests` :35-66 is the simplest fan-out; use it as the tracer) | exact |
| `internal/ado/workitems.go` | service | request-response | `internal/ado/pullrequests.go` `GetPullRequests` | exact |
| `internal/ado/pipelines.go` | service | request-response | `internal/ado/pullrequests.go` `GetPullRequests` | exact |
| `internal/ado/releases.go` | service | request-response | `internal/ado/pullrequests.go` `GetPullRequests` | exact |
| `internal/ado/branches.go` | service | request-response (nested fan-out) | `internal/ado/pullrequests.go` outer loop; no analog for the nested per-repo swallow | role-match |
| `internal/ado/dashboard.go` | service | request-response (helper-level) | *none* — `getProjectCounts` is the only in-place multi-swallow helper | **no analog** |
| `internal/ado/client.go` | config/wiring | — | *self* — `NewClient` :38-43 | exact |
| `internal/config/config.go` | config | — | `PollInterval` at `config.go:46` + `:82` | exact |
| `internal/config/config_test.go` **(NEW)** | test | — | *none* — `internal/config` has zero test files | **no analog** |
| `internal/ado/pullrequests_test.go` | test | — | *self* `TestGetPullRequestsSwallowsProjectFailure` :253-293 | exact |
| `internal/ado/workitems_test.go` | test | — | `pullrequests_test.go:253-293` | exact |
| `internal/ado/pipelines_test.go` | test | — | `pullrequests_test.go:253-293` | exact |
| `internal/ado/releases_test.go` | test | — | `pullrequests_test.go:253-293` | exact |
| `internal/ado/branches_test.go` | test | — | *self* :14-59 | exact |
| `internal/ado/dashboard_test.go` | test | — | *self* :20-90 | exact |
| `internal/ui/prs/prs.go` | component (view) | event-driven | *self* — msg :33-36, cmd :126-133, reducer :154-162, view :380-383 | exact |
| `internal/ui/pipelines/pipelines.go` | component (view) | event-driven | `internal/ui/prs/prs.go` | exact |
| `internal/ui/releases/releases.go` | component (view) | event-driven | `internal/ui/prs/prs.go` | exact |
| `internal/ui/branches/branches.go` | component (view) | event-driven | `internal/ui/prs/prs.go` | exact |
| `internal/ui/workitems/workitems.go` | component (view) | event-driven | `internal/ui/prs/prs.go` | exact |
| `internal/ui/dashboard/dashboard.go` | component (view) | event-driven | `internal/ui/prs/prs.go` | exact |
| `internal/ui/styles/styles.go` | config (style vocabulary) | — | *self* — `var (...)` block :5-69 | exact |
| `internal/ui/{prs,pipelines,releases,branches,workitems}/*_test.go` **(5 NEW)** | test | — | `internal/ui/releases/releases_test.go` | exact |

---

## Pattern Assignments

### `internal/ado/pullrequests.go` — the tracer fan-out (service, request-response)

**Analog:** itself. This is the simplest of the eight and the reference shape for
`workitems.go`, `pipelines.go` (×2), `releases.go` (×2), and the outer loop of
`branches.go`. Invert this one first; the other five follow mechanically.

**Current swallow shape to invert** (`pullrequests.go:43-65`):

```go
	var allPRs []PullRequest

	for _, project := range targetProjects {
		prs, err := c.gitClient.GetPullRequestsByProject(ctx, git.GetPullRequestsByProjectArgs{
			Project: &project,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				Status: &git.PullRequestStatusValues.Active,
			},
		})
		if err != nil {
			log.Error("Failed to get pull requests", "project", project, "error", err)
			continue
		}

		if prs != nil {
			for _, pr := range *prs {
				allPRs = append(allPRs, c.convertPR(pr, project))
			}
		}
	}

	log.Info("Pull requests fetched", "total_count", len(allPRs))
	return allPRs, nil
```

**Imports pattern** (`pullrequests.go:3-13`) — three groups, module-local last:

```go
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"

	"sentinovo.ai/bizInt/internal/logging"
)
```

**Log-then-handle pattern to preserve** (`pullrequests.go:36`, `:53`, `:64`):

- `log := logging.Logger()` is the first statement of the method body.
- The failure log stays where it is; the `Failure` entry carries the *same*
  `project` + `error` pair already being logged (CONTEXT §Established Patterns —
  "should carry the same information already being logged").
- The terminal `log.Info("... fetched", "total_count", len(...))` line exists in
  every one of the eight; extend it with a failure count rather than adding a
  second log statement.

**What changes:** `continue` becomes "record a `Failure`, then `continue`", the
`var allPRs []PullRequest` accumulator gains a `var failures []Failure` sibling,
and `return allPRs, nil` becomes a `Result[PullRequest]` construction. The
`continue` itself stays — D-08 forbids short-circuiting.

---

### `internal/ado/dashboard.go` — the structural outlier (service, helper-level)

**Analog:** none. `getProjectCounts` is the only helper in the package that
swallows three failures in place and returns `nil` error unconditionally.

**The three in-place swallows** (`dashboard.go:100-128`) — note each is an
`if err != nil { log } else if <nil-guard> { assign }` chain, so removing the
`else` changes the assignment guard too:

```go
	workItems, err := c.workitemClient.QueryByWiql(ctx, query)
	if err != nil {
		log.Error("Failed to query work items", "project", project, "error", err)
	} else if workItems != nil && workItems.WorkItems != nil {
		counts.WorkItems = len(*workItems.WorkItems)
		log.Info("Dashboard work item count", "project", project, "count", counts.WorkItems)
	}

	// ... PR count ...
	if err != nil {
		log.Error("Failed to get pull requests", "project", project, "error", err)
	} else if prs != nil {
		counts.PRs = len(*prs)
	}

	// ... build count, inside `if len(c.pipelines) > 0` ...
		if err != nil {
			log.Error("Failed to get builds", "project", project, "error", err)
		} else if builds != nil && builds.Value != nil {
```

**The unconditional success return** (`dashboard.go:141`):

```go
	return counts, nil
```

**The dead outer `continue`** (`dashboard.go:48-52`) — unreachable today:

```go
		projectCounts, err := c.getProjectCounts(ctx, project)
		if err != nil {
			log.Error("Failed to get counts for project", "project", project, "error", err)
			continue // Skip projects with errors, aggregate what we can
		}
```

**D-03 constraint the analog does not cover:** `getProjectCounts` must return
*both* the partial `counts` and its failures. The outer loop must keep summing
`projectCounts.WorkItems/.PRs/.Pipelines` (`dashboard.go:59-61`) even when
failures came back, or "sum of the projects that answered" degrades to "sum of
the projects that answered perfectly". There is no existing code in the repo that
does this — the plan must specify the signature explicitly.

**A fourth swallow the CONTEXT inventory omits:** `GetRecentIterations` at
`dashboard.go:80-82` is logged at `Warn` and discarded with an explicit
"using date filter only" fallback comment. This is a *degradation*, not a
failure — it changes the WIQL filter and therefore the count. The plan should
decide consciously whether it becomes a `Failure` (making the count honestly
incomplete) or stays a silent fallback.

---

### `internal/ado/result.go` (NEW — model + utility, transform)

**Analog:** partial. Two files set the precedent for a small non-domain support
file in `internal/ado`:

`internal/ado/interfaces.go` (47 lines) — the Phase 1 precedent for a
support-only file, and the header comment style the new file should match:

```go
// These package-local interfaces make destructive write methods testable without
// crossing the Azure DevOps REST boundary (SEAM-01). Each interface declares only
// methods this package calls; adding an uncalled method violates ROADMAP SC4.
```

`internal/ado/dashboard.go:14-19` — the doc-comment + flat-struct style all
`ado` types use:

```go
// DashboardCounts holds summary counts for the dashboard
type DashboardCounts struct {
	WorkItems int
	PRs       int
	Pipelines int
}
```

**Struct field comment pattern for the enumerated `Kind` field**
(`releases.go:19` style, cited in CONVENTIONS §Comments):

```go
	Status           string // "draft", "active", "abandoned"
```

Apply the same to `FailureKind`: annotate the legal value set inline
(`"auth"`, `"timeout"`, `"rate_limit"`, `"other"` — D-07).

**No analog exists for:** generics (zero in the codebase), `errors.As`
(zero sites — CONVENTIONS gap table #13 confirms `errors.Is`/`errors.As` are
never used). The classifier is genuinely new code; use the two-target structure
from RESEARCH §Pattern 1 verbatim in shape.

---

### `internal/config/config.go` — two new duration keys (config)

**Analog:** `PollInterval`, the exact precedent D-11 names.

**Struct field pattern** (`config.go:38-49`) — note aligned `yaml:` tags in
snake_case with an optional trailing comment:

```go
type Config struct {
	PAT                string                       `yaml:"pat"`
	OrgURL             string                       `yaml:"org_url"`
	Projects           StringSlice                  `yaml:"projects"`
	Project            string                       `yaml:"project"`             // Backwards compat: single project
	Pipelines          StringSlice                  `yaml:"pipelines"`           // Optional: specific pipeline names to monitor
	ReleaseDefinitions StringSlice                  `yaml:"release_definitions"` // Optional: separate release definition names (falls back to pipelines)
	Repositories       StringSlice                  `yaml:"repositories"`        // Optional: specific repository names to filter PRs
	PollInterval       time.Duration                `yaml:"poll_interval"`
	StateTransitions   map[string]map[string]string `yaml:"state_transitions"`
	UserEmail          string                       `yaml:"user_email"` // User's email for "Assigned to Me" filter
}
```

**Default-application pattern** (`config.go:80-90`) — defaults go in the struct
literal *before* `loadFromFile()`, which is why an absent key keeps the default:

```go
func Load() (*Config, error) {
	cfg := &Config{
		PollInterval: 30 * time.Second,
	}

	if err := cfg.loadFromFile(); err != nil {
		// File not found is ok, we'll use env vars
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
```

**Add both keys to this literal.** Pitfall 5 in RESEARCH: omitting one produces a
zero `time.Duration`, which means *no timeout* for `http.Client` and *instant
expiry* for `context.WithTimeout` — two opposite catastrophic behaviours from one
missing line.

**Validation pattern** (`config.go:144-152`) — if the plan adds the
`FetchTimeout > PollInterval` check RESEARCH recommends, it goes here and uses
the existing `ConfigError`:

```go
func (c *Config) Validate() error {
	if c.PAT == "" {
		return &ConfigError{Field: "pat", Message: "Personal Access Token is required"}
	}
	if c.OrgURL == "" {
		return &ConfigError{Field: "org_url", Message: "Organization URL is required"}
	}
	return nil
}
```

**No env-var override.** `loadFromEnv` (`config.go:122-142`) handles exactly four
names and `PollInterval` is deliberately absent from it. Match that — do not add
`ADO_REQUEST_TIMEOUT`.

---

### `internal/ado/client.go` — connection timeout (config/wiring)

**Analog:** itself. The insertion point is between :39 and :41.

```go
func NewClient(cfg *config.Config) (*Client, error) {
	connection := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)

	ctx := context.Background()

	workitemClient, err := workitemtracking.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}
```

`connection.Timeout` is `*time.Duration`, so this needs an addressable local —
the codebase has two competing idioms for that (CONVENTIONS gap #14): the
`strPtr` helper at `client.go:234` and taking the address of a local at
`releases.go:117-119`. For a one-off `time.Duration` use the local-address form;
`strPtr` is string-only.

**Constraint the analog cannot show:** the five `NewClient` calls at `:43`, `:48`,
`:53`, `:58`, `:63` and `location.NewClient` at `:70` each read
`connection.Timeout` once and cache the result. Setting it after any of them is a
silent no-op (RESEARCH Pattern 3, Pitfall 2).

---

### `internal/ui/prs/prs.go` — the reducer/message analog for all six views

**Analog:** itself. `prs` is the cleanest full example of the message → command →
reducer → view chain and is the shape to copy into `pipelines`, `releases`,
`branches`, `workitems`, and `dashboard`.

**Message type** (`prs.go:32-36`) — D-16 replaces `Items []ado.PullRequest` with
the envelope:

```go
// PRsMsg is sent when pull requests are fetched
type PRsMsg struct {
	Items []ado.PullRequest
	Err   error
}
```

**Fetch command** (`prs.go:126-133`) — fields captured into locals so the closure
never captures the receiver. This is where D-09's `context.WithTimeout` goes;
`cfg.FetchTimeout` must be captured into a local alongside `projects`/`client`:

```go
func (m Model) fetchPRs() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		items, err := client.GetPullRequests(context.Background(), projects)
		return PRsMsg{Items: items, Err: err}
	}
}
```

**Reducer** (`prs.go:154-162`) — D-13/D-14/D-15 all land in this switch arm:

```go
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

Note D-15's "keep last good rows" is **already the behavior** — `m.items` is
untouched on the error branch. What is missing is the stale *marker*, not the
retention. Size the task accordingly.

**Error banner render** (`prs.go:379-383`) — D-14's blocking auth banner replaces
the table, so it must gate the `if/else if/else` block that follows at :387:

```go
	// Show error if any
	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
	}

	// Show loading or table. Once rows exist a refresh keeps them on screen and
	// only annotates the count line, rather than blanking the table.
	if m.loading && len(m.items) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("Loading pull requests..."))
	} else if len(m.items) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("No open pull requests found"))
	} else {
```

The `else if len(m.items) == 0` arm at :389 is the exact "empty vs failed"
confusion this phase removes — it must become conditional on completeness.
`pipelines.go:232-233` has the same bug in a worse form (a *status-line* message
naming config as the cause).

**Existing status-line downgrade pattern** for secondary fetches — keep this for
`DefinitionsMsg` / `ApprovalsMsg` per RESEARCH's recommendation that only the
primary message drives the auth banner:

```go
	case DefinitionsMsg:
		if msg.Err != nil {
			m.statusMsg = "Warning: Could not load pipeline definitions - trigger may not work"
			m.statusErr = true
```
— `internal/ui/pipelines/pipelines.go:238-241`

---

### `internal/ado/*_test.go` — inverting the eight characterization tests

**Analog:** `internal/ado/branches_test.go` (59 lines, the tightest) and
`internal/ado/pullrequests_test.go:253-293`.

**Full characterization-test shape** (`branches_test.go:12-59`) — note the
doc comment that names the phase, the two-project `failedProject` /
`survivingProject` const pair, the *poisoned* value on the failing project so a
leak is detectable, and the struct-literal injection:

```go
// TestGetBranchesSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
func TestGetBranchesSwallowsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-repositories-fail"
		survivingProject = "project-repositories-succeed"
	)

	client := &Client{
		projects: []string{failedProject, survivingProject},
		gitClient: &fakeGitAPI{
			t: t,
			GetRepositoriesFunc: func(_ context.Context, args git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
				// ...
				if args.Project != nil && *args.Project == failedProject {
					return &repositories, errors.New("repositories failed")
				}
				return &repositories, nil
			},
		},
	}

	branches, err := client.GetBranches(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GetBranches() error = %v, want nil", err)
	}
	if len(branches) != 1 {
		t.Fatalf("GetBranches() returned %d branches, want 1 surviving branch: %+v", len(branches), branches)
	}
	if branches[0].Project != survivingProject || branches[0].Name != "main" {
		t.Errorf("GetBranches()[0] = %+v, want project %q and branch main", branches[0], survivingProject)
	}
}
```

**Assertion message convention:** `t.Errorf("Method() field = %v, want %v", got, want)`
— the callable-name-with-parens form is used in all eight.

**Fake construction** (`fakes_test.go:20-36`) — every fake carries `t *testing.T`,
declares one `XxxFunc` field per interface method, and `t.Fatalf`s on an
unstubbed call. New failure-path tests only need to set the one `Func` they
exercise:

```go
type fakeWorkItemAPI struct {
	t *testing.T

	UpdateWorkItemFunc        func(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	// ...
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}
```

**Compile-time drift assertions** (`fakes_test.go:13-18`) — keep these:

```go
var (
	_ workItemAPI = (*fakeWorkItemAPI)(nil)
	_ gitAPI      = (*fakeGitAPI)(nil)
	_ buildAPI    = (*fakeBuildAPI)(nil)
	_ releaseAPI  = (*fakeReleaseAPI)(nil)
)
```

**Multi-fake wiring for dashboard** (`dashboard_test.go:71-76`) — the only test
that injects three fakes at once, including a bare `&fakeBuildAPI{t: t}` that
asserts `GetBuilds` is never called:

```go
	client := &Client{
		projects:       []string{failingProject, survivingProject},
		workitemClient: api,
		gitClient:      gitAPI,
		buildClient:    &fakeBuildAPI{t: t},
	}
```

**Package constraint:** `Client` fields are all unexported (`client.go:20-36`), so
every test file is `package ado`, never `package ado_test`.

---

### `internal/ui/{prs,pipelines,releases,branches,workitems}/*_test.go` (5 NEW — test)

**Analog:** `internal/ui/releases/releases_test.go` — the only view package with a
test, and the only working example of constructing a view `Model` without a live
`*ado.Client`.

**Nil-client test helper** (`releases_test.go:9-16`) — the load-bearing insight is
the comment: reducer and row-building paths never dereference the client:

```go
// newTestModel builds a Model backed by the given releases. The client is never
// dereferenced when building rows or resolving the selection, so nil is fine.
func newTestModel(releases []ado.Release) Model {
	m := New(nil, "https://dev.azure.com/org", nil, 0)
	m.releases = releases
	m.table = m.table.WithRows(m.buildRows())
	return m
}
```

**Regression-test comment style** (`releases_test.go:18-20`, `:39-41`) — a
`// Regression:` prefix stating the *old broken behavior*:

```go
// Regression: the selection used to be resolved by matching the truncated
// display name, so releases sharing a prefix all resolved to the first one.
func TestGetSelectedReleaseDistinguishesSharedNamePrefix(t *testing.T) {
```

**Applying this to the reducer tests:** each new view test calls
`m.Update(XxxMsg{...})` with an envelope carrying (a) an `auth` failure, (b) a
partial result, (c) a failure after a prior good load — then asserts on the
`View()` string or the model fields. The `New(nil, ...)` construction pattern
carries over directly; only the constructor arity differs per package
(`prs.New` takes 5 args, `releases.New` takes 4).

**Note:** this analog covers `Model` construction and assertion style only. There
is no existing example of driving `Update()` in a test — that part is new.

---

## Shared Patterns

### Import grouping
**Source:** `internal/ado/pullrequests.go:3-13`, `internal/ado/dashboard.go:3-12`
**Apply to:** every file touched
Three blank-line-separated groups: stdlib, third-party, module-local. `gofmt -l .`
must stay empty (CONVENTIONS §Toolchain Baseline).

### Logger retrieval
**Source:** `internal/ado/pullrequests.go:36`, `internal/ado/dashboard.go:24`, `:75`
**Apply to:** every `internal/ado` method touched

```go
	log := logging.Logger()
```

First statement of the function body, never stored on a struct. Structured
key-value pairs only — `log.Error("Failed to get pull requests", "project", project, "error", err)`.

### Receiver naming
**Source:** CONVENTIONS §Receiver naming (28 `(c *Client)` sites, 58 `(m Model)`)
**Apply to:** all new methods
`(c *Client)` in `internal/ado`. `(m Model)` value receiver for the Bubble Tea
trio and read-only helpers; `(m *Model)` only when mutating.

### Style vocabulary for the new warning/stale/banner rendering
**Source:** `internal/ui/styles/styles.go:5-69`
**Apply to:** all six views if the shared-helper route is chosen

Single package-level `var (...)` block, grouped by comment header. `WarningStyle`
and `ErrorStyle` already exist and cover D-13 and D-14 respectively:

```go
	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)
```

A stale marker needs no new color — `MutedStyle` (`:46-47`) is the existing
vocabulary for de-emphasized text.

### Rule-of-three justification
**Source:** CONVENTIONS §Simplicity, gap table rows #6–#8
**Apply to:** the envelope (D-01) and the failure-rendering helpers

The gap table already records three unextracted rule-of-three violations
(`IsTextInputFocused` ×6, `buildRows` ×5, style selectors ×4). The plan should
state that the envelope at 8 call sites and the rendering helpers at 6 views both
clear the bar, so extraction here is compliance rather than speculative
generality — and cite rule 5's Phase 1 carve-out in the same words.

### Commit convention
**Source:** CONVENTIONS §Commits
**Apply to:** every commit in this phase

`<type>(<scope>): <description>` — scope is `ado`, `config`, `ui`, or the view
package name. Imperative, lowercase, no trailing period, ≤72 chars. No
`Co-Authored-By`. Branch is `dev`, never `master`/`main`.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/ado/dashboard.go` (`getProjectCounts` inversion) | service | request-response | No existing helper returns *both* a partial value and its failures. D-03's "keep the counts you collected while reporting what failed" has no precedent in the repo. RESEARCH Pitfall 4 confirms this is the one read a mechanical edit will silently miss. |
| `internal/config/config_test.go` | test | — | `internal/config` has zero test files. Nearest style reference is `internal/ado/branches_test.go`, but it is a different package with a different construction seam. `Load()` also reads `$HOME/.config/adoBizInt/config.yaml` unconditionally (`config.go:107-119`) with no injection point — a defaults test must either use `t.Setenv("HOME", t.TempDir())` or the plan must accept that no config test is written. |
| `internal/ado/result.go` classifier | utility | transform | `errors.As` and `errors.Is` have **zero** uses in the codebase (CONVENTIONS §Error handling, gap row #13). Generics likewise have zero uses. Use RESEARCH §Pattern 1 and §Pattern 2 as the source instead of a repo analog. |
| Bubble Tea `Update()` under test | test | event-driven | `releases_test.go` exercises `buildRows`/`getSelectedRelease` only. No existing test drives a reducer with a message. |

---

## Planner Notes

Three things the analogs actively contradict or under-describe:

1. **D-15 is smaller than it reads.** `prs.go:154-162` already retains `m.items`
   on error — the rows persist today. Only the stale *marker* is missing.
2. **The `else if len(m.items) == 0` arm** (`prs.go:389`, and worse at
   `pipelines.go:232-233`) is live "empty means no work" messaging in five views.
   It must be gated on completeness or the phase's stated goal is only half met.
3. **`GetRecentIterations` at `dashboard.go:80-82`** is a fourth dashboard swallow
   not listed in the CONTEXT inventory. It silently changes the WIQL filter, so it
   changes the count. Decide consciously whether it becomes a `Failure`.

## Metadata

**Analog search scope:** `internal/ado`, `internal/ui/*`, `internal/config`, `internal/ui/styles`
**Files scanned:** 12 read in full or in targeted ranges; 31 sized via `wc -l`
**Pattern extraction date:** 2026-09-07
