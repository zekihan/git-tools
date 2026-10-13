package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var errCleanupUnsafe = errors.New("cleanup refused")

const (
	cleanupWorktreeKind = "worktree"
)

type cleanupEntry struct {
	repo, path, branch, head, base, blocked string
}

func cleanupGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	raw, err := cmd.CombinedOutput()

	output := strings.TrimSpace(string(raw))
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, output)
	}

	return output, nil
}

func cleanupDefaultDir(ctx context.Context, cwd string) (string, error) {
	root, err := cleanupGit(ctx, cwd, "rev-parse", "--show-toplevel")
	if err == nil {
		return root, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, "repos"), nil
}

func cleanupRepos(ctx context.Context, dir string) ([]string, bool, error) {
	root, err := cleanupGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err == nil {
		return []string{root}, true, nil
	}

	repos, err := findGitRepos(ctx, dir)

	return repos, false, err
}

func cleanupRepoHasCandidates(ctx context.Context, repo string) (bool, error) {
	output, err := cleanupGit(ctx, repo, cleanupWorktreeKind, "list", "--porcelain", "-z")
	if err != nil {
		return false, err
	}

	worktrees := parseCleanupWorktrees(output, repo, "")
	if len(worktrees) > 1 {
		return true, nil
	}

	output, err = cleanupGit(ctx, repo, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return false, err
	}

	for branch := range strings.SplitSeq(output, "\n") {
		if branch != "" && (len(worktrees) == 0 || branch != worktrees[0].branch) {
			return true, nil
		}
	}

	return false, nil
}

