# Phase 4: Guarded writes — Research

**Researched:** 2026-09-12
**Domain:** Go CLI write-path safety over the Azure DevOps SDK (dry-run, confirmation, audit, nullable patch fields, outcome-unknown)
**Confidence:** HIGH on the codebase seams read this session; MEDIUM on the three write files not in the read list (`pullrequests.go`, `releases.go`, `pipelines.go`), which are cited from 04-CONTEXT.md rather than read.

**Method note:** this run was restricted to `read` + `grep` over a fixed file list. Every claim below is tagged HIGH (I read the cited lines this session), MEDIUM (cited from an authoritative in-repo planning doc I read, but the source file itself was not opened), or LOW (inference).

---

<user_constraints>
## User Constraints (from 04-CONTEXT.md)

### Locked Decisions (verbatim summary of D-01..D-26)

- **D-01** A dry run **fetches current ADO state and reports `current -> proposed`**, not a bare echo of intent. Only `GetWorkItemDetail` exists today as a by-id read; PR, release-approval and pipeline-definition by-id lookups are **new methods on `internal/ado`**, with new interface entries and new fakes.
- **D-02** A **failed pre-read is fatal to the dry run** — report, exit non-zero, emit no dry-run report.
- **D-03** The **pre-read also gates the `--confirm` path**; a change already applied is **refused with no mutation attempted**. Known limit: read-then-write has a race window; the audit `from` value is a claim about the moment of the pre-read.
- **D-04** `--confirm` is an **accident guard, not an authorization boundary** (AGENT-01..04 are v2).
- **D-05** The flag is **`--confirm`**.
- **D-06** **Writes never prompt interactively**; identical on and off a TTY.
- **D-07** **A completed dry run exits 0.**
- **D-08** **A refused no-op gets its own reserved 4+ exit code** ("already in the requested state, nothing written"), on both paths.
- **D-09 / D-21** **Outcome-unknown gets a second reserved 4+ code, distinct from exit 3.** Exactly two new codes, distinct from each other and from 0/1/2/3. The specific integers are the planner's to assign. One-way door.
- **D-10** `--json` emits a **new top-level write envelope, distinct from `Result[T]`**: `{"write":{"command":...,"target":{...},"changes":[{"field":...,"from":...,"to":...}],"executed":false,"outcome":"dry_run"}}`. One-way door.
- **D-11** **Dry run and executed write share one envelope shape**, differing only in `executed` / `outcome`.
- **D-12** The audit sink is a **dedicated append-only JSONL file** (e.g. `~/.config/adoBizInt/audit.jsonl`), one object per executed write — not the shared application log. Place it beside the log file so Phase 5 HARD-06 permissions apply.
- **D-13** **A missing or unwritable audit sink is fatal, and it is checked BEFORE the ADO call.** The write path opens the audit file for append as a pre-flight step, before the pre-read result is acted on.
- **D-14** **Only executed writes are audited.** Dry runs and refused no-ops produce no record.
- **D-15** An audit record carries **timestamp, command, target identity, the field-level `from -> to` from the pre-read, and the outcome**. Actor identity omitted. **Full argv omitted deliberately** (credential-disclosure path).
- **D-16** An **outcome-unknown write IS audited**, marked unknown. The one exception to D-14.
- **D-17** The nullable-effort fix lands **at the `internal/ado` signature** — `*float64` or a struct with pointer fields, nil fields omitted from the patch document. Costly to reverse.
- **D-18** The CLI distinguishes omitted from zero via **`fs.Visit` flag presence**, so `--remaining 0` is a real zero.
- **D-19** **One verb: `workitems set-state`, with optional effort flags.** Whether the two `ado` methods merge is open.
- **D-20** **No verify-read after a timeout.** Report unknown and let the caller decide.
- **D-22** **A mid-write SIGINT follows the outcome-unknown path**, not Phase 3 D-25's exit 3 plus crash record. Supersedes D-25 for write verbs only.
- **D-23** The target is a **positional argument** (`prs approve 4821`). One-way door.
- **D-24** **`--project` / `--repo` reuse the read names with an arity of exactly one enforced.** Zero or multiple is a parse error. No "empty means configured list" fallback for writes.
- **D-25** **One target per invocation.** Multiple positional ids is a parse error.
- **D-26** **`--state` is required** on `workitems set-state`; the CLI never consults `GetNextState` or `config.StateTransitions`.

### the agent's Discretion (planner must settle or escalate)

The user delegated no decision. These fell out of the decisions above:

- The actual integers for the two new exit codes (D-08, D-09), within 4+.
- Verb names for the seven mutations; `prs complete` vs `prs merge` in particular.
- Whether `releases approve` / `releases reject` expose `--comment`.
- `QueuePipeline`'s `--branch`: required, defaulted, or parse error when omitted.
- Whether the two effort `ado` methods merge into one.
- Where the new by-id read methods live (`internal/ado` is already ~2,015 lines against a 300-line standard).
- Whether `--confirm` is registered on read verbs (likely a write-specific registration function mirroring `filterSupportByCommand`).
- What a rejected write (409, permission denied) exits with — distinct from auth (2), unknown (D-09), no-op (D-08). Probably 1, but 1 currently means "partial read".
- Whether the write envelope's `changes` array is empty or absent on a refused no-op, and what `outcome` value the refusal carries.
- Audit file rotation and unbounded growth.
- Whether the pre-read runs under the same `--timeout` budget as the write.

### Deferred Ideas (OUT OF SCOPE)

Agent capability tiers (AGENT-01..04); batched multi-target writes; automatic verify-read after timeout; interactive confirmation prompts; audit rotation/retention/size caps; auditing dry runs and refused no-ops; actor identity in the audit record; work item creation/comments/links (STORY-01..03); `config.Priorities` sort and HTML stripping; promoting the Phase 1 fakes to an exported package; rate-limit backoff and fan-out caps (PLAT-03); attachment host allow-list, path containment, WIQL escaping, nil guards, CI, log/audit file permissions (Phase 5, HARD-01..06); a fifth read exit code for usage errors.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-03 | Write subcommands exist for the seven existing mutations | `## CLI Surface` — `commands` map + `commandFunc` shape at `dispatch.go:79-137`; `newFlagSet` at `flags.go:103` |
| SAFE-01 | Write verbs are dry-run by default and require an explicit confirmation flag | `## Dry-Run Diff`, `## CLI Surface` — `fs.Visit` presence check, `--confirm` registration |
| SAFE-02 | Every executed write appends an audit record identifying what changed | `## Audit Record` — JSONL sink, pre-flight writability (D-13) |
| SAFE-03 | Effort updates accept omitted fields as "leave unchanged" | `## Work Item Write Semantics` — `workitems.go:295-316` blind `Add` on all four paths |
| SAFE-04 | A write that times out reports outcome-unknown | `## Existing Outcome Model` — `classifyFailure` at `result.go:146-177` |
</phase_requirements>

---

## Summary

The phase has four structurally independent pieces and one cross-cutting prerequisite. The prerequisite is D-01's new by-id reads: three new `internal/ado` methods, three new entries on the Phase 1 `-API` interfaces (`interfaces.go:17-47`), and three new fake methods (`fakes_test.go:44-242`). Everything else — the dry-run gate, the write envelope, the audit sink, nullable effort — depends on those reads existing, so they sequence first.

The CLI layer is well-shaped for this. `dispatch.go` already isolates a single generic read body (`runRead`, `dispatch.go:159-258`) that eight verbs share by differing only in fetch func, filter and renderer; a `runWrite` sibling is the same shape with pre-read/diff/confirm-gate/audit/mutate substituted for filter/render. `newFlagSet` (`flags.go:103-141`) is already the single `flag.NewFlagSet` site, with a per-command capability map (`filterSupportByCommand`, `flags.go:57-63`) that write flags can mirror exactly.

