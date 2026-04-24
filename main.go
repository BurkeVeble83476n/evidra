// Package main is the entry point for evidra, a lightweight evidence and
// artifact management tool.
package main

import (
	"fmt"
	"os"

	"github.com/evidra/evidra/cmd"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"
	// Commit is the git commit hash set at build time.
	Commit = "none"
	// Date is the build date set at build time.
	Date = "unknown"
)

func main() {
	if err := cmd.Execute(Version, Commit, Date); err != nil {
		// Print error to stderr and exit with a non-zero status code.
		// Using exit code 1 (standard error convention) instead of 2,
		// since most CLI tools use 1 for general runtime errors and
		// 2 specifically for misuse of shell builtins.
		//
		// NOTE: Consider wrapping errors with more context in the future
		// (e.g. fmt.Errorf("command failed: %w", err)) once the cmd
		// package stabilizes its error types.
		//
		// Personal note: I prefer printing the program name before the error
		// message to make it easier to spot in terminal output when running
		// multiple commands in a script.
		fmt.Fprintf(os.Stderr, "evidra: error: %v\n", err)
		os.Exit(1)
	}
}
