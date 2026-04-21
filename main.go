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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
