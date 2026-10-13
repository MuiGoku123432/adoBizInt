---
phase: 02-honest-reads
plan: 06
subsystem: api
tags: [go, azure-devops, failure-envelope, n-plus-one, fetch-deadline, gap-closure]

requires:
  - phase: 02-honest-reads
    provides: "Result[T] envelope, appendFailure/classifyFailure (02-01), per-project D-12 pre-flight (02-02), 02-04 JSON field contract"
  - phase: 05-hardening
    provides: "Name/Id guard on the collecting loops and TestCollectedDefinitionsGuardNameAndID (05-02)"
provides:
  - "GetPipelineRuns records a failed per-definition GetBuilds call as a classified Failure ('get latest build for <definition>') and omits the row"
  - "GetReleases records a failed per-definition GetReleases call as a classified Failure ('get latest release for <definition>') and omits the row"
  - "GetPipelineDefinitions records a failed per-definition GetDefinition call as a classified Failure ('get definition detail for <definition>') and omits the definition"
  - "Per-definition ctx.Err() pre-flight (D-12) inside all three N+1 loops"
  - "Test helpers assertDefinitionFailure and twoPipelineDefinitions in internal/ado/pipelines_test.go"
affects: [02-verification, phase-03-cli-json, pipelines-view, releases-view, trigger-modal]

actuals:
  tokens: 5683
  tasks: 3
  commits: 6
plan_head_before: cb4faddaa13748a606c19ab69bfcd2275edb2976

tech-stack:
  added: []
  patterns:
    - "Per-definition failure label: op<Name> + \" for \" + definition name, mirroring GetBranches' per-repository label"
    - "A per-definition lookup that failed is omitted from Items and named in Failures (G1); legitimate empty answers keep neverRun/noReleases"
    - "D-12 pre-flight repeated inline inside each N+1 loop, not only between projects"

key-files:
  created: []
  modified:
    - internal/ado/pipelines.go
    - internal/ado/pipelines_test.go
    - internal/ado/releases.go
    - internal/ado/releases_test.go

key-decisions:
  - "G1: a definition whose per-definition lookup failed is omitted from Items, not given an explicit unknown-status row. This matches GetBranches, leaves the 02-04 JSON contract and the views unchanged, and is reversible"
  - "G1 applied to GetPipelineDefinitions: a failed detail lookup omits the definition rather than returning it with a blank repository_id/default_branch, which the trigger modal would later surface as an unrelated branch-fetch error. A nil body with no error still yields the row"
  - "The three per-definition loops stay inline. The shared logic is already appendFailure; each site differs in its SDK call, row type and success mapping"

patterns-established:
  - "Per-definition failure label: op + \" for \" + name, for every N+1 sub-request"
  - "Failure-path test shape: two matching definitions (ok, broken), table-driven auth/timeout subtests, plus a 100ms-deadline unreached test that counts SDK calls"

requirements-completed: [TRUST-01, TRUST-02, TRUST-03]

coverage:
  - id: D1
    description: "GetPipelineRuns names a failed per-definition build lookup in Failures (auth and timeout kinds), omits the row, and sets Complete=false"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineRunsReportsDefinitionFailure"
        status: pass
    human_judgment: false
  - id: D2
    description: "GetReleases names a failed per-definition release lookup in Failures (pointer-form 403 auth and timeout), omits the row, and sets Complete=false"
    requirement: TRUST-03
    verification:
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetReleasesReportsDefinitionFailure"
        status: pass
    human_judgment: false
  - id: D3
    description: "GetPipelineDefinitions names a failed per-definition detail lookup in Failures and omits the definition. The surviving definition keeps its repository_id/default_branch, and the nil-body path is unchanged"
    requirement: TRUST-01
    verification:
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineDefinitionsReportsDetailFailure"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineDefinitionsHappyPathRowCountUnchanged"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-12 inside all three per-definition loops: once the deadline expires, unreached definitions get a timeout Failure naming them and no further SDK call is issued"
    requirement: TRUST-02
    verification:
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineRunsMarksUnreachedDefinitions"
        status: pass
      - kind: unit
        ref: "internal/ado/releases_test.go#TestGetReleasesMarksUnreachedDefinitions"
        status: pass
      - kind: unit
        ref: "internal/ado/pipelines_test.go#TestGetPipelineDefinitionsMarksUnreachedDefinitions"
        status: pass
    human_judgment: false
  - id: D5
    description: "Structural guards and contracts unchanged: TestCollectedDefinitionsGuardNameAndID still finds exactly 2 loops with nilguard_test.go unedited, and TestJSONFieldNames and the SkipsNilID tests stay green"
    verification:
      - kind: unit
        ref: "internal/ado/nilguard_test.go#TestCollectedDefinitionsGuardNameAndID"
        status: pass
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... && go test -race -count=1 ./... && test -z \"$(gofmt -l .)\""
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-10-12
status: complete
---

