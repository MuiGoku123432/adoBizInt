package logging

import (
	"log/slog"
	"os"
	"path/filepath"
)

var logger *slog.Logger

// Init initializes the file-based logger
func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".config", "adoBizInt")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(logDir, "adoBizInt.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
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
