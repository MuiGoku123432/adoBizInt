package ado

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
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

// dashboardClientFor wires several projects, each with its own sub-query
// outcomes, and visits them in the given order.
func dashboardClientFor(t *testing.T, order []string, byProject map[string]dashboardStubs) *Client {
	t.Helper()

	stubsFor := func(project string) dashboardStubs {
		stubs, ok := byProject[project]
		if !ok {
			t.Fatalf("unexpected call for project %q, want one of %v", project, order)
		}
		return stubs
	}

	return &Client{
		projects:  order,
		pipelines: []string{dashboardPipeline},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				if args.Project == nil {
					t.Fatal("GetClassificationNode project = nil")
				}
				if err := stubsFor(*args.Project).iterErr; err != nil {
					return nil, err
				}
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				if args.Project == nil {
					t.Fatal("QueryByWiql project = nil")
				}
				stubs := stubsFor(*args.Project)
				if stubs.wiqlErr != nil {
					return nil, stubs.wiqlErr
				}
				refs := make([]workitemtracking.WorkItemReference, stubs.workItems)
				return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
			},
		},
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				if args.Project == nil {
					t.Fatal("GetPullRequestsByProject project = nil")
				}
				stubs := stubsFor(*args.Project)
				if stubs.prErr != nil {
					return nil, stubs.prErr
				}
				prs := make([]git.GitPullRequest, stubs.prs)
				return &prs, nil
			},
		},
		buildClient: &fakeBuildAPI{
			t: t,
			GetBuildsFunc: func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				if args.Project == nil {
					t.Fatal("GetBuilds project = nil")
				}
				stubs := stubsFor(*args.Project)
				if stubs.buildErr != nil {
					return nil, stubs.buildErr
				}
				name := dashboardPipeline
				builds := make([]build.Build, stubs.builds)
				for i := range builds {
					builds[i].Definition = &build.DefinitionReference{Name: &name}
				}
				return &build.GetBuildsResponseValue{Value: builds}, nil
			},
		},
	}
}

// dashboardTotal unwraps the single aggregate the dashboard envelope carries.
// The dashboard's payload is one summed value rather than a list, so it rides in
// a one-element Items slice to keep the failure list and completeness flag the
// same shape as the other seven reads (D-01).
func dashboardTotal(t *testing.T, result Result[DashboardCounts]) DashboardCounts {
	t.Helper()
	if len(result.Items) != 1 {
		t.Fatalf("GetDashboardCounts() Items length = %d, want exactly 1 aggregate", len(result.Items))
	}
	return result.Items[0]
}

// Regression: a failing sub-query used to be swallowed in place inside
// getProjectCounts, so the dashboard rendered an understated total that looked
// exactly like a real one. The failing project's work-item query is now
// reported by name, its PRs still count, and the total is labelled incomplete.
//
// Dashboard swallowing is structurally different from the other seven reads: it
// happened per call inside getProjectCounts, not at the aggregate loop's continue.
func TestGetDashboardCountsReportsProjectFailure(t *testing.T) {
	const (
		failingProject   = "project-query-fails"
		survivingProject = "project-query-succeeds"
	)
	const (
		failingProjectPRs   = 2
		survivingProjectPRs = 3
		survivingWorkItems  = 4
	)

	client := dashboardClientFor(t, []string{failingProject, survivingProject}, map[string]dashboardStubs{
		failingProject:   {wiqlErr: errors.New("query failed"), prs: failingProjectPRs},
		survivingProject: {workItems: survivingWorkItems, prs: survivingProjectPRs},
	})

	result, err := client.GetDashboardCounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
	}
	counts := dashboardTotal(t, result)

	if counts.WorkItems != survivingWorkItems {
		t.Errorf("GetDashboardCounts() WorkItems = %d, want %d from only the surviving project", counts.WorkItems, survivingWorkItems)
	}
	// This two-project sum is load-bearing: in-place swallowing continues to the
	// failing project's PR call, while project-level skipping would lose its PRs.
	if counts.PRs != failingProjectPRs+survivingProjectPRs {
		t.Errorf("GetDashboardCounts() PRs = %d, want %d from both projects", counts.PRs, failingProjectPRs+survivingProjectPRs)
	}

	// A non-zero total with a failure list is the exact case D-03 exists for: the
	// number is kept and labelled understated rather than zeroed or unmarked.
	if len(result.Failures) != 1 {
		t.Fatalf("GetDashboardCounts() Failures = %v, want exactly one", failureOps(result.Failures))
	}
	if result.Failures[0].Project != failingProject {
		t.Errorf("Failures[0].Project = %q, want %q", result.Failures[0].Project, failingProject)
	}
	if result.Failures[0].Operation != opDashboardWorkItems {
		t.Errorf("Failures[0].Operation = %q, want %q", result.Failures[0].Operation, opDashboardWorkItems)
	}
	if result.Complete {
		t.Error("GetDashboardCounts() Complete = true, want false — a partial total must never report complete")
	}
}