# Phase 2 Plan 06: Per-definition lookup failures reported, not swallowed

**GetPipelineRuns, GetReleases and GetPipelineDefinitions now record a failed per-definition N+1 call as a classified, named Failure and omit the row, with a D-12 deadline pre-flight inside each loop. A rejected token or timeout can no longer render as neverRun, noReleases, or a blank repository_id under complete:true.**

## Performance

- **Duration:** about 5 min
- **Started:** 2026-10-12T15:34:54Z
- **Completed:** 2026-10-12T15:40Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Gap 1 (BLOCKER) is closed for both reads the verifier named. GetBuilds and GetReleases errors inside the per-definition loops now go through `appendFailure`, are logged at Error, and leave the definition out of Items.
- The fourth swallow of the same class, the GetDefinition call inside GetPipelineDefinitions (coordinator-approved), is closed the same way. The `err == nil && fullDef != nil` condition no longer drops the error.
- D-12 now holds inside all three N+1 loops. Once the fetch deadline expires, each unreached definition gets a timeout Failure naming it, and no doomed SDK call is issued.
- There are three new operation labels: `opGetLatestBuild = "get latest build"`, `opGetLatestRelease = "get latest release"` and `opGetDefinitionDetail = "get definition detail"`. Each is used as `op + " for " + <definition name>`, so the view's warning line reads like `could not read: P (auth: get latest build for Deploy-API)`.

## Task Commits

TDD tasks each produced a RED test commit and then a GREEN fix commit:

1. **Task 1 (tracer): GetPipelineRuns per-definition build failures**: `ae99ba8` (test, RED) and `cc9edbf` (fix, GREEN)
2. **Task 2: GetReleases per-definition release failures**: `a7ca993` (test, RED) and `b84ed33` (fix, GREEN)
3. **Task 3: GetPipelineDefinitions per-definition detail failures**: `25eae66` (test, RED) and `a1d29ad` (fix, GREEN)

No REFACTOR commit was needed. After GREEN the three loops read alike and stay inline, as the plan directs.

## RED → GREEN Evidence

Each RED run used the task's own test names, in the form `go test -race -count=1 -v ./internal/ado -run '^(<new tests>)$'`. Every RED run exited 1, and every failure was an assertion about the planned behavior. None were compile errors.

| Test | RED (before fix; exit 1) | GREEN |
|------|--------------------------|-------|
| TestGetPipelineRunsReportsDefinitionFailure/auth | Items held `pipeline-broken` with `Status:neverRun`, and `Failures = []` (so Complete was true) | PASS |
| TestGetPipelineRunsReportsDefinitionFailure/timeout | same: a `neverRun` row and `Failures = []` | PASS |
| TestGetPipelineRunsMarksUnreachedDefinitions | `GetBuilds call 2 issued after the fetch deadline expired`, `calls = 2, want 1`, a neverRun row and `Failures = []` | PASS (1 call, 1 timeout Failure) |
| TestGetReleasesReportsDefinitionFailure/auth | Items held `DefinitionID:32 DefinitionName:release-broken ... Status:noReleases`, and `Failures = []` | PASS |
| TestGetReleasesReportsDefinitionFailure/timeout | same: a `noReleases` row and `Failures = []` | PASS |
| TestGetReleasesMarksUnreachedDefinitions | `GetReleases call 2 issued after the fetch deadline expired`, `calls = 2, want 1`, `Failures = []` | PASS |
| TestGetPipelineDefinitionsReportsDetailFailure/auth | Items held `{ID:18 Name:pipeline-broken ... RepositoryID: DefaultBranch:}`, a blank-repository row | PASS (survivor carries `repo-guid-17` / `main`) |
| TestGetPipelineDefinitionsReportsDetailFailure/timeout | same blank-repository row for pipeline-broken | PASS |
| TestGetPipelineDefinitionsMarksUnreachedDefinitions | `GetDefinition call 2 issued after the fetch deadline expired`, `calls = 2, want 1`, `Failures = []` | PASS |

