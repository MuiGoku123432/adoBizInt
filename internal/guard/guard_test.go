package guard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops"

	"sentinovo.ai/bizInt/internal/guard/guardtest"
)

var _ Client = (*guardtest.Client)(nil)

// oneChange is the diff every confirmable test op proposes.
var oneChange = []FieldChange{{Field: FieldState, From: "Active", To: "Closed"}}

// tempSink returns an audit path under t.TempDir and an opener for it, so no
// test touches the real ~/.config.
func tempSink(t *testing.T) (string, func() (*os.File, error)) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	return path, func() (*os.File, error) { return OpenAuditSink(path) }
}

// auditLines returns the audit file's lines; a missing file has none.
func auditLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// testOp builds an inline Op over the given closures.
func testOp(plan func(context.Context) ([]FieldChange, error), mutate func(context.Context) error) Op {
	return Op{Name: "workitems set-state", Target: Target{Project: "P", ID: 4821}, Plan: plan, Mutate: mutate}
}

func planOne(context.Context) ([]FieldChange, error) { return oneChange, nil }

func mustNotPlan(t *testing.T) func(context.Context) ([]FieldChange, error) {
	return func(context.Context) ([]FieldChange, error) {
		t.Fatalf("Plan was called")
		return nil, nil
	}
}

func mustNotMutate(t *testing.T) func(context.Context) error {
	return func(context.Context) error {
		t.Fatalf("Mutate was called")
		return nil
	}
}

func confirmed([]FieldChange) bool { return true }

func wrappedError(status int, message string) error {
	return azuredevops.WrappedError{Message: &message, StatusCode: &status}
}

func TestRunOpensSinkBeforePreRead(t *testing.T) {
	// A regular file standing where the audit directory should be makes the
	// open fail with no dependence on permissions, so it holds under root too.
	notADir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(notADir, nil, 0600); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}
	path := filepath.Join(notADir, "audit.jsonl")

	got := Run(context.Background(), testOp(mustNotPlan(t), mustNotMutate(t)), Options{
		Confirm: confirmed,
		Open:    func() (*os.File, error) { return OpenAuditSink(path) },
		Actor:   ActorTUI,
	})

	if got.SinkErr == nil {
		t.Fatalf("SinkErr = nil, want the open error")
	}
	if got.Outcome != "" {
		t.Errorf("Outcome = %q, want empty when nothing was attempted", got.Outcome)
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("audit file %s exists; an unopenable sink must create nothing", path)
	}
}

func TestRunRefusesNoOpWithoutMutation(t *testing.T) {
	tests := []struct {
		name string
		plan []FieldChange
	}{
		{name: "empty slice", plan: []FieldChange{}},
		{name: "nil slice", plan: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, open := tempSink(t)
			plan := func(context.Context) ([]FieldChange, error) { return tt.plan, nil }

			// Confirmed on purpose: the no-op refusal precedes the confirm gate.
			got := Run(context.Background(), testOp(plan, mustNotMutate(t)), Options{Confirm: confirmed, Open: open, Actor: ActorTUI})

			if got.Outcome != OutcomeNoOp {
				t.Errorf("Outcome = %q, want %q", got.Outcome, OutcomeNoOp)
			}
			if got.Changes == nil || len(got.Changes) != 0 {
				t.Errorf("Changes = %#v, want a non-nil empty slice", got.Changes)
			}
			if n := len(auditLines(t, path)); n != 0 {
				t.Errorf("audit holds %d lines, want 0", n)
			}
		})
	}
}

