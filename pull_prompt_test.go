package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPullCanReadLongHostVerificationPrompt(t *testing.T) {
	const (
		promptPageCount = 6
		detailOffset    = 7
		fingerprint     = "SHA256:0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"
	)

	model := &pullModel{
		dir:       "/repos",
		width:     tuiMinWidth,
		height:    tuiMinHeight,
		offset:    detailOffset,
		colorless: true,
		prompts: []*askpassPrompt{{
			kind: askpassVisible,
			repo: "/repos/local",
			text: "The authenticity of host 'git.example.test' can't be established.\n" +
				"ED25519 key fingerprint is " + fingerprint + ".\n" +
				"This key is not known by any other names.\n" +
				"Are you sure you want to continue connecting (yes/no/[fingerprint])? ",
		}},
	}
	_, _ = model.Update(tea.PasteMsg{Content: "yes"})

	lastPage := model.View().Content
	if !strings.Contains(lastPage, "> yes") {
		t.Fatal("the prompt hid the answer")
	}

	var pages strings.Builder

	for range promptPageCount {
		_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})

		view := model.View().Content
		if !strings.Contains(view, "> yes") {
			t.Fatal("paging hid the answer")
		}

		pages.WriteString(view)
	}

	for _, want := range []string{"SHA256:0123456789", "/repos/local"} {
		if !strings.Contains(pages.String(), want) {
			t.Errorf("could not page back to %q in the host verification prompt", want)
		}
	}

	for range promptPageCount {
		_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}

	if model.View().Content != lastPage || model.offset != detailOffset {
		t.Fatal("paging did not return to the end of the prompt or changed the Git output offset")
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	_, _ = model.Update(tea.PasteMsg{Content: fingerprint})

	view := model.View().Content
	for _, line := range []string{"> SHA256:0123456789abcdef", "ghijklmnopqrstuvwxyzABCDE", "FG"} {
		if !strings.Contains(view, line) {
			t.Errorf("pasted fingerprint line %q is missing:\n%s", line, view)
		}
	}
}
