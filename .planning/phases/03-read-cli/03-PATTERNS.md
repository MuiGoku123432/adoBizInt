# Phase 03: Read CLI - Pattern Map

**Mapped:** 2026-09-08
**Files analyzed:** 13 (4 new, 9 modified)
**Analogs found:** 9 / 13

## Precedence Note: RESEARCH.md Overrides CONTEXT.md

Per the mapping brief, RESEARCH.md's verified claims win where they conflict with
CONTEXT.md. Confirmed independently this session by reading source:

| CONTEXT.md claim | Verified reality | Source read this session |
|---|---|---|
| D-08: `os.Exit(1)` at `main.go:27, :38, :46` | **`:27, :42, :49`** (plus a fourth at `:66`) | `cmd/adoBizInt/main.go:27,42,49,66` |
| D-17/D-16: search filter in `prs` + `branches`; rewire touches 3 packages | **Five packages** have a search filter: `workitems`, `prs`, `branches`, `releases`, `pipelines` | all five `buildRows()` read below |
| D-17: three workitems filters | **Four** — `hideTasksFilter` (`item.Type == "Task"`) is unnamed by D-17 | `workitems.go:628` |
| D-15: `internal/ado` is 2,015 lines / 7 files, `workitems.go` 551 | 2,517 lines / 9 files, `workitems.go` 591 | RESEARCH.md §4d `wc -l` |
| `classifyFailure` cancellation branch at `result.go:135` | `:135` is the func decl; the `errors.Is(err, context.Canceled)` is at **`:143`** | RESEARCH.md §6 |

**Newly resolved this session (RESEARCH.md Open Question 2):** `GetBranches` *does*
filter by repository internally, and its rule is byte-identical to `prs.go`'s —
lowercase exact-match set membership, skipped entirely when the set is empty. See
the "by-repo" shared pattern below. The promoted predicate can therefore be one
function used by both without a semantics split.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/adoBizInt/main.go` (modify) | entrypoint | request-response | *itself* (current 69-line shape) | self |
| `cmd/adoBizInt/dispatch.go` (new) | router | request-response | — | **no analog** |
| `cmd/adoBizInt/render.go` (new) | formatter | transform | — | **no analog** |
| `cmd/adoBizInt/exit.go` (new) | mapper | transform | `ado.classifyFailure` (`result.go:135-145`) | partial |
| `cmd/adoBizInt/main_test.go` (new) | test | table-driven | `internal/utils/text_test.go` | role-match |
| `internal/ado/filters.go` (new) | domain predicate | transform | `GetBranches` repo-filter block (`branches.go:60-63,92-94`) | role-match |
| `internal/ado/filters_test.go` (new) | test | table-driven | `internal/utils/text_test.go` + `internal/ado/branches_test.go` | exact |
| `internal/ado/result.go` (modify — nil slices) | model | — | *itself* (`newResult` `:106-112`) | self |
| `internal/ui/fetchctx/fetchctx.go` (modify) | utility | — | *itself* (`New` `:28-33`) | self |
| `internal/ui/workitems/workitems.go` (modify) | view | transform | `buildRows()` `:612` | self |
| `internal/ui/prs/prs.go` (modify) | view | transform | `buildRows()` `:497` | self |
| `internal/ui/branches/branches.go` (modify) | view | transform | `buildRows()` `:327` | self |
| `internal/ui/releases/releases.go` (modify) | view | transform | `buildRows()` `:532` | self |
| `internal/ui/pipelines/pipelines.go` (modify) | view | transform | `buildRows()` `:505` | self |

## Pattern Assignments

### `cmd/adoBizInt/main.go` (entrypoint, request-response) — MODIFY

**Analog:** itself. This is the pattern being replaced; the excerpt below is what a
plan step diffs against.

**Current imports** (`main.go:3-13`):
```go
import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/ui"
)
```
Convention: stdlib block, blank line, third-party block, blank line, `sentinovo.ai/bizInt/internal/...` block. New files must match.

**The error-then-exit pattern, repeated four times verbatim** (`main.go:23-28`, the template):
```go
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load config", "error", err)
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
```
Three fixed elements per site: structured `log.Error` with `"error", err` kv pair,
`fmt.Fprintf(os.Stderr, ...)` human message, then exit. **Every one of the four
`os.Exit` sites must become `return <code>` out of a `run() int`** — RESEARCH.md
Pitfall 1. Changing only the integer leaves the D-23 panic handler dead code.

**All four exit sites, verbatim** (`main.go:27, 42, 49, 66`):
```go
27: 		os.Exit(1)   // config.Load failure       -> return 3 (D-08)
42: 		os.Exit(1)   // cfg.Validate failure      -> return 3 (D-08)
49: 		os.Exit(1)   // ado.NewClient failure     -> return 3 (D-08)
66: 		os.Exit(1)   // p.Run() failure, TUI only -> return 1 (CLI-01: unchanged)
```

**Logging init pattern that must survive on the CLI path** (`main.go:16-21`, D-24 depends on it):
```go
	// Initialize logging first
	if err := logging.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize logging: %v\n", err)
	}
	log := logging.Logger()
	log.Info("Starting adoBizInt")
