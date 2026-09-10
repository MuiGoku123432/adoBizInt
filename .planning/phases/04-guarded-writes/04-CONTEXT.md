# Phase 4: Guarded writes - Context

**Gathered:** 2026-09-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Add write subcommands to the Phase 3 noun-verb CLI covering the seven existing
`internal/ado` mutations, each behind a dry-run default that reports a real
before/after diff, an explicit `--confirm` flag, an append-only audit record per
executed write, nullable effort fields so an omitted value leaves ADO unchanged,
and an outcome-unknown classification distinct from both success and "nothing
ran".

**The seven mutations and their target verbs:**

| `ado` method | CLI verb |
|---|---|
| `UpdateWorkItemState` | `workitems set-state` |
| `UpdateWorkItemStateWithEffort` | `workitems set-state` (same verb, optional effort flags — D-16) |
| `ApprovePullRequest` | `prs approve` |
| `CompletePullRequest` | `prs complete` |
| `ApproveRelease` | `releases approve` |
| `RejectRelease` | `releases reject` |
| `QueuePipeline` | `pipelines queue` |

**In scope:** write verbs on the existing nouns; the shared dry-run /
`--confirm` contract; by-id pre-reads for the write targets that lack one today;
a new top-level write envelope for `--json`; new reserved exit codes in the 4+
range; the JSONL audit sink and its pre-flight writability check; nullable
effort parameters in `internal/ado` and the three TUI call sites they touch;
outcome-unknown handling for timeout and mid-write SIGINT; tests for every guard
using the Phase 1 fake seam.

**Not in scope:** agent capability tiers (AGENT-01 through AGENT-04 — v2, and
explicitly a different mechanism from `--confirm`, see D-04); attachment host
allow-list, path containment, WIQL escaping, nil guards, CI, log file
permissions (all Phase 5); work item creation, comments and links (v2); batched
multi-target writes (deferred, D-25); retry and rate-limit backoff (PLAT-03).

</domain>

<decisions>
## Implementation Decisions

### Dry-run semantics

- **D-01:** A dry run **fetches current ADO state and reports `current -> proposed`**,
  not a bare echo of intent. A preview that cannot tell you the change is a no-op
  or already impossible is a confident statement made without looking, which is
  the project's core failure mode one layer out from the read path. Rejected:
  echo-only (cheap and total, but blind) and opt-in pre-read (two dry-run output
  shapes to define, document and test for no gain once the diff is the default).

  **Cost the planner must surface, not absorb:** only `GetWorkItemDetail` exists
  today as a by-id read. The eight aggregate reads are project-scoped fan-outs.
  PR, release-approval and pipeline-definition by-id lookups are **new methods on
  `internal/ado`**, with new interface entries on the Phase 1 seam and new fakes.
  This is a material part of the phase's size and is easy to under-estimate as
  "just a preview".

- **D-02:** A **failed pre-read is fatal to the dry run.** Report the failure,
  exit non-zero, emit no dry-run report. A dry run that says "would set
  State=Closed" without having established the current state is exactly the
  broken-call-looking-like-a-normal-one shape Phase 2 was written to eliminate.
  Rejected: degrade-to-echo-with-a-marker (a caller scripting on `$?` alone
  scripts past the marker) and degrade-silently.

- **D-03:** The **pre-read also gates the `--confirm` path**, and a change that is
  already applied is **refused with no mutation attempted**. This makes every
  write idempotent-by-refusal and closes the "no idempotency" half of
  CONCERNS.md C2, which nothing else in this phase touches. Rejected: pre-read as
  dry-run-only advice — it leaves C2's idempotency finding open and lets a no-op
  write emit an audit record asserting a change that did not happen.

  **Known limit the plan should state rather than hide:** a read-then-write pair
  has a race window. This narrows the failure, it does not eliminate it, and the
  audit record's `from` value is a claim about the moment of the pre-read.

