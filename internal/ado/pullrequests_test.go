package ado

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"
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

// TestGetPullRequestsSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
func TestGetPullRequestsSwallowsProjectFailure(t *testing.T) {
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

	pullRequests, err := client.GetPullRequests(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPullRequests() error = %v, want nil", err)
	}
	if len(pullRequests) != 1 {
		t.Fatalf("GetPullRequests() returned %d pull requests, want 1 surviving pull request: %+v", len(pullRequests), pullRequests)
	}
	if pullRequests[0].Project != survivingProject || pullRequests[0].Title != "surviving pull request" {
		t.Errorf("GetPullRequests()[0] = %+v, want surviving project result", pullRequests[0])
	}
}
