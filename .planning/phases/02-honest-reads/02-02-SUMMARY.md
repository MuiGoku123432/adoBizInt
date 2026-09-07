---
phase: 02-honest-reads
plan: 02
subsystem: api
tags: [go, azure-devops, error-classification, timeouts, generics, bubbletea]

requires:
  - phase: 02-honest-reads
    provides: "ado.Result[T], ado.Failure, classifyFailure, statusCodeOf, appendFailure, newResult and the typed SDK error constructors from plan 02-01"
provides:
  - "GetWorkItems returning the shared envelope, with batch failures attributed separately from query failures"
  - "getProjectWorkItems propagating its per-batch failures alongside the items it collected"
  - "GetPipelineRuns and GetPipelineDefinitions returning the shared envelope"
  - "GetReleases and GetPendingApprovals returning the shared envelope"
  - "GetBranches returning the shared envelope, with per-repository failures labelled by repository and empty repositories suppressed"
  - "Envelope-carrying view messages for workitems, pipelines, releases and branches (compile fix only)"
affects: [02-03-dashboard-partial-counts, 02-04-json-field-contract, 02-05-tui-failure-surfacing, phase-03-cli, phase-05-hardening]

actuals:
  tokens: 41000
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "A nested fan-out propagates its inner failures out to the caller rather than returning only an outer error, so a partially-answering unit is never reported complete"
    - "Structural status-code suppression: a not-found response from a per-repository read is an ordinary empty result, not a failure, and is distinguished by status rather than by error text"

key-files:
  created: []
  modified:
    - internal/ado/workitems.go
    - internal/ado/workitems_test.go
    - internal/ado/pipelines.go
    - internal/ado/pipelines_test.go
    - internal/ado/releases.go
    - internal/ado/releases_test.go
    - internal/ado/branches.go
    - internal/ado/branches_test.go
    - internal/ui/workitems/workitems.go
    - internal/ui/pipelines/pipelines.go
    - internal/ui/releases/releases.go
    - internal/ui/branches/branches.go

key-decisions:
  - "An empty repository is identified by status 404 exactly, not by a 4xx range, so a 403 on a per-repository branch fetch still records an auth failure rather than being silently suppressed"
  - "No fifth FailureKind was added for not-found; the empty-repository case is suppressed rather than named, per D-07"
  - "The per-repository failure label is built as 'get branch stats for <repo>' so the repository is identifiable from the operation field without widening the Failure struct (T-02-01)"
  - "getProjectWorkItems returns ([]WorkItem, []Failure, error) rather than a wrapped envelope, because it is a package-private helper whose caller owns the aggregate"
  - "The D-12 guard adds one continue per read function; every pre-existing continue is byte-for-byte unchanged and the happy-path row-count tests are the real detector for a converted data filter"

patterns-established:
  - "Every expanded read logs a failure_count on its terminal log.Info rather than adding a second log statement"
  - "Each of the six reads visits every configured project even after an auth failure (D-08)"

requirements-completed: [TRUST-01, TRUST-02, TRUST-03, TRUST-04]

coverage:
  - id: D1
    description: "GetWorkItems returns a surviving project's items alongside exactly one named failure for the failing project, and reports incomplete"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsSwallowsProjectFailure"
        status: pass
    human_judgment: false
  - id: D2
    description: "A work-item batch fetch failure is attributed to its project, the items already retrieved are still returned, and the project is not reported complete"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsReportsBatchFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsReportsBatchFailureKeepsRetrievedItems"
        status: pass
    human_judgment: false
  - id: D3
    description: "Both pipeline reads return the envelope with a surviving project's rows plus exactly one named failure, and report incomplete"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineRunsSwallowsProjectFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineDefinitionsSwallowsProjectFailure"
        status: pass
    human_judgment: false
  - id: D4
    description: "Both release reads return the envelope with a surviving project's rows plus exactly one named failure, and report incomplete"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetReleasesSwallowsProjectFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetPendingApprovalsSwallowsProjectFailure"
        status: pass
    human_judgment: false
  - id: D5
    description: "No data filter was converted into a failure record: a fully successful two-project fetch returns the same row count as before the change for all four pipeline and release reads"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineRunsHappyPathRowCountUnchanged"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineDefinitionsHappyPathRowCountUnchanged"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetReleasesHappyPathRowCountUnchanged"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetPendingApprovalsHappyPathRowCountUnchanged"
        status: pass
    human_judgment: false
  - id: D6
    description: "GetBranches reports a per-repository branch-stats failure with the repository named in the operation label, while a not-found response is treated as an ordinary empty repository and records no failure"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/branches_test.go#TestGetBranchesReportsRepoFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/branches_test.go#TestGetBranchesSwallowsProjectFailure"
        status: pass
    human_judgment: false
  - id: D7
    description: "An injected typed 401/403 on any of the six reads produces an auth-kind failure and a non-empty failure list even when the items slice is empty"
    requirement: TRUST-02
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestGetWorkItemsClassifiesRejectedToken"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineRunsClassifiesRejectedToken"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineDefinitionsClassifiesRejectedToken"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetReleasesClassifiesRejectedToken"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetPendingApprovalsClassifiesRejectedToken"
        status: pass
      - kind: unit
        ref: "internal/ado/branches_test.go#TestGetBranchesClassifiesRejectedToken"
        status: pass
    human_judgment: false
  - id: D8
    description: "An already-expired fetch context produces a timeout-kind failure for each unreached project, in configured iteration order, with no doomed calls issued"
    requirement: TRUST-04
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestFetchTimeoutMarksUnreachedWorkItems"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestFetchTimeoutMarksUnreachedPipelines"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestFetchTimeoutMarksUnreachedReleases"
        status: pass
      - kind: unit
        ref: "internal/ado/branches_test.go#TestFetchTimeoutMarksUnreachedBranches"
        status: pass
    human_judgment: false
  - id: D9
    description: "Assumption A4 — that an empty Azure DevOps repository yields a 404-family response to a branch-stats fetch — remains inferred from the pre-existing log comment, not observed against the real service"
    verification: []
    human_judgment: true
    rationale: "Only a real org containing an empty repository can reveal the status the service actually returns. If the inference is wrong, the consequence is a noisy failure entry rather than a lost failure, which is the safe direction, so it was not worth a live probe in this plan."

