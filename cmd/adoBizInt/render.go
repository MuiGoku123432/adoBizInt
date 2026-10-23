package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
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

func renderTeams(w io.Writer, items []ado.Team) {
	writeRows(w, items, func(b *strings.Builder, i ado.Team) {
		row(b, i.Project, i.Team, i.TeamID)
	})
}

func renderTeamMembers(w io.Writer, items []ado.TeamMember) {
	writeRows(w, items, func(b *strings.Builder, i ado.TeamMember) {
		kind := "user"
		if i.IsGroup {
			kind = "group"
		}
		row(b, i.Project, i.Team, i.DisplayName, i.UniqueName, kind,
			fmt.Sprintf("admin=%t", i.IsTeamAdmin), fmt.Sprintf("inactive=%t", i.Inactive))
	})
}

func renderSprints(w io.Writer, items []ado.Sprint) {
	writeRows(w, items, func(b *strings.Builder, i ado.Sprint) {
		row(b, i.Project, i.Team, i.Name, i.StartDate+".."+i.FinishDate)
	})
}

// renderSprintItems writes one row per item, each task indented under its
// story. Points are blank when the item holds none, never 0.
func renderSprintItems(w io.Writer, items []ado.SprintItem) {
	pts := func(p *float64) string {
		if p == nil {
			return ""
		}
		return strconv.FormatFloat(*p, 'f', -1, 64)
	}
	writeRows(w, items, func(b *strings.Builder, i ado.SprintItem) {
		row(b, i.Project, i.Team, strconv.Itoa(i.ID), i.Type, i.State, pts(i.Points), i.Title)
		for _, t := range i.Tasks {
			row(b, "", "", "  "+strconv.Itoa(t.ID), t.Type, t.State, pts(t.RemainingWork), t.Title)
		}
	})
}

func renderProcessFields(w io.Writer, items []ado.ProcessFields) {
	writeRows(w, items, func(b *strings.Builder, i ado.ProcessFields) {
		types := slices.Sorted(maps.Keys(i.DoneStates))
		done := make([]string, 0, len(types))
		for _, t := range types {
			done = append(done, t+"="+strings.Join(i.DoneStates[t], "/"))
		}
		row(b, i.Project, "points="+i.PointsField, "rank="+i.RankField, "done: "+strings.Join(done, ", "))
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
		where := f.Project
		if f.Team != "" {
			// Team names come from ADO (ResolveTeams returns its spelling).
			where += "/" + sanitize(f.Team)
		}
		fmt.Fprintf(&b, "warning: %s: %s failed (%s)\n", where, f.Operation, f.Kind)
	}
	_, _ = io.WriteString(w, b.String())
}

// renderFindings writes one line per finding to the stderr writer, after the
// scope line, the human counterpart of the envelope's findings. Detail is
// built by code but embeds team names ADO returned, so it is sanitized.
func renderFindings(w io.Writer, findings []ado.Finding) {
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "finding: %s: %s\n", f.Kind, sanitize(f.Detail))
	}
	_, _ = io.WriteString(w, b.String())
}

// renderScope writes the read's fixed server-side scope as one line to the
// stderr writer (D-04), never to stdout. It prints even when the read returned
// zero rows: an empty result under an undisclosed filter is exactly audit W4.
// A nil scope (an unscoped read) writes nothing. Every value is a code constant
// from internal/ado, so no field needs sanitize. Under --json the envelope's
// scope object carries the same fact instead.
func renderScope(w io.Writer, s *ado.Scope) {
	if s == nil {
		return
	}
	var parts []string
	if len(s.ExcludedStates) > 0 {
		parts = append(parts, "excludes "+strings.Join(s.ExcludedStates, "/"))
	}
	if s.Iterations != "" {
		parts = append(parts, s.Iterations+" iterations only")
	}
	if s.PRStatus != "" {
		parts = append(parts, s.PRStatus+" pull requests only")
	}
	if s.ApprovalStatus != "" {
		parts = append(parts, s.ApprovalStatus+" approvals only")
	}
	if s.AsOf != "" {
		parts = append(parts, "as of "+s.AsOf)
	}
	if s.AreaPathsCurrent {
		parts = append(parts, "team area paths as configured today")
	}
	if s.Caps != nil {
		parts = append(parts, fmt.Sprintf("caps max_teams=%d max_sprint_history=%d max_backlog_depth=%d",
			s.Caps.MaxTeams, s.Caps.MaxSprintHistory, s.Caps.MaxBacklogDepth))
	}
	_, _ = io.WriteString(w, "scope: "+strings.Join(parts, "; ")+"\n")
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
