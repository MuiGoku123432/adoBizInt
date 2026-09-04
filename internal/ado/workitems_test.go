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

func TestUpdateWorkItemStateWithEffortIssuesExactPatch(t *testing.T) {
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

	if err := client.UpdateWorkItemStateWithEffort(context.Background(), 84, "Closed", 8.0, 3.5, 4.5); err != nil {
		t.Fatalf("UpdateWorkItemStateWithEffort() error = %v, want nil", err)
	}

	wantID := 84
	wantOp := webapi.OperationValues.Add
	wantOriginalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
	wantRemainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	wantCompletedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"
	wantStatePath := "/fields/System.State"
	want := workitemtracking.UpdateWorkItemArgs{
		Id: &wantID,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &wantOp,
				Path:  &wantOriginalEstimatePath,
				Value: 8.0,
			},
			{
				Op:    &wantOp,
				Path:  &wantRemainingPath,
				Value: 3.5,
			},
			{
				Op:    &wantOp,
				Path:  &wantCompletedPath,
				Value: 4.5,
			},
			{
				Op:    &wantOp,
				Path:  &wantStatePath,
				Value: "Closed",
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateWorkItem args = %+v, want %+v", got, want)
	}
}

func TestGetNextState(t *testing.T) {
	transitions := map[string]map[string]string{
		"Task": {
			"Active": "Closed",
		},
	}

	tests := []struct {
		name         string
		workItemType string
		currentState string
		wantState    string
		wantOK       bool
	}{
		{
			name:         "configured transition",
			workItemType: "Task",
			currentState: "Active",
			wantState:    "Closed",
			wantOK:       true,
		},
		{
			name:         "unknown work item type",
			workItemType: "Bug",
			currentState: "Active",
			wantState:    "",
			wantOK:       false,
		},
		{
			name:         "unmapped current state",
			workItemType: "Task",
			currentState: "New",
			wantState:    "",
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotOK := GetNextState(transitions, tt.workItemType, tt.currentState)
			if gotState != tt.wantState || gotOK != tt.wantOK {
				t.Errorf("GetNextState() = (%q, %t), want (%q, %t)", gotState, gotOK, tt.wantState, tt.wantOK)
			}
		})
	}
}
