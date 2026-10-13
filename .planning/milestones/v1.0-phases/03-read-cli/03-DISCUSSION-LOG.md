# Phase 3: Read CLI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-08
**Phase:** 03-read-cli
**Areas discussed:** Command surface shape, Exit code contract, JSON output shape, Filter promotion, Flag parsing mechanism, Panic and outcome-unknown

---

## Command surface shape

### Q1 — How should the command tree be shaped?

| Option | Description | Selected |
|--------|-------------|----------|
| Noun-verb: `adoBizInt prs list` | Explicit verb; leaves room for Phase 4 writes without restructuring | ✓ |
| Flat noun: `adoBizInt prs` | Terser, but Phase 4 writes would bolt on as flags | |
| Hyphenated: `adoBizInt list-prs` | Simplest stdlib parsing; does not scale past ~15 commands | |

**User's choice:** Noun-verb (the recommended option)

### Q2 — Eight read methods, six nouns. How do the splits map?

| Option | Description | Selected |
|--------|-------------|----------|
| Second verb on the noun | `pipelines runs`/`definitions`, `releases list`/`approvals`; 1:1 to ado methods | ✓ |
| Flag that swaps payload | `pipelines list --definitions`; a boolean changing output schema | |
| Separate top-level nouns | Eight nouns; diverges from ROADMAP SC2 | |

**User's choice:** Second verb on the noun (recommended)

### Q3 — What verb for the dashboard?

| Option | Description | Selected |
|--------|-------------|----------|
| `dashboard counts` | No bare-noun exception; matches GetDashboardCounts | ✓ |
| `dashboard` (bare) | Terser but permanently inconsistent | |
| `dashboard show` | `show` implies rendering, not an aggregate object | |

**User's choice:** `dashboard counts` (recommended)

### Q4 — Noun spelling?

| Option | Description | Selected |
|--------|-------------|----------|
| `workitems` / `prs` | Matches Go package and type names; no quoting hazard | ✓ |
| `work-items` / `pull-requests` | Reads better; diverges from every internal name | |
| Accept aliases | Friendlier; two names per command means script divergence | |

**User's choice:** Match the code (recommended)

**Notes:** Asked whether to continue with open sub-questions (bare noun with no verb, unknown-subcommand fallthrough, root `--help` behavior). User moved to the next area; these were recorded under the agent's Discretion in CONTEXT.md.

---

## Exit code contract

### Q1 — Does a partial result exit non-zero?

| Option | Description | Selected |
|--------|-------------|----------|
| Partial exits non-zero | An agent checking only `$?` must not read partial as clean | ✓ |
| Partial exits 0 | Friendlier to pipelines; exit code and payload answer different questions | |
| Configurable via flag | Punts the decision; two behaviors to test | |

**User's choice:** Partial exits non-zero (recommended)

### Q2 — What integers?

| Option | Description | Selected |
|--------|-------------|----------|
| 0/1/2/3 sequential | Leaves 4+ for Phase 4; note 1 collides with Go's default os.Exit(1) | ✓ |
| 0/2/3/4, reserve 1 | Prevents a stray os.Exit(1) masquerading as partial | |
| Spaced 0/10/20/30 | Room to subdivide; implies a taxonomy that does not exist | |

**User's choice:** 0/1/2/3 sequential (recommended)

**Notes:** The reserve-1 ambiguity raised in the option text was resolved by Q4 below, which moved the three existing `os.Exit(1)` sites to 3.

### Q3 — Precedence when failure kinds are mixed?

| Option | Description | Selected |
|--------|-------------|----------|
| Auth wins | Any auth failure forces exit 2; not transient, requires operator action | ✓ |
| Most frequent kind wins | One auth among three timeouts would exit 1 and hide a dead token | |
| Always partial | Collapses the distinction OUT-04 asks for | |

**User's choice:** Auth wins (recommended)

### Q4 — What about main.go's existing `os.Exit(1)` sites?

| Option | Description | Selected |
|--------|-------------|----------|
| Move them to 3 | Nothing was attempted; also clears code 1 of a second meaning | ✓ |
| Client-construction exits 2 | Honest for a rejected PAT, but covers only one of three sites | |
| Leave them at 1 | Least churn; conflates "partial" with "nothing ran" | |

**User's choice:** Move them to 3 (recommended)

**Notes:** Asked whether to continue on SIGINT exit code and whether exit codes count as TUI "behavior" under CLI-01. User moved on; SIGINT was covered later in the panic area, and the CLI-01 question was recorded under the agent's Discretion.

---

## JSON output shape

### Q1 — What does `--json` emit?

