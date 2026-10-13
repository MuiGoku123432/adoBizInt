# Phase 1: Test seam for internal/ado - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-04
**Phase:** 1-Test seam for internal/ado
**Areas discussed:** Fake strategy, Interface shape, Injection shape, The location client inside ApprovePullRequest, SEAM-04 ordering

---

## Area Selection

Five gray areas were presented; the user selected all five.

| Area | Selected |
|------|----------|
| Fake strategy — hand-written vs. generated | ✓ |
| Interface shape — exported or unexported, and how many | ✓ |
| Injection shape — how fakes get in | ✓ |
| The location client hiding inside ApprovePullRequest | ✓ |
| SEAM-04 depends on Phase 2 work — resolve the ordering | ✓ |

---

## Fake Strategy

### How should the fakes be built?

| Option | Description | Selected |
|--------|-------------|----------|
| Hand-written fakes with func fields | One struct per interface with func fields; unset methods panic loudly. Zero new dependencies, matches existing stdlib table-driven style. ~4 structs, 20 methods | ✓ |
| Recording spy that captures every call | Appends every invocation to a `calls []call` slice; direct fit for "assert the exact call", awkward when controlling return values | |
| Generated mocks via gomock | Adds `go.uber.org/mock`, `//go:generate` directives, committed generated mocks. Free matchers and call-count assertions, but a new dependency and a generate step | |

**User's choice:** Hand-written fakes with func fields
**Notes:** Keeps `go.mod` untouched, honoring PROJECT.md's "no new dependencies without stated justification". Matches `internal/utils/text_test.go`.

### How should a write test assert the call it captured?

| Option | Description | Selected |
|--------|-------------|----------|
| Declare the expected patch document and DeepEqual it | `want := []webapi.JsonPatchOperation{...}` spelled out, compared with `reflect.DeepEqual`, which follows the SDK's pointer fields. Field paths appear literally in the test | ✓ |
| Compare against a table of (op, path, value) tuples | Flatten each captured operation to a comparable tuple; fits the table-driven convention, needs a deref helper | |
| Golden file snapshot of the marshalled document | Marshal and diff against a committed `.golden` with a `-update` flag. Catches all drift, but a reviewer approving a regenerated golden unread is how a bad field path slips through | |

**User's choice:** Declare the expected patch document and DeepEqual it
**Notes:** Chosen specifically so a source typo in a field path literal cannot match the test. Directly targets TESTING.md:221.

### What should a fake do when the code calls a method the test did not stub?

| Option | Description | Selected |
|--------|-------------|----------|
| Fail the test with a named error | Fake holds a `*testing.T`; each method guards `if f.XFunc == nil { f.t.Fatalf("unexpected call to X") }`. Makes the assertion two-sided | ✓ |
| Let the nil func field panic | Zero extra code; surfaces as a panicking test rather than a clean failure | |
| Return zero values silently | Terser tests, but an unexpected write would return success and the test would pass — the exact failure mode this phase exists to prevent | |

**User's choice:** Fail the test with a named error
**Notes:** Framed during discussion as the same principle as the project's core value applied to the test harness.

### Where should the fakes live?

| Option | Description | Selected |
|--------|-------------|----------|
| `internal/ado/fakes_test.go`, in-package, promote later | Out of the shipped binary and out of the public API. Phase 4 promotes or fakes at the `ado.Client` level instead | ✓ |
| `internal/ado/adotest/fakes.go`, exported helper package | Phase 4 gets the seam free, but fakes become shipped code and public surface with no consumer today | |
| Split fakes across per-domain `_test.go` files | Each fake next to its tests, but `git.Client` serves three source files so at least one fake has no natural home | |

**User's choice:** `internal/ado/fakes_test.go`, in-package, promote later if needed
**Notes:** Raised in the question that ROADMAP Phase 4 SC6 depends on "the Phase 1 seam" from a different package. Accepted as a later, better-informed refactor.

---

## Interface Shape

### Should the interfaces be exported or unexported?

| Option | Description | Selected |
|--------|-------------|----------|
| Unexported | Matches TESTING.md's recommendation, keeps public API unchanged, preserves CONVENTIONS.md rule 17 as far as possible. Exporting later is non-breaking | ✓ |
| Exported so other packages can substitute later | Lets UI and CLI depend on interfaces, but is public API from day one with no consumer — the speculative generality PROJECT.md forbids | |
| Mixed — export only what a real consumer needs | Collapses to unexported in Phase 1, but names the rule for later phases | |

**User's choice:** Unexported
**Notes:** Counter-argument surfaced and rejected: every UI package holds `client *ado.Client` concretely and Phase 2 plan 02-04 touches those call sites anyway.

### How should the interfaces be decomposed?

| Option | Description | Selected |
|--------|-------------|----------|
| One interface per SDK sub-client, all called methods | Four interfaces mirroring the four live fields. Satisfies SC4. Field types change one word each; reads become fake-drivable for free | ✓ |
| Split reader and writer interfaces per sub-client | Eight interfaces; most honest about what Phase 1 needs, but requires duplicate fields or embedding that no caller asks for | |
| One combined interface for everything | One field, one fake struct, but stops mirroring the SDK and hands each fake irrelevant methods | |

