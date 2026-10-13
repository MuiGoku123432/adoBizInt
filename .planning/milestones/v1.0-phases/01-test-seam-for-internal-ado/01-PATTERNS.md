# Phase 1: Test seam for internal/ado - Pattern Map

**Mapped:** 2026-09-04
**Mode:** gap closure
**Files analyzed:** 3 modified files
**Analogs found:** 3 / 3

## Scope Guard

This map covers only the two verified gaps:

1. Make `TestCompletePullRequestIssuesFetchThenMerge` compare the exact initial `git.GetPullRequestArgs` while retaining its existing call-order and exact update assertions.
2. Keep the successful construction-time identity cache, but restore a testable live identity lookup whenever `currentUserID` is empty. Preserve the old retry and error-wrapping semantics without adding public API.

Do not revisit the four SDK interfaces, the other six write tests, aggregate-read characterizations, UI code, or deferred phases.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/ado/pullrequests_test.go` | test | request-response, ordered two-call interaction | Same file: `TestApprovePullRequestIssuesExactReviewerCall` at lines 12-47 plus the existing completion test at lines 59-117 | exact |
| `internal/ado/client.go` | service/client construction and injectable provider | request-response, cached lookup with live fallback | Same file: SDK clients initialized before the `Client` literal at lines 37-108; `internal/ado/fakes_test.go:70-104` for the package-private func-field seam | composite exact |
| `internal/ado/pullrequests.go` | service | request-response, cache-first identity lookup then mutation | Pre-phase implementation preserved in git history, plus current `CompletePullRequest` wrapped-read pattern at lines 185-193 | exact behavioral analog |

## Pattern Assignments

### `internal/ado/pullrequests_test.go` (test, request-response)

**Primary analog:** `internal/ado/pullrequests_test.go:12-47`, `TestApprovePullRequestIssuesExactReviewerCall`

**Imports pattern** (lines 3-10):

```go
import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)
```

No import is needed for the fetch-argument fix. The identity fallback tests will likely add the existing SDK `location` and `identity` packages plus `github.com/google/uuid`; `uuid` is already an indirect dependency through the pinned SDK, so `go.mod` remains unchanged.

**Capture and exact literal comparison pattern** (lines 12-46):

```go
var got git.CreatePullRequestReviewerArgs
client := &Client{
	currentUserID: "authenticated-user-id",
	gitClient: &fakeGitAPI{
		t: t,
		CreatePullRequestReviewerFunc: func(_ context.Context, args git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error) {
			got = args
			return &git.IdentityRefWithVote{}, nil
		},
	},
}

wantProject := "project-one"
wantRepositoryID := "repository-one"
wantPullRequestID := 42
want := git.CreatePullRequestReviewerArgs{
	RepositoryId:  &wantRepositoryID,
	PullRequestId: &wantPullRequestID,
	Project:       &wantProject,
	// ...remaining exact fields...
}

if !reflect.DeepEqual(got, want) {
	t.Errorf("CreatePullRequestReviewer args = %+v, want %+v", got, want)
}
```

Apply this directly to the initial fetch in `TestCompletePullRequestIssuesFetchThenMerge`:

- Split the current generic `got` into descriptive captures such as `gotGet` and `gotUpdate`.
- Change the callback's ignored argument at line 66 to `args git.GetPullRequestArgs`, assign `gotGet = args`, and then append `"get"` as it does today.
- Declare a literal expected fetch value using pointers to `"project-one"`, `"repository-one"`, and `42`.
- Compare with `reflect.DeepEqual`.

The pinned SDK type confirms the complete expected shape at `.../azuredevops@v1.0.0-b5/git/client.go:3375-3392`:

```go
type GetPullRequestArgs struct {
	RepositoryId       *string
	PullRequestId      *int
	Project            *string
	MaxCommentLength   *int
	Skip               *int
	Top                *int
	IncludeCommits     *bool
	IncludeWorkItemRefs *bool
}
```

Only the first three fields should be non-nil. A literal with those fields therefore also pins every optional field to nil.

**Existing ordered two-call pattern to retain unchanged** (`internal/ado/pullrequests_test.go:59-89`):

```go
var calls []string

GetPullRequestFunc: func(_ context.Context, _ git.GetPullRequestArgs) (*git.GitPullRequest, error) {
	calls = append(calls, "get")
	// return merge source commit
},
UpdatePullRequestFunc: func(_ context.Context, args git.UpdatePullRequestArgs) (*git.GitPullRequest, error) {
	calls = append(calls, "update")
	got = args
	return &git.GitPullRequest{}, nil
},

