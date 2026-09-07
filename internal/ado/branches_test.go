package ado

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

// TestGetBranchesSwallowsProjectFailure now pins the inverted TRUST-01
// behavior. The name is retained until the phase-wide rename sweep.
func TestGetBranchesSwallowsProjectFailure(t *testing.T) {
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
// A 404 is an ordinary empty repository, not a failure; anything else is.
func TestGetBranchesReportsRepoFailure(t *testing.T) {
	const (
		project    = "project-one"
		repository = "repository-one"
	)

	t.Run("empty repository is not a failure", func(t *testing.T) {
		client := branchTestClient(t, project, repository, wrappedErrorValue(404, "no branches"))

		result, err := client.GetBranches(context.Background(), nil, nil)
		if err != nil {
			t.Fatalf("GetBranches() error = %v, want nil", err)
		}
		if len(result.Failures) != 0 {
			t.Fatalf("GetBranches() Failures = %+v, want none — an empty repository is not a broken read", result.Failures)
		}
		if !result.Complete {
			t.Errorf("GetBranches() Complete = %v, want true", result.Complete)
		}
		if len(result.Items) != 0 {
			t.Errorf("GetBranches() Items = %+v, want empty", result.Items)
		}
	})

	t.Run("server error names the repository", func(t *testing.T) {
		client := branchTestClient(t, project, repository, wrappedErrorValue(500, "internal error"))

		result, err := client.GetBranches(context.Background(), nil, nil)
		if err != nil {
			t.Fatalf("GetBranches() error = %v, want nil", err)
		}
		if len(result.Failures) != 1 {
			t.Fatalf("GetBranches() Failures = %+v, want exactly one per-repository entry", result.Failures)
		}
		if result.Failures[0].Project != project {
			t.Errorf("GetBranches() Failures[0].Project = %q, want %q", result.Failures[0].Project, project)
		}
		if !strings.Contains(result.Failures[0].Operation, repository) {
			t.Errorf("GetBranches() Failures[0].Operation = %q, want it to identify repository %q", result.Failures[0].Operation, repository)
		}
		if result.Complete {
			t.Errorf("GetBranches() Complete = %v, want false", result.Complete)
		}
	})
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
