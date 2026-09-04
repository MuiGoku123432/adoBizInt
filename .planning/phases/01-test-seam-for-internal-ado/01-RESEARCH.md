# Phase 1: Test seam for internal/ado - Research

**Researched:** 2026-09-04
**Domain:** Go interface extraction and hand-written test fakes over the Azure DevOps Go SDK
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

**These are locked. Research below explores only within them.**

### Locked Decisions

#### Fakes and assertions

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

#### Interface shape

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

#### Injection

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

#### The location client inside ApprovePullRequest

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

#### SEAM-04 ordering

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

*(Research recommendations on all four appear in §Recommendations on the Agent's
Discretion Items below.)*

### Deferred Ideas (OUT OF SCOPE)

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
</user_constraints>

## Summary

This phase is almost entirely grounded in code that already exists on disk, so the
research was verification rather than discovery. Every structural claim in
`01-CONTEXT.md` was checked against source this session and **all of them hold**:
the four sub-client method lists in D-06 match the real call sites exactly
(5/7/4/4), `workClient` is genuinely called from nowhere, the seven write methods
are where TESTING.md says they are, and the line references in the CONTEXT
canonical-refs block are accurate.

The single most important technical finding is one CONTEXT.md does not state
explicitly: **the Azure DevOps SDK's `NewClient` functions already return
interfaces, not concrete structs.** `git.NewClient` returns `git.Client`,
`workitemtracking.NewClient` returns `workitemtracking.Client`, and so on. This
means the narrow interfaces in D-06 are satisfied by *interface-to-interface
narrowing*, which Go resolves at compile time. I verified this compiles with a
throwaway module and `go vet`. Three consequences: the assignments in `NewClient`
give the compile-time drift proof the CONTEXT discretion section asks about (so
`var _ gitAPI = (*git.ClientImpl)(nil)` is both redundant *and* wrong — it would
assert against a struct the constructor never returns); no type assertion or
wrapper is needed anywhere; and the diff to `client.go` is four field type
changes and nothing else.

The two findings that will actually change the plan are traps in the D-14/D-15
characterization tests. First, `GetWorkItems` and `GetDashboardCounts` both call
`GetRecentIterations` before their real query, and a `GetClassificationNode`
failure there is **warned and swallowed** rather than propagated — so a test that
fails that call produces a *passing* project, not a failing one. The failure must
be injected at `QueryByWiql`. Second, `GetPipelineRuns` returns early with an
empty slice when `c.pipelines` is empty, so a struct-literal `Client` that sets
only `projects` will never reach the sub-client at all and the test will
vacuously pass.

**Primary recommendation:** Change four struct field types in `client.go`, add
four unexported interfaces in a new `internal/ado/interfaces.go`, and write
fakes as structs with func fields plus a `t *testing.T`. Touch nothing else. The
seam is a type-declaration change, not a refactor — no call site, no UI file, and
no constructor signature changes.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Narrow interface declarations | `internal/ado` (package-internal type layer) | — | Unexported per D-05; no consumer outside the package |
| Sub-client substitution | `internal/ado` struct fields | — | D-09: in-package tests assign fields directly; no constructor tier involved |
| Fakes | `internal/ado` test binary (`_test.go`) | — | D-04: test-only, never compiled into the shipped binary |
| Identity resolution (`currentUserID`) | `NewClient` (construction tier) | — | D-11 moves it out of the request tier into construction, where the data is already fetched |
| Write-argument assertion | test tier | — | Pure value comparison; no I/O tier participates |

## Project Constraints (from AGENTS.md)

**No `AGENTS.md` or `.opencode/AGENTS.md` exists in this repository** [VERIFIED:
`ls AGENTS.md .opencode/AGENTS.md` returned nothing this session]. No
`.claude/skills/` or `.agents/skills/` directory exists either. The governing
constraints therefore come from `.planning/codebase/CONVENTIONS.md` and
`.planning/PROJECT.md`, which the planner should treat with equivalent authority:

- Rule 5: do not add an interface to make something testable **unless the current
  task is specifically to make that thing testable**. This phase is that carve-out
  and must not generalize past the methods in D-06.
- Rule 6: no new dependency without stated justification. D-01 keeps `go.mod`
  untouched; this phase adds zero.
- Files under 300 lines. `internal/ado/workitems.go` is already 551 and
  `pipelines.go` is 397 [VERIFIED: `wc -l internal/ado/*.go`], so the interfaces
  should not be appended to either.
- The TUI must not change behavior (ROADMAP SC3).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEAM-01 | `internal/ado` exposes narrow interfaces for the ADO sub-clients it consumes | Exact method sets verified against every call site (§Verified Call-Site Inventory). Interface-to-interface narrowing verified to compile (§Pattern 1) |
| SEAM-02 | `ado.Client` accepts injected sub-clients so tests can substitute fakes | Satisfied by assignable unexported fields per D-09; no constructor change. `NewClient` performs network I/O at `client.go:70` so it is untestable regardless (§Pitfall 4) |
| SEAM-03 | All seven write methods have test coverage asserting the exact ADO call they issue | All seven located and read this session with verbatim argument structures (§The Seven Writes) |
| SEAM-04 | The aggregate read failure paths from TRUST-01 are covered by tests | All eight aggregate reads located; the per-project failure-injection point identified for each (§The Eight Aggregate Reads), including the two where the obvious injection point does not work |

## Verified Call-Site Inventory

Every `internal/ado` call into an SDK sub-client, from
`grep -n "c\.workitemClient\.\|c\.gitClient\.\|c\.buildClient\.\|c\.releaseClient\.\|c\.workClient\.\|c\.connection" internal/ado/*.go`
[VERIFIED: this session].

| Field | Distinct methods called | Call sites |
|-------|------------------------|-----------|
| `workitemClient` | 5 | `client.go:127`, `dashboard.go:100`, `workitems.go:112,151,228,278,300,417,465` |
| `gitClient` | 7 | `branches.go:48,71`, `dashboard.go:109`, `pipelines.go:322`, `pullrequests.go:46,172,194,229` |
| `buildClient` | 4 | `dashboard.go:123`, `pipelines.go:109,160,248,295,364` |
| `releaseClient` | 4 | `releases.go:68,121,214,281,306` |
| `workClient` | **0** | none — confirms D-08 |
| `connection` | 2 | `pullrequests.go:154` (D-11 removes), `workitems.go:526` (Phase 5, leave) |