wantCalls := []string{"get", "update"}
if !reflect.DeepEqual(calls, wantCalls) {
	t.Fatalf("call order = %v, want %v", calls, wantCalls)
}
```

Do not replace this with only argument assertions. The test must prove both exact calls and their order.

**Existing exact merge assertion to retain unchanged** (`internal/ado/pullrequests_test.go:91-116`):

```go
want := git.UpdatePullRequestArgs{
	GitPullRequestToUpdate: &git.GitPullRequest{
		Status: &wantStatus,
		CompletionOptions: &git.GitPullRequestCompletionOptions{
			DeleteSourceBranch: &wantDeleteSourceBranch,
			MergeStrategy:      &wantMergeStrategy,
		},
		LastMergeSourceCommit: &git.GitCommitRef{
			CommitId: &wantCommitID,
		},
	},
	RepositoryId:  &wantRepositoryID,
	PullRequestId: &wantPullRequestID,
	Project:       &wantProject,
}

if !reflect.DeepEqual(got, want) {
	t.Errorf("UpdatePullRequest args = %+v, want %+v", got, want)
}
```

Rename only local `got`/`want` variables as needed to avoid ambiguity. Do not weaken the literal or compare selected fields.

**Identity fallback tests -- closest fake pattern** (`internal/ado/fakes_test.go:70-104`):

```go
type fakeGitAPI struct {
	t *testing.T

	GetPullRequestFunc func(context.Context, git.GetPullRequestArgs) (*git.GitPullRequest, error)
}

func (f *fakeGitAPI) GetPullRequest(ctx context.Context, args git.GetPullRequestArgs) (*git.GitPullRequest, error) {
	f.t.Helper()
	if f.GetPullRequestFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.GetPullRequest")
	}
	return f.GetPullRequestFunc(ctx, args)
}
```

Use the same direct func injection idea for the new package-private identity provider, but do not add another fake type unless needed. A function-valued `Client` field can be assigned an inline closure in the test.

Required behavioral cases:

1. **Cache hit:** non-empty `currentUserID`; inject a provider closure that calls `t.Fatal` if reached; exact reviewer call remains as lines 12-47 already assert.
2. **Retry and recovery:** empty `currentUserID`; make the provider return a sentinel error on the first approval and a `location.ConnectionData` with `AuthenticatedUser.Id` on the second. Assert the first error wraps the sentinel, the second approval succeeds, the provider ran twice, and Git ran only for the successful attempt. This is the direct regression test for the user-selected retry contract.
3. **Live lookup returns no authenticated ID:** empty cache; provider returns nil/incomplete connection data; return exactly `could not determine current user ID`; leave Git fake methods unstubbed so any write fails by name.

The existing empty-ID test at lines 49-57 is no longer correct as written once fallback is restored. Convert it to inject a provider result with no ID rather than relying on the zero-valued `Client` alone.

---

### `internal/ado/client.go` (service/client construction, request-response)

**Primary analog:** `internal/ado/client.go:20-35` and `:37-108`

**Imports pattern** (lines 3-18):

```go
import (
	"context"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"
	// other SDK packages

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
)
```

Keep `location` in this file. The package-private provider should use the exact SDK signature, avoiding a new abstraction around `location.ConnectionData`:

```go
func(context.Context, location.GetConnectionDataArgs) (*location.ConnectionData, error)
```

**Client field injection pattern** (lines 20-35, analogous to existing unexported seams):

```go
type Client struct {
	connection     *azuredevops.Connection
	workitemClient workItemAPI
	gitClient      gitAPI
	// ...
	currentUser   string
	currentUserID string
}
```

Add one unexported function field adjacent to `currentUserID`, for example:

```go
getConnectionData func(context.Context, location.GetConnectionDataArgs) (*location.ConnectionData, error)
```

This is the minimal repository-aligned seam:

- Package-private, matching all existing injectable SDK fields.
- Function-valued, matching `internal/ado/fakes_test.go`'s established `XFunc` convention.
- Exact SDK method signature, verified at `location/client.go:23-28`.
- No exported constructor or interface, and no change to `NewClient`'s public signature.

**Production initialization pattern** (lines 67-86 and 95-108):

```go
locationClient := location.NewClient(ctx, connection)
currentUser := ""
currentUserID := ""
currentUserEmail := cfg.UserEmail
if locationClient != nil {
	connData, err := locationClient.GetConnectionData(ctx, location.GetConnectionDataArgs{})
	if err == nil && connData != nil {
		if connData.AuthorizedUser != nil && connData.AuthorizedUser.ProviderDisplayName != nil {
			currentUser = *connData.AuthorizedUser.ProviderDisplayName
		}
		if connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
			currentUserID = connData.AuthenticatedUser.Id.String()
		}
	}
}

