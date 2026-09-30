---
phase: 05-hardening
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/ado/interfaces.go
  - internal/ado/fakes_test.go
  - internal/ado/attachments.go
  - internal/ado/attachments_test.go
  - internal/ado/workitems.go
  - internal/ado/json_test.go
  - internal/ado/client.go
  - internal/ui/workitems/detail.go
autonomous: true
requirements: [HARD-01, HARD-02]

estimate:
  tokens: 62000
  raw_tokens: 62000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Downloading an attachment from the work-item detail modal fetches it through `workitemClient.GetAttachmentContent` with the GUID parsed from the relation URL, and writes the returned bytes to `~/.config/adoBizInt/attachments/<name>` (HARD-01, D-01)."
    - "`ado.Client.DownloadAttachment` has the signature `(ctx context.Context, id uuid.UUID, destDir, name string) error`. No parameter accepts a URL, so neither a caller nor a server-supplied relation URL chooses the host the PAT is sent to (HARD-01, D-01, D-03)."
    - "No non-test file in `internal/ado` imports `net/http`, reads `AuthorizationString`, or declares a struct field of type `*azuredevops.Connection`. `TestNoRawHTTPInADO` asserts all three over the AST, so a prose comment can neither satisfy nor defeat it (HARD-01, D-01, RESEARCH OQ4)."
    - "The download inherits the SDK connection's `Timeout`, which resolves the TRUST-04 exclusion Phase 2 deferred to this phase. It is resolved through the SDK connection (D-01), not through a host allow-list."
    - "A name that is empty, `.`, `..`, `../`, absolute, NUL-bearing or all-separator returns an error from `DownloadAttachment` before any `GetAttachmentContent` call and leaves `destDir` empty. Every accepted name resolves to a direct child of `destDir` (HARD-02, D-03, D-08)."
    - "`attachmentPath(name, destDir)` is a pure function whose hostile-name table test asserts, on every row, that the result is an error or a direct child of `destDir` (D-08)."
    - "A refused name reaches the TUI as `AttachmentDownloadedMsg{Err: ...}` and renders as `Error: ...` on the same path a network failure takes. It never renders as `Downloaded:` (D-07)."
    - "An `AttachedFile` relation whose URL has no parsable, non-nil GUID in its last path segment is not appended to `WorkItemDetail.Attachments`, and `GetWorkItemDetail` records no failure for it (D-06)."
    - "`Attachment` marshals with exactly the keys `id`, `name`, `url`, `size`. `id` is the canonical GUID string, and `url` is kept only as information for a human (D-05)."
    - "If the body copy fails partway, including when the connection timeout fires mid-body, `DownloadAttachment` returns the error, closes the body, and removes the partial file. A truncated file never sits at the destination looking complete (edge-probe HARD-01/concurrency; RESEARCH Pitfall 7)."
    - "`DownloadAttachment` applies no byte cap (D-04: the cap is deferred with the CLI verb)."
    - "No CLI attachment verb ships. `TestDispatchTableMatchesD02` passes with `wantCommands` and `wantWriteCommands` unedited (D-02)."
    - statement: "A real attachment downloaded through the TUI detail modal lands at ~/.config/adoBizInt/attachments/<name> with contents identical to the file in the ADO web UI. Checked by the Task 3 human-check."
      verification: backstop
  prohibitions:
    - statement: "The attachment path must never fetch by URL. This includes a fallback for an attachment whose GUID cannot be parsed, and any second download path. An attachment without a parsable GUID is simply not offered (D-06)."
      status: resolved
      verification: test
    - statement: "A download refused by containment must never be reported to the operator as a successful download and must never leave a file on disk."
      status: resolved
      verification: test
  artifacts:
    - path: "internal/ado/attachments.go"
      provides: "DownloadAttachment by GUID through the SDK, and the attachmentPath containment guard"
      contains: "GetAttachmentContent"
    - path: "internal/ado/attachments_test.go"
      provides: "End-to-end by-ID test, hostile-name table, refuse-before-fetch, partial-file cleanup, D-06 extraction rows, TestNoRawHTTPInADO"
      contains: "TestNoRawHTTPInADO"
    - path: "internal/ado/interfaces.go"
      provides: "workItemAPI.GetAttachmentContent seam entry"
      contains: "GetAttachmentContent"
    - path: "internal/ui/workitems/detail.go"
      provides: "Sole caller, passing att.ID, the attachments directory and att.Name"
      contains: "att.ID"
  key_links:
    - from: "internal/ui/workitems/detail.go"
      to: "internal/ado/attachments.go"
      via: "downloadAttachment tea.Cmd calls client.DownloadAttachment(ctx, att.ID, destDir, att.Name)"
      pattern: "DownloadAttachment\\(context\\.Background\\(\\), att\\.ID"
    - from: "internal/ado/attachments.go"
      to: "internal/ado/interfaces.go"
      via: "attachmentPath runs first; only an accepted name reaches workitemClient.GetAttachmentContent"
      pattern: "GetAttachmentContent\\(ctx, workitemtracking\\.GetAttachmentContentArgs\\{Id: &id\\}\\)"
    - from: "internal/ado/workitems.go"
      to: "internal/ado/attachments.go"
      via: "GetWorkItemDetail parses the relation URL into Attachment.ID, which is the only ID source the download accepts"
      pattern: "uuid\\.Parse\\(path\\.Base"
