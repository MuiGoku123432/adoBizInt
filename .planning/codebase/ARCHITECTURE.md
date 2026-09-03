<!-- refreshed: 2026-09-03 -->
# Architecture

**Analysis Date:** 2026-09-03

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                    Process entry (main)                      │
│                 `cmd/adoBizInt/main.go` (69)                 │
│  logging.Init -> config.Load -> config.Validate ->           │
│  ado.NewClient -> config.LoadPriorities -> ui.NewModel ->    │
│  tea.NewProgram(model, tea.WithAltScreen()).Run()            │
└────────────────────────────┬────────────────────────────────┘
                             │ single unconditional path
                             ▼
┌─────────────────────────────────────────────────────────────┐
│              Root Bubbletea Model (Elm loop)                 │
│                  `internal/ui/app.go` (224)                  │
│  View enum + 6 concrete sub-models + keymap + help           │
│  Update: routes tea.Msg to exactly one active sub-model      │
└──┬────────┬────────┬────────┬────────┬────────┬─────────────┘
   │        │        │        │        │        │
   ▼        ▼        ▼        ▼        ▼        ▼
┌──────┐┌────────┐┌──────┐┌─────────┐┌────────┐┌────────┐
│dash  ││workitem││ prs  ││pipelines││releases││branches│
│board ││s (1669)││(524) ││  (895)  ││ (829)  ││ (350)  │
│(327) ││        ││      ││         ││        ││        │
└──┬───┘└───┬────┘└──┬───┘└────┬────┘└───┬────┘└───┬────┘
   │        │        │         │         │         │
   │   shared: ui/styles (69), ui/link (39), ui/modal (24),
   │           internal/utils (81)
   │        │        │         │         │         │
   └────────┴────────┴────┬────┴─────────┴─────────┘
                          │ all calls take context.Context
                          ▼
┌─────────────────────────────────────────────────────────────┐
│         Service layer -- UI-independent, zero TUI imports    │
│                 `internal/ado/` (2,015 lines)                │
│  25 exported methods on *ado.Client. Plain-struct returns.   │
│  client.go workitems.go pullrequests.go pipelines.go         │
│  releases.go branches.go dashboard.go                        │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│  github.com/microsoft/azure-devops-go-api/azuredevops        │
│  5 typed sub-clients: workitemtracking, work, git,           │
│  build, release. Plus location (identity bootstrap).         │
└─────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| `main` | Bootstrap order, fatal-error exit, TUI launch | `cmd/adoBizInt/main.go` |
| `ui.Model` | View switching, window-size fan-out, help bar, message routing | `internal/ui/app.go` |
| `ado.Client` | All Azure DevOps I/O; owns 5 SDK sub-clients and cached identity | `internal/ado/client.go:20` |
| `config.Config` | YAML file + env var merge, validation, default state transitions | `internal/config/config.go:38` |
| `config.Priorities` | Persisted work item ordering (`priorities.yaml`), dirty-flag save | `internal/config/priorities.go:13` |
| `keymap.KeyMap` | Global key bindings and help groupings | `internal/keymap/keymap.go:5` |
| `logging` | Package-level `*slog.Logger` writing to a file | `internal/logging/logging.go:9` |
| `ui/styles` | Lipgloss color and style constants | `internal/ui/styles/styles.go` |
| `ui/link` | `tea.Cmd` factories for clipboard-copy and browser-open | `internal/ui/link/link.go` |
| `ui/modal` | Composite a rendered foreground string over a rendered background | `internal/ui/modal/modal.go:21` |
| `utils` | URL builders, clipboard/browser shell-out, rune-safe `Truncate` | `internal/utils/links.go`, `internal/utils/text.go` |

## Pattern Overview

**Overall:** Elm architecture (Bubbletea) over a layered service client.

