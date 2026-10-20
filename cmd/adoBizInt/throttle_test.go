package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"
)

// TestThrottleSummary pins the D-05/D-06 stderr line: nothing when the run was
// neither paced nor retried, otherwise exactly one line with singular nouns
// for exactly 1 and the total in whole seconds.
func TestThrottleSummary(t *testing.T) {
	tests := []struct {
		name    string
		waits   int64
		waited  time.Duration
		retries int64
		want    string
	}{
		{"neither paced nor retried", 0, 0, 0, ""},
		{"paced and retried once", 4, 31 * time.Second, 1, "throttled by ADO: 4 waits, 31s total, 1 retry\n"},
		{"one wait, rounded down", 1, 7400 * time.Millisecond, 0, "throttled by ADO: 1 wait, 7s total, 0 retries\n"},
		{"retried without a wait", 0, 0, 2, "throttled by ADO: 0 waits, 0s total, 2 retries\n"},
		{"half a second rounds up", 2, 31500 * time.Millisecond, 0, "throttled by ADO: 2 waits, 32s total, 0 retries\n"},
		{"over a minute stays in seconds", 3, 90 * time.Second, 0, "throttled by ADO: 3 waits, 90s total, 0 retries\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer
			renderThrottleSummary(&b, tt.waits, tt.waited, tt.retries)
			if got := b.String(); got != tt.want {
				t.Errorf("renderThrottleSummary(%d, %v, %d) wrote %q, want %q", tt.waits, tt.waited, tt.retries, got, tt.want)
			}
		})
	}
}

// TestThrottleSummaryOnlyOnCLIPath pins D-07 over the AST of main.go: the one
// top-level statement of run that calls renderThrottleSummary is the statement
// that calls dispatch, which returns, so the TUI path after it can never
// print the line onto the alt-screen; and the summary is rendered after
// dispatch, once the counters are final. A comment can neither satisfy nor
// defeat it (STATE.md F-03-3).
func TestThrottleSummaryOnlyOnCLIPath(t *testing.T) {
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

	// callIn returns the call of the plain function name inside stmt, or nil.
	callIn := func(stmt ast.Stmt, name string) *ast.CallExpr {
		var hit *ast.CallExpr
		ast.Inspect(stmt, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == name {
					hit = call
				}
			}
			return hit == nil
		})
		return hit
	}

	renderAt := -1
	var render, dispatchCall *ast.CallExpr
	for i, stmt := range run.Body.List {
		call := callIn(stmt, "renderThrottleSummary")
		if call == nil {
			continue
		}
		if renderAt != -1 {
			t.Fatalf("renderThrottleSummary is called in more than one statement of run")
		}
		renderAt, render = i, call
		dispatchCall = callIn(stmt, "dispatch")
	}
	if render == nil {
		t.Fatalf("renderThrottleSummary is not called in run; the D-07 check would be vacuous")
	}
	if dispatchCall == nil {
		t.Fatalf("statement %d of run calls renderThrottleSummary but not dispatch; the summary must live on the CLI path only", renderAt)
	}
	if render.Pos() < dispatchCall.Pos() {
		t.Errorf("renderThrottleSummary is called before dispatch; the counters are only final after the command has run")
	}
}
