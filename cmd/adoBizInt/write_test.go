package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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

// effortPreReadFake answers the pre-read with a work item whose effort fields
// are all unset, and captures the effort pointers the mutating call receives.
// UpdateWorkItemStateFunc is left nil on purpose: routing an effort write
// through the no-effort method would fail by name rather than silently drop the
// effort operations.
func effortPreReadFake(t *testing.T, got *[]*float64) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t: t,
		fakeWriteClient: &fakeWriteClient{
			t: t,
			GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
				d := &ado.WorkItemDetail{}
				d.ID = id
				d.State = "Active"
				return d, nil
			},
			UpdateWorkItemStateWithEffortFunc: func(_ context.Context, _ int, _ string, oe, rem, comp *float64) error {
				*got = []*float64{oe, rem, comp}
				return nil
			},
		},
	}
}

// TestExplicitZeroEffortIsDistinctFromOmitted is D-18 stated at its narrowest.
// `--remaining 0` is a REAL zero — closing a task legitimately means
// RemainingWork 0 — and must be distinguishable from omitting the flag. The
// mechanism is fs.Visit (flags actually set) rather than a value comparison,
// because a value comparison cannot tell 0-because-asked from 0-by-default.
func TestExplicitZeroEffortIsDistinctFromOmitted(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// want is indexed originalEstimate, remaining, completed; nil means the
		// pointer must be nil.
		want []*float64
	}{
		{
			name: "explicit zero remaining is a non-nil pointer to 0",
			args: setStateArgs("--confirm", "--remaining", "0"),
			want: []*float64{nil, floatPtr(0), nil},
		},
		{
			name: "two effort flags supplied, one omitted",
			args: setStateArgs("--confirm", "--remaining", "3.5", "--completed", "4.5"),
			want: []*float64{nil, floatPtr(3.5), floatPtr(4.5)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempAuditSink(t)
			_, stderr := captureOutput(t)

			var got []*float64
			if code := dispatch(context.Background(), effortPreReadFake(t, &got), tc.args); code != exitOK {
				t.Fatalf("write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
			}

			if len(got) != 3 {
				t.Fatalf("UpdateWorkItemStateWithEffort was not called (got = %v)", got)
			}
			for i, want := range tc.want {
				switch {
				case want == nil && got[i] != nil:
					t.Errorf("effort[%d] = %v, want nil (an omitted flag must not become a value)", i, *got[i])
				case want != nil && got[i] == nil:
					t.Errorf("effort[%d] = nil, want %v (a supplied flag must survive)", i, *want)
				case want != nil && got[i] != nil && *want != *got[i]:
					t.Errorf("effort[%d] = %v, want %v", i, *got[i], *want)
				}
			}
		})
	}
}

