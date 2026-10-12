# Phase 6: Core-value cleanup - Pattern Map

**Mapped:** 2026-10-12
**Files analyzed:** 24 (21 source or test files, 1 planning doc, 2 optional comment-only fixes)
**Analogs found:** 24 / 24. Every file is a modification of existing code, and each change has an in-repo analog, usually in the same file.

Every analog path below is git-tracked (`git ls-files cmd internal .planning/BACKLOG.md`). None is a `.gsd/` mirror.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/adoBizInt/usage.go` (W2 probe, D-06 scope help) | controller (CLI pre-config router) | request-response | self: `handleUsage` 143-189; `runWrite` parse 139-147 (`write.go`); `verbSummaries` 46-62 | exact |
| `cmd/adoBizInt/usage_test.go` (W2 rows, D-06 help test) | test | request-response | `TestHelpNeedsNoCredentials` 51-87, `TestHandleUsagePassesValidSubcommandsThrough` 163-170 | exact |
| `cmd/adoBizInt/writeverbs_test.go` (D-08 `--comment -h` row) | test | request-response | `TestReleaseCommentReachesTheMutation` 136-168 | exact (extend table) |
| `internal/config/config.go` (W3) | config/validator | transform | self: `Validate` 171-184 | exact |
| `internal/config/config_test.go` (W3) | test | transform | `TestValidate` 96-162, `writeConfig` 18-32 | exact |
| `cmd/adoBizInt/main.go` (W3 hint, D-14 comments) | entrypoint | request-response | self: lines 26-28, 62-64, 91-97 | exact |
| `cmd/adoBizInt/main_test.go` (W3 fixture fix, structural test for Validate ordering, W4 human-line test) | test | request-response | `TestTimeoutOverridesConfiguredBudgetAfterValidation` 734-770, `TestRenderFailuresToStderrOnly` 596-618, `TestRenderNotInvokedUnderJSON` 662-679; `parsePackageFiles` (`internal/ado/nilguard_test.go:139-159`) for the AST walk | exact / role-match (AST) |
| `internal/ado/result.go` (W4 `Scope` type and field) | model (envelope) | transform | self: `Result[T]` 90-102, `newResult` 111-123 | exact |
| `internal/ado/workitems.go` (W4) | service (read) | request-response, fan-out | self: `GetWorkItems` 92-138 (two returns: 104, 137) | exact |
| `internal/ado/dashboard.go` (W4) | service (read) | request-response, fan-out | self: `GetDashboardCounts` 43-103 (two returns: 60, 102) | exact |
| `internal/ado/pullrequests.go` (W4) | service (read) | request-response, fan-out | self: `GetPullRequests` 44-83 (one return: 82) | exact |
| `internal/ado/releases.go` (W4, D-15) | service (read) | request-response, fan-out | self: `GetPendingApprovals` 238-315 (one return: 314) | exact |
| `internal/ado/json_test.go` (W4 contract) | test | transform | `TestJSONFieldNames` 155-167, `TestNewResultEmitsEmptyArraysNotNull` 423-454 | exact |
| `internal/ado/wiql_test.go` (W4 scope-to-WIQL tie) | test | request-response | `TestGetWorkItemsUsesProjectMacro` 27-74, `TestDashboardUsesProjectMacro` 105-135 | exact |
| `internal/ado/pullrequests_test.go` (W4 PR scope) | test | request-response | `TestGetPullRequestsReportsCompleteWhenEveryProjectAnswers` 340-361 | exact |
| `internal/ado/releases_test.go` (W4 approvals scope) | test | request-response | `TestGetPendingApprovalsHappyPathRowCountUnchanged` 259-272 | exact |
| `cmd/adoBizInt/render.go` (W4 `renderScope`) | utility (renderer) | transform, stderr | `renderFailures` 84-103 | exact |
| `cmd/adoBizInt/dispatch.go` (W4 call site, D-17 stray positional) | controller | request-response | self: `runRead` 243-251 and 323-330; `runWrite` arity check `write.go:163-166` | exact |
| `cmd/adoBizInt/flags.go` (D-05 `--hide-done` text) | config (flag registrar) | n/a | self: 227-230 | exact |
| `cmd/adoBizInt/filter_test.go` or `usage_test.go` (D-05 help text test) | test | n/a | `TestFilterFlagsRegisteredOnSupportedVerbsOnly` 397-446 (`newFlagSet` + `fs.Lookup`) | role-match |
| `cmd/adoBizInt/write.go` (W1 recover before mutate) | controller (write path) | request-response, audited | self: `runWrite` 137-277; `guard` (`main.go:36-43`) for the recover shape | exact |
| `cmd/adoBizInt/crash.go` (W1 `handlePanic(r, code)`, D-14) | utility (crash handler) | event (panic) | self: `handlePanic` 75-99 | exact |
| `cmd/adoBizInt/crash_test.go` / `write_test.go` (W1 tests + structural defer test) | test | event (panic), file-I/O (audit) | `TestPanicEmitsDistinctErrorObject` (`crash_test.go:198-230`), `TestOutcomeUnknownWriteIsAudited` (`audit_test.go:201-221`), `TestAuditAppendFailureReportsUnknown` (`audit_test.go:256-295`), `TestInterruptedWriteReportsOutcomeUnknownNotNothingRan` (`write_test.go:336-373`) | exact |
| `.planning/BACKLOG.md` (D-08: drop WR-05) | doc | n/a | line 81 | exact |
| (optional) `internal/ado/client.go` 167/209/231, `workitems.go:147`, `dashboard.go:120` ("90 days" to "30 days" comments) | comment fix | n/a | n/a | n/a |

---

## Pattern Assignments

### `cmd/adoBizInt/usage.go` (controller, request-response) — W2 and D-06

**Analog:** self, plus `runWrite`'s parse block in `cmd/adoBizInt/write.go`.

**Current imports** (lines 3-8). The probe needs `errors` and `io` added:
```go
import (
	"flag"
	"fmt"
	"sort"
	"strings"
)
```

**Block being replaced** (lines 178-188):
```go
	// A valid subcommand: verb-level help is handled here rather than by the
	// FlagSet, so it too runs before config is loaded.
	for _, arg := range args[2:] {
		if helpTokens[arg] {
			fs, _, _, _ := newFlagSet(noun + " " + verb)
			usageVerb(noun+" "+verb, fs)
			return exitOK, true
		}
	}

	return exitOK, false
