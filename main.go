package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	defaultThreadCount = runtime.NumCPU()
	defaultLogLevel    = "info"
)

var (
	verbose  = flag.Bool("v", false, "Enable verbose (DEBUG level) logging (global)")
	logLevel = flag.String(
		"log-level",
		defaultLogLevel,
		"Set log level: debug, info, warn, error (global)",
	)
	threadCount = flag.Int(
		"threads",
		defaultThreadCount,
		"Number of threads to use for git operations (global)",
	)
	showVersion = flag.Bool("version", false, "Print version information and exit (global)")
)

func main() {
	os.Exit(run())
}

//nolint:gocyclo // Main command dispatcher, not complex enough to warrant refactoring.
func run() int {
	flag.Usage = printGlobalHelp // Set global usage

	flag.Parse()

	if *showVersion {
		printVersion()

		return 0
	}

	if *verbose {
		initLogger("debug")
	} else {
		initLogger(*logLevel)
	}

	args := flag.Args()
	if len(args) < 1 {
		printGlobalHelp()

		return 1
	}

	// Cancel in-flight git operations when interrupted (e.g. Ctrl-C).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "pull":
		err := handlePullCmd(ctx, args[1:])
		if err != nil {
			return 1
		}
	case "clone":
		err := handleCloneCmd(ctx, args[1:])
		if err != nil {
			return 1
		}
	case "worktreepath":
		err := handleWorktreepathCmd(ctx, args[1:])
		if err != nil {
			return 1
		}
	case "repopath":
		err := handleRepopathCmd(args[1:])
		if err != nil {
			return 1
		}
	case "cleanup":
		err := handleCleanupCmd(ctx, args[1:])
		if err != nil {
			return 1
		}
	case "version":
		printVersion()
	case "help":
		// Allow "help <command>" for specific help.
		if len(args) <= 1 {
			printGlobalHelp()

			return 0
		}

		switch args[1] {
		case "cleanup":
			printCleanupHelp()
		case "pull":
			printPullHelp()
		case "clone":
			printCloneHelp()
		case "worktreepath":
			printWorktreepathHelp()
		case "repopath":
			printRepopathHelp()
		default:
			fmt.Printf("Unknown command: %s\n\n", args[1])
			printGlobalHelp()
		}
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		printGlobalHelp()

		return 1
	}

	return 0
}
