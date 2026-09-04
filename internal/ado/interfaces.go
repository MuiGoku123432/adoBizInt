package ado

import (
	"context"

	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// These package-local interfaces make destructive write methods testable without
// crossing the Azure DevOps REST boundary (SEAM-01). Each interface declares only
// methods this package calls; adding an uncalled method violates ROADMAP SC4.
// The SDK constructors return interfaces, so their assignments in NewClient are
// already compile-time drift checks. Do not add concrete ClientImpl assertions.
type workItemAPI interface {
	UpdateWorkItem(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	GetWorkItems(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
	GetWorkItem(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiql(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetClassificationNode(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
}
