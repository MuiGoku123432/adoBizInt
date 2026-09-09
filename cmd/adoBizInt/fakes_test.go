package main

import (
	"context"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// fakeReadClient satisfies readClient with per-call closure fields, mirroring
// the internal/ado fake convention: an unset closure fails the test by
// interface and method name rather than returning a misleading zero value, so
// an unexpected read is a loud failure at the call site.
type fakeReadClient struct {
	t *testing.T

	GetWorkItemsFunc           func(ctx context.Context, projects []string) (ado.Result[ado.WorkItem], error)
	GetPullRequestsFunc        func(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error)
	GetPipelineRunsFunc        func(ctx context.Context, projects []string) (ado.Result[ado.PipelineRun], error)
	GetPipelineDefinitionsFunc func(ctx context.Context, projects []string) (ado.Result[ado.PipelineDefinition], error)
	GetReleasesFunc            func(ctx context.Context, projects []string) (ado.Result[ado.Release], error)
	GetPendingApprovalsFunc    func(ctx context.Context, projects []string) (ado.Result[ado.ReleaseApproval], error)
	GetBranchesFunc            func(ctx context.Context, projects, repositories []string) (ado.Result[ado.Branch], error)
	GetDashboardCountsFunc     func(ctx context.Context, projects []string) (ado.Result[ado.DashboardCounts], error)
}

func (f *fakeReadClient) GetWorkItems(ctx context.Context, projects []string) (ado.Result[ado.WorkItem], error) {
	if f.GetWorkItemsFunc == nil {
		f.t.Fatalf("readClient.GetWorkItems called with no GetWorkItemsFunc set")
	}
	return f.GetWorkItemsFunc(ctx, projects)
}

func (f *fakeReadClient) GetPullRequests(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error) {
	if f.GetPullRequestsFunc == nil {
		f.t.Fatalf("readClient.GetPullRequests called with no GetPullRequestsFunc set")
	}
	return f.GetPullRequestsFunc(ctx, projects)
}

func (f *fakeReadClient) GetPipelineRuns(ctx context.Context, projects []string) (ado.Result[ado.PipelineRun], error) {
	if f.GetPipelineRunsFunc == nil {
		f.t.Fatalf("readClient.GetPipelineRuns called with no GetPipelineRunsFunc set")
	}
	return f.GetPipelineRunsFunc(ctx, projects)
}

func (f *fakeReadClient) GetPipelineDefinitions(ctx context.Context, projects []string) (ado.Result[ado.PipelineDefinition], error) {
	if f.GetPipelineDefinitionsFunc == nil {
		f.t.Fatalf("readClient.GetPipelineDefinitions called with no GetPipelineDefinitionsFunc set")
	}
	return f.GetPipelineDefinitionsFunc(ctx, projects)
}

func (f *fakeReadClient) GetReleases(ctx context.Context, projects []string) (ado.Result[ado.Release], error) {
	if f.GetReleasesFunc == nil {
		f.t.Fatalf("readClient.GetReleases called with no GetReleasesFunc set")
	}
	return f.GetReleasesFunc(ctx, projects)
}

func (f *fakeReadClient) GetPendingApprovals(ctx context.Context, projects []string) (ado.Result[ado.ReleaseApproval], error) {
	if f.GetPendingApprovalsFunc == nil {
		f.t.Fatalf("readClient.GetPendingApprovals called with no GetPendingApprovalsFunc set")
	}
	return f.GetPendingApprovalsFunc(ctx, projects)
}

func (f *fakeReadClient) GetBranches(ctx context.Context, projects, repositories []string) (ado.Result[ado.Branch], error) {
	if f.GetBranchesFunc == nil {
		f.t.Fatalf("readClient.GetBranches called with no GetBranchesFunc set")
	}
	return f.GetBranchesFunc(ctx, projects, repositories)
}

func (f *fakeReadClient) GetDashboardCounts(ctx context.Context, projects []string) (ado.Result[ado.DashboardCounts], error) {
	if f.GetDashboardCountsFunc == nil {
		f.t.Fatalf("readClient.GetDashboardCounts called with no GetDashboardCountsFunc set")
	}
	return f.GetDashboardCountsFunc(ctx, projects)
}

// allReadsFake returns a fake whose every closure is set to an empty, complete
// result, so a test can exercise any of the eight verbs without caring which
// method the dispatch table happens to call.
func allReadsFake(t *testing.T) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t: t,
		GetWorkItemsFunc: func(context.Context, []string) (ado.Result[ado.WorkItem], error) {
			return emptyResult[ado.WorkItem](), nil
		},
		GetPullRequestsFunc: func(context.Context, []string) (ado.Result[ado.PullRequest], error) {
			return emptyResult[ado.PullRequest](), nil
		},
		GetPipelineRunsFunc: func(context.Context, []string) (ado.Result[ado.PipelineRun], error) {
			return emptyResult[ado.PipelineRun](), nil
		},
		GetPipelineDefinitionsFunc: func(context.Context, []string) (ado.Result[ado.PipelineDefinition], error) {
			return emptyResult[ado.PipelineDefinition](), nil
		},
		GetReleasesFunc: func(context.Context, []string) (ado.Result[ado.Release], error) {
			return emptyResult[ado.Release](), nil
		},
		GetPendingApprovalsFunc: func(context.Context, []string) (ado.Result[ado.ReleaseApproval], error) {
			return emptyResult[ado.ReleaseApproval](), nil
		},
		GetBranchesFunc: func(context.Context, []string, []string) (ado.Result[ado.Branch], error) {
			return emptyResult[ado.Branch](), nil
		},
		GetDashboardCountsFunc: func(context.Context, []string) (ado.Result[ado.DashboardCounts], error) {
			return emptyResult[ado.DashboardCounts](), nil
		},
	}
}

// emptyResult is the shape a real read produces for configured projects with
// nothing to report: both slices present and empty, never nil.
func emptyResult[T any]() ado.Result[T] {
	return ado.Result[T]{Items: []T{}, Failures: []ado.Failure{}, Complete: true}
}
