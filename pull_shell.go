package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"
)

type pullShellFinishedMsg struct{ err error }

func repositoryShell(dir string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	args := []string{"-i"}

	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}

		args = nil
	} else if shell == "" {
		shell = "/bin/sh"
	}

	//nolint:gosec,noctx // Bubble Tea owns the terminal and lifetime of the user-configured shell.
	cmd := exec.Command(shell, args...)
	cmd.Dir = dir
	cmd.Env = cmd.Environ()

	return cmd
}

func (model *pullModel) openShell() tea.Cmd {
	selected := model.selectedIndex()
	if selected < 0 {
		model.notice = "Select a repository first."

		return nil
	}

	entry := model.entries[selected]
	if entry.result.status == pullQueued || entry.result.status == pullRunning {
		model.notice = "Wait for this repository's pull to finish before opening a shell."

		return nil
	}

	return tea.ExecProcess(repositoryShell(entry.path), func(err error) tea.Msg {
		return pullShellFinishedMsg{err: err}
	})
}

func (model *pullModel) shellFinished(msg pullShellFinishedMsg) {
	model.notice = "Returned from shell. Press r to retry the selected repository."
	if msg.err != nil {
		model.notice = fmt.Sprintf("Shell: %v", msg.err)
	}
}
