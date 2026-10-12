package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	cleanupMinWidth       = 60
	cleanupMinHeight      = 16
	cleanupChromeHeight   = 7
	cleanupPanelFrame     = 4
	cleanupPanelBorder    = 2
	cleanupItemHeight     = 3
	cleanupLeftWidthRatio = 3
	cleanupMaxLeftWidth   = 44
	cleanupDialogWidth    = 76
)

var (
	cleanupAccent = lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#82aaff"}
	cleanupMuted  = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#8291a8"})
	cleanupHeading  = lipgloss.NewStyle().Bold(true).Foreground(cleanupAccent)
	cleanupSelected = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#dbeafe"}).
			Background(lipgloss.AdaptiveColor{Light: "#e8f0fe", Dark: "#24344f"})
	cleanupAdded = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#91d7a3"})
	cleanupRemoved = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#be123c", Dark: "#f08c9b"})
	cleanupHunk = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#89dceb"})
	cleanupWarning = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#f5c27c"})
)

func renderCleanup(model *cleanupModel) string {
	if model.width < cleanupMinWidth || model.height < cleanupMinHeight {
		return cleanupMuted.Render("Resize terminal to at least 60x16. q: quit") + "\n"
	}

	if model.confirm || model.forceConfirm {
		return renderCleanupConfirmation(model)
	}

	header := cleanupHeading.Render(
		"git-tools",
	) + cleanupMuted.Render(
		" / ",
	) + lipgloss.NewStyle().
		Bold(true).
		Render("cleanup")
	scope := cleanupMuted.Render(cleanupClip(model.dir, model.width))
	height := model.height - cleanupChromeHeight
	leftWidth := min(cleanupMaxLeftWidth, model.width/cleanupLeftWidthRatio)
	rightWidth := model.width - leftWidth - 1
	left := renderCleanupList(model, leftWidth, height)
	right := renderCleanupDiff(model, rightWidth, height)
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

	help := cleanupHeading.Render(
		"j/k",
	) + " select  " + cleanupHeading.Render(
		"d",
	) + " remove  " + cleanupHeading.Render(
		"b",
	) + " remove + branch  " + cleanupHeading.Render(
		"r",
	) + " refresh  " + cleanupHeading.Render(
		"q",
	) + " quit"

	return header + "\n" + scope + "\n\n" + panes + "\n" + cleanupMuted.Render(
		cleanupClip(notice, model.width),
	) + "\n" + ansi.Truncate(
		help,
		model.width,
		"…",
	) + "\n"
}

func cleanupClip(text string, width int) string {
	text = strings.ReplaceAll(cleanupText(text), "\t", "    ")
	text = strings.ReplaceAll(text, "\n", " ")

	return ansi.Truncate(text, max(0, width), "…")
}

func cleanupPanel(content string, width, height int, active bool) string {
	border := lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#3b4961"}
	if active {
		border = cleanupAccent
	}

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).
		Width(width - cleanupPanelBorder).Height(height - cleanupPanelBorder).Render(content)
}

func renderCleanupList(model *cleanupModel, width, height int) string {
	inner := width - cleanupPanelFrame
	heading := cleanupHeading.Render(cleanupClip("WORKTREES / BRANCHES", inner))
	lines := []string{heading, ""}
	count := max(1, (height-cleanupPanelFrame-1)/cleanupItemHeight)

	switch {
	case model.loading:
		lines = append(lines, cleanupMuted.Render("Scanning..."))
	case len(model.entries) == 0:
		lines = append(lines, cleanupMuted.Render(cleanupClip("No cleanup candidates.", inner)))
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

			title := cleanupClip(prefix+name, inner)

			metadata := cleanupClip("  "+filepath.Base(entry.repo)+" / "+kind, inner)
			if index == model.cursor {
				lines = append(
					lines,
					cleanupSelected.Width(inner).Render(title),
					cleanupSelected.Width(inner).Bold(false).Render(metadata),
				)
			} else {
				lines = append(lines, title, cleanupMuted.Render(metadata))
			}

			lines = append(lines, "")
		}
	}

	contentHeight := height - cleanupPanelBorder
	for len(lines) < contentHeight-1 {
		lines = append(lines, "")
	}

	lines = append(
		lines,
		cleanupMuted.Render(
			cleanupClip(
				fmt.Sprintf(
					"%d / %d items",
					min(model.cursor+1, len(model.entries)),
					len(model.entries),
				),
				inner,
			),
		),
	)

	return cleanupPanel(strings.Join(lines, "\n"), width, height, true)
}

