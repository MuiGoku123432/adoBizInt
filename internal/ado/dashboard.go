package ado

import (
	"context"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"

	"sentinovo.ai/bizInt/internal/logging"
)

// DashboardCounts holds summary counts for the dashboard
type DashboardCounts struct {
	WorkItems int `json:"work_items"`
	PRs       int `json:"prs"`
	Pipelines int `json:"pipelines"`
}

// opDashboard* label the SDK calls in any Failure this read records. The
// dashboard is the only read that issues three independent sub-queries per
// project, so each needs its own label: a partial total is only actionable if
// the user can tell which of the three did not answer.
const (
	opDashboardIterations   = "get dashboard iterations"
	opDashboardWorkItems    = "get dashboard work item count"
	opDashboardPullRequests = "get dashboard pull request count"
	opDashboardBuilds       = "get dashboard build count"
)

// GetDashboardCounts fetches counts for work items, PRs, and pipelines.
// If projects is empty, uses all configured projects.
//
// The returned envelope carries the summed counts as a single-element Items
// slice. The dashboard's payload is one aggregate rather than a list, but the
// failure list and completeness flag must be the same shape as the other seven
// reads so Phase 3 writes its JSON handling once (D-01).
//
// The total is the sum of the projects that answered, including projects that
// answered only partially. It is never zeroed or discarded when something
// failed; instead Complete is false, which is the only signal that a single
// number with no visible seam is understated (D-03).
func (c *Client) GetDashboardCounts(ctx context.Context, projects []string) (Result[DashboardCounts], error) {
	log := logging.Logger()

	// D-01: disclose the fixed filters. The states and iterations mirror
	// getProjectCounts' WIQL, and PRStatus is derived from the constant the PR
	// request sends, so the two cannot drift. Built per call so no two results
	// share a Scope.
	// note: GetWorkItems builds the same two work-item fields; two uses, not extracted.
	scope := &Scope{
		ExcludedStates: []string{"Done", "Closed", "Removed"},
		Iterations:     "recent",
		PRStatus:       string(git.PullRequestStatusValues.Active),
	}

	// Use provided projects or fall back to configured projects
	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	log.Debug("GetDashboardCounts called",
		"requested_projects", projects,
		"target_projects", targetProjects,
		"configured_projects", c.projects,
	)

	if len(targetProjects) == 0 {
		log.Warn("No projects to fetch counts for - projects list is empty")
		result := newResult([]DashboardCounts{{}}, nil)
		result.Scope = scope
		return result, nil
	}

	counts := &DashboardCounts{}
	var failures []Failure

	// Aggregate counts across all target projects
	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opDashboardWorkItems, err)
			continue
		}

		log.Debug("Fetching counts for project", "project", project)
		// Every project is visited, including ones that already failed: D-08
		// rejected short-circuiting, and a project that answered partially still
		// contributes what it did return.
		projectCounts, projectFailures := c.getProjectCounts(ctx, project)
		if len(projectFailures) > 0 {
			log.Error("Incomplete counts for project", "project", project, "failure_count", len(projectFailures))
			failures = append(failures, projectFailures...)
		}
		log.Debug("Got counts for project",
			"project", project,
			"work_items", projectCounts.WorkItems,
			"prs", projectCounts.PRs,
			"pipelines", projectCounts.Pipelines,
		)
		counts.WorkItems += projectCounts.WorkItems
		counts.PRs += projectCounts.PRs
		counts.Pipelines += projectCounts.Pipelines
	}

	log.Info("Dashboard counts fetched",
		"total_work_items", counts.WorkItems,
		"total_prs", counts.PRs,
		"total_pipelines", counts.Pipelines,
		"failure_count", len(failures),
	)

	result := newResult([]DashboardCounts{*counts}, failures)
	result.Scope = scope
	return result, nil
}

// getProjectCounts fetches counts for a single project. It returns both the
// counts it managed to collect and the failures it hit, so a project that
// answered two of its three sub-queries still contributes those two to the
// dashboard total while the third is reported by name (D-03).
//
// It deliberately does not return an error: an error return would push the
// caller back onto a `continue` and discard the counts this project did
// provide, degrading "the sum of the projects that answered" into "the sum of
// the projects that answered perfectly". The counts value is non-nil on every
// path, including the path where all three sub-queries failed.
func (c *Client) getProjectCounts(ctx context.Context, project string) (*DashboardCounts, []Failure) {
	log := logging.Logger()
	counts := &DashboardCounts{}
	var failures []Failure

	// Get recent iterations (sprints active in last 30 days)
	iterations, err := c.GetRecentIterations(ctx, project)
	if err != nil {
		log.Warn("Failed to get iterations for dashboard, using date filter only", "project", project, "error", err)
		// This is a degradation, not a neutral fallback: the WIQL filter below
		// silently loses its iteration clause, so the work-item count that gets
		// summed into the dashboard total was taken under a different query than
		// intended. A count produced under a substituted filter is presented with
		// the same authority as a correct one, so it is recorded rather than left
		// silent (T-02-05).
		failures = appendFailure(failures, project, opDashboardIterations, err)
	}

	// Get work item count using WIQL query with filters. The project never
	// enters the query text: @project resolves server-side from args.Project,
	// which the SDK sends in the request route (D-10 revised).
	wiql := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project AND [System.State] NOT IN ('Done', 'Closed', 'Removed')" + iterationClause(iterations)
	query := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &wiql},
		Project: &project,
	}

	// The query text is not logged (D-11).
	log.Info("Executing dashboard WIQL query", "project", project, "iterations", len(iterations))

	workItems, err := c.workitemClient.QueryByWiql(ctx, query)
	if err != nil {
		log.Error("Failed to query work items", "project", project, "error", err)
		failures = appendFailure(failures, project, opDashboardWorkItems, err)
	} else if workItems != nil && workItems.WorkItems != nil {
		counts.WorkItems = len(*workItems.WorkItems)
		log.Info("Dashboard work item count", "project", project, "count", counts.WorkItems)
	}

	// Get PR count
	prs, err := c.gitClient.GetPullRequestsByProject(ctx, git.GetPullRequestsByProjectArgs{
		Project: &project,
		SearchCriteria: &git.GitPullRequestSearchCriteria{
			Status: &git.PullRequestStatusValues.Active,
		},
	})
	if err != nil {
		log.Error("Failed to get pull requests", "project", project, "error", err)
		failures = appendFailure(failures, project, opDashboardPullRequests, err)
	} else if prs != nil {
		counts.PRs = len(*prs)
	}

	// Get pipeline/build count only if specific pipelines are configured
	if len(c.pipelines) > 0 {
		builds, err := c.buildClient.GetBuilds(ctx, build.GetBuildsArgs{
			Project: &project,
		})
		if err != nil {
			log.Error("Failed to get builds", "project", project, "error", err)
			failures = appendFailure(failures, project, opDashboardBuilds, err)
		} else if builds != nil && builds.Value != nil {
			// Filter to only configured pipelines using normalized matching
			for _, b := range builds.Value {
				if b.Definition != nil && b.Definition.Name != nil {
					if pipelineNameMatches(*b.Definition.Name, c.pipelines) {
						counts.Pipelines++
					}
				}
			}
			log.Info("Pipeline count", "project", project, "configured_pipelines", c.pipelines, "count", counts.Pipelines)
		}
	}

	return counts, failures
}
