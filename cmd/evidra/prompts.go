package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"samebits.com/evidra/internal/promptfactory"
	promptdata "samebits.com/evidra/prompts"
)

func cmdPrompts(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printPromptsUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printPromptsUsage(stdout)
		return 0
	case "generate":
		return runPromptsGenerate(args[1:], stdout, stderr)
	case "verify":
		return runPromptsVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown prompts subcommand: %s\n", args[0])
		printPromptsUsage(stderr)
		return 2
	}
}

func runPromptsGenerate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prompts generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	contractVersion := fs.String("contract", promptdata.DefaultContractVersion, "Contract version to generate")
	root := fs.String("root", ".", "Repository root containing prompts/")
	writeActive := fs.Bool("write-active", true, "Write active runtime prompt paths under prompts/mcpserver and prompts/experiments")
	writeGenerated := fs.Bool("write-generated", true, "Write generated prompt artifacts under prompts/generated/<contract>/")
	writeManifest := fs.Bool("write-manifest", true, "Write prompt manifest under prompts/manifests/<contract>.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "resolve --root: %v\n", err)
		return 2
	}

	files, err := promptfactory.Generate(promptfactory.GenerateOptions{
		RootDir:         rootAbs,
		ContractVersion: *contractVersion,
		WriteActive:     *writeActive,
		WriteGenerated:  *writeGenerated,
		WriteManifest:   *writeManifest,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prompts generate: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "generated %d prompt files for %s\n", len(files), *contractVersion)
	return 0
}

func runPromptsVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prompts verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	contractVersion := fs.String("contract", promptdata.DefaultContractVersion, "Contract version to verify")
	root := fs.String("root", ".", "Repository root containing prompts/")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "resolve --root: %v\n", err)
		return 2
	}

	if err := promptfactory.Verify(rootAbs, *contractVersion); err != nil {
		fmt.Fprintf(stderr, "prompts verify: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "prompt verification passed for %s\n", *contractVersion)
	return 0
}

func printPromptsUsage(w io.Writer) {
	fmt.Fprintln(w, "evidra prompts <subcommand>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "SUBCOMMANDS:")
	fmt.Fprintln(w, "  generate   Generate prompt artifacts from canonical contract sources")
	fmt.Fprintln(w, "  verify     Verify active/generated prompt files against canonical sources + manifest")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Defaults: --contract %s --root .\n", promptdata.DefaultContractVersion)
}
