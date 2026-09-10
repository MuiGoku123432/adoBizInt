# Phase 4: Guarded writes - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-09
**Phase:** 04-guarded-writes
**Areas discussed:** Dry-run semantics + output, Confirmation flag + write exit codes, Audit record, Nullable effort fields, Outcome-unknown on timeout, Write command surface + arg shape

---

## Area Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Dry-run semantics + output | Pre-read diff vs echo; JSON shape for writes | ✓ |
| Confirmation flag + write exit codes | Flag spelling, prompting, dry-run exit status | ✓ |
| Audit record: sink, shape, failure | Where it lands, what it carries, failure handling | ✓ |
| Nullable effort fields (SAFE-03) | Where "omitted means unchanged" is enforced | ✓ |
| Outcome-unknown on timeout (SAFE-04) | Verify-read, exit code, SIGINT handling | ✓ |
| Write command surface + arg shape | Positional vs flag, arity, batching, --state | ✓ |

**User's choice:** All six areas.

---

## Dry-run semantics + output

> **Note:** this area was discussed once, then restarted at the user's request.
> Only the second pass is recorded; both passes reached the same answers on the
> two questions that were re-asked.

### Q1 — Does a dry run fetch current ADO state?

| Option | Description | Selected |
|--------|-------------|----------|
| Fetch current state, show before -> after | Catches no-ops and impossible changes; costs a round trip; needs new by-id reads for PR, release, pipeline | ✓ |
| Echo intent only, no read | Zero extra network, no new read methods, but blind | |
| Echo by default, opt-in pre-read | Cheap default; two output shapes to define and test | |

**User's choice:** Fetch current state, show before -> after.
**Notes:** Surfaced during the question that only `GetWorkItemDetail` exists as a by-id read today; the other three are new work on the Phase 1 interface seam.

### Q2 — What happens when the pre-read itself fails?

| Option | Description | Selected |
|--------|-------------|----------|
| Fatal — dry run fails, no report | Caller never gets a report built on unknown state | ✓ |
| Degrade to echo, mark unknown, exit non-zero | Report survives with a marker; caller may script past it | |
| Degrade silently, exit 0 | Cheapest, weakest | |

**User's choice:** Fatal.

### Q3 — Does the pre-read also gate the `--confirm` path?

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-read gates the real write too; no-op refused | Idempotent-by-refusal, closes CONCERNS.md C2's idempotency half; two round trips; race window remains | ✓ |
| Dry-run only; `--confirm` writes directly | One round trip; C2 stays open; no-op writes emit false audit records | |
| Pre-read gates, but a no-op exits 0 | Idempotent in effect; conflates changed with already-so | |

**User's choice:** Pre-read gates the real write too.

### Q4 — What does `--json` emit for a write?

| Option | Description | Selected |
|--------|-------------|----------|
| New top-level write envelope, distinct from `Result[T]` | Mirrors Phase 3 D-26's crash record reasoning; one shape for dry-run and executed, differing by `executed`/`outcome` | ✓ |
| Reuse the Phase 2 `Result[T]` envelope | One shape for the whole CLI; but `complete`/`failures` mean something different, and `.items \| length` reads a write as a row | |
| Separate dry-run and executed shapes | Each precise; two shapes to version, caller must branch | |

**User's choice:** New top-level write envelope.

---

## Confirmation flag + write exit codes

### Q1 — What is the flag called?

| Option | Description | Selected |
|--------|-------------|----------|
| `--confirm` | Matches SAFE-01's own wording; traceability stays mechanical | ✓ |
| `--execute` | Emphasises causation; diverges from requirement text | |
| `--yes` | Familiar idiom; implies a prompt that does not exist | |

**User's choice:** `--confirm`.

### Q2 — Does a write ever prompt interactively?

| Option | Description | Selected |
|--------|-------------|----------|
| Never prompt — `--confirm` is the whole mechanism | Identical on and off a TTY; extends Phase 3 D-14's rejection of context-dependent behaviour | ✓ |
| Prompt on a TTY, dry-run otherwise | Friendlier for humans; two behaviours, hang risk, reintroduces context dependence | |
| Prompt unless `--confirm`, with `--no-input` escape | Most flexible, most surface | |

**User's choice:** Never prompt.

### Q3 — What does a successful dry run exit with?

| Option | Description | Selected |
|--------|-------------|----------|
| 0 — a completed dry run is a success | Nothing failed; a reserved code would abort every preview under `set -e` | ✓ |
| A reserved 4+ code meaning "not executed" | `$?` alone proves nothing was written; but tells the caller what it already knows | |
| 0 if no change would occur, non-zero if it would | Encodes the diff in the status; inverts the usual meaning of 0 | |

**User's choice:** 0.

### Q4 — What does a refused no-op exit with?

