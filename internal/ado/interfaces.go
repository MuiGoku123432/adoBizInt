package ado

import (
	"context"
	"io"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// These package-local interfaces make destructive write methods testable without
// crossing the Azure DevOps REST boundary (SEAM-01). Each interface declares only
// methods this package calls; adding an uncalled method violates ROADMAP SC4.
// NewClient assigns concrete &X.ClientImpl values, built with
// NewClientWithOptions on the injected transport (D-11), to these
// interface-typed fields. That assignment is itself the compile-time drift
// check, so no separate ClientImpl assertion is added.
type workItemAPI interface {
	UpdateWorkItem(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error)
	GetWorkItems(context.Context, workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
	GetWorkItem(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error)
	QueryByWiql(context.Context, workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetClassificationNode(context.Context, workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error)
	GetAttachmentContent(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error)
}

type gitAPI interface {
	CreatePullRequestReviewer(context.Context, git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error)
	GetBranches(context.Context, git.GetBranchesArgs) (*[]git.GitBranchStats, error)
	GetPullRequest(context.Context, git.GetPullRequestArgs) (*git.GitPullRequest, error)
	GetPullRequestsByProject(context.Context, git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error)
	GetRefs(context.Context, git.GetRefsArgs) (*git.GetRefsResponseValue, error)
	GetRepositories(context.Context, git.GetRepositoriesArgs) (*[]git.GitRepository, error)
	UpdatePullRequest(context.Context, git.UpdatePullRequestArgs) (*git.GitPullRequest, error)
}

type buildAPI interface {
	GetBuilds(context.Context, build.GetBuildsArgs) (*build.GetBuildsResponseValue, error)
	GetDefinition(context.Context, build.GetDefinitionArgs) (*build.BuildDefinition, error)
	GetDefinitions(context.Context, build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error)
	QueueBuild(context.Context, build.QueueBuildArgs) (*build.Build, error)
}

type releaseAPI interface {
	GetApprovals(context.Context, release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error)
	GetReleaseDefinitions(context.Context, release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error)
	GetReleases(context.Context, release.GetReleasesArgs) (*release.GetReleasesResponseValue, error)
	UpdateReleaseApproval(context.Context, release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error)
}

type coreAPI interface {
	GetTeams(context.Context, core.GetTeamsArgs) (*[]core.WebApiTeam, error)
	GetTeamMembersWithExtendedProperties(context.Context, core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error)
}

type workAPI interface {
	GetTeamIterations(context.Context, work.GetTeamIterationsArgs) (*[]work.TeamSettingsIteration, error)
	GetBacklogConfigurations(context.Context, work.GetBacklogConfigurationsArgs) (*work.BacklogConfiguration, error)
}
