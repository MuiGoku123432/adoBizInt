# Phase 1: Test seam for internal/ado - Context

**Gathered:** 2026-09-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the seven production-mutating write methods in `internal/ado` substitutable
and covered by tests that touch no network, with no observable change to the TUI.

**In scope:** narrow interfaces over the four live ADO SDK sub-clients, a
test-only fake for each, tests for the seven writes, and characterization tests
for the eight aggregate reads.

**Not in scope:** changing read error semantics (that is TRUST-01, Phase 2),
CLI work (Phase 3), dry-run and audit (Phase 4), the attachment and WIQL
hardening findings (Phase 5).

</domain>

<decisions>
## Implementation Decisions

### Fakes and assertions

- **D-01:** Fakes are hand-written structs with func fields, one per interface
  (`fakeWorkItemAPI{ UpdateWorkItemFunc func(...) ... }`). No mock generator, no
  new dependency; `go.mod` is untouched. This matches the existing stdlib
  table-driven style in `internal/utils/text_test.go` and honors PROJECT.md's
  "no new dependencies without stated justification".
- **D-02:** A write test asserts the exact ADO call by declaring the expected
  argument literally and comparing with `reflect.DeepEqual`. For the JSON Patch
  writes this means the test spells out the full `[]webapi.JsonPatchOperation`,
  so every ADO field path string (`Microsoft.VSTS.Scheduling.RemainingWork` and
  friends) appears verbatim in the test and a typo in source can no longer match.
  `reflect.DeepEqual` follows the SDK's pointer fields correctly.
- **D-03:** Each fake holds a `t *testing.T` and every method opens with
  `if f.XFunc == nil { f.t.Fatalf("unexpected call to X") }`. An unstubbed call
  fails by name instead of returning a zero value. This makes "asserts the exact
  ADO call it issues" true in both directions -- the expected call happened, and
  no other call did. It is the same principle as the project's core value applied
  to the tests themselves.
- **D-04:** Fakes live in `internal/ado/fakes_test.go`, in-package
  (`package ado`), test-only. Not promoted to an exported `adotest` package --
  no out-of-package consumer exists yet. If Phase 4 needs to drive writes from
  CLI tests it either promotes these then, with real requirements in hand, or
  fakes at the `ado.Client` level, which may be the more appropriate seam there.
  — **Reversibility:** reversible — promoting a `_test.go` fake to an exported
  package is a file move plus capitalization, with no existing callers to break.

### Interface shape

- **D-05:** Interfaces are **unexported**. The module's public API is unchanged,
  which keeps SC3 cheap and preserves CONVENTIONS.md rule 17 ("zero interfaces")
  as far as this phase allows. In-package fakes satisfy unexported interfaces
  fine. Exporting later, if a real consumer appears, is additive and
  non-breaking.
- **D-06:** One interface per SDK sub-client, each declaring **only** the methods
  `internal/ado` actually calls -- this is ROADMAP SC4:
  - `workItemAPI` (5): `UpdateWorkItem`, `GetWorkItems`, `GetWorkItem`,
    `QueryByWiql`, `GetClassificationNode`
  - `gitAPI` (7): `UpdatePullRequest`, `GetPullRequest`,
    `GetPullRequestsByProject`, `CreatePullRequestReviewer`, `GetRefs`,
    `GetRepositories`, `GetBranches`
  - `buildAPI` (4): `GetDefinitions`, `GetDefinition`, `GetBuilds`, `QueueBuild`
  - `releaseAPI` (4): `GetReleases`, `GetReleaseDefinitions`, `GetApprovals`,
    `UpdateReleaseApproval`

  Rejected: splitting reader/writer interfaces per sub-client. A single struct
  field must serve every caller, so a write-only interface would force either a
  duplicate field or interface embedding -- structure no current caller asks for.
  Rejected: one combined 20-method interface, which stops mirroring the SDK and
  hands every fake methods it does not need.
- **D-07:** Interfaces are suffixed `-API`; struct field names stay exactly as
  they are (`workitemClient`, `gitClient`, `buildClient`, `releaseClient`). This
  avoids `gitClient gitClient` shadowing while leaving all ~25 call sites
  untouched, keeping the diff behaviorally inert per SC3.
- **D-08:** The dormant `workClient work.Client` field (`client.go:23`,
  constructed at `:54`, called from nowhere) stays as-is with no interface -- an
  interface over zero called methods would be empty. Add a comment at the field
  recording that it is intentionally unused, why it exists, and that the v2 scrum
  requirements consume it, so a future cleanup pass does not delete it as dead
  code. PROJECT.md names it as the v2 seam.