func renderCleanupDiff(model *cleanupModel, width, height int) string {
	inner := width - cleanupPanelFrame
	selected := "Select an item to preview changes."

	if len(model.entries) > 0 {
		entry := model.entries[model.cursor]

		selected = entry.path
		if selected == "" {
			selected = entry.repo + " / " + entry.branch
		}
	}

	lines := []string{
		cleanupHeading.Render("DIFF PREVIEW"),
		cleanupMuted.Render(cleanupClip(selected, inner)),
		"",
	}
	preview := strings.Split(strings.ReplaceAll(cleanupText(model.preview), "\t", "    "), "\n")
	available := max(1, height-cleanupPanelBorder-len(lines)-1)
	start := min(model.offset, max(0, len(preview)-1))

	end := min(len(preview), start+available)
	for _, line := range preview[start:end] {
		lines = append(lines, cleanupDiffLine(line, inner))
	}

	for len(lines) < height-cleanupPanelBorder-1 {
		lines = append(lines, "")
	}

	progress := fmt.Sprintf("Lines %d-%d / %d  |  PgUp/PgDown", start+1, end, len(preview))
	lines = append(lines, cleanupMuted.Render(cleanupClip(progress, inner)))

	return cleanupPanel(strings.Join(lines, "\n"), width, height, false)
}

func cleanupDiffLine(line string, width int) string {
	clipped := cleanupClip(line, width)
	switch {
	case strings.HasPrefix(line, "diff --git"),
		strings.HasPrefix(line, "--- "),
		strings.HasPrefix(line, "+++ "):
		return cleanupHeading.Render(clipped)
	case strings.HasPrefix(line, "@@"):
		return cleanupHunk.Render(clipped)
	case strings.HasPrefix(line, "+"):
		return cleanupAdded.Render(clipped)
	case strings.HasPrefix(line, "-"):
		return cleanupRemoved.Render(clipped)
	case strings.HasPrefix(line, "Against "),
		strings.HasPrefix(line, "Behind / ahead:"),
		strings.HasPrefix(line, "index "):
		return cleanupMuted.Render(clipped)
	case strings.HasPrefix(line, "Local changes"):
		return cleanupWarning.Render(clipped)
	default:
		return clipped
	}
}

func renderCleanupConfirmation(model *cleanupModel) string {
	width := min(cleanupDialogWidth, model.width-cleanupPanelFrame)
	inner := width - cleanupPanelFrame
	title := cleanupHeading.Render("CONFIRM REMOVAL")
	prompt := cleanupHeading.Render(
		"y",
	) + " remove    " + cleanupMuted.Render(
		"any other key cancels",
	)

	lines := []string{title, "", ansi.Hardwrap(cleanupText(model.action()), inner, false), ""}
	if model.forceConfirm {
		lines[0] = cleanupWarning.Render("CONFIRM PERMANENT REMOVAL")
		for _, risk := range model.risks {
			lines = append(
				lines,
				cleanupWarning.Render(ansi.Hardwrap(cleanupText(risk), inner, false)),
				"",
			)
		}

		prompt = "Type delete, then Enter\n" + cleanupSelected.Width(inner).
			Render(" > "+model.typed) +
			"\n" + cleanupMuted.Render(
			"Esc cancels",
		)
	}

	lines = append(lines, prompt)
	dialog := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cleanupAccent).
		Padding(1).Width(width - cleanupPanelBorder).Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		model.width,
		model.height-1,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	) + "\n"
}
