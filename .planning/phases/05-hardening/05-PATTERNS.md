# Phase 5: Hardening - Pattern Map

**Mapped:** 2026-09-30
**Files analyzed:** 25 (9 new, 16 modified)
**Analogs found:** 24 / 25 (only `.github/workflows/ci.yml` has no in-repo analog)

All analog paths below are git-tracked (`git ls-files` verified). No `.gsd/` mirror paths are used.

**Correction to the orchestrator brief:** there are no go/ast structural tests in `cmd/adoBizInt`. The repo's two go/ast tests are `internal/ado/preread_test.go:289-330` (Phase 4, `TestPreReadNeverReturnsNilNilStructural`) and `internal/ui/workitems/workitems_test.go:131-199` (BL-01, `TestUpdateWorkItemStateWithEffortCallSitesForwardPointers`). The cmd-side "structural" test (`write_test.go:393`, `TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`) is a FlagSet lookup, not an AST walk. Use `preread_test.go` as the go/ast analog for every new structural test in this phase.

**D-10 is REVISED:** both WIQL sites switch to `[System.TeamProject] = @project`. The project name never enters query text. Iteration paths keep `''` escaping through a call expression. Do not implement the original `strings.ReplaceAll(project, …)` escaping.

## File Classification

| New/Modified File | Plan | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|---|
| `internal/ado/attachments.go` (NEW) | 05-01 | service (SDK wrapper) | streaming / file-I/O | `internal/ado/workitems.go:611-643` (the code being replaced) + `workitems.go:306-339`-style SDK args call | exact (same method, rewritten) |
| `internal/ado/attachments_test.go` (NEW) | 05-01 | test (unit + structural) | file-I/O | `internal/ado/workitems_test.go:306-339` (args capture); `internal/ado/preread_test.go:289-330` (go/ast) | exact |
| `internal/ado/interfaces.go` (MOD) | 05-01 | interface seam | request-response | itself, `:17-23` | exact |
| `internal/ado/fakes_test.go` (MOD) | 05-01 | test fake | request-response | itself, `:44-92` | exact |
| `internal/ado/json_test.go` (MOD) | 05-01 | test | transform | itself, `:148-150` | exact |
| `internal/ado/workitems.go` (MOD, 05-01 part) | 05-01 | model + service | transform | `workitems.go:524-538` (child-ID parse from relation URL) | role-match |
| `internal/ado/client.go` (MOD, optional, RESEARCH OQ4) | 05-01 | service | n/a | itself, `:27`, `:125` | exact |
| `internal/ui/workitems/detail.go` (MOD) | 05-01 | component (tea.Cmd) | event-driven | itself, `:29-34`, `:186-194` | exact |
| `internal/ado/pipelines.go` (MOD) | 05-02 | service | CRUD (read) | itself, `:156-164` | exact |
| `internal/ado/releases.go` (MOD) | 05-02 | service | CRUD (read) | itself, `:103-118` | exact |
| `internal/ado/nilguard_test.go` (NEW) | 05-02 | test (structural + fake) | request-response | `preread_test.go:289-330`; `pipelines_test.go:151-181`; `releases_test.go:202-236` | exact |
| `internal/ado/workitems.go` (MOD, 05-03 part) | 05-03 | service | request-response | itself, `:133-155` | exact |
| `internal/ado/dashboard.go` (MOD) | 05-03 | service | request-response | itself, `:120-148` | exact |
| `internal/ado/wiql_test.go` (NEW) | 05-03 | test (structural + fake) | request-response | `preread_test.go:289-330`; `workitems_test.go:85-137`; `dashboard_test.go:177-230` | exact |
| `cmd/adoBizInt/main.go` (MOD) | 05-03 | entrypoint/config wiring | request-response | itself, `:99-105` (`readBudget = cfg.FetchTimeout`) | exact |
| `cmd/adoBizInt/dispatch.go` (MOD) | 05-03 | controller | request-response | itself, `:38-49` (var), `:243-252` (post-Parse) | exact |
| `cmd/adoBizInt/write.go` (MOD) | 05-03 | controller | request-response | itself, `:111-121` (`oneProject`), `:139-179` (arity block) | exact |
| `cmd/adoBizInt/flags.go` (MOD, if the check helper lives here) | 05-03 | utility (validation) | request-response | `write.go:111-121` `oneProject` | role-match |
| `cmd/adoBizInt/main_test.go` (MOD: `withConfiguredProjects`) | 05-03 | test helper | n/a | itself, `:691-699` `withReadBudget` | exact |
| `cmd/adoBizInt/{write,writeverbs,filter,usage}_test.go` (MOD, ~30 sites) | 05-03 | test | n/a | `main_test.go:694-699` call style | exact |
| `TestUnknownProjectRejected` (new test, in an existing cmd `_test.go`) | 05-03 | test | request-response | `write_test.go:354-387` `TestWriteParseErrorsExitUnknown` | exact |
| `.github/workflows/ci.yml` (NEW) | 05-04 | config (CI) | batch | none in repo | no analog |
| `internal/logging/logging.go` (MOD) | 05-05 | utility | file-I/O | itself, `:12-34` | exact |
| `internal/logging/logging_test.go` (NEW, first test in pkg) | 05-05 | test | file-I/O | `cmd/adoBizInt/main_test.go:199-206` (HOME override); `write_test.go:28-35` (`t.TempDir` seam) | role-match |
| `cmd/adoBizInt/patleak_test.go` (NEW) | 05-05 | test (regression) | event-driven (log capture) | `cmd/adoBizInt/crash_test.go:164-181, 233-320` | exact |

