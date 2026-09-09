package main

import (
	"fmt"
	"io"

	"sentinovo.ai/bizInt/internal/ado"
)

// The post-filters below run on the Items slice after the read returns, never
// as query text sent to Azure DevOps (T-03-13). Filtering after the read is
// also what keeps the failure list and the Complete flag describing the read
// itself, which is what the exit-code contract keys on: a search that matches
// nothing is not an incomplete read.
//
// Two of the six filter flags are NOT here. --project and --repo-on-branches
// narrow the read at the method parameter instead, because the read methods
// already take those arguments; post-filtering on top would be redundant at
// best and a double filter at worst.
//
// Every predicate is the promoted one from internal/ado, so the CLI and the TUI
// cannot drift on any rule (CLI-06).

// warnUnusableIdentity reports an --assigned-to-me that cannot filter. Phase 1
// D-12 leaves identity resolution non-fatal, so an empty cache is a legitimate
// state, and ado.MatchesIdentity is a deliberate no-op on it. Silently
// returning an unfiltered list to a caller who asked for a filtered one is the
// spoofing failure T-03-12 names, so the no-op is announced on stderr rather
// than hidden. stderr, not stdout: OUT-03 reserves stdout for parseable output.
func warnUnusableIdentity(ff *filterFlags, warn io.Writer) {
	if ff.assignedToMe && ff.identity == "" {
		fmt.Fprintln(warn, "Warning: --assigned-to-me had no effect: the authenticated identity is unavailable, so no items were filtered.")
	}
}

func filterWorkItems(items []ado.WorkItem, ff *filterFlags, warn io.Writer) []ado.WorkItem {
	warnUnusableIdentity(ff, warn)
	out := make([]ado.WorkItem, 0, len(items))
	for _, item := range items {
		if ff.assignedToMe && !ado.MatchesIdentity(item.AssignedToEmail, ff.identity) {
			continue
		}
		if ff.hideDone && ado.IsDoneState(item.State) {
			continue
		}
		if ff.hideTasks && ado.IsTaskType(item.Type) {
			continue
		}
		// The searched field set matches the work items view exactly: the
		// formatted ID and the title.
		if !ado.MatchesSearch(ff.search, fmt.Sprintf("%d", item.ID), item.Title) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterPullRequests(items []ado.PullRequest, ff *filterFlags, _ io.Writer) []ado.PullRequest {
	out := make([]ado.PullRequest, 0, len(items))
	for _, item := range items {
		// GetPullRequests has no read-level repository parameter, so --repo
		// post-filters here. An empty slice disables the filter (D-18).
		if !ado.MatchesRepository(item.RepositoryName, ff.repos) {
			continue
		}
		if !ado.MatchesSearch(ff.search,
			fmt.Sprintf("%d", item.ID), item.Title, item.RepositoryName, item.CreatedBy) {
			continue
		}
		out = append(out, item)
	}
	return out
}

// filterBranches applies search only. --repo is passed into GetBranches as its
// repositories argument and filtered inside the read (branches.go:92), so
// post-filtering on repository here would be a second application of the same
// rule.
func filterBranches(items []ado.Branch, ff *filterFlags, _ io.Writer) []ado.Branch {
	out := make([]ado.Branch, 0, len(items))
	for _, item := range items {
		if !ado.MatchesSearch(ff.search, item.Name, item.RepositoryName, item.LastCommitBy) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterReleases(items []ado.Release, ff *filterFlags, _ io.Writer) []ado.Release {
	out := make([]ado.Release, 0, len(items))
	for _, item := range items {
		if !ado.MatchesSearch(ff.search, item.Name, item.DefinitionName) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterPipelineRuns(items []ado.PipelineRun, ff *filterFlags, _ io.Writer) []ado.PipelineRun {
	out := make([]ado.PipelineRun, 0, len(items))
	for _, item := range items {
		if !ado.MatchesSearch(ff.search,
			item.PipelineName, item.SourceBranch, item.RequestedBy, item.BuildNumber) {
			continue
		}
		out = append(out, item)
	}
	return out
}
