package main

import (
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

const pullPromptReservedRows = 3 // Heading, paging hint, and controls.

func waitAskpassClosed(prompt *askpassPrompt) tea.Cmd {
	return func() tea.Msg {
		<-prompt.done

		return askpassClosedMsg{prompt: prompt}
	}
}

// updatePrompt handles askpass messages and, while Git waits for an answer,
// all keyboard input so typed secrets never reach the list shortcuts.
func (model *pullModel) updatePrompt(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case askpassPromptMsg:
		model.prompts = append(model.prompts, msg.prompt)

		return tea.Batch(model.askpass.next, waitAskpassClosed(msg.prompt)), true
	case askpassClosedMsg:
		model.removePrompt(msg.prompt)

		return nil, true
	case tea.PasteMsg:
		if model.activePrompt() != nil {
			model.typePrompt(msg.Content)
		}

		return nil, true
	case tea.KeyPressMsg:
		if prompt := model.activePrompt(); prompt != nil {
			return model.promptKey(prompt, msg), true
		}
	}

	return nil, false
}

// activePrompt returns the prompt that receives keyboard input. Notifications,
// such as security-key touch requests, need no answer and never capture input.
func (model *pullModel) activePrompt() *askpassPrompt {
	for _, prompt := range model.prompts {
		if prompt.kind != askpassNotify {
			return prompt
		}
	}

	return nil
}

func (model *pullModel) removePrompt(prompt *askpassPrompt) {
	if prompt == model.activePrompt() {
		model.clearInput()
	}

	model.prompts = slices.DeleteFunc(model.prompts, func(item *askpassPrompt) bool {
		return item == prompt
	})
}

func (model *pullModel) clearInput() {
	clear(model.input)
	model.input = model.input[:0]
}

func (model *pullModel) answerPrompt(prompt *askpassPrompt, answer string, ok bool) {
	prompt.respond(answer, ok)
	model.removePrompt(prompt)
}

func (model *pullModel) typePrompt(text string) {
	for _, char := range text {
		if len(model.input) >= askpassInputLimit {
			return
		}

		if !unicode.IsControl(char) {
			model.input = append(model.input, char)
		}
	}
}

func (model *pullModel) promptKey(prompt *askpassPrompt, msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()

	switch {
	case key == "ctrl+c":
		model.answerPrompt(prompt, "", false)

		return model.key(key)
	case msg.Code == tea.KeyEscape, prompt.kind == askpassConfirm && key == "n":
		model.answerPrompt(prompt, "", false)
	case msg.Code == tea.KeyPgUp, msg.Code == tea.KeyPgDown:
		model.pagePrompt(prompt, msg.Code == tea.KeyPgDown)
	case prompt.kind == askpassConfirm:
		if key == "y" || msg.Code == tea.KeyEnter {
			model.answerPrompt(prompt, "yes", true)
		}
	case msg.Code == tea.KeyEnter:
		model.answerPrompt(prompt, string(model.input), true)
	case msg.Code == tea.KeyBackspace:
		if len(model.input) > 0 {
			model.input[len(model.input)-1] = 0
			model.input = model.input[:len(model.input)-1]
		}
	case key == "ctrl+u":
		model.clearInput()
	default:
		model.typePrompt(msg.Text)
	}

	return nil
}

func (model *pullModel) pagePrompt(prompt *askpassPrompt, down bool) {
	if model.width < tuiMinWidth || model.height < tuiMinHeight {
		return
	}

	leftWidth := min(pullMaxLeftWidth, model.width/pullLeftWidthRatio)
	inner := model.width - leftWidth - 1 - tuiPanelFrame
	body, _, rows := model.promptContent(prompt, inner, model.height-tuiChromeHeight)
	lastPage := max(0, len(body)-rows)
	prompt.scrollBack = lastPage - tuiScrollPage(lastPage-prompt.scrollBack, len(body), rows, down)
}

func (model *pullModel) promptNotice() string {
	for _, prompt := range model.prompts {
		if prompt.kind == askpassNotify {
			return strings.TrimSpace(prompt.text)
		}
	}

	return ""
}

func (model *pullModel) promptContent(
	prompt *askpassPrompt,
	inner, height int,
) ([]string, []string, int) {
	text := strings.TrimRight(prompt.text, " \n")

	if prompt.repo != "" {
		text = "Repository: " + prompt.repo + "\n\n" + text
	}

	text = strings.ReplaceAll(tuiText(text), "\t", "    ")
	body := strings.Split(ansi.Hardwrap(text, inner, false), "\n")

	var input string

	switch prompt.kind {
	case askpassConfirm:
		input = "[y/n]"
	case askpassSecret:
		input = strings.Repeat("*", len(model.input))
	case askpassVisible, askpassNotify:
		input = string(model.input)
	default:
	}

	inputLines := strings.Split(ansi.Hardwrap("> "+input, inner, true), "\n")
	// Reserve at least one row for the prompt regardless of how much was typed.
	maxInputRows := tuiBodyRows(height, pullPromptReservedRows+1)
	inputLines = inputLines[max(0, len(inputLines)-maxInputRows):]

	return body, inputLines, tuiBodyRows(height, pullPromptReservedRows+len(inputLines))
}

func renderPullPrompt(model *pullModel, width, height int, styles *tuiStyles) string {
	inner := width - tuiPanelFrame
	prompt := model.activePrompt()
	body, input, rows := model.promptContent(prompt, inner, height)
	lastPage := max(0, len(body)-rows)
	start := max(0, lastPage-prompt.scrollBack)

	hint := ""
	if lastPage > 0 {
		hint = "PgUp/Dn read prompt"
	}

	lines := []string{
		styles.heading.Render(tuiClip("GIT NEEDS INPUT", inner)),
		styles.muted.Render(tuiClip(hint, inner)),
	}
	for _, line := range body[start:min(start+rows, len(body))] {
		lines = append(lines, tuiClip(line, inner))
	}

	help := "enter submit  esc cancel"
	if prompt.kind == askpassConfirm {
		help = "y allow  n deny"
	}

	for len(lines) < height-tuiPanelBorder-len(input)-1 {
		lines = append(lines, "")
	}

	for _, line := range input {
		lines = append(lines, tuiClip(line, inner))
	}

	lines = append(lines, styles.muted.Render(tuiClip(help, inner)))

	return tuiPanel(strings.Join(lines, "\n"), width, height, true, styles)
}
