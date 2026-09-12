package ado

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/release"

	"sentinovo.ai/bizInt/internal/logging"
)

// This file holds the by-id pre-reads the guarded-write path needs (D-01). It
// is a NEW file rather than three more functions on workitems.go, which is
// already 643 lines against CONVENTIONS.md's 300-line standard.
//
// Every method here deviates from GetWorkItemDetail (workitems.go:369) in one
// deliberate way: GetWorkItemDetail returns (nil, nil) when the item or its
// fields are nil, and that shape is WRONG for a write pre-read. D-02 makes a
// failed pre-read fatal precisely so a dry run can never report a diff it did
// not establish; a (nil, nil) would instead let the write path compute an empty
// diff and refuse a "no-op" for a target that does not exist (threat T-04-13).
// A missing target is an error here, always.
//
// Error messages name only the kind and the id. No URL, no headers, no raw
// response body: these strings reach stderr and, by way of the outcome, the
// persisted audit record (D-15, threat T-04-14).

// PullRequestDetail is the by-id view of a pull request the prs write verbs
// diff against. MyVote is the acting identity's current reviewer vote in Azure
// DevOps' own vocabulary (10 approved, 5 approved with suggestions, 0 no vote,
// -5 waiting for author, -10 rejected); zero means "no vote recorded", which is
// the same value ADO reports, not a sentinel this package invented.
type PullRequestDetail struct {
	ID          int
	Title       string
	Status      string
	MergeStatus string
	MyVote      int
}

// ReleaseApprovalDetail is the by-id view of a release approval.
type ReleaseApprovalDetail struct {
	ID      int
	Status  string
	Release string
}

// PipelineDefinitionDetail is the by-id view of a build definition.
type PipelineDefinitionDetail struct {
	ID            int
	Name          string
	DefaultBranch string
}

// GetPullRequestDetail reads one pull request by id for the write path.
//
// It returns an error rather than a nil detail when the PR is absent, so a
// caller can never read a missing PR as an empty diff.
func (c *Client) GetPullRequestDetail(ctx context.Context, project, repositoryID string, prID int) (*PullRequestDetail, error) {
	log := logging.Logger()

	pr, err := c.gitClient.GetPullRequest(ctx, git.GetPullRequestArgs{
		RepositoryId:  &repositoryID,
		PullRequestId: &prID,
		Project:       &project,
	})
	if err != nil {
		log.Error("Failed to read pull request before writing", "prID", prID, "error", err)
		return nil, err
	}
	if pr == nil {
		log.Error("Pull request not found before writing", "prID", prID)
		return nil, fmt.Errorf("pull request %d was not found", prID)
	}

	detail := &PullRequestDetail{ID: prID}
	if pr.PullRequestId != nil {
		detail.ID = *pr.PullRequestId
	}
	if pr.Title != nil {
		detail.Title = *pr.Title
	}
	if pr.Status != nil {
		detail.Status = string(*pr.Status)
	}
	if pr.MergeStatus != nil {
		detail.MergeStatus = string(*pr.MergeStatus)
	}

	// The acting identity's own vote is what `prs approve` diffs against, so it
	// is matched on the cached user id rather than on a display name: Phase 1
	// D-12 makes identity resolution non-fatal, so currentUserID can legitimately
	// be empty, and an empty id must match nobody rather than the first reviewer.
	if pr.Reviewers != nil && c.currentUserID != "" {
		for _, r := range *pr.Reviewers {
			if r.Id != nil && strings.EqualFold(*r.Id, c.currentUserID) && r.Vote != nil {
				detail.MyVote = *r.Vote
				break
			}
		}
	}

	log.Info("Pull request pre-read", "prID", detail.ID, "status", detail.Status, "vote", detail.MyVote)
	return detail, nil
}

// GetReleaseApprovalDetail reads one release approval by id for the write path.
//
// The SDK exposes no by-id approval endpoint, only the project-scoped list
// (release/client.go:274), so no entry is added to releaseAPI for this: the
// existing GetApprovals is called and the result filtered by id. The status
// filter is widened past the endpoint's `pending` default because an approval
// already approved or rejected is exactly the case D-03 refuses as a no-op, and
// it would be invisible in the default list.
func (c *Client) GetReleaseApprovalDetail(ctx context.Context, project string, approvalID int) (*ReleaseApprovalDetail, error) {
	log := logging.Logger()

	statusFilter := release.ApprovalStatus("pending,approved,rejected")

	resp, err := c.releaseClient.GetApprovals(ctx, release.GetApprovalsArgs{
		Project:      &project,
		StatusFilter: &statusFilter,
	})
	if err != nil {
		log.Error("Failed to read release approval before writing", "approvalID", approvalID, "error", err)
		return nil, err
	}
	if resp == nil {
		log.Error("Release approval not found before writing", "approvalID", approvalID)
		return nil, fmt.Errorf("release approval %d was not found", approvalID)
	}

	for _, a := range resp.Value {
		if a.Id == nil || *a.Id != approvalID {
			continue
		}
		detail := &ReleaseApprovalDetail{ID: approvalID}
		if a.Status != nil {
			detail.Status = string(*a.Status)
		}
		if a.Release != nil && a.Release.Name != nil {
			detail.Release = *a.Release.Name
		}
		log.Info("Release approval pre-read", "approvalID", detail.ID, "status", detail.Status)
		return detail, nil
	}

	log.Error("Release approval not found before writing", "approvalID", approvalID)
	return nil, fmt.Errorf("release approval %d was not found", approvalID)
}

// GetPipelineDefinitionDetail reads one build definition by id for the write
// path. `pipelines queue` does not diff against the definition's state — a queue
// is not an idempotent state change — but D-01 still requires the target to be
// established before anything is issued, and D-02 makes that establishment
// fatal when it fails.
func (c *Client) GetPipelineDefinitionDetail(ctx context.Context, project string, definitionID int) (*PipelineDefinitionDetail, error) {
	log := logging.Logger()

	def, err := c.buildClient.GetDefinition(ctx, build.GetDefinitionArgs{
		Project:      &project,
		DefinitionId: &definitionID,
	})
	if err != nil {
		log.Error("Failed to read pipeline definition before writing", "definitionID", definitionID, "error", err)
		return nil, err
	}
	if def == nil {
		log.Error("Pipeline definition not found before writing", "definitionID", definitionID)
		return nil, fmt.Errorf("pipeline definition %d was not found", definitionID)
	}

	detail := &PipelineDefinitionDetail{ID: definitionID}
	if def.Id != nil {
		detail.ID = *def.Id
	}
	if def.Name != nil {
		detail.Name = *def.Name
	}
	if def.Repository != nil && def.Repository.DefaultBranch != nil {
		detail.DefaultBranch = strings.TrimPrefix(*def.Repository.DefaultBranch, "refs/heads/")
	}

	log.Info("Pipeline definition pre-read", "definitionID", detail.ID, "name", detail.Name)
	return detail, nil
}
