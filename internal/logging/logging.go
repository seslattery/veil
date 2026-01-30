// Package logging provides structured logging for veil.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Setup creates a logger that writes to ~/.veilwarden/veil.log.
func Setup(level string) (*slog.Logger, func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}

	logDir := filepath.Join(home, ".veilwarden")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	logPath := filepath.Join(logDir, "veil.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, err
	}

	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	// Write to both file and stderr
	w := io.MultiWriter(f, os.Stderr)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	logger := slog.New(handler)

	cleanup := func() { f.Close() }
	return logger, cleanup, nil
}
