package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

//nolint:forbidigo // TestMain must report the helper or test exit status.
func TestMain(m *testing.M) {
	// Git runs this test binary as its askpass helper in the integration tests.
	if isAskpassInvocation() {
		os.Exit(runAskpass(os.Args[1:], os.Stdout, os.Stderr))
	}

	os.Exit(m.Run())
}

func askpassTestServer(t *testing.T) *askpassServer {
	t.Helper()

	server, err := startAskpassServer(t.Context())
	if err != nil {
		t.Skipf("askpass socket unavailable: %v", err)
	}

	t.Cleanup(server.shutdown)

	return server
}

func askpassTestPrompt(t *testing.T, server *askpassServer) *askpassPrompt {
	t.Helper()

	result := make(chan tea.Msg, 1)
	go func() { result <- server.next() }()

	select {
	case msg := <-result:
		prompt, ok := msg.(askpassPromptMsg)
		if !ok {
			t.Fatalf("unexpected message %#v", msg)
		}

		return prompt.prompt
	case <-time.After(10 * time.Second):
		t.Fatal("no prompt reached the TUI")
	}

	return nil
}

func TestAskpassAnswersGitCredentialPrompts(t *testing.T) {
	server := askpassTestServer(t)

	cmd := exec.CommandContext(
		t.Context(),
		"git",
		"-c",
		"credential.helper=",
		"credential",
		"fill",
	)
	cmd.Dir = t.TempDir()
	cmd.Stdin = strings.NewReader("protocol=https\nhost=example.invalid\n\n")

	cmd.Env = append(
		os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+t.TempDir(),
		askpassRepoEnv+"=/repos/example",
	)
	cmd.Env = append(cmd.Env, server.env()...)
	detachGitProcess(cmd)

	type outcome struct {
		err    error
		output []byte
	}

	finished := make(chan outcome, 1)

	go func() {
		output, err := cmd.Output()
		finished <- outcome{output: output, err: err}
	}()

	username := askpassTestPrompt(t, server)
	if username.kind != askpassVisible || !strings.Contains(username.text, "Username") ||
		username.repo != "/repos/example" {
		t.Fatalf("unexpected username prompt: %+v", username)
	}

	username.respond("alice", true)

	password := askpassTestPrompt(t, server)
	if password.kind != askpassSecret || !strings.Contains(password.text, "Password") {
		t.Fatalf("unexpected password prompt: %+v", password)
	}

	password.respond("s3cret value", true)

	result := <-finished
	if result.err != nil {
		t.Fatalf("credential fill failed: %v", result.err)
	}

	if !strings.Contains(string(result.output), "username=alice\n") ||
		!strings.Contains(string(result.output), "password=s3cret value\n") {
		t.Fatalf("answers not returned to Git: %q", result.output)
	}
}

func TestAskpassCancelFailsGitWithoutTerminalPrompt(t *testing.T) {
	server := askpassTestServer(t)

	cmd := exec.CommandContext(t.Context(), "git", "-c", "credential.helper=", "credential", "fill")
	cmd.Dir = t.TempDir()
	cmd.Stdin = strings.NewReader("protocol=https\nhost=example.invalid\n\n")

	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "HOME="+t.TempDir())
	cmd.Env = append(cmd.Env, server.env()...)
	detachGitProcess(cmd)

	finished := make(chan error, 1)
	go func() { finished <- cmd.Run() }()

	askpassTestPrompt(t, server).respond("", false)

	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("canceled prompt still succeeded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Git did not stop after cancel")
	}
}

type askpassAskResult struct {
	response askpassResponse
	ok       bool
}

func askpassTestAsk(
	server *askpassServer,
	parent int,
	gone chan struct{},
) chan askpassAskResult {
	result := make(chan askpassAskResult, 1)

	go func() {
		request := &askpassRequest{
			Kind:   askpassSecret,
			Prompt: "Enter passphrase for key '/k': ",
			Parent: parent,
		}
		prompt := &askpassPrompt{
			reply: make(chan askpassResponse, 1),
			done:  make(chan struct{}),
			kind:  request.Kind,
			text:  request.Prompt,
		}

		response, ok := server.ask(request, prompt, gone)
		result <- askpassAskResult{response: response, ok: ok}
	}()

	return result
}

