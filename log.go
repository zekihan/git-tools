package main

import (
	"io"
	"log/slog"
	"time"

	"github.com/lmittmann/tint"
)

func initLogger(logLevel string, stderr io.Writer) {
	level := slog.LevelInfo
	invalidLevel := false

	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info", "":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		invalidLevel = true
	}

	// Initialize the logger. Logs go to stderr so commands that print their
	// result to stdout (e.g. repopath) stay capturable in shells.
	logger := slog.New(
		tint.NewTextHandler(stderr, &tint.Options{Level: level, TimeFormat: time.RFC3339}),
	)
	if invalidLevel {
		logger.Warn("Invalid log level, using info", slog.String("level", logLevel))
	}

	slog.SetDefault(logger)
}
