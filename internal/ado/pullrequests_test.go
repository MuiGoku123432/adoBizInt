package ado

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/location"
)

func TestApprovePullRequestIssuesExactReviewerCall(t *testing.T) {
	var got git.CreatePullRequestReviewerArgs
	client := &Client{
		currentUserID: "authenticated-user-id",
		getConnectionData: func(context.Context, location.GetConnectionDataArgs) (*location.ConnectionData, error) {
			t.Fatal("getConnectionData called for cached current user ID")
			return nil, nil
		},
		gitClient: &fakeGitAPI{
			t: t,
			CreatePullRequestReviewerFunc: func(_ context.Context, args git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error) {
				got = args
				return &git.IdentityRefWithVote{}, nil
			},
		},
	}

	if err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42); err != nil {
		t.Fatalf("ApprovePullRequest() error = %v, want nil", err)
	}

	wantVote := 10
	wantProject := "project-one"
	wantRepositoryID := "repository-one"
	wantPullRequestID := 42
	wantReviewerID := "authenticated-user-id"
	want := git.CreatePullRequestReviewerArgs{
		Reviewer: &git.IdentityRefWithVote{
			Vote: &wantVote,
		},
		RepositoryId:  &wantRepositoryID,
		PullRequestId: &wantPullRequestID,
		ReviewerId:    &wantReviewerID,
		Project:       &wantProject,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("CreatePullRequestReviewer args = %+v, want %+v", got, want)
	}
}

func TestApprovePullRequestRetriesIdentityLookupAfterTransientFailure(t *testing.T) {
	lookupErr := errors.New("identity lookup unavailable")
	authenticatedID := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	providerCalls := 0
	var reviewerCalls []git.CreatePullRequestReviewerArgs

	client := &Client{
		getConnectionData: func(_ context.Context, _ location.GetConnectionDataArgs) (*location.ConnectionData, error) {
			providerCalls++
			if providerCalls == 1 {
				return nil, lookupErr
			}
			return &location.ConnectionData{
				AuthenticatedUser: &identity.Identity{Id: &authenticatedID},
			}, nil
		},
		gitClient: &fakeGitAPI{
			t: t,
			CreatePullRequestReviewerFunc: func(_ context.Context, args git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error) {
				reviewerCalls = append(reviewerCalls, args)
				return &git.IdentityRefWithVote{}, nil
			},
		},
	}

	err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("ApprovePullRequest() error = %v, want wrapped %v", err, lookupErr)
	}
	if got, want := err.Error(), "failed to get current user: identity lookup unavailable"; got != want {
		t.Errorf("ApprovePullRequest() error = %q, want %q", got, want)
	}
	if providerCalls != 1 || len(reviewerCalls) != 0 {
		t.Fatalf("after attempt 1 provider/Git calls = %d/%d, want 1/0", providerCalls, len(reviewerCalls))
	}

	if err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42); err != nil {
		t.Fatalf("ApprovePullRequest() attempt 2 error = %v, want nil", err)
	}
	if providerCalls != 2 || len(reviewerCalls) != 1 {
		t.Fatalf("after attempt 2 provider/Git calls = %d/%d, want 2/1", providerCalls, len(reviewerCalls))
	}

	if err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42); err != nil {
		t.Fatalf("ApprovePullRequest() attempt 3 error = %v, want nil", err)
	}
	if providerCalls != 3 || len(reviewerCalls) != 2 {
		t.Fatalf("after attempt 3 provider/Git calls = %d/%d, want 3/2", providerCalls, len(reviewerCalls))
	}

	for i, got := range reviewerCalls {
		wantVote := 10
		wantProject := "project-one"
		wantRepositoryID := "repository-one"
		wantPullRequestID := 42
		wantReviewerID := authenticatedID.String()
		want := git.CreatePullRequestReviewerArgs{
			Reviewer: &git.IdentityRefWithVote{
				Vote: &wantVote,
			},
			RepositoryId:  &wantRepositoryID,
			PullRequestId: &wantPullRequestID,
			ReviewerId:    &wantReviewerID,
			Project:       &wantProject,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("CreatePullRequestReviewer attempt %d args = %+v, want %+v", i+2, got, want)
		}
	}
}

func TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit(t *testing.T) {
	client := &Client{
		getConnectionData: nil,
		gitClient:         &fakeGitAPI{t: t},
	}

	err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42)
	if err == nil {
		t.Fatal("ApprovePullRequest() error = nil, want non-nil")
	}
	if got, want := err.Error(), "could not determine current user ID"; got != want {
		t.Errorf("ApprovePullRequest() error = %q, want %q", got, want)
	}
}

