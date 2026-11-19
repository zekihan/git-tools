package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func cleanupScopeRoot(dir string, local bool) (string, error) {
	if local {
		return "", nil
	}

	root, err := canonicalPath(dir)
	if err != nil {
		return "", fmt.Errorf("resolve cleanup root: %w", err)
	}

	return root, nil
}

// cleanupCheckScope verifies the selected repository and confines recursive discovery.
func cleanupCheckScope(ctx context.Context, repo, root string) error {
	gitDir := ".git"

	_, err := os.Lstat(filepath.Join(repo, gitDir))
	if os.IsNotExist(err) {
		// The primary worktree record can point to a bare or separate Git directory.
		gitDir = "."
	} else if err != nil {
		return fmt.Errorf("%w: inspect selected repository: %w", errCleanupUnsafe, err)
	}

	// An explicit Git directory prevents discovery from falling back to an enclosing checkout.
	commonDir, err := cleanupGit(
		ctx, repo, "--git-dir="+gitDir, "rev-parse", "--path-format=absolute", "--git-common-dir",
	)
	if err != nil {
		return fmt.Errorf("%w: selected repository unavailable; refresh and review again: %w",
			errCleanupUnsafe, err)
	}

	if root == "" {
		return nil
	}

	err = cleanupCheckPath(root, commonDir)
	if err != nil {
		return err
	}

	return cleanupCheckPath(root, repo)
}

func cleanupCheckPath(root, path string) error {
	resolved, err := canonicalPath(path)
	if err != nil {
		return fmt.Errorf("%w: resolve cleanup path: %w", errCleanupUnsafe, err)
	}

	if !pathWithinRoot(root, resolved) {
		return fmt.Errorf(
			"%w: outside recursive cleanup root: %s; select repository directly",
			errCleanupUnsafe, resolved,
		)
	}

	return nil
}
