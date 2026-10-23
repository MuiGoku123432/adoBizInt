package ado

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"

	"sentinovo.ai/bizInt/internal/logging"
)

// SprintItem is one work item in a team's sprint, as Azure DevOps holds it at
// the requested instant (DATA-05). Numeric fields are nil when the item does
// not hold them, never 0 (DATA-02).
type SprintItem struct {
	Project                 string       `json:"project"`
	Team                    string       `json:"team"`
	ID                      int          `json:"id"`
	Type                    string       `json:"type"`
	Title                   string       `json:"title"`
	State                   string       `json:"state"`
	StateCategory           string       `json:"state_category"`
	Done                    bool         `json:"done"`
	AssignedTo              string       `json:"assigned_to"`
	ParentID                *int         `json:"parent_id"`
	Points                  *float64     `json:"points"`
	RemainingWork           *float64     `json:"remaining_work"`
	CompletedWork           *float64     `json:"completed_work"`
	OriginalEstimate        *float64     `json:"original_estimate"`
	AreaPath                string       `json:"area_path"`
	IterationPath           string       `json:"iteration_path"`
	AcceptanceCriteriaEmpty bool         `json:"acceptance_criteria_empty"`
	PullRequestIDs          []int        `json:"pull_request_ids"`
	Tasks                   []SprintItem `json:"tasks"`
}

// SprintTeamScope discloses, per team, exactly what selected its items.
type SprintTeamScope struct {
	Project       string     `json:"project"`
	Team          string     `json:"team"`
	AsOf          string     `json:"as_of"`
	IterationPath string     `json:"iteration_path"`
	IterationID   int        `json:"iteration_id"`
	AreaPaths     []AreaPath `json:"area_paths"`
	PointsField   string     `json:"points_field"`
}

// Snapshot instants (DATA-04). Live reads carry no instant.
const (
	SnapshotLive     = "live"
	SnapshotBaseline = "baseline"
	SnapshotClose    = "close"
)

const opGetSprintItems = "get sprint items"

// sprintBatchSize is the workitemsbatch id limit.
const sprintBatchSize = 200

const (
	fieldRemaining = "Microsoft.VSTS.Scheduling.RemainingWork"
	fieldCompleted = "Microsoft.VSTS.Scheduling.CompletedWork"
	fieldOriginal  = "Microsoft.VSTS.Scheduling.OriginalEstimate"
	fieldAC        = "Microsoft.VSTS.Common.AcceptanceCriteria"
)

// GetSprintItems reads each team's sprint at the snapshot: "live" and
// "baseline" (sprint start 00:00 UTC) read the current sprint; "close" reads
// the most recently closed sprint at the day after its inclusive finish,
// 00:00 UTC. now decides which sprints have closed. Every failure
// is per team; other teams still return. Items claimed by two teams produce a
// team_overlap finding (TEAM-06).
func (c *Client) GetSprintItems(ctx context.Context, teams []Team, snapshot string, now time.Time) (Result[SprintItem], error) {
	var items []SprintItem
	var failures []Failure
	var sets []TeamItems
	scope := &Scope{AsOf: snapshot, AreaPathsCurrent: true, Teams: []SprintTeamScope{}}

	process := map[string]ProcessFields{}
	processErr := map[string]error{}
	iterIDs := map[string]map[string]int{}
	iterErr := map[string]error{}

	for _, team := range teams {
		fail := func(op string, err error) {
			logging.Logger().Error("Sprint items read failed", "project", team.Project, "team", team.Team, "operation", op, "error", err)
			failures = appendFailure(failures, team.Project, team.Team, op, err)
		}
		if err := ctx.Err(); err != nil {
			fail(opGetSprintItems, err)
			continue
		}

		if _, done := process[team.Project]; !done && processErr[team.Project] == nil {
			pf, err := c.processFields(ctx, team)
			if err != nil {
				processErr[team.Project] = err
			} else {
				process[team.Project] = pf
			}
		}
		if err := processErr[team.Project]; err != nil {
			fail(opResolveProcessFields, err)
			continue
		}
		pf := process[team.Project]

		var sprint Sprint
		var err error
		if snapshot == SnapshotClose {
			sprint, err = c.lastClosedSprint(ctx, team, now)
		} else {
			sprint, err = c.currentSprint(ctx, team)
		}
		if err != nil {
			fail(opGetCurrentSprint, err)
			continue
		}
		if _, done := iterIDs[team.Project]; !done && iterErr[team.Project] == nil {
			ids, err := c.iterationIDs(ctx, team.Project)
			if err != nil {
				iterErr[team.Project] = err
			} else {
				iterIDs[team.Project] = ids
			}
		}
		if err := iterErr[team.Project]; err != nil {
			fail(opResolveSprintIteration, err)
			continue
		}
		iterationID, ok := iterIDs[team.Project][strings.ToLower(sprint.Path)]
		if !ok {
			fail(opResolveSprintIteration, fmt.Errorf("iteration %q not found in the project's iteration tree", sprint.Path))
			continue
		}

		asOf, err := snapshotInstant(snapshot, sprint)
		if err != nil {
			fail(opGetSprintItems, err)
			continue
		}
		areas, err := c.teamAreaPaths(ctx, team)
		if err != nil {
			fail(opGetTeamAreaPaths, err)
			continue
		}
		ids, err := c.querySprintIDs(ctx, team, iterationID, areas, asOf)
		if err != nil {
			fail(opQuerySprintItems, err)
			continue
		}
		teamAsOf := SnapshotLive
		if asOf != nil {
			teamAsOf = asOf.Format(time.RFC3339)
		}
		scope.Teams = append(scope.Teams, SprintTeamScope{
			Project: team.Project, Team: team.Team, AsOf: teamAsOf, IterationPath: sprint.Path,
			IterationID: iterationID, AreaPaths: areas, PointsField: pf.PointsField,
		})
		sets = append(sets, TeamItems{Project: team.Project, Team: team.Team, IDs: ids})

		teamItems, err := c.readSprintItems(ctx, team, pf, ids, asOf)
		if err != nil {
			// A dropped-id report still returns what was read.
			fail(opGetSprintItems, err)
		}
		items = append(items, teamItems...)
	}

	result := newResult(items, failures)
	result.Findings = append(result.Findings, teamOverlap(sets)...)
	result.Scope = scope
	return result, nil
}

