# Coding Conventions

**Analysis Date:** 2026-09-03

Scope: `sentinovo.ai/bizInt`, Go 1.25.4, 30 Go files, 7,601 lines (7,519 non-test).

This document has two halves:

1. **Observed conventions** -- what the code actually does today, verified by reading source.
2. **Target standards** -- the rules the project is adopting going forward, plus a gap table showing where current code violates them.

Where the two disagree, the target standard wins for new code. Existing code is not rewritten wholesale; it is corrected opportunistically when a task already touches the file.

---

## Toolchain Baseline

Verified by running each command at repo root on 2026-09-03.

| Command | Result | Notes |
|---------|--------|-------|
| `go build ./...` | exit 0 | Clean |
| `go vet ./...` | exit 0, no output | Clean, no findings |
| `gofmt -l .` | exit 0, no output | Zero formatting drift across all 30 files |
| `go test ./... -cover` | 2 packages with tests, 14 without | See `TESTING.md` |

No linter beyond `go vet` is configured. Verified absent at repo root:

| Tool | File checked | Present |
|------|--------------|---------|
| golangci-lint | `.golangci.yml` / `.yaml` / `.toml` | No |
| Make | `Makefile` / `makefile` | No |
| Task | `Taskfile.yml` | No |
| just | `justfile` | No |
| EditorConfig | `.editorconfig` | No |
| pre-commit | `.pre-commit-config.yaml` | No |
| GitHub Actions | `.github/` | No |
| GitLab CI | `.gitlab-ci.yml` | No |
| Azure Pipelines | `azure-pipelines.yml` | No |

The only dotfiles at repo root are `.gitignore` (2 lines, ignores the compiled binary) and `.planning/`. There is no CI of any kind. `gofmt`, `go vet`, and `go build` cleanliness is currently maintained by hand, not enforced.

---

## Observed Conventions

### Package layout and naming

- Package name always matches its directory name (`internal/ui/releases` -> `package releases`).
- All packages live under `internal/` except the single binary entry point `cmd/adoBizInt/main.go`.
- Package names are lowercase, single word, no underscores: `ado`, `config`, `keymap`, `logging`, `link`, `modal`, `styles`, `utils`.
- One UI view per package under `internal/ui/<view>/`, with the primary file named after the package (`internal/ui/releases/releases.go`).
- Modal sub-components live beside their parent view in the same package, named for their role: `internal/ui/releases/approval_modal.go`, `internal/ui/pipelines/trigger_modal.go`, `internal/ui/workitems/effort.go`, `internal/ui/workitems/detail.go`.
- The `internal/ado` package is split one file per Azure DevOps domain: `client.go`, `workitems.go`, `pipelines.go`, `releases.go`, `pullrequests.go`, `branches.go`, `dashboard.go`.

### Import grouping

Three blank-line-separated groups, in this order, consistently across every file checked (`internal/ado/client.go:3`, `internal/ado/releases.go:3`, `internal/ui/app.go:3`, `internal/ui/releases/releases.go:3`, `cmd/adoBizInt/main.go:3`):

```go
import (
	"context"                                              // 1. stdlib
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops" // 2. third-party

	"sentinovo.ai/bizInt/internal/config"                  // 3. module-local
)
```

The bubbletea import is always aliased `tea` (`internal/ui/app.go:8`, `cmd/adoBizInt/main.go:7`). No other aliasing except `overlay` at `internal/ui/modal/modal.go:7`.

### Receiver naming

| Receiver | Count | Where |
|----------|-------|-------|
| `(m Model)` | 58 | All bubbletea view models, value receiver |
| `(c *Client)` | 28 | `internal/ado`, always pointer |
| `(m *Model)` | 19 | View methods that mutate state |
| `(p *Priorities)` | 9 | `internal/config/priorities.go` |
| `(m DetailModel)` | 8 | `internal/ui/workitems/detail.go` |
| `(m TriggerModal)` / `(m EffortModal)` / `(m ApprovalModal)` | 4 / 4 / 3 | Modal sub-models |
| `(c *Config)` / `(e *ConfigError)` / `(s *StringSlice)` | 3 / 1 / 1 | `internal/config/config.go` |
| `(v view)` / `(k KeyMap)` | 3 / 2 | `internal/ui/modal/modal.go`, `internal/keymap/keymap.go` |

