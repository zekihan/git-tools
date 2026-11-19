package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRepositoryShellStartsInSelectedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}

	t.Setenv("SHELL", "/bin/sh")
	dir := filepath.Join(t.TempDir(), "repo with spaces")

	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	cmd := repositoryShell(dir)
	if len(cmd.Args) != 2 || cmd.Args[1] != "-i" {
		t.Fatalf("not interactive: %v", cmd.Args)
	}

	cmd.Args = []string{cmd.Path, "-c", "printf '%s' \"$PWD\""}

	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != dir {
		t.Fatalf("output=%q error=%v", output, err)
	}

	t.Setenv("SHELL", "")

	if repositoryShell(dir).Path != "/bin/sh" {
		t.Fatal("missing fallback shell")
	}
}

func TestRepositoryShellUsesSelectedGitCheckout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}

	t.Setenv("SHELL", "/bin/sh")
	selected, other := t.TempDir(), t.TempDir()
	cleanupTestGit(t, selected, "init", "-b", "main")
	cleanupTestGit(t, other, "init", "-b", "main")
	want := cleanupTestGit(t, selected, "rev-parse", "--show-toplevel")

	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)

	cmd := repositoryShell(selected)
	cmd.Args = []string{cmd.Path, "-c", "git rev-parse --show-toplevel"}

	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSuffix(string(output), "\n") != want {
		t.Fatalf("Git reported %q, want %q (error: %v)", output, want, err)
	}
}

func TestPullShellUsesVisibleSelectionAndProtectsActivePulls(t *testing.T) {
	model := &pullModel{hideSucceeded: true, entries: []pullEntry{
		{path: "/hidden", result: pullResult{status: pullSucceeded}},
		{path: t.TempDir(), result: pullResult{status: pullFailed, detail: "original error"}},
	}}
	if model.key("s") == nil {
		t.Fatal("shell not available for selected failure")
	}

	_, _ = model.Update(pullShellFinishedMsg{})
	if model.entries[1].result.detail != "original error" ||
		model.entries[1].result.status != pullFailed {
		t.Fatal("shell exit rewrote pull result")
	}

	if !strings.Contains(model.notice, "Returned") {
		t.Fatal("missing return notice")
	}

	for _, status := range []pullStatus{pullQueued, pullRunning} {
		model.entries[1].result.status = status
		if model.key("s") != nil || model.notice == "" {
			t.Fatal("shell allowed during pull")
		}
	}

	model.entries = nil
	if model.key("s") != nil {
		t.Fatal("shell opened without selection")
	}

	_, _ = model.Update(pullShellFinishedMsg{err: os.ErrNotExist})
	if !strings.Contains(model.notice, os.ErrNotExist.Error()) {
		t.Fatal("missing shell error")
	}
}