```
Note this is non-fatal — a failed init warns and continues. The CLI path must keep
that, or D-24's `log_path` reference in the crash record dangles.

**The dispatch insertion point** (`main.go:54-60`):
```go
	// Load priorities for work item ordering
	priorities, err := config.LoadPriorities()
	if err != nil {
		log.Warn("Failed to load priorities, using empty list", "error", err)
	}

	model := ui.NewModel(client, cfg.OrgURL, cfg.Projects, ...)
	p := tea.NewProgram(model, tea.WithAltScreen())
```
The CLI branch lands between `:57` and `:59`. `ui.NewModel` eagerly builds six
sub-models; branching after it pays full TUI construction for a `--json` read.

---

### `cmd/adoBizInt/exit.go` (mapper, transform) — NEW

**Analog:** `internal/ado/result.go:135-145` — `classifyFailure`. Partial match: same
shape (switch/branch over a value producing a small enum), different direction.

**Ordering-comment pattern to mirror** (`result.go:140-145`):
```go
	// Cancellation is checked before the timeout branch and deliberately falls
	// through to FailureOther. Phase 3's CLI-04 interrupt handling depends on a
	// user-initiated cancel not being reported as a server timeout.
	if errors.Is(err, context.Canceled) {
		return FailureOther
	}
```
This is the house style for a precedence rule: the *reason* for the ordering is a
comment above the branch, naming the downstream requirement. D-07's "auth wins"
needs exactly this treatment — the reason (auth is non-transient and needs a human)
belongs above the scan, not in a commit message.

**Mapping shape** (from `FailureKind` constants at `result.go:25-36`, no new vocabulary):
```
any Failure.Kind == FailureAuth  -> 2
else !result.Complete            -> 1
else                             -> 0
panic / SIGINT / usage / pre-read -> 3
```

---

### `internal/ado/filters.go` (domain predicate, transform) — NEW

**Analog:** `internal/ado/branches.go:60-63` + `:92-94` — the only existing
in-package filter predicate, and the semantics the promoted by-repo function must match.

**Set-build + membership pattern** (`branches.go:60-63`, then `:92-94`):
```go
	validRepos := make(map[string]bool, len(repositories))
	for _, r := range repositories {
		validRepos[strings.ToLower(r)] = true
	}
	...
			if len(validRepos) > 0 && !validRepos[strings.ToLower(*repo.Name)] {
				continue
			}