This matches D-06 exactly. No method in D-06 is unused, and no called method is
missing from D-06.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` (stdlib) | go1.25.4 | Test driver, table-driven subtests | Already the only test framework in the repo |
| `reflect` (stdlib) | go1.25.4 | `reflect.DeepEqual` for JSON Patch document assertion | D-02; correctly follows the SDK's pervasive pointer fields |

**No new dependencies.** `go.mod` requires 7 direct modules and none is a test
library [VERIFIED: `cat go.mod`]. `go.sum` contains no testify, no gomock, no
mockery.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-written fakes | `go.uber.org/mock` (gomock) | Adds a dependency and a `go:generate` step; violates CONVENTIONS rule 6 for 4 small interfaces. Rejected by D-01 |
| `reflect.DeepEqual` | `github.com/google/go-cmp` | Better diffs on failure, but a new dependency. `DeepEqual` is sufficient — see Pitfall 2 for the one case where it needs care |

**Installation:** none. This phase adds no packages.

## Package Legitimacy Audit

**Not applicable.** This phase installs zero external packages by design (D-01,
CONVENTIONS rule 6). `go.mod` and `go.sum` are untouched. All types used come
from the stdlib or from `github.com/microsoft/azure-devops-go-api/azuredevops
v1.0.0-b5`, which is already a direct dependency and already in the module cache
[VERIFIED: `go env GOMODCACHE` path resolved and read this session].

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                     ┌──────────────────────────────────────────┐
   TUI (unchanged)   │  internal/ui/*  holds *ado.Client         │
                     └───────────────────┬──────────────────────┘
                                         │ calls exported methods
                                         │ (signatures unchanged)
                                         ▼
   ┌─────────────────────────────────────────────────────────────┐
   │  ado.Client                                                 │
   │                                                             │
   │   connection    *azuredevops.Connection   ── unchanged      │
   │   workClient    work.Client               ── unchanged (D-08)│
   │   currentUserID string                    ── NEW (D-11)     │
   │                                                             │
   │   workitemClient  workItemAPI  ◄── THE SEAM (4 fields)      │
   │   gitClient       gitAPI                                    │
   │   buildClient     buildAPI                                  │
   │   releaseClient   releaseAPI                                │
   └───────┬─────────────────────────────────────┬───────────────┘
           │                                     │
   production path                        test path
           │                                     │
           ▼                                     ▼
   ┌───────────────────────┐        ┌──────────────────────────┐
   │ NewClient(cfg)        │        │ &Client{                 │
   │  git.NewClient(...)   │        │   gitClient: &fakeGitAPI{│
   │   → returns git.Client│        │     t: t,                │
   │     (an INTERFACE)    │        │     CreatePullRequest... │
   │  assigned to gitAPI   │        │       Func: capture,     │
   │  ── narrowing, checked│        │   },                     │
   │     at COMPILE TIME   │        │   projects: []string{"P"}│
   └───────┬───────────────┘        │ }                        │
           │                        └──────────┬───────────────┘
           ▼                                   │
   ┌───────────────────┐                       ▼
   │ Azure DevOps REST │            ┌──────────────────────┐
   │ (real network)    │            │ captured args        │
   └───────────────────┘            │ reflect.DeepEqual    │
                                    │ vs literal expected  │
                                    └──────────────────────┘
```

Note the two paths converge on the *same* struct fields. No wrapper type, no
adapter, and no constructor branch exists between them — that is the whole point
of D-09.

### Recommended Project Structure

```
internal/ado/
├── interfaces.go        # NEW: the four unexported interfaces, ~30 lines
├── client.go            # 4 field types change; currentUserID added (D-11)
├── fakes_test.go        # NEW: four fakes, D-01/D-03 shape
├── workitems_test.go    # NEW: 2 writes + GetWorkItems characterization
├── pullrequests_test.go # NEW: 2 writes + GetPullRequests characterization
├── releases_test.go     # NEW: 2 writes + GetReleases/GetPendingApprovals char.
├── pipelines_test.go    # NEW: 1 write + GetPipelineRuns/Definitions char.
├── branches_test.go     # NEW: GetBranches characterization
└── dashboard_test.go    # NEW: GetDashboardCounts characterization
```

A separate `interfaces.go` is recommended over appending to `client.go`.
`client.go` is 236 lines [VERIFIED: `wc -l`]; adding ~30 lines of interface
declarations keeps it under 300 but crowds the constructor, and the CONTEXT
code-insights section already flags the package's file-length problem.

### Pattern 1: Interface-to-interface narrowing (the load-bearing mechanism)

**What:** The SDK constructors return interfaces, so assigning them to a narrower
interface is a standard Go interface conversion, statically checked.

**Verified evidence** [VERIFIED: `$GOMODCACHE/github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5/*/client.go`]:

```go
// git/client.go:267
func NewClient(ctx context.Context, connection *azuredevops.Connection) (Client, error)
// build/client.go:190
func NewClient(ctx context.Context, connection *azuredevops.Connection) (Client, error)
// release/client.go:92
func NewClient(ctx context.Context, connection *azuredevops.Connection) (Client, error)
// workitemtracking/client.go:188
func NewClient(ctx context.Context, connection *azuredevops.Connection) (Client, error)
// location/client.go:48
func NewClient(ctx context.Context, connection *azuredevops.Connection) Client   // note: no error
```

**Proof it compiles:** I built a throwaway module declaring `gitAPI` and
`workItemAPI` exactly as D-06 specifies, assigned `git.NewClient(...)` and
`workitemtracking.NewClient(...)` results to them, and ran `go vet`. Result:
`VET OK: narrowing compiles` [VERIFIED: executed this session against
azuredevops v1.0.0-b5].

**Consequence for the CONTEXT discretion item on compile-time assertions:** do
**not** add `var _ gitAPI = (*git.ClientImpl)(nil)`. `git.ClientImpl` exists
(`git/client.go:263`) but `NewClient` returns the `Client` interface, so such an
assertion pins a type the production path never actually produces. The
assignments at `client.go:87-91` already fail the build if the SDK drifts. Skip
the assertions.

