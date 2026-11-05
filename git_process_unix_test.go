//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPullKillsGitProcessGroupOnCancel(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child")
	script := "#!/bin/sh\nsleep 30 &\necho $! > \"$PULL_TEST_CHILD\"\nwait\n"

	//nolint:gosec // The fixture must be executable to stand in for Git.
	err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PULL_TEST_CHILD", pidFile)

	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan pullResult, 1)

	go func() { finished <- pullRepo(ctx, dir) }()

	var pid int

	waitUntil(t, "fixture child never started", func() bool {
		raw, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))

		return pid != 0
	})

	cancel()
	<-finished

	waitUntil(t, "Git child survived cancellation", func() bool {
		return syscall.Kill(pid, 0) != nil
	})
}

func waitUntil(t *testing.T, failure string, done func() bool) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Second)

	for !done() {
		select {
		case <-ticker.C:
		case <-timeout:
			t.Fatal(failure)
		}
	}
}