The riskiest under-estimate is D-01's cost, and the riskiest silent-failure is D-13's ordering: "audit is fatal" is weak, "audit sink verified writable before the ADO call is issued" is the guarantee. An implementation that appends after the write satisfies the word and reverses the decision.

**Primary recommendation:** Sequence as `04-01` (new by-id reads + interface/fake entries + `runWrite` skeleton + two exit codes + envelope type) → `04-04` (nullable effort at the `ado` signature, TUI call sites, regression against the existing exact-patch test) → `04-02` (seven verbs) → `04-03` (audit sink + outcome-unknown), and treat `04-01` as the largest plan, not the smallest.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| JSON-Patch document construction, nil-field omission | `internal/ado` | — | D-17 fixes at the signature; `internal/ado` owns the SDK boundary and has no display formatting (04-CONTEXT.md `<code_context>`) |
| By-id pre-reads | `internal/ado` | Phase 1 interfaces + fakes | D-01; same tier as `GetWorkItemDetail` (`workitems.go:334`) |
| Dry-run diff computation and rendering | `cmd/adoBizInt` | — | Presentation and the write envelope are CLI-layer concerns |
| `--confirm` gate, flag-presence detection | `cmd/adoBizInt/flags.go` | — | `fs.Visit` is a FlagSet-level API (D-18) |
| Exit-code mapping for writes | `cmd/adoBizInt/exit.go` | — | New mapper, not an overload of `exitCodeFor` |
| Audit sink (open, pre-flight check, append) | `cmd/adoBizInt` | `internal/logging` for the directory | D-12 places it beside `LogPath()`; serialization is CLI-layer |
| Outcome classification (timeout / cancel / 5xx) | `internal/ado` (`classifyFailure`) | `cmd` write mapper | `result.go:146` already classifies structurally |

---

## Existing Seam

**The Phase 1 seam is four unexported interfaces plus struct-literal injection.** `interfaces.go:17-47` declares `workItemAPI` (5 methods), `gitAPI` (7), `buildAPI` (4), `releaseAPI` (4). Each declares only methods this package calls — adding an uncalled method violates ROADMAP SC4 (`interfaces.go:12-16`). **HIGH.**

**`Client` holds them as unexported fields**, so fakes are injected by struct literal from inside package `ado` only: `connection`, `workitemClient workItemAPI`, `workClient work.Client`, `gitClient gitAPI`, `buildClient buildAPI`, `releaseClient releaseAPI` (`client.go:26-43`). **HIGH.** This is why `dispatch.go:48-58` declares its own consumer-side `readClient` interface rather than importing a fake: "the Phase 1 fake seam uses unexported Client fields and therefore cannot be constructed from this package" (`dispatch.go:50-52`). **HIGH.**

**The fake shape is func-field-per-method with a nil-guard `t.Fatalf`.** Example (`fakes_test.go:54-60`):

```go
func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}
```

**HIGH.** Compile-time conformance is pinned by `var _ workItemAPI = (*fakeWorkItemAPI)(nil)` and three siblings (`fakes_test.go:14-19`).

### How to assert "no mutating call was made" — SC2's core assertion

**This is already free.** Because every fake method fails the test when its func field is nil, a dry-run test constructs the fake with **only the read func set and the mutating func left nil**. If the dry-run path issues the mutation, `f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")` fires and the test fails with a precise message. **HIGH** — this is the exact mechanism at `fakes_test.go:56-58`, and it is a structural assertion, not a prose one, which satisfies the STATE.md "Pending Todos" preference cited in 04-CONTEXT.md's canonical refs.

Prefer this over a call-counter. A counter asserts `== 0`; a nil func field makes the call *impossible to make silently*. Where a counter is still wanted (e.g. asserting exactly one write on the `--confirm` path), the existing shape is a captured `var got workitemtracking.UpdateWorkItemArgs` inside the closure plus `reflect.DeepEqual` against a literal `want` (`workitems_test.go:306-339`). **HIGH.**

**Error injection helpers already exist** for the two SDK error shapes: `wrappedErrorValue(status, msg)`, `wrappedErrorPointer(status, msg)`, `wrappedErrorNoStatus(msg)` (`fakes_test.go:28-42`). The two shapes exist because `UnwrapError` returns a pointer on two paths and a value on two others (`fakes_test.go:21-24`, mirrored at `result.go:179-186`). **HIGH.** A write test injecting a 409 or a 403 should use these rather than `errors.New`.

**Existing exact-call write tests to extend or mirror:** `TestUpdateWorkItemStateIssuesExactPatch` (`workitems_test.go:306`), `TestUpdateWorkItemStateWithEffortIssuesExactPatch` (`:341`), `TestApprovePullRequestIssuesExactReviewerCall` (`pullrequests_test.go:16`), `TestCompletePullRequestIssuesFetchThenMerge` (`:165`), `TestApproveReleaseIssuesExactApproval` (`releases_test.go:13`), `TestRejectReleaseIssuesExactApproval` (`:47`), `TestQueuePipelineIssuesExactQueueCall` (`pipelines_test.go:13`). Seven methods, seven tests — SEAM-03 is satisfied. **HIGH** (grep-confirmed test names; bodies read only for the two workitems ones).

**Consequence for D-01:** each new by-id read needs (a) a method on the matching `-API` interface in `interfaces.go`, (b) a `XxxFunc` field + guarded method on the matching fake in `fakes_test.go`, (c) the `ado` wrapper method. `gitAPI.GetPullRequest` already exists (`interfaces.go:28`) with a fake (`fakes_test.go:122`) — **so the PR pre-read may need no new interface entry at all**, only a new `ado`-level wrapper. `releaseAPI.GetApprovals` (`interfaces.go:43`) and `buildAPI.GetDefinition` (`interfaces.go:37`) likewise already exist. **HIGH on the interface entries existing; MEDIUM on whether they are sufficient for the by-id lookups** — `GetApprovals` is a list call, so a by-id approval lookup may need `GetApproval` added.

> **This materially lowers D-01's estimated cost.** 04-CONTEXT.md D-01 says "new interface entries on the Phase 1 seam and new fakes"; three of the four underlying SDK methods are already on the interfaces. Planner should verify each against the SDK rather than assuming three new entries.

---

## Existing Outcome Model

### Read-side classification (`internal/ado/result.go`)

`FailureKind` is a **closed set of four** — `FailureAuth` ("auth"), `FailureTimeout` ("timeout"), `FailureRateLimit` ("rate_limit"), `FailureOther` ("other") (`result.go:23-36`). A caller may switch exhaustively. **HIGH.**

`classifyFailure` (`result.go:146-177`) classifies **structurally, never by error text** — the SDK message is server-controlled, localisable, and can be an HTML fragment, so text matching is both fragile and a spoofing surface (`result.go:142-145`). Order matters and is load-bearing for Phase 4:

1. `errors.Is(err, context.Canceled)` → **`FailureOther`**, deliberately falling through, so a user-initiated cancel is never reported as a server timeout (`result.go:152-156`). **HIGH.**
2. `errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())` → `FailureTimeout`. Belt and braces: the deadline may come from `context.WithTimeout` (fan-out budget) **or** `http.Client.Timeout` (per-call budget via `connection.Timeout`), and the `net.Error` check does not depend on modern toolchains surfacing both as `DeadlineExceeded` (`result.go:158-165`). **HIGH.**
3. HTTP status via `statusCodeOf`: 401/403 → `FailureAuth`, 429 → `FailureRateLimit` (`result.go:167-174`). **HIGH.**
4. Everything else → `FailureOther`. **Note: 5xx falls here.** There is no `FailureServer`. **HIGH.**

