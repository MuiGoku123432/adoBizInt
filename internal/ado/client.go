package ado

import (
	"context"
	"sort"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"
	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
)

type Client struct {
	connection     *azuredevops.Connection
	workitemClient workitemtracking.Client
	workClient     work.Client
	gitClient      git.Client
	buildClient    build.Client
	projects       []string
	pipelines      []string
	currentUser    string
}

func NewClient(cfg *config.Config) (*Client, error) {
	connection := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)

	ctx := context.Background()

	workitemClient, err := workitemtracking.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	gitClient, err := git.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	buildClient, err := build.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	workClient, err := work.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	// Fetch current user from PAT identity
	locationClient := location.NewClient(ctx, connection)
	currentUser := ""
	if locationClient != nil {
		connData, err := locationClient.GetConnectionData(ctx, location.GetConnectionDataArgs{})
		if err == nil && connData != nil && connData.AuthorizedUser != nil && connData.AuthorizedUser.ProviderDisplayName != nil {
			currentUser = *connData.AuthorizedUser.ProviderDisplayName
		}
	}

	return &Client{
		connection:     connection,
		workitemClient: workitemClient,
		workClient:     workClient,
		gitClient:      gitClient,
		buildClient:    buildClient,
		projects:       cfg.Projects,
		pipelines:      cfg.Pipelines,
		currentUser:    currentUser,
	}, nil
}

func (c *Client) Projects() []string {
	return c.projects
}

func (c *Client) WorkItemClient() workitemtracking.Client {
	return c.workitemClient
}

func (c *Client) GitClient() git.Client {
	return c.gitClient
}

func (c *Client) BuildClient() build.Client {
	return c.buildClient
}

func (c *Client) CurrentUser() string {
	return c.currentUser
}

// GetRecentIterations returns iteration paths for the current and previous sprint
func (c *Client) GetRecentIterations(ctx context.Context, project string) ([]string, error) {
	log := logging.Logger()

	// Get all team iterations (without timeframe filter to get all)
	iterations, err := c.workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
		Project: &project,
	})
	if err != nil {
		log.Warn("Failed to get team iterations", "project", project, "error", err)
		return nil, err
	}

	if iterations == nil || len(*iterations) == 0 {
		log.Info("No iterations found for project", "project", project)
		return nil, nil
	}

	iterList := *iterations

	// Log all iterations for debugging
	for _, iter := range iterList {
		timeFrame := "unknown"
		startDate := "unknown"
		if iter.Attributes != nil {
			if iter.Attributes.TimeFrame != nil {
				timeFrame = string(*iter.Attributes.TimeFrame)
			}
			if iter.Attributes.StartDate != nil {
				startDate = iter.Attributes.StartDate.Time.Format("2006-01-02")
			}
		}
		path := "unknown"
		if iter.Path != nil {
			path = *iter.Path
		}
		log.Info("Found iteration", "project", project, "path", path, "timeFrame", timeFrame, "startDate", startDate)
	}

	// Find current and past iterations based on TimeFrame attribute
	var currentPath string
	var pastPaths []string

	for _, iter := range iterList {
		if iter.Attributes == nil || iter.Path == nil {
			continue
		}

		timeFrame := ""
		if iter.Attributes.TimeFrame != nil {
			timeFrame = strings.ToLower(string(*iter.Attributes.TimeFrame))
		}

		if timeFrame == "current" {
			currentPath = *iter.Path
			log.Info("Selected CURRENT iteration", "path", *iter.Path)
		} else if timeFrame == "past" {
			pastPaths = append(pastPaths, *iter.Path)
		}
	}

	// Build result: current + most recent past
	var recentPaths []string
	if currentPath != "" {
		recentPaths = append(recentPaths, currentPath)
	}

	// Sort past iterations by start date descending to get most recent past first
	if len(pastPaths) > 0 {
		// Find the iteration objects for past paths and sort by date
		var pastIters []work.TeamSettingsIteration
		for _, iter := range iterList {
			if iter.Path == nil {
				continue
			}
			for _, pp := range pastPaths {
				if *iter.Path == pp {
					pastIters = append(pastIters, iter)
					break
				}
			}
		}
		sort.Slice(pastIters, func(i, j int) bool {
			if pastIters[i].Attributes == nil || pastIters[i].Attributes.StartDate == nil {
				return false
			}
			if pastIters[j].Attributes == nil || pastIters[j].Attributes.StartDate == nil {
				return true
			}
			return pastIters[i].Attributes.StartDate.Time.After(pastIters[j].Attributes.StartDate.Time)
		})

		// Add most recent past iteration
		if len(pastIters) > 0 && pastIters[0].Path != nil {
			recentPaths = append(recentPaths, *pastIters[0].Path)
			log.Info("Selected PAST iteration", "path", *pastIters[0].Path)
		}
	}

	// Fallback if no current found - use most recent by date
	if len(recentPaths) == 0 {
		log.Warn("No current iteration found, falling back to most recent by date", "project", project)
		sort.Slice(iterList, func(i, j int) bool {
			if iterList[i].Attributes == nil || iterList[i].Attributes.StartDate == nil {
				return false
			}
			if iterList[j].Attributes == nil || iterList[j].Attributes.StartDate == nil {
				return true
			}
			return iterList[i].Attributes.StartDate.Time.After(iterList[j].Attributes.StartDate.Time)
		})
		for i, iter := range iterList {
			if iter.Path != nil && i < 2 {
				recentPaths = append(recentPaths, *iter.Path)
				log.Info("Selected FALLBACK iteration", "path", *iter.Path)
			}
		}
	}

	log.Info("Final selected iterations", "project", project, "count", len(recentPaths), "paths", recentPaths)
	return recentPaths, nil
}