```
`prs.go:508-511` and `:520-522` are the same code with `make(map[string]bool)` (no
capacity hint) and `item.RepositoryName` instead of `*repo.Name`. **Same rule, two
copies.** The promoted predicate replaces both.

Note the empty-set semantics: `len(set) > 0 &&` means an empty config slice matches
everything, not nothing. The promoted function must preserve this or the TUI silently
empties for users with no `repositories:` configured.

**Doc-comment style for a decision-bearing function** (`fetchctx.go:20-27`):
```go
// defaultBudget bounds a fetch when no budget is configured. An unconfigured
// budget must not mean an unbounded one — that is the failure this package
// exists to remove.
const defaultBudget = 60 * time.Second

// New returns a context carrying the fan-out budget and its cancel func. The
// caller must defer the cancel inside the command closure, not at construction
// time, or the deadline fires before the request is made.
```
Pattern: state what the thing does, then the failure it prevents. Promoted filters
carry a D-number reference the same way `branches.go:70` cites `(D-12)`.

---

### `internal/ui/fetchctx/fetchctx.go` (utility) — MODIFY

**Analog:** itself, `:28-33`:
```go
func New(budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultBudget
	}
	return context.WithTimeout(context.Background(), budget)
}
```
`context.Background()` is hard-wired. The package doc (`:6-9`) states the invariant
this protects: *"no view file mentions context.Background() on a read path at all,
so the absence of an unbounded read context is visible by inspection."*

**The change that preserves the invariant** — add a parent-taking variant and make
`New` delegate:
```go
func New(budget time.Duration) (context.Context, context.CancelFunc) {
	return NewWithParent(context.Background(), budget)
}

func NewWithParent(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultBudget
	}
	return context.WithTimeout(parent, budget)
}
```
The zero-budget default stays in one place. A CLI-local `context.WithTimeout` (option
b in RESEARCH §7) duplicates it and reintroduces the unbounded-read failure the
package exists to remove.

---

### View packages — `buildRows()` rewire (view, transform) — MODIFY ×5

Every one of the five opens identically:
```go
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
```
[`workitems.go:614`, `prs.go:499`, `branches.go:329`, `releases.go:534`, `pipelines.go:507`]

#### `workitems.go:612-640` — the four filters (verbatim)
```go
func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	// First, filter items
	var filteredItems []ado.WorkItem
	for _, item := range m.items {
		// Apply "Assigned to Me" filter (compare emails, case-insensitive)
		if m.assignedToMeFilter && m.currentUserEmail != "" && !strings.EqualFold(item.AssignedToEmail, m.currentUserEmail) {
			continue
		}
		// Apply "Hide Done/Resolved" filter
		if m.hideDoneFilter && doneStates[item.State] {
			continue
		}
		// Apply "Hide Tasks" filter
		if m.hideTasksFilter && item.Type == "Task" {
			continue
		}
		// Apply search filter (ID or Title)
		if searchQuery != "" {
			idStr := fmt.Sprintf("%d", item.ID)
			titleLower := strings.ToLower(item.Title)
			if !strings.Contains(idStr, searchQuery) && !strings.Contains(titleLower, searchQuery) {
				continue
			}
		}
		filteredItems = append(filteredItems, item)
	}
```
Three things a plan must not miss:
1. `hideTasksFilter` at `:628` — the filter D-17 never names.
2. assigned-to-me is guarded by `m.currentUserEmail != ""` — an empty identity cache
   makes the filter a no-op, not a match-nothing. This is the defined behavior the
   `--assigned-to-me` discretion item asks for; it already exists, copy it.
3. `EqualFold` on email, not `ToLower` + `==`.

**The priority-sort hazard** (`workitems.go:642-664`) — sits *between* the filter loop
and the row loop, gated on the same flag being extracted:
```go
	// Sort by priority when "Assigned to Me" filter is active
	if m.assignedToMeFilter && m.priorities != nil {
		var prioritized []ado.WorkItem
		var unprioritized []ado.WorkItem
		for _, item := range filteredItems {
			if m.priorities.Contains(item.ID) {
				prioritized = append(prioritized, item)
			} else {
				unprioritized = append(unprioritized, item)
			}
		}
		sort.Slice(prioritized, func(i, j int) bool { ... })
		filteredItems = append(prioritized, unprioritized...)
	}