// snapshotInstant turns a snapshot name into the UTC instant to read at.
func snapshotInstant(snapshot string, sprint Sprint) (*time.Time, error) {
	switch snapshot {
	case SnapshotLive:
		return nil, nil
	case SnapshotBaseline:
		t, err := time.Parse("2006-01-02", sprint.StartDate)
		return &t, err
	case SnapshotClose:
		t, err := time.Parse("2006-01-02", sprint.FinishDate)
		if err != nil {
			return nil, err
		}
		t = t.AddDate(0, 0, 1)
		return &t, nil
	}
	return nil, fmt.Errorf("unknown snapshot %q", snapshot)
}

// readSprintItems reads the ids in two batch calls per 200: fields (as of the
// instant when historical) and, live only, relations. $expand and fields are
// mutually exclusive (10-RESEARCH Q2). Under errorPolicy Omit ADO drops a
// missing or unreadable id silently, so requested and returned ids are
// compared and any gap is returned as an error alongside the items read.
func (c *Client) readSprintItems(ctx context.Context, team Team, pf ProcessFields, ids []int, asOf *time.Time) ([]SprintItem, error) {
	fields := []string{"System.Id", "System.WorkItemType", "System.Title", "System.State", "System.AssignedTo",
		"System.Parent", "System.AreaPath", "System.IterationPath", fieldAC, fieldRemaining, fieldCompleted, fieldOriginal}
	if !slices.Contains(fields, pf.PointsField) {
		fields = append(fields, pf.PointsField)
	}
	omit := workitemtracking.WorkItemErrorPolicyValues.Omit
	var at *azuredevops.Time
	if asOf != nil {
		at = &azuredevops.Time{Time: *asOf}
	}

	byID := map[int]*SprintItem{}
	var order []int
	var unlinked []int
	for i := 0; i < len(ids); i += sprintBatchSize {
		batch := ids[i:min(i+sprintBatchSize, len(ids))]
		got, err := c.workitemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
			Ids: &batch, Project: &team.Project, Fields: &fields, AsOf: at, ErrorPolicy: &omit,
		})
		if err != nil {
			return nestTasks(order, byID), err
		}
		if got != nil {
			for _, wi := range *got {
				if wi.Id == nil || wi.Fields == nil {
					continue
				}
				it := sprintItemFromFields(team, pf, *wi.Fields)
				it.ID = *wi.Id
				if asOf == nil {
					it.PullRequestIDs = []int{}
				}
				byID[it.ID] = &it
				order = append(order, it.ID)
			}
		}

		if asOf != nil {
			continue
		}
		expand := workitemtracking.WorkItemExpandValues.Relations
		rel, err := c.workitemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
			Ids: &batch, Project: &team.Project, Expand: &expand, ErrorPolicy: &omit,
		})
		if err != nil {
			return nestTasks(order, byID), err
		}
		linked := map[int]bool{}
		if rel != nil {
			for _, wi := range *rel {
				if wi.Id == nil || byID[*wi.Id] == nil {
					continue
				}
				linked[*wi.Id] = true
				if wi.Relations != nil {
					byID[*wi.Id].PullRequestIDs = pullRequestIDs(*wi.Relations)
				}
			}
		}
		// An item missing from the relations read has unknown links, not
		// none: null, and reported with the dropped ids below.
		for _, id := range batch {
			if it := byID[id]; it != nil && !linked[id] {
				it.PullRequestIDs = nil
				unlinked = append(unlinked, id)
			}
		}
	}

	out := nestTasks(order, byID)
	var dropped []int
	for _, requested := range ids {
		if byID[requested] == nil {
			dropped = append(dropped, requested)
		}
	}
	switch {
	case len(dropped) > 0 && len(unlinked) > 0:
		return out, fmt.Errorf("%d requested ids not returned: %v; links not returned for %v", len(dropped), dropped, unlinked)
	case len(dropped) > 0:
		return out, fmt.Errorf("%d requested ids not returned: %v", len(dropped), dropped)
	case len(unlinked) > 0:
		return out, fmt.Errorf("links not returned for %d ids: %v", len(unlinked), unlinked)
	}
	return out, nil
}