// DashboardCounts holds summary counts for the dashboard
type DashboardCounts struct {
	WorkItems int
	PRs       int
	Pipelines int
}

// GetDashboardCounts fetches counts for work items, PRs, and pipelines
// If projects is empty, uses all configured projects
func (c *Client) GetDashboardCounts(ctx context.Context, projects []string) (*DashboardCounts, error) {
	log := logging.Logger()

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
		return &DashboardCounts{}, nil
	}

	counts := &DashboardCounts{}

	// Aggregate counts across all target projects
	for _, project := range targetProjects {
		log.Debug("Fetching counts for project", "project", project)
		projectCounts, err := c.getProjectCounts(ctx, project)
		if err != nil {
			log.Error("Failed to get counts for project", "project", project, "error", err)
			continue // Skip projects with errors, aggregate what we can
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
	)

	return counts, nil
}

// getProjectCounts fetches counts for a single project
func (c *Client) getProjectCounts(ctx context.Context, project string) (*DashboardCounts, error) {
	log := logging.Logger()
	counts := &DashboardCounts{}

	// Get work item count using WIQL query with filters (last 90 days, exclude closed states)
	wiql := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = '" + project + "' AND [System.State] NOT IN ('Done', 'Closed', 'Removed') AND [System.ChangedDate] >= @Today - 90"
	query := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &wiql},
		Project: &project,
	}

	log.Info("Executing dashboard WIQL query", "project", project, "query", wiql)

	workItems, err := c.workitemClient.QueryByWiql(ctx, query)
	if err != nil {
		log.Error("Failed to query work items", "project", project, "error", err)
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
		} else if builds != nil && builds.Value != nil {
			// Filter to only configured pipelines
			pipelineSet := make(map[string]bool)
			for _, p := range c.pipelines {
				pipelineSet[p] = true
			}
			for _, b := range builds.Value {
				if b.Definition != nil && b.Definition.Name != nil {
					if pipelineSet[*b.Definition.Name] {
						counts.Pipelines++
					}
				}
			}
			log.Info("Pipeline count", "project", project, "configured_pipelines", c.pipelines, "count", counts.Pipelines)
		}
	}

	return counts, nil
}

// WorkItem represents a work item for display
type WorkItem struct {
	ID          int
	Title       string
	Summary     string
	StoryPoints int
	Project     string
	Type        string
	State       string
	AssignedTo  string
}

// GetWorkItems fetches work items for the specified projects
func (c *Client) GetWorkItems(ctx context.Context, projects []string) ([]WorkItem, error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	log.Debug("GetWorkItems called", "projects", targetProjects)

	if len(targetProjects) == 0 {
		log.Warn("No projects to fetch work items for")
		return []WorkItem{}, nil
	}

	var allItems []WorkItem

	for _, project := range targetProjects {
		items, err := c.getProjectWorkItems(ctx, project)
		if err != nil {
			log.Error("Failed to get work items for project", "project", project, "error", err)
			continue
		}
		allItems = append(allItems, items...)
	}

	log.Info("Work items fetched", "total_count", len(allItems))
	return allItems, nil
}

