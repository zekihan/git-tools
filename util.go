package main

import (
	"context"
	"fmt"
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
//
//nolint:unparam // this is a utility function.
func runCommand(ctx context.Context, dir, name string, arg ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

// findGitRepos recursively walks through directories and updates Git repos.
func findGitRepos(searchDir string) ([]string, error) {
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %w", searchDir, err)
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