Rule in force: single lowercase letter derived from the type name. Bubbletea `Model` deliberately mixes value and pointer receivers -- value for the `tea.Model` interface trio (`Init`, `Update`, `View`) and read-only accessors, pointer for internal mutators such as `copySelectedBranchLink` (`internal/ui/branches/branches.go:317`).

### Bubbletea view contract

Every view package implements the same shape. This is the strongest structural convention in the codebase and new views must match it.

| Element | Signature | Example |
|---------|-----------|---------|
| Constructor | `func New(client *ado.Client, orgURL string, ...) Model` | `internal/ui/releases/releases.go:89` |
| Init | `func (m Model) Init() tea.Cmd` | `internal/ui/releases/releases.go:124` |
| Update | `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)` | `internal/ui/releases/releases.go:175` |
| View | `func (m Model) View() string` | `internal/ui/releases/releases.go:389` |
| Row builder | `func (m Model) buildRows() []table.Row` | 5 views: `releases.go:462`, `prs.go:425`, `pipelines.go:432`, `workitems.go:564`, `branches.go:271` |
| Focus probe | `func (m Model) IsTextInputFocused() bool` | 6 views, see gap table |

Note the root `internal/ui/app.go:75` returns `(tea.Model, tea.Cmd)` because it is the program root; sub-views return their concrete `Model` so the parent can assign without a type assertion.

### Message types

Async results flow back as struct types suffixed `Msg`, declared at the top of the file that owns them, always carrying an `Err error` field when the operation can fail.

```go
// ReleasesMsg is sent when releases are fetched
type ReleasesMsg struct {
	Releases []ado.Release
	Err      error
}
```
-- `internal/ui/releases/releases.go:36`

27 `Msg` types exist. Naming is `<Noun>Msg` for data arrival (`ReleasesMsg`, `PRsMsg`, `CountsMsg`), `<Verb>ResultMsg` for action completion (`ApprovalResultMsg`, `TriggerResultMsg`), `Close<Thing>Msg` for modal dismissal (`CloseApprovalMsg`, `CloseEffortMsg`, `CloseDetailMsg`), and bare `TickMsg struct{}` for the poll timer.

Commands capture the fields they need into locals before returning the closure, so the closure never captures the receiver:

```go
func (m Model) fetchReleases() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		releases, err := client.GetReleases(context.Background(), projects)
		return ReleasesMsg{Releases: releases, Err: err}
	}
}
```
-- `internal/ui/releases/releases.go:141`

### Error handling

| Pattern | Prevalence | Evidence |
|---------|-----------|----------|
| Bare `return err` | Dominant | `internal/ado/client.go:41`, `:46`, `:51`, `:56`, `:61`; `internal/ado/releases.go:289`, `:314` |
| `fmt.Errorf` with `%w` wrap | 3 sites only | `internal/ado/pullrequests.go:157`, `:200`; `internal/ui/workitems/workitems.go:293` |
| `fmt.Errorf` without wrap | ~26 sites | `internal/utils/links.go:59`, `internal/ui/branches/branches.go:320` |
| `errors.New` | 0 sites | Never used |
| `errors.Is` / `errors.As` | 0 sites | Never used |
| Custom error type | 1 | `ConfigError` at `internal/config/config.go:154`, with `Error()` at `:159` |
| `panic` / `log.Fatal` | 0 sites | Verified absent |

Errors surface in three ways, by layer:

1. `internal/ado` logs at `Error`/`Warn` level then returns the error to the caller (`internal/ado/releases.go:288`).
2. Loop-scoped failures inside `internal/ado` are logged and `continue`d so one bad project does not abort the whole fetch (`internal/ado/releases.go:72`, `:219`).
3. UI layer stores the error on the model (`err error` field) or in `statusMsg` + `statusErr bool`, and renders it via `styles.ErrorStyle` (`internal/ui/app.go:180`).

