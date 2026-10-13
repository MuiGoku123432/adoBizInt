---
phase: 05-hardening
plan: 05
subsystem: api
tags: [go, wiql, azure-devops, parameterized-query, go-ast, slog, security]

requires:
  - phase: 05-hardening
    provides: "05-01 left workitems.go and client.go in their post-attachment shape; 05-02's note asked for the package-file parse walk to be extracted on a third use; 05-04 is the cmd half of HARD-04"
provides:
  - "func iterationClause(paths []string) string in internal/ado/client.go: the single escaping path for server-supplied iteration paths, and the one callee the structural test allow-lists"
  - "Both WIQL sites (getProjectWorkItems, getProjectCounts) filter on [System.TeamProject] = @project; the project travels only in QueryByWiqlArgs.Project (the SDK request route)"
  - "Executing WIQL query and Executing dashboard WIQL query log only project and the iteration count"
  - "internal/ado/wiql_test.go: TestGetWorkItemsUsesProjectMacro, TestDashboardUsesProjectMacro, TestIterationClauseEscapesQuotes, TestWIQLInterpolationStructural"
  - "parsePackageFiles(t) in internal/ado/nilguard_test.go, shared by the three package-wide structural tests"
affects: [05-06 log-permissions-pat-test, 05-VALIDATION HARD-04 rows, /gsd-secure-phase T-05-19..T-05-22, end-of-phase UAT (phase-blocking live read)]

actuals:
  tokens: 5265
  tasks: 2
  commits: 6
plan_head_before: 191b40362bed53fdfa33ac8898fa0dee06da7e8a

tech-stack:
  added: []
  patterns:
    - "WIQL names the project with the @project macro; the name is carried only by the SDK route value, never by query text"
    - "A go/ast structural test with an allow-listed callee, a package-wide literal count to catch evasion outside + chains, and an exact site count so a new site must be reviewed"

key-files:
  created:
    - internal/ado/wiql_test.go
  modified:
    - internal/ado/client.go
    - internal/ado/workitems.go
    - internal/ado/dashboard.go
    - internal/ado/nilguard_test.go
    - internal/ado/attachments_test.go

key-decisions:
  - "iterationClause lives in client.go beside GetRecentIterations and is called inline at both sites. Two uses is below the rule of three; the extraction is forced by D-12 (no bare identifier operand) and carries a note: comment"
  - "The structural test allow-lists exactly one non-literal operand, a call to iterationClause (RESEARCH OQ2). Any other call, such as strings.TrimSpace(project), fails"
  - "The structural test matches FROM WorkItems case-insensitively, looks through parentheses when flattening + chains, and checks @project and the quoted-TeamProject ban against the chain's joined literal text"
  - "parsePackageFiles was extracted because TestWIQLInterpolationStructural is the third glob-and-parse walk, as 05-02's note and the rule of three require"

patterns-established:
  - "Package-wide structural tests call parsePackageFiles(t) instead of repeating the glob-and-parse loop"

requirements-completed: [HARD-04]

