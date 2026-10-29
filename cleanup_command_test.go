package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupBrokenRefWarningDoesNotCreateCandidates(t *testing.T) {
	for _, test := range []struct {
		name  string
		extra bool
		count int
	}{
		{name: "main-only", count: 1},
		{name: "with-feature", extra: true, count: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := cleanupTestRepo(t)
			if test.extra {
				cleanupTestGit(t, repo, "branch", "feature")
			}
			// Git warns on stderr and succeeds when an unrelated loose ref is broken.
			err := os.WriteFile(
				filepath.Join(repo, ".git", "refs", "heads", "broken"),
				[]byte("broken\n"),
				0o600,
			)
			if err != nil {
				t.Fatal(err)
			}

			candidates, err := cleanupRepoHasCandidates(t.Context(), repo)
			if err != nil || candidates != test.extra {
				t.Fatalf("candidates = %v, want %v; error = %v", candidates, test.extra, err)
			}

			entries, err := cleanupEntriesForRepo(t.Context(), repo)
			if err != nil {
				t.Fatal(err)
			}

			if len(entries) != test.count {
				t.Fatalf("want %d real entries, got %+v", test.count, entries)
			}

			if test.extra {
				err = cleanupDelete(t.Context(), cleanupTestEntry(t, repo, "feature"), false, false)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCleanupGitPreservesFailureDiagnostics(t *testing.T) {
	repo := cleanupTestRepo(t)
	output, err := cleanupGit(
		t.Context(),
		repo,
		"worktree",
		"remove",
		"--",
		filepath.Join(repo, "missing"),
	)

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() == 0 {
		t.Fatalf("want wrapped Git exit error, got %v", err)
	}

	if output != "" || !strings.Contains(err.Error(), "not a working tree") {
		t.Fatalf("output = %q, error = %v", output, err)
	}
}
