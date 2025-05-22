package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isGitDir checks if a directory is a Git repository.
func isGitDir(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// runCommand executes a command in a given directory and returns its output.
// It suppresses stdout and stderr of the command.
func runCommand(dir string, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// pull performs git operations in the specified directory.
func pull(dir string) (bool, error) {
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

// findGitRepos recursively walks through directories and updates Git repos.
func findGitRepos(searchDir string) ([]string, error) {
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", searchDir, err)
	}

	repos := make([]string, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if entry.Name() == ".git" {
			repos = append(repos, searchDir)
			continue
		}

		fullPath := filepath.Join(searchDir, entry.Name())
		if isGitDir(fullPath) {
			repos = append(repos, fullPath)
			continue
		}

		subRepos, err := findGitRepos(fullPath)
		if err != nil {
			return nil, err
		}
		repos = append(repos, subRepos...)
	}

	return repos, nil
}

func run(absSearchDir string) error {
	repos, err := findGitRepos(absSearchDir)
	if err != nil {
		return fmt.Errorf("error walking directory %s: %v", absSearchDir, err)
	}

	for _, repo := range repos {
		ok, err := pull(repo)
		if !ok && err != nil {
			return fmt.Errorf("error pulling in %s: %v", repo, err)
		} else if ok && err != nil {
			slog.Warn("Warning while pulling in repo", slog.String("repo", repo), slog.Any("error", err))
		} else {
			slog.Info("Pulled in repo", slog.String("repo", repo))
		}
	}

	return nil
}
