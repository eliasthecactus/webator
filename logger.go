package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// setupLogger creates and configures an slog.Logger based on the Config and
// the debug flag.
//
// debug=true  → human-readable text to stdout, LevelDebug, no file
// debug=false → JSON to cfg.LogFile, level from cfg.LogLevel
//
// The returned func() must be called (e.g. via defer) to flush/close the log
// file if one was opened.
func setupLogger(cfg *Config, debug bool) (*slog.Logger, func(), error) {
	if debug {
		h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		return slog.New(h), func() {}, nil
	}

	// Ensure parent directory exists.
	logPath := cfg.LogFile
	if logPath == "" {
		logPath = filepath.Join(os.TempDir(), "browser-automation.log")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log dir: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %s: %w", logPath, err)
	}

	level := parseLogLevel(cfg.LogLevel)
	h := slog.NewJSONHandler(io.Writer(f), &slog.HandlerOptions{
		Level: level,
	})

	cleanup := func() {
		_ = f.Sync()
		_ = f.Close()
	}

	return slog.New(h), cleanup, nil
}

// parseLogLevel converts a string level name to the corresponding slog.Level.
// Unrecognised values default to slog.LevelInfo.
func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