---

## Pattern Assignments

### `internal/ado/attachments.go` (NEW — service, streaming/file-I/O) — 05-01

**Analog:** `internal/ado/workitems.go:611-643` (the method being moved and rewritten). Delete it from `workitems.go`. Do not leave a copy.

**What gets removed** (`workitems.go:612-624`). This whole block goes, along with the `net/http` import at `:7`:
```go
func (c *Client) DownloadAttachment(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	...
	req.Header.Set("Authorization", c.connection.AuthorizationString)
	resp, err := http.DefaultClient.Do(req)
	...
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
```

**What stays in shape** (`workitems.go:626-643`): the MkdirAll → Create → io.Copy tail. Change it in two ways. Use `0o700` instead of `0755`. Return `out.Close()` instead of `defer out.Close()`:
```go
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil { return err }
	out, err := os.Create(destPath)
	if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
```

**SDK call shape to copy.** Every existing `c.workitemClient.X(ctx, workitemtracking.XArgs{...})` call looks like this. `workitems.go:509-512` (`GetChildWorkItems`):
```go
	item, err := c.workitemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:     &parentID,
		Expand: &expand,
	})
```
Apply it as `c.workitemClient.GetAttachmentContent(ctx, workitemtracking.GetAttachmentContentArgs{Id: &id})`. The result is an `io.ReadCloser`, so `defer body.Close()`.

**Imports block convention** (`workitems.go:3-18`): stdlib group, then SDK group, then `sentinovo.ai/bizInt/internal/...`. The new file needs `context`, `errors`/`fmt`, `io`, `os`, `path/filepath`, `strings`, `github.com/google/uuid`, `github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking`. Logging (`sentinovo.ai/bizInt/internal/logging`) is optional. If you log, log the name with `%q`/attrs, never a URL.

**Pure containment function.** Use RESEARCH.md Pattern 2 (`attachmentPath(name, destDir) (string, error)`) verbatim. D-08 sanctions the two-use extraction. Add a one-line comment citing D-08 so a reviewer does not flag it under rule of three.

**Doc-comment house style.** Every exported/load-bearing func carries a "why" paragraph that cites the decision ID. See `preread_test.go:274-282` and `write.go:111-116` (`oneProject`): "It is deliberately NOT emptyIfNil…threat T-04-12".

---

### `internal/ado/attachments_test.go` (NEW — test) — 05-01

**Analog A (fake with args capture):** `internal/ado/workitems_test.go:306-339`:
```go
func TestUpdateWorkItemStateIssuesExactPatch(t *testing.T) {
	var got workitemtracking.UpdateWorkItemArgs
	client := &Client{
		workitemClient: &fakeWorkItemAPI{
			t: t,
			UpdateWorkItemFunc: func(_ context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
				got = args
				return &workitemtracking.WorkItem{}, nil
			},
		},
	}
	if err := client.UpdateWorkItemState(context.Background(), 42, "Active"); err != nil { ... }
	...
	if !reflect.DeepEqual(got, want) { t.Errorf(...) }
}
```
- `TestDownloadAttachmentFetchesByID`: capture `args.Id`, return `io.NopCloser(strings.NewReader("payload"))`, and assert that `*args.Id == id` and that the file bytes under `t.TempDir()` match.
- `TestDownloadAttachmentRefusesBeforeFetch`: construct `&fakeWorkItemAPI{t: t}` with `GetAttachmentContentFunc` **left nil**. The fake's `t.Fatalf("unexpected call …")` (see `fakes_test.go:54-60`) is the assertion. This is the same "nil closure is the assertion" idiom as `cmd/adoBizInt/write_test.go:39-41` (`preReadFake`).

