# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
# Format and tidy
make tidy

# Lint (runs tidy first, then golangci-lint)
make lint

# Run tests
make test

# Run a single test
./scripts/go_test.sh -v -run TestName ./...

# Run tests with coverage
make test/cover

# Quality audit (vet, staticcheck, govulncheck)
make audit
```

## Architecture

This is a single-package Go CLI (`package main`) that provides git workflow utilities. The binary name matches the directory name (`git-tools`).

**Command dispatch:** `main.go` parses global flags (`-v`, `--log-level`, `--threads`) then dispatches to handlers in `subcommands.go` based on the first positional argument.

**Commands:**
- `pull [dir]` — recursively finds all git repos under a directory (default `$HOME/repos`) and pulls each concurrently, controlled by `--threads`
- `clone <url> [name]` — clones a repo into a structured path under `$HOME/repos/<host>/<user>/<repo>`
- `worktree [-repo-url <url>] <branch>` — adds a git worktree at `$HOME/worktrees/<host>/<user>/<repo>/<branch>`; auto-clones if the repo doesn't exist locally; can infer repo URL from the current working directory
- `worktreepath [-repo-url <url>] <branch>` — prints the calculated worktree path without creating it
- `repopath <url> [name]` — prints the calculated clone path without cloning

**Path convention:** Both repos and worktrees follow the same host/user/repo hierarchy derived from the git URL. `clone.go` implements URL parsing (HTTPS and SSH/git@ formats) and path derivation. `worktree.go` reuses the same derivation for `$HOME/worktrees/`.

**Concurrency model:** `pull.go` runs `pullRepo` goroutines bounded by a semaphore channel of size `--threads` (defaults to `runtime.NumCPU()`). Repos not on their default branch or with local changes emit warnings and are skipped without failing.

**Logging:** Uses `log/slog` with the `tint` handler (colored terminal output). Initialized in `log.go`; structured fields used throughout for context-rich log lines.
