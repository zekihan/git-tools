package main

import (
	"context"
	"fmt"
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

// cleanupCheckScope confines discovered repositories, while allowing their linked worktrees.
func cleanupCheckScope(ctx context.Context, repo, root string) error {
	if root == "" {
		return nil
	}

	identity, err := cleanupRepoIdentity(ctx, repo)
	if err != nil {
		return err
	}

	err = cleanupCheckPath(root, identity)
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
