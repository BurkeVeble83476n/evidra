# Skill Install Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `evidra skill install` CLI command that writes the embedded SKILL.md to the user's skills directory, and update all public docs and landing page to explain why the skill matters.

**Architecture:** Embed `prompts/skill/SKILL.md` in the `evidra` binary via `//go:embed`. New `cmd/evidra/skill.go` handler reads embedded content and writes to `~/.claude/skills/evidra/SKILL.md` (global) or `.claude/skills/evidra/SKILL.md` (project). Documentation updates across 4 markdown files + 1 React component.

**Tech Stack:** Go, `//go:embed`, stdlib `flag`, React/TypeScript (landing page)

---

### Task 1: Embed skill file and add ReadSkill function

**Files:**
- Modify: `prompts/embed.go:20-25`
- Modify: `prompts/embed_test.go`

**Step 1: Write the failing test**

Add to `prompts/embed_test.go`:

```go
func TestReadSkill_ReturnsNonEmptyContent(t *testing.T) {
	t.Parallel()

	content, err := ReadSkill()
	if err != nil {
		t.Fatalf("ReadSkill: %v", err)
	}
	if content == "" {
		t.Fatal("ReadSkill returned empty content")
	}
	if !strings.HasPrefix(content, "---\nname: evidra\n") {
		t.Fatal("skill missing YAML frontmatter")
	}
	if !strings.Contains(content, "prescribe") {
		t.Fatal("skill missing prescribe content")
	}
}
```

Make sure `"strings"` is in the imports.

**Step 2: Run test to verify it fails**

Run: `go test ./prompts/ -run TestReadSkill -v -count=1`
Expected: FAIL — `ReadSkill` undefined

**Step 3: Add skill to embed directive and ReadSkill function**

In `prompts/embed.go`, change the `//go:embed` directive on line 23 to include the skill file:

```go
//go:embed mcpserver/initialize/instructions.txt mcpserver/tools/prescribe_description.txt mcpserver/tools/report_description.txt mcpserver/tools/get_event_description.txt mcpserver/resources/content/agent_contract_v1.md skill/SKILL.md
```

Add the `ReadSkill` function after the existing `Read` function:

```go
func ReadSkill() (string, error) {
	return Read(SkillPath)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./prompts/ -run TestReadSkill -v -count=1`
Expected: PASS

**Step 5: Run all prompts tests**

Run: `go test ./prompts/ -v -count=1`
Expected: All PASS

**Step 6: Format**

Run: `gofmt -w prompts/embed.go prompts/embed_test.go`

---

### Task 2: Create skill CLI command handler

**Files:**
- Create: `cmd/evidra/skill.go`

**Step 1: Write the failing test**

Create `cmd/evidra/skill_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdSkill_InstallGlobal(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// First install.
	var out, errBuf bytes.Buffer
	run([]string{"skill", "install", "--target", "claude", "--scope", "global"}, &out, &errBuf)

	// Second install — should say "updated".
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
	t.Parallel()

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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/evidra/ -run TestCmdSkill -v -count=1`
Expected: FAIL — `cmdSkill` undefined

**Step 3: Create skill.go**

Create `cmd/evidra/skill.go`:

```go
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
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *target != "claude" {
		fmt.Fprintf(stderr, "unsupported target: %s (supported: claude)\n", *target)
		return 2
	}

	content, err := promptdata.ReadSkill()
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
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "The skill guides AI agents to follow the Evidra prescribe/report protocol")
	fmt.Fprintln(stdout, "with 100% compliance for infrastructure mutations.")
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
}
```

**Step 4: Register the skill command**

In `cmd/evidra/command_registry.go`, add the skill command to `orderedCommands` after the `keygen` entry (before `version`):

```go
{name: "skill", description: "Install Evidra skill for AI agent protocol compliance", run: cmdSkill},
```

**Step 5: Run tests to verify they pass**

Run: `go test ./cmd/evidra/ -run TestCmdSkill -v -count=1`
Expected: All 6 tests PASS

**Step 6: Run all cmd/evidra tests**

Run: `go test ./cmd/evidra/ -v -count=1`
Expected: All PASS

**Step 7: Verify command_registry test still passes**

Run: `go test ./cmd/evidra/ -run TestPrintUsage -v -count=1`
Expected: PASS — the `skill` command appears in usage output

**Step 8: Format**

Run: `gofmt -w cmd/evidra/skill.go cmd/evidra/skill_test.go cmd/evidra/command_registry.go`

**Step 9: Build and smoke test**

Run: `make build`
Expected: Builds successfully

Run: `bin/evidra skill --help`
Expected: Shows skill usage with `install` subcommand

Run: `bin/evidra skill install --scope project --project-dir /tmp/test-skill`
Expected: Writes skill to `/tmp/test-skill/.claude/skills/evidra/SKILL.md`

---

### Task 3: Commit embed + CLI command

**Step 1: Run full test suite**

Run: `make test`
Expected: All PASS

