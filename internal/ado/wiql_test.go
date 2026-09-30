package ado

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// quoteProject is a project name that would break out of a quoted WIQL string
// if it were ever concatenated into the query text.
const quoteProject = "O'Brien"

// TestGetWorkItemsUsesProjectMacro pins D-10 revised and D-11 at the work-item
// WIQL site: the project reaches the server only through the request route
// (args.Project), the query names it with the @project macro, and the log line
// carries no query text.
func TestGetWorkItemsUsesProjectMacro(t *testing.T) {
	// note: this log capture is duplicated in TestDashboardUsesProjectMacro;
	// extract it on a third use. logging.Logger() falls back to slog.Default()
	// because no test in this package calls logging.Init.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	var gotQuery, gotProject string
	api := &fakeWorkItemAPI{
		t: t,
		GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			return nil, nil
		},
		QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			if args.Wiql == nil || args.Wiql.Query == nil || args.Project == nil {
				t.Fatal("QueryByWiql called without a query or a project")
			}
			gotQuery, gotProject = *args.Wiql.Query, *args.Project
			return &workitemtracking.WorkItemQueryResult{}, nil
		},
	}
	client := &Client{projects: []string{quoteProject}, workitemClient: api}

	if _, err := client.GetWorkItems(context.Background(), nil); err != nil {
		t.Fatalf("GetWorkItems() error = %v, want nil", err)
	}

	if !strings.Contains(gotQuery, "[System.TeamProject] = @project") {
		t.Errorf("query = %q, want it to filter on [System.TeamProject] = @project", gotQuery)
	}
	for _, name := range []string{quoteProject, "O''Brien"} {
		if strings.Contains(gotQuery, name) {
			t.Errorf("query = %q, contains the project name %q; it must travel only in the route", gotQuery, name)
		}
	}
	if gotProject != quoteProject {
		t.Errorf("args.Project = %q, want %q so @project resolves to it", gotProject, quoteProject)
	}
	logs := buf.String()
	if !strings.Contains(logs, "Executing WIQL query") {
		t.Errorf("log capture is vacuous: no %q line in %q", "Executing WIQL query", logs)
	}
	if strings.Contains(logs, "FROM WorkItems") {
		t.Errorf("log output contains query text (D-11): %q", logs)
	}
}

// TestIterationClauseEscapesQuotes pins iterationClause, the only path by which
// server-supplied iteration values enter WIQL text (D-10 revised): an empty or
// nil list adds no clause, paths keep their input order, and a single quote is
// doubled so it cannot close the quoted value.
func TestIterationClauseEscapesQuotes(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  string
	}{
		{name: "nil", paths: nil, want: ""},
		{name: "empty", paths: []string{}, want: ""},
		{
			name:  "ordered and escaped",
			paths: []string{`Proj\Sprint 1`, `O'Brien\Sprint 2`},
			want:  ` AND [System.IterationPath] IN ('Proj\Sprint 1', 'O''Brien\Sprint 2')`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := iterationClause(tc.paths); got != tc.want {
				t.Errorf("iterationClause(%q) = %q, want %q", tc.paths, got, tc.want)
			}
		})
	}
}

// TestDashboardUsesProjectMacro pins D-10 revised and D-11 at the dashboard
// WIQL site, with the same four assertions as TestGetWorkItemsUsesProjectMacro.
func TestDashboardUsesProjectMacro(t *testing.T) {
	// note: this log capture is duplicated in TestGetWorkItemsUsesProjectMacro;
	// extract it on a third use.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	var gotQuery, gotProject string
	client := &Client{
		projects: []string{quoteProject},
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				if args.Wiql == nil || args.Wiql.Query == nil || args.Project == nil {
					t.Fatal("QueryByWiql called without a query or a project")
				}
				gotQuery, gotProject = *args.Wiql.Query, *args.Project
				return &workitemtracking.WorkItemQueryResult{}, nil
			},
		},
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestsByProjectFunc: func(_ context.Context, _ git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
				return &[]git.GitPullRequest{}, nil
			},
		},
	}

	if _, err := client.GetDashboardCounts(context.Background(), nil); err != nil {
		t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
	}

	if !strings.Contains(gotQuery, "[System.TeamProject] = @project") {
		t.Errorf("query = %q, want it to filter on [System.TeamProject] = @project", gotQuery)
	}
	for _, name := range []string{quoteProject, "O''Brien"} {
		if strings.Contains(gotQuery, name) {
			t.Errorf("query = %q, contains the project name %q; it must travel only in the route", gotQuery, name)
		}
	}
	if gotProject != quoteProject {
		t.Errorf("args.Project = %q, want %q so @project resolves to it", gotProject, quoteProject)
	}
	logs := buf.String()
	if !strings.Contains(logs, "Executing dashboard WIQL query") {
		t.Errorf("log capture is vacuous: no %q line in %q", "Executing dashboard WIQL query", logs)
	}
	if strings.Contains(logs, "FROM WorkItems") {
		t.Errorf("log output contains query text (D-11): %q", logs)
	}
}
