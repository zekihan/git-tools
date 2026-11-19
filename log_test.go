package main

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestRunInvalidLogLevelUsesCurrentStderr(t *testing.T) {
	previous := slog.Default()

	t.Cleanup(func() { slog.SetDefault(previous) })

	var previousStderr, stderr bytes.Buffer

	code := run([]string{"--log-level=warn", "help"}, io.Discard, &previousStderr)
	if code != 0 {
		t.Fatalf("initial invocation exited with %d", code)
	}

	code = run([]string{"--log-level=invalid", "help"}, io.Discard, &stderr)
	if code != 0 {
		t.Fatalf("invocation with an invalid level exited with %d", code)
	}

	if previousStderr.Len() != 0 {
		t.Errorf("warning leaked into the previous invocation's stderr: %q", &previousStderr)
	}

	if !strings.Contains(stderr.String(), "Invalid log level, using info") {
		t.Errorf("current stderr is missing the warning: %q", &stderr)
	}
}
