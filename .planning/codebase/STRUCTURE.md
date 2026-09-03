# Codebase Structure

**Analysis Date:** 2026-09-03

Module `sentinovo.ai/bizInt`, Go 1.25.4. 30 `.go` files, 7,601 lines, 16 packages. No `vendor/` directory.

## Directory Layout

```text
adoBizInt/
├── go.mod                       # module + 7 direct deps
├── go.sum
├── cmd/
│   └── adoBizInt/
│       └── main.go              # sole entry point (69)
├── internal/
│   ├── ado/                     # ADO service layer, zero TUI imports (2,015)
│   │   ├── client.go            # connection, sub-clients, accessors (236)
│   │   ├── workitems.go         # work items + effort + attachments (551)
│   │   ├── pipelines.go         # builds, definitions, queueing (397)
│   │   ├── releases.go          # releases + approvals (319)
│   │   ├── pullrequests.go      # PRs, approve, complete (243)
│   │   ├── dashboard.go         # aggregate counts (142)
│   │   └── branches.go          # repo branches (127)
│   ├── config/                  # YAML + env config (319)
│   │   ├── config.go            # Config, Load, Validate (161)
│   │   └── priorities.go        # persisted work item ordering (158)
│   ├── keymap/
│   │   └── keymap.go            # global key bindings (88)
│   ├── logging/
│   │   └── logging.go           # file-backed slog singleton (49)
│   ├── ui/                      # presentation layer (4,657)
│   │   ├── app.go               # root tea.Model, view router (224)
│   │   ├── dashboard/
│   │   │   └── dashboard.go     # counts + project filter (327)
│   │   ├── workitems/
│   │   │   ├── workitems.go     # table, filters, priorities (918)
│   │   │   ├── detail.go        # detail modal (449)
│   │   │   └── effort.go        # effort-entry modal (302)
│   │   ├── prs/
│   │   │   └── prs.go           # PR table, approve, merge (524)
│   │   ├── pipelines/
│   │   │   ├── pipelines.go     # run table, polling (594)
│   │   │   └── trigger_modal.go # definition + branch picker (301)
│   │   ├── releases/
│   │   │   ├── releases.go      # release table, polling (625)
│   │   │   ├── approval_modal.go# approve/reject modal (152)
│   │   │   └── releases_test.go # selection regressions (52)
│   │   ├── branches/
│   │   │   └── branches.go      # branch table, PR-create link (350)
│   │   ├── link/
│   │   │   └── link.go          # clipboard/browser tea.Cmds (39)
│   │   ├── modal/
│   │   │   └── modal.go         # overlay compositor (24)
│   │   └── styles/
│   │       └── styles.go        # lipgloss palette (69)
│   └── utils/                   # TUI-free helpers (111)
│       ├── links.go             # ADO URL builders, clipboard, browser (62)
│       ├── text.go              # rune-safe Truncate (19)
│       └── text_test.go         # Truncate table tests (30)
└── .planning/
    └── codebase/                # GSD brownfield analysis output
```

## Package Inventory

| Package | Import path | Files | Lines | TUI deps |
|---------|-------------|-------|-------|----------|
| `main` | `sentinovo.ai/bizInt/cmd/adoBizInt` | 1 | 69 | bubbletea |
| `ado` | `.../internal/ado` | 7 | 2,015 | none |
| `config` | `.../internal/config` | 2 | 319 | none |
| `keymap` | `.../internal/keymap` | 1 | 88 | bubbles/key |
| `logging` | `.../internal/logging` | 1 | 49 | none |
| `ui` | `.../internal/ui` | 1 | 224 | bubbletea, bubbles |
| `dashboard` | `.../internal/ui/dashboard` | 1 | 327 | bubbletea, bubbles, lipgloss |
| `workitems` | `.../internal/ui/workitems` | 3 | 1,669 | bubbletea, bubbles, lipgloss, bubble-table |
| `prs` | `.../internal/ui/prs` | 1 | 524 | bubbletea, bubbles, lipgloss, bubble-table |
| `pipelines` | `.../internal/ui/pipelines` | 2 | 895 | bubbletea, bubbles, lipgloss, bubble-table |
| `releases` | `.../internal/ui/releases` | 3 | 829 | bubbletea, bubbles, lipgloss, bubble-table |
| `branches` | `.../internal/ui/branches` | 1 | 350 | bubbletea, bubbles, lipgloss, bubble-table |
| `link` | `.../internal/ui/link` | 1 | 39 | bubbletea |
| `modal` | `.../internal/ui/modal` | 1 | 24 | bubbletea, bubbletea-overlay |
| `styles` | `.../internal/ui/styles` | 1 | 69 | lipgloss |
| `utils` | `.../internal/utils` | 3 | 111 | none |

