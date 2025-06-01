package main

import (
	"fmt"
	"log/slog"
	"sync"
)

// pullRepo performs git operations in the specified directory.
func pullRepo(dir string) (bool, error) {
	// Check if there is a remote
	remoteOutput, err := runCommand(dir, "git", "remote", "-v")
	if err != nil {
		slog.Debug("Error checking remotes", slog.String("dir", dir), slog.Any("error", err), slog.String("output", remoteOutput))
		return false, fmt.Errorf("error checking remotes: %w", err)
	}
	if remoteOutput == "" {
		return true, fmt.Errorf("no remote found")
	}

	// Check if the remote is reachable
	originOutput, err := runCommand(dir, "git", "remote", "show", "origin")
	if err != nil {
		slog.Debug("Error checking remote", slog.String("dir", dir), slog.Any("error", err), slog.String("output", originOutput))
		return false, fmt.Errorf("error checking remote: %w", err)
	}

	// Skip if not in main or master branch
	branchOutput, err := runCommand(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		slog.Debug("Error getting current branch", slog.String("dir", dir), slog.Any("error", err), slog.String("output", branchOutput))
		return false, fmt.Errorf("error getting current branch: %w", err)
	}
	if branchOutput != "main" && branchOutput != "master" {
		return true, fmt.Errorf("not on main or master branch. Current branch: %s", branchOutput)
	}

	// Skip if there are local changes
	statusOutput, err := runCommand(dir, "git", "status", "--porcelain")
	if err != nil {
		slog.Debug("Error checking status", slog.String("dir", dir), slog.Any("error", err), slog.String("output", statusOutput))
		return false, fmt.Errorf("error checking status: %w", err)
	}
	if statusOutput != "" {
		return true, fmt.Errorf("local changes detected")
	}

	slog.Info("Fetching in repo", slog.String("repo", dir))
	fetchOutput, err := runCommand(dir, "git", "fetch", "--all")
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
