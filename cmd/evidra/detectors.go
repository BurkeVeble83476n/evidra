package main

import (
	"flag"
	"fmt"
	"io"
	"sort"

	"samebits.com/evidra/internal/detectors"
	_ "samebits.com/evidra/internal/detectors/all"
)

func cmdDetectors(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evidra detectors list [--stable-only]")
		return 2
	}

	switch args[0] {
	case "list":
		return cmdDetectorsList(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown detectors subcommand: %s\n", args[0])
		return 2
	}
}

func cmdDetectorsList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("detectors list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stableOnly := fs.Bool("stable-only", false, "Show only stable detectors")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var items []detectors.TagMetadata
	if *stableOnly {
		items = detectors.StableOnly()
	} else {
		items = detectors.AllMetadata()
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Tag < items[j].Tag
	})

	return writeJSON(stdout, stderr, "encode detectors list", map[string]interface{}{
		"count": len(items),
		"items": items,
	})
}
