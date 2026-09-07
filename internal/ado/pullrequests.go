package ado

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/location"

	"sentinovo.ai/bizInt/internal/logging"
)

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

// opGetPullRequests labels the SDK call in any Failure this read records.
const opGetPullRequests = "get pull requests"

// GetPullRequests fetches all active pull requests across specified projects.
// Per-project failures are reported in the returned envelope rather than
// swallowed, so a caller can tell a rejected token from an empty backlog.
//
// The caller owns the deadline: this method deliberately creates no
// context.WithTimeout of its own, because a method-internal budget would be
// invisible to the CLI's --timeout in Phase 3.
func (c *Client) GetPullRequests(ctx context.Context, projects []string) (Result[PullRequest], error) {
	log := logging.Logger()

	targetProjects := projects
	if len(targetProjects) == 0 {
		targetProjects = c.projects
	}

	var allPRs []PullRequest
	var failures []Failure

	for _, project := range targetProjects {
		// Record the remaining projects as unreached rather than issuing calls
		// that are already doomed (D-12).
		if err := ctx.Err(); err != nil {
			failures = appendFailure(failures, project, opGetPullRequests, err)
			continue
		}

		prs, err := c.gitClient.GetPullRequestsByProject(ctx, git.GetPullRequestsByProjectArgs{
			Project: &project,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				Status: &git.PullRequestStatusValues.Active,
			},
		})
		if err != nil {
			log.Error("Failed to get pull requests", "project", project, "error", err)
			// The loop still visits every configured project: D-08 rejected
			// short-circuiting on auth failure explicitly.
			failures = appendFailure(failures, project, opGetPullRequests, err)
		} else if prs != nil {
			for _, pr := range *prs {
				allPRs = append(allPRs, c.convertPR(pr, project))
			}
		}
	}

	log.Info("Pull requests fetched", "total_count", len(allPRs), "failure_count", len(failures))
	return newResult(allPRs, failures), nil
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

	userID := c.currentUserID
	if userID == "" {
		if c.getConnectionData == nil {
			return fmt.Errorf("could not determine current user ID")
		}
		connData, err := c.getConnectionData(ctx, location.GetConnectionDataArgs{})
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		if connData == nil || connData.AuthenticatedUser == nil || connData.AuthenticatedUser.Id == nil {
			return fmt.Errorf("could not determine current user ID")
		}
		userID = connData.AuthenticatedUser.Id.String()
	}

	vote := 10 // Approved
	reviewer := &git.IdentityRefWithVote{
		Vote: &vote,
	}

	_, err := c.gitClient.CreatePullRequestReviewer(ctx, git.CreatePullRequestReviewerArgs{
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
