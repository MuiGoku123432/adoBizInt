package ado

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	connection       *azuredevops.Connection
	workitemClient   workitemtracking.Client
	workClient       work.Client
	gitClient        git.Client
	buildClient      build.Client
	projects         []string
	pipelines        []string
	currentUser      string
	currentUserEmail string
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
	log := logging.Logger()
	locationClient := location.NewClient(ctx, connection)
	currentUser := ""
	currentUserEmail := cfg.UserEmail // Use configured email if available
	if locationClient != nil {
		connData, err := locationClient.GetConnectionData(ctx, location.GetConnectionDataArgs{})
		if err == nil && connData != nil && connData.AuthorizedUser != nil {
			if connData.AuthorizedUser.ProviderDisplayName != nil {
				currentUser = *connData.AuthorizedUser.ProviderDisplayName
			}
		}
	}
	log.Info("Current user identity", "displayName", currentUser, "email", currentUserEmail)

	return &Client{
		connection:       connection,
		workitemClient:   workitemClient,
		workClient:       workClient,
		gitClient:        gitClient,
		buildClient:      buildClient,
		projects:         cfg.Projects,
		pipelines:        cfg.Pipelines,
		currentUser:      currentUser,
		currentUserEmail: currentUserEmail,
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

func (c *Client) CurrentUserEmail() string {
	return c.currentUserEmail
}

// GetRecentIterations returns iteration paths for sprints active in the last 90 days
func (c *Client) GetRecentIterations(ctx context.Context, project string) ([]string, error) {
	log := logging.Logger()

	// Get all project iterations using classification nodes API
	depth := 10 // Get nested iterations up to 10 levels deep
	structureGroup := workitemtracking.TreeStructureGroupValues.Iterations
	nodes, err := c.workitemClient.GetClassificationNode(ctx, workitemtracking.GetClassificationNodeArgs{
		Project:        &project,
		StructureGroup: &structureGroup,
		Depth:          &depth,
	})
	if err != nil {
		log.Warn("Failed to get project iterations", "project", project, "error", err)
		return nil, err
	}

	if nodes == nil {
		log.Info("No iterations found for project", "project", project)
		return nil, nil
	}

	// Collect all iteration paths with dates from the last 30 days
	cutoffDate := time.Now().AddDate(0, 0, -30)
	var recentPaths []string

	// Recursively collect iterations
	var collectIterations func(node *workitemtracking.WorkItemClassificationNode, parentPath string)
	collectIterations = func(node *workitemtracking.WorkItemClassificationNode, parentPath string) {
		if node == nil {
			return
		}

		currentPath := parentPath
		if node.Name != nil {
			if parentPath != "" {
				currentPath = parentPath + "\\" + *node.Name
			} else {
				currentPath = *node.Name
			}
		}

		// Check if this iteration has dates within last 90 days
		if node.Attributes != nil {
			attrs := *node.Attributes
			startDate, hasStart := attrs["startDate"]
			finishDate, hasFinish := attrs["finishDate"]

			var start, finish time.Time
			if hasStart && startDate != nil {
				if t, ok := startDate.(time.Time); ok {
					start = t
				} else if s, ok := startDate.(string); ok {
					start, _ = time.Parse(time.RFC3339, s)
				}
			}
			if hasFinish && finishDate != nil {
				if t, ok := finishDate.(time.Time); ok {
					finish = t
				} else if s, ok := finishDate.(string); ok {
					finish, _ = time.Parse(time.RFC3339, s)
				}
			}

			// Include iteration if it overlaps with last 90 days
			// (start date is after cutoff OR finish date is after cutoff)
			if !start.IsZero() || !finish.IsZero() {
				includeIteration := false
				if !finish.IsZero() && finish.After(cutoffDate) {
					includeIteration = true
				} else if !start.IsZero() && start.After(cutoffDate) {
					includeIteration = true
				}

				if includeIteration && node.Path != nil {
					// Clean up the iteration path for WIQL
					// API returns paths like \Project\Iteration\Sprint or \Process\Iteration\Sprint
					// WIQL expects ProjectName\Sprint (no "Iteration\" in the middle)
					path := strings.TrimPrefix(*node.Path, "\\")
					path = strings.TrimPrefix(path, "Process\\Iteration\\")

					// Remove "\Iteration\" from the middle of the path (e.g., "Project\Iteration\Sprint" -> "Project\Sprint")
					path = strings.Replace(path, "\\Iteration\\", "\\", 1)

					// If path doesn't start with project name, prepend it
					if !strings.HasPrefix(path, project+"\\") && !strings.HasPrefix(path, project+"/") {
						path = project + "\\" + path
					}

					recentPaths = append(recentPaths, path)
				}
			}
		}

		// Process children
		if node.Children != nil {
			for _, child := range *node.Children {
				collectIterations(&child, currentPath)
			}
		}
	}

	// Start from the root node's children (the root is just "Iterations")
	if nodes.Children != nil {
		for _, child := range *nodes.Children {
			collectIterations(&child, "")
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

	// Get recent iterations (sprints active in last 90 days)
	iterationFilter := ""
	iterations, err := c.GetRecentIterations(ctx, project)
	if err != nil {
		log.Warn("Failed to get iterations for dashboard, using date filter only", "project", project, "error", err)
	} else if len(iterations) > 0 {
		escapedPaths := make([]string, len(iterations))
		for i, path := range iterations {
			escapedPaths[i] = "'" + strings.ReplaceAll(path, "'", "''") + "'"
		}
		iterationFilter = " AND [System.IterationPath] IN (" + strings.Join(escapedPaths, ", ") + ")"
	}

	// Get work item count using WIQL query with filters
	wiql := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = '" + project + "' AND [System.State] NOT IN ('Done', 'Closed', 'Removed')" + iterationFilter
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
	ID              int
	Title           string
	Summary         string
	StoryPoints     int
	Project         string
	Type            string
	State           string
	AssignedTo      string
	AssignedToEmail string
}

// WorkItemDetail represents a work item with full details
type WorkItemDetail struct {
	WorkItem
	Description        string
	AcceptanceCriteria string
	ReproSteps         string // For bugs
	Attachments        []Attachment
	CreatedDate        time.Time
	ModifiedDate       time.Time
	CreatedBy          string
	ModifiedBy         string
	IterationPath      string
	AreaPath           string
}

// Attachment represents a work item attachment
type Attachment struct {
	Name string
	URL  string
	Size int64
}

// PullRequest represents a pull request for display
type PullRequest struct {
	ID             int
	Title          string
	Description    string
	Project        string
	RepositoryID   string
	RepositoryName string
	SourceBranch   string
	TargetBranch   string
	CreatedBy      string
	CreatedByID    string
	CreationDate   time.Time
	Status         string
	IsDraft        bool
	ReviewerCount  int
	ApprovalStatus string
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

	// Get recent iterations (sprints active in last 90 days)
	iterationFilter := ""
	iterations, err := c.GetRecentIterations(ctx, project)
	if err != nil {
		log.Warn("Failed to get iterations, using date filter only", "project", project, "error", err)
	} else if len(iterations) > 0 {
		escapedPaths := make([]string, len(iterations))
		for i, path := range iterations {
			escapedPaths[i] = "'" + strings.ReplaceAll(path, "'", "''") + "'"
		}
		iterationFilter = " AND [System.IterationPath] IN (" + strings.Join(escapedPaths, ", ") + ")"
		log.Info("Using iteration filter", "project", project, "sprints", iterations)
	}

	// Query for work items with relevant fields
	wiql := "SELECT [System.Id], [System.Title], [System.Description], [Microsoft.VSTS.Scheduling.StoryPoints], [System.WorkItemType], [System.State], [System.AssignedTo] FROM WorkItems WHERE [System.TeamProject] = '" + project + "' AND [System.State] NOT IN ('Done', 'Closed', 'Removed')" + iterationFilter + " ORDER BY [System.Id] DESC"
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

	// Fetch full work item details in batches (Azure DevOps API limit is 200 IDs per request)
	const batchSize = 200
	fields := []string{"System.Id", "System.Title", "System.Description", "Microsoft.VSTS.Scheduling.StoryPoints", "System.WorkItemType", "System.State", "System.AssignedTo"}

	var allItems []workitemtracking.WorkItem
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batchIDs := ids[i:end]

		log.Debug("Fetching work item batch", "project", project, "batch", i/batchSize+1, "ids_in_batch", len(batchIDs), "total_ids", len(ids))

		items, err := c.workitemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
			Ids:    &batchIDs,
			Fields: &fields,
		})
		if err != nil {
			log.Error("Failed to get work item batch", "project", project, "batch", i/batchSize+1, "error", err)
			continue // Skip failed batches, get as many as possible
		}
		if items != nil {
			allItems = append(allItems, *items...)
		}
	}

	var workItems []WorkItem
	for _, item := range allItems {
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
				if uniqueName, ok := v["uniqueName"].(string); ok {
					wi.AssignedToEmail = uniqueName
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

// UpdateWorkItemStateWithEffort updates the state of a work item along with effort fields
// Used for Tasks transitioning to Closed which require effort tracking
func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, itemID int, newState string, originalEstimate, remaining, completed float64) error {
	log := logging.Logger()

	op := webapi.OperationValues.Add
	statePath := "/fields/System.State"
	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
	remainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	completedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"

	document := []webapi.JsonPatchOperation{
		{
			Op:    &op,
			Path:  &statePath,
			Value: newState,
		},
		{
			Op:    &op,
			Path:  &originalEstimatePath,
			Value: originalEstimate,
		},
		{
			Op:    &op,
			Path:  &remainingPath,
			Value: remaining,
		},
		{
			Op:    &op,
			Path:  &completedPath,
			Value: completed,
		},
	}

	_, err := c.workitemClient.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
		Id:       &itemID,
		Document: &document,
	})

	if err != nil {
		log.Error("Failed to update work item state with effort", "id", itemID, "state", newState, "error", err)
		return err
	}

	log.Info("Work item state updated with effort", "id", itemID, "state", newState,
		"originalEstimate", originalEstimate, "remaining", remaining, "completed", completed)
	return nil
}