`statusCodeOf` tries both `azuredevops.WrappedError` (value) and `*azuredevops.WrappedError` (pointer) because `errors.As` will not convert between them (`result.go:186-197`). **HIGH.**

### Exit codes today (`cmd/adoBizInt`)

```go
exitOK      = 0 // the read completed and nothing failed
exitPartial = 1 // the read ran but did not reach every configured project
exitAuth    = 2 // at least one failure was an authentication rejection
exitUnknown = 3 // the read never ran: usage error, config error, client error
```
(`dispatch.go:21-26`) — declared a **public contract and a one-way door** (`dispatch.go:19-20`). **HIGH.**

`exitCodeFor(failures, complete)` (`exit.go:19-29`) scans for `FailureAuth` first and returns 2 regardless of co-occurring failures, because auth is the one kind that is definitely not transient (`exit.go:6-13`); then `!complete` → 1; else 0. It **never compares counts and never matches error text** (`exit.go:15-18`). **HIGH.**

### Where outcome-unknown slots in

The write path needs its **own mapper**, not an overload of `exitCodeFor` — that function's whole signature is `([]ado.Failure, bool)`, and writes return bare `error` with no failure list and no completeness flag (04-CONTEXT.md `<code_context>`: "Every `ado` write returns bare `error`, not `Result[T]`"). **HIGH** (structurally confirmed: `UpdateWorkItemState` returns `error` at `workitems.go:255`, `UpdateWorkItemStateWithEffort` at `:284`).

