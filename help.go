package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func printGlobalHelpTo(out io.Writer, flags *flag.FlagSet) {
	fmt.Fprintln(out, "Usage: git-tools [global options] <command> [command options] [arguments]")
	fmt.Fprintln(out, "\nGlobal options:")
	flags.PrintDefaults() // Prints global flags
	fmt.Fprintln(out, "\nAvailable commands:")
	fmt.Fprintln(out, "  pull           Interactively pulls git repositories in a directory.")
	fmt.Fprintln(out, "  clone          Clones a git repository into a structured directory.")
	fmt.Fprintln(out, "  repopath       Calculates and prints the local path for a git repository.")
	fmt.Fprintln(out,
		"  cleanup        Interactively removes worktrees and local branches with a diff preview.",
	)
	fmt.Fprintln(out, "  version        Print version information.")
	fmt.Fprintln(
		out,
		"  help           Show this help message or help for a specific command (e.g., git-tools help pull).",
	)
	fmt.Fprintln(out, "\nUse \"git-tools <command> --help\" for more information about a command.")
}

func printPullHelp() { printPullHelpTo(os.Stdout) }

func printPullHelpTo(out io.Writer) {
	fmt.Fprintf(out, "Usage: git-tools pull [directory]\n\n")
	fmt.Fprintf(out, "Interactively pulls git repositories in a directory.\n")
	fmt.Fprintf(out, "If no directory is specified, it defaults to $HOME/repos.\n")
	fmt.Fprintln(out, "Pulls start automatically, limited by the global --threads option.")
	fmt.Fprintln(out, "Shows queued, pulling, pulled, skipped, and errored repositories.")
	fmt.Fprintln(out, "Changed repositories and non-default branches are skipped.")
	fmt.Fprintln(out,
		"Untracked and ignored files also cause a repository to be skipped to protect local data.",
	)
	fmt.Fprintln(out,
		"j/k or arrows: select; PgUp/PgDown: scroll Git output; q: quit and cancel pending work.",
	)
	fmt.Fprintln(
		out,
		"h: toggle hiding successfully pulled repositories; totals include hidden results.",
	)
	fmt.Fprintln(out,
		"s: open your shell in the selected repository after its pull finishes; exit returns here.",
	)
	fmt.Fprintln(out, "r: retry only the selected repository once current pulls finish.")
	fmt.Fprintln(out,
		"SSH passphrases, host-key checks, and Git credentials are asked inside the TUI;",
	)
	fmt.Fprintln(out,
		"one answer serves concurrent pulls for the same prompt, and esc cancels them.",
	)
	fmt.Fprintln(out, "Results stay open until you quit. Requires an interactive terminal.")
}

func printCloneHelp() { printCloneHelpTo(os.Stdout) }

func printCloneHelpTo(out io.Writer) {
	fmt.Fprintf(out, "Usage: git-tools clone <repository-url> [custom-name]\n\n")
	fmt.Fprintf(out, "Clones a git repository into a structured directory based on the URL.\n")
	fmt.Fprintf(
		out,
		"The default location is $HOME/repos/<hostname>/<user>/<repo> or $HOME/repos/<hostname>/<repo>.\n",
	)
	fmt.Fprintf(
		out,
		"If [custom-name] is provided, it will be used as the final directory name for the repository.\n\n",
	)
	fmt.Fprintf(out, "Arguments:\n")
	fmt.Fprintf(out, "  <repository-url>   The URL of the git repository to clone.\n")
	fmt.Fprintf(out, "  [custom-name]      Optional. A custom name for the repository directory.\n")
}

func printRepopathHelpTo(out io.Writer) {
	fmt.Fprintf(out, "Usage: git-tools repopath <repository-url> [custom-name]\n\n")
	fmt.Fprintf(
		out,
		"Calculates and prints the local path for a git repository based on its URL.\n",
	)
	fmt.Fprintf(
		out,
		"The default location is $HOME/repos/<hostname>/<user>/<repo> or $HOME/repos/<hostname>/<repo>.\n",
	)
	fmt.Fprintf(
		out,
		"If [custom-name] is provided, it will be used as the final directory name in the path.\n\n",
	)
	fmt.Fprintf(out, "Arguments:\n")
	fmt.Fprintf(out, "  <repository-url>   The URL of the git repository.\n")
	fmt.Fprintf(
		out,
		"  [custom-name]      Optional. A custom name for the repository directory part of the path.\n",
	)
}

func printCleanupHelp() { printCleanupHelpTo(os.Stdout) }

func printCleanupHelpTo(out io.Writer) {
	fmt.Fprintln(out, "Usage: git-tools cleanup [directory]")
	fmt.Fprintln(out,
		"\nInside a repository: browse that repository. Outside: scan $HOME/repos recursively.",
	)
	fmt.Fprintln(out, "An explicit directory overrides the default scope.")
	fmt.Fprintln(
		out,
		"Directory scans show repositories with linked worktrees or extra local branches.",
	)
	fmt.Fprintln(
		out,
		"Scans exclude repositories with Git metadata or worktrees outside the scan root.",
	)
	fmt.Fprintln(out, "Select such a repository directly to review its cleanup candidates.")
	fmt.Fprintln(
		out,
		"Preview committed changes from the merge base with cached origin/HEAD, plus local worktree changes.",
	)
	fmt.Fprintln(out,
		"No automatic fetch. Configure origin/HEAD with git remote set-head origin -a if missing.",
	)
	fmt.Fprintln(
		out,
		"\nj/k or arrows: move; Space: mark; a: toggle all; PgUp/PgDown: scroll; r: refresh; q: quit.",
	)
	fmt.Fprintln(
		out,
		"d: remove a worktree (keep its branch), or delete a branch without a worktree.",
	)
	fmt.Fprintln(out,
		"b: remove worktrees and their branches. d/b act on marked items, or the current item.",
	)
	fmt.Fprintln(out,
		"Dirty, locked, default, unmerged, or external worktrees require a second y confirmation.",
	)
	fmt.Fprintln(out,
		"Git does not support removing the main worktree.",
	)
	fmt.Fprintln(out, "Only local branches are deleted; remote branches are untouched.")
}
