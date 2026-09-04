package ado

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

// TestGetBranchesSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
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

	branches, err := client.GetBranches(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("GetBranches() error = %v, want nil", err)
	}
	if len(branches) != 1 {
		t.Fatalf("GetBranches() returned %d branches, want 1 surviving branch: %+v", len(branches), branches)
	}
	if branches[0].Project != survivingProject || branches[0].Name != "main" {
		t.Errorf("GetBranches()[0] = %+v, want project %q and branch main", branches[0], survivingProject)
	}
}