func TestApprovePullRequestWithoutAuthenticatedUserIDDoesNotCallGit(t *testing.T) {
	providerCalls := 0
	client := &Client{
		getConnectionData: func(_ context.Context, _ location.GetConnectionDataArgs) (*location.ConnectionData, error) {
			providerCalls++
			return &location.ConnectionData{AuthenticatedUser: &identity.Identity{}}, nil
		},
		gitClient: &fakeGitAPI{t: t},
	}

	err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42)
	if err == nil {
		t.Fatal("ApprovePullRequest() error = nil, want non-nil")
	}
	if got, want := err.Error(), "could not determine current user ID"; got != want {
		t.Errorf("ApprovePullRequest() error = %q, want %q", got, want)
	}
	if providerCalls != 1 {
		t.Errorf("getConnectionData calls = %d, want 1", providerCalls)
	}
}

func TestCompletePullRequestIssuesFetchThenMerge(t *testing.T) {
	var gotFetch git.GetPullRequestArgs
	var gotUpdate git.UpdatePullRequestArgs
	var calls []string
	commitID := "source-commit-id"
	client := &Client{
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestFunc: func(_ context.Context, args git.GetPullRequestArgs) (*git.GitPullRequest, error) {
				gotFetch = args
				calls = append(calls, "get")
				return &git.GitPullRequest{
					LastMergeSourceCommit: &git.GitCommitRef{
						CommitId: &commitID,
					},
				}, nil
			},
			UpdatePullRequestFunc: func(_ context.Context, args git.UpdatePullRequestArgs) (*git.GitPullRequest, error) {
				calls = append(calls, "update")
				gotUpdate = args
				return &git.GitPullRequest{}, nil
			},
		},
	}

	if err := client.CompletePullRequest(context.Background(), "project-one", "repository-one", 42); err != nil {
		t.Fatalf("CompletePullRequest() error = %v, want nil", err)
	}

	wantCalls := []string{"get", "update"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("call order = %v, want %v", calls, wantCalls)
	}

	wantFetchProject := "project-one"
	wantFetchRepositoryID := "repository-one"
	wantFetchPullRequestID := 42
	wantFetch := git.GetPullRequestArgs{
		RepositoryId:  &wantFetchRepositoryID,
		PullRequestId: &wantFetchPullRequestID,
		Project:       &wantFetchProject,
	}
	if !reflect.DeepEqual(gotFetch, wantFetch) {
		t.Errorf("GetPullRequest args = %+v, want %+v", gotFetch, wantFetch)
	}

	wantProject := "project-one"
	wantRepositoryID := "repository-one"
	wantPullRequestID := 42
	wantDeleteSourceBranch := false
	wantMergeStrategy := git.GitPullRequestMergeStrategyValues.NoFastForward
	wantStatus := git.PullRequestStatusValues.Completed
	wantCommitID := "source-commit-id"
	wantUpdate := git.UpdatePullRequestArgs{
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

	if !reflect.DeepEqual(gotUpdate, wantUpdate) {
		t.Errorf("UpdatePullRequest args = %+v, want %+v", gotUpdate, wantUpdate)
	}
}

func TestCompletePullRequestWithNilMergeCommitReturnsError(t *testing.T) {
	client := &Client{
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestFunc: func(_ context.Context, _ git.GetPullRequestArgs) (*git.GitPullRequest, error) {
				return &git.GitPullRequest{}, nil
			},
		},
	}

	if err := client.CompletePullRequest(context.Background(), "project-one", "repository-one", 42); err == nil {
		t.Fatal("CompletePullRequest() error = nil, want non-nil")
	}
}

// Regression: a project whose pull request query failed used to be dropped
// silently, so an expired token was indistinguishable from an empty backlog.
// The failure now reaches the caller by name and the result is incomplete.
func TestGetPullRequestsReportsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-pull-requests-fail"
		survivingProject = "project-pull-requests-succeed"
	)

	client := &Client{
		projects: []string{failedProject, survivingProject},
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				pullRequestID := 42
				pullRequestTitle := "surviving pull request"
				if args.Project != nil && *args.Project == failedProject {
					pullRequestTitle = "poisoned failed-project pull request"
				}
				pullRequests := []git.GitPullRequest{{
					PullRequestId: &pullRequestID,
					Title:         &pullRequestTitle,
				}}
				if args.Project != nil && *args.Project == failedProject {
					return &pullRequests, errors.New("pull requests failed")
				}
				return &pullRequests, nil
			},
		},
	}

	result, err := client.GetPullRequests(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPullRequests() error = %v, want nil", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("GetPullRequests() returned %d pull requests, want 1 surviving pull request: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].Project != survivingProject || result.Items[0].Title != "surviving pull request" {
		t.Errorf("GetPullRequests()[0] = %+v, want surviving project result", result.Items[0])
	}
	if len(result.Failures) != 1 {
		t.Fatalf("GetPullRequests() Failures = %+v, want exactly one entry for the failing project", result.Failures)
	}
	if result.Failures[0].Project != failedProject {
		t.Errorf("GetPullRequests() Failures[0].Project = %v, want %v", result.Failures[0].Project, failedProject)
	}
	if result.Failures[0].Operation == "" {
		t.Errorf("GetPullRequests() Failures[0].Operation = %q, want a named operation", result.Failures[0].Operation)
	}
	if result.Complete {
		t.Errorf("GetPullRequests() Complete = %v, want false when a project failed", result.Complete)
	}
}

