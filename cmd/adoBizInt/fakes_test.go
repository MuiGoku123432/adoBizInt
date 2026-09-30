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

	// fakeWriteClient is embedded so one fake satisfies the combined client
	// interface the dispatch table takes. It is nil in the read-only tests that
	// predate the write path, which is the correct behaviour: a read test that
	// somehow reached a mutation would panic on the nil receiver rather than
	// silently succeed.
	*fakeWriteClient

	GetWorkItemsFunc           func(ctx context.Context, projects []string) (ado.Result[ado.WorkItem], error)
	GetPullRequestsFunc        func(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error)
	GetPipelineRunsFunc        func(ctx context.Context, projects []string) (ado.Result[ado.PipelineRun], error)
	GetPipelineDefinitionsFunc func(ctx context.Context, projects []string) (ado.Result[ado.PipelineDefinition], error)
	GetReleasesFunc            func(ctx context.Context, projects []string) (ado.Result[ado.Release], error)
	GetPendingApprovalsFunc    func(ctx context.Context, projects []string) (ado.Result[ado.ReleaseApproval], error)
	GetBranchesFunc            func(ctx context.Context, projects, repositories []string) (ado.Result[ado.Branch], error)
	GetDashboardCountsFunc     func(ctx context.Context, projects []string) (ado.Result[ado.DashboardCounts], error)

	// currentUserEmail is a plain field rather than a closure: it stands in for
	// a cached value the real client resolved at construction, and its empty
	// zero value is the legitimate unresolved-identity case (Phase 1 D-12) that
	// --assigned-to-me must warn about rather than silently no-op on.
	currentUserEmail string
}

func (f *fakeReadClient) CurrentUserEmail() string { return f.currentUserEmail }

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

// fakeWriteClient satisfies writeClient with the same per-call closure shape as
// fakeReadClient, and for a sharper reason.
//
// The nil-closure t.Fatalf IS the "no mutation occurred" assertion. A dry-run
// test sets only GetWorkItemDetailFunc and leaves UpdateWorkItemStateFunc nil,
// so any mutation fails the test immediately, by name, at the call site. That is
// strictly better than a call counter asserted == 0: a counter records a call
// that already happened, whereas a nil field makes the call impossible to make
// silently. Do not add a counter here.
type fakeWriteClient struct {
	t *testing.T

	GetWorkItemDetailFunc   func(ctx context.Context, id int) (*ado.WorkItemDetail, error)
	UpdateWorkItemStateFunc func(ctx context.Context, project string, itemID int, newState string) error

	UpdateWorkItemStateWithEffortFunc func(ctx context.Context, project string, itemID int, newState string, originalEstimate, remaining, completed *float64) error

	GetPullRequestDetailFunc        func(ctx context.Context, project, repositoryID string, prID int) (*ado.PullRequestDetail, error)
	GetReleaseApprovalDetailFunc    func(ctx context.Context, project string, approvalID int) (*ado.ReleaseApprovalDetail, error)
	GetPipelineDefinitionDetailFunc func(ctx context.Context, project string, definitionID int) (*ado.PipelineDefinitionDetail, error)

	ApprovePullRequestFunc  func(ctx context.Context, project, repositoryID string, prID int) error
	CompletePullRequestFunc func(ctx context.Context, project, repositoryID string, prID int) error
	ApproveReleaseFunc      func(ctx context.Context, project string, approvalID int, comment string) error
	RejectReleaseFunc       func(ctx context.Context, project string, approvalID int, comment string) error
	QueuePipelineFunc       func(ctx context.Context, project string, definitionID int, branch string) (*ado.PipelineRun, error)
}

func (f *fakeWriteClient) GetWorkItemDetail(ctx context.Context, id int) (*ado.WorkItemDetail, error) {
	f.t.Helper()
	if f.GetWorkItemDetailFunc == nil {
		f.t.Fatalf("writeClient.GetWorkItemDetail called with no GetWorkItemDetailFunc set")
	}
	return f.GetWorkItemDetailFunc(ctx, id)
}