// GetWorkItemDetail fetches full details for a single work item
func (c *Client) GetWorkItemDetail(ctx context.Context, id int) (*WorkItemDetail, error) {
	log := logging.Logger()

	// Use Relations expand to get attachments (can't use Fields with Expand)
	expand := workitemtracking.WorkItemExpandValues.Relations

	item, err := c.workitemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:     &id,
		Expand: &expand,
	})
	if err != nil {
		log.Error("Failed to get work item detail", "id", id, "error", err)
		return nil, err
	}

	if item == nil || item.Fields == nil {
		return nil, nil
	}

	detail := &WorkItemDetail{}
	itemFields := *item.Fields

	// Basic fields
	detail.ID = id
	if title, ok := itemFields["System.Title"].(string); ok {
		detail.Title = title
	}
	if desc, ok := itemFields["System.Description"].(string); ok {
		detail.Description = desc
	}
	if wiType, ok := itemFields["System.WorkItemType"].(string); ok {
		detail.Type = wiType
	}
	if state, ok := itemFields["System.State"].(string); ok {
		detail.State = state
	}
	if points, ok := itemFields["Microsoft.VSTS.Scheduling.StoryPoints"].(float64); ok {
		detail.StoryPoints = int(points)
	}
	if ac, ok := itemFields["Microsoft.VSTS.Common.AcceptanceCriteria"].(string); ok {
		detail.AcceptanceCriteria = ac
	}
	if repro, ok := itemFields["Microsoft.VSTS.TCM.ReproSteps"].(string); ok {
		detail.ReproSteps = repro
	}
	if iter, ok := itemFields["System.IterationPath"].(string); ok {
		detail.IterationPath = iter
	}
	if area, ok := itemFields["System.AreaPath"].(string); ok {
		detail.AreaPath = area
	}

	// Assignee
	if assignedTo, ok := itemFields["System.AssignedTo"]; ok && assignedTo != nil {
		if v, ok := assignedTo.(map[string]interface{}); ok {
			if displayName, ok := v["displayName"].(string); ok {
				detail.AssignedTo = displayName
			}
		}
	}

	// Created/Modified info
	if createdBy, ok := itemFields["System.CreatedBy"]; ok && createdBy != nil {
		if v, ok := createdBy.(map[string]interface{}); ok {
			if displayName, ok := v["displayName"].(string); ok {
				detail.CreatedBy = displayName
			}
		}
	}
	if changedBy, ok := itemFields["System.ChangedBy"]; ok && changedBy != nil {
		if v, ok := changedBy.(map[string]interface{}); ok {
			if displayName, ok := v["displayName"].(string); ok {
				detail.ModifiedBy = displayName
			}
		}
	}

	// Parse dates
	if createdDate, ok := itemFields["System.CreatedDate"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdDate); err == nil {
			detail.CreatedDate = t
		}
	}
	if changedDate, ok := itemFields["System.ChangedDate"].(string); ok {
		if t, err := time.Parse(time.RFC3339, changedDate); err == nil {
			detail.ModifiedDate = t
		}
	}

	// Extract attachments from relations
	if item.Relations != nil {
		for _, rel := range *item.Relations {
			if rel.Rel != nil && *rel.Rel == "AttachedFile" {
				attachment := Attachment{}
				if rel.Url != nil {
					attachment.URL = *rel.Url
				}
				if rel.Attributes != nil {
					attrs := *rel.Attributes
					if name, ok := attrs["name"].(string); ok {
						attachment.Name = name
					}
					if size, ok := attrs["resourceSize"].(float64); ok {
						attachment.Size = int64(size)
					}
				}
				if attachment.Name != "" {
					detail.Attachments = append(detail.Attachments, attachment)
				}
			}
		}
	}

	log.Debug("Got work item detail", "id", id, "title", detail.Title, "attachments", len(detail.Attachments))
	return detail, nil
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

