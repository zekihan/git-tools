package main

import (
	"fmt"
	"os"
	"testing"
)

func Test_getWorktreePath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get user home directory: %v", err)
	}

	tests := []struct {
		err     error
		name    string
		repoURL string
		branch  string
		want    string
	}{
		{
			name:    "https with user",
			repoURL: "https://git.com/user/repo.git",
			branch:  "main",
			want:    homeDir + "/worktrees/git.com/user/repo/main",
		},
		{
			name:    "https no user",
			repoURL: "https://git.com/repo.git",
			branch:  "develop",
			want:    homeDir + "/worktrees/git.com/repo/develop",
		},
		{
			name:    "ssh with user and slash branch",
			repoURL: "git@git.com:user/repo.git",
			branch:  "feature/test",
			want:    homeDir + "/worktrees/git.com/user/repo/feature/test",
		},
		{
			name:    "missing branch",
			repoURL: "https://git.com/user/repo.git",
			branch:  "",
			err:     fmt.Errorf("%w", errBranchRequired),
		},
		{
			name:    "unsupported url",
			repoURL: "ftp://git.com/user/repo",
			branch:  "main",
			err: fmt.Errorf(
				"error getting repository path: %w: %s",
				ErrUnsupportedGitURL,
				"ftp://git.com/user/repo",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getWorktreePath(tt.repoURL, tt.branch)
			if tt.err != nil {
				if err == nil {
					t.Fatalf("getWorktreePath() expected error %v, got nil", tt.err)
				}

				if err.Error() != tt.err.Error() {
					t.Fatalf("getWorktreePath() error = %v, wantErr %v", err, tt.err)
				}

				return
			}

			if err != nil {
				t.Fatalf("getWorktreePath() error = %v, want nil", err)
			}

			if got != tt.want {
				t.Fatalf("getWorktreePath() got = %v, want %v", got, tt.want)
			}
		})
	}
}
