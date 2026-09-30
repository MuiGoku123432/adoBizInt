---
phase: 05-hardening
reviewed: 2026-09-30T17:49:05Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - .github/workflows/ci.yml
  - cmd/adoBizInt/audit_test.go
  - cmd/adoBizInt/filter_test.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/main_test.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/patleak_test.go
  - cmd/adoBizInt/usage_test.go
  - cmd/adoBizInt/write_test.go
  - cmd/adoBizInt/writeverbs_test.go
  - internal/ado/attachments_test.go
  - internal/ado/attachments.go
  - internal/ado/client.go
  - internal/ado/dashboard.go
  - internal/ado/fakes_test.go
  - internal/ado/interfaces.go
  - internal/ado/json_test.go
  - internal/ado/nilguard_test.go
  - internal/ado/pipelines.go
  - internal/ado/releases.go
  - internal/ado/wiql_test.go
  - internal/ado/workitems.go
  - internal/logging/logging_test.go
  - internal/logging/logging.go
  - internal/ui/workitems/detail.go
findings:
  critical: 1
  warning: 6
  info: 7
  total: 14
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-09-30T17:49:05Z
**Depth:** standard
**Files Reviewed:** 25
**Status:** issues_found

## Summary

I reviewed the Phase 5 hardening diff (`33a0ebf^..HEAD`) at standard depth: attachment download by GUID through the SDK, containment in `attachmentPath`, the `@project` WIQL macro with `iterationClause`, the `--project` allow-list, owner-only log modes, the PAT-leak tripwire, nil-Id guards and the CI workflow. `go build`, `go vet`, `go test -race ./...` and `gofmt -l .` all pass locally. I checked both pinned action SHAs (`actions/checkout@3d3c42e…` = v7.0.1, `actions/setup-go@b7ad1da…` = v7.0.0) against the upstream tags, and they match.

Several things hold up. The raw-HTTP PAT path is gone. The two WIQL sites no longer interpolate the project. Server-supplied iteration paths are quote-escaped in one place. The nil-Id guards cover both collecting loops. The log directory and file are tightened on every `Init`.

The main problem is that the new `--project` allow-list protects less than its comments say. `workitems set-state` checks `--project` and then never uses it. The mutation lands on whatever project owns the work item ID, and the audit record names the project the caller typed. The attachment download has three smaller problems: it overwrites and then deletes files with the same name, it creates files with world-readable modes, and the per-request HTTP timeout now caps every download at the whole-request budget. Two of the new structural and leak tests cannot fail for the cases their doc comments say they guard.

Known deferrals confirmed and not re-reported: `cmd/adoBizInt/audit.go:77,80` still uses `MkdirAll 0755` / `OpenFile 0644` (T-05-26). Because `logging.Init` now makes the parent directory 0700, other users cannot traverse to the file. `--project P,P` duplicates still pass the allow-list, and write verbs reject them only through `oneProject` arity.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: The `--project` allow-list does not scope `workitems set-state`, so a write lands on an unconfigured project and the audit record names the wrong one

**File:** `cmd/adoBizInt/flags.go:29-31` (claim), `cmd/adoBizInt/write.go:285-323` (gap), `internal/ado/workitems.go:264-289`
**Issue:** The new allow-list comment says: "a near-miss must exit 3 rather than land a read or a write on a project the caller did not name exactly." For `workitems set-state` that does not happen. `resolve` validates `--project` and stores it in `writeTarget.Project`. After that, `plan` calls `c.GetWorkItemDetail(ctx, t.ID)` and `mutate` calls `UpdateWorkItemState(ctx, t.ID, …)` / `UpdateWorkItem` with no `Project`, and `GetWorkItemDetail` never fills `detail.Project` (no `System.TeamProject` read). Work item IDs are unique across the whole org. So `adoBizInt workitems set-state 999 --project P --state Closed --confirm` changes item 999 even when it belongs to project Z, which is not configured. The audit line and the result envelope then record `"project": "P"` (write.go:54, 568). That makes the audit trail wrong about which project was changed, which matters for SOX-style change evidence. The other five write verbs send the project in the SDK route, so this gap is specific to set-state. The gap existed before Phase 5, but this phase added the allow-list whose guarantee it breaks.
**Fix:** Have the pre-read establish which project owns the item, and refuse a mismatch before any mutation. Pass the project in the mutation route as well:
```go
// workitems.go, GetWorkItemDetail: populate the owning project
if p, ok := itemFields["System.TeamProject"].(string); ok {
	detail.Project = p
}

// write.go, setStateOp.plan: refuse a cross-project target (exit 3, nothing ran)
if detail.Project != t.Project {
	return nil, fmt.Errorf("work item %d belongs to project %q, not %q", t.ID, detail.Project, t.Project)
}
```
Also thread `t.Project` into `UpdateWorkItemState` / `UpdateWorkItemStateWithEffort` as `UpdateWorkItemArgs.Project`. Add a `TestUnknownProjectRejected`-style row where the pre-read fake returns an item from another project and `UpdateWorkItemStateFunc` is nil.