---

<objective>
Close CONCERNS.md C3 and H6 through the one function they share. `DownloadAttachment`
stops taking a URL, fetches by attachment GUID through the SDK's `GetAttachmentContent`,
and refuses any name that would write outside the attachments directory before it makes
a network call.

Purpose: today a server-supplied relation URL decides where the PAT goes, and a
server-supplied file name decides where bytes land. After this plan the SDK owns the
host, and `internal/ado` owns the path.
Output: `internal/ado/attachments.go` (new), `attachments_test.go` (new), the retyped
`Attachment`, the rewired TUI caller, and a structural test that keeps raw HTTP out of
`internal/ado`.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/05-hardening/05-CONTEXT.md
@.planning/phases/05-hardening/05-RESEARCH.md
@.planning/phases/05-hardening/05-PATTERNS.md
@.planning/phases/05-hardening/05-VALIDATION.md
</context>

## Artifacts this phase produces

Phase 05 creates or changes the symbols below. None of them exists in the codebase yet,
so do not check them against current source. **This plan owns the 05-01 rows.**

| Plan | Symbol / file | Kind |
|---|---|---|
| 05-01 | `internal/ado/attachments.go` | new file |
| 05-01 | `func (c *Client) DownloadAttachment(ctx context.Context, id uuid.UUID, destDir, name string) error` | replaces the URL-taking method in `workitems.go` |
| 05-01 | `func attachmentPath(name, destDir string) (string, error)` | new, unexported (the D-08 two-use extraction) |
| 05-01 | `Attachment.ID uuid.UUID` with tag `json:"id"` | new field |
| 05-01 | `workItemAPI.GetAttachmentContent(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error)` | new interface entry |
| 05-01 | `fakeWorkItemAPI.GetAttachmentContentFunc` and its method | new test fake member |
| 05-01 | removed: `Client.connection` field, `AttachmentDownloadedMsg.Path` field, URL-based `DownloadAttachment` | deletions |
| 05-01 | tests `TestDownloadAttachmentFetchesByID`, `TestAttachmentPath`, `TestDownloadAttachmentRefusesBeforeFetch`, `TestDownloadAttachmentRemovesPartialFile`, `TestGetWorkItemDetailAttachmentID`, `TestNoRawHTTPInADO` | new tests |
| 05-02 | `internal/ado/nilguard_test.go`: `TestGetPipelineRunsSkipsNilID`, `TestGetReleasesSkipsNilID`, `TestCollectedDefinitionsGuardNameAndID` | new tests |
| 05-03 | `.github/workflows/ci.yml` (workflow `ci`, job `check`) | new file |
| 05-04 | `var configuredProjects []string` in `cmd/adoBizInt/flags.go`; `--project` registered through `fs.Func` with the allow-list check | new var, changed registration |
| 05-04 | `withConfiguredProjects(t *testing.T, names ...string)` in `cmd/adoBizInt/main_test.go`; `TestUnknownProjectRejected` in `filter_test.go` | new test helper and test |
| 05-05 | `func iterationClause(paths []string) string` in `internal/ado/client.go` | new, unexported |
| 05-05 | `internal/ado/wiql_test.go`: `TestGetWorkItemsUsesProjectMacro`, `TestDashboardUsesProjectMacro`, `TestIterationClauseEscapesQuotes`, `TestWIQLInterpolationStructural` | new tests |
| 05-06 | `internal/logging/logging_test.go`: `TestInitSetsOwnerOnlyPermissions`; `cmd/adoBizInt/patleak_test.go`: `TestPATNeverLogged` | new tests (no new production symbol) |

