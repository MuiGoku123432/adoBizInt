package ado

import (
	"context"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"
	"github.com/microsoft/azure-devops-go-api/azuredevops/release"
	"github.com/microsoft/azure-devops-go-api/azuredevops/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
)

// Client is the twelfth exported struct in this package and the one that is
// deliberately NOT given JSON struct tags (OUT-02, D-04). It is not
// serializable: every field is unexported, and it holds live SDK sub-clients.
// The omission is intentional, not an oversight — do not "fix" it by adding
// tags. Plan 02-04 asserts that this file contains zero encoding/json struct
// tags, so adding one here fails an acceptance criterion.
type Client struct {
	workitemClient workItemAPI
	// workClient is intentionally dormant; the v2 scrum requirements consume
	// this seam, so it must not be removed as dead code.
	workClient         work.Client
	gitClient          gitAPI
	buildClient        buildAPI
	releaseClient      releaseAPI
	projects           []string
	pipelines          []string
	releaseDefinitions []string // Separate release definition names (falls back to pipelines if empty)
	currentUser        string
	currentUserID      string
	currentUserEmail   string
	fetchTimeout       time.Duration
	getConnectionData  func(context.Context, location.GetConnectionDataArgs) (*location.ConnectionData, error)
}

// newConnection builds the SDK connection with the per-call budget already
// installed.
//
// The ordering matters and is the whole point of this helper: azuredevops.NewClient
// reads connection.Timeout once at construction, copies it into the http.Client,
// and the Connection caches the constructed client. Setting Timeout after any
// sub-client has been built compiles, vets, passes every test, and bounds nothing.
//
// A zero configured budget leaves Timeout nil rather than installing a zero
// duration, which http.Client reads as "no deadline at all".
func newConnection(cfg *config.Config) *azuredevops.Connection {
	connection := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)
	if cfg.RequestTimeout > 0 {
		timeout := cfg.RequestTimeout
		connection.Timeout = &timeout
	}
	return connection
}

func NewClient(cfg *config.Config) (*Client, error) {
	connection := newConnection(cfg)

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

	releaseClient, err := release.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	// Fetch current user from PAT identity
	log := logging.Logger()
	locationClient := location.NewClient(ctx, connection)
	currentUser := ""
	currentUserID := ""
	currentUserEmail := cfg.UserEmail // Use configured email if available
	var getConnectionData func(context.Context, location.GetConnectionDataArgs) (*location.ConnectionData, error)
	if locationClient != nil {
		getConnectionData = locationClient.GetConnectionData
		connData, err := getConnectionData(ctx, location.GetConnectionDataArgs{})
		if err == nil && connData != nil {
			if connData.AuthorizedUser != nil && connData.AuthorizedUser.ProviderDisplayName != nil {
				currentUser = *connData.AuthorizedUser.ProviderDisplayName
			}
			if connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
				currentUserID = connData.AuthenticatedUser.Id.String()
			}
		}
	}
	if currentUserID == "" {
		log.Warn("Could not cache current user ID; pull request approval will retry identity lookup")
	}
	log.Info("Current user identity", "displayName", currentUser, "email", currentUserEmail)

	// Use separate release definitions if configured, otherwise fall back to pipelines
	releaseDefinitions := cfg.ReleaseDefinitions
	if len(releaseDefinitions) == 0 {
		releaseDefinitions = cfg.Pipelines
	}

	return &Client{
		workitemClient:     workitemClient,
		workClient:         workClient,
		gitClient:          gitClient,
		buildClient:        buildClient,
		releaseClient:      releaseClient,
		projects:           cfg.Projects,
		pipelines:          cfg.Pipelines,
		releaseDefinitions: releaseDefinitions,
		currentUser:        currentUser,
		currentUserID:      currentUserID,
		currentUserEmail:   currentUserEmail,
		fetchTimeout:       cfg.FetchTimeout,
		getConnectionData:  getConnectionData,
	}, nil
}

// FetchTimeout reports the configured budget for one whole multi-project
// fan-out. Views read it here rather than having it threaded through six view
// constructors and app.go for a value that already travels with the client.
func (c *Client) FetchTimeout() time.Duration {
	return c.fetchTimeout
}

func (c *Client) Projects() []string {
	return c.projects
}

func (c *Client) Pipelines() []string {
	return c.pipelines
}

func (c *Client) ReleaseDefinitions() []string {
	return c.releaseDefinitions
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

// iterationClause renders the WIQL iteration filter for paths returned by
// GetRecentIterations, or "" when there are none. It is the only escaping path
// for iteration values (D-10 revised): each path is single-quoted with every
// embedded single quote doubled, in input order.
//
// note: two call sites (getProjectWorkItems, getProjectCounts), below the rule
// of three. It is extracted because D-12's structural test forbids a bare
// identifier operand in a WIQL + chain, and this is the one callee
// TestWIQLInterpolationStructural allow-lists (RESEARCH OQ2).
func iterationClause(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	quoted := make([]string, len(paths))
	for i, path := range paths {
		quoted[i] = "'" + strings.ReplaceAll(path, "'", "''") + "'"
	}
	return " AND [System.IterationPath] IN (" + strings.Join(quoted, ", ") + ")"
}

// strPtr is a helper to get a pointer to a string
func strPtr(s string) *string {
	return &s
}
