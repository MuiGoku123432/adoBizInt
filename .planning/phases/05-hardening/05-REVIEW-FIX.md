---
phase: 05-hardening
fixed_at: 2026-09-30T18:22:57Z
review_path: .planning/phases/05-hardening/05-REVIEW.md
iteration: 1
findings_in_scope: 7
fixed: 7
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-09-30T18:22:57Z
**Source review:** .planning/phases/05-hardening/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 7 (CR-01, WR-01 to WR-06; Info is out of scope)
- Fixed: 7
- Skipped: 0

**Where verification ran:** in the main checkout on branch `dev`, with no worktree. The orchestrator said no other agent was running. `.claude/worktrees/` is not gitignored in this repo, so a worktree would have left untracked files behind. After every fix, and again uncached (`-count=1`) on the final clean tree, `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"` passed. go.mod and go.sum were not touched.

**Commits:** plain `git commit` on explicit paths, one per finding. The `gsd_run query commit` wrapper was not used so that the required Co-Authored-By trailer could go in the message.

## Fixed Issues

### CR-01: The `--project` allow-list does not scope `workitems set-state`

**Files modified:** `internal/ado/workitems.go`, `cmd/adoBizInt/write.go`, `cmd/adoBizInt/dispatch.go`, `internal/ui/workitems/workitems.go`, plus tests (`cmd/adoBizInt/{write,audit,patleak,fakes}_test.go`, `internal/ado/workitems_test.go`, `internal/ui/workitems/workitems_test.go`)
**Commit:** 24233ac
**Status:** fixed: requires human verification (a logic and guard change)
**Applied fix:**
- `GetWorkItemDetail` now fills `detail.Project` from `System.TeamProject`.
- `setStateOp.plan` refuses when `detail.Project != t.Project`. The error reads `work item %d belongs to project %q, not %q`. The check runs inside `plan`, which comes after `auditOpen` but before the no-op check, the `--confirm` branch, `mutate` and the audit append. The refusal therefore reaches the existing pre-read failure path: exit 3, nothing on stdout, no audit record. No other exit code changed.
- An item whose project the server leaves out (an empty string) is refused as well, so the check fails closed.
- The comparison is exact and case-sensitive, to match the allow-list's documented rule (flags.go, assumption A2).
- `UpdateWorkItemState` and `UpdateWorkItemStateWithEffort` take `project` and send it as `UpdateWorkItemArgs.Project`. The SDK already drops an empty project from the route. The four TUI call sites pass `""`, so TUI behaviour is unchanged; routing TUI writes through `runWrite` stays deferred as BL-01 part 2.
- New tests: `TestSetStateRefusesWorkItemInAnotherProject` covers dry-run and `--confirm` with both projects configured and `UpdateWorkItemStateFunc` nil. `TestGetWorkItemDetailReportsOwningProject` covers the pre-read. The exact-patch tests now pin `Project` in the args, and the cmd fakes assert that the mutation receives `"P"`.
- RED was confirmed before the fix: the dry run exited 0, and the confirm path called the nil mutation.
- **Human check:** adding `Project` to the PATCH route has not been exercised against the live org. One live `set-state` dry run and one confirm should be done before the phase counts as done, the same bar D-10 set for `@project`.

### WR-01: A download overwrites a same-named attachment, and a failed download then deletes the earlier good copy

**Files modified:** `internal/ado/attachments.go`, `internal/ado/attachments_test.go`, `internal/ui/workitems/detail.go`
**Commit:** b6ff397
**Status:** fixed: requires human verification (a file-handling logic change)
**Applied fix:**
- `DownloadAttachment` writes to `os.CreateTemp(destDir, ".download-*")` and hard-links the finished file to the final name with `os.Link`. `os.Link` refuses an existing target, where `os.Rename` would replace it.
- A name that is already taken gets `name (1).ext`, `name (2).ext`, and so on, up to 100 attempts.
- The temp file is removed on every path.
- The signature is now `(string, error)` and returns the path actually written. The TUI status shows that path.
- New test `TestDownloadAttachmentNeverClobbers` covers a failed download that must keep the earlier copy, and a second success that must take the next free name. `TestDownloadAttachmentRemovesPartialFile` now also asserts that no temp file is left.
- Known limit: `os.Link` needs a filesystem with hard links. APFS, ext4 and NTFS all have them; FAT and some network mounts do not, and there the download returns an error instead of writing.

### WR-02: Downloaded attachments are world-readable, and an old 0755 attachments directory is never tightened

**Files modified:** `internal/ado/attachments.go`, `internal/ado/attachments_test.go`
**Commit:** c120077
**Applied fix:**
- `os.Chmod(destDir, 0o700)` now runs on every download, after `MkdirAll`, following the D-20 reasoning.
- The file gets 0600 from `CreateTemp` (part of WR-01), and the hard link keeps that mode.
- `TestDownloadAttachmentOwnerOnlyModes` starts from a 0755 directory and asserts 0700 on the directory and 0600 on the file. It is skipped on Windows. RED on the directory mode was confirmed; the file half already held after WR-01.

### WR-03: The per-request HTTP timeout caps every attachment download, and a zero timeout lets it hang with no way to cancel

