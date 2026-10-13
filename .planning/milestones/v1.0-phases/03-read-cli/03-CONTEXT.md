# Phase 3: Read CLI - Context

**Gathered:** 2026-09-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Add a non-TUI command surface over the eight existing aggregate reads in
`internal/ado`: subcommand dispatch inserted before `ui.NewModel`, machine-parseable
JSON on stdout under `--json`, an exit-code contract keyed on Phase 2's four failure
kinds, agent-relevant filters promoted out of the view packages into `internal/ado`,
per-invocation deadline and interrupt handling, and a structured crash record. The
no-argument TUI path must not change behavior.

**In scope:** argument parsing and subcommand dispatch in `cmd/adoBizInt/main.go`;
read subcommands for the six nouns (eight verbs); `--json` serialization of the
Phase 2 `Result[T]` envelope; a human-readable default renderer; the exit-code
mapping from `FailureKind` to process status; `--timeout` and SIGINT handling;
promotion of hide-done, assigned-to-me, by-project, by-repo and free-text search
out of the view `buildRows()` methods into `internal/ado`, with the TUI rewired to
consume them; `--help` at three levels; the panic and interrupt record.

**Not in scope:** write subcommands (CLI-03, Phase 4); dry-run, confirmation and
audit (Phase 4); attachment host allow-list, path containment, WIQL escaping and
nil guards (Phase 5); CI (Phase 5); the `config.Priorities` sort and HTML
description stripping still fused in `internal/ui/workitems` (deferred);
rate-limit backoff and N+1 fan-out restructuring (deferred, PLAT-03).

</domain>

<decisions>
## Implementation Decisions

### Command surface shape

- **D-01:** The tree is **noun-verb** — `adoBizInt prs list`. Chosen over a flat
  noun (`adoBizInt prs`) and hyphenated forms (`adoBizInt list-prs`) explicitly to
  leave room for Phase 4's seven write verbs on the same nouns without
  restructuring. The alternative would force writes on as flags
  (`adoBizInt prs --approve 123`), which is worse.
  — **Reversibility:** one-way — once agents and scripts invoke these command
  paths, renaming them breaks every caller. This is the CLI's published surface.

- **D-02:** The eight read methods map to **six nouns with a second verb where the
  noun has two reads**: `pipelines runs` / `pipelines definitions`, and
  `releases list` / `releases approvals`. Each verb maps 1:1 to exactly one `ado`
  read method, so no command silently merges two envelopes. Rejected: a flag that
  swaps the payload (`pipelines list --definitions`) — a boolean that changes the
  output schema is hostile to an agent parsing it. Rejected: eight top-level nouns,
  which diverges from ROADMAP SC2's six-noun wording.

  Full command list:

  | Command | `ado` method |
  |---|---|
  | `workitems list` | `GetWorkItems` |
  | `prs list` | `GetPullRequests` |
  | `pipelines runs` | `GetPipelineRuns` |
  | `pipelines definitions` | `GetPipelineDefinitions` |
  | `releases list` | `GetReleases` |
  | `releases approvals` | `GetPendingApprovals` |
  | `branches list` | `GetBranches` |
  | `dashboard counts` | `GetDashboardCounts` |

- **D-03:** The dashboard **keeps a verb** — `dashboard counts`, not a bare
  `dashboard`. Every command is `<noun> <verb>` with no exceptions, so a caller or
  a generated wrapper can assume the shape. `counts` is also accurate: the method
  is `GetDashboardCounts` and it returns a one-element `Items` slice of
  `DashboardCounts` (Phase 2 D-01). Rejected `dashboard show` — `show` implies
  rendering, and the command emits an aggregate object.

- **D-04:** Nouns **match the Go package names**: `workitems` and `prs`, not
  `work-items` / `pull-requests`. No aliases. Two names for one command means
  agent-generated scripts diverge and help output documents both.

### Exit code contract