return &Client{
	connection:    connection,
	// ...existing client assignments...
	currentUser:   currentUser,
	currentUserID: currentUserID,
}, nil
```

Preserve this eager call and successful cache exactly. Bind the same already-created location client into the returned `Client`, for example by assigning its method value. To retain the existing nil check, capture it in a local only after confirming the SDK client is non-nil:

```go
var getConnectionData func(context.Context, location.GetConnectionDataArgs) (*location.ConnectionData, error)
if locationClient != nil {
	getConnectionData = locationClient.GetConnectionData
	connData, err := getConnectionData(ctx, location.GetConnectionDataArgs{})
	// preserve the existing non-fatal extraction logic
}

return &Client{
	// existing fields
	getConnectionData: getConnectionData,
}
```

This preserves construction-time identity behavior and lets `ApprovePullRequest` retry through the same authenticated location client. It also allows in-package tests to inject a closure without a network or PAT.

The existing warning at lines 84-85 becomes false once fallback exists. Keep a credential-free warning, but describe the actual degraded state rather than permanent failure, for example:

```go
log.Warn("Could not cache current user ID; pull request approval will retry identity lookup")
```

Avoid these alternatives:

- Do not store the wide `location.Client` interface. Only one method is consumed; the requested shape is a narrow function provider.
- Do not add `NewClientWith...` or any exported API.
- Do not move the initial lookup out of `NewClient`; a successful startup lookup must remain cached.
- Do not make startup identity failure fatal.

**SDK signature and identity field contract**:

`location/client.go:23-28,48-53,85-115`:

```go
type Client interface {
	GetConnectionData(context.Context, GetConnectionDataArgs) (*ConnectionData, error)
}

