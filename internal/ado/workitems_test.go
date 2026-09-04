package ado

import (
	"context"
	"reflect"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

func TestUpdateWorkItemStateIssuesExactPatch(t *testing.T) {
	var got workitemtracking.UpdateWorkItemArgs
	client := &Client{
		workitemClient: &fakeWorkItemAPI{
			t: t,
			UpdateWorkItemFunc: func(_ context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
				got = args
				return &workitemtracking.WorkItem{}, nil
			},
		},
	}

	if err := client.UpdateWorkItemState(context.Background(), 42, "Active"); err != nil {
		t.Fatalf("UpdateWorkItemState() error = %v, want nil", err)
	}

	wantID := 42
	wantOp := webapi.OperationValues.Add
	wantPath := "/fields/System.State"
	want := workitemtracking.UpdateWorkItemArgs{
		Id: &wantID,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &wantOp,
				Path:  &wantPath,
				Value: "Active",
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateWorkItem args = %+v, want %+v", got, want)
	}
}
