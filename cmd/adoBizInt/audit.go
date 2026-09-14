package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// auditPath is the append-only JSONL sink every executed write records into
// (D-12). It is a dedicated file rather than the shared application log,
// because an audit trail that a raised log level or a rotation can silence is
// not an audit trail.
//
// It sits beside adoBizInt.log in ~/.config/adoBizInt so Phase 5 HARD-06's
// 0600/0700 permission work covers both with one change. The path is built here
// by joining fixed literal segments onto the home directory rather than being
// derived from logging.LogPath(), which returns the log FILE path and not its
// directory.
//
// No component is caller-controlled, so there is no traversal input to sanitise
// (threat T-04-03). It is a package-level var only so a test can point it at
// t.TempDir(), mirroring how stdoutWriter is swapped; nothing ever assigns it
// from argv.
var auditPath = defaultAuditPath()

func defaultAuditPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "adoBizInt", "audit.jsonl")
}

// auditRecord is one line of the audit file: exactly five keys and nothing else
// (D-15).
//
// It carries no actor identity — there is one identity, the PAT's, and Phase 1
// D-12 makes its resolution non-fatal so the field could legitimately be empty.
// It carries no argv: argv is caller-controlled and could hold a secret someone
// passed on a command line, which would make this persisted file a
// credential-disclosure path — the same class Phase 3 D-24 closed for stack
// traces (threat T-04-01). It carries no URL and no headers for the same reason,
// and it never embeds a raw error value; a cause is flattened to a message
// string the way Failure.MarshalJSON does.
//
// The key set is a one-way door. D-12 makes the file append-only, so records
// already written can never be migrated: a key added later leaves every older
// line permanently without it.
type auditRecord struct {
	Timestamp string        `json:"timestamp"`
	Command   string        `json:"command"`
	Target    writeTarget   `json:"target"`
	Changes   []fieldChange `json:"changes"`
	Outcome   string        `json:"outcome"`
}

// newAuditRecord stamps the record at the moment of the append. The timestamp
// is RFC3339 in UTC so lines sort lexically in the order they were written.
func newAuditRecord(r writeRecord) auditRecord {
	return auditRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   r.Command,
		Target:    r.Target,
		Changes:   r.Changes,
		Outcome:   r.Outcome,
	}
}

// openAuditSink opens the audit file for append, creating its directory if
// needed, and returns the error rather than swallowing it.
//
// The whole value of D-13 is WHERE this is called, not that it exists: the
// write path calls it BEFORE issuing the ADO mutation, so there is no window in
// which ADO changed and nothing recorded it. "The audit append is fatal" is a
// weak guarantee; "the sink was proven writable before the request left" is the
// one SAFE-02 actually needs. Implementing the check after the write would
// reverse D-13 while appearing to honour it.
func openAuditSink() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// auditOpen is the seam the write path actually calls. It exists ONLY so a test
// can inject a sink that OPENS cleanly and fails on Write — the ENOSPC/EIO/quota
// window that D-13's pre-flight open provably does not cover. D-13 proves the
// sink was writable at open time; it cannot prove the eventual append succeeds,
// and CR-01 is precisely the case where it does not.
//
// Like auditPath, this is a package-level var only for that test seam; nothing
// ever assigns it from argv.
var auditOpen = openAuditSink

// appendAuditRecord marshals to a byte slice, appends the newline, and writes
// it with ONE Write — the crash.go discipline, for a sharper reason here: a
// partial line in a JSONL file is an unparseable record forever, and it is
// unparseable in the file whose entire purpose is to survive the moment
// something went wrong.
//
// json.NewEncoder(f).Encode is deliberately not used: it can leave a partial
// line behind. Unlike writeCrashRecord, a marshal or write failure here is
// returned rather than swallowed, because D-13 makes this sink fatal.
//
// The write is followed by f.Sync(), and the sync error is returned as the
// append error. Without it the durability claim is untrue: a buffered line lost
// to a crash or a power cut is exactly as invisible as one never written, and
// this file's entire purpose is to survive the moment something went wrong. A
// caller that has been told "recorded" must be able to rely on the record
// existing on disk.
func appendAuditRecord(f *os.File, r auditRecord) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if _, err = f.Write(payload); err != nil {
		return err
	}
	return f.Sync()
}
