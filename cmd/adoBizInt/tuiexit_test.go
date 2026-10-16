package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestTUIWaitsForInflightWriteBeforeExit pins the main.go wiring that closes
// CR-01 (GUARD-01, GUARD-02): a TUI write interrupted by SIGTERM, a non-TTY
// SIGINT or a panic bubbletea recovers is audited before the process exits.
// The assertions are over the AST of run's top-level statements, so a comment
// can neither satisfy nor defeat them (STATE.md F-03-3 precedent). Each rule
// exists for one reason:
//
//  1. Exactly one statement runs the program (p.Run), and it holds no return:
//     the wait must be reachable on every path out of Run.
//  2. Exactly one statement calls confirm.Wait, after the Run statement, with
//     no return between them: Run returns an error after a panic it
//     recovered, which is when another goroutine may be mid-write, so the
//     error branch must not skip the wait.
//  3. Exactly one statement calls confirm.SetParent with one identifier,
//     before Run: a write that starts before the parent is installed runs
//     under a parent no signal cancels, and its interrupt goes unaudited.
//  4. That identifier is defined, before SetParent, by a top-level
//     signal.NotifyContext over os.Interrupt and syscall.SIGTERM: those are
//     the two signals bubbletea turns into a quit the model never sees.
//  5. A top-level defer calls the stop func NotifyContext returned: it keeps
//     both signals captured through the wait, so a second SIGTERM cannot kill
//     the process mid-append, and it cancels the parent as run returns, so a
//     write that starts late is refused at its pre-read (T-07-38).
func TestTUIWaitsForInflightWriteBeforeExit(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var run *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "run" {
			run = fn
		}
	}
	if run == nil {
		t.Fatalf("func run not found in main.go")
	}
	stmts := run.Body.List

	// selector reports whether expr is recv.name with recv an identifier; an
	// empty recv matches any identifier receiver.
	selector := func(expr ast.Expr, recv, name string) bool {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != name {
			return false
		}
		x, ok := sel.X.(*ast.Ident)
		return ok && (recv == "" || x.Name == recv)
	}
	hasReturn := func(stmt ast.Stmt) bool {
		found := false
		ast.Inspect(stmt, func(n ast.Node) bool {
			if _, ok := n.(*ast.ReturnStmt); ok {
				found = true
			}
			return !found
		})
		return found
	}

	// only returns the one top-level statement of run whose subtree holds a
	// call of recv.name, and that call, failing unless exactly one does.
	only := func(rule, recv, name string) (int, *ast.CallExpr) {
		t.Helper()
		at, found := -1, (*ast.CallExpr)(nil)
		for i, stmt := range stmts {
			var hit *ast.CallExpr
			ast.Inspect(stmt, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && selector(call.Fun, recv, name) {
					hit = call
				}
				return hit == nil
			})
			if hit == nil {
				continue
			}
			if at != -1 {
				t.Fatalf("%s: %s is called in more than one statement of run", rule, name)
			}
			at, found = i, hit
		}
		if at == -1 {
			t.Fatalf("%s: %s is not called in run; the check would be vacuous", rule, name)
		}
		return at, found
	}

	runAt, _ := only("rule 1", "", "Run")
	if hasReturn(stmts[runAt]) {
		t.Errorf("rule 1: the p.Run statement (%d) holds a return, so a path out of Run can skip confirm.Wait", runAt)
	}

	waitAt, _ := only("rule 2", "confirm", "Wait")
	if waitAt <= runAt {
		t.Errorf("rule 2: confirm.Wait is statement %d, p.Run %d; the wait must follow Run", waitAt, runAt)
	}
	for i := runAt + 1; i < waitAt; i++ {
		if hasReturn(stmts[i]) {
			t.Errorf("rule 2: statement %d between p.Run (%d) and confirm.Wait (%d) holds a return, so that path exits without waiting for an in-flight write", i, runAt, waitAt)
		}
	}

	setAt, setCall := only("rule 3", "confirm", "SetParent")
	if setAt >= runAt {
		t.Errorf("rule 3: confirm.SetParent is statement %d, p.Run %d; the parent must be installed before Run", setAt, runAt)
	}
	if len(setCall.Args) != 1 {
		t.Fatalf("rule 3: confirm.SetParent has %d args, want 1", len(setCall.Args))
	}
	parent, ok := setCall.Args[0].(*ast.Ident)
	if !ok {
		t.Fatalf("rule 3: confirm.SetParent's argument is %T, want an identifier", setCall.Args[0])
	}

	stop := ""
	for _, stmt := range stmts[:setAt] {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
			continue
		}
		ctxID, ok1 := assign.Lhs[0].(*ast.Ident)
		stopID, ok2 := assign.Lhs[1].(*ast.Ident)
		call, ok3 := assign.Rhs[0].(*ast.CallExpr)
		if !ok1 || !ok2 || !ok3 || ctxID.Name != parent.Name || !selector(call.Fun, "signal", "NotifyContext") {
			continue
		}
		interrupt, sigterm := false, false
		for _, arg := range call.Args {
			interrupt = interrupt || selector(arg, "os", "Interrupt")
			sigterm = sigterm || selector(arg, "syscall", "SIGTERM")
		}
		if interrupt && sigterm {
			stop = stopID.Name
		}
	}
	if stop == "" {
		t.Fatalf("rule 4: no top-level %s, stop := signal.NotifyContext(..., os.Interrupt, syscall.SIGTERM) precedes confirm.SetParent", parent.Name)
	}

	deferred := false
	for _, stmt := range stmts {
		if d, ok := stmt.(*ast.DeferStmt); ok {
			if id, ok := d.Call.Fun.(*ast.Ident); ok && id.Name == stop {
				deferred = true
			}
		}
	}
	if !deferred {
		t.Fatalf("rule 5: no top-level defer %s(); the signals must stay captured through confirm.Wait", stop)
	}
}