// DownloadAttachment downloads an attachment to the specified path
func (c *Client) DownloadAttachment(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	// connection.AuthorizationString is already "Basic base64..." format
	req.Header.Set("Authorization", c.connection.AuthorizationString)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// GetPullRequests fetches all active pull requests across specified projects
func (c *Client) GetPullRequests(ctx context.Context, projects []string) ([]PullRequest, error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allPRs []PullRequest

	for _, project := range targetProjects {
		prs, err := c.gitClient.GetPullRequestsByProject(ctx, git.GetPullRequestsByProjectArgs{
			Project: &project,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				Status: &git.PullRequestStatusValues.Active,
			},
		})
		if err != nil {
			log.Error("Failed to get pull requests", "project", project, "error", err)
			continue
		}

		if prs != nil {
			for _, pr := range *prs {
				allPRs = append(allPRs, c.convertPR(pr, project))
			}
		}
	}

	log.Info("Pull requests fetched", "total_count", len(allPRs))
	return allPRs, nil
}

func (c *Client) convertPR(pr git.GitPullRequest, project string) PullRequest {
	result := PullRequest{
		Project: project,
	}

	if pr.PullRequestId != nil {
		result.ID = *pr.PullRequestId
	}
	if pr.Title != nil {
		result.Title = *pr.Title
	}
	if pr.Description != nil {
		result.Description = *pr.Description
	}
	if pr.Repository != nil {
		if pr.Repository.Id != nil {
			result.RepositoryID = pr.Repository.Id.String()
		}
		if pr.Repository.Name != nil {
			result.RepositoryName = *pr.Repository.Name
		}
	}
	if pr.SourceRefName != nil {
		result.SourceBranch = strings.TrimPrefix(*pr.SourceRefName, "refs/heads/")
	}
	if pr.TargetRefName != nil {
		result.TargetBranch = strings.TrimPrefix(*pr.TargetRefName, "refs/heads/")
	}
	if pr.CreatedBy != nil && pr.CreatedBy.DisplayName != nil {
		result.CreatedBy = *pr.CreatedBy.DisplayName
	}
	if pr.CreatedBy != nil && pr.CreatedBy.Id != nil {
		result.CreatedByID = *pr.CreatedBy.Id
	}
	if pr.CreationDate != nil {
		result.CreationDate = pr.CreationDate.Time
	}
	if pr.Status != nil {
		result.Status = string(*pr.Status)
	}
	if pr.IsDraft != nil {
		result.IsDraft = *pr.IsDraft
	}
	if pr.Reviewers != nil {
		result.ReviewerCount = len(*pr.Reviewers)
		result.ApprovalStatus = calculateApprovalStatus(*pr.Reviewers)
	}

	return result
}

