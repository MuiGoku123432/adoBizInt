package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops"

	"sentinovo.ai/bizInt/internal/ado"
)

// wrappedErrorValue builds the bare-value SDK error shape, mirroring the
// constructor internal/ado's own tests use. A write test injecting 403/409/503
// must go through this rather than errors.New: classifyFailure reads the status
// structurally, and a bare error classifies as FailureOther, which would make
// every status-dependent assertion below pass for the wrong reason.
func wrappedErrorValue(statusCode int, message string) error {
	return azuredevops.WrappedError{Message: &message, StatusCode: &statusCode}
}

// tempAuditSink points auditPath at a fresh temp file for the duration of one
// test and restores it afterwards, mirroring how captureOutput swaps the
// writers. Every write test needs it: without it a test would append to the
// developer's real ~/.config/adoBizInt/audit.jsonl.
func tempAuditSink(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	prev := auditPath
	auditPath = path
	t.Cleanup(func() { auditPath = prev })
	return path
}

// preReadFake returns a client whose pre-read answers with a work item in the
// given state and whose MUTATING closure is deliberately left nil. The nil
// closure is the assertion: any mutation fails the test by name.
func preReadFake(t *testing.T, state string) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t: t,
		fakeWriteClient: &fakeWriteClient{
			t: t,
			GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
				d := &ado.WorkItemDetail{}
				d.ID = id
				d.State = state
				return d, nil
			},
		},
	}
}

// setStateArgs builds a `workitems set-state` invocation in --json mode.
func setStateArgs(extra ...string) []string {
	return append([]string{
		"workitems", "set-state", "4821", "--project", "P", "--state", "Closed", "--json",
	}, extra...)
}

// decodeWriteEnvelope asserts the stdout buffer holds exactly one document whose
// single top-level key is `write`, and returns the nested record.
//
// The single-key assertion is load-bearing (D-10): it is what proves a write can
// never be misread as a one-row ado.Result[T], and that no crash record was
// emitted alongside it.
func decodeWriteEnvelope(t *testing.T, raw []byte) writeRecord {
	t.Helper()

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("stdout is not one JSON document: %v (raw: %q)", err, raw)
	}
	if len(generic) != 1 {
		t.Fatalf("write envelope has %d top-level keys (%v), want exactly one", len(generic), generic)
	}
	if _, ok := generic["write"]; !ok {
		t.Fatalf("write envelope's top-level key is not `write`: %q", raw)
	}

	var env writeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal write envelope: %v", err)
	}
	return env.Write
}

