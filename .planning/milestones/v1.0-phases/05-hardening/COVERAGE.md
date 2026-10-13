# API Coverage — Azure DevOps capabilities touched by Phase 05 (hardening)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
> Scope: this phase closes security findings and adds no capability. Each capability below is an
> Azure DevOps SDK capability that the phase newly consumes or changes how it consumes.

## Capabilities consumed

| capability | decision | reason |
|---|---|---|
| `GetAttachmentContent` by attachment GUID (05-01, D-01) | INTEGRATE | |
| `QueryByWiql` with the `@project` macro (05-05, D-10 revised) | INTEGRATE | |

The first row is used by `ado.Client.DownloadAttachment`. The second covers both WIQL sites, filtering on `[System.TeamProject] = @project` with the project carried in the request route.

## Explicit opt-outs

| capability | decision | reason |
|---|---|---|
| `GetAttachmentContentArgs.Project` route scoping | OPT-OUT | not needed — the relation URL names an organization-level attachment route, and passing `Id` alone matches it exactly |
| `GetAttachmentContentArgs.FileName` and `.Download` | OPT-OUT | not needed — the local file name comes from the relation's `name` attribute through `attachmentPath`, and `Download` only changes the server's Content-Disposition header |
| `workitemtracking.GetAttachmentZip` | OPT-OUT | not needed — the TUI downloads one file at a time |
| `workitemtracking.CreateAttachment` (upload) | OPT-OUT | explicitly out of scope — Phase 5 adds no capability (05-CONTEXT.md domain), and story management is v2 |
| CLI attachment download verb (`workitems download-attachment`) | OPT-OUT | deferred by D-02, with its `--output-dir` contract and the byte cap deferred by D-04 |
| WIQL `@currentIteration` and team-scoped macros | OPT-OUT | not needed yet — iteration filtering stays on explicit paths from `GetRecentIterations`, and team scoping belongs to TEAM-01..05 in v2 |
