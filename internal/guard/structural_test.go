package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRunRecoverPrecedesMutate pins Phase 4 D-12's boundary structurally. It
// was TestWriteRecoverPrecedesMutate in cmd/adoBizInt until the guarded-write
// body moved here (GUARD-01). The recover's POSITION is the request-issued
// signal: it must be the statement immediately before op.Mutate, and it must
// follow defer sink.Close() so that, with defers running last-in first-out, the
// sink is still open when it runs.
//
// The recover must append the best-effort unknown record and must never
// re-panic: the guard returns a panic as data so neither surface can be
// terminated mid-write by the guard itself.
func TestRunRecoverPrecedesMutate(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "guard.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse guard.go: %v", err)
	}

	var run *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "Run" {
			run = fn
		}
	}
	if run == nil {
		t.Fatalf("func Run not found in guard.go")
	}

	isSelector := func(e ast.Expr, x, sel string) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok || s.Sel.Name != sel {
			return false
		}
		id, ok := s.X.(*ast.Ident)
		return ok && id.Name == x
	}
	isIdentCall := func(name string) func(*ast.CallExpr) bool {
		return func(c *ast.CallExpr) bool {
			id, ok := c.Fun.(*ast.Ident)
			return ok && id.Name == name
		}
	}
	contains := func(n ast.Node, match func(*ast.CallExpr) bool) bool {
		hit := false
		ast.Inspect(n, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && match(call) {
				hit = true
			}
			return !hit
		})
		return hit
	}

	iClose, iRecover, iMutate := -1, -1, -1
	var recoverLit *ast.FuncLit
	set := func(what string, idx *int, i int) {
		if *idx != -1 {
			t.Fatalf("%s found in more than one statement of Run", what)
		}
		*idx = i
	}
	for i, stmt := range run.Body.List {
		if d, ok := stmt.(*ast.DeferStmt); ok {
			if isSelector(d.Call.Fun, "sink", "Close") {
				set("defer sink.Close()", &iClose, i)
			}
			if lit, ok := d.Call.Fun.(*ast.FuncLit); ok && contains(lit, isIdentCall("recover")) {
				set("deferred recover", &iRecover, i)
				recoverLit = lit
			}
		}
		if contains(stmt, func(c *ast.CallExpr) bool { return isSelector(c.Fun, "op", "Mutate") }) {
			set("op.Mutate", &iMutate, i)
		}
	}
	if iClose == -1 || iRecover == -1 || iMutate == -1 {
		t.Fatalf("defer sink.Close() at %d, deferred recover at %d, op.Mutate at %d; each must be found", iClose, iRecover, iMutate)
	}
	if iClose >= iRecover {
		t.Errorf("deferred recover (%d) must follow defer sink.Close() (%d) so it runs while the sink is open", iRecover, iClose)
	}
	if iRecover != iMutate-1 {
		t.Errorf("deferred recover is statement %d, op.Mutate %d; the recover must be immediately before the mutate", iRecover, iMutate)
	}
	if !contains(recoverLit, isIdentCall("appendAuditRecord")) {
		t.Errorf("the deferred recover must call appendAuditRecord (D-13 best effort)")
	}
	if contains(recoverLit, isIdentCall("panic")) {
		t.Errorf("the deferred recover must never re-panic; a panic is returned as Report.Panic")
	}
}

// TestGuardNeverPrintsOrExits pins GUARD-01's surface contract: the guard never
// writes stdout and never exits, because the CLI's stdout is one parsed document
// and the TUI owns the terminal. It is an AST walk, so neither a comment nor a
// string literal can satisfy or defeat it. The runtime half is
// TestRunNeverWritesStdout.
func TestGuardNeverPrintsOrExits(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package files: %v", err)
	}
	scanned := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		scanned++
		for _, finding := range forbiddenOutput(fset, file) {
			t.Errorf("%s", finding)
		}
	}
	if scanned < 3 {
		t.Fatalf("scanned %d non-test files, want at least guard.go, audit.go and ops.go", scanned)
	}

	t.Run("tripwire", func(t *testing.T) {
		src := `package snippet

import (
	"fmt"
	stdlog "log"
	"os"
)

func bad() {
	os.Exit(1)
	_ = os.Stdout
	fmt.Print("x")
	fmt.Printf("x")
	fmt.Println("x")
	stdlog.Fatal("x")
	stdlog.Fatalf("x")
	stdlog.Panicln("x")
	print("x")
	println("x")
}
`
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "snippet.go", src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse snippet: %v", err)
		}
		if got := forbiddenOutput(fset, file); len(got) != 10 {
			t.Errorf("checker found %d forms, want one per forbidden form (10): %v", len(got), got)
		}
	})
}

// forbiddenOutput reports every os.Exit, os.Stdout, fmt.Print*, log.Fatal* and
// log.Panic* selector, and every print/println builtin call, in file. Package
// selectors are resolved through the file's imports, so an alias cannot hide one.
func forbiddenOutput(fset *token.FileSet, file *ast.File) []string {
	pkgOf := map[string]string{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		pkgOf[local] = path
	}

	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			x, ok := n.X.(*ast.Ident)
			if !ok {
				return true
			}
			sel := n.Sel.Name
			forbidden := false
			switch pkgOf[x.Name] {
			case "os":
				forbidden = sel == "Exit" || sel == "Stdout"
			case "fmt":
				forbidden = sel == "Print" || sel == "Printf" || sel == "Println"
			case "log":
				forbidden = strings.HasPrefix(sel, "Fatal") || strings.HasPrefix(sel, "Panic")
			}
			if forbidden {
				findings = append(findings, fset.Position(n.Pos()).String()+": "+x.Name+"."+sel)
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				findings = append(findings, fset.Position(n.Pos()).String()+": "+id.Name)
			}
		}
		return true
	})
	return findings
}