## Warnings

### WR-01: A download overwrites a same-named attachment, and a failed download then deletes the earlier good copy

**File:** `internal/ado/attachments.go:69-81`
**Issue:** `os.Create(path)` truncates any existing file at `destDir/<base>`. ADO work items often carry several attachments with the same name, for example every pasted screenshot is `image.png`. Downloading the second one silently replaces the first, and the UI reports `Downloaded: image.png` both times. The remove-on-failure path added this phase makes it worse. If the second download fails mid-body, `os.Remove(path)` deletes the file that `os.Create` had already truncated, so the earlier complete download is gone and nothing replaces it. `TestDownloadAttachmentRemovesPartialFile` only covers the empty-directory case.
**Fix:** Write to a temp file in the same directory, and rename it into place only on success. Refuse or uniquify a name that already exists:
```go
tmp, err := os.CreateTemp(destDir, ".download-*")
if err != nil { return err }
defer os.Remove(tmp.Name()) // no-op after a successful rename
if _, err := io.Copy(tmp, body); err != nil { tmp.Close(); return err }
if err := tmp.Close(); err != nil { return err }
// Refuse to clobber (or pick "name (1).ext"): os.Link+Remove, or Stat-then-Rename.
return os.Rename(tmp.Name(), path)
```

### WR-02: Downloaded attachments are world-readable, and an old 0755 attachments directory is never tightened

