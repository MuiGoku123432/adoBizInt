package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// assertAuditLineCount reads the audit sink back and asserts how many records
// it holds. A missing file counts as zero: the dry-run and no-op paths never
// append, and D-13's pre-flight open creates the file even when nothing is
// written to it, so both shapes are legitimate.
func assertAuditLineCount(t *testing.T, path string, want int) []string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if want != 0 {
				t.Fatalf("audit file %s does not exist; want %d records", path, want)
			}
			return nil
		}
		t.Fatalf("read audit file: %v", err)
	}

	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != want {
		t.Fatalf("audit file holds %d records, want %d (raw: %q)", len(lines), want, raw)
	}
	return lines
}

// decodeAuditRecord unmarshals one audit line into a generic map so the key set
// itself can be asserted, rather than only the fields the struct happens to
// declare.
func decodeAuditRecord(t *testing.T, line string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("audit line is not JSON: %v (raw: %q)", err, line)
	}
	return got
}

// TestExecutedWriteAppendsAuditRecord pins SAFE-02 and D-15's exact key set.
//
// The key set is asserted rather than just the values because D-12 makes the
// file append-only: a key added later leaves every older line permanently
// without it, so the set is a one-way door and must fail loudly when it moves.
func TestExecutedWriteAppendsAuditRecord(t *testing.T) {
	withConfiguredProjects(t, "P")
	sink := tempAuditSink(t)
	_, stderr := captureOutput(t)

	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error { return nil }

	if code := dispatch(context.Background(), c, setStateArgs("--confirm")); code != exitOK {
		t.Fatalf("executed write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}

	lines := assertAuditLineCount(t, sink, 1)
	got := decodeAuditRecord(t, lines[0])

	gotKeys := make([]string, 0, len(got))
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)

	wantKeys := []string{"actor", "changes", "command", "outcome", "target", "timestamp", "user_approved"}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("audit record keys = %v, want %v", gotKeys, wantKeys)
	}
	if got["outcome"] != outcomeExecuted {
		t.Errorf("outcome = %v, want %q", got["outcome"], outcomeExecuted)
	}
	if got["command"] != "workitems set-state" {
		t.Errorf("command = %v, want %q", got["command"], "workitems set-state")
	}
	if ts, _ := got["timestamp"].(string); ts == "" {
		t.Error("audit record has an empty timestamp")
	}
}

// TestAuditRecordOmitsArgvAndCredentialFields pins threat T-04-01. The audit
// file is persisted, and argv is caller-controlled and can carry a secret
// someone passed on a command line — which would make this file a
// credential-disclosure path, the same class the no-stack-trace rule closed for
// crash records. Actor is a fixed surface label (tui, cli-human or agent,
// GUARD-04), not an identity; user identity is still never recorded.
func TestAuditRecordOmitsArgvAndCredentialFields(t *testing.T) {
	withConfiguredProjects(t, "P")
	sink := tempAuditSink(t)
	captureOutput(t)

	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error { return nil }

	if code := dispatch(context.Background(), c, setStateArgs("--confirm")); code != exitOK {
		t.Fatalf("executed write = %d, want %d", code, exitOK)
	}

	lines := assertAuditLineCount(t, sink, 1)

	// Scan the raw line rather than only the top level: a forbidden key nested
	// inside target or changes is the same disclosure.
	forbidden := []string{"argv", "url", "headers", "token", "pat", "authorization"}
	lower := strings.ToLower(lines[0])
	for _, key := range forbidden {
		if strings.Contains(lower, `"`+key+`"`) {
			t.Errorf("audit record carries a %q key: %s", key, lines[0])
		}
	}
}

