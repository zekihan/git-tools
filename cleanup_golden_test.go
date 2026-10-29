package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

//nolint:gosmopolitan // Wide characters and emoji deliberately exercise terminal cell widths.
func cleanupGoldenModels() map[string]cleanupModel {
	base := cleanupModel{
		dir: "/repos/example",
		entries: []cleanupEntry{
			{
				repo:   "/repos/example",
				branch: "feature/界面-👩‍💻-long-name",
				head:   "abc123",
				path:   "/worktrees/example/feature",
			},
			{repo: "/repos/example", branch: "merged-feature", head: "def456"},
			{repo: "/repos/other", branch: "main", head: "ghi789", blocked: "current branch"},
		},
		width:  80,
		height: 24,
		preview: "Against origin/main\nBehind / ahead: 0 / 1\n\n" +
			"diff --git a/file.txt b/file.txt\nindex 1234567..7654321 100644\n" +
			"--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old value\n+new value\n" +
			"Local changes will be discarded\n\twide 界面 👩‍💻 line",
	}

	models := map[string]cleanupModel{"ready-80x24": base}
	for name, size := range map[string][2]int{
		"ready-60x16":  {60, 16},
		"ready-120x32": {120, 32},
		"narrow":       {59, 24},
		"short":        {80, 15},
	} {
		model := base
		model.width, model.height = size[0], size[1]
		models[name] = model
	}

	loading := base
	loading.loading = true
	models["loading"] = loading
	empty := base
	empty.entries, empty.preview = nil, ""
	models["empty"] = empty
	confirm := base
	confirm.pending = base.entries[0]
	confirm.confirm, confirm.both = true, true
	models["confirm"] = confirm
	force := base
	force.pending = base.entries[0]
	force.forceConfirm = true
	force.risks = []string{"Unmerged commits will be lost.", "Local changes will be discarded."}
	models["force-confirm"] = force
	busy := base
	busy.busy, busy.notice = true, "Previous status"
	models["busy"] = busy

	return models
}

// These fixtures were captured from main (67d26f1) before the Charm v2 migration.
func TestCleanupLayoutMatchesV1(t *testing.T) {
	for name, model := range cleanupGoldenModels() {
		t.Run(name, func(t *testing.T) {
			expected, err := os.ReadFile(filepath.Join("testdata", "cleanup", name+".golden"))
			if err != nil {
				t.Fatal(err)
			}

			view := model.View()
			if !view.AltScreen {
				t.Fatal("cleanup must stay in the alternate screen in every state")
			}

			if actual := ansi.Strip(view.Content); actual != string(expected) {
				t.Fatalf("layout changed from v1\nwant:\n%s\ngot:\n%s", expected, actual)
			}
		})
	}
}
