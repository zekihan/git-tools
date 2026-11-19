package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupRefusesEnclosingRepoAfterCheckoutDisappears(t *testing.T) {
	for _, mode := range []string{"direct", "recursive"} {
		t.Run(mode, func(t *testing.T) {
			root, outer, inner := cleanupNestedTestRepo(t)

			dir := inner
			if mode == "recursive" {
				dir = root
			}

			entries, err := loadCleanupEntries(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}

			var selected cleanupEntry

			for _, entry := range entries {
				if entry.repo == inner && entry.branch == "feature" {
					selected = entry

					break
				}
			}

			if selected.head == "" || selected.blocked != "" {
				t.Fatalf("nested repository branch is not selectable: %+v", entries)
			}

			err = os.Rename(filepath.Join(inner, ".git"), filepath.Join(root, "moved-git-dir"))
			if err != nil {
				t.Fatal(err)
			}

			err = cleanupDelete(t.Context(), selected, false, nil)
			if !errors.Is(err, errCleanupUnsafe) {
				t.Errorf("cleanup should refuse the stale selection, got %v", err)
			}

			_, err = cleanupGit(t.Context(), outer, "rev-parse", "--verify", "refs/heads/feature")
			if err != nil {
				t.Errorf("enclosing repository lost its feature branch: %v", err)
			}
		})
	}
}

func cleanupNestedTestRepo(t *testing.T) (string, string, string) {
	t.Helper()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	outer := filepath.Join(root, "outer")

	err = os.Rename(cleanupTestRepo(t), outer)
	if err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(outer, "inner")
	cleanupTestGit(t, outer, "branch", "feature")
	cleanupTestGit(t, outer, "clone", "--", outer, inner)
	cleanupTestGit(t, inner, "remote", "set-head", "origin", "main")
	cleanupTestGit(t, inner, "branch", "feature")

	return root, outer, inner
}