- **D-05:** A **partial result exits non-zero.** `complete:false` already rides in
  the payload, but an agent checking only `$?` would read an understated dashboard
  total as a clean fetch. Phase 2's entire premise is that a broken call must not
  look like a normal one; silence at the exit-code layer reintroduces exactly that.
  Accepted cost: naive `set -e` scripts abort on partial data.

- **D-06:** The codes are **0 = success, 1 = partial, 2 = auth failure,
  3 = outcome-unknown**. Sequential, leaves 4+ free for Phase 4 write outcomes,
  avoids the shell-reserved 126/127.
  — **Reversibility:** one-way — the integers become the contract every agent and
  CI job branches on; changing them silently breaks callers that still run.

- **D-07:** When a read returns **failures of mixed kinds, `auth` wins.** Any
  `FailureAuth` entry in the list forces exit 2 regardless of what else failed. A
  rejected token is the one failure that is definitely not transient and definitely
  requires operator action, so it must never be masked by a co-occurring timeout.
  The complete failure list remains in the payload for anything wanting detail.
  Rejected: most-frequent-kind-wins (one auth among three timeouts would exit 1 and
  the operator would never learn the token is dead) and always-partial (collapses
  the distinction OUT-04 asks for, and Phase 2's four-kind taxonomy exists
  precisely to make this mapping mechanical).

- **D-08:** The **three existing `os.Exit(1)` sites in `main.go` move to exit 3.**
  Config load (`main.go:27`), validation (`main.go:38`) and client construction
  (`main.go:46`) all happen before any read is attempted — no data, no failure
  list, nothing tried — so outcome-unknown is the accurate classification. This
  also clears code 1 of a second meaning, closing the ambiguity that otherwise
  makes a stray `os.Exit(1)` indistinguishable from a legitimate partial result.

- **D-09:** A **flag parse error or unknown subcommand exits 3**, not stdlib's
  default 2. See D-17.

- **D-10:** A **panic and a SIGINT both exit 3.** See D-21.

### JSON output shape

- **D-11:** `--json` emits the **raw Phase 2 `Result[T]` envelope and nothing
  else** — `{"items":[...],"failures":[...],"complete":true}`. Phase 2 D-01 built
  one envelope specifically so Phase 3 writes its JSON handling once; wrapping it
  in invocation metadata would add a second shape on top of the one designed to be
  the contract, and would add fields with no current consumer (CONVENTIONS rule 5).
  Rejected: an `exit_code` field in the payload — it duplicates a value the caller
  already has and creates a second source of truth.
  — **Reversibility:** one-way — this is the parse target for every downstream
  agent. Adding a wrapper later changes every access path.

- **D-12:** There **is a human-readable default mode**; `--json` opts in. The same
  binary is the TUI, so a human running `adoBizInt prs list` at a terminal should
  get readable output. Accepted cost: a formatter for eight payload types, each a
  place where displayed output can drift from the JSON contract. Rejected:
  JSON-always (would make `--json` a no-op flag that OUT-01 names explicitly).

- **D-13:** In human mode, **rows go to stdout and failures go to stderr.** In
  `--json` mode failures already ride inside the envelope on stdout, which is
  parseable and satisfies OUT-03. The split means `adoBizInt prs list > out.txt`
  leaves the warning visible on the terminal and never contaminates the data
  stream. Rejected: suppressing failures in human mode — a human running the
  default is exactly the caller Phase 2 says must not mistake a broken read for an
  empty one.

- **D-14:** JSON is **compact single-line; there is no `--pretty` flag.**
  Byte-for-byte reproducible, diff-able across runs, friendly to log capture. `jq .`
  is universally available for anyone who wants indentation. Rejected: TTY
  auto-detection — output that changes shape based on invocation context is a
  classic "works in my shell, breaks in CI" bug.

### Filter promotion

