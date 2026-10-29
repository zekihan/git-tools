package main

import (
	"strings"
	"testing"
)

func TestPullPreservesWhitespaceInGitNames(t *testing.T) {
	for _, scenario := range []string{"branch", "remote"} {
		t.Run(scenario, func(t *testing.T) {
			source, checkout := pullTestCheckout(t)

			branch, remote := "main", "origin"
			if scenario == "branch" {
				branch = "main\u00a0"
				cleanupTestGit(t, source, "branch", "-m", branch)
				cleanupTestGit(t, checkout, "branch", "-m", branch)
				cleanupTestGit(
					t,
					checkout,
					"config",
					"branch."+branch+".merge",
					"refs/heads/"+branch,
				)
				cleanupTestGit(t, checkout, "fetch", "origin")
				cleanupTestGit(t, checkout, "remote", "set-head", "origin", "-a")
			} else {
				remote = "my\u00a0remote"
				cleanupTestGit(t, checkout, "remote", "rename", "origin", remote)
			}

			cleanupTestGit(t, source, "commit", "--allow-empty", "-m", "Advance remote")

			result := pullRepo(t.Context(), checkout)
			if result.status != pullSucceeded || result.branch != branch ||
				result.remote != remote {
				t.Fatalf("pull with whitespace in %s failed: %+v", scenario, result)
			}

			if !strings.Contains(result.detail, "$ git pull") {
				t.Fatal("pull did not run")
			}

			if got, want := cleanupTestGit(
				t,
				checkout,
				"rev-parse",
				"HEAD",
			), cleanupTestGit(
				t,
				source,
				"rev-parse",
				"HEAD",
			); got != want {
				t.Fatalf("checkout did not advance: got %s, want %s", got, want)
			}
		})
	}
}
