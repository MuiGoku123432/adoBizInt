package ado

import (
	"context"
	"errors"
	"slices"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"

	"sentinovo.ai/bizInt/internal/logging"
)

// ProcessFields is a project's resolved process template (TEAM-07): the
// points field, the backlog rank field, and the done-category states of each
// requirement-level work item type.
type ProcessFields struct {
	Project     string              `json:"project"`
	PointsField string              `json:"points_field"`
	RankField   string              `json:"rank_field"`
	DoneStates  map[string][]string `json:"done_states"`
}

const opResolveProcessFields = "resolve process fields"

// stateCategoryCompleted is the state category Azure DevOps treats as done.
const stateCategoryCompleted = "Completed"

// GetProcessFields resolves each project's process fields from the backlog
// configuration of its first selected team (D-6). A project whose points
// field, rank field or done states cannot be resolved is a named per-project
// failure, recorded against the team queried (D-7).
func (c *Client) GetProcessFields(ctx context.Context, teams []Team) (Result[ProcessFields], error) {
	var all []ProcessFields
	var failures []Failure
	var done []string
	for _, team := range teams {
		if slices.Contains(done, team.Project) {
			continue
		}
		done = append(done, team.Project)
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, team.Project, team.Team, opResolveProcessFields, err)
			continue
		}
		pf, err := c.processFields(ctx, team)
		if err != nil {
			logging.Logger().Error("Failed to resolve process fields", "project", team.Project, "team", team.Team, "error", err)
			failures = appendFailure(failures, team.Project, team.Team, opResolveProcessFields, err)
			continue
		}
		all = append(all, pf)
	}
	return newResult(all, failures), nil
}

func (c *Client) processFields(ctx context.Context, team Team) (ProcessFields, error) {
	cfg, err := c.workClient.GetBacklogConfigurations(ctx, work.GetBacklogConfigurationsArgs{Project: &team.Project, Team: &team.TeamID})
	if err != nil {
		return ProcessFields{}, err
	}
	if cfg == nil {
		return ProcessFields{}, errors.New("no backlog configuration returned")
	}
	pf := ProcessFields{Project: team.Project, DoneStates: map[string][]string{}}
	if cfg.BacklogFields != nil && cfg.BacklogFields.TypeFields != nil {
		pf.PointsField = (*cfg.BacklogFields.TypeFields)["Effort"]
		pf.RankField = (*cfg.BacklogFields.TypeFields)["Order"]
	}
	if pf.PointsField == "" {
		return ProcessFields{}, errors.New("points field (Effort) is not mapped in the backlog configuration")
	}
	if pf.RankField == "" {
		return ProcessFields{}, errors.New("backlog rank field (Order) is not mapped in the backlog configuration")
	}

	var reqTypes []string
	if rb := cfg.RequirementBacklog; rb != nil && rb.WorkItemTypes != nil {
		for _, t := range *rb.WorkItemTypes {
			if t.Name != nil {
				reqTypes = append(reqTypes, *t.Name)
			}
		}
	}
	if len(reqTypes) == 0 {
		return ProcessFields{}, errors.New("requirement backlog has no work item types")
	}
	if cfg.WorkItemTypeMappedStates != nil {
		for _, ts := range *cfg.WorkItemTypeMappedStates {
			name := deref(ts.WorkItemTypeName)
			if !slices.Contains(reqTypes, name) || ts.States == nil {
				continue
			}
			for state, category := range *ts.States {
				if category == stateCategoryCompleted {
					pf.DoneStates[name] = append(pf.DoneStates[name], state)
				}
			}
			slices.Sort(pf.DoneStates[name])
		}
	}
	for _, t := range reqTypes {
		if len(pf.DoneStates[t]) == 0 {
			return ProcessFields{}, errors.New("no done-category state for requirement type " + t)
		}
	}
	return pf, nil
}
