package ado

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// TestGetWorkItemsSwallowsProjectFailure now pins the inverted TRUST-01
// behavior: the project query failure reaches the caller instead of being
// swallowed. The name is retained until the phase-wide rename sweep.
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

		result, err := client.GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		if len(result.Items) != 1 {
			t.Fatalf("GetWorkItems() returned %v, want one surviving item", result.Items)
		}
		if result.Items[0].ID != survivingID || result.Items[0].Project != survivingProject {
			t.Errorf("GetWorkItems()[0] = %+v, want ID %d from %q", result.Items[0], survivingID, survivingProject)
		}
		if len(result.Failures) != 1 {
			t.Fatalf("GetWorkItems() Failures = %+v, want exactly one entry for the failing project", result.Failures)
		}
		if result.Failures[0].Project != failingProject {
			t.Errorf("GetWorkItems() Failures[0].Project = %v, want %v", result.Failures[0].Project, failingProject)
		}
		if result.Failures[0].Operation == "" {
			t.Errorf("GetWorkItems() Failures[0].Operation = %q, want a named operation", result.Failures[0].Operation)
		}
		if result.Complete {
			t.Errorf("GetWorkItems() Complete = %v, want false when a project failed", result.Complete)
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

		result, err := client.GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		// getProjectWorkItems warns and discards classification-node failures,
		// so both projects continue through QueryByWiql and return an item.
		if len(result.Items) != 2 {
			t.Fatalf("GetWorkItems() returned %d items, want 2 because both projects succeed", len(result.Items))
		}
		if !result.Complete || len(result.Failures) != 0 {
			t.Errorf("GetWorkItems() Complete/Failures = %v/%+v, want true/none — a discarded iteration warning is not a failure", result.Complete, result.Failures)
		}
	})
}

// TestGetWorkItemsReportsBatchFailure covers workitems.go's batch swallow: a
// project whose WIQL query succeeded but whose detail batch failed must not be
// presented as complete just because the query answered.
func TestGetWorkItemsReportsBatchFailure(t *testing.T) {
	const project = "project-batch-fails"

	queryID := 7
	queryRefs := []workitemtracking.WorkItemReference{{Id: &queryID}}
	client := &Client{
		projects: []string{project},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				return &workitemtracking.WorkItemQueryResult{WorkItems: &queryRefs}, nil
			},
			GetWorkItemsFunc: func(_ context.Context, _ workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
				return nil, errors.New("batch fetch failed")
			},
		},
	}

	result, err := client.GetWorkItems(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetWorkItems() error = %v, want nil", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("GetWorkItems() Failures = %+v, want exactly one batch failure", result.Failures)
	}
	if result.Failures[0].Project != project {
		t.Errorf("GetWorkItems() Failures[0].Project = %q, want %q", result.Failures[0].Project, project)
	}
	if result.Failures[0].Operation == "" {
		t.Errorf("GetWorkItems() Failures[0].Operation = %q, want a named batch operation", result.Failures[0].Operation)
	}
	if result.Complete {
		t.Errorf("GetWorkItems() Complete = %v, want false when a batch failed", result.Complete)
	}
}

// TestGetWorkItemsReportsBatchFailureKeepsRetrievedItems asserts the items that
// were retrieved before the failing batch are still returned.
func TestGetWorkItemsReportsBatchFailureKeepsRetrievedItems(t *testing.T) {
	const project = "project-second-batch-fails"

	// Two batches: the API limit is 200 IDs per request, so 201 IDs split into
	// a first batch that answers and a second that fails.
	ids := make([]int, 201)
	refs := make([]workitemtracking.WorkItemReference, len(ids))
	for i := range ids {
		ids[i] = i + 1
		refs[i] = workitemtracking.WorkItemReference{Id: &ids[i]}
	}

	calls := 0
	client := &Client{
		projects: []string{project},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
			},
			GetWorkItemsFunc: func(_ context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
				calls++
				if calls == 1 {
					batch := make([]workitemtracking.WorkItem, 0, len(*args.Ids))
					for i := range *args.Ids {
						id := (*args.Ids)[i]
						fields := map[string]interface{}{"System.Title": "retrieved"}
						batch = append(batch, workitemtracking.WorkItem{Id: &id, Fields: &fields})
					}
					return &batch, nil
				}
				return nil, errors.New("second batch failed")
			},
		},
	}

	result, err := client.GetWorkItems(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetWorkItems() error = %v, want nil", err)
	}
	if len(result.Items) != 200 {
		t.Fatalf("GetWorkItems() returned %d items, want the 200 retrieved before the failing batch", len(result.Items))
	}
	if len(result.Failures) != 1 {
		t.Fatalf("GetWorkItems() Failures = %+v, want exactly one batch failure", result.Failures)
	}
	if result.Complete {
		t.Errorf("GetWorkItems() Complete = %v, want false", result.Complete)
	}
}

func TestGetWorkItemsClassifiesRejectedToken(t *testing.T) {
	const project = "project-work-items-unauthorized"

	client := &Client{
		projects: []string{project},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				return nil, wrappedErrorValue(401, "TF400813: The user is not authorized")
			},
		},
	}

	result, err := client.GetWorkItems(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetWorkItems() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetWorkItems() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("GetWorkItems() Failures = %+v, want one entry — empty must be distinguishable from failed", result.Failures)
	}
	if result.Failures[0].Kind != FailureAuth {
		t.Errorf("GetWorkItems() Failures[0].Kind = %v, want %v", result.Failures[0].Kind, FailureAuth)
	}
	if result.Complete {
		t.Errorf("GetWorkItems() Complete = %v, want false", result.Complete)
	}
}

// TestFetchTimeoutMarksUnreachedWorkItems is D-12's gate for GetWorkItems.
func TestFetchTimeoutMarksUnreachedWorkItems(t *testing.T) {
	projects := []string{"project-one", "project-two", "project-three"}

	client := &Client{
		projects: projects,
		workitemClient: &fakeWorkItemAPI{
			t: t,
			QueryByWiqlFunc: func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				t.Errorf("QueryByWiql called after the deadline elapsed; want no doomed calls issued")
				return nil, nil
			},
		},
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result, err := client.GetWorkItems(ctx, nil)
	if err != nil {
		t.Fatalf("GetWorkItems() error = %v, want nil", err)
	}
	if len(result.Failures) != len(projects) {
		t.Fatalf("GetWorkItems() Failures length = %d, want %d — one per unreached project", len(result.Failures), len(projects))
	}
	for i, failure := range result.Failures {
		if failure.Kind != FailureTimeout {
			t.Errorf("GetWorkItems() Failures[%d].Kind = %v, want %v", i, failure.Kind, FailureTimeout)
		}
		if failure.Project != projects[i] {
			t.Errorf("GetWorkItems() Failures[%d].Project = %v, want %v", i, failure.Project, projects[i])
		}
	}
	if result.Complete {
		t.Errorf("GetWorkItems() Complete = %v, want false", result.Complete)
	}
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