func calculateApprovalStatus(reviewers []git.IdentityRefWithVote) string {
	hasApproval := false
	hasRejection := false
	hasWaiting := false

	for _, r := range reviewers {
		if r.Vote != nil {
			switch *r.Vote {
			case 10, 5: // Approved or Approved with suggestions
				hasApproval = true
			case -10: // Rejected
				hasRejection = true
			case -5: // Waiting for author
				hasWaiting = true
			}
		}
	}

	if hasRejection {
		return "Rejected"
	}
	if hasWaiting {
		return "Waiting"
	}
	if hasApproval {
		return "Approved"
	}
	return "No votes"
}

// ApprovePullRequest approves a pull request (vote=10)
func (c *Client) ApprovePullRequest(ctx context.Context, project, repositoryID string, prID int) error {
	log := logging.Logger()

	// Get current user's ID from the connection data
	locationClient := location.NewClient(ctx, c.connection)
	connData, err := locationClient.GetConnectionData(ctx, location.GetConnectionDataArgs{})
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	var userID string
	if connData != nil && connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
		userID = connData.AuthenticatedUser.Id.String()
	} else {
		return fmt.Errorf("could not determine current user ID")
	}

	vote := 10 // Approved
	reviewer := &git.IdentityRefWithVote{
		Vote: &vote,
	}

	_, err = c.gitClient.CreatePullRequestReviewer(ctx, git.CreatePullRequestReviewerArgs{
		Reviewer:      reviewer,
		RepositoryId:  &repositoryID,
		PullRequestId: &prID,
		ReviewerId:    &userID,
		Project:       &project,
	})

	if err != nil {
		log.Error("Failed to approve pull request", "prID", prID, "error", err)
		return err
	}

	log.Info("Pull request approved", "prID", prID)
	return nil
}

