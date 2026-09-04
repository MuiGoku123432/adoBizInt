---
phase: 01-test-seam-for-internal-ado
verified: 2026-09-04T23:28:27Z
status: gaps_found
score: 25/29 must-haves verified
behavior_unverified: 1
overrides_applied: 0
gaps:
  - truth: "All seven write methods have tests asserting the exact ADO interaction they issue, including every call in a multi-call write."
    status: partial
    reason: "CompletePullRequest makes two ADO calls, but TestCompletePullRequestIssuesFetchThenMerge discards git.GetPullRequestArgs and checks only call order plus the final UpdatePullRequestArgs. A regression that fetches the wrong project, repository, or pull request can pass, so ROADMAP success criterion 1 and SEAM-03 are not fully met."
    artifacts:
      - path: "internal/ado/pullrequests_test.go"
        issue: "Lines 66-73 ignore GetPullRequestArgs; there is no literal expected GetPullRequestArgs or reflect.DeepEqual assertion for the initial fetch."
    missing:
      - "Capture git.GetPullRequestArgs in TestCompletePullRequestIssuesFetchThenMerge."
      - "Compare the captured fetch arguments with the exact literal {Project: project-one, RepositoryId: repository-one, PullRequestId: 42}, while retaining the call-order and exact update assertions."
  - truth: "The TUI builds and runs with no behavior change."
    status: failed
    reason: "The TUI compiles and its source callers are unchanged, but approval behavior changed: before Phase 1, ApprovePullRequest fetched identity on every approval; now NewClient caches the ID once. A transient startup identity-fetch failure now leaves the process unable to approve even if ADO later recovers, and the later error text also differs from the former fetch-error path. No override accepts this roadmap deviation."
    artifacts:
      - path: "internal/ado/client.go"
        issue: "Lines 67-86 cache a possibly empty currentUserID once during construction."
      - path: "internal/ado/pullrequests.go"
        issue: "Lines 152-155 consume only the cached value and never retry identity resolution."
    missing:
      - "Either preserve the prior per-approval retry semantics while retaining a network-free test seam, or record an explicit accepted override for the intentional runtime behavior change."
      - "Add a behavioral test for the chosen identity-fetch failure and recovery contract."
behavior_unverified_items:
  - truth: "NewClient remains usable when identity lookup fails and records the lost approval capability."
    test: "Drive NewClient through an identity-fetch failure without live Azure DevOps, then inspect the returned client and warning."
    expected: "Construction succeeds, the warning names the lost approval capability without credentials, and subsequent behavior matches the accepted contract."
    why_human: "NewClient has 0.0% test coverage and constructs SDK/location clients internally, so the failure transition is present in source but not exercised."
unverified_backstop_items:
  - truth: "Two aggregate reads returning results for the same project have stable, specified output order under repeated runs."
    reason: insufficient_spec
    why_human: "The PLAN does not identify the two reads or the required order, and no named ordering test exists."
---

# Phase 1: Test seam for internal/ado Verification Report

**Phase Goal:** `internal/ado` write paths can be tested without touching Azure DevOps
**Verified:** 2026-09-04T23:28:27Z
**Status:** gaps_found
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

