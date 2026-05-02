package main

import (
	"fmt"
	"testing"
)

func Test_parseWorktreeArgs(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		args        []string
		wantRepoURL string
		wantBranch  string
		wantErr     error
	}{
		{
			name:       "branch only",
			args:       []string{"main"},
			wantBranch: "main",
		},
		{
			name:        "repo url from flag and branch",
			repoURL:     "https://git.com/user/repo.git",
			args:        []string{"feature/a"},
			wantRepoURL: "https://git.com/user/repo.git",
			wantBranch:  "feature/a",
		},
		{
			name:    "missing branch",
			args:    []string{},
			wantErr: fmt.Errorf("%w", errBranchRequired),
		},
		{
			name:    "repo url without branch",
			repoURL: "https://git.com/user/repo.git",
			args:    []string{},
			wantErr: fmt.Errorf("%w", errBranchRequired),
		},
		{
			name:    "too many args",
			args:    []string{"main", "extra"},
			wantErr: fmt.Errorf("%w", errTooManyWorktreeArgs),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoURL, branch, err := parseWorktreeArgs(tt.repoURL, tt.args)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("parseWorktreeArgs() expected error %v, got nil", tt.wantErr)
				}

				if err.Error() != tt.wantErr.Error() {
					t.Fatalf("parseWorktreeArgs() error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseWorktreeArgs() error = %v, want nil", err)
			}

			if repoURL != tt.wantRepoURL {
				t.Fatalf("parseWorktreeArgs() repoURL = %q, want %q", repoURL, tt.wantRepoURL)
			}

			if branch != tt.wantBranch {
				t.Fatalf("parseWorktreeArgs() branch = %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}

func Test_toWorktreeRepoContextErr(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want error
	}{
		{
			name: "maps not-inside-git-repo",
			in:   ErrNotInsideGitRepo,
			want: ErrRepoURLOrGitRepoContext,
		},
		{
			name: "passes through other errors",
			in:   ErrMissingOriginRemote,
			want: ErrMissingOriginRemote,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toWorktreeRepoContextErr(tt.in)
			if got == nil {
				t.Fatalf("toWorktreeRepoContextErr() returned nil")
			}

			if got.Error() != tt.want.Error() {
				t.Fatalf("toWorktreeRepoContextErr() = %v, want %v", got, tt.want)
			}
		})
	}
}
