package ado

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// dashboardStubs wires one project's three sub-queries plus the iteration
// lookup, so a test can fail exactly one of them and leave the rest healthy.
// This is the shape the dashboard's partial-count contract (D-03) has to be
// exercised against: no other read in this package can answer two of three.
type dashboardStubs struct {
	iterErr  error
	wiqlErr  error
	prErr    error
	buildErr error

	workItems int
	prs       int
	builds    int
}

const dashboardPipeline = "release-pipeline"

func (s dashboardStubs) client(t *testing.T, projects ...string) *Client {
	t.Helper()
	return &Client{
		projects:  projects,
		pipelines: []string{dashboardPipeline},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				if s.iterErr != nil {
					return nil, s.iterErr
				}
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				if s.wiqlErr != nil {
					return nil, s.wiqlErr
				}
				refs := make([]workitemtracking.WorkItemReference, s.workItems)
				return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
			},
		},
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, _ git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				if s.prErr != nil {
					return nil, s.prErr
				}
				prs := make([]git.GitPullRequest, s.prs)
				return &prs, nil
			},
		},
		buildClient: &fakeBuildAPI{
			t: t,
			GetBuildsFunc: func(_ context.Context, _ build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				if s.buildErr != nil {
					return nil, s.buildErr
				}
				name := dashboardPipeline
				builds := make([]build.Build, s.builds)
				for i := range builds {
					builds[i].Definition = &build.DefinitionReference{Name: &name}
				}
				return &build.GetBuildsResponseValue{Value: builds}, nil
			},
		},
	}
}

func failureOps(failures []Failure) []string {
	ops := make([]string, len(failures))
	for i, f := range failures {
		ops[i] = f.Operation
	}
	return ops
}

// TestGetProjectCountsReportsPartialFailures pins D-03's contract at the one
// place in the repo that needs it: a helper returning the counts it did collect
// alongside the failures it hit. A project that answered two of three sub-queries
// contributes those two rather than being discarded whole.
func TestGetProjectCountsReportsPartialFailures(t *testing.T) {
	boom := errors.New("sub-query failed")

	tests := []struct {
		name         string
		stubs        dashboardStubs
		wantCounts   DashboardCounts
		wantFailures []string
	}{
		{
			name:         "work item query fails, PR and build survive",
			stubs:        dashboardStubs{wiqlErr: boom, prs: 3, builds: 2},
			wantCounts:   DashboardCounts{WorkItems: 0, PRs: 3, Pipelines: 2},
			wantFailures: []string{opDashboardWorkItems},
		},
		{
			name:         "PR fetch fails, work item count survives",
			stubs:        dashboardStubs{prErr: boom, workItems: 5, builds: 2},
			wantCounts:   DashboardCounts{WorkItems: 5, PRs: 0, Pipelines: 2},
			wantFailures: []string{opDashboardPullRequests},
		},
		{
			name:         "build count fails, work items and PRs survive",
			stubs:        dashboardStubs{buildErr: boom, workItems: 5, prs: 3},
			wantCounts:   DashboardCounts{WorkItems: 5, PRs: 3, Pipelines: 0},
			wantFailures: []string{opDashboardBuilds},
		},
		{
			name:       "all three sub-queries fail",
			stubs:      dashboardStubs{wiqlErr: boom, prErr: boom, buildErr: boom},
			wantCounts: DashboardCounts{},
			wantFailures: []string{
				opDashboardWorkItems,
				opDashboardPullRequests,
				opDashboardBuilds,
			},
		},
		{
			name:         "all three sub-queries succeed",
			stubs:        dashboardStubs{workItems: 5, prs: 3, builds: 2},
			wantCounts:   DashboardCounts{WorkItems: 5, PRs: 3, Pipelines: 2},
			wantFailures: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.stubs.client(t, "proj")

			counts, failures := client.getProjectCounts(context.Background(), "proj")

			if counts == nil {
				t.Fatal("getProjectCounts() counts = nil, want a non-nil value on every path")
			}
			if *counts != tt.wantCounts {
				t.Errorf("getProjectCounts() counts = %+v, want %+v", *counts, tt.wantCounts)
			}

			gotOps := failureOps(failures)
			if len(gotOps) != len(tt.wantFailures) {
				t.Fatalf("getProjectCounts() failure operations = %v, want %v", gotOps, tt.wantFailures)
			}
			for i, want := range tt.wantFailures {
				if gotOps[i] != want {
					t.Errorf("failure[%d].Operation = %q, want %q", i, gotOps[i], want)
				}
			}

			seen := map[string]bool{}
			for _, op := range gotOps {
				if seen[op] {
					t.Errorf("failure operation %q reported twice; the three sub-queries must be distinguishable", op)
				}
				seen[op] = true
			}
			for _, f := range failures {
				if f.Project != "proj" {
					t.Errorf("failure.Project = %q, want %q", f.Project, "proj")
				}
			}
		})
	}
}

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
