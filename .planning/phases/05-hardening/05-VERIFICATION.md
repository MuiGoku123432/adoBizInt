---
phase: 05-hardening
verified: 2026-09-30T18:30:54Z
status: passed
score: 5/6 roadmap success criteria verified in code (SC5 awaits an observed GitHub Actions run); 38/42 plan truths verified, 4 backstop truths routed to human
covered_files:

  - .github/workflows/ci.yml
  - .planning/REQUIREMENTS.md
  - .planning/phases/05-hardening/05-01-SUMMARY.md
  - .planning/phases/05-hardening/05-01-attachment-by-id-PLAN.md
  - .planning/phases/05-hardening/05-02-SUMMARY.md
  - .planning/phases/05-hardening/05-02-nil-guards-PLAN.md
  - .planning/phases/05-hardening/05-03-SUMMARY.md
  - .planning/phases/05-hardening/05-03-ci-workflow-PLAN.md
  - .planning/phases/05-hardening/05-04-SUMMARY.md
  - .planning/phases/05-hardening/05-04-project-allow-list-PLAN.md
  - .planning/phases/05-hardening/05-05-SUMMARY.md
  - .planning/phases/05-hardening/05-05-wiql-project-macro-PLAN.md
  - .planning/phases/05-hardening/05-06-SUMMARY.md
  - .planning/phases/05-hardening/05-06-log-permissions-pat-test-PLAN.md
  - cmd/adoBizInt/audit_test.go
  - cmd/adoBizInt/dispatch.go
  - cmd/adoBizInt/fakes_test.go
  - cmd/adoBizInt/filter_test.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/main_test.go
  - cmd/adoBizInt/patleak_test.go
  - cmd/adoBizInt/usage_test.go
  - cmd/adoBizInt/write.go
  - cmd/adoBizInt/write_test.go
  - cmd/adoBizInt/writeverbs_test.go
  - internal/ado/attachments.go
  - internal/ado/attachments_test.go
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
  - internal/ado/workitems_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/logging/logging.go
  - internal/logging/logging_test.go
  - internal/ui/workitems/detail.go
  - internal/ui/workitems/detail_test.go
  - internal/ui/workitems/workitems.go
  - internal/ui/workitems/workitems_test.go

covered_digest: "v1:sha256:0cf70660f79e99eedd528ff5978be74fd6622707856c3376858c1bd4ed7c2720"
behavior_unverified: 0
overrides_applied: 0
deferred:

  - truth: "audit.jsonl is created 0644 inside the config directory (cmd/adoBizInt/audit.go:77,80)"
    addressed_in: "Known deferral T-05-26 (not a later roadmap phase)"
    evidence: "05-06 plan truth: file mode deferred; parent directory is now 0700 so other local users cannot traverse to it"
  - truth: "--project P,P duplicates pass the allow-list"
    addressed_in: "Pre-existing, known deferral"
    evidence: "Write verbs still reject duplicates through oneProject arity; reads de-duplicate nothing but query only configured projects"
  - truth: "Review info items IN-01..IN-07 (go.mod tidy, go-version-file/persist-credentials, shadowed identifiers, chmod-failure fallback, --project help wording, uuid.Nil guard, priorities.yaml 0644)"
    addressed_in: "Known deferral (review info, out of fix scope)"
    evidence: "05-REVIEW-FIX.md: Info is out of scope"
