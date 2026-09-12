# Roadmap: adoBizInt

## Overview

Five phases turn a TUI-only Azure DevOps client into a tool an AI agent can
drive. The ordering is dictated by a single constraint: an agent cannot see a
screen, so it can only trust what the exit code and JSON tell it. That makes
correctness a prerequisite for surface area, not a follow-up to it.

Phases 1 and 2 make the existing service layer honest and testable. Phase 3
delivers the read CLI, which is the first point of real agent value. Phase 4
adds writes behind dry-run and confirmation. Phase 5 closes the remaining
security findings and adds CI.

This sequence follows the remediation ordering in `.planning/codebase/CONCERNS.md`.
The audit is explicit that shipping an unverified `--dry-run` flag is worse than
shipping no flag at all, which is why the test seam comes first.

## Phases

- [ ] **Phase 1: Test seam for internal/ado** - Narrow interfaces and injectable sub-clients so write paths become testable (awaiting human verification)
- [ ] **Phase 2: Honest reads** - Aggregate reads surface failures, auth errors are fatal, every call has a deadline (awaiting human verification)
- [x] **Phase 3: Read CLI** - Subcommand dispatch, JSON output, exit codes, agent-relevant filters (verified 2026-09-09)
- [ ] **Phase 4: Guarded writes** - Write verbs behind dry-run default, explicit confirmation, and an audit record
- [ ] **Phase 5: Hardening** - Attachment and path fixes, nil guards, WIQL escaping, CI

## Phase Details

### Phase 1: Test seam for internal/ado

**Goal**: `internal/ado` write paths can be tested without touching Azure DevOps
**Depends on**: Nothing (first phase)
**Requirements**: SEAM-01, SEAM-02, SEAM-03, SEAM-04
**Success Criteria** (what must be TRUE):

  1. Each of the seven write methods has a test asserting the exact ADO call it issues, with no network access
  2. `go test ./internal/ado/...` passes and reports non-zero coverage where it currently reports 0.0%
  3. The TUI builds and runs with no behavior change
  4. Interfaces declare only the methods `internal/ado` actually calls, not the full SDK surface

**Plans**: 6/6 plans executed (awaiting human verification)

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Tracer: one narrow interface, one retyped field, one fake, one green write test

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — Remaining three interfaces, cached current user ID, all four fakes

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-03-PLAN.md — The seven write methods plus `GetNextState`, with a release approve/reject cross-check

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-04-PLAN.md — Characterization tests for the two work-item-query reads, including the dashboard's structurally different swallow

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01-05-PLAN.md — Characterization tests for the remaining six aggregate reads, plus the eight-test phase gate

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 01-06-PLAN.md — Gap closure: exact PR fetch assertion and retryable identity lookup seam

### Phase 2: Honest reads

**Goal**: A caller can distinguish an empty result from a failed one
**Depends on**: Phase 1
**Requirements**: TRUST-01, TRUST-02, TRUST-03, TRUST-04, OUT-02
**Success Criteria** (what must be TRUE):

  1. Every aggregate read reports which projects failed and why, rather than dropping errors with `continue`
  2. An expired PAT produces a distinct fatal error, never an empty list
  3. A partial result is explicitly flagged incomplete
  4. Every ADO call carries a deadline and `connection.Timeout` is set
  5. Exported `ado` structs carry `json:` tags with stable lower-cased names
  6. The TUI renders failures rather than silently showing an empty table

**Plans**: 5/5 plans executed (awaiting human verification)

Plans:

- [x] 02-01-tracer-honest-read-PLAN.md — Wave 1: result envelope, failure classifier, both timeout budgets, and the pull-request read wired end to end through the prs view
- [x] 02-02-expand-aggregate-reads-PLAN.md — Wave 2: expand the envelope across work items, both pipeline reads, both release reads, and branches
- [x] 02-03-dashboard-partial-counts-PLAN.md — Wave 2: dashboard partial counts reported per sub-query without discarding the projects that answered
- [x] 02-04-json-field-contract-PLAN.md — Wave 3: json tags on the eleven serializable structs and retirement of the Phase 1 characterization test names
- [x] 02-05-tui-failure-surfacing-PLAN.md — Wave 3: the remaining five views render auth banners, partial warnings, and stale markers

### Phase 3: Read CLI

