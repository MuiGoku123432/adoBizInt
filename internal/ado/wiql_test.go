package ado

import (
	"bytes"
	"context"
	"go/ast"
	"go/token"
	"log/slog"
	"strconv"
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

// TestWIQLInterpolationStructural keeps every WIQL query in package ado in the
// parameterized shape D-10 revised requires. A site is an outermost + chain with
// a string literal operand containing "FROM WorkItems" (matched without regard
// to case, as WIQL keywords are). For every site:
//
//   - each operand is a string literal or a call to iterationClause, the one
//     allow-listed escaper (D-12, RESEARCH OQ2). A bare identifier such as
//     project, or any other call such as strings.TrimSpace(project), fails;
//   - the chain's literal text contains @project and does not contain
//     "[System.TeamProject] = '", so the project is resolved from the request
//     route rather than quoted into the text.
//
// Every "FROM WorkItems" literal in the package must sit inside a checked
// site. A literal passed to fmt.Sprintf, placed in a composite literal, or
// assigned on its own makes the package-wide count disagree with the in-site
// count (RESEARCH Pitfall 4, defeat vector 1).
//
// The site count must be exactly 2 (getProjectWorkItems and getProjectCounts):
// a third WIQL site, or a split literal that hides one, must fail until it is
// reviewed (D-12).
//
// This is an AST walk rather than a grep for a reason that is specific to this
// repository: STATE.md records three Phase 3 incidents where a "grep returns
// nothing" criterion was satisfied and then quietly defeated by a prose comment
// explaining the forbidden thing. Comments are not part of the AST, so a doc
// line can neither satisfy nor defeat this assertion — only real code can.
func TestWIQLInterpolationStructural(t *testing.T) {
	fset, files := parsePackageFiles(t)

	sites, inSite, total := 0, 0, 0
	for _, file := range files {
		// A separate pass: the site pass below stops descending at each site.
		ast.Inspect(file, func(n ast.Node) bool {
			if isWIQLLiteral(n) {
				total++
			}
			return true
		})

		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || bin.Op != token.ADD {
				return true
			}
			var operands []ast.Expr
			flattenAdd(bin, &operands)

			isSite := false
			for _, op := range operands {
				if isWIQLLiteral(op) {
					isSite = true
				}
			}
			if !isSite {
				return true
			}
			sites++

			var text strings.Builder
			for _, op := range operands {
				switch op := op.(type) {
				case *ast.BasicLit:
					v, ok := stringValue(op)
					if !ok {
						t.Errorf("%s: WIQL chain operand %s is not a string literal", fset.Position(op.Pos()), op.Value)
						continue
					}
					if isWIQLLiteral(op) {
						inSite++
					}
					text.WriteString(v)
				case *ast.CallExpr:
					if fun, ok := op.Fun.(*ast.Ident); !ok || fun.Name != "iterationClause" {
						t.Errorf("%s: WIQL chain operand calls something other than iterationClause; only iterationClause may add non-literal text", fset.Position(op.Pos()))
					}
				default:
					t.Errorf("%s: WIQL chain operand is %T; only string literals and iterationClause calls are allowed (D-12)", fset.Position(op.Pos()), op)
				}
			}
			if !strings.Contains(text.String(), "@project") {
				t.Errorf("%s: WIQL chain does not filter on @project (D-10 revised)", fset.Position(bin.Pos()))
			}
			if strings.Contains(text.String(), "[System.TeamProject] = '") {
				t.Errorf("%s: WIQL chain quotes a project into [System.TeamProject]; use @project (D-10 revised)", fset.Position(bin.Pos()))
			}
			// Sub-chains of a site are part of it; do not count them again.
			return false
		})
	}

	if total != inSite {
		t.Errorf("found %d FROM WorkItems literals but only %d inside checked + chains; a WIQL query is built outside a + chain (fmt.Sprintf, a composite literal, or a standalone assignment)", total, inSite)
	}
	if sites != 2 {
		t.Errorf("found %d WIQL + chains, want exactly 2 (getProjectWorkItems and getProjectCounts); review any new site against D-10 revised and D-12", sites)
	}
}

// flattenAdd appends the operands of a + chain to out, looking through
// parentheses so that grouping cannot hide an operand.
func flattenAdd(e ast.Expr, out *[]ast.Expr) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		flattenAdd(x.X, out)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			flattenAdd(x.X, out)
			flattenAdd(x.Y, out)
			return
		}
		*out = append(*out, e)
	default:
		*out = append(*out, e)
	}
}

// isWIQLLiteral reports whether n is a string literal naming the WorkItems
// source, which is what marks a WIQL query.
func isWIQLLiteral(n ast.Node) bool {
	lit, ok := n.(*ast.BasicLit)
	if !ok {
		return false
	}
	v, ok := stringValue(lit)
	return ok && strings.Contains(strings.ToLower(v), "from workitems")
}

// stringValue returns the unquoted value of a string literal.
func stringValue(lit *ast.BasicLit) (string, bool) {
	if lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}
