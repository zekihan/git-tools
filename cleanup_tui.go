package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	cleanupInitialWidth    = 80
	cleanupInitialHeight   = 24
	cleanupScrollStep      = 5
	cleanupListRows        = 6
	cleanupPreviewPadding  = 2
	cleanupListHeightRatio = 4
)

type cleanupLoaded struct {
	err     error
	notice  string
	dir     string
	entries []cleanupEntry
}
type cleanupPreviewMsg struct {
	entry cleanupEntry
	text  string
}
type cleanupRiskMsg struct {
	err   error
	risks []string
}

//nolint:containedctx // Bubble Tea commands need the command lifetime context.
type cleanupModel struct {
	ctx          context.Context
	preview      string
	dir          string
	typed        string
	notice       string
	entries      []cleanupEntry
	risks        []string
	offset       int
	height       int
	width        int
	cursor       int
	loading      bool
	busy         bool
	confirm      bool
	forceConfirm bool
	both         bool
}

func handleCleanupCmd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("cleanup", flag.ContinueOnError)

	flags.Usage = printCleanupHelp

	err := flags.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return fmt.Errorf("parse cleanup options: %w", err)
	}

	if flags.NArg() > 1 {
		return fmt.Errorf("%w: expected at most one directory", errTooManyWorktreeArgs)
	}

	dir := "."
	if flags.NArg() == 1 {
		dir = os.ExpandEnv(flags.Arg(0))
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve cleanup directory: %w", err)
	}

	if flags.NArg() == 0 {
		dir, err = cleanupDefaultDir(ctx, dir)
		if err != nil {
			return err
		}
	}

	model := cleanupModel{
		ctx:     ctx,
		dir:     dir,
		width:   cleanupInitialWidth,
		height:  cleanupInitialHeight,
		loading: true,
	}

	_, err = tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if err != nil {
		slog.ErrorContext(ctx, "Cleanup failed", slog.Any("error", err))

		return fmt.Errorf("run cleanup: %w", err)
	}

	return nil
}

//nolint:gocritic // Value models keep asynchronous command snapshots independent.
func (model cleanupModel) Init() tea.Cmd { return model.load("") }

//nolint:ireturn,gocyclo,gocritic // Bubble Tea events form one state machine with immutable snapshots.
func (model cleanupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.width, model.height = msg.Width, msg.Height
	case cleanupLoaded:
		model.loading, model.busy = false, false

		model.notice = msg.notice
		if msg.dir != "" {
			model.dir = msg.dir
		}

		if msg.err != nil {
			model.notice = msg.err.Error()

			return model, nil
		}

		model.entries = msg.entries
		model.cursor = max(0, min(model.cursor, len(model.entries)-1))
		model.offset, model.preview = 0, "Loading preview..."

		return model, model.previewCmd()
	case cleanupPreviewMsg:
		if len(model.entries) > 0 && model.entries[model.cursor] == msg.entry {
			model.preview = msg.text
		}
	case cleanupRiskMsg:
		model.busy = false
		if msg.err != nil {
			model.notice = msg.err.Error()

			return model, nil
		}

		if len(msg.risks) > 0 {
			model.risks, model.forceConfirm, model.typed = msg.risks, true, ""

			return model, nil
		}

		return model.startDelete(false)
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return model, tea.Quit
		}

		if model.busy {
			return model, nil
		}

		if model.forceConfirm {
			return model.forceKey(msg)
		}

		if model.confirm {
			model.confirm = false
			if key == "y" {
				model.busy = true
				entry, both := model.entries[model.cursor], model.both

				return model, func() tea.Msg {
					risks, err := cleanupRisks(model.ctx, entry, both)

					return cleanupRiskMsg{risks: risks, err: err}
				}
			}

			return model, nil
		}

		switch key {
		case "q", "esc":
			return model, tea.Quit
		case "r":
			model.loading = true

			return model, model.load("")
		case "up", "k":
			model.cursor = max(0, model.cursor-1)
			model.offset = 0
			model.preview = "Loading preview..."

			return model, model.previewCmd()
		case "down", "j":
			model.cursor = max(0, min(len(model.entries)-1, model.cursor+1))
			model.offset = 0
			model.preview = "Loading preview..."

			return model, model.previewCmd()
		case "pgdown":
			model.offset = min(
				max(0, len(strings.Split(model.preview, "\n"))-1),
				model.offset+cleanupScrollStep,
			)
		case "pgup":
			model.offset = max(0, model.offset-cleanupScrollStep)
		case "d", "b":
			if len(model.entries) > 0 && !model.loading {
				model.both, model.confirm = key == "b", true
			}
		}
	}

	return model, nil
}