- **D-04:** `--confirm` is an **accident guard, not an authorization boundary.**
  REQUIREMENTS.md AGENT-01 through AGENT-04 (v2) already record that an agent
  passing `--confirm` is using the flag exactly as designed, so authorization
  needs a capability tier instead. Nothing in this phase should be built as if
  `--confirm` restrains intent.

### Confirmation and exit codes

- **D-05:** The flag is **`--confirm`**. Matches SAFE-01's own wording, so
  traceability stays mechanical. Rejected: `--execute` (diverges from the
  requirement text) and `--yes` (implies a prompt that does not exist, which
  misleads an agent reading `--help`).

- **D-06:** **Writes never prompt interactively.** `--confirm` is the entire
  mechanism, identical on and off a TTY. This extends Phase 3 D-14's reasoning
  from output shape to behaviour: anything that changes based on invocation
  context is a works-in-my-shell, breaks-in-CI bug, and a prompt additionally
  introduces a hang risk when TTY detection is wrong.
  — **Reversibility:** costly — adding a prompt later changes the behaviour of
  every existing non-`--confirm` invocation in a caller's scripts.

- **D-07:** **A completed dry run exits 0.** It did precisely what it was asked
  to do; nothing failed. Reserving a non-zero code for "you did not pass
  `--confirm`" punishes correct usage and aborts every preview under `set -e`.
  The `executed` / `outcome` fields in the write envelope carry the distinction,
  and the caller already knows whether it passed the flag.

- **D-08:** **A refused no-op gets its own reserved 4+ exit code** meaning
  "already in the requested state, nothing written", on both the dry-run and
  `--confirm` paths. Rejected: exit 0 idempotent-success (collapses "I changed
  it" and "it was already so", the exact conflation this project exists to
  prevent) and reusing exit 1 (which means "the read did not reach every
  project" — overloading it restores the ambiguity Phase 3 D-08 deliberately
  cleared).

- **D-09:** **Outcome-unknown gets a second reserved 4+ code, distinct from
  exit 3.** Exit 3 means "the read never ran — no data, nothing tried" (Phase 3
  D-08). A write that timed out mid-flight is the opposite: something was tried
  and production may have changed. Telling a caller "nothing happened" when the
  truth is "something might have" is the most consequential possible instance of
  this project's core failure mode. Rejected: reusing exit 3 (an agent could not
  distinguish "safe to retry blindly" from "check before retrying") and a
  distinct code per unknown-outcome cause (proliferation in a contract Phase 3
  kept deliberately small).
  — **Reversibility:** one-way — these integers join the published contract
  Phase 3 D-06 established; changing them silently breaks callers still running.

  **The specific integers are the planner's to assign** within the 4+ range
  Phase 3 D-06 reserved. The constraint from this discussion is that there are
  exactly two new codes, they are distinct from each other, and both are distinct
  from 0/1/2/3.

### Write output shape

- **D-10:** `--json` emits a **new top-level write envelope, distinct from the
  Phase 2 `Result[T]`** — shape along the lines of
  `{"write":{"command":...,"target":{...},"changes":[{"field":...,"from":...,"to":...}],"executed":false,"outcome":"dry_run"}}`.
  An agent distinguishes a write report from a result set by which top-level key
  is present, with no chance of reading a write as a one-row read. This is the
  same reasoning that produced Phase 3 D-26's distinct crash-record shape.
  Rejected: reusing `Result[T]` (`complete` and `failures` mean something
  different for a write, and `.items | length` would report a write as a row).
  — **Reversibility:** one-way — this is the parse target for every downstream
  agent driving a write.

- **D-11:** **Dry run and executed write share one envelope shape**, differing
  only in the `executed` / `outcome` fields. A caller parses one shape and reads
  a field, rather than branching on which of two documents it received. Rejected:
  separate preview and receipt shapes.

### Audit record

