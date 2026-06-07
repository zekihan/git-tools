package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrWorktreeDetached = errors.New("worktree is in detached HEAD state")
	ErrWorktreeDirty    = errors.New("worktree has local changes")
	ErrRebaseConflict   = errors.New("rebase conflict detected")
)

type worktreeEntry struct {
	path   string
	branch string
}

func parseWorktreeList(output string) []worktreeEntry {
	stanzas := strings.Split(output, "\n\n")

	entries := make([]worktreeEntry, 0, len(stanzas)-1)

	for _, stanza := range stanzas[1:] {
		stanza = strings.TrimSpace(stanza)
		if stanza == "" {
			continue
		}

		entry := worktreeEntry{}

		for line := range strings.SplitSeq(stanza, "\n") {
			if after, ok := strings.CutPrefix(line, "worktree "); ok {
				entry.path = after
			} else if after, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
				entry.branch = after
			}
		}

		if entry.path != "" {
			entries = append(entries, entry)
		}
	}

	return entries
}

func getDefaultBranch(ctx context.Context, dir, remoteName string) (string, error) {
	setHeadOutput, err := runCommand(ctx, dir, "git", "remote", "set-head", remoteName, "-a")
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error setting remote default branch",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", setHeadOutput),
		)

		return "", fmt.Errorf("error setting remote default branch: %w", err)
	}

	branchOutput, err := runCommand(
		ctx,
		dir,
		"git",
		"symbolic-ref",
		"--short",
		"refs/remotes/"+remoteName+"/HEAD",
	)
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error getting default branch",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", branchOutput),
		)

		return "", fmt.Errorf("error getting default branch: %w", err)
	}

	branch := strings.TrimPrefix(branchOutput, remoteName+"/")
	if branch == "" {
		return "", fmt.Errorf("%w", ErrNoUpstreamBranch)
	}

	return branch, nil
}

func pruneWorktree(
	ctx context.Context,
	mainRepoDir, remoteName, mainBranch string,
	entry worktreeEntry,
) (bool, error) {
	if entry.branch == "" {
		return true, fmt.Errorf("%w: %s", ErrWorktreeDetached, entry.path)
	}

	_, mergeBaseErr := runCommand(
		ctx,
		mainRepoDir,
		"git",
		"merge-base",
		"--is-ancestor",
		entry.branch,
		remoteName+"/"+mainBranch,
	)
	if mergeBaseErr == nil {
		statusOutput, err := runCommand(
			ctx,
			entry.path,
			"git",
			"status",
			"--porcelain",
			"--untracked-files=no",
		)
		if err != nil {
			return false, fmt.Errorf("error checking worktree status: %w", err)
		}

		if statusOutput != "" {
			return true, fmt.Errorf("%w: %s", ErrWorktreeDirty, entry.path)
		}

		removeOutput, err := runCommand(
			ctx,
			mainRepoDir,
			"git",
			"worktree",
			"remove",
			entry.path,
		)
		if err != nil {
			return false, fmt.Errorf("error removing worktree: %w\nOutput: %s", err, removeOutput)
		}

		branchOutput, err := runCommand(
			ctx,
			mainRepoDir,
			"git",
			"branch",
			"-d",
			entry.branch,
		)
		if err != nil {
			return false, fmt.Errorf("error deleting branch: %w\nOutput: %s", err, branchOutput)
		}

		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(mergeBaseErr, &exitErr) && exitErr.ExitCode() == 1 {
		statusOutput, err := runCommand(
			ctx,
			entry.path,
			"git",
			"status",
			"--porcelain",
			"--untracked-files=no",
		)
		if err != nil {
			return false, fmt.Errorf("error checking worktree status: %w", err)
		}

		if statusOutput != "" {
			return true, fmt.Errorf("%w: %s", ErrWorktreeDirty, entry.path)
		}

		rebaseOutput, rebaseErr := runCommand(
			ctx,
			entry.path,
			"git",
			"rebase",
			remoteName+"/"+mainBranch,
		)
		if rebaseErr != nil {
			_, _ = runCommand( //nolint:errcheck // best-effort cleanup; original error is returned
				ctx,
				entry.path,
				"git",
				"rebase",
				"--abort",
			)

			return true, fmt.Errorf(
				"%w: %s\nOutput: %s",
				ErrRebaseConflict,
				entry.path,
				rebaseOutput,
			)
		}

		return true, nil
	}

	return false, fmt.Errorf("error checking merge-base: %w", mergeBaseErr)
}

func pruneWorktreesForRepo(ctx context.Context, repoDir string) error {
	remoteName, err := validateAndGetRemoteName(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("error validating remote name: %w", err)
	}

	slog.InfoContext(ctx, "Fetching in repo", slog.String("repo", repoDir))

	fetchOutput, err := runCommand(ctx, repoDir, "git", "fetch", remoteName)
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error fetching",
			slog.String("dir", repoDir),
			slog.Any("error", err),
			slog.String("output", fetchOutput),
		)

		return fmt.Errorf("error fetching: %w", err)
	}

	mainBranch, err := getDefaultBranch(ctx, repoDir, remoteName)
	if err != nil {
		return fmt.Errorf("error getting default branch: %w", err)
	}

	worktreeListOutput, err := runCommand(ctx, repoDir, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("error listing worktrees: %w", err)
	}

	entries := parseWorktreeList(worktreeListOutput)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get user home directory: %w", err)
	}

	worktreesRoot := filepath.Join(homeDir, "worktrees") + string(filepath.Separator)

	for _, entry := range entries {
		if !strings.HasPrefix(entry.path, worktreesRoot) {
			continue
		}

		ok, err := pruneWorktree(ctx, repoDir, remoteName, mainBranch, entry)

		switch {
		case !ok && err != nil:
			slog.ErrorContext(
				ctx,
				"Error pruning worktree",
				slog.String("path", entry.path),
				slog.Any("error", err),
			)
		case ok && err != nil:
			slog.WarnContext(
				ctx,
				"Warning while pruning worktree",
				slog.String("path", entry.path),
				slog.Any("error", err),
			)
		default:
			slog.InfoContext(ctx, "Pruned worktree", slog.String("path", entry.path))
		}
	}

	return nil
}

func pruneAllWorktrees(ctx context.Context, absSearchDir string, threadCount int) error {
	repos, err := findGitRepos(absSearchDir)
	if err != nil {
		return fmt.Errorf("error walking directory %s: %w", absSearchDir, err)
	}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, threadCount)
	defer close(semaphore)

	wg.Add(len(repos))

	for _, repo := range repos {
		semaphore <- struct{}{}

		go func(repo string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			err := pruneWorktreesForRepo(ctx, repo)
			if err != nil {
				slog.ErrorContext(
					ctx,
					"Error pruning worktrees for repo",
					slog.String("repo", repo),
					slog.Any("error", err),
				)
			}
		}(repo)
	}

	wg.Wait()

	return nil
}