// TestRunUnconfirmedIsDryRunAndUnaudited pins the guard half of D-04: a
// declined or unconfirmed edit is not audited, on either surface.
func TestRunUnconfirmedIsDryRunAndUnaudited(t *testing.T) {
	var shown []FieldChange
	tests := []struct {
		name    string
		confirm func([]FieldChange) bool
	}{
		{name: "confirm nil", confirm: nil},
		{name: "confirm declines", confirm: func(c []FieldChange) bool { shown = c; return false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, open := tempSink(t)

			got := Run(context.Background(), testOp(planOne, mustNotMutate(t)), Options{Confirm: tt.confirm, Open: open, Actor: ActorTUI})

			if got.Outcome != OutcomeDryRun {
				t.Errorf("Outcome = %q, want %q", got.Outcome, OutcomeDryRun)
			}
			if !reflect.DeepEqual(got.Changes, oneChange) {
				t.Errorf("Changes = %v, want %v", got.Changes, oneChange)
			}
			if n := len(auditLines(t, path)); n != 0 {
				t.Errorf("audit holds %d lines, want 0", n)
			}
		})
	}
	if !reflect.DeepEqual(shown, oneChange) {
		t.Errorf("Confirm received %v, want the fresh diff %v", shown, oneChange)
	}
}

func TestRunExecutedAppendsOneRecord(t *testing.T) {
	path, open := tempSink(t)
	op := testOp(planOne, func(context.Context) error { return nil })

	got := Run(context.Background(), op, Options{Confirm: confirmed, Open: open, Actor: ActorTUI})

	if got.Outcome != OutcomeExecuted || got.MutateErr != nil || got.AuditErr != nil {
		t.Fatalf("report = %+v, want executed with no errors", got)
	}
	lines := auditLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("audit holds %d lines, want 1", len(lines))
	}
	var rec auditRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("decode audit line %q: %v", lines[0], err)
	}
	if rec.Command != op.Name || rec.Target != op.Target || !reflect.DeepEqual(rec.Changes, oneChange) || rec.Outcome != OutcomeExecuted {
		t.Errorf("audit record = %+v, want command %q, target %+v, changes %v, outcome executed", rec, op.Name, op.Target, oneChange)
	}
}

func TestRunClassifiesOutcomes(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name      string
		parent    context.Context
		mutateErr error
		want      string
		wantLines int
	}{
		{name: "nil", parent: context.Background(), mutateErr: nil, want: OutcomeExecuted, wantLines: 1},
		{name: "403", parent: context.Background(), mutateErr: wrappedError(403, "forbidden"), want: OutcomeFailed, wantLines: 0},
		{name: "409", parent: context.Background(), mutateErr: wrappedError(409, "conflict"), want: OutcomeFailed, wantLines: 0},
		{name: "503", parent: context.Background(), mutateErr: wrappedError(503, "unavailable"), want: OutcomeUnknown, wantLines: 1},
		{name: "deadline", parent: context.Background(), mutateErr: context.DeadlineExceeded, want: OutcomeUnknown, wantLines: 1},
		{name: "no status", parent: context.Background(), mutateErr: errors.New("connection reset"), want: OutcomeUnknown, wantLines: 1},
		{name: "cancelled parent", parent: cancelled, mutateErr: nil, want: OutcomeUnknown, wantLines: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, open := tempSink(t)
			op := testOp(planOne, func(context.Context) error { return tt.mutateErr })

			got := Run(tt.parent, op, Options{Confirm: confirmed, Open: open, Actor: ActorTUI})

			if got.Outcome != tt.want {
				t.Errorf("Outcome = %q, want %q", got.Outcome, tt.want)
			}
			if !errors.Is(got.MutateErr, tt.mutateErr) {
				t.Errorf("MutateErr = %v, want %v", got.MutateErr, tt.mutateErr)
			}
			if n := len(auditLines(t, path)); n != tt.wantLines {
				t.Errorf("audit holds %d lines, want %d", n, tt.wantLines)
			}
		})
	}
}

// TestRunAuditAppendFailureDowngradesToUnknown is CR-01 at the guard: a sink
// that opens and then fails on Write must not let a landed write report
// executed. An already-closed file fails Write without relying on permissions.
func TestRunAuditAppendFailureDowngradesToUnknown(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "closed-sink-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}
	op := testOp(planOne, func(context.Context) error { return nil })

	got := Run(context.Background(), op, Options{Confirm: confirmed, Open: func() (*os.File, error) { return f, nil }, Actor: ActorTUI})

	if got.Outcome != OutcomeUnknown {
		t.Errorf("Outcome = %q, want %q", got.Outcome, OutcomeUnknown)
	}
	if got.AuditErr == nil {
		t.Errorf("AuditErr = nil, want the append failure")
	}
}

