---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 04
current_phase_name: guarded-writes
status: executing
stopped_at: Completed 04-03-PLAN.md
last_updated: "2026-09-12T07:34:31.267Z"
last_activity: 2026-09-12
last_activity_desc: Phase 04 execution started
progress:
  total_phases: 4
  completed_phases: 2
  total_plans: 20
  completed_plans: 14
state_head: 98c2c0013e8206fdede4bd1fb9cbefd5da93593b
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-03)

**Core value:** An agent driving this tool must never mistake a broken call for an empty result.
**Current focus:** Phase 04 — guarded-writes

## Current Position

Phase: 04 (guarded-writes) — EXECUTING
Plan: 4 of 4
Status: Ready to execute
Next: Phase 04 (guarded writes)
Last activity: 2026-09-12 — Phase 04 execution started

Progress: [███████░░░] 70%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: --
- Total execution time: 0.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: none
- Trend: --

**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01 | 4 min | 2 tasks | 4 files |
| Phase 01 P02 | 9 min | 3 tasks | 4 files |
| Phase 01 P03 | 15 min | 3 tasks | 4 files |
| Phase 01 P04 | 18 min | 2 tasks | 2 files |
| Phase 01 P05 | 11 min | 3 tasks | 4 files |
| Phase 01 P06 | 9 min | 2 tasks | 3 files |
| Phase 02 P01 | 41 min | 4 tasks | 12 files |
| Phase 02 P02 | 34 min | 3 tasks | 12 files |
| Phase 02 P03 | 22 min | 2 tasks | 3 files |
| Phase 02 P04 | 16 min | 3 tasks | 15 files |
| Phase 02 P05 | 41 min | 4 tasks | 13 files |
| Phase 03 P01 | 34 min | 3 tasks | 7 files |
| Phase 03 P02 | 41 min | 3 tasks | 8 files |
| Phase 03 P03 | 26 min | 2 tasks | 7 files |
| Phase 03 P04 | 34 min | 3 tasks | 19 files |
| Phase 03 P05 | 29 min | 3 tasks | 8 files |
| Phase 04 P01 | 42 min | 3 tasks | 15 files |
| Phase 04 P02 | 24 min | 3 tasks | 8 files |
| Phase 04 P03 | 26 min | 3 tasks | 11 files |

## Accumulated Context

### Decisions

Full log in PROJECT.md Key Decisions.

