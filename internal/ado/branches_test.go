package ado

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

// Regression: a project whose repository listing failed used to be skipped
// silently, so a caller could not tell a failed read from a project with no
// branches. The failure is now named in the envelope and the result is marked
// incomplete.
func TestGetBranchesReportsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-repositories-fail"
		survivingProject = "project-repositories-succeed"
	)

	client := &Client{
		projects: []string{failedProject, survivingProject},
		gitClient: &fakeGitAPI{
			t: t,
			GetRepositoriesFunc: func(_ context.Context, args git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
				repositoryID := uuid.UUID{1}
				repositoryName := "repository-one"
				defaultBranch := "refs/heads/main"
				repositories := []git.GitRepository{{
					Id:            &repositoryID,
					Name:          &repositoryName,
					DefaultBranch: &defaultBranch,
				}}
				if args.Project != nil && *args.Project == failedProject {
					return &repositories, errors.New("repositories failed")
				}
				return &repositories, nil
			},
			GetBranchesFunc: func(_ context.Context, args git.GetBranchesArgs) (*[]git.GitBranchStats, error) {
				branchName := "main"
				if args.Project != nil && *args.Project == failedProject {
					branchName = "poisoned-failed-project-branch"
				}
				branches := []git.GitBranchStats{{Name: &branchName}}
				return &branches, nil
			},
		},
	}

	result, err := client.GetBranches(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GetBranches() error = %v, want nil", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("GetBranches() returned %d branches, want 1 surviving branch: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].Project != survivingProject || result.Items[0].Name != "main" {
		t.Errorf("GetBranches()[0] = %+v, want project %q and branch main", result.Items[0], survivingProject)
	}
	if len(result.Failures) != 1 || result.Failures[0].Project != failedProject || result.Failures[0].Operation == "" {
		t.Fatalf("GetBranches() Failures = %+v, want exactly one named entry for %q", result.Failures, failedProject)
	}
	if result.Complete {
		t.Errorf("GetBranches() Complete = %v, want false", result.Complete)
	}
}

// branchTestClient builds a single-project client with one repository whose
// branch-stats call behaves per the supplied hook.
func branchTestClient(t *testing.T, project, repository string, statsErr error) *Client {
	t.Helper()
	return &Client{
		projects: []string{project},
		gitClient: &fakeGitAPI{
			t: t,
			GetRepositoriesFunc: func(_ context.Context, _ git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
				repositoryID := uuid.UUID{1}
				name := repository
				defaultBranch := "refs/heads/main"
				return &[]git.GitRepository{{
					Id:            &repositoryID,
					Name:          &name,
					DefaultBranch: &defaultBranch,
				}}, nil
			},
			GetBranchesFunc: func(_ context.Context, _ git.GetBranchesArgs) (*[]git.GitBranchStats, error) {
				if statsErr != nil {
					return nil, statsErr
				}
				branchName := "main"
				return &[]git.GitBranchStats{{Name: &branchName}}, nil
			},
		},
	}
}

