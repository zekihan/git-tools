package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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
	ErrUnsupportedGitURL = errors.New("unsupported Git URL format: Please use HTTPS or SSH")
	ErrRepoAlreadyExists = errors.New("repository already exists")
	errInvalidRepoName   = errors.New(
		"repository name must be a single nonempty directory component",
	)
	ErrDirAlreadyExists = errors.New(
		"directory already exists. Please remove it before cloning again",
	)
)

type repoParts struct {
	hostname       string
	repositoryPath string
}

// repositoryPathPattern captures the optional namespace and the final repository name.
const repositoryPathPattern = `(?:([^/]+(?:/[^/]+)*)/)?([^/]+)$`

var scpGitURLPattern = regexp.MustCompile(`^git@([\w.-]+(?:[:\d]+)?):` + repositoryPathPattern)

// Each pattern captures the host, namespace, and repository name in that order.
var supportedFormats = []*regexp.Regexp{
	regexp.MustCompile(`^https://([^/]+)/` + repositoryPathPattern),
	scpGitURLPattern,
	regexp.MustCompile(`^ssh://git@([^/]+)/` + repositoryPathPattern),
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

	root := filepath.Join(homeDir, "repos")
	destination := filepath.Join(root, repoSubPath)

	canonicalRoot, err := canonicalPath(root)
	if err != nil {
		return "", fmt.Errorf("resolve clone root: %w", err)
	}

	canonicalDestination, err := canonicalPath(destination)
	if err != nil {
		return "", fmt.Errorf("resolve clone destination: %w", err)
	}

	if canonicalDestination == canonicalRoot ||
		!pathWithinRoot(canonicalRoot, canonicalDestination) {
		return "", fmt.Errorf("%w: destination outside clone root", errInvalidRepoName)
	}

	return destination, nil
}

func getRepoSubPath(repoURL, name string) (string, error) {
	parts, err := parseRepoParts(repoURL, name)
	if err != nil {
		return "", err
	}

	return filepath.Join(parts.hostname, parts.repositoryPath), nil
}

func parseRepoParts(repoURL, name string) (repoParts, error) {
	parseURL, err := clonePathURL(repoURL)
	if err != nil {
		return repoParts{}, fmt.Errorf("%w: %s", err, safeGitURL(repoURL))
	}

	var hostname, namespace, repo string

	for _, format := range supportedFormats {
		matches := format.FindStringSubmatch(parseURL)
		if matches != nil {
			hostname, namespace, repo = matches[1], matches[2], matches[3]

			break
		}
	}

	if hostname == "" {
		return repoParts{}, fmt.Errorf("%w: %s", ErrUnsupportedGitURL, safeGitURL(repoURL))
	}

	if name != "" {
		repo = name
	}

	repo = strings.TrimSuffix(repo, ".git")
	if !validRepoComponent(repo) || !validRepoComponent(hostname) {
		return repoParts{}, errInvalidRepoName
	}

	if namespace != "" {
		for component := range strings.SplitSeq(namespace, "/") {
			if !validRepoComponent(component) {
				return repoParts{}, errInvalidRepoName
			}
		}
	}

	return repoParts{hostname: hostname, repositoryPath: filepath.Join(namespace, repo)}, nil
}

func validRepoComponent(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\\x00") && filepath.VolumeName(name) == ""
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

	return cloneRepoToPath(ctx, repoURL, repoPath)
}

func cloneRepoToPath(ctx context.Context, repoURL, repoPath string) (string, error) {
	err := os.MkdirAll(filepath.Dir(repoPath), defaultDirPermissions)
	if err != nil {
		return "", fmt.Errorf("error creating parent directory for %s: %w", repoPath, err)
	}

	// Mkdir atomically acquires the leaf directory. MkdirAll would also succeed
	// for a competing clone's directory, allowing our failure cleanup to delete it.
	err = os.Mkdir(repoPath, defaultDirPermissions)
	if os.IsExist(err) {
		return repoPath, fmt.Errorf("%w: %s", ErrDirAlreadyExists, repoPath)
	}

	if err != nil {
		return "", fmt.Errorf("error creating directory %s: %w", repoPath, err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, repoPath)
	cmd.Env = gitCheckoutEnv(cmd.Environ())

	var output commandBuffer

	cmd.Stdout, cmd.Stderr = &output, &output

	err = cmd.Run()
	if err != nil {
		// Remove only the directory we exclusively created so a partial/empty checkout does
		// not block a later retry with ErrDirAlreadyExists.
		_ = os.RemoveAll(repoPath) // best-effort cleanup; original error is returned

		return "", fmt.Errorf(
			"error cloning repository: %w\nOutput: %s",
			err,
			cloneDiagnostics(&output, repoURL),
		)
	}

	return repoPath, nil
}

func clonePathURL(repoURL string) (string, error) {
	if !strings.HasPrefix(repoURL, "https://") {
		return repoURL, nil
	}

	parsed, err := url.Parse(repoURL)
	if err != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrUnsupportedGitURL
	}

	parsed.User = nil

	return parsed.String(), nil
}