| Option | Description | Selected |
|--------|-------------|----------|
| A reserved 4+ code — "already in the requested state" | Distinct from "I changed it"; uses the range Phase 3 D-06 reserved | ✓ |
| 0 — desired state reached, idempotent success | Retry-friendly; conflates changed with already-so | |
| Reuse exit 1 | Overloads "partial read" with a second meaning; restores the ambiguity D-08 cleared | |

**User's choice:** A reserved 4+ code.

---

## Audit record

### Q1 — Where does it land?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated append-only JSONL audit file | Not diluted by debug output, not silenceable by log level or rotation; inherits Phase 5 HARD-06's permission work | ✓ |
| The existing log file via `logging.Logger()` | Zero new plumbing; but a config change can silence the trail | |
| The stdout write envelope is the record | No new file; but SAFE-02 says "appends", and discarded stdout leaves no trace | |

**User's choice:** Dedicated append-only JSONL file.

### Q2 — Is a failed audit append fatal?

| Option | Description | Selected |
|--------|-------------|----------|
| Fatal, checked before the write | No audit sink, no mutation; the only option under which SAFE-02 is a guarantee | ✓ |
| Write first, loud non-fatal warning on failed append | Mutation already happened; trail has a hole exactly when something went wrong | |
| Non-fatal warning, exit code unaffected | Simplest; SAFE-02 becomes advisory | |

**User's choice:** Fatal, checked before the write.
**Notes:** This makes the ordering load-bearing — the sink is opened for append as a pre-flight step, before the ADO call.

### Q3 — Which invocations produce a record?

| Option | Description | Selected |
|--------|-------------|----------|
| Executed writes only | The file answers "what changed in ADO" | ✓ |
| Every write-verb invocation, tagged with outcome | Complete record of intent; volume, and redefines "audit record" | |
| Executed writes plus dry runs behind a flag | Completeness depends on a flag | |

**User's choice:** Executed writes only.
**Notes:** Later qualified by the outcome-unknown answer — an unresolved write IS audited.

### Q4 — What does a record carry?

| Option | Description | Selected |
|--------|-------------|----------|
| Timestamp, command, target, from -> to, outcome | Enough to answer "what changed and did it land" | ✓ |
| The above plus resolved actor identity | Explicit rather than inferred; but Phase 1 D-12 makes it legitimately empty | |
| The above plus full command line | Reproduces the invocation; argv is caller-controlled and could carry a secret | |

**User's choice:** Timestamp, command, target, from -> to, outcome.

---

## Nullable effort fields

### Q1 — Where is "omitted means unchanged" enforced?

| Option | Description | Selected |
|--------|-------------|----------|
| At the `ado` signature — nullable params, omit from patch | Fixes the TUI blind-overwrite too; touches three TUI call sites; Phase 1 tests guard it | ✓ |
| At the CLI — fill omitted fields from the pre-read | No signature change; leaves the footgun in `internal/ado`, adds a read-modify-write race | |
| A second method alongside the existing one | Two methods for one concept; the TUI keeps calling the footgun | |

**User's choice:** Fix at the `ado` signature.
**Notes:** Established during the question that `UpdateWorkItemStateWithEffort` sends all three fields with `Add` unconditionally, so the TUI blind-overwrites today — this is a live bug, not a CLI-only gap.

### Q2 — How does the CLI distinguish omitted from `0`?

| Option | Description | Selected |
|--------|-------------|----------|
| Flag presence via `fs.Visit` | Stdlib's own mechanism; an explicit `--remaining 0` is a real zero, the primary close-a-task case | ✓ |
| A custom nullable-float `flag.Value` type | Self-contained; `stringList` is precedent | |
| A sentinel default value | Least plumbing; a sentinel is a value, so the conflation just moves | |

**User's choice:** `fs.Visit`.

### Q3 — One verb or two?

| Option | Description | Selected |
|--------|-------------|----------|
| One verb — `workitems set-state` with optional effort flags | Once effort is nullable, the state-only method is the same call with three nils | ✓ |
| Two verbs — `set-state` and `set-state-with-effort` | Explicit about the ADO call; two verbs for one concept | |
| Separate `set-state` and `set-effort` | Clean separation; a closing Task needs both in one atomic patch | |

**User's choice:** One verb.

---

## Outcome-unknown on timeout

### Q1 — Verify-read after a timeout?

| Option | Description | Selected |
|--------|-------------|----------|
| Report unknown, do not re-check | Honest and simple; caller decides | ✓ |
| Attempt a verify-read on a fresh deadline | Resolves unknown where possible; extends wall-clock past `--timeout`, can itself time out, a concurrent change makes it lie | |
| Verify only under an opt-in flag | Default stays fast; another flag | |

**User's choice:** Report unknown, do not re-check.

### Q2 — What exit code?

| Option | Description | Selected |
|--------|-------------|----------|
| A reserved 4+ code, distinct from "never ran" | Exit 3 means nothing was tried; a timed-out write may have mutated production | ✓ |
| Reuse exit 3 | Keeps the contract small; caller cannot distinguish safe-to-retry from check-first | |
| A distinct code per unknown-outcome cause | Most precise; proliferation | |

