# Evidra

[![CI](https://github.com/vitas/evidra/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/vitas/evidra/actions/workflows/ci.yml)
[![Release Pipeline](https://github.com/vitas/evidra/actions/workflows/release.yml/badge.svg?event=push)](https://github.com/vitas/evidra/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**DevOps MCP server with smart output, built-in flight recorder, and reliability scoring**

One MCP server for kubectl, helm, terraform, and aws. Token-efficient output.
Every mutation automatically recorded. No extra agent code needed.

## Quick Start

```json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": ["--evidence-dir", "~/.evidra/evidence"]
    }
  }
}
```

That's it. Your agent gets `run_command` with smart output and automatic evidence recording for every mutation.

```bash
# Install
brew install samebits/tap/evidra
```

## What Your Agent Gets

### Smart output — fewer tokens, same information

```
Agent: run_command("kubectl get deployment web -n bench")

# Without evidra-mcp (raw JSON): ~2,400 tokens
{"apiVersion":"apps/v1","metadata":{"managedFields":[...],...},"spec":{...},"status":{...}}

# With evidra-mcp (smart output): ~40 tokens
deployment/web (bench): 0/2 ready | image: nginx:99.99 | Available=False
```

### Auto-evidence for mutations — zero agent code

```
Agent: run_command("kubectl apply -f fix.yaml")
  → evidra auto-prescribes (intent recorded)
  → kubectl executes
  → evidra auto-reports (outcome recorded)
  → smart output returned to agent
```

Read-only commands (`get`, `describe`, `logs`) execute directly — no overhead.

### Skills — tested on real infrastructure

Install the [Evidra skill](docs/guides/skill-setup.md) to give your agent
operational discipline: diagnosis before fix, safety boundaries, domain-specific
patterns. Skills are tested on 62 real scenarios via [infra-bench](https://lab.evidra.cc)
before shipping — skills that hurt performance don't ship.

### 5 tools, not 270

| Tool | Description |
|---|---|
| `run_command` | Execute kubectl, helm, terraform, aws — with smart output |
| `prescribe_smart` | Record intent before mutation (optional, for explicit protocol) |
| `prescribe_full` | Record intent with full artifact (optional) |
| `report` | Record outcome (optional, auto-recorded in proxy mode) |
| `get_event` | Look up evidence |

Most agents only need `run_command`. Evidence is automatic.

## Why Not Just kubectl-mcp-server?

| | kubectl-mcp-server | evidra-mcp |
|---|---|---|
| Tools | 270 specialized | 5 (one `run_command` for all) |
| Output | Raw JSON (~2400 tokens) | Smart summary (~40 tokens) |
| Evidence | None | Auto prescribe/report for mutations |
| Security | Open | Command allowlist + blocked subcommands |
| Skills | None | Bench-tested, installable |
| Scoring | None | Reliability scorecards + behavioral signals |

## For Platform Teams

### Self-hosted analytics

```bash
docker compose up --build -d
```

Centralize evidence across agents, pipelines, and controllers:
- Which agents retry the same operation?
- Which scenarios cause the most failures?
- How does model X compare to model Y on real infrastructure?

### CI/CD integration

```bash
# Wrap any command — CLI records prescribe/execute/report
evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml

# Import completed operations
evidra import --input record.json

# View reliability scorecard
evidra scorecard --period 30d
```

References: [Self-hosted setup](docs/guides/self-hosted-setup.md) · [CLI reference](docs/integrations/cli-reference.md) · [API reference](docs/api-reference.md)

## For Agent Benchmarking

Test which skills and tools actually improve your agent. 62 real scenarios
on real Kubernetes clusters.

```bash
# Baseline — no skill
infra-bench certify --track cka --model sonnet --provider bifrost

# With role skill
infra-bench certify --track cka --model sonnet --role k8s-admin

# Result: skills help L1 (75% fewer turns) but break L2 diagnosis
```

Bench repo: [evidra-infra-bench](https://github.com/vitas/evidra-infra-bench) |
Dashboard: [lab.evidra.cc/bench](https://lab.evidra.cc/bench)

## Intelligence Layer

From the evidence chain, Evidra computes:

- **Risk assessment** — pluggable pipeline with multiple assessors
- **Behavioral signals** — protocol violations, retry loops, blast radius, drift detection
- **Reliability scorecards** — 0-100 score with band and confidence

Eight behavioral signals documented in the [Signal specification](docs/system-design/EVIDRA_SIGNAL_SPEC_V1.md).

## Explicit Protocol (Advanced)

For agents that want full control over evidence recording:

```text
prescribe  →  canonicalize artifact → assess risk → record intent
execute    →  run the command (or decline to act)
report     →  record verdict, exit code, or refusal reason
```

Three modes:

| Mode | How | Agent awareness |
|---|---|---|
| **Proxy** | Auto prescribe/report via `run_command` | None needed |
| **Smart** | Agent calls `prescribe_smart` + `report` | Minimal (~30 tokens) |
| **Full** | Agent calls `prescribe_full` with artifact | Full artifact (~300 tokens) |

Most users should use Proxy mode (default). Smart and Full are for teams
that want agents to see risk assessments before executing.

## Proxy Mode — Wrap Any MCP Server

Add evidence to an existing MCP server — zero agent changes:

```json
{
  "mcpServers": {
    "infra": {
      "command": "evidra-mcp",
      "args": ["--proxy", "--", "npx", "-y", "@anthropic/mcp-server-kubernetes"]
    }
  }
}
```

## Docs

- [MCP Setup Guide](docs/guides/mcp-setup.md)
- [Skill Setup Guide](docs/guides/skill-setup.md)
- [CLI Reference](docs/integrations/cli-reference.md)
- [API Reference](docs/api-reference.md)
- [Architecture](docs/system-design/EVIDRA_ARCHITECTURE_V1.md)
- [Protocol Specification](docs/system-design/EVIDRA_PROTOCOL_V1.md)
- [Supported Tools](docs/supported-tools.md)

## Development

```bash
make build
make test
make lint
```

## License

Licensed under the [Apache License 2.0](LICENSE).