- **D-12:** The audit sink is a **dedicated append-only JSONL file** (e.g.
  `~/.config/adoBizInt/audit.jsonl`), one object per executed write — not the
  shared application log. An audit trail that a raised log level or a rotation
  can silence is not an audit trail. Rejected: `logging.Logger()` into
  `adoBizInt.log` (records interleave with TUI debug output and are
  config-silenceable) and treating the stdout envelope as the record (SAFE-02
  says "appends", and a caller discarding stdout leaves no trace).

  Note for Phase 5: HARD-06 already plans `0600` on the log file and `0700` on
  its directory. An audit file in the same directory inherits that work — the
  plan should place it accordingly rather than inventing a second location.

- **D-13:** **A missing or unwritable audit sink is fatal, and it is checked
  BEFORE the ADO call.** No audit sink, no mutation. This is what turns SAFE-02
  from a best effort into a guarantee: there is no window in which ADO changed
  and nothing recorded it. Rejected: write-then-append with a loud warning (the
  mutation has already happened, so the trail has a hole exactly when something
  went wrong) and non-fatal warning.

  **Concrete plan consequence:** the write path opens the audit file for append
  as a pre-flight step, before the pre-read result is acted on.

- **D-14:** **Only executed writes are audited.** Dry runs and refused no-ops
  produce no record — the file answers "what changed in ADO", and neither
  changed anything. Rejected: auditing every invocation tagged with outcome
  (volume, and it redefines "audit record" as a record of intent).

- **D-15:** An audit record carries **timestamp, command, target identity
  (project / repo / item or PR / release / definition id), the field-level
  `from -> to` captured by the pre-read, and the outcome**. Actor identity is
  omitted — there is one identity, the PAT's, and Phase 1 D-12 makes its
  resolution non-fatal so the field can legitimately be empty. Full argv is
  omitted deliberately: argv is caller-controlled and could carry a secret
  someone passed on a command line, which is a credential-disclosure path into a
  persisted file — the same class Phase 3 D-24 closed for stack traces.

- **D-16 (see also D-22):** An **outcome-unknown write IS audited**, with the
  outcome field marking it unknown. The case where a mutation may or may not have
  landed is precisely the case you most want a trail for. This is the one
  exception to D-14's "executed writes only", and it is deliberate: D-14 excludes
  writes known not to have happened, not writes whose outcome is unresolved.

### Nullable effort fields