func TestGetPullRequestsClassifiesRejectedToken(t *testing.T) {
	const failedProject = "project-pull-requests-unauthorized"

	client := &Client{
		projects: []string{failedProject},
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, _ git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				return nil, wrappedErrorValue(401, "TF400813: The user is not authorized")
			},
		},
	}

	result, err := client.GetPullRequests(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPullRequests() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetPullRequests() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("GetPullRequests() Failures = %+v, want one entry — empty must be distinguishable from failed", result.Failures)
	}
	if result.Failures[0].Kind != FailureAuth {
		t.Errorf("GetPullRequests() Failures[0].Kind = %v, want %v", result.Failures[0].Kind, FailureAuth)
	}
	if result.Complete {
		t.Errorf("GetPullRequests() Complete = %v, want false", result.Complete)
	}
}

func TestGetPullRequestsReportsCompleteWhenEveryProjectAnswers(t *testing.T) {
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
	if err != nil {
		t.Fatalf("GetPullRequests() error = %v, want nil", err)
	}
	if !result.Complete {
		t.Errorf("GetPullRequests() Complete = %v, want true", result.Complete)
	}
	if len(result.Failures) != 0 {
		t.Errorf("GetPullRequests() Failures = %+v, want none", result.Failures)
	}
}

// TestGetPullRequestsScope pins D-01 and audit W4 for prs list: the envelope
// discloses the active-only filter, equal to the status the request sends.
func TestGetPullRequestsScope(t *testing.T) {
	const want = `{"pr_status":"active"}`

	t.Run("scope matches the request", func(t *testing.T) {
		var gotStatus *git.PullRequestStatus
		client := &Client{
			projects: []string{"project-one", "project-two"},
			gitClient: &fakeGitAPI{
				t: t,
				GetPullRequestsByProjectFunc: func(_ context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
					gotStatus = args.SearchCriteria.Status
					return &[]git.GitPullRequest{}, nil
				},
			},
		}

		result, err := client.GetPullRequests(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetPullRequests() error = %v, want nil", err)
		}
		if result.Scope == nil {
			t.Fatal("Scope is nil, want the PR status filter")
		}
		raw, err := json.Marshal(result.Scope)
		if err != nil {
			t.Fatalf("marshal scope: %v", err)
		}
		if string(raw) != want {
			t.Errorf("scope = %s, want %s", raw, want)
		}
		if gotStatus == nil || string(*gotStatus) != result.Scope.PRStatus {
			t.Errorf("SearchCriteria.Status = %v, want %q", gotStatus, result.Scope.PRStatus)
		}
		if !result.Complete || len(result.Failures) != 0 {
			t.Errorf("Complete = %v, Failures = %+v, want true and none", result.Complete, result.Failures)
		}
	})

	t.Run("no projects still carries the scope", func(t *testing.T) {
		result, err := (&Client{}).GetPullRequests(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetPullRequests() error = %v, want nil", err)
		}
		raw, err := json.Marshal(result.Scope)
		if err != nil {
			t.Fatalf("marshal scope: %v", err)
		}
		if string(raw) != want {
			t.Errorf("scope = %s, want %s", raw, want)
		}
	})
}

// TestFetchTimeoutMarksUnreached is D-12's only mechanical gate in this phase.
func TestFetchTimeoutMarksUnreached(t *testing.T) {
	projects := []string{"project-one", "project-two", "project-three"}

	client := &Client{
		projects: projects,
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, _ git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				t.Errorf("GetPullRequestsByProject called after the deadline elapsed; want no doomed calls issued")
				return nil, nil
			},
		},
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result, err := client.GetPullRequests(ctx, nil)
	if err != nil {
		t.Fatalf("GetPullRequests() error = %v, want nil", err)
	}
	if len(result.Failures) != len(projects) {
		t.Fatalf("GetPullRequests() Failures length = %d, want %d — one per unreached project", len(result.Failures), len(projects))
	}
	for i, failure := range result.Failures {
		if failure.Kind != FailureTimeout {
			t.Errorf("GetPullRequests() Failures[%d].Kind = %v, want %v", i, failure.Kind, FailureTimeout)
		}
		if failure.Project != projects[i] {
			t.Errorf("GetPullRequests() Failures[%d].Project = %v, want %v — failures follow configured iteration order", i, failure.Project, projects[i])
		}
	}
	if result.Complete {
		t.Errorf("GetPullRequests() Complete = %v, want false", result.Complete)
	}
}