Non-TUI packages total 2,494 lines (33% of the codebase): `ado`, `config`, `logging`, `utils`.

## Directory Purposes

**`cmd/adoBizInt/`:**
- Purpose: process bootstrap
- Contains: exactly one file, one `main()`
- Key files: `cmd/adoBizInt/main.go`
- The only place `os.Exit` is called (4 sites: `:27`, `:42`, `:49`, `:66`)

**`internal/ado/`:**
- Purpose: every Azure DevOps read and write
- Contains: one `Client` struct plus 12 exported data types, split by ADO domain
- Key files: `internal/ado/client.go` (construction and accessors), one file per domain
- Depends only on `internal/config` and `internal/logging` plus the ADO SDK

**`internal/config/`:**
- Purpose: settings and user-controlled ordering
- Contains: `Config` (YAML + env) and `Priorities` (its own YAML file with dirty-flag save)
- Key files: `internal/config/config.go:38` (Config), `internal/config/priorities.go:13` (Priorities)
- Both read from `~/.config/adoBizInt/`

**`internal/ui/`:**
- Purpose: the root Bubbletea model and view routing
- Contains: one file. Sub-view packages are children, not siblings.
- Key files: `internal/ui/app.go`

**`internal/ui/<view>/` (6 packages):**
- Purpose: one self-contained Elm triple per Azure DevOps domain
- Contains: a `Model`, its message types, and optionally modal files
- Naming: main file matches the package name; modals are separate files (`detail.go`, `effort.go`, `trigger_modal.go`, `approval_modal.go`)

**`internal/ui/{link,modal,styles}/`:**
- Purpose: shared presentation utilities, imported by view packages but never importing them
- All three are small (39, 24, 69 lines) and single-responsibility

**`internal/utils/`:**
- Purpose: helpers that carry no TUI dependency
- Contains: ADO URL construction, clipboard and browser shell-outs, rune-safe truncation

## Import / Dependency Graph

```text
                        cmd/adoBizInt/main.go
                       /       |      |      \
                      /        |      |       \
                     v         v      v        v
                 config     logging  ado    internal/ui
                     ^         ^      |          |
                     |         |      |          | (imports all 6 views
                     |         |      |          |  + keymap + styles
                     |         +------+          |  + ado + config)
                     +----------------+          |
                                                 v
        +----------+----------+----------+-------+-------+
        |          |          |          |       |       |
   dashboard  workitems     prs     pipelines releases branches
        |          |          |          |       |       |
        |          |          |          |       |       |
        +----------+----------+----------+-------+-------+
                     |        |          |
                     v        v          v
                  ui/link  ui/modal  ui/styles
                     |
                     v
                  internal/utils
                     ^
        (also imported directly by 7 view files)
```

**Package-level edges, complete:**

| From | To |
|------|-----|
| `cmd/adoBizInt` | `ado`, `config`, `logging`, `ui` |
| `ui` | `ado`, `config`, `keymap`, `ui/branches`, `ui/dashboard`, `ui/pipelines`, `ui/prs`, `ui/releases`, `ui/styles`, `ui/workitems` |
| `ui/workitems` | `ado`, `config`, `logging`, `ui/link`, `ui/modal`, `ui/styles`, `utils` |
| `ui/releases` | `ado`, `ui/link`, `ui/modal`, `ui/styles`, `utils` |
| `ui/pipelines` | `ado`, `ui/link`, `ui/modal`, `ui/styles`, `utils` |
| `ui/prs` | `ado`, `ui/link`, `ui/styles`, `utils` |
| `ui/branches` | `ado`, `ui/link`, `ui/styles`, `utils` |
| `ui/dashboard` | `ado`, `ui/styles` |
| `ui/link` | `utils` |
| `ui/modal` | (external only) |
| `ui/styles` | (external only) |
| `ado` | `config`, `logging` |
| `config` | (external only: `gopkg.in/yaml.v3`) |
| `keymap` | (external only: `bubbles/key`) |
| `logging` | (stdlib only) |
| `utils` | (external only: `atotto/clipboard`) |