coverage:
  - id: D1
    description: "The work-item WIQL filters on @project, contains neither O'Brien nor O''Brien, sends O'Brien as args.Project, and logs no query text"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "internal/ado/wiql_test.go#TestGetWorkItemsUsesProjectMacro"
        status: pass
    human_judgment: false
  - id: D2
    description: "The dashboard WIQL meets the same four assertions, against the Executing dashboard WIQL query line"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "internal/ado/wiql_test.go#TestDashboardUsesProjectMacro"
        status: pass
    human_judgment: false
  - id: D3
    description: "iterationClause returns an empty string for nil and empty input, keeps input order, and doubles single quotes"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "internal/ado/wiql_test.go#TestIterationClauseEscapesQuotes"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every FROM WorkItems + chain holds only literals and iterationClause calls, uses @project, never quotes a project into TeamProject, and there are exactly 2 sites. Sprintf and other out-of-chain literals are caught"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "internal/ado/wiql_test.go#TestWIQLInterpolationStructural"
        status: pass
      - kind: other
        ref: "Mutations, all reverted: quoted project at workitems.go:148 and dashboard.go:136, fmt.Sprintf at the dashboard site, strings.TrimSpace(project) operand, and the base-commit sites. Each made the test fail and name file:line:col"
        status: pass
    human_judgment: false
  - id: D5
    description: "Behaviour is otherwise unchanged: iteration failures still warn (work items) and record opDashboardIterations (dashboard); workitems_test.go and dashboard_test.go pass unedited"
    requirement: HARD-04
    verification:
      - kind: unit
        ref: "go test -race ./internal/ado (TestGetWorkItems*, TestGetDashboardCounts*)"
        status: pass
    human_judgment: false
  - id: D6
    description: "PHASE-BLOCKING live read: for one configured project, workitems list --json item count and dashboard counts --json work_items from the base-commit binary equal those from the HEAD binary, both complete: true"
    requirement: HARD-04
    verification: []
    human_judgment: true
    rationale: "@project has not been exercised against the live org (RESEARCH A3), and CI has no credentials (D-17). Needs the operator's own PAT on their machine. Task 2 human-check, harvested into end-of-phase UAT. Phase 5 is not complete until it passes."

duration: 8min
completed: 2026-09-30
status: complete
---

# Phase 05 Plan 05: WIQL @project Macro Summary

**Both WIQL queries in `internal/ado` now filter on `[System.TeamProject] = @project`, so the project name never enters query text and travels only in the SDK request route. Iteration paths go through a single escaper, `iterationClause`. Neither site logs the query text any more. A go/ast test holds every current and future WIQL site to that shape.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-09-30T17:25:39Z
- **Completed:** 2026-09-30T17:33:30Z
- **Tasks:** 2
- **Files modified:** 6 (3 production, 3 test)

## Accomplishments
- Closed CONCERNS H5 in the library (T-05-19). Before this plan the name was not even escaped. The RED run captured `[System.TeamProject] = 'O'Brien'` in the query sent to the server. The name now reaches ADO only as `args.Project`, which `QueryByWiql` places in the route, where `@project` resolves. `Project: &project` is kept at both sites.
- The TUI never runs 05-04's `--project` allow-list, and it is now safe at this layer too. With 05-04 landed, HARD-04 is complete.
- `iterationClause` is the only place where server-supplied iteration paths enter WIQL (T-05-20). It returns an empty string when there are no paths, keeps input order, and doubles single quotes.
- The query text is no longer logged (D-11, T-05-21). Both `Executing ... WIQL query` lines carry `project` and `iterations` (a count). The work-item `Using iteration filter` line is kept, because it logs server-supplied paths and not query text.
- `TestWIQLInterpolationStructural` fails the suite in any of these cases:
  - a bare identifier operand
  - a call other than `iterationClause`
  - a missing `@project`
  - a quoted `[System.TeamProject] = '`
  - a `FROM WorkItems` literal outside a checked `+` chain
  - a site count other than 2

## Task Commits

1. **Task 1: The work-item WIQL uses @project, escapes iterations through one call, and logs no query (tracer)**: `9cb1ef9` (test, RED) and `933466b` (fix, GREEN)
2. **Task 2: The dashboard WIQL uses @project, and a go/ast test pins both sites**: `5183a9b` (test, RED), `df05d1e` (fix, GREEN), `685ea86` (refactor, parse-walk extraction) and `7359103` (test, structural guard)

## TDD and Tracer Gates