```
Keep `helpTokens` (11-16). The `args[0]` check (148) and `args[1]` check (168) still use it for `adoBizInt help` and `adoBizInt workitems help` (D-07).

**Parse + ErrHelp idiom to copy** (`cmd/adoBizInt/write.go:139-147`). This is the repo's existing shape for "stdlib reports help":
```go
	fs, gf, _, wf := newFlagSet(name)
	if err := fs.Parse(hoistLeadingPositionals(args)); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUnknown
	}
```
`hoistLeadingPositionals` is at `write.go:597-612`. Reuse it as is.

**Why the probe must wrap `Set`** (`cmd/adoBizInt/flags.go:198-215`). The `--project` closure rejects every value while `configuredProjects` is nil, which is the state before config loads:
```go
	fs.Func("project",
		"restrict the read to a project; repeatable or comma-separated",
		func(value string) error {
			...
				if len(configuredProjects) == 0 {
					return fmt.Errorf("not a configured project: %q (no projects are configured)", part)
				}
```
`newFlagSet` also calls `fs.SetOutput(stderrWriter)` (line 186). The probe must override that with `probe.SetOutput(io.Discard)` and `probe.Usage = func() {}`, then print help from a fresh `newFlagSet(name)` (RESEARCH Pattern 1, Pitfall 3). The `helpProbeValue` shape from RESEARCH Pattern 1 (lines 243-266) is the only new type. It is single-use, so it stays private in usage.go.

**D-06 scope paragraph.** Copy the `verbSummaries` map shape (lines 46-62) for a `verbScopes` map with three entries: `workitems list`, `dashboard counts`, `prs list`, plus `releases approvals` per D-15. Print it in `usageVerb` next to the summary (lines 120-123):
```go
	fmt.Fprintf(stderrWriter, "Usage: adoBizInt %s [flags]\n", name)
	if summary := verbSummaries[name]; summary != "" {
		fmt.Fprintf(stderrWriter, "\n%s\n", summary)
	}
```
The wording must say **30 days**, not 90, and must state that a project with no dated recent iteration is not limited by iteration (RESEARCH Pitfall 7; `internal/ado/client.go:190`, `:233-239`).

---

### `cmd/adoBizInt/usage_test.go` (test) — W2, D-06

**Analog:** `TestHelpNeedsNoCredentials` (lines 51-87). It has the table, `captureOutput`, the `handled`/`code` checks and the stdout-empty check:
```go
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			code, handled := handleUsage(tt.args)
			if !handled {
				t.Fatalf("handleUsage(%q) reported unhandled, want handled", tt.args)
			}
			if code != exitOK {
				t.Errorf("handleUsage(%q) = %d, want %d", tt.args, code, exitOK)
			}
			if !strings.Contains(stderr.String(), tt.want) { ... }
			if stdout.Len() != 0 { ... OUT-03 ... }
		})
	}
