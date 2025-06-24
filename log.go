package main

import (
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
	"time"
)

func initLogger(s string) {
	logLevel := slog.LevelInfo

	switch s {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	case "":
		logLevel = slog.LevelInfo
	default:
		slog.Warn("Invalid log level, using info", slog.String("level", s))
	}

	// Initialize the logger
	logger := slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      logLevel,
			TimeFormat: time.Kitchen,
		}),
	)
	slog.SetDefault(logger)
}
