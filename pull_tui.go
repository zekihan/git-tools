package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-isatty"

	tea "charm.land/bubbletea/v2"
)

func handlePullCmd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("pull", flag.ContinueOnError)

	flags.Usage = printPullHelp

	err := flags.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return fmt.Errorf("parse pull options: %w", err)
	}

	if flags.NArg() > 1 {
		return errPullArguments
	}

	if *threadCount < 1 {
		return errInvalidThreads
	}

	dir := "$HOME/repos"
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}

	dir, err = filepath.Abs(os.ExpandEnv(dir))
	if err != nil {
		return fmt.Errorf("resolve pull directory: %w", err)
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return errPullTerminal
	}

	runCtx, cancel := context.WithCancel(ctx)
	session := startPullSession(runCtx, dir, *threadCount)

	profile, noColor := tuiColorProfile(
		colorprofile.Detect(os.Stdout, os.Environ()),
		os.Getenv("NO_COLOR"),
		os.Getenv("CLICOLOR"),
		os.Getenv("CLICOLOR_FORCE"),
	)
	model := &pullModel{
		lightBackground: tuiLightBackground(
			os.Getenv("COLORFGBG"),
		),
		colorless: profile <= colorprofile.ASCII,
		noColor:   noColor,
		dir:       dir,
		session:   session,
		cancel:    cancel,
		scanning:  true,
		width:     tuiInitialWidth,
		height:    tuiInitialHeight,
	}

	defer func() { cancel(); <-model.session.done }()

	_, err = tea.NewProgram(model, tea.WithColorProfile(profile), tea.WithContext(ctx)).Run()
	if err != nil && !model.stopping {
		return fmt.Errorf("run pull: %w", err)
	}

	return model.exitError()
}

type pullModel struct {
	notice          string
	err             error
	session         *pullSession
	cancel          context.CancelFunc
	dir             string
	entries         []pullEntry
	width           int
	cursor          int
	offset          int
	height          int
	hideSucceeded   bool
	scanning        bool
	done            bool
	stopping        bool
	lightBackground bool
	colorless       bool
	noColor         bool
}

func (model *pullModel) Init() tea.Cmd {
	return tea.Batch(model.session.next, tea.RequestBackgroundColor)
}

//nolint:ireturn // Bubble Tea requires tea.Model.
func (model *pullModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		model.colorless = model.noColor || msg.Profile <= colorprofile.ASCII
	case tea.BackgroundColorMsg:
		model.lightBackground = !msg.IsDark()
	case tea.WindowSizeMsg:
		model.width, model.height = msg.Width, msg.Height
	case pullScanMsg:
		model.scanning, model.err = false, msg.err
		for _, path := range msg.paths {
			model.entries = append(
				model.entries,
				pullEntry{path: path, result: pullResult{status: pullQueued}},
			)
		}

		return model, model.session.next
	case pullStartedMsg:
		model.entries[int(msg)].result = pullResult{status: pullRunning}

		return model, model.session.next
	case pullFinishedMsg:
		selected := model.selectedIndex()
		model.entries[msg.index].result = msg.result
		model.selectEntry(selected)

		return model, model.session.next
	case pullDoneMsg:
		model.done = true
		if model.stopping {
			return model, tea.Quit
		}
	case pullShellFinishedMsg:
		model.shellFinished(msg)
	case tea.KeyPressMsg:
		cmd := model.key(msg.String())

		return model, cmd
	}

	return model, nil
}

func (model *pullModel) View() tea.View {
	content := renderPull(model)
	if model.colorless {
		content = ansi.Strip(content)
	}

	view := tea.NewView(content)
	view.AltScreen = true

	return view
}

func (model *pullModel) key(key string) tea.Cmd {
	model.notice = ""

	switch key {
	case "q", "esc", "ctrl+c":
		if model.done {
			return tea.Quit
		}

		model.stopping = true
		model.cancel()
	case "r":
		return model.retrySelected()
	case "s":
		return model.openShell()
	case "h":
		selected := model.selectedIndex()
		model.hideSucceeded = !model.hideSucceeded
		model.selectEntry(selected)
		model.offset = 0
	case "up", "k":
		model.cursor = max(0, model.cursor-1)
		model.offset = 0
	case "down", "j":
		model.cursor = max(0, min(len(model.visibleIndices())-1, model.cursor+1))
		model.offset = 0
	case "pgup":
		model.offset = max(0, model.offset-tuiScrollStep)
	case "pgdown":
		model.offset = min(max(0, len(model.detailLines())-1), model.offset+tuiScrollStep)
	}

	return nil
}

func (model *pullModel) exitError() error {
	if model.err != nil {
		return model.err
	}

	if model.stopping {
		return context.Canceled
	}

	failures := 0

	for _, entry := range model.entries {
		if entry.result.status == pullFailed {
			failures++
		}
	}

	if failures > 0 {
		return fmt.Errorf("%w: %d failed", ErrSomeReposFailed, failures)
	}

	return nil
}

func (model *pullModel) visibleIndices() []int {
	indices := make([]int, 0, len(model.entries))
	for index, entry := range model.entries {
		if !model.hideSucceeded || entry.result.status != pullSucceeded {
			indices = append(indices, index)
		}
	}

	return indices
}

func (model *pullModel) selectedIndex() int {
	indices := model.visibleIndices()
	if model.cursor < 0 || model.cursor >= len(indices) {
		return -1
	}

	return indices[model.cursor]
}

func (model *pullModel) selectEntry(selected int) {
	indices := model.visibleIndices()
	for cursor, index := range indices {
		if index == selected {
			model.cursor = cursor

			return
		}
	}

	model.cursor = max(0, min(model.cursor, len(indices)-1))
	model.offset = 0
}

func (model *pullModel) retrySelected() tea.Cmd {
	selected := model.selectedIndex()
	if selected < 0 {
		model.notice = "Select a repository first."

		return nil
	}

	if !model.done || model.stopping {
		model.notice = "Wait for the current pulls to finish before retrying."

		return nil
	}

	model.entries[selected].result = pullResult{status: pullRunning}
	model.offset = 0
	model.done = false
	model.session = model.session.retry(model.entries[selected].path, selected)

	return model.session.next
}