`cmd/adoBizInt/main.go` is the only place that terminates: `log.Error(...)` then `fmt.Fprintf(os.Stderr, ...)` then `os.Exit(1)` (`main.go:25-27`, `:38-42`, `:47-49`, `:64-66`).

Deliberately ignored errors are rare and use the blank identifier: `start, _ = time.Parse(time.RFC3339, s)` at `internal/ado/client.go:173` and `:180`, and `home, _ := os.UserHomeDir()` at `internal/logging/logging.go:47`.

### Logging

`log/slog` from stdlib, wrapped in a package-level singleton.

- Initialized once in `main` before anything else (`cmd/adoBizInt/main.go:17`).
- Writes to `~/.config/adoBizInt/adoBizInt.log`, dir `0755`, file `0644`, level `Debug` (`internal/logging/logging.go:18-31`).
- Retrieved per-function, not stored on structs: `log := logging.Logger()` as the first statement of the function body. Verified at `internal/ado/client.go:122`, `internal/ado/releases.go:57`, `:202`, `:273`, `:298`.
- Falls back to `slog.Default()` when uninitialized rather than nil-panicking (`internal/logging/logging.go:38-41`).
- Always structured key-value pairs, never `Printf`-style: `log.Error("Failed to approve release", "approvalID", approvalID, "error", err)` (`internal/ado/releases.go:288`).
- Key naming is inconsistent: `camelCase` (`"approvalID"`, `"displayName"`) at `internal/ado/releases.go:288` and `internal/ado/client.go:77`, `snake_case` (`"total_count"`, `"configured_definitions"`, `"org_url"`) at `internal/ado/releases.go:196`, `:267` and `cmd/adoBizInt/main.go:32`. No rule is in force.
- Messages are capitalized sentences without trailing periods: `"Releases fetched"`, `"Failed to get approvals"`.
- The PAT is never logged. `cmd/adoBizInt/main.go:30` carries an explicit `// Log config (redact PAT)` comment and logs only `org_url`, `projects`, `projects_count`.

### Comments

Two distinct styles coexist, split by age.

**Older style** -- most of the codebase. Doc comment starts with the identifier name, no trailing period, no package doc:

```go
// GetReleases fetches recent releases for configured pipelines
func (c *Client) GetReleases(ctx context.Context, projects []string) ([]Release, error) {
```
-- `internal/ado/releases.go:55`

**Newer style** -- `internal/ui/link`, `internal/ui/modal`, `internal/config/priorities.go`, and the two test files. Full sentences with trailing periods, plus a package doc comment:

```go
// Package link provides the commands each view uses to copy an Azure DevOps
// URL to the clipboard or open it in a browser, plus the message they report
// back with.
package link
```
-- `internal/ui/link/link.go:1`

Only 2 of 28 non-test files carry a package doc comment: `internal/ui/link/link.go:1` and `internal/ui/modal/modal.go:3`.

Inline comments explain *why*, not *what*, and the newer code is notably better at this:

```go
	// columnKeyReleaseID carries the release ID as hidden row metadata. It has no
	// matching column so it never renders, but stays attached to the row for
	// getSelectedRelease to look up.
	columnKeyReleaseID = "releaseID"
```
-- `internal/ui/releases/releases.go:30`

Struct field comments annotate the legal value set for stringly-typed fields:

```go
	Status           string // "draft", "active", "abandoned"
```
-- `internal/ado/releases.go:19`, same pattern at `:30`, `:31`, `:47`, `:48`

Zero `TODO`, `FIXME`, `HACK`, or `XXX` markers exist anywhere in the codebase. Verified.

Zero `// note:` markers exist. This is the marker the target standard reserves for the two-occurrence duplication case; it is currently unused.

### Struct and constant patterns

- Domain types in `internal/ado` are flat structs of Go-native values with no pointers, converted from the SDK's pointer-heavy types at the boundary (`internal/ado/releases.go:13`, `:27`, `:40`).
- The SDK-to-domain conversion is always an explicit nil-checked field-by-field copy, never reflection or a generic mapper (`internal/ado/releases.go:139-153`, `:232-261`).
- Table column keys are unexported `const` blocks at the top of each view file, prefixed `columnKey` (`internal/ui/releases/releases.go:22-34`).
- View enum uses `iota` with a named type: `type View int` then `DashboardView View = iota` (`internal/ui/app.go:22-31`).
- Styles are package-level `var` in a single `var (...)` block in `internal/ui/styles/styles.go:5-69`, grouped by comment headers (`// Colors`, `// Base styles`, `// Status styles`).
- YAML config uses struct tags with snake_case keys and a trailing comment explaining optionality (`internal/config/config.go:39-49`).

