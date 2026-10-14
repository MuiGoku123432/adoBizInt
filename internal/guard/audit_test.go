package guard

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpenAuditSinkCreatesOwnerOnlyFileAndDir pins D-11: v2.0 writes
// agent-supplied text to the audit log, so a newly created file and directory
// are owner-only.
func TestOpenAuditSinkCreatesOwnerOnlyFileAndDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh")
	path := filepath.Join(dir, "audit.jsonl")

	f, err := OpenAuditSink(path)
	if err != nil {
		t.Fatalf("OpenAuditSink: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	assertMode(t, path, 0600)
	assertMode(t, dir, 0700)
}

// TestOpenAuditSinkLeavesExistingModeAlone pins the other half of D-11: nothing
// is chmodded on open, so an existing file and directory keep their modes.
func TestOpenAuditSinkLeavesExistingModeAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "existing")
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}
	// Chmod after creation so the starting modes do not depend on the umask.
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatalf("chmod file: %v", err)
	}

	f, err := OpenAuditSink(path)
	if err != nil {
		t.Fatalf("OpenAuditSink: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	assertMode(t, path, 0644)
	assertMode(t, dir, 0755)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %#o, want %#o", path, got, want)
	}
}