**User's choice:** One interface per SDK sub-client, all called methods
**Notes:** The constraint that forced the question: `c.workitemClient` is one field serving both `UpdateWorkItem` and `QueryByWiql`, so a write-only interface cannot be that field's type without added structure.

### What should happen to the unused workClient field?

| Option | Description | Selected |
|--------|-------------|----------|
| Leave it alone; injectable constructor ignores it | Smallest diff, zero SC3 risk, v2 finds it where PROJECT.md says it is | |
| Remove it as dead code | Removes a constructor call per startup and state for an out-of-scope milestone, but contradicts PROJECT.md's stated intent | |
| Keep it and document why it is unused | Costs nothing, prevents a future cleanup pass deleting it without knowing the intent | ✓ |

**User's choice:** Keep it and document why it is unused
**Notes:** An interface over it would be empty since zero methods are called.

### How should the interfaces be named?

| Option | Description | Selected |
|--------|-------------|----------|
| Suffix with API: `gitAPI`, `buildAPI`, `releaseAPI`, `workItemAPI` | Fields keep current names so no call site changes; no shadowing; suffix signals "the slice of the SDK we call" | ✓ |
| Interfaces keep the Client suffix; rename the fields instead | Interface names read naturally, but ~25 call sites change, enlarging a diff SC3 wants inert | |
| Behaviour-oriented names (`workItemStore`, `pullRequestStore`) | More idiomatic on paper, but obscures which SDK package a fake must match, making SC4 harder to verify | |

**User's choice:** Suffix the interfaces with API
**Notes:** Raised because `gitClient gitClient` is legal Go (field and type names are separate namespaces) but poor to read across seven files.

---

## Injection Shape

### How should fakes get into an ado.Client?

| Option | Description | Selected |
|--------|-------------|----------|
| No new constructor — in-package struct literal | Adds nothing to the API. Tests build `&Client{workitemClient: fake, ...}` directly. Requires reading SEAM-02 as satisfied by assignable fields | ✓ |
| Unexported test-only helper constructor in `fakes_test.go` | Same zero production footprint, plus one place for sane defaults. Maps more cleanly onto SEAM-02's wording | |
| Exported second constructor `NewClientWithAPIs` | Unambiguously satisfies SEAM-02 and gives Phase 4 an out-of-package seam, but forces the interfaces to be exported, reversing the previous decision | |

**User's choice:** No new constructor — in-package struct literal
**Notes:** Two facts surfaced before the question that changed its shape: (1) the earlier fake-placement decision already made a struct literal sufficient, since unexported fields are in-package accessible; (2) `NewClient` performs network I/O in the constructor (`client.go:70`), so no test can call it regardless.

### How should the SEAM-02 interpretation be recorded?

| Option | Description | Selected |
|--------|-------------|----------|
| Record the interpretation in CONTEXT.md and the plan | Requirement text untouched, traceability intact, reasoning travels with the phase | ✓ |
| Amend SEAM-02 in REQUIREMENTS.md to match | Removes ambiguity permanently, but rewriting a requirement to match an implementation is how a real descope gets hidden | |
| Leave it and handle it at verification | Lets the verifier be the forcing function, with real evidence from having written the tests | |

**User's choice:** Record the interpretation in CONTEXT.md and the plan
**Notes:** Raised because `/gsd-verify-work` would otherwise read SEAM-02's "accepts injected sub-clients" as unmet.

---

## The Location Client Inside ApprovePullRequest

Facts established before the question: `client.go:66-76` already calls `GetConnectionData` and keeps only `AuthorizedUser.ProviderDisplayName`, discarding the rest; `pullrequests.go:154-165` builds a second location client and calls `GetConnectionData` again for `AuthenticatedUser.Id`. A struct-literal `Client` has a nil `connection`, so the method is unreachable in tests until addressed. This area was not covered by any of the three provisional plan titles in ROADMAP.md.

### How should the location dependency be handled?

| Option | Description | Selected |
|--------|-------------|----------|
| Cache `currentUserID` at construction | Location client leaves the method entirely; `gitAPI` plus a struct literal is enough to test it; removes a round-trip per approve. One edge: a startup hiccup now fails approve where today it would retry | ✓ |
| Cache with a live-fetch fallback | Behaviour strictly preserved and happy path testable, but keeps `location.NewClient` in the body and leaves one branch permanently uncovered | |
| Inject a fifth `locationAPI` interface | Preserves fetch-at-call-time timing exactly and makes the call assertable, but adds a fifth interface and field and keeps the redundant round trip | |

**User's choice:** Cache `currentUserID` at construction
**Notes:** Noted during discussion that `NewClient` reads `AuthorizedUser` while `ApprovePullRequest` reads `AuthenticatedUser` — different fields on the same response, to be verified against the SDK rather than assumed equivalent.