```
D-17 leaves the sort in place, but the extraction must keep `m.assignedToMeFilter`
readable at this point and must not change `filteredItems`' order going in. The
behavior-equivalence test for this file asserts **row order**, not row set.

#### `prs.go:497-536` — by-project, by-repo, 4-field search
```go
	validProjects := make(map[string]bool)
	for _, p := range m.projects {
		validProjects[strings.ToLower(p)] = true
	}
	validRepos := make(map[string]bool)
	for _, r := range m.repositories {
		validRepos[strings.ToLower(r)] = true
	}

	for _, item := range m.items {
		if len(validProjects) > 0 && !validProjects[strings.ToLower(item.Project)] {
			continue
		}
		if len(validRepos) > 0 && !validRepos[strings.ToLower(item.RepositoryName)] {
			continue
		}
		if searchQuery != "" {
			idStr := fmt.Sprintf("%d", item.ID)
			titleLower := strings.ToLower(item.Title)
			repoLower := strings.ToLower(item.RepositoryName)
			authorLower := strings.ToLower(item.CreatedBy)
			if !strings.Contains(idStr, searchQuery) &&
				!strings.Contains(titleLower, searchQuery) &&
				!strings.Contains(repoLower, searchQuery) &&
				!strings.Contains(authorLower, searchQuery) {
				continue
			}
		}
```
The set-building is rebuilt on **every** `buildRows()` call. Promotion is an
opportunity to hoist it, but hoisting changes allocation timing only, not behavior.

#### `branches.go:327-339` — 3-field search
```go
	for i, item := range m.items {
		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(item.Name), searchQuery) &&
				!strings.Contains(strings.ToLower(item.RepositoryName), searchQuery) &&
				!strings.Contains(strings.ToLower(item.LastCommitBy), searchQuery) {
				continue
			}
		}
```
Note: inlines `strings.ToLower` inside `Contains` rather than hoisting to locals like
`prs`/`releases`/`pipelines`. Cosmetic, but a mechanical extraction will normalize it.
Also note `for i, item :=` — this loop uses the index downstream; the others do not.

#### `releases.go:532-545` — 2-field search
```go
	for _, rel := range m.releases {
		if searchQuery != "" {
			releaseLower := strings.ToLower(rel.Name)
			defLower := strings.ToLower(rel.DefinitionName)
			if !strings.Contains(releaseLower, searchQuery) &&
				!strings.Contains(defLower, searchQuery) {
				continue
			}
		}
```
Field is `m.releases`, not `m.items`.

#### `pipelines.go:505-522` — 4-field search
```go
	for _, run := range m.runs {
		if searchQuery != "" {
			pipelineLower := strings.ToLower(run.PipelineName)
			branchLower := strings.ToLower(run.SourceBranch)
			requestedByLower := strings.ToLower(run.RequestedBy)
			buildNumLower := strings.ToLower(run.BuildNumber)
			if !strings.Contains(pipelineLower, searchQuery) &&
				!strings.Contains(branchLower, searchQuery) &&
				!strings.Contains(requestedByLower, searchQuery) &&
				!strings.Contains(buildNumLower, searchQuery) {
				continue
			}
		}
```
Field is `m.runs`.

**Shape for the promoted search matcher** (RESEARCH Pitfall 5): five different field
sets means one typed predicate per payload type, or a shared
`matchesAny(query string, fields ...string) bool` that each view calls with its own
field list. The latter is closer to what the code already does and costs one function.
Note `workitems` and `prs` both include a `fmt.Sprintf("%d", item.ID)` string — the
ID field is numeric, not a string, so a naive `...string` variadic needs the caller
to format it.

---

### `internal/config/config.go` — `--timeout` validation trap

**`Validate()` verbatim** (`config.go:150-163`):
```go
func (c *Config) Validate() error {
	if c.PAT == "" {
		return &ConfigError{Field: "pat", Message: "Personal Access Token is required"}
	}
	if c.OrgURL == "" {
		return &ConfigError{Field: "org_url", Message: "Organization URL is required"}
	}
	// A fan-out budget at or below the poll interval cancels itself exactly as the
	// next tick fires, producing a view that fails permanently and looks like an outage.
	if c.FetchTimeout <= c.PollInterval {
		return &ConfigError{Field: "fetch_timeout", Message: "must be greater than poll_interval"}
	}
	return nil
}
```
Two traps confirmed in one function:
- `c.PAT == ""` aborts before any dispatch. `adoBizInt --help` must be handled
  **before** `config.Load`/`Validate` or help fails for lack of credentials
  (RESEARCH Open Question 1).
- `c.FetchTimeout <= c.PollInterval` (default `PollInterval` 30s) rejects
  `--timeout 10s` with a message about a TUI refresh concept. Apply the override
  after `Validate()`, or exempt the CLI path.

**Error type to reuse for CLI config errors** (`config.go:165-172`):
```go
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
```

---

### `cmd/adoBizInt/main_test.go` and `internal/ado/filters_test.go` (test, table-driven) — NEW

**Analog A — table-driven shape:** `internal/utils/text_test.go` (whole file, 30 lines):
```go
package utils

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"trimmed with ellipsis", "hello world", 8, "hello..."},
		// Regression: truncating by byte split multi-byte characters in half.
		{"accented characters", "héllo wörld", 8, "héllo..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.in, tt.maxLen); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
			}
		})
	}
}
```
Conventions: anonymous struct slice named `tests`, first field `name`, `t.Run(tt.name, ...)`,
`t.Errorf` with `%q` verbs naming inputs and both got/want. Regression cases carry a
`// Regression:` comment stating the original bug.

