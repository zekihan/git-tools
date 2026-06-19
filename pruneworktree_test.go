package main

import (
	"reflect"
	"testing"
)

func Test_parseWorktreeList(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []worktreeEntry
	}{
		{
			// The first stanza is always the main worktree and is skipped.
			name: "main with linked branch and detached worktrees",
			output: "worktree /home/u/repos/git.com/user/repo\n" +
				"HEAD aaaaaaaaaaaa\n" +
				"branch refs/heads/main\n" +
				"\n" +
				"worktree /home/u/worktrees/git.com/user/repo/feature\n" +
				"HEAD bbbbbbbbbbbb\n" +
				"branch refs/heads/feature\n" +
				"\n" +
				"worktree /home/u/worktrees/git.com/user/repo/wip\n" +
				"HEAD cccccccccccc\n" +
				"detached\n",
			want: []worktreeEntry{
				{path: "/home/u/worktrees/git.com/user/repo/feature", branch: "feature"},
				{path: "/home/u/worktrees/git.com/user/repo/wip", branch: ""},
			},
		},
		{
			name: "branch with slashes is preserved",
			output: "worktree /home/u/repos/git.com/user/repo\n" +
				"HEAD aaaaaaaaaaaa\n" +
				"branch refs/heads/main\n" +
				"\n" +
				"worktree /home/u/worktrees/git.com/user/repo/feature/nested\n" +
				"HEAD bbbbbbbbbbbb\n" +
				"branch refs/heads/feature/nested\n",
			want: []worktreeEntry{
				{
					path:   "/home/u/worktrees/git.com/user/repo/feature/nested",
					branch: "feature/nested",
				},
			},
		},
		{
			name: "only the main worktree yields no entries",
			output: "worktree /home/u/repos/git.com/user/repo\n" +
				"HEAD aaaaaaaaaaaa\n" +
				"branch refs/heads/main\n",
			want: []worktreeEntry{},
		},
		{
			name:   "empty output yields no entries",
			output: "",
			want:   []worktreeEntry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorktreeList(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseWorktreeList() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