### Pattern 2: Fake with `t` and fail-on-unexpected (D-03)

```go
// internal/ado/fakes_test.go
type fakeWorkItemAPI struct {
	t *testing.T

	UpdateWorkItemFunc func(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiqlFunc    func(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	// ...one field per method in workItemAPI
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}
```

`f.t.Helper()` is worth adding beyond what D-03 specifies: it makes the
`Fatalf` report the *caller's* line rather than the fake's, which matters when
one fake serves a dozen tests.

### Pattern 3: Write-call assertion (D-02)

```go
func TestUpdateWorkItemStateIssuesExactPatch(t *testing.T) {
	var got workitemtracking.UpdateWorkItemArgs
	c := &Client{workitemClient: &fakeWorkItemAPI{
		t: t,
		UpdateWorkItemFunc: func(_ context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
			got = args
			return &workitemtracking.WorkItem{}, nil
		},
	}}

	if err := c.UpdateWorkItemState(context.Background(), 42, "Active"); err != nil {
		t.Fatalf("UpdateWorkItemState() error = %v, want nil", err)
	}

	wantID := 42
	wantOp := webapi.OperationValues.Add
	wantPath := "/fields/System.State"
	want := workitemtracking.UpdateWorkItemArgs{
		Id: &wantID,
		Document: &[]webapi.JsonPatchOperation{
			{Op: &wantOp, Path: &wantPath, Value: "Active"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateWorkItem args = %+v, want %+v", got, want)
	}
}
```

`webapi.JsonPatchOperation` has exactly four fields — `From *string`,
`Op *Operation`, `Path *string`, `Value interface{}` [VERIFIED:
`webapi/models.go`, quoted verbatim below]. A literal that omits `From` leaves it
nil, which matches the source, which also never sets it.

```go
type JsonPatchOperation struct {
	From *string `json:"from,omitempty"`
	Op *Operation `json:"op,omitempty"`
	Path *string `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}
```

### Anti-Patterns to Avoid

- **Adding a `NewClientWithAPIs` constructor.** Rejected by D-09, and
  independently unnecessary given Pattern 1. It would force the interfaces
  exported, reversing D-05.
- **Asserting against `*git.ClientImpl`.** See Pattern 1.
- **Comparing `Value interface{}` across numeric types.** See Pitfall 2.
- **Calling `logging.Init()` in a `TestMain`.** See Pitfall 5.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Deep comparison of pointer-heavy SDK arg structs | Field-by-field nil-checked comparison helpers | `reflect.DeepEqual` | The SDK arg structs are 100% pointer fields; hand-written comparison is exactly the code most likely to contain the bug the test exists to catch |
| Reporting which call was unexpected | Returning zero values from unstubbed fakes | `f.t.Fatalf` in each method (D-03) | A zero return makes the caller fail later, at a confusing site |
| Interface conformance checking | `var _ I = (*Impl)(nil)` assertions | The existing `NewClient` assignments | Already compile-time proof; the assertion form is wrong here (Pattern 1) |

**Key insight:** this phase's temptation is to build *infrastructure* — a mock
framework, an assertion DSL, a shared test-client builder. Every one of those
would be larger than the four interfaces it serves. The seam is 30 lines of
type declarations; keep the machinery proportional.

## The Seven Writes

All seven read this session. Each row names the fake needed and the exact
argument struct to assert.

| # | Method | Location | Sub-client call | Fake needed | Assertion target |
|---|--------|----------|-----------------|-------------|------------------|
| 1 | `UpdateWorkItemState` | `workitems.go:215` | `UpdateWorkItem` (`:228`) | `workItemAPI` | `UpdateWorkItemArgs{Id, Document}` — 1 patch op |
| 2 | `UpdateWorkItemStateWithEffort` | `workitems.go:244` | `UpdateWorkItem` (`:278`) | `workItemAPI` | `UpdateWorkItemArgs{Id, Document}` — 4 patch ops, **the highest-value test** |
| 3 | `ApproveRelease` | `releases.go:272` | `UpdateReleaseApproval` (`:281`) | `releaseAPI` | `UpdateReleaseApprovalArgs{Project, ApprovalId, Approval}` |
| 4 | `RejectRelease` | `releases.go:297` | `UpdateReleaseApproval` (`:306`) | `releaseAPI` | same shape, `Status` differs |
| 5 | `QueuePipeline` | `pipelines.go:349` | `QueueBuild` (`:364`) | `buildAPI` | `QueueBuildArgs{Build, Project}` |
| 6 | `ApprovePullRequest` | `pullrequests.go:150` | `CreatePullRequestReviewer` (`:172`) | `gitAPI` **only, after D-11** | `CreatePullRequestReviewerArgs{Reviewer, RepositoryId, PullRequestId, ReviewerId, Project}` |
| 7 | `CompletePullRequest` | `pullrequests.go:190` | `GetPullRequest` (`:194`) then `UpdatePullRequest` (`:229`) | `gitAPI` | **two** calls; the second is the merge |

### Write 2 — the four field paths, verbatim

These are the string literals TESTING.md:221 flags. They must appear character-
for-character in the test [VERIFIED: `internal/ado/workitems.go:248-251`, quoted]:

```go
	statePath := "/fields/System.State"
	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
	remainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	completedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"
```

**Document order is `OriginalEstimate, RemainingWork, CompletedWork, State`**
[VERIFIED: `workitems.go:255-276`] — effort first, state last, per the source
comment "Update effort fields first, then state change". `reflect.DeepEqual` on
a slice is order-sensitive, so the test pins this ordering as a side effect. That
is desirable: the ordering is load-bearing per the comment, and pinning it is
free.

All four use `webapi.OperationValues.Add`; there is a single `addOp` variable at
`:247`, so all four `Op` pointers alias one value. `reflect.DeepEqual` compares
pointed-to values, not addresses, so a test declaring one `wantOp` and taking its
address four times compares equal. Confirmed by the type: `Op *Operation` where
`Operation` is a string type.

### Writes 3 and 4 — the copy-paste risk

Read this session and they are byte-identical apart from one token
[VERIFIED: `releases.go:272-294` vs `:297-319`]:

```go
	status := release.ApprovalStatusValues.Approved   // :275
	status := release.ApprovalStatusValues.Rejected   // :300
