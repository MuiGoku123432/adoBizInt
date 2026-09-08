package ado

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"

	"sentinovo.ai/bizInt/internal/logging"
)

// PipelineRun represents a single pipeline/build run for display
//
// Duration marshals as the encoding/json default for time.Duration: an integer
// nanosecond count, so a one-hour run emits 3600000000000. The key carries the
// _ns suffix because the unit is not otherwise discoverable from the JSON. This
// is a locked contract decision (D-04, Task 1 of plan 02-04) — Phase 3's --json
// makes the value type a one-way door, and a later switch to a string or to
// seconds would fail silently for any consumer doing arithmetic on it.
//
// The time.Time fields are values, not pointers: a zero timestamp therefore
// marshals as the year-one RFC 3339 sentinel rather than being omitted. That is
// an accepted, recorded cost, not an oversight — ,omitempty cannot suppress a
// struct, so omission would require pointers or a custom marshaller.
type PipelineRun struct {
	ID            int           `json:"id"`
	BuildNumber   string        `json:"build_number"`
	PipelineName  string        `json:"pipeline_name"`
	PipelineID    int           `json:"pipeline_id"`
	Project       string        `json:"project"`
	Status        string        `json:"status"` // "inProgress", "completed", "cancelling", "postponed", "notStarted", "none"
	Result        string        `json:"result"` // "succeeded", "partiallySucceeded", "failed", "canceled", "none"
	SourceBranch  string        `json:"source_branch"`
	SourceVersion string        `json:"source_version"`
	RequestedBy   string        `json:"requested_by"`
	RequestedFor  string        `json:"requested_for"`
	StartTime     time.Time     `json:"start_time"`
	FinishTime    time.Time     `json:"finish_time"`
	QueueTime     time.Time     `json:"queue_time"`
	Duration      time.Duration `json:"duration_ns"`
	RepositoryID  string        `json:"repository_id"`
}

// PipelineDefinition represents a pipeline definition for triggering new runs
type PipelineDefinition struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Project       string `json:"project"`
	RepositoryID  string `json:"repository_id"`
	DefaultBranch string `json:"default_branch"`
	QueueStatus   string `json:"queue_status"` // "enabled", "paused", "disabled"
}

// normalizePipelineName normalizes a pipeline name for comparison
// Trims whitespace and converts to lowercase
func normalizePipelineName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

// pipelineNameMatches checks if a pipeline name matches any configured patterns
// Supports:
//   - Exact match (normalized): "MyPipeline" matches "mypipeline", " MyPipeline "
//   - Prefix match with *: "MyPipeline*" matches "MyPipeline-Dev", "MyPipeline-Prod"
//   - Contains match with **: "**Pipeline**" matches anything containing "pipeline"
func pipelineNameMatches(name string, configuredPipelines []string) bool {
	normalizedName := normalizePipelineName(name)

	for _, configured := range configuredPipelines {
		pattern := normalizePipelineName(configured)

		// Contains match: **term** matches anything containing "term"
		if strings.HasPrefix(pattern, "**") && strings.HasSuffix(pattern, "**") && len(pattern) > 4 {
			substr := pattern[2 : len(pattern)-2]
			if strings.Contains(normalizedName, substr) {
				return true
			}
			continue
		}

		// Prefix match: "term*" matches anything starting with "term"
		if strings.HasSuffix(pattern, "*") && !strings.HasSuffix(pattern, "**") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(normalizedName, prefix) {
				return true
			}
			continue
		}

		// Exact match (normalized)
		if normalizedName == pattern {
			return true
		}
	}
	return false
}

// opGetPipelineRuns and opGetPipelineDefinitions label the SDK calls in any
// Failure these reads record.
const (
	opGetPipelineRuns        = "get pipeline definitions for runs"
	opGetPipelineDefinitions = "get pipeline definitions"
)

