# Phase 6: Core-value cleanup - Context

**Gathered:** 2026-10-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the four v1.0 milestone-audit warnings that let an agent mistake a
non-read, a misconfigured read, a pre-filtered read, or an in-flight write for
a complete, honest answer — the project's core value. Source:
`.planning/v1.0-MILESTONE-AUDIT.md` W1, W2, W3, W4.

- **W2** — a help token used as a flag value triggers help (exit 0, empty stdout, no read).
- **W3** — an empty or blank `projects` list passes config validation; every read reports `complete: true` over nothing.
- **W4** — `workitems list`, `dashboard counts` and `prs list` apply a fixed server-side scope that is not disclosed anywhere.
- **W1** — a panic after a write's request was sent exits 3 ("nothing ran") with no audit record.

This phase hardens already-satisfied requirements (CLI-05, CLI-06, OUT-04,
OUT-05, TRUST-03, SAFE-02, SAFE-04). It adds no new capability: no new verbs,
no query widening, no new flags beyond what disclosure requires.

</domain>

<decisions>
## Implementation Decisions

### W4 — Scope disclosure
- **D-01:** Every read envelope gains a **`scope` object**, structured data an
  agent can read without parsing prose. Example for `workitems list` /
  `dashboard counts`: `{"excluded_states": ["Done","Closed","Removed"], "iterations": "recent"}`;
  for `prs list`: `{"status": "active"}`. Exact key names are the planner's
  call within this shape, but they become part of the published JSON contract.
  — **Reversibility:** one-way — a published field in the `--json` envelope that agents branch on (Phase 2 D-01, Phase 3 D-06 contract); renaming or removing it later breaks callers.
- **D-02:** `scope` is present on **all 8 read envelopes**, `null` when a read
  has no built-in server-side filter. One shape for agents to check.
  — **Reversibility:** one-way — same published-contract reason as D-01.
- **D-03:** **Disclose only — no widening.** No `--include-done`,
  `--all-iterations` or `--status all` in this phase; changing the query is new
  capability and overlaps v2 DATA-01, which rewrites the same WIQL.
- **D-04:** Human (non-JSON) output prints **one scope line on stderr** for
  scoped reads (e.g. `scope: excludes Done/Closed/Removed; recent iterations only`).
  Stdout stays rows only (Phase 3 OUT-03 split).
- **D-05:** **Reword `--hide-done` help** so it is accurate: Done, Closed and
  Removed are always excluded by the server; the flag additionally drops
  Resolved and other done-like states client-side. The flag's behaviour is unchanged.
- **D-06:** Each scoped verb's `--help` text states its fixed scope.

### W2 — Help-token parsing
- **D-07:** `-h`, `--help`, `-help` mean help **only in flag position**, never
  as a flag's value. The bare word `help` means help only as the noun or verb
  (`adoBizInt help`, `adoBizInt workitems help`). `workitems list --search help`
  runs the read with search term "help".
- **D-08:** The same fix covers the **write side** (BL-02 WR-05,
  `--comment -h`), since it is the same `handleUsage` path. Remove WR-05 from
  `.planning/BACKLOG.md` BL-02 when this lands.
- **D-09:** `--help` must still work with no PAT and no config (Phase 3
  CLI-05); help detection stays ahead of `config.Load`.

### W3 — Empty projects
- **D-10:** `config.Validate` **rejects an empty `projects` list** with a
  `ConfigError{Field: "projects"}`. This applies to the CLI (exit 3, nothing
  ran) and the TUI (startup error) alike — a TUI with no projects shows only
  empty views, which is the same lie.
- **D-11:** **Blank or whitespace-only entries are rejected** too (e.g.
  `projects: [""]`), as a config error naming the `projects` field.

### W1 — Panic during a write
- **D-12:** Once a write's request has been issued, a **panic exits 5**
  (outcome unknown), mirroring Phase 4 D-22 for interrupts. A panic before the
  request is issued stays exit 3 (nothing ran).
- **D-13:** On a post-request panic, the recover path makes a **best-effort
  audit append with outcome `unknown`** if the audit sink is still usable. If
  that append itself fails, the crash record still goes to stderr and the
  exit is still 5. SAFE-02: every write that may have landed leaves a trace.
- **D-14:** Correct the stale comments in `cmd/adoBizInt/main.go` and
  `cmd/adoBizInt/crash.go` that call 3 "the outcome-unknown exit code".

