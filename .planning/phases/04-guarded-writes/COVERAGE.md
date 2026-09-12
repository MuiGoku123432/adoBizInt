# API Coverage — Azure DevOps write surface

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
> Scope: Phase 04 (guarded writes). Capability = an Azure DevOps API capability the
> CLI exposes or must consume in this phase.

## Mutating capabilities (CLI-03 — the seven mutations)

| capability | decision | reason |
|---|---|---|
| `workitemtracking.UpdateWorkItem` — state only (`ado.UpdateWorkItemState`) | INTEGRATE | |
| `workitemtracking.UpdateWorkItem` — state + effort (`ado.UpdateWorkItemStateWithEffort`) | INTEGRATE | |
| `git.CreatePullRequestReviewer` — approve (`ado.ApprovePullRequest`) | INTEGRATE | |
| `git.UpdatePullRequest` — complete/merge (`ado.CompletePullRequest`) | INTEGRATE | |
| `release.UpdateReleaseApproval` — approve (`ado.ApproveRelease`) | INTEGRATE | |
| `release.UpdateReleaseApproval` — reject (`ado.RejectRelease`) | INTEGRATE | |
| `build.QueueBuild` — queue pipeline (`ado.QueuePipeline`) | INTEGRATE | |

## Read capabilities newly consumed by the dry-run pre-read (D-01)

| capability | decision | reason |
|---|---|---|
| `workitemtracking.GetWorkItem` — by-id work item, incl. the three `Microsoft.VSTS.Scheduling.*` effort fields | INTEGRATE | |
| `git.GetPullRequest` — by-id PR (status, reviewer votes, merge status) | INTEGRATE | |
| `release.GetApprovals` / by-id approval lookup — current approval status | INTEGRATE | |
| `build.GetDefinition` — by-id pipeline definition (existence + default branch) | INTEGRATE | |

## Explicit opt-outs

| capability | decision | reason |
|---|---|---|
| JSON-Patch `Remove` operation on effort fields ("explicit clear") | OPT-OUT | No decision in 04-CONTEXT.md defines a clear semantic; D-17/D-18 define only absent / value / zero. Inventing a `--clear-*` flag would exceed the locked scope. |
| `workitemtracking.CreateWorkItem`, work-item comments and links | OPT-OUT | STORY-01..03 are v2 per 04-CONTEXT.md `<deferred>`; this phase guards the existing surface rather than extending it. |
| Batched multi-target mutation endpoints | OPT-OUT | D-25 locks one target per invocation; batching needs a partial-failure exit rule the code contract has no answer for. |
| Post-timeout verify-read | OPT-OUT | D-20 rejects it explicitly; a verify-read can itself time out and a concurrent change makes it lie with confidence. |
| Retry / 429 backoff endpoints and headers | OPT-OUT | PLAT-03, deferred. |
