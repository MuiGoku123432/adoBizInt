---
phase: 03-read-cli
plan: 04
subsystem: cli
tags: [go, filters, search, refactor, deduplication, tdd, stdlib-flag, generics]

requires:
  - phase: 03-read-cli
    provides: readClient seam, eight-entry dispatch table, generic runRead body, newFlagSet registrar, --project parsed-but-unapplied
  - phase: 03-read-cli
    provides: internal/ado/filters.go with five promoted predicates, empty-slice-means-match-all
provides:
  - "ado.MatchesSearch, the single free-text search rule consumed by all five searching views"
  - "five view packages with ordered-row equivalence tests over buildRows()"
  - "per-verb filter flags --search, --assigned-to-me, --hide-done, --hide-tasks, --repo"
  - "the application point for --project: the projects argument of every dispatched read"
  - "cmd/adoBizInt/filter.go, the five post-filters over Result.Items"
  - "readClient.CurrentUserEmail(), the identity seam --assigned-to-me filters on"
affects: [03-05]

actuals:
  tokens: 26000
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "a shared matcher takes the caller's field list variadically rather than type-switching over payloads"
    - "read-narrowing flags become read-method arguments; post-filtering flags are applied to Items after the read"
    - "an omitted list flag passes an empty slice, never the configured list, so the fallback lives in one place"
    - "per-verb flag registration keyed on the same noun-verb string as the dispatch table"

key-files:
  created:
    - internal/ui/branches/branches_rows_test.go
    - internal/ui/releases/releases_rows_test.go
    - internal/ui/pipelines/pipelines_rows_test.go
    - cmd/adoBizInt/filter.go
    - cmd/adoBizInt/filter_test.go
  modified:
    - internal/ado/filters.go
    - internal/ado/filters_test.go
    - internal/ui/workitems/workitems.go
    - internal/ui/workitems/workitems_rows_test.go
    - internal/ui/prs/prs.go
    - internal/ui/prs/prs_rows_test.go
    - internal/ui/branches/branches.go
    - internal/ui/releases/releases.go
    - internal/ui/pipelines/pipelines.go
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/usage.go
    - cmd/adoBizInt/usage_test.go
    - cmd/adoBizInt/fakes_test.go

key-decisions:
  - "MatchesSearch takes a variadic list of field strings rather than an item value, because five views search five different field sets and a single item-typed signature would recover them with a type switch — the duplication in a new shape"
  - "The matcher trims and case-folds the query itself, so it is idempotent over the normalization the five views already perform and there is no second rule that can drift"
  - "--project and --repo narrow the read at the method parameter; --search, --assigned-to-me, --hide-done and --hide-tasks post-filter Items after the read returns"
  - "An omitted --project or --repo passes an empty slice rather than the configured list, because every read method already falls back to its own configured list on an empty slice"
  - "Filter flags are registered only on the verbs whose payload supports them, because a flag that parses but cannot mean anything misleads an agent reading --help"
  - "--assigned-to-me on an empty identity cache warns on stderr rather than silently returning an unfiltered list the caller believes is filtered"
  - "readClient gained CurrentUserEmail() rather than threading an identity through main, keeping the client seam the single source of the cached identity"
  - "projectList was renamed stringList because it now backs --repo as well; keeping the old name for a repository list would have been a naming lie"

patterns-established:
  - "Search field sets are named at the call site, never inferred from a payload type"
  - "Row-equivalence tests assert the full ordered []table.RowData, including order-bearing indices"
  - "A read verb applies no filter the caller did not request, asserted by a zero-flag test per filterable verb"

requirements-completed: [CLI-06, CLI-01]

