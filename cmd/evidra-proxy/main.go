// evidra-proxy is a transparent MCP stdio proxy that auto-records evidence
// for infrastructure mutations. Wrap any MCP server to get instant observability.
//
// Usage:
//
//	evidra-proxy [flags] -- <upstream-command> [upstream-args...]
//
// Example:
//
//	evidra-proxy -- npx -y @example/kubectl-mcp
//	evidra-proxy --verbose -- kubectl-mcp-server
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"samebits.com/evidra/pkg/proxy"
)

var (
	version = "dev"
	commit  = "dev"
)

func main() {
	evidenceDir := flag.String("evidence-dir", defaultEvidenceDir(), "evidence output directory")
	verbose := flag.Bool("verbose", false, "log interceptions to stderr")
	dryRun := flag.Bool("dry-run", false, "detect mutations but don't record evidence")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("evidra-proxy %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	args := flag.Args()

	// Find the "--" separator
	upstreamCmd, upstreamArgs := splitArgs(args)
	if upstreamCmd == "" {
		fmt.Fprintf(os.Stderr, "Usage: evidra-proxy [flags] -- <upstream-command> [args...]\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  evidra-proxy -- kubectl-mcp-server\n")
		fmt.Fprintf(os.Stderr, "  evidra-proxy --verbose -- npx -y @example/kubectl-mcp\n")
		os.Exit(1)
	}

	// Create evidence writer
	var evidence *proxy.EvidenceWriter
	if !*dryRun {
		var err error
		evidence, err = proxy.NewEvidenceWriter(*evidenceDir)
		if err != nil {
			log.Fatalf("evidra-proxy: %v", err)
		}
		defer evidence.Close()
		if *verbose {
			log.Printf("[proxy] evidence dir: %s", evidence.Dir())
		}
	}

	// Create and run proxy
	p := &proxy.Proxy{
		UpstreamCmd:  upstreamCmd,
		UpstreamArgs: upstreamArgs,
		Evidence:     evidence,
		Verbose:      *verbose,
		DryRun:       *dryRun,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := p.Run(ctx); err != nil {
		// Exit code 0 is normal (upstream exited cleanly)
		if ctx.Err() != nil {
			os.Exit(0)
		}
		log.Fatalf("evidra-proxy: %v", err)
	}
}

func splitArgs(args []string) (cmd string, cmdArgs []string) {
	// Args after flag.Parse() don't include flags, just positional
	// The "--" separator is consumed by flag.Parse, so args start with the upstream command
	if len(args) == 0 {
		return "", nil
	}
	// Skip leading "--" if present (some shells pass it through)
	start := 0
	if args[0] == "--" {
		start = 1
	}
	if start >= len(args) {
		return "", nil
	}
	return args[start], args[start+1:]
}

func defaultEvidenceDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".evidra/evidence"
	}
	return filepath.Join(home, ".evidra", "evidence")
}