| Option | Description | Selected |
|--------|-------------|----------|
| Raw Result[T], nothing added | Phase 2 D-01 built one envelope so Phase 3 writes JSON handling once | ✓ |
| Wrap with invocation metadata | Provenance; adds a nesting level and fields with no consumer | |
| Envelope plus exit_code field | Duplicates a value the caller has; second source of truth | |

**User's choice:** Raw envelope (recommended)

### Q2 — Is there a human-readable mode?

| Option | Description | Selected |
|--------|-------------|----------|
| Human default, `--json` opts in | Same binary is the TUI; matches OUT-01's framing | ✓ |
| JSON always, `--json` is a no-op | One contract, zero drift; but `--json` becomes meaningless | |
| JSON default, `--human` opts out | Same two formatters, inverted default | |

**User's choice:** Human-readable default (recommended)

### Q3 — Where do failures render?

| Option | Description | Selected |
|--------|-------------|----------|
| Rows to stdout, failures to stderr | Redirection keeps warnings visible and data clean | ✓ |
| Failures inline on stdout | One stream for a human; breaks `\| grep` and `> file` | |
| Suppress failures in human mode | Contradicts the core value for a human caller | |

**User's choice:** Rows to stdout, failures to stderr (recommended)

### Q4 — Compact or indented JSON?

| Option | Description | Selected |
|--------|-------------|----------|
| Compact, no `--pretty` | Reproducible, diff-able; `jq .` covers readability | ✓ |
| Add `--pretty` | Readable in-terminal; non-reproducible bytes, two code paths | |
| Auto-detect TTY | Convenient; classic "works in my shell, breaks in CI" bug | |

**User's choice:** Compact single-line (recommended)

