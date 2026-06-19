package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	defaultRemoteName = "origin"
)

var (
	ErrNoRemoteFound        = errors.New("no remote found")
	ErrNoRemotesFound       = errors.New("no remotes found")
	ErrNoUpstreamBranch     = errors.New("no upstream branch found")
	ErrNotOnDefaultBranch   = errors.New("not on default branch")
	ErrLocalChangesDetected = errors.New("local changes detected")
	ErrSomeReposFailed      = errors.New("one or more repositories failed to pull")
)

// pullRepo performs git operations in the specified directory.
func pullRepo(ctx context.Context, dir string) (bool, error) {
	remoteName, err := validateAndGetRemoteName(ctx, dir)
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error validating remote name",
			slog.String("dir", dir),
			slog.Any("error", err),
		)

		return false, fmt.Errorf("error validating remote name: %w", err)
	}

	// Skip if not in main or master branch.
	localBranchOutput, err := runCommand(ctx, dir, "git", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error getting current branch",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", localBranchOutput),
		)

		return false, fmt.Errorf("error getting current branch: %w", err)
	}

	setRemoteDefaultBranchOutput, err := runCommand(
		ctx,
		dir,
		"git",
		"remote",
		"set-head",
		remoteName,
		"-a",
	)
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error setting remote default branch",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", setRemoteDefaultBranchOutput),
		)

		return false, fmt.Errorf("error setting remote default branch: %w", err)
	}

	remoteBranchOutput, err := runCommand(
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
			"Error getting upstream branch",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", remoteBranchOutput),
		)

		return false, fmt.Errorf("error getting upstream branch: %w", err)
	}

	remoteBranchOutput = strings.TrimPrefix(remoteBranchOutput, remoteName+"/")
	if remoteBranchOutput == "" {
		return false, fmt.Errorf("%w", ErrNoUpstreamBranch)
	}

	if localBranchOutput != remoteBranchOutput {
		return true, fmt.Errorf(
			"%w: local %s, remote %s",
			ErrNotOnDefaultBranch,
			localBranchOutput,
			remoteBranchOutput,
		)
	}

	// Skip if there are local changes.
	statusOutput, err := runCommand(
		ctx,
		dir,
		"git",
		"status",
		"--porcelain",
		"--untracked-files=no",
	)
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error checking status",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", statusOutput),
		)

		return false, fmt.Errorf("error checking status: %w", err)
	}

	if statusOutput != "" {
		return true, fmt.Errorf("%w", ErrLocalChangesDetected)
	}

	slog.InfoContext(ctx, "Pulling in repo", slog.String("repo", dir))

	pullOutput, err := runCommand(ctx, dir, "git", "pull")
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error pulling",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", pullOutput),
		)

		return false, fmt.Errorf("error pulling: %w", err)
	}

	return true, nil
}

func validateAndGetRemoteName(ctx context.Context, dir string) (string, error) {
	// Check if there is a remote.
	remoteOutput, err := runCommand(ctx, dir, "git", "remote")
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error checking remotes",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", remoteOutput),
		)

		return "", fmt.Errorf("error checking remotes: %w", err)
	}

	if remoteOutput == "" {
		return "", fmt.Errorf("%w", ErrNoRemoteFound)
	}

	remotesRaw := strings.Split(remoteOutput, "\n")
	remotes := make([]string, 0)

	for _, remote := range remotesRaw {
		remote = strings.TrimSpace(remote)
		if remote != "" {
			remotes = append(remotes, remote)
		}
	}

	if len(remotes) == 0 {
		return "", fmt.Errorf("%w", ErrNoRemotesFound)
	}

	var remoteName string
	if slices.Contains(remotes, defaultRemoteName) {
		remoteName = defaultRemoteName
		slog.DebugContext(
			ctx,
			"Default remote found",
			slog.String("dir", dir),
			slog.String("remote", remoteName),
			slog.Any("remotes", remotes),
		)
	} else {
		remoteName = remotes[0]
		slog.DebugContext(
			ctx,
			"Default remote not found, using first remote",
			slog.String("dir", dir),
			slog.String("remote", remoteName),
			slog.Any("remotes", remotes),
		)
	}

	// Check if the remote is reachable.
	remoteShowOutput, err := runCommand(ctx, dir, "git", "remote", "show", remoteName)
	if err != nil {
		slog.DebugContext(
			ctx,
			"Error checking remote",
			slog.String("dir", dir),
			slog.Any("error", err),
			slog.String("output", remoteShowOutput),
		)

		return "", fmt.Errorf("error checking remote: %w", err)
	}

	return remoteName, nil
}

func pullAll(ctx context.Context, absSearchDir string, threadCount int) error {
	repos, err := findGitRepos(absSearchDir)
	if err != nil {
		return fmt.Errorf("error walking directory %s: %w", absSearchDir, err)
	}

	var (
		wg       sync.WaitGroup
		failures atomic.Int64
	)

	semaphore := make(chan struct{}, threadCount) // Limit to threadCount concurrent operations
	defer close(semaphore)

	wg.Add(len(repos))

	for _, repo := range repos {
		semaphore <- struct{}{} // Acquire a slot

		go func(repo string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release the slot

			ok, err := pullRepo(ctx, repo)
			switch {
			case !ok && err != nil:
				failures.Add(1)

				slog.ErrorContext(
					ctx,
					"Error pulling in repo",
					slog.String("repo", repo),
					slog.Any("error", err),
				)
			case ok && err != nil:
				slog.WarnContext(
					ctx,
					"Warning while pulling in repo",
					slog.String("repo", repo),
					slog.Any("error", err),
				)
			default:
				slog.InfoContext(ctx, "Pulled in repo", slog.String("repo", repo))
			}
		}(repo)
	}

	wg.Wait() // Wait for all goroutines to finish

	if n := failures.Load(); n > 0 {
		return fmt.Errorf("%w: %d failed", ErrSomeReposFailed, n)
	}

	return nil
}
