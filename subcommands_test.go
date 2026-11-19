package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryCommandsPreserveNestedNamespace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	source := cleanupTestRepo(t)

	const repoURL = "https://example.invalid/group/subgroup/tools.repo.git"

	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+filepath.ToSlash(source)+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", repoURL)

	wantPath := filepath.Join(home, "repos", "example.invalid", "group", "subgroup", "tools.repo")
	args := []string{repoURL}

	t.Run("repopath", func(t *testing.T) {
		var output bytes.Buffer

		err := handleRepopathCmdTo(args, &output)
		if err != nil || output.String() != wantPath+"\n" {
			t.Fatalf("repopath = %q, error = %v, want %q", output.String(), err, wantPath+"\n")
		}
	})

	t.Run("clone", func(t *testing.T) {
		err := handleCloneCmd(t.Context(), args)
		if err != nil {
			t.Fatal(err)
		}

		wantHead := cleanupTestGit(t, source, "rev-parse", "HEAD")
		if got := cleanupTestGit(t, wantPath, "rev-parse", "HEAD"); got != wantHead {
			t.Fatalf("cloned HEAD = %s, want %s", got, wantHead)
		}
	})
}

func TestRepositoryCommandsRejectExtraArguments(t *testing.T) {
	for _, command := range []string{"clone", "repopath"} {
		t.Run(command, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			// Rewrite the HTTPS fixture to a disposable local repository. Even the
			// unfixed clone must never contact a service or use a real checkout.
			source := cleanupTestRepo(t)
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", "url.file://"+filepath.ToSlash(source)+".insteadOf")
			t.Setenv("GIT_CONFIG_VALUE_0", "https://example.invalid/team/repo.git")

			for _, args := range [][]string{
				{"https://example.invalid/team/repo.git", "my", "project"},
				{"https://example.invalid/team/repo.git", "my", "project", "extra"},
				{"--", "https://example.invalid/team/repo.git", "my", "project"},
			} {
				output := captureRepositoryCommandOutput(t, func() {
					var err error
					if command == "clone" {
						err = handleCloneCmd(t.Context(), args)
					} else {
						err = handleRepopathCmd(args)
					}

					if !errors.Is(err, errTooManyRepoArgs) {
						t.Errorf("%v: want argument-count error, got %v", args, err)
					}
				})
				if output != "" {
					t.Errorf("unexpected path output for invalid arguments: %q", output)
				}

				_, err := os.Stat(filepath.Join(home, "repos"))
				if !os.IsNotExist(err) {
					t.Errorf("invalid arguments created clone directories: %v", err)
				}
			}
		})
	}
}

func captureRepositoryCommandOutput(t *testing.T, run func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	previous := os.Stdout
	os.Stdout = writer

	t.Cleanup(func() { os.Stdout = previous })

	defer reader.Close()
	defer writer.Close()

	run()

	os.Stdout = previous

	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	return string(output)
}
