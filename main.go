package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	verbose := flag.Bool("v", false, "Enable verbose (DEBUG level) logging")
	logLevel := flag.String("log-level", "info", "Set log level: debug, info, warn, error")

	flag.Parse()

	if *verbose {
		initLogger("debug")
	} else {
		initLogger(*logLevel)
	}

	searchDir := "$HOME/repos"
	// Check if positional arguments are provided after flags
	if flag.NArg() > 0 {
		searchDir = flag.Arg(0)
	}

	searchDir = os.ExpandEnv(searchDir)
	absSearchDir, err := filepath.Abs(searchDir)
	if err != nil {
		slog.Error("Error getting absolute path", slog.String("dir", searchDir), slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Starting git pull script", slog.String("dir", absSearchDir))

	if err := run(absSearchDir); err != nil {
		slog.Error("Error running git pull", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Git pull script completed", slog.String("dir", absSearchDir))
}