**Analog B — the injection seam for anything needing a `*Client`:** `internal/ado/branches_test.go:24-51`:
```go
// Regression: a project whose repository listing failed used to be skipped
// silently, so a caller could not tell a failed read from a project with no
// branches. The failure is now named in the envelope and the result is marked
// incomplete.
func TestGetBranchesReportsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-repositories-fail"
		survivingProject = "project-repositories-succeed"
	)

	client := &Client{
		projects: []string{failedProject, survivingProject},
		gitClient: &fakeGitAPI{
			t: t,
			GetRepositoriesFunc: func(_ context.Context, args git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
				...
			},
		},
	}

	result, err := client.GetBranches(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GetBranches() error = %v, want nil", err)
	}
```
Struct-literal injection with per-call `...Func` closures on the fake. Used at 20+
sites across `internal/ado/*_test.go`. **This seam is package-internal** — a
`cmd/adoBizInt` test cannot construct `&ado.Client{...}` because the fields are
unexported. Any CLI test that needs to drive a failure path must either live in
`package ado`, or the CLI must dispatch through an interface the test can satisfy.
**This is a real design constraint on the dispatch table and the planner should
state the answer explicitly.**

**Analog C — exact-key-set assertion, the guard on the JSON contract:**
`internal/ado/json_test.go:18-37`:
```go
// keysOf marshals v and returns the top-level object's key names, sorted.
func keysOf(t *testing.T, v any) []string {
	t.Helper()

	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal %T into map: %v (raw: %s)", v, err, raw)
	}
	...
}
```
File header states the rule: *"Each subtest asserts an EXACT key set rather than the
presence of individual keys: an exact-set assertion is what catches a *missing* tag."*
The `items`/`failures` nil-vs-`[]` test belongs alongside these, in this file's style.

## Shared Patterns

### Structured logging
**Source:** `cmd/adoBizInt/main.go:20, 25, 38, 47, 56`
**Apply to:** every new `cmd/` file
```go
	log := logging.Logger()
	log.Error("Failed to load config", "error", err)
	log.Warn("Failed to load priorities, using empty list", "error", err)
	log.Info("ADO client created successfully")
```
`slog` kv pairs, message is a capitalized sentence fragment, error always keyed
`"error"`. Never `%v` into the message.

