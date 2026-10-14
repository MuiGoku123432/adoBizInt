package main

import (
	"sentinovo.ai/bizInt/internal/ado"
	writeguard "sentinovo.ai/bizInt/internal/guard"
)

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

// The write outcome strings are writeguard's; these aliases keep the CLI's
// names. They are a one-way door: see the vocabulary's doc in internal/guard.
const (
	outcomeDryRun   = writeguard.OutcomeDryRun
	outcomeNoOp     = writeguard.OutcomeNoOp
	outcomeExecuted = writeguard.OutcomeExecuted
	outcomeFailed   = writeguard.OutcomeFailed
	outcomeUnknown  = writeguard.OutcomeUnknown
)

// writeExitCodeFor maps a write outcome, as writeguard.Run classified it, to the
// process exit code. It is separate from exitCodeFor: a write returns a bare
// error, so there is no []ado.Failure and no completeness flag.
//
// The 5xx, timeout, no-status and interrupted-parent rules live only in the
// guard's classifier; by the time an outcome reaches here they are already
// "unknown". The one distinction left is within "failed": an authentication
// rejection exits 2 rather than 1, keying on ado.FailureKind and never on error
// text, because auth is the one kind that is definitely not transient and
// definitely requires operator action (P-08 for the rest).
func writeExitCodeFor(outcome string, err error) int {
	switch outcome {
	case outcomeExecuted:
		return exitOK
	case outcomeFailed:
		if ado.ClassifyFailure(err) == ado.FailureAuth {
			return exitAuth
		}
		return exitPartial
	default:
		return exitOutcomeUnknown
	}
}