## Planner decisions recorded in this plan

| Item | Decision | Rationale |
|---|---|---|
| RESEARCH OQ3: backslash names such as `..\` | **Accept D-03 as written.** No extra rule rejects separators. | On Linux and macOS, `..\` is a literal file name, and it lands as a direct child of `destDir`. On Windows, `filepath.Base` strips it to `..`, which is rejected. The table test asserts the containment invariant (error, or a direct child of `destDir`) on every row, so it passes on either OS. Rejecting every separator would tighten D-03 without a stated need. |
| RESEARCH OQ4: `Client.connection` | **Delete the field** (Task 3). | After D-01 the field is written in `NewClient` and read nowhere. Deleting it follows directly from D-01, so no struct in `internal/ado` holds the credential-bearing connection. D-01's wording "through `c.connection`" is satisfied by `c.workitemClient`, which `NewClient` builds from that same connection. `newConnection` and its two tests stay. |
| RESEARCH Pitfall 7: partial file on a mid-body failure | **Remove the partial file** when the copy or the close fails. | The core value applies: a truncated file must not look like a completed download. |
| "resolved destination directory" in D-03 | `filepath.Clean(destDir)` plus an `IsAbs` check, **not** `EvalSymlinks` | On macOS `t.TempDir()` sits under the `/var` → `/private/var` symlink. Resolving only one side rejects legitimate paths. A planted symlink needs local write access to `~/.config/adoBizInt`, which is outside this threat model. `os.Root` is deferred with the CLI verb. |
| `go.mod` | **Unchanged** | `github.com/google/uuid` is already in the module graph and is marked `// indirect`. `go build` does not need `go mod tidy`, and CI does not check tidiness. |
| Live TUI download check | **A `<human-check>` on Task 3**, not on the Task 1 tracer (end-of-phase UAT) | The verifier harvests `<human-check>` only from `auto` tasks. On the tracer, the check would halt an interactive run mid-flight and be skipped in auto mode, so it would never reach UAT. Task 3 is the last task, so the check exercises the finished client. |
| Tracer file count (7) | Above the 5-file heuristic. **Accepted and recorded here.** | The files form one unit that cannot be split without a broken build between commits. The interface entry and the fake are tied together by the `_ workItemAPI = (*fakeWorkItemAPI)(nil)` assertion. The new signature and its sole caller are tied together by the compiler. The new `id` JSON key and `TestJSONFieldNames` must change in the same commit. The whole diff is about 120 lines. |

## Edge-probe and prohibition-probe disposition (spec-less fallback)