`gsd-tools check tdd-red-evidence` parses node:test TAP output and cannot read Go `testing` output. The plan is `type: execute` with `tdd_mode: false`, so the plan-level gate does not apply, and the table above is the RED record.

Plan-level gate-specific run: 6 top-level `--- PASS` lines and 6 subtest `--- PASS` lines, with no FAIL.

## Files Created/Modified

- `internal/ado/pipelines.go`: adds the `opGetLatestBuild` and `opGetDefinitionDetail` labels, a per-definition D-12 pre-flight in GetPipelineRuns and GetPipelineDefinitions, recording of GetBuilds and GetDefinition errors with the row omitted, and updated doc comments.
- `internal/ado/releases.go`: adds the `opGetLatestRelease` label, a per-definition D-12 pre-flight, recording of GetReleases errors with the row omitted, and an updated doc comment.
- `internal/ado/pipelines_test.go`: adds four new tests plus the `assertDefinitionFailure` helper (6 callers across two files) and `twoPipelineDefinitions` (4 callers).
- `internal/ado/releases_test.go`: adds two new tests.

## Decisions Made

- **G1 (omit the failed row).** A definition whose per-definition lookup failed is left out of Items and named in Failures. There is no explicit "unknown" status row. This mirrors GetBranches, adds no field or `status` value to the frozen 02-04 contract, and needs no view change. The existing `could not read:` warning already prints the operation, which now names the definition. A known consequence, consistent with D-14: a per-definition 401/403 is classified `auth`, so the view shows its blocking auth banner, just as a per-repository branch failure already does.
- **G1 applied to GetPipelineDefinitions.** A failed detail lookup omits the definition rather than keeping it with a blank `repository_id`/`default_branch`. Only the `err != nil` path changed. A nil body with no error still yields the row, which TestGetPipelineDefinitionsHappyPathRowCountUnchanged pins. The view's secondary-fetch warning ("Could not load pipeline definitions - trigger may not work") fires on any Failure, and the remaining definitions stay usable.
- **Nil-Id edge case (recorded, not changed).** The GetPipelineDefinitions loop guards only `d.Name`. A matching definition with a nil Id makes the SDK return ArgumentNilError on the client side. It is now recorded as an other-kind Failure for that definition, where before it produced an ID-0 row with blank repository fields. This errs toward noise rather than a lost failure. Adding a nil-Id guard is Phase 5 D-14 scope and was prohibited here.
- **Structural guard unchanged.** TestCollectedDefinitionsGuardNameAndID still counts exactly 2 collecting loops, and `git diff --stat -- internal/ado/nilguard_test.go` prints nothing. The GetPipelineRuns and GetReleases loops keep `defID := *def.Id` / `defName := *def.Name` as their first two statements. The GetPipelineDefinitions loop still appends the constructed `def`, not its range value, so the AST walk still does not count it.
- **Pre-flight position in GetPipelineDefinitions.** The pre-flight sits after the pattern-match filter, so unmatched definitions stay data filters and are never recorded.

## Deviations from Plan

None. The plan executed exactly as written.

There was one small judgment call within the plan's latitude. A one-build answer and a repository-detail answer each appear in exactly two tests. Following the user's rule of three, they stay inline with `// note:` comments rather than becoming helpers. `twoPipelineDefinitions` (4 callers) and `assertDefinitionFailure` (6 callers) are shared test helpers.

## Issues Encountered

None.

## User Setup Required

None. No external service configuration is required.

## Human-check items (end-of-phase)

None are required by this plan; every verify step was automated. An optional live check: with a token that can list definitions but not builds, the pipelines view should show `could not read: <project> (auth: get latest build for <pipeline>)` instead of a "never run" row.

## Next Phase Readiness

- Gap 1 is closed. Ready for 02-07, the other gap-closure plan, and then Phase 2 re-verification by the orchestrator.
- No blockers.

## Self-Check: PASSED

- FOUND: internal/ado/pipelines.go, internal/ado/pipelines_test.go, internal/ado/releases.go, internal/ado/releases_test.go
- FOUND commits: ae99ba8, cc9edbf, a7ca993, b84ed33, 25eae66, a1d29ad (`git rev-list --count cb4fadd..HEAD` = 6 before this SUMMARY)
- The module gate (build, vet, race tests, gofmt) exits 0. The key-link patterns `opGetLatestBuild+" for "+defName`, `opGetLatestRelease+" for "+defName` and `opGetDefinitionDetail+" for "+*d.Name` are all present.

---
*Phase: 02-honest-reads*
*Completed: 2026-10-12*
