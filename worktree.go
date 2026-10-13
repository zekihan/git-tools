package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrNotInsideGitRepo        = errors.New("current directory is not inside a git repository")
	ErrMissingOriginRemote     = errors.New("could not determine repository URL from origin remote")
	ErrRepoURLOrGitRepoContext = errors.New(
		"repository URL is required when not inside a git repository",
	)
)

func getWorktreePath(repoURL, branch string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("%w", errBranchRequired)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	repoSubPath, err := getRepoSubPath(repoURL, "")
	if err != nil {
		return "", fmt.Errorf("error getting repository path: %w", err)
	}

	destination := filepath.Join(homeDir, "worktrees", repoSubPath, branch)

	return strings.TrimSuffix(destination, ".git"), nil
}

func getCurrentRepoInfo(ctx context.Context) (string, string, error) {
	topLevelCmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")

	topLevelOut, err := topLevelCmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("%w", ErrNotInsideGitRepo)
	}

	repoPath := strings.TrimSpace(string(topLevelOut))
	if repoPath == "" {
		return "", "", fmt.Errorf("%w", ErrNotInsideGitRepo)
	}

	originCmd := exec.CommandContext(
		ctx,
		"git",
		"-C",
		repoPath,
		"config",
		"--get",
		"remote.origin.url",
	)

	originOut, err := originCmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("%w", ErrMissingOriginRemote)
	}

	repoURL := strings.TrimSpace(string(originOut))
	if repoURL == "" {
		return "", "", fmt.Errorf("%w", ErrMissingOriginRemote)
	}

	return repoPath, repoURL, nil
}
