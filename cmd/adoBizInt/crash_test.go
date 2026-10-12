package main

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
)

// cancelledParent returns a context that is already done, standing in for a
// SIGINT that arrived during the read. Driving cancellation through an injected
// context rather than a real signal keeps the test hermetic: a test that raises
// SIGINT at itself is at the mercy of the runner's own signal handling.
func cancelledParent() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// interruptedClient returns a fake whose read observes the cancellation and
// reports it the way a real fan-out read does.
func interruptedClient(t *testing.T) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(ctx context.Context, _ []string) (ado.Result[ado.PullRequest], error) {
			return ado.Result[ado.PullRequest]{}, ctx.Err()
		},
	}
}

// decodeCrash unmarshals stdout into a generic map so the test can assert on
// the exact top-level key set rather than on a struct that would silently
// tolerate extra keys.
func decodeCrash(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("stdout %q is not a single JSON document: %v", raw, err)
	}
	return top
}

// TestInterruptExitsThree pins D-25. Not 130: a fifth code outside the
// published four would force every caller to learn a second convention. Not 0:
// a script interrupting its own child could not then tell "interrupted, no
// data" from "clean read, nothing matched".
func TestInterruptExitsThree(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	captureOutput(t)

	code := dispatch(cancelledParent(), interruptedClient(t), []string{"prs", "list", "--json"})
	if code != exitUnknown {
		t.Errorf("interrupted dispatch = %d, want %d", code, exitUnknown)
	}
}