**Analog B (hostile-name table):** use a `{name string; input string; wantErr bool}` table. Assert `err != nil || filepath.Dir(got) == destDir` on every row (RESEARCH Pitfall 2). Use `destDir := t.TempDir()` for an absolute directory. The table map/loop style follows `write_test.go:357-387` (`cases := map[string][]string{…}; for name, args := range cases { t.Run(name, …) }`).

**Analog C (structural, `TestNoRawHTTPInADO`):** `internal/ado/preread_test.go:289-330`. Copy the imports (`go/ast`, `go/parser`, `go/token`) and the `parser.ParseFile(fset, …, nil, parser.SkipObjectResolution)` call. The difference is scope: this test walks **every non-test `.go` file in the package**, not one file. Do that with `filepath.Glob("*.go")`, skip `strings.HasSuffix(name, "_test.go")`, and `ParseFile` each file. Do not use `parser.ParseDir`: `go doc` marks it Deprecated under the 1.25+ toolchain. Assertions:
- no `file.Imports[i].Path.Value == "\"net/http\""`
- no `*ast.SelectorExpr` with `Sel.Name == "AuthorizationString"`
- non-vacuity: assert that the file count is > 0, the way preread_test's `seen` map does (`:325-329`).

Copy the doc-comment rationale from `preread_test.go:274-282` ("STATE.md records three Phase 3 incidents where a 'grep returns nothing' criterion was satisfied and then quietly defeated by a prose comment…").

---

### `internal/ado/interfaces.go` (MOD) — 05-01

