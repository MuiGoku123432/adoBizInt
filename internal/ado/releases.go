package ado

import (
	"context"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/release"

	"sentinovo.ai/bizInt/internal/logging"
)

// Release represents a classic release for display
type Release struct {
	ID               int                  `json:"id"`
	Name             string               `json:"name"`
	DefinitionID     int                  `json:"definition_id"`
	DefinitionName   string               `json:"definition_name"`
	Project          string               `json:"project"`
	Status           string               `json:"status"` // "draft", "active", "abandoned"
	CreatedOn        time.Time            `json:"created_on"`
	ModifiedOn       time.Time            `json:"modified_on"`
	Environments     []ReleaseEnvironment `json:"environments"`
	PendingApprovals int                  `json:"pending_approvals"`
}

// ReleaseEnvironment represents an environment within a release
type ReleaseEnvironment struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`            // "notStarted", "inProgress", "succeeded", "canceled", "rejected", "queued", "partiallySucceeded"
	DeploymentStatus string `json:"deployment_status"` // "notDeployed", "inProgress", "succeeded", "partiallySucceeded", "failed"
	ReleaseID        int    `json:"release_id"`
	ReleaseName      string `json:"release_name"`
	DefinitionID     int    `json:"definition_id"`
	DefinitionName   string `json:"definition_name"`
	Project          string `json:"project"`
}

// ReleaseApproval represents a pending approval
type ReleaseApproval struct {
	ID              int       `json:"id"`
	ReleaseID       int       `json:"release_id"`
	ReleaseName     string    `json:"release_name"`
	EnvironmentID   int       `json:"environment_id"`
	EnvironmentName string    `json:"environment_name"`
	Project         string    `json:"project"`
	Status          string    `json:"status"`        // "pending", "approved", "rejected", "reassigned", "canceled", "skipped"
	ApprovalType    string    `json:"approval_type"` // "preDeploy", "postDeploy"
	Approver        string    `json:"approver"`
	ApproverID      string    `json:"approver_id"`
	CreatedOn       time.Time `json:"created_on"`
	Comments        string    `json:"comments"`
}

// opGetReleases and opGetPendingApprovals label the SDK calls in any Failure
// these reads record.
const (
	opGetReleases         = "get release definitions"
	opGetPendingApprovals = "get pending approvals"
)

// GetReleases fetches recent releases for configured pipelines. Per-project
// failures are reported in the returned envelope.
func (c *Client) GetReleases(ctx context.Context, projects []string) (Result[Release], error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allReleases []Release
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12). This N+1 fan-out is among the reads
		// most likely to exhaust the fan-out budget mid-sweep.
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetReleases, err)
			continue
		}

		// Step 1: Fetch ALL release definitions for this project
		releaseDefs, err := c.releaseClient.GetReleaseDefinitions(ctx, release.GetReleaseDefinitionsArgs{
			Project: &project,
		})
		if err != nil {
			log.Error("Failed to get release definitions", "project", project, "error", err)
			// The loop still visits every configured project: D-08 rejected
			// short-circuiting on auth failure explicitly.
			failures = appendFailure(failures, project, opGetReleases, err)
			continue
		}

		if releaseDefs == nil || len(releaseDefs.Value) == 0 {
			continue
		}

		// Filter definitions to only those matching configured patterns
		var matchingDefs []release.ReleaseDefinition
		var allDefNames []string
		for _, rd := range releaseDefs.Value {
			if rd.Name == nil || rd.Id == nil {
				continue
			}
			allDefNames = append(allDefNames, *rd.Name)

			// Filter to configured release definitions (if configured)
			if len(c.releaseDefinitions) > 0 {
				if pipelineNameMatches(*rd.Name, c.releaseDefinitions) {
					matchingDefs = append(matchingDefs, rd)
				}
			} else {
				// No filter configured, include all
				matchingDefs = append(matchingDefs, rd)
			}
		}

		log.Debug("ALL release definitions in ADO", "project", project, "count", len(allDefNames), "definitions", allDefNames)
		log.Info("Release definition count", "project", project, "configured_definitions", c.releaseDefinitions, "matching", len(matchingDefs), "total", len(releaseDefs.Value))

		// Step 2: For each matching definition, fetch its latest release
		for _, def := range matchingDefs {
			defID := *def.Id
			defName := *def.Name

			// Create base release entry for this definition
			rel := Release{
				Project:        project,
				DefinitionID:   defID,
				DefinitionName: defName,
				Status:         "noReleases", // Default status if no releases found
			}

			// Fetch the latest release for this specific definition
			top := 1
			queryOrder := release.ReleaseQueryOrderValues.Descending
			expand := release.ReleaseExpandsValues.Environments

			releases, err := c.releaseClient.GetReleases(ctx, release.GetReleasesArgs{
				Project:      &project,
				DefinitionId: &defID,
				Top:          &top,
				QueryOrder:   &queryOrder,
				Expand:       &expand,
			})
			if err != nil {
				log.Debug("Failed to get releases for definition", "project", project, "definition", defName, "error", err)
				// Still add the definition with noReleases status
				allReleases = append(allReleases, rel)
				continue
			}

			// If we found a release, populate the details
			if releases != nil && len(releases.Value) > 0 {
				r := releases.Value[0]

				if r.Id != nil {
					rel.ID = *r.Id
				}
				if r.Name != nil {
					rel.Name = *r.Name
				}
				if r.Status != nil {
					rel.Status = string(*r.Status)
				}
				if r.CreatedOn != nil {
					rel.CreatedOn = r.CreatedOn.Time
				}
				if r.ModifiedOn != nil {
					rel.ModifiedOn = r.ModifiedOn.Time
				}

				// Convert environments
				if r.Environments != nil {
					for _, env := range *r.Environments {
						relEnv := ReleaseEnvironment{
							Project:        project,
							ReleaseID:      rel.ID,
							ReleaseName:    rel.Name,
							DefinitionID:   rel.DefinitionID,
							DefinitionName: rel.DefinitionName,
						}

						if env.Id != nil {
							relEnv.ID = *env.Id
						}
						if env.Name != nil {
							relEnv.Name = *env.Name
						}
						if env.Status != nil {
							relEnv.Status = string(*env.Status)
						}

						// Extract DeploymentStatus from the latest deployment attempt
						if env.DeploySteps != nil && len(*env.DeploySteps) > 0 {
							attempts := *env.DeploySteps
							latestAttempt := attempts[len(attempts)-1]
							if latestAttempt.Status != nil {
								relEnv.DeploymentStatus = string(*latestAttempt.Status)
							}
						} else {
							relEnv.DeploymentStatus = "notDeployed"
						}

						rel.Environments = append(rel.Environments, relEnv)
					}
				}
			}

			allReleases = append(allReleases, rel)
		}
	}

	log.Info("Releases fetched", "total_count", len(allReleases), "configured_definitions", c.releaseDefinitions, "failure_count", len(failures))
	return newResult(allReleases, failures), nil
}

// GetPendingApprovals fetches pending approvals across projects. Per-project
// failures are reported in the returned envelope.
func (c *Client) GetPendingApprovals(ctx context.Context, projects []string) (Result[ReleaseApproval], error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allApprovals []ReleaseApproval
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetPendingApprovals, err)
			continue
		}

		statusFilter := release.ApprovalStatusValues.Pending

		approvals, err := c.releaseClient.GetApprovals(ctx, release.GetApprovalsArgs{
			Project:      &project,
			StatusFilter: &statusFilter,
		})
		if err != nil {
			log.Error("Failed to get approvals", "project", project, "error", err)
			failures = appendFailure(failures, project, opGetPendingApprovals, err)
			continue
		}

		if approvals == nil || len(approvals.Value) == 0 {
			continue
		}

		for _, a := range approvals.Value {
			approval := ReleaseApproval{
				Project: project,
			}

			if a.Id != nil {
				approval.ID = *a.Id
			}
			if a.Release != nil && a.Release.Id != nil {
				approval.ReleaseID = *a.Release.Id
			}
			if a.Release != nil && a.Release.Name != nil {
				approval.ReleaseName = *a.Release.Name
			}
			if a.ReleaseEnvironment != nil && a.ReleaseEnvironment.Id != nil {
				approval.EnvironmentID = *a.ReleaseEnvironment.Id
			}
			if a.ReleaseEnvironment != nil && a.ReleaseEnvironment.Name != nil {
				approval.EnvironmentName = *a.ReleaseEnvironment.Name
			}
			if a.Status != nil {
				approval.Status = string(*a.Status)
			}
			if a.ApprovalType != nil {
				approval.ApprovalType = string(*a.ApprovalType)
			}
			if a.Approver != nil && a.Approver.DisplayName != nil {
				approval.Approver = *a.Approver.DisplayName
			}
			if a.Approver != nil && a.Approver.Id != nil {
				approval.ApproverID = *a.Approver.Id
			}
			if a.CreatedOn != nil {
				approval.CreatedOn = a.CreatedOn.Time
			}

			allApprovals = append(allApprovals, approval)
		}
	}

	log.Info("Pending approvals fetched", "total_count", len(allApprovals), "failure_count", len(failures))
	return newResult(allApprovals, failures), nil
}

// ApproveRelease approves a pending approval with optional comment
func (c *Client) ApproveRelease(ctx context.Context, project string, approvalID int, comment string) error {
	log := logging.Logger()

	status := release.ApprovalStatusValues.Approved
	approval := &release.ReleaseApproval{
		Status:   &status,
		Comments: &comment,
	}

	_, err := c.releaseClient.UpdateReleaseApproval(ctx, release.UpdateReleaseApprovalArgs{
		Project:    &project,
		ApprovalId: &approvalID,
		Approval:   approval,
	})

	if err != nil {
		log.Error("Failed to approve release", "approvalID", approvalID, "error", err)
		return err
	}

	log.Info("Release approved", "approvalID", approvalID)
	return nil
}

// RejectRelease rejects a pending approval with optional comment
func (c *Client) RejectRelease(ctx context.Context, project string, approvalID int, comment string) error {
	log := logging.Logger()

	status := release.ApprovalStatusValues.Rejected
	approval := &release.ReleaseApproval{
		Status:   &status,
		Comments: &comment,
	}

	_, err := c.releaseClient.UpdateReleaseApproval(ctx, release.UpdateReleaseApprovalArgs{
		Project:    &project,
		ApprovalId: &approvalID,
		Approval:   approval,
	})

	if err != nil {
		log.Error("Failed to reject release", "approvalID", approvalID, "error", err)
		return err
	}

	log.Info("Release rejected", "approvalID", approvalID)
	return nil
}
