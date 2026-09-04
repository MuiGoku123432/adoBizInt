package ado

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// TestGetDashboardCountsSwallowsProjectFailure characterizes the current
// TRUST-01 behavior. Phase 2 inverts this contract so the query failure reaches
// the caller instead of making that project's work-item count look empty.
//
// Dashboard swallowing is structurally different from the other seven reads: it
// happens per call inside getProjectCounts, not at the aggregate loop's continue.
// Phase 2 must make getProjectCounts propagate each failure; grepping only for
// per-project loop continues will find seven sites and silently miss this one.
func TestGetDashboardCountsSwallowsProjectFailure(t *testing.T) {
	const (
		failingProject   = "project-query-fails"
		survivingProject = "project-query-succeeds"
	)
	const (
		failingProjectPRs   = 2
		survivingProjectPRs = 3
		survivingWorkItems  = 4
	)

	workItemRefs := make([]workitemtracking.WorkItemReference, survivingWorkItems)
	api := &fakeWorkItemAPI{
		t: t,
		GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			return nil, nil
		},
		QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			if args.Project == nil {
				t.Fatal("QueryByWiql project = nil")
			}
			switch *args.Project {
			case failingProject:
				return nil, errors.New("query failed")
			case survivingProject:
				return &workitemtracking.WorkItemQueryResult{WorkItems: &workItemRefs}, nil
			default:
				t.Fatalf("QueryByWiql project = %q, want %q or %q", *args.Project, failingProject, survivingProject)
				return nil, nil
			}
		},
	}
	gitAPI := &fakeGitAPI{
		t: t,
		GetPullRequestsByProjectFunc: func(_ context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
			if args.Project == nil {
				t.Fatal("GetPullRequestsByProject project = nil")
			}
			var count int
			switch *args.Project {
			case failingProject:
				count = failingProjectPRs
			case survivingProject:
				count = survivingProjectPRs
			default:
				t.Fatalf("GetPullRequestsByProject project = %q, want %q or %q", *args.Project, failingProject, survivingProject)
			}
			prs := make([]git.GitPullRequest, count)
			return &prs, nil
		},
	}
	client := &Client{
		projects:       []string{failingProject, survivingProject},
		workitemClient: api,
		gitClient:      gitAPI,
		buildClient:    &fakeBuildAPI{t: t},
	}

	counts, err := client.GetDashboardCounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
	}
	if counts.WorkItems != survivingWorkItems {
		t.Errorf("GetDashboardCounts().WorkItems = %d, want %d from only the surviving project", counts.WorkItems, survivingWorkItems)
	}
	// This two-project sum is load-bearing: in-place swallowing continues to the
	// failing project's PR call, while project-level skipping would lose its PRs.
	if counts.PRs != failingProjectPRs+survivingProjectPRs {
		t.Errorf("GetDashboardCounts().PRs = %d, want %d from both projects", counts.PRs, failingProjectPRs+survivingProjectPRs)
	}
}
