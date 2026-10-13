---
phase: 05-hardening
plan: 01
subsystem: api
tags: [security, attachments, azure-devops-sdk, path-containment, go-ast]

requires:
  - phase: 01
    provides: workItemAPI seam and closure fakes (SEAM-01)
  - phase: 02
    provides: connection.Timeout via newConnection (TRUST-04 budget)
provides:
  - "DownloadAttachment(ctx, id uuid.UUID, destDir, name) fetching through SDK GetAttachmentContent"
  - "attachmentPath containment guard (pure, table-tested)"
  - "Attachment.ID parsed from relation URL; GUID-less attachments not offered"
  - "TestNoRawHTTPInADO structural guard (no net/http, no AuthorizationString, no *azuredevops.Connection struct field)"
affects: [05-06 PAT-leak test, deferred CLI attachment verb]

actuals:
  tokens: 6052
  tasks: 3
  commits: 5
plan_head_before: 333c61b7f5d00a3f61c30e58fe5e3554a5723801

tech-stack:
  added: []
  patterns:
    - "Guard-before-fetch: validate server-supplied names before any SDK call; nil fake closure is the no-call assertion"
    - "Checked Close plus os.Remove on a failed write so a partial file never looks complete"

key-files:
  created:
    - internal/ado/attachments.go
    - internal/ado/attachments_test.go
  modified:
    - internal/ado/interfaces.go
    - internal/ado/fakes_test.go
    - internal/ado/workitems.go
    - internal/ado/json_test.go
    - internal/ado/client.go
    - internal/ui/workitems/detail.go

key-decisions:
  - "Client.connection deleted (RESEARCH OQ4); newConnection and every sub-client built from it stay"
  - "Backslash names accepted as literal file names on Unix (RESEARCH OQ3, D-03 as written); table asserts containment only for those rows"
  - "Containment uses filepath.Clean + IsAbs + exact-child Dir check, not EvalSymlinks (macOS /var symlink)"
  - "TestNoRawHTTPInADO checks *azuredevops.Connection only inside struct types, because newConnection's result list is also an *ast.Field"

patterns-established:
  - "Structural security guard as go/ast test with a non-vacuity check (seen attachments.go)"

requirements-completed: [HARD-01, HARD-02]

coverage:
  - id: D1
    description: "Attachment download goes by GUID through SDK GetAttachmentContent; the URL-based raw HTTP path is deleted"
    requirement: HARD-01
    verification:
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestDownloadAttachmentFetchesByID"
        status: pass
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestNoRawHTTPInADO"
        status: pass
    human_judgment: false
  - id: D2
    description: "Hostile attachment names are refused before any fetch and every accepted name is a direct child of destDir"
    requirement: HARD-02
    verification:
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestAttachmentPath"
        status: pass
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestDownloadAttachmentRefusesBeforeFetch"
        status: pass
    human_judgment: false
  - id: D3
    description: "A mid-body failure returns the error, closes the body and removes the partial file"
    requirement: HARD-01
    verification:
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestDownloadAttachmentRemovesPartialFile"
        status: pass
    human_judgment: false
  - id: D4
    description: "Attachment carries ID parsed from the relation URL; relations without a usable GUID are dropped silently; JSON keys id/name/url/size"
    requirement: HARD-01
    verification:
      - kind: unit
        ref: "internal/ado/attachments_test.go#TestGetWorkItemDetailAttachmentID"
        status: pass
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames"
        status: pass
    human_judgment: false
  - id: D5
    description: "A real attachment downloaded through the TUI detail modal lands at ~/.config/adoBizInt/attachments/<name>, byte-identical to the ADO web UI copy"
    requirement: HARD-01
    verification: []
    human_judgment: true
    rationale: "Needs the operator's own PAT and a real attachment; CI has no credentials by design (D-17). Task 3 human-check, harvested into end-of-phase UAT."

duration: 5min
completed: 2026-09-30
status: complete
---

# Phase 5 Plan 01: Attachment by ID Summary

**`DownloadAttachment` now takes an attachment GUID and fetches it through the SDK's `GetAttachmentContent`. A pure `attachmentPath` guard refuses traversal, NUL-bearing and empty names before any request is made. A go/ast test keeps `net/http`, `AuthorizationString` and `*azuredevops.Connection` struct fields out of `internal/ado`.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-09-30T16:57:45Z
- **Completed:** 2026-09-30T17:02:46Z
- **Tasks:** 3
- **Files modified:** 8 (2 created, 6 modified)

## Accomplishments

