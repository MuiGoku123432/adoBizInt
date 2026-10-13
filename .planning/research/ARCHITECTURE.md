# Architecture Research

**Domain:** Scrum-master capabilities (team scoping, sprint data, nag rules, ceremony synthesis, Sprint tab, grooming, agent tiers, story writes, rate limits, SDK upgrade) added to an existing Go Azure DevOps TUI+CLI
**Researched:** 2026-10-13
**Confidence:** HIGH for integration points (read from the current code, and the SDK upgrade was trial-compiled). MEDIUM for the new ADO API usage patterns (SDK source checked, live behaviour not yet). LOW where noted. Those items need research in their own phase.

Confidence tags used below:
- **[CODE]**: read directly from the repo at HEAD `dc25a3c`, or from the SDK source in the Go module cache. HIGH.
- **[TRIAL]**: verified by experiment in a scratch copy of the repo. HIGH.
- **[DOC]**: Microsoft Learn pages. The research seam classifies `webfetch` as LOW. These are official pages, but treat them as unverified until a live call in the phase confirms them.

---

## Standard Architecture

### System Overview (v2.0 target; `NEW` / `MOD` marked)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ cmd/adoBizInt  (package main)                                                │
│  main.go  ── no args ──► ui.NewModel (TUI)                                   │
│           ── args ────► dispatch() ──► commands[noun verb]                   │
│  dispatch.go  MOD: entry = {run, tier}; tier gate runs before run()         │
│  write.go     MOD: shrinks to flag parsing, resolve, render. Guard body     │
│               moves out                                                     │
│  sprint verbs NEW: sprint nag|report|standup|planning|items, teams list     │
│  story verbs  NEW: workitems create|comment|link|unlink|update              │
└───────┬───────────────────────────────┬──────────────────────────────────────┘
        │ reads (Result[T])              │ writes (guard.Op)
        │                                ▼
        │              ┌──────────────────────────────────────────────┐
        │              │ internal/guard  NEW (extracted from cmd)     │
        │              │  audit sink, Op{plan,mutate}, Run(confirm),  │
        │              │  outcome classification, writeRecord         │
        │              └──────────▲───────────────────────────────────┘
        │                         │ same body for TUI writes (BL-01)
┌───────┼─────────────────────────┼────────────────────────────────────────────┐
│ internal/ui     MOD app.go (7th view), keymap.go (Tab7)                      │
│  workitems/prs/releases/pipelines  MOD: write closures call guard.Run       │
│  sprint/  NEW: rollup │ team drill │ findings   (sub-views, secondary key)  │
│  sprint/groom NEW: full-screen single-item grooming mode                    │
└───────┬───────────────────────────────────────────────────────────────────────┘
        │ fetch                     │ evaluate (pure, no I/O)
        ▼                           ▼
┌───────────────────────────┐   ┌──────────────────────────────────────────────┐
│ internal/ado  MOD          │   │ internal/scrum  NEW (pure functions)          │
│  teams.go   NEW            │──►│  nag.go: rules NAG-01..13 → []Finding         │
│  sprint.go  NEW snapshot   │   │  report.go / standup.go / planning.go         │
│  stories.go NEW create/    │   │  input: ado.SprintSnapshot, Thresholds, now   │
│   comment/link/fields/     │   └──────────────────────────────────────────────┘
│   reorder                  │
│  interfaces.go MOD +work,  │
│   +core narrow APIs        │
│  result.go MOD Failure.Team│
│  transport.go NEW retry    │
│  client.go MOD v7 import,  │
│   core client, transport   │
└────────────┬───────────────┘
             ▼
  azure-devops-go-api/azuredevops/v7 v7.1.0  (MOD from v1.0.0-b5)
  workitemtracking, work, core NEW, git, build, release, location
