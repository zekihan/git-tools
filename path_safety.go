package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// canonicalPath resolves existing ancestors even when the final path does not exist yet.
// A dangling symlink is an error, not a missing directory to append lexically.
func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	_, statErr := os.Lstat(absolute)
	if !os.IsNotExist(statErr) || filepath.Dir(absolute) == absolute {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	parent, err := canonicalPath(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}

	return filepath.Join(parent, filepath.Base(absolute)), nil
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)

	return err == nil && filepath.IsLocal(relative)
}
