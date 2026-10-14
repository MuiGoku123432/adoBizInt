// Package guard is the single guarded-write sequence the CLI and the TUI share
// (GUARD-01). A guard that lives in one shared body cannot be forgotten on a
// surface; a guard copied per surface can.
//
// The package never writes stdout and never exits the process. Each surface maps
// the returned Report to its own output and exit code: the CLI to its envelope
// and exit codes, the TUI to its status line. Confirmation is an accident guard,
// not an authorization boundary (Phase 4 D-04).
package guard

import (
	"context"
	"fmt"
	"os"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/fetchctx"
	"sentinovo.ai/bizInt/internal/logging"
)

// Write outcome strings. These are the `outcome` field of the CLI write
// envelope and of the persisted audit record, so they are a one-way door twice
// over: an agent branches on them, and D-12's append-only sink means a record
// already on disk can never be migrated to a different vocabulary (D-10, D-23).
const (
	OutcomeDryRun   = "dry_run"
	OutcomeNoOp     = "no_op"
	OutcomeExecuted = "executed"
	OutcomeFailed   = "failed"
	OutcomeUnknown  = "unknown"
)

// Actor labels name the surface that issued a write (GUARD-04). They are the
// only values Run accepts for Options.Actor, and each is persisted verbatim as
// the audit record's actor key, so they are a one-way door like the outcomes.
const (
	ActorTUI      = "tui"
	ActorCLIHuman = "cli-human"
	ActorAgent    = "agent"
)

// FieldChange is one line of the current -> proposed diff D-01 requires. The
// `from` value is a claim about the moment of the pre-read, not a guarantee
// about the moment of the write: D-03 acknowledges the read-then-write race
// explicitly, and narrowing it is all a pre-read can do.
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// Target identifies what was (or would be) mutated. Repo is omitempty because
// most write verbs have no repository; a caller reads target.id and
// target.project unconditionally.
type Target struct {
	Project string `json:"project"`
	Repo    string `json:"repo,omitempty"`
	ID      int    `json:"id"`
}

// Op is the per-verb half of a guarded write. Each field is the whole of one
// verb's difference:
//   - Plan runs the by-id pre-read and computes the current -> proposed diff. A
//     non-nil error is fatal and nothing is attempted (D-02).
//   - Mutate issues the single mutation.
type Op struct {
	Name   string
	Target Target
	Plan   func(ctx context.Context) ([]FieldChange, error)
	Mutate func(ctx context.Context) error
}

// Options carries what differs between surfaces.
//
// Confirm is the confirm gate: nil, or a func returning false, means the run is
// a dry run. It receives the fresh diff so a surface can compare it to the one
// it showed. Open is the audit opener: nil means
// OpenAuditSink(DefaultAuditPath()), evaluated at call time so a redirected
// HOME takes effect.
type Options struct {
	Budget       time.Duration
	Confirm      func(changes []FieldChange) bool
	Open         func() (*os.File, error)
	Actor        string
	UserApproved bool
}

// Report is everything a surface needs to describe the run. Outcome is "" when
// SinkErr or PreReadErr is set, because nothing was attempted. Panic carries a
// value recovered from Mutate; the guard never re-panics.
type Report struct {
	Changes    []FieldChange
	Outcome    string
	SinkErr    error
	PreReadErr error
	MutateErr  error
	AuditErr   error
	Panic      any
}

