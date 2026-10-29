package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	cleanupLeftWidthRatio = 3
	cleanupMaxLeftWidth   = 44
	cleanupDialogWidth    = 76
)

func renderCleanup(model *cleanupModel) string {
	styles := newTuiStyles(!model.lightBackground)

	if model.width < tuiMinWidth || model.height < tuiMinHeight {
		return styles.muted.Render("Resize terminal to at least 60x16. q: quit") + "\n"
	}

	if model.confirm || model.forceConfirm {
		return renderCleanupConfirmation(model, &styles)
	}

	header := styles.heading.Render(
		"git-tools",
	) + styles.muted.Render(
		" / ",
	) + lipgloss.NewStyle().
		Bold(true).
		Render("cleanup")
	scope := styles.muted.Render(tuiClip(model.dir, model.width))
	height := model.height - tuiChromeHeight
	leftWidth := min(cleanupMaxLeftWidth, model.width/cleanupLeftWidthRatio)
	rightWidth := model.width - leftWidth - 1
	left := renderCleanupList(model, leftWidth, height, &styles)
	right := renderCleanupDiff(model, rightWidth, height, &styles)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	notice := model.notice
	if notice == "" {
		notice = "Origin refs are cached. Fetch separately to update the baseline."
	}

	if model.loading {
		notice = "Scanning repositories..."
	}

	if model.busy {
		notice = "Applying cleanup..."
	}

	help := styles.heading.Render(
		"j/k",
	) + " select  " + styles.heading.Render(
		"d",
	) + " remove  " + styles.heading.Render(
		"b",
	) + " remove + branch  " + styles.heading.Render(
		"r",
	) + " refresh  " + styles.heading.Render(
		"q",
	) + " quit"

	return header + "\n" + scope + "\n\n" + panes + "\n" + styles.muted.Render(
		tuiClip(notice, model.width),
	) + "\n" + ansi.Truncate(
		help,
		model.width,
		"…",
	) + "\n"
}

func renderCleanupList(model *cleanupModel, width, height int, styles *tuiStyles) string {
	inner := width - tuiPanelFrame
	heading := styles.heading.Render(tuiClip("WORKTREES / BRANCHES", inner))
	lines := []string{heading, ""}
	count := max(1, (height-tuiPanelFrame-1)/tuiItemHeight)

	switch {
	case model.loading:
		lines = append(lines, styles.muted.Render("Scanning..."))
	case len(model.entries) == 0:
		lines = append(lines, styles.muted.Render(tuiClip("No cleanup candidates.", inner)))
	default:
		start := min(max(0, model.cursor-count/2), max(0, len(model.entries)-count))
		for index := start; index < min(len(model.entries), start+count); index++ {
			entry := model.entries[index]

			name, kind := entry.branch, "branch"
			if entry.path != "" {
				kind = "worktree"
			}

			if name == "" {
				name = "detached HEAD"
			}

			if entry.head == "" {
				name, kind = filepath.Base(entry.repo), "unavailable"
			}

			if entry.blocked != "" {
				kind = entry.blocked
			}

			prefix := "  "
			if index == model.cursor {
				prefix = "> "
			}

			title := tuiClip(prefix+name, inner)

			metadata := tuiClip("  "+filepath.Base(entry.repo)+" / "+kind, inner)
			if index == model.cursor {
				lines = append(
					lines,
					styles.selected.Width(inner).Render(title),
					styles.selected.Width(inner).Bold(false).Render(metadata),
				)
			} else {
				lines = append(lines, title, styles.muted.Render(metadata))
			}

			lines = append(lines, "")
		}
	}

	contentHeight := height - tuiPanelBorder
	for len(lines) < contentHeight-1 {
		lines = append(lines, "")
	}

	lines = append(
		lines,
		styles.muted.Render(
			tuiClip(
				fmt.Sprintf(
					"%d / %d items",
					min(model.cursor+1, len(model.entries)),
					len(model.entries),
				),
				inner,
			),
		),
	)

	return tuiPanel(strings.Join(lines, "\n"), width, height, true, styles)
}

func renderCleanupDiff(model *cleanupModel, width, height int, styles *tuiStyles) string {
	inner := width - tuiPanelFrame
	selected := "Select an item to preview changes."

	if len(model.entries) > 0 {
		entry := model.entries[model.cursor]

		selected = entry.path
		if selected == "" {
			selected = entry.repo + " / " + entry.branch
		}
	}

	lines := []string{
		styles.heading.Render("DIFF PREVIEW"),
		styles.muted.Render(tuiClip(selected, inner)),
		"",
	}
	preview := strings.Split(strings.ReplaceAll(tuiText(model.preview), "\t", "    "), "\n")
	available := max(1, height-tuiPanelBorder-len(lines)-1)
	start := min(model.offset, max(0, len(preview)-1))

	end := min(len(preview), start+available)
	for _, line := range preview[start:end] {
		lines = append(lines, cleanupDiffLine(line, inner, styles))
	}

	for len(lines) < height-tuiPanelBorder-1 {
		lines = append(lines, "")
	}

	progress := fmt.Sprintf("Lines %d-%d / %d  |  PgUp/PgDown", start+1, end, len(preview))
	lines = append(lines, styles.muted.Render(tuiClip(progress, inner)))

	return tuiPanel(strings.Join(lines, "\n"), width, height, false, styles)
}

func cleanupDiffLine(line string, width int, styles *tuiStyles) string {
	clipped := tuiClip(line, width)
	switch {
	case strings.HasPrefix(line, "diff --git"),
		strings.HasPrefix(line, "--- "),
		strings.HasPrefix(line, "+++ "):
		return styles.heading.Render(clipped)
	case strings.HasPrefix(line, "@@"):
		return styles.hunk.Render(clipped)
	case strings.HasPrefix(line, "+"):
		return styles.added.Render(clipped)
	case strings.HasPrefix(line, "-"):
		return styles.removed.Render(clipped)
	case strings.HasPrefix(line, "Against "),
		strings.HasPrefix(line, "Behind / ahead:"),
		strings.HasPrefix(line, "index "):
		return styles.muted.Render(clipped)
	case strings.HasPrefix(line, "Local changes"):
		return styles.warning.Render(clipped)
	default:
		return clipped
	}
}

func renderCleanupConfirmation(model *cleanupModel, styles *tuiStyles) string {
	width := min(cleanupDialogWidth, model.width-tuiPanelFrame)
	inner := width - tuiPanelFrame
	title := styles.heading.Render("CONFIRM REMOVAL")
	prompt := styles.heading.Render(
		"y",
	) + " remove    " + styles.muted.Render(
		"any other key cancels",
	)

	lines := []string{title, "", ansi.Hardwrap(tuiText(model.action()), inner, false), ""}
	if model.forceConfirm {
		lines[0] = styles.warning.Render("CONFIRM PERMANENT REMOVAL")
		for _, risk := range model.risks {
			lines = append(
				lines,
				styles.warning.Render(ansi.Hardwrap(tuiText(risk), inner, false)),
				"",
			)
		}

		prompt = styles.heading.Render("y") + " confirm permanent removal    " +
			styles.muted.Render("any other key cancels")
	}

	lines = append(lines, prompt)
	dialog := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.accent).
		Padding(1).Width(width).Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		model.width,
		model.height-1,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	) + "\n"
}
