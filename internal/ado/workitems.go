package ado

import (
	"context"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"

	"sentinovo.ai/bizInt/internal/logging"
)

// WorkItem represents a work item for display
type WorkItem struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
	StoryPoints     int    `json:"story_points"`
	Project         string `json:"project"`
	Type            string `json:"type"`
	State           string `json:"state"`
	AssignedTo      string `json:"assigned_to"`
	AssignedToEmail string `json:"assigned_to_email"`
}

// WorkItemDetail represents a work item with full details
//
// The embedded WorkItem is deliberately left untagged so encoding/json inlines
// its fields into the same JSON object rather than nesting them under a
// "WorkItem" key. A consumer therefore reads a detail record as one flat object
// whose key set is a superset of the list record's. This is a decision, not a
// default — pinned by TestJSONFieldNames/WorkItemDetail_embeds_flat.
type WorkItemDetail struct {
	WorkItem
	Description        string       `json:"description"`
	AcceptanceCriteria string       `json:"acceptance_criteria"`
	ReproSteps         string       `json:"repro_steps"` // For bugs
	Attachments        []Attachment `json:"attachments"`
	CreatedDate        time.Time    `json:"created_date"`
	ModifiedDate       time.Time    `json:"modified_date"`
	CreatedBy          string       `json:"created_by"`
	ModifiedBy         string       `json:"modified_by"`
	IterationPath      string       `json:"iteration_path"`
	AreaPath           string       `json:"area_path"`

	// The three scheduling effort fields are pointers because "ADO does not
	// hold this field" and "ADO holds zero" are different facts, and a write
	// path diffing current against proposed must be able to tell them apart
	// (D-17). A float64 zero value would report an unset field as 0 and make a
	// genuine `--remaining 0` look like a no-op.
	OriginalEstimate *float64 `json:"original_estimate"`
	RemainingWork    *float64 `json:"remaining_work"`
	CompletedWork    *float64 `json:"completed_work"`
}

// Attachment represents a work item attachment
//
// ID is the attachment GUID parsed from the relation URL, and it is the only
// identifier DownloadAttachment accepts (D-01). URL is kept for a human to
// paste into a browser; no code path fetches it (D-05).
//
// URL is a single lower-case word, not a letter-split "u_r_l": initialisms are
// mapped deliberately rather than by mechanical camel-to-snake conversion.
type Attachment struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	URL  string    `json:"url"`
	Size int64     `json:"size"`
}

// opGetWorkItems and opGetWorkItemsBatch label the SDK calls in any Failure the
// work-item read records. The batch label is distinct because a project can
// answer its query and still fail its detail fetch (D-02): reporting only the
// outer failure would mark a partially-answering project complete. The
// iteration label exists because a failed lookup makes the WIQL query run
// without its sprint clause, the same degradation the dashboard records as
// opDashboardIterations (T-02-05).
const (
	opGetWorkItems           = "get work items"
	opGetWorkItemsBatch      = "get work item batch"
	opGetWorkItemsIterations = "get work item iterations"
)

// GetWorkItems fetches work items for the specified projects. Per-project query
// failures and per-batch detail failures are both reported in the returned
// envelope rather than swallowed.
func (c *Client) GetWorkItems(ctx context.Context, projects []string) (Result[WorkItem], error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	log.Debug("GetWorkItems called", "projects", targetProjects)

	if len(targetProjects) == 0 {
		log.Warn("No projects to fetch work items for")
		return newResult([]WorkItem{}, nil), nil
	}

	var allItems []WorkItem
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetWorkItems, err)
			continue
		}

		items, projectFailures, err := c.getProjectWorkItems(ctx, project)
		// Sub-request failures (iteration lookup, detail batches) are attributed
		// even when the query itself answered, so a project that answered
		// partially is never reported as complete. A project whose iteration
		// lookup and QueryByWiql both fail intentionally yields two entries, the
		// opGetWorkItemsIterations one followed by the opGetWorkItems one,
		// because they are two distinct failed sub-requests (D-02).
		failures = append(failures, projectFailures...)
		if err != nil {
			log.Error("Failed to get work items for project", "project", project, "error", err)
			// The loop still visits every configured project: D-08 rejected
			// short-circuiting on auth failure explicitly.
			failures = appendFailure(failures, project, opGetWorkItems, err)
			continue
		}
		allItems = append(allItems, items...)
	}

	log.Info("Work items fetched", "total_count", len(allItems), "failure_count", len(failures))
	return newResult(allItems, failures), nil
}

