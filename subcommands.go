package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var (
	errRepoURLRequired     = errors.New("repository URL is required")
	errBranchRequired      = errors.New("branch is required")
	errTooManyWorktreeArgs = errors.New(
		"too many arguments for worktree command; expected only <branch>",
	)
)

//nolint:dupl // mirrors handleWorktreepruneCmd by design
func handlePullCmd(ctx context.Context, args []string) error {
	pullCmd := flag.NewFlagSet("pull", flag.ExitOnError)
	pullCmd.Usage = func() { printPullHelp(pullCmd) }

	err := pullCmd.Parse(args)
	if err != nil {
		slog.ErrorContext(ctx, "Error parsing pull command", slog.Any("error", err))

		return fmt.Errorf("error parsing pull command: %w", err)
	}

	searchDir := "$HOME/repos"
	if pullCmd.NArg() > 0 {
		searchDir = pullCmd.Arg(0)
	}

	searchDir = os.ExpandEnv(searchDir)

	absSearchDir, err := filepath.Abs(searchDir)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Error getting absolute path",
			slog.String("dir", searchDir),
			slog.Any("error", err),
		)

		return fmt.Errorf("error getting absolute path for %s: %w", searchDir, err)
	}

	slog.InfoContext(ctx, "Starting git pull script", slog.String("dir", absSearchDir))

	err = pullAll(ctx, absSearchDir, *threadCount)
	if err != nil {
		slog.ErrorContext(ctx, "Error running git pull", slog.Any("error", err))

		return err
	}

	slog.InfoContext(ctx, "Git pull script completed", slog.String("dir", absSearchDir))

	return nil
}

func handleCloneCmd(ctx context.Context, args []string) error {
	cloneCmd := flag.NewFlagSet("clone", flag.ExitOnError)
	cloneCmd.Usage = func() { printCloneHelp(cloneCmd) }

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
		slog.String("url", repoURL),
		slog.String("customName", customName),
	)

	clonedPath, err := cloneRepo(ctx, repoURL, customName)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Error cloning repository",
			slog.String("url", repoURL),
			slog.Any("error", err),
		)

		return err
	}

	slog.InfoContext(
		ctx,
		"Successfully cloned repository",
		slog.String("url", repoURL),
		slog.String("path", clonedPath),
	)

	return nil
}

func handleWorktreeCmd(ctx context.Context, args []string) error {
	worktreeCmd := flag.NewFlagSet("worktree", flag.ExitOnError)
	repoURLFlag := worktreeCmd.String(
		"repo-url",
		"",
		"Repository URL to use when outside a git repository",
	)
	worktreeCmd.Usage = func() { printWorktreeHelp(worktreeCmd) }

	err := worktreeCmd.Parse(args)
	if err != nil {
		slog.ErrorContext(ctx, "Error parsing worktree command", slog.Any("error", err))

		return fmt.Errorf("error parsing worktree command: %w", err)
	}

	repoURL, branch, err := parseWorktreeArgs(*repoURLFlag, worktreeCmd.Args())
	if err != nil {
		slog.ErrorContext(ctx, "Invalid worktree arguments", slog.Any("error", err))
		worktreeCmd.Usage()

		return err
	}

	var repoPath string
	if repoURL == "" {
		repoPath, repoURL, err = getCurrentRepoInfo(ctx)
		if err != nil {
			return toWorktreeRepoContextErr(err)
		}
	} else {
		repoPath, err = ensureLocalRepoForWorktree(ctx, repoURL)
		if err != nil {
			return err
		}
	}

	slog.InfoContext(
		ctx,
		"Starting git worktree add",
		slog.String("url", repoURL),
		slog.String("branch", branch),
	)

	worktreePath, err := addWorktree(ctx, repoPath, repoURL, branch)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Error adding worktree",
			slog.String("url", repoURL),
			slog.String("branch", branch),
			slog.Any("error", err),
		)

		return err
	}

	slog.InfoContext(
		ctx,
		"Successfully added worktree",
		slog.String("url", repoURL),
		slog.String("branch", branch),
		slog.String("path", worktreePath),
	)

	return nil
}