// TestInterruptEmitsDistinctErrorObject is the D-26 assertion. The record must
// not be representable as a normal read outcome: an agent distinguishes a crash
// from a result by which top-level key is present, so a synthetic empty
// envelope would be read as real data.
func TestInterruptEmitsDistinctErrorObject(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	dispatch(cancelledParent(), interruptedClient(t), []string{"prs", "list", "--json"})

	top := decodeCrash(t, stdout.Bytes())
	if len(top) != 1 {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	if _, ok := top["error"]; !ok {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	for _, forbidden := range []string{"items", "failures", "complete"} {
		if _, ok := top[forbidden]; ok {
			t.Errorf("top-level key %q present; a crash must never look like a result envelope", forbidden)
		}
	}
}

// TestInterruptRecordShape pins the published fields: kind, a message, and the
// log path. No stack — that is Task 3's credential-disclosure constraint and it
// applies to every crash record, not only the panic one.
func TestInterruptRecordShape(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	dispatch(cancelledParent(), interruptedClient(t), []string{"prs", "list", "--json"})

	var env struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
			LogPath string `json:"log_path"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout %q does not decode: %v", stdout.String(), err)
	}
	if env.Error.Kind != "interrupt" {
		t.Errorf("kind = %q, want %q", env.Error.Kind, "interrupt")
	}
	if env.Error.Message == "" {
		t.Errorf("message is empty; the record must say what happened")
	}
	if env.Error.LogPath == "" {
		t.Errorf("log_path is empty; the record must name where the detail went")
	}

	inner := decodeCrash(t, stdout.Bytes())["error"]
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(inner, &fields); err != nil {
		t.Fatalf("error object does not decode: %v", err)
	}
	for _, forbidden := range []string{"stack", "stack_trace", "goroutines"} {
		if _, ok := fields[forbidden]; ok {
			t.Errorf("record carries %q; a stack prints function arguments and would leak the credential", forbidden)
		}
	}
}

// TestInterruptRecordNotEmittedOnCleanRead guards the other direction: an
// uninterrupted read must produce the ordinary envelope and no crash record.
func TestInterruptRecordNotEmittedOnCleanRead(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	code := dispatch(context.Background(), emptyReadClient(t), []string{"prs", "list", "--json"})
	if code != exitOK {
		t.Fatalf("clean dispatch = %d, want %d", code, exitOK)
	}
	top := decodeCrash(t, stdout.Bytes())
	if _, ok := top["error"]; ok {
		t.Errorf("clean read emitted a crash record: %s", stdout.String())
	}
	if _, ok := top["items"]; !ok {
		t.Errorf("clean read did not emit the result envelope: %s", stdout.String())
	}
}

// keysOf renders a decoded map's key set for failure messages.
func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// patFixture stands in for a real credential. Assertions search stdout and
// stderr for it, so a stack trace rendering a config argument would be caught
// rather than merely forbidden by comment.
const patFixture = "pat-fixture-do-not-disclose-9f3c1a"

// panickingClient closes over a configuration value carrying the credential and
// panics inside the read. The closure matters: a Go stack trace prints function
// arguments, so if the recovery handler ever put a stack on stdout this fixture
// is what it would disclose.
func panickingClient(t *testing.T) *fakeReadClient {
	t.Helper()
	cfg := &config.Config{PAT: patFixture, OrgURL: "https://dev.azure.com/fixture"}
	return &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(context.Context, []string) (ado.Result[ado.PullRequest], error) {
			panic("deliberate panic reading " + cfg.OrgURL)
		},
	}
}

// TestPanicExitsThree pins that a panic inside a read is recovered and mapped to
// exit 3 (nothing ran), rather than killing the process with the runtime's own
// exit status and an unstructured dump.
func TestPanicExitsThree(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	captureOutput(t)

	code := guard(func() int {
		return dispatch(context.Background(), panickingClient(t), []string{"prs", "list", "--json"})
	})
	if code != exitUnknown {
		t.Errorf("guarded panic = %d, want %d", code, exitUnknown)
	}
}

// TestPanicEmitsDistinctErrorObject is OUT-05 stated as an assertion: a panic
// must not look like an empty result.
func TestPanicEmitsDistinctErrorObject(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	guard(func() int {
		return dispatch(context.Background(), panickingClient(t), []string{"prs", "list", "--json"})
	})

	top := decodeCrash(t, stdout.Bytes())
	if len(top) != 1 {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	if _, ok := top["error"]; !ok {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	if _, ok := top["items"]; ok {
		t.Errorf("panic produced an items key; a crash must never be readable as data")
	}

	var env crashEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout does not decode: %v", err)
	}
	if env.Error.Kind != "panic" {
		t.Errorf("kind = %q, want %q", env.Error.Kind, "panic")
	}
	if !strings.Contains(env.Error.Message, "deliberate panic") {
		t.Errorf("message = %q, want the panic value rendered as a string", env.Error.Message)
	}
	if env.Error.LogPath == "" {
		t.Errorf("log_path is empty; the record must name where the stack went")
	}
}

// TestPanicRecordCarriesNoStackOrCredential is the T-03-14 assertion.
func TestPanicRecordCarriesNoStackOrCredential(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	guard(func() int {
		return dispatch(context.Background(), panickingClient(t), []string{"prs", "list", "--json"})
	})

	out := stdout.String()
	if strings.Contains(out, patFixture) {
		t.Errorf("stdout discloses the credential fixture: %s", out)
	}
	for _, marker := range []string{"goroutine ", ".go:", "runtime."} {
		if strings.Contains(out, marker) {
			t.Errorf("stdout looks like it carries a stack (%q): %s", marker, out)
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(decodeCrash(t, stdout.Bytes())["error"], &fields); err != nil {
		t.Fatalf("error object does not decode: %v", err)
	}
	if _, ok := fields["stack"]; ok {
		t.Errorf("record carries a stack field")
	}
	if len(fields) != 3 {
		t.Errorf("record fields = %v, want exactly kind, message and log_path", keysOf(fields))
	}
}

// TestPanicWithFailedLoggingWritesNoStackToStderr closes the residual path in
// T-03-14. When logging init fails the package logger falls back to the default
// one, which writes to stderr — so an unconditional stack write would print the
// credential-bearing frames to the terminal. The stack write is gated on a
// successful init; the JSON record on stdout is identical either way.
func TestPanicWithFailedLoggingWritesNoStackToStderr(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "home-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("HOME", notADir)

	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	// slog's fallback writes to the process stream, not the injected writer, so
	// only a process-level capture can prove nothing reached it.
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	prevStderr := os.Stderr
	os.Stderr = writeEnd
	stderrWriter = writeEnd

	initLogging()
	code := guard(func() int {
		return dispatch(context.Background(), panickingClient(t), []string{"prs", "list", "--json"})
	})

	os.Stderr = prevStderr
	if err := writeEnd.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	captured, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	if code != exitUnknown {
		t.Errorf("guarded panic = %d, want %d", code, exitUnknown)
	}
	if bytes.Contains(captured, []byte(patFixture)) {
		t.Errorf("stderr discloses the credential fixture: %s", captured)
	}
	for _, marker := range []string{"goroutine ", "runtime.gopanic"} {
		if bytes.Contains(captured, []byte(marker)) {
			t.Errorf("stderr carries a stack (%q): %s", marker, captured)
		}
	}

	// The structured record is unchanged by the logging failure.
	var env crashEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout %q does not decode: %v", stdout.String(), err)
	}
	if env.Error.Kind != "panic" {
		t.Errorf("kind = %q, want %q", env.Error.Kind, "panic")
	}
}

// TestGuardPassesThroughWhenNothingPanics keeps the guard honest: it must not
// alter the return value of an ordinary run.
func TestGuardPassesThroughWhenNothingPanics(t *testing.T) {
	if got := guard(func() int { return exitPartial }); got != exitPartial {
		t.Errorf("guard(returns %d) = %d, want %d", exitPartial, got, exitPartial)
	}
}

// TestPanicAfterWriteIssuedExitsOutcomeUnknown pins D-12: once a write's
// request may have left, a panic must not report exit 3 ("nothing ran"). It is
// outcome unknown (exit 5), the same answer Phase 4 D-22 gives an interrupt in
// that window. The crash record still goes to stdout as the only document
// (D-18): no write envelope precedes it.
func TestPanicAfterWriteIssuedExitsOutcomeUnknown(t *testing.T) {
	withConfiguredProjects(t, "P")
	tempAuditSink(t)
	stdout, stderr := captureOutput(t)

	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error {
		panic("deliberate panic mid-write")
	}

	code := guard(func() int { return dispatch(context.Background(), c, setStateArgs("--confirm")) })
	if code != exitOutcomeUnknown {
		t.Fatalf("post-request panic = %d, want %d (stderr: %q)", code, exitOutcomeUnknown, stderr.String())
	}

	top := decodeCrash(t, stdout.Bytes())
	if _, ok := top["error"]; !ok || len(top) != 1 {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	var env crashEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout does not decode: %v", err)
	}
	if env.Error.Kind != crashKindPanic {
		t.Errorf("kind = %q, want %q", env.Error.Kind, crashKindPanic)
	}
	for _, want := range []string{"deliberate panic mid-write", "outcome unknown"} {
		if !strings.Contains(env.Error.Message, want) {
			t.Errorf("message = %q, want it to contain %q", env.Error.Message, want)
		}
	}
	if env.Error.LogPath == "" {
		t.Errorf("log_path is empty; the record must name where the stack went")
	}
	if n := strings.Count(stdout.String(), "\n"); n != 1 {
		t.Errorf("stdout holds %d newlines, want exactly one document: %q", n, stdout.String())
	}
}

// TestPanicBeforeWriteIssuedExitsThree is the D-12 lower boundary: a panic in
// the pre-read happens before any request could have changed ADO, so it still
// exits 3 (nothing ran) and leaves no audit record.
func TestPanicBeforeWriteIssuedExitsThree(t *testing.T) {
	withConfiguredProjects(t, "P")
	sink := tempAuditSink(t)
	stdout, _ := captureOutput(t)

	c := preReadFake(t, "Active")
	c.GetWorkItemDetailFunc = func(context.Context, int) (*ado.WorkItemDetail, error) {
		panic("deliberate panic in pre-read")
	}

	code := guard(func() int { return dispatch(context.Background(), c, setStateArgs("--confirm")) })
	if code != exitUnknown {
		t.Fatalf("pre-request panic = %d, want %d", code, exitUnknown)
	}

	top := decodeCrash(t, stdout.Bytes())
	if _, ok := top["error"]; !ok || len(top) != 1 {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	var env crashEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout does not decode: %v", err)
	}
	if env.Error.Kind != crashKindPanic {
		t.Errorf("kind = %q, want %q", env.Error.Kind, crashKindPanic)
	}
	assertAuditLineCount(t, sink, 0)
}

// TestWriteRecoverPrecedesMutate pins D-12's boundary structurally. The
// recover's POSITION is the request-issued signal: it must be the statement
// immediately before op.mutate, and it must follow defer sink.Close() so that,
// with defers running last-in first-out, the sink is still open when it runs.
//
// note: this is the second inline parser.ParseFile in package main, after
// TestValidateGuardsCLIAndTUI in main_test.go; a shared helper waits for a
// third use (rule of three).
func TestWriteRecoverPrecedesMutate(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "write.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse write.go: %v", err)
	}

	var runWrite *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "runWrite" {
			runWrite = fn
		}
	}
	if runWrite == nil {
		t.Fatalf("func runWrite not found in write.go")
	}

	isSelector := func(e ast.Expr, x, sel string) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok || s.Sel.Name != sel {
			return false
		}
		id, ok := s.X.(*ast.Ident)
		return ok && id.Name == x
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
			t.Fatalf("%s found in more than one statement of runWrite", what)
		}
		*idx = i
	}
	for i, stmt := range runWrite.Body.List {
		if d, ok := stmt.(*ast.DeferStmt); ok {
			if isSelector(d.Call.Fun, "sink", "Close") {
				set("defer sink.Close()", &iClose, i)
			}
			if lit, ok := d.Call.Fun.(*ast.FuncLit); ok && contains(lit, func(c *ast.CallExpr) bool {
				id, ok := c.Fun.(*ast.Ident)
				return ok && id.Name == "recover"
			}) {
				set("deferred recover", &iRecover, i)
				recoverLit = lit
			}
		}
		if contains(stmt, func(c *ast.CallExpr) bool { return isSelector(c.Fun, "op", "mutate") }) {
			set("op.mutate", &iMutate, i)
		}
	}
	if iClose == -1 || iRecover == -1 || iMutate == -1 {
		t.Fatalf("defer sink.Close() at %d, deferred recover at %d, op.mutate at %d; each must be found", iClose, iRecover, iMutate)
	}
	if iClose >= iRecover {
		t.Errorf("deferred recover (%d) must follow defer sink.Close() (%d) so it runs while the sink is open", iRecover, iClose)
	}
	if iRecover != iMutate-1 {
		t.Errorf("deferred recover is statement %d, op.mutate %d; the recover must be immediately before the mutate", iRecover, iMutate)
	}

	if !contains(recoverLit, func(c *ast.CallExpr) bool {
		id, ok := c.Fun.(*ast.Ident)
		if !ok || id.Name != "handlePanic" || len(c.Args) != 2 {
			return false
		}
		code, ok := c.Args[1].(*ast.Ident)
		return ok && code.Name == "exitOutcomeUnknown"
	}) {
		t.Errorf("the deferred recover must call handlePanic(..., exitOutcomeUnknown)")
	}
}

// panicOnFirstWrite panics on its first Write and forwards every later Write to
// out. Swapped in as stdoutWriter, it panics while the write report is being
// emitted, after the normal audit append, and still captures the crash record.
type panicOnFirstWrite struct {
	out     *bytes.Buffer
	tripped bool
}

func (w *panicOnFirstWrite) Write(p []byte) (int, error) {
	if !w.tripped {
		w.tripped = true
		panic("deliberate panic emitting the write report")
	}
	return w.out.Write(p)
}

// TestPanicAfterWriteIssuedIsAudited pins D-13 and SAFE-02: every write that may
// have landed leaves a trace, even when the process panics, and never two. The
// best-effort record carries the write's own fields only, never the panic text,
// so the five-key D-15 shape and the T-04-01 no-argv rule hold.
func TestPanicAfterWriteIssuedIsAudited(t *testing.T) {
	panicMutate := func(context.Context, string, int, string) error { panic("deliberate panic mid-write") }

	cases := []struct {
		name        string
		ctx         context.Context
		failSink    bool
		mutate      func(context.Context, string, int, string) error
		panicStdout bool
		wantLines   int
		wantOutcome string
		wantStderr  string
	}{
		{name: "mutate panics", ctx: context.Background(), mutate: panicMutate, wantLines: 1, wantOutcome: outcomeUnknown},
		{name: "append fails, still unknown", ctx: context.Background(), failSink: true, mutate: panicMutate, wantStderr: "Write may have landed but was NOT recorded"},
		{
			name: "panic after the normal append writes no second record", ctx: context.Background(),
			mutate:      func(context.Context, string, int, string) error { return nil },
			panicStdout: true, wantLines: 1, wantOutcome: outcomeExecuted,
		},
		{name: "cancelled parent still unknown", ctx: cancelledParent(), mutate: panicMutate, wantLines: 1, wantOutcome: outcomeUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withConfiguredProjects(t, "P")
			sink := tempAuditSink(t)
			if tc.failSink {
				failingAuditSink(t)
			}
			stdout, stderr := captureOutput(t)
			if tc.panicStdout {
				prev := stdoutWriter
				stdoutWriter = &panicOnFirstWrite{out: stdout}
				t.Cleanup(func() { stdoutWriter = prev })
			}

			c := preReadFake(t, "Active")
			c.UpdateWorkItemStateFunc = tc.mutate

			code := guard(func() int { return dispatch(tc.ctx, c, setStateArgs("--confirm")) })
			if code != exitOutcomeUnknown {
				t.Fatalf("exit = %d, want %d (stderr: %q)", code, exitOutcomeUnknown, stderr.String())
			}

			var env crashEnvelope
			if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
				t.Fatalf("stdout %q is not one crash document: %v", stdout.String(), err)
			}
			if top := decodeCrash(t, stdout.Bytes()); len(top) != 1 || env.Error.Kind != crashKindPanic {
				t.Errorf("stdout = %s, want exactly one panic crash record", stdout.String())
			}
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
			if tc.failSink {
				return
			}

			lines := assertAuditLineCount(t, sink, tc.wantLines)
			if strings.Contains(lines[0], "deliberate panic") {
				t.Errorf("audit record carries the panic text: %s", lines[0])
			}
			got := decodeAuditRecord(t, lines[0])
			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if want := []string{"changes", "command", "outcome", "target", "timestamp"}; !reflect.DeepEqual(keys, want) {
				t.Errorf("audit keys = %v, want %v", keys, want)
			}
			if got["outcome"] != tc.wantOutcome || got["command"] != "workitems set-state" {
				t.Errorf("outcome = %v, command = %v; want %q, %q", got["outcome"], got["command"], tc.wantOutcome, "workitems set-state")
			}
			target, _ := got["target"].(map[string]any)
			if target["id"] != float64(4821) || target["project"] != "P" {
				t.Errorf("target = %v, want id 4821 in project P", got["target"])
			}
			wantChanges := []any{map[string]any{"field": "System.State", "from": "Active", "to": "Closed"}}
			if !reflect.DeepEqual(got["changes"], wantChanges) {
				t.Errorf("changes = %v, want %v", got["changes"], wantChanges)
			}
		})
	}
}
