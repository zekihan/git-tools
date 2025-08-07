package main

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

const (
	defaultRemoteName = "origin"
)

// pullRepo performs git operations in the specified directory.
func pullRepo(dir string) (bool, error) {
	// Check if there is a remote
	remoteOutput, err := runCommand(dir, "git", "remote")
	if err != nil {
		slog.Debug("Error checking remotes", slog.String("dir", dir), slog.Any("error", err), slog.String("output", remoteOutput))
		return false, fmt.Errorf("error checking remotes: %w", err)
	}
	if remoteOutput == "" {
		return true, fmt.Errorf("no remote found")
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
		return true, fmt.Errorf("no remotes found")
	}

	remoteName := defaultRemoteName
	if slices.Contains(remotes, defaultRemoteName) {
		remoteName = defaultRemoteName
		slog.Debug("Default remote found", slog.String("dir", dir), slog.String("remote", remoteName), slog.Any("remotes", remotes))
	} else {
		remoteName = remotes[0]
		slog.Debug("Default remote not found, using first remote", slog.String("dir", dir), slog.String("remote", remoteName), slog.Any("remotes", remotes))
	}

	// Check if the remote is reachable
	remoteShowOutput, err := runCommand(dir, "git", "remote", "show", remoteName)
	if err != nil {
		slog.Debug("Error checking remote", slog.String("dir", dir), slog.Any("error", err), slog.String("output", remoteShowOutput))
		return false, fmt.Errorf("error checking remote: %w", err)
	}

	// Skip if not in main or master branch
	localBranchOutput, err := runCommand(dir, "git", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		slog.Debug("Error getting current branch", slog.String("dir", dir), slog.Any("error", err), slog.String("output", localBranchOutput))
		return false, fmt.Errorf("error getting current branch: %w", err)
	}

	setRemoteDefaultBranchOutput, err := runCommand(dir, "git", "remote", "set-head", remoteName, "-a")
	if err != nil {
		slog.Debug("Error setting remote default branch", slog.String("dir", dir), slog.Any("error", err), slog.String("output", setRemoteDefaultBranchOutput))
		return false, fmt.Errorf("error setting remote default branch: %w", err)
	}

	remoteBranchOutput, err := runCommand(dir, "git", "symbolic-ref", "--short", "refs/remotes/"+remoteName+"/HEAD")
	if err != nil {
		slog.Debug("Error getting upstream branch", slog.String("dir", dir), slog.Any("error", err), slog.String("output", remoteBranchOutput))
		return false, fmt.Errorf("error getting upstream branch: %w", err)
	}

	remoteBranchOutput = strings.TrimPrefix(remoteBranchOutput, remoteName+"/")
	if remoteBranchOutput == "" {
		return false, fmt.Errorf("no upstream branch found")
	}

	if localBranchOutput != remoteBranchOutput {
		return true, fmt.Errorf("not on default branch: local %s, remote %s", localBranchOutput, remoteBranchOutput)
	}

	// Skip if there are local changes
	statusOutput, err := runCommand(dir, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		slog.Debug("Error checking status", slog.String("dir", dir), slog.Any("error", err), slog.String("output", statusOutput))
		return false, fmt.Errorf("error checking status: %w", err)
	}
	if statusOutput != "" {
		return true, fmt.Errorf("local changes detected")
	}

	slog.Info("Fetching in repo", slog.String("repo", dir))
	fetchOutput, err := runCommand(dir, "git", "fetch", remoteName)
	if err != nil {
		slog.Debug("Error fetching", slog.String("dir", dir), slog.Any("error", err), slog.String("output", fetchOutput))
		return false, fmt.Errorf("error fetching: %w", err)
	}

	slog.Info("Pulling in repo", slog.String("repo", dir))
	pullOutput, err := runCommand(dir, "git", "pull")
	if err != nil {
		slog.Debug("Error pulling", slog.String("dir", dir), slog.Any("error", err), slog.String("output", pullOutput))
		return false, fmt.Errorf("error pulling: %w", err)
	}

	return true, nil
}

func pullAll(absSearchDir string, threadCount int) error {
	repos, err := findGitRepos(absSearchDir)
	if err != nil {
		return fmt.Errorf("error walking directory %s: %v", absSearchDir, err)
	}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, threadCount) // Limit to threadCount concurrent operations
	defer close(semaphore)

	wg.Add(len(repos))
	for _, repo := range repos {
		semaphore <- struct{}{} // Acquire a slot

		go func(r string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release the slot

			ok, err := pullRepo(r)
			if !ok && err != nil {
				slog.Error("Error pulling in repo", slog.String("repo", r), slog.Any("error", err))
			} else if ok && err != nil {
				slog.Warn("Warning while pulling in repo", slog.String("repo", r), slog.Any("error", err))
			} else {
				slog.Info("Pulled in repo", slog.String("repo", r))
			}
		}(repo)
	}

	wg.Wait() // Wait for all goroutines to finish

	return nil
}