func askpassTestNoPrompt(t *testing.T, server *askpassServer) {
	t.Helper()

	select {
	case prompt := <-server.prompts:
		t.Fatalf("unexpected prompt %+v", prompt)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAskpassSharesAnswersAcrossConcurrentRequests(t *testing.T) {
	server := askpassTestServer(t)
	gone := make(chan struct{})

	first := askpassTestAsk(server, 1, gone)
	prompt := askpassTestPrompt(t, server)
	second := askpassTestAsk(server, 2, gone)

	askpassTestNoPrompt(t, server)
	prompt.respond("passphrase", true)

	for _, result := range []chan askpassAskResult{first, second} {
		got := <-result
		if !got.ok || !got.response.OK || got.response.Answer != "passphrase" {
			t.Fatalf("unexpected result %+v", got)
		}
	}

	// The same ssh process asking again means the passphrase was wrong.
	retry := askpassTestAsk(server, 1, gone)
	askpassTestPrompt(t, server).respond("corrected", true)

	got := <-retry
	if got.response.Answer != "corrected" {
		t.Fatalf("rejected answer was reused: %+v", got)
	}

	got = <-askpassTestAsk(server, 3, gone)
	if got.response.Answer != "corrected" {
		t.Fatalf("corrected answer not shared: %+v", got)
	}

	server.forget()

	forgotten := askpassTestAsk(server, 4, gone)
	askpassTestPrompt(t, server).respond("again", true)

	if got := <-forgotten; got.response.Answer != "again" {
		t.Fatalf("forget kept answer: %+v", got)
	}
}

func askpassTestWaitRequests(t *testing.T, server *askpassServer, count uint64) {
	t.Helper()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Second)

	for {
		server.mu.Lock()
		sequence := server.sequence
		server.mu.Unlock()

		if sequence >= count {
			return
		}

		select {
		case <-ticker.C:
		case <-timeout:
			t.Fatal("requests did not arrive")
		}
	}
}

func TestAskpassCancelAppliesToWaitingRequests(t *testing.T) {
	server := askpassTestServer(t)
	gone := make(chan struct{})

	first := askpassTestAsk(server, 1, gone)
	prompt := askpassTestPrompt(t, server)
	second := askpassTestAsk(server, 2, gone)

	askpassTestWaitRequests(t, server, 2)
	prompt.respond("", false)

	for _, result := range []chan askpassAskResult{first, second} {
		got := <-result
		if !got.ok || got.response.OK {
			t.Fatalf("waiting request was not canceled: %+v", got)
		}
	}

	later := askpassTestAsk(server, 3, gone)
	askpassTestPrompt(t, server).respond("now", true)

	if got := <-later; got.response.Answer != "now" {
		t.Fatalf("later request inherited cancel: %+v", got)
	}
}

func TestAskpassDropsPromptWhenRequesterExits(t *testing.T) {
	server := askpassTestServer(t)

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", server.socket)
	if err != nil {
		t.Fatal(err)
	}

	err = json.NewEncoder(conn).Encode(askpassRequest{
		Token:  server.token,
		Kind:   askpassNotify,
		Prompt: "Confirm user presence for key ED25519-SK",
	})
	if err != nil {
		t.Fatal(err)
	}

	prompt := askpassTestPrompt(t, server)
	if prompt.kind != askpassNotify {
		t.Fatalf("unexpected prompt %+v", prompt)
	}

	_ = conn.Close()

	select {
	case <-prompt.done:
	case <-time.After(5 * time.Second):
		t.Fatal("prompt outlived its requester")
	}
}

func TestAskpassRejectsWrongToken(t *testing.T) {
	server := askpassTestServer(t)

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", server.socket)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = conn.Close() }()

	err = json.NewEncoder(conn).Encode(askpassRequest{Token: "wrong", Prompt: "Password: "})
	if err != nil {
		t.Fatal(err)
	}

	askpassTestNoPrompt(t, server)
}

func TestAskpassKindFor(t *testing.T) {
	for _, test := range []struct {
		prompt, hint string
		want         askpassKind
	}{
		{"Enter passphrase for key '/home/u/.ssh/id_ed25519': ", "", askpassSecret},
		{"Password for 'https://u@example.com': ", "", askpassSecret},
		{"Username for 'https://example.com': ", "", askpassVisible},
		{
			"Are you sure you want to continue connecting (yes/no/[fingerprint])? ",
			"",
			askpassVisible,
		},
		{"Allow use of key?", "confirm", askpassConfirm},
		{"Confirm user presence for key", "none", askpassNotify},
	} {
		if got := askpassKindFor(test.prompt, test.hint); got != test.want {
			t.Errorf("%q: got %s, want %s", test.prompt, got, test.want)
		}
	}
}

func TestPullModelAnswersPromptsWithoutLeakingInput(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	model := &pullModel{
		session: &pullSession{events: make(chan tea.Msg)},
		askpass: askpassTestServer(t),
		cancel:  cancel,
		width:   100,
		height:  24,
		dir:     "/repos",
	}
	prompt := &askpassPrompt{
		reply: make(chan askpassResponse, 1),
		done:  make(chan struct{}),
		kind:  askpassSecret,
		text:  "Enter passphrase for key '/home/u/.ssh/id_ed25519': ",
		repo:  "/repos/example",
	}

	_, _ = model.Update(askpassPromptMsg{prompt: prompt})

	for _, key := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'x', Text: "x"},
		{Code: tea.KeyBackspace},
	} {
		_, _ = model.Update(key)
	}

	_, _ = model.Update(tea.PasteMsg{Content: "pass\nword"})

	view := model.View().Content
	if ctx.Err() != nil || model.stopping {
		t.Fatal("typing into the prompt quit the TUI")
	}

	if strings.Contains(view, "password") || !strings.Contains(view, "*********") ||
		!strings.Contains(
			view,
			"Enter passphrase for key",
		) || !strings.Contains(view, "/repos/example") {
		t.Fatalf("prompt not rendered safely:\n%s", view)
	}

	_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := <-prompt.reply; !got.OK || got.Answer != "qpassword" {
		t.Fatalf("unexpected answer %+v", got)
	}

	if model.activePrompt() != nil || len(model.input) != 0 {
		t.Fatal("answered prompt still active")
	}

	// A closed message after answering must be harmless.
	_, _ = model.Update(askpassClosedMsg{prompt: prompt})
}

