package main

import (
	"fmt"
	"os"
	"testing"
)

func Test_getRepoPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get user home directory: %v", err)
	}

	type args struct {
		repoURL string
		name    string
	}
	tests := []struct {
		name string
		args args
		want string
		err  error
	}{
		// HTTPS URLS
		{
			name: "https with user and .git",
			args: args{
				repoURL: "https://git.com/user/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "https with user no .git",
			args: args{
				repoURL: "https://git.com/user/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "https simple with .git",
			args: args{
				repoURL: "https://git.com/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/repo", homeDir),
			err:  nil,
		},
		{
			name: "https simple no .git",
			args: args{
				repoURL: "https://git.com/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/repo", homeDir),
			err:  nil,
		},
		{
			name: "https simple with .git and custom name",
			args: args{
				repoURL: "https://git.com/repo.git",
				name:    "repo_name",
			},
			want: fmt.Sprintf("%s/repos/git.com/repo_name", homeDir),
			err:  nil,
		},
		{
			name: "https simple no .git and custom name",
			args: args{
				repoURL: "https://git.com/repo",
				name:    "repo_name",
			},
			want: fmt.Sprintf("%s/repos/git.com/repo_name", homeDir),
			err:  nil,
		},
		{
			name: "https with user, .git and port",
			args: args{
				repoURL: "https://git.com:8080/user/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com:8080/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "https with user, no .git and port",
			args: args{
				repoURL: "https://git.com:8080/user/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com:8080/user/repo", homeDir),
			err:  nil,
		},
		// SSH URLS
		{
			name: "ssh with user and .git",
			args: args{
				repoURL: "git@git.com:user/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh with user no .git",
			args: args{
				repoURL: "git@git.com:user/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh simple with .git",
			args: args{
				repoURL: "ssh://git@git.com/user/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh simple with no .git",
			args: args{
				repoURL: "ssh://git@git.com/user/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh with user, .git and port",
			args: args{
				repoURL: "git@git.com:8080:user/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com:8080/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh with user, no .git and port",
			args: args{
				repoURL: "git@git.com:8080:user/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com:8080/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh simple with .git and port",
			args: args{
				repoURL: "ssh://git@git.com:8080/user/repo.git",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com:8080/user/repo", homeDir),
			err:  nil,
		},
		{
			name: "ssh simple with no .git and port",
			args: args{
				repoURL: "ssh://git@git.com:8080/user/repo",
				name:    "",
			},
			want: fmt.Sprintf("%s/repos/git.com:8080/user/repo", homeDir),
			err:  nil,
		},
		// ERROR CASES
		{
			name: "unsupported URL format",
			args: args{
				repoURL: "ftp://git.com/user/repo",
				name:    "",
			},
			want: "",
			err:  fmt.Errorf("unsupported Git URL format: ftp://git.com/user/repo. Please use HTTPS or SSH"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRepoPath(tt.args.repoURL, tt.args.name)
			if tt.err != nil {
				if err == nil {
					t.Errorf("getRepoPath() expected error %v, got nil", tt.err)
					return
				}
				if !(err.Error() == tt.err.Error()) {
					t.Errorf("getRepoPath() error = %v, wantErr %v", err, tt.err)
				}
				return
			} else if err != nil {
				t.Errorf("getRepoPath() error = %v, want nil", err)
				return
			}
			if got != tt.want {
				t.Errorf("getRepoPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}