**Goal**: An AI agent can query every existing read operation and parse the result
**Depends on**: Phase 2
**Requirements**: CLI-01, CLI-02, CLI-04, CLI-05, CLI-06, OUT-01, OUT-03, OUT-04, OUT-05 (all complete)
**Success Criteria** (what must be TRUE):

  1. `adoBizInt` with no arguments launches the TUI exactly as before
  2. Read subcommands exist for work items, PRs, pipelines, releases, branches and dashboard
  3. `--json` emits parseable JSON on stdout with nothing else written to stdout
  4. Exit codes distinguish success, partial, auth failure and outcome-unknown
  5. `--timeout` bounds the invocation and interrupts are handled
  6. Agent-relevant filters work as flags, promoted out of the view packages
  7. A panic emits structured JSON rather than dying silently

**Plans**: 5/5 plans executed — VERIFIED 2026-09-09, 9/9 requirements, human UAT closed

Plans:

**Wave 1**

- [x] 03-01-PLAN.md — Tracer: `run() int`, dispatch seam, `prs list --json` end to end, exit-code contract

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-02-PLAN.md — Remaining seven read verbs, shared global flags, three-level help, plain-text renderer
- [x] 03-03-PLAN.md — Promote hide-done, hide-tasks, assigned-to-me, by-project and by-repo into `internal/ado`; rewire workitems and prs

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 03-04-PLAN.md — Promote free-text search across all five filtering views; expose filter flags defaulting to off

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 03-05-PLAN.md — `--timeout`, `fetchctx.NewWithParent`, SIGINT handling, and the structured panic record

### Phase 4: Guarded writes

**Goal**: An agent can mutate Azure DevOps without a mistake being silent or irreversible-by-surprise
**Depends on**: Phase 3
**Requirements**: CLI-03, SAFE-01, SAFE-02, SAFE-03, SAFE-04
**Success Criteria** (what must be TRUE):

  1. Every write verb performs a dry run unless an explicit confirmation flag is passed
  2. A dry run reports exactly what would change and makes no ADO call that mutates state
  3. Every executed write appends an audit record identifying what changed
  4. Omitting an effort field leaves it unchanged rather than zeroing it
  5. A write that times out reports outcome-unknown, distinct from both success and failure
  6. All of the above are covered by tests using the Phase 1 seam

**Plans**: 3/4 plans executed

Plans:

**Wave 1**

- [x] 04-01-PLAN.md — Tracer: `workitems set-state` end to end — dry-run default, `--confirm`, pre-read diff, pre-flight audit sink, outcome classification, two new exit codes

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-02-PLAN.md — Nullable effort fields at the `internal/ado` signature, TUI call sites, and `fs.Visit` flag-presence detection

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 04-03-PLAN.md — By-id pre-reads and the remaining five write verbs, with three-level `--help` coverage

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 04-04-PLAN.md — Phase-wide guard matrix over every write verb, validation map completion, and live-ADO confirmation

### Phase 5: Hardening

**Goal**: The security findings that a CLI makes reachable are closed
**Depends on**: Phase 4
**Requirements**: HARD-01, HARD-02, HARD-03, HARD-04, HARD-05, HARD-06
**Success Criteria** (what must be TRUE):

  1. Attachment download takes an identifier, not a URL, and the PAT cannot reach an arbitrary host
  2. Attachment writes cannot escape the intended directory
  3. The nil-dereference sites in CONCERNS.md MED4 are guarded
  4. WIQL project interpolation is escaped and validated against configured projects
  5. CI runs build, vet, race tests and gofmt on every push and fails on regression
  6. The log file is opened 0600 with a 0700 directory, and a test asserts the PAT never appears in log output (HARD-06, raised by Phase 3 T-03-15)

**Plans**: TBD

Plans:

- [ ] 05-01: Attachment identifier, host allow-list, timeouts, size cap
- [ ] 05-02: Path containment and nil guards
- [ ] 05-03: WIQL escaping and project validation
- [ ] 05-04: CI workflow and linter config
- [ ] 05-05: Log file permissions and PAT-redaction regression test (HARD-06)

## Deferred

Tracked in REQUIREMENTS.md v2, deliberately outside this roadmap:
scrum master features via the dormant `work.Client`, work item creation and
comments, a native MCP server, the Azure DevOps SDK upgrade, and rate limit
backoff.
