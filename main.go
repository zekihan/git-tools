package main

import (
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
		return false, fmt.Errorf("error checking remotes: %w", err)
	}
	if remoteOutput == "" {
		return true, fmt.Errorf("no remote found")
	}

	// Check if the remote is reachable
	if _, err := runCommand(dir, "git", "remote", "show", "origin"); err != nil {
		return false, fmt.Errorf("error checking remote: %w", err)
	}

	// Skip if not in main or master branch
	branch, err := runCommand(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return false, fmt.Errorf("error getting current branch: %w\nOutput: %s", err, branch)
	}
	if branch != "main" && branch != "master" {
		return true, fmt.Errorf("not on main or master branch")
	}

	// Skip if there are local changes
	status, err := runCommand(dir, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("error checking status: %w", err)
	}
	if status != "" {
		return true, fmt.Errorf("local changes detected")
	}

	fmt.Printf("Fetching in: %s\n", dir)
	_, err = runCommand(dir, "git", "fetch", "--all")
	if err != nil {
		return false, fmt.Errorf("error fetching: %w", err)
	}

	fmt.Printf("Pulling in: %s\n", dir)
	_, err = runCommand(dir, "git", "pull")
	if err != nil {
		return false, fmt.Errorf("error pulling: %w", err)
	}

	return true, nil
}

// findGitRepos recursively walks through directories and updates Git repos.
func findGitRepos(searchDir string) ([]string, error) {
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, fmt.Errorf("Error reading directory %s: %v", searchDir, err)
	}

	repos := make([]string, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
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
			fmt.Printf("Warning while pulling in %s: %v\n", repo, err)
		} else {
			fmt.Printf("Successfully pulled in %s\n", repo)
		}
	}

	return nil
}

func main() {
	searchDir := "$HOME/repos"
	if len(os.Args) > 1 {
		searchDir = os.Args[1]
	}

	searchDir = os.ExpandEnv(searchDir)
	absSearchDir, err := filepath.Abs(searchDir)
	if err != nil {
		fmt.Printf("Error getting absolute path for %s: %v\n", searchDir, err)
		os.Exit(1)
	}

	fmt.Printf("Starting git pull in directory: %s\n", absSearchDir)

	if err := run(absSearchDir); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Git pull script completed.")
}