coverage:
  - id: D1
    description: "The free-text search rule is defined once in internal/ado and consumed by all five views that have a search filter"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesSearch"
        status: pass
      - kind: unit
        ref: "internal/ado/filters_test.go#TestFilterMatchesSearchIsIdempotentOverNormalization"
        status: pass
      - kind: other
        ref: "grep -rn 'strings.Contains(.*searchQuery' internal/ui/ | grep -v _test.go returns nothing"
        status: pass
    human_judgment: false
  - id: D2
    description: "Each of the five views searches exactly the same field set after the rewire as before it, and buildRows() returns the same ordered rows"
    requirement: CLI-01
    verification:
      - kind: unit
        ref: "internal/ui/workitems/workitems_rows_test.go#TestBuildRowsSearchEquivalence"
        status: pass
      - kind: unit
        ref: "internal/ui/prs/prs_rows_test.go#TestBuildRowsSearchEquivalence"
        status: pass
      - kind: unit
        ref: "internal/ui/branches/branches_rows_test.go#TestBuildRowsSearchEquivalence"
        status: pass
      - kind: unit
        ref: "internal/ui/releases/releases_rows_test.go#TestBuildRowsSearchEquivalence"
        status: pass
      - kind: unit
        ref: "internal/ui/pipelines/pipelines_rows_test.go#TestBuildRowsSearchEquivalence"
        status: pass
    human_judgment: false
  - id: D3
    description: "branches keeps its 'for i, item :=' loop and carries the original m.items index onto each row, so a filtered-out earlier item does not renumber the survivors"
    requirement: CLI-01
    verification:
      - kind: unit
        ref: "internal/ui/branches/branches_rows_test.go#TestBuildRowsPreservesOriginalIndex"
        status: pass
      - kind: other
        ref: "grep -n 'for i, item := range m.items' internal/ui/branches/branches.go matches"
        status: pass
    human_judgment: false
  - id: D4
    description: "A read verb invoked with no filter flags returns the unfiltered set, even where the TUI defaults to filtering (D-18, T-03-11)"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestDefaultsApplyNoFilter"
        status: pass
    human_judgment: false
  - id: D5
    description: "--search, --assigned-to-me, --hide-done and --hide-tasks post-filter the returned Items and are registered only on the verbs whose payload supports them"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestFilterFlagsPostFilterItems"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestFilterFlagsRegisteredOnSupportedVerbsOnly"
        status: pass
      - kind: other
        ref: "ADO_PAT= go run ./cmd/adoBizInt workitems list --help lists all four; releases approvals --help lists none"
        status: pass
    human_judgment: false
  - id: D6
    description: "--project narrows the read as the projects argument; an omitted flag passes an empty slice so every configured project is returned"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestFilterFlagsProjectNarrowsTheRead"
        status: pass
    human_judgment: false
  - id: D7
    description: "--repo on branches list is GetBranches' repositories argument and is not additionally post-filtered; an omitted flag passes an empty slice, never the configured repository list"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestFilterFlagsRepoNarrowsTheBranchesRead"
        status: pass
      - kind: other
        ref: "grep -rn 'cfg\\.Repositories|cfg\\.Projects' cmd/adoBizInt/dispatch.go returns nothing"
        status: pass
    human_judgment: false
  - id: D8
    description: "--assigned-to-me with an unresolved identity returns every item AND announces the no-op on stderr (T-03-12)"
    requirement: CLI-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/filter_test.go#TestFilterFlagsAssignedToMeWarnsOnEmptyIdentity"
        status: pass
    human_judgment: false
  - id: D9
    description: "The TUI renders and filters identically after the five-view rewire, against a live Azure DevOps organisation"
    requirement: CLI-01
    verification:
      - kind: manual_procedural
        ref: "03-04-PLAN.md Task 3 checkpoint:human-verify steps 2-5"
        status: pass
    human_judgment: true
    rationale: "Terminal rendering has no assertion harness in this repo. The ordered-row tests pin buildRows() against fixtures, but whether the six views start, filter and search identically with a real PAT, a real identity and real priorities on disk needs a human at an interactive terminal. Verified and approved at the Task 3 checkpoint."

duration: 34 min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 04: Shared Search Matcher and the CLI Filter Surface Summary

**The last inline filtering rule — a `strings.Contains` chain duplicated across five views over five different field sets — collapsed into `ado.MatchesSearch`, and the whole promoted filter set surfaced as per-verb CLI flags that default to off.**

## Performance

- **Duration:** 34 min
- **Tasks:** 3 (2 code, 1 human-verify checkpoint)
- **Files modified:** 19 (5 created, 14 modified)

## Accomplishments