### Interfaces

**Zero interfaces are declared anywhere in this codebase.** Verified with a search for `^type \w+ interface` across all Go files -- no matches.

All dependency wiring is against the concrete `*ado.Client`, whose fields are unexported and hold live Azure DevOps SDK clients (`internal/ado/client.go:20-32`). Consequences for testing are covered in `TESTING.md`.

### Control flow

- Early returns and `continue` are the norm; deep nesting is avoided in the newer files. `internal/ado/releases.go:84-86` guards with `if rd.Name == nil { continue }`.
- Long `switch` over message type or view enum rather than dispatch maps (`internal/ui/app.go:78`, `:148`, `:185`, `:208`).
- One recursive closure exists, declared then assigned so it can call itself: `var collectIterations func(...)` at `internal/ado/client.go:147-148`.

### Security-adjacent conventions

- PAT is read from `ADO_PAT` env var or `~/.config/adoBizInt/config.yaml`, never committed (`internal/config/config.go:123`, `:113`).
- Config file is required to exist only optionally; env vars override file values (`internal/config/config.go:85-92`).
- URL construction always escapes user-controlled path/query segments with `url.QueryEscape` (`internal/utils/links.go:24`, `:30`).
- Directories created `0755`, files `0644` (`internal/logging/logging.go:19`, `:24`; `internal/config/priorities.go:59`).

---

## Target Standards

These are the rules for new and modified code. They are not yet enforced by tooling.

### Simplicity

| Rule | Meaning here |
|------|--------------|
| Simplest code that solves the current task | Optimize for a senior engineer reading the diff, not for the abstract "future maintainer" |
| Single-use code stays inline | If a helper is called from exactly one place, keep it in that file. A private function in the same file is fine for readability. Never move it to `internal/utils` or a new shared package |
| Rule of three | Extract shared code only when the same logic appears in 3 or more places. Two occurrences stay duplicated with a `// note:` comment pointing at the twin |
| No speculative generality | No interfaces, generic type parameters, config options, plugin hooks, or manager/factory/provider layers unless the current task requires them |
| YAGNI | No future-proofing. No unused parameters. No exported symbol without a current external caller |

### Reuse before creation

Before writing any new function, type, view, or util:

1. Query Gortex for an existing equivalent.
2. If one exists, extend or reuse it. Never fork a second implementation.
3. If none exists, prefer a private function in the file that needs it.

**Blocker, verified 2026-09-03:** Gortex reports this repository as **not tracked**. Every graph query returns `repo_not_tracked`. Until `gortex track /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt` is run and the client reconnects, this rule can only be satisfied by manual `rg` search. Tracking the repo is a prerequisite for this standard being operable.

### Structure and size

| Target | Value | Interpretation for Go |
|--------|-------|----------------------|
| Component | under 150 lines | One `.go` file |
| Module | under 300 lines | One package |
| Responsibility | one per file | A file owns one view, one modal, or one ADO domain |

These are split signals, not hard failures. A file crossing 150 lines is a prompt to ask whether it holds more than one responsibility, not an automatic defect.

Follow existing folder conventions. Match neighboring code. Never invent a new directory structure when one already exists.

### Dependencies and layering

- No new module dependency without stated justification and explicit confirmation.
- No new architectural layer (a new `internal/` package that sits between existing ones) without the same.
- `go.mod` currently has 7 direct requires. Additions must be argued.

### Style

- Early returns over nesting.
- Plain functions over method-heavy types.
- Stdlib over clever one-liners and over third-party convenience packages.
- Boring, direct, obvious.

### Commits

Format: `<type>(<scope>): <description>`

