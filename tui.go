package main

import (
	"image/color"
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

const (
	tuiInitialWidth  = 80
	tuiInitialHeight = 24
	tuiScrollStep    = 5
	tuiMinWidth      = 60
	tuiMinHeight     = 16
	tuiChromeHeight  = 7
	tuiPanelFrame    = 4
	tuiPanelBorder   = 2
	tuiItemHeight    = 3
)

type tuiStyles struct {
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

// tuiLightBackground keeps the COLORFGBG fallback used by Lip Gloss v1.
// A terminal's background-color response takes precedence once Bubble Tea receives it.
//
//nolint:mnd // The ANSI palette bounds and 16-bit HSL threshold preserve the v1 fallback.
func tuiLightBackground(colorFGBG string) bool {
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

func newTuiStyles(hasDarkBackground bool) tuiStyles {
	lightDark := lipgloss.LightDark(hasDarkBackground)
	accent := lightDark(lipgloss.Color("#2563eb"), lipgloss.Color("#82aaff"))

	return tuiStyles{
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

func tuiClip(text string, width int) string {
	text = strings.ReplaceAll(tuiText(text), "\t", "    ")
	text = strings.ReplaceAll(text, "\n", " ")

	return ansi.Truncate(text, max(0, width), "…")
}

func tuiPanel(content string, width, height int, active bool, styles *tuiStyles) string {
	border := styles.border
	if active {
		border = styles.accent
	}

	// Lip Gloss v2 dimensions include borders, unlike v1.
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).
		Width(width).Height(height).Render(content)
}

// tuiColorProfile preserves the color-environment precedence used by Lip Gloss v1.
func tuiColorProfile(
	detected colorprofile.Profile,
	noColor, cliColor, cliColorForce string,
) (colorprofile.Profile, bool) {
	forced := cliColorForce != "" && cliColorForce != "0"
	if noColor != "" || (cliColor == "0" && !forced) {
		return colorprofile.ASCII, true
	}

	if forced {
		return max(detected, colorprofile.ANSI), false
	}

	return detected, false
}

func tuiText(text string) string {
	return strings.Map(func(char rune) rune {
		if char != '\n' && char != '\t' && unicode.IsControl(char) {
			return -1
		}

		return char
	}, text)
}
