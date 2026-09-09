package main

import (
	"encoding/json"

	"sentinovo.ai/bizInt/internal/logging"
)

// Crash record kinds. These are a published output contract (D-26): an agent
// learns to detect a broken invocation by the presence of the top-level `error`
// key and branches on `kind`, so both the shape and these two strings are a
// one-way door.
const (
	crashKindInterrupt = "interrupt"
	crashKindPanic     = "panic"
)

// crashRecord describes an invocation that produced no read outcome at all.
//
// It carries the message and the log path, and deliberately nothing else. In
// particular it carries no stack: Go renders stack frames with their function
// arguments, and the client constructor takes the configuration value carrying
// the credential, so a stack on stdout would be a credential-disclosure path
// (D-24, threat T-03-14). This is the same constraint the failure type in
// internal/ado already documents for server error text.
type crashRecord struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	LogPath string `json:"log_path"`
}

// crashEnvelope is the top-level document. The record is nested under a single
// `error` key rather than being expressed as a result envelope with a synthetic
// failure entry (D-26). A crash must not be representable as a normal read
// outcome, because an empty item list carrying a completeness flag reads as
// real data — "nothing matched" — which is the precise confusion this project
// exists to prevent (threat T-03-16).
type crashEnvelope struct {
	Error crashRecord `json:"error"`
}

// writeCrashRecord emits the record as one document on stdout.
//
// Marshal to a byte slice, then one Write — the same discipline the result
// envelope uses (D-P4). A streaming encoder could leave a partial document
// behind, and the whole value of this record is that a caller finds exactly one
// parseable thing on stdout no matter how the invocation ended (D-23).
//
// A marshal failure is swallowed rather than escalated: the record is built
// from three plain strings and cannot fail in practice, and the alternative —
// writing a fallback string to stdout — would put an unparseable byte sequence
// on the stream this function exists to keep clean.
func writeCrashRecord(kind, message string) {
	payload, err := json.Marshal(crashEnvelope{
		Error: crashRecord{
			Kind:    kind,
			Message: message,
			LogPath: logging.LogPath(),
		},
	})
	if err != nil {
		return
	}
	payload = append(payload, '\n')
	_, _ = stdoutWriter.Write(payload)
}