**Analog:** itself, `:17-23`. Add one line to `workItemAPI` (capital I; CONTEXT's `workitemAPI` is a typo):
```go
type workItemAPI interface {
	UpdateWorkItem(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	...
	GetClassificationNode(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
	GetAttachmentContent(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error) // NEW
}
```
Add `"io"` to the import block at `:3-10`. The header comment at `:12-16` forbids adding an uncalled method (SC4). This one is called.

---

### `internal/ado/fakes_test.go` (MOD) — 05-01

**Analog:** itself, `:44-52` (struct field) and `:86-92` (method). Copy exactly:
```go
	GetAttachmentContentFunc func(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error)
...
func (f *fakeWorkItemAPI) GetAttachmentContent(ctx context.Context, args workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error) {
	f.t.Helper()
	if f.GetAttachmentContentFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetAttachmentContent")
	}
	return f.GetAttachmentContentFunc(ctx, args)
}
```
The compile-time assertion `_ workItemAPI = (*fakeWorkItemAPI)(nil)` at `:15` then fails until the method exists, which is the drift check.

---

### `internal/ado/json_test.go` (MOD) — 05-01

**Analog:** itself, `:148-150`:
```go
	t.Run("Attachment", func(t *testing.T) {
		assertKeySet(t, Attachment{}, []string{"name", "url", "size"})
	})
```
Change it to `[]string{"id", "name", "url", "size"}`.

---

### `internal/ado/workitems.go` — Attachment model and GUID extraction (MOD) — 05-01

**Struct** (`:63-71`): add `ID uuid.UUID \`json:"id"\``. Keep the existing initialism comment style ("URL is a single lower-case word…"). Add a line saying URL is informational only after D-05.

**Extraction** (`:475-497`) is the current shape to extend:
```go
	if item.Relations != nil {
		for _, rel := range *item.Relations {
			if rel.Rel != nil && *rel.Rel == "AttachedFile" {
				attachment := Attachment{}
				if rel.Url != nil {
					attachment.URL = *rel.Url
				}
				...
				if attachment.Name != "" {
					detail.Attachments = append(detail.Attachments, attachment)
				}
```
**In-file analog for URL last-segment parsing:** `workitems.go:527-538` (`GetChildWorkItems`) uses `strings.LastIndex(url, "/")` + `strconv.Atoi`. Do **not** copy that approach for the GUID, because the attachment URL carries `?fileName=`. Use `url.Parse` + `path.Base(u.Path)` + `uuid.Parse` (RESEARCH Pattern 3). Note the local variable `url` at `:530` shadows the package name there. In the attachment block, name the parsed value `u`, not `url`, or the `net/url` import will collide.

**Guard change** (`:492`): `if attachment.Name != "" && hasID {` (D-06, silent suppression).

**Imports** (`:3-18`): after the move, `net/http` has to go (the structural test enforces it). `io`, `os` and `path/filepath` go too if unused. Add `net/url`, `path` and `github.com/google/uuid`. The compiler flags leftovers.

**Test for extraction (`TestGetWorkItemDetailAttachmentID`)**: drive `GetWorkItemDetail` (`workitems.go:370`) through `fakeWorkItemAPI.GetWorkItemFunc`. The closure style is at `workitems_test.go:106-114`. Return a `WorkItem` whose `Relations` contains one `AttachedFile` with a `…/_apis/wit/attachments/<guid>?fileName=Spec.txt` URL and one with a non-GUID tail. Assert one attachment with the parsed ID.

---

### `internal/ado/client.go` (MOD, optional, RESEARCH Open Question 4) — 05-01

After D-01, `connection` is written at `:125` (`connection: connection,`) and read nowhere. The only reader today is `workitems.go:618`. If the planner accepts OQ4, delete the field at `:27` and the assignment at `:125`. Keep `newConnection` (`:55-62`) and the local `connection` in `NewClient` (`:65`), because the sub-clients are built from it. The struct doc comment at `:20-25` mentions "plus a live connection". Update it in the same edit.

---

### `internal/ui/workitems/detail.go` (MOD — component, event-driven) — 05-01

**Analog:** itself. The current caller is `:186-194`:
```go
func (m DetailModel) downloadAttachment(index int) tea.Cmd {
	att := m.item.Attachments[index]
	client := m.client
	return func() tea.Msg {
		home, _ := os.UserHomeDir()
		destPath := filepath.Join(home, ".config", "adoBizInt", "attachments", att.Name)
		err := client.DownloadAttachment(context.Background(), att.URL, destPath)
		return AttachmentDownloadedMsg{Name: att.Name, Path: destPath, Err: err}
	}
}
```
Target: `home, err := os.UserHomeDir(); if err != nil { return AttachmentDownloadedMsg{Name: att.Name, Err: err} }`, then `destDir := filepath.Join(home, ".config", "adoBizInt", "attachments")` and `client.DownloadAttachment(context.Background(), att.ID, destDir, att.Name)`.

**Message struct** (`:29-34`): drop `Path string`. It is never read. The only consumer is `:106-112`, which reads `msg.Err` and `msg.Name`. D-07 errors flow through the existing `"Error: " + msg.Err.Error()` at `:109` unchanged.

`os` and `filepath` stay imported. Line 189-190 remain their only users.

---

### `internal/ado/pipelines.go` and `internal/ado/releases.go` (MOD — nil guards) — 05-02

**Analog:** the existing guard immediately above each site.

`pipelines.go:156-164`:
```go
		for _, d := range definitions.Value {
			if d.Name == nil {          // -> if d.Name == nil || d.Id == nil {
				continue
			}
			allDefNames = append(allDefNames, *d.Name)
			if pipelineNameMatches(*d.Name, c.pipelines) {
				matchingDefs = append(matchingDefs, d)
			}
		}
```
Deref site: `pipelines.go:171` `defID := *def.Id`.

`releases.go:103-118` has the same shape (`rd`), with the deref at `releases.go:125`.

**In-repo precedent for the combined guard:** `internal/ado/branches.go:84` `if repo.Name == nil || repo.Id == nil {`.

**Negative control for the structural test:** `pipelines.go:311-327` (`GetPipelineDefinitions`) also loops `definitions.Value` with an `if d.Name == nil { continue }` guard. It appends a freshly built `PipelineDefinition`, not `d`, and guards `Id` inline (`if d.Id != nil`). The "range value appears as `append(_, v)`" selector must **not** match it. Assert that exactly 2 loops were found, not ≥2, if you want a third collecting loop to force a review.

---

### `internal/ado/nilguard_test.go` (NEW) — 05-02

**Structural half (`TestCollectedDefinitionsGuardNameAndID`):** copy `preread_test.go:289-330` scaffolding (fset, ParseFile, `seen`/count non-vacuity check). Walk package non-test files via glob, as above. Use `ast.Inspect` for `*ast.RangeStmt`. Read the value ident from `rs.Value.(*ast.Ident)`, then find `append` `*ast.CallExpr` in `rs.Body` whose `Args[1]` is that ident. Require `rs.Body.List[0]` to be `*ast.IfStmt` with body `continue` and an `||`-flattened condition containing `v.Name == nil` and `v.Id == nil`. RESEARCH Pattern 4 records the prototype output (`pipelines.go:156`, `releases.go:103`).

**Behavioural half (`TestGetPipelineRunsSkipsNilID`, `TestGetReleasesSkipsNilID`):**
- Analog: `internal/ado/pipelines_test.go:151-181` (`pipelineTestClient`). Its fixture already includes a nil-**Name** definition (`{Id: &unmatchedID}` at `:170`). Build a local client with a `{Name: strPtr("pipeline-one")}` (nil `Id`) entry beside a normal one. Assert no panic and `len(result.Items) == 1`. Follow the assertion style of `pipelines_test.go:186-198`.
- Analog: `internal/ado/releases_test.go:202-236` (`releaseTestClient`), same approach with `{Name: strPtr("release-definition-one")}`.
- `strPtr` lives at `internal/ado/client.go:283`.
- The existing helpers take a hook for errors, not definitions. Either inline a `&Client{…buildClient: &fakeBuildAPI{…}}` in the new test (preferred: single use) or widen the helper. Do not fork a second helper with the same name.

---

### `internal/ado/workitems.go` and `internal/ado/dashboard.go` — WIQL (MOD) — 05-03

**Current shape** (`workitems.go:133-155`; `dashboard.go:120-148` is identical except for the dashboard's `appendFailure` on the error branch at `:132`):
```go
	iterationFilter := ""
	iterations, err := c.GetRecentIterations(ctx, project)
	if err != nil {
		log.Warn("Failed to get iterations, using date filter only", "project", project, "error", err)
	} else if len(iterations) > 0 {
		escapedPaths := make([]string, len(iterations))
		for i, path := range iterations {
			escapedPaths[i] = "'" + strings.ReplaceAll(path, "'", "''") + "'"
		}
		iterationFilter = " AND [System.IterationPath] IN (" + strings.Join(escapedPaths, ", ") + ")"
		...
	}
	wiql := "SELECT … FROM WorkItems WHERE [System.TeamProject] = '" + project + "' AND … " + iterationFilter + " ORDER BY [System.Id] DESC"
	query := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &wiql},
		Project: &project,          // keep: this is what @project resolves from
	}
	log.Info("Executing WIQL query", "project", project, "query", wiql)   // D-11: drop "query", wiql
```
**Changes (revised D-10 + D-11 + D-12):**
1. `[System.TeamProject] = '" + project + "'` → `[System.TeamProject] = @project` inside the literal. Keep `Project: &project` on the args, because the SDK route carries it.
2. Replace the duplicated 5-line escaping loop with one private helper (e.g. `iterationClause(paths []string) string`) called inline in the `+` chain. D-12 forces this: a bare `iterationFilter` ident fails the structural rule (RESEARCH Pitfall 4). Two uses sit below rule of three, so record it as a justified exception with a `// note:` comment, as D-08 does. Put the helper in `workitems.go` or `client.go` next to `GetRecentIterations` (`client.go:170`). Do not create a new file for it.
3. Keep the error branches unchanged (dashboard `appendFailure(failures, project, opDashboardIterations, err)` at `dashboard.go:132`).
4. Log line: `log.Info("Executing WIQL query", "project", project, "iterations", len(iterations))`. For the result count, the existing `"total_found"` line at `workitems.go:168` and `"count"` at `dashboard.go:156` already cover it.

**Live-org check:** D-10 revised requires one live read per site before the phase is done. The planner should add a manual verification task, because CI has no credentials (D-17).

---

### `internal/ado/wiql_test.go` (NEW) — 05-03

**Structural (`TestWIQLInterpolationStructural`):** scaffolding as in `preread_test.go:289-330`. Walk the package's non-test files and use the `flattenAdd` skeleton in RESEARCH §Code Examples. Rules:
- every `+` chain containing a `"FROM WorkItems"` STRING `BasicLit` has only `*ast.BasicLit` or allow-listed `*ast.CallExpr` operands (`iterationClause`; RESEARCH OQ2 recommends allow-listing callees)
- the chain contains `@project` and no `[System.TeamProject] = '` literal
- the count of all `FROM WorkItems` literals equals the in-chain count, which closes the `fmt.Sprintf` evasion
- sites == 2 (non-vacuity)

**Behavioural (`TestGetWorkItemsUsesProjectMacro`, `TestDashboardUsesProjectMacro`):**
- Analog: `internal/ado/workitems_test.go:85-137`. A `fakeWorkItemAPI` with `GetClassificationNodeFunc` + `QueryByWiqlFunc` reading `args.Project`. Capture `*args.Wiql.Query` instead. Use a project like `O'Brien` and assert the query contains `@project` and does **not** contain `O'Brien` or `O''Brien`. Construct the client as `&Client{projects: []string{"O'Brien"}, workitemClient: api}` (`:116-119`).
- Analog: `internal/ado/dashboard_test.go:177-230` (`dashboardClientFor`), same capture on `QueryByWiqlFunc` at `:202-212`.
- Log assertion (no query text): capture logs by swapping `slog.Default()`. `logging.Logger()` falls back to it (`internal/logging/logging.go:37-43`). Save/restore with `t.Cleanup`, and assert the buffer lacks `FROM WorkItems`.

---

### `cmd/adoBizInt/main.go` + `dispatch.go` + `write.go` (+ `flags.go`) — project allow-list (MOD) — 05-03

**Package-var analog:** `dispatch.go:38-49` `var readBudget time.Duration`, with a long "why" doc comment. Declare `var configuredProjects []string` beside it, with a comment citing D-09 and saying that a nil value rejects every `--project` (fail closed).

**Assignment analog:** `main.go:99-105`:
```go
	// The fan-out budget is captured only now, after validation has passed (D-P6). …
	readBudget = cfg.FetchTimeout
```
Add `configuredProjects = cfg.Projects` immediately after it. `cfg.Projects` is `config.StringSlice` (`internal/config/config.go:13`, `type StringSlice []string`), so either convert it or declare the var as `[]string` and assign `[]string(cfg.Projects)`.

**Check-helper analog:** `write.go:111-121` `oneProject`:
```go
func oneProject(name string, gf *globalFlags) (string, error) {
	if len(gf.projects) != 1 {
		return "", fmt.Errorf("%s requires exactly one --project value", name)
	}
	return gf.projects[0], nil
}
```
Copy the shape for a private `unknownProject(gf *globalFlags) error` (name is the planner's call). It returns `fmt.Errorf("unknown --project %q; configured projects: …", p)` for the first value not in `configuredProjects`. Use `%q` on agent-supplied strings. The helper has two call sites (runRead, runWrite), so it belongs beside `oneProject` in `write.go` or near `globalFlags` in `flags.go`. It does not belong in a new file.

**Call site 1 (read):** `dispatch.go:243-252`. After `fs.Parse` succeeds, and before `log := logging.Logger()` and `emptyIfNil(gf.projects)` at `:266`:
```go
	fs, gf, ff, _ := newFlagSet(name)
	if err := fs.Parse(args); err != nil { …; return exitUnknown }
	ff.identity = identity
	// NEW: if err := unknownProject(gf); err != nil { fmt.Fprintf(stderrWriter, "%v\n", err); return exitUnknown }
```
**Call site 2 (write):** `write.go:139-179`. Put it inside the "Arity, all failing to exit 3" block (comment at `:156-157`), next to `op.resolve` at `:175-179`, which already uses the exact error-emit idiom:
```go
	target, err := op.resolve(gf, wf, id)
	if err != nil {
		fmt.Fprintf(stderrWriter, "%v\n", err)
		return exitUnknown
	}
```
It must run **before** `auditOpen()` (`write.go:~196`) and before `fetchctx.NewWithParent` (`:189`).

---

### `cmd/adoBizInt/main_test.go` — `withConfiguredProjects` (MOD) — 05-03

**Analog:** itself, `:691-699`:
```go
// withReadBudget sets the package-level fan-out budget for one test and
// restores it afterwards, standing in for the assignment run() makes after
// config validation.
func withReadBudget(t *testing.T, d time.Duration) {
	t.Helper()
	prev := readBudget
	readBudget = d
	t.Cleanup(func() { readBudget = prev })
}
```
Copy it as `withConfiguredProjects(t *testing.T, names ...string)`. Call it explicitly in each affected test, the same way `withReadBudget(t, 2*time.Minute)` is called at the top of `crash_test.go:185`. Do not hide it in `captureOutput` or a `TestMain` (RESEARCH Pitfall 6).

**Sites to update** (RESEARCH Pitfall 6): `write_test.go` ~20 (most via `setStateArgs` at `write_test.go:59-63`, which hardcodes `--project P`), `writeverbs_test.go` ~6, `filter_test.go` 2 (`:151` `"one"`, and the alpha/beta cases), `usage_test.go` 2 (`:28` `TestProjectListAccumulates`, which may parse only and not dispatch, so check before adding). Run `go test ./cmd/adoBizInt` before and after.

---

### `TestUnknownProjectRejected` (new test in an existing cmd `_test.go`) — 05-03

**Analog:** `cmd/adoBizInt/write_test.go:354-387`:
```go
func TestWriteParseErrorsExitUnknown(t *testing.T) {
	cases := map[string][]string{ … }
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			tempAuditSink(t)
			stdout, stderr := captureOutput(t)
			// Neither closure is set: any client call fails this subtest by name.
			c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}
			if code := dispatch(context.Background(), c, args); code != exitUnknown { t.Fatalf(…) }
			if stdout.Len() != 0 { t.Errorf(…) }
		})
	}
}
```
Rows: read `workitems list --project bogus`, read mixed `--project P,bogus` (fail closed), write `setStateArgs` with `--project bogus`. Each calls `withConfiguredProjects(t, "P")`. Also assert that stderr contains the `%q`-quoted value. Place it in `filter_test.go` (read-path `--project` tests live there, `:146`) or `write_test.go`. The planner decides. Do not create a new file for one test.

---

### `.github/workflows/ci.yml` (NEW — config) — 05-04

**No in-repo analog.** `.github/` does not exist. Use the RESEARCH §Code Examples "CI workflow" block verbatim: SHA-pinned `actions/checkout@3d3c42e5…` (v7.0.1) and `actions/setup-go@b7ad1dad…` (v7.0.0), `permissions: contents: read`, `go-version: '1.25.4'` (quoted, D-16), four steps, and a gofmt step that lists offending files. No secrets or env (D-17). `.gitignore` (`/adoBizInt` only) needs no change.

---

### `internal/logging/logging.go` (MOD — utility, file-I/O) — 05-05

**Analog:** itself, `:12-34`:
```go
	logDir := filepath.Join(home, ".config", "adoBizInt")
	if err := os.MkdirAll(logDir, 0755); err != nil {          // -> 0o700
		return err
	}
	// NEW: if err := os.Chmod(logDir, 0o700); err != nil { return err }   (D-20: every call)
	logPath := filepath.Join(logDir, "adoBizInt.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)   // -> 0o600
	if err != nil {
		return err
	}
	// NEW: if err := file.Chmod(0o600); err != nil { file.Close(); return err }   (Pitfall 9)
	logger = slog.New(...)
```
Error policy (D-21): return the error. The caller `cmd/adoBizInt/main.go:53-60` (`initLogging`) already warns and sets `loggingReady = false`, so no caller change is needed. Level and rotation are unchanged (D-18).

**Adjacent observation, out of scope per D-18:** `cmd/adoBizInt/audit.go:77,80` creates the same `~/.config/adoBizInt` directory with `0755` and the audit file with `0644`. The D-20 chmod repairs the directory on every `Init`. The audit file mode stays as it is. Record it as a follow-up and do not fix it in this phase.

---

### `internal/logging/logging_test.go` (NEW — first test in the package) — 05-05

**Analog (HOME redirection):** `cmd/adoBizInt/main_test.go:199-206`:
```go
	notADir := filepath.Join(t.TempDir(), "home-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil { t.Fatalf("write fixture: %v", err) }
	t.Setenv("HOME", notADir)
```
Invert it: `home := t.TempDir(); t.Setenv("HOME", home)`. Pre-create `dir` at `0o755` and the log at `0o644` for the "existing install" subtest. Skip pre-creation for the "fresh install" subtest. After `Init()`, assert `Stat(...).Mode().Perm()`. Guard with `if runtime.GOOS == "windows" { t.Skip(...) }` (Pitfall 9). RESEARCH §Code Examples has the skeleton.

Package-state note: `Init` sets the package-level `logger`. Restore it with `t.Cleanup(func() { logger = nil })`. This is an in-package test (`package logging`), so the unexported var is reachable. It follows the save/restore idiom of `withReadBudget`.

---

### `cmd/adoBizInt/patleak_test.go` (NEW — regression) — 05-05

**Analog A (panic path + credential fixture):** `cmd/adoBizInt/crash_test.go:164-181`:
```go
const patFixture = "pat-fixture-do-not-disclose-9f3c1a"

func panickingClient(t *testing.T) *fakeReadClient {
	t.Helper()
	cfg := &config.Config{PAT: patFixture, OrgURL: "https://dev.azure.com/fixture"}
	return &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(context.Context, []string) (ado.Result[ado.PullRequest], error) {
			panic("deliberate panic reading " + cfg.OrgURL)
		},
	}
}
```
Reuse `patFixture` as the sentinel rather than declaring a second constant, because it is already a distinctive, never-real value in the same package. RESEARCH's `SENTINELPAT-…` literal is a suggestion only. Also check for `azuredevops.CreateBasicAuthHeaderValue("", patFixture)` (Pitfall 8).

**Analog B (assert-absent + guard call):** `crash_test.go:233-262` (`TestPanicRecordCarriesNoStackOrCredential`):
```go
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)
	guard(func() int {
		return dispatch(context.Background(), panickingClient(t), []string{"prs", "list", "--json"})
	})
	out := stdout.String()
	if strings.Contains(out, patFixture) { t.Errorf(…) }
```
The difference from that test: capture **slog**, not stdout. Swap `slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))` with `t.Cleanup` restore, and set `loggingReady = true` with restore (var at `crash.go:73`, gate at `crash.go:89-94`) so `handlePanic` writes the stack into the buffer. Non-vacuity: assert that `buf` contains `"panic recovered"` (the message at `crash.go:90`).

**Drive the three paths with existing fixtures:**
- read: `dispatch(ctx, allReadsFake(t), []string{"prs", "list", "--json"})`. `allReadsFake` is at `cmd/adoBizInt/fakes_test.go:101-130`.
- write (dry run): `tempAuditSink(t)` (`write_test.go:28-35`) + `dispatch(ctx, preReadFake(t, "Active"), setStateArgs())` (`write_test.go:42-56`, `:59-63`) + `withConfiguredProjects(t, "P")` (from 05-03).
- panic: as in Analog B.

Do **not** call `initLogging()` here. Both existing callers (`main_test.go:221`, `crash_test.go:288`) force it to fail by setting HOME to a file. A successful `Init` would redirect `logging.Logger()` away from `slog.Default()` and make the capture vacuous (Pitfall 8). No `t.Parallel()`, since no cmd test uses it.

---

## Shared Patterns

### Fake seam: the nil closure is the assertion
**Source:** `internal/ado/fakes_test.go:54-92` (ado), `cmd/adoBizInt/write_test.go:39-56` + `:376` (cmd)
**Apply to:** attachments_test (refuse-before-fetch), nilguard_test, wiql_test, TestUnknownProjectRejected, patleak_test
```go
	if f.XFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.X")
	}
```
Leave every closure a test must not reach set to nil. Never count calls after the fact.

### go/ast structural assertion with non-vacuity
**Source:** `internal/ado/preread_test.go:274-330` (also `internal/ui/workitems/workitems_test.go:131-160`)
**Apply to:** `TestNoRawHTTPInADO`, `TestCollectedDefinitionsGuardNameAndID`, `TestWIQLInterpolationStructural`
- `fset := token.NewFileSet()`; `parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)`
- report positions with `fset.Position(n.Pos())` in the error text
- finish with a `seen`/count check that fails if nothing was covered (`preread_test.go:325-329`)
- doc comment cites the three Phase 3 grep-defeat incidents (`preread_test.go:276-282`)
- package-wide variant: `filepath.Glob("*.go")` minus `_test.go` (the test's cwd is the package dir). Avoid the deprecated `parser.ParseDir`.

### Package-var + `withX(t, …)` save/restore
**Source:** `cmd/adoBizInt/main_test.go:691-699` (`withReadBudget`), `write_test.go:28-35` (`tempAuditSink`), `main_test.go:55-68` (`captureOutput`)
**Apply to:** `configuredProjects` / `withConfiguredProjects`, `loggingReady` and `slog.Default` swaps in patleak_test, `logging.logger` reset in logging_test
```go
	prev := X
	X = v
	t.Cleanup(func() { X = prev })
```

### Pre-work rejection → exit 3, stderr only, stdout empty
**Source:** `cmd/adoBizInt/write.go:156-179`; exit constants `dispatch.go:21-29` (`exitUnknown = 3 // the read never ran`)
**Apply to:** the D-09 project check in both `runRead` and `runWrite`
```go
	fmt.Fprintf(stderrWriter, "%v\n", err)
	return exitUnknown
```
Format agent- and server-supplied strings with `%q` (see `write.go:169` `"%s: %q is not a numeric id\n"`).

### Suppress malformed server data silently (Phase 2 D-07)
**Source:** `pipelines.go:157-159`, `releases.go:104-106`, `workitems.go:492`, `branches.go:84`
**Apply to:** D-06 (unparsable GUID not appended) and D-14 (nil `Id` → `continue`). No `Failure`, no log line required.

### Logging through `logging.Logger()` with structured attrs
**Source:** `workitems.go:132` (`log := logging.Logger()`), `:168` (`log.Info("Query returned work items", "project", project, "total_found", totalFound)`)
**Apply to:** the D-11 rewrite of both WIQL log lines, and any log in attachments.go. Never log a query string, a URL carrying a token, or a config struct.

### Doc-comment "why + decision ID" house style
**Source:** `write.go:111-116`, `main.go:99-104`, `crash.go:75-86`
**Apply to:** every new function, var and test. Comments explain the decision (cite D-xx / T-xx / Pitfall) and never narrate the code.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `.github/workflows/ci.yml` | config (CI) | batch | No CI, no `.github/` directory, and no build automation exist in the repo. Use RESEARCH.md §Code Examples "CI workflow". |

Partial gaps (analog exists, but the specific mechanism is new to the repo):
- **Package-wide AST walk.** Both existing go/ast tests parse one named file. The glob-over-package loop is new. The scaffolding still comes from `preread_test.go`.
- **Log capture into a buffer.** No existing test swaps `slog.Default()`. The two existing tests capture process stderr/stdout with `os.Pipe` (`main_test.go:213-230`, `crash_test.go:280-300`). The slog swap is RESEARCH's "Don't Hand-Roll" recommendation and needs no production change.
- **File-mode assertions.** No test in the repo asserts `Mode().Perm()` today.

## Metadata

**Analog search scope:** `internal/ado/`, `internal/logging/`, `internal/ui/workitems/`, `internal/config/`, `cmd/adoBizInt/` (all git-tracked; `.gsd/` excluded)
**Files scanned:** 24 source/test files read or grepped
**Pattern extraction date:** 2026-09-30
