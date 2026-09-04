package ado

import (
	"context"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/release"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

var (
	_ workItemAPI = (*fakeWorkItemAPI)(nil)
	_ gitAPI      = (*fakeGitAPI)(nil)
	_ buildAPI    = (*fakeBuildAPI)(nil)
	_ releaseAPI  = (*fakeReleaseAPI)(nil)
)

type fakeWorkItemAPI struct {
	t *testing.T

	UpdateWorkItemFunc        func(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiqlFunc           func(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetWorkItemsFunc          func(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
	GetWorkItemFunc           func(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error)
	GetClassificationNodeFunc func(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}

func (f *fakeWorkItemAPI) QueryByWiql(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
	f.t.Helper()
	if f.QueryByWiqlFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.QueryByWiql")
	}
	return f.QueryByWiqlFunc(ctx, args)
}

func (f *fakeWorkItemAPI) GetWorkItems(ctx context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.GetWorkItemsFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetWorkItems")
	}
	return f.GetWorkItemsFunc(ctx, args)
}

func (f *fakeWorkItemAPI) GetWorkItem(ctx context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.GetWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetWorkItem")
	}
	return f.GetWorkItemFunc(ctx, args)
}

func (f *fakeWorkItemAPI) GetClassificationNode(ctx context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
	f.t.Helper()
	if f.GetClassificationNodeFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.GetClassificationNode")
	}
	return f.GetClassificationNodeFunc(ctx, args)
}

type fakeGitAPI struct {
	t *testing.T

	CreatePullRequestReviewerFunc func(context.Context, git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error)
	GetBranchesFunc               func(context.Context, git.GetBranchesArgs) (*[]git.GitBranchStats, error)
	GetPullRequestFunc            func(context.Context, git.GetPullRequestArgs) (*git.GitPullRequest, error)
	GetPullRequestsByProjectFunc  func(context.Context, git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error)
	GetRefsFunc                   func(context.Context, git.GetRefsArgs) (*git.GetRefsResponseValue, error)
	GetRepositoriesFunc           func(context.Context, git.GetRepositoriesArgs) (*[]git.GitRepository, error)
	UpdatePullRequestFunc         func(context.Context, git.UpdatePullRequestArgs) (*git.GitPullRequest, error)
}

func (f *fakeGitAPI) CreatePullRequestReviewer(ctx context.Context, args git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error) {
	f.t.Helper()
	if f.CreatePullRequestReviewerFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.CreatePullRequestReviewer")
	}
	return f.CreatePullRequestReviewerFunc(ctx, args)
}

func (f *fakeGitAPI) GetBranches(ctx context.Context, args git.GetBranchesArgs) (*[]git.GitBranchStats, error) {
	f.t.Helper()
	if f.GetBranchesFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.GetBranches")
	}
	return f.GetBranchesFunc(ctx, args)
}

func (f *fakeGitAPI) GetPullRequest(ctx context.Context, args git.GetPullRequestArgs) (*git.GitPullRequest, error) {
	f.t.Helper()
	if f.GetPullRequestFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.GetPullRequest")
	}
	return f.GetPullRequestFunc(ctx, args)
}

func (f *fakeGitAPI) GetPullRequestsByProject(ctx context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
	f.t.Helper()
	if f.GetPullRequestsByProjectFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.GetPullRequestsByProject")
	}
	return f.GetPullRequestsByProjectFunc(ctx, args)
}

func (f *fakeGitAPI) GetRefs(ctx context.Context, args git.GetRefsArgs) (*git.GetRefsResponseValue, error) {
	f.t.Helper()
	if f.GetRefsFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.GetRefs")
	}
	return f.GetRefsFunc(ctx, args)
}

func (f *fakeGitAPI) GetRepositories(ctx context.Context, args git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
	f.t.Helper()
	if f.GetRepositoriesFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.GetRepositories")
	}
	return f.GetRepositoriesFunc(ctx, args)
}

