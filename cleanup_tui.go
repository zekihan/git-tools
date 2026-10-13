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

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

var errTooManyCleanupArgs = errors.New("too many arguments for cleanup command")

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
	ctx             context.Context
	preview         string
	dir             string
	typed           string
	notice          string
	entries         []cleanupEntry
	risks           []string
	offset          int
	height          int
	width           int
	cursor          int
	loading         bool
	busy            bool
	confirm         bool
	forceConfirm    bool
	both            bool
	lightBackground bool
	colorless       bool
	noColor         bool
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
		return fmt.Errorf("%w: expected at most one directory", errTooManyCleanupArgs)
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

	profile, noColor := tuiColorProfile(
		colorprofile.Detect(os.Stdout, os.Environ()),
		os.Getenv("NO_COLOR"), os.Getenv("CLICOLOR"), os.Getenv("CLICOLOR_FORCE"),
	)

	model := cleanupModel{
		ctx:             ctx,
		dir:             dir,
		width:           tuiInitialWidth,
		height:          tuiInitialHeight,
		loading:         true,
		lightBackground: tuiLightBackground(os.Getenv("COLORFGBG")),
		colorless:       profile <= colorprofile.ASCII,
		noColor:         noColor,
	}

	_, err = tea.NewProgram(model, tea.WithContext(ctx), tea.WithColorProfile(profile)).Run()
	if err != nil {
		slog.ErrorContext(ctx, "Cleanup failed", slog.Any("error", err))

		return fmt.Errorf("run cleanup: %w", err)
	}

	return nil
}

//nolint:gocritic // Value models keep asynchronous command snapshots independent.
func (model cleanupModel) Init() tea.Cmd {
	return tea.Batch(model.load(""), tea.RequestBackgroundColor)
}

//nolint:ireturn,gocyclo,gocritic // Bubble Tea events form one state machine with immutable snapshots.
func (model cleanupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		model.colorless = model.noColor || msg.Profile <= colorprofile.ASCII
	case tea.BackgroundColorMsg:
		model.lightBackground = !msg.IsDark()
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
	case tea.PasteMsg:
		return model.paste(msg.Content), nil
	case tea.KeyPressMsg:
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
				model.offset+tuiScrollStep,
			)
		case "pgup":
			model.offset = max(0, model.offset-tuiScrollStep)
		case "d", "b":
			if len(model.entries) > 0 && !model.loading {
				model.both, model.confirm = key == "b", true
			}
		}
	}

	return model, nil
}

//nolint:gocritic // Bubble Tea keeps immutable value model snapshots.
func (model cleanupModel) View() tea.View {
	content := renderCleanup(&model)
	if model.colorless {
		// Lip Gloss v1 omitted all styling, including bold, for colorless output.
		content = ansi.Strip(content)
	}

	view := tea.NewView(content)
	view.AltScreen = true

	return view
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
func (model cleanupModel) forceKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		model.typed = cleanupForceText(model.typed, key.Text)
	}

	return model, nil
}

//nolint:gocritic // Value models keep input updates independent.
func (model cleanupModel) paste(text string) cleanupModel {
	if model.busy {
		return model
	}

	if model.forceConfirm {
		model.typed = cleanupForceText(model.typed, text)
	} else {
		model.confirm = false
	}

	return model
}

// cleanupForceText preserves the same filtering for typed and pasted confirmation text.
func cleanupForceText(typed, text string) string {
	for _, char := range text {
		if char >= 'a' && char <= 'z' && len(typed) < 6 {
			typed += string(char)
		}
	}

	return typed
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