duration: 34 min
completed: 2026-09-07
status: complete
---

# Phase 02 Plan 02: Expand Aggregate Reads Summary

**Six more aggregate reads now return the `Result` envelope, including the two structurally distinct nested swallows — the work-item detail batch and the per-repository branch fetch — with an ordinary empty repository still producing no warning.**

## Performance

- **Duration:** 34 min
- **Tasks:** 3 (all TDD: RED then GREEN)
- **Files modified:** 12 (0 created, 12 modified)

## Accomplishments

- `GetWorkItems` returns the envelope and attributes batch failures separately from query failures. `getProjectWorkItems` now returns `([]WorkItem, []Failure, error)` so a project whose WIQL query answered but whose detail batch failed is reported incomplete with its retrieved items intact — the specific failure D-02 exists to prevent.
- All four pipeline and release reads return the envelope with per-project failures recorded at exactly the four error swallow sites named in the plan. Every one of the seven data filters in those two files is byte-for-byte unchanged, proven by four happy-path row-count tests rather than by diff proximity.
- `GetBranches` reports both its per-project repository-listing failure and its nested per-repository branch-stats failure, with the repository name carried in the operation label so the eventual warning line identifies which repository is broken.
- A status-404 response to a per-repository branch fetch is suppressed as an ordinary empty repository. No fifth `FailureKind` was added; the constant count in `result.go` is unchanged at four.
- All six outer loops carry the D-12 already-expired-context guard, so an exhausted fan-out budget records timeout-kind failures for unreached projects instead of issuing a burst of doomed calls. The four N+1 reads — where CONCERNS.md MED3 says the budget is most likely to be exhausted mid-sweep — are covered.
- Four view packages were updated with the minimum edit needed to keep `go build ./...` green: the message type carries the envelope per D-16 and the reducer unwraps `.Items`. No reducer or render work was started; that belongs to 02-05.

## Task Commits

1. **Task 1 RED: failing tests for honest work-item reads** — `bd65fa9` (test)
2. **Task 1 GREEN: honest work-item read with batch attribution** — `4c8462f` (feat)
3. **Task 2 RED: failing tests for honest pipeline and release reads** — `1490bec` (test)
4. **Task 2 GREEN: honest pipeline and release reads** — `ffe75eb` (feat)
5. **Task 3 RED: failing tests for the honest branch read** — `b4c3098` (test)
6. **Task 3 GREEN: honest branch read with per-repository attribution** — `bc697f0` (feat)

## Files Created/Modified

- `internal/ado/workitems.go` — envelope return, two operation labels, batch-failure propagation out of `getProjectWorkItems`, D-12 guard
- `internal/ado/workitems_test.go` — characterization inverted in place, plus batch-failure, retained-items, auth and timeout cases
- `internal/ado/pipelines.go` — envelope returns for both reads, failures at `:114` and `:253` equivalents, D-12 guards
- `internal/ado/pipelines_test.go` — two characterizations inverted, plus a shared fixture, happy-path row-count guards, auth and timeout cases, and the `assertAllUnreached` helper shared with the release tests
- `internal/ado/releases.go` — envelope returns for both reads, failures at `:73` and `:220` equivalents, D-12 guards
- `internal/ado/releases_test.go` — two characterizations inverted, plus happy-path row-count guards, auth and timeout cases
- `internal/ado/branches.go` — envelope return, the 404 suppression with its D-07 comment, per-repository operation label, D-12 guard
- `internal/ado/branches_test.go` — characterization inverted, plus the 404/500 pair, auth and timeout cases
- `internal/ui/workitems/workitems.go`, `internal/ui/pipelines/pipelines.go`, `internal/ui/releases/releases.go`, `internal/ui/branches/branches.go` — envelope-carrying message types and `.Items` unwrapping; compile fix only

## Decisions Made

