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
