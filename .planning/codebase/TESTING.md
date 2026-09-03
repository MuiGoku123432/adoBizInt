# Testing Patterns

**Analysis Date:** 2026-09-03

Scope: `sentinovo.ai/bizInt`, Go 1.25.4, 30 Go files, 7,601 lines.

Headline: **2 test files, 82 lines of test code, 3 test functions.** 14 of 16 packages have zero tests. The package that performs irreversible writes against production Azure DevOps has 0.0% coverage and no seam that would allow it to be tested.

---

## Test Framework

**Runner:** Go stdlib `testing`. No third-party test framework.

**Assertion library:** None. Tests use plain `if got != want { t.Errorf(...) }`.

`github.com/stretchr/testify` appears at `go.sum:69-70` but is **not** in any `require` block in `go.mod`. It is a transitive entry only and cannot be imported without a `go get`. Verified.

**Configuration:** None. No `TestMain`, no build tags, no `-race` default, no coverage threshold.

**Run commands** (no Makefile or task runner exists; these are raw Go commands):

```bash
go test ./...                          # Run all tests
go test ./... -cover                   # With coverage summary
go test ./... -race                    # With race detector (never run in this repo per git history)
go test ./internal/utils -v            # Single package, verbose
go test ./... -coverprofile=c.out && go tool cover -html=c.out
```

**CI:** None. Verified absent: `.github/`, `.gitlab-ci.yml`, `azure-pipelines.yml`, `Makefile`, `Taskfile.yml`, `justfile`, `.pre-commit-config.yaml`. Nothing runs these tests except a human typing `go test`.

---

## Coverage Reality

Measured 2026-09-03 with `go test ./... -cover`.

| Package | Coverage | Test files | Non-test lines |
|---------|----------|------------|----------------|
| `internal/utils` | 36.4% | 1 | 81 |
| `internal/ui/releases` | 13.1% | 1 | 777 |
| `internal/ado` | 0.0% | 0 | 2,015 |
| `internal/ui/workitems` | 0.0% | 0 | 1,669 |
| `internal/ui/pipelines` | 0.0% | 0 | 895 |
| `internal/ui/prs` | 0.0% | 0 | 524 |
| `internal/ui/branches` | 0.0% | 0 | 350 |
| `internal/ui/dashboard` | 0.0% | 0 | 327 |
| `internal/config` | 0.0% | 0 | 319 |
| `internal/ui` | 0.0% | 0 | 224 |
| `internal/keymap` | 0.0% | 0 | 88 |
| `cmd/adoBizInt` | 0.0% | 0 | 69 |
| `internal/logging` | 0.0% | 0 | 49 |
| `internal/ui/link` | 0.0% | 0 | 39 |
| `internal/ui/modal` | 0.0% | 0 | 24 |
| `internal/ui/styles` | no test files | 0 | 69 |

Only `internal/ui/styles` reports `[no test files]` rather than `0.0%` because it declares no functions, only package-level `var` blocks.

Aggregate: roughly **2.3% of the codebase is covered** (the two covered packages contribute ~140 covered statements against a 7,519-line non-test body). Treat that number as indicative, not precise -- Go reports coverage per statement, not per line.

---

## What the 2 Existing Tests Actually Cover

### `internal/utils/text_test.go` (30 lines, 1 test function)

Covers exactly one function: `Truncate` at `internal/utils/text.go:6`.

Style: **table-driven with subtests.** This is the better of the two files and is the reference pattern for pure-function testing in this repo.

```go
tests := []struct {
	name   string
	in     string
	maxLen int
	want   string
}{
	{"shorter than limit", "hello", 10, "hello"},
	...
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		if got := Truncate(tt.in, tt.maxLen); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
		}
	})
}
```
-- `internal/utils/text_test.go:6-29`

Observed conventions in this file:

| Convention | Evidence |
|-----------|----------|
| Anonymous struct slice named `tests`, loop var `tt` | `text_test.go:6`, `:23` |
| Descriptive lowercase case names, no `Test` prefix | `"trimmed with ellipsis"`, `"accented at boundary"` |
| `t.Run(tt.name, ...)` subtests | `text_test.go:24` |
| `got`/`want` naming | `text_test.go:25-26` |
| `%q` for strings in failure messages | `text_test.go:26` |
| Failure message restates the full call and both values | `text_test.go:26` |
| Regression cases carry a `// Regression:` comment explaining the original bug | `text_test.go:16` |
| Internal test package (`package utils`, not `utils_test`) | `text_test.go:1` |

