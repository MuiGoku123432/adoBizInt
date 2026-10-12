package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
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
// parameterized shape D-10 revised requires. A site is an outermost + chain
// whose string literals, joined, contain "FROM WorkItem" (matched without
// regard to case, as WIQL keywords are), so a literal split across two
// operands still marks one, and FROM WorkItemLinks counts as well as FROM
// WorkItems. For every site:
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
// The check is also anchored on the sink (WR-06): every Wiql{Query: &x} must
// take x from exactly one := whose right side is a checked site, so a query
// whose source text comes from a variable, or one extended by += after the
// site, fails even though no literal names the source.
//
// The site count and the QueryByWiql call count must both be exactly 2
// (getProjectWorkItems and getProjectCounts): a third WIQL query must fail
// until it is reviewed (D-12). TestWIQLCheckCatchesBypasses proves the walk
// reports each bypass shape.
//
// This is an AST walk rather than a grep for a reason that is specific to this
// repository: STATE.md records three Phase 3 incidents where a "grep returns
// nothing" criterion was satisfied and then quietly defeated by a prose comment
// explaining the forbidden thing. Comments are not part of the AST, so a doc
// line can neither satisfy nor defeat this assertion — only real code can.
func TestWIQLInterpolationStructural(t *testing.T) {
	fset, files := parsePackageFiles(t)
	sites, sinks, problems := checkWIQL(fset, files)
	for _, p := range problems {
		t.Error(p)
	}
	if sites != 2 {
		t.Errorf("found %d WIQL + chains, want exactly 2 (getProjectWorkItems and getProjectCounts); review any new site against D-10 revised and D-12", sites)
	}
	if sinks != 2 {
		t.Errorf("found %d QueryByWiql calls, want exactly 2; a new query must be reviewed against D-10 revised and D-12", sinks)
	}
}

// checkWIQL is TestWIQLInterpolationStructural's walk. It returns the number of
// WIQL + chains, the number of QueryByWiql calls, and every violation, so
// TestWIQLCheckCatchesBypasses can prove it fails on the shapes it claims to
// catch.
func checkWIQL(fset *token.FileSet, files map[string]*ast.File) (int, int, []string) {
	var problems []string
	sites := map[ast.Expr]bool{}
	inSite, total := 0, 0
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

			// A chain is a site when its literals JOINED name the source, so a
			// literal split across "FROM " + "WorkItems" cannot hide one (WR-06).
			var joined strings.Builder
			for _, op := range operands {
				if lit, ok := op.(*ast.BasicLit); ok {
					v, _ := stringValue(lit)
					joined.WriteString(v)
				}
			}
			if !isWIQLText(joined.String()) {
				return true
			}
			sites[bin] = true

			var text strings.Builder
			for _, op := range operands {
				switch op := op.(type) {
				case *ast.BasicLit:
					v, ok := stringValue(op)
					if !ok {
						problems = append(problems, fmt.Sprintf("%s: WIQL chain operand %s is not a string literal", fset.Position(op.Pos()), op.Value))
						continue
					}
					if isWIQLLiteral(op) {
						inSite++
					}
					text.WriteString(v)
				case *ast.CallExpr:
					if fun, ok := op.Fun.(*ast.Ident); !ok || fun.Name != "iterationClause" {
						problems = append(problems, fmt.Sprintf("%s: WIQL chain operand calls something other than iterationClause; only iterationClause may add non-literal text", fset.Position(op.Pos())))
					}
				default:
					problems = append(problems, fmt.Sprintf("%s: WIQL chain operand is %T; only string literals and iterationClause calls are allowed (D-12)", fset.Position(op.Pos()), op))
				}
			}
			if !strings.Contains(text.String(), "@project") {
				problems = append(problems, fmt.Sprintf("%s: WIQL chain does not filter on @project (D-10 revised)", fset.Position(bin.Pos())))
			}
			if strings.Contains(text.String(), "[System.TeamProject] = '") {
				problems = append(problems, fmt.Sprintf("%s: WIQL chain quotes a project into [System.TeamProject]; use @project (D-10 revised)", fset.Position(bin.Pos())))
			}
			// Sub-chains of a site are part of it; do not count them again.
			return false
		})
	}

	if total != inSite {
		problems = append(problems, fmt.Sprintf("found %d FROM WorkItems literals but only %d inside checked + chains; a WIQL query is built outside a + chain (fmt.Sprintf, a composite literal, or a standalone assignment)", total, inSite))
	}

	// The sink half (WR-06). The text test above can only see chains that name
	// the source; a query whose source comes from a variable is invisible to
	// it. So every Wiql{Query: &x} must take x from exactly one assignment,
	// and that assignment must be a checked site. A second assignment (a +=
	// after the site) fails, as does a Wiql built without a Query.
	sinks := 0
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CallExpr:
					if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "QueryByWiql" {
						sinks++
					}
				case *ast.CompositeLit:
					if sel, ok := n.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Wiql" {
						if p := checkWiqlQuery(fn.Body, n, sites); p != "" {
							problems = append(problems, fmt.Sprintf("%s: %s", fset.Position(n.Pos()), p))
						}
					}
				}
				return true
			})
		}
	}
	return len(sites), sinks, problems
}