- **D-15:** Promoted filters live **in `internal/ado` as pure functions.**
  Filtering is a domain rule, not a display rule — `doneStates` and assigned-to-me
  matching are ADO semantics. Chosen over a new intermediate package (which
  ARCHITECTURE.md itself suggested) and over duplicating the logic in the CLI.

  **Recorded cost the planner must surface rather than absorb:** `internal/ado` is
  already 2,015 lines across 7 files against CONVENTIONS.md's 300-line standard,
  and `workitems.go` alone is 551. Adding a pure-logic concern to a package that is
  otherwise an SDK boundary makes this worse. Where the promoted functions live
  inside the package is a real decision — see the agent's Discretion.

- **D-16:** **The TUI is rewired to consume the promoted functions in this phase.**
  `buildRows()` keeps building `[]table.Row` but calls the promoted `ado` functions
  for the filtering decision instead of inlining it, so there is exactly one
  definition of `doneStates` and assigned-to-me matching and the CLI and TUI cannot
  drift. Rejected: leaving `buildRows()` untouched and having the CLI call the new
  functions — that leaves two copies of the same domain rule, which is precisely
  what CLI-06 was written to prevent.

  **Risk the plan must address:** this refactor touches live TUI code that CLI-01
  requires to have no behavior change. The plan needs a behavior-equivalence test
  per promoted filter, before and after.
  — **Reversibility:** costly — undoing means re-inlining the predicates into two
  view packages and re-establishing the duplication CLI-06 removed.

- **D-17:** The promoted set is the **four filters CLI-06 names plus free-text
  search**: hide-done (the `doneStates` map at `workitems.go:36`), assigned-to-me
  (matches `currentUser` / `currentUserEmail`), by-project, by-repo, and the search
  filter currently in the `prs` and `branches` views. Search is included because an
  agent listing PRs will want `--search` immediately and it is already implemented;
  leaving it would mean a second promotion pass. The `config.Priorities` sort
  (`workitems.go:590-616`) and HTML description stripping (`workitems.go:755`) are
  **not** promoted — the sort is display ordering, and JSON output ordering is a
  different decision.

- **D-18:** **The CLI defaults to no filtering.** The workitems TUI defaults
  `assignedToMeFilter` and `hideDoneFilter` to true (`workitems.go:139-140`)
  because that is the useful human view; a CLI must not hide data the caller did
  not ask to hide. An agent running `workitems list` and silently receiving a
  filtered subset is the exact failure this project exists to prevent — and
  `complete:true` would be true while the item list is not the whole picture.
  Filters are opt-in flags: `--assigned-to-me`, `--hide-done`. Rejected:
  config-driven defaults — the same command would return different data on two
  machines, hostile to agent reproducibility.

### Flag parsing mechanism

- **D-19:** **stdlib `flag`, one `flag.NewFlagSet` per subcommand.** Zero new
  dependencies; PROJECT.md forbids them without stated justification and `go.mod`'s
  seven direct requires all serve the TUI. A two-level noun-verb tree of eight
  commands is well within what stdlib handles. Rejected: cobra (large dependency
  tree for a problem stdlib solves at this scale) and urfave/cli (still a new
  dependency, and a different mental model than the rest of the codebase).
  Accepted cost: CLI-05's `--help` needs a hand-written usage function per level,
  and stdlib does not give subcommand `-h` for free.

- **D-20:** **Global flags are registered into every FlagSet by a shared
  function.** `--json`, `--timeout` and `--project` are meaningful on every read
  command, so they are defined once and registered into each subcommand's FlagSet.
  This prevents the drift where `prs list` accepts `--timeout` and
  `releases approvals` silently does not — stdlib gives no compile-time check
  against eight independent declarations. Rejected: parsing globals before the
  subcommand (`adoBizInt --json prs list`) — unusual placement that both agents and
  humans get wrong, and stdlib's parser stops at the first non-flag argument anyway.