The graph is acyclic. `internal/ado` never imports `internal/ui`.

**Shared-package fan-in:**

| Package | Imported by (files) | Count |
|---------|---------------------|-------|
| `ado` | `main.go`, `app.go`, `dashboard.go`, `workitems.go`, `detail.go`, `effort.go`, `prs.go`, `pipelines.go`, `trigger_modal.go`, `releases.go`, `approval_modal.go`, `branches.go`, `releases_test.go` | 13 |
| `ui/styles` | `app.go`, `dashboard.go`, `workitems.go`, `detail.go`, `effort.go`, `prs.go`, `pipelines.go`, `trigger_modal.go`, `releases.go`, `approval_modal.go`, `branches.go` | 11 |
| `logging` | all 7 `ado/*.go` except `dashboard.go` has it too, plus `main.go`, `detail.go` | 9 |
| `utils` | `workitems.go`, `detail.go`, `effort.go`, `prs.go`, `pipelines.go`, `releases.go`, `branches.go`, `link.go` | 8 |
| `ui/link` | `workitems.go`, `prs.go`, `pipelines.go`, `releases.go`, `branches.go` | 5 |
| `config` | `main.go`, `app.go`, `workitems.go`, `ado/client.go` | 4 |
| `ui/modal` | `workitems.go`, `pipelines.go`, `releases.go` | 3 |
| `keymap` | `app.go` | 1 |

`ui/dashboard` is the only view package that imports neither `ui/link`, `ui/modal`, nor `utils`. It renders counts, not a table.

## File Inventory

### `internal/ado` -- 7 files, 2,015 lines

| File | Lines | Exported types | Exported methods / funcs |
|------|-------|----------------|--------------------------|
| `client.go` | 236 | `Client` | `NewClient`, `Projects`, `Pipelines`, `ReleaseDefinitions`, `CurrentUser`, `CurrentUserEmail`, `GetRecentIterations` |
| `workitems.go` | 551 | `WorkItem`, `WorkItemDetail`, `Attachment` | `GetWorkItems`, `UpdateWorkItemState`, `UpdateWorkItemStateWithEffort`, `GetWorkItemDetail`, `GetChildWorkItems`, `DownloadAttachment`, `GetNextState` |
| `pipelines.go` | 397 | `PipelineRun`, `PipelineDefinition` | `GetPipelineRuns`, `GetPipelineDefinitions`, `GetBranchesForPipeline`, `QueuePipeline` |
| `releases.go` | 319 | `Release`, `ReleaseEnvironment`, `ReleaseApproval` | `GetReleases`, `GetPendingApprovals`, `ApproveRelease`, `RejectRelease` |
| `pullrequests.go` | 243 | `PullRequest` | `GetPullRequests`, `ApprovePullRequest`, `CompletePullRequest` |
| `dashboard.go` | 142 | `DashboardCounts` | `GetDashboardCounts` |
| `branches.go` | 127 | `Branch` | `GetBranches` |

25 exported methods on `*Client` plus the package-level `GetNextState` (`workitems.go:510`) and `NewClient`. Unexported helpers: `getProjectWorkItems` (`workitems.go:86`), `convertPR` (`pullrequests.go:68`), `getProjectCounts` (`dashboard.go:74`), `strPtr` (`client.go:234`).

### `internal/ui` -- 14 files, 4,657 lines

