package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
)

var errRepoURLRequired = errors.New("repository URL is required")

func handleCloneCmd(ctx context.Context, args []string) error {
	cloneCmd := flag.NewFlagSet("clone", flag.ExitOnError)
	cloneCmd.Usage = printCloneHelp

	err := cloneCmd.Parse(args)
	if err != nil {
		slog.ErrorContext(ctx, "Error parsing clone command", slog.Any("error", err))

		return fmt.Errorf("error parsing clone command: %w", err)
	}

	if cloneCmd.NArg() < 1 {
		slog.ErrorContext(ctx, "Repository URL is required")
		cloneCmd.Usage()

		return fmt.Errorf("%w", errRepoURLRequired)
	}

	repoURL := cloneCmd.Arg(0)

	customName := ""
	if cloneCmd.NArg() > 1 {
		customName = cloneCmd.Arg(1)
	}

	slog.InfoContext(
		ctx,
		"Starting git clone",
		slog.String("url", safeGitURL(repoURL)),
		slog.String("customName", customName),
	)

	clonedPath, err := cloneRepo(ctx, repoURL, customName)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Error cloning repository",
			slog.String("url", safeGitURL(repoURL)),
			slog.Any("error", err),
		)

		return err
	}

	slog.InfoContext(
		ctx,
		"Successfully cloned repository",
		slog.String("url", safeGitURL(repoURL)),
		slog.String("path", clonedPath),
	)

	return nil
}

func handleRepopathCmd(args []string) error {
	repopathCmd := flag.NewFlagSet("repopath", flag.ExitOnError)
	repopathCmd.Usage = printRepopathHelp

	err := repopathCmd.Parse(args)
	if err != nil {
		slog.Error("Error parsing repopath command", slog.Any("error", err))

		return fmt.Errorf("error parsing repopath command: %w", err)
	}

	if repopathCmd.NArg() < 1 {
		slog.Error("Repository URL is required")
		repopathCmd.Usage()

		return fmt.Errorf("%w", errRepoURLRequired)
	}

	repoURL := repopathCmd.Arg(0)

	customName := ""
	if repopathCmd.NArg() > 1 {
		customName = repopathCmd.Arg(1)
	}

	slog.Debug(
		"Calculating repository path",
		slog.String("url", safeGitURL(repoURL)),
		slog.String("customName", customName),
	)

	calculatedPath, err := getRepoPath(repoURL, customName)
	if err != nil {
		slog.Error(
			"Error calculating repository path",
			slog.String("url", safeGitURL(repoURL)),
			slog.Any("error", err),
		)

		return err
	}

	fmt.Println(calculatedPath) // Print the path to stdout
	slog.Debug(
		"Successfully calculated repository path",
		slog.String("url", safeGitURL(repoURL)),
		slog.String("path", calculatedPath),
	)

	return nil
}
