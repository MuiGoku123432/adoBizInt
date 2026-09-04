---
phase: 01-test-seam-for-internal-ado
reviewed: 2026-09-04T23:19:54Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - internal/ado/branches_test.go
  - internal/ado/client.go
  - internal/ado/dashboard_test.go
  - internal/ado/fakes_test.go
  - internal/ado/interfaces.go
  - internal/ado/pipelines_test.go
  - internal/ado/pullrequests.go
  - internal/ado/pullrequests_test.go
  - internal/ado/releases_test.go
  - internal/ado/workitems_test.go
findings:
  critical: 0
  warning: 1
  info: 0
  total: 1
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-09-04T23:19:54Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

The four interfaces match the 20 SDK methods consumed by `internal/ado`, and `NewClient` assigns every SDK client to the corresponding seam. The cached identity is read from `AuthenticatedUser.Id`, while the display name still comes from `AuthorizedUser.ProviderDisplayName`.

All seven destructive methods have network-free fake-backed tests, all eight aggregate-read characterization tests execute with non-empty fixtures, both pipeline tests bypass the empty-configuration guards, and the dashboard test distinguishes in-place failure swallowing from project-level skipping. The test suite passes without ADO credentials (`go test ./internal/ado -count=1 -race -cover`, 57.2%), and the full repository passes `go vet ./...` and `go test ./... -count=1 -race`. No test contains a PAT, organization URL, connection constructor, HTTP client, or live network path. The five read/write mutation targets that were not intentionally changed are byte-identical to `befd7ff`; `pullrequests.go` is byte-identical to the intended identity-cache commit (`3325366`). No deliberate mutation remains.

One test-contract defect remains: the pull-request completion test does not verify the exact initial ADO fetch call, despite claiming an exact fetch-then-merge interaction.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: CompletePullRequest test ignores the first ADO call's arguments

**Classification:** WARNING

**File:** `internal/ado/pullrequests_test.go:66-73`

**Issue:** `TestCompletePullRequestIssuesFetchThenMerge` records only that `GetPullRequest` happened; its fake discards `git.GetPullRequestArgs`. The test deeply compares the subsequent `UpdatePullRequest` call, but a regression that fetches the wrong repository, project, or pull-request ID would still pass as long as the update remains correct. This makes the Phase 1 claim that every destructive write test asserts its exact ADO interaction false for the only write that performs two ADO calls.

**Fix:** Capture and compare the fetch arguments as well as the update arguments:

```go
var gotGet git.GetPullRequestArgs
var gotUpdate git.UpdatePullRequestArgs

GetPullRequestFunc: func(_ context.Context, args git.GetPullRequestArgs) (*git.GitPullRequest, error) {
    gotGet = args
    calls = append(calls, "get")
    return &git.GitPullRequest{LastMergeSourceCommit: &git.GitCommitRef{CommitId: &commitID}}, nil
},

wantGet := git.GetPullRequestArgs{
    RepositoryId:  &wantRepositoryID,
    PullRequestId: &wantPullRequestID,
    Project:       &wantProject,
}
if !reflect.DeepEqual(gotGet, wantGet) {
    t.Errorf("GetPullRequest args = %+v, want %+v", gotGet, wantGet)
}
```

Keep the existing call-order and exact `UpdatePullRequestArgs` assertions.

---

_Reviewed: 2026-09-04T23:19:54Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
