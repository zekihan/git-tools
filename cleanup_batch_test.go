package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

func TestCleanupMultiSelection(t *testing.T) {
	entries := []cleanupEntry{
		{branch: "first", head: "1"},
		{branch: "second", head: "2"},
		{branch: "main", head: "3", blocked: cleanupMainWorktree},
		{blocked: "unavailable"},
	}
	model := cleanupModel{entries: entries}
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	marked := cleanupTestModel(t, updated)
	if len(marked.selected) != 1 || len(model.selected) != 0 {
		t.Fatal("mark changed original snapshot")
	}

	model = marked.toggleSelection(true)
	if len(model.selected) != 2 || len(marked.selected) != 1 {
		t.Fatal("select all included protected entries or changed snapshot")
	}

	model = model.toggleSelection(true)
	if len(model.selected) != 0 {
		t.Fatal("all did not clear selection")
	}

	model = marked
	model.cursor = 1
	updated, _ = model.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})

	model = cleanupTestModel(t, updated)
	if !model.confirm || !model.both || len(model.pendingBatch) != 1 ||
		model.pending != entries[0] {
		t.Fatal("cleanup did not target marked item")
	}

	model.cursor = 0

	model = model.toggleSelection(false)
	if model.pendingEntries()[0] != entries[0] {
		t.Fatal("pending target changed after unmarking")
	}
}

//nolint:gocyclo // Exercise both confirmations and the complete destructive batch lifecycle.
func TestCleanupBatchConfirmationAndDeletion(t *testing.T) {
	repo := cleanupTestRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	cleanupTestGit(t, repo, "branch", "keep")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", linked)
	cleanupTestGit(t, repo, "branch", "second")

	err := os.WriteFile(filepath.Join(linked, "untracked"), []byte("local"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := cleanupEntriesForRepo(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}

	model := cleanupModel{
		ctx:      t.Context(),
		dir:      repo,
		entries:  entries,
		selected: make(map[cleanupEntry]bool),
	}
	for _, entry := range entries {
		if entry.branch == "feature" || entry.branch == "second" {
			model.selected[entry] = true
		}
	}

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})

	model = cleanupTestModel(t, updated)
	if !strings.Contains(model.action(), "2 selected items") {
		t.Fatal("batch action missing targets")
	}

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	model = cleanupTestModel(t, updated)
	updated, cmd = model.Update(cmd())

	model = cleanupTestModel(t, updated)
	if cmd != nil || !model.forceConfirm || len(model.risks) == 0 {
		t.Fatal("dirty batch did not need second confirmation")
	}

	if got := cleanupTestGit(t, repo, "branch", "--list", "second"); got == "" {
		t.Fatal("first confirmation removed branch")
	}

	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	model = cleanupTestModel(t, updated)
	updated, _ = model.Update(cmd())

	model = cleanupTestModel(t, updated)
	if model.notice != "Removed 2 selected items." || len(model.selected) != 0 {
		t.Fatalf("batch result: %s", model.notice)
	}

	if got := cleanupTestGit(t, repo, "branch", "--list", "feature", "second"); got != "" {
		t.Fatalf("selected branches remain: %s", got)
	}

	if got := cleanupTestGit(t, repo, "branch", "--list", "keep"); got == "" {
		t.Fatal("unmarked branch removed")
	}
}

func TestCleanupBatchStopsOnChangedEntry(t *testing.T) {
	repo := cleanupTestRepo(t)
	for _, branch := range []string{"first", "changed", "last"} {
		cleanupTestGit(t, repo, "branch", branch)
	}

	entries, err := cleanupEntriesForRepo(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}

	byBranch := make(map[string]cleanupEntry)
	for _, entry := range entries {
		byBranch[entry.branch] = entry
	}

	cleanupTestGit(t, repo, "branch", "-D", "changed")

	notice := cleanupDeleteBatch(
		t.Context(),
		[]cleanupEntry{byBranch["first"], byBranch["changed"], byBranch["last"]},
		true,
		true,
	)
	if !strings.Contains(notice, "Removed 1 / 3 items; stopped") {
		t.Fatalf("missing partial result: %s", notice)
	}

	if got := cleanupTestGit(t, repo, "branch", "--list", "last"); got == "" {
		t.Fatal("batch continued after failure")
	}
}

func TestCleanupBatchConfirmationScroll(t *testing.T) {
	model := cleanupModel{width: 80, height: 24, confirm: true}
	for index := range 30 {
		model.pendingBatch = append(
			model.pendingBatch,
			cleanupEntry{repo: "/repo", branch: fmt.Sprintf("branch-%02d", index)},
		)
	}

	model.pending = model.pendingBatch[0]
	for range 20 {
		updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})

		model = cleanupTestModel(t, updated)
		if cmd != nil || !model.confirm {
			t.Fatal("scroll changed confirmation")
		}
	}

	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "branch-29") || !strings.Contains(view, "any other key cancels") {
		t.Fatal("last batch target or confirmation hidden")
	}

	if len(strings.Split(strings.TrimSuffix(view, "\n"), "\n")) > model.height {
		t.Fatal("confirmation exceeds terminal height")
	}
}

func TestCleanupBatchAcrossRepositoriesKeepsBranches(t *testing.T) {
	var batch []cleanupEntry

	repos := make([]string, 0, 2)

	for range 2 {
		repo := cleanupTestRepo(t)
		repos = append(repos, repo)
		linked := filepath.Join(t.TempDir(), "linked")
		cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", linked)

		entries, err := cleanupEntriesForRepo(t.Context(), repo)
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range entries {
			if entry.branch == "feature" {
				batch = append(batch, entry)
			}
		}
	}

	notice := cleanupDeleteBatch(t.Context(), batch, false, false)
	if notice != "Removed 2 selected items." {
		t.Fatal(notice)
	}

	for _, repo := range repos {
		if got := cleanupTestGit(t, repo, "branch", "--list", "feature"); got == "" {
			t.Fatal("d removed a worktree's branch")
		}
	}
}
