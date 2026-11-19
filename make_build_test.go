//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMakeBuildUsesCheckoutNameWithSpaces(t *testing.T) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make is unavailable")
	}

	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}

	checkout := filepath.Join(t.TempDir(), "checkout with spaces")

	err = os.Mkdir(checkout, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	for name, data := range map[string][]byte{
		"Makefile": makefile,
		"go.mod":   []byte("module example.invalid/build-fixture\n\ngo 1.27.3\n"),
		"main.go":  []byte("package main\n\nfunc main() {}\n"),
	} {
		err = os.WriteFile(filepath.Join(checkout, name), data, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	command := exec.CommandContext(t.Context(), makePath, "build")
	command.Dir = checkout
	command.Env = append(command.Environ(), "MAKEFLAGS=", "GOWORK=off")

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("make build failed: %v\n%s", err, output)
	}

	artifact := filepath.Join(checkout, "tmp", filepath.Base(checkout))

	info, err := os.Stat(artifact)
	if err != nil {
		t.Fatalf("make build did not create %q: %v\n%s", artifact, err, output)
	}

	if !info.Mode().IsRegular() {
		t.Fatalf("build output %q is not a regular file", artifact)
	}
}
