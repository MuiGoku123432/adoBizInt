package main

import (
	"io"

	"sentinovo.ai/bizInt/internal/ado"
)

// Human-mode renderers (D-12). Plain text only: nothing here imports
// internal/ui, because lipgloss writes ANSI escape sequences and a redirected
// stdout full of escapes is hostile to the agent this CLI exists for.
//
// RED phase: every function below is a deliberate no-op so the behaviour tests
// fail before the implementation exists.

func renderWorkItems(w io.Writer, items []ado.WorkItem) {}

func renderPullRequests(w io.Writer, items []ado.PullRequest) {}

func renderPipelineRuns(w io.Writer, items []ado.PipelineRun) {}

func renderPipelineDefinitions(w io.Writer, items []ado.PipelineDefinition) {}

func renderReleases(w io.Writer, items []ado.Release) {}

func renderReleaseApprovals(w io.Writer, items []ado.ReleaseApproval) {}

func renderBranches(w io.Writer, items []ado.Branch) {}

func renderDashboardCounts(w io.Writer, items []ado.DashboardCounts) {}

func renderFailures(w io.Writer, failures []ado.Failure) {}