// TestOmittedEffortRoutesThroughTheNoEffortMethod is the other half of D-18: with
// no effort flag supplied at all, the write must NOT go through the effort
// method, because that method would then be handed three nils for no reason.
// UpdateWorkItemStateWithEffortFunc is nil here, so taking that route fails by
// name.
func TestOmittedEffortRoutesThroughTheNoEffortMethod(t *testing.T) {
	tempAuditSink(t)
	_, stderr := captureOutput(t)

	called := false
	c := &fakeReadClient{
		t: t,
		fakeWriteClient: &fakeWriteClient{
			t: t,
			GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
				d := &ado.WorkItemDetail{}
				d.ID = id
				d.State = "Active"
				return d, nil
			},
			UpdateWorkItemStateFunc: func(_ context.Context, _ int, _ string) error {
				called = true
				return nil
			},
		},
	}

	if code := dispatch(context.Background(), c, setStateArgs("--confirm")); code != exitOK {
		t.Fatalf("write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}
	if !called {
		t.Error("UpdateWorkItemState was not called for an effort-free write")
	}
}

// TestEffortChangesAreOrderedBeforeState pins the fixed diff order —
// OriginalEstimate, RemainingWork, CompletedWork, then System.State — which
// matches the JSON-Patch operation order the exact-patch test in internal/ado
// asserts. The diff a caller reads and the document ADO receives describe the
// same sequence.
func TestEffortChangesAreOrderedBeforeState(t *testing.T) {
	tempAuditSink(t)
	stdout, stderr := captureOutput(t)

	args := setStateArgs("--remaining", "3.5", "--completed", "4.5")
	if code := dispatch(context.Background(), preReadFake(t, "Active"), args); code != exitOK {
		t.Fatalf("dry run = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}

	got := decodeWriteEnvelope(t, stdout.Bytes())
	want := []fieldChange{
		{Field: fieldRemainingWork, From: "", To: "3.5"},
		{Field: fieldCompletedWork, From: "", To: "4.5"},
		{Field: fieldState, From: "Active", To: "Closed"},
	}
	if !reflect.DeepEqual(got.Changes, want) {
		t.Errorf("changes = %#v, want %#v", got.Changes, want)
	}
}

// TestEffortFlagsNotRegisteredOnNonEffortVerbs is the same structural rule
// --confirm follows: a flag that parses but cannot mean anything is worse than
// a flag that does not exist. An agent reading `prs approve --help` must not
// find a --remaining that cannot set remaining work.
func TestEffortFlagsNotRegisteredOnNonEffortVerbs(t *testing.T) {
	effortFlagNames := []string{"original-estimate", "remaining", "completed"}

	for _, name := range commandKeys() {
		t.Run(name, func(t *testing.T) {
			wantRegistered := writeSupportByCommand[name].effort

			fs, _, _, _ := newFlagSet(name)
			for _, flagName := range effortFlagNames {
				registered := fs.Lookup(flagName) != nil
				if wantRegistered && !registered {
					t.Errorf("effort verb %q does not register --%s", name, flagName)
				}
				if !wantRegistered && registered {
					t.Errorf("verb %q registers --%s, which cannot mean anything there", name, flagName)
				}
			}
		})
	}
}

// TestEffortFlagOnNonEffortVerbIsAParseError is the runtime consequence of the
// registration rule above: exit 3, nothing attempted.
func TestEffortFlagOnNonEffortVerbIsAParseError(t *testing.T) {
	_, _ = captureOutput(t)

	c := &fakeReadClient{t: t}
	code := dispatch(context.Background(), c, []string{"workitems", "list", "--remaining", "3"})
	if code != exitUnknown {
		t.Errorf("workitems list --remaining = %d, want %d", code, exitUnknown)
	}
}

func floatPtr(f float64) *float64 { return &f }

// ---------------------------------------------------------------------------
// The phase-wide write guard matrix
// ---------------------------------------------------------------------------

// writeFixture describes one write verb well enough to drive the whole guard
// set against it.
//
// newFake is the load-bearing field. When mutate is nil it leaves the verb's
// mutating closure nil, so a mutation is a t.Fatalf naming the method at the
// call site rather than an assertion about a counter after the fact. That is
// the difference between "no mutation happened" and "no mutation was possible".
type writeFixture struct {
	// args is a complete invocation in --json mode with every required flag
	// present and --confirm absent.
	args []string
	// hasNoOpCondition is false only for `pipelines queue`: queuing a run is
	// not an idempotent state change, so no pre-read state makes it redundant.
	hasNoOpCondition bool
	// wantChange is the single diff row a pending target produces.
	wantChange fieldChange
	// newFake builds the client. applied selects the already-applied pre-read
	// state; mutate, when non-nil, is installed as the verb's mutating closure.
	newFake func(t *testing.T, applied bool, mutate func() error) *fakeReadClient
	// parseErrors are per-verb invocations that must exit 3 with no client call.
	parseErrors map[string][]string
}

// writeGuardFixtures is the per-verb half of the matrix. It is keyed on the same
// `noun verb` string as writeSupportByCommand so the two can be compared
// directly; TestWriteGuardMatrix fails on any key present there and missing
// here.
func writeGuardFixtures() map[string]writeFixture {
	return map[string]writeFixture{
		"workitems set-state": {
			args:             []string{"workitems", "set-state", "4821", "--project", "P", "--state", "Closed", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldState, From: "Active", To: "Closed"},
			newFake: func(t *testing.T, applied bool, mutate func() error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
						d := &ado.WorkItemDetail{}
						d.ID = id
						d.State = "Active"
						if applied {
							d.State = "Closed"
						}
						return d, nil
					}}
				if mutate != nil {
					w.UpdateWorkItemStateFunc = func(context.Context, int, string) error { return mutate() }
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"missing state": {"workitems", "set-state", "4821", "--project", "P"},
			},
		},
		"prs approve": {
			args:             []string{"prs", "approve", "4821", "--project", "P", "--repo", "R", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldReviewerVote, From: "0", To: "10"},
			newFake: func(t *testing.T, applied bool, mutate func() error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetPullRequestDetailFunc: func(_ context.Context, _, _ string, prID int) (*ado.PullRequestDetail, error) {
						d := &ado.PullRequestDetail{ID: prID, Status: "active"}
						if applied {
							d.MyVote = voteApproved
						}
						return d, nil
					}}
				if mutate != nil {
					w.ApprovePullRequestFunc = func(context.Context, string, string, int) error { return mutate() }
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"zero repos": {"prs", "approve", "4821", "--project", "P"},
				"two repos":  {"prs", "approve", "4821", "--project", "P", "--repo", "R", "--repo", "S"},
			},
		},
		"prs complete": {
			args:             []string{"prs", "complete", "4821", "--project", "P", "--repo", "R", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldPRStatus, From: "active", To: "completed"},
			newFake: func(t *testing.T, applied bool, mutate func() error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetPullRequestDetailFunc: func(_ context.Context, _, _ string, prID int) (*ado.PullRequestDetail, error) {
						d := &ado.PullRequestDetail{ID: prID, Status: "active"}
						if applied {
							d.Status = "completed"
						}
						return d, nil
					}}
				if mutate != nil {
					w.CompletePullRequestFunc = func(context.Context, string, string, int) error { return mutate() }
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"zero repos": {"prs", "complete", "4821", "--project", "P"},
				"two repos":  {"prs", "complete", "4821", "--project", "P", "--repo", "R", "--repo", "S"},
			},
		},
		"releases approve": {
			args:             []string{"releases", "approve", "77", "--project", "P", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldApprovalStatus, From: "pending", To: "approved"},
			newFake: func(t *testing.T, applied bool, mutate func() error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetReleaseApprovalDetailFunc: func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
						d := &ado.ReleaseApprovalDetail{ID: id, Status: "pending"}
						if applied {
							d.Status = "approved"
						}
						return d, nil
					}}
				if mutate != nil {
					w.ApproveReleaseFunc = func(context.Context, string, int, string) error { return mutate() }
				}
				return wrapFake(t, w)
			},
		},
		"releases reject": {
			args:             []string{"releases", "reject", "77", "--project", "P", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldApprovalStatus, From: "pending", To: "rejected"},
			newFake: func(t *testing.T, applied bool, mutate func() error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetReleaseApprovalDetailFunc: func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
						d := &ado.ReleaseApprovalDetail{ID: id, Status: "pending"}
						if applied {
							d.Status = "rejected"
						}
						return d, nil
					}}
				if mutate != nil {
					w.RejectReleaseFunc = func(context.Context, string, int, string) error { return mutate() }
				}
				return wrapFake(t, w)
			},
		},
		"pipelines queue": {
			// --branch is required, never defaulted from the definition (P-04).
			args:             []string{"pipelines", "queue", "12", "--project", "P", "--branch", "main", "--json"},
			hasNoOpCondition: false,
			wantChange:       fieldChange{Field: fieldQueuedBuild, From: "", To: "main"},
			newFake: func(t *testing.T, _ bool, mutate func() error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetPipelineDefinitionDetailFunc: func(_ context.Context, _ string, id int) (*ado.PipelineDefinitionDetail, error) {
						return &ado.PipelineDefinitionDetail{ID: id, Name: "nightly", DefaultBranch: "trunk"}, nil
					}}
				if mutate != nil {
					w.QueuePipelineFunc = func(context.Context, string, int, string) (*ado.PipelineRun, error) {
						return &ado.PipelineRun{ID: 900}, mutate()
					}
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"missing branch": {"pipelines", "queue", "12", "--project", "P"},
			},
		},
	}
}