func loadCleanupEntries(ctx context.Context, dir string) ([]cleanupEntry, error) {
	repos, local, err := cleanupRepos(ctx, dir)
	if err != nil {
		return nil, err
	}

	var entries []cleanupEntry

	for _, repo := range repos {
		if !local {
			hasCandidates, candidateErr := cleanupRepoHasCandidates(ctx, repo)
			if candidateErr != nil {
				entries = append(entries, cleanupEntry{repo: repo, blocked: candidateErr.Error()})

				continue
			}

			if !hasCandidates {
				continue
			}
		}

		found, err := cleanupEntriesForRepo(ctx, repo)
		if err != nil {
			entries = append(entries, cleanupEntry{repo: repo, blocked: err.Error()})

			continue
		}

		for _, entry := range found {
			if !local && entry.blocked == "main worktree" {
				continue
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func cleanupEntriesForRepo(ctx context.Context, repo string) ([]cleanupEntry, error) {
	base, err := cleanupGit(ctx, repo, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return nil, fmt.Errorf(
			"origin default branch unavailable; fetch origin and run git remote set-head origin -a: %w",
			err,
		)
	}

	if !strings.HasPrefix(base, "refs/remotes/origin/") {
		return nil, fmt.Errorf("%w: invalid origin/HEAD target", errCleanupUnsafe)
	}

	_, err = cleanupGit(ctx, repo, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return nil, err
	}

	output, err := cleanupGit(ctx, repo, cleanupWorktreeKind, "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	entries := parseCleanupWorktrees(output, repo, base)
	if len(entries) > 0 {
		repo = entries[0].path
		for index := range entries {
			entries[index].repo = repo
		}
	}

	occupied := make(map[string]bool)
	for _, entry := range entries {
		occupied[entry.branch] = true
	}

	output, err = cleanupGit(
		ctx,
		repo,
		"for-each-ref",
		"--format=%(refname:short) %(objectname)",
		"refs/heads/",
	)
	if err != nil {
		return nil, err
	}

	defaultBranch := strings.TrimPrefix(base, "refs/remotes/origin/")

	for line := range strings.SplitSeq(output, "\n") {
		branch, head, ok := strings.Cut(line, " ")
		if !ok || occupied[branch] {
			continue
		}

		entry := cleanupEntry{repo: repo, branch: branch, head: head, base: base}
		if branch == defaultBranch {
			entry.blocked = "default branch"
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func parseCleanupWorktrees(output, repo, base string) []cleanupEntry {
	var entries []cleanupEntry

	entry := cleanupEntry{repo: repo, base: base}

	for field := range strings.SplitSeq(output, "\x00") {
		switch {
		case field == "":
			if entry.path == "" {
				continue
			}

			if len(entries) == 0 {
				entry.blocked = "main worktree"
			}

			entries = append(entries, entry)
			entry = cleanupEntry{repo: repo, base: base}
		case strings.HasPrefix(field, "worktree "):
			entry.path = strings.TrimPrefix(field, "worktree ")
		case strings.HasPrefix(field, "HEAD "):
			entry.head = strings.TrimPrefix(field, "HEAD ")
		case strings.HasPrefix(field, "branch refs/heads/"):
			entry.branch = strings.TrimPrefix(field, "branch refs/heads/")
		case strings.HasPrefix(field, "locked"):
			entry.blocked = "locked worktree"
		case strings.HasPrefix(field, "prunable"):
			entry.blocked = "missing worktree; use git worktree prune"
		}
	}

	return entries
}

func cleanupDiff(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var output commandBuffer

	cmd.Stdout = &output

	cmd.Stderr = &output

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("diff unavailable: %w: %s", err, output.String())
	}

	result := output.String()
	if output.Len() == commandOutputLimit {
		result += "\n[preview truncated]"
	}

	return result, nil
}

//nolint:gocritic // Keep an immutable entry snapshot for asynchronous operations.
func cleanupPreview(ctx context.Context, entry cleanupEntry) string {
	if entry.head == "" {
		return entry.blocked
	}

	counts, err := cleanupGit(
		ctx,
		entry.repo,
		"rev-list",
		"--left-right",
		"--count",
		entry.base+"..."+entry.head,
	)
	if err != nil {
		return err.Error()
	}

	diff, err := cleanupDiff(
		ctx,
		entry.repo,
		"diff",
		"--no-ext-diff",
		"--no-textconv",
		"--color=never",
		"--stat",
		"--patch",
		"--unified=2",
		entry.base+"..."+entry.head,
		"--",
	)
	if err != nil {
		return err.Error()
	}

	if diff == "" {
		diff = "No committed changes."
	}

	preview := fmt.Sprintf(
		"Against %s (cached; no fetch)\nBehind / ahead: %s\n\n%s",
		strings.TrimPrefix(entry.base, "refs/remotes/"),
		counts,
		diff,
	)
	if entry.path != "" {
		status, statusErr := cleanupGit(
			ctx,
			entry.path,
			"status",
			"--short",
			"--untracked-files=all",
		)
		if statusErr != nil {
			return preview + "\n\n" + statusErr.Error()
		}

		if status != "" {
			local, localErr := cleanupDiff(
				ctx,
				entry.path,
				"diff",
				"--no-ext-diff",
				"--no-textconv",
				"--color=never",
				"--stat",
				"--patch",
				"--unified=2",
				"HEAD",
				"--",
			)
			if localErr != nil {
				local = localErr.Error()
			}

			preview += "\n\nLocal changes (including untracked):\n" + status + "\n" + local
		}
	}

	return preview
}

//nolint:gocritic // Keep an immutable entry snapshot for asynchronous operations.
func cleanupDelete(ctx context.Context, expected cleanupEntry, both, force bool) error {
	entries, err := cleanupEntriesForRepo(ctx, expected.repo)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.path != expected.path || entry.branch != expected.branch {
			continue
		}

		if entry.head != expected.head {
			return fmt.Errorf("%w: HEAD changed; refresh and review again", errCleanupUnsafe)
		}

		return cleanupDeleteEntry(ctx, entry, both, force)
	}

	return fmt.Errorf("%w: entry changed; refresh and review again", errCleanupUnsafe)
}

//nolint:gocritic // Keep an immutable entry snapshot.
func cleanupDeleteEntry(ctx context.Context, entry cleanupEntry, both, force bool) error {
	risks, err := cleanupRisks(ctx, entry, both)
	if err != nil {
		return err
	}

	if len(risks) > 0 && !force {
		return fmt.Errorf(
			"%w: %s; confirm forced removal",
			errCleanupUnsafe,
			strings.Join(risks, "; "),
		)
	}

	deleteBranch := entry.branch != "" && (entry.path == "" || both)

	if entry.path != "" {
		err = cleanupRemoveWorktree(ctx, entry, force)
		if err != nil {
			return err
		}
	}

	if deleteBranch {
		// Ancestry was checked against origin above; Git -d instead checks HEAD or upstream.
		_, err = cleanupGit(ctx, entry.repo, "branch", "-D", "--", entry.branch)
		if err != nil {
			return fmt.Errorf(
				"branch retained (worktree removal, if requested, succeeded): %w",
				err,
			)
		}
	}

	return nil
}

//nolint:gocritic,gocyclo // Keep all destructive risk checks together on an immutable snapshot.
func cleanupRisks(ctx context.Context, entry cleanupEntry, both bool) ([]string, error) {
	var risks []string

	switch entry.blocked {
	case "main worktree":
		return nil, fmt.Errorf("%w: %s", errCleanupUnsafe, entry.blocked)
	case "":
	default:
		risks = append(risks, entry.blocked)
	}

	if entry.branch == strings.TrimPrefix(entry.base, "refs/remotes/origin/") && entry.path != "" &&
		both {
		risks = append(risks, "default branch")
	}

	if entry.path == "" || both || entry.branch == "" {
		_, err := cleanupGit(ctx, entry.repo, "merge-base", "--is-ancestor", entry.head, entry.base)
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				return nil, err
			}

			risks = append(risks, "commits not merged into origin's default branch may be lost")
		}
	}

	if entry.path != "" && !strings.HasPrefix(entry.blocked, "missing worktree") {
		status, err := cleanupGit(
			ctx,
			entry.path,
			"status",
			"--porcelain",
			"--untracked-files=all",
			"--ignored",
		)
		if err != nil {
			return nil, err
		}

		if status != "" {
			risks = append(
				risks,
				"tracked, untracked, and ignored local changes will be permanently discarded",
			)
		}
	}

	return risks, nil
}

//nolint:gocritic // Keep an immutable entry snapshot.
func cleanupRemoveWorktree(ctx context.Context, entry cleanupEntry, force bool) error {
	args := []string{cleanupWorktreeKind, "remove"}
	if force {
		args = append(args, "--force", "--force")
	}

	args = append(args, "--", entry.path)

	_, err := cleanupGit(ctx, entry.repo, args...)
	if err != nil {
		return err
	}

	return nil
}