human_verification:

  - test: "PHASE-BLOCKING (05-03 T2). Push dev to origin and open the Actions tab for the pushed HEAD commit."
    expected: "The `ci` run is green with build, vet, `go test -race` and gofmt steps all passing. Optional: a throwaway branch with an unformatted .go file goes red at the gofmt step and names the file. Also confirm by eye that the run used no repository secret (D-17 prohibition has no committed test)."
    why_human: "GitHub Actions is an external service; dev is 220 commits ahead of origin/dev and has not been pushed."
  - test: "PHASE-BLOCKING (05-05 T2). With your own PAT on this machine only, build a binary at the 05-05 base commit and one at HEAD, and for one configured project run `workitems list --project P --json` and `dashboard counts --project P --json` with each."
    expected: "Item count and dashboard work_items count are equal between the two binaries and both envelopes report complete: true."
    why_human: "`@project` resolution from the route has only been exercised against fakes; confirming it against the live organization needs a real PAT, which CI and this session do not have."
  - test: "05-01 T3. Run the TUI, open a work item with a known attachment, select it and press `d`."
    expected: "Status shows `Downloaded to \"<path>\"`; the file under ~/.config/adoBizInt/attachments is byte-identical to the ADO web UI download; the directory is drwx------ and the file -rw-------. Downloading the same attachment again produces `<name> (1).<ext>` rather than overwriting. Pressing esc during a slow download closes the modal without hanging."
    why_human: "Needs a real attachment and the live SDK GetAttachmentContent route; visual TUI state."
  - test: "CR-01 fix. Against a configured project P, run `workitems set-state <id> --project P --state <next>` (dry run), then the same with `--confirm`, for an item that belongs to P. Then run the dry run for an item that belongs to a different project."
    expected: "Dry run shows the current -> proposed diff and exits 0; --confirm changes the state in ADO and writes one audit record naming P. The cross-project item is refused with `work item N belongs to project \"Z\", not \"P\"`, exit 3, no audit record. Also confirm the configured project name matches ADO's canonical casing for System.TeamProject exactly (the comparison is case-sensitive, so a config entry `kinetic` against a server value `Kinetic` would refuse every set-state)."
    why_human: "Sending Project in the PATCH route and reading System.TeamProject from the live response have only been exercised against fakes."
---

# Phase 5: Hardening Verification Report

**Phase Goal:** The security findings that a CLI makes reachable are closed
**Verified:** 2026-09-30T18:30:54Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