**Step 2: Commit**

```bash
git add prompts/embed.go prompts/embed_test.go \
  cmd/evidra/skill.go cmd/evidra/skill_test.go \
  cmd/evidra/command_registry.go
git commit -s -m "feat: add evidra skill install CLI command

Embed SKILL.md in the binary and add 'evidra skill install' command
that writes the skill to ~/.claude/skills/evidra/SKILL.md (global)
or .claude/skills/evidra/SKILL.md (project).

The skill guides AI agents to follow the prescribe/report protocol
with 100% compliance for infrastructure mutations."
```

---

### Task 4: Create skill setup guide

**Files:**
- Create: `docs/guides/skill-setup.md`

**Step 1: Write the guide**

Create `docs/guides/skill-setup.md`:

```markdown
# Evidra Skill Setup

- Status: Guide
- Version: current
- Canonical for: Skill installation and usage
- Audience: public

The Evidra skill teaches AI agents the prescribe/report protocol for infrastructure mutations. When installed, agents follow the protocol with 100% compliance — calling prescribe before every mutation and report after every execution or refusal.

---

## Why Install the Skill

The Evidra MCP server gives AI agents access to `prescribe`, `report`, and `get_event` tools. But having tools available does not guarantee the agent will use them correctly. Without guidance, agents may:

- Skip `prescribe` for some mutations
- Forget to call `report` after failures
- Omit required fields like `exit_code` or `actor.skill_version`
- Call `prescribe` for read-only operations that don't need it
- Reuse a `prescription_id` on retry instead of calling `prescribe` again

The skill embeds protocol rules, invariants, classification tables, and a decision flowchart directly into the agent's context. In testing, agents with the skill installed achieved **100% protocol compliance** compared to inconsistent behavior with the MCP server alone.

**The MCP server gives agents the tools. The skill teaches them when and how to use them.**

---

## Install

```bash
# Install globally (recommended — works across all projects)
evidra skill install

# Install for a specific project only
evidra skill install --scope project
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | `claude` | Target platform. Currently supports `claude` (Claude Code). |
| `--scope` | `global` | `global` installs to `~/.claude/skills/evidra/SKILL.md`. `project` installs to `.claude/skills/evidra/SKILL.md` in the current directory. |
| `--project-dir` | `.` | Project directory when using `--scope project`. |

### Global vs Project

- **Global** (`~/.claude/skills/evidra/SKILL.md`): The skill is available in every Claude Code session on this machine. Recommended for individual use.
- **Project** (`.claude/skills/evidra/SKILL.md`): The skill is scoped to one repository. Useful when you want to commit the skill alongside your project so all team members get it automatically.

---

## Verify

After installing, start a new Claude Code session and ask:

> "What skills do you have?"

You should see `evidra` in the list. Then ask:

> "Apply this deployment to staging" (with a YAML manifest)

The agent should call `prescribe` before executing `kubectl apply` and `report` after.

---

## How Skill and MCP Server Work Together

```
MCP Server (evidra-mcp)          Skill (SKILL.md)
─────────────────────────        ─────────────────────────
Provides tools:                  Provides protocol knowledge:
  • prescribe                      • When to call prescribe
  • report                         • What fields are required
  • get_event                      • How to handle failures
                                   • Classification tables
Observes and scores              • Decision flowchart
  • Risk assessment                • Retry rules
  • Signal detection
  • Reliability scoring

         ┌──────────────────────┐
         │  Both are needed for │
         │  100% compliance     │
         └──────────────────────┘
```

The MCP server handles evidence recording, risk analysis, and scoring. The skill handles agent behavior — ensuring the agent calls the right tool at the right time with the right inputs.

---

## Update

When you update Evidra (`brew upgrade evidra` or download a new release), run `evidra skill install` again to get the latest skill version. The command overwrites the existing file.

---

## Supported Platforms

| Platform | Status | Target flag |
|----------|--------|-------------|
| Claude Code | Supported | `--target claude` (default) |
| Cursor | Planned | — |
| Codex | Planned | — |
| Windsurf | Planned | — |
```

**Step 2: Verify the file renders correctly**

Run: `head -20 docs/guides/skill-setup.md`
Expected: Shows header with title and metadata

---

### Task 5: Update CLI reference

**Files:**
- Modify: `docs/integrations/cli-reference.md`

**Step 1: Add skill to Command Groups table**

In the Command Groups table (after the `keygen` row, before `version`), add:

```markdown
| `skill` | Install Evidra skill for AI agent protocol compliance |
```

**Step 2: Add skill install section**

After the `evidra prompts` section (after line 224) and before the Developer Commands section, add:

```markdown
### `evidra skill install` Flags

| Flag | Description |
|---|---|
| `--target` | Target platform: `claude` (default: `claude`) |
| `--scope` | Installation scope: `global` (default) or `project` |
| `--project-dir` | Project directory for `--scope project` (default: `.`) |