- Onboarding: CLI with `--json` chosen over a native MCP server -- composable, no new dependency
- Onboarding: Ship the CLI over existing features before adding scrum master features
- Onboarding: Fix read error propagation before shipping any read verb
- Onboarding: Extract interfaces in `internal/ado` despite the no-abstraction standard -- required to test seven production-mutating writes
- Design session: Scrum posture is "agent informs, TUI acts" -- most scrum value lands as CLI read verbs on the v1 foundation
- Design session: Capacity is display-only and never a rule input, because days off are unmaintained and capacity is therefore overstated
- [Phase 01]: Use the assignable unexported Client.workitemClient field as the injection seam; add no constructor or adapter. — In-package tests can assign the field directly, preserving the public API and avoiding constructor network I/O.
- [Phase 01]: Rely on NewClient's SDK-interface assignment as compile-time conformance proof; add no ClientImpl assertion. — The SDK constructor returns an interface, so the production assignment is the correct compile-time relationship.
- [Phase 01]: Require each fake method to fail by interface and method name when its function field is unset. — Unexpected ADO calls must fail at the call site instead of returning misleading zero values.
- [Phase 01]: Cache AuthenticatedUser.Id rather than AuthorizedUser.Id for pull request approval under delegated access. — This preserves the identity used by the prior approval implementation.
- [Phase 01]: Keep identity resolution non-fatal during NewClient construction while warning when approval identity is unavailable. — Read-only TUI startup remains compatible while the lost approval capability becomes visible.
- [Phase 01]: Keep workClient concrete and intentionally dormant until v2 scrum requirements consume it. — It has no current call sites, so an empty interface would add meaningless abstraction.
- [Phase 01]: Represent expected effort patch values as float64 literals so interface{} dynamic types match production values. — reflect.DeepEqual compares dynamic types as well as values.
- [Phase 01]: Use cached Client.currentUserID as the approval reviewer ID. — It preserves AuthenticatedUser.Id semantics from plan 01-02.
- [Phase 01]: Keep a dedicated release status cross-check in addition to exact-call tests. — It detects aligned test-oracle drift that independent literals can miss.
- [Phase 01]: Inject aggregate work-item failures at QueryByWiql, because classification-node failures are warned and discarded. — This is the first error path that actually fails GetWorkItems at project scope.
- [Phase 01]: Assert distinct positive pull request counts from both dashboard projects. — The two-project sum distinguishes in-place query swallowing from project skipping.
- [Phase 01]: Use continue-to-return propagation for the GetWorkItems mutation proof. — Deleting continue alone is behaviorally inert because the failed result slice is nil.
- [Phase 01]: Use poisoned non-nil fixture data with injected project errors so deleting continue produces an observable failed-project result. — A nil error result can make literal continue removal behaviorally inert; poisoned data proves the characterization is non-vacuous.
- [Phase 01]: Use the anchored top-level-only pattern ^=== RUN   TestGet.*SwallowsProjectFailure$ for the eight-test phase gate. — The anchor excludes the intentional GetWorkItems trap subtest without deleting it and remains stable if more subtests are added.
- [Phase 01]: Preserve dashboard inversion as a getProjectCounts propagation change rather than an outer-loop-only edit. — Dashboard errors are swallowed in place and its outer continue is unreachable until the helper propagates.
- [Phase 01]: Keep successful eager identity caching and retry through the retained location provider whenever the approval cache is empty. — This restores transient-failure recovery without adding public API or shared mutable state.
- [Phase 01]: Do not memoize fallback identity results; each empty-cache approval performs a fresh lookup. — The 1/0, 2/1, and 3/2 provider/Git contract preserves per-attempt retry semantics.
- [Phase 02]: Expose the fan-out budget via a Client.FetchTimeout() accessor rather than threading a parameter through six view constructors and app.go
- [Phase 02]: Leave connection.Timeout nil for a zero configured budget, because a zero duration means no bound at all for http.Client
- [Phase 02]: Check cancellation before the timeout branch so Phase 3 CLI-04 interrupts never classify as server timeouts
- [Phase 02]: Exclude DownloadAttachment's http.DefaultClient from TRUST-04 and defer it to Phase 5 with the host allow-list
- [Phase 02]: Leave location.NewClient and getConnectionData on an unbounded startup context as a recorded scope call
- [Phase 02]: Assumption A1 confirmed: a rejected PAT returns 401 as a bare WrappedError value, so the dual-target errors.As extraction is load-bearing
- [Phase 02]: Defer routing startup-time auth failures through the classified auth banner; out of scope for plan 02-01
- [Phase 02]: Empty repositories are identified by status 404 exactly, not a 4xx range, so a 403 on a per-repository branch fetch still records an auth failure
- [Phase 02]: No fifth FailureKind for not-found; the empty-repository case is suppressed rather than named (D-07)
- [Phase 02]: The repository identity rides in the Failure operation label rather than a new struct field, keeping T-02-01 intact
- [Phase 02]: getProjectWorkItems returns ([]WorkItem, []Failure, error) so batch failures are attributed without double-wrapping the envelope
- [Phase 02]: Dashboard aggregate rides a one-element Items slice in Result[DashboardCounts] rather than a bespoke struct — D-01's purpose is that Phase 3 writes failure/completeness JSON handling once; a second envelope shape would be a permanent tax on every downstream consumer
- [Phase 02]: The failed dashboard iteration lookup is recorded as a failure, not a silent fallback — It silently drops the WIQL iteration clause, so the count is taken under a different query and renders overstated with no visible seam (T-02-05)
- [Phase 02]: [Phase 02]: PipelineRun.Duration serializes as integer nanoseconds under the key duration_ns — agents and jq handle integers trivially; a Go duration string is not natively parseable outside Go. Locked one-way door at Phase 3 --json.
- [Phase 02]: [Phase 02]: Timestamps stay value semantics; a zero time.Time emits the year-one RFC 3339 sentinel rather than being omitted — omitempty cannot suppress a struct, so omission would need pointers or a custom marshaller
- [Phase 02]: [Phase 02]: Failure.Err is json:"-" and re-emitted as a plain string by a custom MarshalJSON — marshalling the interface would serialize the SDK error's exported fields, the exact structural leak T-02-01 forbids
- [Phase 02]: [Phase 02]: The machine-readable failure message is intentionally untruncated, unlike the terminal rendering from 02-01; OUT-03's stdout/stderr split is what gates it, not the struct
- [Phase 02]: [Phase 02]: The embedded WorkItem in WorkItemDetail is left untagged so its fields inline into one flat object rather than nesting
- [Phase 02]: [Phase 02]: ado.Client is the one exported struct deliberately left untagged — unexported fields and live SDK clients; the exclusion is stated in a comment on the declaration
- [Phase 02]: [Phase 02]: Assumption A3 confirmed — a zero time.Time value field marshals to 0001-01-01T00:00:00Z and is not omitted
- [Phase 03]: The dispatch table is keyed on the two-token noun verb string, storing one closure per command over an unexported readClient interface in package main — Consumer-side interface is Go convention and *ado.Client satisfies it structurally; the Phase 1 fake seam uses unexported fields and cannot be built from cmd/
- [Phase 03]: The exit mapper scans for FailureAuth before consulting the completeness flag — A caller that retries a 1 eventually succeeds; a caller that retries a 2 loops forever against a rejected credential, so a co-occurring timeout must never mask auth (D-07)
- [Phase 03]: Startup failures return 3 rather than 1 — A config, validation or client-construction failure means the read never ran, which is a different fact from an incomplete read (D-08)
- [Phase 03]: newResult pre-allocates Items and Failures — failures was nil on every successful read, so failures:null was the happy-path output and an agent evaluating .failures | length errored on ordinary success
- [Phase 03]: runRead is generic over the payload type so the eight read verbs differ only in fetch func and renderer
- [Phase 03]: newFlagSet is the only flag.NewFlagSet site, making D-20 mechanically checkable
- [Phase 03]: handleUsage runs before config.Load so --help never requires a PAT
- [Phase 03]: Promoted five filter predicates into internal/ado; doneStates has exactly one definition repo-wide
- [Phase 03]: MatchesRepository also replaced the inline GetBranches filter, making the promotion a three-way deduplication
- [Phase 03]: Row-equivalence tests compare ordered table.RowData, not table.Row, because NewRow stamps a global incrementing id
- [Phase 04]: Ratified P-01/P-02, the write envelope schema and the five-key audit record as the published one-way contract
- [Phase 04]: Exported ado.ClassifyFailure alongside StatusCodeOf so writeExitCodeFor keys on FailureKind instead of duplicating the cancellation-is-not-a-timeout rule
- [Phase 04]: Added hoistLeadingPositionals: stdlib flag stops at the first non-flag arg, so the D-23 target-first spelling needed argument reordering before Parse
- [Phase 04]: The nil check gates the JSON-Patch append, never the Value field: a nil *float64 in Value serializes as value:null, which CLEARS the field rather than omitting it
- [Phase 04]: fs.Visit (flags actually set), not fs.VisitAll or a value comparison, is what keeps --remaining 0 distinct from an omitted --remaining
- [Phase 04]: internal/ui/workitems/effort.go left untouched: blank input still means 0, so TUI behaviour is unchanged by the nullable signature
- [Phase 04]: No releaseAPI interface entry added: the Azure DevOps SDK exposes no by-id approval endpoint, so GetReleaseApprovalDetail filters the existing GetApprovals list by id — Keeps ROADMAP SC4 (no uncalled interface entry) satisfied by construction
- [Phase 04]: runWrite generalised over a writeOp closure struct (resolve/plan/mutate) so all six write verbs share one guard body — A guard in a shared body cannot be forgotten on a new verb; six copies can drift
- [Phase 04]: Structural prohibitions asserted with go/ast rather than grep-negative criteria — STATE.md records three Phase 3 incidents where a grep criterion was defeated by a prose comment; comments are not in the AST

