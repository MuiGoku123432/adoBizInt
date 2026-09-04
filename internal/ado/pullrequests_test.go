package ado

import (
	"context"
	"reflect"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

func TestApprovePullRequestIssuesExactReviewerCall(t *testing.T) {
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

func TestApprovePullRequestWithoutCurrentUserIDDoesNotCallGit(t *testing.T) {
	client := &Client{
		gitClient: &fakeGitAPI{t: t},
	}

	if err := client.ApprovePullRequest(context.Background(), "project-one", "repository-one", 42); err == nil {
		t.Fatal("ApprovePullRequest() error = nil, want non-nil")
	}
}

func TestCompletePullRequestIssuesFetchThenMerge(t *testing.T) {
	var got git.UpdatePullRequestArgs
	var calls []string
	commitID := "source-commit-id"
	client := &Client{
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestFunc: func(_ context.Context, _ git.GetPullRequestArgs) (*git.GitPullRequest, error) {
				calls = append(calls, "get")
				return &git.GitPullRequest{
					LastMergeSourceCommit: &git.GitCommitRef{
						CommitId: &commitID,
					},
				}, nil
			},
			UpdatePullRequestFunc: func(_ context.Context, args git.UpdatePullRequestArgs) (*git.GitPullRequest, error) {
				calls = append(calls, "update")
				got = args
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

	wantProject := "project-one"
	wantRepositoryID := "repository-one"
	wantPullRequestID := 42
	wantDeleteSourceBranch := false
	wantMergeStrategy := git.GitPullRequestMergeStrategyValues.NoFastForward
	wantStatus := git.PullRequestStatusValues.Completed
	wantCommitID := "source-commit-id"
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