```

### Component Responsibilities: new vs modified

| Component | Status | Responsibility in v2.0 | File(s) |
|-----------|--------|------------------------|---------|
| `internal/guard` | **NEW** (extracted) | One guarded-write body for both surfaces: open the audit sink before anything is sent, plan (pre-read and diff), refuse a no-op, apply the confirm gate, mutate under a recover, classify the outcome, append the audit record | moved from `cmd/adoBizInt/write.go:103-309`, `audit.go`, `exit.go:30-97` |
| `internal/fetchctx` | **MOD** (moved) | Budget-context constructor. It now has three consumers (ui, cmd, guard). It sits under `internal/ui/` today but `cmd` already imports it, and guard must not import a ui package | `internal/ui/fetchctx/fetchctx.go` |
| `internal/scrum` | **NEW** | Pure rule and synthesis functions: nag findings, sprint report, standup, planning. No SDK and no network. `now` is injected | new |
| `internal/ado/teams.go` | **NEW** | Team discovery, membership, current and recent iterations per team, team settings (working days) and team field values (area scope) | new, consumes dormant `workClient` and a new `coreClient` |
| `internal/ado/sprint.go` | **NEW** | `GetSprintSnapshot(ctx, teams, opts) (Result[SprintSnapshot], error)`. Returns sprint-scoped items with no state filter, effort fields, tasks, parent links, PR link facts, and the baseline as-of snapshot | new (DATA-01..04) |
| `internal/ado/stories.go` | **NEW** | `CreateWorkItem`, `AddComment`/`GetComments`, `AddLink`/`RemoveLink`, `UpdateWorkItemFields` (points, assignee, iteration, AC), `ReorderBacklogItem` | new (STORY, GROOM-02/03) |
| `internal/ado/transport.go` | **NEW** | `http.RoundTripper` that honours `Retry-After` on 429, stays inside the context deadline, and never retries a non-429 failure on a non-GET request | new (PLAT-03) |
| `internal/ado/interfaces.go` | **MOD** | Adds narrow `workAPI` and `coreAPI` interfaces, plus new methods on `workItemAPI`/`gitAPI`, each declared only when first called (existing SC4 rule) | `interfaces.go:17-49` |
| `internal/ado/result.go` | **MOD** | `Failure` gains `Team` (always emitted). Adds a team-attributed `appendFailure` variant. Adds new `Scope` keys for sprint reads | `result.go:58-63`, `:98-103` |
| `internal/ado/client.go` | **MOD** | v7 import path. Constructs `core` client and installs the transport. The dormant `workClient work.Client` becomes `workAPI` | `client.go:27-41`, `:63-138` |
| `internal/ado/workitems.go`, `filters.go` | **MOD** | Receive `findDevelopmentTask` and `stripHTML` from the UI (second consumer appears: NAG-12, NAG-03). Add a `IsDelivered` predicate separate from `IsDoneState` | `ui/workitems/workitems.go:816,942` → ado |
| `internal/config` | **MOD** | `Teams` (per project), `Scrum` thresholds block, validation that every team's project is configured | `config.go:38-52`, `:144-174`. Thresholds go in a new `config/scrum.go` to keep `config.go` small |
| `cmd/adoBizInt/dispatch.go` | **MOD** | Table value becomes `{run commandFunc; tier tier}`. `dispatch()` enforces the tier. `readClient`/`writeClient` gain the new methods | `dispatch.go:59-130`, `:154-201` |
| `cmd/adoBizInt/write.go` | **MOD** | Keeps arity/resolve/flag parsing and human rendering. Calls `guard.Run`. Exit-code mapping stays here | `write.go:137-309` |
| `cmd/adoBizInt/usage.go` | **MOD** | Verb summaries for the new verbs. Help shows the tier | `usage.go:49+` |
| `internal/ui/app.go`, `internal/keymap` | **MOD** | Seventh `View` (`SprintView`), `Tab7` binding, a seventh arm in every hand-written switch | `app.go:22-30`, `:75-177`; `keymap.go` |
| `internal/ui/{workitems,prs,releases,pipelines}` | **MOD** | Write closures call `guard.Run` instead of `client.X` directly. They pass the item's real project instead of `""` and use a bounded context | `workitems.go:266,335,350,359`; `prs.go:361,384`; `releases.go:206,214`; `pipelines.go:205` |
| `internal/ui/sprint` | **NEW** | Sprint tab: rollup, team drill, findings list, jump to item | new view package |
| `internal/ui/sprint/groom` (or `internal/ui/groom`) | **NEW** | Full-screen grooming model, launched from the Sprint tab (SPRINT-05) | new |
| `.github` CI | **MOD** | `govulncheck` step (PLAT-02 gate). Run via `go run golang.org/x/vuln/cmd/govulncheck@<pin>` so it does not become a module dependency | CI workflow |

---

## Recommended Project Structure (delta only)

```
cmd/adoBizInt/
├── dispatch.go        # MOD: command{run, tier}; tier gate in dispatch()
├── write.go           # MOD: thin. Parse, resolve, guard.Run, render, exit map
├── sprint.go          # NEW: sprint nag|report|standup|planning|items closures
├── story.go           # NEW: workitems create|comment|link|unlink|update ops
└── tier.go            # NEW: tier type, interactive-terminal check
internal/
├── guard/             # NEW: extracted guard body (BL-01)
│   ├── guard.go       #   Op, Run, Report, outcome constants
│   └── audit.go       #   sink path, open, append (moved verbatim)
├── fetchctx/          # MOD: moved from internal/ui/fetchctx
├── scrum/             # NEW: pure synthesis, table-tested with no fakes
│   ├── nag.go         #   readiness + state-lie + slippage rules
│   ├── report.go      #   CEREMONY-01 five fields, velocity series
│   ├── standup.go     #   CEREMONY-02 moved/not-moved by person
│   └── planning.go    #   CEREMONY-03 ready / not ready + gaps
├── ado/
│   ├── teams.go       # NEW
│   ├── sprint.go      # NEW (snapshot + baseline)
│   ├── stories.go     # NEW (create/comment/link/fields/reorder)
│   └── transport.go   # NEW (429 / Retry-After)
├── config/scrum.go    # NEW: Teams + thresholds + defaults + validation
└── ui/sprint/         # NEW: tab + sub-views (+ groom/)
```

### Structure Rationale

- **`internal/guard` is a new package, not a file in `internal/ado`.** The guard decides policy: when the audit is written, what counts as unknown, whether confirm is required. It also owns a file sink. `internal/ado` is "all ADO I/O, no policy". The guard cannot stay in `package main` either, because the TUI cannot import `main`. With two callers (CLI, TUI) and an "audit must not fork" invariant, the reuse rule requires extraction. The standards forbid a second implementation.
- **`internal/scrum` is a new package, not more files in `internal/ado`.** The rules are table-testable with no fakes, need an injected `now`, and are consumed by the CLI and the TUI alike. `filters.go` set the precedent for small predicates in `ado`. Thirteen rules plus three ceremony syntheses is roughly 800 to 1,000 lines of non-I/O logic. Putting them in `ado` would make it the largest package and mix in time arithmetic that has nothing to do with HTTP. The justification is the same carve-out logic as `Result[T]`: the current task needs it, and it has two consumers on day one.
- **One file per concern** keeps every new file under 300 lines. `sprint.go` in `ado` is the one at risk. If the baseline as-of logic pushes it over, split it into `sprint.go` and `baseline.go`.

---

## Architectural Patterns

### Pattern 1: Fetch once, evaluate purely (nag and ceremony placement)

**What:** Every scrum verb is two calls. `ado.GetSprintSnapshot` fetches plain structs and returns `Result[SprintSnapshot]`, one item per team. Then a pure `scrum.X(snapshot, thresholds, now)` maps each team's snapshot to findings, a report, a standup or a planning view. The `Failures` and `Complete` of the fetch carry through to the output envelope unchanged.

**When:** All of NAG, CEREMONY and SPRINT (the TUI calls the same two functions).

**Trade-offs:** One snapshot shape must carry every field any rule needs, so it is wider than any single rule needs. That is the price of fetching once. It is also the protection against N+1 per rule.

```go
// cmd/adoBizInt/sprint.go
"sprint nag": {tier: tierRead, run: func(ctx context.Context, c client, args []string) int {
    return runRead(ctx, "sprint nag", args, "",
        func(ctx context.Context, teams, _ []string) (ado.Result[scrum.Finding], error) {
            snap, err := c.GetSprintSnapshot(ctx, teams, ado.SprintOpts{Baseline: thresholds.BaselineGrace})
            if err != nil { return ado.Result[scrum.Finding]{}, err }
            return mapResult(snap, func(s ado.SprintSnapshot) []scrum.Finding {
                return scrum.Nag(s, thresholds, time.Now())
            }), nil
        }, filterFindings, renderFindings)
}},
```

`mapResult[A,B]` is justified once four verbs use it (nag, report, standup, planning), which passes the rule of three. It must copy `Failures`, `Complete` and `Scope` verbatim.

**Core-value invariant:** a team whose snapshot failed yields zero findings and `complete:false`. "Zero findings" must never read as "the board is honest". Sub-reads inside a snapshot (PR links, baseline, team settings) record their own `Failure`. A rule whose input is missing must be skipped. It must never run on an empty input that looks like real data. This mirrors the existing `opGetWorkItemsIterations` degradation handling (`workitems.go:163-171`).

### Pattern 2: Team scoping as a value that threads config → ado → Result

**What:** One value type, `ado.TeamRef{Project, Team}` with JSON tags, is used everywhere.

- **config:** `teams: {ProjectA: [Team1, Team2]}`. `Validate` rejects a team whose project is not in `projects`, so the same allow-list rule as HARD-04 applies (`--team` must be configured). Discovery (`core.GetAllTeams`) is a read verb, `teams list`, that helps you fill in config. It never silently widens scope.
- **ado:** scrum reads take `teams []TeamRef`. An empty slice means "configured teams", the same convention as `projects` (`workitems.go:102-105`). The per-team loop copies the per-project loop, including the `ctx.Err()` short-circuit that records unreached teams as failures (`workitems.go:121-126`).
- **Result:** `Failure` gains `Team string json:"team"`, always emitted and `""` for project-scoped reads, so the key set stays stable (the D-P3 lesson). This is the "fifth field" `result.go:49-52` warns about. Update the exact-key-set test in the same change. Every payload item carries `team` and `project`.
- **Scope:** new keys are fixed-filter disclosures only. Examples: `"sprint":"current"`, `"baseline":"start+48h"`, `"backlog_depth":100`. Never a server-provided iteration name: scope values are code constants (`result.go:91-93`).
- **Rollup:** the rollup is the per-team item list. Do not emit synthetic "all teams" total rows. A total over three of four teams looks complete, which is the C1 failure. The TUI sums, and it sees `Complete`.
- **CLI flag:** `--team Project/Team`, repeatable, validated against config. TEAM-04 says single-team is a filter, so absence means all configured teams.

### Pattern 3: Time travel by ASOF, not revision replay (DATA-04, CEREMONY scope change, standup)

**What:** Baseline membership is the set of items in iteration *I* at `start + grace`. Get it with a WIQL query carrying an `ASOF '<utc>'` clause, then fetch fields with `GetWorkItems{AsOf: t}` [CODE: `GetWorkItemsArgs.AsOf` exists in v7 and v1]. The same mechanism answers:

| Question | as-of A | as-of B | Diff |
|----------|---------|---------|------|
| Scope change (CEREMONY-01) | baseline (start+48h) | now / finish | added, removed, and re-pointed (points differ) as three numbers |
| Carryover | finish of N | start of N+1 | in both and not delivered at N finish |
| Velocity series | finish of each past sprint | — | delivered points per sprint |
| Standup "what moved" (CEREMONY-02) | caller `--since` | now | state or remaining changed, grouped by assignee |

**Why not `ReadReportingRevisionsPost` as DATA-04 names it:** that endpoint is project-scoped with no iteration or team filter, and it pages by continuation token over every revision since `startDateTime` [DOC]. A sprint baseline would scan the whole project's history and filter on the client. That is an unbounded fan-out, exactly what PLAT-03 is trying to cap. ASOF costs one query plus ⌈n/200⌉ batches per team per point in time. **Recommendation:** amend DATA-04 to "reconstructed by as-of query". Keep the revisions API only if as-of turns out not to work live.

**Parameterisation (organization rule):** do not build the iteration path into the WIQL text. Filter by the integer `[System.IterationId] = <int>`, formatted with `strconv.Itoa` from the classification node ID. Format the `ASOF` literal from `time.Time` in UTC. Neither value is caller-supplied text. That keeps the `@project` macro discipline (HARD-04) and avoids a second `iterationClause`-style escaper (`client.go:280-298`, which is currently string escaping, not binding). **[LOW: confirm in DATA phase research that `System.IterationId` with `ASOF` behaves as expected live.]**

**Team scoping inside one iteration:** teams with a shared cadence (TEAM-05) usually share iteration paths, so `IterationId` alone over-selects. Filter on the client with `scrum`/`ado`'s `inTeamArea(areaPath, teamFieldValues)`, using `work.GetTeamFieldValues` [CODE: exists in both SDKs]. Apply the same filter to current and baseline sets. If the two sets are scoped by different mechanisms, scope change reports phantom adds and removes. That is why this recommends WIQL plus a client-side area filter for both, rather than `GetIterationWorkItems` (server-scoped) for current and something else for baseline.

### Pattern 4: One guard body, two front doors (BL-01)

**What:** move the `runWrite` steps from audit-open through audit append into `guard.Run(ctx, op guard.Op, confirm bool) guard.Report`. Each surface keeps only what is specific to it:

| Step | Owner after BL-01 |
|------|-------------------|
| Flag parse, positional id, `--project` arity, `resolve` | cmd (`write.go`) |
| Interactive preview and confirm keystroke | TUI modal |
| Open audit sink (D-13 position), `plan`, no-op refusal, confirm gate, `recover` around mutate, outcome classification, audit append, CR-01 append-failure → unknown | **guard** |
| Map outcome to exit code (0/1/2/4/5), emit JSON envelope, crash record | cmd |
| Map outcome to status line or error banner | TUI |

```go
// internal/guard/guard.go
type Op struct {
    Name   string                                         // "workitems set-state", the audit command string
    Target Target
    Plan   func(ctx context.Context) ([]Change, error)    // pre-read + diff
    Mutate func(ctx context.Context) error
}
type Report struct {
    Changes  []Change
    Outcome  string   // dry_run | no_op | executed | failed | unknown
    Err      error
    Panicked any      // non-nil → caller emits its own crash artifact
}
func Run(ctx context.Context, op Op, confirm bool) Report
```

`guard.Run` returns the outcome. It never calls `os.Exit` or writes to stdout. Today `runWrite`'s recover calls `handlePanic` (stdout crash record), which is meaningless inside a TUI. So the guard recovers, audits `unknown`, and reports `Panicked`. The CLI then calls `handlePanic(..., exitOutcomeUnknown)`, which keeps D-12 behaviour byte-identical.

**TUI flow:** press `n` or submit the effort modal. Call `guard.Run(confirm=false)`, which is a dry run and returns the diff. A confirm modal shows `field: from → to`. Pressing `y` calls `guard.Run(confirm=true)`. This re-plans. The audit then records what was actually executed, which narrows the D-03 race. The User Story resolve flow is a **compound write** (close the dev task with effort, then resolve the story, at `workitems.go:328-352`). It becomes two sequential `guard.Run`s. The second runs only if the first returned `executed`. Two audit records, one per mutation, matches D-25 (one target per write).

Whether the TUI keeps a separate confirm step or treats the modal submit as confirmation is BL-01's open product decision (`BACKLOG.md:60-63`). The architecture supports both by passing `confirm` explicitly.

### Pattern 5: Tier is table data, checked at one choke point (AGENT)

**What:** the dispatch map value becomes `command{run commandFunc; tier tier}`, where `tier ∈ {read, agent, human}`. `dispatch()` checks the tier before calling `run`. Tier is per verb, never per flag. That forces the verb split AGENT-02 and AGENT-03 imply:

| Verb | Tier | Why |
|------|------|-----|
| all reads, `teams list`, `sprint *`, `workitems comments` | read | — |
| `workitems update` (`--points`, `--assignee`, `--iteration`) | agent | AGENT-02 |
| `workitems create` (incl. `--parent` for tasks), `workitems comment`, `workitems link` / `unlink` | agent | AGENT-02, STORY |
| `workitems set-state` (state and effort), `prs complete`, `releases approve`/`reject`, `pipelines queue` | human | AGENT-03 |
| `prs approve` | **undecided** | Not named in AGENT-02 or AGENT-03. Decide in the AGENT phase. Recommend human (an approval vote is a review gate) |

**Enforcement mechanism (recommended):** human-tier verbs require stdin and stderr to be interactive terminals (`os.Stdin.Stat()` with `ModeCharDevice`, stdlib only) and a typed echo of the target id, in addition to `--confirm`. An agent subprocess has no TTY. Refuse with exit 3, or a new exit code if the phase decides "refused by tier" must be distinguishable. Exit codes are a one-way door (`dispatch.go:19-28`), so decide this deliberately. A unit test iterates `commands` and asserts every write key has a non-read tier, in the same style as `TestEveryWriteVerbAppearsInNounUsage`.

**Honest limitation, to document and not oversell:** the agent runs as the same OS user, so it can read the PAT in `~/.config/adoBizInt/config.yaml`. The TTY gate stops a well-behaved agent from using the tool. It does not stop a hostile process. The real capability boundary is a separate PAT scoped to Work Items (Read & Write) only, with no Code, Build or Release scopes, so ADO refuses human-tier mutations server-side. Recommend the AGENT phase document an `ADO_PAT` agent-profile pattern. That is configuration, not code.

**Breaking change to flag:** retrofitting `human` onto the five v1 write verbs means non-interactive scripts can no longer run `set-state` or `pipelines queue`. No known consumers exist (v1 shipped 2026-10-12), but record it as a decision.

### Pattern 6: Rate limiting at the transport, fan-out capping before the first request (PLAT-03)

**Retry/backoff: an `http.RoundTripper` in `internal/ado/transport.go`.** It is the only layer that can see `Retry-After`. The SDK's `WrappedError` carries `StatusCode` but no headers [CODE: `models.go:129-139` in v7], so a decorator at the narrow-interface seam could detect a 429 but not how long to wait.

- On 429: sleep `Retry-After` (seconds) and retry, up to N attempts, **only if** the wait fits inside the request context's remaining deadline. Otherwise return the 429 at once. It then classifies as the existing `FailureRateLimit` (`result.go:31-32,192-194`) and maps to exit 1. No new vocabulary is needed.
- Retry 429 for any method, because the request was blocked, not executed [DOC]. **Never** retry 5xx, timeouts or transport errors on non-GET requests. Those are the outcome-unknown cases `writeExitCodeFor` already reasons about (`exit.go:71-96`). A transparent retry would produce two mutations and one audit record.
- When `Retry-After` arrives on a 200 [DOC: it can], record a not-before time and delay the next request. This is a serial program, so one `atomic`/mutex-guarded timestamp is enough.
- The body is replayed via `req.GetBody`. Confirm the SDK builds requests from a `bytes.Reader`, which sets `GetBody`, in phase research.
- The transport must never log request headers. It sees `Authorization` (the PAT constraint in PROJECT.md).

**Installing the transport (this is the SDK-upgrade coupling):** in v1.0.0-b5 the SDK builds its own `http.Client` inside `azuredevops.NewClient` with no injection point [CODE: `client.go:53-73` v1]. The only hook is replacing the process-global `http.DefaultTransport`. v7 adds `NewClientWithOptions(conn, url, WithHTTPClient(hc))` [CODE]. Typed sub-clients still go through `connection.GetClientByUrl → NewClient` with no options, so build them explicitly: resolve resource areas once via the exported `GetResourceAreas`, then `&work.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, areaURL, WithHTTPClient(hc))}`. That is about 25 lines in `client.go` with no global mutation. **This is a strong reason to do PLAT-02 before PLAT-03.** Keep the `connection.Timeout` ordering rule (`client.go:43-53`) by putting the timeout on `hc` itself.

**Fan-out caps: request-shape limits validated before the first call, not a concurrency limiter.** Every existing fan-out is a serial `for` loop [CODE: no goroutines anywhere in `internal/` or `cmd/`]. The thing to cap is the request count, which is set by teams × sprints × backlog depth:

- `--sprints N` (velocity series) is capped by config (default 6, max 12). Exceeding it is a usage error, exit 3, before any request. Never truncate silently.
- Backlog scope for NAG-13 and GROOM-04 takes the top `backlog_depth` items by rank (default 100), **disclosed in `Scope`** as `backlog_depth`. The W4 lesson: an undisclosed limit makes a filtered result look complete.
- The number of teams comes from config, so it is known before the first call. Validate a ceiling there.
- Do not add a concurrency semaphore or errgroup in v2.0. Serial loops plus shape caps bound the request count by construction, and `golang.org/x/sync` would be a new dependency. Revisit only if a measured sprint read across all teams exceeds the fetch budget.

---

## Data Flow

### Scrum read (CLI and TUI share everything below the surface)

```
sprint report --team P/T --json
  └─ dispatch: tier=read ─► runRead (unchanged body)
       └─ fetch = GetSprintSnapshot(ctx, teams)          internal/ado
            per team (serial; ctx.Err() → unreached failures):
              work.GetTeamIterations(timeframe=current)   → iteration id/dates   (TEAM-03)
              work.GetTeamSettings                        → working days          (NAG-09)
              work.GetTeamFieldValues                     → area scope            (team filter)
              wit.QueryByWiql  [IterationId]=n            → ids (no state filter) (DATA-01/03)
              wit.GetWorkItems batches (fields incl. effort, Parent, AC)          (DATA-02)
              wit.QueryByWiql ... ASOF baseline + GetWorkItems{AsOf}              (DATA-04)
              PR facts for New/Active items (see gap below)                       (NAG-06/10)
            each sub-read → appendTeamFailure on error; snapshot keeps what answered
       └─ mapResult(snapshot, scrum.Report)               internal/scrum (pure)
  └─ emitJSON(Result[scrum.TeamReport]) ; exitCodeFor(failures, complete)
