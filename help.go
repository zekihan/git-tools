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
	fmt.Println("  pull   Recursively pulls git repositories in a directory.")
	fmt.Println("  clone  Clones a git repository into a structured directory.")
	fmt.Println("  worktree Adds a git worktree into a structured directory.")
	fmt.Println("  worktreepath Calculates and prints the local path for a git worktree.")
	fmt.Println("  repopath Calculates and prints the local path for a git repository.")
	fmt.Println(
		"  help   Show this help message or help for a specific command (e.g., git-tools help pull).",
	)
	fmt.Println("\nUse \"git-tools <command> --help\" for more information about a command.")
}

func printPullHelp(_ *flag.FlagSet) {
	fmt.Printf("Usage: git-tools pull [directory]\n\n")
	fmt.Printf("Recursively pulls git repositories in a directory.\n")
	fmt.Printf("If no directory is specified, it defaults to $HOME/repos.\n\n")
}

func printCloneHelp(_ *flag.FlagSet) {
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

func printWorktreeHelp(_ *flag.FlagSet) {
	fmt.Printf("Usage: git-tools worktree [--repo-url <repository-url>] <branch>\n\n")
	fmt.Printf("Adds a git worktree using a structured destination path.\n")
	fmt.Printf(
		"The default location is $HOME/worktrees/<hostname>/<user>/<repo>/<branch> or $HOME/worktrees/<hostname>/<repo>/<branch>.\n\n",
	)
	fmt.Printf("Behavior:\n")
	fmt.Printf("  - Inside a git repository: infers repository from current directory.\n")
	fmt.Printf("  - Outside a git repository: provide --repo-url to clone (if needed) before adding worktree.\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  --repo-url <repository-url>  Optional. Explicit repository URL.\n\n")
	fmt.Printf("Arguments:\n")
	fmt.Printf("  <branch>           Branch or commit-ish to check out in the new worktree.\n")
}

func printWorktreepathHelp(_ *flag.FlagSet) {
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

func printRepopathHelp(_ *flag.FlagSet) {
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