### Pending Todos

- **[Phase 4 planning] Acceptance criteria style.** Three incidents in Phase 3 where a
  `grep returns nothing` criterion was satisfied then defeated by a prose comment
  explaining the forbidden thing. The failure mode gets worse as documentation
  improves, which is at odds with this repo's comment-heavy house style. Prefer a
  structural assertion (the `newFlagSet` + `TestFlagsRegisteredOnEverySubcommand`
  shape); where a textual check is unavoidable, scope it with `--include` and strip
  comment lines first. (Verification F-03-3, open.)

### Blockers/Concerns

Carried from `.planning/codebase/CONCERNS.md`. All are addressed by the roadmap.

- **C1 (critical)**: All eight aggregate reads swallow per-project errors. An expired PAT is indistinguishable from an empty backlog. Phase 2.
- **C2 (critical)**: Seven destructive writes have no dry-run, no idempotency, no audit. The only confirmation lives in the TUI at `internal/ui/prs/prs.go:69` and a CLI bypasses it. Phase 4.
- **C3 (critical)**: `DownloadAttachment` sends the PAT to a caller-supplied URL with no allow-list or timeout (`internal/ado/workitems.go:520`). Phase 5.
- **H1 (high)**: `internal/ado` is 0.0% covered with zero interface seam. Phase 1.
- **H2 (high)**: 22 `context.Background()` sites, no timeouts, `connection.Timeout` never set. Phase 2.

Affecting the v2 scrum milestone, not v1:

- **Days off unmaintained in ADO.** ADO capacity is `capacityPerDay x workingDays - daysOff`, so every capacity figure is overstated. Constrains v2 design: capacity is display-only, and no nag rule may read it. Revisit if teams start maintaining days off.

### Environment Notes

- Gortex has this repo tracked and indexed (31 files, 2352 nodes, 14808 edges) with churn enrichment. The Gortex MCP tool surface was loaded before tracking, so it reports `repo_not_tracked` until the session reconnects. The `gortex call ...` CLI path works now.
- `gsd` CLI is not on PATH. GSD is reachable through its MCP server.

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Hardening | Log opened 0600 / dir 0700 + PAT-redaction test — filed as HARD-06, Phase 5 plan 05-05 | Phase 5 | 2026-09-09 |
| Scrum master | Sprint CRUD, capacity, velocity, burndown, backlog rank | v2 | 2026-09-03 |
| Story management | Work item creation, comments, links | v2 | 2026-09-03 |
| Platform | Native MCP server | v2 | 2026-09-03 |
| Platform | Azure DevOps SDK upgrade off 2020 beta | v2 | 2026-09-03 |

## Session Continuity

Last session: 2026-09-12T07:34:31.261Z
Stopped at: Completed 04-03-PLAN.md
Resume file: None