**File:** `internal/ado/attachments.go:65-71`
**Issue:** `os.Create` uses mode 0666 & ~umask, which is 0644 under the usual 022 umask. `os.MkdirAll(destDir, 0o700)` applies 0700 only when it creates the directory. Every existing install already has `~/.config/adoBizInt/attachments` at 0755 from the deleted implementation (`MkdirAll(…, 0755)`). `logging.Init` makes the same point about mode-at-creation in its own doc comment, but the attachment path repeats the mistake. Right now the files are protected only because `logging.Init` in another package makes the parent 0700. If that `Init` fails or is skipped (D-21's non-fatal path), attachment contents are world-readable. Attachments can carry customer data (CPNI), so defense in depth matters here.
**Fix:**
```go
if err := os.MkdirAll(destDir, 0o700); err != nil { return err }
if err := os.Chmod(destDir, 0o700); err != nil { return err }
out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // or CreateTemp (0600) per WR-01
```

### WR-03: The per-request HTTP timeout now caps every attachment download (30s by default), and a zero timeout lets it hang with no way to cancel

**File:** `internal/ado/attachments.go:62`, `internal/ado/client.go:54-58`, `internal/ui/workitems/detail.go:193`
**Issue:** `GetAttachmentContent` runs on the SDK's `http.Client`, whose `Timeout` is `cfg.RequestTimeout` (default `30 * time.Second`, `config.go:87`). `http.Client.Timeout` covers the whole exchange, including reading the body. So any attachment that takes more than 30s to stream fails partway through, and the partial file is deleted. For example, a 100 MB log at 2 MB/s always fails. The deleted implementation used `http.DefaultClient` with no deadline, so this is a functional regression for large files. The `failingBody` test comment ("the way a connection timeout interrupts a body mid-read") shows the case was anticipated but not handled. The opposite case is also a problem. With `request_timeout: 0`, `newConnection` leaves `Timeout` nil, and `detail.go` passes `context.Background()`. A stalled download then never returns, `m.downloading` stays `true`, and every later `d` keypress is ignored until restart. Closing the modal does not cancel it.
**Fix:** Pick an explicit download budget that is separate from the per-request budget, and make it cancellable. For example, pass a context from the model with `context.WithTimeout(ctx, downloadBudget)` and cancel it on `CloseDetailMsg`. Because the SDK bakes `Timeout` into the shared client at construction, a large-download path needs either a second SDK connection built without `Timeout` (context-bounded only) or a documented size ceiling that fits within `RequestTimeout`.

### WR-04: The server-controlled attachment name reaches the terminal and the filesystem with control characters intact

**File:** `internal/ado/attachments.go:21-31`, `internal/ui/workitems/detail.go:110,266`
**Issue:** `attachmentPath`'s doc says: "The name is formatted with %q in every error so control characters from the server cannot reach the operator's terminal." That holds only for the error strings. The success status `"Downloaded: " + msg.Name` (detail.go:110) and the attachment list `fmt.Sprintf("%s%s (%s)\n", prefix, att.Name, sizeStr)` (detail.go:266) render the raw name. Anyone who can attach a file to a work item in the org can therefore inject ANSI/OSC sequences into the operator's TUI, such as OSC 52 clipboard writes, OSC 8 links or screen rewrites. The guard also rejects only NUL, so ESC, CR, LF and other C0/C1 bytes are written into on-disk filenames. The success message shows the unsanitized name (`../../.ssh/authorized_keys`) rather than the file actually written (`authorized_keys`). Since `Path` was dropped from `AttachmentDownloadedMsg`, the operator is never told where the file went. On Windows, a name such as `CON`, `NUL` or `a:stream` passes the guard and resolves to a device or an NTFS alternate data stream.
**Fix:** Reject or replace any rune for which `unicode.IsControl(r)` is true in `attachmentPath`. On Windows, also reject `:` and reserved device basenames. Return the resolved path from `DownloadAttachment` (`(string, error)`) and show that path, quoted, in the status line. Render names in the list through a sanitizer, for example `strings.Map` that drops control runes, or `strconv.Quote`.

### WR-05: `TestPATNeverLogged` cannot fail for the code that actually holds the PAT

**File:** `cmd/adoBizInt/patleak_test.go:32-92`, `cmd/adoBizInt/main.go:77-89`
**Issue:** The test drives `dispatch` with fakes. No value reachable from `dispatch` holds the PAT, and the doc comment says so. Setting `ADO_PAT` has no effect because nothing in the exercised path reads it. The only code that handles the credential is `config.Load`, the `"Config loaded"` log in `run()`, `ado.NewClient` / `newConnection`, and `cfg.Validate`'s error, and none of it runs. The most likely "careless slog attribute", such as `log.Info("Config loaded", "config", cfg)` in `main.go:85`, would print the PAT and still pass the test. Nothing structural blocks that either: `config.Config` has no `slog.LogValuer` / `String()` redaction, and `PAT` is a plain exported field. So HARD-06 is asserted only for code that never had the secret.
**Fix:** Make redaction structural and test it where the secret lives. Add `func (c Config) LogValue() slog.Value` (and `String()`) that omits or masks `PAT`. Add a test that runs `config.Load()` with `ADO_PAT=patFixture` and a temp `HOME`, logs the loaded config the way `run()` does, and asserts no form of the fixture appears. Alternatively, move the post-validation logging out of `run()` into a helper the test can call.

### WR-06: `TestWIQLInterpolationStructural` misses the split literals it says it catches

**File:** `internal/ado/wiql_test.go:159-178,198-215,275-284`
**Issue:** A chain counts as a site only when a single string literal contains `from workitems` (`isWIQLLiteral`). The doc comment says "a third WIQL site, or a split literal that hides one, must fail". A split literal does not. For example, `"SELECT [System.Id] FROM " + "WorkItems WHERE [System.TeamProject] = '" + project + "'"` has no literal with the full substring. It is not a site, it does not count toward `total` or `inSite`, and `sites` stays 2, so the test passes while the project is interpolated again. A `FROM WorkItemLinks` query, which is also WIQL and also project-scoped, is never matched either. This test replaced a code-review control for SQL-style injection (OWASP A03), so a guard that can be silently bypassed is a reliability problem.
**Fix:** Anchor on the sink rather than the text. Find every `QueryByWiqlArgs` composite literal, or every `QueryByWiql(` call, and require that its `Wiql.Query` resolves to an identifier assigned from a checked `+` chain. Alternatively, decide whether a chain is a site from the joined text of all its literal operands (`strings.Contains(strings.ToLower(joined), "from workitem")`), not from each literal alone. Add a count assertion on `QueryByWiql` call sites (currently 2) so a new query cannot bypass the check.

## Info

### IN-01: go.mod is untidy, and `github.com/google/uuid` is still marked `// indirect` even though code now imports it directly

**File:** `go.mod:26`
**Issue:** `internal/ado/attachments.go` and `workitems.go` now import `github.com/google/uuid` directly, but go.mod still lists it as `// indirect`. `go mod tidy -diff` reports a diff (this entry plus a pre-existing `atotto/clipboard` one). CI does not check tidiness, so the file will keep drifting.
**Fix:** Run `go mod tidy`. Optionally add a `go mod tidy -diff` step to `.github/workflows/ci.yml`.

### IN-02: The CI Go version is hard-coded separately from go.mod, and checkout keeps the token in `.git/config`

**File:** `.github/workflows/ci.yml:17-20`
**Issue:** `go-version: '1.25.4'` repeats the value from `go.mod:3`. D-16 ("matches go.mod") holds only as long as someone remembers to change both. `actions/checkout` defaults to `persist-credentials: true`, which writes the GITHUB_TOKEN into `.git/config` for later steps. The token is read-only under `permissions: contents: read`, but that still conflicts with D-17 "no credentials".
**Fix:** Use `with: { go-version-file: go.mod }` on setup-go and `with: { persist-credentials: false }` on checkout.

### IN-03: Shadowed identifiers in workitems.go

**File:** `internal/ado/workitems.go:485`, `internal/ado/workitems.go:268`
**Issue:** At line 485, `if id, err := uuid.Parse(...)` shadows `GetWorkItemDetail`'s `id int` parameter, which is logged again at line 507. At line 268, `path := "/fields/System.State"` now shadows the newly imported `path` package inside `UpdateWorkItemState`. Neither is a bug today, but both are traps for the next edit.
**Fix:** Rename them to `attID` and `fieldPath`.

### IN-04: A chmod failure now turns off file logging entirely

**File:** `internal/logging/logging.go:29-41`
**Issue:** `os.Chmod(logDir)` or `file.Chmod` now fails `Init` in cases where file logging used to work, for example a directory on a filesystem that rejects chmod, or a directory owned by another user from an earlier `sudo` run. `Logger()` then falls back to `slog.Default()` on stderr. In TUI mode that corrupts the alt screen, and routine lines such as `"Current user identity" … "email"` go to the terminal. This follows D-21 as designed, but it is a new failure mode worth recording.
**Fix:** Consider treating the chmod as best-effort with a single warning, keeping the successfully opened file as the sink. Alternatively, document the tradeoff in D-21.

### IN-05: The `--project` help text says "read" on write verbs

**File:** `cmd/adoBizInt/flags.go:199`
**Issue:** The rewritten registration keeps `"restrict the read to a project; repeatable or comma-separated"`. On the six write verbs, `--project` is required and exactly one value is allowed, so an agent reading `workitems set-state --help` is misled.
**Fix:** Use neutral wording such as "project to act on (must be configured); reads accept several, writes exactly one", or register separate usage strings by `writeSupportByCommand` membership.

### IN-06: `DownloadAttachment` accepts `uuid.Nil`

**File:** `internal/ado/attachments.go:55`
**Issue:** The exported method relies on its only caller having dropped Nil IDs (D-06 in `GetWorkItemDetail`). A future caller, such as the deferred CLI verb, that passes a zero-valued `Attachment.ID` sends a request for `00000000-…` instead of failing locally.
**Fix:** `if id == uuid.Nil { return fmt.Errorf("attachment has no ID") }` before `attachmentPath`.

### IN-07: Other config-directory files are still created with loose modes

**File:** `internal/config/priorities.go:59,70`
**Issue:** `priorities.yaml` is written 0644 and its directory is created 0755. This case is not in the listed deferrals, unlike `audit.go` (T-05-26). Today the parent directory being 0700 mitigates it, as with the audit file.
**Fix:** Use 0o700 / 0o600 for consistency, or add it to the T-05-26 deferral.

---

_Reviewed: 2026-09-30T17:49:05Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