### Injection

- **D-09:** **No new constructor.** Tests build `&Client{workitemClient: fake,
  projects: []string{"P"}}` directly -- unexported fields are accessible from
  in-package tests, so the seam already exists once D-05/D-06 land. `NewClient`
  is untouched.

  Rejected: an exported `NewClientWithAPIs`. It would force the interfaces to be
  exported (reversing D-05) and add permanent public API with no consumer -- the
  speculative generality PROJECT.md's constraints and CONVENTIONS.md rule 5
  forbid. Note `NewClient` performs network I/O in the constructor
  (`location.GetConnectionData`, `client.go:70`), so no test could call it
  anyway without a further refactor.
- **D-10:** **SEAM-02 traceability.** SEAM-02 reads "`ado.Client` accepts
  injected sub-clients so tests can substitute fakes". D-09 satisfies this
  through assignable struct fields rather than a constructor parameter. PLAN.md
  MUST state this explicitly so `/gsd-verify-work` does not read SEAM-02 as a
  gap. REQUIREMENTS.md text is deliberately **not** amended -- rewriting a
  requirement to match an implementation hides descopes, and traceability to the
  original wording is worth more than the ambiguity costs.

### The location client inside ApprovePullRequest

Context: `ApprovePullRequest` builds a `location.NewClient(ctx, c.connection)`
inside its own body (`pullrequests.go:154`) and calls `GetConnectionData` to get
`AuthenticatedUser.Id`. `NewClient` already made that same call at
`client.go:66-76` and kept only `AuthorizedUser.ProviderDisplayName`, discarding
the rest. A struct-literal `Client` has a nil `connection`, so this method is
unreachable in tests until it is addressed.

- **D-11:** Add a `currentUserID string` field, populated in `NewClient` from the
  `GetConnectionData` response already being fetched. `ApprovePullRequest` reads
  `c.currentUserID`; `location.NewClient` leaves the method body entirely. This
  makes the method testable with `gitAPI` alone and removes a redundant network
  round-trip from every approve.
  — **Reversibility:** reversible — one field and two call sites, no persisted
  state and no published contract.
- **D-12:** On a failed identity fetch, `NewClient` still constructs
  successfully (as it does today -- `client.go:71` only proceeds on `err == nil`)
  but logs a warning naming what was lost and what will break.
  `ApprovePullRequest` returns the existing `could not determine current user ID`
  error **verbatim** when `currentUserID` is empty. Observable behavior is
  identical to today, so SC3 holds; the silent-degradation window becomes visible
  in the log file. Yields a test case: an empty `currentUserID` must error
  without ever touching `gitAPI`.

  Rejected: making the identity fetch fatal in `NewClient`. Philosophically
  consistent with the core value, but it would stop the TUI from starting in a
  case where it currently starts and serves every read view -- a real regression
  for read-only use and a direct SC3 violation.
- **D-13:** `NewClient` reads `AuthorizedUser.ProviderDisplayName` while
  `ApprovePullRequest` reads `AuthenticatedUser.Id`. These are **different
  fields** on the same response. Verify both against the SDK when implementing
  the cache -- do not assume they are aliases.

### SEAM-04 ordering

Context: SEAM-04 requires tests for "the aggregate read failure paths from
TRUST-01", but TRUST-01 ships in Phase 2. Those paths do not exist yet. All
eight aggregate reads currently log and `continue`, then `return x, nil`
(`workitems.go:75-77` is the canonical example).

- **D-14:** Phase 1 writes **characterization tests** pinning today's actual
  behavior, and SEAM-04 stays in Phase 1. No ROADMAP or REQUIREMENTS edit.
  Each test drives one project to fail through a fake and asserts that the
  function returns the surviving project's results **and a nil error**, with a
  comment stating it documents current behavior and Phase 2 inverts it.

  Rationale: it satisfies SEAM-04 in Phase 1, exercises the new seam on read
  paths (not just the seven writes) so Phase 2 does not discover seam gaps late,
  and makes plan 02-01's change land as a visible test diff rather than an
  unverifiable rewrite. Accepted cost: the repo temporarily holds tests asserting
  behavior already judged wrong. That is the standard characterization-test
  tradeoff and it is time-boxed to one phase.
- **D-15:** **All eight** aggregate reads get one: `GetWorkItems`,
  `GetPullRequests`, `GetPipelineRuns`, `GetPipelineDefinitions`, `GetReleases`,
  `GetPendingApprovals`, `GetBranches`, `GetDashboardCounts`. Phase 2 touches all
  eight, so every one must be consciously inverted -- none can be missed or
  half-changed. Their arg types and sub-clients differ enough that a single table
  will not cleanly cover them; the resulting duplication is explicitly tolerated
  by CONVENTIONS.md (rule of three, duplicate with a note).