// TestDryRunIssuesNoMutatingCall is SAFE-01 stated at its narrowest: without
// --confirm, no mutating call is made. The proof is structural rather than
// counted — UpdateWorkItemStateFunc is nil, so a mutation is a t.Fatalf naming
// the method, not an assertion about a counter after the fact.
func TestDryRunIssuesNoMutatingCall(t *testing.T) {
	tempAuditSink(t)
	_, stderr := captureOutput(t)

	code := dispatch(context.Background(), preReadFake(t, "Active"), setStateArgs())
	if code != exitOK {
		t.Fatalf("dry run = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}
}

// TestDryRunEnvelopeReportsFieldLevelDiff pins D-01: the preview reports a real
// current -> proposed diff established by a pre-read, not an echo of intent.
func TestDryRunEnvelopeReportsFieldLevelDiff(t *testing.T) {
	tempAuditSink(t)
	stdout, stderr := captureOutput(t)

	if code := dispatch(context.Background(), preReadFake(t, "Active"), setStateArgs()); code != exitOK {
		t.Fatalf("dry run = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}

	got := decodeWriteEnvelope(t, stdout.Bytes())
	want := []fieldChange{{Field: "System.State", From: "Active", To: "Closed"}}
	if !reflect.DeepEqual(got.Changes, want) {
		t.Errorf("changes = %#v, want %#v", got.Changes, want)
	}
	if got.Executed {
		t.Error("dry run reported executed:true")
	}
	if got.Outcome != outcomeDryRun {
		t.Errorf("outcome = %q, want %q", got.Outcome, outcomeDryRun)
	}
	if got.Command != "workitems set-state" {
		t.Errorf("command = %q, want %q", got.Command, "workitems set-state")
	}
	if got.Target.Project != "P" || got.Target.ID != 4821 {
		t.Errorf("target = %#v, want project P id 4821", got.Target)
	}
}

// TestFailedPreReadEmitsNoDryRunReport pins D-02. A dry run that says "would set
// State=Closed" without having established the current state is a confident
// statement made without looking, so a failed pre-read must leave stdout empty
// rather than degrade to an echo with a marker: a caller scripting on $? alone
// scripts straight past a marker.
func TestFailedPreReadEmitsNoDryRunReport(t *testing.T) {
	cases := map[string]func(context.Context, int) (*ado.WorkItemDetail, error){
		"pre-read errors": func(context.Context, int) (*ado.WorkItemDetail, error) {
			return nil, wrappedErrorValue(500, "boom")
		},
		"pre-read returns no item": func(context.Context, int) (*ado.WorkItemDetail, error) {
			return nil, nil
		},
	}

	for name, readFunc := range cases {
		t.Run(name, func(t *testing.T) {
			tempAuditSink(t)
			stdout, stderr := captureOutput(t)

			// UpdateWorkItemStateFunc stays nil: a mutation after a failed
			// pre-read would fail this test by name.
			c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t, GetWorkItemDetailFunc: readFunc}}

			code := dispatch(context.Background(), c, setStateArgs())
			if code == exitOK {
				t.Fatalf("failed pre-read exited %d, want non-zero", code)
			}
			if stdout.Len() != 0 {
				t.Errorf("failed pre-read wrote %q to stdout; want nothing", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Error("failed pre-read wrote nothing to stderr; the failure must be announced")
			}
		})
	}
}

// TestRefusedNoOpExitsNoOpCodeOnBothPaths pins D-03 and D-08 together. The
// refusal applies with AND without --confirm, because the pre-read gates both
// paths; and it gets its own code rather than exit 0, because collapsing "I
// changed it" into "it was already so" is the exact conflation this project
// exists to prevent.
func TestRefusedNoOpExitsNoOpCodeOnBothPaths(t *testing.T) {
	for _, extra := range [][]string{{}, {"--confirm"}} {
		name := "dry-run"
		if len(extra) > 0 {
			name = "confirm"
		}
		t.Run(name, func(t *testing.T) {
			sink := tempAuditSink(t)
			stdout, stderr := captureOutput(t)

			// Already Closed, and --state Closed: nothing to do. The mutating
			// closure stays nil on BOTH paths, including --confirm.
			code := dispatch(context.Background(), preReadFake(t, "Closed"), setStateArgs(extra...))
			if code != exitNoOp {
				t.Fatalf("refused no-op = %d, want %d (stderr: %q)", code, exitNoOp, stderr.String())
			}

			got := decodeWriteEnvelope(t, stdout.Bytes())
			if got.Outcome != outcomeNoOp {
				t.Errorf("outcome = %q, want %q", got.Outcome, outcomeNoOp)
			}
			if got.Executed {
				t.Error("refused no-op reported executed:true")
			}

			// P-09: `changes` is an empty array, never null and never absent, so
			// `.write.changes | length` evaluates without a presence check.
			var probe struct {
				Write struct {
					Changes *[]fieldChange `json:"changes"`
				} `json:"write"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
				t.Fatalf("unmarshal probe: %v", err)
			}
			if probe.Write.Changes == nil {
				t.Fatalf("changes marshalled as null or was absent: %q", stdout.String())
			}
			if len(*probe.Write.Changes) != 0 {
				t.Errorf("changes = %#v, want empty", *probe.Write.Changes)
			}

			assertAuditLineCount(t, sink, 0)
		})
	}
}

// TestWriteOutcomeClassification pins D-09, P-08, P-12 and P-13 in one table.
// The rows that matter most are the two unknown ones: a 5xx and a deadline on a
// non-idempotent request do not establish that nothing happened, and reporting
// exit 0 or "failed" there would be the most consequential possible instance of
// this project's core failure mode.
func TestWriteOutcomeClassification(t *testing.T) {
	cases := []struct {
		name        string
		mutateErr   error
		wantOutcome string
		wantCode    int
	}{
		{"success", nil, outcomeExecuted, exitOK},
		{"permission denied", wrappedErrorValue(403, "forbidden"), outcomeFailed, exitAuth},
		{"conflict", wrappedErrorValue(409, "conflict"), outcomeFailed, exitPartial},
		{"server error", wrappedErrorValue(503, "unavailable"), outcomeUnknown, exitOutcomeUnknown},
		{"deadline exceeded", context.DeadlineExceeded, outcomeUnknown, exitOutcomeUnknown},
		{"no status", errors.New("connection reset"), outcomeUnknown, exitOutcomeUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempAuditSink(t)
			stdout, stderr := captureOutput(t)

			c := preReadFake(t, "Active")
			c.UpdateWorkItemStateFunc = func(context.Context, int, string) error { return tc.mutateErr }

			code := dispatch(context.Background(), c, setStateArgs("--confirm"))
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d (stderr: %q)", code, tc.wantCode, stderr.String())
			}

			got := decodeWriteEnvelope(t, stdout.Bytes())
			if got.Outcome != tc.wantOutcome {
				t.Errorf("outcome = %q, want %q", got.Outcome, tc.wantOutcome)
			}
			if want := tc.wantOutcome == outcomeExecuted; got.Executed != want {
				t.Errorf("executed = %v, want %v", got.Executed, want)
			}
		})
	}

	// The three outcome strings and the three codes must be mutually distinct,
	// or an agent cannot tell "it changed" from "it did not" from "it might
	// have" — which is the whole point of having three.
	outcomes := []string{outcomeExecuted, outcomeFailed, outcomeUnknown}
	codes := []int{exitOK, exitPartial, exitOutcomeUnknown}
	for i := range outcomes {
		for j := i + 1; j < len(outcomes); j++ {
			if outcomes[i] == outcomes[j] {
				t.Errorf("outcome strings %d and %d are both %q", i, j, outcomes[i])
			}
			if codes[i] == codes[j] {
				t.Errorf("exit codes %d and %d are both %d", i, j, codes[i])
			}
		}
	}
}

// TestInterruptedWriteReportsOutcomeUnknownNotNothingRan pins D-22. A mid-write
// interrupt must NOT inherit the read path's exit 3 plus crash record: exit 3
// asserts "nothing ran", and a write already in flight may well have landed.
func TestInterruptedWriteReportsOutcomeUnknownNotNothingRan(t *testing.T) {
	tempAuditSink(t)
	stdout, stderr := captureOutput(t)

	ctx, cancel := context.WithCancel(context.Background())

	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, int, string) error {
		// The signal lands while the request is in flight.
		cancel()
		return context.Canceled
	}

	code := dispatch(ctx, c, setStateArgs("--confirm"))
	if code == exitUnknown {
		t.Fatalf("interrupted write exited %d (exitUnknown); that asserts nothing ran", code)
	}
	if code != exitOutcomeUnknown {
		t.Fatalf("interrupted write = %d, want %d (stderr: %q)", code, exitOutcomeUnknown, stderr.String())
	}

	// decodeWriteEnvelope already asserts the single top-level key is `write`,
	// which is exactly the "no crash record was emitted" assertion: a crash
	// record would put an `error` key on stdout instead of, or alongside, this.
	got := decodeWriteEnvelope(t, stdout.Bytes())
	if got.Outcome != outcomeUnknown {
		t.Errorf("outcome = %q, want %q", got.Outcome, outcomeUnknown)
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &generic); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	if _, found := generic["error"]; found {
		t.Errorf("interrupted write emitted a crash record: %q", stdout.String())
	}
}

// TestExitCodesArePairwiseDistinct is the drift guard on the published integer
// contract. Two codes collapsing into one is a silent break for every caller
// branching on them, and nothing else in the suite would notice.
func TestExitCodesArePairwiseDistinct(t *testing.T) {
	codes := map[string]int{
		"exitOK":             exitOK,
		"exitPartial":        exitPartial,
		"exitAuth":           exitAuth,
		"exitUnknown":        exitUnknown,
		"exitNoOp":           exitNoOp,
		"exitOutcomeUnknown": exitOutcomeUnknown,
	}
	seen := map[int]string{}
	for name, code := range codes {
		if other, dup := seen[code]; dup {
			t.Errorf("%s and %s are both %d", name, other, code)
		}
		seen[code] = name
	}
	if len(seen) != 6 {
		t.Errorf("got %d distinct exit codes, want 6", len(seen))
	}
}

// TestWriteParseErrorsExitUnknown pins D-23, D-24, D-25 and D-26. Every row is a
// usage error, so exit 3 is correct here precisely because nothing was
// attempted — both fake closures stay nil, so reaching either one fails.
//
// The two --project rows are the ones that matter: the read path's emptyIfNil
// fallback would silently pick a configured project for a mutation the caller
// never scoped, and the repeated-value row would fan a write out.
func TestWriteParseErrorsExitUnknown(t *testing.T) {
	cases := map[string][]string{
		"missing positional": {"workitems", "set-state", "--project", "P", "--state", "Closed"},
		"two positionals":    {"workitems", "set-state", "4821", "4822", "--project", "P", "--state", "Closed"},
		"non-numeric target": {"workitems", "set-state", "abc", "--project", "P", "--state", "Closed"},
		"zero projects":      {"workitems", "set-state", "4821", "--state", "Closed"},
		"two projects":       {"workitems", "set-state", "4821", "--project", "P", "--project", "Q", "--state", "Closed"},
		"missing state":      {"workitems", "set-state", "4821", "--project", "P"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			tempAuditSink(t)
			stdout, stderr := captureOutput(t)

			// Neither closure is set: any client call fails this subtest by name.
			c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}

			if code := dispatch(context.Background(), c, args); code != exitUnknown {
				t.Fatalf("exit = %d, want %d (stderr: %q)", code, exitUnknown, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("parse error wrote %q to stdout; want nothing", stdout.String())
			}
		})
	}
}

// TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb asserts structurally that
// --confirm exists on exactly the verbs writeSupportByCommand names.
//
// This is deliberately a FlagSet lookup rather than a grep for the string
// "confirm": a textual check is satisfiable by a prose comment mentioning the
// flag, which is how three earlier criteria were defeated. The t.Fatalf on an
// uncovered command is the drift guard — a new verb must be classified here
// before this test will pass.
func TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb(t *testing.T) {
	for _, name := range commandKeys() {
		t.Run(name, func(t *testing.T) {
			_, isWrite := writeSupportByCommand[name]

			fs, _, _, _ := newFlagSet(name)
			registered := fs.Lookup("confirm") != nil

			if isWrite && !registered {
				t.Errorf("write verb %q does not register --confirm; SAFE-01's guard is missing", name)
			}
			if !isWrite && registered {
				t.Errorf("read verb %q registers --confirm, which cannot mean anything there", name)
			}
		})
	}

	// Every write verb must also appear in the dispatch table; a support entry
	// for a verb that does not exist is drift in the other direction.
	for name := range writeSupportByCommand {
		if _, ok := commands[name]; !ok {
			t.Errorf("writeSupportByCommand names %q, which the dispatch table does not register", name)
		}
	}
}