- **The empty-repository suppression matches status 404 exactly, not a 4xx range.** A range check would swallow a 403 on a per-repository branch fetch, which is precisely the auth failure TRUST-02 exists to surface. Narrowing to 404 makes the suppression the smallest possible hole.
- **No fifth `FailureKind`.** D-07 fixed the set at four and deferred a not-found kind. The empty-repository case is suppressed rather than named, and `branches.go` carries a comment recording that this is the site where a not-found kind would have earned its keep and that `02-CONTEXT.md`'s deferred-ideas list is where it would return from.
- **The repository identity rides in the operation label** (`get branch stats for <repo>`) rather than in a new `Failure` field, keeping T-02-01's constraint intact — the struct still holds only project, operation, kind and error.
- **`getProjectWorkItems` returns a bare `[]Failure`, not a nested envelope.** It is a package-private helper; the aggregate belongs to `GetWorkItems`, and wrapping twice would force the caller to unwrap and re-derive completeness for no gain.

## Deviations from Plan

### 1. [Rule 3 - Blocking] The `continue`-count acceptance criteria are mechanically unsatisfiable as written

- **Found during:** Task 1 acceptance verification, and again in Tasks 2 and 3
- **Issue:** Three acceptance criteria require `grep -c 'continue'` on `workitems.go`, `pipelines.go` and `releases.go` to be unchanged from before the task. The same tasks also mandate adding the D-12 already-expired-context guard, whose only clean form is `if err := ctx.Err(); err != nil { ...; continue }`, and Task 1 additionally instructs "Keep the `continue`" at the existing swallow site. The plan therefore requires adding a `continue` and leaving the count unchanged, which cannot both hold.
- **Observed delta:** `workitems.go` 3 → 4, `pipelines.go` 10 → 12, `releases.go` 6 → 8. Exactly **+1 per read function carrying a D-12 guard** (one in workitems, two in pipelines, two in releases), and nothing else.
- **Resolution:** The behavioral intent — no data filter converted into a failure record — is met and independently proven by the mechanism the plan itself names as the real detector: `TestGetPipelineRunsHappyPathRowCountUnchanged`, `TestGetPipelineDefinitionsHappyPathRowCountUnchanged`, `TestGetReleasesHappyPathRowCountUnchanged` and `TestGetPendingApprovalsHappyPathRowCountUnchanged` each assert the row count on a fully successful two-project fetch, with fixtures that deliberately include a non-matching definition name and a nil name so a converted filter would change the count. No code was restructured to chase the grep.

### 2. [Rule 2 - Missing Critical] The D-12 gate was split into per-read test functions rather than added to the existing shared one

- **Found during:** Task 1 test design
- **Issue:** The acceptance criteria call for `go test -run TestFetchTimeoutMarksUnreached` to pass with a case per read. That function lives in `pullrequests_test.go`, which this plan's `files_modified` does not include.
- **Resolution:** Added `TestFetchTimeoutMarksUnreachedWorkItems`, `...Pipelines`, `...Releases` and `...Branches` in the respective test files. Go's `-run` is an unanchored regex, so `-run TestFetchTimeoutMarksUnreached` matches all five (including the original) and the criterion is satisfied without touching a file outside scope.

---

**Total deviations:** 1 acceptance-criterion contradiction (documented, no code changed to chase it), 1 scope-preserving test placement change
**Impact on plan:** No scope creep. No production behavior differs from what the plan specified.

## Known Stubs

None. Every failure site named in the plan is wired and covered by a passing test.

## Issues Encountered

None. All three RED phases failed as expected (compile failures against the old bare-slice return types), and all three GREEN phases passed on first run.

## Assumption Still Unverified

**A4 — that an empty Azure DevOps repository yields a 404-family response to a branch-stats fetch — remains inferred, not observed.** It was inferred from the pre-existing `// Empty repositories have no branch stats to report` comment in `branches.go`, not from a live probe. If the inference is wrong, the consequence is that an empty repository produces a spurious failure entry — noise on the warning line — rather than a real failure being lost. That is the safe direction for it to be wrong in, which is why it was not worth a live-org probe in this plan. If 02-05's warning line proves noisy in practice, this is the first place to look.

## User Setup Required

None.

## Next Phase Readiness

Seven of the eight aggregate reads now return the envelope. Remaining for the phase:

- **02-03** owns `internal/ado/dashboard.go` and `internal/ui/dashboard/dashboard.go`. It was correctly sequenced into wave 3: it edits the same Go package this plan edited, and Go compiles per package, so interleaving would have produced spurious failures in both plans' verify runs.
- **02-05** owns all reducer and render work — the auth banner, the persistent warning line, the stale marker and the completeness-gated zero-row messages of D-13 through D-15. The four views touched here carry the envelope but do not yet read `Failures` or `Complete`; each currently unwraps `.Items` only. That is a deliberate handoff, not an oversight.
- The `TestGet.*SwallowsProjectFailure` characterization names remain in place across all eight reads by design; the rename sweep is the phase's completion signal.

---
*Phase: 02-honest-reads*
*Completed: 2026-09-07*

## Self-Check: PASSED

All 12 modified files verified present on disk. All 6 commit hashes verified present in git history. Full suite green: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test ./... -count=1`.