- **D-21:** **`--help` works at three levels.** `adoBizInt --help` lists the six
  nouns; `adoBizInt prs --help` lists its verbs; `adoBizInt prs list --help` lists
  its flags. Each level is discoverable from the one above, which is how an agent
  explores an unfamiliar CLI. Rejected: two levels (a bare `adoBizInt prs --help`
  failing is a dead end an agent will hit) and a single flat page (unreadable once
  Phase 4 adds seven write verbs with their own dry-run and confirmation flags).

- **D-22:** **stdlib's default flag-error behavior is overridden.** `FlagSet` is
  constructed with `ContinueOnError` and `Usage` is replaced, because stdlib prints
  usage and exits **2** — which is now `auth failure` and would be a lie. A bad
  flag or unknown subcommand prints usage to stderr and exits **3**: nothing ran,
  so outcome-unknown is accurate. Rejected: a fifth exit code for usage errors —
  OUT-04 names four outcomes, and a usage error genuinely is "nothing was
  attempted". Rejected: accepting stdlib's 2 — a typo in a flag name would be
  indistinguishable from a rejected PAT, which an agent retrying with fresh
  credentials would get badly wrong.

### Panic and outcome-unknown

- **D-23:** A panic emits a **structured JSON record on stdout.** OUT-05's stated
  purpose is that a panic must not "look like an empty result" — an agent parsing
  stdout must get a record it recognises as a crash rather than nothing at all.
  Rejected: stderr-only, which produces exactly the empty stdout OUT-05 forbids.

  **Constraint the plan must honor:** if the panic fires mid-encode, a partially
  written payload could precede the crash record on stdout. The payload must be
  marshalled to a buffer and written atomically for this decision to hold.

- **D-24:** The record **carries no stack trace**; the full stack goes to the log
  file at `~/.config/adoBizInt/adoBizInt.log`, which `logging.Init` already opens.
  Go stack traces print function arguments, and `ado.NewClient(cfg)` takes a
  `config.Config` carrying the PAT — including the stack would be a
  credential-disclosure path onto stdout, the same class of leak Phase 2's T-02-01
  forbids. The record carries the panic message plus the log path.

- **D-25:** **A SIGINT during a fetch exits 3, the same as a panic.** Both are "the
  read did not complete and no result is known". Phase 2 already classifies
  cancellation as `FailureOther` rather than `FailureTimeout` (`result.go:135`)
  specifically so an interrupt is never mistaken for a server problem; exit 3
  extends that reasoning to the process level. Each emits a record naming which
  occurred, so the payload carries the distinction the exit code does not. Rejected:
  130 (128+SIGINT) — a fifth code outside OUT-04's four, requiring the caller to
  know the convention. Rejected: SIGINT exits 0 — a script that interrupts its own
  child could not distinguish "interrupted, no data" from "clean read, no matching
  items".

- **D-26:** The crash record is **a distinct top-level error object, not a
  `Result[T]` envelope** — shape along the lines of
  `{"error":{"kind":"panic"|"interrupt","message":"...","log_path":"..."}}`. An
  agent distinguishes a crash from a result by which key is present, with no chance
  of reading a fabricated empty `items` as real data. Rejected: reusing `Result[T]`
  with a synthetic failure entry — it makes a crash representable as a normal read
  outcome, and a caller checking only `.items` sees emptiness.
  — **Reversibility:** one-way — this is a published output shape; changing it
  breaks any agent that learned to detect crashes by it.

### the agent's Discretion

The user answered every question asked directly and delegated no decision. The
following were surfaced during discussion but not opened as areas, or fall out of
decisions above. The planner should decide them or raise them:

- **`"items":null` vs `"items":[]` (highest priority).** Go marshals a nil slice as
  `null`, and Phase 2's `newResult` (`result.go:100`) does not pre-allocate. An
  empty read would emit `{"items":null,...}`, and an agent evaluating
  `.items | length` gets an error rather than `0`. This is a direct instance of the
  project's core value at the JSON boundary and should be settled explicitly, with
  a test, before any read verb ships. The same applies to `failures`.