// TestGetBranchesReportsRepoFailure covers the nested per-repository swallow.
// D-06a: ADO answers an empty repository with HTTP 400 and a message beginning
// "VS403403:" (live capture against the org on 2026-10-12: typeKey
// ArgumentException, errorCode 0). Only that conjunction is an ordinary empty
// repository; every other outcome on the call is a failure naming the
// repository, including VS403403 text under any other status or with none.
func TestGetBranchesReportsRepoFailure(t *testing.T) {
	const (
		project    = "project-one"
		repository = "repository-one"
		vs403      = "VS403403: Cannot find any branches for the repository-one repository."
	)

	// The live capture carries TypeKey and ErrorCode; the shared helpers set only
	// Message and StatusCode, so the two 400 ArgumentException rows are spelled out.
	liveError := func(message string) error {
		status := 400
		typeKey := "ArgumentException"
		errorCode := 0
		return azuredevops.WrappedError{
			Message:    &message,
			TypeKey:    &typeKey,
			ErrorCode:  &errorCode,
			StatusCode: &status,
		}
	}

	// D-06b: a disabled repository is still enumerated by GetRepositories, and
	// its branch-stats call answers 404 with typeKey GitRepositoryNotFoundException
	// (live capture, 2026-10-12, 130 repositories across both projects).
	typedError := func(status int, typeKey, message string) azuredevops.WrappedError {
		return azuredevops.WrappedError{Message: &message, TypeKey: &typeKey, StatusCode: &status}
	}
	tf401019 := "TF401019: The Git repository with name or identifier repository-one does not exist or you do not have permissions for the operation you are attempting."
	disabled := typedError(404, "GitRepositoryNotFoundException", tf401019)

	tests := []struct {
		name     string
		err      error
		recorded bool
		kind     FailureKind
	}{
		{name: "disabled repository is skipped", err: disabled},
		{name: "disabled repository pointer form is skipped", err: &disabled},
		{name: "404 with another typeKey is recorded", err: typedError(404, "ProjectDoesNotExistException", tf401019), recorded: true, kind: FailureOther},
		{name: "repository-not-found typeKey under 400 is recorded", err: typedError(400, "GitRepositoryNotFoundException", tf401019), recorded: true, kind: FailureOther},
		{name: "repository-not-found typeKey on a rejected token stays auth", err: typedError(403, "GitRepositoryNotFoundException", tf401019), recorded: true, kind: FailureAuth},
		{name: "live VS403403 capture is an empty repository", err: liveError(vs403)},
		{name: "VS403403 pointer form is an empty repository", err: wrappedErrorPointer(400, vs403)},
		{
			name:     "other 400 ArgumentException is recorded",
			err:      liveError("TF401019: The Git repository with name or identifier repository-one does not exist or you do not have permissions for the operation you are attempting."),
			recorded: true,
			kind:     FailureOther,
		},
		{name: "VS403403 under 404 is recorded", err: wrappedErrorValue(404, vs403), recorded: true, kind: FailureOther},
		{name: "VS403403 text on a rejected token stays auth", err: wrappedErrorValue(403, vs403), recorded: true, kind: FailureAuth},
		{name: "rejected token is auth", err: wrappedErrorValue(401, "TF400813: The user is not authorized"), recorded: true, kind: FailureAuth},
		{name: "VS403403 text without an SDK status is recorded", err: errors.New(vs403), recorded: true, kind: FailureOther},
		{name: "timeout is recorded", err: context.DeadlineExceeded, recorded: true, kind: FailureTimeout},
		{name: "server error names the repository", err: wrappedErrorValue(500, "internal error"), recorded: true, kind: FailureOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := branchTestClient(t, project, repository, tt.err)

			result, err := client.GetBranches(context.Background(), nil, nil)
			if err != nil {
				t.Fatalf("GetBranches() error = %v, want nil", err)
			}
			if len(result.Items) != 0 {
				t.Errorf("GetBranches() Items = %+v, want empty", result.Items)
			}

			if !tt.recorded {
				if len(result.Failures) != 0 {
					t.Fatalf("GetBranches() Failures = %+v, want none: an empty repository is not a broken read", result.Failures)
				}
				if !result.Complete {
					t.Errorf("GetBranches() Complete = %v, want true", result.Complete)
				}
				return
			}

			if len(result.Failures) != 1 {
				t.Fatalf("GetBranches() Failures = %+v, want exactly one per-repository entry", result.Failures)
			}
			got := result.Failures[0]
			if got.Project != project {
				t.Errorf("GetBranches() Failures[0].Project = %q, want %q", got.Project, project)
			}
			if !strings.Contains(got.Operation, repository) {
				t.Errorf("GetBranches() Failures[0].Operation = %q, want it to identify repository %q", got.Operation, repository)
			}
			if got.Kind != tt.kind {
				t.Errorf("GetBranches() Failures[0].Kind = %q, want %q", got.Kind, tt.kind)
			}
			if result.Complete {
				t.Errorf("GetBranches() Complete = %v, want false", result.Complete)
			}
		})
	}
}

func TestGetBranchesClassifiesRejectedToken(t *testing.T) {
	client := &Client{
		projects: []string{"project-unauthorized"},
		gitClient: &fakeGitAPI{
			t: t,
			GetRepositoriesFunc: func(_ context.Context, _ git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
				return nil, wrappedErrorValue(401, "TF400813: The user is not authorized")
			},
		},
	}

	result, err := client.GetBranches(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GetBranches() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetBranches() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 || result.Failures[0].Kind != FailureAuth {
		t.Fatalf("GetBranches() Failures = %+v, want one auth entry", result.Failures)
	}
	if result.Complete {
		t.Errorf("GetBranches() Complete = %v, want false", result.Complete)
	}
}

// TestFetchTimeoutMarksUnreachedBranches is D-12's gate for GetBranches.
func TestFetchTimeoutMarksUnreachedBranches(t *testing.T) {
	projects := []string{"project-one", "project-two", "project-three"}

	client := &Client{
		projects: projects,
		gitClient: &fakeGitAPI{
			t: t,
			GetRepositoriesFunc: func(_ context.Context, _ git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
				t.Errorf("GetRepositories called after the deadline elapsed; want no doomed calls issued")
				return nil, nil
			},
		},
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result, err := client.GetBranches(ctx, nil, nil)
	if err != nil {
		t.Fatalf("GetBranches() error = %v, want nil", err)
	}
	assertAllUnreached(t, "GetBranches", result.Failures, result.Complete, projects)
}