**Notes:** Asked whether to continue on `"items":null` vs `[]` (Go marshals a nil slice as null and Phase 2's `newResult` does not pre-allocate) and on whether human mode reuses lipgloss styling. User moved on; both were recorded under the agent's Discretion, with the null hazard flagged as highest priority.

---

## Filter promotion

### Q1 — Where do the promoted filters land?

| Option | Description | Selected |
|--------|-------------|----------|
| Into `internal/ado` as pure functions | Filtering is a domain rule; but the package is already 2,015 lines | ✓ |
| New package between ado and ui | Keeps ado as the SDK boundary; ARCHITECTURE.md's own suggestion | |
| Duplicate in the CLI | Zero TUI risk; leaves duplicated domain rules that drift | |

**User's choice:** Into `internal/ado` — chosen against the recommendation. The 300-line cost was recorded in CONTEXT.md D-15 for the planner to surface rather than absorb.

### Q2 — Does the TUI get rewired this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, TUI consumes the promoted functions | One definition; needs behavior-equivalence tests for CLI-01 | ✓ |
| No, leave buildRows() as-is | Zero TUI risk; two copies of the same domain rule | |
| Promote only the boolean filters | Smaller blast radius; arbitrary boundary, half the drift remains | |

**User's choice:** Yes (recommended)

### Q3 — Which filters get promoted?

| Option | Description | Selected |
|--------|-------------|----------|
| The four named filters plus free-text search | hide-done, assigned-to-me, by-project, by-repo, search | ✓ |
| Also the priority sort and HTML stripping | More complete; sort is display ordering, a separate decision | |
| Only the four named, no search | Smallest scope; would need a second promotion pass | |

**User's choice:** Four filters plus search (recommended)

### Q4 — What are the CLI filter defaults?

| Option | Description | Selected |
|--------|-------------|----------|
| No filtering by default | A CLI must not hide data the caller did not ask to hide | ✓ |
| Mirror the TUI defaults (both on) | Consistent; silently omits rows while `complete:true` | |
| Config-driven defaults | Tunable; same command returns different data per machine | |

**User's choice:** No filtering by default (recommended)

**Notes:** Asked whether to continue on file placement inside `internal/ado` and on identity resolution for `--assigned-to-me`. User moved on; both recorded under the agent's Discretion.

---

## Flag parsing mechanism

### Q1 — What parses arguments?

| Option | Description | Selected |
|--------|-------------|----------|
| stdlib `flag`, FlagSet per subcommand | Zero new dependencies; PROJECT.md forbids them without justification | ✓ |
| spf13/cobra | Generates help; large dependency tree for eight commands | |
| urfave/cli | Lighter than cobra; still a new dependency, different mental model | |

**User's choice:** stdlib `flag` (recommended)

### Q2 — How are global flags wired?

| Option | Description | Selected |
|--------|-------------|----------|
| Shared registration into every FlagSet | Prevents drift; stdlib gives no compile-time check | ✓ |
| Each subcommand declares independently | Simplest per-command; eight chances to forget one | |
| Globals parsed before the subcommand | Unusual placement; stdlib stops at the first non-flag arg | |

**User's choice:** Shared registration (recommended)

### Q3 — What does the help tree look like?

| Option | Description | Selected |
|--------|-------------|----------|
| Three levels: root, noun, verb | Each level discoverable from the one above | ✓ |
| Two levels: root list plus per-command | Less code; `adoBizInt prs --help` becomes a dead end | |
| Single flat help page | One source; unreadable once Phase 4 adds seven write verbs | |

**User's choice:** Three levels (recommended)

### Q4 — What happens on a bad flag or unknown subcommand?

| Option | Description | Selected |
|--------|-------------|----------|
| Override: usage to stderr, exit 3 | stdlib's default 2 now means auth failure and would be a lie | ✓ |
| New exit code 4 for usage errors | Honest; OUT-04 names four outcomes | |
| Accept stdlib's exit 2 | Least code; a flag typo becomes indistinguishable from a dead PAT | |

**User's choice:** Override to exit 3 (recommended)

**Notes:** Asked whether to continue on what `--timeout` overrides (Phase 2 defined two separate budgets) and on single- vs double-dash conventions. User moved on; the `--timeout` mapping was recorded under the agent's Discretion, along with the note that `fetchctx.New` wraps `context.Background()` internally and cannot serve a signal-handling CLI as-is.

---

## Panic and outcome-unknown

### Q1 — Which stream carries the crash record?

| Option | Description | Selected |
|--------|-------------|----------|
| Structured JSON on stdout | OUT-05 says a panic must not look like an empty result | ✓ |
| JSON on stderr, stdout empty | Strictly clean stdout; but stdout-only agents see emptiness | |
| Follow the output mode | Mode-dependent crash format; human mode yields unparseable text | |

**User's choice:** stdout (recommended). The mid-encode partial-write risk was noted as a constraint the plan must honor via buffered atomic writes.

### Q2 — Does the record carry a stack trace?

| Option | Description | Selected |
|--------|-------------|----------|
| No stack; full stack to the log file | Go stacks print args, and `NewClient(cfg)` takes the PAT | ✓ |
| Include the full stack | Self-contained bug reports; credential-disclosure path to stdout | |
| Message only, no log path | Smallest surface; makes crashes undiagnosable | |

**User's choice:** No stack in the record (recommended)

### Q3 — Does SIGINT share the panic exit code?

| Option | Description | Selected |
|--------|-------------|----------|
| Both exit 3 | Both mean "did not complete, no result known" | ✓ |
| Panic 3, SIGINT 130 | Shell convention; a fifth code outside OUT-04's four | |
| Panic 3, SIGINT 0 | Interrupt is not a tool failure; reintroduces the core-value violation | |

**User's choice:** Both exit 3 (recommended)

### Q4 — What shape is the crash record?

| Option | Description | Selected |
|--------|-------------|----------|
| A distinct error object | An agent tells a crash from a result by which key is present | ✓ |
| Reuse Result[T] with a synthetic failure | One contract; makes a crash representable as a normal read | |
| Flat top-level fields | Simplest; different top-level keys for panic vs interrupt | |

**User's choice:** Distinct error object (recommended)

---

## the agent's Discretion

The user answered every question directly and delegated no decision. The following were surfaced during discussion but never opened as areas, and are recorded in CONTEXT.md for the planner to decide or raise:

- `"items":null` vs `"items":[]` — Go marshals nil slices as null and `newResult` does not pre-allocate (highest priority)
- Unknown first argument: error under D-22, or fall through to the TUI
- What `--timeout` overrides: `request_timeout`, `fetch_timeout`, or both
- File placement of the promoted filters inside a 2,015-line `internal/ado`
- Identity source for `--assigned-to-me` on an empty cache
- Whether the CLI path calls `logging.Init` (D-24 depends on the log file existing)
- Determinism of output ordering across runs
- Whether human-mode rendering reuses lipgloss styling or emits plain text
- Whether the D-08 exit-code changes count as TUI behavior under CLI-01

## Deferred Ideas

Recorded in full in CONTEXT.md `<deferred>`. Summary of what was raised and set aside during this discussion specifically:

- A fifth exit code for usage errors (offered under flag parsing, folded into outcome-unknown instead)
- `--pretty` JSON output (offered, rejected in favor of `jq`)
- Invocation metadata in the JSON envelope (offered, rejected as speculative)
- A shared `internal/filter` package between ado and ui (offered, rejected in favor of `internal/ado`)
- The `config.Priorities` sort and HTML description stripping (excluded from the promoted set)
- Exit codes 4+ reserved for Phase 4 write outcomes

No scope creep was raised; discussion stayed inside the phase boundary throughout.