func (f *fakeGitAPI) UpdatePullRequest(ctx context.Context, args git.UpdatePullRequestArgs) (*git.GitPullRequest, error) {
	f.t.Helper()
	if f.UpdatePullRequestFunc == nil {
		f.t.Fatalf("unexpected call to gitAPI.UpdatePullRequest")
	}
	return f.UpdatePullRequestFunc(ctx, args)
}

type fakeBuildAPI struct {
	t *testing.T

	GetBuildsFunc      func(context.Context, build.GetBuildsArgs) (*build.GetBuildsResponseValue, error)
	GetDefinitionFunc  func(context.Context, build.GetDefinitionArgs) (*build.BuildDefinition, error)
	GetDefinitionsFunc func(context.Context, build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error)
	QueueBuildFunc     func(context.Context, build.QueueBuildArgs) (*build.Build, error)
}

func (f *fakeBuildAPI) GetBuilds(ctx context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
	f.t.Helper()
	if f.GetBuildsFunc == nil {
		f.t.Fatalf("unexpected call to buildAPI.GetBuilds")
	}
	return f.GetBuildsFunc(ctx, args)
}

func (f *fakeBuildAPI) GetDefinition(ctx context.Context, args build.GetDefinitionArgs) (*build.BuildDefinition, error) {
	f.t.Helper()
	if f.GetDefinitionFunc == nil {
		f.t.Fatalf("unexpected call to buildAPI.GetDefinition")
	}
	return f.GetDefinitionFunc(ctx, args)
}

func (f *fakeBuildAPI) GetDefinitions(ctx context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
	f.t.Helper()
	if f.GetDefinitionsFunc == nil {
		f.t.Fatalf("unexpected call to buildAPI.GetDefinitions")
	}
	return f.GetDefinitionsFunc(ctx, args)
}

func (f *fakeBuildAPI) QueueBuild(ctx context.Context, args build.QueueBuildArgs) (*build.Build, error) {
	f.t.Helper()
	if f.QueueBuildFunc == nil {
		f.t.Fatalf("unexpected call to buildAPI.QueueBuild")
	}
	return f.QueueBuildFunc(ctx, args)
}

type fakeReleaseAPI struct {
	t *testing.T

	GetApprovalsFunc          func(context.Context, release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error)
	GetReleaseDefinitionsFunc func(context.Context, release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error)
	GetReleasesFunc           func(context.Context, release.GetReleasesArgs) (*release.GetReleasesResponseValue, error)
	UpdateReleaseApprovalFunc func(context.Context, release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error)
}

func (f *fakeReleaseAPI) GetApprovals(ctx context.Context, args release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
	f.t.Helper()
	if f.GetApprovalsFunc == nil {
		f.t.Fatalf("unexpected call to releaseAPI.GetApprovals")
	}
	return f.GetApprovalsFunc(ctx, args)
}

func (f *fakeReleaseAPI) GetReleaseDefinitions(ctx context.Context, args release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
	f.t.Helper()
	if f.GetReleaseDefinitionsFunc == nil {
		f.t.Fatalf("unexpected call to releaseAPI.GetReleaseDefinitions")
	}
	return f.GetReleaseDefinitionsFunc(ctx, args)
}

func (f *fakeReleaseAPI) GetReleases(ctx context.Context, args release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
	f.t.Helper()
	if f.GetReleasesFunc == nil {
		f.t.Fatalf("unexpected call to releaseAPI.GetReleases")
	}
	return f.GetReleasesFunc(ctx, args)
}

func (f *fakeReleaseAPI) UpdateReleaseApproval(ctx context.Context, args release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error) {
	f.t.Helper()
	if f.UpdateReleaseApprovalFunc == nil {
		f.t.Fatalf("unexpected call to releaseAPI.UpdateReleaseApproval")
	}
	return f.UpdateReleaseApprovalFunc(ctx, args)
}
