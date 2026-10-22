package ado

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
)

// Team is one Azure DevOps team. TeamID is what every later team-scoped call
// uses; names are for display and matching only.
type Team struct {
	Project     string `json:"project"`
	Team        string `json:"team"`
	TeamID      string `json:"team_id"`
	Description string `json:"description"`
}

// opGetTeams labels the SDK call in any Failure this read records.
const opGetTeams = "get teams"

// teamsPageSize is set explicitly on every GetTeams call. Omitting $top leaves
// the page size to the server default, which silently truncates a large
// project (09-RESEARCH section 1).
const teamsPageSize = 100

// projectTeams pages core.GetTeams for one project. Paging is offset-only:
// there is no continuation token and no total count, so it stops on a page
// shorter than the page size, an empty page included.
func (c *Client) projectTeams(ctx context.Context, project string) ([]Team, error) {
	var teams []Team
	top := teamsPageSize
	for skip := 0; ; {
		page, err := c.coreClient.GetTeams(ctx, core.GetTeamsArgs{ProjectId: &project, Top: &top, Skip: &skip})
		if err != nil {
			return nil, err
		}
		n := 0
		if page != nil {
			n = len(*page)
			for _, t := range *page {
				team := Team{Project: project}
				if t.Name != nil {
					team.Team = *t.Name
				}
				if t.Id != nil {
					team.TeamID = t.Id.String()
				}
				if t.Description != nil {
					team.Description = *t.Description
				}
				teams = append(teams, team)
			}
		}
		if n < top {
			return teams, nil
		}
		skip += n
	}
}

// GetTeams lists every team in each project for discovery (`teams list`). It
// needs no teams config. A project whose listing fails is a project-scoped
// failure (team ""), and the other projects still return.
//
// The caller owns the deadline, as for every read in this package.
func (c *Client) GetTeams(ctx context.Context, projects []string) (Result[Team], error) {
	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var all []Team
	var failures []Failure
	for _, project := range targetProjects {
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, "", opGetTeams, err)
			continue
		}
		teams, err := c.projectTeams(ctx, project)
		if err != nil {
			logging.Logger().Error("Failed to get teams", "project", project, "error", err)
			failures = appendFailure(failures, project, "", opGetTeams, err)
			continue
		}
		all = append(all, teams...)
	}
	return newResult(all, failures), nil
}

// ResolveTeams checks each selected "project -> team names" entry against the
// project's full paged team list and returns the matching teams with their IDs,
// in project then configured order (TEAM-01). Names match by config.SameName.
//
// Validation is a precondition, not a read, so it is never partial: the first
// listing error is returned as is (the caller maps auth to exit 2), and a name
// ADO does not know is a *config.ConfigError naming "project/team".
func (c *Client) ResolveTeams(ctx context.Context, projects []string, selected map[string][]string) ([]Team, error) {
	var resolved []Team
	for _, project := range projects {
		names := selected[project]
		if len(names) == 0 {
			continue
		}
		known, err := c.projectTeams(ctx, project)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			i := slices.IndexFunc(known, func(t Team) bool { return config.SameName(t.Team, name) })
			if i < 0 {
				return nil, &config.ConfigError{Field: "teams", Message: fmt.Sprintf("unknown team %q", project+"/"+name)}
			}
			resolved = append(resolved, known[i])
		}
	}
	return resolved, nil
}

// TeamMember is one member of a team, keyed by ID (TEAM-02). A group is
// listed and flagged, never expanded or dropped.
type TeamMember struct {
	Project     string `json:"project"`
	Team        string `json:"team"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	UniqueName  string `json:"unique_name"`
	IsGroup     bool   `json:"is_group"`
	IsTeamAdmin bool   `json:"is_team_admin"`
	Inactive    bool   `json:"inactive"`
}

const opGetTeamMembers = "get team members"

// GetTeamMembers lists the members of each team, paged like GetTeams. A team
// whose listing fails is a per-team failure; the other teams still return.
func (c *Client) GetTeamMembers(ctx context.Context, teams []Team) (Result[TeamMember], error) {
	var all []TeamMember
	var failures []Failure
	for _, team := range teams {
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, team.Project, team.Team, opGetTeamMembers, err)
			continue
		}
		members, err := c.teamMembers(ctx, team)
		if err != nil {
			logging.Logger().Error("Failed to get team members", "project", team.Project, "team", team.Team, "error", err)
			failures = appendFailure(failures, team.Project, team.Team, opGetTeamMembers, err)
			continue
		}
		all = append(all, members...)
	}
	return newResult(all, failures), nil
}

func (c *Client) teamMembers(ctx context.Context, team Team) ([]TeamMember, error) {
	var members []TeamMember
	top := teamsPageSize
	for skip := 0; ; {
		page, err := c.coreClient.GetTeamMembersWithExtendedProperties(ctx, core.GetTeamMembersWithExtendedPropertiesArgs{
			ProjectId: &team.Project, TeamId: &team.TeamID, Top: &top, Skip: &skip,
		})
		if err != nil {
			return nil, err
		}
		n := 0
		if page != nil {
			n = len(*page)
			for _, m := range *page {
				member := TeamMember{Project: team.Project, Team: team.Team}
				if m.IsTeamAdmin != nil {
					member.IsTeamAdmin = *m.IsTeamAdmin
				}
				if id := m.Identity; id != nil {
					member.ID = deref(id.Id)
					member.DisplayName = deref(id.DisplayName)
					member.UniqueName = deref(id.UniqueName)
					member.Inactive = id.Inactive != nil && *id.Inactive
					// IsContainer is deprecated in favour of the descriptor's
					// subject type, so either marks a group (09-RESEARCH section 2).
					d := deref(id.Descriptor)
					member.IsGroup = (id.IsContainer != nil && *id.IsContainer) ||
						strings.HasPrefix(d, "vssgp.") || strings.HasPrefix(d, "aadgp.")
				}
				members = append(members, member)
			}
		}
		if n < top {
			return members, nil
		}
		skip += n
	}
}

// deref returns *p, or "" for nil.
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