// rawChangesField returns the `changes` value of a write envelope as it was
// actually serialized. A struct-level length assertion cannot distinguish `[]`
// from `null`, and `null` is the shape an agent's `.changes | length` errors on
// — which is the same class of bug newResult's pre-allocation closed for reads.
func rawChangesField(t *testing.T, raw []byte) string {
	t.Helper()
	var env struct {
		Write struct {
			Changes json.RawMessage `json:"changes"`
		} `json:"write"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal write envelope: %v (raw: %q)", err, raw)
	}
	return string(env.Write.Changes)
}

// TestWriteGuardMatrix is the phase-wide statement of every guarded-write
// guarantee, driven over writeSupportByCommand — the same map the dispatcher
// and newFlagSet consult — rather than a hand-written list of verb names.
//
// That iteration source is the whole point. A guard proven per verb by hand is
// a guard that can be forgotten on verb seven; ranging over the registration map
// means a verb cannot enter the published surface without a fixture here, and
// the t.Fatalf below is what says so.
func TestWriteGuardMatrix(t *testing.T) {
	fixtures := writeGuardFixtures()

	// Reverse drift: a fixture for a verb no longer registered is dead weight
	// that would keep asserting against a command nobody can invoke.
	for name := range fixtures {
		if _, ok := writeSupportByCommand[name]; !ok {
			t.Errorf("the guard matrix covers %q, which writeSupportByCommand no longer registers", name)
		}
	}

	for name := range writeSupportByCommand {
		f, ok := fixtures[name]
		if !ok {
			t.Fatalf("command %q is not covered by the write guard matrix", name)
		}

		t.Run(name, func(t *testing.T) {
			// Guard 1 — no mutation without --confirm (SAFE-01, D-01), and the
			// preview reports a real field-level diff (SAFE-02, D-02).
			t.Run("dry run reports a field-level diff and mutates nothing", func(t *testing.T) {
				sink := tempAuditSink(t)
				stdout, stderr := captureOutput(t)

				code := dispatch(context.Background(), f.newFake(t, false, nil), f.args)
				if code != exitOK {
					t.Fatalf("dry run = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
				}
				got := decodeWriteEnvelope(t, stdout.Bytes())
				if got.Executed {
					t.Error("dry run reported executed:true")
				}
				if got.Outcome != outcomeDryRun {
					t.Errorf("outcome = %q, want %q", got.Outcome, outcomeDryRun)
				}
				if got.Command != name {
					t.Errorf("command = %q, want %q", got.Command, name)
				}
				if len(got.Changes) != 1 || got.Changes[0] != f.wantChange {
					t.Errorf("changes = %#v, want exactly %#v", got.Changes, f.wantChange)
				}
				// D-14: a preview changed nothing, so it records nothing.
				assertAuditLineCount(t, sink, 0)
			})

			// Guard 2 — an already-applied change is refused, on BOTH paths, so
			// the refusal precedes the --confirm branch (D-03).
			if f.hasNoOpCondition {
				for _, confirm := range []bool{false, true} {
					label := "no-op refused on the dry-run path"
					args := f.args
					if confirm {
						label = "no-op refused on the --confirm path"
						args = withConfirm(f.args)
					}
					t.Run(label, func(t *testing.T) {
						sink := tempAuditSink(t)
						stdout, stderr := captureOutput(t)

						// mutate stays nil even with --confirm.
						code := dispatch(context.Background(), f.newFake(t, true, nil), args)
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
						if raw := rawChangesField(t, stdout.Bytes()); raw != "[]" {
							t.Errorf("changes serialized as %s, want []", raw)
						}
						assertAuditLineCount(t, sink, 0)
					})
				}
			}

			// Guard 3 — a confirmed write executes and is audited exactly once
			// (SAFE-02, D-15).
			t.Run("confirmed write executes and audits exactly once", func(t *testing.T) {
				sink := tempAuditSink(t)
				stdout, stderr := captureOutput(t)

				c := f.newFake(t, false, func() error { return nil })
				code := dispatch(context.Background(), c, withConfirm(f.args))
				if code != exitOK {
					t.Fatalf("confirmed write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
				}
				got := decodeWriteEnvelope(t, stdout.Bytes())
				if !got.Executed || got.Outcome != outcomeExecuted {
					t.Errorf("executed = %v, outcome = %q; want true / %q", got.Executed, got.Outcome, outcomeExecuted)
				}
				lines := assertAuditLineCount(t, sink, 1)
				if rec := decodeAuditRecord(t, lines[0]); rec["outcome"] != outcomeExecuted {
					t.Errorf("audit outcome = %v, want %q", rec["outcome"], outcomeExecuted)
				}
			})

			// Guard 4 — a write whose result never came back is outcome-unknown,
			// not failed and not "nothing ran", and it IS audited (SAFE-04,
			// D-16).
			t.Run("deadline exceeded is outcome-unknown and audited", func(t *testing.T) {
				sink := tempAuditSink(t)
				stdout, _ := captureOutput(t)

				c := f.newFake(t, false, func() error { return context.DeadlineExceeded })
				code := dispatch(context.Background(), c, withConfirm(f.args))
				if code != exitOutcomeUnknown {
					t.Fatalf("timed-out write = %d, want %d", code, exitOutcomeUnknown)
				}
				got := decodeWriteEnvelope(t, stdout.Bytes())
				if got.Outcome != outcomeUnknown {
					t.Errorf("outcome = %q, want %q", got.Outcome, outcomeUnknown)
				}
				if got.Executed {
					t.Error("an unresolved write reported executed:true")
				}
				lines := assertAuditLineCount(t, sink, 1)
				if rec := decodeAuditRecord(t, lines[0]); rec["outcome"] != outcomeUnknown {
					t.Errorf("audit outcome = %v, want %q", rec["outcome"], outcomeUnknown)
				}
			})

			// Guard 5 — D-13's ordering, per verb. Unlike every row above, the
			// mutating closure IS set here, so the mutation would be perfectly
			// observable; the assertion is that it never happens, which is only
			// true if openAuditSink runs before the ADO call.
			t.Run("unwritable audit sink prevents the mutation", func(t *testing.T) {
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
				c := f.newFake(t, false, func() error {
					mutated = true
					return nil
				})

				code := dispatch(context.Background(), c, withConfirm(f.args))
				if mutated {
					t.Fatalf("%s issued a mutation with an unwritable audit sink; D-13 has been reversed", name)
				}
				if code == exitOK {
					t.Fatalf("unwritable audit sink exited %d, want non-zero (stderr: %q)", code, stderr.String())
				}
				if stdout.Len() != 0 {
					t.Errorf("unwritable audit sink wrote %q to stdout; want nothing", stdout.String())
				}
			})

			// Usage errors never reach the client at all (D-24, D-25).
			t.Run("parse errors exit 3 with no client call", func(t *testing.T) {
				noun, verb := splitKey(name)
				cases := map[string][]string{
					"missing positional": stripPositional(f.args),
					"two positionals":    append([]string{noun, verb, "1", "2"}, f.args[3:]...),
					"non-numeric target": append([]string{noun, verb, "abc"}, f.args[3:]...),
					"zero projects":      stripFlag(f.args, "--project"),
					"two projects":       append(append([]string{}, f.args...), "--project", "Q"),
				}
				for label, args := range f.parseErrors {
					cases[label] = args
				}

				for label, args := range cases {
					t.Run(label, func(t *testing.T) {
						tempAuditSink(t)
						stdout, stderr := captureOutput(t)

						// Every closure nil: any client call fails by name.
						c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}
						if code := dispatch(context.Background(), c, args); code != exitUnknown {
							t.Fatalf("exit = %d, want %d (stderr: %q)", code, exitUnknown, stderr.String())
						}
						if stdout.Len() != 0 {
							t.Errorf("parse error wrote %q to stdout; want nothing", stdout.String())
						}
					})
				}
			})

			// A failed pre-read is fatal: no diff can be trusted, so nothing is
			// reported and nothing is attempted (D-02).
			t.Run("failed pre-read is fatal and emits no report", func(t *testing.T) {
				tempAuditSink(t)
				stdout, stderr := captureOutput(t)

				if code := dispatch(context.Background(), failingPreReadFake(t, name), f.args); code == exitOK {
					t.Fatal("failed pre-read exited 0, want non-zero")
				}
				if stdout.Len() != 0 {
					t.Errorf("failed pre-read wrote %q to stdout; want nothing", stdout.String())
				}
				if stderr.Len() == 0 {
					t.Error("failed pre-read wrote nothing to stderr; the failure must be announced")
				}
			})
		})
	}
}
