//go:build experimental

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	benchmarkFeatureEnv = "EVIDRA_BENCHMARK_EXPERIMENTAL"
	benchmarkRoadmapURL = "docs/system-design/EVIDRA_BENCHMARK_CLI.md"
)

func cmdBenchmark(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printBenchmarkUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printBenchmarkUsage(stdout)
		return 0
	case "run", "list", "validate", "record", "compare":
		return runBenchmarkStub(args[0], args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "benchmark-cli-stub 0.1.0")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown benchmark subcommand: %s\n", args[0])
		printBenchmarkUsage(stderr)
		return 2
	}
}

func runBenchmarkStub(subcommand string, args []string, _ io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("benchmark "+subcommand, flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if !isBenchmarkExperimentalEnabled() {
		fmt.Fprintf(stderr,
			"benchmark %s is not yet implemented (dataset engine pending). set %s=1 to enable preview stubs. roadmap: %s\n",
			subcommand, benchmarkFeatureEnv, benchmarkRoadmapURL,
		)
		return 3
	}

	fmt.Fprintf(stderr,
		"benchmark %s preview stub: command surface is enabled, execution engine is not implemented yet. roadmap: %s\n",
		subcommand, benchmarkRoadmapURL,
	)
	return 4
}

func isBenchmarkExperimentalEnabled() bool {
	return strings.TrimSpace(os.Getenv(benchmarkFeatureEnv)) == "1"
}

func printBenchmarkUsage(w io.Writer) {
	fmt.Fprintln(w, "evidra benchmark <subcommand>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "SUBCOMMANDS:")
	fmt.Fprintln(w, "  run       Run benchmark dataset (stub)")
	fmt.Fprintln(w, "  list      List benchmark cases (stub)")
	fmt.Fprintln(w, "  validate  Validate dataset integrity (stub)")
	fmt.Fprintln(w, "  record    Record evidence chains from scenarios (stub)")
	fmt.Fprintln(w, "  compare   Compare benchmark runs (stub)")
	fmt.Fprintln(w, "  version   Print benchmark CLI stub version")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Feature gate: set %s=1 to enable preview stubs.\n", benchmarkFeatureEnv)
	fmt.Fprintf(w, "Roadmap: %s\n", benchmarkRoadmapURL)
}