- **D-16:** Tests follow the source-file naming convention recorded at
  TESTING.md:329 (`workitems_test.go`, `pullrequests_test.go`,
  `pipelines_test.go`, `releases_test.go`, `branches_test.go`,
  `dashboard_test.go`) rather than a single `characterization_test.go`. Each
  carries an identical header comment naming TRUST-01 and Phase 2 as the
  inverter, and a consistent name shape --
  `TestGetWorkItemsSwallowsProjectFailure` -- so Phase 2 finds all eight with one
  grep.

### Agent's Discretion

The user did not delegate any decision. The following were surfaced during
discussion but never opened as areas; the planner should treat them as open and
decide, or raise them:

- Whether the pure functions TESTING.md flags as high-value and seam-free
  (`pipelineNameMatches` at `pipelines.go:57`, `GetNextState` at
  `workitems.go:510`, `calculateApprovalStatus` at `pullrequests.go:119`) are
  covered in this phase. They need no seam and `GetNextState` directly feeds the
  destructive state write.
- Whether to add compile-time assertions (`var _ gitAPI = (*git.ClientImpl)(nil)`
  or equivalent) to catch SDK drift. Note the assignments inside `NewClient`
  already give compile-time proof, so this may be redundant.
- Whether `ApproveRelease` and `RejectRelease` get a test specifically targeting
  the copy-paste risk -- TESTING.md:223 records that they are byte-identical
  apart from the status constant, so a swapped constant would not be caught by
  two independently-written tests that each assert only their own status.
- Whether the plan should give Phase 4 advance notice that it may need to promote
  the seam out of `_test.go` (see D-04).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase-defining analysis

- `.planning/codebase/TESTING.md` — **Highest-value read for this phase.**
  Contains the full seam analysis (§"The `internal/ado` Testability Problem"),
  the seven-write table with exact file:line and the SDK call each issues
  (:209-215), the explicit narrow-unexported-interface recommendation (:278),
  the "test the JSON Patch document construction" recommendation and why it is
  the highest-value test in the repo (:279, :221-223), the byte-identical
  `ApproveRelease`/`RejectRelease` risk (:223), and the test-file naming
  convention (:329).
- `.planning/codebase/CONVENTIONS.md` §"Interfaces" (:221-225) and §"Applying
  These Rules" (:380-390) — records that zero interfaces exist anywhere, and
  rule 5: "Do not add an interface to make something testable unless the current
  task is specifically to make that thing testable." This phase **is** that
  exception. Rule 6 requires justification before adding a dependency.
- `.planning/codebase/CONCERNS.md` — finding H1 (`internal/ado` 0.0% covered,
  no seam) is the origin of this phase. C1 (aggregate reads swallow errors) is
  the origin of the D-14 characterization tests.

### Requirements and scope

- `.planning/REQUIREMENTS.md` §"Test Seam" — SEAM-01 through SEAM-04. Also
  §"Read Trustworthiness" TRUST-01, which D-14/D-16 forward-reference.
- `.planning/ROADMAP.md` §"Phase 1" — goal, four success criteria, and the three
  provisional plan titles. §"Phase 2" and §"Phase 4" for the downstream
  dependencies this phase's decisions affect.
- `.planning/PROJECT.md` §"Key Decisions" — the locked decision that interfaces
  in `internal/ado` are permitted despite the no-abstraction standard.
  §"Constraints" — no new dependencies without justification, TUI unchanged,
  files under 300 lines.

### Source files this phase modifies

- `internal/ado/client.go` — `Client` struct (:20-32), `NewClient` (:34-98),
  the existing identity fetch (:66-77), the dormant `workClient` (:23, :54-57).
- `internal/ado/pullrequests.go` — `ApprovePullRequest` (:150-187) including the
  in-body location client (:154), `CompletePullRequest` (:190-243).
- `internal/ado/workitems.go` — `UpdateWorkItemState` (:215-240),
  `UpdateWorkItemStateWithEffort` (:244-291), `GetWorkItems` swallow site
  (:75-77).
- `internal/ado/releases.go` — `ApproveRelease` (:272-294), `RejectRelease`
  (:297-319).
- `internal/ado/pipelines.go` — `QueuePipeline` (:349-397).
- `internal/ado/branches.go`, `internal/ado/dashboard.go` — aggregate reads
  needing characterization tests.

### Style reference for the tests

