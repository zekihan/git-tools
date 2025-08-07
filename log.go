package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func initLogger(logLevel string) {
	level := slog.LevelInfo

	switch logLevel {
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
		slog.Warn("Invalid log level, using info", slog.String("level", logLevel))
	}

	// Initialize the logger.
	logger := slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.RFC3339,
		}),
	)
	slog.SetDefault(logger)
}
