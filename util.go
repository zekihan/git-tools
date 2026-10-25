package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

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

		subRepos, err := findGitRepos(ctx, fullPath)
		if err != nil {
			return nil, err
		}

		repos = append(repos, subRepos...)
	}

	return repos, nil
}

const commandOutputLimit = 16384

// Keep the buffer private: promoted ReadFrom and WriteString methods would bypass Write.
type commandBuffer struct {
	data      bytes.Buffer
	truncated bool
}

func (buffer *commandBuffer) Len() int {
	return buffer.data.Len()
}

func (buffer *commandBuffer) String() string {
	return buffer.data.String()
}

func (buffer *commandBuffer) Write(data []byte) (int, error) {
	size := len(data)

	remaining := commandOutputLimit - buffer.Len()
	if size > remaining {
		buffer.truncated = true
	}

	if remaining > 0 {
		_, _ = buffer.data.Write(data[:min(remaining, size)])
	}

	return size, nil
}