- **D-17:** The fix lands **at the `internal/ado` signature** — nullable effort
  parameters (`*float64`, or a struct with pointer fields), with nil fields
  omitted from the JSON patch document. `UpdateWorkItemStateWithEffort`
  (`workitems.go:284`) currently sends all three fields with `Add` on every call,
  so **the TUI blind-overwrites too**; this is not a CLI-layer bug and fixing it
  at the CLI would leave the footgun loaded for every future caller. Rejected:
  filling omitted fields from the pre-read in the CLI (leaves the overwrite in
  `internal/ado`, adds a read-modify-write race, and makes "omitted means
  unchanged" a CLI convention rather than a domain guarantee) and adding a second
  method alongside the existing one (two methods for one concept, one of them a
  known footgun the TUI keeps calling — the two-definitions drift Phase 3 D-16
  eliminated for filters).
  — **Reversibility:** costly — undoing means reverting the signature and
  re-touching the TUI call sites.

  **Call sites the plan must update:** `internal/ui/workitems/workitems.go:266`,
  `:333`, `:347`, `:354`. CLI-01 forbids behaviour change, not signature change,
  and the TUI passes all three values explicitly today, so passing pointers to
  the same values preserves its behaviour exactly. Phase 1's exact-call tests on
  this method are the regression guard.

- **D-18:** The CLI distinguishes omitted from zero via **`fs.Visit` flag
  presence**, so `--remaining 0` is a real zero. Closing a task legitimately
  means setting RemainingWork to 0, so a mechanism that cannot express zero would
  break the primary use case. Rejected: a sentinel default (a sentinel is a
  value, and any real value colliding with it silently means something else — the
  same conflation as float64 zero, just relocated) and a custom nullable-float
  `flag.Value` type (viable, and the codebase already has `stringList` as
  precedent, but `fs.Visit` is stdlib's own answer to this question).

- **D-19:** **One verb: `workitems set-state`, with optional effort flags.** Once
  effort is nullable per D-17, `UpdateWorkItemState` is literally the same call
  with three nils, so the two-verb split would preserve a distinction that only
  exists because of the bug being fixed. Rejected: `set-state` plus
  `set-state-with-effort` (two verbs for one concept, and a caller must know the
  second exists to find it) and separate `set-state` / `set-effort` verbs (a Task
  closing needs state and effort in one atomic patch, so this would need a third
  combined verb anyway).

  **Open for the planner:** whether the two `ado` methods actually merge into one
  or stay separate behind the single verb. D-19 fixes the CLI surface, not the
  package's internal shape.

### Outcome-unknown

- **D-20:** **No verify-read after a timeout.** Report unknown and let the caller
  decide. Rejected: an automatic verify-read on a fresh deadline (it extends
  wall-clock past the caller's own `--timeout`, can itself time out into the same
  unknown state, and a concurrent change by another actor makes the verify lie
  with full confidence — a worse failure than admitting ignorance) and an opt-in
  verify flag.

- **D-21:** See D-09 — outcome-unknown carries its own reserved 4+ code, not
  exit 3.

- **D-22:** **A mid-write SIGINT follows the outcome-unknown path**, not Phase 3
  D-25's exit 3 plus crash record. D-25 was written for reads, where an interrupt
  means nothing mutated; a write already in flight is a materially different
  event and must not inherit a code that asserts "nothing ran". The recorded
  cause distinguishes interrupt from timeout; the process-level classification is
  the same.

  **Supersedes** Phase 3 D-25 for write verbs only. Read verbs keep exit 3 and
  the crash record unchanged.

### Write command surface

- **D-23:** The target is a **positional argument** — `prs approve 4821`,
  `workitems set-state 4821 --state Closed`. A missing positional is a parse
  error; a missing `--id` defaults to zero and could plausibly be issued.
  Rejected: `--id` flag form and accepting both spellings (two spellings for one
  input means agent-generated scripts diverge and `--help` documents both, which
  is what Phase 3 D-04 rejected aliases for).
  — **Reversibility:** one-way — the command shape is the published surface.

- **D-24:** **Project and repository reuse the `--project` / `--repo` names with
  an arity of exactly one enforced.** Zero or multiple values is a parse error
  (nothing attempted). Same vocabulary as the read verbs, stricter arity where
  fan-out is meaningless. This also removes the read path's "empty means the
  configured list" fallback for writes — silently picking a project the caller
  did not name is unacceptable for a mutation. Rejected: additional bare
  positionals (`prs approve <project> <repo> <id>` is order-sensitive and
  transposing project and repo on an approve has no type system to catch it) and
  new singular flag names (two names for one concept across read and write
  verbs).

- **D-25:** **One target per invocation.** Multiple positional ids is a parse
  error. One invocation, one mutation, one audit record, one exit code. Batching
  would force a partial-failure exit rule the code contract has no clean answer
  for, would require a per-target outcome shape and an atomic N-change dry-run
  preview, and would multiply the blast radius of a single mistyped `--confirm` —
  all in the phase whose subject is safety. A caller wanting three writes runs
  three commands and gets three independent outcomes.

- **D-26:** **`--state` is required** on `workitems set-state`; the CLI never
  consults `GetNextState` or `config.StateTransitions` to pick a state. A
  config-driven default means the same command produces different mutations on
  two machines, which Phase 3 D-18 rejected for filters and which matters more
  for a write. It also makes D-01's `current -> proposed` diff meaningful, since
  "proposed" came from the caller. `GetNextState` remains a TUI convenience.

### the agent's Discretion

The user answered every question directly and delegated no decision. The
following fell out of decisions above without being opened as areas, and the
planner should settle or escalate them:

- **The actual integers for the two new exit codes** (D-08, D-09) within the 4+
  range. Constraint: exactly two, distinct from each other and from 0/1/2/3.
- **Verb names for the seven mutations.** The table in `<domain>` is this
  discussion's working set, not a ratified decision. `prs complete` vs
  `prs merge` in particular is worth a deliberate choice — the `ado` method is
  `CompletePullRequest` but "merge" is what a user would search for. Whatever is
  chosen is one-way per Phase 3 D-01's reasoning.
- **Whether `releases approve` / `releases reject` expose `--comment`.** Both
  `ado` methods take a comment parameter. Omitting it means passing empty; the
  flag is nearly free and the comment is likely audit-relevant.
- **`QueuePipeline`'s `--branch`.** Required or defaulted, and whether an omitted
  branch is a parse error or the definition's default.
- **Whether the two effort `ado` methods merge into one** (see D-19).
- **Where the new by-id read methods live** given D-01. `internal/ado` is already
  2,015 lines against CONVENTIONS.md's 300-line standard and Phase 3 D-15 already
  recorded this cost once. Adding four by-id reads makes it worse; the plan
  should state the file placement rather than scatter them.
- **Whether `--confirm` is registered on read verbs.** Phase 3 D-20 registers
  globals into every FlagSet by a shared function. `--confirm` is meaningless on
  a read and, per that decision's own reasoning, a flag that parses but cannot
  mean anything is worse than one that does not exist — so it likely belongs to a
  write-specific registration function, mirroring `filterSupportByCommand`.
- **What a rejected write exits with** — ADO accepted the request and returned an
  error (409 conflict, permission denied on the specific item). Distinct from
  auth (exit 2), from unknown (D-09) and from a refused no-op (D-08). Probably
  exit 1, but exit 1 currently means "partial read" and nothing has assigned it a
  write meaning.
- **Whether the write envelope's `changes` array is empty or absent** when the
  pre-read shows a no-op, and what `outcome` value D-08's refusal carries.
- **Audit file rotation and unbounded growth.** D-12 chose append-only with no
  rotation discussed.
- **Whether the pre-read runs under the same `--timeout` budget as the write**,
  or gets its own. Two round trips now share one deadline.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Prior phase decisions — required reading

- `.planning/phases/03-read-cli/03-CONTEXT.md` — **Highest-value read for this
  phase.** D-01/D-02 (the noun-verb tree these write verbs attach to, built
  explicitly to accommodate them), D-06 (the exit-code contract and the 4+
  reservation D-08/D-09 above consume), D-08 (why exit 3 means "nothing ran",
  which is the whole argument for D-09), D-11/D-14 (JSON shape rules the write
  envelope inherits), D-13 (stdout/stderr split), D-18 (config-driven behaviour
  rejected — the basis for D-24 and D-26), D-19/D-20/D-22 (stdlib `flag`, shared
  global registration, `ContinueOnError` + exit 3 on parse failure), D-24 (the
  no-stack-trace credential rule D-15 above extends to argv), D-25 (read
  interrupt at exit 3, which D-22 above supersedes for writes), D-26 (the
  distinct crash-record shape D-10 above mirrors). Its `<deferred>` section names
  this phase as the home for the write subcommands and exit codes 4+.
- `.planning/phases/01-test-seam-for-internal-ado/01-CONTEXT.md` — the unexported
  `-API` interfaces and struct-literal injection seam every write guard test uses
  (ROADMAP SC6 makes this mandatory). D-12 records that identity resolution is
  non-fatal, which is why D-15 above omits actor identity from the audit record.
  New by-id read methods (D-01) need new interface entries and new fake methods
  here.
- `.planning/phases/02-honest-reads/02-CONTEXT.md` — the `Result[T]` envelope
  D-10 deliberately does not reuse, the four `FailureKind` values the exit mapper
  keys on, and the two timeout budgets (`request_timeout` per-call,
  `fetch_timeout` fan-out) relevant to the pre-read/write budget question in the
  agent's Discretion. T-02-01 is the no-structural-error-leak constraint the
  write envelope must also honour.

### Requirements and scope

- `.planning/REQUIREMENTS.md` §"Write Safety" SAFE-01 through SAFE-04 and
  §"CLI Surface" CLI-03. §"Traceability" maps all five to Phase 4.
  §"Agent Authorization" AGENT-01 through AGENT-04 — **v2, not this phase**, and
  the source of D-04's framing that `--confirm` is not an authorization boundary.
- `.planning/ROADMAP.md` §"Phase 4: Guarded writes" — goal, the six success
  criteria, and the four provisional plan titles (04-01 through 04-04). Note SC1,
  SC2 and SC5 are each materially expanded by D-01, D-03 and D-09 respectively.
- `.planning/PROJECT.md` §"Core Value" — "An agent driving this tool must never
  mistake a broken call for an empty result." D-01, D-02, D-08, D-09 and D-22
  each trace directly to it. §"Constraints" — no new dependencies without
  justification, TUI behaviour unchanged (relevant to D-17's TUI call-site
  changes), files under 300 lines.
- `.planning/STATE.md` §"Pending Todos" — **the open acceptance-criteria-style
  todo is addressed to this phase's planning.** Three Phase 3 incidents where a
  `grep returns nothing` criterion was satisfied and then defeated by a prose
  comment explaining the forbidden thing. Prefer structural assertions (the
  `newFlagSet` + `TestFlagsRegisteredOnEverySubcommand` shape); where a textual
  check is unavoidable, scope it with `--include` and strip comment lines first.

### Codebase analysis

- `.planning/codebase/CONCERNS.md` — **C2 is this phase's reason to exist**: seven
  destructive writes with no dry-run, no idempotency, no audit, and a TUI-only
  confirmation at `internal/ui/prs/prs.go:69` that a CLI bypasses entirely. D-03
  is the only decision here that addresses C2's idempotency half.
- `.planning/codebase/CONVENTIONS.md` — rule 5 (no abstraction unless required),
  rule 6 (justify dependencies), and the 300-line file standard the new by-id
  reads run against in an already-2,015-line package.
- `.planning/codebase/TESTING.md` — table-driven stdlib style, no testify, no
  mock generator.
- `.planning/codebase/ARCHITECTURE.md` §"TUI-coupled — must be extracted before
  reuse" — the rows Phase 3 deferred here: Development-task selection, User Story
  resolve-with-effort, PR merge confirmation, pipeline trigger.
- `.planning/codebase/INTEGRATIONS.md` §"Complete Method Inventory" — the SDK
  calls each mutation issues.

### Source files this phase modifies

- `cmd/adoBizInt/dispatch.go` — the `commands` map gains seven write entries;
  `readClient` gains a sibling write interface (or is extended). `runRead` is the
  structural model for a `runWrite` sibling. `exitOK`/`exitPartial`/`exitAuth`/
  `exitUnknown` are declared here and gain the two 4+ codes.
- `cmd/adoBizInt/exit.go` — `exitCodeFor` maps read outcomes; the write path needs
  its own mapper, not an overload of this one.
- `cmd/adoBizInt/flags.go` — `newFlagSet`, `globalFlags`, `filterSupportByCommand`,
  and `stringList`. `--confirm` and the effort flags register here; D-18's
  `fs.Visit` presence check and D-24's arity-of-one rule live at this layer.
- `cmd/adoBizInt/usage.go` — three-level `--help` (Phase 3 D-21) gains seven verbs
  with their own dry-run and confirmation flags.
- `internal/ado/workitems.go:255` (`UpdateWorkItemState`), `:284`
  (`UpdateWorkItemStateWithEffort` — the three `Add` operations at `:296-318` are
  the D-17 blind-overwrite), `:331` (`GetWorkItemDetail`, the one existing by-id
  read).
- `internal/ado/pullrequests.go:167` (`ApprovePullRequest`), `:208`
  (`CompletePullRequest`).
- `internal/ado/releases.go:302` (`ApproveRelease`), `:327` (`RejectRelease`).
- `internal/ado/pipelines.go:391` (`QueuePipeline`).
- `internal/ui/workitems/workitems.go:266`, `:333`, `:347`, `:354` — the four
  effort/state call sites D-17 changes.
- `internal/ado/result.go` — `Failure`, `FailureKind`, `classifyFailure` at `:135`
  (cancellation before timeout, which D-22 depends on for distinguishing an
  interrupt from a server timeout).
- `internal/logging/logging.go` — `LogPath()` at `:44`; D-12's audit file sits
  beside it and inherits Phase 5 HARD-06's permission work.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **The Phase 3 dispatch layer is the template.** `runRead` in `dispatch.go` is
  one generic body — parse, fetch, filter, emit, map to exit code — that eight
  verbs share by differing only in fetch func and renderer. A `runWrite` sibling
  with parse, pre-read, diff, confirm-gate, audit, mutate, emit is the same
  shape, and `commandFunc` / the `commands` map need no structural change.
- **`newFlagSet` is already the single `flag.NewFlagSet` site**, which Phase 3
  D-20 established specifically so global-flag registration cannot drift across
  subcommands. Write flags extend it rather than introducing a second path, and
  `TestFlagsRegisteredOnEverySubcommand` already exists as the structural guard.
- **`stringList` is an existing custom `flag.Value`** in `flags.go`, so there is
  precedent if D-18's `fs.Visit` approach turns out to need a typed alternative.
- **The Phase 1 fake seam drives every guard with no network.**
  `&Client{workitemClient: fake, ...}` covers the four write clients already;
  D-01's new by-id reads extend the same interfaces and fakes.
- **`GetWorkItemDetail` (`workitems.go:331`) is the one by-id read that exists**,
  and it already expands Relations. It is the model for the three that D-01 adds.
- **`stdoutWriter` / `stderrWriter` are package-level `io.Writer` vars** swapped
  by tests, so stdout purity and audit-vs-stdout separation are assertable
  without spawning a process.
- **`emitJSON` marshals to a buffer and writes once** (Phase 3 D-P4), so a crash
  mid-encode cannot leave a partial document. The write envelope must use the
  same path, not a streaming encoder.

### Established Patterns

- **Every `ado` write returns bare `error`, not `Result[T]`.** There is no
  failure list and no completeness flag on the write path, which is why D-10's
  new envelope is built rather than reused — this is a structural fact, not a
  preference.
- **All four writes take `ctx` first**, so `fetchctx.NewWithParent` (added in
  Phase 3) already gives the write path a cancellable, deadline-bounded context
  with signal handling attached.
- **`UpdateWorkItemStateWithEffort` builds a `[]webapi.JsonPatchOperation` with
  `Add` for all four paths unconditionally.** Omitting a nil field is a matter of
  not appending its operation — D-17 is a small change to a well-isolated
  document builder, not a rewrite.
- **Errors go to stderr via `fmt.Fprintf(stderrWriter, ...)`** at every failure
  site; the audit file is a third sink and the plan must keep all three distinct.
- **`internal/ado` has no display formatting and no TUI imports.** The audit
  record's serialization is a CLI-layer concern and should not be pushed into the
  package.

### Integration Points

- **`internal/ui/prs/prs.go:69`** holds the only confirmation that exists today,
  and it is TUI-only — CONCERNS.md C2's specific finding is that a CLI bypasses
  it. The CLI guard is new construction, not a lift.
- **`config.StateTransitions` (`internal/config/config.go:53`)** and
  `GetNextState` encode the configured flow. D-26 keeps the CLI out of them; the
  TUI keeps using them.
- **`fetchctx.NewWithParent`** already exists from Phase 3 and is what the write
  path derives its deadline from. The pre-read and the write share this parent
  unless the planner separates them (see the agent's Discretion).
- **`logging.Init` / `LogPath()`** — the audit file's directory. Phase 5 HARD-06
  will set `0600`/`0700` here; D-12's file placement should anticipate it.
- **The four write-capable sub-client interfaces from Phase 1** gain by-id read
  methods under D-01, which means both the interfaces and the fakes change — a
  cross-cutting edit the plan should sequence before the verbs that depend on it.

</code_context>

<specifics>
## Specific Ideas

- The organising principle across every decision here is the project's core value
  applied to mutation: **an agent must never mistake "it might have changed" for
  "nothing happened", nor "it was already so" for "I changed it".** D-08 (no-op
  gets its own code), D-09 (outcome-unknown distinct from never-ran), D-14/D-16
  (audit the unknown, not the no-op) and D-22 (write interrupt is not a read
  interrupt) are the same sentence applied to four surfaces. Worth carrying into
  the plan's acceptance criteria verbatim, the way Phase 3 carried the read
  equivalent.
- **D-03 is the only thing in this phase that touches C2's idempotency finding.**
  SAFE-01 through SAFE-04 cover dry-run, audit, effort and timeout; nothing in
  the requirement text addresses "the same write issued twice". The pre-read gate
  is what closes it, and it should be recognised as load-bearing rather than as a
  convenience that falls out of the dry-run diff.
- **D-13's ordering is the whole decision.** "Audit is fatal" is weak; "audit
  sink is verified writable before the ADO call is issued" is what makes SAFE-02
  a guarantee. If the plan implements the check after the write, the decision has
  been silently reversed while appearing to be honoured.
- **D-17 fixes a live TUI bug, not just a CLI gap.** The blind-overwrite at
  `workitems.go:296-318` affects the daily-use TUI today. That framing should
  survive into the plan, because it changes the risk calculus on touching the
  three TUI call sites — this is a fix propagating outward, not churn imposed on
  working code for the CLI's benefit.
- **D-01's cost is the most likely under-estimate in this phase.** "Dry run shows
  a diff" sounds like formatting; it is three new `ado` methods, three new
  interface entries, three new fake methods, and a 2,015-line package getting
  larger. The planner should size it as such.

</specifics>

<deferred>
## Deferred Ideas

- **Agent capability tiers (AGENT-01 through AGENT-04)** — v2. Explicitly a
  different mechanism from `--confirm` per D-04; REQUIREMENTS.md already records
  that state changes, PR merge, release approval and pipeline queue are
  human-only at any confirmation level. Do not build toward it here.
- **Batched multi-target writes** — rejected under D-25 for this phase. Revisit
  only if the round trips prove painful in practice; it needs a partial-failure
  exit rule and a per-target outcome shape first.
- **An automatic verify-read after a timeout** — offered and rejected under D-20.
  Revisit if outcome-unknown proves too common to be actionable.
- **Interactive confirmation prompts** — rejected under D-06. Revisit only if a
  human-at-a-terminal complaint materialises, and note it would change existing
  invocation behaviour.
- **Audit file rotation, retention and size caps** — not discussed. D-12 chose
  append-only. Revisit when the file grows or when someone needs to query it.
- **Auditing dry runs and refused no-ops** — offered and rejected under D-14. The
  first lever if "what did the agent try to do" becomes a question worth
  answering.
- **Actor identity in the audit record** — offered and rejected under D-15.
  Revisit if the tool is used on a shared machine or with rotating PATs.
- **Work item creation, comments and links (STORY-01 through STORY-03)** — v2.
  Extends the ADO surface rather than exposing it.
- **`config.Priorities` sort and HTML description stripping** — carried forward
  unchanged from Phase 3's deferred list.
- **Promoting the Phase 1 fakes to an exported package** — deferred to Phase 4 by
  Phase 1 and not opened in this discussion. Still deferred; the write tests live
  in-package or in `cmd/` with their own fakes, as Phase 3's did.
- **Rate-limit backoff, 429 handling and N+1 fan-out caps (PLAT-03)** — unchanged.
  Note D-01 and D-03 each add a round trip per write, which makes throttling
  marginally more reachable.
- **Attachment host allow-list, path containment, WIQL escaping, nil guards, CI,
  and log/audit file permissions** — all Phase 5 (HARD-01 through HARD-06).
- **A fifth read exit code for usage errors** — carried forward from Phase 3
  D-22's rejection, unchanged.

</deferred>

---

*Phase: 4-Guarded writes*
*Context gathered: 2026-09-09*