func TestPullModelPromptCancelQuitAndNotify(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	model := &pullModel{
		session: &pullSession{events: make(chan tea.Msg)},
		askpass: askpassTestServer(t),
		cancel:  cancel,
		width:   100,
		height:  24,
		dir:     "/repos",
	}
	newPrompt := func(kind askpassKind, text string) *askpassPrompt {
		return &askpassPrompt{
			reply: make(chan askpassResponse, 1),
			done:  make(chan struct{}),
			kind:  kind,
			text:  text,
		}
	}

	notify := newPrompt(askpassNotify, "Confirm user presence for key ED25519-SK")
	_, _ = model.Update(askpassPromptMsg{prompt: notify})

	if model.activePrompt() != nil ||
		!strings.Contains(model.View().Content, "Confirm user presence") {
		t.Fatal("notification not shown as a notice")
	}

	_, _ = model.Update(askpassClosedMsg{prompt: notify})
	if strings.Contains(model.View().Content, "Confirm user presence") {
		t.Fatal("notification outlived ssh")
	}

	confirm := newPrompt(askpassConfirm, "Allow use of key?")
	_, _ = model.Update(askpassPromptMsg{prompt: confirm})
	_, _ = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})

	if got := <-confirm.reply; got.OK {
		t.Fatal("confirm denial allowed")
	}

	secret := newPrompt(askpassSecret, "Password: ")
	_, _ = model.Update(askpassPromptMsg{prompt: secret})
	_, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if got := <-secret.reply; got.OK {
		t.Fatal("escape submitted the prompt")
	}

	waiting := newPrompt(askpassSecret, "Password: ")
	_, _ = model.Update(askpassPromptMsg{prompt: waiting})
	_, _ = model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if got := <-waiting.reply; got.OK || ctx.Err() == nil || !model.stopping {
		t.Fatal("ctrl+c did not cancel the prompt and quit")
	}
}

func TestPullModelPromptFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{60, 16}, {80, 24}} {
		model := &pullModel{width: size[0], height: size[1], dir: "/repos"}
		model.prompts = []*askpassPrompt{{
			kind: askpassVisible,
			text: strings.Repeat("The authenticity of host can't be established.\n", 20) +
				"Are you sure you want to continue connecting (yes/no/[fingerprint])? ",
			repo: "/repos/" + strings.Repeat("name", 30),
		}}
		model.input = []rune("yes")

		view := model.View().Content
		lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

		if len(lines) > model.height || !strings.Contains(view, "> yes") {
			t.Fatalf("prompt overflowed or hid input at %v:\n%s", size, view)
		}
	}
}
