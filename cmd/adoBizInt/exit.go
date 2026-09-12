package main

import "sentinovo.ai/bizInt/internal/ado"

// exitCodeFor maps a read outcome to the process exit code (D-06).
//
// The failure list is scanned for an authentication rejection before the
// completeness flag is consulted, and D-07 rejected the alternative of mapping
// by the most common or most recent failure kind: auth is the one kind that is
// definitely not transient and definitely requires operator action, so a
// co-occurring timeout must never mask it. A caller that retries a 1 will
// eventually succeed; a caller that retries a 2 will loop forever against a
// rejected credential.
//
// The scan matches on FailureKind only. It never compares failure counts and
// never matches on error text: result.go:131-134 records that the server error
// message is localisable and server-controlled, so text matching would be both
// fragile and a spoofing surface.
func exitCodeFor(failures []ado.Failure, complete bool) int {
	for _, f := range failures {
		if f.Kind == ado.FailureAuth {
			return exitAuth
		}
	}
	if !complete {
		return exitPartial
	}
	return exitOK
}

// Write outcome strings. These are the `outcome` field of the write envelope
// and of the persisted audit record, so they are a one-way door twice over:
// an agent branches on them, and D-12's append-only sink means a record already
// on disk can never be migrated to a different vocabulary (D-10, D-23).
const (
	outcomeDryRun   = "dry_run"
	outcomeNoOp     = "no_op"
	outcomeExecuted = "executed"
	outcomeFailed   = "failed"
	outcomeUnknown  = "unknown"
)

// writeExitCodeFor maps a write outcome to its outcome string and process exit
// code. It is a NEW function rather than an overload of exitCodeFor: a write
// returns a bare error, so there is no []ado.Failure and no completeness flag
// and that signature cannot be reused.
//
// It copies exitCodeFor's two invariants. Auth short-circuits before any status
// comparison, because auth is the one kind that is definitely not transient and
// definitely requires operator action. And it keys on ado.FailureKind plus the
// parent's ctx.Err(), never on error text: result.go records that the server
// message is localisable and server-controlled, so matching it would be both
// fragile and a spoofing surface.
//
// parentCtxErr is consulted FIRST and is the only way to see a signal:
// classifyFailure maps context.Canceled to FailureOther on purpose so a
// user-initiated abort is never reported as a server timeout. A mid-write
// interrupt routes here to outcome-unknown rather than to Phase 3 D-25's exit 3
// plus crash record (D-22). Exit 3 asserts "nothing ran", and a write already in
// flight may well have landed — telling a caller nothing happened when something
// might have is the most consequential form of this project's core failure mode.
func writeExitCodeFor(err error, parentCtxErr error) (string, int) {
	if parentCtxErr != nil {
		return outcomeUnknown, exitOutcomeUnknown
	}
	if err == nil {
		return outcomeExecuted, exitOK
	}

	switch ado.ClassifyFailure(err) {
	case ado.FailureTimeout:
		// D-20: report unknown and return. No verify-read is issued to discover
		// the true outcome — it would extend wall-clock past the caller's own
		// --timeout, could itself time out into the same state, and a concurrent
		// change by another actor would make it lie with full confidence.
		return outcomeUnknown, exitOutcomeUnknown
	case ado.FailureAuth:
		return outcomeFailed, exitAuth
	}

	status, ok := ado.StatusCodeOf(err)
	if !ok {
		// P-13: a transport error carrying no HTTP status leaves the outcome
		// genuinely unresolved. Claiming "nothing happened" is the worse error.
		return outcomeUnknown, exitOutcomeUnknown
	}
	if status >= 500 {
		// P-12: a 5xx on a non-idempotent PATCH does not establish that the
		// mutation did not land.
		return outcomeUnknown, exitOutcomeUnknown
	}

	// P-08: ADO accepted the request and refused it — a 409 conflict, an
	// item-level permission denial. The operation ran and did not succeed,
	// which is the meaning exit 1 already carries on the read path.
	return outcomeFailed, exitPartial
}
