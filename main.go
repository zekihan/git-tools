package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

var (
	defaultThreadCount = runtime.NumCPU()
	defaultLogLevel    = "info"
)

var (
	verbose     = flag.Bool("v", false, "Enable verbose (DEBUG level) logging (global)")
	logLevel    = flag.String("log-level", defaultLogLevel, "Set log level: debug, info, warn, error (global)")
	threadCount = flag.Int("threads", defaultThreadCount, "Number of threads to use for git operations (global)")
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
	case "clone":
		handleCloneCmd(args[1:])
	case "help":
		// Allow "help <command>" for specific help
		if len(args) <= 1 {
			printGlobalHelp()
			os.Exit(0)
		}

		switch args[1] {
		case "pull":
			printPullHelp(flag.NewFlagSet("pull", flag.ExitOnError)) // Pass a dummy flagset for consistency
		case "clone":
			printCloneHelp(flag.NewFlagSet("clone", flag.ExitOnError)) // Pass a dummy flagset for consistency
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

func printPullHelp(_ *flag.FlagSet) {
	fmt.Printf("Usage: git-tools pull [directory]\n\n")
	fmt.Printf("Recursively pulls git repositories in a directory.\n")
	fmt.Printf("If no directory is specified, it defaults to $HOME/repos.\n\n")
}

func handlePullCmd(args []string) {
	pullCmd := flag.NewFlagSet("pull", flag.ExitOnError)
	pullCmd.Usage = func() { printPullHelp(pullCmd) }
	err := pullCmd.Parse(args)
	if err != nil {
		slog.Error("Error parsing pull command", slog.Any("error", err))
		os.Exit(1)
	}

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

	if err := pullAll(absSearchDir, *threadCount); err != nil {
		slog.Error("Error running git pull", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Git pull script completed", slog.String("dir", absSearchDir))
}

func printCloneHelp(_ *flag.FlagSet) {
	fmt.Printf("Usage: git-tools clone <repository-url> [custom-name]\n\n")
	fmt.Printf("Clones a git repository into a structured directory based on the URL.\n")
	fmt.Printf("The default location is $HOME/repos/<hostname>/<user>/<repo> or $HOME/repos/<hostname>/<repo>.\n")
	fmt.Printf("If [custom-name] is provided, it will be used as the final directory name for the repository.\n\n")
	fmt.Printf("Arguments:\n")
	fmt.Printf("  <repository-url>   The URL of the git repository to clone.\n")
	fmt.Printf("  [custom-name]      Optional. A custom name for the repository directory.\n")
}

func handleCloneCmd(args []string) {
	cloneCmd := flag.NewFlagSet("clone", flag.ExitOnError)
	cloneCmd.Usage = func() { printCloneHelp(cloneCmd) }
	err := cloneCmd.Parse(args)
	if err != nil {
		slog.Error("Error parsing clone command", slog.Any("error", err))
		os.Exit(1)
	}

	if cloneCmd.NArg() < 1 {
		slog.Error("Repository URL is required")
		cloneCmd.Usage()
		os.Exit(1)
	}

	repoURL := cloneCmd.Arg(0)
	customName := ""
	if cloneCmd.NArg() > 1 {
		customName = cloneCmd.Arg(1)
	}

	slog.Info("Starting git clone", slog.String("url", repoURL), slog.String("customName", customName))

	clonedPath, err := cloneRepo(repoURL, customName)
	if err != nil {
		slog.Error("Error cloning repository", slog.String("url", repoURL), slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Successfully cloned repository", slog.String("url", repoURL), slog.String("path", clonedPath))
}

func printGlobalHelp() {
	fmt.Println("Usage: git-tools [global options] <command> [command options] [arguments]")
	fmt.Println("\nGlobal options:")
	flag.PrintDefaults() // Prints global flags
	fmt.Println("\nAvailable commands:")
	fmt.Println("  pull   Recursively pulls git repositories in a directory.")
	fmt.Println("  clone  Clones a git repository into a structured directory.")
	fmt.Println("  help   Show this help message or help for a specific command (e.g., git-tools help pull).")
	fmt.Println("\nUse \"git-tools <command> --help\" for more information about a command.")
}
