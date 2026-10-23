package ado

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// This file decides which work items belong to a team's sprint (TEAM-06):
// the sprint's iteration id intersected with the team's area paths. An
// iteration is shared by every team in a project (10-RESEARCH: 556 items in
// the iteration, 5 under one team's areas), so the area filter is the part
// that makes a sprint a team's.

// AreaPath is one of a team's area-path settings.
type AreaPath struct {
	Path            string `json:"path"`
	IncludeChildren bool   `json:"include_children"`
}

const (
	opGetTeamAreaPaths       = "get team area paths"
	opResolveSprintIteration = "resolve sprint iteration"
)

// teamAreaPaths reads the team field values. Only an area-path team field is
// supported; any other is an error naming it, never a silent unfiltered read.
func (c *Client) teamAreaPaths(ctx context.Context, team Team) ([]AreaPath, error) {
	v, err := c.workClient.GetTeamFieldValues(ctx, work.GetTeamFieldValuesArgs{Project: &team.Project, Team: &team.TeamID})
	if err != nil {
		return nil, err
	}
	if v == nil || v.Field == nil || deref(v.Field.ReferenceName) != "System.AreaPath" {
		field := ""
		if v != nil && v.Field != nil {
			field = deref(v.Field.ReferenceName)
		}
		return nil, fmt.Errorf("team field is %q; only System.AreaPath is supported", field)
	}
	var areas []AreaPath
	if v.Values != nil {
		for _, fv := range *v.Values {
			if p := deref(fv.Value); p != "" {
				areas = append(areas, AreaPath{Path: p, IncludeChildren: fv.IncludeChildren != nil && *fv.IncludeChildren})
			}
		}
	}
	if len(areas) == 0 {
		return nil, errors.New("team has no area paths")
	}
	return areas, nil
}

// opQuerySprintItems labels the sprint WIQL in any Failure.
const opQuerySprintItems = "query sprint items"

// querySprintIDs runs the one query that selects a team's sprint items, live
// or as of an instant, in the team's context. It has no state filter
// (DATA-01). Its non-literal text comes only from three reviewed escapers, as
// TestWIQLInterpolationStructural requires.
func (c *Client) querySprintIDs(ctx context.Context, team Team, iterationID int, areas []AreaPath, asOf *time.Time) ([]int, error) {
	wiql := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project" +
		iterationIDClause(iterationID) + areaClause(areas) + asOfClause(asOf)
	res, err := c.workitemClient.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &wiql},
		Project: &team.Project,
		Team:    &team.TeamID,
	})
	if err != nil {
		return nil, err
	}
	ids := []int{}
	if res != nil && res.WorkItems != nil {
		for _, r := range *res.WorkItems {
			if r.Id != nil {
				ids = append(ids, *r.Id)
			}
		}
	}
	return ids, nil
}

// iterationIDClause formats the iteration id as an integer with strconv,
// never a string from the server (DATA-01).
func iterationIDClause(id int) string {
	return " AND [System.IterationId] = " + strconv.Itoa(id)
}

// areaClause ORs the team's area paths: UNDER when the setting includes
// children, = when it does not, each quoted with the ” escaper iterationClause
// uses (TEAM-06).
func areaClause(areas []AreaPath) string {
	clauses := make([]string, len(areas))
	for i, a := range areas {
		op := "="
		if a.IncludeChildren {
			op = "UNDER"
		}
		clauses[i] = "[System.AreaPath] " + op + " '" + strings.ReplaceAll(a.Path, "'", "''") + "'"
	}
	return " AND (" + strings.Join(clauses, " OR ") + ")"
}

// asOfClause is empty for a live read, else an ASOF at the UTC instant in
// RFC 3339, the form the spike verified (DATA-04).
func asOfClause(asOf *time.Time) string {
	if asOf == nil {
		return ""
	}
	return " ASOF '" + asOf.UTC().Format(time.RFC3339) + "'"
}

// iterationIDs maps every iteration in a project, keyed by its team-style
// path lowercased ("kinetic\s2w 2026\sprint 218", as ADO compares paths
// without case), to its integer id. It reads the whole
// tree once, as GetRecentIterations does, rather than looking up one node by
// path: the SDK escapes a path route value as a single segment, which the
// spike did not verify the server accepts.
func (c *Client) iterationIDs(ctx context.Context, project string) (map[string]int, error) {
	depth := 10
	group := workitemtracking.TreeStructureGroupValues.Iterations
	root, err := c.workitemClient.GetClassificationNode(ctx, workitemtracking.GetClassificationNodeArgs{
		Project: &project, StructureGroup: &group, Depth: &depth,
	})
	if err != nil {
		return nil, err
	}
	ids := map[string]int{}
	var walk func(n *workitemtracking.WorkItemClassificationNode)
	walk = func(n *workitemtracking.WorkItemClassificationNode) {
		if n.Id != nil && n.Path != nil {
			// Node paths read "\Kinetic\Iteration\S2W 2026"; team iteration
			// paths read "Kinetic\S2W 2026".
			segs := strings.Split(strings.TrimPrefix(*n.Path, `\`), `\`)
			if len(segs) >= 2 {
				ids[strings.ToLower(strings.Join(append(segs[:1:1], segs[2:]...), `\`))] = *n.Id
			}
		}
		if n.Children != nil {
			for i := range *n.Children {
				walk(&(*n.Children)[i])
			}
		}
	}
	if root != nil {
		walk(root)
	}
	return ids, nil
}

// TeamItems is one team's sprint item ids, the input to teamOverlap.
type TeamItems struct {
	Project, Team string
	IDs           []int
}

// teamOverlap reports one team_overlap finding per item claimed by two or
// more teams, in ascending item id order (TEAM-06).
func teamOverlap(sets []TeamItems) []Finding {
	owners := map[int][]string{}
	project := map[int]string{}
	for _, s := range sets {
		for _, id := range s.IDs {
			name := s.Project + "/" + s.Team
			if !slices.Contains(owners[id], name) {
				owners[id] = append(owners[id], name)
			}
			if p, ok := project[id]; !ok {
				project[id] = s.Project
			} else if p != s.Project {
				project[id] = ""
			}
		}
	}
	var ids []int
	for id, teams := range owners {
		if len(teams) > 1 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	findings := []Finding{}
	for _, id := range ids {
		findings = append(findings, Finding{
			Kind:    FindingTeamOverlap,
			Project: project[id],
			Teams:   owners[id],
			Detail:  fmt.Sprintf("work item %d is in the sprint of %d teams: %s", id, len(owners[id]), strings.Join(owners[id], ", ")),
		})
	}
	return findings
}