**Files modified:** `internal/ui/workitems/detail.go`, `internal/ui/workitems/detail_test.go` (new test file), `internal/ado/attachments.go` (doc comment only)
**Commit:** 66a5e40
**Status:** fixed: requires human verification (a TUI state-handling change)
**Choice recorded:**
- The connection timeout stays. D-01 locks the download to the SDK connection and says it "inherits the Phase 2 `connection.Timeout`", which is how TRUST-04 was resolved. A second, timeout-free SDK connection would undo that reasoning, and the orchestrator ruled out removing the timeout for reads.
- The smallest change was taken instead: make the download cancellable, and document the ceiling.

**Applied fix:**
- On `d`, `DetailModel` creates a `context.WithCancel`, stores the cancel func in `cancelDownload`, and passes the context to `DownloadAttachment`.
- `esc`/`q` calls it before emitting `CloseDetailMsg`. `AttachmentDownloadedMsg` calls it to release the context.
- `DownloadAttachment`'s doc now says that `request_timeout` (default 30s) is the ceiling for one whole download, that raising it is how to fetch larger files, and that with `request_timeout: 0` the context is the only way to stop a stalled download.
- New test `TestCloseDetailCancelsDownload`. RED was confirmed as a compile failure, because the field did not exist yet.

**Not addressed, by design:**
- A large file still fails at `request_timeout`. The documented setting to change is `request_timeout`.
- A late `AttachmentDownloadedMsg` from a cancelled download can put a stale "Error: context canceled" status on a modal reopened straight afterwards. This is cosmetic.

### WR-04: The server-controlled attachment name reaches the terminal and the filesystem with control characters intact

**Files modified:** `internal/ado/attachments.go`, `internal/ado/attachments_test.go`, `internal/ui/workitems/detail.go`, `internal/ui/workitems/detail_test.go`
**Commit:** 74751ed
**Applied fix:**
- `attachmentPath` refuses any name that contains a rune for which `unicode.IsControl` is true. This replaces the NUL-only check, and NUL is still refused.
- On Windows it also refuses a `:` (NTFS alternate stream) and the reserved device stems: CON, PRN, AUX, NUL, COM1-9 and LPT1-9, with or without an extension.
- The TUI attachment list drops control runes with `strings.Map`.
- The success status is `Downloaded to %q` and shows the resolved path. Returning that path came with WR-01.
- New table rows cover ESC/OSC 52, CR, LF and C1 CSI, which must be refused, and device names and a stream colon, which must be refused on Windows and only contained elsewhere.
- New test `TestDetailNeverRendersRawControlBytes` checks only the unstyled attachment line, because lipgloss emits its own ANSI codes when a terminal is attached.
- The Windows rows only assert containment on this macOS host. `GOOS=windows go vet` and `go test -c` compile cleanly, but the Windows refusal branch has not been executed.

### WR-05: `TestPATNeverLogged` cannot fail for the code that actually holds the PAT

**Files modified:** `internal/config/config.go`, `internal/config/config_test.go`, `cmd/adoBizInt/patleak_test.go` (doc comment only)
**Commit:** 0e452dd
**Applied fix:**
- `config.Config` now has `LogValue()`, `String()` and `GoString()`, all value receivers. Each renders `org_url`, `projects` and `pat_set` and never the PAT. `user_email` is left out as PII.
- New test `TestLoadedConfigNeverRendersPAT` runs `config.Load()` with `ADO_PAT` set to a fixture and a temp `HOME`. It logs `"config", cfg` and `*cfg` through slog's text and JSON handlers, and formats both with `%v`, `%+v`, `%#v` and `%s`. It asserts the fixture never appears and that the org URL does, so the check is not vacuous.
- RED was confirmed: 9 renders leaked the PAT before the fix.
- `run()`'s existing "Config loaded" line was left alone, because it never held the PAT. `TestPATNeverLogged`'s doc now points to the config test for the half it cannot reach.

### WR-06: `TestWIQLInterpolationStructural` misses the split literals it says it catches

**Files modified:** `internal/ado/wiql_test.go`
**Commit:** 64bbcff
**Applied fix:**
- The walk moved into `checkWIQL(fset, files) (sites, sinks, problems)`.
- A `+` chain is now a site when its literals, joined, contain `from workitem`. This catches split literals and `FROM WorkItemLinks`.
- The check is anchored on the sink: every `Wiql{Query: &x}` must take `x` from exactly one `:=` whose right side is a checked site. A variable-built source, a `+=` after the site, or a `Wiql` with no `Query` fails.
- The `QueryByWiql` call count is pinned at 2, next to the existing site count of 2.
- New test `TestWIQLCheckCatchesBypasses` feeds parsed snippets for the split literal, WorkItemLinks, variable-source and `+=` bypasses, which must be reported, and for the real parameterized shape, which must be accepted. RED was confirmed: all four bypasses went unreported under the old logic.

## Skipped Issues

None. No fix contradicted a locked CONTEXT.md decision. For WR-03, the variant that would have contradicted D-01 (a second connection without a timeout) was rejected in favour of cancellation plus a documented ceiling, as described above.

---

_Fixed: 2026-09-30T18:22:57Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
