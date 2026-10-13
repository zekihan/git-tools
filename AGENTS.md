# AGENTS.md

This file provides guidance to AI Agents when working with code in this repository.

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

**Command dispatch:** `main.go` parses global flags (`-v`, `--log-level`, `--threads`) then dispatches based on the first positional argument. Non-interactive handlers live in `subcommands.go`; interactive handlers live in `pull_tui.go` and `cleanup_tui.go`.

**Commands:**
- `pull [dir]` — recursively finds all git repos under a directory (default `$HOME/repos`) and pulls each concurrently in a TUI, controlled by `--threads`
- `clone <url> [name]` — clones a repo into a structured path under `$HOME/repos/<host>/<user>/<repo>`
- `repopath <url> [name]` — prints the calculated clone path without cloning
- `cleanup [dir]` — interactively removes local worktrees and branches with diff previews and confirmations

**Path convention:** Repositories follow a host/user/repo hierarchy derived from the git URL. `clone.go` implements URL parsing (HTTPS and SSH/git@ formats) and path derivation.

**Pull architecture:** `pull.go` performs Git operations and returns structured results with bounded output. `pull_session.go` owns discovery and a worker pool limited by `--threads` (defaults to `runtime.NumCPU()`). `pull_tui.go` handles input and status messages; `pull_view.go` renders the repository list and selected details. Repos not on their default branch or with local changes are skipped without failing. Quitting cancels pending work and waits for workers to stop.

**Shared terminal UI:** `tui.go` provides colors, panels, clipping, and text sanitization for pull and cleanup. Clone and repopath remain non-interactive.

**Logging:** Uses `log/slog` with the `tint` handler (colored terminal output). Initialized in `log.go`; structured fields used throughout for context-rich log lines.
