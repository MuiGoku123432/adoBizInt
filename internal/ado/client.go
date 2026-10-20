package ado

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/location"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/transport"
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
	// readRetries is the one counter every retry decorator NewClient builds
	// adds to (D-03); FetchRetries reads it for the CLI summary (D-05).
	readRetries *atomic.Int64
}

// NewClient builds every SDK sub-client this package holds on the one HTTP
// client tr provides, so every request goes through the org-bound transport
// (D-10, D-11).
//
// After resource-area discovery and before any sub-client exists, every
// discovered location is checked against the transport's host rule (D-10a,
// checkAreaHosts); a foreign one returns an org_url ConfigError, which main
// maps to exit 3, and nothing further is sent.
//
// Each sub-client is built explicitly with NewClientWithOptions on its
// discovered resource-area location. The SDK's own constructors cannot be used:
// a typed X.NewClient(ctx, connection), location.NewClient and
// connection.GetClientByUrl all build a default http.Client that bypasses the
// transport. WithHTTPClient also ignores connection.Timeout, so the per-call
// budget lives on tr.HTTPClient() rather than on the connection.
//
// Discovery and identity run on an unbounded startup context, unchanged from
// before the v7 upgrade (g-01-1 is carried forward, not fixed here).
func NewClient(cfg *config.Config, tr *transport.Transport) (*Client, error) {
	conn := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)
	opt := azuredevops.WithHTTPClient(tr.HTTPClient())

	ctx := context.Background()

	root := azuredevops.NewClientWithOptions(conn, conn.BaseUrl, opt)
	discovered, err := root.GetResourceAreas(ctx)
	if err != nil {
		return nil, err
	}
	var areas []azuredevops.ResourceAreaInfo
	if discovered != nil {
		areas = *discovered
	}
	if err := checkAreaHosts(areas, tr); err != nil {
		return nil, err
	}

	witURL, err := areaURL(areas, workitemtracking.ResourceAreaId, conn.BaseUrl)
	if err != nil {
		return nil, err
	}
	gitURL, err := areaURL(areas, git.ResourceAreaId, conn.BaseUrl)
	if err != nil {
		return nil, err
	}
	buildURL, err := areaURL(areas, build.ResourceAreaId, conn.BaseUrl)
	if err != nil {
		return nil, err
	}
	releaseURL, err := areaURL(areas, release.ResourceAreaId, conn.BaseUrl)
	if err != nil {
		return nil, err
	}
	workURL, err := areaURL(areas, work.ResourceAreaId, conn.BaseUrl)
	if err != nil {
		return nil, err
	}

	// D-03: reads on these clients retry a 429 in the service layer, writes
	// never; every decorator shares this one counter.
	retries := new(atomic.Int64)
	workitemClient := &workitemtracking.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, witURL, opt)}
	gitClient := retryingGit{gitAPI: &git.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, gitURL, opt)}, retries: retries}
	buildClient := &build.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, buildURL, opt)}
	releaseClient := &release.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, releaseURL, opt)}
	workClient := &work.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, workURL, opt)}

	// Fetch current user from PAT identity
	log := logging.Logger()
	locationClient := &location.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, conn.BaseUrl, opt)}
	currentUser := ""
	currentUserID := ""
	currentUserEmail := cfg.UserEmail // Use configured email if available
	getConnectionData := locationClient.GetConnectionData
	connData, err := getConnectionData(ctx, location.GetConnectionDataArgs{})
	if err == nil && connData != nil {
		if connData.AuthorizedUser != nil && connData.AuthorizedUser.ProviderDisplayName != nil {
			currentUser = *connData.AuthorizedUser.ProviderDisplayName
		}
		if connData.AuthenticatedUser != nil && connData.AuthenticatedUser.Id != nil {
			currentUserID = connData.AuthenticatedUser.Id.String()
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
		readRetries:        retries,
	}, nil
}

// areaURL returns the base URL a sub-client for resource area id is built on.
// It mirrors the SDK's own lookup (v7 connection.go:84-109): an empty area list
// is the on-prem answer and falls back to baseURL, the org_url the operator
// configured; an area that is missing, or listed with no location, is
// ResourceAreaIdNotRegisteredError. The location is normalized exactly as the
// SDK normalizes it, because live LocationUrls carry the org's own casing and a
// trailing slash, and the SDK's location cache is keyed by the exact base URL
// (Pitfall 7).
func areaURL(areas []azuredevops.ResourceAreaInfo, id uuid.UUID, baseURL string) (string, error) {
	if len(areas) == 0 {
		return baseURL, nil
	}
	for _, area := range areas {
		if area.Id == nil || *area.Id != id {
			continue
		}
		if area.LocationUrl == nil || *area.LocationUrl == "" {
			break
		}
		return strings.ToLower(strings.TrimRight(*area.LocationUrl, "/")), nil
	}
	return "", azuredevops.ResourceAreaIdNotRegisteredError{ResourceAreaId: id, Url: baseURL}
}

// checkAreaHosts is the D-10a startup check. It applies the transport's host
// rule (D-09) to the location of EVERY area the discovery returned, not only
// the five areas a sub-client is built on (08-CONTEXT D-10a confirmation,
// user, 2026-10-19; RESEARCH Open Question 3's narrowing to the used areas is
// not adopted). An unused area on an unexpected host therefore stops startup:
// refusing an unexpected but legitimate host is the safe way to fail, and the
// live smoke run surfaces any such case.
//
// An entry with no id or no location names no host and is skipped; areaURL
// refuses a used area in that state. Each location is normalized as the SDK
// normalizes it before the check, so the URL checked is the URL a sub-client
// would be built on.
func checkAreaHosts(areas []azuredevops.ResourceAreaInfo, tr *transport.Transport) error {
	for _, area := range areas {
		if area.Id == nil || area.LocationUrl == nil || *area.LocationUrl == "" {
			continue
		}
		loc := strings.ToLower(strings.TrimRight(*area.LocationUrl, "/"))
		u, err := url.Parse(loc)
		if err == nil && tr.Allowed(u) {
			continue
		}
		name := area.Id.String()
		if area.Name != nil && *area.Name != "" {
			name = *area.Name
		}
		if err != nil {
			return &config.ConfigError{Field: "org_url", Message: fmt.Sprintf(
				"resource area %q reports an unparsable location %q; the PAT will not be sent there", name, loc)}
		}
		return &config.ConfigError{Field: "org_url", Message: fmt.Sprintf(
			"resource area %q is located at %q on host %q, outside the organization org_url names; the PAT will not be sent there",
			name, loc, u.Hostname())}
	}
	return nil
}

// FetchTimeout reports the configured budget for one whole multi-project
// fan-out. Views read it here rather than having it threaded through six view
// constructors and app.go for a value that already travels with the client.
func (c *Client) FetchTimeout() time.Duration {
	return c.fetchTimeout
}

// FetchRetries reports how many rate-limited reads this client has re-issued
// so far (D-03), counted as retries attempted (see retryRead). The CLI reads
// it once after dispatch for the D-05/D-06 stderr summary; the TUI never calls
// it (D-07). The Fetch prefix keeps TestNoUIFileCallsADOMutation classifying
// it as a read.
func (c *Client) FetchRetries() int64 {
	return c.readRetries.Load()
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

// GetRecentIterations returns iteration paths for sprints active in the last 30 days
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

		// Check if this iteration has dates within last 30 days
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

			// Include iteration if it overlaps with last 30 days
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
