package main

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMalformedRepositoryDiagnostics(t *testing.T) {
	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(
		slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)
	t.Cleanup(func() { slog.SetDefault(previous) })

	for _, repoURL := range []string{
		"https:/fixture-user:fixture-secret@host/team/repo",
		"https:fixture-user:fixture-secret@host/team/repo",
		"https:///fixture-user:fixture-secret@host/team/repo",
		"https:////fixture-user:fixture-secret@host/team/repo",
		"//fixture-user:fixture-secret@host/team/repo",
		"://fixture-user:fixture-secret@host/team/repo",
		"https://fixture-user:fixture%zzsecret@host/team/repo",
		"https://fixture%zzuser:fixture-secret@host/team/repo",
		"https://fixture-user:fixture%40secret@host/team/repo%zz",
		"https://fixture-user:fixture-secret@host:bad/team/repo",
	} {
		t.Run(repoURL, func(t *testing.T) {
			if got := safeGitURL(repoURL); got != "[invalid repository URL]" {
				t.Fatalf("unsafe URL representation: %q", got)
			}

			for _, command := range []func() error{
				func() error { return handleCloneCmd(t.Context(), []string{repoURL}) },
				func() error { return handleRepopathCmd([]string{repoURL}) },
			} {
				logs.Reset()

				err := command()
				if !errors.Is(err, ErrUnsupportedGitURL) {
					t.Fatalf("expected unsupported URL, got %v", err)
				}

				for _, output := range []string{err.Error(), logs.String()} {
					for _, secret := range []string{repoURL, "fixture-user", "fixture-secret", "fixture%"} {
						if strings.Contains(output, secret) {
							t.Fatalf("repository input leaked: %q", output)
						}
					}
				}
			}
		})
	}
}

func TestRepositoryDiagnosticsRetainOnlyConnectionContext(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"https://fixture-user:fixture-secret@host/team/repo", "https://host"},
		{"https://fixture-user:fixture%40secret@host:8443/team/repo", "https://host:8443"},
		{"ssh://git@host:2222/team/repo", "ssh://host:2222"},
		{"git@host:team/repo", "ssh://host"},
		{"https://[::1]:8443/team/repo", "https://[::1]:8443"},
	} {
		if got := safeGitURL(test.input); got != test.want {
			t.Errorf("safeGitURL(%q) = %q, want %q", test.input, got, test.want)
		}
	}

	_, err := getRepoPath("https://fixture-user:fixture-secret@host/team/nested/repo", "")
	if !errors.Is(err, ErrUnsupportedGitURL) || !strings.HasSuffix(err.Error(), ": https://host") {
		t.Fatalf("expected host-only unsupported URL context, got %v", err)
	}
}

func TestCloneBoundsAndRedactsLargeOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}

	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	completed := filepath.Join(t.TempDir(), "completed")
	t.Setenv("CLONE_FIXTURE_COMPLETED", completed)
	//nolint:gosec // Local executable fixture emits synthetic diagnostics; no network calls.
	err := os.WriteFile(filepath.Join(bin, "git"), []byte(`#!/bin/sh
printf '%s\n' "$2" 'fixture-user fixture-secret@example fixture-secret%40example'
i=0
while [ "$i" -lt 32768 ]; do
    printf '%s\n' 'stdout diagnostic padding'
    printf '%s\n' 'stderr diagnostic padding' >&2
    i=$((i + 1))
done
printf 'completed' > "$CLONE_FIXTURE_COMPLETED"
exit 1
`), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = cloneRepo(
		t.Context(),
		"https://fixture-user:fixture-secret%40example@host/team/repo",
		"",
	)
	if err == nil {
		t.Fatal("expected clone failure")
	}

	output := err.Error()
	if len(output) > commandOutputLimit+256 || !strings.Contains(output, "[output truncated]") {
		t.Fatalf("unexpected diagnostic size %d or missing truncation marker", len(output))
	}

	for _, secret := range []string{"fixture-user", "fixture-secret", "fixture-secret%40example"} {
		if strings.Contains(output, secret) {
			t.Fatalf("credential leaked: %s", secret)
		}
	}

	if !strings.Contains(output, "[redacted]") ||
		!strings.Contains(output, "stdout diagnostic padding") ||
		!strings.Contains(output, "stderr diagnostic padding") {
		t.Fatal("missing retained and redacted diagnostics")
	}

	_, statErr := os.Stat(completed)
	if statErr != nil {
		t.Fatalf("child did not finish emitting diagnostics: %v", statErr)
	}
}

func TestCommandBufferTruncationBoundary(t *testing.T) {
	for _, size := range []int{0, commandOutputLimit, commandOutputLimit + 1, commandOutputLimit * 64} {
		var buffer commandBuffer

		written, err := buffer.Write(bytes.Repeat([]byte("x"), size))
		if err != nil || written != size || buffer.Len() != min(size, commandOutputLimit) ||
			buffer.truncated != (size > commandOutputLimit) {
			t.Fatalf("size %d: written=%d retained=%d truncated=%t error=%v",
				size, written, buffer.Len(), buffer.truncated, err)
		}

		written, err = buffer.Write([]byte("tail"))
		if err != nil || written != 4 || buffer.Len() != min(size+4, commandOutputLimit) ||
			buffer.truncated != (size+4 > commandOutputLimit) {
			t.Fatalf("size %d: subsequent write violated buffer bound", size)
		}
	}
}
