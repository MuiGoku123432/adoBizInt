package confirm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/guard/guardtest"
)

// TestExecuteInterruptedMidMutateIsAuditedUnknown pins CR-01 on the TUI path.
// Cancelling the parent stands in for signal.NotifyContext firing on SIGTERM
// while a request is on the wire. The guard must classify the write unknown and
// append exactly one record with actor tui (Phase 4 D-22, D-08), and Wait must
// hold until that record is on disk, so main cannot exit inside the D-13
// window.
func TestExecuteInterruptedMidMutateIsAuditedUnknown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	parent, cancel := context.WithCancel(context.Background())
	SetParent(parent)
	// One cleanup, in this order: a failing run never leaves Execute blocked in
	// Mutate, and writeParent is reset only once no Execute can still read it.
	t.Cleanup(func() {
		cancel()
		Wait()
		SetParent(context.Background())
	})

	var mutates atomic.Int32
	entered := make(chan struct{})
	op := guard.Op{
		Name:   "workitems set-state",
		Target: guard.Target{Project: "P", ID: 5},
		Plan: func(context.Context) ([]guard.FieldChange, error) {
			return []guard.FieldChange{{Field: guard.FieldState, From: "Active", To: "Resolved"}}, nil
		},
		Mutate: func(ctx context.Context) error {
			mutates.Add(1)
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	b := Preview(Batch{ID: 5, Ops: []guard.Op{op}, Labels: []string{"resolved"}}, time.Minute)
	if len(b.Reports) != 1 || b.Reports[0].Outcome != guard.OutcomeDryRun {
		t.Fatalf("Preview reports = %+v, want one dry_run", b.Reports)
	}

	executed := make(chan Batch, 1)
	go func() { executed <- Execute(b, time.Minute) }()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Mutate was not entered within 5s")
	}

	waited := make(chan struct{})
	go func() {
		Wait()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("Wait returned while Mutate was in flight; main could exit before the audit append")
	case <-time.After(100 * time.Millisecond):
	}
	if records := guardtest.AuditRecords(t); len(records) != 0 {
		t.Fatalf("audit records while Mutate is in flight = %v, want none", records)
	}

	cancel() // what signal.NotifyContext does on SIGTERM

	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return within 5s of the parent being cancelled")
	}

	records := guardtest.AuditRecords(t)
	if len(records) != 1 {
		t.Fatalf("audit records = %d (%v), want exactly 1", len(records), records)
	}
	rec := records[0]
	if rec["outcome"] != guard.OutcomeUnknown || rec["actor"] != guard.ActorTUI || rec["user_approved"] != false {
		t.Errorf("audit record outcome %v actor %v user_approved %v, want unknown, tui, false", rec["outcome"], rec["actor"], rec["user_approved"])
	}

	var got Batch
	select {
	case got = <-executed:
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return within 5s of Wait")
	}
	if len(got.Reports) != 1 {
		t.Fatalf("Execute reports = %+v, want exactly one", got.Reports)
	}
	report := got.Reports[0]
	if report.Outcome != guard.OutcomeUnknown {
		t.Errorf("Outcome = %q, want %q", report.Outcome, guard.OutcomeUnknown)
	}
	if !errors.Is(report.MutateErr, context.Canceled) {
		t.Errorf("MutateErr = %v, want context.Canceled (the parent ended Mutate, not the one-minute budget)", report.MutateErr)
	}
	if report.AuditErr != nil {
		t.Errorf("AuditErr = %v, want nil", report.AuditErr)
	}
	if n := mutates.Load(); n != 1 {
		t.Errorf("Mutate calls = %d, want exactly 1 (no retry, D-08, D-09)", n)
	}
}
