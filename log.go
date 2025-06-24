package main

import (
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
	"time"
)

func initLogger(s string) {
	level := slog.LevelInfo

	switch s {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "":
		level = slog.LevelInfo
	default:
		slog.Warn("Invalid log level, using info", slog.String("level", s))
	}

	// Initialize the logger
	logger := slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.RFC3339,
		}),
	)
	slog.SetDefault(logger)
}