```
**Negative-direction analog:** `TestHandleUsagePassesValidSubcommandsThrough` (163-170), which asserts `handled == false`.

**Pre-config state:** call `withConfiguredProjects(t)` with **no names** (`main_test.go:704-709`) so the `--project P --help` row models an empty allow-list (Pitfall 1).

**Regression guards that must stay green unchanged:** `TestHelpIsDeterministic` (91-106) and `TestBadFlagExitsThreeWithCleanStdout` (143-159). The second one depends on `runRead`'s FlagSet still printing usage on `--nonsense`, so the probe must return "not handled" on non-help parse errors.

---

### `cmd/adoBizInt/writeverbs_test.go` (test) — D-08

**Analog:** extend the table in `TestReleaseCommentReachesTheMutation` (lines 136-168):
```go
		{"supplied", []string{"releases", "approve", "77", "--project", "P", "--comment", "ship it", "--confirm", "--json"}, "ship it"},
		{"omitted", []string{"releases", "approve", "77", "--project", "P", "--confirm", "--json"}, ""},
```
Add a row with `"--comment", "-h"` and want `"-h"`. The test drives `dispatch` directly. For end-to-end D-08 coverage, also assert `handleUsage(tc.args)` returns `handled == false` for that row.

---

### `internal/config/config.go` (config validator) — W3

**Analog:** self, `Validate` (lines 171-184). Insert two checks after the `org_url` check (line 177) in the same `return &ConfigError{...}` shape:
```go
func (c *Config) Validate() error {
	if c.PAT == "" {
		return &ConfigError{Field: "pat", Message: "Personal Access Token is required"}
	}
	if c.OrgURL == "" {
		return &ConfigError{Field: "org_url", Message: "Organization URL is required"}
	}
	// <- len(c.Projects)==0 and strings.TrimSpace(p)=="" checks, Field: "projects"
```
`strings` is already imported (line 8). The env path already trims (lines 156-164: `if p = strings.TrimSpace(p); p != ""`). The YAML sequence path and the legacy `project:` path do not, which is why D-11 needs the blank check.

---

### `internal/config/config_test.go` (test) — W3

**Analog:** `TestValidate` (lines 96-162). Fix the `base()` fixture in the same commit as the rule (Pitfall 5):
```go
	base := func() *Config {
		return &Config{
			PAT:          "token",
			OrgURL:       "https://dev.azure.com/example",
			PollInterval: 30 * time.Second,
			FetchTimeout: 2 * time.Minute,
			// + Projects: StringSlice{"P"},
		}
	}
```
Add rows in the existing `{name, mutate, wantField}` shape (lines 111-137), for example `{name: "nil projects", mutate: func(c *Config) { c.Projects = nil }, wantField: "projects"}`, plus rows for an empty slice, `""` and `"  "`.

**For a Load-level blank-entry test** (`projects: [""]` through YAML), copy `writeConfig` (18-32) and the `Load()` call shape of `TestLoadParsesTimeoutDurationsFromYAML` (81-94). Also set `t.Setenv("ADO_PROJECTS", "")` so the env path cannot mask the YAML value.

---

### `cmd/adoBizInt/main.go` (entrypoint) — W3 hint, D-14 comments

**Analog:** self.
- Lines 91-97: the validation failure branch. Update the hint at line 94 (`"Please set ADO_PAT and ADO_ORG_URL environment variables"`) so it also mentions `ADO_PROJECTS` / `projects:`. Keep `return exitUnknown`.
- Lines 26-28 (`guard` doc): "...a structured crash record and the outcome-unknown exit code". Rewrite to say exit 3, nothing ran (D-14).
- Lines 62-64 (`run` doc): "its outcome is unknown rather than incomplete". Rewrite to say "nothing ran" (D-14).
- `guard` body (36-43) changes only its call: `code = handlePanic(r)` becomes `handlePanic(r, exitUnknown)`.

**Structural fact for the W3 TUI test:** `cfg.Validate()` at line 91 precedes `dispatch(...)` at line 135 and `ui.NewModel(...)` at line 138 in `run`'s body.

---

### `cmd/adoBizInt/main_test.go` (test) — W3 fixture, W3 structural test, W4 human line

**W3 fixture fix** (lines 735-740). Add `Projects: config.StringSlice{"P"}`:
```go
	cfg := &config.Config{
		PAT:          "fixture-pat",
		OrgURL:       "https://dev.azure.com/fixture",
		PollInterval: 30 * time.Second,
		FetchTimeout: 2 * time.Minute,
	}
```

**Structural test (Validate before CLI and TUI branches).** No AST helper exists in `cmd/`. The analog is `parsePackageFiles` (`internal/ado/nilguard_test.go:139-159`). Copy only its `parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)` call, inline and single-file (`main.go`), and do not extract a helper:
```go
	fset := token.NewFileSet()
	...
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
```
Walk `run`'s `FuncDecl.Body.List` and compare the statement index of the `cfg.Validate()` call with those containing `dispatch(` and `ui.NewModel(`. The same inline parse also serves the W1 defer-position test. That makes two uses, so leave a `// note:` comment rather than extracting (rule of three; precedent `wiql_test.go:28`).

**W4 human scope line test.** Analog: `TestRenderFailuresToStderrOnly` (596-618) for "on stderr, not stdout", and `TestRenderNotInvokedUnderJSON` (662-679) for "nothing on stderr under `--json`":
```go
	if bytes.Count(stdout.Bytes(), []byte("\n")) != 1 { ... exactly one JSON document }
	if stderr.Len() != 0 {
		t.Errorf("--json wrote %q to stderr, want nothing", stderr.String())
	}
```
The fake must return a Result with `Scope` set. Copy the closure shape of `deadlineCapturingClient` (714-727), returning `emptyResult[T]()` (`fakes_test.go:134-136`) with `.Scope` assigned. Zero rows is the case to pin (RESEARCH Pattern 3: the scope line prints even when there are no rows).

**Guard to preserve:** `TestRenderEmptyWritesNothing` (642-658) asserts **stderr is empty** for all 8 verbs through `allReadsFake`. It stays green only because `emptyResult` leaves `Scope` nil. Do not change `emptyResult` or `allReadsFake` to carry a scope.

---

### `internal/ado/result.go` (model) — W4

**Analog:** self, `Result[T]` (lines 90-102):
```go
// Result is the envelope every aggregate read returns. ...
type Result[T any] struct {
	Items    []T       `json:"items"`
	Failures []Failure `json:"failures"`
	Complete bool      `json:"complete"`
}
```
Add `Scope *Scope \`json:"scope"\``. Use **no `omitempty`**, because D-02 requires `null`. Declare `Scope` just above it, using the snake_case tag convention from the comment at lines 94-97. `newResult` (111-123) is unchanged. Scoped reads assign `r.Scope` after calling it.

---

### `internal/ado/workitems.go`, `dashboard.go`, `pullrequests.go`, `releases.go` (service reads) — W4, D-15

**Analog:** self. Each read has a `return newResult(...), nil` that must carry the scope. **Every return path counts** (Pitfall 6):

| File | Return lines | Scope |
|------|-------------|-------|
| `workitems.go` `GetWorkItems` | **104** (early, no projects) and **137** | excluded states + iterations |
| `dashboard.go` `GetDashboardCounts` | **60** (early) and **102** | excluded states + iterations + PR status |
| `pullrequests.go` `GetPullRequests` | **82** | PR status |
| `releases.go` `GetPendingApprovals` | **314** | approval status |

Early-return shape (`workitems.go:102-105`):
```go
	if len(targetProjects) == 0 {
		log.Warn("No projects to fetch work items for")
		return newResult([]WorkItem{}, nil), nil
	}
```

**Derive the status strings from the SDK constant the request already uses**, so the two cannot drift:
- `pullrequests.go:63-68` and `dashboard.go:155-160`: `Status: &git.PullRequestStatusValues.Active`. Use `string(git.PullRequestStatusValues.Active)`.
- `releases.go:257`: `statusFilter := release.ApprovalStatusValues.Pending`. Use `string(release.ApprovalStatusValues.Pending)`.

**Keep the WIQL literal untouched** (`workitems.go:163`, `dashboard.go:136`):
```go
... AND [System.State] NOT IN ('Done', 'Closed', 'Removed')" + iterationClause(iterations) + ...
```
`TestWIQLInterpolationStructural` (`wiql_test.go:195-207`) fails if the `NOT IN` list is built from a variable. Tie scope to query with a test instead (next section).

**Shared scope value:** the work-item and dashboard scopes share excluded states and iterations, which makes two uses. Per the rule of three, either write the literal twice with a `// note:` comment, or use one package-level value if the planner judges that clearer. Do not build a constructor or helper.

**Optional honesty fix:** the "90 days" comments at `workitems.go:147`, `dashboard.go:120` and `client.go:167,209,231` are wrong. The code uses 30 (`client.go:190`).

---

### `internal/ado/json_test.go` (test) — W4 contract

**Analog:** `TestJSONFieldNames` (lines 155-167). Every `Result[...]` key set gains `"scope"`:
```go
		assertKeySet(t, Result[WorkItem]{}, []string{"items", "failures", "complete"})
		...
		assertKeySet(t, Result[DashboardCounts]{}, []string{"items", "failures", "complete"})
```
**Null assertion analog:** `TestNewResultEmitsEmptyArraysNotNull` (423-454). It marshals `newResult[PullRequest](nil, nil)` and decodes into a `json.RawMessage` struct:
```go
	var envelope struct {
		Items    json.RawMessage `json:"items"`
		Failures json.RawMessage `json:"failures"`
	}
	...
	if string(envelope.Items) != "[]" { ... }
```
Add a `Scope json.RawMessage \`json:"scope"\`` field and assert `string(envelope.Scope) == "null"`. Use a struct, not a map, so a missing key also fails: an absent key decodes to a nil RawMessage, which is not `"null"`.

---

### `internal/ado/wiql_test.go` (test) — W4 scope-to-WIQL tie

**Analog:** the capture fakes in `TestGetWorkItemsUsesProjectMacro` (27-74) and `TestDashboardUsesProjectMacro` (105-135):
```go
	var gotQuery, gotProject string
	api := &fakeWorkItemAPI{
		t: t,
		GetClassificationNodeFunc: func(...) (*workitemtracking.WorkItemClassificationNode, error) {
			return nil, nil
		},
		QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			...
			gotQuery, gotProject = *args.Wiql.Query, *args.Project
			return &workitemtracking.WorkItemQueryResult{}, nil
		},
	}
	client := &Client{projects: []string{quoteProject}, workitemClient: api}
```
Assert `strings.Contains(gotQuery, "NOT IN ('"+strings.Join(result.Scope.ExcludedStates, "', '")+"')")`. The dashboard variant also needs the `gitClient: &fakeGitAPI{... GetPullRequestsByProjectFunc ...}` from lines 129-134. Those tests carry a slog capture that already has a "duplicated, extract on a third use" note (line 28). A new scope test does not need log capture, so do not copy it. Copying it would be the third use.

**Early-return path:** `&Client{projects: nil}` with no fakes set reaches `workitems.go:104` / `dashboard.go:60` without any API call. Assert `Scope != nil` there.

---

### `internal/ado/pullrequests_test.go`, `releases_test.go` (test) — W4, D-15

**Analog (PRs):** `TestGetPullRequestsReportsCompleteWhenEveryProjectAnswers` (340-361):
```go
	client := &Client{
		projects: []string{"project-one", "project-two"},
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, _ git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				return &[]git.GitPullRequest{}, nil
			},
		},
	}
	result, err := client.GetPullRequests(context.Background(), nil)
	...
	if !result.Complete { ... }
```
Capture `args.SearchCriteria.Status` in the closure and assert it equals `result.Scope.PRStatus`. That ties the scope to the request. Also assert `Complete` stays true (Pitfall 8).

**Analog (approvals):** `TestGetPendingApprovalsHappyPathRowCountUnchanged` (`releases_test.go:259-272`), which uses `releaseTestClient(t, projects, func(string) error { return nil })`. For a tie test, copy the inline `GetApprovalsFunc` closure shape at `releases_test.go:170` and capture `args.StatusFilter`.

**Null scope for the other four reads (D-16):** one assertion per read (`GetBranches`, `GetPipelineRuns`, `GetPipelineDefinitions`, `GetReleases`) that `result.Scope == nil`, using the existing happy-path fixtures in their test files (e.g. `TestGetReleasesHappyPathRowCountUnchanged`, `releases_test.go:244`).

---

### `cmd/adoBizInt/render.go` (renderer) — W4, D-04

**Analog:** `renderFailures` (lines 84-103). It has the same stderr-only, nothing-when-empty, single-`WriteString` discipline:
```go
func renderFailures(w io.Writer, failures []ado.Failure) {
	if len(failures) == 0 {
		return
	}
	var b strings.Builder
	for _, f := range failures {
		fmt.Fprintf(&b, "warning: %s: %s failed (%s)\n", f.Project, f.Operation, f.Kind)
	}
	_, _ = io.WriteString(w, b.String())
}
```
`renderScope(w io.Writer, s *ado.Scope)` returns early on `nil` and issues one `io.WriteString`. Imports (`fmt`, `io`, `strings`, `ado`, lines 3-10) already cover it. The scope values are constants, never server text, so `sanitize` is not needed.

---

### `cmd/adoBizInt/dispatch.go` (controller) — W4 call site, D-17

**W4 call site:** the human branch of `runRead` (lines 323-330):
```go
	} else {
		// Human mode: rows to stdout, failures to stderr (D-13). ...
		render(stdoutWriter, result.Items)
		renderFailures(stderrWriter, result.Failures)
	}
```
Add `renderScope(stderrWriter, result.Scope)` in this branch only. The `--json` branch (317-322) emits `result` unchanged, and `scope` rides in the envelope. Phase 3 D-11 forbids a wrapper in `emitJSON` (345-357).

**D-17 stray positional:** right after the parse block in `runRead` (lines 243-251), before `ff.identity = identity`. Analog: the arity check in `runWrite` (`write.go:163-166`):
```go
	if len(fs.Args()) != 1 {
		fmt.Fprintf(stderrWriter, "%s requires exactly one id as a positional argument\n", name)
		return exitUnknown
	}
```
For reads the condition is `len(fs.Args()) > 0`. Write to stderr, return `exitUnknown`, put nothing on stdout. The test analog is `TestDispatchUsageErrors` (`main_test.go:163-192`), whose nil-closure `&fakeReadClient{t: t}` proves no read ran. A row like `{"workitems", "list", "help", "--json"}` must exit 3 with stdout empty.

---

### `cmd/adoBizInt/flags.go` (flag registrar) — D-05

**Analog:** self, lines 227-230:
```go
	if sup.hideDone {
		fs.BoolVar(&ff.hideDone, "hide-done", false,
			"drop items in a done or resolved state (off by default)")
	}
```
Change only the usage string. `doneStates` (`internal/ado/filters.go:9-14`) is Done/Closed/Resolved/Removed. The server already drops three of them, so the flag's only effective addition is Resolved.

**Test analog:** `TestFilterFlagsRegisteredOnSupportedVerbsOnly` (`filter_test.go:397-446`) uses `fs, _, _, _ := newFlagSet(name)` and `fs.Lookup(flagName)`. Assert on `fs.Lookup("hide-done").Usage` containing `Resolved`, `Done`, `Closed` and `Removed`.

---

### `cmd/adoBizInt/write.go` (write controller) — W1, D-12, D-13

**Analog:** self, `runWrite` (137-277). The recover shape comes from `guard` (`main.go:36-43`):
```go
func guard(body func() int) (code int) {
	defer func() {
		if r := recover(); r != nil {
			code = handlePanic(r)
		}
	}()
	return body()
}
```
**Insertion points in `runWrite`:**
- Line 137: `func runWrite(ctx context.Context, op writeOp, args []string) int` gets a **named return** `(code int)`. Line 237 `outcome, code := writeExitCodeFor(...)` must then become `=` (declare `outcome` separately) or it shadows the named return.
- Line 202: `defer sink.Close()`. The recover defer must be registered **after** this line, because defers run LIFO and the sink must still be open when the recover runs (Pitfall 9).
- Line 230: `err = op.mutate(writeCtx, target, wf)`. The recover defer goes **immediately before** this statement. Its position is the "request issued" signal (D-12), and the structural test pins it.
- Lines 244-248: the normal audit append. Set the `audited` local where this append is attempted, so a later panic (e.g. inside `finishWrite`'s `emitJSON`) does not append a second record (Pitfall 10):
```go
	if outcome == outcomeExecuted || outcome == outcomeUnknown {
		record := newAuditRecord(writeRecord{
			Command: name, Target: target, Changes: changes, Outcome: outcome,
		})
		if auditErr := appendAuditRecord(sink, record); auditErr != nil {
			...
			fmt.Fprintf(stderrWriter, "Write may have landed but was NOT recorded in %s: %v\n", auditPath, auditErr)
```
The best-effort append inside the recover reuses exactly this `newAuditRecord` + `appendAuditRecord` + stderr message, with `Outcome: outcomeUnknown`. Do not add a new writer. `write.go` is 612 lines, so keep the addition to about 20 lines.

---

### `cmd/adoBizInt/crash.go` (crash handler) — W1, D-14, D-18

**Analog:** self, lines 75-99:
```go
// handlePanic turns a recovered value into the published crash record and the
// outcome-unknown exit code.
...
func handlePanic(r any) int {
	message := fmt.Sprint(r)
	if loggingReady {
		logging.Logger().Error("panic recovered", "message", message, "stack", string(debug.Stack()))
	}
	writeCrashRecord(crashKindPanic, message)
	return exitUnknown
}
```
Change the signature to `handlePanic(r any, code int) int` and `return code`. Fix the doc comment at 75-76 (D-14). `writeCrashRecord` (55-68) stays on **stdout** (D-18). Do not touch `crashRecord` / `crashEnvelope` (28-42); their shape is a one-way contract. If the planner wants a message prefix for the post-request case, the precedent is the interrupt message at `dispatch.go:297-298` (`"read interrupted before any output was written: "+...`).

---

### `cmd/adoBizInt/crash_test.go` / `write_test.go` (tests) — W1

**Analogs and helpers, all existing and to be reused:**
- `tempAuditSink(t)` (`write_test.go:31-38`), `preReadFake(t, "Active")` (`write_test.go:43-58`), `setStateArgs("--confirm")` (`write_test.go:61-65`)
- `assertAuditLineCount(t, path, n)` / `decodeAuditRecord(t, line)` (`audit_test.go:18-54`)
- `failingAuditSink(t)` (`audit_test.go:230-244`) for the "append itself fails, still exit 5" case
- `decodeCrash` / `keysOf` (`crash_test.go:44-51`, `152-159`) for "stdout is exactly `{error}`"

**Mutate-panic test shape.** Combine `TestOutcomeUnknownWriteIsAudited` (`audit_test.go:201-221`):
```go
	withConfiguredProjects(t, "P")
	sink := tempAuditSink(t)
	_, stderr := captureOutput(t)

	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error {
		return context.DeadlineExceeded
	}

	code := dispatch(context.Background(), c, setStateArgs("--confirm"))
	...
	lines := assertAuditLineCount(t, sink, 1)
	got := decodeAuditRecord(t, lines[0])
	if got["outcome"] != outcomeUnknown { ... }
```
with `TestPanicExitsThree` / `TestPanicEmitsDistinctErrorObject` (`crash_test.go:184-230`). The test wraps the call in `guard(func() int { return dispatch(...) })`, then asserts `top-level keys == [error]` and `env.Error.Kind == "panic"`. The closure body becomes `panic("deliberate panic mid-write")`.

**Pre-mutate panic (must stay 3, zero audit lines):** panic inside `GetWorkItemDetailFunc` (the pre-read closure in `preReadFake`). Assert `exitUnknown` and `assertAuditLineCount(t, sink, 0)`.

**Read-panic regressions that must stay unchanged:** `TestPanicExitsThree` (184-194), `TestPanicWithFailedLoggingWritesNoStackToStderr` (268+), `TestPATNeverLogged` (`patleak_test.go:37`).

**Structural defer-position test:** use an inline `parser.ParseFile` on `write.go` (analog `parsePackageFiles`, `internal/ado/nilguard_test.go:152`). In `runWrite`'s `Body.List`, assert that the `*ast.DeferStmt` whose `FuncLit` calls `recover()` sits at index `i-1`, where statement `i` contains the `op.mutate` selector call, and that it comes after the `defer sink.Close()` statement.

---

### `.planning/BACKLOG.md` (doc) — D-08

Line 81 (`- **WR-05:** a flag value of \`-h\` ...`) under `## BL-02` (line 73). Remove it, or strike it through with a closing note in the same style as line 88 (`~~...~~ — **closed 2026-10-12** by ...`). D-16 also adds a BL-02 follow-up for the empty-`pipelines:` complete-over-nothing case. Put it in the "Follow-ups" list (lines 86-94).

---

## Shared Patterns

### Exit-code vocabulary
**Source:** `cmd/adoBizInt/dispatch.go:21-29`
**Apply to:** usage.go, dispatch.go (D-17), main.go, write.go, crash.go
```go
	exitOK      = 0
	exitPartial = 1
	exitAuth    = 2
	exitUnknown = 3 // the read never ran: usage error, config error, client error
	exitNoOp           = 4
	exitOutcomeUnknown = 5 // the write was issued and its outcome could not be established
```
Use the constants. Never use bare integers. The integers are published.

### stdout/stderr split and single-write discipline
**Source:** `emitJSON` (`dispatch.go:345-357`), `renderFailures` (`render.go:94-103`), `writeCrashRecord` (`crash.go:55-68`)
**Apply to:** renderScope, the D-17 diagnostic, the W1 audit-failure message
Diagnostics go to `stderrWriter` and parseable output to `stdoutWriter`. Build the output, then make one `Write`. The crash record stays on stdout (D-18).

### Test output capture and global swaps
**Source:** `captureOutput` (`main_test.go:55-68`), `withConfiguredProjects` (`main_test.go:704-709`), `withReadBudget` (`main_test.go:694-699`), `tempAuditSink` (`write_test.go:31-38`), `failingAuditSink` (`audit_test.go:230-244`)
**Apply to:** every new cmd test
Every global swap restores through `t.Cleanup`. Do not add new swap helpers. These five cover W1, W2, W4 and D-17.

### Nil-closure fakes as "no call happened" assertions
**Source:** `fakes_test.go:138-150` (fakeWriteClient doc), `&fakeReadClient{t: t}` in `TestDispatchUsageErrors` (`main_test.go:179`)
**Apply to:** the D-17 test, the W2 "help still handled" tests that go through dispatch, and the W1 pre-mutate panic test (leave `UpdateWorkItemStateFunc` nil)

### Structural (go/ast) tests over grep
**Source:** `parsePackageFiles` (`internal/ado/nilguard_test.go:136-159`), `TestWIQLInterpolationStructural` (`internal/ado/wiql_test.go:185-207`, including the rationale comment at 190-194)
**Apply to:** the W1 defer-position test (`write.go`) and the W3 Validate-ordering test (`main.go`)
`cmd/` has no helper. Two inline `parser.ParseFile` uses get a `// note:` comment (rule of three, precedent `wiql_test.go:28-30`), not an extracted helper.

### ConfigError shape
**Source:** `internal/config/config.go:186-193`
**Apply to:** both W3 checks. `&ConfigError{Field: "projects", Message: ...}`. `run()` prints it as `Invalid config: projects: ...` (`main.go:93`).

### Reads keep `Complete` derived only from failures
**Source:** `newResult` (`internal/ado/result.go:111-123`)
**Apply to:** all four scoped reads. Set `Scope` after `newResult`. Never touch `Complete` (Pitfall 8).

## No Analog Found

None. Two pieces are new constructs inside existing files, and the planner should take their shape from RESEARCH.md rather than from a codebase analog:

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/adoBizInt/usage.go` `helpProbeValue` | utility type (flag.Value wrapper) | transform | No `flag.Value` wrapper exists in the repo. Use RESEARCH Pattern 1 (lines 243-266). `stringList` (`flags.go:157-165`) is the only existing `flag.Value` implementation, for method-set reference. |
| `internal/ado/result.go` `Scope` | model | transform | New struct. Use RESEARCH Pattern 3 (lines 327-339) and follow the snake_case tag convention of the `Result` / `Failure` types. |

## Metadata

**Analog search scope:** `cmd/adoBizInt/`, `internal/ado/`, `internal/config/`, `.planning/BACKLOG.md`
**Files scanned:** 27 (usage, crash, main, write, exit, dispatch, render, flags; config.go; result, workitems, dashboard, pullrequests, releases; tests: usage_test, crash_test, write_test, audit_test, writeverbs_test, main_test, fakes_test, filter_test, json_test, wiql_test, nilguard_test, pullrequests_test, releases_test)
**Pattern extraction date:** 2026-10-12