### What should happen when the identity fetch fails at construction?

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve today's behaviour, add a startup warning log | Identical observable behaviour, SC3 holds trivially, silent degradation becomes visible in the log. Yields a test case: empty `currentUserID` must error without touching `gitAPI` | ✓ |
| Preserve behaviour exactly, no new logging | Smallest diff and zero SC3 risk, but a failed identity fetch stays invisible until someone approves a PR | |
| Fail fast: `NewClient` errors when identity is unresolvable | Consistent with the core value, but stops the TUI starting where it currently serves every read view — a real read-only regression and a direct SC3 violation | |

**User's choice:** Preserve today's behaviour, add a startup warning log
**Notes:** The pull toward failing fast was raised explicitly and rejected on SC3 grounds, then re-filed as a deferred idea for Phase 3 where the CLI exit-code contract gives it a proper home.

---

## SEAM-04 Ordering

Facts established before the question: SEAM-04 requires tests for "the aggregate read failure paths from TRUST-01", but TRUST-01 is Phase 2. All eight aggregate reads currently log and `continue`, then `return x, nil` (`workitems.go:75-77` confirmed by inspection).

### How should SEAM-04 be handled?

| Option | Description | Selected |
|--------|-------------|----------|
| Characterization tests now, inverted in Phase 2 | Satisfies SEAM-04 in Phase 1, exercises the seam on read paths, makes Phase 2's change a visible test diff. Cost: temporarily commits tests asserting behaviour already judged wrong | ✓ |
| Move SEAM-04 to Phase 2 where TRUST-01 lands | Test sits with the code it tests, Phase 1 closes honestly against SEAM-01/02/03. Cost: seam exercised only on writes, and a roadmap edit before planning | |
| Split: seam smoke test now, semantics with Phase 2 | Validates the seam on reads without asserting semantics anyone intends to change, but still needs the roadmap edit | |

**User's choice:** Characterization tests now, inverted in Phase 2
**Notes:** Avoids a ROADMAP/REQUIREMENTS edit entirely. The standard characterization-test tradeoff, time-boxed to one phase.

### How many aggregate reads should get characterization tests?

| Option | Description | Selected |
|--------|-------------|----------|
| All eight | Phase 2's before/after is complete; every function it touches must be consciously inverted. Cost: eight near-identical tests whose differing arg types resist a single table | ✓ |
| One per sub-client — four tests | Proves the swallow and the read seam across all four sub-clients at half the cost, but four reads enter Phase 2 unpinned | |
| `GetWorkItems` only, as a pattern | Minimum footprint, but SEAM-04 only nominally satisfied and almost none of the before/after benefit | |

**User's choice:** All eight
**Notes:** The eight are `GetWorkItems`, `GetPullRequests`, `GetPipelineRuns`, `GetPipelineDefinitions`, `GetReleases`, `GetPendingApprovals`, `GetBranches`, `GetDashboardCounts`. CONVENTIONS.md explicitly tolerates the resulting duplication.

### How should the characterization tests be organized and marked?

| Option | Description | Selected |
|--------|-------------|----------|
| Follow the file convention, mark with a standard comment and name shape | Honors TESTING.md:329, keeps each test next to its source, and all eight are findable with one grep | ✓ |
| One dedicated `characterization_test.go` | Phase 2 opens exactly one file, impossible to miss, but breaks the source-file naming convention | |
| Per-file placement plus a handoff list in SUMMARY.md | Belt and braces, at the cost of duplication between code and summary that can drift | |

**User's choice:** Follow the file convention, mark with a standard comment and name shape
**Notes:** Name shape agreed as `TestGetWorkItemsSwallowsProjectFailure`, with an identical header comment naming TRUST-01 and Phase 2 as the inverter.

---

## Agent's Discretion

The user did not delegate any decision. No "you decide" option was selected in any question.

Four threads surfaced during discussion but were never opened as areas. The user chose "I'm ready for context" when offered the chance to explore them, so they are recorded in CONTEXT.md as open items for the planner:

- Whether the seam-free pure functions TESTING.md flags as high-value (`pipelineNameMatches`, `GetNextState`, `calculateApprovalStatus`) are covered in this phase.
- Whether compile-time SDK-drift assertions are worth adding, given `NewClient`'s assignments already provide compile-time proof.
- Whether `ApproveRelease`/`RejectRelease` need a test specifically targeting the byte-identical copy-paste risk, which two independently-written per-method tests would not catch.
- Whether the plan should give Phase 4 advance notice about promoting the seam out of `_test.go`.

---

## Deferred Ideas

- Promoting fakes to an exported `internal/ado/adotest` package — Phase 4, only if CLI write-verb tests need sub-client-level substitution.
- Exporting the interfaces so UI packages can depend on them — Phase 2 plan 02-04, where UI call sites are touched anyway.
- `DownloadAttachment`'s direct use of `c.connection.AuthorizationString` (`workitems.go:526`) — HARD-01/HARD-02, Phase 5.
- Making the identity fetch fatal at construction — Phase 3, where the CLI exit-code contract (OUT-04) gives it a proper home.
- Setting `connection.Timeout` — TRUST-04, Phase 2.
- Scrum-master use of the dormant `workClient` — v2, per PROJECT.md Out of Scope.