// TestUnwritableAuditSinkPreventsMutation is the test that turns D-13 from a
// claim into a guarantee.
//
// Unlike the dry-run tests, the mutating closure IS set here, so a mutation
// would be perfectly observable — the assertion is that it never happens. That
// is only true if openAuditSink runs BEFORE the ADO call. Move the open below
// the mutate in runWrite and this test fails, which is precisely its job:
// "the audit append is fatal" is weak, "the sink was proven writable before the
// request left" is what SAFE-02 needs.
func TestUnwritableAuditSinkPreventsMutation(t *testing.T) {
	withConfiguredProjects(t, "P")
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(dir, 0500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	prev := auditPath
	auditPath = filepath.Join(dir, "audit.jsonl")
	t.Cleanup(func() { auditPath = prev })

	stdout, stderr := captureOutput(t)

	mutated := false
	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error {
		mutated = true
		return nil
	}

	code := dispatch(context.Background(), c, setStateArgs("--confirm"))
	if mutated {
		t.Fatal("a mutation was issued with an unwritable audit sink; D-13 has been reversed")
	}
	if code == exitOK {
		t.Fatalf("unwritable audit sink exited %d, want non-zero (stderr: %q)", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("unwritable audit sink wrote %q to stdout; want nothing", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Error("unwritable audit sink wrote nothing to stderr; the refusal must be announced")
	}
}

// TestDryRunAndNoOpAppendNoAuditRecord pins D-14. The file answers "what changed
// in ADO", and neither a preview nor a refused no-op changed anything.
func TestDryRunAndNoOpAppendNoAuditRecord(t *testing.T) {
	withConfiguredProjects(t, "P")
	cases := map[string]struct {
		state string
		extra []string
	}{
		"dry run":         {state: "Active"},
		"no-op dry run":   {state: "Closed"},
		"no-op --confirm": {state: "Closed", extra: []string{"--confirm"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sink := tempAuditSink(t)
			captureOutput(t)

			dispatch(context.Background(), preReadFake(t, tc.state), setStateArgs(tc.extra...))
			assertAuditLineCount(t, sink, 0)
		})
	}
}

// TestOutcomeUnknownWriteIsAudited pins D-16, the deliberate exception to D-14.
// A mutation that may or may not have landed is precisely the case most worth a
// trail, so "only executed writes are audited" must not be read as "only writes
// known to have succeeded".
func TestOutcomeUnknownWriteIsAudited(t *testing.T) {
	withConfiguredProjects(t, "P")
	sink := tempAuditSink(t)
	_, stderr := captureOutput(t)

	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error {
		return context.DeadlineExceeded
	}

	code := dispatch(context.Background(), c, setStateArgs("--confirm"))
	if code != exitOutcomeUnknown {
		t.Fatalf("timed-out write = %d, want %d (stderr: %q)", code, exitOutcomeUnknown, stderr.String())
	}

	lines := assertAuditLineCount(t, sink, 1)
	got := decodeAuditRecord(t, lines[0])
	if got["outcome"] != outcomeUnknown {
		t.Errorf("outcome = %v, want %q", got["outcome"], outcomeUnknown)
	}
}

// failingAuditSink swaps auditOpen for one that returns a sink which OPENS
// cleanly and fails on Write, restoring the original via t.Cleanup.
//
// The failure is produced by handing back an already-closed *os.File: Write on a
// closed descriptor returns os.ErrClosed. File permissions are deliberately NOT
// used — a permission-based test is meaningless when the suite runs as root,
// which is WR-09's standing complaint and must not be repeated here.
func failingAuditSink(t *testing.T) {
	t.Helper()
	prev := auditOpen
	auditOpen = func() (*os.File, error) {
		f, err := os.CreateTemp(t.TempDir(), "closed-sink-*")
		if err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		return f, nil
	}
	t.Cleanup(func() { auditOpen = prev })
}

// TestAuditAppendFailureReportsUnknown is the CR-01 regression proof.
//
// An executed write whose audit record cannot be persisted must NOT report exit
// 0 / outcome "executed": SAFE-02 is falsified for that run, and claiming
// success would be the exact conflation this project exists to prevent.
//
// Row 1 (mutate succeeds) is the load-bearing row — it is the only one that
// fails against the pre-fix code. Row 2 (a 503 already classified unknown by
// writeExitCodeFor at exit.go:87-91) pins IDEMPOTENCE of the downgrade only; the
// reassignment is a no-op there and the row passes before and after the fix.
func TestAuditAppendFailureReportsUnknown(t *testing.T) {
	withConfiguredProjects(t, "P")
	cases := []struct {
		name      string
		mutateErr error
	}{
		{name: "executed write is downgraded to unknown", mutateErr: nil},
		{name: "already-unknown write stays unknown", mutateErr: wrappedErrorValue(503, "service unavailable")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempAuditSink(t)
			failingAuditSink(t)
			stdout, stderr := captureOutput(t)

			c := preReadFake(t, "Active")
			c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error { return tc.mutateErr }

			code := dispatch(context.Background(), c, setStateArgs("--confirm"))
			if code != exitOutcomeUnknown {
				t.Fatalf("exit = %d, want %d (stderr: %q)", code, exitOutcomeUnknown, stderr.String())
			}

			got := decodeWriteEnvelope(t, stdout.Bytes())
			if got.Outcome != outcomeUnknown {
				t.Errorf("outcome = %q, want %q", got.Outcome, outcomeUnknown)
			}
			// Asserted independently of Outcome so a future refactor that passes
			// `executed` separately cannot reintroduce the
			// executed:true / outcome:"unknown" envelope this fix exists to prevent.
			if got.Executed {
				t.Errorf("executed = true, want false when the audit record was not persisted")
			}
			if !strings.Contains(stderr.String(), "may have landed but was NOT recorded") {
				t.Errorf("stderr = %q, want the operator-actionable audit-failure message", stderr.String())
			}
		})
	}
}
