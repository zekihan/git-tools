package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	defaultThreadCount = runtime.NumCPU()
	defaultLogLevel    = "info"
)

const flagErrorExitCode = 2

var threadCount = new(int)

func main() {
	if isAskpassInvocation() {
		os.Exit(runAskpass(os.Args[1:], os.Stdout, os.Stderr))
	}

	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

//nolint:gocyclo // Main command dispatcher, not complex enough to warrant refactoring.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	verbose := flags.Bool("v", false, "Enable verbose (DEBUG level) logging (global)")
	logLevel := flags.String(
		"log-level",
		defaultLogLevel,
		"Set log level: debug, info, warn, error (global)",
	)
	flags.IntVar(
		threadCount,
		"threads",
		defaultThreadCount,
		"Number of threads to use for git operations (global)",
	)
	showVersion := flags.Bool("version", false, "Print version information and exit (global)")

	flags.Usage = func() { printGlobalHelpTo(stdout, flags) }

	err := flags.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		return flagErrorExitCode
	}

	if *showVersion {
		printVersionTo(stdout)

		return 0
	}

	if *verbose {
		initLogger("debug", stderr)
	} else {
		initLogger(*logLevel, stderr)
	}

	args = flags.Args()
	if len(args) < 1 {
		printGlobalHelpTo(stdout, flags)

		return 1
	}

	// Cancel in-flight git operations when interrupted (e.g. Ctrl-C).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "pull":
		err := handlePullCmd(ctx, args[1:])
		if err != nil {
			slog.ErrorContext(ctx, "Pull failed", slog.Any("error", err))

			return 1
		}
	case "clone":
		err := handleCloneCmd(ctx, args[1:])
		if err != nil {
			return 1
		}
	case "repopath":
		err := handleRepopathCmdTo(args[1:], stdout)
		if err != nil {
			return 1
		}
	case "cleanup":
		err := handleCleanupCmd(ctx, args[1:])
		if err != nil {
			slog.ErrorContext(ctx, "Cleanup failed", slog.Any("error", err))

			return 1
		}
	case "version":
		printVersionTo(stdout)
	case "help":
		// Allow "help <command>" for specific help.
		if len(args) <= 1 {
			printGlobalHelpTo(stdout, flags)

			return 0
		}

		switch args[1] {
		case "cleanup":
			printCleanupHelpTo(stdout)
		case "pull":
			printPullHelpTo(stdout)
		case "clone":
			printCloneHelpTo(stdout)
		case "repopath":
			printRepopathHelpTo(stdout)
		default:
			fmt.Fprintf(stdout, "Unknown command: %s\n\n", args[1])
			printGlobalHelpTo(stdout, flags)

			return 1
		}
	default:
		fmt.Fprintf(stdout, "Unknown command: %s\n\n", args[0])
		printGlobalHelpTo(stdout, flags)

		return 1
	}

	return 0
}
