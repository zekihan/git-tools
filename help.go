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
	fmt.Println("  pull           Interactively pulls git repositories in a directory.")
	fmt.Println("  clone          Clones a git repository into a structured directory.")
	fmt.Println("  repopath       Calculates and prints the local path for a git repository.")
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
	fmt.Printf("Interactively pulls git repositories in a directory.\n")
	fmt.Printf("If no directory is specified, it defaults to $HOME/repos.\n")
	fmt.Println("Pulls start automatically, limited by the global --threads option.")
	fmt.Println("Shows queued, pulling, pulled, skipped, and errored repositories.")
	fmt.Println("Changed repositories and non-default branches are skipped.")
	fmt.Println(
		"Untracked and ignored files also cause a repository to be skipped to protect local data.",
	)
	fmt.Println(
		"j/k or arrows: select; PgUp/PgDown: scroll Git output; q: quit and cancel pending work.",
	)
	fmt.Println("h: toggle hiding successfully pulled repositories; totals include hidden results.")
	fmt.Println(
		"s: open your shell in the selected repository after its pull finishes; exit returns here.",
	)
	fmt.Println("r: retry only the selected repository once current pulls finish.")
	fmt.Println("Results stay open until you quit. Requires an interactive terminal.")
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
		"\nInside a repository: browse that repository. Outside: scan $HOME/repos recursively.",
	)
	fmt.Println("An explicit directory overrides the default scope.")
	fmt.Println("Directory scans show repositories with linked worktrees or extra local branches.")
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