| Field | Rule |
|-------|------|
| type | One of `feat`, `fix`, `docs`, `refactor`, `chore`, `style`, `test`, `build`, `ci`, `perf` |
| scope | Package or area, lowercase: `ado`, `ui`, `releases`, `utils`, `config` |
| description | Imperative mood, lowercase first letter, no trailing period |
| subject | 72 characters maximum |
| trailers | Never append `Co-Authored-By` |

Never commit feature work directly to `master` or `main`. Use a branch.

Current branch is `dev` (verified). No `master`/`main` commits are at risk right now, but the rule stands.

---

## Gap Table: Target Standard vs Current Code

| # | Target standard | Current reality | Evidence | Severity |
|---|-----------------|-----------------|----------|----------|
| 1 | Component under 150 lines | 18 of 28 non-test files exceed 150 lines | See size table below | High |
| 2 | Module under 300 lines | 7 of 16 packages exceed 300 lines; `internal/ado` is 2,015 lines | See size table below | High |
| 3 | One responsibility per file | `internal/ui/workitems/workitems.go` at 918 lines holds the view, filtering, sorting, row building, HTML stripping, style selection, and child-task lookup | `internal/ui/workitems/workitems.go:564`, `:755`, `:809`, `:881` | High |
| 4 | Reuse before creating; query Gortex first | Gortex is not tracking this repo; the check is currently impossible to perform | Gortex returns `repo_not_tracked` for this path | High |
| 5 | Never create a second implementation | `min(a, b int)` is hand-rolled, shadowing the Go 1.21+ builtin `min` | `internal/ui/workitems/detail.go:444` | Medium |
| 6 | Rule of three -- extract at 3+ occurrences | `IsTextInputFocused()` is byte-identical (`return m.filterFocused == 1`) in 6 view packages, well past the threshold, and is not extracted | `branches.go:312`, `dashboard.go:325`, `pipelines.go:549`, `prs.go:504`, `releases.go:580`, `workitems.go:827` | Medium |
| 7 | Rule of three -- extract at 3+ occurrences | `buildRows() []table.Row` exists in 5 view packages with the same shape (iterate slice, truncate fields, attach hidden index key) | `releases.go:462`, `prs.go:425`, `pipelines.go:432`, `workitems.go:564`, `branches.go:271` | Medium |
| 8 | Rule of three -- extract at 3+ occurrences | Style-selector functions repeat the same `switch status { ... default: Muted }` shape in 4 places | `pipelines.go:503`, `pipelines.go:520`, `prs.go:490`, `releases.go:516`, `workitems.go:809` | Medium |
| 9 | Two occurrences stay duplicated with a `// note:` comment | Zero `// note:` comments exist; duplication is silent | Verified: no matches for `// note:` | Low |
| 10 | Boring, direct code | `TickMsg struct{}` is declared independently in 3 packages rather than shared or distinguished | `pipelines.go:53`, `prs.go:39`, `releases.go:56` | Low |
| 11 | Boring, direct code | `BranchesMsg` is declared twice with different meanings -- one for the branches view, one for the pipeline trigger modal's branch list | `internal/ui/branches/branches.go:32`, `internal/ui/pipelines/trigger_modal.go:27` | Low |
| 12 | Conventional commits | 3 of 54 commits comply. The other 51 include `stuff`, `more stuff`, `many stuff`, `many thing`, `test 25`, `lots of things`, `hopeful releases fix` | `git log --oneline`; compliant: `3aca6cb`, `4f8d5eb`, `acb4b3f` | Medium (historical, not fixable) |
| 13 | Error wrapping so callers can inspect | Only 3 of ~29 error returns wrap with `%w`; `errors.Is`/`errors.As` never used, so no caller can distinguish an auth failure from a 404 | `pullrequests.go:157`, `:200`; `workitems.go:293` | Medium |
| 14 | Reuse before creating | `strPtr` helper exists in `internal/ado` but SDK arg structs are elsewhere filled by taking the address of a local, so two idioms coexist | `internal/ado/client.go:234` vs `internal/ado/releases.go:117-119` | Low |
| 15 | Consistent conventions across the package | Log attribute keys mix `camelCase` and `snake_case` with no rule | `releases.go:288` (`"approvalID"`) vs `releases.go:196` (`"total_count"`) | Low |
| 16 | Consistent conventions across the package | Doc comment style is split: 2 of 28 files have package docs; trailing-period usage varies by file age | `link.go:1`, `modal.go:3` have package docs; no other file does | Low |
| 17 | No speculative generality | Compliant. Zero interfaces, zero generics, zero factory/provider layers. This standard is already met and should be preserved | Verified: no `type ... interface` matches | None |
| 18 | Never commit to master/main | Compliant. Work is on branch `dev` | `git branch --show-current` | None |