| Probe item | Disposition |
|---|---|
| HARD-01 / concurrency ("if interrupted, what is guaranteed?") | resolved (explicit). A mid-body failure removes the partial file (truth 10). A failed fetch never creates the file, because `os.Create` runs only after `GetAttachmentContent` returns a body. |
| HARD-02 / unclassified | **Flagged assumption.** The probe could not classify HARD-02. I reviewed it manually and authored truths 5, 6 and 7 from the D-08 hostile set. The probe row stays `unresolved` per the fallback rules. |
| Prohibition recall | Two bespoke items kept (see `must_haves.prohibitions`). Path traversal and credential forwarding to an attacker host are canon OWASP items, covered by `/gsd-secure-phase`, and are not minted as prohibitions here. |

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: Download one attachment by ID from the detail modal, end to end</name>
  <reversibility rating="costly">D-01/D-03: the `destPath` → `(id, destDir, name)` signature change touches the interface and the call site. Undoing it means reinstating the raw HTTP path, which is the vulnerability itself.</reversibility>
  <files>internal/ado/interfaces.go, internal/ado/fakes_test.go, internal/ado/attachments.go, internal/ado/attachments_test.go, internal/ado/workitems.go, internal/ado/json_test.go, internal/ui/workitems/detail.go</files>
  <read_first>
    - internal/ado/workitems.go (imports at :3-18; `Attachment` at :63-71; attachment extraction in `GetWorkItemDetail` at :475-497; the URL-based `DownloadAttachment` at :611-643; `GetChildWorkItems`' local `url` variable at :530)
    - internal/ado/interfaces.go (whole file; `workItemAPI` at :17-23 and the header comment at :12-16 forbidding uncalled entries)
    - internal/ado/fakes_test.go (compile-time assertions at :14-19; `fakeWorkItemAPI` at :44-92)
    - internal/ado/json_test.go:145-152 (the `Attachment` key-set subtest)
    - internal/ui/workitems/detail.go (`AttachmentDownloadedMsg` at :29-34; its handler at :106-112; `downloadAttachment` at :186-194)
    - internal/ado/workitems_test.go:306-339 (args-capture fake idiom to copy)
    - .planning/phases/05-hardening/05-RESEARCH.md § "SDK facts", Pattern 1, Pattern 2, Pattern 3
    - .planning/phases/05-hardening/05-PATTERNS.md § attachments.go, interfaces.go, fakes_test.go, json_test.go, workitems.go, detail.go
  </read_first>
  <behavior>
    - A work item whose `AttachedFile` relation URL is `https://dev.azure.com/fabrikam/_apis/wit/attachments/098a279a-60b9-40a8-868b-b7fd00c0a439?fileName=Spec.txt`, with attributes `name: Spec.txt` and `resourceSize: 7`, yields exactly one `Attachment`. Its `ID` equals that GUID and its `Name` is `Spec.txt`.
    - `DownloadAttachment(ctx, thatID, dir, "Spec.txt")` calls the fake's `GetAttachmentContent` exactly once. The args deep-equal `workitemtracking.GetAttachmentContentArgs{Id: &thatID}`, with no Project, FileName or Download set. The call writes the fake body's bytes (`payload`) to `dir/Spec.txt`.
    - `TestJSONFieldNames/Attachment` expects the key set `id`, `name`, `url`, `size`.
  </behavior>
  <action>
    Write `TestDownloadAttachmentFetchesByID` in the new `internal/ado/attachments_test.go` first, and confirm it fails to compile or fails. Then implement until it passes.

    The test drives the real path at the `internal/ado` layer. Build `&Client{workitemClient: &fakeWorkItemAPI{...}}`. Set `GetWorkItemFunc` to return a `workitemtracking.WorkItem` with non-nil `Fields` (at least `System.Title`) and `Relations` holding the `AttachedFile` relation from `<behavior>`. Set `GetAttachmentContentFunc` to capture its args and return `io.NopCloser(strings.NewReader("payload"))`. Call `GetWorkItemDetail`, take `Attachments[0]`, and pass its `ID` and `Name` to `DownloadAttachment` with `destDir := t.TempDir()`. Assert the captured args with `reflect.DeepEqual` against `GetAttachmentContentArgs{Id: &want}`, and assert the file's bytes.

    **`internal/ado/interfaces.go`**: add `GetAttachmentContent(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error)` as the last `workItemAPI` entry, and add `io` to the imports. This interface is `workItemAPI` with a capital I. CONTEXT.md's `workitemAPI` is a typo. The header comment at :12-16 still holds, because the entry is called (D-01).

    **`internal/ado/fakes_test.go`**: add a `GetAttachmentContentFunc` field and a `GetAttachmentContent` method to `fakeWorkItemAPI`. Copy the shape of the existing five exactly: `f.t.Helper()`, then `f.t.Fatalf("unexpected call to workItemAPI.GetAttachmentContent")` when the field is nil. Add `io` to the imports.

    **`internal/ado/attachments.go` (new)**: the imports follow the house grouping (stdlib, then SDK and `github.com/google/uuid`, then no internal packages). The file declares two functions.

    `attachmentPath(name, destDir string) (string, error)`. Its doc comment cites D-08 and says why it is a two-use extraction: the download now, and the deferred CLI verb later. The guard is the feature, so a reviewer should not flag it under the rule of three. In order, it must:
    1. Reject a non-absolute `destDir` (`filepath.IsAbs`).
    2. Reject a name containing a NUL byte.
    3. Take `filepath.Base(name)`, and reject `.`, `..` and any absolute result.
    4. Join the base onto `filepath.Clean(destDir)`, and reject a joined path whose `filepath.Dir` is not exactly the cleaned `destDir`. This is D-03's post-join check in its exact-child form.
    5. Return the joined path.

    Every error message formats the server-supplied name with `%q`, so control characters cannot reach the terminal.

    `func (c *Client) DownloadAttachment(ctx context.Context, id uuid.UUID, destDir, name string) error`. Its doc comment cites D-01, D-03, D-04 and D-07: the SDK owns host and auth, the guard runs before any request, there is no byte cap, and a refusal is a plain error. In order, it must:
    1. Call `attachmentPath`, and return its error before any network call.
    2. Call `c.workitemClient.GetAttachmentContent(ctx, workitemtracking.GetAttachmentContentArgs{Id: &id})` and return any error. The SDK maps a non-2xx status to an error, so no status check remains.
    3. `defer body.Close()`.
    4. Call `os.MkdirAll(destDir, 0o700)`, then `os.Create(path)`.
    5. Copy with `io.Copy`. If the copy fails, close the file, `os.Remove` the path, and return the copy error.
    6. Close the file and return its error. If the close fails, `os.Remove` the path first. Do not `defer out.Close()`, because that drops the error that reports a failed flush.

    The file must not import `net/http` and must not log a URL.

    **`internal/ado/workitems.go`**:
    - Delete the URL-based `DownloadAttachment` at :611-643 outright. Do not leave a copy or a wrapper.
    - Add `ID uuid.UUID` with tag `json:"id"` as the first field of `Attachment`. Extend its doc comment: `ID` is the only identifier the download accepts (D-01), and `URL` is informational only (D-05).
    - In the extraction block at :475-497, keep `attachment.URL = *rel.Url`. Also parse the URL with `url.Parse`, take `path.Base(u.Path)`, and `uuid.Parse` it into `attachment.ID`. Name the parsed URL `u`, not `url`, so it does not collide with the package name. Use `path.Base` rather than `filepath.Base`, because URL paths are slash-separated.
    - Change the append guard at :492 to require both a non-empty name and `attachment.ID != uuid.Nil` (D-06: silent suppression, no failure, no log line).
    - Fix the imports as the compiler reports them. `fmt`, `io`, `net/http`, `os` and `path/filepath` become unused. `net/url`, `path` and `github.com/google/uuid` are new. Leave `GetChildWorkItems` alone: its local `url` variable at :530 legally shadows the package name inside that function only.

    **`internal/ado/json_test.go`**: change the `Attachment` key set at :149 to `id`, `name`, `url`, `size`.

    **`internal/ui/workitems/detail.go`**:
    - Delete the `Path` field from `AttachmentDownloadedMsg`. Nothing reads it; the handler at :106-112 reads only `Name` and `Err`.
    - In `downloadAttachment`, stop discarding the `os.UserHomeDir()` error. On error, return `AttachmentDownloadedMsg{Name: att.Name, Err: err}`.
    - Build `destDir` as the home directory joined with `.config`, `adoBizInt`, `attachments`. Call `client.DownloadAttachment(context.Background(), att.ID, destDir, att.Name)` and return `AttachmentDownloadedMsg{Name: att.Name, Err: err}`.

    The handler is unchanged. A containment refusal therefore renders through the existing `"Error: " + msg.Err.Error()` line (D-07). The `context.Background()` is now bounded by the SDK connection's `Timeout` (TRUST-04, resolved via D-01).

    Do not add a CLI verb (D-02) and do not add a byte cap (D-04).
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test -race ./internal/ado/... ./internal/ui/workitems/... && go test ./internal/ado -run 'TestDownloadAttachmentFetchesByID|TestJSONFieldNames' -v</automated>
    <fails_when>non-zero exit, or the -v output lacks a `--- PASS: TestDownloadAttachmentFetchesByID` line, or it contains `no tests to run`</fails_when>
  </verify>
  <acceptance_criteria>
    - `go doc ./internal/ado Client.DownloadAttachment` prints `func (c *Client) DownloadAttachment(ctx context.Context, id uuid.UUID, destDir, name string) error`.
    - `go doc ./internal/ado Attachment` lists the field `ID uuid.UUID` with tag `json:"id"`.
    - `internal/ado/attachments.go` exists and declares both `func attachmentPath(` and `func (c *Client) DownloadAttachment(`. `internal/ado/workitems.go` declares neither.
    - `TestDownloadAttachmentFetchesByID` and `TestJSONFieldNames` pass under `-race`.
    - `go build ./...` succeeds with `internal/ui/workitems/detail.go` passing `att.ID` to `DownloadAttachment`. `AttachmentDownloadedMsg` has exactly two fields, `Name` and `Err`.
    - `TestDispatchTableMatchesD02` still passes with no edit to `cmd/adoBizInt/main_test.go` (D-02).
    - `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>A TUI attachment download goes by GUID through the SDK, and the old URL path no longer exists. Committed.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Pin containment. Hostile names refuse before any fetch, and a failed copy leaves no file</name>
  <files>internal/ado/attachments_test.go</files>
  <read_first>
    - internal/ado/attachments.go and internal/ado/attachments_test.go (as written in Task 1)
    - internal/ado/fakes_test.go:44-60 (the nil-closure-is-the-assertion idiom)
    - cmd/adoBizInt/write_test.go:357-387 (`cases := map[...]` table-loop style to mirror)
    - .planning/phases/05-hardening/05-RESEARCH.md § Pitfall 2 (the probed hostile-name table) and Pitfall 7
  </read_first>
  <behavior>
    - `TestAttachmentPath`, with `destDir := t.TempDir()`:
      - Exact outcomes on every OS: `Spec.txt` → `destDir/Spec.txt`; `../../.ssh/authorized_keys` → `destDir/authorized_keys`; `/etc/passwd` → `destDir/passwd`; `../`, `..`, `.`, empty, `a` + NUL + `b`, and `///` → error; a relative `destDir` (`attachments`) with name `Spec.txt` → error.
      - OS-dependent rows (`..\`, `..\..\evil`, three backslashes) assert the invariant only.
      - Every row asserts the invariant: `err != nil` or `filepath.Dir(got) == filepath.Clean(destDir)`.
    - `TestDownloadAttachmentRefusesBeforeFetch`: for each rejecting name, `DownloadAttachment` returns a non-nil error, and `os.ReadDir(destDir)` is empty afterwards. `GetAttachmentContentFunc` is left nil, so any fetch fails the subtest by name.
    - `TestDownloadAttachmentRemovesPartialFile`: the fake body yields `part`, then returns an injected error from `Read`. `DownloadAttachment` returns that error (`errors.Is`), `os.Stat(destDir/name)` reports not-exist, and the fake body's `Close` was called.
  </behavior>
  <action>
    Add the three tests to `internal/ado/attachments_test.go`, table-driven where there is more than one input, in the `write_test.go:357-387` map-and-`t.Run` style.

    For the OS-dependent rows, record in a one-line comment why they assert containment only. On Linux and macOS a backslash is a literal file-name byte. On Windows `filepath.Base` strips it. Either answer satisfies HARD-02 (RESEARCH OQ3, accepted as D-03 is written).

    For the refusal test, construct `&Client{workitemClient: &fakeWorkItemAPI{t: t}}` with no closure set. The fake's own `t.Fatalf` is the "no network call" assertion (D-08 integration half). Do not add a call counter.

    For the partial-file test, declare one small test-local `io.ReadCloser` type that serves a fixed prefix, then an error, and records `Close`. This is single-use, so keep it inside the test file. Use a sentinel error value so the assertion is `errors.Is`, not a message match.

    If a test exposes a gap in `attachmentPath` or `DownloadAttachment`, fix it in `attachments.go` and note the fix in the summary. The Task 1 implementation is expected to pass as written.
  </action>
  <verify>
    <automated>go test -race ./internal/ado -run 'TestAttachmentPath|TestDownloadAttachmentRefusesBeforeFetch|TestDownloadAttachmentRemovesPartialFile' -v</automated>
    <fails_when>non-zero exit, or `no tests to run`, or fewer than three `--- PASS: TestAttachmentPath|TestDownloadAttachmentRefusesBeforeFetch|TestDownloadAttachmentRemovesPartialFile` top-level PASS lines</fails_when>
  </verify>
  <acceptance_criteria>
    - All three test functions exist in `internal/ado/attachments_test.go` and pass under `-race`.
    - `TestAttachmentPath` has at least 13 rows covering every input named in `<behavior>`, and asserts the containment invariant on each.
    - In `TestDownloadAttachmentRefusesBeforeFetch`, `GetAttachmentContentFunc` is never assigned (the fake field stays nil).
    - Temporarily moving the `attachmentPath` call below the `GetAttachmentContent` call makes `TestDownloadAttachmentRefusesBeforeFetch` fail. Revert after confirming.
  </acceptance_criteria>
  <done>Every D-08 hostile name is refused before a request leaves, and a failed copy cannot masquerade as a download.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Keep raw HTTP and the credential-bearing connection out of internal/ado structurally, and pin D-06</name>
  <files>internal/ado/client.go, internal/ado/attachments_test.go</files>
  <read_first>
    - internal/ado/client.go (the `Client` doc comment at :19-25, which mentions "plus a live connection"; the `connection` field at :27; `newConnection` at :44-62; the `connection: connection,` assignment at :125)
    - internal/ado/client_test.go (both `newConnection` tests stay untouched)
    - internal/ado/preread_test.go:272-330 (go/ast scaffolding, the `seen` non-vacuity check, and the doc-comment rationale to copy)
    - internal/ado/workitems.go extraction block (as changed in Task 1)
    - .planning/phases/05-hardening/05-RESEARCH.md § Pattern 5 and Pitfall 10
  </read_first>
  <behavior>
    - `TestNoRawHTTPInADO` parses every non-test `.go` file in the package directory and fails if any of the following holds:
      - an import path is `net/http`;
      - any selector's `Sel.Name` is `AuthorizationString`;
      - any struct field's type is a pointer to the selector `azuredevops.Connection`;
      - the parsed-file set does not include `attachments.go` (non-vacuity).
    - `TestGetWorkItemDetailAttachmentID` feeds `GetWorkItemDetail` six relations and expects exactly one attachment, the valid one, with its parsed ID, name, URL and size:
      1. a valid GUID URL with `?fileName=`;
      2. a URL whose last segment is not a GUID;
      3. a nil URL;
      4. a valid GUID with an empty name;
      5. the all-zero GUID;
      6. a non-`AttachedFile` relation.

      The call also returns a nil error.
  </behavior>
  <action>
    **`internal/ado/client.go`** (RESEARCH OQ4, a direct consequence of D-01): delete the `connection *azuredevops.Connection` field and the `connection: connection,` line in the `NewClient` return literal. Reword the struct doc comment so it no longer claims the struct holds "a live connection". Keep the sentence about the deliberate absence of JSON tags.

    Keep `newConnection`, the local `connection` variable in `NewClient`, and the `azuredevops` import; `newConnection` still returns `*azuredevops.Connection`, and every sub-client is still built from it. `workClient` stays; its comment says it must not be removed as dead code.

    **`internal/ado/attachments_test.go`**: add `TestNoRawHTTPInADO`. Enumerate the package's non-test files with `filepath.Glob("*.go")`, skipping names ending in `_test.go`; the test binary's working directory is the package directory. Do not use the deprecated `parser.ParseDir`. For each file, call `parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)`. Check `file.Imports` for `net/http`, and walk the file with `ast.Inspect` for the selector and the struct-field shape in `<behavior>`. Report each violation with `fset.Position`.

    Copy the doc-comment rationale from `preread_test.go:283-288`: comments and string literals are not AST nodes, so neither can satisfy or defeat the assertion (the three Phase 3 grep-defeat incidents in STATE.md). Add one sentence tying it to ROADMAP SC1 ("the PAT cannot reach an arbitrary host") and to OQ4.

    Add `TestGetWorkItemDetailAttachmentID` in the same file. It drives `GetWorkItemDetail` through `GetWorkItemFunc` with the six relations in `<behavior>`, and asserts the single surviving attachment with `reflect.DeepEqual` against a literal `[]Attachment` (D-05, D-06).
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test -race ./internal/ado -run 'TestNoRawHTTPInADO|TestGetWorkItemDetailAttachmentID|TestNewClientSetsTimeout|TestNewConnectionLeavesTimeoutUnsetForZeroBudget' -v && go test -race ./...</automated>
    <fails_when>non-zero exit, or `no tests to run`, or the -v output lacks `--- PASS: TestNoRawHTTPInADO` or `--- PASS: TestGetWorkItemDetailAttachmentID`</fails_when>
    <human-check>
      <test>Run this once all three tasks have landed. Build with `go build -o adoBizInt ./cmd/adoBizInt`, run `./adoBizInt` (TUI), open a work item that has a known attachment, select the attachment and trigger the download.</test>
      <expected>The modal shows `Downloaded: <name>`. `~/.config/adoBizInt/attachments/<name>` exists with contents byte-identical to the file downloaded from the ADO web UI. `ls -ld ~/.config/adoBizInt/attachments` shows `drwx------` if the directory was created by this run.</expected>
      <why_human>Needs the operator's own PAT and a real attachment. CI has no credentials by design (D-17). Never paste the PAT into the session. The check sits on this `auto` task, not on the Task 1 tracer, because the end-of-phase harvest collects human-check blocks only from `auto` tasks. As the last task, it also exercises the final client, with `connection` removed.</why_human>
    </human-check>
  </verify>
  <acceptance_criteria>
    - `go doc -u ./internal/ado Client` lists no field named `connection`.
    - `TestNoRawHTTPInADO` passes, and it fails when a scratch `import _ "net/http"` is temporarily added to `internal/ado/attachments.go`. Revert after confirming; this proves the test is not vacuous.
    - `TestGetWorkItemDetailAttachmentID` passes with exactly one surviving attachment.
    - `TestNewClientSetsTimeout` and `TestNewConnectionLeavesTimeoutUnsetForZeroBudget` pass unchanged.
    - `go test -race ./...` exits 0 and `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>A future raw-HTTP path, a PAT header read, or a credential-bearing struct field in `internal/ado` fails the build's tests.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| ADO server → `GetWorkItemDetail` | Relation URLs and attachment names are server-supplied and untrusted |
| `internal/ado` → Azure DevOps REST | The PAT crosses here, and only through the SDK connection built from the configured org URL |
| `DownloadAttachment` → local filesystem | A server-supplied name becomes a file path under the operator's home |
| `internal/ado` error → TUI terminal | An error string carrying a server-supplied name is rendered in the terminal |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-05-01 | Information Disclosure | `DownloadAttachment` sending the PAT to a relation-URL host (C3) | high | mitigate | ID-only signature (D-01). The SDK's `GetAttachmentContent` resolves the org host and adds the header. `TestNoRawHTTPInADO` forbids `net/http`, `AuthorizationString` reads and a `*azuredevops.Connection` struct field in `internal/ado`. |
| T-05-02 | Tampering / Elevation of Privilege | Attachment name → file path (H6) | high | mitigate | `attachmentPath`: `IsAbs(destDir)`, NUL reject, `filepath.Base`, `.`/`..`/absolute reject, exact-child post-join check. All run before any network call. Pinned by `TestAttachmentPath` and `TestDownloadAttachmentRefusesBeforeFetch`. |
| T-05-03 | Tampering | Truncated file left after a mid-body timeout | low | mitigate | Copy or close failure removes the file. Pinned by `TestDownloadAttachmentRemovesPartialFile`. |
| T-05-04 | Denial of Service | Unbounded attachment size fills the disk | low | accept | D-04: human-initiated, one-at-a-time downloads from the operator's own org. The cap is deferred with the CLI verb. |
| T-05-05 | Information Disclosure | A redirect carrying the PAT to blob storage | medium | accept | The SDK uses a default `http.Client`. Per `go doc net/http.Client`, `Authorization` is dropped on a redirect to a non-matching domain (RESEARCH § SDK facts). |
| T-05-06 | Elevation of Privilege | Symlink planted inside the attachments directory | low | accept | Requires local write access to `~/.config/adoBizInt`, which is outside this threat model. `os.Root` is recorded for the deferred CLI verb. |
| T-05-07 | Tampering | Control characters in a server-supplied name reaching the terminal through an error | low | mitigate | Every `attachmentPath` error formats the name with `%q`. |
| T-05-SC | Tampering | npm/pip/cargo installs | high | mitigate | Not applicable. No package install occurs. `github.com/google/uuid` v1.1.1 is already in `go.sum` and pinned by the SDK (RESEARCH § Package Legitimacy Audit). |
</threat_model>

<verification>
- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- `go test ./internal/ado -run 'Attachment|NoRawHTTP|JSONFieldNames' -v` shows every named test passing.
- End of phase: the Task 3 human-check (a real attachment downloaded through the TUI) is harvested into UAT.
</verification>

<success_criteria>
- ROADMAP Phase 5 SC1 and SC2 hold. The PAT reaches only the SDK-resolved org host, and every attachment write is a direct child of the attachments directory.
- The 05-VALIDATION.md rows for HARD-01 and HARD-02 each have a green named test.
- The live TUI download is queued in UAT as the Task 3 human-check.
</success_criteria>

<output>
Create `.planning/phases/05-hardening/05-01-SUMMARY.md` when done
</output>
