package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// GUARD-02: no file under internal/ui may call an internal/ado mutation
// directly; every TUI write goes through the guard. The tests live in package
// guard because the property is about the guard: nothing in the UI bypasses
// it. They are AST-based so that a comment or a string naming a method can
// neither trip nor satisfy them (STATE.md records three Phase 3 grep gates that
// prose defeated).

// readOnlyClientMethod reports whether an exported *ado.Client method is a
// read. Everything else is treated as a mutation, so a mutation added to
// internal/ado later is forbidden in internal/ui without touching this test.
func readOnlyClientMethod(name string) bool {
	for _, prefix := range []string{"Get", "Current", "Fetch", "Download"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return name == "Projects" || name == "Pipelines" || name == "ReleaseDefinitions"
}

// adoMutations derives the forbidden set from the exported methods declared on
// *Client in the non-test files of ../ado.
func adoMutations(t *testing.T) map[string]bool {
	t.Helper()
	names, err := filepath.Glob(filepath.Join("..", "ado", "*.go"))
	if err != nil {
		t.Fatalf("glob ../ado: %v", err)
	}
	fset := token.NewFileSet()
	forbidden := map[string]bool{}
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
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || !fn.Name.IsExported() {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			if id, ok := star.X.(*ast.Ident); ok && id.Name == "Client" && !readOnlyClientMethod(fn.Name.Name) {
				forbidden[fn.Name.Name] = true
			}
		}
	}
	return forbidden
}

// parseUIFiles parses every non-test .go file under ../ui, keyed by its path
// relative to ../ui.
func parseUIFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	root := filepath.Join("..", "ui")
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = file
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return fset, files
}

// mutationCalls reports, as file:line, every call in file whose function is a
// selector naming a method in forbidden. The one exception is a call through
// this package's own import: guard.ApproveRelease, guard.QueuePipeline and
// friends are the op constructors that route a write through the guard, and
// they share their names with the ado methods they wrap.
func mutationCalls(fset *token.FileSet, file *ast.File, forbidden map[string]bool) []string {
	guardName := ""
	for _, imp := range file.Imports {
		if imp.Path.Value != `"sentinovo.ai/bizInt/internal/guard"` {
			continue
		}
		guardName = "guard"
		if imp.Name != nil {
			guardName = imp.Name.Name
		}
	}
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !forbidden[sel.Sel.Name] {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && guardName != "" && pkg.Name == guardName {
			return true
		}
		found = append(found, fset.Position(call.Pos()).String()+": "+sel.Sel.Name)
		return true
	})
	return found
}

func TestNoUIFileCallsADOMutation(t *testing.T) {
	forbidden := adoMutations(t)

	// Tripwire: the derived set must hold every mutation known today, or the
	// derivation (not the UI) is what is broken.
	for _, name := range []string{
		"ApprovePullRequest", "ApproveRelease", "CompletePullRequest", "QueuePipeline",
		"RejectRelease", "UpdateWorkItemState", "UpdateWorkItemStateWithEffort",
	} {
		if !forbidden[name] {
			t.Errorf("derived forbidden set %v lacks %s", forbidden, name)
		}
	}

	fset, files := parseUIFiles(t)
	if len(files) == 0 {
		t.Fatalf("found no .go files under ../ui")
	}
	for _, file := range files {
		for _, call := range mutationCalls(fset, file, forbidden) {
			t.Errorf("%s: a direct internal/ado mutation; route it through guard.Run in confirm/batch.go (GUARD-02)", call)
		}
	}

	// Tripwire: the scanner itself must find calls, so an empty result above
	// means the UI is clean rather than that the scanner sees nothing.
	// The comment, the string and the guard op constructor must not count.
	src := `package ui

import "sentinovo.ai/bizInt/internal/guard"

// c.ApprovePullRequest(ctx) in a comment is not a call.
func f(c client) {
	_ = c.ApprovePullRequest(ctx, "p", "r", 1)
	_ = c.UpdateWorkItemStateWithEffort(ctx, "p", 1, "Closed", nil, nil, nil)
	_ = "c.QueuePipeline(ctx)"
	_ = guard.ApproveRelease(c, "p", 1, "")
}
`
	snippetSet := token.NewFileSet()
	snippet, err := parser.ParseFile(snippetSet, "snippet.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if got := mutationCalls(snippetSet, snippet, forbidden); len(got) != 2 {
		t.Errorf("scanner found %v in the tripwire snippet, want 2 findings", got)
	}
}

// GUARD-04 for the TUI: every guard.Run call under internal/ui is in
// confirm/batch.go and names the TUI as the actor.
func TestUIGuardCallsCarryTUIActor(t *testing.T) {
	isGuardSelector := func(e ast.Expr, name string) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != name {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == "guard"
	}

	fset, files := parseUIFiles(t)
	calls := 0
	for name, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isGuardSelector(call.Fun, "Run") {
				return true
			}
			calls++
			pos := fset.Position(call.Pos())
			if name != "confirm/batch.go" {
				t.Errorf("%s: guard.Run outside confirm/batch.go", pos)
			}
			if len(call.Args) != 3 {
				t.Errorf("%s: guard.Run has %d args, want 3", pos, len(call.Args))
				return true
			}
			lit, ok := call.Args[2].(*ast.CompositeLit)
			if !ok {
				t.Errorf("%s: guard.Run options are %T, want a guard.Options literal", pos, call.Args[2])
				return true
			}
			tui := false
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Actor" && isGuardSelector(kv.Value, "ActorTUI") {
					tui = true
				}
			}
			if !tui {
				t.Errorf("%s: guard.Run options lack Actor: guard.ActorTUI (GUARD-04)", pos)
			}
			return true
		})
	}
	if calls == 0 {
		t.Fatalf("found no guard.Run call under ../ui; the TUI's guarded writes must go through confirm/batch.go")
	}
}
