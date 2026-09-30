package ado

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/release"
)

// TestGetPipelineRunsSkipsNilID pins the MED4 fix in GetPipelineRuns: a matching
// definition the server returned without an Id is skipped silently (D-14), and
// the well-formed definition beside it still produces its row.
func TestGetPipelineRunsSkipsNilID(t *testing.T) {
	definitionID := 17
	client := &Client{
		projects:  []string{"project-one"},
		pipelines: []string{"pipeline-one"},
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionsFunc: func(_ context.Context, _ build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				return &build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{
					{Id: &definitionID, Name: strPtr("pipeline-one")},
					{Name: strPtr("pipeline-one")},
				}}, nil
			},
			GetBuildsFunc: func(_ context.Context, _ build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				return &build.GetBuildsResponseValue{}, nil
			},
			GetDefinitionFunc: func(_ context.Context, _ build.GetDefinitionArgs) (*build.BuildDefinition, error) {
				return nil, nil
			},
		},
	}

	result, err := client.GetPipelineRuns(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if len(result.Items) != 1 || result.Items[0].PipelineID != definitionID {
		t.Fatalf("GetPipelineRuns() Items = %+v, want exactly one row with PipelineID %d", result.Items, definitionID)
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetPipelineRuns() Complete/Failures = %v/%+v, want true/none — a nil Id is malformed data, not a failed call", result.Complete, result.Failures)
	}
}

// TestGetReleasesSkipsNilID is the GetReleases counterpart of the test above.
func TestGetReleasesSkipsNilID(t *testing.T) {
	definitionID := 31
	client := &Client{
		projects:           []string{"project-one"},
		releaseDefinitions: []string{"release-definition-one"},
		releaseClient: &fakeReleaseAPI{
			t: t,
			GetReleaseDefinitionsFunc: func(_ context.Context, _ release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
				return &release.GetReleaseDefinitionsResponseValue{Value: []release.ReleaseDefinition{
					{Id: &definitionID, Name: strPtr("release-definition-one")},
					{Name: strPtr("release-definition-one")},
				}}, nil
			},
			GetReleasesFunc: func(_ context.Context, _ release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
				return &release.GetReleasesResponseValue{}, nil
			},
		},
	}

	result, err := client.GetReleases(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	if len(result.Items) != 1 || result.Items[0].DefinitionID != definitionID {
		t.Fatalf("GetReleases() Items = %+v, want exactly one row with DefinitionID %d", result.Items, definitionID)
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetReleases() Complete/Failures = %v/%+v, want true/none — a nil Id is malformed data, not a failed call", result.Complete, result.Failures)
	}
}

// TestCollectedDefinitionsGuardNameAndID pins the shape that produced MED4. A
// "collecting loop" is a range loop that appends its range value to a slice
// inside a function that later dereferences `*X.Id`. Every such loop must open
// with `if v.Name == nil || v.Id == nil { continue }`, so nothing without an Id
// reaches the later dereference.
//
// The rule is scoped to "collect, then dereference .Id" rather than to every
// pointer dereference in the package (D-13). A flow-insensitive check of all
// dereferences cannot see the enclosing guard, so most hits would be false
// positives. The exact count of 2 is deliberate: a third collecting loop fails
// this test and forces a review of whether it needs the same guard.
//
// This is an AST walk rather than a grep for a reason that is specific to this
// repository: STATE.md records three Phase 3 incidents where a "grep returns
// nothing" criterion was satisfied and then quietly defeated by a prose comment
// explaining the forbidden thing. Comments and string literals are not part of
// the AST, so neither a doc line nor a message string can satisfy or defeat this
// assertion — only real code can.
func TestCollectedDefinitionsGuardNameAndID(t *testing.T) {
	// note: the glob-and-parse walk over non-test files is duplicated in
	// TestNoRawHTTPInADO (attachments_test.go); extract it on a third use.
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package files: %v", err)
	}

	fset := token.NewFileSet()
	found := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				loop, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				v, ok := loop.Value.(*ast.Ident)
				if !ok || !appendsIdent(loop.Body, v.Name) || !derefsIDAfter(fn.Body, loop.End()) {
					return true
				}
				found++
				if !guardsNameAndID(loop.Body, v.Name) {
					t.Errorf("%s: loop over %s collects definitions that are later dereferenced via *.Id, but does not open with `if %s.Name == nil || %s.Id == nil { continue }`",
						fset.Position(loop.Pos()), v.Name, v.Name, v.Name)
				}
				return true
			})
		}
	}

	if found != 2 {
		t.Errorf("found %d collecting loops, want exactly 2 (GetPipelineRuns and GetReleases); review any new one for the Name/Id guard", found)
	}
}

// appendsIdent reports whether body contains an append call that passes the
// bare identifier name as one of the appended values.
func appendsIdent(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun, ok := call.Fun.(*ast.Ident)
		if !ok || fun.Name != "append" {
			return true
		}
		for _, arg := range call.Args[1:] {
			if id, ok := arg.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		}
		return true
	})
	return found
}

// derefsIDAfter reports whether body contains a `*X.Id` dereference positioned
// after pos.
func derefsIDAfter(body *ast.BlockStmt, pos token.Pos) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		star, ok := n.(*ast.StarExpr)
		if !ok || star.Pos() <= pos {
			return true
		}
		if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Id" {
			found = true
		}
		return true
	})
	return found
}

// guardsNameAndID reports whether body opens with an if whose body is a single
// continue and whose condition, flattened over ||, contains both
// `name.Name == nil` and `name.Id == nil`.
func guardsNameAndID(body *ast.BlockStmt, name string) bool {
	if len(body.List) == 0 {
		return false
	}
	guard, ok := body.List[0].(*ast.IfStmt)
	if !ok || len(guard.Body.List) != 1 {
		return false
	}
	if br, ok := guard.Body.List[0].(*ast.BranchStmt); !ok || br.Tok != token.CONTINUE {
		return false
	}

	var operands []ast.Expr
	var flatten func(ast.Expr)
	flatten = func(e ast.Expr) {
		if bin, ok := e.(*ast.BinaryExpr); ok && bin.Op == token.LOR {
			flatten(bin.X)
			flatten(bin.Y)
			return
		}
		operands = append(operands, e)
	}
	flatten(guard.Cond)

	checked := map[string]bool{}
	for _, operand := range operands {
		bin, ok := operand.(*ast.BinaryExpr)
		if !ok || bin.Op != token.EQL {
			continue
		}
		sel, ok := bin.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != name {
			continue
		}
		if nilIdent, ok := bin.Y.(*ast.Ident); ok && nilIdent.Name == "nil" {
			checked[sel.Sel.Name] = true
		}
	}
	return checked["Name"] && checked["Id"]
}
