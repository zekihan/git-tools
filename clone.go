package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// repoURLFormat defines a structure for parsing different Git URL formats.
// It holds a compiled regular expression and a function to extract
// hostname, user, and repository name from the matches.
type repoURLFormat struct {
	regex   *regexp.Regexp
	extract func(matches []string) (hostname, user, repo string)
}

// supportedFormats is a list of parsers for different Git URL schemes.
// Each entry defines a regex to match a URL pattern and an extraction function.
var supportedFormats = []repoURLFormat{
	{
		// Regex for HTTPS URLs with user and repo: https://host.com/user/repo.git
		regex: regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/.]+)(\.git)?$`),
		extract: func(matches []string) (string, string, string) {
			// matches[1]: hostname (e.g., git.com, git.com:8080)
			// matches[2]: user
			// matches[3]: repo
			return matches[1], matches[2], matches[3]
		},
	},
	{
		// Regex for HTTPS URLs with repo only: https://host.com/repo.git
		regex: regexp.MustCompile(`^https://([^/]+)/([^/.]+)(\.git)?$`),
		extract: func(matches []string) (string, string, string) {
			// matches[1]: hostname (e.g., git.com, git.com:8080)
			// matches[2]: repo
			return matches[1], "", matches[2] // user is empty
		},
	},
	{
		// Regex for git@host:path URLs. Handles optional port in host, and optional user in path.
		// Examples: git@host.com:user/repo.git, git@host.com:repo.git, git@host.com:port:user/repo.git
		regex: regexp.MustCompile(`^git@([\w.-]+(?:[:\d]+)?):(?:([^/]+)/)?([^/.]+)(\.git)?$`),
		extract: func(matches []string) (string, string, string) {
			// matches[1]: hostname (e.g., host.com, host.com:port)
			// matches[2]: user (optional, can be empty)
			// matches[3]: repo
			return matches[1], matches[2], matches[3]
		},
	},
	{
		// Regex for ssh://git@host/path URLs. Handles optional port in host, and optional user in path.
		// Examples: ssh://git@host.com/user/repo.git, ssh://git@host.com/repo.git, ssh://git@host.com:port/user/repo.git
		regex: regexp.MustCompile(`^ssh://git@([^/]+)/(?:([^/]+)/)?([^/.]+)(\.git)?$`),
		extract: func(matches []string) (string, string, string) {
			// matches[1]: hostname (e.g., host.com, host.com:port)
			// matches[2]: user (optional, can be empty)
			// matches[3]: repo
			return matches[1], matches[2], matches[3]
		},
	},
}

// getRepoPath extracts the path from a Git repository URL and an optional name.
func getRepoPath(repoURL string, name string) (string, error) {
	var hostname, user, repo string
	parsedSuccessfully := false

	for _, format := range supportedFormats {
		matches := format.regex.FindStringSubmatch(repoURL)
		if len(matches) > 0 {
			hostname, user, repo = format.extract(matches)
			parsedSuccessfully = true
			break
		}
	}

	if !parsedSuccessfully {
		return "", fmt.Errorf("unsupported Git URL format: %s. Please use HTTPS or SSH", repoURL)
	}

	if hostname == "" {
		// This check acts as a safeguard. If a regex matches, hostname should be populated.
		return "", fmt.Errorf("failed to extract hostname from the URL: %s", repoURL)
	}

	if name != "" {
		repo = name
	}

	if repo == "" {
		// This check acts as a safeguard. If a regex matches, repo should be populated.
		return "", fmt.Errorf("failed to extract repository name from the URL: %s", repoURL)
	}

	userRepo := repo
	if user != "" {
		userRepo = filepath.Join(user, repo)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	destination := filepath.Join(homeDir, "repos", hostname, userRepo)
	destination = strings.TrimSuffix(destination, ".git") // Ensures .git is not part of the final directory name

	return destination, nil
}
