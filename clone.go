package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultDirPermissions = 0o700 // Default permissions for created directories
)

var (
	ErrUnsupportedGitURL     = errors.New("unsupported Git URL format: Please use HTTPS or SSH")
	ErrFailedExtractHostname = errors.New("failed to extract hostname from the URL")
	ErrFailedExtractRepoName = errors.New("failed to extract repository name from the URL")
	ErrRepoAlreadyExists     = errors.New("repository already exists")
	ErrDirAlreadyExists      = errors.New(
		"directory already exists. Please remove it before cloning again",
	)
)

// repoURLFormat defines a structure for parsing different Git URL formats.
// It holds a compiled regular expression and a function to extract
// hostname, user, and repository name from the matches.
type repoURLFormat struct {
	regex   *regexp.Regexp
	extract func(matches []string) (hostname, user, repo string)
}

type repoParts struct {
	hostname string
	userRepo string
}

// supportedFormats is a list of parsers for different Git URL schemes.
// Each entry defines a regex to match a URL pattern and an extraction function.
//
// The repo capture group uses [^/]+ (dots allowed) so repository names such as
// "socket.io" or "next.js" are matched. A trailing ".git" suffix is stripped in
// parseRepoParts.
var supportedFormats = []repoURLFormat{
	{
		// Regex for HTTPS URLs with user and repo: https://host.com/user/repo.git
		regex: regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+)$`),
		extract: func(matches []string) (string, string, string) {
			// Matches[1]: hostname (e.g., git.com, git.com:8080)
			// matches[2]: user
			// matches[3]: repo.
			return matches[1], matches[2], matches[3]
		},
	},
	{
		// Regex for HTTPS URLs with repo only: https://host.com/repo.git
		regex: regexp.MustCompile(`^https://([^/]+)/([^/]+)$`),
		extract: func(matches []string) (string, string, string) {
			// Matches[1]: hostname (e.g., git.com, git.com:8080)
			// matches[2]: repo.
			return matches[1], "", matches[2] // user is empty
		},
	},
	{
		// Regex for git@host:path URLs. Handles optional port in host, and optional user in path.
		// Examples:
		//  - git@host.com:user/repo.git
		//  - git@host.com:repo.git
		//  - git@host.com:port:user/repo.git.
		regex: regexp.MustCompile(`^git@([\w.-]+(?:[:\d]+)?):(?:([^/]+)/)?([^/]+)$`),
		extract: func(matches []string) (string, string, string) {
			// Matches[1]: hostname (e.g., host.com, host.com:port)
			// matches[2]: user (optional, can be empty)
			// matches[3]: repo.
			return matches[1], matches[2], matches[3]
		},
	},
	{
		// Regex for ssh://git@host/path URLs.
		// Handles optional port in host, and optional user in path.
		// Examples:
		//  - ssh://git@host.com/user/repo.git
		//  - ssh://git@host.com/repo.git
		//  - ssh://git@host.com:port/user/repo.git
		regex: regexp.MustCompile(`^ssh://git@([^/]+)/(?:([^/]+)/)?([^/]+)$`),
		extract: func(matches []string) (string, string, string) {
			// Matches[1]: hostname (e.g., host.com, host.com:port)
			// matches[2]: user (optional, can be empty)
			// matches[3]: repo.
			return matches[1], matches[2], matches[3]
		},
	},
}

// getRepoPath extracts the path from a Git repository URL and an optional name.
func getRepoPath(repoURL, name string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	repoSubPath, err := getRepoSubPath(repoURL, name)
	if err != nil {
		return "", err
	}

	destination := filepath.Join(homeDir, "repos", repoSubPath)

	return destination, nil
}

func getRepoSubPath(repoURL, name string) (string, error) {
	parts, err := parseRepoParts(repoURL, name)
	if err != nil {
		return "", err
	}

	return filepath.Join(parts.hostname, parts.userRepo), nil
}

func parseRepoParts(repoURL, name string) (repoParts, error) {
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
		return repoParts{}, fmt.Errorf("%w: %s", ErrUnsupportedGitURL, repoURL)
	}

	if hostname == "" {
		return repoParts{}, fmt.Errorf("%w: %s", ErrFailedExtractHostname, repoURL)
	}

	if name != "" {
		repo = name
	}

	if repo == "" {
		return repoParts{}, fmt.Errorf("%w: %s", ErrFailedExtractRepoName, repoURL)
	}

	userRepo := repo
	if user != "" {
		userRepo = filepath.Join(user, repo)
	}

	userRepo = strings.TrimSuffix(userRepo, ".git")

	return repoParts{hostname: hostname, userRepo: userRepo}, nil
}

func cloneRepo(ctx context.Context, repoURL, name string) (string, error) {
	repoPath, err := getRepoPath(repoURL, name)
	if err != nil {
		return "", fmt.Errorf("error getting repository path: %w", err)
	}

	gitDir := filepath.Join(repoPath, ".git")

	_, err = os.Stat(gitDir)
	if err == nil {
		return repoPath, fmt.Errorf("%w: %s", ErrRepoAlreadyExists, repoPath)
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking %s: %w", gitDir, err)
	}

	_, err = os.Stat(repoPath)
	if err == nil {
		return repoPath, fmt.Errorf("%w: %s", ErrDirAlreadyExists, repoPath)
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking %s: %w", repoPath, err)
	}

	err = os.MkdirAll(repoPath, defaultDirPermissions)
	if err != nil {
		return "", fmt.Errorf("error creating directory %s: %w", repoPath, err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, repoPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Remove the directory we just created so a partial/empty checkout does
		// not block a later retry with ErrDirAlreadyExists.
		_ = os.RemoveAll(repoPath) // best-effort cleanup; original error is returned

		return "", fmt.Errorf("error cloning repository: %w\nOutput: %s", err, output)
	}

	return repoPath, nil
}
