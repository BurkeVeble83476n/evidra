package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
