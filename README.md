# git-tools

`git-tools` provides Git repository management commands. It requires Git; `pull`
and `cleanup` use an interactive terminal.

Build from a checkout with `go build -o git-tools .`, then run:

```text
git-tools [global options] <command> [arguments]
```

Global options precede the command: `-v` enables debug logging, `--log-level`
sets the log level (`debug`, `info`, `warn`, or `error`; default `info`),
`--threads` controls concurrent pulls (default: number of CPUs), and `--version`
prints version information.

- `pull [directory]` discovers repositories recursively and starts pulls in a
  terminal UI. The default directory is `$HOME/repos`. Repositories with local
  changes, untracked files, or a non-default branch are skipped; ignored files
  alone do not cause a skip. Press `q` to quit and cancel pending work.
- `clone <repository-url> [custom-name]` clones into
  `$HOME/repos/<hostname>/<namespace>/<repo>`. The namespace may contain multiple
  segments or be absent. HTTPS, `git@host:path`, and `ssh://git@host/path` URLs
  are supported.
- `repopath <repository-url> [custom-name]` prints the path that `clone` would use.
- `cleanup [directory]` opens an interactive view for removing local worktrees
  and branches. Without a directory, it uses the current repository when inside
  one; otherwise it scans `$HOME/repos`. It previews changes against the cached
  `origin/HEAD` and does not fetch automatically.
- `version` prints version information; `help [command]` shows usage and UI
  controls. Command-specific help is also available with `--help`.

Run `make lint` and `make test` for local checks. Release instructions are in
[docs/publishing.md](docs/publishing.md).
