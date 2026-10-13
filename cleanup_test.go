package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func cleanupTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()

	output, err := cleanupGit(t.Context(), repo, args...)
	if err != nil {
		t.Fatal(err)
	}

	return output
}

func cleanupTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	cleanupTestGit(t, repo, "init", "-b", "main")
	cleanupTestGit(t, repo, "config", "user.name", "Test")
	cleanupTestGit(t, repo, "config", "user.email", "test@example.invalid")
	cleanupTestGit(t, repo, "config", "commit.gpgsign", "false")

	err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cleanupTestGit(t, repo, "add", "file.txt")
	cleanupTestGit(t, repo, "commit", "-m", "Initialize fixture")
	cleanupTestGit(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	cleanupTestGit(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	return repo
}

func cleanupTestEntry(t *testing.T, repo, branch string) cleanupEntry {
	t.Helper()

	entries, err := cleanupEntriesForRepo(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.branch == branch {
			return entry
		}
	}

	t.Fatalf("branch %s missing", branch)

	return cleanupEntry{}
}

//nolint:noinlineerr // Keep assertions next to the operation being verified.
func TestCleanupUnmergedBranchRequiresForce(t *testing.T) {
	repo := cleanupTestRepo(t)
	cleanupTestGit(t, repo, "checkout", "-b", "feature")

	err := os.WriteFile(
		filepath.Join(repo, "file.txt"),
		[]byte("feature\n"),
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanupTestGit(t, repo, "commit", "-am", "Change fixture")
	cleanupTestGit(t, repo, "checkout", "main")
	entry := cleanupTestEntry(t, repo, "feature")

	preview := cleanupPreview(t.Context(), entry)
	if !strings.Contains(preview, "+feature") || !strings.Contains(preview, "origin/main") {
		t.Fatalf("unexpected preview: %s", preview)
	}

	err = cleanupDelete(t.Context(), entry, false, false)
	if !errors.Is(err, errCleanupUnsafe) {
		t.Fatalf("want refusal before confirmation, got %v", err)
	}

	cleanupTestGit(t, repo, "rev-parse", "--verify", "refs/heads/feature")

	err = cleanupDelete(t.Context(), entry, false, true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cleanupGit(
		t.Context(),
		repo,
		"rev-parse",
		"--verify",
		"refs/heads/feature",
	); err == nil {
		t.Fatal("branch survived forced removal")
	}
}

//nolint:noinlineerr // Keep assertions next to the operation being verified.
func TestCleanupDirtyLockedWorktreeRequiresForce(t *testing.T) {
	repo := cleanupTestRepo(t)
	//nolint:gocritic // Deliberately test newline path parsing.
	path := filepath.Join(t.TempDir(), "tree with spaces\nand newline")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", path)

	if err := os.WriteFile(
		filepath.Join(path, "untracked.txt"),
		[]byte("keep until confirmed"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cleanupTestGit(t, repo, "worktree", "lock", path)
	entry := cleanupTestEntry(t, repo, "feature")

	realPath, resolveErr := filepath.EvalSymlinks(path)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}

	if entry.path != realPath {
		t.Fatalf("path changed: %q", entry.path)
	}

	risks, err := cleanupRisks(t.Context(), entry, false)
	if err != nil || len(risks) != 2 {
		t.Fatalf("risks = %v, %v", risks, err)
	}

	if err := cleanupDelete(t.Context(), entry, false, false); !errors.Is(err, errCleanupUnsafe) {
		t.Fatalf("want second confirmation, got %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal("worktree removed before confirmation")
	}

	if err := cleanupDelete(t.Context(), entry, false, true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree survived: %v", err)
	}

	cleanupTestGit(t, repo, "rev-parse", "--verify", "refs/heads/feature")
}

//nolint:noinlineerr // Keep assertions next to the operation being verified.
func TestCleanupMergedWorktreeAndBranch(t *testing.T) {
	repo := cleanupTestRepo(t)
	path := filepath.Join(t.TempDir(), "tree")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", path)

	entry := cleanupTestEntry(t, repo, "feature")

	err := cleanupDelete(t.Context(), entry, true, false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree survived: %v", err)
	}

	if _, err := cleanupGit(
		t.Context(),
		repo,
		"rev-parse",
		"--verify",
		"refs/heads/feature",
	); err == nil {
		t.Fatal("branch survived")
	}
}

func TestCleanupChangedHeadAndMainAreProtected(t *testing.T) {
	repo := cleanupTestRepo(t)
	cleanupTestGit(t, repo, "branch", "feature")
	entry := cleanupTestEntry(t, repo, "feature")
	cleanupTestGit(t, repo, "checkout", "feature")
	cleanupTestGit(t, repo, "commit", "--allow-empty", "-m", "Move fixture HEAD")
	cleanupTestGit(t, repo, "checkout", "main")

	err := cleanupDelete(t.Context(), entry, false, true)
	if !errors.Is(err, errCleanupUnsafe) {
		t.Fatalf("changed head was not protected: %v", err)
	}

	err = cleanupDelete(
		t.Context(),
		cleanupTestEntry(t, repo, "main"),
		true,
		true,
	)
	if !errors.Is(
		err,
		errCleanupUnsafe,
	) {
		t.Fatalf("main worktree was not protected: %v", err)
	}
}

func TestCleanupMissingOriginDoesNotGuess(t *testing.T) {
	repo := cleanupTestRepo(t)
	cleanupTestGit(t, repo, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")

	entries, err := loadCleanupEntries(t.Context(), repo)
	if err != nil || len(entries) != 1 ||
		!strings.Contains(entries[0].blocked, "origin default branch unavailable") {
		t.Fatalf("entries = %v, error = %v", entries, err)
	}
}

func TestCleanupConfirmationFlow(t *testing.T) {
	repo := cleanupTestRepo(t)
	cleanupTestGit(t, repo, "branch", "feature")
	model := cleanupModel{
		ctx:     t.Context(),
		dir:     repo,
		entries: []cleanupEntry{cleanupTestEntry(t, repo, "feature")},
		width:   80,
		height:  24,
	}
	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})

	model = cleanupTestModel(t, updated)
	if cmd != nil || !model.confirm {
		t.Fatal("deletion did not require first confirmation")
	}

	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})

	model = cleanupTestModel(t, updated)
	if cmd != nil || model.confirm {
		t.Fatal("cancel did not cancel")
	}

	updated, cmd = model.Update(cleanupRiskMsg{risks: []string{"local changes will be discarded"}})

	model = cleanupTestModel(t, updated)
	if cmd != nil || !model.forceConfirm || !strings.Contains(model.View().Content, "Type delete") {
		t.Fatal("second confirmation missing")
	}

	updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = cleanupTestModel(t, updated)

	if cmd != nil {
		t.Fatal("empty confirmation started deletion")
	}

	updated, _ = model.Update(tea.PasteMsg{Content: "delete"})
	model = cleanupTestModel(t, updated)
	updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	model = cleanupTestModel(t, updated)
	if cmd == nil || !model.busy {
		t.Fatal("typed confirmation did not start removal")
	}
}

//nolint:noinlineerr // Keep assertions next to the operation being verified.
func TestCleanupPreviewBoundedAndSanitized(t *testing.T) {
	var buffer commandBuffer

	data := []byte(strings.Repeat("x", commandOutputLimit*2))
	if size, err := buffer.Write(
		data,
	); err != nil || size != len(data) ||
		buffer.Len() != commandOutputLimit {
		t.Fatalf("buffer size = %d, len = %d, error = %v", size, buffer.Len(), err)
	}

	if strings.ContainsRune(tuiText("hello\x1b[2J\x07"), '\x1b') {
		t.Fatal("terminal escape survived sanitization")
	}
}

func cleanupTestModel(t *testing.T, model tea.Model) cleanupModel {
	t.Helper()

	result, ok := model.(cleanupModel)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}

	return result
}

func TestCleanupForceRemovesUnmergedDirtyWorktreeAndBranch(t *testing.T) {
	repo := cleanupTestRepo(t)
	path := filepath.Join(t.TempDir(), "tree")
	cleanupTestGit(t, repo, "worktree", "add", "-b", "feature", path)
	cleanupTestGit(t, path, "commit", "--allow-empty", "-m", "Unmerged fixture")

	err := os.WriteFile(filepath.Join(path, "file.txt"), []byte("dirty\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	entry := cleanupTestEntry(t, path, "feature")

	risks, err := cleanupRisks(t.Context(), entry, true)
	if err != nil || len(risks) != 2 {
		t.Fatalf("risks = %v, %v", risks, err)
	}

	err = cleanupDelete(t.Context(), entry, true, true)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("worktree survived: %v", err)
	}

	_, err = cleanupGit(t.Context(), repo, "rev-parse", "--verify", "refs/heads/feature")
	if err == nil {
		t.Fatal("branch survived")
	}

	cleanupTestGit(t, repo, "rev-parse", "--verify", "refs/remotes/origin/main")
}

func TestCleanupMergedIntoOriginButNotLocalHead(t *testing.T) {
	repo := cleanupTestRepo(t)
	cleanupTestGit(t, repo, "checkout", "-b", "feature")
	cleanupTestGit(t, repo, "commit", "--allow-empty", "-m", "Origin merged fixture")
	cleanupTestGit(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	cleanupTestGit(t, repo, "checkout", "main")
	entry := cleanupTestEntry(t, repo, "feature")

	err := cleanupDelete(t.Context(), entry, false, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCleanupDefaultScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := cleanupTestRepo(t)
	nested := filepath.Join(repo, "nested")

	err := os.Mkdir(nested, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := cleanupDefaultDir(t.Context(), nested)
	if err != nil {
		t.Fatal(err)
	}

	expected := cleanupTestGit(t, repo, "rev-parse", "--show-toplevel")
	if dir != expected {
		t.Fatalf("inside repository: got %s, want %s", dir, expected)
	}

	outside := t.TempDir()

	dir, err = cleanupDefaultDir(t.Context(), outside)
	if err != nil {
		t.Fatal(err)
	}

	if dir != filepath.Join(home, "repos") {
		t.Fatalf("outside repository: got %s", dir)
	}
}

func TestCleanupDirectoryScanFiltersRepositories(t *testing.T) {
	root := t.TempDir()
	plain := cleanupTestRepo(t)
	branched := cleanupTestRepo(t)

	linked := cleanupTestRepo(t)
	for name, source := range map[string]string{"plain": plain, "branched": branched, "linked": linked} {
		err := os.Rename(source, filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
	}

	plain = filepath.Join(root, "plain")
	branched = filepath.Join(root, "branched")
	linked = filepath.Join(root, "linked")

	cleanupTestGit(t, plain, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
	cleanupTestGit(t, branched, "branch", "extra")
	cleanupTestGit(
		t,
		linked,
		"worktree",
		"add",
		"-b",
		"feature",
		filepath.Join(t.TempDir(), "tree"),
	)

	entries, err := loadCleanupEntries(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("want two cleanup candidates, got %v", entries)
	}

	branches := make(map[string]bool)

	for _, entry := range entries {
		if entry.blocked != "" {
			t.Fatalf("unexpected blocked entry: %v", entry)
		}

		branches[entry.branch] = true
	}

	if !branches["extra"] || !branches["feature"] {
		t.Fatalf("missing candidates: %v", branches)
	}

	entries, err = loadCleanupEntries(t.Context(), branched)
	if err != nil || len(entries) != 2 {
		t.Fatalf("single repository scope changed: %v, %v", entries, err)
	}
}