**Key Characteristics:**
- Strict one-directional dependency: `cmd` -> `ui` -> `ado` -> ADO SDK. No package in `internal/ado` imports anything from `internal/ui`.
- Sub-models are **concrete types**, not `tea.Model` implementations. Each returns its own `Model` type: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)` (`internal/ui/prs/prs.go:135`, `internal/ui/workitems/workitems.go:222`, and 4 others). Only the root implements `tea.Model` (`internal/ui/app.go:75` returns `tea.Model`).
- All state is value-copied through `Update`. Mutation happens by returning a modified copy, except for pointer-receiver helpers that set `actionPending`/`statusMsg` before returning a command (`internal/ui/prs/prs.go:290`, `internal/ui/prs/prs.go:313`).
- All ADO calls are dispatched off the update loop as `tea.Cmd` closures and delivered back as typed messages.

## Layers

**Entry layer (`cmd/adoBizInt`):**
- Purpose: bootstrap and fatal-error handling
- Location: `cmd/adoBizInt/main.go`
- Contains: 69 lines, one `main()` function, no branching on arguments
- Depends on: `ado`, `config`, `logging`, `ui`
- Used by: nothing

**Presentation layer (`internal/ui/...`):**
- Purpose: rendering, keyboard handling, per-view filtering, async dispatch
- Location: `internal/ui/` (root) plus 6 view packages and 3 shared packages
- Contains: 4,657 lines across 14 files
- Depends on: `ado`, `config`, `keymap`, `logging`, `utils`, Bubbletea/lipgloss/bubble-table
- Used by: `cmd/adoBizInt/main.go:59`

**Service layer (`internal/ado`):**
- Purpose: every Azure DevOps read and write
- Location: `internal/ado/`, 7 files, 2,015 lines
- Contains: `Client` struct, 25 exported methods, 12 exported plain data types
- Depends on: `config`, `logging`, the ADO SDK. Verified zero imports of `bubbletea`, `lipgloss`, `bubbles`, or `bubble-table`.
- Used by: all 6 view packages, both modal files, `cmd/adoBizInt/main.go:45`

**Support layer (`internal/config`, `internal/logging`, `internal/utils`, `internal/keymap`):**
- `config` and `logging` and `utils` have zero TUI imports.
- `keymap` imports `github.com/charmbracelet/bubbles/key` (`internal/keymap/keymap.go:3`) -- it is the only support package with a TUI dependency.

## Data Flow

### Startup path

1. `logging.Init()` opens `~/.config/adoBizInt/adoBizInt.log` (`cmd/adoBizInt/main.go:17`, `internal/logging/logging.go:12`)
2. `config.Load()` reads `~/.config/adoBizInt/config.yaml`, then overlays `ADO_PAT`/`ADO_ORG_URL`/`ADO_PROJECTS`/`ADO_PROJECT` env vars (`internal/config/config.go:80`, `:107`, `:122`)
3. `cfg.Validate()` requires PAT and OrgURL only; process exits 1 on failure (`internal/config/config.go:144`, `cmd/adoBizInt/main.go:37`)
4. `ado.NewClient(cfg)` constructs a PAT connection and 5 SDK sub-clients, then calls `location.GetConnectionData` to resolve the current user display name (`internal/ado/client.go:34`, `:70`)
5. `config.LoadPriorities()` reads `priorities.yaml`; a failure is logged as a warning and does not abort (`cmd/adoBizInt/main.go:54`)
6. `ui.NewModel(...)` eagerly constructs all 6 sub-models (`internal/ui/app.go:52`)
7. `tea.NewProgram(model, tea.WithAltScreen()).Run()` (`cmd/adoBizInt/main.go:60`)

### Async ADO request path

Every ADO call follows an identical four-step shape. Using PR approval as the reference (`internal/ui/prs/prs.go:290`):

1. **Capture** -- the pointer-receiver helper copies the needed values out of the model into locals (`client`, `prID`, `project`, `repoID`) and sets `actionPending = true` and a status message.
2. **Dispatch** -- it returns a `tea.Cmd`, a `func() tea.Msg` closure over those locals. Bubbletea runs it on its own goroutine. The closure calls `client.ApprovePullRequest(context.Background(), ...)` (`internal/ui/prs/prs.go:308`).
3. **Deliver** -- the closure returns a typed message carrying only the result: `PRActionMsg{PRID, Action, Err}` (`internal/ui/prs/prs.go:42`).
4. **Reduce** -- the `Update` switch matches the message type, clears `actionPending`, sets a success or error status, and may return a follow-up fetch command (`internal/ui/prs/prs.go:164-174`).

The closure never touches `m`. This is what keeps the model race-free.

### Message inventory

Each view package declares its own message types. There is no shared message vocabulary except `link.ActionMsg`.

| Package | Messages | Declared at |
|---------|----------|-------------|
| `ui/link` | `ActionMsg` (the only cross-package message) | `internal/ui/link/link.go:13` |
| `ui/dashboard` | `CountsMsg` | `internal/ui/dashboard/dashboard.go:17` |
| `ui/workitems` | `WorkItemsMsg`, `StateUpdateMsg`, `EffortSubmitMsg`, `CloseEffortMsg`, `ChildTasksLoadedMsg`, `DetailLoadedMsg`, `CloseDetailMsg`, `AttachmentDownloadedMsg` | `workitems.go:44,50`; `effort.go:18,30,33`; `detail.go:21,27,30` |
| `ui/prs` | `PRsMsg`, `TickMsg`, `PRActionMsg` | `internal/ui/prs/prs.go:33,39,42` |
| `ui/pipelines` | `PipelinesMsg`, `DefinitionsMsg`, `TriggerResultMsg`, `TickMsg`, `CloseTriggerMsg`, `TriggerSubmitMsg`, `BranchesMsg` | `pipelines.go:34,40,46,53`; `trigger_modal.go:16,19,27` |
| `ui/releases` | `ReleasesMsg`, `ApprovalsMsg`, `ApprovalResultMsg`, `TickMsg`, `CloseApprovalMsg`, `ApprovalSubmitMsg` | `releases.go:37,43,49,56`; `approval_modal.go:16,19` |
| `ui/branches` | `BranchesMsg` | `internal/ui/branches/branches.go:32` |

`TickMsg` and `BranchesMsg` are each declared independently in two packages. Because Go type identity is package-scoped, these do not collide, but a `pipelines.TickMsg` delivered while the releases view is active is silently ignored.

### Root message routing

`internal/ui/app.go:75-177` applies three rules in order:

1. **Global keys first** -- `Quit` and `Help` are matched before anything else (`app.go:81-85`).
2. **Tab keys are gated on text-input focus** -- `m.isCurrentViewTextInputFocused()` is consulted first (`app.go:88`), so typing `2` into a search box does not switch views. Each sub-model exposes `IsTextInputFocused() bool` for this (`app.go:207-224`, implementation e.g. `internal/ui/prs/prs.go:504`).
3. **`tea.WindowSizeMsg` fans out to all six sub-models** (`app.go:136-143`); every other message goes to exactly the one sub-model matching `m.view` (`app.go:147-174`). The explicit `isWindowSize` guard at `app.go:147` prevents a double-update.

### Lazy sub-view initialisation

Tab2 through Tab6 call `sub.Init()` only when the view is not already active (`app.go:93-127`). `Init()` is where the first fetch and the polling ticker are started (`internal/ui/prs/prs.go:113`). Tab1 (`app.go:90-92`) does **not** call `m.dashboard.Init()`, because the dashboard is initialised once at program start (`app.go:70-73`).

## Key Abstractions

**The view sub-model:**
- Purpose: one self-contained Elm triple per Azure DevOps domain
- Examples: `internal/ui/prs/prs.go:48`, `internal/ui/workitems/workitems.go:56`, `internal/ui/branches/branches.go`
- Pattern: `New(client, orgURL, projects, ...) Model` / `Init() tea.Cmd` / `Update(tea.Msg) (Model, tea.Cmd)` / `View() string` / `IsTextInputFocused() bool`
- Common fields across all six: `client *ado.Client`, `orgURL string`, `projects []string`, `loading bool`, `err error`, `width int`, `height int`

**The `buildRows` render function:**
- Purpose: filter the raw `[]ado.X` slice and convert survivors to `[]table.Row`
- Examples: `internal/ui/prs/prs.go:425`, `internal/ui/workitems/workitems.go:564`, `internal/ui/pipelines/pipelines.go:432`, `internal/ui/releases/releases.go:462`, `internal/ui/branches/branches.go:271`
- Pattern: filtering, sorting, truncation, and styling are all fused into one pass. Called both to render and to count (`internal/ui/prs/prs.go:393` calls it just to compute "Showing N of M").

**The modal sub-sub-model:**
- Purpose: a focused overlay owning its own inputs
- Examples: `DetailModel` (`internal/ui/workitems/detail.go:62`), `EffortModal` (`internal/ui/workitems/effort.go:106`), `TriggerModal` (`internal/ui/pipelines/trigger_modal.go:58`), `ApprovalModal` (`internal/ui/releases/approval_modal.go:48`)
- Pattern: same Init/Update/View triple, returning its own concrete type. Held as a nilable pointer plus a `show*` bool on the parent (`internal/ui/workitems/workitems.go:79-83`).

**`ado.GetNextState`:**
- Purpose: pure state-machine lookup over the configured transition map
- Location: `internal/ado/workitems.go:510` -- a package-level function, not a method
- Consumed at `internal/ui/workitems/workitems.go:180` (to act) and `:622` (to decide whether to render the `[n] Next` hint)

## Entry Points

**`main()`:**
- Location: `cmd/adoBizInt/main.go:15`
- Triggers: process start
- Responsibilities: init logging, load and validate config, build the ADO client, build the root model, run the Bubbletea program

There is exactly one entry point. Verified by `rg '"flag"|spf13/cobra|urfave/cli|os\.Args'` across all `.go` files: no matches. There is no subcommand dispatch, no argument inspection, and no alternate output mode.

## The modal / overlay pattern

`internal/ui/modal/modal.go` is 24 lines and does one thing: wrap two already-rendered strings in throwaway `tea.Model` shims and hand them to `bubbletea-overlay` centred (`modal.go:21-23`).

Usage is identical at all four call sites:

```go
if m.showEffort && m.effortModal != nil {
    return modal.Center(m.effortModal.View(), m.renderTableView())
}
```
(`internal/ui/workitems/workitems.go:655-657`; also `:661-663`, `internal/ui/pipelines/pipelines.go:363`, `internal/ui/releases/releases.go:392`)

The parent's normal render body is extracted into `renderTableView()` so `View()` reduces to a modal check plus a delegate.

Input routing is handled separately, before the main `switch`. Three checks run in sequence at the top of the parent `Update`:

1. Close message -- clear the `show*` flag and nil the pointer (`internal/ui/pipelines/pipelines.go:179-181`)
2. Submit message -- clear the modal, then dispatch the resulting ADO command (`internal/ui/pipelines/pipelines.go:185-191`)
3. Delegate -- if the modal is open, forward the raw message to it and return early, so the parent's key handlers never see the keystroke (`internal/ui/pipelines/pipelines.go:194-206`)

Window size is pushed onto the modal by direct field assignment during delegation (`pipelines.go:200-201`), not through a message.

## Polling behaviour

Polling is opt-in per view, implemented with `tea.Tick`, and is **not** uniform.

| View | Polls | Ticker start | Guard conditions |
|------|-------|--------------|------------------|
| `dashboard` | No | -- | fetches once at program start |
| `workitems` | No | -- | `Init()` is a bare fetch (`workitems.go:129-131`) |
| `branches` | No | -- | -- |
| `prs` | Yes | `Init()` batches fetch + tick (`prs.go:114`) | `autoRefresh && !loading && !actionPending && !confirmComplete` (`prs.go:147`) |
| `pipelines` | Yes | `Init()` (`pipelines.go:127`) | `autoRefresh && !loading` (`pipelines.go:216`) |
| `releases` | Yes | `Init()` (`releases.go:128`) | `autoRefresh && !loading`; ticker is re-armed but the fetch is skipped entirely while the approval modal is open (`releases.go:201-203`, `:228`) |

Mechanics common to all three pollers:
- Interval comes from `config.PollInterval`, default 30s (`internal/config/config.go:82`), passed down through `ui.NewModel` (`app.go:62-64`).
- `tickCmd()` defensively re-defaults to 30s if the interval is non-positive (`prs.go:118-120`). Because the receiver is a value, this assignment does not persist -- the fallback is recomputed on every tick.
- The ticker is self-rescheduling: each `TickMsg` handler must append a fresh `m.tickCmd()` on both the refresh and the skip branch, or polling stops permanently (`prs.go:149,151`).
- Toggled at runtime by the `t` key (`prs.go:252-261`).

## Architectural Constraints

- **Threading:** Single-threaded update loop. Bubbletea serialises all `Update` calls; every `tea.Cmd` runs on its own goroutine but communicates only by returning a `tea.Msg`. No mutexes or channels exist in application code.
- **Context:** Every `ado` method takes a `context.Context` as its first parameter, but every one of the 21 call sites in `internal/ui/` passes `context.Background()`. `rg 'context.WithTimeout|context.WithCancel|context.WithDeadline' internal/` returns no matches. Nothing can be cancelled or time-limited. An in-flight fetch continues after a view switch and its result message is discarded by the root router.
- **Global state:** `internal/logging/logging.go:9` holds a package-level `*slog.Logger`. It falls back to `slog.Default()` when uninitialised (`logging.go:38-41`), so it is safe but is genuinely global.
- **Shared mutable state:** `*config.Priorities` is created once in `main` (`main.go:54`) and handed to the workitems model (`app.go:61`). It is the only pointer-shared mutable object below the client. It writes to disk from inside the update loop at six sites (`workitems.go:369,396,511,525,537,547`).
- **Circular imports:** None. The graph is a DAG; `internal/ado` has no knowledge of `internal/ui`.
- **Sub-model polymorphism:** None. Sub-models do not satisfy a common interface, so the root cannot iterate over them. Every operation is a hand-written six-arm switch.

## Anti-Patterns

### Filtering fused into row rendering

**What happens:** Domain filtering lives inside `buildRows()`, interleaved with truncation and lipgloss styling. `internal/ui/prs/prs.go:429-450` builds `validProjects` and `validRepos` lookup sets and applies them in the same loop that calls `utils.Truncate` and `getApprovalStyle`.

**Why it's wrong here:** The result type is `[]table.Row`, so no non-TUI caller can reuse the filter. It also forces `buildRows()` to be called a second time purely to obtain a count (`internal/ui/prs/prs.go:393`), re-running all filtering and styling on every frame.

**Do this instead:** Split into a pure `filter(items) []ado.PullRequest` and a `toRows(items) []table.Row`. The filter half then becomes callable from a non-TUI surface.

### Domain rules living in UI packages

**What happens:** `findDevelopmentTask` (`internal/ui/workitems/workitems.go:881`) encodes the rule for locating the development child task of a User Story. `doneStates` (`internal/ui/workitems/workitems.go:36`) defines which ADO states count as complete. `stripHTML` (`internal/ui/workitems/workitems.go:755`) sanitises ADO description markup.

**Why it's wrong here:** All three are Azure DevOps domain knowledge, not presentation. The counter-example is already in the tree: `ado.GetNextState` (`internal/ado/workitems.go:510`) is the same kind of rule and it correctly lives in the service layer.

**Do this instead:** Move them next to `GetNextState` in `internal/ado/workitems.go`.

### Multi-step workflows expressed only as message chains

**What happens:** Resolving a User Story is a four-hop chain with no single function that represents it:

| Hop | Trigger | Effect | Location |
|-----|---------|--------|----------|
| 1 | `n` key | detects `User Story` + `Active` -> `Resolved`, dispatches a child-fetch command | `workitems.go:198-212` |
| 2 | `ChildTasksLoadedMsg` | runs `findDevelopmentTask`, opens `EffortModal` | `workitems.go:240-260` |
| 3 | `EffortSubmitMsg` | reads the three effort fields off the modal | `effort.go:18` |
| 4 | command | calls `client.UpdateWorkItemStateWithEffort` | `internal/ado/workitems.go:244` |

**Why it's wrong here:** The orchestration exists only as reducer arms keyed on message types. It is not callable, not testable in isolation, and not reusable from any other surface.

**Do this instead:** Add `ado.ResolveUserStoryWithEffort(ctx, id, effort)` that performs the fetch-children, select-task, and update sequence. The UI keeps the modal for gathering input, and the reducer collapses to one command.

### Confirmation encoded as transient key state

**What happens:** Merging a PR sets `confirmComplete = true`, and the next keypress is intercepted before the normal bindings; `y` proceeds, anything else cancels (`internal/ui/prs/prs.go:215-224`, armed at `:237-251`).

**Why it's wrong here:** The safety gate for a destructive operation is a keyboard-only state machine. There is no parameterised path to the same operation. The comment at `prs.go:213-214` documents the ordering hazard this creates: the check must precede the regular bindings because `y` is otherwise bound to copy-link.

**Do this instead:** Keep the confirmation in the UI, but ensure the underlying operation is reachable by a plain call with an explicit force argument.

## Error Handling

**Strategy:** Errors travel inside the result message rather than short-circuiting.

**Patterns:**
- Every fetch/action message carries an `Err error` field alongside its payload (`PRsMsg` at `internal/ui/prs/prs.go:33`, `WorkItemsMsg` at `workitems.go:44`, `link.ActionMsg` at `link.go:13`).
- The reducer branches on `msg.Err` and writes into two model fields: `err error` for fetch failures rendered as a banner, and `statusMsg string` + `statusErr bool` for action failures rendered in the status line (`internal/ui/prs/prs.go:154-174`).
- Validation failures that never reach the network still return through the same channel, using `link.Fail(...)` or an inline error-only message, so the reducer path is uniform (`internal/ui/prs/prs.go:293-295`, `internal/ui/link/link.go:20`).
- The root `Model` also has an `err` field (`app.go:49`) and short-circuits `View()` when set (`app.go:180-182`), but no code path assigns to it. Verified: no `m.err =` in `internal/ui/app.go`.
- In `internal/ado`, non-fatal partial failures are logged and skipped rather than returned. `NewClient` swallows the identity lookup error and proceeds with an empty display name (`internal/ado/client.go:69-76`). `GetRecentIterations` logs a warning and returns the error (`client.go:132-135`).
- Only `main` exits. Four `os.Exit(1)` sites, all in `cmd/adoBizInt/main.go` (`:27`, `:42`, `:49`, `:66`).

## Cross-Cutting Concerns

**Logging:** `slog` text handler to `~/.config/adoBizInt/adoBizInt.log` at `LevelDebug` (`internal/logging/logging.go:23-31`). File-based specifically because stdout belongs to the TUI. Used by all 7 `ado` files and by `internal/ui/workitems/detail.go`.

**Validation:** Only two fields are required (`PAT`, `OrgURL`) at `internal/config/config.go:144-152`. `Projects`, `Pipelines`, `Repositories`, and `ReleaseDefinitions` are all optional and are treated as "no filter" when empty (`internal/ui/prs/prs.go:443,448`).

**Authentication:** A single PAT, exchanged once for an `azuredevops.Connection` at `internal/ado/client.go:35`, shared by all 5 SDK sub-clients. The PAT is read from `ADO_PAT` or the YAML `pat` key. `main.go:31-35` deliberately logs config without the PAT.

**Configuration precedence:** file, then env overlay (`config.go:85,92`), then backward-compat single-`project` promotion (`config.go:95-97`), then default state transitions if unset (`config.go:100-102`). Inside the client, `ReleaseDefinitions` falls back to `Pipelines` when empty (`internal/ado/client.go:80-83`).

## Suitability for a non-TUI CLI surface driven by an AI agent

The service layer is unusually well positioned for this. The presentation layer is not, and the entry point does not accommodate it at all.

### Reusable as-is

| Asset | Lines | Why it transfers cleanly |
|-------|-------|--------------------------|
| `internal/ado` (all 7 files) | 2,015 | Zero TUI imports, verified. All 25 exported methods are `ctx`-first. Returns `[]WorkItem`, `[]PullRequest`, `[]Release`, `*DashboardCounts` etc. -- plain structs with no display formatting (`internal/ado/workitems.go:21`, `pullrequests.go:16`, `releases.go:13`, `pipelines.go:16`, `branches.go:15`). |
| `internal/config` | 319 | YAML + env only. `Priorities` is a pure ordered-list manipulator with its own persistence. |
| `internal/logging` | 49 | Already writes to a file, not stdout, which is what a CLI wants for diagnostics. |
| `internal/utils/links.go` | 62 | Six pure URL builders. A CLI emitting links for an agent to follow needs exactly these. |
| `ado.GetNextState` | -- | Package-level pure function at `internal/ado/workitems.go:510`. |

A `bizint pr list --json` command could be built on `client.GetPullRequests(ctx, projects)` with no changes to `internal/ado` whatsoever. The write paths are equally direct: `ApprovePullRequest`, `CompletePullRequest`, `QueuePipeline`, `ApproveRelease`, `RejectRelease`, `UpdateWorkItemState` all take scalars and return `error`.

### TUI-coupled -- must be extracted before reuse

| Capability | Current location | Blocking detail |
|------------|------------------|-----------------|
| PR project/repo filtering | `internal/ui/prs/prs.go:429-450` | Fused into `buildRows()`, returns `[]table.Row` |
| Work item filtering (assigned-to-me, hide-done, hide-tasks) | `internal/ui/workitems/workitems.go:564-616` | Same fusion; `doneStates` map at `:36` |
| Priority ordering | `internal/ui/workitems/workitems.go:590-616` | Sort against `config.Priorities` happens inside `buildRows()` |
| HTML stripping of descriptions | `internal/ui/workitems/workitems.go:755` | Any text or JSON output needs this |
| Development-task selection | `internal/ui/workitems/workitems.go:881` | Domain rule stranded in the UI package |
| User Story resolve-with-effort | `workitems.go:198-212` -> `:240-260` -> `effort.go:18` -> `ado/workitems.go:244` | Exists only as a message chain, not a function |
| PR merge confirmation | `internal/ui/prs/prs.go:215-224` | Keypress-gated; no parameterised equivalent |
| Pipeline trigger (definition + branch selection) | `internal/ui/pipelines/trigger_modal.go` | Branch list fetch and selection are modal-resident |

### Structural gaps at the entry point

- **No argument handling exists.** `rg '"flag"|spf13/cobra|urfave/cli|os\.Args'` returns nothing across the repo. `main()` runs one unconditional path from config load to `p.Run()` (`cmd/adoBizInt/main.go:15-68`). A CLI surface requires inserting a dispatch branch before `ui.NewModel` at `main.go:59`.
- **No structured output path.** The only rendering is `View() string` producing ANSI-styled terminal text. None of the 12 exported `ado` structs carry `json:` tags, so `encoding/json` would emit Go-cased keys (`ID`, `RepositoryName`, `ApprovalStatus`). Verified by inspecting each type definition. Tags are additive and non-breaking.
- **No cancellation.** Every call site passes `context.Background()`. A CLI wants a per-invocation deadline. This is a caller-side fix only -- `internal/ado` already accepts and threads the context correctly.
- **Startup cost is unconditional.** `ui.NewModel` eagerly constructs all six sub-models including six `bubble-table` instances and several `textinput` widgets (`internal/ui/app.go:53-67`). A CLI path must branch before this line, not after.

### Assessment

The `cmd -> ui -> ado` split is real and clean, not nominal. Roughly 2,400 of 7,601 lines (`ado` + `config` + `logging` + `utils/links.go`) are directly reusable. Adding a CLI surface is an additive change: a new `cmd/` binary or a subcommand branch in `main()`, plus a thin formatter package. No change to `internal/ado` is required for the read and write primitives.

The friction is that the composite behaviours an agent would most want -- "list the PRs I actually care about", "resolve this story and log the effort", "trigger this pipeline on this branch" -- currently exist only as reducer arms in view packages. Making those agent-drivable means promoting filtering and workflow orchestration down into `internal/ado` (or a new package between `ado` and `ui`), then having the TUI consume the promoted functions. That refactor is bounded and mechanical, but it is not zero.

The single hardest-blocking fact is the absence of any argument parsing: there is currently no seam at which a CLI could attach.

---

*Architecture analysis: 2026-09-03*