Global installs to `~/.claude/skills/evidra/SKILL.md`. Project installs to `.claude/skills/evidra/SKILL.md` in the specified directory.
```

---

### Task 6: Update MCP setup guide

**Files:**
- Modify: `docs/guides/mcp-setup.md`

**Step 1: Add skill install section**

After the "2. Connect to your agent" section (after line 89, before "### 3. Test"), add a new section:

```markdown
### 2.5 Install the Evidra skill (Claude Code)

The MCP server gives agents the tools. The skill teaches them when and how to use them — agents with the skill installed achieve 100% protocol compliance.

```bash
evidra skill install
```

This writes the skill to `~/.claude/skills/evidra/SKILL.md`. For project-scoped installation: `evidra skill install --scope project`.

Full guide: [Skill Setup](skill-setup.md)
```

---

### Task 7: Update README

**Files:**
- Modify: `README.md`

**Step 1: Add skill install to Fastest Path section**

After the "### Install" code block (after line 86), add a new subsection:

```markdown
### Install the Skill (Claude Code)

The MCP server gives agents the tools. The skill teaches them when and how to use them — agents with the skill achieve 100% protocol compliance for infrastructure mutations.

```bash
evidra skill install
```
```

**Step 2: Add skill to MCP Service section**

After the MCP Service references list (after line 153), add:

```markdown
For Claude Code users, install the Evidra skill for 100% protocol compliance:

```bash
evidra skill install
```

See the [Skill Setup Guide](docs/guides/skill-setup.md) for details.
```

**Step 3: Add skill setup guide to Docs Map**

In the "Integration and operations" section of Docs Map (after the CLI Reference line), add:

```markdown
- [Skill Setup Guide](docs/guides/skill-setup.md)
```

---

### Task 8: Commit documentation updates

**Step 1: Commit**

```bash
git add docs/guides/skill-setup.md \
  docs/integrations/cli-reference.md \
  docs/guides/mcp-setup.md \
  README.md
git commit -s -m "docs: add skill setup guide and update CLI reference, MCP setup, README

New skill-setup.md guide explains why the skill matters (100% protocol
compliance) and how to install it. Updated CLI reference with skill
install flags. Added skill install step to MCP setup guide. Added
skill install to README Fastest Path section."
```

---

### Task 9: Update landing page

**Files:**
- Modify: `ui/src/pages/Landing.tsx`

**Step 1: Add skill install step to McpSetup component**

In the `McpSetup` function, between the "2. Connect to your editor" `div` (ending around line 445) and the "3. Verify" `div` (starting around line 447), add a new step that is only visible when `claude-code` tab is selected:

```tsx
{editor === "claude-code" && (
  <div className="mb-8">
    <h3 className="text-[0.95rem] font-bold text-fg mb-3">3. Install the Evidra skill</h3>
    <CodeBlock code="evidra skill install" />
    <p className="text-[0.83rem] text-fg-muted mt-2">
      The MCP server gives agents the tools. The skill teaches them when and how to use them — agents with the skill achieve <strong className="text-fg">100% protocol compliance</strong> for infrastructure mutations.{" "}
      <a href="https://github.com/vitas/evidra/blob/main/docs/guides/skill-setup.md" target="_blank" rel="noopener" className="font-semibold">Setup guide &rarr;</a>
    </p>
  </div>
)}
```

**Step 2: Renumber the Verify step**

Change the existing "3. Verify" heading to be dynamic — show "4. Verify" when `claude-code` tab is selected, "3. Verify" otherwise:

```tsx
<h3 className="text-[0.95rem] font-bold text-fg mb-3">{editor === "claude-code" ? "4" : "3"}. Verify</h3>
```

**Step 3: Add skill guide card to GUIDES array**

Add a new entry to the `GUIDES` array (after the MCP Setup entry):

```typescript
{ tag: "AI Agents", title: "Skill Setup", desc: "Install the Evidra skill for 100% protocol compliance. The skill teaches AI agents when and how to use prescribe/report.", href: "https://github.com/vitas/evidra/blob/main/docs/guides/skill-setup.md" },
```

---

### Task 10: Commit landing page update

**Step 1: Build the UI to verify**

Run: `cd ui && npm run build`
Expected: Build succeeds with no errors

**Step 2: Commit**

```bash
git add ui/src/pages/Landing.tsx
git commit -s -m "feat: add skill install step to landing page

Show 'Install the Evidra skill' step in MCP setup section when
Claude Code tab is selected. Add skill setup guide card to guides
section. Explains that MCP + skill = 100% protocol compliance."
```

---

### Task 11: Copy generated skill to local skills directory

**Step 1: Run the install command**

```bash
bin/evidra skill install
```

Expected: Writes to `~/.claude/skills/evidra/SKILL.md` with confirmation message.

**Step 2: Verify the installed skill matches the generated one**

```bash
diff prompts/skill/SKILL.md ~/.claude/skills/evidra/SKILL.md
```

Expected: No differences (the embedded content matches the generated file).
