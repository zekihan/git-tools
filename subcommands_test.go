package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

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