//nolint:gocyclo,gocritic // Render the list, preview, and confirmation from an immutable model.
func (model cleanupModel) View() string {
	if model.width < 30 || model.height < 12 {
		return "Resize terminal to at least 30x12. q: quit\n"
	}

	if model.confirm || model.forceConfirm {
		prompt := "Confirm removal\n\n" + model.action() + "\n\nRemove this item? [y/N]"
		if model.forceConfirm {
			prompt = "Confirm permanent removal\n\n" + model.action() + "\n\n" + strings.Join(
				model.risks,
				"\n",
			) + "\n\nType delete, then Enter (Esc cancels): " + model.typed
		}

		return ansi.Hardwrap(cleanupText(prompt), model.width, false) + "\n"
	}

	lines := []string{
		"Worktree and branch cleanup",
		"j/k: select  d: remove  b: remove + branch  r: refresh  q: quit",
		"PgUp/PgDown: scroll diff | origin refs are cached; fetch separately",
		"",
	}

	switch {
	case model.loading:
		lines = append(lines, "Loading repositories...")
	case len(model.entries) == 0:
		lines = append(lines, "No worktrees or local branches found.")
	default:
		count := min(cleanupListRows, max(1, model.height/cleanupListHeightRatio))
		start := max(0, model.cursor-count/2)

		start = min(start, max(0, len(model.entries)-count))
		for index := start; index < min(len(model.entries), start+count); index++ {
			entry := model.entries[index]

			cursor, kind, name := " ", "branch", entry.branch
			if index == model.cursor {
				cursor = ">"
			}

			if entry.path != "" {
				kind = cleanupWorktreeKind

				if name == "" {
					name = "(detached)"
				}
			}

			row := fmt.Sprintf("%s %s | %s | %s", cursor, filepath.Base(entry.repo), kind, name)
			if entry.blocked != "" {
				row += " [" + entry.blocked + "]"
			}

			lines = append(lines, row)
		}

		entry := model.entries[model.cursor]

		lines = append(
			lines,
			fmt.Sprintf("Item %d/%d | %s", model.cursor+1, len(model.entries), entry.repo),
		)
		if entry.path != "" {
			lines = append(lines, entry.path)
		}
	}

	footer := model.notice
	if model.busy {
		footer = "Working..."
	}

	footerLines := strings.Split(cleanupText(footer), "\n")

	lines = append(lines, "", "Diff preview:")
	preview := strings.Split(cleanupText(model.preview), "\n")
	space := max(0, model.height-len(lines)-len(footerLines)-cleanupPreviewPadding)
	start := min(model.offset, len(preview))
	lines = append(lines, preview[start:min(len(preview), start+space)]...)
	lines = append(lines, "")

	lines = append(lines, footerLines...)
	for index, line := range lines {
		lines[index] = ansi.Truncate(cleanupText(line), model.width, "…")
	}

	return strings.Join(lines, "\n") + "\n"
}

//nolint:gocritic // Value models keep asynchronous command snapshots independent.
func (model cleanupModel) load(notice string) tea.Cmd {
	return func() tea.Msg {
		entries, err := loadCleanupEntries(model.ctx, model.dir)

		return cleanupLoaded{entries: entries, err: err, notice: notice}
	}
}

//nolint:gocritic // Value models keep asynchronous command snapshots independent.
func (model cleanupModel) previewCmd() tea.Cmd {
	if len(model.entries) == 0 {
		return nil
	}

	entry := model.entries[model.cursor]

	return func() tea.Msg { return cleanupPreviewMsg{entry: entry, text: cleanupPreview(model.ctx, entry)} }
}

//nolint:ireturn,gocritic // Bubble Tea uses immutable value model snapshots.
func (model cleanupModel) forceKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		model.forceConfirm = false
		model.typed = ""
	case "backspace", "ctrl+h":
		if model.typed != "" {
			model.typed = model.typed[:len(model.typed)-1]
		}
	case "enter":
		if model.typed == "delete" {
			model.forceConfirm = false

			return model.startDelete(true)
		}
	default:
		if key.Type == tea.KeyRunes {
			for _, char := range key.Runes {
				if char >= 'a' && char <= 'z' && len(model.typed) < 6 {
					model.typed += string(char)
				}
			}
		}
	}

	return model, nil
}

//nolint:ireturn,gocritic // Bubble Tea uses immutable value model snapshots.
func (model cleanupModel) startDelete(force bool) (tea.Model, tea.Cmd) {
	model.busy = true
	entry, both := model.entries[model.cursor], model.both

	return model, func() tea.Msg {
		err := cleanupDelete(model.ctx, entry, both, force)

		notice := "Removed selected item."
		if err != nil {
			notice = err.Error()
		}

		dir := model.dir

		_, statErr := os.Stat(dir)
		if os.IsNotExist(statErr) {
			dir = entry.repo
		}

		entries, loadErr := loadCleanupEntries(model.ctx, dir)

		return cleanupLoaded{entries: entries, err: loadErr, notice: notice, dir: dir}
	}
}

func cleanupText(text string) string {
	return strings.Map(func(char rune) rune {
		if char != '\n' && char != '\t' && unicode.IsControl(char) {
			return -1
		}

		return char
	}, text)
}

//nolint:gocritic // Value models keep asynchronous command snapshots independent.
func (model cleanupModel) action() string {
	entry := model.entries[model.cursor]
	if entry.path == "" {
		return "delete branch " + entry.branch
	}

	if model.both {
		return "remove worktree " + entry.path + " and delete branch " + entry.branch
	}

	return "remove worktree " + entry.path + " (keep branch)"
}