// checkWiqlQuery reports why lit's Query does not come from a checked site, or
// "" when it does.
func checkWiqlQuery(body *ast.BlockStmt, lit *ast.CompositeLit, sites map[ast.Expr]bool) string {
	var name string
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Query" {
			if addr, ok := kv.Value.(*ast.UnaryExpr); ok && addr.Op == token.AND {
				if id, ok := addr.X.(*ast.Ident); ok {
					name = id.Name
				}
			}
		}
	}
	if name == "" {
		return "Wiql literal has no Query: &name; the query text must come from a named, checked + chain"
	}

	var assigned []ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
				var rhs ast.Expr
				if len(as.Rhs) == len(as.Lhs) && as.Tok == token.DEFINE {
					rhs = ast.Unparen(as.Rhs[i])
				}
				assigned = append(assigned, rhs)
			}
		}
		return true
	})
	if len(assigned) != 1 {
		return fmt.Sprintf("WIQL query %s is assigned %d times, want exactly one := from a checked + chain", name, len(assigned))
	}
	if !sites[assigned[0]] {
		return fmt.Sprintf("WIQL query %s is not built by a checked + chain naming FROM WorkItems", name)
	}
	return ""
}

// TestWIQLCheckCatchesBypasses pins WR-06: checkWIQL must report each shape
// that would put the project back into WIQL text, and must accept the shape
// the two real sites use, or the structural test proves nothing.
func TestWIQLCheckCatchesBypasses(t *testing.T) {
	cases := map[string]struct {
		body    string
		wantErr bool
	}{
		"parameterized site": {wantErr: false, body: `
	wiql := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project" + iterationClause(nil)`},
		"split literal": {wantErr: true, body: `
	wiql := "SELECT [System.Id] FROM " + "WorkItems WHERE [System.TeamProject] = '" + project + "'"`},
		"WorkItemLinks source": {wantErr: true, body: `
	wiql := "SELECT [System.Id] FROM WorkItemLinks WHERE [Source].[System.TeamProject] = '" + project + "'"`},
		"source split by a variable": {wantErr: true, body: `
	wiql := "SELECT [System.Id] FROM " + source + " WHERE [System.TeamProject] = @project"`},
		"appended after the site": {wantErr: true, body: `
	wiql := "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project" + iterationClause(nil)
	wiql += " AND [System.Title] = '" + title + "'"`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := "package ado\n\nfunc f(project, source, title string) {" + tc.body + `
	_ = workitemtracking.QueryByWiqlArgs{Wiql: &workitemtracking.Wiql{Query: &wiql}}
	_, _ = c.workitemClient.QueryByWiql(ctx, query)
}
`
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "snippet.go", src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse snippet: %v", err)
			}
			_, _, problems := checkWIQL(fset, map[string]*ast.File{"snippet.go": file})
			if tc.wantErr && len(problems) == 0 {
				t.Error("checkWIQL reported nothing, want a violation")
			}
			if !tc.wantErr && len(problems) != 0 {
				t.Errorf("checkWIQL reported %v, want none", problems)
			}
		})
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

// isWIQLLiteral reports whether n is a string literal naming a work item
// source, which is what marks a WIQL query.
func isWIQLLiteral(n ast.Node) bool {
	lit, ok := n.(*ast.BasicLit)
	if !ok {
		return false
	}
	v, ok := stringValue(lit)
	return ok && isWIQLText(v)
}

// isWIQLText reports whether s names a work item source. The prefix match
// covers FROM WorkItemLinks as well as FROM WorkItems; both are WIQL and both
// are project-scoped.
func isWIQLText(s string) bool {
	return strings.Contains(strings.ToLower(s), "from workitem")
}

