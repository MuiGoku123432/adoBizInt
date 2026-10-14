// Package guardtest provides Client, a fake guard.Client for tests. It is shared
// by the guard's own tests and the TUI view test packages (rule of three met);
// cmd/adoBizInt keeps its own locked fakeWriteClient so the CLI tests stay
// byte-identical (GUARD-06).
//
// The nil-func t.Fatalf IS the "no mutation occurred" assertion: a test sets
// only the funcs it expects to be called, so any other call fails the test
// immediately, by name, at the call site. That is strictly better than a call
// counter asserted == 0, which records a call that already happened. Do not add
// counters here.
//
// The package must not import internal/guard: guard's in-package tests import
// guardtest, and a cycle would not compile. The conformance assertion lives in
// internal/guard/guard_test.go instead.
package guardtest

import (
	"context"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// Client satisfies guard.Client with one optional func per method.
type Client struct {
	T *testing.T

	GetWorkItemDetailFunc             func(ctx context.Context, id int) (*ado.WorkItemDetail, error)
	UpdateWorkItemStateFunc           func(ctx context.Context, project string, itemID int, newState string) error
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

func (c *Client) GetWorkItemDetail(ctx context.Context, id int) (*ado.WorkItemDetail, error) {
	c.T.Helper()
	if c.GetWorkItemDetailFunc == nil {
		c.T.Fatalf("guard.Client.GetWorkItemDetail called with no GetWorkItemDetailFunc set")
	}
	return c.GetWorkItemDetailFunc(ctx, id)
}

func (c *Client) UpdateWorkItemState(ctx context.Context, project string, itemID int, newState string) error {
	c.T.Helper()
	if c.UpdateWorkItemStateFunc == nil {
		c.T.Fatalf("guard.Client.UpdateWorkItemState called with no UpdateWorkItemStateFunc set")
	}
	return c.UpdateWorkItemStateFunc(ctx, project, itemID, newState)
}

func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, project string, itemID int, newState string, originalEstimate, remaining, completed *float64) error {
	c.T.Helper()
	if c.UpdateWorkItemStateWithEffortFunc == nil {
		c.T.Fatalf("guard.Client.UpdateWorkItemStateWithEffort called with no UpdateWorkItemStateWithEffortFunc set")
	}
	return c.UpdateWorkItemStateWithEffortFunc(ctx, project, itemID, newState, originalEstimate, remaining, completed)
}

func (c *Client) GetPullRequestDetail(ctx context.Context, project, repositoryID string, prID int) (*ado.PullRequestDetail, error) {
	c.T.Helper()
	if c.GetPullRequestDetailFunc == nil {
		c.T.Fatalf("guard.Client.GetPullRequestDetail called with no GetPullRequestDetailFunc set")
	}
	return c.GetPullRequestDetailFunc(ctx, project, repositoryID, prID)
}

func (c *Client) GetReleaseApprovalDetail(ctx context.Context, project string, approvalID int) (*ado.ReleaseApprovalDetail, error) {
	c.T.Helper()
	if c.GetReleaseApprovalDetailFunc == nil {
		c.T.Fatalf("guard.Client.GetReleaseApprovalDetail called with no GetReleaseApprovalDetailFunc set")
	}
	return c.GetReleaseApprovalDetailFunc(ctx, project, approvalID)
}

func (c *Client) GetPipelineDefinitionDetail(ctx context.Context, project string, definitionID int) (*ado.PipelineDefinitionDetail, error) {
	c.T.Helper()
	if c.GetPipelineDefinitionDetailFunc == nil {
		c.T.Fatalf("guard.Client.GetPipelineDefinitionDetail called with no GetPipelineDefinitionDetailFunc set")
	}
	return c.GetPipelineDefinitionDetailFunc(ctx, project, definitionID)
}

func (c *Client) ApprovePullRequest(ctx context.Context, project, repositoryID string, prID int) error {
	c.T.Helper()
	if c.ApprovePullRequestFunc == nil {
		c.T.Fatalf("guard.Client.ApprovePullRequest called with no ApprovePullRequestFunc set")
	}
	return c.ApprovePullRequestFunc(ctx, project, repositoryID, prID)
}

func (c *Client) CompletePullRequest(ctx context.Context, project, repositoryID string, prID int) error {
	c.T.Helper()
	if c.CompletePullRequestFunc == nil {
		c.T.Fatalf("guard.Client.CompletePullRequest called with no CompletePullRequestFunc set")
	}
	return c.CompletePullRequestFunc(ctx, project, repositoryID, prID)
}

func (c *Client) ApproveRelease(ctx context.Context, project string, approvalID int, comment string) error {
	c.T.Helper()
	if c.ApproveReleaseFunc == nil {
		c.T.Fatalf("guard.Client.ApproveRelease called with no ApproveReleaseFunc set")
	}
	return c.ApproveReleaseFunc(ctx, project, approvalID, comment)
}

func (c *Client) RejectRelease(ctx context.Context, project string, approvalID int, comment string) error {
	c.T.Helper()
	if c.RejectReleaseFunc == nil {
		c.T.Fatalf("guard.Client.RejectRelease called with no RejectReleaseFunc set")
	}
	return c.RejectReleaseFunc(ctx, project, approvalID, comment)
}

func (c *Client) QueuePipeline(ctx context.Context, project string, definitionID int, branch string) (*ado.PipelineRun, error) {
	c.T.Helper()
	if c.QueuePipelineFunc == nil {
		c.T.Fatalf("guard.Client.QueuePipeline called with no QueuePipelineFunc set")
	}
	return c.QueuePipelineFunc(ctx, project, definitionID, branch)
}
