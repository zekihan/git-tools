package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConcurrentClonePreservesWinningCheckout(t *testing.T) {
	source := cleanupTestRepo(t)
	destination := filepath.Join(t.TempDir(), "host", "team", "checkout")

	// Both invocations have passed cloneRepo's absence checks. Suspend the
	// loser at that boundary until the winner has populated the destination.
	// This forces the stale-check interleaving without scheduler timing or sleeps.
	_, err := os.Stat(destination)
	if !os.IsNotExist(err) {
		t.Fatalf("destination should initially be absent: %v", err)
	}

	winnerDone := make(chan struct{})
	loserResult := make(chan error, 1)

	go func() {
		<-winnerDone

		_, cloneErr := cloneRepoToPath(t.Context(), source, destination)
		loserResult <- cloneErr
	}()

	got, winnerErr := cloneRepoToPath(t.Context(), source, destination)

	close(winnerDone)

	loserErr := <-loserResult

	if winnerErr != nil || got != destination {
		t.Fatalf("winning clone = %q, error = %v", got, winnerErr)
	}

	if !errors.Is(loserErr, ErrDirAlreadyExists) {
		t.Errorf("losing clone should refuse ownership: %v", loserErr)
	}

	contents, err := os.ReadFile(filepath.Join(destination, "file.txt"))
	if err != nil || string(contents) != "base\n" {
		t.Fatalf("winning checkout was damaged: contents = %q, error = %v", contents, err)
	}

	if cleanupTestGit(t, destination, "rev-parse", "HEAD") !=
		cleanupTestGit(t, source, "rev-parse", "HEAD") {
		t.Fatal("winning repository was damaged")
	}
}

func TestCloneFailureCleansOnlyOwnedDestination(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "checkout")
	marker := filepath.Join(parent, "keep.txt")

	err := os.WriteFile(marker, []byte("keep\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cloneRepoToPath(t.Context(), filepath.Join(t.TempDir(), "missing"), destination)
	if err == nil {
		t.Fatal("expected local clone failure")
	}

	_, err = os.Stat(destination)
	if !os.IsNotExist(err) {
		t.Fatalf("failed clone destination remains: %v", err)
	}

	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "keep\n" {
		t.Fatalf("parent contents were damaged: %q, %v", contents, err)
	}

	_, err = cloneRepoToPath(t.Context(), cleanupTestRepo(t), destination)
	if err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
}

func TestClonePreservesExistingDestinationErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	const repoURL = "https://example.invalid/team/repo.git"

	destination, err := getRepoPath(repoURL, "")
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(destination, defaultDirPermissions)
	if err != nil {
		t.Fatal(err)
	}

	got, err := cloneRepo(t.Context(), repoURL, "")
	if !errors.Is(err, ErrDirAlreadyExists) || got != destination {
		t.Fatalf("existing directory = %q, error = %v", got, err)
	}

	cleanupTestGit(t, destination, "init", "-b", "main")

	got, err = cloneRepo(t.Context(), repoURL, "")
	if !errors.Is(err, ErrRepoAlreadyExists) || got != destination {
		t.Fatalf("existing repository = %q, error = %v", got, err)
	}
}