// stringValue returns the unquoted value of a string literal.
func stringValue(lit *ast.BasicLit) (string, bool) {
	if lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

// workItemScopeJSON is the published scope of workitems list (D-01).
const workItemScopeJSON = `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}`

// TestGetWorkItemsScope pins D-01 and audit W4 for workitems list: every return
// path discloses the fixed WIQL filter, the excluded states match the query's
// NOT IN list in order, and scope never changes completeness.
func TestGetWorkItemsScope(t *testing.T) {
	scopeJSON := func(t *testing.T, s *Scope) string {
		t.Helper()
		if s == nil {
			t.Fatal("Scope is nil, want the work-item filter")
		}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal scope: %v", err)
		}
		return string(raw)
	}

	t.Run("scope matches the query", func(t *testing.T) {
		var gotQuery string
		api := &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				gotQuery = *args.Wiql.Query
				return &workitemtracking.WorkItemQueryResult{}, nil
			},
		}
		client := &Client{projects: []string{"P"}, workitemClient: api}

		result, err := client.GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		if got := scopeJSON(t, result.Scope); got != workItemScopeJSON {
			t.Errorf("scope = %s, want %s", got, workItemScopeJSON)
		}
		notIn := "[System.State] NOT IN ('" + strings.Join(result.Scope.ExcludedStates, "', '") + "')"
		if !strings.Contains(gotQuery, notIn) {
			t.Errorf("query = %q, want it to contain %q so scope and request cannot drift", gotQuery, notIn)
		}
		if !result.Complete || len(result.Failures) != 0 {
			t.Errorf("Complete = %v, Failures = %v, want true and none", result.Complete, result.Failures)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		if want := `"scope":` + workItemScopeJSON; !strings.Contains(string(raw), want) {
			t.Errorf("result JSON = %s, want it to contain %s", raw, want)
		}
	})

	t.Run("early return carries the scope", func(t *testing.T) {
		result, err := (&Client{}).GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		if got := scopeJSON(t, result.Scope); got != workItemScopeJSON {
			t.Errorf("scope = %s, want %s", got, workItemScopeJSON)
		}
	})

	t.Run("a failed project keeps the scope", func(t *testing.T) {
		api := &fakeWorkItemAPI{
			t: t,
			GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
				return nil, nil
			},
			QueryByWiqlFunc: func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
				return nil, errors.New("query failed")
			},
		}
		client := &Client{projects: []string{"P"}, workitemClient: api}

		result, err := client.GetWorkItems(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetWorkItems() error = %v, want nil", err)
		}
		if result.Complete || len(result.Failures) != 1 {
			t.Errorf("Complete = %v, Failures = %v, want false and one entry", result.Complete, result.Failures)
		}
		if got := scopeJSON(t, result.Scope); got != workItemScopeJSON {
			t.Errorf("scope = %s, want %s", got, workItemScopeJSON)
		}
	})

	t.Run("results do not share a scope", func(t *testing.T) {
		client := &Client{}
		first, _ := client.GetWorkItems(context.Background(), nil)
		second, _ := client.GetWorkItems(context.Background(), nil)
		if first.Scope == nil || first.Scope == second.Scope {
			t.Errorf("Scope pointers %p and %p, want two distinct non-nil values", first.Scope, second.Scope)
		}
	})
}

// dashboardScopeJSON is the published scope of dashboard counts (D-01).
const dashboardScopeJSON = `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent","pr_status":"active"}`

// TestGetDashboardCountsScope pins D-01 and audit W4 for dashboard counts: both
// returns disclose the work-item WIQL filter and the PR status filter, each tied
// to the request the dashboard actually sends.
func TestGetDashboardCountsScope(t *testing.T) {
	t.Run("scope matches the requests", func(t *testing.T) {
		var gotQuery string
		var gotStatus *git.PullRequestStatus
		client := &Client{
			projects: []string{"P"},
			workitemClient: &fakeWorkItemAPI{
				t: t,
				GetClassificationNodeFunc: func(_ context.Context, _ workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
					return nil, nil
				},
				QueryByWiqlFunc: func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
					gotQuery = *args.Wiql.Query
					return &workitemtracking.WorkItemQueryResult{}, nil
				},
			},
			gitClient: &fakeGitAPI{
				t: t,
				GetPullRequestsByProjectFunc: func(_ context.Context, args git.GetPullRequestsByProjectArgs) (*[]git.GitPullRequest, error) {
					gotStatus = args.SearchCriteria.Status
					return &[]git.GitPullRequest{}, nil
				},
			},
		}

		result, err := client.GetDashboardCounts(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
		}
		if result.Scope == nil {
			t.Fatal("Scope is nil, want the dashboard filters")
		}
		raw, err := json.Marshal(result.Scope)
		if err != nil {
			t.Fatalf("marshal scope: %v", err)
		}
		if string(raw) != dashboardScopeJSON {
			t.Errorf("scope = %s, want %s", raw, dashboardScopeJSON)
		}
		notIn := "[System.State] NOT IN ('" + strings.Join(result.Scope.ExcludedStates, "', '") + "')"
		if !strings.Contains(gotQuery, notIn) {
			t.Errorf("query = %q, want it to contain %q so scope and request cannot drift", gotQuery, notIn)
		}
		if gotStatus == nil || string(*gotStatus) != result.Scope.PRStatus {
			t.Errorf("SearchCriteria.Status = %v, want %q", gotStatus, result.Scope.PRStatus)
		}
		if !result.Complete {
			t.Errorf("Complete = false, want true (Failures = %v)", result.Failures)
		}
	})

	t.Run("early return carries the scope", func(t *testing.T) {
		result, err := (&Client{}).GetDashboardCounts(context.Background(), nil)
		if err != nil {
			t.Fatalf("GetDashboardCounts() error = %v, want nil", err)
		}
		raw, err := json.Marshal(result.Scope)
		if err != nil {
			t.Fatalf("marshal scope: %v", err)
		}
		if string(raw) != dashboardScopeJSON {
			t.Errorf("scope = %s, want %s", raw, dashboardScopeJSON)
		}
	})
}