### File size against the 150-line target

18 of 28 non-test files exceed the 150-line component target.

| File | Lines | Over 150 by |
|------|-------|-------------|
| `internal/ui/workitems/workitems.go` | 918 | 768 |
| `internal/ui/releases/releases.go` | 625 | 475 |
| `internal/ui/pipelines/pipelines.go` | 594 | 444 |
| `internal/ado/workitems.go` | 551 | 401 |
| `internal/ui/prs/prs.go` | 524 | 374 |
| `internal/ui/workitems/detail.go` | 449 | 299 |
| `internal/ado/pipelines.go` | 397 | 247 |
| `internal/ui/branches/branches.go` | 350 | 200 |
| `internal/ui/dashboard/dashboard.go` | 327 | 177 |
| `internal/ado/releases.go` | 319 | 169 |
| `internal/ui/workitems/effort.go` | 302 | 152 |
| `internal/ui/pipelines/trigger_modal.go` | 301 | 151 |
| `internal/ado/pullrequests.go` | 243 | 93 |
| `internal/ado/client.go` | 236 | 86 |
| `internal/ui/app.go` | 224 | 74 |
| `internal/config/config.go` | 161 | 11 |
| `internal/config/priorities.go` | 158 | 8 |
| `internal/ui/releases/approval_modal.go` | 152 | 2 |

Within target (10 files): `internal/ado/dashboard.go` 142, `internal/ado/branches.go` 127, `internal/keymap/keymap.go` 88, `internal/ui/styles/styles.go` 69, `cmd/adoBizInt/main.go` 69, `internal/utils/links.go` 62, `internal/logging/logging.go` 49, `internal/ui/link/link.go` 39, `internal/ui/modal/modal.go` 24, `internal/utils/text.go` 19.

### Package size against the 300-line target

7 of 16 packages exceed the 300-line module target. Test lines excluded.

| Package | Lines | Files | Over 300 by |
|---------|-------|-------|-------------|
| `internal/ado` | 2,015 | 7 | 1,715 |
| `internal/ui/workitems` | 1,669 | 3 | 1,369 |
| `internal/ui/pipelines` | 895 | 2 | 595 |
| `internal/ui/releases` | 777 | 2 | 477 |
| `internal/ui/prs` | 524 | 1 | 224 |
| `internal/ui/branches` | 350 | 1 | 50 |
| `internal/config` | 319 | 2 | 19 |
| `internal/ui/dashboard` | 327 | 1 | 27 |

Within target: `internal/ui` 224, `internal/keymap` 88, `internal/utils` 81, `internal/ui/styles` 69, `cmd/adoBizInt` 69, `internal/logging` 49, `internal/ui/link` 39, `internal/ui/modal` 24.

Note the two smallest packages (`link`, `modal`) are also the two newest and the only two with package doc comments. They are the reference for what the target standard looks like applied.

---

## Applying These Rules

When a task touches a file:

1. Match the surrounding style in that file, not the style of a different file.
2. If adding a view, copy the contract from `internal/ui/branches/branches.go` (350 lines, closest to target of the full views) rather than `internal/ui/workitems/workitems.go`.
3. If adding an ADO call, put it in the existing domain file (`releases.go`, `pipelines.go`, etc.) and follow the `log := logging.Logger()` / nil-checked conversion / log-and-return pattern.
4. Do not extract a shared helper on the second occurrence. Duplicate it and leave `// note: same shape as <file>:<line>`.
5. Do not add an interface to make something testable unless the current task is specifically to make that thing testable.
6. Do not add a dependency without saying why and getting confirmation.

---

*Convention analysis: 2026-09-03*
