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

func TestCloneCredentialPaths(t *testing.T) {
	for _, repoURL := range []string{
		"https://example-user:example-token@github.com/team/repo.git",
		"https://example-token@github.com/team/repo.git",
		"https://example-user:encoded%40token@github.com/team/repo.git",
	} {
		got, err := getRepoSubPath(repoURL, "")
		if err != nil || got != filepath.Join("github.com", "team", "repo") {
			t.Fatalf("credential-free path = %q, error = %v", got, err)
		}
	}
}

func TestCloneRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{
		"../../../outside", "../other", "/absolute", "nested/name", `nested\name`,
		".", "..", ".git", "...git", "bad\x00name",
	} {
		_, err := getRepoPath("https://github.com/team/repo", name)
		if !errors.Is(err, errInvalidRepoName) {
			t.Errorf("name %q: expected invalid-name error, got %v", name, err)
		}
	}

	for _, repoURL := range []string{
		"https://github.com/../repo", "https://github.com/team/..", "git@github.com:../repo",
	} {
		_, err := getRepoPath(repoURL, "")
		if !errors.Is(err, errInvalidRepoName) {
			t.Errorf("URL %q accepted: %v", repoURL, err)
		}
	}
}

func TestCloneRedactsErrorsAndLogs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := t.TempDir()
	//nolint:gosec // Executable fixture replaces Git without contacting a remote.
	err := os.WriteFile(
		filepath.Join(bin, "git"),
		[]byte("#!/bin/sh\nprintf '%s\\n' \"$@\" 'decoded secret-token@example' >&2\nexit 1\n"),
		0o700,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(
		slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	)
	t.Cleanup(func() { slog.SetDefault(previous) })

	//nolint:gosec // Deliberately fake credentials exercise redaction.
	repoURL := "https://private-user:secret-token%40example@github.com/team/repo.git"

	err = handleCloneCmd(t.Context(), []string{repoURL})
	if err == nil {
		t.Fatal("expected clone failure")
	}

	for _, secret := range []string{"private-user", "secret-token", "secret-token%40example"} {
		if strings.Contains(err.Error(), secret) || strings.Contains(logs.String(), secret) {
			t.Fatalf("credential leaked: %s", secret)
		}
	}

	if !strings.Contains(logs.String(), "[redacted]") {
		t.Fatal("missing redacted log")
	}

	_, err = os.Stat(filepath.Join(home, "repos", "github.com", "team", "repo"))
	if !os.IsNotExist(err) {
		t.Fatalf("failed clone directory remains: %v", err)
	}
}

func TestUnsupportedURLRedactsCredentials(t *testing.T) {
	for _, repoURL := range []string{
		"ftp://private-user:secret-token@host/team/repo", "https://private-user:bad%zz@host/team/repo",
	} {
		_, err := getRepoPath(repoURL, "")
		if err == nil || strings.Contains(err.Error(), "private-user") ||
			strings.Contains(err.Error(), "secret-token") ||
			strings.Contains(err.Error(), "bad%zz") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
}
