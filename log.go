package main

import (
	"log/slog"
	"os"
)

func init() {
	// Initialize the logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}
