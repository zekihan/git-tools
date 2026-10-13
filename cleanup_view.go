package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
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

type cleanupStyles struct {
	accent   color.Color
	border   color.Color
	muted    lipgloss.Style
	heading  lipgloss.Style
	selected lipgloss.Style
	added    lipgloss.Style
	removed  lipgloss.Style
	hunk     lipgloss.Style
	warning  lipgloss.Style
}

// cleanupLightBackground keeps the COLORFGBG fallback used by Lip Gloss v1.
// A terminal's background-color response takes precedence once Bubble Tea receives it.
//
//nolint:mnd // The ANSI palette bounds and 16-bit HSL threshold preserve the v1 fallback.
func cleanupLightBackground(colorFGBG string) bool {
	parts := strings.Split(colorFGBG, ";")
	if len(parts) < 2 {
		return false
	}

	index, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || index < 0 || index > 255 {
		return false
	}

	red, green, blue, _ := lipgloss.Color(strconv.Itoa(index)).RGBA()

	// Match v1's HSL lightness threshold without querying the terminal before startup.
	return max(red, green, blue)+min(red, green, blue) >= 0xffff
}

func newCleanupStyles(hasDarkBackground bool) cleanupStyles {
	lightDark := lipgloss.LightDark(hasDarkBackground)
	accent := lightDark(lipgloss.Color("#2563eb"), lipgloss.Color("#82aaff"))

	return cleanupStyles{
		accent: accent,
		border: lightDark(lipgloss.Color("#cbd5e1"), lipgloss.Color("#3b4961")),
		muted: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#64748b"), lipgloss.Color("#8291a8"))),
		heading: lipgloss.NewStyle().Bold(true).Foreground(accent),
		selected: lipgloss.NewStyle().Bold(true).
			Foreground(lightDark(lipgloss.Color("#1d4ed8"), lipgloss.Color("#dbeafe"))).
			Background(lightDark(lipgloss.Color("#e8f0fe"), lipgloss.Color("#24344f"))),
		added: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#15803d"), lipgloss.Color("#91d7a3"))),
		removed: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#be123c"), lipgloss.Color("#f08c9b"))),
		hunk: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#0369a1"), lipgloss.Color("#89dceb"))),
		warning: lipgloss.NewStyle().Bold(true).
			Foreground(lightDark(lipgloss.Color("#b45309"), lipgloss.Color("#f5c27c"))),
	}
}

func renderCleanup(model *cleanupModel) string {
	styles := newCleanupStyles(!model.lightBackground)

	if model.width < cleanupMinWidth || model.height < cleanupMinHeight {
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
	scope := styles.muted.Render(cleanupClip(model.dir, model.width))
	height := model.height - cleanupChromeHeight
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

func cleanupPanel(content string, width, height int, active bool, styles *cleanupStyles) string {
	border := styles.border
	if active {
		border = styles.accent
	}

	// Lip Gloss v2 dimensions include borders, unlike v1.
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).
		Width(width).Height(height).Render(content)
}

func renderCleanupList(model *cleanupModel, width, height int, styles *cleanupStyles) string {
	inner := width - cleanupPanelFrame
	heading := styles.heading.Render(cleanupClip("WORKTREES / BRANCHES", inner))
	lines := []string{heading, ""}
	count := max(1, (height-cleanupPanelFrame-1)/cleanupItemHeight)

	switch {
	case model.loading:
		lines = append(lines, styles.muted.Render("Scanning..."))
	case len(model.entries) == 0:
		lines = append(lines, styles.muted.Render(cleanupClip("No cleanup candidates.", inner)))
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
					styles.selected.Width(inner).Render(title),
					styles.selected.Width(inner).Bold(false).Render(metadata),
				)
			} else {
				lines = append(lines, title, styles.muted.Render(metadata))
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
		styles.muted.Render(
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

	return cleanupPanel(strings.Join(lines, "\n"), width, height, true, styles)
}

func renderCleanupDiff(model *cleanupModel, width, height int, styles *cleanupStyles) string {
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
		styles.heading.Render("DIFF PREVIEW"),
		styles.muted.Render(cleanupClip(selected, inner)),
		"",
	}
	preview := strings.Split(strings.ReplaceAll(cleanupText(model.preview), "\t", "    "), "\n")
	available := max(1, height-cleanupPanelBorder-len(lines)-1)
	start := min(model.offset, max(0, len(preview)-1))

	end := min(len(preview), start+available)
	for _, line := range preview[start:end] {
		lines = append(lines, cleanupDiffLine(line, inner, styles))
	}

	for len(lines) < height-cleanupPanelBorder-1 {
		lines = append(lines, "")
	}

	progress := fmt.Sprintf("Lines %d-%d / %d  |  PgUp/PgDown", start+1, end, len(preview))
	lines = append(lines, styles.muted.Render(cleanupClip(progress, inner)))

	return cleanupPanel(strings.Join(lines, "\n"), width, height, false, styles)
}

func cleanupDiffLine(line string, width int, styles *cleanupStyles) string {
	clipped := cleanupClip(line, width)
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

func renderCleanupConfirmation(model *cleanupModel, styles *cleanupStyles) string {
	width := min(cleanupDialogWidth, model.width-cleanupPanelFrame)
	inner := width - cleanupPanelFrame
	title := styles.heading.Render("CONFIRM REMOVAL")
	prompt := styles.heading.Render(
		"y",
	) + " remove    " + styles.muted.Render(
		"any other key cancels",
	)

	lines := []string{title, "", ansi.Hardwrap(cleanupText(model.action()), inner, false), ""}
	if model.forceConfirm {
		lines[0] = styles.warning.Render("CONFIRM PERMANENT REMOVAL")
		for _, risk := range model.risks {
			lines = append(
				lines,
				styles.warning.Render(ansi.Hardwrap(cleanupText(risk), inner, false)),
				"",
			)
		}

		prompt = "Type delete, then Enter\n" + styles.selected.Width(inner).
			Render(" > "+model.typed) +
			"\n" + styles.muted.Render(
			"Esc cancels",
		)
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
