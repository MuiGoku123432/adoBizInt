---
phase: 01-test-seam-for-internal-ado
verified: 2026-09-06T06:19:31Z
status: passed
score: 27/29 must-haves verified
behavior_unverified: 1
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 25/29
  gaps_closed:
    - "TestCompletePullRequestIssuesFetchThenMerge now captures and whole-value compares the exact git.GetPullRequestArgs while retaining exact update arguments and get/update ordering."
    - "Approval now preserves cache-first recovery semantics through an injectable per-attempt identity provider, including nil/ID-less guards, transient recovery, non-memoized fallback success, and exact/wrapped errors."
  gaps_remaining: []
  regressions: []
unverified_backstop_items:

  - truth: "Two aggregate reads returning results for the same project have stable, specified output order under repeated runs."
    reason: insufficient_spec
    disposition: explicitly_unclaimed_non_goal
    blocking: false
    rationale: "The plan never identifies the two reads or the required order. This item was introduced only as an under-specified plan-level backstop, is not a ROADMAP success criterion or SEAM requirement, and plan 01-06 explicitly excludes it from the surgical gap closure. Honest-verifier semantics require abstention, not a fabricated pass; because it is not part of the phase goal contract, the abstention does not block phase completion."
behavior_unverified_items:

  - truth: "NewClient remains non-fatal when eager identity lookup fails, keeps successful AuthenticatedUser.Id caching, and retains the provider for later approval retries."
    test: "Construct a client through NewClient against a controlled identity endpoint that fails once and then recovers; inspect the returned client/warning and perform subsequent approvals."
    expected: "Construction succeeds after the eager lookup failure, a successful eager lookup caches AuthenticatedUser.Id, and a client created during failure retains the provider so later approval attempts recover without changing public behavior."
    why_human: "NewClient still constructs concrete SDK clients and performs the identity request internally. No pre-existing test executes this constructor transition; source wiring plus direct-provider tests prove the pieces but cannot prove the constructor-time state transition at runtime."
human_verification:

  - test: "Exercise NewClient through a controlled eager identity failure and recovery path."
    expected: "Construction remains non-fatal, the warning describes retry behavior, successful eager lookup caches AuthenticatedUser.Id, and subsequent approval can recover through the retained provider."
    why_human: "No automated test invokes NewClient's identity-failure transition without a network-capable SDK client; presence and structural wiring alone are insufficient under behavior-dependent verification rules."
---

# Phase 1: Test seam for internal/ado Verification Report

**Phase Goal:** `internal/ado` write paths can be tested without touching Azure DevOps
**Verified:** 2026-09-06T06:19:31Z
**Status:** human_needed
**Re-verification:** Yes -- after gap-closure plan 01-06

## Goal Achievement

### Observable Truths