func NewClient(ctx context.Context, connection *azuredevops.Connection) Client
```

`location/models.go:28-33`:

```go
type ConnectionData struct {
	AuthenticatedUser *identity.Identity `json:"authenticatedUser,omitempty"`
	AuthorizedUser    *identity.Identity `json:"authorizedUser,omitempty"`
}
```

`identity/models.go:90-105`:

```go
type Identity struct {
	Id                  *uuid.UUID `json:"id,omitempty"`
	ProviderDisplayName *string    `json:"providerDisplayName,omitempty"`
}
```

Use `AuthenticatedUser.Id` for the reviewer ID. Keep `AuthorizedUser.ProviderDisplayName` solely for `currentUser`; they are distinct SDK fields.

---

### `internal/ado/pullrequests.go` (service, request-response)

**Primary analog:** the pre-phase `ApprovePullRequest` behavior, corroborated by current wrapped read handling in `CompletePullRequest` at lines 185-193.

**Imports pattern** (lines 3-12):

```go
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"

	"sentinovo.ai/bizInt/internal/logging"
)
```

Add the `location` import for the exact `GetConnectionDataArgs{}` value. Keep location-client **construction** in `NewClient`; `ApprovePullRequest` calls only the injected function provider.

**Cache-first fallback pattern to implement**:

```go
userID := c.currentUserID
if userID == "" {
	connData, err := c.getConnectionData(ctx, location.GetConnectionDataArgs{})
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	if connData != nil && connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
		userID = connData.AuthenticatedUser.Id.String()
	} else {
		return fmt.Errorf("could not determine current user ID")
	}
}
```

If this exact placement requires a `location` import for `GetConnectionDataArgs`, add it. The key restriction is no `location.NewClient` construction in this method, not no reference to the SDK's argument type.

Behavioral rules:

- A non-empty cache is authoritative for that approval and avoids another lookup.
- An empty cache triggers a fresh lookup on every approval attempt, preserving recovery from a transient startup or prior approval-time failure.
- Do not memoize either a failed fallback or its later success. Use the live ID for that approval only. This most closely preserves the former per-approval lookup after startup failed, avoids adding mutable shared state, and guarantees every later attempt can retry.
- Tests that exercise an empty cache must inject the provider explicitly. Do not invent a network fallback from `c.connection` inside this method because that recreates the untestable seam.

**Error wrapping pattern** (`internal/ado/pullrequests.go:185-193`):

```go
pr, err := c.gitClient.GetPullRequest(ctx, git.GetPullRequestArgs{
	RepositoryId:  &repositoryID,
	PullRequestId: &prID,
	Project:       &project,
})
if err != nil {
	return fmt.Errorf("failed to get PR details: %w", err)
}
```

Use the same `%w` convention for a live identity lookup error, restoring the baseline text exactly:

```go
return fmt.Errorf("failed to get current user: %w", err)
```

For missing identity data, retain the existing exact message:

```go
return fmt.Errorf("could not determine current user ID")
```

After identity resolution, leave the reviewer construction, exact Git call, logging, and returned Git error at lines 157-176 unchanged.

## Shared Patterns

### Package-private dependency injection

**Source:** `internal/ado/fakes_test.go:20-35,70-104`

**Apply to:** `internal/ado/client.go`, identity fallback tests

```go
type fakeWorkItemAPI struct {
	t *testing.T
	UpdateWorkItemFunc func(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}
```

The repository has no production struct function field today. Therefore, `fakes_test.go` is the closest and controlling local analog for a function-valued seam. Keep the new field narrow and unexported.

### Exact SDK argument assertions

**Source:** `internal/ado/pullrequests_test.go:12-47,91-116`

**Apply to:** both `git.GetPullRequestArgs` and existing `git.UpdatePullRequestArgs`

- Capture the entire argument struct at the fake boundary.
- Build the expected struct literally with independently declared pointer values.
- Use `reflect.DeepEqual`, not selected-field checks.
- Keep optional SDK fields implicitly nil in the literal.

### Error preservation

**Source:** `internal/ado/pullrequests.go:185-193`; baseline approval behavior from the phase-base diff

**Apply to:** live identity fallback

```go
if err != nil {
	return fmt.Errorf("failed to get current user: %w", err)
}
```

Test the wrapped cause with `errors.Is`. This protects both caller-visible context and programmatic unwrapping.

### Construction and cache semantics

**Source:** `internal/ado/client.go:67-108`

**Apply to:** `NewClient`

- Construct the location client once beside other SDK clients.
- Perform the existing eager identity lookup.
- Cache `AuthenticatedUser.Id` only on success.
- Keep failure non-fatal and retain the warning.
- Store a package-private callable provider for later retries.

### Test style

**Source:** `internal/utils/text_test.go:5-29`, `internal/ado/pullrequests_test.go:12-174`

- Stdlib `testing` only.
- Direct setup/action/assertion.
- Descriptive `TestX` names.
- `t.Fatalf` for prerequisites and unexpected errors; `t.Errorf` for value mismatches.
- Regression comments only where they explain a previously observed bug.

## No Analog Found

| File/Concern | Role | Data Flow | Reason |
|--------------|------|-----------|--------|
| Production `Client` function field for identity lookup | service seam | request-response | No production struct currently stores a function callback. Use the already-established func-field fake pattern and exact SDK signature; do not invent a broader provider abstraction. |
| Direct unit test of `NewClient` identity failure | constructor test | request-response | `NewClient` still constructs several concrete SDK clients and a real PAT connection. Gap closure should test fallback behavior through a struct-literal `Client`, while preserving constructor wiring by source and build verification. |

## Verification Targets

Run at minimum:

```bash
gofmt -l internal/ado/
go vet ./...
go test ./internal/ado/ -count=1 -run 'TestCompletePullRequestIssuesFetchThenMerge|TestApprovePullRequest' -v
go test ./internal/ado/... -count=1 -race -cover
go test ./... -count=1
```

Gap-specific assertions to inspect after the run:

- `TestCompletePullRequestIssuesFetchThenMerge` still checks `[]string{"get", "update"}`.
- It deep-compares both `git.GetPullRequestArgs` and `git.UpdatePullRequestArgs`.
- Cache-hit approval does not call the identity provider.
- Empty-cache approval retries the provider and can recover.
- Provider errors retain `failed to get current user: %w` semantics.
- Missing identity data retains `could not determine current user ID` and never calls Git.

## Metadata

**Analog search scope:** `internal/ado`, `internal/ui`, `internal/utils`, phase git history, pinned Azure DevOps SDK v1.0.0-b5
**Strong analogs used:** 5 concrete ranges across 6 files, including SDK source
**Pattern extraction date:** 2026-09-04
**Public API change:** none
**Dependency change:** none