**User's choice:** A reserved 4+ code.

### Q3 — Does an outcome-unknown write produce an audit record?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — audit it with outcome unknown | The case you most want a trail for; sink already verified writable | ✓ |
| No — only confirmed executions | Keeps the file to confirmed changes; leaves the worst case untracked | |
| Yes, but segregated from confirmed writes | Shape question the planner resolves | |

**User's choice:** Yes, with outcome unknown.

### Q4 — What about a mid-write SIGINT?

| Option | Description | Selected |
|--------|-------------|----------|
| Same as a timeout — outcome-unknown, audited | A write in flight is materially different from an interrupted read | ✓ |
| Keep Phase 3 D-25 — exit 3 and a crash record | One rule for signals across the CLI; but exit 3 asserts "nothing ran", which is false | |
| Depends on when the signal lands relative to the request | Most accurate; genuinely racy to determine | |

**User's choice:** Same as a timeout.
**Notes:** This supersedes Phase 3 D-25 for write verbs only; read verbs are unchanged.

---

## Write command surface + arg shape

### Q1 — Positional or flag for the target?

| Option | Description | Selected |
|--------|-------------|----------|
| Positional ID — `prs approve 4821` | Missing positional is a parse error; missing `--id` defaults to a plausible-looking zero | ✓ |
| Flag ID — `prs approve --id 4821` | Uniform, order-independent; zero value is a valid-looking id | |
| Both accepted | Two spellings for one input; Phase 3 D-04 rejected aliases for this reason | |

**User's choice:** Positional ID.

### Q2 — How are project and repository supplied?

| Option | Description | Selected |
|--------|-------------|----------|
| Same flag names, arity of exactly one enforced | Same vocabulary as reads, stricter arity; removes the "empty means configured list" fallback for writes | ✓ |
| Additional positionals — `prs approve <project> <repo> <id>` | Unambiguous arity; order-sensitive and transposable with no type check | |
| New singular flags — `--project-name` / `--repo-name` | Arity visible in `--help`; two names for one concept | |

**User's choice:** Same flag names, arity of exactly one.

### Q3 — Multiple targets per invocation?

| Option | Description | Selected |
|--------|-------------|----------|
| One target per invocation | One mutation, one audit record, one exit code; no partial-failure rule needed | ✓ |
| Accept multiple, report per-target outcomes | Fewer round trips; needs a partial-failure exit rule and an atomic N-change preview | |
| Single now, defer batching | Same as option one with intent recorded | |

**User's choice:** One target per invocation.

### Q4 — Is `--state` required?

| Option | Description | Selected |
|--------|-------------|----------|
| Required `--state` — the caller names the target state | Same command must mean the same thing on two machines; makes the `current -> proposed` diff meaningful | ✓ |
| Optional, defaulting to `GetNextState` / `config.StateTransitions` | Matches the TUI keypress; config-driven behaviour an agent cannot predict | |
| Explicit `--state` with `--advance` as an alternative | Both available; two mechanisms where one is needed | |

**User's choice:** Required `--state`.

---

## the agent's Discretion

The user answered every question directly and delegated no decision. The
following were surfaced by the discussion or fall out of decisions above, and
are recorded in CONTEXT.md for the planner:

- The specific integers for the two new 4+ exit codes
- Final verb names for the seven mutations (`prs complete` vs `prs merge` in particular)
- Whether `releases approve` / `releases reject` expose `--comment`
- `QueuePipeline`'s `--branch` handling
- Whether the two effort `ado` methods merge into one
- File placement for the new by-id read methods in an already-2,015-line package
- Whether `--confirm` is registered on read verbs (likely a write-specific registration)
- What a rejected write exits with (ADO returned an error, distinct from auth, unknown and no-op)
- Whether the `changes` array is empty or absent on a refused no-op
- Audit file rotation and unbounded growth
- Whether the pre-read and the write share one `--timeout` budget

## Deferred Ideas

- Agent capability tiers (AGENT-01 through AGENT-04) — v2, explicitly not `--confirm`
- Batched multi-target writes
- Automatic verify-read after a timeout
- Interactive confirmation prompts
- Audit file rotation, retention and size caps
- Auditing dry runs and refused no-ops
- Actor identity in the audit record
- Work item creation, comments and links (STORY-01 through STORY-03) — v2
- `config.Priorities` sort and HTML description stripping — carried from Phase 3
- Promoting the Phase 1 fakes to an exported package
- Rate-limit backoff, 429 handling, N+1 fan-out caps (PLAT-03)
- Attachment host allow-list, path containment, WIQL escaping, nil guards, CI, log/audit permissions — Phase 5
- A fifth read exit code for usage errors — carried from Phase 3 D-22
