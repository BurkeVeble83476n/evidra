package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	promptdata "samebits.com/evidra/prompts"
)

func TestCmdSkill_InstallGlobal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install", "--target", "claude", "--scope", "global"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill install exit %d: %s", code, errBuf.String())
	}

	skillPath := filepath.Join(tmp, ".claude", "skills", "evidra", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !strings.HasPrefix(string(content), "---\nname: evidra\n") {
		t.Fatal("installed skill missing YAML frontmatter")
	}
	if !strings.Contains(out.String(), "installed") {
		t.Fatalf("stdout missing 'installed': %s", out.String())
	}
}

func TestCmdSkill_InstallProject(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install", "--target", "claude", "--scope", "project", "--project-dir", tmp}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill install exit %d: %s", code, errBuf.String())
	}

	skillPath := filepath.Join(tmp, ".claude", "skills", "evidra", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !strings.HasPrefix(string(content), "---\nname: evidra\n") {
		t.Fatal("installed skill missing YAML frontmatter")
	}
	if !strings.Contains(out.String(), "installed") {
		t.Fatalf("stdout missing 'installed': %s", out.String())
	}
}

func TestCmdSkill_InstallOverwriteShowsUpdated(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out, errBuf bytes.Buffer
	run([]string{"skill", "install", "--target", "claude", "--scope", "global"}, &out, &errBuf)

	out.Reset()
	errBuf.Reset()
	code := run([]string{"skill", "install", "--target", "claude", "--scope", "global"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill install exit %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "updated") {
		t.Fatalf("stdout missing 'updated' on overwrite: %s", out.String())
	}
}

func TestCmdSkill_DefaultsToClaudeGlobal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill install exit %d: %s", code, errBuf.String())
	}

	skillPath := filepath.Join(tmp, ".claude", "skills", "evidra", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("skill not found at default path: %v", err)
	}
}

func TestCmdSkill_UnsupportedTargetFails(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install", "--target", "vim"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit for unsupported target")
	}
	if !strings.Contains(errBuf.String(), "unsupported target") {
		t.Fatalf("stderr missing unsupported target message: %s", errBuf.String())
	}
}

func TestCmdSkill_NoSubcommandShowsUsage(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"skill"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill usage exit %d", code)
	}
	if !strings.Contains(out.String(), "install") {
		t.Fatalf("usage missing 'install': %s", out.String())
	}
}

func TestCmdSkill_UnsupportedScopeFails(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install", "--scope", "workspace"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "unsupported scope") {
		t.Fatalf("stderr missing unsupported scope message: %s", errBuf.String())
	}
}

func TestCmdSkill_UnknownSubcommandFails(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "update"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "unknown skill subcommand") {
		t.Fatalf("stderr missing unknown subcommand message: %s", errBuf.String())
	}
}

func TestCmdSkill_HelpShowsUsage(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill help exit %d", code)
	}
	if !strings.Contains(out.String(), "install") {
		t.Fatalf("help missing 'install': %s", out.String())
	}
	if !strings.Contains(out.String(), "--target") {
		t.Fatalf("help missing '--target': %s", out.String())
	}
}

func TestCmdSkill_OutputContainsContractVersion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill install exit %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Contract version:") {
		t.Fatalf("stdout missing contract version: %s", out.String())
	}
	if !strings.Contains(out.String(), "Target: claude (global)") {
		t.Fatalf("stdout missing target info: %s", out.String())
	}
}

func TestCmdSkill_InstalledContentMatchesEmbedded(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out, errBuf bytes.Buffer
	code := run([]string{"skill", "install"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("skill install exit %d: %s", code, errBuf.String())
	}

	installed, err := os.ReadFile(filepath.Join(tmp, ".claude", "skills", "evidra", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}

	embedded, err := promptdata.ReadSkill()
	if err != nil {
		t.Fatalf("ReadSkill: %v", err)
	}

	if string(installed) != embedded {
		t.Fatal("installed skill content does not match embedded content")
	}
}