func TestGetDashboardCountsReportsCompleteWhenEveryProjectAnswers(t *testing.T) {
	const (
		first  = "project-one"
		second = "project-two"
	)

	client := dashboardClientFor(t, []string{first, second}, map[string]dashboardStubs{
		first:  {workItems: 4, prs: 3, builds: 2},
		second: {workItems: 1, prs: 5, builds: 7},
	})

	result, err := client.GetDashboardCounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
	}
	counts := dashboardTotal(t, result)

	want := DashboardCounts{WorkItems: 5, PRs: 8, Pipelines: 9}
	if counts != want {
		t.Errorf("GetDashboardCounts() total = %+v, want %+v", counts, want)
	}
	if len(result.Failures) != 0 {
		t.Errorf("GetDashboardCounts() Failures = %v, want none", failureOps(result.Failures))
	}
	if !result.Complete {
		t.Error("GetDashboardCounts() Complete = false, want true on a clean two-project sweep")
	}
}

// TestGetDashboardCountsRecordsDegradedIterationLookup pins the decision taken on
// the fourth swallow at dashboard.go's iteration lookup. A failed lookup silently
// substitutes a date-only WIQL filter, which changes the work-item count the user
// reads as authoritative, so it is recorded as a failure rather than left silent
// (T-02-05).
func TestGetDashboardCountsRecordsDegradedIterationLookup(t *testing.T) {
	const project = "project-degraded-filter"

	client := dashboardClientFor(t, []string{project}, map[string]dashboardStubs{
		project: {iterErr: errors.New("iterations unavailable"), workItems: 4, prs: 3, builds: 2},
	})

	result, err := client.GetDashboardCounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
	}

	counts := dashboardTotal(t, result)
	if counts.WorkItems != 4 {
		t.Errorf("GetDashboardCounts() WorkItems = %d, want 4 — the degraded query still returns counts", counts.WorkItems)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("GetDashboardCounts() Failures = %v, want exactly one for the degraded iteration lookup", failureOps(result.Failures))
	}
	if result.Failures[0].Operation != opDashboardIterations {
		t.Errorf("Failures[0].Operation = %q, want %q", result.Failures[0].Operation, opDashboardIterations)
	}
	if result.Complete {
		t.Error("GetDashboardCounts() Complete = true, want false — a count taken under a substituted filter is not the whole picture")
	}
}

func TestFetchTimeoutMarksUnreachedDashboard(t *testing.T) {
	projects := []string{"project-one", "project-two"}
	client := dashboardClientFor(t, projects, map[string]dashboardStubs{
		projects[0]: {},
		projects[1]: {},
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result, err := client.GetDashboardCounts(ctx, nil)
	if err != nil {
		t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
	}

	assertAllUnreached(t, "GetDashboardCounts", result.Failures, result.Complete, projects)

	counts := dashboardTotal(t, result)
	if counts != (DashboardCounts{}) {
		t.Errorf("GetDashboardCounts() total = %+v, want a zero total when no project was reached", counts)
	}
}