This report rechecks all 29 previously enumerated truths. The two prior blockers received full implementation, wiring, behavioral, and mutation verification. Previously passing items received regression checks. Summary claims were not used as implementation evidence.

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | Each of the seven write methods has a test asserting the exact ADO interaction it issues, with no network access. | VERIFIED | Seven named write tests exist and passed with ADO credential variables removed. `TestCompletePullRequestIssuesFetchThenMerge` now captures `git.GetPullRequestArgs`, compares the complete `{Project: project-one, RepositoryId: repository-one, PullRequestId: 42}` literal, retains exact `git.UpdatePullRequestArgs`, and asserts `get` then `update` (`pullrequests_test.go:164-235`). |
| 2 | `go test ./internal/ado/...` passes with non-zero coverage. | VERIFIED | Independent run: `go test ./internal/ado/... -count=1 -cover` passed at 57.4%. |
| 3 | The TUI builds and runs with no behavior change. | VERIFIED | `go build ./...` passes; no `internal/ui/` or `cmd/` file changed since the Phase 1 base. The prior approval regression is closed: cached IDs bypass lookup, while empty-cache approvals retry the retained provider per attempt and preserve former recovery/error semantics. |
| 4 | Interfaces declare only methods `internal/ado` actually calls. | VERIFIED | Independent parser and production selector inventory both produce the same exact sets: work item 5, Git 7, build 4, release 4. |
| 5 | Build and vet succeed with all four sub-client fields narrowed to package-local interfaces. | VERIFIED | `client.go:22,26-28`; independent `go build ./...` and `go vet ./...` both exited 0. |
| 6 | `TestUpdateWorkItemStateIssuesExactPatch` passes without network or PAT. | VERIFIED | Credential-cleared targeted run passed; static scan found no PAT, URL, HTTP, connection, or authorization construction in `internal/ado/*_test.go`. |
| 7 | A struct-literal client with only `workitemClient` drives `UpdateWorkItemState`. | VERIFIED | `workitems_test.go:120-153` injects only `fakeWorkItemAPI` and the behavioral test passes. |
| 8 | All four fakes fail by interface and method name for every unstubbed method. | VERIFIED | `fakes_test.go` has exact fake method counts 5/7/4/4, 20 `Fatalf` guards, and four compile-time interface assignments. |
| 9 | Mutating `/fields/System.State` makes the tracer test fail and restoring makes it pass again. | VERIFIED | Prior independent verification executed the mutation and clean restoration; current source still contains the expected literal, the named test passes, and tracked `internal/ado` source is clean. |
| 10 | Four interfaces total exactly 20 methods in the 5/7/4/4 split. | VERIFIED | Independent parser output: `workItemAPI=5`, `gitAPI=7`, `buildAPI=4`, `releaseAPI=4`. |
| 11 | `ApprovePullRequest` reaches `gitAPI` using cached `currentUserID`, without constructing a location client in its body. | VERIFIED | `pullrequests.go:150-187` is cache-first and calls only the injected provider on a miss; no `location.NewClient` exists in the method. Cache-hit test passes with a provider that fails immediately if called. |
| 12 | `NewClient` derives `currentUserID` from `AuthenticatedUser.Id` while preserving the `AuthorizedUser` display-name read. | VERIFIED | `client.go:68-89` binds `locationClient.GetConnectionData`, eagerly invokes it, reads display name from `AuthorizedUser`, and caches ID only from `AuthenticatedUser.Id`; structural gate passed. |
| 13 | `NewClient` remains non-fatal when identity fetch fails and records recoverable approval behavior. | PRESENT_BEHAVIOR_UNVERIFIED | Source keeps the lookup under non-fatal `err == nil` extraction, retains the provider, and logs retry behavior (`client.go:74-89,111`); direct-provider tests prove approval fallback. However, `NewClient` remains at 0.0% coverage and no test executes the constructor-time failure/cache transition itself. |
| 14 | Empty `currentUserID` with no usable provider returns the pre-existing error without a Git API call. | VERIFIED | Nil-provider and ID-less-provider tests both pass, compare exact `could not determine current user ID`, and leave all Git fake methods unstubbed so any Git call fails by name. |
| 15 | Dormant `workClient` is retained and documented for v2. | VERIFIED | `client.go:23-25` retains the concrete field and explanatory v2 comment. |
| 16 | No credential reaches the identity warning log. | VERIFIED | The warning at `client.go:88` contains no arguments, PAT, authorization string, or connection data; test/static scans found no credential construction in tests. |
| 17 | `GetNextState` covers configured, unknown-type, and unmapped-state cases. | VERIFIED | `workitems_test.go:208-253`; all three named subtests passed independently. |
| 18 | The effort-write test spells out all four ADO paths and pins OriginalEstimate, RemainingWork, CompletedWork, State order. | VERIFIED | `workitems_test.go:171-205` contains the four literal paths in order and whole-slice comparison; targeted test passed. |
| 19 | Release approve/reject cross-check proves statuses differ. | VERIFIED | `TestApproveAndRejectReleaseSendDifferentStatuses` passed after invoking both methods through one fake. |
| 20 | Complete-pull-request stubs both SDK calls, asserts their order, and covers nil merge commit. | VERIFIED | `pullrequests_test.go:164-251` stubs fetch/update, asserts exact order and both whole values, and separately leaves update unstubbed for nil-commit handling. Both tests passed. |
| 21 | The race detector is green. | VERIFIED | Independent `go test ./... -count=1 -race -cover` exited 0. |
| 22 | `GetWorkItems` pins project query swallowing and the classification-node trap. | VERIFIED | The two subtests inject the distinct failures and assert one-project versus two-project results; aggregate run passed. |
| 23 | Dashboard characterization pins its structurally different in-place swallow. | VERIFIED | The test proves the failed query contributes zero work items while both projects contribute PRs (4 work items, 5 PRs); aggregate run passed. |
| 24 | Exactly eight tests use the `SwallowsProjectFailure` suffix and header comments naming TRUST-01 and Phase 2. | VERIFIED | Independent declaration/header script found exactly eight unique top-level tests with the required adjacent comment tokens. |
| 25 | The remaining six aggregate reads return an identified non-empty survivor and nil error when one project fails. | VERIFIED | Independent aggregate run passed all six; fixtures and assertions require the named survivor and reject empty/wrong-project output. |
| 26 | Both pipeline characterizations set non-empty matching pipeline configuration. | VERIFIED | `pipelines_test.go:56-59,99-102`; both tests reached their fakes and passed. |
| 27 | The two plan 01-04 characterization tests were demonstrated red under their specified mutations. | VERIFIED | Prior verifier independently executed both mutations and clean restoration; current tests and production files remain green and clean. |
| 28 | The six plan 01-05 characterization tests were demonstrated red when their reachable `continue` was removed. | VERIFIED | Prior verifier independently executed all six mutations; current aggregate suite passes and tracked production source is clean. |
| 29 | Two aggregate reads returning results for the same project have stable, specified output order under repeated runs. | UNVERIFIED (insufficient_spec, non-blocking) | Honest-verifier abstention retained. Neither reads nor required order are specified. This is explicitly unclaimed by 01-06 and is not part of the ROADMAP goal/success criteria or SEAM-01..04 contract, so it cannot honestly pass but does not block this phase goal. |

