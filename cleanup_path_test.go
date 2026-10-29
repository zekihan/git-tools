package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPreservesRepositoryPathWhitespace(t *testing.T) {
	for _, suffix := range []string{" ", "\t", "\n"} {
		t.Run("suffix="+suffix, func(t *testing.T) {
			root := t.TempDir()
			sibling := filepath.Join(root, "repo")
			repo := sibling + suffix

			for _, path := range []string{repo, sibling} {
				fixture := cleanupTestRepo(t)

				err := os.Rename(fixture, path)
				if err != nil {
					t.Fatal(err)
				}
			}

			cleanupTestGit(t, repo, "branch", "wanted")
			cleanupTestGit(t, sibling, "branch", "unrelated")

			realRepo, err := filepath.EvalSymlinks(repo)
			if err != nil {
				t.Fatal(err)
			}

			t.Run("default directory", func(t *testing.T) {
				got, err := cleanupDefaultDir(t.Context(), repo)
				if err != nil || got != realRepo {
					t.Fatalf("default directory = %q, %v; want %q", got, err, realRepo)
				}
			})

			t.Run("candidate selection and deletion", func(t *testing.T) {
				cleanupTestWhitespaceSelection(t, realRepo, sibling)
			})
		})
	}
}

func cleanupTestWhitespaceSelection(t *testing.T, repo, sibling string) {
	t.Helper()

	entries, err := loadCleanupEntries(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}

	var wanted cleanupEntry

	for _, entry := range entries {
		if entry.repo != repo {
			t.Fatalf("cleanup selected another repository: %+v", entry)
		}

		if entry.branch == "wanted" {
			wanted = entry
		}
	}

	if wanted.branch == "" {
		t.Fatal("requested repository's branch missing")
	}

	err = cleanupDelete(t.Context(), wanted, false, false)
	if err != nil {
		t.Fatal(err)
	}

	cleanupTestGit(t, sibling, "rev-parse", "--verify", "refs/heads/unrelated")

	_, err = cleanupGit(t.Context(), repo, "rev-parse", "--verify", "refs/heads/wanted")
	if err == nil {
		t.Fatal("selected branch survived deletion")
	}
}