```

The TUI Sprint tab runs the same `GetSprintSnapshot` inside a `tea.Cmd` under `fetchctx.New(client.FetchTimeout())`. It then calls `scrum.Nag` and `scrum.Report` in the reducer or in the cmd closure. Both are pure, so either works. Keep them in the closure so `Update` stays cheap.

### Guarded write (both surfaces after BL-01)

```
CLI: parse → resolve ─┐                        TUI: key/modal → build Op (real project) ─┐
                      ▼                                                                    ▼
             guard.Run(ctx, op, confirm) ── auditOpen ─► Plan ─► no-op? ─► confirm? ─► Mutate (recover)
                                             │ fail→refuse      │ yes→no_op   │ no→dry_run   │
                                             ▼                                               ▼
                                         Report{outcome}  ◄─ audit append (executed|unknown) ◄┘
CLI: exit code + envelope                                      TUI: status line / reload
```

### Key Data Flows

1. **Team scope:** `config.Teams` → `TeamRef` list → per-team loop in `ado` → `Failure.Team` and `item.team` → `--team` post-filter and TUI drill. One type end to end.
2. **Thresholds:** `config.Scrum` (baseline grace, new-item days-left N, burn tolerance, backlog depth, max sprints) → passed by value into `scrum.*`. Never read from a global inside a rule.
3. **Readiness queue (GROOM-04):** `GetBacklogSnapshot` (team backlog via `work.GetBacklogLevelWorkItems`, which returns rank order [CODE: exists in both SDKs]) → `scrum.Nag(scope=backlog)` → keep only the NAG-01..04 and NAG-12 findings → ordered item queue → groom model. The same fetch provides real ADO rank for GROOM-03.
4. **NAG-12 remediation:** `sprint nag` finding `{rule: NAG-12, suggested_action: "create Development task"}` → agent runs `workitems create --type Task --parent <id> --title Development` (agent tier) → guard. The plan step refuses as a no-op (exit 4) if a child already matches `findDevelopmentTask`. That gives the agent idempotency against a retry after exit 5.

---

## Integration Points by Feature

| Feature | Integrates at | New | Modified | Hard dependency |
|---------|---------------|-----|----------|-----------------|
| **BL-01** | `cmd/write.go`, `audit.go`, `exit.go`, four TUI view packages | `internal/guard` | `write.go`, TUI write closures (9 call sites), `fetchctx` location, `EffortSubmitMsg` already `*float64` (part 1 closed) | none |
| **PLAT-02 SDK v7** | every `import` of the SDK; `go.mod` | — | all `internal/ado/*.go` and test imports; CI adds govulncheck | none (do early) |
| **PLAT-03** | `client.go` construction | `transport.go` | `client.go`; config caps | PLAT-02 (clean injection) |
| **TEAM** | `config`, `ado`, `result.go`, `dispatch.go` | `teams.go`, `config/scrum.go`, `teams list` verb, `coreAPI`/`workAPI` | `Failure`, `Client` struct, `readClient` | PLAT-02 if sequenced as recommended |
| **DATA** | `ado` | `sprint.go`, `sprint items` verb | `workitems.go` (receives `findDevelopmentTask`/`stripHTML`), `filters.go` (`IsDelivered`) | TEAM |
| **NAG** | `scrum`, `cmd` | `scrum/nag.go`, `sprint nag`, `mapResult` | `config/scrum.go` thresholds | DATA |
| **CEREMONY** | `scrum`, `cmd` | `report.go`, `standup.go`, `planning.go`, three verbs | — | DATA. `planning` also needs NAG readiness rules |
| **AGENT** | `dispatch.go` | `tier.go` | command table shape, help | BL-01 (guard), before any new agent-tier write ships |
| **STORY** | `ado`, `cmd` | `stories.go`, five verbs | `writeClient`, `workItemAPI` (+`CreateWorkItem`, `AddComment`, `GetComments`) | BL-01, AGENT |
| **SPRINT tab** | `ui/app.go`, `keymap` | `ui/sprint` | `app.go` (7th arm), `keymap.go` (Tab7) | DATA, NAG |
| **GROOM** | `ui/sprint` | `groom` model | — | BL-01, TEAM-02 (assignee choices), TEAM-03 (sprint choices), NAG-13, `UpdateWorkItemFields` + reorder (STORY phase) |

### External Services

| Service | Integration pattern | Notes |
|---------|---------------------|-------|
| `work` (teamsettings, backlogs, reorder) | dormant `workClient` → narrow `workAPI` | Team-routed URLs: the team name is a route value, not query text |
| `core` (teams, members) | new `core.NewClient` → narrow `coreAPI` | One more resource-area client. Discovery is already mandatory at startup (offline issue, `PROJECT.md:100`), so this adds no new failure mode |
| `wit` as-of / batch / create / comments | existing `workItemAPI` + methods | `CreateWorkItem{ValidateOnly:true}` [CODE] gives a server-validated plan for create dry-runs |
| `git` PR facts | existing `gitAPI` | See gap: avoid per-PR N+1 |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| cmd ↔ guard, ui ↔ guard | direct call, `guard.Report` value | guard never writes stdout or exits |
| ado ↔ scrum | plain structs (`ado.SprintSnapshot`) | scrum imports ado types only. ado never imports scrum |
| ui/sprint ↔ ui/workitems | root-level message or exported detail modal | Decide in the SPRINT phase. Avoid a view-to-view import if a root `JumpToWorkItemMsg` will do |
| config ↔ everything | value passed at construction | no new global state |

---

## Suggested Build Order

| # | Phase | Delivers | Why here |
|---|-------|----------|----------|
| 1 | **Guard extraction + TUI writes (BL-01)** | `internal/guard`; `fetchctx` move; nine TUI call sites through the guard; real project instead of `""`; bounded write contexts; end-to-end modal test | Every later write (STORY, AGENT tiers, GROOM edits and reorder) must land on one guard. Building them first would mean writing each twice or building on the bypass. PROJECT.md already orders this first |
| 2 | **SDK v7 + transport (PLAT-02, PLAT-03 backoff) + govulncheck** | import-path upgrade; explicit client construction with injected `http.Client`; 429 retry; the deferred v1.0 live smoke run once against v7 | [TRIAL] v7.1.0 builds, vets and passes the whole test suite after a mechanical import rewrite. So it is cheap now, before a third SDK client (`core`) and new interfaces are added, and before any new code is written against v1 types and 5.1 API docs. The transport injection is only clean on v7. The runtime API version jump (5.1 → 7.1-preview) is the real risk. One live smoke covers v1 verbs and the upgrade together |
| 3 | **Team foundation (TEAM-01..05) + fan-out shape caps** | `config.Teams`, `teams list`, membership, current sprint per team, `Failure.Team`, cadence check | Every scrum API is `(project, team)` scoped. Nothing below can be keyed without it |
| 4 | **Sprint data access (DATA-01..04)** | `GetSprintSnapshot`, as-of baseline, `sprint items` verb; promote `findDevelopmentTask`, `stripHTML`, add `IsDelivered` | Data before rules. Shipping `sprint items` alone lets live data be checked before any synthesis is built on it. **Needs phase research** (as-of with IterationId, team-area filter, PR link strategy) |
| 5 | **Board honesty (NAG-01..13)** | `internal/scrum/nag.go`, `sprint nag`, thresholds, backlog scope with disclosed depth | Pure functions over phase 4 output. Planning (6) and GROOM-04 (10) reuse its readiness rules |
| 6 | **Ceremony output (CEREMONY-01..05)** | `sprint report` first (highest priority), then `standup`, `planning` | Report needs as-of (4). Planning needs readiness rules (5) |
| 7 | **Agent tiers (AGENT-01..04)** | `command{run,tier}`, TTY gate, tier test over the table, PAT-scope guidance | Must land **before** the first new agent-reachable write. Retrofits tiers onto v1 verbs |
| 8 | **Story management (STORY-01..03) + field update** | create, comment, link, `workitems update` (points, assignee, iteration, AC), `UpdateWorkItemFields`, `ReorderBacklogItem` on the guard | Needs guard (1) and tiers (7). Produces the write methods GROOM reuses |
| 9 | **Sprint tab (SPRINT-01..05)** | `ui/sprint` rollup, drill, findings navigation, Tab7 | Needs snapshot (4) and findings (5). Can run in parallel with 7 and 8 (disjoint files) |
| 10 | **Grooming mode (GROOM-01..04)** | full-screen groom model, inline edits, reorder | Last: depends on 1, 3 (members, iterations), 5 (queue), 8 (write methods), 9 (launch point) |

**Ordering notes:**
- Phases 7 and 8 could merge into one phase with tiers as its first plan. Do not invert them.
- Phase 2 could run in parallel with phase 1. Only `go.mod` and imports overlap with BL-01's moved files. Keep them sequential anyway so the live smoke in phase 2 also exercises the new TUI guard path.
- If phase 2 slips, phase 3 onward still works on v1-b5. All required SDK methods exist there [CODE: `GetTeamIterations`, `GetTeamMembersWithExtendedProperties`, `GetAllTeams`, `ReorderBacklogWorkItems`, `CreateWorkItem`, `AddComment`, `GetPullRequestWorkItemRefs`, `GetTeamFieldValues`, `GetBacklogLevelWorkItems`, `GetWorkItems.AsOf` in both]. The transport would then need the global `http.DefaultTransport` swap.

---

## Scaling Considerations

The scale that matters is ADO's per-user TSTU budget, not users: 200 TSTU per sliding 5 minutes, delays up to 30 s, then 429 [DOC].

| Scale | Adjustment |
|-------|------------|
| 1 to 3 teams, current sprint | Serial loops are fine. Roughly 6 to 8 requests per team |
| 5 to 10 teams + velocity series (6 sprints) | Roughly 10 teams × (6 + 6×2) ≈ 180 requests per `sprint report`. Shape caps and transport backoff matter here. WIQL cost scales with project size [DOC], so keep `SELECT` minimal |
| TUI polling + Sprint tab open | Do not auto-poll the Sprint tab, or poll at ≥ 5 min. Existing views already poll every 30 s (`prs`, `pipelines`, `releases`) |

### Scaling Priorities

1. **First bottleneck:** velocity and carryover history (as-of queries × sprints × teams). Cap `--sprints` and cache nothing in v2.0. Re-measure live.
2. **Second bottleneck:** NAG-06 PR facts if done per item or per PR (N+1). See gap below.

---

## Anti-Patterns

### Anti-Pattern 1: TUI writes calling `client.X` directly after BL-01
**What people do:** add the grooming inline edit as another `client.UpdateWorkItemFields(context.Background(), ...)` closure, like the nine existing ones.
**Why it's wrong:** this is exactly BL-01. No audit, no pre-read, and `context.Background()` gives no deadline (`workitems.go:266` and others).
**Do this instead:** every mutation goes through `guard.Run`. Add a structural test asserting no `internal/ui` file calls an `ado.Client` mutation method directly, in the same style as the existing structural WIQL test.

### Anti-Pattern 2: Using `IsDoneState` as "delivered"
**What people do:** reuse `ado.IsDoneState` for CEREMONY "delivered".
**Why it's wrong:** `doneStates` includes `Removed` and `Done` (`filters.go:9-14`). A removed story would count as delivered velocity. The design-session definition is Resolved or Closed only.
**Do this instead:** add a separate `IsDelivered` predicate next to it, with a test pinning the difference.

### Anti-Pattern 3: Mixed scoping mechanisms between current and baseline sets
**What people do:** current sprint via `GetIterationWorkItems` (server team-scoped), baseline via a project-wide query.
**Why it's wrong:** items from sibling teams on the same iteration path appear as "added" or "removed". Scope-change numbers are confidently wrong.
**Do this instead:** one query shape for both points in time, plus the same `inTeamArea` filter.

### Anti-Pattern 4: Synthetic rollup totals
**What people do:** append an `"team":"ALL"` row with summed points.
**Why it's wrong:** with one team failed, the total is a partial sum presented as a fact. That is the core-value violation.
**Do this instead:** per-team items plus `failures`/`complete`. Consumers (deck pipeline, TUI) sum.

### Anti-Pattern 5: Retrying writes at the transport on anything but 429
**What people do:** generic "retry 5xx 3 times" middleware.
**Why it's wrong:** a 5xx on a PATCH does not prove the write failed (P-12 in `exit.go:88-92`). A retry can apply it twice under one audit line.
**Do this instead:** retry only 429 (blocked, not executed) and only within the deadline. Every other non-GET failure surfaces unchanged to `writeExitCodeFor`.

### Anti-Pattern 6: Rich-text overwrite in grooming
**What people do:** edit acceptance criteria as plain text and PATCH it back.
**Why it's wrong:** the field is HTML (NAG-03 strips it to judge emptiness). Writing stripped text over existing AC destroys formatting and lists.
**Do this instead:** in v2.0, allow AC edit only when AC is empty after stripping, which is exactly the NAG-03 remediation case. Otherwise show it read-only.

### Anti-Pattern 7: Two orderings for one backlog
**What people do:** groom reorder writes ADO rank while the Work Items tab keeps sorting by local `priorities.yaml` (`config/priorities.go`, `workitems.go:590-616`).
**Why it's wrong:** two sources of truth for "what's next". Users will reorder in one and see no change in the other.
**Do this instead:** decide in the GROOM phase. Either document that `priorities.yaml` is personal and separate from team rank, or have the groom view display ADO rank only and label it so.

---

## Gaps and Open Questions for Phase Research

- **NAG-06/NAG-10 PR facts without N+1 (DATA phase):** `GetPullRequestWorkItemRefs` is per PR [CODE]. The candidate is work item `$expand=Relations` to read `ArtifactLink` PR URLs, joined against one `GetPullRequestsByProject` per project (status completed, bounded by sprint start). ADO rejects `$expand` together with `fields` (MEDIUM, from experience), so this is a second batch call limited to New and Active items. Verify live.
- **`System.IterationId` + `ASOF` live behaviour.** LOW until tested.
- **SDK body replay (`GetBody`) for POST retries.** Confirm in the v7 `CreateRequestMessage`.
- **v7 runtime differences:** 5.1 → 7.1-preview API versions for PR completion, release approvals (vsrm) and comments (`AddComment` vs `AddWorkItemComment`). Fakes cannot catch these. Only the live smoke can.
- **Audit record surface/tier:** the record has exactly five keys (`audit.go:31-53`). Agent-versus-human attribution is the forensic question v2 creates. Decide whether to add a key (the one-way door accepts additions, but older lines will lack it) or encode it in `command` (e.g. `tui:workitems set-state`). Recommend deciding in phase 1 before the TUI starts appending.
- **`prs approve` tier**, and whether "refused by tier" gets its own exit code.
- **TEAM-05 drift:** if two teams' current iteration dates differ, the rollup's shared axis breaks. Emit each team's dates on its item and have the TUI detect mismatch. Do not fail the read.

---

## Sources

- Repository at HEAD `dc25a3c`: `cmd/adoBizInt/{dispatch,write,audit,exit,main,flags,usage}.go`, `internal/ado/{client,interfaces,result,workitems,filters}.go`, `internal/config/config.go`, `internal/ui/fetchctx/fetchctx.go`, `internal/ui/workitems/workitems.go`, `internal/keymap/keymap.go` [CODE, HIGH]
- SDK source in module cache: `azuredevops@v1.0.0-b5` and `azuredevops/v7@v7.1.0` (`client.go`, `connection.go`, `models.go`, `work/client.go`, `core/client.go`, `workitemtracking/{client,models}.go`) [CODE, HIGH]
- Trial upgrade: repo copy with imports rewritten to `/v7`, `go build ./...`, `go vet ./...`, `go test ./...` all pass on v7.1.0 [TRIAL, HIGH]
- Rate and usage limits: https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits (updated 2026-07-31) [DOC]
- Get Iteration Work Items (7.1): https://learn.microsoft.com/en-us/rest/api/azure/devops/work/iterations/get-iteration-work-items?view=azure-devops-rest-7.1 [DOC]
- Read Reporting Revisions Post (7.1): https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/reporting-work-item-revisions/read-reporting-revisions-post?view=azure-devops-rest-7.1 [DOC]
- WIQL syntax incl. `ASOF`: https://learn.microsoft.com/en-us/azure/devops/boards/queries/wiql-syntax?view=azure-devops (updated 2026-07-23) [DOC]

---
*Architecture research for: adoBizInt v2.0 Scrum Master*
*Researched: 2026-10-13*