### Post-research decisions (2026-10-12)
- **D-15:** `releases approvals` reports `scope: {"approval_status": "pending"}` — its request already filters to pending approvals server-side, so D-02 applies. (Research recommends `pr_status` rather than `status` for the PR key, since `dashboard counts` also counts only active PRs.)
  — **Reversibility:** one-way — published `--json` field (same contract as D-01).
- **D-16:** `pipelines runs` and `releases list` report `scope: null` — "latest run/release per definition" is the verb's purpose, not a hidden filter. The empty-but-complete result when no pipelines are configured goes to BL-02.
- **D-17:** A stray positional argument on a **read** verb (e.g. `workitems list help --json`) is a usage error: exit 3, nothing runs. Same class as W2 (an agent's request silently misread); in scope by user decision.
  — **Reversibility:** costly — changes accepted argv for read verbs; scripts passing stray words would start failing.
- **D-18:** The structured crash record stays on **stdout** (Phase 3 D-23/D-26 contract). D-13's "stderr" refers only to the audit-append failure message.

### Claude's Discretion
- Exact key names inside `scope` and the stderr wording, within D-01/D-04.
- How the "request issued" moment is signalled to the panic handler for D-12 (e.g. a flag set in `runWrite` before `op.mutate`), provided it is structural and tested.
- Whether `scope` lives on the shared `Result` envelope or is added at the CLI emit layer, provided all 8 reads carry it (D-02) and the TUI is not required to render it.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source of this phase
- `.planning/v1.0-MILESTONE-AUDIT.md` — W1–W4 definitions, evidence and affected requirements
- `.planning/BACKLOG.md` — BL-02 (WR-05 write-side help token, closed by D-08)

### Contracts this phase must preserve
- `.planning/phases/02-honest-reads/02-CONTEXT.md` — D-01 Result envelope contract, D-06/D-07 failure kinds
- `.planning/phases/03-read-cli/03-CONTEXT.md` — exit-code vocabulary (D-06..D-08), stdout/stderr split, help before config
- `.planning/phases/04-guarded-writes/04-CONTEXT.md` — write guards, D-22 interrupt → exit 5, audit-before-mutate
- `.planning/phases/05-hardening/05-CONTEXT.md` — `--project` allow-list in `newFlagSet` (D-09), exit 3 for policy rejection

### Requirements
- `.planning/REQUIREMENTS.md` — CLI-05, CLI-06, OUT-04, OUT-05, TRUST-03, SAFE-02, SAFE-04

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `newFlagSet` (`cmd/adoBizInt/flags.go`): single registrar for all 14 verbs; the Go `flag` package already distinguishes flag position from flag value, which D-07 can lean on instead of hand-scanning argv.
- `newResult` / `Result[T]` (`internal/ado/result.go`) and `emitJSON` (`cmd/adoBizInt/dispatch.go`): the two candidate homes for `scope`.
- `appendAuditRecord`, `writeExitCodeFor`, `exitOutcomeUnknown` (`cmd/adoBizInt/write.go`, `exit.go`): the pieces D-12/D-13 compose.
- `handlePanic` / `guard` (`cmd/adoBizInt/crash.go`): the panic path W1 changes.

### Established Patterns
- Structural go/ast tests over grep-negative criteria (Phase 4/5 precedent, STATE.md pending todo).
- RED-first commits; one test per behaviour; fakes from Phase 1 seam.
- `--help` handled before `config.Load` (`handleUsage` in `cmd/adoBizInt/usage.go:143`).

### Integration Points
- `cmd/adoBizInt/usage.go` `handleUsage` — W2.
- `internal/config/config.go` `Validate` (line ~171) — W3; also gates TUI startup.
- `internal/ado/workitems.go:163` (WIQL state/iteration filter), `internal/ado/dashboard.go` (same filter), `internal/ado/pullrequests.go:65` (Active status) — the three scoped reads for W4.
- `cmd/adoBizInt/crash.go` `handlePanic`, `cmd/adoBizInt/write.go` `runWrite` — W1.

</code_context>

<specifics>
## Specific Ideas

- Live evidence from 2026-10-12: all eight reads currently return `complete: true` against the Windstream org; the `scope` disclosure must not change any `complete` value.
- `--search help` is a realistic query (work items titled "help desk"), which is why W2 matters.

</specifics>

<deferred>
## Deferred Ideas

- Widening flags (`--include-done`, `--all-iterations`, `--status all`) — v2 DATA-01 territory.
- Remaining audit tech debt (W5 dashboard string-literal coupling, W6 operation-name sanitisation, orphaned exports, prs.go duplicated summariser) — stays in the audit report / BL-02.

</deferred>

---

*Phase: 06-core-value-cleanup*
*Context gathered: 2026-10-12*