func (f *fakeWriteClient) UpdateWorkItemState(ctx context.Context, project string, itemID int, newState string) error {
	f.t.Helper()
	if f.UpdateWorkItemStateFunc == nil {
		f.t.Fatalf("writeClient.UpdateWorkItemState called with no UpdateWorkItemStateFunc set")
	}
	return f.UpdateWorkItemStateFunc(ctx, project, itemID, newState)
}

func (f *fakeWriteClient) UpdateWorkItemStateWithEffort(ctx context.Context, project string, itemID int, newState string, originalEstimate, remaining, completed *float64) error {
	f.t.Helper()
	if f.UpdateWorkItemStateWithEffortFunc == nil {
		f.t.Fatalf("writeClient.UpdateWorkItemStateWithEffort called with no UpdateWorkItemStateWithEffortFunc set")
	}
	return f.UpdateWorkItemStateWithEffortFunc(ctx, project, itemID, newState, originalEstimate, remaining, completed)
}

func (f *fakeWriteClient) GetPullRequestDetail(ctx context.Context, project, repositoryID string, prID int) (*ado.PullRequestDetail, error) {
	f.t.Helper()
	if f.GetPullRequestDetailFunc == nil {
		f.t.Fatalf("writeClient.GetPullRequestDetail called with no GetPullRequestDetailFunc set")
	}
	return f.GetPullRequestDetailFunc(ctx, project, repositoryID, prID)
}

func (f *fakeWriteClient) GetReleaseApprovalDetail(ctx context.Context, project string, approvalID int) (*ado.ReleaseApprovalDetail, error) {
	f.t.Helper()
	if f.GetReleaseApprovalDetailFunc == nil {
		f.t.Fatalf("writeClient.GetReleaseApprovalDetail called with no GetReleaseApprovalDetailFunc set")
	}
	return f.GetReleaseApprovalDetailFunc(ctx, project, approvalID)
}

func (f *fakeWriteClient) GetPipelineDefinitionDetail(ctx context.Context, project string, definitionID int) (*ado.PipelineDefinitionDetail, error) {
	f.t.Helper()
	if f.GetPipelineDefinitionDetailFunc == nil {
		f.t.Fatalf("writeClient.GetPipelineDefinitionDetail called with no GetPipelineDefinitionDetailFunc set")
	}
	return f.GetPipelineDefinitionDetailFunc(ctx, project, definitionID)
}

func (f *fakeWriteClient) ApprovePullRequest(ctx context.Context, project, repositoryID string, prID int) error {
	f.t.Helper()
	if f.ApprovePullRequestFunc == nil {
		f.t.Fatalf("writeClient.ApprovePullRequest called with no ApprovePullRequestFunc set")
	}
	return f.ApprovePullRequestFunc(ctx, project, repositoryID, prID)
}

func (f *fakeWriteClient) CompletePullRequest(ctx context.Context, project, repositoryID string, prID int) error {
	f.t.Helper()
	if f.CompletePullRequestFunc == nil {
		f.t.Fatalf("writeClient.CompletePullRequest called with no CompletePullRequestFunc set")
	}
	return f.CompletePullRequestFunc(ctx, project, repositoryID, prID)
}

func (f *fakeWriteClient) ApproveRelease(ctx context.Context, project string, approvalID int, comment string) error {
	f.t.Helper()
	if f.ApproveReleaseFunc == nil {
		f.t.Fatalf("writeClient.ApproveRelease called with no ApproveReleaseFunc set")
	}
	return f.ApproveReleaseFunc(ctx, project, approvalID, comment)
}

func (f *fakeWriteClient) RejectRelease(ctx context.Context, project string, approvalID int, comment string) error {
	f.t.Helper()
	if f.RejectReleaseFunc == nil {
		f.t.Fatalf("writeClient.RejectRelease called with no RejectReleaseFunc set")
	}
	return f.RejectReleaseFunc(ctx, project, approvalID, comment)
}

func (f *fakeWriteClient) QueuePipeline(ctx context.Context, project string, definitionID int, branch string) (*ado.PipelineRun, error) {
	f.t.Helper()
	if f.QueuePipelineFunc == nil {
		f.t.Fatalf("writeClient.QueuePipeline called with no QueuePipelineFunc set")
	}
	return f.QueuePipelineFunc(ctx, project, definitionID, branch)
}
