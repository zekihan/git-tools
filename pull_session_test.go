package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPullSessionDoesNotPullParentWhenCheckoutDisappears(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", "")
	source, checkout := pullSessionTestCheckout(t)
	before := cleanupTestGit(t, checkout, "rev-parse", "HEAD")
	nested := filepath.Join(checkout, "nested")
	cleanupTestGit(t, checkout, "init", nested)
	pullSessionTestCommit(t, source, "Advance remote")

	ctx, cancel := context.WithCancel(t.Context())
	session := startPullSession(ctx, nested, 1, nil)

	defer func() { cancel(); <-session.done }()

	msg := session.next()

	scan, ok := msg.(pullScanMsg)
	if !ok || scan.err != nil || len(scan.paths) != 1 || scan.paths[0] != nested {
		t.Fatalf("expected the nested checkout in the scan, got %+v", msg)
	}

	// Workers wait for the started message to be read before running Git.
	err := os.Rename(filepath.Join(nested, ".git"), filepath.Join(t.TempDir(), "moved-git-dir"))
	if err != nil {
		t.Fatal(err)
	}

	if msg = session.next(); msg != pullStartedMsg(0) {
		t.Fatalf("expected the nested checkout to start, got %+v", msg)
	}

	checkPullSessionResult(t, session, pullFailed)

	retry := session.retry(nested, 0)

	defer func() { cancel(); <-retry.done }()

	checkPullSessionResult(t, retry, pullFailed)

	if after := cleanupTestGit(t, checkout, "rev-parse", "HEAD"); after != before {
		t.Errorf("parent HEAD changed from %s to %s", before, after)
	}
}

func TestPullSessionUpdatesLinkedWorktree(t *testing.T) {
	source, checkout := pullSessionTestCheckout(t)
	linked := filepath.Join(t.TempDir(), "linked")
	cleanupTestGit(t, checkout, "switch", "--detach")
	cleanupTestGit(t, checkout, "worktree", "add", linked, "main")
	pullSessionTestCommit(t, source, "Advance remote")

	ctx, cancel := context.WithCancel(t.Context())
	session := startPullSession(ctx, linked, 1, nil)

	defer func() { cancel(); <-session.done }()

	msg := session.next()

	scan, ok := msg.(pullScanMsg)
	if !ok || scan.err != nil || len(scan.paths) != 1 || scan.paths[0] != linked {
		t.Fatalf("expected the linked worktree in the scan, got %+v", msg)
	}

	if msg = session.next(); msg != pullStartedMsg(0) {
		t.Fatalf("expected the linked worktree to start, got %+v", msg)
	}

	checkPullSessionResult(t, session, pullSucceeded)

	want := cleanupTestGit(t, source, "rev-parse", "HEAD")
	if got := cleanupTestGit(t, linked, "rev-parse", "HEAD"); got != want {
		t.Errorf("linked HEAD = %s, want %s", got, want)
	}
}

func checkPullSessionResult(t *testing.T, session *pullSession, want pullStatus) {
	t.Helper()

	msg := session.next()

	finished, ok := msg.(pullFinishedMsg)
	if !ok || finished.index != 0 {
		t.Fatalf("expected a result for the selected checkout, got %+v", msg)
	}

	if finished.result.status != want {
		t.Errorf(
			"pull status = %s, want %s\n%s",
			finished.result.status,
			want,
			finished.result.detail,
		)
	}

	<-session.done
}

func pullSessionTestCheckout(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	source := t.TempDir()
	cleanupTestGit(t, source, "init", "-b", "main")
	pullSessionTestCommit(t, source, "Initial commit")

	checkout := filepath.Join(t.TempDir(), "checkout")
	cleanupTestGit(t, source, "clone", "--", source, checkout)

	return source, checkout
}

func pullSessionTestCommit(t *testing.T, repo, subject string) {
	t.Helper()
	cleanupTestGit(
		t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
		"commit", "--allow-empty", "-m", subject,
	)
}
