package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// Git and ssh run the git-tools binary as their askpass program. These variables
// tell that child process to forward the prompt to the running TUI.
//
//nolint:gosec // Environment variable names, not credentials.
const (
	askpassHelperEnv = "GIT_TOOLS_ASKPASS"
	askpassSocketEnv = "GIT_TOOLS_ASKPASS_SOCKET"
	askpassTokenEnv  = "GIT_TOOLS_ASKPASS_TOKEN"
	askpassRepoEnv   = "GIT_TOOLS_ASKPASS_REPO"

	askpassRequestLimit = 64 << 10
	askpassInputLimit   = 4096
)

type askpassKind string

const (
	askpassSecret  askpassKind = "secret"
	askpassVisible askpassKind = "visible"
	askpassConfirm askpassKind = "confirm"
	askpassNotify  askpassKind = "notify"
)

var (
	errAskpassUnavailable = errors.New("git-tools prompt is unavailable")
	errAskpassCanceled    = errors.New("prompt canceled")
)

type askpassRequest struct {
	Token  string      `json:"token"`
	Kind   askpassKind `json:"kind"`
	Prompt string      `json:"prompt"`
	Repo   string      `json:"repo"`
	Parent int         `json:"parent"`
}

type askpassResponse struct {
	Answer string `json:"answer"`
	OK     bool   `json:"ok"`
}

// askpassPrompt is shown by the TUI until it is answered or its requester exits.
type askpassPrompt struct {
	reply chan askpassResponse
	done  chan struct{}
	kind  askpassKind
	text  string
	repo  string
}

func (prompt *askpassPrompt) respond(answer string, ok bool) {
	select {
	case prompt.reply <- askpassResponse{Answer: answer, OK: ok}:
	default:
	}
}

type askpassCacheEntry struct {
	parents map[int]struct{}
	answer  string
}

type askpassServer struct {
	listener net.Listener
	prompts  chan *askpassPrompt
	turn     chan struct{}
	closed   chan struct{}
	cache    map[string]askpassCacheEntry
	canceled map[string]uint64
	dir      string
	socket   string
	token    string
	helper   string
	conns    sync.WaitGroup
	mu       sync.Mutex
	once     sync.Once
	sequence uint64
}

func startAskpassServer(ctx context.Context) (*askpassServer, error) {
	helper, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate askpass helper: %w", err)
	}

	dir, err := os.MkdirTemp("", "git-tools-")
	if err != nil {
		return nil, fmt.Errorf("create askpass directory: %w", err)
	}

	socket := filepath.Join(dir, "askpass.sock")

	listener, err := (&net.ListenConfig{}).Listen(ctx, "unix", socket)
	if err != nil {
		_ = os.RemoveAll(dir)

		return nil, fmt.Errorf("listen for askpass: %w", err)
	}

	server := &askpassServer{
		listener: listener,
		prompts:  make(chan *askpassPrompt),
		turn:     make(chan struct{}, 1),
		closed:   make(chan struct{}),
		cache:    make(map[string]askpassCacheEntry),
		canceled: make(map[string]uint64),
		dir:      dir,
		socket:   socket,
		token:    rand.Text(),
		helper:   helper,
	}
	server.conns.Go(server.accept)

	return server, nil
}

// env routes ssh passphrase, host-key, and Git credential prompts to the TUI.
// SSH_ASKPASS_REQUIRE=force keeps ssh from opening /dev/tty when one exists.
func (server *askpassServer) env() []string {
	if server == nil {
		return nil
	}

	return []string{
		"GIT_ASKPASS=" + server.helper,
		"SSH_ASKPASS=" + server.helper,
		"SSH_ASKPASS_REQUIRE=force",
		askpassHelperEnv + "=" + server.helper,
		askpassSocketEnv + "=" + server.socket,
		askpassTokenEnv + "=" + server.token,
	}
}

func (server *askpassServer) shutdown() {
	if server == nil {
		return
	}

	server.once.Do(func() {
		close(server.closed)
		_ = server.listener.Close()
		server.conns.Wait()
		_ = os.RemoveAll(server.dir)
	})
}

// forget drops remembered answers so a retry can recover from a wrong secret.
func (server *askpassServer) forget() {
	if server == nil {
		return
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	clear(server.cache)
	clear(server.canceled)
}

//nolint:ireturn // Bubble Tea commands return messages.
func (server *askpassServer) next() tea.Msg {
	select {
	case prompt := <-server.prompts:
		return askpassPromptMsg{prompt: prompt}
	case <-server.closed:
		return nil
	}
}

func (server *askpassServer) accept() {
	for {
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}

		server.conns.Go(func() { server.handle(conn) })
	}
}

func (server *askpassServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	var request askpassRequest

	err := json.NewDecoder(io.LimitReader(conn, askpassRequestLimit)).Decode(&request)
	if err != nil ||
		subtle.ConstantTimeCompare([]byte(request.Token), []byte(server.token)) != 1 {
		return
	}

	// The requester keeps the connection open while it waits. EOF means ssh or
	// Git gave up, so the TUI must stop showing the prompt.
	gone := make(chan struct{})

	go func() {
		defer close(gone)

		_, copyErr := io.Copy(io.Discard, conn)
		if copyErr != nil {
			slog.Debug("Askpass requester disconnected", slog.Any("error", copyErr))
		}
	}()

	prompt := &askpassPrompt{
		reply: make(chan askpassResponse, 1),
		done:  make(chan struct{}),
		kind:  request.Kind,
		text:  request.Prompt,
		repo:  request.Repo,
	}
	defer close(prompt.done)

	if request.Kind == askpassNotify {
		if server.deliver(prompt, gone) {
			select {
			case <-gone:
			case <-server.closed:
			}
		}

		return
	}

	response, ok := server.ask(&request, prompt, gone)
	if !ok {
		return
	}

	err = json.NewEncoder(conn).Encode(response)
	if err != nil {
		slog.Debug("Askpass requester left before the answer", slog.Any("error", err))
	}
}

