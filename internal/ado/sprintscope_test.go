package ado

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// TestQuerySprintIDs is AC1/AC3: exact text sent in the team's context, UNDER
// vs = per includeChildren, OR'ed and escaped, no state clause, optional ASOF
// in RFC 3339 UTC.
func TestQuerySprintIDs(t *testing.T) {
	areas := []AreaPath{{Path: `Kinetic\Pricing`, IncludeChildren: true}, {Path: `Kinetic\O'Brien`, IncludeChildren: false}}
	want := `SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project AND [System.IterationId] = 11351 AND ([System.AreaPath] UNDER 'Kinetic\Pricing' OR [System.AreaPath] = 'Kinetic\O''Brien')`
	var sent []string
	c := &Client{workitemClient: &fakeWorkItemAPI{t: t, QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
		if *args.Project != "P" || *args.Team != "id-core" {
			t.Errorf("context = %s/%s", *args.Project, *args.Team)
		}
		sent = append(sent, *args.Wiql.Query)
		id1, id2 := 7, 3
		return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &id1}, {Id: &id2}}}, nil
	}}}
	ids, err := c.querySprintIDs(context.Background(), scrumTeams[0], 11351, areas, nil)
	if err != nil || len(ids) != 2 || ids[0] != 7 {
		t.Fatalf("ids = %v err %v", ids, err)
	}
	at := time.Date(2026, 10, 13, 0, 0, 0, 0, time.FixedZone("X", -5*3600))
	_, _ = c.querySprintIDs(context.Background(), scrumTeams[0], 11351, areas, &at)
	if sent[0] != want {
		t.Errorf("live:\n got %s\nwant %s", sent[0], want)
	}
	if sent[1] != want+" ASOF '2026-10-13T05:00:00Z'" {
		t.Errorf("asof: %s", sent[1])
	}
	if strings.Contains(sent[0], "State") {
		t.Error("sprint WIQL must not filter on state (DATA-01)")
	}
}

func TestTeamAreaPaths(t *testing.T) {
	field := func(ref string, vals ...work.TeamFieldValue) *fakeWorkAPI {
		return &fakeWorkAPI{t: t, GetTeamFieldValuesFunc: func(_ context.Context, args work.GetTeamFieldValuesArgs) (*work.TeamFieldValues, error) {
			if *args.Team != "id-core" {
				t.Fatalf("team = %s", *args.Team)
			}
			return &work.TeamFieldValues{Field: &work.FieldReference{ReferenceName: strPtr(ref)}, Values: &vals}, nil
		}}
	}
	c := &Client{workClient: field("System.AreaPath",
		work.TeamFieldValue{Value: strPtr(`P\A`), IncludeChildren: boolPtr(true)},
		work.TeamFieldValue{Value: strPtr(`P\B`)})}
	got, err := c.teamAreaPaths(context.Background(), scrumTeams[0])
	if err != nil || len(got) != 2 || !got[0].IncludeChildren || got[1].IncludeChildren || got[1].Path != `P\B` {
		t.Errorf("areas = %+v err %v", got, err)
	}
	c = &Client{workClient: field("Custom.Team", work.TeamFieldValue{Value: strPtr("x")})}
	if _, err := c.teamAreaPaths(context.Background(), scrumTeams[0]); err == nil || !strings.Contains(err.Error(), `"Custom.Team"`) {
		t.Errorf("non-area team field err = %v", err)
	}
	c = &Client{workClient: field("System.AreaPath")}
	if _, err := c.teamAreaPaths(context.Background(), scrumTeams[0]); err == nil {
		t.Error("no area paths must be an error")
	}
}

// TestIterationIDs is A2: the tree walk maps team-style paths, leaves
// included, to node ids, so the last segment can never be dropped.
func TestIterationIDs(t *testing.T) {
	node := func(id int, path string, kids ...workitemtracking.WorkItemClassificationNode) workitemtracking.WorkItemClassificationNode {
		n := workitemtracking.WorkItemClassificationNode{Id: &id, Path: strPtr(path)}
		if len(kids) > 0 {
			n.Children = &kids
		}
		return n
	}
	root := node(1, `\Kinetic\Iteration`,
		node(11329, `\Kinetic\Iteration\S2W 2026`,
			node(11351, `\Kinetic\Iteration\S2W 2026\Sprint 218`)))
	c := &Client{workitemClient: &fakeWorkItemAPI{t: t, GetClassificationNodeFunc: func(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
		return &root, nil
	}}}
	ids, err := c.iterationIDs(context.Background(), "Kinetic")
	if err != nil {
		t.Fatal(err)
	}
	if ids[`Kinetic\S2W 2026\Sprint 218`] != 11351 || ids[`Kinetic\S2W 2026`] != 11329 {
		t.Errorf("ids = %v", ids)
	}
}

func TestTeamOverlap(t *testing.T) {
	got := teamOverlap([]TeamItems{
		{Project: "P", Team: "A", IDs: []int{5, 3, 1}},
		{Project: "P", Team: "B", IDs: []int{3, 9, 5}},
		{Project: "P", Team: "C", IDs: []int{7}},
	})
	if len(got) != 2 || !strings.Contains(got[0].Detail, "work item 3 ") || !strings.Contains(got[1].Detail, "work item 5 ") {
		t.Fatalf("findings = %+v", got)
	}
	if got[0].Kind != FindingTeamOverlap || got[0].Project != "P" || len(got[0].Teams) != 2 {
		t.Errorf("finding = %+v", got[0])
	}
	if f := teamOverlap(nil); f == nil || len(f) != 0 {
		t.Errorf("no overlap = %#v, want empty non-nil", f)
	}
}
