package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
)

// Human-mode renderers (D-12). Plain text only: this file imports nothing from
// the TUI packages. The responsibility map forbids it, and styled ANSI escapes
// in a redirected stdout are hostile to the agent this CLI exists for — plain
// text is the answer to that discretion item.
//
// Every renderer writes tab-separated columns, one record per line, and writes
// nothing at all for an empty slice. No header, no count, no "(none)"
// placeholder: a caller counting lines must get zero for an empty read rather
// than one line it would have to recognise and discard.
//
// Each renderer builds its whole output in a strings.Builder and issues a
// single Write, matching emitJSON's discipline — a partial line ahead of a
// crash record is the failure mode that buys.

func renderWorkItems(w io.Writer, items []ado.WorkItem) {
	writeRows(w, items, func(b *strings.Builder, i ado.WorkItem) {
		row(b, fmt.Sprint(i.ID), i.Project, i.Type, i.State, i.AssignedTo, i.Title)
	})
}

func renderPullRequests(w io.Writer, items []ado.PullRequest) {
	writeRows(w, items, func(b *strings.Builder, i ado.PullRequest) {
		row(b, fmt.Sprint(i.ID), i.Project, i.RepositoryName, i.Status, i.CreatedBy, i.Title)
	})
}

func renderPipelineRuns(w io.Writer, items []ado.PipelineRun) {
	writeRows(w, items, func(b *strings.Builder, i ado.PipelineRun) {
		row(b, fmt.Sprint(i.ID), i.Project, i.PipelineName, i.BuildNumber,
			i.Status, i.Result, i.SourceBranch, stamp(i.FinishTime))
	})
}

func renderPipelineDefinitions(w io.Writer, items []ado.PipelineDefinition) {
	writeRows(w, items, func(b *strings.Builder, i ado.PipelineDefinition) {
		row(b, fmt.Sprint(i.ID), i.Project, i.Name, i.QueueStatus, i.DefaultBranch)
	})
}

func renderReleases(w io.Writer, items []ado.Release) {
	writeRows(w, items, func(b *strings.Builder, i ado.Release) {
		row(b, fmt.Sprint(i.ID), i.Project, i.DefinitionName, i.Name, i.Status,
			fmt.Sprint(i.PendingApprovals), stamp(i.ModifiedOn))
	})
}

func renderReleaseApprovals(w io.Writer, items []ado.ReleaseApproval) {
	writeRows(w, items, func(b *strings.Builder, i ado.ReleaseApproval) {
		row(b, fmt.Sprint(i.ID), i.Project, i.ReleaseName, i.EnvironmentName,
			i.ApprovalType, i.Status, i.Approver)
	})
}

func renderBranches(w io.Writer, items []ado.Branch) {
	writeRows(w, items, func(b *strings.Builder, i ado.Branch) {
		row(b, i.Project, i.RepositoryName, i.Name,
			fmt.Sprintf("+%d/-%d", i.AheadCount, i.BehindCount),
			fmt.Sprintf("default=%t", i.IsDefault), stamp(i.LastCommitDate))
	})
}

// renderDashboardCounts renders the single-element Items slice the dashboard
// read returns (Phase 2 D-01). It stays a loop rather than an items[0] index so
// an empty envelope writes nothing instead of panicking.
func renderDashboardCounts(w io.Writer, items []ado.DashboardCounts) {
	writeRows(w, items, func(b *strings.Builder, i ado.DashboardCounts) {
		row(b, fmt.Sprintf("work_items=%d", i.WorkItems),
			fmt.Sprintf("prs=%d", i.PRs),
			fmt.Sprintf("pipelines=%d", i.Pipelines))
	})
}

// renderFailures writes one line per unreachable project to the stderr writer
// (D-13, threat T-03-07). Failures are never suppressed in human mode and never
// written to stdout: a human running the default is precisely the caller who
// must not mistake a broken read for an empty one, and a caller piping stdout
// must not have failure prose land in the parseable channel.
//
// The underlying error message is deliberately not printed. result.go:131-134
// records that the server error text is localisable and server-controlled;
// project, operation and kind are the three fields that are actionable, and the
// full text is available under --json and in the log file.
func renderFailures(w io.Writer, failures []ado.Failure) {
	if len(failures) == 0 {
		return
	}
	var b strings.Builder
	for _, f := range failures {
		fmt.Fprintf(&b, "warning: %s: %s failed (%s)\n", f.Project, f.Operation, f.Kind)
	}
	_, _ = io.WriteString(w, b.String())
}

// writeRows is the shared body: build every line, then write once, and write
// nothing at all when there is nothing to say.
func writeRows[T any](w io.Writer, items []T, render func(*strings.Builder, T)) {
	if len(items) == 0 {
		return
	}
	var b strings.Builder
	for _, item := range items {
		render(&b, item)
	}
	_, _ = io.WriteString(w, b.String())
}

// row writes one tab-separated line. Embedded tabs and newlines in
// server-supplied text (a work item title can contain either) are replaced with
// spaces, so one record is always exactly one line and a caller splitting on
// tabs gets a stable column count.
func row(b *strings.Builder, fields ...string) {
	for i, f := range fields {
		if i > 0 {
			b.WriteByte('\t')
		}
		b.WriteString(sanitize(f))
	}
	b.WriteByte('\n')
}

// sanitize flattens a field to a single line. It also strips ESC, which is the
// only byte that could turn server-supplied text into a terminal escape
// sequence in output this file otherwise guarantees is escape-free.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r':
			return ' '
		case '\x1b':
			return -1
		}
		return r
	}, s)
}

// stamp formats a timestamp, rendering the zero value as an empty column rather
// than as a misleading year-one date.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