The 8 cases cover: shorter than limit, exactly at limit, trimmed with ellipsis, `maxLen <= 3` edge, and four multi-byte cases (accented, accented at boundary, accented exact fit, CJK). The multi-byte cases are the reason the function converts to `[]rune` at `internal/utils/text.go:11`.

`internal/utils` sits at 36.4% and not higher because `internal/utils/links.go` (62 lines, 8 exported functions) has **zero** tests. The six `Build*URL` functions there are pure string formatters and are the single easiest coverage win in the entire repo.

### `internal/ui/releases/releases_test.go` (52 lines, 2 test functions + 1 helper)

Covers exactly one method: `getSelectedRelease` at `internal/ui/releases/releases.go` (called at `releases_test.go:29`, `:45`), plus incidentally `New` and `buildRows`.

Style: **regression-driven, not table-driven.** Each test documents a bug that was fixed.

```go
// newTestModel builds a Model backed by the given releases. The client is never
// dereferenced when building rows or resolving the selection, so nil is fine.
func newTestModel(releases []ado.Release) Model {
	m := New(nil, "https://dev.azure.com/org", nil, 0)
	m.releases = releases
	m.table = m.table.WithRows(m.buildRows())
	return m
}
```
-- `internal/ui/releases/releases_test.go:9-16`

This is the only test helper in the codebase. It is worth reading closely because it demonstrates both the one testing technique the repo has and the limit of that technique.

| Observation | Detail |
|-------------|--------|
| The seam is `nil` | `New(nil, ...)` passes a nil `*ado.Client`. This works only because `getSelectedRelease` and `buildRows` never dereference `m.client` |
| The comment states the precondition explicitly | `releases_test.go:9-10` -- it is documented, not accidental |
| Test is in the internal package | `package releases` at `:1`, so it can touch unexported fields `m.releases` and `m.table` |
| No mocking | Nothing is faked. The nil client is a hole, not a substitute |
| `t.Fatalf` for nil result, `t.Errorf` for wrong value | `:31` vs `:34` -- correct distinction, fatal when continuing is pointless |
| Loop over expected IDs rather than subtests | `:26` uses `for row, wantID := range []int{101, 102}` with no `t.Run` |
| Each test carries a `// Regression:` block explaining the original defect | `:18-19`, `:39-40` |

The two tests:

1. `TestGetSelectedReleaseDistinguishesSharedNamePrefix` (`:20`) -- two releases whose names share a prefix once truncated must still resolve to distinct IDs. Guards the hidden `columnKeyReleaseID` row metadata introduced at `internal/ui/releases/releases.go:33`.
2. `TestGetSelectedReleaseHandlesUnnamedRelease` (`:41`) -- a release with no name renders a styled `"-"` placeholder and must still be selectable.

13.1% is the ceiling this approach reaches. Everything in `internal/ui/releases/releases.go` that touches `m.client` -- `fetchReleases` (`:141`), `fetchApprovals` (`:150`), `approveRelease` (`:159`), `rejectRelease` (`:167`) -- is unreachable because a nil client would panic.

---

## Test Infrastructure: What Exists

**Nothing.** Every item below was checked and is absent.

| Infrastructure | Present | Verified by |
|----------------|---------|-------------|
| `testdata/` directory | No | `find` for `testdata` -- zero matches |
| Golden files (`*.golden`) | No | `find` -- zero matches |
| `fixtures/` directory | No | `find` -- zero matches |
| Mock files (`*_mock.go`, `mock_*.go`) | No | `find` -- zero matches |
| Fake files (`*_fake.go`) | No | `find` -- zero matches |
| `export_test.go` | No | `find` -- zero matches |
| `main_test.go` / `TestMain` | No | `find` -- zero matches |
| `helpers_test.go` | No | `find` -- zero matches |
| Build tags (`//go:build`) | No | `rg "^//go:build"` -- zero matches |
| `net/http/httptest` usage | No | `rg httptest` -- zero matches |
| Mock generator (gomock, mockery) | No | Absent from `go.mod` and `go.sum` |
| Assertion library (testify) | No (transitive only) | `go.sum:69-70`, not in `go.mod` require |
| Benchmarks (`func Benchmark*`) | No | Zero matches |
| Fuzz targets (`func Fuzz*`) | No | Zero matches |
| Example tests (`func Example*`) | No | Zero matches |

