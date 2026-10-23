package ado

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"

	"sentinovo.ai/bizInt/internal/logging"
)

// Sprint is a team's current iteration (TEAM-03). Dates are civil dates,
// YYYY-MM-DD; FinishDate is inclusive, the sprint's last day.
type Sprint struct {
	Project     string `json:"project"`
	Team        string `json:"team"`
	IterationID string `json:"iteration_id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	StartDate   string `json:"start_date"`
	FinishDate  string `json:"finish_date"`
}

const opGetCurrentSprint = "get current sprint"

// timeframeCurrent is the only $timeframe value the API supports.
const timeframeCurrent = "current"

// GetCurrentSprints resolves each team's current sprint. A team with no
// current iteration, more than one, or missing dates is a per-team failure,
// never an empty sprint (TEAM-03). When the teams' sprints differ, one
// cadence_mismatch finding names every team and its dates (TEAM-05).
func (c *Client) GetCurrentSprints(ctx context.Context, teams []Team) (Result[Sprint], error) {
	var sprints []Sprint
	var failures []Failure
	for _, team := range teams {
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, team.Project, team.Team, opGetCurrentSprint, err)
			continue
		}
		sprint, err := c.currentSprint(ctx, team)
		if err != nil {
			logging.Logger().Error("Failed to get current sprint", "project", team.Project, "team", team.Team, "error", err)
			failures = appendFailure(failures, team.Project, team.Team, opGetCurrentSprint, err)
			continue
		}
		sprints = append(sprints, sprint)
	}
	result := newResult(sprints, failures)
	if f, ok := cadenceMismatch(sprints); ok {
		result.Findings = append(result.Findings, f)
	}
	return result, nil
}

func (c *Client) currentSprint(ctx context.Context, team Team) (Sprint, error) {
	tf := timeframeCurrent
	its, err := c.workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{Project: &team.Project, Team: &team.TeamID, Timeframe: &tf})
	if err != nil {
		return Sprint{}, err
	}
	if its == nil || len(*its) == 0 {
		return Sprint{}, errors.New("team has no current iteration")
	}
	if len(*its) > 1 {
		return Sprint{}, fmt.Errorf("team has %d current iterations, want 1", len(*its))
	}
	return sprintFromIteration(team, (*its)[0])
}

// lastClosedSprint is the team's most recent sprint whose close instant (the
// day after its inclusive finish, 00:00 UTC) is at or before now. The current
// sprint has by definition not closed, so --at close reads this one.
// $timeframe supports only "current", so every iteration is listed; ones with
// missing dates are skipped.
func (c *Client) lastClosedSprint(ctx context.Context, team Team, now time.Time) (Sprint, error) {
	its, err := c.workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{Project: &team.Project, Team: &team.TeamID})
	if err != nil {
		return Sprint{}, err
	}
	var best Sprint
	found := false
	if its != nil {
		for _, it := range *its {
			s, err := sprintFromIteration(team, it)
			if err != nil {
				continue
			}
			finish, _ := time.Parse("2006-01-02", s.FinishDate)
			if finish.AddDate(0, 0, 1).After(now) {
				continue
			}
			if !found || s.FinishDate > best.FinishDate {
				best, found = s, true
			}
		}
	}
	if !found {
		return Sprint{}, errors.New("team has no closed sprint")
	}
	return best, nil
}

// sprintFromIteration converts a team iteration to civil dates, refusing one
// with missing or inverted dates (TEAM-03).
func sprintFromIteration(team Team, it work.TeamSettingsIteration) (Sprint, error) {
	a := it.Attributes
	if a == nil || a.StartDate == nil || a.FinishDate == nil {
		return Sprint{}, errors.New("iteration has no start or finish date")
	}
	// The API stores date-only values as UTC midnight. Reading them in UTC is
	// what keeps the civil date stable in every local zone; .Local() would
	// move it back a day west of UTC (09-RESEARCH section 3).
	start, finish := a.StartDate.Time.UTC(), a.FinishDate.Time.UTC()
	if finish.Before(start) {
		return Sprint{}, errors.New("iteration finishes before it starts")
	}
	s := Sprint{
		Project:    team.Project,
		Team:       team.Team,
		Name:       deref(it.Name),
		Path:       deref(it.Path),
		StartDate:  start.Format("2006-01-02"),
		FinishDate: finish.Format("2006-01-02"),
	}
	if it.Id != nil {
		s.IterationID = it.Id.String()
	}
	return s, nil
}

// cadenceMismatch reports one finding when the sprints do not all share the
// same start and finish dates. Teams are listed in input order; detail groups
// them by date range.
func cadenceMismatch(sprints []Sprint) (Finding, bool) {
	var ranges []string
	byRange := map[string][]string{}
	teams := []string{}
	project := ""
	for i, s := range sprints {
		r := s.StartDate + ".." + s.FinishDate
		if _, seen := byRange[r]; !seen {
			ranges = append(ranges, r)
		}
		name := s.Project + "/" + s.Team
		byRange[r] = append(byRange[r], name)
		teams = append(teams, name)
		if i == 0 {
			project = s.Project
		} else if project != s.Project {
			project = ""
		}
	}
	if len(ranges) < 2 {
		return Finding{}, false
	}
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		parts = append(parts, fmt.Sprintf("%s (%s)", r, strings.Join(byRange[r], ", ")))
	}
	slices.Sort(parts)
	return Finding{
		Kind:    FindingCadenceMismatch,
		Project: project,
		Teams:   teams,
		Detail:  "teams have different current sprints: " + strings.Join(parts, "; "),
	}, true
}
