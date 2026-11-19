package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name     string
		contains string
		args     []string
		code     int
	}{
		{name: "no args", args: nil, code: 1, contains: "Usage: git-tools"},
		{name: "version flag", args: []string{"--version"}, code: 0, contains: "git-tools "},
		{name: "version command", args: []string{"version"}, code: 0, contains: "git-tools "},
		{name: "help", args: []string{"help"}, code: 0, contains: "Usage: git-tools"},
		{
			name:     "help pull",
			args:     []string{"help", "pull"},
			code:     0,
			contains: "Usage: git-tools pull",
		},
		{
			name:     "help clone",
			args:     []string{"help", "clone"},
			code:     0,
			contains: "Usage: git-tools clone",
		},
		{
			name:     "help repopath",
			args:     []string{"help", "repopath"},
			code:     0,
			contains: "Usage: git-tools repopath",
		},
		{
			name:     "help cleanup",
			args:     []string{"help", "cleanup"},
			code:     0,
			contains: "Usage: git-tools cleanup",
		},
		{
			name:     "help unknown",
			args:     []string{"help", "bogus"},
			code:     1,
			contains: "Unknown command: bogus",
		},
		{name: "unknown", args: []string{"bogus"}, code: 1, contains: "Unknown command: bogus"},
		{name: "invalid threads", args: []string{"--threads=0", "pull"}, code: 1, contains: ""},
		{
			name:     "repopath",
			args:     []string{"repopath", "https://github.com/example/repo.git"},
			code:     0,
			contains: filepath.Join(home, "repos", "github.com", "example", "repo"),
		},
		{
			name:     "invalid repopath",
			args:     []string{"repopath", "a", "b", "c"},
			code:     1,
			contains: "",
		},
		{name: "invalid clone", args: []string{"clone", "a", "b", "c"}, code: 1, contains: ""},
		{name: "invalid cleanup", args: []string{"cleanup", "a", "b"}, code: 1, contains: ""},
		{name: "verbose", args: []string{"-v", "help"}, code: 0, contains: "Usage: git-tools"},
		{name: "global help flag", args: []string{"--help"}, code: 0, contains: "Usage: git-tools"},
		{name: "invalid flag", args: []string{"--bogus"}, code: 2, contains: "Usage: git-tools"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.code {
				t.Errorf("run() = %d, want %d", got, tt.code)
			}

			if !strings.Contains(stdout.String(), tt.contains) {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.contains)
			}
		})
	}
}

type unwritableRepopathOutput struct{}

func (unwritableRepopathOutput) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestRunRepopathReportsOutputFailure(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	var stderr bytes.Buffer

	args := []string{"repopath", "https://example.invalid/team/repo.git"}
	if code := run(args, unwritableRepopathOutput{}, &stderr); code != 1 {
		t.Errorf("run() = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
		t.Errorf("stderr = %q, want output write error", stderr.String())
	}
}
