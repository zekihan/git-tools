package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var (
	verbose     = flag.Bool("v", false, "Enable verbose (DEBUG level) logging (global)")
	logLevel    = flag.String("log-level", "info", "Set log level: debug, info, warn, error (global)")
	threadCount = flag.Int("threads", 8, "Number of threads to use for git operations (global)")
)

func main() {
	flag.Usage = printGlobalHelp // Set global usage
	flag.Parse()

	if *verbose {
		initLogger("debug")
	} else {
		initLogger(*logLevel)
	}

	args := flag.Args()
	if len(args) < 1 {
		printGlobalHelp()
		os.Exit(1)
	}

	switch args[0] {
	case "pull":
		handlePullCmd(args[1:])
	case "help":
		// Allow "help <command>" for specific help
		if len(args) <= 1 {
			printGlobalHelp()
			os.Exit(0)
		}

		switch args[1] {
		case "pull":
			printPullHelp(flag.NewFlagSet("pull", flag.ExitOnError)) // Pass a dummy flagset for consistency
		default:
			fmt.Printf("Unknown command: %s\n\n", args[1])
			printGlobalHelp()
		}
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		printGlobalHelp()
		os.Exit(1)
	}
}

func printPullHelp(fs *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, "Usage: git-tools pull [directory]\n\n")
	fmt.Fprintf(os.Stderr, "Recursively pulls git repositories in a directory.\n")
	fmt.Fprintf(os.Stderr, "If no directory is specified, it defaults to $HOME/repos.\n\n")
}

func handlePullCmd(args []string) {
	pullCmd := flag.NewFlagSet("pull", flag.ExitOnError)
	pullCmd.Usage = func() { printPullHelp(pullCmd) }
	pullCmd.Parse(args)

	searchDir := "$HOME/repos"
	if pullCmd.NArg() > 0 {
		searchDir = pullCmd.Arg(0)
	}

	searchDir = os.ExpandEnv(searchDir)
	absSearchDir, err := filepath.Abs(searchDir)
	if err != nil {
		slog.Error("Error getting absolute path", slog.String("dir", searchDir), slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Starting git pull script", slog.String("dir", absSearchDir))

	if err := run(absSearchDir, *threadCount); err != nil {
		slog.Error("Error running git pull", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Git pull script completed", slog.String("dir", absSearchDir))
}

func printGlobalHelp() {
	fmt.Println("Usage: git-tools [global options] <command> [command options] [arguments]")
	fmt.Println("\nGlobal options:")
	flag.PrintDefaults() // Prints global flags
	fmt.Println("\nAvailable commands:")
	fmt.Println("  pull   Recursively pulls git repositories in a directory.")
	fmt.Println("  help   Show this help message or help for a specific command (e.g., git-tools help pull).")
	fmt.Println("\nUse \"git-tools <command> --help\" for more information about a command.")
}