- Filtering now exists in exactly one place. `grep -rn 'strings.Contains(.*searchQuery' internal/ui/` returns nothing outside tests: no view package holds a search comparison any more, which is the duplication CLI-06 exists to remove.
- `MatchesSearch` takes the caller's field list variadically rather than an item value. RESEARCH Pitfall 5 named the trap precisely — five call sites with five field sets collapse into a type switch if you force one signature — and the variadic form avoids it by letting each view name its own fields while sharing only the comparison.
- The matcher trims and case-folds the query itself, so it is **idempotent** over the normalization the five views already perform. The call-site `strings.ToLower(strings.TrimSpace(...))` is now a no-op rather than a second rule that can drift, and that idempotence is pinned by its own test.
- All three of the plan's named mechanical traps were live in the source and all three were avoided: `branches.go` really does range with `for i, item :=` and consume the index downstream; the receiver field is `m.items` / `m.releases` / `m.runs` across the five; and `branches.go` really did inline `strings.ToLower` inside `Contains`. A blind five-file substitution would have broken the first.
- First test files covering `buildRows()` in `branches`, `releases` and `pipelines` at all. Combined with 03-03's two, all five filtering views now have an ordered-row regression net that did not exist before this phase.
- `--project` is finally applied. 03-02 shipped it parsed-but-discarded at a `_ = gf.projects` line; it is now the `projects` argument of every one of the eight dispatched reads, and the honesty debt 03-02 recorded is paid.
- **Two flags narrow the read, four post-filter it, and the distinction is enforced rather than documented.** `--project` and `--repo`-on-branches become read-method arguments; `filterBranches` deliberately applies search only, and a test asserts it does not re-apply the repository rule.
- An omitted `--project` or `--repo` passes an **empty slice**, never `cfg.Projects` / `cfg.Repositories`. The branches test proves this with a fixture branch in a repository outside any plausible config list: a closure that leaked the configured list would drop it, silently handing a caller who asked for no filter a config-filtered subset.
- Filter flags are registered only on the verbs whose payload supports them. `workitems list --help` shows all four; `releases approvals --help` shows none. A flag that parses but cannot mean anything is worse than one that does not exist, because an agent reading help would believe it filters.
- `--assigned-to-me` on an unresolved identity returns every item **and says so on stderr**. Phase 1 D-12 makes identity resolution non-fatal, so `ado.MatchesIdentity` is a deliberate no-op on an empty cache; silently returning an unfiltered list to a caller who asked for a filtered one is exactly the spoofing failure T-03-12 names.

## Task Commits

1. **Task 1 RED: failing matcher table** — `322a37e` (test)
2. **Task 1 GREEN: shared search matcher** — `4131104` (feat)
3. **Task 1: five-view rewire + ordered-row tests** — `c6e0bca` (refactor)
4. **Task 2: CLI filter flags** — `a319f33` (feat)

**Plan metadata:** this commit (docs)

## Files Created/Modified

- `internal/ado/filters.go` — `MatchesSearch`, the sixth and last promoted predicate
- `internal/ado/filters_test.go` — twelve behavior rows plus the normalization-idempotence check
- `internal/ui/workitems/workitems.go` — search chain replaced; ID formatted at the call site
- `internal/ui/prs/prs.go` — four-field search chain replaced
- `internal/ui/branches/branches.go` — three-field chain replaced; `for i, item :=` loop preserved
- `internal/ui/releases/releases.go` — two-field chain replaced
- `internal/ui/pipelines/pipelines.go` — four-field chain replaced
- `internal/ui/{branches,releases,pipelines}/*_rows_test.go` — new ordered-row search equivalence, plus the branches original-index pin
- `internal/ui/{workitems,prs}/*_rows_test.go` — search subtests added to the 03-03 files
- `cmd/adoBizInt/flags.go` — `filterFlags`, `filterSupportByCommand`, per-verb registration; `projectList` renamed `stringList`
- `cmd/adoBizInt/filter.go` — the five post-filters and `warnUnusableIdentity`
- `cmd/adoBizInt/dispatch.go` — `readClient.CurrentUserEmail()`, `ignoreRepos`, `runRead` gains identity/post-filter and applies `--project`/`--repo`
- `cmd/adoBizInt/filter_test.go` — the six filter-flag behavior tests
- `cmd/adoBizInt/{usage.go,usage_test.go,fakes_test.go}` — updated for the three-value `newFlagSet` and the widened interface

## Decisions Made

Beyond the plan's stated decisions, three judgment calls were made during execution:

- **`readClient` gained `CurrentUserEmail()`.** The plan says to resolve the identity from the client's cache but does not say where the seam lives. Threading it from `main` would have created a second identity source; putting it on the interface keeps the client the single origin and lets the fake set an empty value to exercise the T-03-12 path directly.
- **`ignoreRepos` adapts the seven reads that take only projects.** `runRead`'s fetch signature is now `(ctx, projects, repositories)` so the one verb with a repositories parameter needs no special case at the call site. The alternative — building the branches fetch closure inside `runRead` after parsing — would have made `runRead` know which verb it was running, which is precisely the drift the generic body exists to prevent.
- **`projectList` renamed to `stringList`.** It now backs `--repo` as well. Reusing a type named for projects to carry repositories is the kind of small lie that later reads as a bug.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Two acceptance-criteria greps matched prose comments rather than code**

