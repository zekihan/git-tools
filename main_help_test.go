package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWritesCompleteGlobalUsageToStdout(t *testing.T) {
	for _, test := range []struct {
		name       string
		diagnostic string
		args       []string
		code       int
	}{
		{name: "help flag", args: []string{"--help"}},
		{name: "help command", args: []string{"help"}},
		{name: "no command", code: 1},
		{name: "unknown help topic", args: []string{"help", "missing"}, code: 1},
		{name: "unknown command", args: []string{"missing"}, code: 1},
		{
			name:       "unknown flag",
			args:       []string{"--missing"},
			code:       flagErrorExitCode,
			diagnostic: "flag provided but not defined: -missing\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(test.args, &stdout, &stderr); got != test.code {
				t.Errorf("run() = %d, want %d", got, test.code)
			}

			_, usage, hasOptions := strings.Cut(stdout.String(), "Global options:\n")

			options, _, hasCommands := strings.Cut(usage, "\nAvailable commands:")
			if !hasOptions || !hasCommands {
				t.Fatalf("missing usage sections on stdout: %q", stdout.String())
			}

			for _, flag := range []string{"-log-level string", "-threads int", "-v\t", "-version"} {
				if !strings.Contains(options, flag) {
					t.Errorf("stdout global options %q are missing %q", options, flag)
				}
			}

			if got := stderr.String(); got != test.diagnostic {
				t.Errorf("stderr = %q, want %q", got, test.diagnostic)
			}
		})
	}
}