The 29 truths below are the four ROADMAP success criteria plus deduplicated PLAN frontmatter truths. Related failures are grouped under two root-cause gaps. Backstop truths are not credited from SUMMARY narration alone.

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | Each of the seven write methods has a test asserting the exact ADO interaction it issues, with no network access. | FAILED | Seven named tests run, but `TestCompletePullRequestIssuesFetchThenMerge` ignores the first call's `git.GetPullRequestArgs` at `pullrequests_test.go:66`; only the final update args are deep-compared at lines 98-116. |
| 2 | `go test ./internal/ado/...` passes with non-zero coverage. | VERIFIED | Independent run: `go test ./internal/ado/... -count=1 -cover` passed at 57.2%. |
| 3 | The TUI builds and runs with no behavior change. | FAILED | Build and caller wiring are green and `cmd/`/`internal/ui/` are unchanged, but runtime approval semantics changed. Baseline `ApprovePullRequest` fetched identity per action; current code caches once in `NewClient`, so a transient startup failure can no longer recover during approval and produces a different failure path. |
| 4 | Interfaces declare only methods `internal/ado` actually calls. | VERIFIED | Source count is exactly 5/7/4/4. Independent call-site inventory is also exactly 5/7/4/4 with identical method names. |
| 5 | Build and vet succeed with all four sub-client fields narrowed to package-local interfaces. | VERIFIED | `client.go:22,26-28`; independent `go build ./... && go vet ./...` exited 0. |
| 6 | `TestUpdateWorkItemStateIssuesExactPatch` passes without network or PAT. | VERIFIED | Targeted credential-cleared write run passed. Test client sets only a fake field; all test files have zero connection, HTTP, URL, PAT, or auth-string construction matches. |
| 7 | A struct-literal client with only `workitemClient` drives `UpdateWorkItemState`. | VERIFIED | `workitems_test.go:120-153` constructs the literal and passes the behavioral test. |
| 8 | All four fakes fail by interface and method name for every unstubbed method. | VERIFIED | `fakes_test.go` contains exact fake method counts 5/7/4/4, 20 nil guards, 20 named `Fatalf` calls, and four compile-time interface assignments. |
| 9 | Mutating `/fields/System.State` makes the tracer test fail and restoring makes it pass. | VERIFIED | Independently executed in a detached worktree: the path mutation made the named test fail; exact checkout restoration left the worktree clean. |
| 10 | Four interfaces total exactly 20 methods in the 5/7/4/4 split. | VERIFIED | Independent parser output: `workItemAPI=5`, `gitAPI=7`, `buildAPI=4`, `releaseAPI=4`. |
| 11 | `ApprovePullRequest` reaches `gitAPI` using cached `currentUserID`, without constructing a location client in its body. | VERIFIED | `pullrequests.go:149-168`; no `location` reference remains there. Exact reviewer-call test passes. |
| 12 | `NewClient` derives `currentUserID` from `AuthenticatedUser.Id` while preserving the `AuthorizedUser` display-name read. | VERIFIED | `client.go:74-81`, with ID assigned to the returned client at line 106. |
| 13 | `NewClient` remains non-fatal when identity fetch fails and warns about lost approval capability. | PRESENT_BEHAVIOR_UNVERIFIED | Source at `client.go:73-86` is present and wired, but `NewClient` has 0.0% coverage and no test exercises the identity-fetch failure transition. |
| 14 | Empty `currentUserID` returns the pre-existing error without a Git API call. | VERIFIED | `pullrequests.go:152-155`; `TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit` passes with every Git fake func nil. |
| 15 | Dormant `workClient` is retained and documented for v2. | VERIFIED | `client.go:23-25`; no `workClient` production call exists. |
| 16 | No credential reaches the identity warning log. | VERIFIED | Backstop has explicit source evidence: warning at `client.go:84-85` has no arguments and no PAT/auth string; PAT remains only in connection construction. |
| 17 | `GetNextState` covers configured, unknown-type, and unmapped-state cases. | VERIFIED | `workitems_test.go:208-253`; targeted run passed all three subtests. |
| 18 | The effort-write test spells out all four ADO paths and pins OriginalEstimate, RemainingWork, CompletedWork, State order. | VERIFIED | Literal paths and order-sensitive full slice comparison appear at `workitems_test.go:171-205`; targeted test passes. |
| 19 | Release approve/reject cross-check proves statuses differ. | VERIFIED | `TestApproveAndRejectReleaseSendDifferentStatuses` passes; it drives both methods through one fake. |
| 20 | Complete-pull-request stubs both SDK calls, asserts their order, and covers nil merge commit. | VERIFIED | Both methods are stubbed, order is asserted, and the nil-commit test leaves update unstubbed; targeted tests pass. This does not cure truth 1's missing first-call argument assertion. |
| 21 | The race detector is green. | VERIFIED | Independent `go test ./... -count=1 -race -cover` passed. |
| 22 | `GetWorkItems` pins project query swallowing and the classification-node trap. | VERIFIED | Test injects failure at `QueryByWiql`, returns one identified survivor, and separately proves classification failure still yields both projects. |
| 23 | Dashboard characterization pins its structurally different in-place swallow. | VERIFIED | Failed-project work items contribute zero while its two PRs still join the survivor's four work items and three PRs; build is forbidden by an unstubbed fake. |
| 24 | Exactly eight tests use the `SwallowsProjectFailure` suffix and header comments naming TRUST-01 and Phase 2. | VERIFIED | Independent enumeration found eight top-level functions; each header states pre/current TRUST-01 behavior and Phase 2 inversion, with the dashboard warning added. |
| 25 | The remaining six aggregate reads return an identified non-empty survivor and nil error when one project fails. | VERIFIED | Targeted run passed all six; fixtures branch on named failed projects and assertions reject empty or wrong-project results. |
| 26 | Both pipeline characterizations set non-empty matching pipeline configuration. | VERIFIED | `pipelines_test.go:56-59,99-102`; both reach `GetDefinitions` and return identified survivor data. |
| 27 | The two plan 01-04 characterization tests were demonstrated red under their specified mutations. | VERIFIED | Independently executed in a detached worktree: GetWorkItems propagation and dashboard `QueryByWiql` propagation each made its named test fail; sources restored cleanly. |
| 28 | The six plan 01-05 characterization tests were demonstrated red when their reachable `continue` was removed. | VERIFIED | Independently removed each targeted `continue` in a detached worktree; all six named tests failed as required and every source was restored cleanly. |
| 29 | Two aggregate reads returning results for the same project have stable, specified output order under repeated runs. | UNCERTAIN (insufficient_spec) | Tagged `verification: backstop`; neither the two reads nor their specified order are identified, and no ordering test exists. Human clarification or a held-out test is required. |

