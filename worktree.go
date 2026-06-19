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
	ErrRepoDoesNotExist        = errors.New("repository does not exist locally")
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

func addWorktree(ctx context.Context, repoPath, repoURL, branch string) (string, error) {
	worktreePath, err := getWorktreePath(repoURL, branch)
	if err != nil {
		return "", err
	}

	_, err = os.Stat(repoPath + "/.git")
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %s", ErrRepoDoesNotExist, repoPath)
	}

	_, err = os.Stat(worktreePath)
	if !os.IsNotExist(err) {
		return worktreePath, fmt.Errorf("%w: %s", ErrDirAlreadyExists, worktreePath)
	}

	err = os.MkdirAll(filepath.Dir(worktreePath), defaultDirPermissions)
	if err != nil {
		return "", fmt.Errorf("error creating directory %s: %w", filepath.Dir(worktreePath), err)
	}

	// First try to add a worktree for an existing branch, tag, or commit-ish.
	// If that fails (e.g. the ref does not exist yet), fall back to creating a
	// new branch with -b.
	output, err := runCommand(ctx, repoPath, "git", "worktree", "add", worktreePath, branch)
	if err == nil {
		return worktreePath, nil
	}

	createOutput, createErr := runCommand(
		ctx,
		repoPath,
		"git",
		"worktree",
		"add",
		"-b",
		branch,
		worktreePath,
	)
	if createErr != nil {
		return "", fmt.Errorf(
			"error adding worktree: %w\nOutput: %s\nfallback (-b) error: %w\nOutput: %s",
			err,
			output,
			createErr,
			createOutput,
		)
	}

	return worktreePath, nil
}

func ensureLocalRepoForWorktree(ctx context.Context, repoURL string) (string, error) {
	repoPath, err := getRepoPath(repoURL, "")
	if err != nil {
		return "", fmt.Errorf("error getting repository path: %w", err)
	}

	_, err = os.Stat(filepath.Join(repoPath, ".git"))
	if os.IsNotExist(err) {
		_, cloneErr := cloneRepo(ctx, repoURL, "")
		if cloneErr != nil {
			return "", cloneErr
		}

		return repoPath, nil
	} else if err != nil {
		return "", fmt.Errorf("error checking repository path %s: %w", repoPath, err)
	}

	return repoPath, nil
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
