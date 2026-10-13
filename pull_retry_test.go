package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetrySelectedAfterRepair(t *testing.T) {
	source, checkout := pullTestCheckout(t)
	cleanupTestGit(t, checkout, "remote", "set-url", "origin", filepath.Join(source, "missing"))

	failure := pullRepo(t.Context(), checkout)
	if failure.status != pullFailed {
		t.Fatal("expected initial failure")
	}

	cleanupTestGit(t, checkout, "remote", "set-url", "origin", source)
	cleanupTestGit(t, source, "commit", "--allow-empty", "-m", "Advance remote")
	model := &pullModel{
		session: &pullSession{ctx: t.Context()}, done: true, hideSucceeded: true,
		entries: []pullEntry{
			{path: "/hidden", result: pullResult{status: pullSucceeded}},
			{path: checkout, result: failure},
		},
	}

	cmd := model.key("r")
	if cmd == nil || model.done || model.entries[1].result.status != pullRunning {
		t.Fatal("retry did not start")
	}

	_, cmd = model.Update(cmd())
	if model.entries[1].result.status != pullSucceeded || model.selectedIndex() != -1 {
		t.Fatalf("retry result/filter not updated: %+v", model.entries[1].result)
	}

	_, cmd = model.Update(cmd())
	if !model.done || cmd != nil || model.exitError() != nil {
		t.Fatal("retry did not finish successfully")
	}

	if !strings.Contains(model.entries[1].result.detail, "$ git pull") {
		t.Fatal("retry output missing")
	}

	if cleanupTestGit(
		t,
		checkout,
		"rev-parse",
		"HEAD",
	) != cleanupTestGit(
		t,
		source,
		"rev-parse",
		"HEAD",
	) {
		t.Fatal("retry did not pull changes")
	}

	<-model.session.done
}

func TestRetryGuardsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	model := &pullModel{
		session: &pullSession{ctx: ctx},
		cancel:  cancel,
		entries: []pullEntry{{path: t.TempDir(), result: pullResult{status: pullFailed}}},
	}
	if model.key("r") != nil {
		t.Fatal("retry started during active batch")
	}

	model.done = true

	cmd := model.key("r")
	if cmd == nil {
		t.Fatal("retry did not start")
	}

	if model.key("r") != nil || model.key("s") != nil {
		t.Fatal("allowed overlapping operation")
	}

	_, _ = model.Update(pullShellFinishedMsg{})
	model.key("q")

	if ctx.Err() == nil || !model.stopping {
		t.Fatal("quit did not cancel retry")
	}
	// Cancellation may race with a completed result; consume until the session closes.
	for cmd != nil {
		_, cmd = model.Update(cmd())
		if model.done {
			break
		}
	}

	<-model.session.done

	model.entries = nil
	if model.key("r") != nil {
		t.Fatal("retried without a selection")
	}
}