| File | Lines | Role |
|------|-------|------|
| `workitems/workitems.go` | 918 | Largest file. Table, 3 filters, priority sort, 2 modal owners, 8 `priorities.Save()` sites |
| `releases/releases.go` | 625 | Table, dual fetch (releases + approvals), polling, approval modal owner |
| `pipelines/pipelines.go` | 594 | Table, dual fetch (runs + definitions), polling, trigger modal owner |
| `prs/prs.go` | 524 | Table, approve, merge-with-confirmation, polling |
| `workitems/detail.go` | 449 | Detail modal, attachment download, viewport scrolling |
| `branches/branches.go` | 350 | Table, ahead/behind, PR-create link |
| `dashboard/dashboard.go` | 327 | Counts, multi-select project filter with its own `FilterKeyMap` (`:22`) |
| `workitems/effort.go` | 302 | Three-field effort modal, two constructors (Task and User Story variants) |
| `pipelines/trigger_modal.go` | 301 | Definition picker + branch picker |
| `app.go` | 224 | Root model, 6-arm switches at `:148`, `:185`, `:208` |
| `releases/approval_modal.go` | 152 | Approve/reject with comment |
| `styles/styles.go` | 69 | 12 colors, 10 named styles, 5 work-item type styles |
| `releases/releases_test.go` | 52 | 2 regression tests on `getSelectedRelease` |
| `link/link.go` | 39 | `ActionMsg` + `Fail`/`Copy`/`Open` |
| `modal/modal.go` | 24 | `Center(fg, bg string) string` |

### Support packages -- 8 files, 567 lines

| File | Lines | Role |
|------|-------|------|
| `config/config.go` | 161 | `Config`, `StringSlice` custom unmarshal (`:15`), `Load`, `Validate`, `DefaultStateTransitions` (`:53`) |
| `config/priorities.go` | 158 | `Priorities` with `AddToTop`, `Remove`, `MoveUp`, `MoveDown`, `GetPosition`, `Contains`, `Clean`, `Save` |
| `keymap/keymap.go` | 88 | 13 bindings, `ShortHelp`, `FullHelp` |
| `utils/links.go` | 62 | 6 URL builders, `CopyToClipboard`, `OpenInBrowser` |
| `logging/logging.go` | 49 | `Init`, `Logger`, `LogPath` |
| `utils/text_test.go` | 30 | 8-case table test for `Truncate` |
| `utils/text.go` | 19 | `Truncate` |
| `cmd/adoBizInt/main.go` | 69 | Bootstrap |

## Naming Conventions

**Files:**
- One file per ADO domain in `internal/ado`, named for the domain plural: `workitems.go`, `pullrequests.go`, `pipelines.go`, `releases.go`, `branches.go`
- The primary file in a view package matches the package name: `internal/ui/prs/prs.go`
- Modal files are named for what they do plus a `_modal` suffix when the package already has a non-modal file: `trigger_modal.go`, `approval_modal.go`. The two `workitems` modals predate that convention: `detail.go`, `effort.go`
- Tests are `<subject>_test.go` and live beside the subject

**Directories:**
- View packages are lowercase, plural, and match the domain: `workitems`, `prs`, `pipelines`, `releases`, `branches`. `dashboard` is singular because it is not a collection.
- Shared UI helpers are lowercase singular: `link`, `modal`, `styles`

**Types:**
- Every view package names its model exactly `Model` -- collision is avoided by package qualification (`prs.Model`, `workitems.Model`)
- Modal models are named distinctly within their package: `DetailModel`, `EffortModal`, `TriggerModal`, `ApprovalModal`
- Message types end in `Msg`: `PRsMsg`, `StateUpdateMsg`, `CloseDetailMsg`
- Table column keys are unexported `columnKey<Field>` constants (`internal/ui/prs/prs.go:21-30`)

**Constructors:**
- `New(...)` in view packages (`prs.New`, `workitems.New`)
- `NewModel(...)` for the root (`ui.NewModel`)
- `New<Thing>Modal(...)` for modals (`NewEffortModal`, `NewEffortModalForUserStory`)
- `NewClient(cfg)` in `ado`

## Where to Add New Code

### A new Azure DevOps read or write

- Implementation: `internal/ado/<domain>.go` -- append to the existing domain file, or create a new one if the domain is new
- Signature: `func (c *Client) DoThing(ctx context.Context, ...) (Result, error)`
- Return plain structs with no display formatting. Follow `internal/ado/pullrequests.go:16` as the reference shape.
- Log with `logging.Logger()`; do not print
- Do not import anything from `internal/ui`

### A new view (a 7th tab)

Four coordinated edits, no shortcuts available -- the sub-models are concrete types, not an interface:

