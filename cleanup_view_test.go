package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCleanupSplitLayoutFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{60, 16}, {80, 24}, {120, 32}} {
		model := cleanupModel{
			dir: "/repos/example",
			entries: []cleanupEntry{
				{repo: "/repos/example", branch: strings.Repeat("branch", 20)},
				{repo: "/repos/other", branch: "feature"},
			},
			width:   size[0],
			height:  size[1],
			preview: strings.Repeat("diff --git a/file b/file\n@@ -1 +1 @@\n-old\n+new\n", 20),
		}
		view := model.View().Content

		lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
		if len(lines) > model.height {
			t.Fatalf("%dx%d: layout has %d lines", model.width, model.height, len(lines))
		}

		for _, line := range lines {
			if ansi.StringWidth(line) > model.width {
				t.Fatalf("%dx%d: line overflows: %q", model.width, model.height, line)
			}
		}

		sidebar, preview := -1, -1

		for _, line := range lines {
			if index := strings.Index(line, "WORKTREES"); index >= 0 {
				sidebar = index
			}

			if index := strings.Index(line, "DIFF PREVIEW"); index >= 0 {
				preview = index
			}
		}

		if sidebar < 0 || preview <= sidebar {
			t.Fatalf("%dx%d: diff is not on the right", model.width, model.height)
		}
	}
}