func handleWorktreepathCmd(ctx context.Context, args []string) error {
	worktreepathCmd := flag.NewFlagSet("worktreepath", flag.ExitOnError)
	repoURLFlag := worktreepathCmd.String(
		"repo-url",
		"",
		"Repository URL to use when outside a git repository",
	)
	worktreepathCmd.Usage = func() { printWorktreepathHelp(worktreepathCmd) }

	err := worktreepathCmd.Parse(args)
	if err != nil {
		slog.ErrorContext(ctx, "Error parsing worktreepath command", slog.Any("error", err))

		return fmt.Errorf("error parsing worktreepath command: %w", err)
	}

	repoURL, branch, err := parseWorktreeArgs(*repoURLFlag, worktreepathCmd.Args())
	if err != nil {
		slog.ErrorContext(ctx, "Invalid worktreepath arguments", slog.Any("error", err))
		worktreepathCmd.Usage()

		return err
	}

	if repoURL == "" {
		_, repoURL, err = getCurrentRepoInfo(ctx)
		if err != nil {
			return toWorktreeRepoContextErr(err)
		}
	}

	calculatedPath, err := getWorktreePath(repoURL, branch)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Error calculating worktree path",
			slog.String("url", repoURL),
			slog.String("branch", branch),
			slog.Any("error", err),
		)

		return err
	}

	fmt.Println(calculatedPath)

	slog.DebugContext(
		ctx,
		"Successfully calculated worktree path",
		slog.String("url", repoURL),
		slog.String("branch", branch),
		slog.String("path", calculatedPath),
	)

	return nil
}

func toWorktreeRepoContextErr(err error) error {
	if errors.Is(err, ErrNotInsideGitRepo) {
		return fmt.Errorf("%w", ErrRepoURLOrGitRepoContext)
	}

	return err
}

func parseWorktreeArgs(repoURL string, args []string) (string, string, error) {
	switch len(args) {
	case 0:
		return "", "", fmt.Errorf("%w", errBranchRequired)
	case 1:
		return repoURL, args[0], nil
	default:
		return "", "", fmt.Errorf("%w", errTooManyWorktreeArgs)
	}
}

//nolint:dupl // mirrors handlePullCmd by design
func handleWorktreepruneCmd(ctx context.Context, args []string) error {
	worktreepruneCmd := flag.NewFlagSet("worktreeprune", flag.ExitOnError)
	worktreepruneCmd.Usage = func() { printWorktreepruneHelp(worktreepruneCmd) }

	err := worktreepruneCmd.Parse(args)
	if err != nil {
		slog.ErrorContext(ctx, "Error parsing worktreeprune command", slog.Any("error", err))

		return fmt.Errorf("error parsing worktreeprune command: %w", err)
	}

	searchDir := "$HOME/repos"
	if worktreepruneCmd.NArg() > 0 {
		searchDir = worktreepruneCmd.Arg(0)
	}

	searchDir = os.ExpandEnv(searchDir)

	absSearchDir, err := filepath.Abs(searchDir)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Error getting absolute path",
			slog.String("dir", searchDir),
			slog.Any("error", err),
		)

		return fmt.Errorf("error getting absolute path for %s: %w", searchDir, err)
	}

	slog.InfoContext(ctx, "Starting worktree prune", slog.String("dir", absSearchDir))

	err = pruneAllWorktrees(ctx, absSearchDir, *threadCount)
	if err != nil {
		slog.ErrorContext(ctx, "Error running worktree prune", slog.Any("error", err))

		return err
	}

	slog.InfoContext(ctx, "Worktree prune completed", slog.String("dir", absSearchDir))

	return nil
}

func handleRepopathCmd(args []string) error {
	repopathCmd := flag.NewFlagSet("repopath", flag.ExitOnError)
	repopathCmd.Usage = func() { printRepopathHelp(repopathCmd) }

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
		slog.String("url", repoURL),
		slog.String("customName", customName),
	)

	calculatedPath, err := getRepoPath(repoURL, customName)
	if err != nil {
		slog.Error(
			"Error calculating repository path",
			slog.String("url", repoURL),
			slog.Any("error", err),
		)

		return err
	}

	fmt.Println(calculatedPath) // Print the path to stdout
	slog.Debug(
		"Successfully calculated repository path",
		slog.String("url", repoURL),
		slog.String("path", calculatedPath),
	)

	return nil
}
