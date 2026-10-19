# API Coverage: Azure DevOps Go SDK (azuredevops/v7 v7.1.0)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

Phase 8 upgrades the SDK the tool already integrates (b5 to v7) and changes how it is constructed (injected org-bound transport). It adds no ADO capability: 08-CONTEXT scope excludes any new read or write verb, the core sub-client and request-shape caps. Every capability the tool calls today is INTEGRATE on v7; the untouched SDK surface is recorded as reasoned OPT-OUT.

| capability | decision | reason |
|---|---|---|
| azuredevops.NewPatConnection (PAT connection, BaseUrl normalisation) | INTEGRATE | |
| azuredevops.NewClientWithOptions + WithHTTPClient (transport injection, D-11) | INTEGRATE | |
| azuredevops.Client.GetResourceAreas (startup discovery, D-10a) | INTEGRATE | |
| location.GetConnectionData (identity) | INTEGRATE | |
| workitemtracking.UpdateWorkItem | INTEGRATE | |
| workitemtracking.GetWorkItems | INTEGRATE | |
| workitemtracking.GetWorkItem | INTEGRATE | |
| workitemtracking.QueryByWiql | INTEGRATE | |
| workitemtracking.GetClassificationNode | INTEGRATE | |
| workitemtracking.GetAttachmentContent | INTEGRATE | |
| git.CreatePullRequestReviewer | INTEGRATE | |
| git.GetBranches | INTEGRATE | |
| git.GetPullRequest | INTEGRATE | |
| git.GetPullRequestsByProject | INTEGRATE | |
| git.GetRefs | INTEGRATE | |
| git.GetRepositories | INTEGRATE | |
| git.UpdatePullRequest | INTEGRATE | |
| build.GetBuilds | INTEGRATE | |
| build.GetDefinition | INTEGRATE | |
| build.GetDefinitions | INTEGRATE | |
| build.QueueBuild | INTEGRATE | |
| release.GetApprovals | INTEGRATE | |
| release.GetReleaseDefinitions | INTEGRATE | |
| release.GetReleases | INTEGRATE | |
| release.UpdateReleaseApproval | INTEGRATE | |
| work client construction (dormant seam) | INTEGRATE | |
| work.* methods (team settings, iterations, capacity, backlogs) | OPT-OUT | not needed yet: first caller is Phase 9 (TEAM-*); 08-CONTEXT D-16 names the dormant gap |
| core.* (projects, teams) | OPT-OUT | not needed yet: the core sub-client is built in Phase 9 (08-CONTEXT scope) |
| workitemtracking create, comments, links and other methods | OPT-OUT | explicitly out of scope: no new verb in Phase 8; story management is Phase 14 |
| git PR threads, comments and other methods | OPT-OUT | explicitly out of scope: no new verb in Phase 8; comments belong to Phase 14 |
| build and release methods not listed above | OPT-OUT | explicitly out of scope: no new verb in Phase 8 |
| default-client constructors (GetClientByUrl, typed X.NewClient, NewClient) | OPT-OUT | explicitly forbidden by D-11: these (and GetClientByResourceAreaId) build a default http.Client that bypasses the pacing transport and host rule |
| all other SDK areas (accounts, analytics, audit, pipelines, test, wiki, graph) | OPT-OUT | not needed: outside the tool's Azure DevOps scope; covers every remaining area package |