Every finding the phase set out to close is closed in the code on HEAD (592675d), and the post-review fixes CR-01 and WR-01..WR-06 hold. What is left needs a person: one green GitHub Actions run, and three live checks against the organization that no fake can stand in for.

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Attachment download takes an identifier, not a URL, and the PAT cannot reach an arbitrary host | VERIFIED | `internal/ado/attachments.go:96` signature is `(ctx, id uuid.UUID, destDir, name string) (string, error)`; the only fetch is `c.workitemClient.GetAttachmentContent(ctx, GetAttachmentContentArgs{Id: &id})` (line 102). A repo-wide grep of non-test Go for `net/http`, `AuthorizationString`, `http.NewRequest` and `http.DefaultClient` returns nothing. `Client` holds no `*azuredevops.Connection` field. The SDK builds the route from the configured org (`workitemtracking/client.go:826-844`). `TestNoRawHTTPInADO` and `TestDownloadAttachmentFetchesByID` pass. The only caller is `internal/ui/workitems/detail.go:208`, which passes `att.ID`. The ID comes only from `uuid.Parse(path.Base(u.Path))` (workitems.go:499), and an attachment whose ID is Nil is not offered (line 514). |
| 2 | Attachment writes cannot escape the intended directory | VERIFIED | `attachmentPath` (attachments.go:33) requires an absolute destDir and refuses control runes, `.`, `..`, empty and absolute names, plus Windows devices and stream colons. It applies `filepath.Base` and checks after the join that `filepath.Dir(path) == dir`. It runs before any network call. The " (n)" suffix candidates come from the contained path, so they stay in the directory. `TestAttachmentPath`, `TestDownloadAttachmentRefusesBeforeFetch`, `TestDownloadAttachmentNeverClobbers` and `TestDownloadAttachmentOwnerOnlyModes` pass. |
| 3 | The nil-dereference sites in CONCERNS.md MED4 are guarded | VERIFIED | `pipelines.go:157` has `if d.Name == nil \|\| d.Id == nil`, which makes the later `*def.Id` at line 171 safe. `releases.go:104` has `if rd.Name == nil \|\| rd.Id == nil`, which does the same for line 125. The third `*d.Id` at pipelines.go:327 sits inside an existing `if d.Id != nil`. `TestGetPipelineRunsSkipsNilID`, `TestGetReleasesSkipsNilID` and the structural `TestCollectedDefinitionsGuardNameAndID` (exactly 2 sites) pass. |
| 4 | WIQL queries take the project as a parameter through `@project`, with no project text interpolated, and `--project` is validated against configured projects | VERIFIED | Both sites (workitems.go:148, dashboard.go:136) use `[System.TeamProject] = @project`. The only concatenated operand is `iterationClause(iterations)`, which doubles single quotes (client.go:289). `Project: &project` stays on `QueryByWiqlArgs`, and the SDK puts it in the route (client.go:2646). Log lines carry only `project` and a count of `iterations`. Allow-list: `fs.Func("project", ...)` in the single `newFlagSet` (flags.go:198) checks against `configuredProjects` exactly and case-sensitively, and fails closed when the list is nil. `run()` assigns the list after `cfg.Validate` (main.go:106). A rejection makes `fs.Parse` fail, which maps to exit 3 in both `runRead` (dispatch.go:244-250) and `runWrite` (write.go:140-146). CR-01: set-state now refuses when `detail.Project != t.Project` (write.go:320) and sends `Project` in the PATCH route (workitems.go:282). Passing tests: `TestGetWorkItemsUsesProjectMacro`, `TestDashboardUsesProjectMacro`, `TestIterationClauseEscapesQuotes`, `TestWIQLInterpolationStructural`, `TestWIQLCheckCatchesBypasses` (split literal, WorkItemLinks, variable source, `+=`), `TestUnknownProjectRejected` (11 rows, including a case mismatch, nothing configured, and write verbs), and `TestSetStateRefusesWorkItemInAnotherProject`. |
| 5 | CI runs build, vet, race tests and gofmt on every push and fails on regression | UNCERTAIN (external) | `.github/workflows/ci.yml` triggers on `push` and `pull_request` with no branch filter. It sets `permissions: contents: read`, pins both actions to 40-hex SHAs, uses `go-version: '1.25.4'` (which matches go.mod), and runs the four gates in order with no secrets. I ran the gofmt step's script against a scratch dir holding an unformatted file: it exited 1 and named the file. The workflow has never run on GitHub, because dev has not been pushed (220 commits ahead). This is phase-blocking human item 1. |
| 6 | The log file is opened 0600 with a 0700 directory, and a test asserts the PAT never appears in log output | VERIFIED | `logging.Init` runs `MkdirAll 0o700`, `os.Chmod(logDir, 0o700)`, `OpenFile 0o600` and `file.Chmod(0o600)` on every call, and closes the file if the chmod fails (logging.go:26-41). `TestInitSetsOwnerOnlyPermissions` passes for fresh, loose and repeated installs. `TestPATNeverLogged` passes. It checks the raw, Basic-header and base64 forms across a read, a confirmed write and a panic, with non-vacuity markers. WR-05: `Config` has `LogValue`/`String`/`GoString`, which render `pat_set` but never the PAT, and `TestLoadedConfigNeverRendersPAT` passes. The only non-test PAT use outside config is `NewPatConnection` (client.go:55). |

**Score:** 5/6 roadmap criteria verified in code. SC5 is pending one observed GitHub run. 0 truths are present-but-behavior-unverified.

### Plan must-have truths

