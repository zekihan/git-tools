package main

import (
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

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

func (model *pullModel) promptNotice() string {
	for _, prompt := range model.prompts {
		if prompt.kind == askpassNotify {
			return strings.TrimSpace(prompt.text)
		}
	}

	return ""
}

func (model *pullModel) promptLines(inner int) []string {
	prompt := model.activePrompt()
	text := strings.TrimRight(prompt.text, " \n")

	if prompt.repo != "" {
		text = "Repository: " + prompt.repo + "\n\n" + text
	}

	input := string(model.input)

	switch prompt.kind {
	case askpassConfirm:
		input = "[y/n]"
	case askpassSecret:
		input = strings.Repeat("*", len(model.input))
	case askpassVisible, askpassNotify:
	default:
	}

	text = strings.ReplaceAll(tuiText(text+"\n\n> "+input), "\t", "    ")

	return strings.Split(ansi.Hardwrap(text, max(1, inner), false), "\n")
}

func renderPullPrompt(model *pullModel, width, height int, styles *tuiStyles) string {
	inner := width - tuiPanelFrame
	lines := []string{styles.heading.Render(tuiClip("GIT NEEDS INPUT", inner)), ""}
	body := model.promptLines(inner)

	// Keep the input line visible when a long prompt does not fit.
	body = body[max(0, len(body)-(height-tuiPanelFrame-1)):]
	for _, line := range body {
		lines = append(lines, tuiClip(line, inner))
	}

	for len(lines) < height-tuiPanelBorder-1 {
		lines = append(lines, "")
	}

	help := "enter submit  esc cancel"
	if model.activePrompt().kind == askpassConfirm {
		help = "y allow  n deny"
	}

	lines = append(lines, styles.muted.Render(tuiClip(help, inner)))

	return tuiPanel(strings.Join(lines, "\n"), width, height, true, styles)
}
