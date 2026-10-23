package ado

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

func TestAcceptanceCriteriaEmpty(t *testing.T) {
	for in, want := range map[string]bool{
		"":                         true,
		"<div><br></div>":          true,
		"&nbsp;":                   true,
		"<p>&nbsp; </p>\n\t":       true,
		"<ul><li></li></ul>":       true,
		"<div>Given a quote</div>": false,
		"x":                        false,
		"<p>&nbsp;Done when</p>":   false,
		"&lt;b&gt;":                false,
	} {
		if got := acceptanceCriteriaEmpty(in); got != want {
			t.Errorf("acceptanceCriteriaEmpty(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestPullRequestIDs(t *testing.T) {
	rel := func(r, u string) workitemtracking.WorkItemRelation {
		return workitemtracking.WorkItemRelation{Rel: strPtr(r), Url: strPtr(u)}
	}
	got := pullRequestIDs([]workitemtracking.WorkItemRelation{
		rel("ArtifactLink", "vstfs:///Git/PullRequestId/aaa%2Fbbb%2F4512"),
		rel("ArtifactLink", "vstfs:///Git/PullRequestId/aaa%2fbbb%2f77"),
		rel("ArtifactLink", "vstfs:///Git/Commit/aaa%2Fbbb%2Fdeadbeef"),
		rel("AttachedFile", "https://x/_apis/wit/attachments/1"),
		rel("System.LinkTypes.Hierarchy-Reverse", "https://x/_apis/wit/workItems/9"),
		rel("ArtifactLink", "vstfs:///Git/PullRequestId/aaa%2Fbbb%2F4512"),
	})
	if len(got) != 2 || got[0] != 4512 || got[1] != 77 {
		t.Errorf("ids = %v", got)
	}
}

// sprintFixture serves one team's sprint end to end. items maps id -> fields;
// drop lists ids the batch read silently omits (errorPolicy Omit).
type sprintFixture struct {
	items     map[int]map[string]interface{}
	relations map[int][]workitemtracking.WorkItemRelation
	drop      map[int]bool
	dropRel   map[int]bool
	queries   []string
	batches   []workitemtracking.GetWorkItemsArgs
}

func (fx *sprintFixture) client(t *testing.T, teamIDs ...int) *Client {
	sprintID := uuid.New()
	start := adoDate(2026, 10, 13)
	finish := adoDate(2026, 10, 26)
	leaf, prev, mid, root := 11351, 11350, 11329, 1
	return &Client{
		workClient: &fakeWorkAPI{t: t,
			GetBacklogConfigurationsFunc: func(context.Context, work.GetBacklogConfigurationsArgs) (*work.BacklogConfiguration, error) {
				cfg := backlogConfig(map[string]string{"Effort": "Microsoft.VSTS.Scheduling.StoryPoints", "Order": "Microsoft.VSTS.Common.StackRank"},
					[]string{"User Story"}, map[string]map[string]string{
						"User Story": {"Active": "InProgress", "ON Hold": "InProgress", "Closed": "Completed", "Removed": "Removed"},
						"Task":       {"Active": "InProgress", "Closed": "Completed"},
					})
				task := []workitemtracking.WorkItemTypeReference{{Name: strPtr("Task")}}
				cfg.TaskBacklog = &work.BacklogLevelConfiguration{WorkItemTypes: &task}
				return cfg, nil
			},
			GetTeamIterationsFunc: func(_ context.Context, args work.GetTeamIterationsArgs) (*[]work.TeamSettingsIteration, error) {
				current := work.TeamSettingsIteration{Id: &sprintID, Path: strPtr(`P\S2W\Sprint 218`),
					Attributes: &work.TeamIterationAttributes{StartDate: start, FinishDate: finish}}
				if args.Timeframe != nil {
					return &[]work.TeamSettingsIteration{current}, nil
				}
				// Listing every iteration: a closed one, a dateless one, the current one.
				past := work.TeamSettingsIteration{Id: &sprintID, Path: strPtr(`P\S2W\Sprint 217`),
					Attributes: &work.TeamIterationAttributes{StartDate: adoDate(2026, 9, 29), FinishDate: adoDate(2026, 10, 12)}}
				return &[]work.TeamSettingsIteration{past, {Path: strPtr(`P\S2W\Backlog`)}, current}, nil
			},
			GetTeamFieldValuesFunc: func(_ context.Context, args work.GetTeamFieldValuesArgs) (*work.TeamFieldValues, error) {
				return &work.TeamFieldValues{Field: &work.FieldReference{ReferenceName: strPtr("System.AreaPath")},
					Values: &[]work.TeamFieldValue{{Value: strPtr(`P\` + *args.Team), IncludeChildren: boolPtr(true)}}}, nil
			},
		},
		workitemClient: &fakeWorkItemAPI{t: t,
			GetClassificationNodeFunc: func(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return &workitemtracking.WorkItemClassificationNode{Id: &root, Path: strPtr(`\P\Iteration`),
					Children: &[]workitemtracking.WorkItemClassificationNode{{Id: &mid, Path: strPtr(`\P\Iteration\S2W`),
						Children: &[]workitemtracking.WorkItemClassificationNode{
							{Id: &prev, Path: strPtr(`\P\Iteration\S2W\Sprint 217`)},
							{Id: &leaf, Path: strPtr(`\P\Iteration\S2W\Sprint 218`)}}}}}, nil
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				fx.queries = append(fx.queries, *args.Wiql.Query)
				var refs []workitemtracking.WorkItemReference
				for _, id := range teamIDs {
					id := id
					refs = append(refs, workitemtracking.WorkItemReference{Id: &id})
				}
				return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
			},
			GetWorkItemsFunc: func(_ context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
				fx.batches = append(fx.batches, args)
				if args.Fields != nil && args.Expand != nil {
					t.Fatal("fields and $expand sent together (ADO returns 400)")
				}
				if args.ErrorPolicy == nil || *args.ErrorPolicy != workitemtracking.WorkItemErrorPolicyValues.Omit {
					t.Fatal("batch without errorPolicy Omit")
				}
				var out []workitemtracking.WorkItem
				for _, id := range *args.Ids {
					if fx.drop[id] || (args.Expand != nil && fx.dropRel[id]) {
						continue
					}
					id := id
					wi := workitemtracking.WorkItem{Id: &id}
					if args.Expand != nil {
						r := fx.relations[id]
						wi.Relations = &r
					} else {
						f := fx.items[id]
						wi.Fields = &f
					}
					out = append(out, wi)
				}
				return &out, nil
			},
		},
	}
}

func baseItems() map[int]map[string]interface{} {
	return map[int]map[string]interface{}{
		10: {"System.WorkItemType": "User Story", "System.Title": "S", "System.State": "ON Hold",
			"Microsoft.VSTS.Scheduling.StoryPoints": 2.5, "Microsoft.VSTS.Common.AcceptanceCriteria": "<div>&nbsp;</div>",
			"System.AssignedTo": map[string]interface{}{"displayName": "Ann"}},
		11: {"System.WorkItemType": "Task", "System.Title": "T", "System.State": "Closed", "System.Parent": 10.0,
			"Microsoft.VSTS.Scheduling.RemainingWork": 0.0},
		12: {"System.WorkItemType": "Task", "System.Title": "Orphan", "System.State": "Active", "System.Parent": 99.0},
		13: {"System.WorkItemType": "User Story", "System.Title": "Done", "System.State": "Closed",
			"Microsoft.VSTS.Common.AcceptanceCriteria": "Given"},
		14: {"System.WorkItemType": "User Story", "System.Title": "Gone", "System.State": "Removed"},
	}
}

func TestGetSprintItemsLive(t *testing.T) {
	fx := &sprintFixture{items: baseItems(), relations: map[int][]workitemtracking.WorkItemRelation{
		10: {{Rel: strPtr("ArtifactLink"), Url: strPtr("vstfs:///Git/PullRequestId/a%2Fb%2F4512")}},
	}}
	c := fx.client(t, 10, 11, 12, 13)
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	result, _ := c.GetSprintItems(context.Background(), scrumTeams[:1], SnapshotLive, now)
	if !result.Complete || len(result.Items) != 3 {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(fx.queries[0], "[System.IterationId] = 11351 AND ([System.AreaPath] UNDER 'P\\id-core')") || strings.Contains(fx.queries[0], "ASOF") {
		t.Errorf("query = %s", fx.queries[0])
	}
	s := result.Items[0]
	if s.ID != 10 || *s.Points != 2.5 || s.RemainingWork != nil || s.StateCategory != "InProgress" || s.Done ||
		s.AssignedTo != "Ann" || !s.AcceptanceCriteriaEmpty || len(s.PullRequestIDs) != 1 || s.PullRequestIDs[0] != 4512 {
		t.Errorf("story = %+v", s)
	}
	if len(s.Tasks) != 1 || s.Tasks[0].ID != 11 || !s.Tasks[0].Done || *s.Tasks[0].RemainingWork != 0 {
		t.Errorf("tasks = %+v", s.Tasks)
	}
	if result.Items[1].ID != 12 || *result.Items[1].ParentID != 99 {
		t.Errorf("orphan task must stay top level: %+v", result.Items[1])
	}
	if !result.Items[2].Done || result.Items[2].AcceptanceCriteriaEmpty {
		t.Errorf("closed story = %+v", result.Items[2])
	}
	raw, _ := json.Marshal(result.Items[1])
	if !strings.Contains(string(raw), `"points":null`) || !strings.Contains(string(raw), `"pull_request_ids":[]`) {
		t.Errorf("json = %s", raw)
	}
	sc := result.Scope
	if sc.AsOf != "live" || !sc.AreaPathsCurrent || len(sc.Teams) != 1 || sc.Teams[0].IterationID != 11351 ||
		sc.Teams[0].PointsField != "Microsoft.VSTS.Scheduling.StoryPoints" {
		t.Errorf("scope = %+v", sc)
	}
}

func TestGetSprintItemsHistorical(t *testing.T) {
	cases := []struct {
		name, snap, want, iterID string
		now                      time.Time
	}{
		{"baseline reads the current sprint start", SnapshotBaseline, "2026-10-13T00:00:00Z", "= 11351", time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)},
		{"close mid-sprint reads the last closed sprint", SnapshotClose, "2026-10-13T00:00:00Z", "= 11350", time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)},
		{"close after the current sprint ends reads it", SnapshotClose, "2026-10-27T00:00:00Z", "= 11351", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := &sprintFixture{items: baseItems()}
			c := fx.client(t, 10, 13)
			result, _ := c.GetSprintItems(context.Background(), scrumTeams[:1], tc.snap, tc.now)
			if len(fx.queries) != 1 || !strings.HasSuffix(fx.queries[0], "ASOF '"+tc.want+"'") || !strings.Contains(fx.queries[0], tc.iterID) {
				t.Fatalf("queries = %v failures %+v", fx.queries, result.Failures)
			}
			if len(fx.batches) != 1 || fx.batches[0].AsOf == nil || !fx.batches[0].AsOf.Time.Equal(mustTime(tc.want)) {
				t.Errorf("batches = %+v (no relations read historically)", fx.batches)
			}
			if result.Scope.AsOf != tc.snap || result.Scope.Teams[0].AsOf != tc.want || result.Items[0].PullRequestIDs != nil {
				t.Errorf("scope %+v pr %v", result.Scope, result.Items[0].PullRequestIDs)
			}
		})
	}
	t.Run("no closed sprint yet is a per-team failure", func(t *testing.T) {
		fx := &sprintFixture{items: baseItems()}
		c := fx.client(t, 10)
		result, _ := c.GetSprintItems(context.Background(), scrumTeams[:1], SnapshotClose, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
		if len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Err.Error(), "no closed sprint") || len(fx.queries) != 0 {
			t.Errorf("result = %+v queries %d", result, len(fx.queries))
		}
	})
	t.Run("historical query keeps the live team filter", func(t *testing.T) {
		fx := &sprintFixture{items: baseItems()}
		c := fx.client(t, 10)
		now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
		_, _ = c.GetSprintItems(context.Background(), scrumTeams[:1], SnapshotLive, now)
		_, _ = c.GetSprintItems(context.Background(), scrumTeams[:1], SnapshotBaseline, now)
		if !strings.HasPrefix(fx.queries[1], fx.queries[0]+" ASOF ") {
			t.Errorf("live %q\nbaseline %q", fx.queries[0], fx.queries[1])
		}
	})
}

func mustTime(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }

func TestGetSprintItemsDroppedIDs(t *testing.T) {
	fx := &sprintFixture{items: baseItems(), drop: map[int]bool{13: true}}
	c := fx.client(t, 10, 13)
	result, _ := c.GetSprintItems(context.Background(), scrumTeams[:1], SnapshotLive, time.Now())
	if result.Complete || len(result.Items) != 1 || len(result.Failures) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if f := result.Failures[0]; f.Team != "Core" || !strings.Contains(f.Err.Error(), "1 requested ids not returned: [13]") {
		t.Errorf("failure = %+v %v", f, f.Err)
	}
}

func TestGetSprintItemsOverlap(t *testing.T) {
	fx := &sprintFixture{items: baseItems()}
	c := fx.client(t, 10, 13)
	result, _ := c.GetSprintItems(context.Background(), scrumTeams, SnapshotLive, time.Now())
	if len(result.Items) != 4 || len(result.Findings) != 2 || result.Findings[0].Kind != FindingTeamOverlap {
		t.Errorf("items %d findings %+v", len(result.Items), result.Findings)
	}
	if len(fx.queries) != 2 {
		t.Errorf("queries = %d, want one per team", len(fx.queries))
	}
}

// TestGetSprintItemsRemovedAndUnlinked: a Removed item is still returned (no
// state filter, DATA-01), and an item missing from the relations read has
// unknown links (null) and is reported, not shown as having no PRs.
func TestGetSprintItemsRemovedAndUnlinked(t *testing.T) {
	fx := &sprintFixture{items: baseItems(), dropRel: map[int]bool{13: true}}
	c := fx.client(t, 13, 14)
	result, _ := c.GetSprintItems(context.Background(), scrumTeams[:1], SnapshotLive, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC))
	if len(result.Items) != 2 || result.Items[1].StateCategory != "Removed" || result.Items[1].Done {
		t.Fatalf("items = %+v", result.Items)
	}
	if result.Items[0].PullRequestIDs != nil || result.Items[1].PullRequestIDs == nil {
		t.Errorf("pr ids = %v / %v", result.Items[0].PullRequestIDs, result.Items[1].PullRequestIDs)
	}
	if result.Complete || len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Err.Error(), "links not returned for 1 ids: [13]") {
		t.Errorf("failures = %+v", result.Failures)
	}
}