### Diagnostics to stderr, never stdout
**Source:** `cmd/adoBizInt/main.go:18, 26, 39-41, 48, 65`
**Apply to:** all CLI error paths (D-13, OUT-03)
```go
	fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
	fmt.Fprintln(os.Stderr, "\nPlease set ADO_PAT and ADO_ORG_URL environment variables")
```
Verified: **zero** `fmt.Print` / `os.Stdout` / `println` calls exist in non-test code.
OUT-03 is currently held by accident. The multi-line remediation hint at `:40-41` is
the existing style for actionable errors — the three-level `--help` usage text should
match its tone.

### Empty-set-means-match-all
**Source:** `internal/ado/branches.go:92` and `internal/ui/prs/prs.go:515,520`
**Apply to:** promoted by-project and by-repo predicates
```go
	if len(validRepos) > 0 && !validRepos[strings.ToLower(*repo.Name)] {
		continue
	}
```
Both existing copies agree. An empty configured slice disables the filter rather
than matching nothing. Preserving this is a CLI-01 requirement, and it also gives
D-18's "CLI defaults to no filtering" a free implementation: pass an empty slice.

### Decision-referencing comments
**Source:** `internal/ado/branches.go:69-70, 79-80, 109-110`
**Apply to:** every promoted filter and every exit-code branch
```go
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		...
			// The loop still visits every configured project: D-08 rejected
			// short-circuiting on auth failure explicitly.
			...
				// Distinguish by status code (D-06): a not-found response is an
				// ordinary empty repository, not a broken read.
```
House style: the comment states the behavior *and* the rejected alternative, with a
D-number. D-07's auth-wins and D-18's no-default-filtering both need this treatment.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `cmd/adoBizInt/dispatch.go` | router | request-response | Zero argument handling exists. No `flag` import, no `os.Args` anywhere in 30 Go files. Use RESEARCH.md §2's stdlib `flag` guidance. |
| `cmd/adoBizInt/render.go` | formatter | transform | The only existing renderer is `bubble-table` + lipgloss in `internal/ui`, which the responsibility map explicitly forbids reusing (pulls in bubbletea; ANSI in redirected stdout). Plain text is new code. |
| panic recovery / `recover()` | — | — | `grep -rn "recover()"` returns nothing repo-wide, including tests. Entirely new. |
| signal handling | — | — | `grep -rn "os/signal\|signal.Notify"` returns nothing repo-wide. Entirely new; use `signal.NotifyContext`. |

For all four, RESEARCH.md §2, §5, §8 and the "Don't Hand-Roll" table are the
substitute for a codebase analog.

## Cross-Cutting Risks Surfaced by Pattern Mapping

1. **The Phase 1 fake seam is package-private.** `&Client{gitClient: fake}` only
   compiles inside `package ado`. Every CLI test that needs a failure path either
   lives in `package ado` or requires the dispatch layer to accept an interface.
   Neither CONTEXT.md nor RESEARCH.md states which. **The planner must decide this
   before writing 03-01, because it determines the dispatch table's type.**
2. **`GetBranches`' repo filter and `prs.go`'s repo filter are already identical** —
   RESEARCH Open Question 2 is answered, and the promoted predicate is a
   three-way deduplication, not a semantics reconciliation. Cheaper than feared.
3. **`branches.go:331` uses `for i, item :=`** while the other four use `for _, x :=`.
   A mechanical five-file search extraction will trip on this one.
4. **Five `buildRows()` bodies, five different receiver field names** (`m.items`,
   `m.items`, `m.items`, `m.releases`, `m.runs`). No uniform loop to template.

## Metadata

**Analog search scope:** `cmd/adoBizInt/`, `internal/ado/`, `internal/ui/{workitems,prs,branches,releases,pipelines,fetchctx}/`, `internal/config/`, `internal/utils/`, `internal/logging/`
**Files read this session:** 12 (4 whole, 8 targeted ranges)
**Pattern extraction date:** 2026-09-08