// GetPipelineRuns fetches ALL matching pipeline definitions and their latest runs.
// Returns one entry per pipeline definition, showing latest run status if
// available. Per-project failures are reported in the returned envelope.
func (c *Client) GetPipelineRuns(ctx context.Context, projects []string) (Result[PipelineRun], error) {
	log := logging.Logger()

	// If no pipelines configured, return empty
	if len(c.pipelines) == 0 {
		log.Info("No pipelines configured, skipping pipeline runs fetch")
		return newResult([]PipelineRun{}, nil), nil
	}

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allRuns []PipelineRun
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12). These N+1 fan-outs are the reads most
		// likely to exhaust the fan-out budget mid-sweep.
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetPipelineRuns, err)
			continue
		}

		// Step 1: Fetch ALL pipeline definitions for this project
		definitions, err := c.buildClient.GetDefinitions(ctx, build.GetDefinitionsArgs{
			Project: &project,
		})
		if err != nil {
			log.Error("Failed to get pipeline definitions", "project", project, "error", err)
			// The loop still visits every configured project: D-08 rejected
			// short-circuiting on auth failure explicitly.
			failures = appendFailure(failures, project, opGetPipelineRuns, err)
			continue
		}

		if definitions == nil || definitions.Value == nil {
			continue
		}

		// Filter definitions to only those matching configured patterns
		var matchingDefs []build.BuildDefinitionReference
		var allDefNames []string
		for _, d := range definitions.Value {
			if d.Name == nil {
				continue
			}
			allDefNames = append(allDefNames, *d.Name)
			if pipelineNameMatches(*d.Name, c.pipelines) {
				matchingDefs = append(matchingDefs, d)
			}
		}

		log.Debug("ALL pipeline definitions in ADO", "project", project, "count", len(allDefNames), "definitions", allDefNames)
		log.Info("Pipeline count", "project", project, "configured_pipelines", c.pipelines, "matching", len(matchingDefs), "total", len(definitions.Value))

		// Step 2: For each matching definition, fetch its latest build
		for _, def := range matchingDefs {
			defID := *def.Id
			defName := *def.Name

			// Create base run entry for this pipeline
			run := PipelineRun{
				Project:      project,
				PipelineName: defName,
				PipelineID:   defID,
				Status:       "neverRun", // Default status if no builds found
			}

			// Get repository ID from definition if available
			if def.Project != nil && def.Project.Id != nil {
				// We'll get repo info from the build if available
			}

			// Fetch the latest build for this specific definition
			top := 1
			queryOrder := build.BuildQueryOrderValues.StartTimeDescending
			defIDs := []int{defID}

			builds, err := c.buildClient.GetBuilds(ctx, build.GetBuildsArgs{
				Project:     &project,
				Definitions: &defIDs,
				Top:         &top,
				QueryOrder:  &queryOrder,
			})
			if err != nil {
				log.Debug("Failed to get builds for definition", "project", project, "definition", defName, "error", err)
				// Still add the definition with neverRun status
				allRuns = append(allRuns, run)
				continue
			}

			// If we found a build, populate the run details
			if builds != nil && builds.Value != nil && len(builds.Value) > 0 {
				b := builds.Value[0]

				if b.Id != nil {
					run.ID = *b.Id
				}
				if b.BuildNumber != nil {
					run.BuildNumber = *b.BuildNumber
				}
				if b.Status != nil {
					run.Status = string(*b.Status)
				}
				if b.Result != nil {
					run.Result = string(*b.Result)
				}
				if b.SourceBranch != nil {
					run.SourceBranch = strings.TrimPrefix(*b.SourceBranch, "refs/heads/")
				}
				if b.SourceVersion != nil {
					run.SourceVersion = *b.SourceVersion
				}
				if b.RequestedBy != nil && b.RequestedBy.DisplayName != nil {
					run.RequestedBy = *b.RequestedBy.DisplayName
				}
				if b.RequestedFor != nil && b.RequestedFor.DisplayName != nil {
					run.RequestedFor = *b.RequestedFor.DisplayName
				}
				if b.StartTime != nil {
					run.StartTime = b.StartTime.Time
				}
				if b.FinishTime != nil {
					run.FinishTime = b.FinishTime.Time
				}
				if b.QueueTime != nil {
					run.QueueTime = b.QueueTime.Time
				}
				if b.Repository != nil && b.Repository.Id != nil {
					run.RepositoryID = *b.Repository.Id
				}

				// Calculate duration
				if !run.StartTime.IsZero() && !run.FinishTime.IsZero() {
					run.Duration = run.FinishTime.Sub(run.StartTime)
				} else if !run.StartTime.IsZero() && run.Status == "inProgress" {
					run.Duration = time.Since(run.StartTime)
				}
			}

			allRuns = append(allRuns, run)
		}
	}

	log.Info("Pipeline runs fetched", "total_count", len(allRuns), "configured_pipelines", c.pipelines, "failure_count", len(failures))
	return newResult(allRuns, failures), nil
}