- **Task 1 RED:** I wrote `TestGetWorkItemsUsesProjectMacro` first and ran it alone. It failed on its behaviour assertions: the query had no `@project`, it contained `O'Brien`, and the log contained `FROM WorkItems`. `gsd-tools check tdd-red-evidence` returned `RED_EVIDENCE_OK` (`target_test_failed`) on a TAP record built from `go test -json`. I then added `TestIterationClauseEscapesQuotes`. At commit `9cb1ef9` it was a compile-level RED (`undefined: iterationClause`), which follows the 05-01 Task 1 precedent.
- **Task 1 GREEN:** At `933466b`, both new tests and every `TestGetWorkItems*` test pass under `-race`.
- **Tracer:** Task 1 is a tracer. Auto mode was off, `human_verify_mode` defaulted to end-of-phase, and the `<verify>` is automated-only. I re-ran it end-to-end with `-count=1`. It passed, so I moved on to Task 2 without a checkpoint.
- **Task 2 RED:** `TestDashboardUsesProjectMacro` failed on its assertions at `5183a9b` (`RED_EVIDENCE_OK`, `target_test_failed`).
- **Task 2 GREEN:** It passes at `df05d1e`, along with every `TestGetDashboardCounts*` test.
- **Commit type:** The GREEN commits are typed `fix(05-05)` rather than `feat(05-05)`, because they close a security defect (H5). The orchestrator's brief used `fix(05-05)` as its example. A grep for `^feat\(05-05\)` finds nothing. The RED-before-GREEN order holds for both tasks.
- **Structural test:** `TestWIQLInterpolationStructural` is a guard, not a behaviour test. It passed as soon as it was written because both sites were already fixed. I proved it with mutations instead (see Verification Results).

## Verification Results

| Check | Result |
|---|---|
| `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` | exit 0, 0 FAIL lines |
| `go test ./internal/ado -run 'ProjectMacro\|IterationClause\|WIQLInterpolation' -v` | 4 `--- PASS` lines |
| Mutation: `'" + project + "'` restored at workitems.go | FAIL at `workitems.go:148:218` (bare `*ast.Ident`), plus the missing-`@project` and quoted-TeamProject errors; reverted |
| Mutation: the same at dashboard.go | FAIL at `dashboard.go:136:79`; reverted |
| Mutation: dashboard WIQL wrapped in `fmt.Sprintf` | FAIL: `found 2 FROM WorkItems literals but only 1 inside checked + chains`, and `found 1 WIQL + chains, want exactly 2`; reverted |
| Mutation: `strings.TrimSpace(project)` as an operand | FAIL: `calls something other than iterationClause`; reverted |
| Base-commit `workitems.go` and `dashboard.go` | FAIL with four bare operands (`project` and `iterationFilter` at each site), reproducing RESEARCH Pitfall 4; reverted |
| `iterationFilter` in workitems.go or dashboard.go | none |
| `git diff 191b403..HEAD -- internal/ado/workitems_test.go internal/ado/dashboard_test.go` | empty |

## Human Check (PHASE-BLOCKING, end-of-phase)

This check was not run here. The plan puts it in end-of-phase UAT, and I did not make live ADO calls. **Phase 5 is not complete until it passes, whatever the automated gates say (D-10 revised, T-05-22).**

- **Setup:** Use your own PAT on your machine, and never paste it into a session. Pick one configured project P. Build both binaries:
  `L=$(git rev-list --grep='(05-05)' HEAD) && B=$(printf '%s\n' "$L" | tail -1) && D=$(mktemp -d) && git worktree add --detach "$D/before" "$B^" && (cd "$D/before" && go build -o "$D/adoBizInt-before" ./cmd/adoBizInt) && go build -o "$D/adoBizInt-after" ./cmd/adoBizInt`
  `B` resolves to `9cb1ef9`, so `$B^` is `191b403`, this plan's base.
- **Reads:** Run these one after another, `-before` then `-after` each time:
  - `workitems list --project P --json | jq '[.complete, (.items|length)]'`
  - `dashboard counts --project P --json | jq '[.complete, .items[0].work_items]'`
- **Cleanup:** `git worktree remove "$D/before"`.
- **Expected:**
  - Every run exits 0 and prints `[true, N]`, with the same N from both binaries for each read.
  - In `~/.config/adoBizInt/adoBizInt.log`, the last `Executing WIQL query` line and the last `Executing dashboard WIQL query` line both come from the `-after` runs, carry `iterations=`, and contain no `FROM WorkItems`.

