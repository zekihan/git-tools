package main

import (
	"flag"
	"fmt"
)

func printGlobalHelp() {
	fmt.Println("Usage: git-tools [global options] <command> [command options] [arguments]")
	fmt.Println("\nGlobal options:")
	flag.PrintDefaults() // Prints global flags
	fmt.Println("\nAvailable commands:")
	fmt.Println("  pull           Recursively pulls git repositories in a directory.")
	fmt.Println("  clone          Clones a git repository into a structured directory.")
	fmt.Println("  worktree       Adds a git worktree into a structured directory.")
	fmt.Println("  worktreepath   Calculates and prints the local path for a git worktree.")
	fmt.Println("  repopath       Calculates and prints the local path for a git repository.")
	fmt.Println("  worktreeprune  Prunes stale git worktrees for all repos under a directory.")
	fmt.Println(
		"  cleanup        Interactively removes worktrees and local branches with a diff preview.",
	)
	fmt.Println("  version        Print version information.")
	fmt.Println(
		"  help           Show this help message or help for a specific command (e.g., git-tools help pull).",
	)
	fmt.Println("\nUse \"git-tools <command> --help\" for more information about a command.")
}

func printPullHelp() {
	fmt.Printf("Usage: git-tools pull [directory]\n\n")
	fmt.Printf("Recursively pulls git repositories in a directory.\n")
	fmt.Printf("If no directory is specified, it defaults to $HOME/repos.\n\n")
}

func printCloneHelp() {
	fmt.Printf("Usage: git-tools clone <repository-url> [custom-name]\n\n")
	fmt.Printf("Clones a git repository into a structured directory based on the URL.\n")
	fmt.Printf(
		"The default location is $HOME/repos/<hostname>/<user>/<repo> or $HOME/repos/<hostname>/<repo>.\n",
	)
	fmt.Printf(
		"If [custom-name] is provided, it will be used as the final directory name for the repository.\n\n",
	)
	fmt.Printf("Arguments:\n")
	fmt.Printf("  <repository-url>   The URL of the git repository to clone.\n")
	fmt.Printf("  [custom-name]      Optional. A custom name for the repository directory.\n")
}

//nolint:lll // just strings
func printWorktreeHelp() {
	fmt.Printf("Usage: git-tools worktree [--repo-url <repository-url>] <branch>\n\n")
	fmt.Printf("Adds a git worktree using a structured destination path.\n")
	fmt.Printf(
		"The default location is $HOME/worktrees/<hostname>/<user>/<repo>/<branch> or $HOME/worktrees/<hostname>/<repo>/<branch>.\n\n",
	)
	fmt.Printf("Behavior:\n")
	fmt.Printf("  - Inside a git repository: infers repository from current directory.\n")
	fmt.Printf(
		"  - Outside a git repository: provide --repo-url to clone (if needed) before adding worktree.\n\n",
	)
	fmt.Printf("Options:\n")
	fmt.Printf("  --repo-url <repository-url>  Optional. Explicit repository URL.\n\n")
	fmt.Printf("Arguments:\n")
	fmt.Printf("  <branch>           Branch or commit-ish to check out in the new worktree.\n")
}

//nolint:lll // just strings
func printWorktreepathHelp() {
	fmt.Printf("Usage: git-tools worktreepath [--repo-url <repository-url>] <branch>\n\n")
	fmt.Printf("Calculates and prints the local path for a git worktree.\n")
	fmt.Printf(
		"The default location is $HOME/worktrees/<hostname>/<user>/<repo>/<branch> or $HOME/worktrees/<hostname>/<repo>/<branch>.\n\n",
	)
	fmt.Printf("Behavior:\n")
	fmt.Printf("  - Inside a git repository: infers repository from current directory.\n")
	fmt.Printf("  - Outside a git repository: provide --repo-url.\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  --repo-url <repository-url>  Optional. Explicit repository URL.\n\n")
	fmt.Printf("Arguments:\n")
	fmt.Printf("  <branch>           Branch or commit-ish used in the calculated worktree path.\n")
}

func printWorktreepruneHelp() {
	fmt.Printf("Usage: git-tools worktreeprune [directory]\n\n")
	fmt.Printf("Prunes stale git worktrees for all repos under a directory.\n")
	fmt.Printf("If no directory is specified, it defaults to $HOME/repos.\n\n")
	fmt.Printf("For each linked worktree under $HOME/worktrees/:\n")
	fmt.Printf(
		"  - If merged into the remote default branch: removes the worktree and deletes the local branch.\n",
	)
	fmt.Printf("  - If not merged and dirty: warns and skips.\n")
	fmt.Printf("  - If not merged and clean: attempts a rebase onto the remote default branch.\n")
	fmt.Printf("  - If the rebase has conflicts: aborts and warns.\n\n")
	fmt.Printf("Arguments:\n")
	fmt.Printf(
		"  [directory]   Optional. Directory to search for repos. Defaults to $HOME/repos.\n",
	)
}

func printRepopathHelp() {
	fmt.Printf("Usage: git-tools repopath <repository-url> [custom-name]\n\n")
	fmt.Printf("Calculates and prints the local path for a git repository based on its URL.\n")
	fmt.Printf(
		"The default location is $HOME/repos/<hostname>/<user>/<repo> or $HOME/repos/<hostname>/<repo>.\n",
	)
	fmt.Printf(
		"If [custom-name] is provided, it will be used as the final directory name in the path.\n\n",
	)
	fmt.Printf("Arguments:\n")
	fmt.Printf("  <repository-url>   The URL of the git repository.\n")
	fmt.Printf(
		"  [custom-name]      Optional. A custom name for the repository directory part of the path.\n",
	)
}

func printCleanupHelp() {
	fmt.Println("Usage: git-tools cleanup [directory]")
	fmt.Println(
		"\nBrowse worktrees and local branches in the current repository, or recursively under a directory.",
	)
	fmt.Println(
		"Preview committed changes from the merge base with cached origin/HEAD, plus local worktree changes.",
	)
	fmt.Println(
		"No automatic fetch. Configure origin/HEAD with git remote set-head origin -a if missing.",
	)
	fmt.Println("\nj/k or arrows: select; PgUp/PgDown: scroll preview; r: refresh; q: quit.")
	fmt.Println("d: remove a worktree (keep its branch), or delete a branch without a worktree.")
	fmt.Println("b: remove a worktree and delete its branch. Every action asks for confirmation.")
	fmt.Println(
		"Dirty, locked, default, or unmerged items require typing delete at a second confirmation.",
	)
	fmt.Println(
		"Git does not support removing the main worktree.",
	)
	fmt.Println("Only local branches are deleted; remote branches are untouched.")
}