The total test surface is: 3 `Test` functions, 1 unexported helper, 82 lines, 2 packages.

---

## The `internal/ado` Testability Problem

This is the most important section of this document.

### There is no seam

`internal/ado.Client` is a concrete struct with entirely unexported fields holding live Azure DevOps SDK clients:

```go
type Client struct {
	connection         *azuredevops.Connection
	workitemClient     workitemtracking.Client
	workClient         work.Client
	gitClient          git.Client
	buildClient        build.Client
	releaseClient      release.Client
	...
}
```
-- `internal/ado/client.go:20-32`

`NewClient` is the only constructor and it unconditionally builds a real PAT connection and five real SDK clients (`internal/ado/client.go:34-98`). There is no alternate constructor, no field setter, and no exported way to substitute any of the six clients.

**Zero interfaces are declared anywhere in this repository.** Verified with a search for `^type \w+ interface` -- no matches in any of the 30 Go files. Consequently:

- No `*ado.Client` method can be exercised without a real PAT and network access to a real Azure DevOps organization.
- Every UI view stores `client *ado.Client` as a concrete pointer (`internal/ui/releases/releases.go:59`, `internal/ui/app.go:34`), so views cannot be given a substitute either. The `nil` trick at `releases_test.go:12` is the only available workaround and it only works for code paths that never touch the client.

### 7 methods perform irreversible writes against production

All at 0.0% coverage. All reachable from a single keypress in the TUI.

| Method | Location | SDK write call | Effect if wrong |
|--------|----------|----------------|-----------------|
| `UpdateWorkItemState` | `internal/ado/workitems.go:215` | `workitemClient.UpdateWorkItem` (`:228`) | Moves a real work item to a new state |
| `UpdateWorkItemStateWithEffort` | `internal/ado/workitems.go:244` | `workitemClient.UpdateWorkItem` (`:278`) | Overwrites `OriginalEstimate`, `RemainingWork`, `CompletedWork` **and** state in one patch document |
| `ApproveRelease` | `internal/ado/releases.go:272` | `releaseClient.UpdateReleaseApproval` (`:281`) | Approves a real release gate. Triggers a deployment |
| `RejectRelease` | `internal/ado/releases.go:297` | `releaseClient.UpdateReleaseApproval` (`:306`) | Rejects a real release gate |
| `QueuePipeline` | `internal/ado/pipelines.go:349` | `buildClient.QueueBuild` (`:364`) | Starts a real build against a chosen branch |
| `ApprovePullRequest` | `internal/ado/pullrequests.go:150` | `gitClient.CreatePullRequestReviewer` (`:172`) | Casts a real approval vote as the PAT owner |
| `CompletePullRequest` | `internal/ado/pullrequests.go:190` | `gitClient.UpdatePullRequest` (`:229`) | **Merges a real pull request** |

`DownloadAttachment` (`internal/ado/workitems.go:520`) additionally writes to the local filesystem at a caller-supplied `destPath`, also untested.

What 0.0% coverage on this layer means concretely:

- The JSON Patch document built in `UpdateWorkItemStateWithEffort` (`internal/ado/workitems.go:254-276`) has four operations whose field paths (`Microsoft.VSTS.Scheduling.OriginalEstimate`, `.RemainingWork`, `.CompletedWork`, `System.State`) are string literals. A typo compiles, passes `go vet`, and fails at runtime against production -- or worse, silently writes to the wrong field. Nothing catches it before a live call.
- The `Add` operation is used for all four fields with the comment `// Using Add operation which works for both new and existing values` (`internal/ado/workitems.go:255`). That assumption is untested.
- `ApproveRelease` and `RejectRelease` are byte-identical apart from the status constant (`internal/ado/releases.go:272-294` vs `:297-319`). A copy-paste error swapping approve for reject would not be caught.
- Argument marshalling everywhere in this package takes the address of a local (`&project`, `&approvalID`, `&defID`). Passing the wrong local is a compile-clean bug.

**The only current verification for any of this is a human running the TUI against a live Azure DevOps organization and observing the result.** There is no dry-run mode, no confirmation-only path, and no CI.

### What is testable in `internal/ado` today, without any refactor

