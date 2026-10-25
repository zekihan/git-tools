package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

func pullTestCheckout(t *testing.T) (string, string) {
	t.Helper()
	source := cleanupTestRepo(t)
	checkout := filepath.Join(t.TempDir(), "checkout")
	cleanupTestGit(t, source, "clone", source, checkout)

	return source, checkout
}

func TestPullUpdatesAndCapturesOutput(t *testing.T) {
	source, checkout := pullTestCheckout(t)

	const incomingContent = "new upstream file\n"

	err := os.WriteFile(filepath.Join(source, "incoming.txt"), []byte(incomingContent), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cleanupTestGit(t, source, "add", "incoming.txt")
	cleanupTestGit(t, source, "commit", "-m", "Advance remote")

	result := pullRepo(t.Context(), checkout)
	if result.status != pullSucceeded || result.branch != "main" || result.remote != "origin" {
		t.Fatalf("unexpected result: %+v", result)
	}

	if !strings.Contains(result.detail, "$ git pull") ||
		!strings.Contains(result.detail, "Fast-forward") {
		t.Fatalf("missing git output: %s", result.detail)
	}

	if cleanupTestGit(
		t,
		source,
		"rev-parse",
		"HEAD",
	) != cleanupTestGit(
		t,
		checkout,
		"rev-parse",
		"HEAD",
	) {
		t.Fatal("checkout did not advance")
	}

	contents, err := os.ReadFile(filepath.Join(checkout, "incoming.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if string(contents) != incomingContent {
		t.Fatalf("incoming file missing or changed: %q", contents)
	}
}

func TestPullPreservesUntrackedAndIgnoredFiles(t *testing.T) {
	for _, name := range []string{"untracked", "ignored"} {
		t.Run(name, func(t *testing.T) {
			source, checkout := pullTestCheckout(t)

			const (
				path         = "local.txt"
				localContent = "local data that must survive\n"
			)

			if name == "ignored" {
				err := os.WriteFile(
					filepath.Join(checkout, ".git", "info", "exclude"),
					[]byte(path+"\n"),
					0o600,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			err := os.WriteFile(filepath.Join(checkout, path), []byte(localContent), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			if name == "ignored" {
				cleanupTestGit(t, checkout, "check-ignore", path)
			}

			err = os.WriteFile(filepath.Join(source, path), []byte("upstream data\n"), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			cleanupTestGit(t, source, "add", path)
			cleanupTestGit(t, source, "commit", "-m", "Track colliding path")
			before := cleanupTestGit(t, checkout, "rev-parse", "HEAD")

			result := pullRepo(t.Context(), checkout)
			if result.status != pullSkipped || strings.Contains(result.detail, "$ git pull") {
				t.Fatalf("unsafe pull: %+v", result)
			}

			contents, err := os.ReadFile(filepath.Join(checkout, path))
			if err != nil {
				t.Fatal(err)
			}

			if string(contents) != localContent {
				t.Fatalf("local file changed: %q", contents)
			}

			if cleanupTestGit(t, checkout, "rev-parse", "HEAD") != before {
				t.Fatal("skipped repo changed")
			}
		})
	}
}

func TestPullSkipsLocalChangesAndOtherBranches(t *testing.T) {
	for _, scenario := range []string{"dirty", "branch"} {
		t.Run(scenario, func(t *testing.T) {
			_, checkout := pullTestCheckout(t)
			if scenario == "dirty" {
				err := os.WriteFile(
					filepath.Join(checkout, "file.txt"),
					[]byte("local changes\n"),
					0o600,
				)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				cleanupTestGit(t, checkout, "checkout", "-b", "feature")
			}

			before := cleanupTestGit(t, checkout, "rev-parse", "HEAD")

			result := pullRepo(t.Context(), checkout)
			if result.status != pullSkipped || strings.Contains(result.detail, "$ git pull") {
				t.Fatalf("unsafe pull: %+v", result)
			}

			if cleanupTestGit(t, checkout, "rev-parse", "HEAD") != before {
				t.Fatal("skipped repo changed")
			}
		})
	}
}

func TestPullErrorsAndCanceledContext(t *testing.T) {
	repo := cleanupTestRepo(t)

	result := pullRepo(t.Context(), repo)
	if result.status != pullFailed || !strings.Contains(result.detail, "no remote found") {
		t.Fatalf("%+v", result)
	}

	cleanupTestGit(t, repo, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing"))
	cleanupTestGit(t, repo, "config", "branch.main.remote", "origin")
	cleanupTestGit(t, repo, "config", "branch.main.merge", "refs/heads/main")

	result = pullRepo(t.Context(), repo)
	if result.status != pullFailed || !strings.Contains(result.detail, "$ git pull") {
		t.Fatalf("%+v", result)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result = pullRepo(ctx, repo)
	if result.status != pullCanceled {
		t.Fatalf("%+v", result)
	}
}

func TestPullSessionBoundsWorkersAndCancels(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one", "two", "three"} {
		dir := filepath.Join(root, name)

		err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700)
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	session := startPullSession(ctx, root, 1)

	defer func() { cancel(); <-session.done }()

	active, completed := 0, 0

	for msg := range session.events {
		switch msg.(type) {
		case pullStartedMsg:
			active++
			if active > 1 {
				t.Fatal("worker limit exceeded")
			}
		case pullFinishedMsg:
			active--
			completed++
		}
	}

	if completed != 3 || active != 0 {
		t.Fatalf("active=%d completed=%d", active, completed)
	}
	// Cancellation must unblock a producer even when the UI stops consuming events.
	session = startPullSession(ctx, root, 1)

	cancel()

	select {
	case <-session.done:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not stop")
	}
}

func TestPullModelKeepsResultsAndShowsSelectedDetails(t *testing.T) {
	session := &pullSession{events: make(chan tea.Msg)}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	model := &pullModel{session: session, cancel: cancel, width: 100, height: 24}
	_, _ = model.Update(pullScanMsg{paths: []string{"/repos/a", "/repos/b"}})

	_, _ = model.Update(pullStartedMsg(0))
	if model.entries[0].result.status != pullRunning ||
		model.entries[1].result.status != pullQueued {
		t.Fatal("wrong initial states")
	}

	_, _ = model.Update(
		pullFinishedMsg{
			index:  0,
			result: pullResult{status: pullSucceeded, detail: "first result"},
		},
	)
	_, _ = model.Update(
		pullFinishedMsg{index: 1, result: pullResult{status: pullFailed, detail: "second error"}},
	)

	_, cmd := model.Update(pullDoneMsg{})
	if cmd != nil {
		t.Fatal("completion closed the results screen")
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if !strings.Contains(model.View().Content, "second error") ||
		strings.Contains(model.View().Content, "first result") {
		t.Fatal("selection did not update details")
	}

	if !errors.Is(model.exitError(), ErrSomeReposFailed) {
		t.Fatal("failed pull returned success")
	}

	model.done = false

	_, _ = model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if ctx.Err() == nil || !errors.Is(model.exitError(), context.Canceled) {
		t.Fatal("quit did not cancel work")
	}
}

func TestPullLayoutFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{20, 8}, {60, 16}, {80, 24}, {120, 32}} {
		model := &pullModel{
			width:  size[0],
			height: size[1],
			dir:    "/repos",
			entries: []pullEntry{
				{
					path: "/repos/" + strings.Repeat("name", 30),
					result: pullResult{
						status: pullFailed,
						detail: strings.Repeat("long output ", 40) + "\nerror",
					},
				},
			},
		}

		lines := strings.Split(strings.TrimSuffix(model.View().Content, "\n"), "\n")
		if len(lines) > model.height {
			t.Fatalf("height overflow at %v: %d", size, len(lines))
		}

		for _, line := range lines {
			if ansi.StringWidth(line) > model.width {
				t.Fatalf("width overflow at %v: %q", size, line)
			}
		}
	}
}

func TestDiscoverLinkedWorktreeAndCancel(t *testing.T) {
	repo := cleanupTestRepo(t)
	worktree := filepath.Join(t.TempDir(), "linked")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", worktree)

	repos, err := findGitRepos(t.Context(), worktree)
	if err != nil || len(repos) != 1 || repos[0] != worktree {
		t.Fatalf("repos=%v error=%v", repos, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = findGitRepos(ctx, repo)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}

func TestPullCancelsRunningGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	script := "#!/bin/sh\nprintf started > \"$PULL_TEST_STARTED\"\nexec sleep 30\n"

	//nolint:gosec // The fixture must be executable to test cancellation of a running process.
	err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PULL_TEST_STARTED", marker)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	finished := make(chan pullResult, 1)
	go func() { finished <- pullRepo(ctx, dir) }()

	timeout := time.NewTimer(6 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err = os.Stat(marker)
			if err == nil {
				cancel()
			}
		case result := <-finished:
			if result.status != pullCanceled {
				t.Fatalf("unexpected result: %+v", result)
			}

			_, err = os.Stat(marker)
			if err != nil {
				t.Fatal("Git never started")
			}

			return
		case <-timeout.C:
			t.Fatal("Git did not stop")
		}
	}
}

func TestPullSessionEmptyAndInvalidDirectory(t *testing.T) {
	empty := t.TempDir()
	for _, dir := range []string{empty, filepath.Join(empty, "missing")} {
		session := startPullSession(t.Context(), dir, 2)

		msg, ok := session.next().(pullScanMsg)
		if !ok {
			t.Fatal("missing scan result")
		}

		if len(msg.paths) != 0 || (msg.err != nil) != (dir != empty) {
			t.Fatalf("unexpected scan: %+v", msg)
		}

		if _, ok = session.next().(pullDoneMsg); !ok {
			t.Fatal("session did not finish")
		}

		<-session.done
	}
}

func TestPullSkipsMismatchedUpstream(t *testing.T) {
	for _, upstream := range []string{"origin/feature", "other/main"} {
		t.Run(upstream, func(t *testing.T) {
			source, checkout := pullTestCheckout(t)
			before := cleanupTestGit(t, checkout, "rev-parse", "HEAD")
			cleanupTestGit(t, source, "commit", "--allow-empty", "-m", "Advance remote")
			cleanupTestGit(t, source, "branch", "feature")
			cleanupTestGit(t, checkout, "remote", "add", "other", source)
			cleanupTestGit(t, checkout, "fetch", "--all")
			cleanupTestGit(t, checkout, "branch", "--set-upstream-to="+upstream, "main")

			result := pullRepo(t.Context(), checkout)
			if result.status != pullSkipped ||
				!strings.Contains(result.detail, "does not match default branch") ||
				strings.Contains(result.detail, "$ git pull") {
				t.Fatalf("unexpected result: %+v", result)
			}

			if got := cleanupTestGit(t, checkout, "rev-parse", "HEAD"); got != before {
				t.Fatal("mismatched upstream changed HEAD")
			}
		})
	}
}

func TestPullMissingUpstream(t *testing.T) {
	_, checkout := pullTestCheckout(t)
	cleanupTestGit(t, checkout, "branch", "--unset-upstream")

	result := pullRepo(t.Context(), checkout)
	if result.status != pullFailed ||
		!strings.Contains(result.detail, ErrNoUpstreamBranch.Error()) ||
		strings.Contains(result.detail, "$ git pull") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestPullSkipsDetachedHead(t *testing.T) {
	_, checkout := pullTestCheckout(t)
	cleanupTestGit(t, checkout, "checkout", "--detach")
	before := cleanupTestGit(t, checkout, "rev-parse", "HEAD")

	result := pullRepo(t.Context(), checkout)
	if result.status != pullSkipped || !strings.Contains(result.detail, "detached HEAD") ||
		strings.Contains(result.detail, "$ git pull") {
		t.Fatalf("unexpected result: %+v", result)
	}

	if got := cleanupTestGit(t, checkout, "rev-parse", "HEAD"); got != before {
		t.Fatal("detached checkout changed")
	}

	model := pullModel{entries: []pullEntry{{result: result}}}

	err := model.exitError()
	if err != nil {
		t.Fatalf("detached checkout failed the batch: %v", err)
	}
}

func TestPullHandlesBranchTagCollisions(t *testing.T) {
	for _, tag := range []string{"main", "origin/main"} {
		t.Run(tag, func(t *testing.T) {
			source, checkout := pullTestCheckout(t)
			cleanupTestGit(t, checkout, "update-ref", "refs/tags/"+tag, "HEAD")
			cleanupTestGit(t, source, "commit", "--allow-empty", "-m", "Advance source")

			result := pullRepo(t.Context(), checkout)
			if result.status != pullSucceeded || result.branch != "main" {
				t.Fatalf("pull with colliding tag failed: %+v", result)
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