Proposed classification for the write mapper (**MEDIUM** — synthesis of D-09/D-20/D-22 with the `classifyFailure` structure; the exact integers are the planner's per D-09):

| Cause | Detected by | Write outcome | Exit |
|---|---|---|---|
| Write succeeded | `err == nil` | `executed` | 0 |
| Pre-read shows change already applied | diff is empty, before any mutating call | `no_op` (D-08) | new code A |
| Pre-read failed | error from the by-id read | fatal, no report (D-02) | 3 (`exitUnknown` — nothing was tried) |
| Parse error (missing positional, arity ≠ 1, multiple ids) | `fs.Parse` / arity check | nothing attempted | 3 (Phase 3 D-22 shape, `dispatch.go:175`) |
| **HTTP client timeout** (`connection.Timeout`, per-call) | `classifyFailure` → `FailureTimeout` via the `net.Error` branch | **`unknown`** (D-09, SAFE-04) | new code B |
| **Context deadline** (`--timeout` fan-out budget) | `classifyFailure` → `FailureTimeout` via `DeadlineExceeded` | **`unknown`** | new code B |
| **Mid-write SIGINT** | `ctx.Err() != nil` on the parent (the `runRead` pattern at `dispatch.go:220`) | **`unknown`** (D-22, supersedes Phase 3 D-25 for writes) | new code B |
| 401 / 403 | `classifyFailure` → `FailureAuth` | `failed` | 2 |
| 429 | `FailureRateLimit` | `failed` | 1 (or B — see Open Questions) |
| 409 conflict / permission-denied-on-item | `FailureOther` with a status | `failed` | 1 (planner's discretion — see below) |
| Non-status transport error | `FailureOther`, no status | **ambiguous** — see Risks | probably B |

**Critical distinction the CLI must preserve, and which `classifyFailure` alone cannot give you:** a `context.Canceled` classifies as `FailureOther`, not `FailureTimeout` (`result.go:154-155`). So a mid-write SIGINT is **invisible to the failure kind by design**. The read path solves this by keying on the *parent* context's `Err()`, not the kind: "the kind alone cannot tell you a signal fired, only the parent can" (`dispatch.go:212-215`). **The write path must copy this exact check** — and per D-22 route it to outcome-unknown rather than to `writeCrashRecord` + exit 3. **HIGH.**

**5xx is the gap.** `classifyFailure` has no 5xx branch; a 500 or 503 on a non-idempotent write (`POST` to queue a build, a JSON-Patch `Add`) lands in `FailureOther` alongside a DNS error and a malformed-response error. A 503 from a gateway in front of ADO genuinely *is* outcome-unknown territory for a non-idempotent write. **The planner must decide whether the write mapper adds a 5xx → unknown rule using `statusCodeOf`** (which is package-private — `result.go:186` — so either the rule lives in `internal/ado` or a small exported classifier is added). **MEDIUM/open — see Open Questions.**

**Non-idempotent retry is out of scope** (PLAT-03 deferred), but note D-01/D-03 each add a round trip per write, making throttling marginally more reachable (04-CONTEXT.md `<deferred>`).

---

## CLI Surface

### How verbs register today

`commands` is a `map[string]commandFunc` keyed on the **two-token `noun verb` string** (`dispatch.go:100-137`), with `commandFunc = func(ctx, client readClient, args []string) int` (`dispatch.go:79`). Each entry is its own closure, so no uniform signature is required — `GetBranches`' extra repositories parameter is absorbed by its own entry rather than by an adapter (`dispatch.go:76-78`). **HIGH.**

`dispatch` routes `args[0]+" "+args[1]`; an unmatched key prints root usage and returns `exitUnknown`, and never reaches a client call (`dispatch.go:143-151`, threat T-03-02 noted at `:83`). **HIGH.**

**Structural consequence:** adding seven write entries requires **no change to `commandFunc` or the `commands` map type** — confirmed by the closure signature at `dispatch.go:79`. But `commandFunc`'s second parameter is typed `readClient`, so either (a) `readClient` is widened with the write methods, or (b) `commandFunc` is re-typed to an interface that embeds both. 04-CONTEXT.md's `<code_context>` says "`readClient` gains a sibling write interface (or is extended)". **A sibling `writeClient` embedded into a combined `client` interface is cleaner** — `readClient`'s doc comment explicitly justifies its membership as "exactly the eight aggregate reads of D-02 — the ones returning `ado.Result[T]`" (`dispatch.go:54-58`), and writes return bare `error`, so widening it would break that stated invariant. **MEDIUM** (design recommendation, not a codebase fact).

### How flags register today

`newFlagSet(name)` is the **single `flag.NewFlagSet` site** (`flags.go:103-141`), returning `(*flag.FlagSet, *globalFlags, *filterFlags)`. It:

- uses `flag.ContinueOnError`, not `ExitOnError`, because stdlib's default exits 2 which now means auth failure and would be a lie (`flags.go:96-97`); **HIGH**
- calls `fs.SetOutput(stderrWriter)` explicitly so flag errors cannot silently reach stdout and break OUT-03 (`flags.go:99-102`); **HIGH**
- registers three globals unconditionally: `--json`, `--timeout`, `--project` (`flags.go:108-113`); **HIGH**
- registers filter flags **conditionally**, gated on `filterSupportByCommand[name]` (`flags.go:116-136`), a `map[string]filterSupport` keyed on the same `noun verb` string as the dispatch table "so the two cannot disagree about which verbs exist" (`flags.go:41-44`); **HIGH**
- sets `fs.Usage = func() { usageVerb(name, fs) }` (`flags.go:138`). **HIGH**

The rationale for conditional registration is directly reusable for `--confirm`: "a flag that parses but cannot mean anything is worse than a flag that does not exist: an agent reading `--help` would believe `releases list --hide-done` filters" (`flags.go:19-22`). **HIGH.** This is the codebase's own argument for the discretion item "whether `--confirm` is registered on read verbs" — the answer the existing code implies is **no**.

`stringList` is an existing custom `flag.Value` (`flags.go:76-87`) accepting both repeated and comma-separated values, so precedent exists if `fs.Visit` proves insufficient (`flags.go:74-75`). **HIGH.**

### Where the dry-run / confirm pair belongs

**Recommendation (MEDIUM, design):** mirror `filterSupportByCommand` with a `writeSupportByCommand map[string]writeSupport` and a `writeFlags` struct, registered inside `newFlagSet` behind that gate. This:

- keeps `newFlagSet` as the single FlagSet site, preserving what D-20 was written to protect (`flags.go:89-94`);
- keeps `TestFlagsRegisteredOnEverySubcommand` (named in 04-CONTEXT.md `<code_context>`) as the structural drift guard — and the write-side analogue becomes "every write verb registers `--confirm`, no read verb does", which is a structural assertion, not a grep-for-a-string;
- puts `--state`, `--original-estimate`, `--remaining`, `--completed`, `--comment`, `--branch` in the same gated block, so `prs approve --state Closed` is a parse error rather than a silently ignored flag.

**D-18's `fs.Visit` mechanism:** `fs.Visit` walks only flags **that were actually set** (as opposed to `fs.VisitAll`). The call must run **after** `fs.Parse` and **inside** the command body where `fs` is in scope. `newFlagSet` returns `fs`, so `runWrite` has it. Shape:

```go
seen := map[string]bool{}
fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
if seen["remaining"] { eff.Remaining = &remainingVal }  // nil otherwise
```

**HIGH on the `fs.Visit` semantics** (stdlib, and `flags.go` already uses the `flag` package idioms); **MEDIUM on the exact shape** (not present in the codebase today).

**D-24's arity-of-one:** `--project` is registered as `stringList` (`flags.go:112`), which accumulates. The write path enforces `len(gf.projects) == 1` after parse and treats 0 or >1 as a parse error. Note this means the write path must **not** reuse `emptyIfNil` (`dispatch.go:263-268`), whose entire purpose is normalising unset to "no filter requested" so the read method falls back to its configured list (`dispatch.go:181-190`). **That fallback is exactly what D-24 forbids for writes.** **HIGH** — the fallback is documented at `dispatch.go:183-185` and at `branches.go:56-59` per that comment.

**D-25's one-target rule:** after `fs.Parse(args)`, `fs.Args()` holds the positionals. Enforce `len(fs.Args()) == 1` and parse it as an int. Zero → parse error; two or more → parse error. **MEDIUM** (mechanism is stdlib; no existing call site in the read path since reads take no positionals).

**Exit code on parse failure:** `runRead` returns `exitOK` on `flag.ErrHelp` (help succeeded) and `exitUnknown` otherwise (`dispatch.go:169-176`). The write path should copy this verbatim — a parse error means nothing was attempted, which is precisely `exitUnknown`'s meaning. **HIGH.**

### Output plumbing to reuse

- `stdoutWriter` / `stderrWriter` are package-level `io.Writer` vars defaulting to the real streams, swapped by tests so stdout purity is assertable without spawning a process (`dispatch.go:28-34`). **HIGH.** The audit sink is a **third** writer and must stay distinct from both.
- `emitJSON` marshals to a byte slice and writes it with a **single** `Write`, plus a trailing newline; a streaming encoder is deliberately not used because it can leave a partial document on stdout ahead of a crash record (`dispatch.go:270-282`). **The write envelope must go through this same function.** **HIGH.**
- `fetchctx.NewWithParent(ctx, budget)` derives the deadline from the caller's cancellable parent so a signal aborts in flight; an explicit `--timeout 0s` means "use the default budget", never "no deadline" (`dispatch.go:199-208`). **HIGH.**

---

## Work Item Write Semantics

### The seven mutations

| `ado` method | File:line | Verb (04-CONTEXT.md working set) | Confidence |
|---|---|---|---|
| `UpdateWorkItemState` | `workitems.go:255` | `workitems set-state` | HIGH (read) |
| `UpdateWorkItemStateWithEffort` | `workitems.go:284` | `workitems set-state` + effort flags (D-19) | HIGH (read) |
| `ApprovePullRequest` | `pullrequests.go:167` | `prs approve` | MEDIUM (04-CONTEXT.md `<canonical_refs>`) |
| `CompletePullRequest` | `pullrequests.go:208` | `prs complete` / `prs merge` (open) | MEDIUM (same) |
| `ApproveRelease` | `releases.go:302` | `releases approve` | MEDIUM (same) |
| `RejectRelease` | `releases.go:327` | `releases reject` | MEDIUM (same) |
| `QueuePipeline` | `pipelines.go:391` | `pipelines queue` | MEDIUM (same) |

Underlying SDK calls, from the interfaces I read (`interfaces.go:17-47`): `workitemtracking.UpdateWorkItem` (both work-item writes), `git.CreatePullRequestReviewer` (approve), `git.UpdatePullRequest` (complete), `release.UpdateReleaseApproval` (approve + reject), `build.QueueBuild` (queue). **HIGH** — these are the only four write-shaped methods on the four interfaces, and each maps 1:1.

Only two of the seven use JSON-Patch: `UpdateWorkItemState` and `UpdateWorkItemStateWithEffort`. The other five pass typed SDK args structs. **HIGH.**

### The JSON-Patch documents, verbatim

`UpdateWorkItemState` (`workitems.go:258-271`) — one operation:

```go
op := webapi.OperationValues.Add
path := "/fields/System.State"
document := []webapi.JsonPatchOperation{
	{
		Op:    &op,
		Path:  &path,
		Value: newState,
	},
}

_, err := c.workitemClient.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
	Id:       &itemID,
	Document: &document,
})
```

**HIGH — read verbatim.**

`UpdateWorkItemStateWithEffort` (`workitems.go:284-321`) — signature and the blind-overwrite:

```go
func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, itemID int, newState string, originalEstimate, remaining, completed float64) error {
```

```go
	addOp := webapi.OperationValues.Add
	statePath := "/fields/System.State"
	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
	remainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	completedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"

	// Update effort fields first, then state change
	// Using Add operation which works for both new and existing values
	document := []webapi.JsonPatchOperation{
		{
			Op:    &addOp,
			Path:  &originalEstimatePath,
			Value: originalEstimate,
		},
		{
			Op:    &addOp,
			Path:  &remainingPath,
			Value: remaining,
		},
		{
			Op:    &addOp,
			Path:  &completedPath,
			Value: completed,
		},
		{
			Op:    &addOp,
			Path:  &statePath,
			Value: newState,
		},
	}
```

**HIGH — read verbatim (`workitems.go:287-316`).**

**The four exact field paths, verbatim** (these are the values a code skeleton may use):
- `"/fields/System.State"`
- `"/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"`
- `"/fields/Microsoft.VSTS.Scheduling.RemainingWork"`
- `"/fields/Microsoft.VSTS.Scheduling.CompletedWork"`

**Operation order is part of the contract**, not incidental: three effort ops, then state, with the comment "Update effort fields first, then state change" (`workitems.go:293-294`). `TestUpdateWorkItemStateWithEffortIssuesExactPatch` asserts the whole `UpdateWorkItemArgs` with `reflect.DeepEqual` against a slice literal in that exact order (`workitems_test.go:363-391`). **Reordering the surviving ops when some are omitted will fail that test.** Preserve the order; only omit. **HIGH.**

**`Op` is `Add`, not `Replace`, for all four** — with the stated rationale "works for both new and existing values" (`workitems.go:294`). Do not "improve" this to `Replace` while doing D-17; `Replace` fails on a field that has never been set. **HIGH.**

### Three-state effort fields

The three states D-17/D-18 require:

| State | CLI expression | Patch document |
|---|---|---|
| Absent — leave unchanged | flag not passed (`fs.Visit` does not report it) | **no operation appended for that path** |
| Explicit value | `--remaining 3.5` | `Add` op with `Value: 3.5` |
| Explicit zero | `--remaining 0` | `Add` op with `Value: 0.0` |

**Note there is no "explicit clear" (set to null) state in scope.** The research brief asked about it; 04-CONTEXT.md D-17 defines only "nil fields omitted from the JSON patch document" and D-18 only distinguishes omitted from zero. ADO JSON-Patch supports a `Remove` operation to clear a field, but **nothing in the decisions asks for it** and no verb is specified for it. Treat "explicit clear" as **out of scope**; flag it in Open Questions rather than inventing a `--clear-remaining` flag. **HIGH on the decisions' silence; the ADO `Remove` semantics claim is LOW (training knowledge, not verified this session).**

### Go pointer-vs-value and `omitempty` pitfalls

These are the specific ways SAFE-03 can be "fixed" and still silently zero a field:

1. **`omitempty` on a `float64` cannot express zero.** `json:"remaining,omitempty"` drops `0.0` identically to an unset field. If any intermediate struct between the flags and the patch builder uses `float64` + `omitempty`, `--remaining 0` is silently dropped and the field is left at its old value — the *inverse* of today's bug but equally wrong, and D-18 explicitly names closing a task at RemainingWork 0 as the primary use case. **Use `*float64` end to end; if a struct is serialized at all, use `*float64` with `omitempty`, where nil-drops-but-zero-survives is exactly right.** **HIGH** (Go `encoding/json` semantics; the risk is codebase-relevant because the value flows into `Value: originalEstimate` at `workitems.go:299` as a bare `float64`).

2. **`webapi.JsonPatchOperation.Value` is `interface{}`.** Assigning a `*float64` to it serializes as a JSON number *or* as `null` if the pointer is nil — it does **not** omit the operation. So the nil check must gate **whether the operation is appended**, never what value it carries. Writing `Value: remainingPtr` with a nil pointer sends `{"op":"add","path":".../RemainingWork","value":null}`, which is a *clear*, not a no-op. This is the single most likely way to implement D-17 and still break it. **HIGH on `Value` being untyped** (it takes a bare `string` at `workitems.go:264` and a bare `float64` at `:299`, so it is `interface{}` or `any`); **MEDIUM on the null-serialization consequence** (standard `encoding/json` behaviour, not verified against the SDK's own marshaller this session).

3. **Loop-variable aliasing on the path strings.** The current code takes `&originalEstimatePath` etc. of distinct named locals (`workitems.go:288-291`), which is safe. If D-17's refactor turns this into a loop over a `[]struct{path string; val *float64}` and takes `&entry.path`, every operation may point at the same backing string on older Go toolchains (pre-1.22 loop-variable semantics). Verify the module's Go version before writing such a loop, or bind `p := entry.path` inside the loop. **MEDIUM** (go.mod not read this session).

4. **The existing exact-patch test is the regression guard, and it passes values not pointers.** `client.UpdateWorkItemStateWithEffort(context.Background(), 84, "Closed", 8.0, 3.5, 4.5)` (`workitems_test.go:353`). Changing the signature to `*float64` **breaks this test's compilation**, which is correct and desirable — it forces the change to be acknowledged. The updated test should pass pointers to the same values and assert the byte-identical document, proving the TUI's behaviour is preserved (D-17's stated guard). **HIGH.**

5. **The TUI call sites pass all three values explicitly today.** `EffortSubmitMsg` carries `OriginalEstimate float64`, `Remaining float64`, `Completed float64` as plain values (`effort.go:18-27`), populated from `parseFloat` which returns `0, nil` for an empty input (`effort.go:193-199`). **So the TUI genuinely sends 0 for a blank field today, and D-17's fix will change that to "omitted" only if the TUI is also changed to distinguish blank from zero.** D-17 says "the TUI passes all three values explicitly today, so passing pointers to the same values preserves its behaviour exactly" — that is the *safe* minimal change (wrap each value in a pointer, always non-nil). **Do not opportunistically make `effort.go` treat blank as nil**: that is a TUI behaviour change, which PROJECT.md constraints forbid, and it is not asked for. **HIGH** — `parseFloat` returning `0, nil` on empty is at `effort.go:195-197`.

6. **`completed <= 0` is already rejected by the TUI modal** (`effort.go:144-147`), so the Completed field can never legitimately be nil-or-zero from the TUI path. The CLI has no such rule and must not acquire one — D-26's reasoning (no config-driven or convenience-driven CLI behaviour) applies. **HIGH.**

---

## Dry-Run Diff

### The computation

Per D-01, the dry run is `pre-read → compute field-level diff → render`, and per D-02 a failed pre-read is fatal with **no report emitted**. Per D-03 the same pre-read runs on the `--confirm` path, and an empty diff is a refusal with **no mutating call**.

Shape (**MEDIUM** — synthesized from D-01/D-02/D-03/D-10/D-11, no such code exists today):

```
1. parse + arity checks            -> parse error: exit 3, nothing attempted
2. open audit sink for append      -> unwritable: fatal, exit ? (D-13; before the pre-read is acted on)
3. by-id pre-read                  -> error: fatal, no report, exit 3 (D-02)
4. build intended patch from flags (fs.Visit presence)
5. diff current vs intended -> []change{field, from, to}
6. if len(changes) == 0            -> refused no-op, no audit, exit code A (D-03, D-08, D-14)
7. if !confirm                     -> emit envelope executed=false outcome=dry_run, exit 0 (D-07)
8. mutate
9. classify outcome, append audit record (D-15/D-16), emit envelope, map exit
```

**Step 2 before step 3 is D-13's whole point** and is the easiest thing to get subtly wrong. 04-CONTEXT.md `<specifics>`: "If the plan implements the check after the write, the decision has been silently reversed while appearing to be honoured." A structural test for this: construct the write path with a read-only audit directory and a fake whose read func **and** write func are both set, then assert the write func was never called. **MEDIUM** (test design; the fake mechanism that makes it assertable is HIGH per `fakes_test.go:56-58`).

**Step 6 before step 7 matters too:** D-08 says the no-op refusal applies "on both the dry-run and `--confirm` paths", so the empty-diff check precedes the confirm branch. A dry run of an already-applied change exits code A, not 0. **HIGH** (D-08 verbatim).

### The change record

Per D-10 the envelope carries `"changes":[{"field":...,"from":...,"to":...}]`. For a work-item state change with effort, `field` should be the ADO field reference name (e.g. `Microsoft.VSTS.Scheduling.RemainingWork`) rather than the flag name — that is what the patch actually targets and what an agent can cross-reference. **LOW/recommendation** — D-10's example shows only `"field"` with no stated vocabulary.

`from` and `to` types: the pre-read returns `WorkItemDetail`, whose effort fields are **not** currently populated — REQUIREMENTS.md DATA-02 records that `RemainingWork`, `CompletedWork` and `OriginalEstimate` "are written at `workitems.go:249-251` but never selected in the read path". **HIGH** (REQUIREMENTS.md:89). I confirmed this against `GetWorkItemDetail` (`workitems.go:334-409`): it reads Title, Description, WorkItemType, State, StoryPoints, AcceptanceCriteria, ReproSteps, IterationPath, AreaPath, AssignedTo, CreatedBy, ChangedBy — **no effort fields**. **HIGH.**

> **This is a concrete, under-flagged prerequisite.** A `current -> proposed` diff for `--remaining 3.5` is impossible today because `GetWorkItemDetail` does not read `RemainingWork`. Either `GetWorkItemDetail` gains the three effort fields, or the work-item dry run can only diff State. 04-CONTEXT.md D-01 discusses adding *new by-id reads for PR/release/pipeline* and treats the work-item read as already existing — **it exists but is insufficient**. This belongs in plan 04-01, not discovered during 04-02.
>
> Note `GetWorkItemDetail` uses `Expand: Relations` and the SDK forbids combining `Fields` with `Expand` (`workitems.go:337`: "Use Relations expand to get attachments (can't use Fields with Expand)"), so adding the effort fields is a matter of reading more keys out of `*item.Fields` — the full field set is already returned. **HIGH.** The addition is three `if v, ok := itemFields["Microsoft.VSTS.Scheduling.RemainingWork"].(float64); ok` blocks plus three struct fields, mirroring the `StoryPoints` block at `workitems.go:370-372`.

### Rendering

Two shapes per D-11/Phase 3 D-13: `--json` → the single-`Write` envelope via `emitJSON` (`dispatch.go:274`); human mode → the diff table on stdout, diagnostics on stderr. The read path's precedent is `render(stdoutWriter, ...)` + `renderFailures(stderrWriter, ...)` (`dispatch.go:253-254`). **HIGH.**

---

## Audit Record

**Format:** newline-delimited JSON, one object per executed (or outcome-unknown) write. **HIGH** (D-12).

**Location:** beside the log file, so Phase 5 HARD-06's `0600`/`0700` work covers it. `logging.LogPath()` is at `internal/logging/logging.go:44` per 04-CONTEXT.md `<canonical_refs>`. **MEDIUM** — I did not read `logging.go` this session; the planner should confirm `LogPath()`'s exact return shape (full file path vs directory) before deriving the audit path from it. D-12's example is `~/.config/adoBizInt/audit.jsonl`.

**Fields (D-15, verbatim):** timestamp, command, target identity (project / repo / item or PR / release / definition id), the field-level `from -> to` captured by the pre-read, and the outcome.

**Explicitly omitted:**
- **Actor identity** — there is one identity, the PAT's, and Phase 1 D-12 makes its resolution non-fatal so the field can legitimately be empty. Confirmed structurally: `NewClient` logs `"Could not cache current user ID"` as a `Warn` and proceeds with `currentUserID == ""` (`client.go:113-115`). **HIGH.**
- **Full argv** — caller-controlled, could carry a secret passed on a command line; a credential-disclosure path into a persisted file, same class as Phase 3 D-24's stack-trace rule. **HIGH** (D-15).

**Also do not add:** request URLs, headers, authorization values, or the PAT. `Failure`'s T-02-01 constraint (`result.go:41-46`) is stated for that struct, but the reasoning — "failures are logged and rendered into a terminal; anything stored here is assumed to reach both" — applies with more force to a persisted append-only file. The audit record must not embed an `error` value directly; if the outcome carries a cause, flatten it to a message string the way `Failure.MarshalJSON` does (`result.go:71-88`). **HIGH.**

**Write discipline:**
- Open for append **before** the ADO call (D-13). Open with `os.O_APPEND|os.O_CREATE|os.O_WRONLY`.
- Marshal each record to a byte slice and write it with **one** `Write` including the trailing newline — same rationale as `emitJSON`'s single-write (`dispatch.go:270-273`): a partial line in a JSONL file is an unparseable record forever. Do not use `json.NewEncoder(f).Encode(v)` here for the same reason the read path rejects a streaming encoder on stdout. **MEDIUM** (recommendation; the precedent is HIGH).
- `Sync()` before returning success is worth considering so a crash between write and process exit does not lose the record. **LOW** (not discussed in D-12/D-13).

**Audited:** executed writes (D-14) **and** outcome-unknown writes (D-16). **Not audited:** dry runs, refused no-ops (D-14). **HIGH.**

---

## Risks & Pitfalls

### Risk 1: D-13's ordering silently reversed
**What goes wrong:** the audit append is implemented after the successful ADO call, because that reads more naturally.
**Why it happens:** "append an audit record" (SAFE-02) describes the append, not the pre-flight check.
**How to avoid:** a test with an unwritable audit directory + a fake whose mutating func is set, asserting the mutating func was never called. Structural, not prose.
**Warning signs:** the audit-open call appears after the mutate call in the function body.
**Confidence:** HIGH (04-CONTEXT.md `<specifics>` names this explicitly).

### Risk 2: nil `*float64` assigned to `Value` instead of skipping the operation
**What goes wrong:** `{"op":"add","path":".../RemainingWork","value":null}` is sent, clearing the field — a worse version of the bug being fixed.
**How to avoid:** the nil check gates `append`, never the value. Assert in test that the document has **length 2** for a state+completed write, not length 4 with two nulls.
**Confidence:** HIGH on the failure mode's plausibility; MEDIUM on the exact SDK serialization.

### Risk 3: `omitempty` on a `float64` reintroduces the zero conflation
**How to avoid:** `*float64` end to end; never `float64` + `omitempty` anywhere between `fs.Visit` and the patch builder.
**Confidence:** HIGH.

### Risk 4: the work-item dry run cannot diff effort because the pre-read does not fetch it
**Detail:** `GetWorkItemDetail` (`workitems.go:334-409`) reads no effort field; REQUIREMENTS.md DATA-02 records this as a known v2 gap. Discovered during 04-02 instead of planned in 04-01, this stalls the plan.
**Confidence:** HIGH.

### Risk 5: mid-write SIGINT classified as `FailureOther` and routed to "failed"
**Detail:** `classifyFailure` maps `context.Canceled` to `FailureOther` deliberately (`result.go:154-155`). Only the parent `ctx.Err()` check reveals a signal (`dispatch.go:212-215`). A write path that keys on the failure kind alone reports an interrupted write as an ordinary failure — asserting "nothing happened" when something might have, which D-09 calls "the most consequential possible instance of this project's core failure mode".
**Confidence:** HIGH.

### Risk 6: 5xx on a non-idempotent write lands in `FailureOther`
**Detail:** no 5xx branch in `classifyFailure` (`result.go:167-174`). A 503 mid-`QueueBuild` is genuinely unknown-outcome, but classifies as "other". `statusCodeOf` is package-private (`result.go:186`), so the write mapper in `cmd/` cannot reach it without a new export.
**Confidence:** HIGH on the code; MEDIUM on whether the planner should close it (see Open Questions).

### Risk 7: D-24's arity rule bypassed by reusing `emptyIfNil`
**Detail:** `emptyIfNil` (`dispatch.go:263-268`) exists to let an omitted `--project` fall back to the configured list inside the read method. Reusing it on the write path makes an omitted `--project` silently pick a project the caller did not name — which D-24 calls "unacceptable for a mutation".
**Confidence:** HIGH.

### Risk 8: exit-code integers collide or drift
**Detail:** D-09 marks the new integers a one-way door joining the published contract at `dispatch.go:19-26`. Two codes, distinct from each other and from 0/1/2/3.
**Mitigation:** declare them in the same `const` block at `dispatch.go:21-26` with the same comment discipline, and add a test asserting all six constants are pairwise distinct.
**Confidence:** HIGH.

### Risk 9: `reflect.DeepEqual` exact-patch tests break on operation reorder
**Detail:** `workitems_test.go:363-391` compares the whole slice literal. Omitting operations must preserve relative order (effort fields, then state).
**Confidence:** HIGH.

### Risk 10: D-01's cost over-estimated, not under-estimated
**Detail:** 04-CONTEXT.md predicts three new interface entries. `gitAPI.GetPullRequest` (`interfaces.go:28`), `buildAPI.GetDefinition` (`interfaces.go:37`) and `releaseAPI.GetApprovals` (`interfaces.go:43`) already exist with fakes. The real cost may be lower — but `GetApprovals` is a list call, so a by-id approval lookup may still need a new method. The planner should check each against the SDK rather than trusting either estimate.
**Confidence:** HIGH on the interface entries existing; MEDIUM on sufficiency.

### Risk 11: `internal/ado` file-size standard
**Detail:** the package is ~2,015 lines against CONVENTIONS.md's 300-line standard, and 04-CONTEXT.md's discretion list asks the plan to **state** where the new by-id reads live rather than scatter them.
**Confidence:** MEDIUM (line count from 04-CONTEXT.md, not counted this session; `workitems.go` alone is 591 lines — HIGH, from the read tool's own total).

---

## Open Questions (RESOLVED)

> Every item below was settled during planning. The settling decision is named at the end
> of each line. Items 6-9 are read-before-writing items, now carried in the owning task's
> `<read_first>` block rather than left open.

1. **The two exit-code integers.** D-09 delegates them. Constraint: exactly two, distinct, ≥ 4. Recommend 4 = no-op-refused, 5 = outcome-unknown (ascending severity). **RESOLVED by P-01** (04-01): `exitNoOp = 4`, `exitOutcomeUnknown = 5`, ratified at 04-01 Task 1.
2. **5xx → outcome-unknown?** `classifyFailure` has no 5xx branch and `statusCodeOf` is unexported. Should the write mapper treat 5xx on a non-idempotent write as unknown? SAFE-04's text says only "times out". **RESOLVED by P-12** (04-01): yes — 5xx maps to `outcome:"unknown"`, exit 5, via a new exported `ado.StatusCodeOf`.
3. **Non-status transport errors** (connection reset mid-request, EOF). These are `FailureOther` with no status. A reset *after* the request was sent is outcome-unknown; a reset during connect is a clean failure. Go's `net/http` does not reliably distinguish these. **RESOLVED by P-13** (04-01): default to unknown for writes, on the D-09 principle that claiming "nothing happened" is the worse error.
4. **"Explicit clear" for effort fields.** The brief asks about a three-state model including explicit clear; the decisions define only absent/value/zero. **RESOLVED as out of scope** — recorded as an opt-out in `COVERAGE.md`; no `--clear-*` flag is planned.
5. **`logging.LogPath()`'s return shape** — file path or directory. Affects how the audit path is derived. **RESOLVED in 04-01 Task 2 `<action>`**: `LogPath` returns the log FILE path (`logging.go:46-49`), so `auditPath` is constructed independently from `home` + fixed segments and the action explicitly forbids deriving it from `LogPath()`.
6. **`pullrequests.go`, `releases.go`, `pipelines.go` internals.** All claims about `ApprovePullRequest`, `CompletePullRequest`, `ApproveRelease`, `RejectRelease`, `QueuePipeline` signatures and line numbers come from 04-CONTEXT.md `<canonical_refs>`. **Carried as a read-before-writing item in 04-03's `<read_first>`.**
7. **`internal/ui/workitems/workitems.go:266,:333,:347,:354`** — the four D-17 call sites. **Carried as a read-before-writing item in 04-02's `<read_first>`.**
8. **`usage.go`** — Phase 3 D-21's three-level help must gain the write verbs. **Carried as a read-before-writing item in 04-03's `<read_first>`**, pinned by `TestEveryWriteVerbAppearsInNounUsage`.
9. **`go.mod` Go version** — affects the loop-variable aliasing pitfall. **Carried as a read-before-writing item in the owning task's `<read_first>`.**
10. **`--comment` on release verbs, `--branch` on `pipelines queue`, `prs complete` vs `prs merge`, whether the two effort `ado` methods merge** — all explicitly delegated by 04-CONTEXT.md's discretion list. **RESOLVED by P-02 through P-06** (04-01): `prs complete`; `--comment` optional on both release verbs; `--branch` required on `pipelines queue`; the two effort methods do NOT merge; pre-reads live in a new `preread.go`.
11. **Whether the pre-read shares the write's `--timeout` budget.** If they share, a slow pre-read can eat the whole budget and force the write into outcome-unknown territory for a reason that has nothing to do with the write. **RESOLVED by P-11** (04-01): they share one `fetchctx.NewWithParent` budget, matching `runRead`, with the limitation documented.
12. **Rejected-write exit code** (409, item-level permission denied). Discretion list says "probably exit 1, but exit 1 currently means partial read". **RESOLVED by P-08** (04-01): exit 1 with `outcome:"failed"`; exit 1 gains a documented write meaning alongside its read meaning.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, table-driven, no testify, no mock generator (per 04-CONTEXT.md `<canonical_refs>` → `.planning/codebase/TESTING.md`) — MEDIUM, TESTING.md not read this session |
| Config file | none (stdlib) |
| Quick run command | `go test ./internal/ado/... ./cmd/adoBizInt/...` |
| Full suite command | `go test ./...` |
| Assertion style | `reflect.DeepEqual(got, want)` against a struct literal (`workitems_test.go:336`, `:389`) — HIGH |
| Fake style | func-field-per-method, nil field → `t.Fatalf("unexpected call to …")` (`fakes_test.go:54-60`) — HIGH |
| Injection | struct literal on unexported `Client` fields, in-package (`workitems_test.go:308-316`) — HIGH |

### SC1..SC6 → validation approach

| SC | Behavior | Test type | Approach | Exists? |
|----|----------|-----------|----------|---------|
| **SC1** | Every write verb dry-runs unless `--confirm` | unit, `cmd/adoBizInt` | Structural, not per-verb: iterate `writeSupportByCommand` (or the write subset of `commands`) and assert each registers `--confirm` via `fs.Lookup("confirm") != nil`, and that no read verb does. Mirrors the existing `TestFlagsRegisteredOnEverySubcommand` shape. Name: `TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`. | ❌ Wave 0 |
| **SC1** | Without `--confirm`, no mutation | unit, `internal/ado` or `cmd` | Fake with read func set, **mutating func left nil** → any mutation is an automatic `t.Fatalf`. Name: `TestDryRunIssuesNoMutatingCall`, table-driven over the seven verbs. | ❌ Wave 0 |
| **SC2** | Dry run reports exactly what would change | unit | Given a fake pre-read returning State="Active" and `--state Closed`, assert the envelope's `changes` equals `[{field:"System.State", from:"Active", to:"Closed"}]` via `reflect.DeepEqual`; assert `executed==false`, `outcome=="dry_run"`. Name: `TestDryRunEnvelopeReportsFieldLevelDiff`. | ❌ Wave 0 |
| **SC2** | No mutating ADO call | unit | Same nil-func-field mechanism as SC1 row 2. This is the canonical assertion. | ❌ Wave 0 |
| **SC2** | Failed pre-read emits no report (D-02) | unit | Pre-read fake returns `wrappedErrorValue(500,…)`; assert `stdoutWriter` buffer is **empty** and exit is non-zero. Name: `TestFailedPreReadEmitsNoDryRunReport`. | ❌ Wave 0 |
| **SC3** | Every executed write appends an audit record | unit | Temp-dir audit sink; run a confirmed write against a fake; read the file back, `json.Unmarshal` the single line, assert the D-15 field set exactly (and assert **no** `argv` / `actor` / `url` key present). Name: `TestExecutedWriteAppendsAuditRecord` + `TestAuditRecordOmitsArgvAndCredentialFields`. | ❌ Wave 0 |
| **SC3** | Audit sink checked **before** the ADO call (D-13) | unit | Read-only audit dir, fake with the mutating func **set** (so a call would be observable); assert the mutating func was never invoked and exit is non-zero. Name: `TestUnwritableAuditSinkPreventsMutation`. **This is the test that makes D-13 a guarantee.** | ❌ Wave 0 |
| **SC3** | Dry run and refused no-op write nothing (D-14) | unit | Assert the audit file does not exist / is zero-length after each. Name: `TestDryRunAndNoOpAppendNoAuditRecord`. | ❌ Wave 0 |
| **SC3** | Outcome-unknown IS audited (D-16) | unit | Mutating fake returns `context.DeadlineExceeded`; assert one record with `outcome:"unknown"`. Name: `TestOutcomeUnknownWriteIsAudited`. | ❌ Wave 0 |
| **SC4** | Omitting an effort field leaves it unchanged | unit, `internal/ado` | Call the nullable signature with `originalEstimate=nil, remaining=ptr(3.5), completed=nil`; assert the document has **exactly two** operations (RemainingWork then State), in that order, with no null values. Name: `TestUpdateWorkItemStateWithEffortOmitsNilFields`. | ❌ Wave 0 |
| **SC4** | `--remaining 0` is a real zero (D-18) | unit, `cmd` | `fs.Parse([]string{"-remaining","0",...})` then assert the built patch contains a `RemainingWork` op with `Value: 0.0`. Name: `TestExplicitZeroEffortIsDistinctFromOmitted`. | ❌ Wave 0 |
| **SC4** | TUI behaviour unchanged (D-17 regression guard) | unit, `internal/ado` | Update `TestUpdateWorkItemStateWithEffortIssuesExactPatch` (`workitems_test.go:341`) to pass pointers to 8.0/3.5/4.5 and assert the **byte-identical four-operation document** it asserts today. | ✅ exists at `workitems_test.go:341` — needs signature update, not rewrite |
| **SC5** | Timeout → outcome-unknown, distinct from success and failure | unit | Three-case table: fake returns `nil` → exit 0/`outcome:"executed"`; returns `wrappedErrorValue(403,…)` → exit 2/`"failed"`; returns `context.DeadlineExceeded` → exit B/`"unknown"`. Assert all three exit codes are distinct. Name: `TestWriteOutcomeClassification`. | ❌ Wave 0 |
| **SC5** | Mid-write SIGINT → unknown, not exit 3 (D-22) | unit | Cancel the parent ctx before the fake returns; assert exit == unknown code B, **not** `exitUnknown` (3), and that no crash record was written. Name: `TestInterruptedWriteReportsOutcomeUnknownNotNothingRan`. | ❌ Wave 0 |
| **SC5** | The two new codes are distinct from 0/1/2/3 and each other | unit | Assert pairwise distinctness over all six constants. Name: `TestExitCodesArePairwiseDistinct`. | ❌ Wave 0 |
| **SC6** | All of the above use the Phase 1 seam | — | Satisfied by construction: every test above injects `&Client{workitemClient: fake, …}` or the `cmd`-level `writeClient` interface. No network. Verify with `grep -rn "http\." --include=*_test.go internal/ado` returning nothing new. | partial |

### Sampling Rate

- **Per task commit:** `go test ./internal/ado/... ./cmd/adoBizInt/...`
- **Per wave merge:** `go test ./... && go vet ./... && gofmt -l .`
- **Phase gate:** `go test -race ./...` green before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `cmd/adoBizInt/write_test.go` — SC1, SC2, SC5 envelope + exit-code tests
- [ ] `cmd/adoBizInt/audit_test.go` — SC3, including the D-13 ordering test
- [ ] `cmd/adoBizInt/flags_test.go` — extend with `--confirm` registration + `fs.Visit` presence tests (file existence UNVERIFIED this session)
- [ ] `internal/ado/workitems_test.go` — update `TestUpdateWorkItemStateWithEffortIssuesExactPatch` for the pointer signature; add the nil-omission test
- [ ] `internal/ado/fakes_test.go` — add fake methods for any new by-id read interface entries
- [ ] A helper for a temp audit sink (`t.TempDir()` + path), to avoid it being re-invented per test

---

## Project Constraints (from PROJECT.md / CONVENTIONS.md, via 04-CONTEXT.md)

> **MEDIUM** — these come from 04-CONTEXT.md `<canonical_refs>`; neither PROJECT.md nor CONVENTIONS.md was read this session. `./AGENTS.md` in the working directory was not read (outside the permitted file list).

- No new dependencies without justification (CONVENTIONS rule 6). **The audit sink, the diff and the envelope are all stdlib-only.** No new module should be needed.
- No abstraction unless the current task requires it (CONVENTIONS rule 5). `result.go:12-19` shows the codebase's own worked example of taking this carve-out explicitly and documenting why.
- Files under 300 lines. `internal/ado` is already far over; the plan must **state** where new by-id reads go.
- **TUI behaviour must not change.** D-17 changes TUI call-site *signatures*; CLI-01 forbids behaviour change, not signature change.
- stdout carries only parseable output (OUT-03); diagnostics to stderr or the log file. The audit file is a **third** sink.

---

## Sources

### Primary (HIGH confidence — read verbatim this session)
- `.planning/phases/04-guarded-writes/04-CONTEXT.md` (581 lines) — all 26 decisions, discretion list, deferred list
- `.planning/REQUIREMENTS.md` (256 lines) — CLI-03, SAFE-01..04, DATA-02, AGENT-01..04, traceability
- `.planning/ROADMAP.md` (180 lines) — Phase 4 goal, SC1..SC6, provisional plan split
- `internal/ado/result.go` (198 lines) — `FailureKind`, `Failure`, `classifyFailure`, `statusCodeOf`
- `internal/ado/interfaces.go` (47 lines) — the four `-API` interfaces
- `internal/ado/client.go` (285 lines) — `Client` fields, `newConnection`, identity resolution
- `internal/ado/workitems.go:230-409` — both work-item writes, `GetWorkItemDetail`
- `internal/ado/fakes_test.go` (242 lines) — fake shape, error constructors, conformance assertions
- `internal/ado/workitems_test.go:300-394` — the two exact-patch write tests
- `cmd/adoBizInt/dispatch.go` (293 lines) — exit codes, writers, `readClient`, `commands`, `runRead`, `emitJSON`
- `cmd/adoBizInt/exit.go` (29 lines) — `exitCodeFor`
- `cmd/adoBizInt/flags.go` (141 lines) — `newFlagSet`, `globalFlags`, `filterSupportByCommand`, `stringList`
- `internal/ui/workitems/effort.go` (302 lines) — `EffortSubmitMsg`, `parseFloat`, the `completed <= 0` rule
- `internal/ado/*_test.go` — test-name inventory via grep (bodies not read except workitems)

### Secondary (MEDIUM — cited from 04-CONTEXT.md, source not opened)
- `internal/ado/pullrequests.go`, `releases.go`, `pipelines.go` — the five non-work-item writes
- `internal/ui/workitems/workitems.go` — the four D-17 call sites
- `internal/logging/logging.go` — `LogPath()`
- `cmd/adoBizInt/usage.go`, `.planning/codebase/*.md`, `.planning/PROJECT.md`, `.planning/STATE.md`

### Not consulted
No web search, no package registry, no external documentation. This phase adds no dependencies, so the Package Legitimacy Audit is **not applicable**.

---

## Metadata

**Confidence breakdown:**
- Existing seam: **HIGH** — `interfaces.go`, `fakes_test.go`, `client.go` read in full
- Outcome model: **HIGH** on the read path; **MEDIUM** on the proposed write mapping (synthesis, plus two genuinely open questions on 5xx and transport errors)
- CLI surface: **HIGH** on what exists; **MEDIUM** on the recommended `writeSupportByCommand` shape
- Work item write semantics: **HIGH** — both patch documents quoted verbatim from source
- Dry-run diff: **MEDIUM** — no such code exists; the `GetWorkItemDetail` effort-field gap is HIGH and is the section's most actionable finding
- Audit record: **HIGH** on the decisions; **MEDIUM** on the path derivation (`logging.go` unread)
- Validation architecture: **HIGH** on the mechanisms (nil-func-field `t.Fatalf`, `reflect.DeepEqual`, struct-literal injection); test names are proposals

**Research date:** 2026-09-12
**Valid until:** stable — this is codebase archaeology with no external dependencies. Re-verify if `internal/ado` or `cmd/adoBizInt` change before planning.