func TestRunMutatePanicRecordsUnknownOnce(t *testing.T) {
	path, open := tempSink(t)
	op := testOp(planOne, func(context.Context) error { panic("deliberate panic mid-write") })

	got := Run(context.Background(), op, Options{Confirm: confirmed, Open: open, Actor: ActorTUI})

	if got.Panic != "deliberate panic mid-write" {
		t.Errorf("Panic = %v, want the recovered value", got.Panic)
	}
	if got.Outcome != OutcomeUnknown {
		t.Errorf("Outcome = %q, want %q", got.Outcome, OutcomeUnknown)
	}
	lines := auditLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("audit holds %d lines, want exactly 1", len(lines))
	}
	if strings.Contains(lines[0], "deliberate panic") {
		t.Errorf("audit record carries the panic text: %s", lines[0])
	}
	var rec auditRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil || rec.Outcome != OutcomeUnknown {
		t.Errorf("audit line %q: outcome %q, err %v; want outcome unknown", lines[0], rec.Outcome, err)
	}
}

// TestRunBoundsContextWithoutParentDeadline pins the bounded half of GUARD-02:
// even with no parent deadline and no budget, Plan and Mutate run under one.
func TestRunBoundsContextWithoutParentDeadline(t *testing.T) {
	_, open := tempSink(t)
	var planBounded, mutateBounded bool
	op := testOp(
		func(ctx context.Context) ([]FieldChange, error) {
			_, planBounded = ctx.Deadline()
			return oneChange, nil
		},
		func(ctx context.Context) error {
			_, mutateBounded = ctx.Deadline()
			return nil
		},
	)

	Run(context.Background(), op, Options{Budget: 0, Confirm: confirmed, Open: open, Actor: ActorTUI})

	if !planBounded || !mutateBounded {
		t.Errorf("deadline seen by Plan = %v, by Mutate = %v; want both true", planBounded, mutateBounded)
	}
}

// TestRunNeverWritesStdout is the runtime half of the no-stdout property; the
// AST half is TestGuardNeverPrintsOrExits.
func TestRunNeverWritesStdout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	prev := os.Stdout
	os.Stdout = w
	func() {
		defer func() { os.Stdout = prev }()
		_, open := tempSink(t)
		opts := Options{Confirm: confirmed, Open: open, Actor: ActorTUI}
		Run(context.Background(), testOp(planOne, func(context.Context) error { return nil }), opts)
		Run(context.Background(), testOp(planOne, func(context.Context) error { panic("deliberate") }), opts)
	}()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Run wrote %d bytes to stdout: %q", len(got), got)
	}
}

// TestOpNamesArePinned keeps the constructors' names equal to the CLI command
// strings, which are persisted in every audit record. Building an Op calls
// nothing, so a fake with no funcs set is enough.
func TestOpNamesArePinned(t *testing.T) {
	c := &guardtest.Client{T: t}
	tests := []struct {
		op   Op
		want string
	}{
		{op: SetState(c, "P", 1, "Closed", Effort{}), want: "workitems set-state"},
		{op: ApprovePR(c, "P", "R", 1), want: "prs approve"},
		{op: CompletePR(c, "P", "R", 1), want: "prs complete"},
		{op: ApproveRelease(c, "P", 1, ""), want: "releases approve"},
		{op: RejectRelease(c, "P", 1, ""), want: "releases reject"},
		{op: QueuePipeline(c, "P", 1, "main"), want: "pipelines queue"},
	}
	for _, tt := range tests {
		if tt.op.Name != tt.want {
			t.Errorf("Op.Name = %q, want %q", tt.op.Name, tt.want)
		}
	}
}
