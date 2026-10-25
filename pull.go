package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const defaultRemoteName = "origin"

type pullStatus string

const (
	pullQueued    pullStatus = "queued"
	pullRunning   pullStatus = "pulling"
	pullSucceeded pullStatus = "pulled"
	pullSkipped   pullStatus = "skipped"
	pullFailed    pullStatus = "errored"
	pullCanceled  pullStatus = "canceled"
)

var (
	ErrNoRemoteFound    = errors.New("no remote found")
	ErrNoUpstreamBranch = errors.New("no upstream branch found")
	ErrSomeReposFailed  = errors.New("one or more repositories failed to pull")
	errInvalidThreads   = errors.New("threads must be greater than zero")
	errPullTerminal     = errors.New("pull requires an interactive terminal")
	errPullArguments    = errors.New("pull expects at most one directory")
)

type pullResult struct {
	status  pullStatus
	branch  string
	remote  string
	detail  string
	elapsed time.Duration
}

// pullRepo captures its own transcript so concurrent Git commands never write over the UI.
func pullRepo(ctx context.Context, dir string) pullResult {
	started := time.Now()

	var transcript commandBuffer

	result := pullResult{status: pullFailed}

	run := func(args ...string) (string, error) {
		_, _ = fmt.Fprintf(&transcript, "$ git %s\n", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir

		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
		cmd.WaitDelay = time.Second

		var output commandBuffer

		cmd.Stdout, cmd.Stderr = &output, &output
		err := cmd.Run()

		text := strings.TrimSpace(output.String())
		if text != "" {
			_, _ = fmt.Fprintln(&transcript, text)
		}

		if err != nil {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}

		return text, nil
	}
	result.detail = pullRepository(&result, run)

	result.elapsed = time.Since(started)
	if transcript.Len() == commandOutputLimit {
		result.detail = "[output truncated]\n" + result.detail
	}

	result.detail = transcript.String() + result.detail
	if ctx.Err() != nil {
		result.status = pullCanceled
	}

	return result
}

func pullRepository(result *pullResult, run func(...string) (string, error)) string {
	output, err := run("remote")
	if err != nil {
		return err.Error()
	}

	remotes := strings.Fields(output)
	if len(remotes) == 0 {
		return ErrNoRemoteFound.Error()
	}

	result.remote = remotes[0]
	if slices.Contains(remotes, defaultRemoteName) {
		result.remote = defaultRemoteName
	}

	result.branch, err = run("symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			result.status = pullSkipped

			return "Skipped: repository has a detached HEAD."
		}

		return err.Error()
	}

	result.branch = strings.TrimPrefix(result.branch, "refs/heads/")
	headRef := "refs/remotes/" + result.remote + "/HEAD"

	branch, err := run("symbolic-ref", headRef)
	if err != nil {
		_, err = run("remote", "set-head", result.remote, "-a")
		if err != nil {
			return err.Error()
		}

		branch, err = run("symbolic-ref", headRef)
		if err != nil {
			return err.Error()
		}
	}

	branch = strings.TrimPrefix(branch, "refs/remotes/"+result.remote+"/")
	if branch == "" {
		return ErrNoUpstreamBranch.Error()
	}

	if result.branch != branch {
		result.status = pullSkipped

		return fmt.Sprintf(
			"Skipped: branch %s is not the default branch %s.",
			result.branch,
			branch,
		)
	}

	// Pull can overwrite ignored paths, so include them in the safety check.
	// Fail closed on all local files that are not represented by the index.
	output, err = run("status", "--porcelain", "--untracked-files=all", "--ignored=matching")
	if err != nil {
		return err.Error()
	}

	if output != "" {
		result.status = pullSkipped

		return "Skipped: repository has local changes or untracked/ignored files."
	}

	return pullDefaultBranch(result, branch, run)
}

func pullDefaultBranch(
	result *pullResult,
	branch string,
	run func(...string) (string, error),
) string {
	upstream, err := run(
		"for-each-ref",
		"--format=%(upstream:remotename) %(upstream:remoteref)",
		"refs/heads/"+result.branch,
	)
	if err != nil {
		return err.Error()
	}

	if upstream == "" {
		return ErrNoUpstreamBranch.Error()
	}

	if upstream != result.remote+" refs/heads/"+branch {
		result.status = pullSkipped

		return fmt.Sprintf(
			"Skipped: upstream %s does not match default branch %s/%s.",
			upstream,
			result.remote,
			branch,
		)
	}

	_, err = run("pull", "--", result.remote, "refs/heads/"+branch)
	if err != nil {
		return err.Error()
	}

	result.status = pullSucceeded

	return "Pull completed."
}