Four package-level functions take no client and can be tested immediately:

| Function | Location | Lines | Why it matters |
|----------|----------|-------|----------------|
| `pipelineNameMatches` | `internal/ado/pipelines.go:57` | 46 | Three matching modes (exact, `prefix*`, `**contains**`) with subtle precedence. Determines which release definitions and pipelines the user sees at all. Highest-value untested pure function in the repo |
| `normalizePipelineName` | `internal/ado/pipelines.go:48` | 3 | Trim + lowercase |
| `calculateApprovalStatus` | `internal/ado/pullrequests.go:119` | ~30 | Reduces reviewer votes to a display status. Drives what the PR view shows |
| `GetNextState` | `internal/ado/workitems.go:510` | ~10 | Maps `(workItemType, currentState)` to the next state using the configured transition map. This is the input to the destructive `UpdateWorkItemState` call |

`GetNextState` deserves emphasis: it is the pure function that decides which state a destructive write will set. Testing it does not make the write safe, but it does make the *decision* verifiable.

---

## What Is Testable Without Refactoring, Repo-Wide

Ranked by value-to-effort. All are pure or near-pure and need no seam.

| Target | Location | Effort | Why |
|--------|----------|--------|-----|
| `pipelineNameMatches` | `internal/ado/pipelines.go:57` | Low | 3 matching modes, precedence rules, `len(pattern) > 4` guard. Table-driven, ~15 cases |
| `GetNextState` | `internal/ado/workitems.go:510` | Low | Feeds the destructive state write |
| 6 `Build*URL` functions | `internal/utils/links.go:13-41` | Low | Pure formatters. Two use `url.QueryEscape` (`:24`, `:30`) -- branch names containing `/` are the interesting case |
| `calculateApprovalStatus` | `internal/ado/pullrequests.go:119` | Low | Vote reduction logic |
| `Config.Validate` | `internal/config/config.go:144` | Low | Two branches, returns `*ConfigError` |
| `StringSlice.UnmarshalYAML` | `internal/config/config.go:15` | Low | Dual-mode YAML decode: sequence node vs comma-separated scalar. Exactly the kind of parser that breaks quietly |
| `Config.loadFromEnv` | `internal/config/config.go:122` | Low | Needs `t.Setenv`, no other seam |
| `stripHTML` | `internal/ui/workitems/workitems.go:755` | Low | HTML from ADO descriptions. Malformed input is the interesting case |
| `formatDuration` | `internal/ui/pipelines/pipelines.go:535` | Low | Pure formatter |
| `formatSize` | `internal/ui/workitems/detail.go:431` | Low | Pure formatter |
| `wrapText` | `internal/ui/workitems/detail.go:395` | Low | Width edge cases, multi-byte input |
| `formatEnvironments` / `getEnvStatusIndicator` | `internal/ui/releases/releases.go:531`, `:556` | Low | Pure string building from `[]ado.ReleaseEnvironment` |
| `findDevelopmentTask` | `internal/ui/workitems/workitems.go:881` | Low | Selects the child task whose effort gets written |
| `Truncate` | `internal/utils/text.go:6` | Done | Already covered |
| Other view `buildRows` | `prs.go:425`, `pipelines.go:432`, `workitems.go:564`, `branches.go:271` | Medium | Reachable via the same nil-client helper pattern as `releases_test.go:11`, provided the row builder does not touch `m.client`. Verify per view before relying on it |
| `Priorities` load/save/reorder | `internal/config/priorities.go` | Medium | Reads and writes `~/.config/adoBizInt/priorities.yaml`. Would need the home dir redirected or the path parameterized -- the path is hardcoded at `:31` and `:58` |

Not testable without a refactor: everything in `internal/ado` that has a `*Client` receiver (28 methods), all `Update`/`Init`/`View` paths that dispatch a `tea.Cmd` touching the client, `internal/logging` (writes to a hardcoded home-relative path, `logging.go:18`), and `cmd/adoBizInt/main.go` (calls `os.Exit`).

---

## Prioritized Test Backlog

Ordered by risk, highest first.

### P0 -- Destructive write path

The `internal/ado` write methods can merge PRs, approve releases, queue builds, and overwrite work item effort fields in production. They have zero automated verification.

