package main

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPullFilterPreservesSelectionDuringUpdates(t *testing.T) {
	model := &pullModel{
		session: &pullSession{}, width: 80, height: 24, cursor: 2,
		entries: []pullEntry{
			{path: "/repos/success", result: pullResult{status: pullSucceeded}},
			{path: "/repos/running", result: pullResult{status: pullRunning}},
			{
				path:   "/repos/failure",
				result: pullResult{status: pullFailed, detail: "selected failure"},
			},
			{path: "/repos/skipped", result: pullResult{status: pullSkipped}},
		},
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if !slices.Equal(model.visibleIndices(), []int{1, 2, 3}) || model.selectedIndex() != 2 {
		t.Fatalf(
			"filter lost selection: indices=%v selected=%d",
			model.visibleIndices(),
			model.selectedIndex(),
		)
	}

	_, _ = model.Update(pullFinishedMsg{index: 1, result: pullResult{status: pullSucceeded}})
	if model.selectedIndex() != 2 || !strings.Contains(model.View().Content, "selected failure") {
		t.Fatal("background completion moved selection or details")
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if model.selectedIndex() != 3 {
		t.Fatal("navigation did not skip hidden entries")
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if model.hideSucceeded || len(model.visibleIndices()) != 4 || model.selectedIndex() != 3 {
		t.Fatal("show all did not restore entries and preserve selection")
	}

	if !strings.Contains(model.summary(), "2 pulled") {
		t.Fatal("summary excluded hidden results")
	}
}

func TestPullFilterHandlesSelectedCompletionAndEmptyList(t *testing.T) {
	model := &pullModel{
		session: &pullSession{}, hideSucceeded: true, width: 80, height: 24, offset: 20,
		entries: []pullEntry{
			{path: "/repos/first", result: pullResult{status: pullRunning}},
			{path: "/repos/second", result: pullResult{status: pullRunning}},
		},
	}

	_, _ = model.Update(pullFinishedMsg{index: 0, result: pullResult{status: pullSucceeded}})
	if model.selectedIndex() != 1 || model.offset != 0 {
		t.Fatal("selection did not advance when hidden")
	}

	_, _ = model.Update(pullFinishedMsg{index: 1, result: pullResult{status: pullSucceeded}})

	_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if model.selectedIndex() != -1 || !strings.Contains(model.View().Content, "All pulled") {
		t.Fatal("empty filtered list was not handled")
	}

	if strings.Contains(strings.Join(model.detailLines(), "\n"), "/repos/") {
		t.Fatal("hidden details still shown")
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if model.selectedIndex() != 0 {
		t.Fatal("cannot restore selection after hiding all entries")
	}
}
