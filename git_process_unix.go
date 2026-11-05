//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// detachGitProcess starts Git in its own session. Without a controlling terminal,
// ssh, credential helpers, and pinentry cannot read from or draw over the TUI;
// they must use the askpass bridge instead. Cancellation kills the whole session
// so ssh and askpass children do not outlive Git.
func detachGitProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}

		if err != nil {
			return cmd.Process.Kill()
		}

		return nil
	}
}