| Plan | Truths | Result | Notes |
|------|--------|--------|-------|
| 05-01 | 13 (1 backstop) | 12 VERIFIED, 1 backstop routed to human item 3 | Two truths have drifted on purpose because of review fixes. The success text is now `Downloaded to %q` rather than `Downloaded:` (WR-04). The call passes a cancellable `ctx` rather than `context.Background()` (WR-03). The partial-file truth now holds through temp file plus `os.Link` (WR-01). Intent holds in every case. |
| 05-02 | 5 | 5 VERIFIED | |
| 05-03 | 8 (1 backstop) | 7 VERIFIED, 1 backstop routed to human item 1 | No committed test reads `ADO_PAT` for real. The only references blank it (usage_test.go:52-53) or set a fixture (patleak_test.go:38, config_test.go:172). |
| 05-04 | 8 | 8 VERIFIED | |
| 05-05 | 7 (1 backstop) | 6 VERIFIED, 1 backstop routed to human item 2 | |
| 05-06 | 8 (1 backstop) | 7 VERIFIED, 1 backstop (tripwire framing) accepted as documentation | The "tripwire, not a present-leak detector" truth is a statement about intent. The test doc matches it, and nothing asserts that the stack leaks. |

### Prohibitions (all test-tier)

| Plan | Prohibition | Enforcement | Disposition |
|------|-------------|-------------|-------------|
| 05-01 | Never fetch by URL, no fallback | `TestNoRawHTTPInADO` (AST), `TestGetWorkItemDetailAttachmentID` (unparsable GUID not offered) | Enforced |
| 05-01 | A refused download is never reported as success and never leaves a file | `TestDownloadAttachmentRefusesBeforeFetch`; `AttachmentDownloadedMsg.Err` renders `Error:` (detail.go:115) | Enforced |
| 05-03 | CI never given credentials, no skip guard | No committed test. I inspected ci.yml directly: no `secrets.`, no `ADO_*`, and the only `t.Skip`s are Windows permission skips | Flagged: unverified-prohibition, human review recommended (folded into human item 1) |
| 05-04 | No case-fold, prefix or fuzzy match | `TestUnknownProjectRejected/case_mismatch_is_not_folded` | Enforced |
| 05-04 | Rejected value never on stdout | `TestUnknownProjectRejected` asserts that stdout is empty; `fs.SetOutput(stderrWriter)` | Enforced |
| 05-05 | Query text never logged; project never concatenated | `TestWIQLInterpolationStructural`, `TestWIQLCheckCatchesBypasses`, log capture in both macro tests | Enforced |
| 05-06 | PAT test never uses a real PAT or real HOME | The test sets a fixture and `HOME=t.TempDir()` | Enforced |

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/ado/attachments.go` | VERIFIED | Substantive (163 lines). Wired from detail.go:208 |
| `internal/ado/attachments_test.go` | VERIFIED | Contains `TestNoRawHTTPInADO` and 8 other tests; all pass |
| `internal/ado/interfaces.go` | VERIFIED | `GetAttachmentContent` at line 24, called at attachments.go:102 |
| `internal/ui/workitems/detail.go` | VERIFIED | Passes `att.ID`, the attachments dir and `att.Name`; cancels on esc/q |
| `internal/ado/nilguard_test.go` | VERIFIED | 3 tests pass |
| `internal/ado/pipelines.go`, `releases.go` | VERIFIED | Guards present |
| `.github/workflows/ci.yml` | VERIFIED (file), run pending | See SC5 |
| `cmd/adoBizInt/flags.go`, `main.go`, `main_test.go`, `filter_test.go` | VERIFIED | Allow-list, assignment, helper and table all present and exercised |
| `internal/ado/client.go` (`iterationClause`), `wiql_test.go`, `workitems.go`, `dashboard.go` | VERIFIED | |
| `internal/logging/logging.go`, `logging_test.go`, `cmd/adoBizInt/patleak_test.go` | VERIFIED | |
| `internal/config/config.go` (WR-05 redaction) | VERIFIED | Value-receiver `LogValue`, so `*Config` is redacted too |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| detail.go | attachments.go | `client.DownloadAttachment(ctx, att.ID, destDir, att.Name)` | WIRED. The plan's regex expected `context.Background()`, and WR-03 replaced it with a cancellable ctx on purpose |
| attachments.go | interfaces.go | `attachmentPath` first, then `GetAttachmentContent(ctx, ...{Id: &id})` | WIRED |
| workitems.go | attachments.go | `uuid.Parse(path.Base(u.Path))` gives `Attachment.ID` | WIRED |
| pipelines.go / releases.go guard | later `*def.Id` | same function | WIRED |
| ci.yml | go.mod | `go-version: '1.25.4'` == `go 1.25.4` | WIRED |
| main.go | flags.go | `configuredProjects = []string(cfg.Projects)` after Validate | WIRED |
| flags.go | dispatch.go / write.go | `fs.Parse` error, then exitUnknown (3) | WIRED |
| workitems.go / dashboard.go | client.go | `+ iterationClause(iterations)` | WIRED |
| workitems.go / dashboard.go | SDK route | `Project: &project` | WIRED (SDK puts it in routeValues) |
| write.go set-state | SDK PATCH route | `UpdateWorkItemState(ctx, t.Project, ...)` into `UpdateWorkItemArgs.Project` | WIRED (CR-01) |
| logging.go | main.go | `logging.Init()` error turns into a non-fatal warning | WIRED |
| patleak_test.go | crash.go | `loggingReady = true` | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real | Status |
|----------|------|--------|------|--------|
| TUI attachment download | attachment bytes | SDK `GetAttachmentContent` via `workItemAPI` | Yes (live route; fake in tests) | FLOWING |
| Attachment.ID | GUID | relation URL last path segment | Yes | FLOWING |
| `configuredProjects` | allow-list | `cfg.Projects` after Validate | Yes | FLOWING |
| `detail.Project` | owning project | `System.TeamProject` from GetWorkItem (Expand=Relations returns all fields) | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full gate | `go build ./... && go vet ./... && go test -race -count=1 ./... && gofmt -l .` | all ok, gofmt empty | PASS |
| HARD-01/02/03/04 ado tests | `go test -count=1 -run '<10 named tests>' ./internal/ado/` | 10/10 PASS | PASS |
| HARD-04 cmd tests, CR-01, HARD-06 | `go test -count=1 -run 'TestUnknownProjectRejected\|TestPATNeverLogged\|TestSetStateRefusesWorkItemInAnotherProject' ./cmd/adoBizInt/` | PASS (11 + 1 + 2 subtests) | PASS |
| HARD-06 perms | `go test -run TestInitSetsOwnerOnlyPermissions ./internal/logging/` | 3 subtests PASS | PASS |
| WR-05 | `go test -run TestLoadedConfigNeverRendersPAT ./internal/config/` | PASS | PASS |
| WR-03/WR-04 TUI | `go test -run 'TestCloseDetailCancelsDownload\|TestDetailNeverRendersRawControlBytes' ./internal/ui/workitems/` | PASS | PASS |
| gofmt step fails on regression | ci.yml gofmt script run in a scratch dir with an unformatted file | exit 1, names `bad.go` | PASS |
| CI on GitHub | n/a | not pushed | SKIP, routed to human item 1 |

### Probe Execution

No `scripts/*/tests/probe-*.sh` exist and no phase artifact declares a probe. Step 7c: SKIPPED.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| HARD-01 | 05-01 | SATISFIED | SC1 evidence |
| HARD-02 | 05-01 | SATISFIED | SC2 evidence |
| HARD-03 | 05-02 | SATISFIED | SC3 evidence |
| HARD-04 | 05-04, 05-05 (+ CR-01 fix) | SATISFIED in code; live check pending (human items 2, 4) | SC4 evidence |
| HARD-05 | 05-03 | SATISFIED as a file; observed run pending (human item 1) | SC5 evidence |
| HARD-06 | 05-06 (+ WR-05 fix) | SATISFIED | SC6 evidence |

No orphaned requirements. REQUIREMENTS.md maps exactly HARD-01..HARD-06 to Phase 5, and every one is claimed by a plan.

### Anti-Patterns Found

I found no `TBD`/`FIXME`/`XXX`/`TODO`/`HACK` markers in any of the 34 non-planning files changed in `33a0ebf^..HEAD`.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| cmd/adoBizInt/write.go | 320 | Exact case-sensitive compare of `detail.Project` against `--project` | Info | This is correct fail-closed behavior. If a config entry differs in case from ADO's canonical `System.TeamProject`, every set-state is refused. Folded into human item 4 |
| cmd/adoBizInt/main.go | 108 vs 106 | `ado.NewClient` (which may contact the configured org to resolve resource areas) runs before dispatch parses `--project`, so a bogus `--project` still costs one connection to the configured host before exit 3 | Info | Not a security issue: the PAT only goes to the configured org. No read or write client method is called |
| internal/ui/workitems/detail.go | 115 | `"Error: " + msg.Err.Error()` renders SDK and server error text unsanitized. Containment errors are `%q`-quoted, but a server error body is not | Info | A small residue of WR-04. Server error messages come from the configured org |
| internal/ado/client.go | 295 | Iteration paths (server-supplied) are still concatenated after `''` escaping | Info | Explicitly accepted by D-10 revised. WIQL offers no parameter binding beyond macros, and the value is not caller-supplied |
| internal/config/priorities.go | 59, 70 | 0755 / 0644 | Info | IN-07, deferred. The parent directory is now 0700 |

### Human Verification Required

#### 1. PHASE-BLOCKING: green GitHub Actions `ci` run (05-03 T2)

**Test:** Push `dev` to origin and find the `ci` run for HEAD in the Actions tab.
**Expected:** The run is green, with build, vet, race tests and gofmt all passing. While you are there, confirm the run used no secrets. Optional: a throwaway unformatted branch goes red at gofmt.
**Why human:** GitHub is an external service, and the branch has not been pushed.

#### 2. PHASE-BLOCKING: live `@project` before/after comparison (05-05 T2)

**Test:** Build a binary at the 05-05 base commit and one at HEAD. For one configured project, compare `workitems list --json` item counts and `dashboard counts --json` work_items counts between them.
**Expected:** The counts are equal and both envelopes report `complete: true`.
**Why human:** This needs a real PAT and the live org.

#### 3. Real TUI attachment download by ID (05-01 T3)

**Test:** Download a known attachment from the work-item detail modal.
**Expected:** The file is byte-identical to the one from the web UI and sits at `Downloaded to "<path>"`. The directory is 0700 and the file 0600. A repeat download gets ` (1)`, and esc cancels a slow download.
**Why human:** This needs a live SDK route and a check of the visual TUI.

#### 4. Live set-state dry run and `--confirm` with the project now in the update route (CR-01 fix)

**Test:** Run a dry run and then `--confirm` on an item in configured project P. Then run a dry run on an item from another project.
**Expected:** The first two succeed, with the audit record naming P. The cross-project item is refused with exit 3 and no audit record. The configured casing must match ADO's `System.TeamProject` exactly.
**Why human:** The PATCH route with a project and the live `System.TeamProject` value have only been exercised against fakes.

### Gaps Summary

There are no code gaps. I checked all six HARD requirements against the code on HEAD rather than trusting the summaries. Each is backed by passing behavioral tests, and HARD-01, HARD-03 and HARD-04 are also backed by go/ast structural tests. I checked each review fix directly:

- **CR-01:** set-state is now scoped to `--project`, in both the pre-read refusal and the PATCH route.
- **WR-01 / WR-02:** downloads never clobber an existing file, and files are owner-only.
- **WR-03:** closing the modal cancels a download.
- **WR-04:** control bytes are refused in names and stripped in the TUI list.
- **WR-05:** `Config` redacts the PAT structurally.
- **WR-06:** the WIQL structural test is anchored on the `QueryByWiql` sink and catches split-literal bypasses.

The phase cannot be marked complete until someone runs the two phase-blocking checks, a green CI run and the live `@project` comparison. The TUI download and the live set-state checks should also be done, because CR-01 changed the write route.

---

_Verified: 2026-09-30T18:30:54Z_
_Verifier: Claude (gsd-verifier)_