```

Everything else — the `&release.ReleaseApproval{Status, Comments}` literal, the
`UpdateReleaseApprovalArgs{Project, ApprovalId, Approval}` literal, the error
handling — is the same. **Recommendation on the CONTEXT discretion item:** yes,
add the cross-check. Two independently written tests that each assert only their
own status genuinely would not catch a swap, because a swap makes both functions
send the same value and each test would need to know what the *other* function
sends to notice. A third test that calls both against one fake and asserts the
two captured statuses differ closes it in three lines:

```go
// Regression guard: ApproveRelease and RejectRelease are byte-identical apart
// from the status constant (releases.go:275 vs :300). A copy-paste swap is
// exactly what a test here must catch.
func TestApproveAndRejectReleaseSendDifferentStatuses(t *testing.T) { ... }
```

### Write 6 — the D-11 change

Current body [VERIFIED: `pullrequests.go:153-165`]:

```go
	// Get current user's ID from the connection data
	locationClient := location.NewClient(ctx, c.connection)
	connData, err := locationClient.GetConnectionData(ctx, location.GetConnectionDataArgs{})
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	var userID string
	if connData != nil && connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
		userID = connData.AuthenticatedUser.Id.String()
	} else {
		return fmt.Errorf("could not determine current user ID")
	}
```

**D-13 resolved.** `AuthenticatedUser` and `AuthorizedUser` are **different
fields of the same type** on `location.ConnectionData` [VERIFIED:
`location/models.go`, quoted]:

```go
	AuthenticatedUser *identity.Identity `json:"authenticatedUser,omitempty"`
	AuthorizedUser *identity.Identity `json:"authorizedUser,omitempty"`
```

Both are `*identity.Identity`, and that type carries **both** fields in use
[VERIFIED: `identity/models.go`, quoted]:

```go
	Id                *uuid.UUID   `json:"id,omitempty"`
	ProviderDisplayName *string `json:"providerDisplayName,omitempty"`
```

So `NewClient` reads `AuthorizedUser.ProviderDisplayName` (`client.go:72-73`)
while `ApprovePullRequest` reads `AuthenticatedUser.Id` (`pullrequests.go:161`).
They are **not** aliases and the D-11 cache must read `AuthenticatedUser.Id`
specifically — reading `AuthorizedUser.Id` would compile, would usually be the
same person, and would be wrong in delegated-identity scenarios. `Id` is a
`*uuid.UUID`, so the existing `!= nil` guard and `.String()` carry over verbatim.

The D-11 shape in `NewClient` (both fields come from the one `connData` already
fetched at `client.go:70`):

```go
	currentUserID := ""
	if connData != nil {
		if connData.AuthorizedUser != nil && connData.AuthorizedUser.ProviderDisplayName != nil {
			currentUser = *connData.AuthorizedUser.ProviderDisplayName
		}
		if connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
			currentUserID = connData.AuthenticatedUser.Id.String()
		}
	}
	if currentUserID == "" {
		log.Warn("Could not resolve current user ID; pull request approval will fail", ...)  // D-12
	}
```

Note this restructures the existing `if err == nil && connData != nil &&
connData.AuthorizedUser != nil` guard at `client.go:71`. Preserve the
`err == nil` condition — D-12 requires construction to still succeed on failure.

## The Eight Aggregate Reads

All eight located and their per-project failure-injection points identified. **Three
of them do not fail where you would expect — and one of them cannot fail at all.**

| # | Function | Location | Swallow site | Fail this call to make a project fail |
|---|----------|----------|--------------|---------------------------------------|
| 1 | `GetWorkItems` | `workitems.go:56` | `:77` | `workItemAPI.QueryByWiql` — **not** `GetClassificationNode`, see below |
| 2 | `GetPullRequests` | `pullrequests.go:35` | `:54` | `gitAPI.GetPullRequestsByProject` (`:46`) |
| 3 | `GetPipelineRuns` | `pipelines.go:91` | `:114` | `buildAPI.GetDefinitions` (`:109`) — **must set `c.pipelines`**, see below |
| 4 | `GetPipelineDefinitions` | `pipelines.go:231` | `:253` | `buildAPI.GetDefinitions` (`:248`) |
| 5 | `GetReleases` | `releases.go:56` | `:73` | `releaseAPI.GetReleaseDefinitions` (`:68`) |
| 6 | `GetPendingApprovals` | `releases.go:201` | `:220` | `releaseAPI.GetApprovals` (`:214`) |
| 7 | `GetBranches` | `branches.go:32` | `:51` | `gitAPI.GetRepositories` (`:48`) |
| 8 | `GetDashboardCounts` | `dashboard.go:23` | `:51` — **UNREACHABLE, see correction** | **no per-project failure path exists**; `workItemAPI.QueryByWiql` (`:100`) makes the project contribute `0`, it does not make it fail |

> **CORRECTION (plan-checker B-1, revision 1).** The original version of this row
> claimed `GetDashboardCounts` shares the `QueryByWiql` trap with #1 and that "all
> eight follow the identical shape". **That is wrong for #8 and is corrected here
> rather than silently rewritten.**
>
> The original research verified the Trap-A *iteration* structure inside
> `getProjectCounts` but never checked whether `getProjectCounts` **propagates**.
> It does not. [VERIFIED: `dashboard.go:74-141` read in full.] Its only return is
> line 141 `return counts, nil`. All four internal error sites (`:81`, `:101`,
> `:115`, `:126`) log in place and fall through to the next call. Therefore:
>
> - `getProjectCounts` **never returns a non-nil error**;
> - the `continue` at `dashboard.go:49-51` is **unreachable dead code**;
> - failing `QueryByWiql` yields `WorkItems == 0` for that project while its
>   `PRs` and `Pipelines` **still populate normally**;
> - **removing that `continue` cannot turn any test red**, so it is not a usable
>   mutation target.
>
> Consequence for planning: **seven** of the eight reads share the identical
> `log.Error(...)` then `continue` shape. `GetDashboardCounts` is the eighth and is
> structurally different — it swallows **per call, in place, one layer down**.
>
> **Phase 2 warning.** A future `02-01` that inverts TRUST-01 by grepping for
> `continue` in the per-project loop will find seven sites and **silently miss the
> dashboard**. Inverting the dashboard requires adding `return nil, err` at each of
> the four sites inside `getProjectCounts` (or restructuring it to propagate)
> *before* the loop's `continue` becomes live code at all.

Seven of the eight (#1-#7) follow the identical shape: `log.Error(...)` then
`continue`, then `return x, nil` at the end. `GetWorkItems` is canonical [VERIFIED:
`workitems.go:73-83`, quoted]:

```go
	for _, project := range targetProjects {
		items, err := c.getProjectWorkItems(ctx, project)
		if err != nil {
			log.Error("Failed to get work items for project", "project", project, "error", err)
			continue
		}
		allItems = append(allItems, items...)
	}

	log.Info("Work items fetched", "total_count", len(allItems))
	return allItems, nil