// ask shows one prompt at a time. Concurrent pulls commonly ask for the same key
// passphrase; once answered, waiting requests reuse it instead of prompting again.
func (server *askpassServer) ask(
	request *askpassRequest,
	prompt *askpassPrompt,
	gone <-chan struct{},
) (askpassResponse, bool) {
	server.mu.Lock()
	server.sequence++
	sequence := server.sequence
	server.mu.Unlock()

	select {
	case server.turn <- struct{}{}:
	case <-gone:
		return askpassResponse{}, false
	case <-server.closed:
		return askpassResponse{}, false
	}

	defer func() { <-server.turn }()

	response, ok := server.cached(request, sequence)
	if ok {
		return response, true
	}

	if !server.deliver(prompt, gone) {
		return askpassResponse{}, false
	}

	select {
	case response = <-prompt.reply:
	case <-gone:
		return askpassResponse{}, false
	case <-server.closed:
		return askpassResponse{}, false
	}

	server.remember(request, response)

	return response, true
}

func (server *askpassServer) deliver(prompt *askpassPrompt, gone <-chan struct{}) bool {
	select {
	case server.prompts <- prompt:
		return true
	case <-gone:
	case <-server.closed:
	}

	return false
}

func (server *askpassServer) cached(request *askpassRequest, sequence uint64) (
	askpassResponse,
	bool,
) {
	server.mu.Lock()
	defer server.mu.Unlock()

	// Canceling a prompt also cancels identical requests that were already waiting.
	if canceled, ok := server.canceled[request.Prompt]; ok && sequence <= canceled {
		return askpassResponse{}, true
	}

	entry, ok := server.cache[request.Prompt]
	if !ok || request.Kind == askpassConfirm {
		return askpassResponse{}, false
	}

	// The same process asking again means the remembered answer was rejected.
	if _, used := entry.parents[request.Parent]; used {
		delete(server.cache, request.Prompt)

		return askpassResponse{}, false
	}

	entry.parents[request.Parent] = struct{}{}

	return askpassResponse{Answer: entry.answer, OK: true}, true
}

func (server *askpassServer) remember(request *askpassRequest, response askpassResponse) {
	server.mu.Lock()
	defer server.mu.Unlock()

	if !response.OK {
		server.canceled[request.Prompt] = server.sequence

		return
	}

	delete(server.canceled, request.Prompt)

	if request.Kind != askpassConfirm {
		server.cache[request.Prompt] = askpassCacheEntry{
			parents: map[int]struct{}{request.Parent: {}},
			answer:  response.Answer,
		}
	}
}

func askpassKindFor(prompt, hint string) askpassKind {
	switch hint {
	case "confirm":
		return askpassConfirm
	case "none":
		return askpassNotify
	}

	lower := strings.ToLower(prompt)
	if strings.HasPrefix(lower, "username") || strings.Contains(lower, "(yes/no") {
		return askpassVisible
	}

	return askpassSecret
}

// isAskpassInvocation is true only when Git or ssh executes the exact helper path
// it was given, so commands run from Git hooks are not mistaken for prompts.
func isAskpassInvocation() bool {
	helper := os.Getenv(askpassHelperEnv)

	return helper != "" && len(os.Args) > 0 && os.Args[0] == helper &&
		os.Getenv(askpassSocketEnv) != ""
}

// runAskpass implements the SSH_ASKPASS and GIT_ASKPASS protocol: the prompt is
// the argument, the answer is written to stdout, and a non-zero exit cancels.
func runAskpass(args []string, stdout, stderr io.Writer) int {
	err := askpass(args, stdout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "git-tools askpass: %v\n", err)

		return 1
	}

	return 0
}

func askpass(args []string, stdout io.Writer) error {
	prompt := strings.Join(args, " ")
	kind := askpassKindFor(prompt, os.Getenv("SSH_ASKPASS_PROMPT"))

	conn, err := (&net.Dialer{}).DialContext(
		context.Background(),
		"unix",
		os.Getenv(askpassSocketEnv),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", errAskpassUnavailable, err)
	}

	defer func() { _ = conn.Close() }()

	err = json.NewEncoder(conn).Encode(askpassRequest{
		Token:  os.Getenv(askpassTokenEnv),
		Kind:   kind,
		Prompt: prompt,
		Repo:   os.Getenv(askpassRepoEnv),
		Parent: os.Getppid(),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errAskpassUnavailable, err)
	}

	if kind == askpassNotify {
		// OpenSSH terminates notification helpers when the operation completes.
		_, err = io.Copy(io.Discard, conn)
		if err != nil {
			return fmt.Errorf("%w: %w", errAskpassUnavailable, err)
		}

		return nil
	}

	var response askpassResponse

	err = json.NewDecoder(io.LimitReader(conn, askpassRequestLimit)).Decode(&response)
	if err != nil {
		return fmt.Errorf("%w: %w", errAskpassUnavailable, err)
	}

	if !response.OK {
		return errAskpassCanceled
	}

	answer := response.Answer
	if kind == askpassConfirm {
		answer = "yes"
	}

	_, err = fmt.Fprintln(stdout, answer)
	if err != nil {
		return fmt.Errorf("write answer: %w", err)
	}

	return nil
}

type (
	askpassPromptMsg struct{ prompt *askpassPrompt }
	askpassClosedMsg struct{ prompt *askpassPrompt }
)