- **Unknown first argument.** CLI-01 says no subcommand launches the TUI. Whether
  an *unrecognised* first argument falls through to the TUI or errors under D-22 is
  unspecified. Erroring is almost certainly right — falling through would make a
  typo start a full-screen TUI in a non-interactive context — but it is not stated.
- **What `--timeout` overrides.** Phase 2 D-11 defined two separate budgets,
  `request_timeout` (per-call, via `connection.Timeout`) and `fetch_timeout` (whole
  fan-out, via `fetchctx.New`). Whether the single `--timeout` flag maps to one,
  the other, or both is open. Note `fetchctx.New` is TUI-oriented and wraps
  `context.Background()` internally — the CLI needs either a variant that accepts a
  parent context or its own construction, since signal handling requires a
  cancellable parent.
- **File placement of the promoted filters** inside `internal/ado`, given D-15's
  recorded 2,015-line problem. A new `filters.go` is the obvious answer but the
  plan should state it rather than scatter predicates across existing files.
- **Identity source for `--assigned-to-me`.** Whether it resolves from the cached
  `CurrentUser()` / `CurrentUserEmail()` on the client or takes an explicit
  `--user` value. Note Phase 1 D-12 leaves identity resolution non-fatal, so the
  cache can be empty — the flag needs defined behavior in that case.
- **Whether the CLI path calls `logging.Init`.** D-24 depends on the log file
  existing. `main.go:17` currently initialises it unconditionally and warns to
  stderr on failure; the CLI path must keep that or D-24's log-path reference is
  dangling.
- **Determinism of output ordering** across runs. Phase 2's reads iterate a
  configured project slice, so ordering is probably stable, but an agent diffing
  successive `--json` outputs depends on it and no test asserts it.
- **Whether human-mode rendering reuses the lipgloss/`bubble-table` styling** or
  emits plain text. ANSI escapes in a redirected stdout are hostile; plain text is
  the safer default but is a new renderer.
- **Whether the exit-code changes in D-08 apply to the TUI path.** CLI-01 forbids
  behavior change, and an exit code is arguably behavior — though the three sites
  in question all abort before the TUI starts, so this is likely moot.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase-defining analysis

- `.planning/codebase/ARCHITECTURE.md` §"Suitability for a non-TUI CLI surface
  driven by an AI agent" (:304-343) — **Highest-value read for this phase.**
  §"Reusable as-is" (:308-318) records that all of `internal/ado` transfers with no
  change and that `bizint pr list --json` is buildable directly on
  `GetPullRequests`. §"TUI-coupled — must be extracted before reuse" (:320-331) is
  the verified table of seven capabilities fused into the view packages with exact
  file:line, and is the source list D-17 selects from. §"Structural gaps at the
  entry point" (:333-338) confirms no argument handling exists anywhere, names
  `main.go:59` as the dispatch point, and warns that `ui.NewModel` eagerly
  constructs six sub-models so the branch must land *before* that line.