## Files Created/Modified
- `internal/ado/client.go`: `iterationClause`, with a doc comment that gives the D-12 and OQ2 rationale and a `note:` about the two call sites.
- `internal/ado/workitems.go`: `getProjectWorkItems` uses the `@project` chain with `iterationClause(iterations)` inline and the reduced log attributes. The warn branch is unchanged.
- `internal/ado/dashboard.go`: `getProjectCounts` gets the same change. The `GetRecentIterations` error branch, the T-02-05 comment, and `appendFailure(..., opDashboardIterations, err)` are unchanged. The unused `strings` import is dropped.
- `internal/ado/wiql_test.go` (new): the two site tests, with the log capture duplicated under a `note:`; the `iterationClause` table test; `TestWIQLInterpolationStructural`; and its private helpers `flattenAdd`, `isWIQLLiteral` and `stringValue`.
- `internal/ado/nilguard_test.go`: `parsePackageFiles(t)`. `TestCollectedDefinitionsGuardNameAndID` uses it, and its old "extract on a third use" note is gone.
- `internal/ado/attachments_test.go`: `TestNoRawHTTPInADO` uses `parsePackageFiles`. The unused `go/parser` and `go/token` imports are dropped.

## Decisions Made
- **Chain text is checked joined.** The `@project` check and the quoted-TeamProject ban run against all of a site's literals joined together, not one literal at a time. That also catches a ban split across two literals.
- **Parentheses are flattened.** `flattenAdd` looks through `ParenExpr`, so wrapping part of a chain in parentheses cannot hide a bare operand.
- **Case-insensitive site detection.** WIQL keywords are case-insensitive, so `from workitems` also counts as a site. This slightly tightens the rule in the plan, which named the exact text `FROM WorkItems`.

## Deviations from Plan

**1. [Rule 2 - CLAUDE.md rule of three] Extracted the package-file parse walk**
- **Found during:** Task 2, while writing `TestWIQLInterpolationStructural`.
- **Issue:** The new test would have been the third copy of the glob-and-parse loop. `nilguard_test.go` carried a 05-02 note saying "extract it on a third use", and the user's rule of three requires the extraction.
- **Fix:** Added `parsePackageFiles(t)` to `nilguard_test.go` and moved both existing callers onto it with no assertion changes. `TestWIQLInterpolationStructural` uses it too.
- **Files modified:** `internal/ado/nilguard_test.go`, `internal/ado/attachments_test.go`. Neither file is in the plan's `files_modified`.
- **Verification:** `TestCollectedDefinitionsGuardNameAndID` and `TestNoRawHTTPInADO` pass unchanged, and the full `-race` suite is green.
- **Committed in:** `685ea86` (refactor)

**Total deviations:** 1 (a CLAUDE.md-driven extraction). **Impact:** test code only. No production scope change.

## Issues Encountered
- My first draft of the structural test counted every `FROM WorkItems` literal in the same `ast.Inspect` pass that returns `false` at each site. It therefore never reached the literals inside sites and reported `found 0 ... but only 2`. I moved the count into its own pass, as the plan specified.

## Threat Flags

None. The plan removes surface: query text leaves the log, and the project leaves the query text. It adds no new endpoint, file access, or trust boundary.

## User Setup Required

None for the code. The phase-blocking human check above needs the operator's own configured PAT.

## Next Phase Readiness
- HARD-04 is complete in code (05-04 cmd layer and 05-05 library layer). Its live-read confirmation is still pending in end-of-phase UAT.
- 05-06 (log permissions and the PAT test) can reuse the `slog.SetDefault` capture seam used here. That would be the third use of the log capture, so extract it then.

## Self-Check: PASSED
- All 6 created or modified files exist on disk.
- Commits `9cb1ef9`, `933466b`, `5183a9b`, `df05d1e`, `685ea86` and `7359103` are present.
- `git rev-list --count 191b403..HEAD` = 6 before the SUMMARY commit.

---
*Phase: 05-hardening*
*Completed: 2026-09-30*
