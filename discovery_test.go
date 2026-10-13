package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverNestedRepositoriesConsistently(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	nested := filepath.Join(parent, "nested")

	linked := filepath.Join(parent, "linked")
	for _, dir := range []string{filepath.Join(parent, ".git", "hidden", ".git"), filepath.Join(nested, ".git"), linked} {
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := os.WriteFile(
		filepath.Join(linked, ".git"),
		[]byte("gitdir: ../.git/worktrees/linked\n"),
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(parent, filepath.Join(nested, "loop"))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{parent, linked, nested}
	for _, start := range []string{root, parent} {
		got, err := findGitRepos(t.Context(), start)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("scan %s = %v, %v; want %v", start, got, err, want)
		}
	}
}