- `.planning/codebase/CONVENTIONS.md` — rule 5 (no abstraction unless the task
  requires it), rule 6 (justify dependencies — governs D-19's stdlib choice), and
  the 300-line file standard that D-15's recorded cost runs against.
  §"Toolchain Baseline" records that only `go vet` is configured and no CI exists.
- `.planning/codebase/CONCERNS.md` — C1 is the origin of Phase 2 and of the exit
  codes in D-06. MED3 records the absence of retry, backoff and 429 handling behind
  the `rate_limit` kind D-07 maps.
- `.planning/codebase/TESTING.md` — test-file naming convention and the
  table-driven stdlib style the CLI tests must match. No testify, no mock
  generator anywhere in `go.mod`.
- `.planning/codebase/INTEGRATIONS.md` §"Complete Method Inventory" — the SDK calls
  each read issues, for anyone reasoning about what a `--timeout` actually bounds.

### Prior phase decisions

- `.planning/phases/02-honest-reads/02-CONTEXT.md` — **Required reading.** D-01
  (the `Result[T]` envelope this phase serializes), D-04 (snake_case field names,
  already marked one-way and now becoming the CLI contract), D-06/D-07 (the four
  failure kinds the exit codes in D-06/D-07 above key on), D-09/D-10/D-11 (the two
  timeout budgets `--timeout` must map onto), and the `duration_ns` /
  RFC 3339-sentinel / `Failure.Err`-as-string serialization decisions recorded in
  STATE.md's Phase 02 decision log.
- `.planning/phases/01-test-seam-for-internal-ado/01-CONTEXT.md` — D-05/D-06/D-09
  define the unexported `-API` interfaces and the struct-literal injection seam
  every CLI test can reuse. D-12 records that identity resolution is non-fatal at
  construction, which is why `--assigned-to-me` needs defined behavior on an empty
  cache. Its deferred list flags Phase 3 as the proper home for revisiting whether
  a failed identity fetch should be fatal for a non-interactive caller.

### Requirements and scope

- `.planning/REQUIREMENTS.md` §"CLI Surface" CLI-01, CLI-02, CLI-04, CLI-05,
  CLI-06 (:31-36) and §"Structured Output" OUT-01, OUT-03, OUT-04, OUT-05
  (:40-44). §"Traceability" (:227-228) maps all nine to Phase 3. OUT-02 is already
  complete (Phase 2). CLI-03 is Phase 4.
- `.planning/ROADMAP.md` §"Phase 3: Read CLI" (:92-114) — goal, the seven success
  criteria, and the four provisional plan titles (03-01 through 03-04).
  §"Phase 4: Guarded writes" (:116+) for the write verbs D-01's noun-verb tree is
  shaped to accommodate.
- `.planning/PROJECT.md` §"Core Value" — "An agent driving this tool must never
  mistake a broken call for an empty result", which D-05, D-13, D-18, D-23 and D-26
  each trace back to. §"Constraints" — no new dependencies without justification
  (D-19), TUI unchanged (CLI-01, and the risk recorded under D-16), files under
  300 lines (the cost recorded under D-15).

### Source files this phase modifies

- `cmd/adoBizInt/main.go` — the whole file. Dispatch branches in before
  `ui.NewModel` at `:59`. The three `os.Exit(1)` sites at `:27`, `:38`, `:46`
  become exit 3 (D-08). `logging.Init()` at `:17` must stay on the CLI path (D-24).
- `internal/ado/` — new file(s) for the promoted filter predicates (D-15, D-17) and
  for the CLI's serialization needs if any. Existing `result.go` defines the
  `Result[T]` / `Failure` / `FailureKind` types D-11 emits and D-06/D-07 map;
  `classifyFailure` at `:135` documents the cancellation-before-timeout ordering
  D-25 depends on.
- `internal/ui/workitems/workitems.go` — `doneStates` (:36), the filter fields
  (:70-74), the defaults (:139-140), and `buildRows()` (:612) rewired to call the
  promoted functions (D-16). The priority sort (:590-616) and HTML stripping (:755)
  stay put (D-17).
- `internal/ui/prs/prs.go` — `buildRows()` (:497) and the search filter (:524).
- `internal/ui/branches/branches.go` — `buildRows()` (:332) and its search filter.
- `internal/ui/fetchctx/fetchctx.go` — `New` wraps `context.Background()`
  internally, which the CLI's signal handling cannot use as-is. Either it gains a
  parent-context variant or the CLI builds its own; see the agent's Discretion.
- `internal/config/config.go` — `RequestTimeout` and `FetchTimeout` fields that
  `--timeout` overrides.

### Style reference

- `internal/utils/text_test.go` — the table-driven stdlib test style to match.
- `internal/logging/logging.go` — already writes to a file, not stdout, so OUT-03
  is nearly free. `LogPath()` (:44) is what D-24's record references.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **The Phase 2 envelope is the JSON contract, already built.** `Result[T]` in
  `internal/ado/result.go` carries `items` / `failures` / `complete` with
  snake_case tags, and `Failure.MarshalJSON` already flattens the error to a plain
  string with a documented no-leak constraint. D-11 serializes it as-is with zero
  new marshalling code.
- **`classifyFailure` already made the exit-code mapping mechanical.** The four
  `FailureKind` constants exist, and the classifier deliberately puts
  `context.Canceled` before the timeout branch (`result.go:135`) with a comment
  naming CLI-04 as the reason. D-07's precedence rule is a `switch` over a field,
  not string matching.
- **All eight reads are already `ctx`-first and return the envelope.** No `ado`
  signature changes for the CLI. Verified: every `GetX` on `*Client` takes
  `ctx context.Context` and returns `(Result[T], error)`.
- **`logging.Init` already writes to a file rather than stdout**, so OUT-03's
  stdout discipline is mostly inherited rather than built. `LogPath()` gives D-24
  its reference string.
- **The Phase 1 fakes and struct-literal injection seam** (`internal/ado/fakes_test.go`,
  `&Client{workitemClient: fake, projects: []string{"P"}}`) let CLI-layer tests
  drive every failure path with no network and no new test infrastructure.
- **`internal/utils/links.go`** — six pure URL builders, directly useful if the CLI
  emits links for an agent to follow.

### Established Patterns

- **Zero argument handling exists.** No `flag` import, no cobra, no `os.Args`
  anywhere in 30 Go files. D-19 introduces the first, which means there is no
  existing convention to conflict with — and none to lean on either.
- **`main()` is one unconditional path** from `logging.Init` through `config.Load`,
  `cfg.Validate`, `ado.NewClient`, `config.LoadPriorities`, `ui.NewModel`,
  `p.Run()`. The dispatch branch is an insertion, not a restructure.
- **Errors already go to stderr via `fmt.Fprintf(os.Stderr, ...)`** at all four
  failure sites in `main.go`. D-13's stdout/stderr split extends an existing habit.
- **Filtering is fused into `buildRows()` returning `[]table.Row`** in every view.
  There is no existing pure predicate to lift — D-16 is extraction, not relocation,
  which is why the behavior-equivalence tests matter.
- **`internal/ado` has no display formatting and no TUI imports**, verified in
  ARCHITECTURE.md. D-15 puts pure filter predicates there, which does not break
  that property, but does add a concern the package did not previously hold.

### Integration Points

- **`main.go:59`** is the exact dispatch point. `ui.NewModel` eagerly constructs
  six sub-models, six `bubble-table` instances and several `textinput` widgets
  (`app.go:53-67`), so a CLI invocation must branch before this line or pay the
  full TUI construction cost for a `--json` read.
- **`ado.NewClient(cfg)`** is shared by both paths and performs network I/O in the
  constructor (`GetConnectionData`). Its failure is D-08's exit 3.
- **`fetchctx.New`** is the TUI's fan-out budget helper and wraps
  `context.Background()` internally by design (the package comment states no view
  file should mention `context.Background()` on a read path). The CLI needs a
  cancellable parent for SIGINT, so this is a real integration seam.
- **`config.Config`** already carries `PollInterval`, `RequestTimeout` and
  `FetchTimeout` as `time.Duration` fields with YAML tags and defaults applied in
  `Load()`. `--timeout` overrides one or both; the flag should parse as a duration.
- **The six view packages hold `client *ado.Client` concretely** and each declares
  its own message types. D-16 touches three of them (`workitems`, `prs`,
  `branches`) — the ones with filters — not all six.

</code_context>

<specifics>
## Specific Ideas

- The framing that drove nearly every decision in this phase is PROJECT.md's core
  value applied one layer out: *an agent must never mistake a broken call for an
  empty result — and that has to hold at the exit code, at the filter defaults, and
  at the crash record, not just in the payload.* D-05 (partial exits non-zero),
  D-18 (CLI defaults to no filtering), D-23 (crash record on stdout) and D-26
  (distinct error shape) are all the same sentence applied to different surfaces.
  Worth carrying into the plan's acceptance criteria verbatim.
- D-07's "auth wins" is deliberately asymmetric and should not be softened into a
  most-severe-numeric rule later. The reasoning is specific: auth is the one
  failure kind that is definitely not transient and definitely requires a human,
  so masking it behind a co-occurring timeout costs an operator real time.
- D-24 is a security decision wearing an ergonomics disguise. The stack trace was
  excluded because Go prints function arguments and `ado.NewClient(cfg)` takes the
  PAT — this is the same T-02-01 class Phase 2 wrote into `result.go`'s doc
  comment, and the plan should not quietly add the stack back for debuggability.
- The `items: null` hazard flagged under the agent's Discretion is the single most
  likely way this phase ships a core-value violation. It is a one-line fix and a
  one-line test, and it will be invisible until an agent hits an empty project.

</specifics>

<deferred>
## Deferred Ideas

- **`config.Priorities` sort and HTML description stripping** — the remaining two
  TUI-coupled capabilities from ARCHITECTURE.md §"TUI-coupled" that D-17 did not
  promote. The sort is display ordering, and JSON output ordering is a separate
  decision; HTML stripping only matters once descriptions are emitted. Revisit if a
  CLI consumer needs either.
- **Development-task selection, User Story resolve-with-effort, PR merge
  confirmation, pipeline trigger** — the remaining rows in ARCHITECTURE.md's
  TUI-coupled table. All are write-path or composite-workflow concerns and belong
  to Phase 4 (CLI-03), not here.
- **Write subcommands** — CLI-03, Phase 4. D-01's noun-verb tree exists specifically
  so `prs approve`, `pipelines trigger`, `releases approve` and the rest land
  without restructuring.
- **Exit codes 4+** — reserved by D-06 for Phase 4 write outcomes (dry-run
  produced-no-change, confirmation-refused, and similar).
- **A fifth exit code for usage errors** — offered under D-22 and rejected in favor
  of folding usage errors into outcome-unknown. Add it only if the conflation
  proves confusing in practice.
- **`--pretty` JSON output** — offered under D-14 and rejected; `jq` covers it.
  Revisit only if a consumer cannot pipe.
- **Invocation metadata in the JSON envelope** (`command`, `generated_at`) —
  offered under D-11 and rejected as speculative. Revisit if output provenance
  becomes a real need, e.g. if results get logged and replayed.
- **A shared `internal/filter` package between `ado` and `ui`** — offered under
  D-15 and rejected in favor of `internal/ado`. The first lever if the 300-line
  problem recorded under D-15 becomes acute.
- **Making a failed identity fetch fatal for non-interactive callers** — carried
  forward from Phase 1's deferred list, which named Phase 3 as its proper home.
  Not opened in this discussion; the `--assigned-to-me` empty-cache question under
  the agent's Discretion is the narrow slice of it that this phase must answer.
- **Rate-limit backoff and 429 handling** — PLAT-03. The `rate_limit` kind makes
  throttling visible and D-06 gives it an exit code, but nothing handles it.
- **N+1 fan-out restructuring and `Top` caps** — CONCERNS.md MED3. Makes `--timeout`
  more likely to fire; fixing it is not in CLI-04.
- **CI** — Phase 5. Note there is no CI, no Makefile and no linter beyond `go vet`
  today, so the exit-code contract in D-06 has no automated consumer yet.
- **Promoting the Phase 1 fakes to an exported package** — still deferred to Phase 4,
  unchanged by this phase.

</deferred>

---

*Phase: 3-Read CLI*
*Context gathered: 2026-09-08*