// getProjectWorkItems returns the work items it collected alongside every
// sub-request failure (the iteration lookup and the detail batches), so the
// caller can attribute a partial or degraded answer to its project.
func (c *Client) getProjectWorkItems(ctx context.Context, project string) ([]WorkItem, []Failure, error) {
	log := logging.Logger()
	var failures []Failure

	// Get recent iterations (sprints active in last 90 days)
	iterations, err := c.GetRecentIterations(ctx, project)
	if err != nil {
		log.Warn("Failed to get iterations, using date filter only", "project", project, "error", err)
		// This is a degradation, not a neutral fallback: the query below loses
		// its iteration clause, so the list is taken under a substituted filter
		// and must not be presented with the same authority as a correct one
		// (T-02-05, D-02). The items are kept, and the caller sees Complete=false.
		failures = appendFailure(failures, project, opGetWorkItemsIterations, err)
	} else if len(iterations) > 0 {
		log.Info("Using iteration filter", "project", project, "sprints", iterations)
	}

	// Query for work items with relevant fields. The project never enters the
	// query text: @project resolves server-side from args.Project, which the
	// SDK sends in the request route (D-10 revised).
	wiql := "SELECT [System.Id], [System.Title], [System.Description], [Microsoft.VSTS.Scheduling.StoryPoints], [System.WorkItemType], [System.State], [System.AssignedTo] FROM WorkItems WHERE [System.TeamProject] = @project AND [System.State] NOT IN ('Done', 'Closed', 'Removed')" + iterationClause(iterations) + " ORDER BY [System.Id] DESC"
	query := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &wiql},
		Project: &project,
	}

	// The query text is not logged (D-11).
	log.Info("Executing WIQL query", "project", project, "iterations", len(iterations))

	result, err := c.workitemClient.QueryByWiql(ctx, query)
	if err != nil {
		return nil, failures, err
	}

	if result == nil || result.WorkItems == nil || len(*result.WorkItems) == 0 {
		log.Info("Query returned no work items", "project", project)
		return []WorkItem{}, failures, nil
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
		return []WorkItem{}, failures, nil
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
			// Still collect as many batches as possible, but report the gap so
			// the caller cannot present this project as complete (D-02).
			failures = appendFailure(failures, project, opGetWorkItemsBatch, err)
			continue
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
	return workItems, failures, nil
}

// UpdateWorkItemState updates the state of a work item.
//
// project is sent in the request route, the same way the other write methods
// send theirs. An empty project leaves the route unscoped, which is what the
// TUI passes: it has no --project to honour.
func (c *Client) UpdateWorkItemState(ctx context.Context, project string, itemID int, newState string) error {
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
		Project:  &project,
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
//
// Each effort parameter is nullable, and nil means "the caller did not name this
// field" — no operation is appended for it and ADO leaves it untouched. A nil
// pointer must never reach the Value field: Value is interface{}, so a nil
// *float64 placed there serializes as "value":null, which CLEARS the field
// rather than omitting it. The nil check therefore gates the append, never the
// value. A non-nil pointer to 0 is a real zero and must survive — closing a task
// legitimately means RemainingWork 0.
//
// project is routed exactly as in UpdateWorkItemState.
func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, project string, itemID int, newState string, originalEstimate, remaining, completed *float64) error {
	log := logging.Logger()

	addOp := webapi.OperationValues.Add
	statePath := "/fields/System.State"
	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
	remainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	completedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"

	// Update effort fields first, then state change. The relative order —
	// OriginalEstimate, RemainingWork, CompletedWork, then State — is a pinned
	// contract; surviving operations are not reordered when some are omitted.
	// Using Add operation which works for both new and existing values.
	document := []webapi.JsonPatchOperation{}
	if originalEstimate != nil {
		document = append(document, webapi.JsonPatchOperation{
			Op:    &addOp,
			Path:  &originalEstimatePath,
			Value: *originalEstimate,
		})
	}
	if remaining != nil {
		document = append(document, webapi.JsonPatchOperation{
			Op:    &addOp,
			Path:  &remainingPath,
			Value: *remaining,
		})
	}
	if completed != nil {
		document = append(document, webapi.JsonPatchOperation{
			Op:    &addOp,
			Path:  &completedPath,
			Value: *completed,
		})
	}
	document = append(document, webapi.JsonPatchOperation{
		Op:    &addOp,
		Path:  &statePath,
		Value: newState,
	})

	_, err := c.workitemClient.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
		Project:  &project,
		Id:       &itemID,
		Document: &document,
	})

	if err != nil {
		log.Error("Failed to update work item state with effort", "id", itemID, "state", newState, "error", err)
		return err
	}

	log.Info("Work item state updated with effort", "id", itemID, "state", newState,
		"originalEstimate", effortLogValue(originalEstimate),
		"remaining", effortLogValue(remaining),
		"completed", effortLogValue(completed))
	return nil
}