- **Found during:** Task 2 verification
- **Issue:** `grep -rn 'cfg\.' cmd/adoBizInt/flags.go` matched a comment explaining the `--timeout` ordering constraint, and `grep -rn 'cfg\.Repositories\|cfg\.Projects' cmd/adoBizInt/dispatch.go` matched a comment explaining why the configured list is *not* passed. Both criteria demand the grep return nothing. The guards were being defeated by their own explanations.
- **Fix:** Reworded both comments to say "config validation" and "the configured repository list". The reasoning is preserved; the mechanical check is now clean.
- **Files modified:** cmd/adoBizInt/flags.go, cmd/adoBizInt/dispatch.go
- **Verification:** Both greps return nothing; `go build ./... && go vet ./... && go test ./...` green.
- **Committed in:** `a319f33`

This is the same class of fix 03-02 applied to its `lipgloss|bubble` criterion, and the second time in this phase a grep-based guard has been undermined by a comment. Worth noting as a pattern: a criterion expressed as "grep returns nothing" constrains prose as well as code.

**2. [Rule 3 - Blocking] Wrong import path in two new test files**

- **Found during:** Task 1
- **Issue:** The new `releases` and `pipelines` row tests imported `sentinovo.ai/bizInt/internal/styles`; the package is at `internal/ui/styles`.
- **Fix:** Corrected both import paths.
- **Files modified:** internal/ui/releases/releases_rows_test.go, internal/ui/pipelines/pipelines_rows_test.go
- **Verification:** Both packages compile and their tests pass.
- **Committed in:** `c6e0bca`

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking).
**Impact on plan:** None on behavior. Neither changed what any code does; one made two guards mechanically honest and one fixed a typo before it was committed.

## Issues Encountered

None beyond the two deviations. Task 1's RED phase failed on its assertions rather than the build, as designed, and went green on the first GREEN commit. Task 2 passed all nine acceptance criteria on the first verification run once the two comment greps were cleaned.

## TDD Gate Compliance

Task 1 carried `tdd="true"` and completed both gates: RED at `322a37e` (`test(03-04)`, `MatchesSearch` stubbed to return `false` so the assertions rather than the build failed — nine of twelve rows failed as intended), GREEN at `4131104` (`feat(03-04)`). No REFACTOR commit was needed. The five-view rewire at `c6e0bca` is Task 1's non-TDD half; its tests are behavior-equivalence pins written alongside the change.

## Checkpoint Outcome

Task 3, `checkpoint:human-verify` with `gate="blocking"`, was **approved**. The human ran the built binary and confirmed all four remaining steps:

- The TUI starts exactly as before.
- Work items: "Assigned to Me", "Hide Done" and "Hide Tasks" each filter as before independently, and prioritized items still sort to the top with "Assigned to Me" on.
- PRs, branches, releases and pipelines: the same rows match on search — specifically PRs on repository and author, branches on last-commit-by, releases on definition name, pipelines on build number.
- Every filter checkbox's default state on first launch is unchanged.

Step 1 (`go build ./... && go test ./...`) was automated and green before the checkpoint was presented.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Ready for 03-05. CLI-06 and CLI-01 are complete: every filtering rule in this repository is now a single exported function in `internal/ado`, consumed by both the TUI and the CLI, and every promoted filter is reachable as a flag.

One honesty debt remains from 03-02 and is 03-05's to pay: **`--timeout`** is still parsed into `globalFlags.timeout` and never turned into a context deadline. The override must land after `cfg.Validate()` so a CLI flag cannot trip the `config.go:159` FetchTimeout check.

**Carried to phase UAT:** nothing new from this plan — the live-organisation behavioral equivalence question that 03-03 deferred was answered by this plan's Task 3 checkpoint (coverage entry D9), which covered all five rewired views rather than only the two 03-03 touched.

## Self-Check: PASSED

- All five created files exist on disk.
- All four task commit hashes resolve in `git log`.
- `go build ./... && go vet ./... && go test ./...` all green.
- `grep -rn 'strings.ToLower(strings.TrimSpace(m.searchInput.Value()))' internal/ui/ | wc -l` returns 5.
- `grep -rn 'strings.Contains(.*searchQuery' internal/ui/ | grep -v _test.go` returns nothing.
- Both Task 2 config greps return nothing.
- The verification build artifact `./adoBizInt` is gitignored at `.gitignore:2` and was not committed.

---
*Phase: 03-read-cli*
*Completed: 2026-09-09*
