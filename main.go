package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
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

	ctx := context.Background()

	switch args[0] {
	case "pull":
		err := handlePullCmd(ctx, args[1:])
		if err != nil {
			os.Exit(1)
		}
	case "clone":
		err := handleCloneCmd(ctx, args[1:])
		if err != nil {
			os.Exit(1)
		}
	case "repopath":
		err := handleRepopathCmd(args[1:])
		if err != nil {
			os.Exit(1)
		}
	case "help":
		// Allow "help <command>" for specific help.
		if len(args) <= 1 {
			printGlobalHelp()
			os.Exit(0)
		}

		switch args[1] {
		case "pull":
			printPullHelp(
				flag.NewFlagSet("pull", flag.ExitOnError),
			) // Pass a dummy flagset for consistency
		case "clone":
			printCloneHelp(
				flag.NewFlagSet("clone", flag.ExitOnError),
			) // Pass a dummy flagset for consistency
		case "repopath":
			printRepopathHelp(
				flag.NewFlagSet("repopath", flag.ExitOnError),
			) // Pass a dummy flagset for consistency
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
