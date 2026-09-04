package ado

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// TestGetWorkItemsSwallowsProjectFailure characterizes the current TRUST-01
// behavior. Phase 2 inverts this contract so a project query failure reaches the
// caller instead of being omitted from an otherwise successful aggregate read.
func TestGetWorkItemsSwallowsProjectFailure(t *testing.T) {
	const (
		failingProject   = "project-query-fails"
		survivingProject = "project-query-succeeds"
	)

	t.Run("query failure skips only the failed project", func(t *testing.T) {
		survivingID := 42
		queryRefs := []workitemtracking.WorkItemReference{{Id: &survivingID}}
		survivingFields := map[string]interface{}{
			"System.Title":        "surviving item",
			"System.WorkItemType": "Task",
			"System.State":        "Active",
		}
		api := &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				if args.Project == nil {
					t.Fatal("QueryByWiql project = nil")
				}
				if *args.Project == failingProject {
					return nil, errors.New("query failed")
				}
				if *args.Project != survivingProject {
					t.Fatalf("QueryByWiql project = %q, want %q or %q", *args.Project, failingProject, survivingProject)
				}
				return &workitemtracking.WorkItemQueryResult{WorkItems: &queryRefs}, nil
			},
			GetWorkItemsFunc: func(_ context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
				if args.Ids == nil || !reflect.DeepEqual(*args.Ids, []int{survivingID}) {
					t.Fatalf("GetWorkItems ids = %v, want [%d]", args.Ids, survivingID)
				}
				items := []workitemtracking.WorkItem{{Id: &survivingID, Fields: &survivingFields}}
				return &items, nil
			},
		}
		client := &Client{
			projects:       []string{failingProject, survivingProject},
			workitemClient: api,
		}

		items, err := client.GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		if items == nil || len(items) != 1 {
			t.Fatalf("GetWorkItems() returned %v, want one non-nil surviving item", items)
		}
		if items[0].ID != survivingID || items[0].Project != survivingProject {
			t.Errorf("GetWorkItems()[0] = %+v, want ID %d from %q", items[0], survivingID, survivingProject)
		}
	})

	t.Run("classification node failure is warned and discarded", func(t *testing.T) {
		queryIDs := map[string]int{
			failingProject:   101,
			survivingProject: 202,
		}
		api := &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, errors.New("classification node failed")
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				if args.Project == nil {
					t.Fatal("QueryByWiql project = nil")
				}
				id, ok := queryIDs[*args.Project]
				if !ok {
					t.Fatalf("QueryByWiql project = %q, want configured project", *args.Project)
				}
				refs := []workitemtracking.WorkItemReference{{Id: &id}}
				return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
			},
			GetWorkItemsFunc: func(_ context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
				if args.Ids == nil || len(*args.Ids) != 1 {
					t.Fatalf("GetWorkItems ids = %v, want one id", args.Ids)
				}
				id := (*args.Ids)[0]
				fields := map[string]interface{}{"System.Title": "item"}
				items := []workitemtracking.WorkItem{{Id: &id, Fields: &fields}}
				return &items, nil
			},
		}
		client := &Client{
			projects:       []string{failingProject, survivingProject},
			workitemClient: api,
		}

		items, err := client.GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		// getProjectWorkItems warns and discards classification-node failures,
		// so both projects continue through QueryByWiql and return an item.
		if len(items) != 2 {
			t.Fatalf("GetWorkItems() returned %d items, want 2 because both projects succeed", len(items))
		}
	})
}

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