// CompletePullRequest completes/merges a pull request with no-fast-forward strategy
func (c *Client) CompletePullRequest(ctx context.Context, project, repositoryID string, prID int) error {
	log := logging.Logger()

	// Get the PR to obtain the last merge source commit
	pr, err := c.gitClient.GetPullRequest(ctx, git.GetPullRequestArgs{
		RepositoryId:  &repositoryID,
		PullRequestId: &prID,
		Project:       &project,
	})
	if err != nil {
		return fmt.Errorf("failed to get PR details: %w", err)
	}

	// Get the last merge source commit ID
	var lastCommit string
	if pr.LastMergeSourceCommit != nil && pr.LastMergeSourceCommit.CommitId != nil {
		lastCommit = *pr.LastMergeSourceCommit.CommitId
	} else {
		return fmt.Errorf("could not determine last merge source commit")
	}

	// Set up completion options
	deleteSourceBranch := false // Keep source branch as per requirements
	mergeStrategy := git.GitPullRequestMergeStrategyValues.NoFastForward
	completedStatus := git.PullRequestStatusValues.Completed

	completionOptions := &git.GitPullRequestCompletionOptions{
		DeleteSourceBranch: &deleteSourceBranch,
		MergeStrategy:      &mergeStrategy,
	}

	prUpdate := &git.GitPullRequest{
		Status:            &completedStatus,
		CompletionOptions: completionOptions,
		LastMergeSourceCommit: &git.GitCommitRef{
			CommitId: &lastCommit,
		},
	}

	_, err = c.gitClient.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: prUpdate,
		RepositoryId:           &repositoryID,
		PullRequestId:          &prID,
		Project:                &project,
	})

	if err != nil {
		log.Error("Failed to complete pull request", "prID", prID, "error", err)
		return err
	}

	log.Info("Pull request completed", "prID", prID)
	return nil
}
