package main

import (
	"os"

	writeguard "sentinovo.ai/bizInt/internal/guard"
)

// auditPath is the append-only JSONL sink every executed write records into
// (D-12). The record shape, the opener's 0700/0600 modes (D-11) and the append
// discipline live in internal/guard; this file only keeps the CLI's seams.
//
// It is a package-level var only so a test can point it at t.TempDir(),
// mirroring how stdoutWriter is swapped; nothing ever assigns it from argv
// (threat T-04-03).
var auditPath = writeguard.DefaultAuditPath()

// openAuditSink opens auditPath, read at call time so a test's reassignment
// takes effect. writeguard.Run calls it before any request leaves (D-13).
func openAuditSink() (*os.File, error) {
	return writeguard.OpenAuditSink(auditPath)
}

// auditOpen is the seam the write path actually hands to writeguard.Run. It
// exists ONLY so a test can inject a sink that OPENS cleanly and fails on Write:
// the ENOSPC/EIO/quota window that D-13's pre-flight open provably does not
// cover, which CR-01 is precisely about.
//
// Like auditPath, this is a package-level var only for that test seam; nothing
// ever assigns it from argv.
var auditOpen = openAuditSink