- Closed CONCERNS C3. No parameter accepts a URL, so the PAT goes only to the org host the SDK resolved, and it inherits `connection.Timeout`. This resolves the TRUST-04 exclusion that Phase 2 deferred.
- Closed CONCERNS H6. `attachmentPath` rejects a relative `destDir`, NUL bytes, and `.`/`..`/absolute base names, and it checks that the joined path is an exact child of `destDir`. All of this runs before `GetAttachmentContent`.
- A failed copy or a failed close removes the partial file.
- `Attachment.ID` (`uuid.UUID`, `json:"id"`) is parsed from the last URL path segment. Relations without a non-nil GUID are not offered (D-06).
- The TUI caller passes `att.ID`, the attachments directory and `att.Name`. It now handles the `UserHomeDir` error, and the unused `AttachmentDownloadedMsg.Path` field is gone.
- `Client.connection` is deleted, so no struct in `internal/ado` holds the credential-bearing connection.

## Task Commits

1. **Task 1: Download by ID, end to end (tracer)**: `d8c86cf` (test, RED) and `e9c4557` (feat, GREEN)
2. **Task 2: Pin containment and partial-file cleanup**: `0c9b6f0` (test)
3. **Task 3: Structural no-raw-HTTP guard and D-06 pin**: `5b31fe7` (test, RED) and `0ca4682` (fix, GREEN)

## Files Created/Modified

- `internal/ado/attachments.go`: `attachmentPath` and `DownloadAttachment` (new)
- `internal/ado/attachments_test.go`: six tests (new)
- `internal/ado/interfaces.go`: `workItemAPI.GetAttachmentContent`
- `internal/ado/fakes_test.go`: `GetAttachmentContentFunc` and its method
- `internal/ado/workitems.go`: old URL download deleted, `Attachment.ID`, GUID extraction, D-06 guard, imports trimmed (`net/http` removed)
- `internal/ado/json_test.go`: `Attachment` key set now `id,name,url,size`
- `internal/ado/client.go`: `connection` field removed, doc comment reworded
- `internal/ui/workitems/detail.go`: new call shape, `Path` field removed, home-dir error handled

## Verification

- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`: exit 0
- `go test ./internal/ado -run 'Attachment|NoRawHTTP|JSONFieldNames' -v`: all named tests PASS
- `TestDispatchTableMatchesD02` passes with `cmd/` unedited (D-02, no CLI verb)
- Mutation checks, both reverted:
  - Moving `attachmentPath` below the fetch made `TestDownloadAttachmentRefusesBeforeFetch` fail with `unexpected call to workItemAPI.GetAttachmentContent`.
  - Adding `_ "net/http"` to `attachments.go` made `TestNoRawHTTPInADO` fail.
- `go doc -u ./internal/ado Client` lists no `connection` field.

## Decisions Made

- `DownloadAttachment` returns the primary error on a failed copy or close. The best-effort `Close` and `Remove` on that path do not replace it.
- `TestNoRawHTTPInADO` checks for the `*azuredevops.Connection` type only in struct field lists. `newConnection` legitimately returns that type, and its result list is also an `*ast.Field`.
- The other planner decisions (OQ3, OQ4, Pitfall 7, no `EvalSymlinks`, `go.mod` unchanged) were applied as recorded in the plan.

## Deviations from Plan

None. The plan was executed as written.

## TDD Gate Compliance

- Task 1: RED `d8c86cf` (compile failure) came before GREEN `e9c4557`.
- Task 2: `0c9b6f0` is a test-only pinning commit. The plan expected the Task 1 implementation to pass these tests as written, and it did. The mutation check shows the refusal test is not vacuous.
- Task 3: RED `5b31fe7` failed `TestNoRawHTTPInADO` on `client.go:27` (`Client.connection`) and was followed by GREEN `0ca4682`. `TestGetWorkItemDetailAttachmentID` passed at RED because Task 1 had already implemented D-06, so it acts as a regression pin.

## Human Check (end-of-phase UAT)

- **Test:** `go build -o adoBizInt ./cmd/adoBizInt`, then run `./adoBizInt` (TUI). Open a work item with a known attachment, select it and download it.
- **Expected:** The modal shows `Downloaded: <name>`, and `~/.config/adoBizInt/attachments/<name>` is byte-identical to the copy from the ADO web UI. If this run created the directory, `ls -ld ~/.config/adoBizInt/attachments` shows `drwx------`.
- **Why human:** It needs the operator's PAT and a real attachment. CI has no credentials (D-17). Never paste the PAT into the session.

## Issues Encountered

None.

## Known Stubs

None.

## Next Phase Readiness

Ready for 05-02. `internal/ado` has no `net/http` import, which 05-06's PAT-leak tripwire assumes.

## Self-Check: PASSED

- FOUND: internal/ado/attachments.go, internal/ado/attachments_test.go
- FOUND commits: d8c86cf, e9c4557, 0c9b6f0, 5b31fe7, 0ca4682
