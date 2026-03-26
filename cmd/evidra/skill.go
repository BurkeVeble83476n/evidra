package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	promptdata "samebits.com/evidra/prompts"
)

func cmdSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printSkillUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printSkillUsage(stdout)
		return 0
	case "install":
		return runSkillInstall(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown skill subcommand: %s\n", args[0])
		printSkillUsage(stderr)
		return 2
	}
}

func runSkillInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	target := fs.String("target", "claude", "Target platform (claude)")
	scope := fs.String("scope", "global", "Installation scope (global, project)")
	projectDir := fs.String("project-dir", ".", "Project directory for --scope project")
	fullPrescribe := fs.Bool("full-prescribe", false, "Install the full-prescribe skill variant")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *target != "claude" {
		fmt.Fprintf(stderr, "unsupported target: %s (supported: claude)\n", *target)
		return 2
	}

	if *scope != "global" && *scope != "project" {
		fmt.Fprintf(stderr, "unsupported scope: %s (supported: global, project)\n", *scope)
		return 2
	}

	readSkill := promptdata.ReadSkill
	modeLabel := "smart"
	if *fullPrescribe {
		readSkill = promptdata.ReadSkillFull
		modeLabel = "full-prescribe"
	}

	content, err := readSkill()
	if err != nil {
		fmt.Fprintf(stderr, "read embedded skill: %v\n", err)
		return 1
	}

	destDir, err := skillDestDir(*target, *scope, *projectDir)
	if err != nil {
		fmt.Fprintf(stderr, "resolve skill path: %v\n", err)
		return 1
	}

	destPath := filepath.Join(destDir, "SKILL.md")
	_, existsErr := os.Stat(destPath)
	exists := existsErr == nil

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "create directory %s: %v\n", destDir, err)
		return 1
	}

	if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(stderr, "write skill file: %v\n", err)
		return 1
	}

	verb := "installed"
	if exists {
		verb = "updated"
	}

	fmt.Fprintf(stdout, "Evidra skill %s: %s\n", verb, destPath)
	fmt.Fprintf(stdout, "Contract version: %s\n", promptdata.DefaultContractVersion)
	fmt.Fprintf(stdout, "Target: %s (%s)\n", *target, *scope)
	fmt.Fprintf(stdout, "Mode: %s\n", modeLabel)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "The skill guides AI agents through Evidra's infrastructure workflow")
	fmt.Fprintln(stdout, "and explicit evidence-recording protocol when needed.")
	return 0
}

func skillDestDir(target, scope, projectDir string) (string, error) {
	switch {
	case target == "claude" && scope == "global":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".claude", "skills", "evidra"), nil
	case target == "claude" && scope == "project":
		absDir, err := filepath.Abs(projectDir)
		if err != nil {
			return "", fmt.Errorf("resolve project directory: %w", err)
		}
		return filepath.Join(absDir, ".claude", "skills", "evidra"), nil
	default:
		return "", fmt.Errorf("unsupported target/scope: %s/%s", target, scope)
	}
}

func printSkillUsage(w io.Writer) {
	fmt.Fprintln(w, "evidra skill <subcommand>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "SUBCOMMANDS:")
	fmt.Fprintln(w, "  install    Install Evidra skill for AI agent protocol compliance")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FLAGS (install):")
	fmt.Fprintln(w, "  --target       Target platform: claude (default: claude)")
	fmt.Fprintln(w, "  --scope        Installation scope: global, project (default: global)")
	fmt.Fprintln(w, "  --project-dir  Project directory for --scope project (default: .)")
	fmt.Fprintln(w, "  --full-prescribe  Install the full-prescribe skill variant")
}
