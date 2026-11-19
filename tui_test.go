package main

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDashboardPagingShowsEveryOutputLine(t *testing.T) {
	const repo = "/repos/example"

	output := make([]string, 12)
	for index := range output {
		output[index] = fmt.Sprintf("output line %02d", index+1)
	}

	detail := strings.Join(output, "\n")
	models := map[string]tea.Model{
		"pull": &pullModel{
			dir: "/repos",
			entries: []pullEntry{{
				path:   repo,
				result: pullResult{status: pullFailed, detail: detail},
			}},
		},
		"cleanup": cleanupModel{
			dir:     repo,
			entries: []cleanupEntry{{repo: repo, branch: "topic", head: "abc123"}},
			preview: detail,
		},
	}

	for name, model := range models {
		t.Run(name, func(t *testing.T) {
			model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 16})

			var visited strings.Builder
			for range output {
				visited.WriteString(model.View().Content)
				model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			}

			for _, line := range output {
				if !strings.Contains(visited.String(), line) {
					t.Errorf("paging skipped %q", line)
				}
			}

			lastPage := model.View().Content
			for _, line := range output[len(output)-3:] {
				if !strings.Contains(lastPage, line) {
					t.Errorf("last page is missing %q", line)
				}
			}

			model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})

			if !strings.Contains(model.View().Content, output[0]) {
				t.Error("page up after enlarging the window did not scroll from the visible page")
			}
		})
	}
}
