package ado

import (
	"context"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

type fakeWorkItemAPI struct {
	t *testing.T

	UpdateWorkItemFunc        func(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiqlFunc           func(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetWorkItemsFunc          func(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
	GetWorkItemFunc           func(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error)
	GetClassificationNodeFunc func(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}

func (f *fakeWorkItemAPI) QueryByWiql(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
	f.t.Helper()
	if f.QueryByWiqlFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.QueryByWiql")
	}
	return f.QueryByWiqlFunc(ctx, args)
}

func (f *fakeWorkItemAPI) GetWorkItems(ctx context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.GetWorkItemsFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetWorkItems")
	}
	return f.GetWorkItemsFunc(ctx, args)
}

func (f *fakeWorkItemAPI) GetWorkItem(ctx context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.GetWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetWorkItem")
	}
	return f.GetWorkItemFunc(ctx, args)
}

func (f *fakeWorkItemAPI) GetClassificationNode(ctx context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
	f.t.Helper()
	if f.GetClassificationNodeFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetClassificationNode")
	}
	return f.GetClassificationNodeFunc(ctx, args)
}
