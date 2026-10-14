package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// DefaultAuditPath is the append-only JSONL sink every executed write records
// into (D-12). It is a dedicated file rather than the shared application log,
// because an audit trail that a raised log level or a rotation can silence is
// not an audit trail.
//
// It sits beside adoBizInt.log in ~/.config/adoBizInt. The path is built by
// joining fixed literal segments onto the home directory rather than being
// derived from logging.LogPath(), which returns the log FILE path and not its
// directory. No component is caller-controlled, so there is no traversal input
// to sanitise (threat T-04-03).
func DefaultAuditPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "adoBizInt", "audit.jsonl")
}

// OpenAuditSink opens the audit file for append, creating its directory if
// needed, and returns the error rather than swallowing it.
//
// The whole value of D-13 is WHERE this is called, not that it exists: Run
// calls it BEFORE issuing the ADO mutation, so there is no window in which ADO
// changed and nothing recorded it. "The audit append is fatal" is a weak
// guarantee; "the sink was proven writable before the request left" is the one
// SAFE-02 actually needs.
//
// A newly created directory is 0700 and a newly created file 0600 (D-11),
// because v2.0 writes agent-supplied text to this log. An existing file or
// directory keeps its mode: nothing is chmodded on open. Tightening existing
// files on open is a deferred idea and out of scope here.
func OpenAuditSink(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
}

// auditRecord is one line of the audit file: exactly five keys and nothing else
// (D-15).
//
// It carries no actor identity: there is one identity, the PAT's, and Phase 1
// D-12 makes its resolution non-fatal so the field could legitimately be empty.
// It carries no argv: argv is caller-controlled and could hold a secret someone
// passed on a command line, which would make this persisted file a
// credential-disclosure path, the same class Phase 3 D-24 closed for stack
// traces (threat T-04-01). It carries no URL and no headers for the same reason,
// and it never embeds a raw error value.
//
// The key set is a one-way door. D-12 makes the file append-only, so records
// already written can never be migrated: a key added later leaves every older
// line permanently without it.
type auditRecord struct {
	Timestamp string        `json:"timestamp"`
	Command   string        `json:"command"`
	Target    Target        `json:"target"`
	Changes   []FieldChange `json:"changes"`
	Outcome   string        `json:"outcome"`
}

// newAuditRecord stamps the record at the moment of the append. The timestamp
// is RFC3339 in UTC so lines sort lexically in the order they were written.
func newAuditRecord(command string, target Target, changes []FieldChange, outcome string) auditRecord {
	return auditRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   command,
		Target:    target,
		Changes:   changes,
		Outcome:   outcome,
	}
}

// appendAuditRecord marshals to a byte slice, appends the newline, and writes
// it with ONE Write, the crash.go discipline, for a sharper reason here: a
// partial line in a JSONL file is an unparseable record forever, and it is
// unparseable in the file whose entire purpose is to survive the moment
// something went wrong.
//
// json.NewEncoder(f).Encode is deliberately not used: it can leave a partial
// line behind. A marshal or write failure is returned rather than swallowed,
// because D-13 makes this sink fatal.
//
// The write is followed by f.Sync(), and the sync error is returned as the
// append error. Without it the durability claim is untrue: a buffered line lost
// to a crash or a power cut is exactly as invisible as one never written. A
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