| Edit | Location |
|------|----------|
| Add `Tab7` binding and add it to `FullHelp` | `internal/keymap/keymap.go:19`, `:86` |
| Add the enum value | `internal/ui/app.go:31` |
| Add the field, the constructor call, the Tab7 switch arm, the Update arm, the View arm, the `IsTextInputFocused` arm | `internal/ui/app.go:45`, `:65`, `:127`, `:172`, `:197`, `:220` |
| Create the package | `internal/ui/<name>/<name>.go` |

Copy `internal/ui/prs/prs.go` as the template. It is the smallest complete view with polling, search, actions, and link handling. Required surface: `New`, `Init`, `Update`, `View`, `IsTextInputFocused`.

### A new modal in an existing view

- Implementation: `internal/ui/<view>/<name>_modal.go`
- Template: `internal/ui/pipelines/trigger_modal.go` (301 lines, has both a picker and an async branch fetch)
- Required: a `Close<Name>Msg` and a `<Name>SubmitMsg` type, plus `Init`/`Update`/`View` returning the concrete modal type
- Parent wiring, in this order at the top of the parent `Update`: close handler, submit handler, delegation guard. See `internal/ui/pipelines/pipelines.go:179-206`.
- Parent `View`: extract the existing body into `renderTableView()` and return `modal.Center(m.modal.View(), m.renderTableView())` when open (`internal/ui/pipelines/pipelines.go:363`)

### A new ADO URL form

- Implementation: `internal/utils/links.go`, alongside the six existing builders
- Keep it a pure `func Build<Thing>URL(orgURL, project, ...) string`
- Use `url.QueryEscape` for anything appearing in a query string (`internal/utils/links.go:24`)

### A new shared string helper

- Implementation: `internal/utils/text.go`
- Must not import lipgloss or bubbletea. `internal/utils` is currently TUI-free and that is load-bearing for reuse.
- Add table tests to `internal/utils/text_test.go`

### A new configuration setting

- Field: `internal/config/config.go:38` on `Config`, with a `yaml:` tag
- Env override, if any: `internal/config/config.go:122` in `loadFromEnv`
- Validation, only if the setting is mandatory: `internal/config/config.go:144`
- Threading: add the parameter to `ui.NewModel` (`internal/ui/app.go:52`) and pass it to the views that need it. There is no config singleton -- everything is passed by value at construction.
- For comma-separated-or-list values, use the `StringSlice` type (`internal/config/config.go:13`)

### A new style or color

- `internal/ui/styles/styles.go`. All colors are declared there; no view package declares its own `lipgloss.Color`.
- Exception in practice: per-status style selection functions live in the view that needs them (`getApprovalStyle` at `internal/ui/prs/prs.go:490`, `getTypeStyle` at `internal/ui/workitems/workitems.go:809`). These select among `styles` colors rather than defining new ones.

### A test

- Location: beside the code, same package (both existing tests are in-package, not `_test` package)
- Existing examples: `internal/utils/text_test.go` (table-driven pure function), `internal/ui/releases/releases_test.go` (constructs a `Model` with a `nil` client, documented as safe at `releases_test.go:9-11`)
- The `nil`-client pattern works for any test that only exercises `buildRows`/selection and never dispatches a command

### A CLI or non-TUI surface

- New binary: `cmd/<name>/main.go`, or a dispatch branch inserted in `cmd/adoBizInt/main.go` before line 59 (`ui.NewModel`)
- Reuse directly: `internal/ado`, `internal/config`, `internal/logging`, `internal/utils/links.go`
- Do not import: any package under `internal/ui/` -- all of them pull in bubbletea
- Filtering and workflow orchestration currently live in view packages and would need promoting into `internal/ado` or a new intermediate package first. See ARCHITECTURE.md for the specific list.

## Special Directories

**`.planning/codebase/`:**
- Purpose: GSD brownfield analysis documents
- Generated: yes, by mapper agents
- Committed: yes

**`~/.config/adoBizInt/` (runtime, outside the repo):**
- `config.yaml` -- read at `internal/config/config.go:113`
- `priorities.yaml` -- read at `internal/config/priorities.go:30`, written at `:63`
- `adoBizInt.log` -- opened append-mode at `internal/logging/logging.go:23`
- Not committed; created on demand by `os.MkdirAll` (`logging.go:19`, `priorities.go:59`)

**No `vendor/`, no `pkg/`, no `test/`, no `docs/`, no `scripts/`.** Verified by directory listing.

---

*Structure analysis: 2026-09-03*
