package main

import (
	"context"
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"
)

//nolint:gocritic // Copy selection before changing the immutable model snapshot.
func (model cleanupModel) toggleSelection(all bool) cleanupModel {
	selected := make(map[cleanupEntry]bool)
	maps.Copy(selected, model.selected)

	targets := []cleanupEntry{model.entries[model.cursor]}
	if all {
		targets = model.entries
	}

	mark := false

	for _, entry := range targets {
		if cleanupSelectable(entry) && !selected[entry] {
			mark = true
		}
	}

	for _, entry := range targets {
		if !cleanupSelectable(entry) {
			continue
		}

		if mark {
			selected[entry] = true
		} else {
			delete(selected, entry)
		}
	}

	model.selected = selected

	return model
}

//nolint:gocritic // Entries are immutable snapshots.
func cleanupSelectable(entry cleanupEntry) bool {
	return entry.head != "" && entry.blocked != cleanupMainWorktree
}

//nolint:gocritic // Selection order follows the displayed immutable snapshot.
func (model cleanupModel) cleanupSelection() []cleanupEntry {
	var entries []cleanupEntry
	for _, entry := range model.entries {
		if model.selected[entry] {
			entries = append(entries, entry)
		}
	}

	if len(entries) == 0 {
		entries = append(entries, model.entries[model.cursor])
	}

	return entries
}

//nolint:gocritic // Preserve compatibility with single-item confirmation snapshots.
func (model cleanupModel) pendingEntries() []cleanupEntry {
	if len(model.pendingBatch) > 0 {
		return model.pendingBatch
	}

	return []cleanupEntry{model.pending}
}

//nolint:gocritic // Commands capture the confirmed immutable snapshot.
func (model cleanupModel) checkRisks() tea.Cmd {
	batch, both := model.pendingEntries(), model.both

	return func() tea.Msg {
		var risks []string

		for _, entry := range batch {
			found, err := cleanupRisks(model.ctx, entry, both)
			if err != nil {
				return cleanupRiskMsg{err: err}
			}

			for _, risk := range found {
				if len(batch) > 1 {
					risk = entry.repo + " / " + entry.branch + ": " + risk
				}

				risks = append(risks, risk)
			}
		}

		return cleanupRiskMsg{risks: risks}
	}
}

func cleanupDeleteBatch(ctx context.Context, batch []cleanupEntry, both, force bool) string {
	for index, entry := range batch {
		err := cleanupDelete(ctx, entry, both, force)
		if err != nil {
			if len(batch) == 1 {
				return err.Error()
			}

			return fmt.Sprintf(
				"Removed %d / %d items; stopped at %s / %s: %v",
				index,
				len(batch),
				entry.repo,
				entry.branch,
				err,
			)
		}
	}

	if len(batch) == 1 {
		return "Removed selected item."
	}

	return fmt.Sprintf("Removed %d selected items.", len(batch))
}