**Score:** 25/29 truths verified (1 present, behavior-unverified; 1 backstop abstained)

### Required Artifacts

The GSD artifact helper returned zero parsed artifacts because these plans use scalar paths rather than structured artifact objects, so artifact checks below were performed directly.

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/ado/interfaces.go` | Four narrow package-local interfaces | VERIFIED | Substantive 47-line file; exact 5/7/4/4 method sets; used by all four `Client` fields. |
| `internal/ado/client.go` | Production SDK construction and injectable interface fields | PARTIAL | Four constructor results wire directly into the narrow fields, but one-time identity caching changes approval retry behavior and its failure path has no behavioral test. |
| `internal/ado/pullrequests.go` | Pull-request read/write implementation using the Git seam | PARTIAL | Both writes use `gitAPI`; cached-ID approval is network-free in tests, but it no longer retries identity lookup as the baseline did. |
| `internal/ado/fakes_test.go` | Four fail-fast hand-written fakes | VERIFIED | 20 fake methods, 20 named nil-guard `Fatalf` calls, and four compile-time fake/interface assignments. |
| `internal/ado/workitems_test.go` | Work-item writes, `GetNextState`, work-item read characterization | VERIFIED | Substantive and runnable; all targeted tests pass. |
| `internal/ado/pullrequests_test.go` | Pull-request writes and read characterization | PARTIAL | Substantive and runnable, but the exact initial `GetPullRequestArgs` assertion is missing. |
| `internal/ado/releases_test.go` | Release writes, status cross-check, two read characterizations | VERIFIED | Exact approve/reject literals and non-empty read fixtures; targeted tests pass. |
| `internal/ado/pipelines_test.go` | Pipeline queue plus two configured read characterizations | VERIFIED | Exact queue literal; both read literals set non-empty matching pipeline config. |
| `internal/ado/branches_test.go` | Branch read characterization | VERIFIED | Injects `GetRepositories` failure and asserts identified non-empty survivor. |
| `internal/ado/dashboard_test.go` | Dashboard structural-outlier characterization | VERIFIED | Correct data-flow distinction: work items only from survivor, PRs from both projects, build call structurally forbidden. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `NewClient` SDK constructors | four interface-typed `Client` fields | direct assignments | WIRED | `client.go:42-64,95-102`; build proves SDK interfaces satisfy the narrow interfaces. |
| Test client literals | production write methods | unexported interface fields and fakes | WIRED | Every write test invokes the real method and terminates the SDK boundary in a fake. |
| `UpdateWorkItemState` | `workItemAPI.UpdateWorkItem` | exact `UpdateWorkItemArgs` | WIRED | Full literal comparison passes. |
| `CompletePullRequest` | `gitAPI.GetPullRequest` then `gitAPI.UpdatePullRequest` | ordered two-call flow | PARTIAL | Order and final mutation are asserted; initial fetch arguments are discarded. |
| Eight aggregate reads | their failure-swallow sites | two-project fake fixtures | WIRED | All eight targeted characterizations pass; dashboard is correctly treated as an in-place outlier. |
| `cmd/adoBizInt.main` | `ado.NewClient` then `ui.NewModel` | unchanged production construction path | WIRED | Main/TUI files are unchanged from phase base and the repository builds. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| Write tests | Captured SDK argument struct | Real production method constructs value, fake captures it | Yes | FLOWING |
| `TestCompletePullRequestIssuesFetchThenMerge` | Fetch and update interactions | Production method performs both calls | Partial | HOLLOW for fetch arguments: only call occurrence/order flows to the assertion; `GetPullRequestArgs` do not. |
| Aggregate read tests | Returned domain slices/counts | Production aggregate/conversion code fed by fake SDK responses | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Build and vet | `gofmt -l internal && go build ./... && go vet ./...` | Exit 0; gofmt produced no output | PASS |
| Seven write tests | `go test ./internal/ado/ -count=1 -run '<seven exact-write names>' -v` | Seven tests passed | PASS |
| Credential-independent writes | `env -u ADO_PAT -u ADO_ORG_URL -u AZURE_DEVOPS_EXT_PAT -u AZURE_DEVOPS_PAT go test ...` | Exit 0 | PASS |
| Guard/error tests | Targeted empty-ID, nil-merge, and release-status cross-check run | Three tests passed | PASS |
| Eight aggregate reads | `go test ./internal/ado/ -count=1 -run '^TestGet.*SwallowsProjectFailure$' -v` | Eight top-level tests and trap subtest passed | PASS |
| `GetNextState` | `go test ./internal/ado/ -count=1 -run '^TestGetNextState$' -v` | Three subtests passed | PASS |
| Package coverage | `go test ./internal/ado/... -count=1 -cover` | 57.2% | PASS |
| Full race/coverage suite | `go test ./... -count=1 -race -cover` | Exit 0 | PASS |
| Nine mutation checks | Detached-worktree script applying state-path, two plan-04, and six plan-05 mutations one at a time | All nine named tests returned non-zero; `restored_clean: True` | PASS |

### Probe Execution

No executable `probe-*.sh` is declared or present. The PLAN's backstop/probe assumptions were resolved from explicit evidence where possible. The exact-call assumption was disproved by source inspection and is reported as the blocking gap rather than silently waived.

The three carried probe interpretations resolve as follows: SEAM-01 is satisfied by exact call-set equality, SEAM-02 is satisfied by executable struct-field injection, and the user's explicit verification contract accepts pre-TRUST-01 swallow characterizations as SEAM-04 coverage. The separate unspecified output-order backstop remains abstained.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|---|---|---|---|---|
| SEAM-01 | 01-01, 01-02 | Narrow interfaces for consumed ADO sub-clients | SATISFIED | Four unexported interfaces exactly match the independently inventoried 5/7/4/4 call sets. |
| SEAM-02 | 01-01, 01-02 | `ado.Client` accepts injected sub-clients | SATISFIED | In-package tests assign fakes to interface fields and drive every write without a constructor, credential, or network. No added adapter/constructor exists. |
| SEAM-03 | 01-01, 01-03 | Seven writes assert exact ADO calls | BLOCKED | `CompletePullRequest` does not assert exact `GetPullRequestArgs`, so the only multi-call write lacks an exact assertion for one interaction. |
| SEAM-04 | 01-04, 01-05 | Aggregate read failure paths are covered | SATISFIED | Eight non-vacuous pre-TRUST-01 characterization tests cover all aggregate reads, including dashboard and classification/pipeline traps. |

No Phase 1 requirement is orphaned: all four ROADMAP/REQUIREMENTS IDs appear in PLAN frontmatter.

### Interface Ceiling and Production Wiring Audit

| Check | Result |
|---|---|
| Interface method ceilings | Exact 5/7/4/4; total 20 |
| Production distinct call sets | Exact 5/7/4/4; names match interfaces |
| Executable `ClientImpl` assertion | None; only the explanatory comment exists |
| Added adapter | None |
| Added constructor | None; only `NewClient` exists |
| `NewClient` production assignments | All four SDK constructor results assigned directly to narrow fields |
| TUI callers | Compile and source wiring unchanged; runtime approval semantics changed by one-time identity caching (blocking SC3) |

### Network, Credential, and Mutation Audit

| Check | Result |
|---|---|
| Test URLs / HTTP clients | 0 matches |
| PAT / ADO credential construction in tests | 0 matches |
| Connection or location client construction in tests | 0 matches |
| Test-only fake termination | All 20 interface methods delegate only to in-process func fields |
| Deliberate source mutation left behind | None; `git status --porcelain internal/ado` and source `git diff --exit-code` are clean |
| Production-source drift after intended seam change | None after commit `3325366`; only tests/docs changed afterward |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---:|---|---|---|
| `internal/ado/pullrequests_test.go` | 66 | Discarded `git.GetPullRequestArgs` in a test claiming exact fetch-then-merge interaction | BLOCKER | Wrong project/repository/PR fetch arguments would not fail the test, so SEAM-03 and ROADMAP SC1 remain false. |
| `internal/ado/client.go`, `internal/ado/pullrequests.go` | 67-86; 152-155 | One-time identity cache replaces per-approval identity lookup | BLOCKER | A transient startup lookup failure now disables approval for the process lifetime; this contradicts ROADMAP SC3's no-behavior-change contract absent an accepted override. |

No unreferenced `TBD`, `FIXME`, or `XXX` markers were found in phase implementation/test files. Empty-slice returns in production are legitimate existing no-data branches, not phase stubs.

### Prohibition Checks

| Prohibition | Status | Evidence |
|---|---|---|
| A destructive-write test must not pass without exercising its write path. | VERIFIED | Every write's mutating fake callback is reached and its captured argument is asserted; `CompletePullRequest` also proves fetch/update order. Its missing fetch-argument assertion remains a separate blocker. |
| An aggregate characterization must not pass without exercising the intended failure. | VERIFIED | Every fixture branches on a named failing project and returns an error; targeted output independently shows each production error branch logging, while assertions require identified surviving data. |
| The dashboard mutation target must be reachable, not its dead outer `continue`. | VERIFIED | Test drives `QueryByWiql`'s error branch at `dashboard.go:100-103`, then proves the same project's PR call still contributes to the total. |
| The six remaining characterization tests must not hide an unexercised swallow. | VERIFIED | Poisoned non-nil failed-project data plus errors and a one-survivor assertion make fallthrough observable in all six fixtures. |

### Human / Backstop Verification Still Required

These do not override the higher-precedence `gaps_found` status.

#### 1. NewClient identity-failure transition

**Test:** Exercise `NewClient` with an injected identity lookup failure, then attempt approval after simulated recovery.

**Expected:** Behavior matches the explicitly accepted contract. Under the roadmap's current no-change wording, approval should retain the baseline retry/recovery behavior.

**Why human:** No constructor seam or behavioral test exercises this state transition; `NewClient` is at 0.0% coverage.

#### 2. Stable aggregate ordering backstop

**Test:** First identify the two reads and required order, then run a held-out repeated-order test.

**Expected:** The explicitly specified order remains stable across runs.

**Why human:** The backstop is under-specified and therefore non-inferable.

### Gaps Summary

Two blocker concerns remain:

1. `CompletePullRequest` is the only multi-call destructive method. Its test proves that a fetch occurs before an exact merge update, but it does not prove that the fetch targets the requested project, repository, and pull request. This directly violates the requirement and roadmap wording that every write's exact ADO interaction be asserted. The advisory review finding is therefore confirmed as phase-blocking, not waived.
2. The TUI compiles with unchanged callers, but approval's identity lookup moved from every action to one-time construction. That creates a concrete transient-failure behavior change and violates ROADMAP SC3 unless explicitly accepted as an override.

The first fix is narrow: capture `git.GetPullRequestArgs`, compare it against a full literal, retain the existing order and update assertions, and rerun the targeted, race, build, vet, and coverage checks. For the second, either preserve retry semantics behind a testable seam or explicitly accept the deviation with a verification override and add a test for the accepted behavior.

---

_Verified: 2026-09-04T23:28:27Z_
_Verifier: the agent (gsd-verifier)_