```

### Trap A: `GetRecentIterations` failure is invisible

`getProjectWorkItems` calls `c.GetRecentIterations` first, and that call's error
is **warned and discarded** — execution continues with an empty iteration filter
[VERIFIED: `workitems.go:91-101`, quoted]:

```go
	iterations, err := c.GetRecentIterations(ctx, project)
	if err != nil {
		log.Warn("Failed to get iterations, using date filter only", "project", project, "error", err)
	} else if len(iterations) > 0 {
```

`GetRecentIterations` is the only caller of `GetClassificationNode`
(`client.go:127`). So a characterization test that stubs `GetClassificationNode`
to return an error does **not** make the project fail — it makes it succeed with
a different WIQL string. The failure must be injected at `QueryByWiql`
(`workitems.go:112`), which is the first error that actually returns
(`:113-114`). `GetDashboardCounts` → `getProjectCounts` has the identical
structure [VERIFIED: `dashboard.go:80-82`], with its `QueryByWiql` at
`dashboard.go:100`.

Practical consequence: the fakes for tests #1 and #8 must stub `GetClassificationNode`
(to something benign — returning `nil, nil` is handled at `client.go:137-140`)
**and** `QueryByWiql` (per-project: error for one, success for the other).
Leaving `GetClassificationNodeFunc` nil will trip the D-03 `Fatalf`.

### Trap B: `GetPipelineRuns` short-circuits on empty config

[VERIFIED: `pipelines.go:94-98`, quoted]:

```go
	// If no pipelines configured, return empty
	if len(c.pipelines) == 0 {
		log.Info("No pipelines configured, skipping pipeline runs fetch")
		return []PipelineRun{}, nil
	}
```

A `&Client{buildClient: fake, projects: []string{"A","B"}}` literal returns
`[]PipelineRun{}, nil` without ever calling the fake — the test passes while
asserting nothing. The literal must also set `pipelines: []string{"..."}`, and
the value must actually match a definition name via `pipelineNameMatches`
(`pipelines.go:57`) for the run to be collected. Recommend using `"*"`-free
exact names in the fixture to keep the match obvious.

`GetReleases` has a comparable dependency on `releaseDefinitions` for its
*filtering* (`releases.go:80+`), though its loop is not short-circuited the same
way. Set it in the literal regardless.

## Common Pitfalls

### Pitfall 1: `GetClassificationNode` failure does not fail the read
**What goes wrong:** A characterization test for `GetWorkItems` or
`GetDashboardCounts` injects an error at the wrong call and asserts a
swallow that never happened. The test passes and proves nothing; Phase 2 then
inverts behavior and the test still passes.
**Why it happens:** `GetRecentIterations`'s error is warned, not returned.
**How to avoid:** Inject at `QueryByWiql`. See Trap A.
**Warning signs:** The test passes when you delete the `continue` from the source.
Every characterization test should be sanity-checked by temporarily breaking the
source it claims to pin.

### Pitfall 2: `Value interface{}` numeric type mismatch under `DeepEqual`
**What goes wrong:** `UpdateWorkItemStateWithEffort` takes `float64` params and
assigns them straight into `Value interface{}` (`workitems.go:259,264,269`). A
test that writes `Value: 8` in the expected literal stores an `int`, and
`reflect.DeepEqual(interface{}(8), interface{}(8.0))` is **false** — different
dynamic types.
**Why it happens:** `DeepEqual` compares dynamic type before value, and untyped
constants in a composite literal default to `int`.
**How to avoid:** Write `Value: 8.0` or `Value: float64(8)` in every expected
patch operation for the effort fields. The state operation's `Value` is a
`string`, so it is unaffected.
**Warning signs:** A failure message where got and want print identically.

### Pitfall 3: forgetting `GetPullRequest` in the `CompletePullRequest` test
**What goes wrong:** The test stubs only `UpdatePullRequest`, hits the D-03
`Fatalf` on the unstubbed `GetPullRequest`, and the author "fixes" it by removing
the D-03 guard.
**Why it happens:** It is the only write of the seven that makes two SDK calls.
**How to avoid:** Stub `GetPullRequestFunc` to return a `*git.GitPullRequest`
with `LastMergeSourceCommit.CommitId` set — the source returns an error if it is
nil (`pullrequests.go:205-209`), which is itself a worthwhile second test case.
**Warning signs:** Any impulse to weaken a fake rather than complete a stub.

### Pitfall 4: assuming `NewClient` is reachable from a test
**What goes wrong:** Someone tries to cover `NewClient` itself.
**Why it happens:** It looks like an ordinary constructor.
**How to avoid:** It performs network I/O — five SDK `NewClient` calls plus
`locationClient.GetConnectionData` at `client.go:70` [VERIFIED]. It is
permanently untestable without a further refactor that is explicitly out of
scope. This is the strongest independent argument for D-09: no constructor
parameter could have helped, because the constructor is the problem.
**Warning signs:** A test that needs a live PAT.

### Pitfall 5: initializing the logger in tests
**What goes wrong:** A `TestMain` calling `logging.Init()` writes to the
developer's real log file at `~/.config/adoBizInt/adoBizInt.log`.
**Why it happens:** Every method opens with `log := logging.Logger()`, so it
looks like initialization is required.
**How to avoid:** It is not. `logging.Logger()` falls back to `slog.Default()`
when uninitialized [VERIFIED: `internal/logging/*.go`, quoted]:

```go
func Logger() *slog.Logger {
	if logger == nil {
		// Fallback to stderr if not initialized
		return slog.Default()
	}
	return logger
}
```

Tests work with no setup. The cost is `slog` output on stderr during
`go test -v`, which is cosmetic. Do not call `Init()`.

### Pitfall 6: growing the interfaces past D-06
**What goes wrong:** Someone copies the whole SDK `Client` interface in, or adds
a method "we'll need in Phase 2."
**Why it happens:** The SDK interfaces are right there and copying is easier.
**How to avoid:** ROADMAP SC4 is an explicit success criterion against exactly
this. The 5/7/4/4 counts in the Verified Call-Site Inventory are the ceiling.
`git.Client` alone declares dozens of methods; every one added is a method every
fake must implement.
**Warning signs:** A fake with methods no test ever stubs.

## Code Examples

### Characterization test for an aggregate read (D-14/D-16 shape)

```go
// This test documents current behavior: a per-project failure is logged and
// swallowed, and the caller receives the surviving projects with a nil error.
// TRUST-01 (Phase 2) inverts this — the failure must then reach the caller.
func TestGetWorkItemsSwallowsProjectFailure(t *testing.T) {
	c := &Client{
		projects: []string{"good", "bad"},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			// Iteration lookup is best-effort; a nil node is handled at
			// client.go:137. Stubbed only to satisfy the fail-on-unexpected guard.
			GetClassificationNodeFunc: func(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				if *args.Project == "bad" {
					return nil, errors.New("boom")
				}
				return &workitemtracking.WorkItemQueryResult{ /* one id */ }, nil
			},
			GetWorkItemsFunc: func(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
				return &[]workitemtracking.WorkItem{ /* ... */ }, nil
			},
		},
	}

	got, err := c.GetWorkItems(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetWorkItems() error = %v, want nil (current behavior swallows)", err)
	}
	if len(got) != 1 {
		t.Errorf("GetWorkItems() returned %d items, want 1 from the surviving project", len(got))
	}
}
```

Passing `nil` for `projects` exercises the `c.projects` fallback
(`workitems.go:59-62`), which is the path the TUI actually uses.

### The interface file

```go
// internal/ado/interfaces.go
package ado

