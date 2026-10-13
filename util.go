package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// isGitDir checks if a directory is a Git repository.
func isGitDir(dir string) bool {
	gitPath := filepath.Join(dir, ".git")

	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}

	return info.IsDir() || info.Mode().IsRegular()
}

// findGitRepos recursively discovers repositories without entering their Git metadata.
func findGitRepos(ctx context.Context, searchDir string) ([]string, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("scan repositories: %w", err)
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %w", searchDir, err)
	}

	repos := make([]string, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			if entry.Name() == ".git" && entry.Type().IsRegular() {
				repos = append(repos, searchDir)
			}

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

		subRepos, err := findGitRepos(ctx, fullPath)
		if err != nil {
			return nil, err
		}

		repos = append(repos, subRepos...)
	}

	return repos, nil
}

const commandOutputLimit = 16384

type commandBuffer struct{ bytes.Buffer }

func (buffer *commandBuffer) Write(data []byte) (int, error) {
	size := len(data)

	remaining := commandOutputLimit - buffer.Len()
	if remaining > 0 {
		_, _ = buffer.Buffer.Write(data[:min(remaining, size)])
	}

	return size, nil
}
