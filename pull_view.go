package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	pullMaxLeftWidth   = 48
	pullLeftWidthRatio = 2
)

func renderPull(model *pullModel) string {
	styles := newTuiStyles(!model.lightBackground)
	if model.width < tuiMinWidth || model.height < tuiMinHeight {
		return tuiClip("Resize terminal to at least 60x16. q: quit", model.width)
	}

	height := model.height - tuiChromeHeight
	leftWidth := min(pullMaxLeftWidth, model.width/pullLeftWidthRatio)
	rightWidth := model.width - leftWidth - 1
	header := styles.heading.Render("git-tools") + styles.muted.Render(" / ") + "pull"
	panes := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderPullList(
			model,
			leftWidth,
			height,
			&styles,
		),
		" ",
		renderPullDetails(model, rightWidth, height, &styles),
	)

	notice := model.summary()
	switch {
	case model.stopping:
		notice = "Canceling Git operations..."
	case model.err != nil:
		notice = model.err.Error()
	case model.scanning:
		notice = "Scanning repositories..."
	case model.notice != "":
		notice = model.notice
	case model.done && len(model.entries) == 0:
		notice = "No repositories found."
	case model.done:
		notice = "Finished. " + notice
	}

	filterHelp := "h hide pulled"
	if model.hideSucceeded {
		filterHelp = "h show pulled"
	}

	return header + "\n" + styles.muted.Render(
		tuiClip(model.dir, model.width),
	) + "\n\n" + panes + "\n" +
		styles.muted.Render(
			tuiClip(notice, model.width),
		) + "\n" +
		tuiClip(
			"j/k move  PgUp/Dn  "+filterHelp+"  s shell  r retry  q quit",
			model.width,
		) + "\n"
}

func (model *pullModel) summary() string {
	counts := make(map[pullStatus]int)
	for _, entry := range model.entries {
		counts[entry.result.status]++
	}

	return fmt.Sprintf(
		"%d pulling  %d pulled  %d skipped  %d errored  %d queued",
		counts[pullRunning],
		counts[pullSucceeded],
		counts[pullSkipped],
		counts[pullFailed],
		counts[pullQueued],
	)
}

func pullStatusStyle(status pullStatus, styles *tuiStyles) lipgloss.Style {
	switch status {
	case pullSucceeded:
		return styles.added
	case pullFailed:
		return styles.removed
	case pullSkipped, pullCanceled:
		return styles.warning
	case pullRunning:
		return styles.heading
	case pullQueued:
		return styles.muted
	default:
		return styles.muted
	}
}

func renderPullList(model *pullModel, width, height int, styles *tuiStyles) string {
	inner := width - tuiPanelFrame

	heading := "REPOSITORIES"
	if model.hideSucceeded {
		heading += " (filtered)"
	}

	lines := []string{styles.heading.Render(tuiClip(heading, inner)), ""}
	indices := model.visibleIndices()
	count := max(1, (height-tuiPanelFrame-1)/tuiItemHeight)

	start := min(max(0, model.cursor-count/2), max(0, len(indices)-count))
	for index := start; index < min(len(indices), start+count); index++ {
		entry := model.entries[indices[index]]

		name, err := filepath.Rel(model.dir, entry.path)
		if err != nil || name == "." {
			name = filepath.Base(entry.path)
		}

		prefix := "  "
		if index == model.cursor {
			prefix = "> "
		}

		title := tuiClip(prefix+name, inner)
		if index == model.cursor {
			title = styles.selected.Width(inner).Render(title)
		}

		status := string(entry.result.status)
		if entry.result.elapsed > 0 {
			status += " · " + entry.result.elapsed.Round(time.Millisecond).String()
		}

		lines = append(
			lines,
			title,
			pullStatusStyle(entry.result.status, styles).Render(tuiClip("  "+status, inner)),
			"",
		)
	}

	if len(indices) == 0 {
		text := "No repositories."
		if model.hideSucceeded && len(model.entries) > 0 {
			text = "All pulled; h shows all."
		}

		if model.scanning {
			text = "Scanning..."
		}

		lines = append(lines, styles.muted.Render(tuiClip(text, inner)))
	}

	for len(lines) < height-tuiPanelBorder-1 {
		lines = append(lines, "")
	}

	lines = append(lines, styles.muted.Render(tuiClip(fmt.Sprintf("%d / %d repositories",
		min(model.cursor+1, len(indices)), len(indices)), inner)))

	return tuiPanel(strings.Join(lines, "\n"), width, height, true, styles)
}

func (model *pullModel) detailLines() []string {
	selected := model.selectedIndex()
	if selected < 0 {
		return []string{"Select a repository to see details."}
	}

	entry := model.entries[selected]

	detail := entry.result.detail
	if detail == "" {
		detail = "Waiting for a worker."
		if entry.result.status == pullRunning {
			detail = "Running Git checks and pull. Output will appear when complete."
		}
	}

	text := fmt.Sprintf("Path: %s\nStatus: %s\nBranch: %s\nRemote: %s\n\n%s",
		entry.path, entry.result.status, entry.result.branch, entry.result.remote, detail)

	inner := max(
		1,
		model.width-min(pullMaxLeftWidth, model.width/pullLeftWidthRatio)-1-tuiPanelFrame,
	)
	text = strings.ReplaceAll(tuiText(text), "\t", "    ")

	return strings.Split(ansi.Hardwrap(text, inner, false), "\n")
}

func renderPullDetails(model *pullModel, width, height int, styles *tuiStyles) string {
	inner := width - tuiPanelFrame
	lines := []string{styles.heading.Render("DETAILS / GIT OUTPUT"), ""}
	details := model.detailLines()
	start := min(model.offset, max(0, len(details)-1))

	end := min(len(details), start+height-tuiPanelFrame-1)
	for _, line := range details[start:end] {
		lines = append(lines, tuiClip(line, inner))
	}

	for len(lines) < height-tuiPanelBorder-1 {
		lines = append(lines, "")
	}

	lines = append(
		lines,
		styles.muted.Render(
			tuiClip(fmt.Sprintf("Lines %d-%d / %d", start+1, end, len(details)), inner),
		),
	)

	return tuiPanel(strings.Join(lines, "\n"), width, height, false, styles)
}