// These interfaces exist solely to make the destructive write methods in this
// package testable (SEAM-01). They are deliberately narrow: each declares only
// the methods this package actually calls, per ROADMAP Phase 1 SC4. Do not add
// a method here that no code in internal/ado calls.
//
// The Azure DevOps SDK constructors return interfaces, so the assignments in
// NewClient are compile-time proof that the SDK still satisfies these. No
// separate `var _ I = ...` assertion is needed or correct.

type workItemAPI interface {
	UpdateWorkItem(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	GetWorkItems(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
	GetWorkItem(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiql(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetClassificationNode(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
}
// ...gitAPI (7), buildAPI (4), releaseAPI (4)
```

Exact signatures for all twenty methods are in the tables below, copied from the
SDK interface declarations [VERIFIED: `sed -n '/^type Client interface/,/^}/p'`
on each SDK `client.go` this session].

**`gitAPI`** — from `git/client.go`:
```go
	CreatePullRequestReviewer(context.Context, CreatePullRequestReviewerArgs) (*IdentityRefWithVote, error)
	GetBranches(context.Context, GetBranchesArgs) (*[]GitBranchStats, error)
	GetPullRequest(context.Context, GetPullRequestArgs) (*GitPullRequest, error)
	GetPullRequestsByProject(context.Context, GetPullRequestsByProjectArgs) (*[]GitPullRequest, error)
	GetRefs(context.Context, GetRefsArgs) (*GetRefsResponseValue, error)
	GetRepositories(context.Context, GetRepositoriesArgs) (*[]GitRepository, error)
	UpdatePullRequest(context.Context, UpdatePullRequestArgs) (*GitPullRequest, error)
```

**`buildAPI`** — from `build/client.go`:
```go
	GetBuilds(context.Context, GetBuildsArgs) (*GetBuildsResponseValue, error)
	GetDefinition(context.Context, GetDefinitionArgs) (*BuildDefinition, error)
	GetDefinitions(context.Context, GetDefinitionsArgs) (*GetDefinitionsResponseValue, error)
	QueueBuild(context.Context, QueueBuildArgs) (*Build, error)
```

**`releaseAPI`** — from `release/client.go`:
```go
	GetApprovals(context.Context, GetApprovalsArgs) (*GetApprovalsResponseValue, error)
	GetReleaseDefinitions(context.Context, GetReleaseDefinitionsArgs) (*GetReleaseDefinitionsResponseValue, error)
	GetReleases(context.Context, GetReleasesArgs) (*GetReleasesResponseValue, error)
	UpdateReleaseApproval(context.Context, UpdateReleaseApprovalArgs) (*ReleaseApproval, error)
```

Note the asymmetry worth carrying into the fakes: `git` and `workitemtracking`
return `*[]T` for collections, while `build` and `release` return
`*GetXResponseValue` wrapper structs with a `.Value` field. The source already
handles both (`releases.go:77` checks `len(releaseDefs.Value)`;
`branches.go:52` checks `repos == nil`).

## Recommendations on the Agent's Discretion Items

CONTEXT.md left four items open without delegating them. Recommendations:

| Item | Recommendation | Rationale |
|------|----------------|-----------|
| Cover the seam-free pure functions (`pipelineNameMatches`, `GetNextState`, `calculateApprovalStatus`) | **Yes for `GetNextState`; defer the other two** | `GetNextState` (`workitems.go:510`) feeds writes #1/#2 directly — an incorrect target state is a destructive-write bug, squarely in this phase's remit. `pipelineNameMatches` is TESTING.md's P1 and needs no seam, so it can land any time without blocking. Keep the phase's scope on the write path. |
| Compile-time assertions for SDK drift | **No** | Redundant *and* incorrect in form — see Pattern 1. Record the reason in `interfaces.go` so it is not re-proposed. |
| Cross-check test for `ApproveRelease`/`RejectRelease` | **Yes** | Two independent tests provably cannot catch a swap. Three lines. See §Writes 3 and 4. |
| Advance notice to Phase 4 re: promoting the seam | **Yes, one line in PLAN.md** | Zero cost now; D-04 already reasons it through. |

## Runtime State Inventory

This phase adds test files and changes type declarations. It is a refactor in
form, so the inventory is completed rather than skipped.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **None** — verified: this phase writes no data and changes no persisted format. `DownloadAttachment` is the only filesystem writer and is untouched (Phase 5) | none |
| Live service config | **None** — verified: no ADO-side configuration is renamed. Field paths like `Microsoft.VSTS.Scheduling.RemainingWork` are *asserted* in tests, not changed | none |
| OS-registered state | **None** — verified: no scheduler, service, or daemon registration exists in this repo (`cmd/` is a single TUI binary) | none |
| Secrets/env vars | **None** — verified: the PAT flows through `config.Config` into `azuredevops.NewPatConnection` (`client.go:35`) and neither is touched. D-11 caches a *user ID*, not a credential | none |
| Build artifacts | **None** — pure Go, no generated code, no `go:generate`, no cgo. `go build` fully regenerates | none |

The one behavioral change in the phase (D-11 removing the redundant
`location.NewClient` round-trip from `ApprovePullRequest`) removes a network call
and adds none. Nothing is cached across process restarts.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | everything | ✓ | go1.27.1 darwin/arm64 | — |
| `go.mod` declared Go | build | ✓ | 1.25.4 (toolchain 1.27.1 satisfies) | — |
| azure-devops-go-api SDK | interface signatures | ✓ | v1.0.0-b5, in module cache | — |
| `gofmt` | HARD-05 style gate | ✓ | bundled; `gofmt -l internal/` currently reports **zero** files | — |
| Network / live PAT | **not required** | n/a | — | This phase's whole purpose is removing this dependency |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** none

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (go1.25.4) |
| Config file | none — Go needs none |
| Quick run command | `go test ./internal/ado/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEAM-01 | Four narrow interfaces exist and the SDK satisfies them | compile | `go build ./...` | ❌ Wave 0 (`internal/ado/interfaces.go`) |
| SEAM-02 | A struct-literal `Client` with fakes drives real methods | unit | `go test ./internal/ado/ -run TestUpdateWorkItemState` | ❌ Wave 0 |
| SEAM-03 | Each of 7 writes issues its exact ADO call | unit | `go test ./internal/ado/ -run 'Test(UpdateWorkItem\|Approve\|Reject\|Queue\|Complete)'` | ❌ Wave 0 |
| SEAM-04 | Each of 8 aggregate reads swallows a per-project failure | unit (characterization) | `go test ./internal/ado/ -run 'SwallowsProjectFailure'` | ❌ Wave 0 |
| SC2 | Non-zero coverage on `internal/ado` | coverage | `go test ./internal/ado/... -cover` | ❌ Wave 0 |
| SC3 | TUI builds and behaves identically | build + existing suite | `go build ./... && go test ./...` | ✅ exists |

Current state: `go test ./internal/...` reports `internal/ado [no test files]`
[VERIFIED: executed this session]. Only `internal/ui/releases` and
`internal/utils` have tests.

### Sampling Rate

- **Per task commit:** `go build ./... && go test ./internal/ado/...`
- **Per wave merge:** `go test ./... && gofmt -l internal/`
- **Phase gate:** `go test ./... -race -cover` green, and `go test
  ./internal/ado/... -cover` reporting non-zero, before `/gsd-verify-work`

`-race` is worth running here even though this phase adds no concurrency: it is
HARD-05's eventual gate and the fakes' captured-variable pattern is exactly what
it checks.

### Wave 0 Gaps

- [ ] `internal/ado/interfaces.go` — covers SEAM-01; blocks everything else
- [ ] `internal/ado/fakes_test.go` — covers SEAM-02; blocks all test files
- [ ] `internal/ado/workitems_test.go` — SEAM-03 (writes 1-2), SEAM-04 (read 1)
- [ ] `internal/ado/pullrequests_test.go` — SEAM-03 (writes 6-7), SEAM-04 (read 2)
- [ ] `internal/ado/releases_test.go` — SEAM-03 (writes 3-4 + cross-check), SEAM-04 (reads 5-6)
- [ ] `internal/ado/pipelines_test.go` — SEAM-03 (write 5), SEAM-04 (reads 3-4)
- [ ] `internal/ado/branches_test.go` — SEAM-04 (read 7)
- [ ] `internal/ado/dashboard_test.go` — SEAM-04 (read 8)
- [ ] Framework install: **not needed**

`interfaces.go` and `fakes_test.go` are a strict serial prefix; the six test
files after them are mutually independent and parallelizable.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | **yes, indirectly** | D-11/D-12 change how the PAT-derived identity is resolved and cached. The control is that the identity is fetched once at construction from the authenticated session and never accepted from a caller |
| V3 Session Management | no | No sessions; PAT per connection |
| V4 Access Control | no | Authorization is entirely ADO-side |
| V5 Input Validation | no (this phase) | WIQL interpolation at `workitems.go:104` is a real injection surface but is HARD-04, Phase 5 |
| V6 Cryptography | no | No crypto added; TLS is the SDK's |
| V7 Error Handling & Logging | **yes** | D-12 adds a warning log. It must name the failure, not the credential |
| V14 Configuration | no | No config change |

### Known Threat Patterns for Go + Azure DevOps SDK

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Credential leaked into test fixtures or logs | Information disclosure | Fakes take no PAT and `Client` literals set no `connection`. The D-12 warning must log *what broke*, never `cfg.PAT` or `AuthorizationString` |
| Identity confusion: caching the wrong user field | Spoofing / Elevation | D-13 resolved above — cache `AuthenticatedUser.Id`, matching what `ApprovePullRequest` reads today. Using `AuthorizedUser.Id` would silently vote as a different identity |
| Silent degradation of an auth-dependent path | Repudiation | D-12: empty `currentUserID` returns the existing error verbatim and the construction-time warning makes the window visible in the log |
| Test that vacuously passes on a destructive path | — (assurance failure) | D-03 fail-on-unexpected, plus the Trap A/B checks. A green test on an unexercised merge path is worse than no test |

Nothing in this phase weakens the security posture. The `location.NewClient`
removal (D-11) *reduces* attack surface by eliminating one network round-trip per
approval. `c.connection.AuthorizationString` at `workitems.go:526` remains the
open finding and is correctly deferred to Phase 5 (HARD-01/HARD-02).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `github.com/golang/mock` | archived; `go.uber.org/mock` is the maintained fork | Jun 2023 | Moot — D-01 uses neither |
| Interfaces defined by the producer | Consumer-defined narrow interfaces ("accept interfaces, return structs") | Long-standing Go idiom | Exactly what D-06 does; the SDK's wide `Client` interfaces are the producer-defined form this phase narrows |

**Deprecated/outdated:**
- The SDK itself is on `v1.0.0-b5`, a 2020 beta [VERIFIED: `go.mod`]. Known debt,
  tracked as PLAT-02 (v2). This phase must not attempt an upgrade — the interface
  signatures above are pinned to b5 and would all change.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Splitting `interfaces.go` out of `client.go` is preferred to appending | Architecture Patterns | Cosmetic. Either satisfies the requirements; only file-length convention is at stake |
| A2 | `f.t.Helper()` in fake methods improves failure locality | Pattern 2 | None — additive to D-03, trivially removable |
| A3 | `-race` is worth running at the phase gate | Validation Architecture | None — a stricter gate than required |
| A4 | `GetNextState` should be covered in this phase but `pipelineNameMatches` deferred | Discretion Recommendations | Scope judgment on an item CONTEXT explicitly left open. The planner or user may reasonably include all three |

All structural claims — interface method sets, file:line references, SDK
signatures, field names, control flow — were read from source or the module cache
this session and are tagged `[VERIFIED]` inline.

## Open Questions (RESOLVED)

All three were resolved before planning completed. Retained with inline
`RESOLVED:` markers rather than deleted, so the audit trail stays honest.

1. **Should `GetNextState` and the other pure functions land in this phase?**
   - What we know: they need no seam, and `GetNextState` (`workitems.go:510`)
     feeds the destructive state write directly.
   - What's unclear: CONTEXT.md surfaced this without delegating it.
   - Recommendation: include `GetNextState`, defer the rest. See the
     Discretion table. Worth one line of confirmation before planning.
   - **RESOLVED:** user scope decision, recorded verbatim at
     `01-01-PLAN.md:104-116`. `GetNextState` is IN scope — seven writes plus
     `GetNextState` = eight covered units. `pipelineNameMatches`
     (`pipelines.go:57`) and `calculateApprovalStatus` (`pullrequests.go:119`)
     are deferred to a later phase. Covered by `01-03` Task 1.

2. **Where exactly do the interfaces live?**
   - What we know: `client.go` is 236 lines and would stay under 300 either way.
   - What's unclear: pure style preference.
   - Recommendation: `internal/ado/interfaces.go`, for the comment block
     documenting the SC4 ceiling and the no-assertion rationale.
   - **RESOLVED:** `internal/ado/interfaces.go`, per the recommendation. Created
     by `01-01` Task 1 and extended by `01-02` Task 1.

3. **Do the characterization tests get a sanity check?**
   - What we know: Trap A shows a characterization test can pass while pinning
     nothing.
   - Recommendation: the plan should require each of the eight to be verified by
     temporarily deleting the `continue` it claims to pin and observing a
     failure. That is a manual step during execution, not a committed test.
   - **RESOLVED:** yes — formalized as the mutation check protocol in `01-01`
     Task 2 and reused by `01-03`, `01-04` and `01-05`. Evidence requirement was
     strengthened in revision 1: the SUMMARY must quote the mutated diff hunk
     alongside the observed `FAIL` text, so the record shows *what was changed*,
     not merely that something failed. **Note the per-read mutation target is not
     uniform:** for reads #1-#7 it is removing the `continue`; for
     `GetDashboardCounts` it is making `getProjectCounts` propagate, because its
     `continue` is unreachable — see the CORRECTION under "The Eight Aggregate
     Reads".

## Sources

### Primary (HIGH confidence)
- `internal/ado/{client,workitems,pullrequests,releases,pipelines,branches,dashboard}.go` — read directly this session; all file:line refs verified
- `internal/logging/` — `Logger()` fallback verified
- `internal/utils/text_test.go`, `internal/ui/releases/releases_test.go` — style reference
- `$GOMODCACHE/.../azuredevops@v1.0.0-b5/{git,build,release,workitemtracking,location}/client.go` — SDK interface declarations and constructor signatures
- `$GOMODCACHE/.../azuredevops@v1.0.0-b5/{webapi,location,identity}/models.go` — struct field definitions
- Empirical compile check of interface-to-interface narrowing (`go vet`, passed)
- `go test ./internal/...`, `wc -l`, `gofmt -l`, `go version` — executed this session

### Secondary (MEDIUM confidence)
- `.planning/codebase/TESTING.md` — seven-write table cross-checked against source and confirmed accurate
- `.planning/codebase/CONVENTIONS.md`, `.planning/PROJECT.md` — constraints
- `.planning/phases/01-.../01-CONTEXT.md` — all structural claims independently verified

### Tertiary (LOW confidence)
- None. No web search was required; this phase is fully grounded in local source.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; verified against `go.mod`/`go.sum`
- Architecture: HIGH — narrowing mechanism verified by compilation, not inference
- SDK signatures: HIGH — read from the pinned module in the local cache
- Pitfalls: HIGH — each traced to a specific quoted line of source
- Discretion recommendations: MEDIUM — reasoned judgment on items left open

**Research date:** 2026-09-04
**Valid until:** stable — the SDK is version-pinned at `v1.0.0-b5` and the source
is local. Only a change to `internal/ado` itself invalidates this.