// effortLogValue renders a nullable effort value for structured logging. Logging
// the pointer itself would print an address, which hides both the value and the
// omitted/supplied distinction this function exists to preserve.
func effortLogValue(v *float64) any {
	if v == nil {
		return "omitted"
	}
	return *v
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
	// The owning project. Work item ids are unique across the organization, so
	// the id does not say which project an item belongs to; the CLI write path
	// compares this against --project before it mutates anything (CR-01).
	if project, ok := itemFields["System.TeamProject"].(string); ok {
		detail.Project = project
	}
	if state, ok := itemFields["System.State"].(string); ok {
		detail.State = state
	}
	if points, ok := itemFields["Microsoft.VSTS.Scheduling.StoryPoints"].(float64); ok {
		detail.StoryPoints = int(points)
	}
	// Effort fields, extracted the same way StoryPoints is. Expand: Relations
	// already returns the full field map, so this is extraction only — no extra
	// round trip. The address is taken of a fresh local per field so the three
	// pointers never alias.
	if v, ok := itemFields["Microsoft.VSTS.Scheduling.OriginalEstimate"].(float64); ok {
		originalEstimate := v
		detail.OriginalEstimate = &originalEstimate
	}
	if v, ok := itemFields["Microsoft.VSTS.Scheduling.RemainingWork"].(float64); ok {
		remainingWork := v
		detail.RemainingWork = &remainingWork
	}
	if v, ok := itemFields["Microsoft.VSTS.Scheduling.CompletedWork"].(float64); ok {
		completedWork := v
		detail.CompletedWork = &completedWork
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
					// The GUID is the last path segment; attributes.id is an
					// integer revision, not the attachment ID.
					if u, err := url.Parse(*rel.Url); err == nil {
						if id, err := uuid.Parse(path.Base(u.Path)); err == nil {
							attachment.ID = id
						}
					}
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
				// D-06: an attachment without a usable GUID cannot be
				// downloaded, so it is not offered.
				if attachment.Name != "" && attachment.ID != uuid.Nil {
					detail.Attachments = append(detail.Attachments, attachment)
				}
			}
		}
	}

	log.Debug("Got work item detail", "id", id, "title", detail.Title, "attachments", len(detail.Attachments))
	return detail, nil
}

// GetChildWorkItems fetches all child work items for a given parent work item ID
func (c *Client) GetChildWorkItems(ctx context.Context, parentID int) ([]WorkItem, error) {
	log := logging.Logger()

	// Fetch the parent work item with relations expanded
	expand := workitemtracking.WorkItemExpandValues.Relations
	item, err := c.workitemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:     &parentID,
		Expand: &expand,
	})
	if err != nil {
		log.Error("Failed to get work item for children", "id", parentID, "error", err)
		return nil, err
	}

	if item == nil || item.Relations == nil {
		log.Debug("No relations found for work item", "id", parentID)
		return []WorkItem{}, nil
	}

	// Extract child IDs from relations
	// "System.LinkTypes.Hierarchy-Forward" means parent -> child relationship
	var childIDs []int
	for _, rel := range *item.Relations {
		if rel.Rel != nil && *rel.Rel == "System.LinkTypes.Hierarchy-Forward" {
			// Extract ID from URL (format: https://dev.azure.com/{org}/{project}/_apis/wit/workItems/{id})
			if rel.Url != nil {
				url := *rel.Url
				// Find last segment after /workItems/
				if lastSlash := strings.LastIndex(url, "/"); lastSlash >= 0 {
					idStr := url[lastSlash+1:]
					if id, err := strconv.Atoi(idStr); err == nil {
						childIDs = append(childIDs, id)
					}
				}
			}
		}
	}

	if len(childIDs) == 0 {
		log.Debug("No child work items found", "parentID", parentID)
		return []WorkItem{}, nil
	}

	log.Debug("Found child work item IDs", "parentID", parentID, "childIDs", childIDs)

	// Batch fetch child work items
	fields := []string{
		"System.Id",
		"System.Title",
		"System.WorkItemType",
		"System.State",
		"System.AssignedTo",
	}
	children, err := c.workitemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:    &childIDs,
		Fields: &fields,
	})
	if err != nil {
		log.Error("Failed to fetch child work items", "parentID", parentID, "error", err)
		return nil, err
	}

	var result []WorkItem
	if children != nil {
		for _, child := range *children {
			wi := WorkItem{}
			if child.Id != nil {
				wi.ID = *child.Id
			}
			if child.Fields != nil {
				fields := *child.Fields
				if title, ok := fields["System.Title"].(string); ok {
					wi.Title = title
				}
				if wiType, ok := fields["System.WorkItemType"].(string); ok {
					wi.Type = wiType
				}
				if state, ok := fields["System.State"].(string); ok {
					wi.State = state
				}
				// Extract assignee
				if assignedTo, ok := fields["System.AssignedTo"]; ok && assignedTo != nil {
					if v, ok := assignedTo.(map[string]interface{}); ok {
						if displayName, ok := v["displayName"].(string); ok {
							wi.AssignedTo = displayName
						}
					}
				}
			}
			result = append(result, wi)
		}
	}

	log.Info("Fetched child work items", "parentID", parentID, "count", len(result))
	return result, nil
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
