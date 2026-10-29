package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCloneRejectsUnsafeHostsBeforeCreatingDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	hosts := []string{".", "..", `host\..\..\outside`, `\server\share`, "bad\x00host"}
	if runtime.GOOS == "windows" {
		hosts = append(hosts, "C:", "C:outside", `C:\outside`, `\\server\share`)
	}

	for _, host := range hosts {
		repoURL := "ssh://git@" + host + "/team/repo.git"

		_, err := cloneRepo(t.Context(), repoURL, "")
		if !errors.Is(err, errInvalidRepoName) {
			t.Errorf("host %q: expected invalid component, got %v", host, err)
		}
	}

	_, err := os.Stat(filepath.Join(home, "repos"))
	if !os.IsNotExist(err) {
		t.Fatalf("rejected URLs created clone root: %v", err)
	}
}

func TestCloneDestinationStaysBelowCanonicalRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	root, err := canonicalPath(filepath.Join(home, "repos"))
	if err != nil {
		t.Fatal(err)
	}

	for _, repoURL := range []string{
		"https://github.com/team/repo.git", "https://example.com/repo.git",
		"ssh://git@example.com/team/repo.git", "git@example.com:team/repo.git",
		"ssh://git@example.com/socket.io.git",
	} {
		destination, pathErr := getRepoPath(repoURL, "")
		if pathErr != nil {
			t.Fatal(pathErr)
		}

		resolved, resolveErr := canonicalPath(destination)
		if resolveErr != nil || resolved == root || !pathWithinRoot(root, resolved) {
			t.Fatalf("URL %q escaped root: %q, %v", repoURL, resolved, resolveErr)
		}
	}
}

func TestCloneRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Windows symlink privileges")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(home, "repos")

	err := os.Mkdir(root, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		link := filepath.Join(root, "example.com")

		err = os.Symlink(target, link)
		if err != nil {
			t.Fatal(err)
		}

		_, err = cloneRepo(t.Context(), "ssh://git@example.com/team/repo", "")
		if err == nil {
			t.Fatal("clone accepted an escaping or dangling symlink")
		}

		err = os.Remove(link)
		if err != nil {
			t.Fatal(err)
		}

		_, err = os.Stat(filepath.Join(target, "team"))
		if !os.IsNotExist(err) {
			t.Fatalf("clone wrote through symlink: %v", err)
		}
	}
}

func TestPathWithinRootRejectsSiblingPrefix(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repos")
	if pathWithinRoot(root, root+"-outside") {
		t.Fatal("sibling prefix accepted as descendant")
	}
}

func FuzzClonePathConfined(f *testing.F) {
	home := f.TempDir()
	f.Setenv("HOME", home)
	f.Setenv("USERPROFILE", home)

	for _, repoURL := range []string{
		"https://github.com/team/repo.git", "git@example.com:team/repo.git",
		"ssh://git@example.com/team/repo.git", `ssh://git@host\..\../team/repo.git`,
		"ssh://git@../team/repo.git", "ssh://git@C:/team/repo.git",
	} {
		f.Add(repoURL, "")
	}

	f.Fuzz(func(t *testing.T, repoURL, name string) {
		destination, err := getRepoPath(repoURL, name)
		if err != nil {
			return
		}

		root, err := canonicalPath(filepath.Join(home, "repos"))
		if err != nil {
			t.Fatal(err)
		}

		resolved, err := canonicalPath(destination)
		if err != nil || resolved == root || !pathWithinRoot(root, resolved) {
			t.Fatalf("accepted URL %q, name %q escaped root: %q, %v", repoURL, name, resolved, err)
		}
	})
}

func TestCleanupRecursiveRejectsExternalGitdir(t *testing.T) {
	root := t.TempDir()
	external := cleanupTestRepo(t)
	cleanupTestGit(t, external, "branch", "keep")

	planted := filepath.Join(root, "planted")

	err := os.Mkdir(planted, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(planted, ".git"),
		[]byte("gitdir: "+filepath.Join(external, ".git")+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := loadCleanupEntries(t.Context(), root)
	if err != nil || len(entries) != 1 ||
		!strings.Contains(entries[0].blocked, "outside recursive cleanup root") {
		t.Fatalf("external gitdir not marked out of scope: %+v, %v", entries, err)
	}

	err = cleanupDelete(t.Context(), entries[0], true, true)
	if !errors.Is(err, errCleanupUnsafe) {
		t.Fatalf("force bypassed scope boundary: %v", err)
	}

	cleanupTestGit(t, external, "rev-parse", "--verify", "refs/heads/keep")

	// An explicitly selected external repository retains the normal local workflow.
	local, err := loadCleanupEntries(t.Context(), external)
	if err != nil || len(local) != 2 || local[1].scope != "" || local[1].blocked != "" {
		t.Fatalf("explicit repository selection unavailable: %+v, %v", local, err)
	}
}

func TestCleanupRecursiveRetainsInRootLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")

	err := os.Rename(cleanupTestRepo(t), repo)
	if err != nil {
		t.Fatal(err)
	}

	linked := filepath.Join(root, "linked")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", linked)

	entries, err := loadCleanupEntries(t.Context(), root)
	if err != nil || len(entries) != 1 || entries[0].branch != "feature" ||
		entries[0].blocked != "" ||
		entries[0].scope == "" {
		t.Fatalf("in-root linked worktree unavailable: %+v, %v", entries, err)
	}

	_, err = cleanupRisks(t.Context(), entries[0], true)
	if err != nil {
		t.Fatalf("in-root candidate refused: %v", err)
	}

	// An unrelated external worktree must not invalidate an in-root candidate.
	external := filepath.Join(t.TempDir(), "external")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "outside", external)

	err = cleanupDelete(t.Context(), entries[0], true, false)
	if err != nil {
		t.Fatalf("external worktree blocked in-root deletion: %v", err)
	}
}

func TestCleanupRecursiveConfirmsExternalLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")

	err := os.Rename(cleanupTestRepo(t), repo)
	if err != nil {
		t.Fatal(err)
	}

	external := filepath.Join(t.TempDir(), "external")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "outside", external)

	external, err = canonicalPath(external)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := loadCleanupEntries(t.Context(), root)
	if err != nil || len(entries) != 1 || entries[0].path != external || entries[0].blocked != "" {
		t.Fatalf("external linked worktree unavailable: %+v, %v", entries, err)
	}

	risks, err := cleanupRisks(t.Context(), entries[0], true)
	if err != nil || len(risks) != 1 ||
		!strings.Contains(risks[0], "outside recursive cleanup root") {
		t.Fatalf("external worktree needs explicit risk confirmation: %v, %v", risks, err)
	}

	err = cleanupDelete(t.Context(), entries[0], true, false)
	if !errors.Is(err, errCleanupUnsafe) {
		t.Fatalf("external worktree removed without second confirmation: %v", err)
	}

	_, err = os.Stat(external)
	if err != nil {
		t.Fatalf("unconfirmed external worktree removed: %v", err)
	}

	err = cleanupDelete(t.Context(), entries[0], true, true)
	if err != nil {
		t.Fatalf("confirmed external worktree deletion refused: %v", err)
	}

	_, err = os.Stat(external)
	if !os.IsNotExist(err) {
		t.Fatalf("confirmed external worktree remains: %v", err)
	}
}
