package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestInitSetsOwnerOnlyPermissions pins HARD-06 and D-20: after Init the log
// directory is 0700 and the log file 0600, whether the install is fresh, already
// exists with the old world-readable modes, or has been initialised before. The
// loose-install case is the one that matters in practice: every existing install
// was created 0755/0644, and a mode passed only at creation would never repair it.
func TestInitSetsOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}

	// setup points HOME at an empty temporary directory, so the real
	// ~/.config/adoBizInt is never touched, and resets the package logger
	// afterwards so a successful Init does not leak into another subtest.
	setup := func(t *testing.T) (dir, logPath string) {
		t.Helper()
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Cleanup(func() { logger = nil })
		dir = filepath.Join(home, ".config", "adoBizInt")
		return dir, filepath.Join(dir, "adoBizInt.log")
	}

	assertModes := func(t *testing.T, dir, logPath string) {
		t.Helper()
		for _, c := range []struct {
			path string
			want os.FileMode
		}{{dir, 0o700}, {logPath, 0o600}} {
			info, err := os.Stat(c.path)
			if err != nil {
				t.Fatalf("stat %s: %v", c.path, err)
			}
			if got := info.Mode().Perm(); got != c.want {
				t.Errorf("%s mode = %#o, want %#o", c.path, got, c.want)
			}
		}
	}

	t.Run("fresh install", func(t *testing.T) {
		dir, logPath := setup(t)
		if err := Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		assertModes(t, dir, logPath)
	})

	t.Run("existing loose install", func(t *testing.T) {
		dir, logPath := setup(t)
		// Chmod explicitly after creating, so the process umask cannot make the
		// fixture tighter than the loose modes an old install actually has.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatalf("chmod fixture dir: %v", err)
		}
		if err := os.WriteFile(logPath, []byte("old line\n"), 0o644); err != nil {
			t.Fatalf("create fixture log: %v", err)
		}
		if err := os.Chmod(logPath, 0o644); err != nil {
			t.Fatalf("chmod fixture log: %v", err)
		}

		if err := Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		assertModes(t, dir, logPath)
	})

	// Two processes starting at once both run Init. Chmod to fixed modes is
	// idempotent, so a second call must succeed and leave the same modes.
	t.Run("repeat init", func(t *testing.T) {
		dir, logPath := setup(t)
		for i := 1; i <= 2; i++ {
			if err := Init(); err != nil {
				t.Fatalf("Init call %d: %v", i, err)
			}
		}
		assertModes(t, dir, logPath)
	})
}
