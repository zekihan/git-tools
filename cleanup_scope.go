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

// cleanupCheckScope keeps recursive cleanup within its original canonical root.
// Selecting a repository directly is the explicit opt-in for its external worktrees.
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

	err = cleanupCheckPath(root, repo)
	if err != nil {
		return err
	}

	output, err := cleanupGit(ctx, repo, cleanupWorktreeKind, "list", "--porcelain", "-z")
	if err != nil {
		return err
	}

	for _, entry := range parseCleanupWorktrees(output, repo, "") {
		err = cleanupCheckPath(root, entry.path)
		if err != nil {
			return err
		}
	}

	return nil
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
