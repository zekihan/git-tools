//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoTestScriptUsesInvokedCheckout(t *testing.T) {
	scriptPath := filepath.Join("scripts", "go_test.sh")

	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		cdpath string
		mode   os.FileMode
	}{
		{name: "CDPATH", cdpath: ".", mode: 0o700},
		{name: "readable script without execute permission", mode: 0o600},
	} {
		t.Run(test.name, func(t *testing.T) {
			checkout := t.TempDir()

			err := os.Mkdir(filepath.Join(checkout, "scripts"), 0o700)
			if err != nil {
				t.Fatal(err)
			}

			//nolint:gosec // Exercise shell invocation with and without an executable script.
			err = os.WriteFile(filepath.Join(checkout, scriptPath), script, test.mode)
			if err != nil {
				t.Fatal(err)
			}

			for name, data := range map[string][]byte{
				"go.mod":  []byte("module example.invalid/test-fixture\n\ngo 1.27.3\n"),
				"main.go": []byte("package main\n\nfunc main() {}\n"),
			} {
				err = os.WriteFile(filepath.Join(checkout, name), data, 0o600)
				if err != nil {
					t.Fatal(err)
				}
			}

			command := exec.CommandContext(t.Context(), "/bin/sh", scriptPath, "./...")
			command.Dir = checkout
			command.Env = append(command.Environ(), "CDPATH="+test.cdpath, "GOWORK=off")

			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("go test wrapper failed: %v\n%s", err, output)
			}

			if !strings.Contains(string(output), "example.invalid/test-fixture") {
				t.Fatalf("go test wrapper did not test the fixture: %s", output)
			}
		})
	}
}
