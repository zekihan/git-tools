package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func handlePullCmd(args []string) {
	pullCmd := flag.NewFlagSet("pull", flag.ExitOnError)
	pullCmd.Usage = func() { printPullHelp(pullCmd) }
	err := pullCmd.Parse(args)
	if err != nil {
		slog.Error("Error parsing pull command", slog.Any("error", err))
		os.Exit(1)
	}

	searchDir := "$HOME/repos"
	if pullCmd.NArg() > 0 {
		searchDir = pullCmd.Arg(0)
	}

	searchDir = os.ExpandEnv(searchDir)
	absSearchDir, err := filepath.Abs(searchDir)
	if err != nil {
		slog.Error("Error getting absolute path", slog.String("dir", searchDir), slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Starting git pull script", slog.String("dir", absSearchDir))

	if err := pullAll(absSearchDir, *threadCount); err != nil {
		slog.Error("Error running git pull", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Git pull script completed", slog.String("dir", absSearchDir))
}

func handleCloneCmd(args []string) {
	cloneCmd := flag.NewFlagSet("clone", flag.ExitOnError)
	cloneCmd.Usage = func() { printCloneHelp(cloneCmd) }
	err := cloneCmd.Parse(args)
	if err != nil {
		slog.Error("Error parsing clone command", slog.Any("error", err))
		os.Exit(1)
	}

	if cloneCmd.NArg() < 1 {
		slog.Error("Repository URL is required")
		cloneCmd.Usage()
		os.Exit(1)
	}

	repoURL := cloneCmd.Arg(0)
	customName := ""
	if cloneCmd.NArg() > 1 {
		customName = cloneCmd.Arg(1)
	}

	slog.Info("Starting git clone", slog.String("url", repoURL), slog.String("customName", customName))

	clonedPath, err := cloneRepo(repoURL, customName)
	if err != nil {
		slog.Error("Error cloning repository", slog.String("url", repoURL), slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Successfully cloned repository", slog.String("url", repoURL), slog.String("path", clonedPath))
}

func handleRepopathCmd(args []string) {
	repopathCmd := flag.NewFlagSet("repopath", flag.ExitOnError)
	repopathCmd.Usage = func() { printRepopathHelp(repopathCmd) }
	err := repopathCmd.Parse(args)
	if err != nil {
		slog.Error("Error parsing repopath command", slog.Any("error", err))
		os.Exit(1)
	}

	if repopathCmd.NArg() < 1 {
		slog.Error("Repository URL is required")
		repopathCmd.Usage()
		os.Exit(1)
	}

	repoURL := repopathCmd.Arg(0)
	customName := ""
	if repopathCmd.NArg() > 1 {
		customName = repopathCmd.Arg(1)
	}

	slog.Debug("Calculating repository path", slog.String("url", repoURL), slog.String("customName", customName))

	calculatedPath, err := getRepoPath(repoURL, customName)
	if err != nil {
		slog.Error("Error calculating repository path", slog.String("url", repoURL), slog.Any("error", err))
		os.Exit(1)
	}

	fmt.Println(calculatedPath) // Print the path to stdout
	slog.Debug("Successfully calculated repository path", slog.String("url", repoURL), slog.String("path", calculatedPath))
}