// GetPipelineDefinitions fetches pipeline definitions for configured pipelines.
// Per-project failures are reported in the returned envelope.
func (c *Client) GetPipelineDefinitions(ctx context.Context, projects []string) (Result[PipelineDefinition], error) {
	log := logging.Logger()

	// If no pipelines configured, return empty
	if len(c.pipelines) == 0 {
		log.Info("No pipelines configured, skipping definitions fetch")
		return newResult([]PipelineDefinition{}, nil), nil
	}

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allDefs []PipelineDefinition
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetPipelineDefinitions, err)
			continue
		}

		definitions, err := c.buildClient.GetDefinitions(ctx, build.GetDefinitionsArgs{
			Project: &project,
		})
		if err != nil {
			log.Error("Failed to get pipeline definitions", "project", project, "error", err)
			failures = appendFailure(failures, project, opGetPipelineDefinitions, err)
			continue
		}

		if definitions == nil || definitions.Value == nil {
			continue
		}

		// Log ALL available pipeline definitions (not just matched ones)
		var allDefNames []string
		for _, d := range definitions.Value {
			if d.Name != nil {
				allDefNames = append(allDefNames, *d.Name)
			}
		}
		log.Debug("ALL pipeline definitions in ADO", "project", project, "count", len(allDefNames), "definitions", allDefNames)

		for _, d := range definitions.Value {
			if d.Name == nil {
				continue
			}

			// Skip if not matching any configured pipeline pattern
			if !pipelineNameMatches(*d.Name, c.pipelines) {
				continue
			}

			def := PipelineDefinition{
				Project: project,
				Name:    *d.Name,
			}

			if d.Id != nil {
				def.ID = *d.Id
			}
			if d.Path != nil {
				def.Path = *d.Path
			}
			if d.QueueStatus != nil {
				def.QueueStatus = string(*d.QueueStatus)
			}

			// Get repository info and default branch from the full definition
			fullDef, err := c.buildClient.GetDefinition(ctx, build.GetDefinitionArgs{
				Project:      &project,
				DefinitionId: d.Id,
			})
			if err == nil && fullDef != nil {
				if fullDef.Repository != nil {
					if fullDef.Repository.Id != nil {
						def.RepositoryID = *fullDef.Repository.Id
					}
					if fullDef.Repository.DefaultBranch != nil {
						def.DefaultBranch = strings.TrimPrefix(*fullDef.Repository.DefaultBranch, "refs/heads/")
					}
				}
			}

			allDefs = append(allDefs, def)
		}
	}

	log.Info("Pipeline definitions fetched", "total_count", len(allDefs), "failure_count", len(failures))
	return newResult(allDefs, failures), nil
}

// GetBranchesForPipeline fetches available branches for a pipeline's repository
func (c *Client) GetBranchesForPipeline(ctx context.Context, project, repositoryID string) ([]string, error) {
	log := logging.Logger()

	refs, err := c.gitClient.GetRefs(ctx, git.GetRefsArgs{
		RepositoryId: &repositoryID,
		Project:      &project,
		Filter:       strPtr("heads/"),
	})
	if err != nil {
		log.Error("Failed to get branches", "project", project, "repositoryID", repositoryID, "error", err)
		return nil, err
	}

	if refs == nil || len(refs.Value) == 0 {
		return []string{}, nil
	}

	var branches []string
	for _, ref := range refs.Value {
		if ref.Name != nil {
			branch := strings.TrimPrefix(*ref.Name, "refs/heads/")
			branches = append(branches, branch)
		}
	}

	log.Info("Branches fetched", "project", project, "repositoryID", repositoryID, "count", len(branches))
	return branches, nil
}

// QueuePipeline triggers a new pipeline run on the specified branch
func (c *Client) QueuePipeline(ctx context.Context, project string, definitionID int, branch string) (*PipelineRun, error) {
	log := logging.Logger()

	sourceBranch := branch
	if !strings.HasPrefix(branch, "refs/heads/") {
		sourceBranch = "refs/heads/" + branch
	}

	buildToQueue := build.Build{
		Definition: &build.DefinitionReference{
			Id: &definitionID,
		},
		SourceBranch: &sourceBranch,
	}

	queuedBuild, err := c.buildClient.QueueBuild(ctx, build.QueueBuildArgs{
		Build:   &buildToQueue,
		Project: &project,
	})
	if err != nil {
		log.Error("Failed to queue pipeline", "project", project, "definitionID", definitionID, "branch", branch, "error", err)
		return nil, err
	}

	if queuedBuild == nil {
		return nil, fmt.Errorf("queued build returned nil")
	}

	run := &PipelineRun{
		Project: project,
	}

	if queuedBuild.Id != nil {
		run.ID = *queuedBuild.Id
	}
	if queuedBuild.BuildNumber != nil {
		run.BuildNumber = *queuedBuild.BuildNumber
	}
	if queuedBuild.Definition != nil && queuedBuild.Definition.Name != nil {
		run.PipelineName = *queuedBuild.Definition.Name
	}
	if queuedBuild.Status != nil {
		run.Status = string(*queuedBuild.Status)
	}
	run.SourceBranch = branch

	log.Info("Pipeline queued", "project", project, "definitionID", definitionID, "branch", branch, "buildID", run.ID)
	return run, nil
}
