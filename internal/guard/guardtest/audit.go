package guardtest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// AuditRecords reads $HOME/.config/adoBizInt/audit.jsonl, one decoded record per
// line; a missing file is zero records. Shared by the TUI view tests (prs,
// releases, pipelines), which point HOME at a temp dir before any write runs.
// The path is built here rather than taken from guard.DefaultAuditPath because
// this package must not import internal/guard.
func AuditRecords(t *testing.T) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "adoBizInt", "audit.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()
	var records []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("audit line %q: %v", scanner.Text(), err)
		}
		records = append(records, rec)
	}
	return records
}