1. **Introduce a seam for the ADO SDK clients.** The minimum viable change is a narrow unexported interface per SDK client used, holding only the methods actually called, with `Client` holding the interface instead of the concrete SDK type. Six SDK clients are held (`internal/ado/client.go:21-26`) but the write surface only needs three: `workitemtracking`, `release`, `git`, plus `build` for `QueueBuild`. Note this deliberately conflicts with the "no speculative generality / no interfaces" convention in `CONVENTIONS.md` -- it is justified *because the current task is to make destructive writes testable*, which is exactly the exception the standard allows. It must not be generalized beyond the methods needed.
2. **Test the JSON Patch document construction** for `UpdateWorkItemState` (`workitems.go:219-226`) and `UpdateWorkItemStateWithEffort` (`workitems.go:254-276`). Assert the exact field paths and operation types. This is the highest-value test in the repo -- it catches typos in string-literal ADO field paths before they hit production.
3. **Test that `ApproveRelease` sends `Approved` and `RejectRelease` sends `Rejected`.** These functions are near-identical (`releases.go:275` vs `:300`); a swap is a one-character bug with a deployment-triggering consequence.
4. **Test `CompletePullRequest` arguments** (`pullrequests.go:190-235`). This merges code.

### P1 -- Filtering and selection logic that silently hides data

5. `pipelineNameMatches` (`pipelines.go:57`). If this is wrong the user sees the wrong pipelines or none, and has no way to tell. Test all three modes and their precedence.
6. `GetNextState` (`workitems.go:510`). Decides the target state of a destructive write.
7. `calculateApprovalStatus` (`pullrequests.go:119`). Drives the PR view's approval column.
8. `findDevelopmentTask` (`workitems.go:881`). Selects which child task receives the effort write.

### P2 -- Config parsing

9. `StringSlice.UnmarshalYAML` (`config.go:15`). Dual-mode decode; a silent failure means the user's project list is empty and the app shows nothing.
10. `Config.Validate` (`config.go:144`) and `loadFromEnv` (`config.go:122`). Cheap, and `loadFromEnv` has a non-obvious clear-then-append behaviour at `:131`.
11. Env-over-file precedence and the backwards-compat `Project` -> `Projects` fallback (`config.go:95-97`).

### P3 -- Pure formatters and remaining URL builders

12. The 6 `Build*URL` functions in `internal/utils/links.go:13-41`, with emphasis on `BuildBranchURL` (`:24`) and `BuildPRCreateURL` (`:30`) where branch names containing `/` must be escaped.
13. `stripHTML`, `wrapText`, `formatSize`, `formatDuration`, `formatEnvironments`, `getEnvStatusIndicator`.

### P4 -- View row building

14. Extend the `newTestModel` nil-client pattern from `releases_test.go:11` to the other four views. Confirm per view that `buildRows` does not dereference `m.client` before relying on it.

### P5 -- Infrastructure

15. Add CI that runs `go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test ./... -race`. All four currently pass, so CI would start green and immediately protect that state.
16. Consider a coverage floor only after P0-P2 land. Setting one now would be met by testing formatters instead of the write path.

---

## Conventions for New Tests

Derived from the two existing files. Follow these.

| Rule | Source |
|------|--------|
| Internal test package (`package foo`, not `foo_test`) so unexported state is reachable | `text_test.go:1`, `releases_test.go:1` |
| Table-driven with `t.Run` subtests for pure functions | `text_test.go:6-29` |
| Anonymous struct slice named `tests`, loop variable `tt` | `text_test.go:6`, `:23` |
| `got` / `want` variable naming | `text_test.go:25` |
| Failure message restates the full call and both values, `%q` for strings | `text_test.go:26` |
| `t.Fatalf` when continuing is pointless (nil result), `t.Errorf` otherwise | `releases_test.go:31` vs `:34` |
| Regression cases carry a `// Regression:` comment describing the original defect | `text_test.go:16`, `releases_test.go:18`, `:39` |
| Test helpers are unexported, defined at the top of the test file, and document their preconditions | `releases_test.go:9-16` |
| Test file sits beside the code it tests, named `<file>_test.go` | Both files |
| No third-party assertion library | Both files |

Test file naming should follow the source file, not the package: a test for `internal/ado/pipelines.go` goes in `internal/ado/pipelines_test.go`. Note `releases_test.go` currently covers `releases.go` only, not `approval_modal.go`.

---

*Testing analysis: 2026-09-03*
