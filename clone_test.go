package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNestedRepoSubPath(t *testing.T) {
	for _, test := range []struct {
		repoURL    string
		customName string
		want       string
	}{
		{
			repoURL: "https://example.invalid:8443/group/libs.git/tools.repo.git",
			want:    filepath.Join("example.invalid:8443", "group", "libs.git", "tools.repo"),
		},
		{
			repoURL:    "ssh://git@example.invalid:2222/group/team/tools.repo.git",
			customName: "local.repo.git",
			want:       filepath.Join("example.invalid:2222", "group", "team", "local.repo"),
		},
		{
			repoURL: "git@example.invalid:group/team/libs/tools.repo",
			want:    filepath.Join("example.invalid", "group", "team", "libs", "tools.repo"),
		},
	} {
		got, err := getRepoSubPath(test.repoURL, test.customName)
		if err != nil || got != test.want {
			t.Errorf(
				"getRepoSubPath(%q, %q) = %q, %v, want %q",
				test.repoURL,
				test.customName,
				got,
				err,
				test.want,
			)
		}
	}
}

func TestRejectInvalidNestedNamespaceComponents(t *testing.T) {
	for _, repoURL := range []string{
		"https://example.invalid/group/../team/repo.git",
		"ssh://git@example.invalid/group/./team/repo.git",
		`git@example.invalid:group/team\libs/repo.git`,
	} {
		got, err := getRepoSubPath(repoURL, "")
		if !errors.Is(err, errInvalidRepoName) || got != "" {
			t.Errorf("getRepoSubPath(%q) = %q, %v, want invalid name", repoURL, got, err)
		}
	}
}

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
		err  error
		args args
		name string
		want string
	}{
		// HTTPS URLS
		{
			name: "https with user and .git",
			args: args{
				repoURL: "https://git.com/user/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com/user/repo",
		},
		{
			name: "https with user no .git",
			args: args{
				repoURL: "https://git.com/user/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com/user/repo",
		},
		{
			name: "https simple with .git",
			args: args{
				repoURL: "https://git.com/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com/repo",
		},
		{
			name: "https simple no .git",
			args: args{
				repoURL: "https://git.com/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com/repo",
		},
		{
			name: "https simple with .git and custom name",
			args: args{
				repoURL: "https://git.com/repo.git",
				name:    "repo_name",
			},
			want: homeDir + "/repos/git.com/repo_name",
		},
		{
			name: "https simple no .git and custom name",
			args: args{
				repoURL: "https://git.com/repo",
				name:    "repo_name",
			},
			want: homeDir + "/repos/git.com/repo_name",
		},
		{
			name: "https with user, .git and port",
			args: args{
				repoURL: "https://git.com:8080/user/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com:8080/user/repo",
		},
		{
			name: "https with user, no .git and port",
			args: args{
				repoURL: "https://git.com:8080/user/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com:8080/user/repo",
		},
		{
			name: "https with dotted repo name and .git",
			args: args{
				repoURL: "https://github.com/socketio/socket.io.git",
				name:    "",
			},
			want: homeDir + "/repos/github.com/socketio/socket.io",
		},
		{
			name: "https with dotted repo name no .git",
			args: args{
				repoURL: "https://github.com/vercel/next.js",
				name:    "",
			},
			want: homeDir + "/repos/github.com/vercel/next.js",
		},
		{
			name: "https repo only with dotted name",
			args: args{
				repoURL: "https://git.com/socket.io",
				name:    "",
			},
			want: homeDir + "/repos/git.com/socket.io",
		},
		// SSH URLS
		{
			name: "ssh with user and .git",
			args: args{
				repoURL: "git@git.com:user/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com/user/repo",
		},
		{
			name: "ssh with user no .git",
			args: args{
				repoURL: "git@git.com:user/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com/user/repo",
		},
		{
			name: "ssh simple with .git",
			args: args{
				repoURL: "ssh://git@git.com/user/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com/user/repo",
		},
		{
			name: "ssh simple with no .git",
			args: args{
				repoURL: "ssh://git@git.com/user/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com/user/repo",
		},
		{
			name: "ssh with user, .git and port",
			args: args{
				repoURL: "git@git.com:8080:user/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com:8080/user/repo",
		},
		{
			name: "ssh with user, no .git and port",
			args: args{
				repoURL: "git@git.com:8080:user/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com:8080/user/repo",
		},
		{
			name: "ssh simple with .git and port",
			args: args{
				repoURL: "ssh://git@git.com:8080/user/repo.git",
				name:    "",
			},
			want: homeDir + "/repos/git.com:8080/user/repo",
		},
		{
			name: "ssh simple with no .git and port",
			args: args{
				repoURL: "ssh://git@git.com:8080/user/repo",
				name:    "",
			},
			want: homeDir + "/repos/git.com:8080/user/repo",
		},
		{
			name: "ssh with dotted repo name",
			args: args{
				repoURL: "git@github.com:socketio/socket.io.git",
				name:    "",
			},
			want: homeDir + "/repos/github.com/socketio/socket.io",
		},
		// ERROR CASES
		{
			err:  fmt.Errorf("%w: %s", ErrUnsupportedGitURL, "ftp://git.com"),
			name: "unsupported URL format",
			args: args{
				repoURL: "ftp://git.com/user/repo",
				name:    "",
			},
			want: "",
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

				if err.Error() != tt.err.Error() {
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
