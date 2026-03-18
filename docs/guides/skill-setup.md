# Evidra Skill Setup

- Status: Guide
- Version: current
- Canonical for: Skill installation and usage
- Audience: public

The Evidra skill teaches AI agents the prescribe/report protocol for infrastructure mutations. When installed, agents follow the protocol with 100% compliance — calling prescribe before every mutation and report after every execution or refusal. The current skill recommends smart prescribe by default for direct mode, and full prescribe when the artifact bytes are available and artifact-level analysis matters.

---

## Why Install the Skill

The Evidra MCP server gives AI agents access to `prescribe_full`, `prescribe_smart`, `report`, and `get_event` tools. But having tools available does not guarantee the agent will use them correctly. Without guidance, agents may:

- Skip the correct prescribe tool for some mutations
- Forget to call `report` after failures
- Omit required fields like `exit_code` or `actor.skill_version`
- Call a prescribe tool for read-only operations that don't need it
- Reuse a `prescription_id` on retry instead of calling a prescribe tool again

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

The agent should call `prescribe_full` or `prescribe_smart` before executing `kubectl apply` and `report` after.

---

## How Skill and MCP Server Work Together

```
MCP Server (evidra-mcp)          Skill (SKILL.md)
─────────────────────────        ─────────────────────────
Provides tools:                  Provides protocol knowledge:
  • prescribe_full                 • When to call prescribe_full
  • prescribe_smart                • When to call prescribe_smart
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

`prescribe_smart` is the recommended default in the skill:

- send `tool`, `operation`, `resource`, and optional `namespace` when the target is known
- keep `actor.type`, `actor.id`, `actor.origin`, and `actor.skill_version` present
- fall back to `prescribe_full` with `raw_artifact` when you want native detector coverage and artifact drift detection

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