- `internal/utils/text_test.go` — the table-driven stdlib style to match.
- `internal/ui/releases/releases_test.go` — regression-comment convention, and
  the existing `New(nil, ...)` nil-client workaround this phase makes obsolete.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **Existing test style is already the target style.** `internal/utils/text_test.go`
  is stdlib table-driven with named subtests and regression comments. No testify,
  no mock generator anywhere in `go.mod` or `go.sum`. The new fakes and tests
  match it rather than introducing a second style.
- **`NewClient`'s existing `GetConnectionData` call** (`client.go:66-76`) already
  fetches the response that `ApprovePullRequest` re-fetches. D-11 consumes data
  that is already on the wire rather than adding a call.
- **The SDK-to-domain conversion pattern** (explicit nil-checked field-by-field
  copy, e.g. `releases.go:139-153`) is established and unaffected by this phase.

### Established Patterns

- **Zero interfaces exist in the repository.** Verified: no `^type \w+ interface`
  matches in any of the 30 Go files. This phase introduces the first four, under
  the explicit CONVENTIONS.md rule-5 carve-out, and must not generalize beyond
  the methods listed in D-06.
- **Every method on `*Client` is context-first** and returns plain structs.
  Nothing about the signatures needs to change for the seam.
- **`log := logging.Logger()` at the top of each method**, then log-and-return on
  error. D-12's new warning follows this.
- **Files under 300 lines is a standard, and `internal/ado` already violates it**
  at 2,015 lines across 7 files (`workitems.go` alone is 551). Adding four
  interface declarations should not make this materially worse -- consider where
  they live (a small `interfaces.go`, or beside their consumers).

### Integration Points

- **`Client` struct fields** (`client.go:21-26`) are the seam. Four of the six
  fields change type; `connection` and `workClient` do not.
- **`NewClient` assignments** (`client.go:85-97`) give compile-time proof that the
  concrete SDK clients satisfy the new interfaces -- no separate assertion needed.
- **Every UI package holds `client *ado.Client` concretely**
  (`internal/ui/app.go:34`, `internal/ui/releases/releases.go:59`). Because the
  interfaces are unexported and the struct's exported surface does not change,
  **no UI file needs to change** -- which is how SC3 is satisfied.
- **`c.connection` still has one other consumer**: `DownloadAttachment` uses
  `c.connection.AuthorizationString` (`workitems.go:526`). That is a Phase 5
  concern (HARD-01) and is deliberately left alone here.

</code_context>

<specifics>
## Specific Ideas

- The JSON Patch assertion is the point of the exercise, not a side effect.
  TESTING.md:221 flags that the four field paths in `UpdateWorkItemStateWithEffort`
  are string literals -- a typo compiles, passes `go vet`, and either fails
  against production or silently writes to the wrong field. D-02 exists so that
  the test contains the literal and a source typo cannot match it.
- Symmetrically, D-03 exists so that a test cannot pass while the code makes a
  call nobody expected. "Assert the exact call" was read as a two-sided claim
  throughout the discussion.
- The phrase used for the `ApproveRelease`/`RejectRelease` pair during discussion
  was "a copy-paste swap is exactly what tests here must catch" -- worth carrying
  into the plan's acceptance criteria for those two methods.

</specifics>

<deferred>
## Deferred Ideas

- **Promoting fakes to an exported `internal/ado/adotest` package** — deferred to
  Phase 4, and only if CLI write-verb tests actually need to substitute at the
  sub-client level rather than at `ado.Client`. See D-04.
- **Exporting the interfaces so UI packages can depend on them** — deferred.
  Phase 2 plan 02-04 already has to touch UI call sites to surface partial and
  failed reads; if a UI-level seam is wanted, that is where the requirement will
  actually appear. See D-05.
- **`DownloadAttachment`'s direct use of `c.connection.AuthorizationString`**
  (`workitems.go:526`) — a real seam gap and a real security finding, but it is
  HARD-01/HARD-02 in Phase 5. Not touched here.
- **Making the identity fetch fatal at construction** — rejected for Phase 1
  because it violates SC3. Worth revisiting in Phase 3, where the CLI's exit-code
  contract (OUT-04, auth-failure code) gives it a proper home and where a
  non-interactive caller has different tolerance for degraded startup than the
  TUI does.
- **Setting `connection.Timeout`** — the seam makes it testable, but it is
  TRUST-04 in Phase 2.
- **Scrum-master use of the dormant `workClient`** — v2, per PROJECT.md
  §"Out of Scope". D-08 preserves the field for it.

</deferred>

---

*Phase: 1-Test seam for internal/ado*
*Context gathered: 2026-09-04*
