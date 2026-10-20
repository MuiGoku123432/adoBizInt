package ado

// Read retries on HTTP 429 live here, in the service layer, and nowhere else
// (D-03). The transport cannot hold them: it sees one HTTP request at a time
// and cannot tell QueryByWiql, a POST that only reads, from QueueBuild, a POST
// that starts a build. A transport retry would re-send a write and could
// double-mutate under one audit record. At the narrow interfaces, reads and
// writes are distinct methods, so the line is drawn by name.
//
// Each decorator embeds the interface it wraps and overrides only its reads.
// Every method it does not override, every write included and any method a
// later phase adds, falls through the embedded field un-retried. That is the
// safe default: forgetting an override costs a retry, never a duplicate write.

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"

	"sentinovo.ai/bizInt/internal/logging"
)

// maxReadRetries is how many times a rate-limited read is re-issued, so a read
// makes at most maxReadRetries+1 attempts. It is a fixed constant, not config
// (D-03): two retries recover a transient 429 without hiding a persistent one.
const maxReadRetries = 2

// retryRead runs call and re-issues it while it fails with a raw HTTP 429, at
// most maxReadRetries times. Any other outcome is returned unchanged on the
// attempt that produced it: success, any status other than exactly 429, an
// error with no status (which covers the transport's D-01 fail-fast and every
// other not-sent error), and any failure once ctx has ended. After the last
// retry the 429 is wrapped as "rate limited after 2 retries", which keeps it
// reachable, so classifyFailure still reports rate_limit (D-08).
//
// Each retry waits where every request waits, in the transport's pacer, which
// has already seen the 429's Retry-After. The re-issued SDK call marshals a
// fresh request body, so nothing is replayed (Pitfall 8).
//
// Counting rule: retries counts retries attempted. A retry whose re-issued
// request the pacer then refuses under D-01, because the wait no longer fits
// the deadline, is counted although that request never left the process. The
// stderr summary therefore reports retries attempted, which is accepted:
// counting only retries that reached the wire would need a transport-to-service
// callback for no decision-relevant gain.
func retryRead[T any](ctx context.Context, retries *atomic.Int64, call func() (T, error)) (T, error) {
	for retry := 0; ; retry++ {
		v, err := call()
		if err == nil {
			return v, nil
		}
		if status, ok := statusCodeOf(err); !ok || status != 429 || ctx.Err() != nil {
			return v, err
		}
		if retry == maxReadRetries {
			return v, fmt.Errorf("rate limited after %d retries: %w", maxReadRetries, err)
		}
		retries.Add(1)
		logging.Logger().Info("Retrying rate-limited read", "retry", retry+1, "max", maxReadRetries)
	}
}

// retryingWorkItems retries the workItemAPI reads, QueryByWiql included: it
// is a POST that only reads. UpdateWorkItem is a write and falls through the
// embedded workItemAPI: never retried.
type retryingWorkItems struct {
	workItemAPI
	retries *atomic.Int64
}

// retryingGit retries the gitAPI reads. CreatePullRequestReviewer and
// UpdatePullRequest are writes and fall through the embedded gitAPI: never
// retried.
type retryingGit struct {
	gitAPI
	retries *atomic.Int64
}

// retryingBuild retries the buildAPI reads. QueueBuild is a write (a POST that
// starts a build) and falls through the embedded buildAPI: never retried.
type retryingBuild struct {
	buildAPI
	retries *atomic.Int64
}

// retryingRelease retries the releaseAPI reads. UpdateReleaseApproval is a
// write and falls through the embedded releaseAPI: never retried.
type retryingRelease struct {
	releaseAPI
	retries *atomic.Int64
}

func (r retryingGit) GetBranches(ctx context.Context, args git.GetBranchesArgs) (*[]git.GitBranchStats, error) {
	return retryRead(ctx, r.retries, func() (*[]git.GitBranchStats, error) { return r.gitAPI.GetBranches(ctx, args) })
}

func (r retryingGit) GetPullRequest(ctx context.Context, args git.GetPullRequestArgs) (*git.GitPullRequest, error) {
	return retryRead(ctx, r.retries, func() (*git.GitPullRequest, error) { return r.gitAPI.GetPullRequest(ctx, args) })
}

func (r retryingGit) GetPullRequestsByProject(ctx context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
	return retryRead(ctx, r.retries, func() (*[]git.GitPullRequest, error) { return r.gitAPI.GetPullRequestsByProject(ctx, args) })
}

func (r retryingGit) GetRefs(ctx context.Context, args git.GetRefsArgs) (*git.GetRefsResponseValue, error) {
	return retryRead(ctx, r.retries, func() (*git.GetRefsResponseValue, error) { return r.gitAPI.GetRefs(ctx, args) })
}

func (r retryingGit) GetRepositories(ctx context.Context, args git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
	return retryRead(ctx, r.retries, func() (*[]git.GitRepository, error) { return r.gitAPI.GetRepositories(ctx, args) })
}

func (r retryingWorkItems) GetWorkItems(ctx context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
	return retryRead(ctx, r.retries, func() (*[]workitemtracking.WorkItem, error) { return r.workItemAPI.GetWorkItems(ctx, args) })
}

func (r retryingWorkItems) GetWorkItem(ctx context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
	return retryRead(ctx, r.retries, func() (*workitemtracking.WorkItem, error) { return r.workItemAPI.GetWorkItem(ctx, args) })
}

func (r retryingWorkItems) QueryByWiql(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
	return retryRead(ctx, r.retries, func() (*workitemtracking.WorkItemQueryResult, error) { return r.workItemAPI.QueryByWiql(ctx, args) })
}

func (r retryingWorkItems) GetClassificationNode(ctx context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
	return retryRead(ctx, r.retries, func() (*workitemtracking.WorkItemClassificationNode, error) {
		return r.workItemAPI.GetClassificationNode(ctx, args)
	})
}

func (r retryingWorkItems) GetAttachmentContent(ctx context.Context, args workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error) {
	return retryRead(ctx, r.retries, func() (io.ReadCloser, error) { return r.workItemAPI.GetAttachmentContent(ctx, args) })
}

func (r retryingBuild) GetBuilds(ctx context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
	return retryRead(ctx, r.retries, func() (*build.GetBuildsResponseValue, error) { return r.buildAPI.GetBuilds(ctx, args) })
}

func (r retryingBuild) GetDefinition(ctx context.Context, args build.GetDefinitionArgs) (*build.BuildDefinition, error) {
	return retryRead(ctx, r.retries, func() (*build.BuildDefinition, error) { return r.buildAPI.GetDefinition(ctx, args) })
}

func (r retryingBuild) GetDefinitions(ctx context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
	return retryRead(ctx, r.retries, func() (*build.GetDefinitionsResponseValue, error) { return r.buildAPI.GetDefinitions(ctx, args) })
}

func (r retryingRelease) GetApprovals(ctx context.Context, args release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
	return retryRead(ctx, r.retries, func() (*release.GetApprovalsResponseValue, error) { return r.releaseAPI.GetApprovals(ctx, args) })
}

func (r retryingRelease) GetReleaseDefinitions(ctx context.Context, args release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
	return retryRead(ctx, r.retries, func() (*release.GetReleaseDefinitionsResponseValue, error) {
		return r.releaseAPI.GetReleaseDefinitions(ctx, args)
	})
}

func (r retryingRelease) GetReleases(ctx context.Context, args release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
	return retryRead(ctx, r.retries, func() (*release.GetReleasesResponseValue, error) { return r.releaseAPI.GetReleases(ctx, args) })
}
