package logging

import (
	"log/slog"
	"os"
	"path/filepath"
)

var logger *slog.Logger

// Init initializes the file-based logger and leaves its directory 0700 and the
// log file 0600 on every call (D-20). The modes passed to MkdirAll and OpenFile
// apply only when they create something, so without the explicit chmods every
// existing install would stay world-readable. The directory is the higher-value
// half: it also holds the PAT-bearing config.yaml.
//
// A chmod failure is returned like any other Init error; initLogging treats it
// as the existing non-fatal warning and logs to slog.Default() instead (D-21).
func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".config", "adoBizInt")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(logDir, 0o700); err != nil {
		return err
	}

	logPath := filepath.Join(logDir, "adoBizInt.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}

	logger = slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return nil
}

// Logger returns the initialized logger instance
func Logger() *slog.Logger {
	if logger == nil {
		// Fallback to stderr if not initialized
		return slog.Default()
	}
	return logger
}

// LogPath returns the path to the log file
func LogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "adoBizInt", "adoBizInt.log")
}