**Score:** 27/29 truths verified; 1 present but behavior-unverified; 1 under-specified, explicitly unclaimed backstop abstained

## Previous Gaps

| Previous blocker | Re-verification result | Evidence |
|---|---|---|
| Exact first call in `CompletePullRequest` | CLOSED | Whole `git.GetPullRequestArgs` capture/literal comparison, exact update comparison, and order assertion pass. Independent fetch-project mutation produced the named `GetPullRequest args` failure; byte restoration and green rerun succeeded. |
| Approval identity recovery semantics | CLOSED | Four focused tests pass: cache bypass, nil provider, ID-less provider, and transient recovery. The recovery test checks wrapped cause and exact text plus immediate provider/Git counts `1/0`, `2/1`, `3/2`, proving success is not memoized. |

## Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/ado/interfaces.go` | Four narrow package-local interfaces | VERIFIED | Exists, substantive, wired into `Client`; exact 5/7/4/4 method sets equal production call sets. |
| `internal/ado/client.go` | Interface fields plus eager cache/provider wiring | PRESENT, BEHAVIOR UNVERIFIED | Exists, substantive, and structurally wired; the location method value is bound, eagerly invoked, and retained. No test executes the constructor's identity-failure/cache transition. |
| `internal/ado/pullrequests.go` | Cache-first approval and exact two-call completion | VERIFIED | Exists, substantive, and reached by TUI plus tests; pre-Git guards and ordered Git operations are implemented. |
| `internal/ado/fakes_test.go` | Four fail-fast hand-written fakes | VERIFIED | 20 methods, 20 named guards, four conformance assignments. |
| `internal/ado/workitems_test.go` | Work-item exact writes, transition cases, characterization | VERIFIED | Named tests pass; full literals and non-vacuous aggregate fixture present. |
| `internal/ado/pullrequests_test.go` | Approval recovery, exact PR writes, characterization | VERIFIED | Both former blockers have substantive behavioral coverage; all targeted tests pass repeatedly. |
| `internal/ado/releases_test.go` | Two exact writes, divergence guard, two characterizations | VERIFIED | Tests pass and use whole-value literals/non-empty fixtures. |
| `internal/ado/pipelines_test.go` | Exact queue write and two configured characterizations | VERIFIED | Tests pass; both aggregate clients have matching non-empty pipeline configuration. |
| `internal/ado/branches_test.go` | Branch characterization | VERIFIED | Named survivor behavior passes. |
| `internal/ado/dashboard_test.go` | Structural-outlier characterization | VERIFIED | In-place swallow and both-project PR contribution are asserted and pass. |

## Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `NewClient` | four interface-typed `Client` fields | SDK constructor assignments | WIRED | Build/vet and exact method-set inventory pass. |
| `NewClient` | `ApprovePullRequest` | retained `getConnectionData` method value | WIRED | `client.go:74-77,111` to `pullrequests.go:155-165`; structural gate confirms eager use and retained fallback. |
| Cache-hit approval test | `gitAPI.CreatePullRequestReviewer` | cached `currentUserID` | WIRED | Provider is a fail-fast closure and remains uncalled; exact reviewer request passes. |
| Empty-cache approval tests | provider then Git | per-attempt lookup, authenticated ID extraction | WIRED | Guards prevent Git on failure/ID absence; recovery counters and two whole request comparisons pass. |
| `CompletePullRequest` test | `GetPullRequest` then `UpdatePullRequest` | captured whole SDK args and call list | WIRED | Both argument structures and ordered call list pass; mutation confirms fetch assertion sensitivity. |
| Eight aggregate reads | per-read fake failure sites | two-project fixtures | WIRED | Exact D-15/D-16 mapping/header gate and all eight tests pass. |
| `cmd/adoBizInt` / TUI | `ado.NewClient` and `*ado.Client` | unchanged caller path | WIRED | No `cmd/` or `internal/ui/` changes in Phase 1; whole repository builds. |

## Data-Flow Trace (Level 4)

| Artifact | Data | Source | Observable sink | Status |
|---|---|---|---|---|
| Approval cache path | `currentUserID` | Eager `AuthenticatedUser.Id` lookup in `NewClient` | Reviewer ID in exact Git request | FLOWING |
| Approval fallback path | live authenticated ID | Injected `getConnectionData` result on each empty-cache attempt | Two independently exact reviewer requests | FLOWING |
| Completion fetch/update | requested project/repository/PR and source commit | Method parameters plus fetched PR | Whole fetch assertion, then whole update assertion | FLOWING |
| Aggregate characterizations | surviving project records/counts | Fake SDK responses through production conversion/aggregation | Non-empty identified results and dashboard counts | FLOWING |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Formatting, build, vet, full race/coverage | `test -z "$(gofmt -l internal/)" && go build ./... && go vet ./... && go test ./... -count=1 -race -cover` | Exit 0; `internal/ado` 57.4% | PASS |
| Package coverage | `go test ./internal/ado/... -count=1 -cover` | Exit 0; 57.4% | PASS |
| Exact writes, approval recovery/guards, errors, cross-check, `GetNextState` | Credential-cleared named test run | All tests/subtests passed | PASS |
| Approval/completion stability | Five gap-focused tests with `-count=50` | Exit 0 | PASS |
| Eight aggregate characterizations | `go test ./internal/ado/ -count=1 -run '^TestGet.*SwallowsProjectFailure$' -v` plus anchored count | Exactly 8 top-level tests; all passed | PASS |
| D-15/D-16 declaration/header contract | Independent Python declaration map and adjacent-header check | Exact map, no duplicates, all headers contain `TRUST-01` and `Phase 2` | PASS |
| Interface and fake ceilings | Independent parser/call-site scripts | Interfaces and production calls exactly 5/7/4/4; fakes 5/7/4/4 with 20 guards | PASS |
| Fetch assertion mutation | Temporarily change only completion fetch `Project` to `repositoryID`; run one named test; restore byte-for-byte | Named test failed at `GetPullRequest args`; restored test passed; tracked source clean | PASS |

## Probe Execution

No `probe-*.sh` is declared or present. No probe result is claimed.

## Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|---|---|---|---|---|
| SEAM-01 | 01-01, 01-02 | Narrow interfaces for consumed ADO sub-clients | SATISFIED | Four unexported interfaces exactly equal independently inventoried production call sets, 5/7/4/4. |
| SEAM-02 | 01-01, 01-02, 01-06 | `ado.Client` accepts injected sub-clients/providers | SATISFIED | In-package struct literals inject fakes/provider and drive real methods with no constructor, credentials, or network. |
| SEAM-03 | 01-01, 01-03, 01-06 | Seven writes assert exact ADO calls | SATISFIED | All seven exact-call tests pass; the sole multi-call write now asserts both whole requests and order, with independent mutation proof. |
| SEAM-04 | 01-04, 01-05, 01-06 | Aggregate-read failure paths are covered | SATISFIED | Eight non-vacuous characterizations pass with exact D-15/D-16 name/file/header mapping. They intentionally pin pre-TRUST-01 behavior for Phase 2 inversion. |

No Phase 1 requirement is orphaned. The checkbox/traceability status in `REQUIREMENTS.md` is planning metadata and was not modified during verification.

## Scope, Security, and Quality Gates

| Check | Result |
|---|---|
| No public API change | PASS -- new seam is package-private; exported method signatures unchanged |
| No dependency change | PASS -- `go.mod` and `go.sum` unchanged from pre-gap baseline |
| No TUI/command source change | PASS -- no `internal/ui/` or `cmd/` path changed across Phase 1 |
| No credentials/network in tests | PASS -- static scan found no PAT, URL, HTTP, connection, or authorization construction; targeted tests passed with credential variables removed |
| No residual mutation | PASS -- tracked `internal/ado` is clean; current `pullrequests.go` is byte-identical to gap implementation commit `b63289a`; independent mutation restored cleanly |
| Nyquist status | PASS -- `01-VALIDATION.md` is `status: validated`, `nyquist_compliant: true`, with all 15 tasks mapped and zero recorded validation gaps |
| Security status | PASS -- `01-SECURITY.md` is `status: verified`, `threats_open: 0` |
| Refreshed review | PASS WITH COVERAGE WARNING -- `01-REVIEW.md` is dated after 01-06, `status: clean`, with zero findings. Independent Gortex review found no line-level defect but returned a risk-only block because changed `NewClient` remains uncovered, matching the behavior-unverified item above. |

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---:|---|---|---|
| None | -- | No unreferenced `TBD`, `FIXME`, or `XXX`; no test credential/network construction; no stub implementation affecting the phase goal | -- | No blocker or warning found |

## Honest-Verifier Backstop Disposition

The aggregate-ordering statement remains `UNVERIFIED (insufficient_spec)`. It names neither the two reads nor an expected order, so passing it would be dishonest. It also remains explicitly unrelated and unclaimed: it is absent from the ROADMAP success criteria and SEAM requirements, and plan 01-06 explicitly excludes it. Therefore it is recorded as an abstained non-goal rather than promoted to a pass or used to block the achieved phase goal.

## Human Verification Required

### 1. `NewClient` eager identity failure and recovery

**Test:** Run `NewClient` against a controlled identity endpoint that fails the eager `GetConnectionData` request, then recovers, and exercise approval on the returned client. Also confirm a successful eager response caches `AuthenticatedUser.Id`.

**Expected:** Construction remains non-fatal on the first failure, the warning describes retry behavior, the provider remains available for later approval attempts, and a successful eager lookup follows the cache-hit path without a second identity call.

**Why human:** The repository has strong structural evidence and direct-provider behavior tests, but no pre-existing test executes `NewClient`; coverage reports `NewClient` at 0.0%. The constructor creates concrete SDK clients, so a source check cannot certify this state transition.

## Gaps Summary

No blocker gap remains. Both prior blockers are closed by executable evidence, all directly testable goal-bearing truths and requirements are satisfied, and no regression was found. Final status is `human_needed`, not `passed`, because constructor-time identity failure/cache behavior is present and wired but has no behavioral test.

---

_Verified: 2026-09-06T06:19:31Z_
_Verifier: the agent (gsd-verifier)_