func (c *Client) getProjectWorkItems(ctx context.Context, project string) ([]WorkItem, error) {
	log := logging.Logger()

	// Query for work items with relevant fields (last 90 days, exclude closed states)
	wiql := "SELECT [System.Id], [System.Title], [System.Description], [Microsoft.VSTS.Scheduling.StoryPoints], [System.WorkItemType], [System.State], [System.AssignedTo] FROM WorkItems WHERE [System.TeamProject] = '" + project + "' AND [System.State] NOT IN ('Done', 'Closed', 'Removed') AND [System.ChangedDate] >= @Today - 90 ORDER BY [System.Id] DESC"
	query := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &wiql},
		Project: &project,
	}

	log.Info("Executing WIQL query", "project", project, "query", wiql)

	result, err := c.workitemClient.QueryByWiql(ctx, query)
	if err != nil {
		return nil, err
	}

	if result == nil || result.WorkItems == nil || len(*result.WorkItems) == 0 {
		log.Info("Query returned no work items", "project", project)
		return []WorkItem{}, nil
	}

	totalFound := len(*result.WorkItems)
	log.Info("Query returned work items", "project", project, "total_found", totalFound)

	// Get the IDs to fetch full details
	var ids []int
	for _, ref := range *result.WorkItems {
		if ref.Id != nil {
			ids = append(ids, *ref.Id)
		}
	}


	if len(ids) == 0 {
		return []WorkItem{}, nil
	}

	// Fetch full work item details
	fields := []string{"System.Id", "System.Title", "System.Description", "Microsoft.VSTS.Scheduling.StoryPoints", "System.WorkItemType", "System.State", "System.AssignedTo"}
	items, err := c.workitemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &project,
		Fields:  &fields,
	})
	if err != nil {
		log.Error("Failed to get work item details", "project", project, "error", err)
		return nil, err
	}

	var workItems []WorkItem
	for _, item := range *items {
		if item.Fields == nil {
			continue
		}
		fields := *item.Fields

		wi := WorkItem{
			Project: project,
		}

		if item.Id != nil {
			wi.ID = *item.Id
		}
		if title, ok := fields["System.Title"].(string); ok {
			wi.Title = title
		}
		if desc, ok := fields["System.Description"].(string); ok {
			wi.Summary = desc
		}
		if points, ok := fields["Microsoft.VSTS.Scheduling.StoryPoints"].(float64); ok {
			wi.StoryPoints = int(points)
		}
		if wiType, ok := fields["System.WorkItemType"].(string); ok {
			wi.Type = wiType
		}
		if state, ok := fields["System.State"].(string); ok {
			wi.State = state
		}
		if assignedTo, ok := fields["System.AssignedTo"]; ok && assignedTo != nil {
			switch v := assignedTo.(type) {
			case map[string]interface{}:
				if displayName, ok := v["displayName"].(string); ok {
					wi.AssignedTo = displayName
				}
			case string:
				wi.AssignedTo = v
			}
		}

		workItems = append(workItems, wi)
	}

	log.Debug("Got work items for project", "project", project, "count", len(workItems))
	return workItems, nil
}

// UpdateWorkItemState updates the state of a work item
func (c *Client) UpdateWorkItemState(ctx context.Context, itemID int, newState string) error {
	log := logging.Logger()

	op := webapi.OperationValues.Add
	path := "/fields/System.State"
	document := []webapi.JsonPatchOperation{
		{
			Op:    &op,
			Path:  &path,
			Value: newState,
		},
	}

	_, err := c.workitemClient.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
		Id:       &itemID,
		Document: &document,
	})

	if err != nil {
		log.Error("Failed to update work item state", "id", itemID, "state", newState, "error", err)
		return err
	}

	log.Info("Work item state updated", "id", itemID, "state", newState)
	return nil
}

// GetNextState returns the next state for a work item type/current state based on config
func GetNextState(config map[string]map[string]string, workItemType, currentState string) (string, bool) {
	if typeMap, ok := config[workItemType]; ok {
		if nextState, ok := typeMap[currentState]; ok {
			return nextState, true
		}
	}
	return "", false
}