// Run is the guarded-write body. The ORDER of its steps is the contract, not an
// implementation detail: in particular the audit sink is opened before anything
// is attempted (D-13), and the no-op refusal precedes the confirm gate because
// D-03 refuses on both paths.
func Run(ctx context.Context, op Op, opts Options) (report Report) {
	log := logging.Logger()

	// P-11: the pre-read and the write share one budget from a single parent.
	// The documented limit is that a slow pre-read can consume the budget and
	// push the write into outcome-unknown for a reason unrelated to the write
	// itself. A zero budget still gets fetchctx's default deadline (GUARD-02).
	writeCtx, cancel := fetchctx.NewWithParent(ctx, opts.Budget)
	defer cancel()

	// GUARD-04: an executed write must say which surface issued it. An actor
	// outside the three labels cannot produce a compliant record, which is the
	// same position as an unopenable sink: refuse before anything is opened,
	// read or sent (D-13). The value is never free text, so an identity or an
	// email cannot reach the persisted record through this field (T-07-11).
	switch opts.Actor {
	case ActorTUI, ActorCLIHuman, ActorAgent:
	default:
		log.Error("Audit actor invalid", "command", op.Name)
		return Report{SinkErr: fmt.Errorf("audit actor %q is not one of %q, %q or %q", opts.Actor, ActorTUI, ActorCLIHuman, ActorAgent)}
	}

	// D-13. This is the load-bearing line of the whole function and its POSITION
	// is the decision: the sink is proven writable before any request leaves, so
	// there is no window in which ADO changed and nothing recorded it. Moving it
	// below the mutate call reverses D-13 while appearing to honour it.
	open := opts.Open
	if open == nil {
		open = func() (*os.File, error) { return OpenAuditSink(DefaultAuditPath()) }
	}
	sink, err := open()
	if err != nil {
		log.Error("Audit sink unavailable", "command", op.Name, "error", err)
		return Report{SinkErr: err}
	}
	defer sink.Close()

	// D-02: a failed pre-read is fatal and nothing is attempted. A dry run that
	// says "would set State=Closed" without having established the current state
	// is a confident statement made without looking.
	changes, err := op.Plan(writeCtx)
	if err != nil {
		log.Error("Write pre-read failed", "command", op.Name, "id", op.Target.ID, "error", err)
		return Report{PreReadErr: err}
	}

	// D-03/D-08: a change already applied is refused with no mutation attempted,
	// on BOTH paths: this branch precedes the confirm gate deliberately. Changes
	// is the empty slice, never nil, so it marshals as [] (P-09).
	if len(changes) == 0 {
		return Report{Changes: []FieldChange{}, Outcome: OutcomeNoOp}
	}

	// D-04/D-07/D-14: an unconfirmed run is a dry run. It changed nothing, so it
	// is not audited.
	if opts.Confirm == nil || !opts.Confirm(changes) {
		return Report{Changes: changes, Outcome: OutcomeDryRun}
	}

	// D-12: this recover's POSITION is the request-issued signal. A panic in Plan
	// never reaches it and propagates to the surface's own guard, which reports
	// "nothing ran"; from here on the mutation may be live in ADO, so a panic is
	// outcome unknown, as an interrupt is (Phase 4 D-22). Treating "mutate
	// entered" as "request issued" over-reports unknown, the safe direction
	// (RESEARCH A4). It is registered after defer sink.Close(), and defers run
	// last-in first-out, so the sink is still open when it runs. It never
	// re-panics: the panic is returned as Report.Panic.
	audited := false
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		// D-13 best effort: leave a trace, unless the normal path already
		// appended one (never two records for one write). The record carries no
		// panic text, so the D-15 key set and the T-04-01 no-argv rule hold. A refused write that later panics is recorded unknown, which
		// over-reports in the safe direction.
		// note: this append-and-report sequence also appears in the normal path
		// below; two uses is below the rule of three, so it is not extracted.
		if !audited {
			if auditErr := appendAuditRecord(sink, newAuditRecord(op.Name, op.Target, changes, OutcomeUnknown, opts.Actor, opts.UserApproved)); auditErr != nil {
				log.Error("Failed to append audit record", "command", op.Name, "error", auditErr)
				report.AuditErr = auditErr
			}
		}
		report.Changes = changes
		report.Outcome = OutcomeUnknown
		report.Panic = r
	}()
	err = op.Mutate(writeCtx)

	// The parent's Err() is consulted, not writeCtx's and not the failure kind:
	// classifyFailure maps context.Canceled to FailureOther on purpose, so the
	// kind alone cannot tell you a signal fired (D-22).
	outcome := classify(err, ctx.Err())

	// D-14/D-16: executed and unknown are audited; dry runs and refused no-ops
	// are not, because the file answers "what changed in ADO" and neither
	// changed anything. Unknown is the deliberate exception: the case where a
	// mutation may or may not have landed is precisely the case most worth a
	// trail.
	if outcome == OutcomeExecuted || outcome == OutcomeUnknown {
		record := newAuditRecord(op.Name, op.Target, changes, outcome, opts.Actor, opts.UserApproved)
		audited = true
		if auditErr := appendAuditRecord(sink, record); auditErr != nil {
			log.Error("Failed to append audit record", "command", op.Name, "error", auditErr)
			report.AuditErr = auditErr

			// CR-01. The mutation landed in ADO and nothing recorded it, which
			// genuinely IS an unknown outcome: SAFE-02 ("every executed write
			// appends an audit record") is falsified for this run, so reporting
			// "executed" would be the exact conflation this project exists to
			// prevent. This reuses the already-ratified D-10/D-23 vocabulary. The
			// reassignment is idempotent: a run already classified unknown is
			// unchanged.
			outcome = OutcomeUnknown
		}
	}

	if err != nil {
		log.Error("Write failed", "command", op.Name, "id", op.Target.ID, "outcome", outcome, "error", err)
	} else {
		log.Info("Write complete", "command", op.Name, "id", op.Target.ID, "outcome", outcome)
	}

	report.Changes = changes
	report.Outcome = outcome
	report.MutateErr = err
	return report
}

// classify maps a mutation's error to its outcome.
//
// It keys on ado.FailureKind plus the parent's ctx.Err(), never on error text:
// result.go records that the server message is localisable and
// server-controlled, so matching it would be both fragile and a spoofing
// surface. Auth short-circuits before any status comparison, because auth is
// the one kind that is definitely not transient.
//
// parentErr is consulted FIRST and is the only way to see a signal:
// classifyFailure maps context.Canceled to FailureOther on purpose so a
// user-initiated abort is never reported as a server timeout. A mid-write
// interrupt is outcome unknown rather than "nothing ran" (D-22): a write already
// in flight may well have landed, and telling a caller nothing happened when
// something might have is the most consequential form of this project's core
// failure mode.
func classify(err, parentErr error) string {
	if parentErr != nil {
		return OutcomeUnknown
	}
	if err == nil {
		return OutcomeExecuted
	}

	switch ado.ClassifyFailure(err) {
	case ado.FailureTimeout:
		// D-20: report unknown and return. No verify-read is issued to discover
		// the true outcome: it would extend wall-clock past the caller's own
		// budget, could itself time out into the same state, and a concurrent
		// change by another actor would make it lie with full confidence.
		return OutcomeUnknown
	case ado.FailureAuth:
		return OutcomeFailed
	}

	status, ok := ado.StatusCodeOf(err)
	if !ok {
		// P-13: a transport error carrying no HTTP status leaves the outcome
		// genuinely unresolved. Claiming "nothing happened" is the worse error.
		return OutcomeUnknown
	}
	if status >= 500 {
		// P-12: a 5xx on a non-idempotent PATCH does not establish that the
		// mutation did not land.
		return OutcomeUnknown
	}

	// P-08: ADO accepted the request and refused it, as a 409 conflict or an
	// item-level permission denial. The operation ran and did not succeed.
	return OutcomeFailed
}
