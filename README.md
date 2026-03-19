# Evidra

[![CI](https://github.com/vitas/evidra/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/vitas/evidra/actions/workflows/ci.yml)
[![Release Pipeline](https://github.com/vitas/evidra/actions/workflows/release.yml/badge.svg?event=push)](https://github.com/vitas/evidra/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**Evidra  — Flight recorder and reliability scoring for infrastructure AI agents**

Your AI agent fixes Kubernetes. Can you prove it?

Evidra records intent, outcome, and refusal in a signed, append-only evidence chain. It shows
risk before execution and reveals patterns like retry loops, drift, and escalation across
agents, pipelines, and controllers.

Evidra informs, not enforces. It is the flight recorder and intelligent scoring engine.

### Three Evidence Modes

| | Records what happened | Shows risk before action | Agent can decline | Works with any model |
|---|---|---|---|---|
| **Proxy Observed** | Yes | No | No | Yes |
| **Smart Prescribe** | Yes | Yes | Yes | Yes |
| **Full Prescribe** | Yes | Yes | Yes | Strong models only |

Proxy Observed records silently — the agent never knows. Smart Prescribe and Full Prescribe are explicit: the agent calls prescribe, receives risk assessment, and decides whether to proceed or decline. Smart Prescribe uses 4 fields (~30 tokens); Full Prescribe sends the complete YAML artifact (~300 tokens) and enables drift detection.


## The Prescribe/Report Protocol

Every infrastructure mutation follows the same lifecycle:

```text
prescribe  →  record intent, risk assessment, canonical form
execute    →  run the command (or decline to act)
report     →  record verdict, exit code, or refusal reason
```

`prescribe_full` and `prescribe_smart` capture intent **before** the command runs. `prescribe_full` records the artifact, its canonical form, digests, the per-source `risk_inputs` panel, and the rolled-up `effective_risk`. `prescribe_smart` records lightweight target context when artifact bytes are not available. `report` captures what **actually happened** — success, failure, or an explicit decision not to act, with structured context for each.

The evidence chain links prescriptions to reports through signed entries with hash chaining. Every entry is timestamped, actor-attributed, and cryptographically verifiable. Evidence cannot be modified after the fact.

When an agent decides not to execute — because risk is too high, because the operation looks wrong — that decision is a first-class evidence entry with trigger and reason. Not a silent gap in the log.

## What You Get

Evidra is one platform with three operating surfaces:

| Surface | What it does |
|---|---|
| `evidra` CLI | Wraps live commands, imports completed operations, computes scorecards |
| `evidra-mcp` | Exposes the prescribe/report protocol to MCP-connected agents and runtimes |
| Self-hosted API | Centralizes evidence across agents, pipelines, and controllers, and provides team-wide analytics |

From the evidence chain, Evidra computes:

- **Risk classification** at operation time — `risk_inputs`, `effective_risk`, canonical action digest
- **Behavioral signals** — protocol violations, retry loops, blast radius detection
- **Reliability scorecards** — score, band, and confidence for comparing agents, sessions, and time windows

Evidra does not replace OTel, Datadog, or Logfire. They record execution telemetry. Evidra records what they cannot: intent before execution, structured decisions, and behavioral patterns across the agent lifecycle.

CLI and MCP are the authoritative analytics surfaces today.

## Fastest Path

### Install

```bash
# Homebrew
brew install samebits/tap/evidra

# Binary release (Linux/macOS)
curl -fsSL https://github.com/samebits/evidra/releases/latest/download/evidra_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz \
  | tar -xz -C /usr/local/bin evidra

# Build from source
make build
```

### Record One Operation

```bash
evidra keygen
export EVIDRA_SIGNING_KEY=<base64>

evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml
```

For local smoke runs without a signing key:

```bash
export EVIDRA_SIGNING_MODE=optional
```

The output includes: `risk_inputs`, `effective_risk`, `score`, `score_band`, `signal_summary`, `basis`, and `confidence`.

### See The Scorecard

```bash
evidra scorecard --period 30d
evidra explain --period 30d
```

Security boundary: `evidra record` executes the wrapped local command directly. Evidra does not sandbox the command. Treat it with the same trust model as direct shell execution — Evidra records evidence around the command, not contain it.

## For AI Agents (MCP)

Evidra speaks MCP. The MCP server exposes the prescribe/report protocol to any MCP-connected agent or runtime.

```bash
evidra-mcp --evidence-dir ~/.evidra/evidence
```

The MCP server gives agents the tools. The skill teaches them when and how to use them — agents with the skill achieve 100% protocol compliance for infrastructure mutations.

```bash
evidra skill install
```

How the protocol looks from the agent's perspective:

```text
Agent: "I need to kubectl apply this deployment"
  → prescribe_smart(tool=kubectl, operation=apply, resource=deployment/web, namespace=default)
  ← prescription_id, effective_risk=medium, risk_inputs=[{source=evidra/matrix, ...}]

Agent: decides to proceed (or decline based on risk)
  → executes kubectl apply
  → report(prescription_id=..., verdict=success, exit_code=0)
  ← score=95, score_band=excellent, signal_summary={...}
```

If the agent decides not to act:

```text
Agent: "Risk too high, declining"
  → report(prescription_id=..., verdict=declined, decision_context={
      trigger: "risk_threshold_exceeded",
      reason: "privileged container in production"
    })
```

Declined verdicts are first-class evidence — not silent gaps in the log.

**Proxy Observed** — one config line, zero agent changes:

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

References: [MCP setup guide](docs/guides/mcp-setup.md) · [Skill setup guide](docs/guides/skill-setup.md) · [Execution schemas](pkg/execcontract/schemas/)

## For CI/CD Pipelines

The prescribe/report protocol also works without MCP. Two CLI modes feed the same lifecycle and scoring engine:

`evidra record` wraps a live command and records the full prescribe/execute/report lifecycle in one step. `evidra import` ingests a completed operation from structured input for pipelines that manage execution separately.

```bash
# Wrap a live command
evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml

# Import a completed operation
evidra import --input record.json
```

Additional workflows: `prescribe`, `report`, `scorecard`, `explain`, `compare`, `validate`, `import-findings`.

References: [CLI reference](docs/integrations/cli-reference.md) · [Record/Import contract](docs/system-design/EVIDRA_RUN_RECORD_CONTRACT_V1.md)

## For Platform Teams (Self-Hosted)

Run the Evidra backend to centralize evidence collection across agents, pipelines, and GitOps controllers, and get team-wide analytics. Argo CD is controller-first in v1; webhook ingestion remains supported, but it is not the only GitOps path.

```bash
export EVIDRA_API_KEY=my-secret-key
docker compose up --build -d
curl http://localhost:8080/healthz
```

The CLI forwards evidence to the backend:

```bash
evidra record --url http://localhost:8080 --api-key my-secret-key \
  -f deploy.yaml -- kubectl apply -f deploy.yaml
```

With centralized evidence, platform teams can compare reliability across agents, pipelines, and controllers, detect fleet-wide patterns, and answer questions like: which agents have incomplete prescribe/report pairs this week? Which controller workflows are retrying the same reconciliation? Which actor has the highest retry loop rate?

References: [Self-hosted setup](docs/guides/self-hosted-setup.md) · [Argo CD GitOps integration](docs/guides/argocd-gitops-integration.md) · [API reference](docs/api-reference.md) · [Setup Evidra Action](docs/guides/setup-evidra-action.md) · [Terraform CI quickstart](docs/guides/terraform-ci-quickstart.md)

## Supported Tools

Built-in adapters canonicalize artifacts across infrastructure tools into a normalized `CanonicalAction` model, enabling cross-tool comparison in a single evidence chain:

- Kubernetes-family YAML via `kubectl`, `helm`, `kustomize`, and `oc`
- Terraform plan JSON via `terraform show -json`
- Docker/container inspect JSON
- Generic fallback ingestion for unsupported tools

Full support details: [Supported tools](docs/supported-tools.md)

## Behavioral Signals

The evidence chain's prescribe/report structure makes agent behavior patterns visible without external instrumentation. Three signals fire immediately in real operations:

**protocol_violation** — a prescribe without a matching report (agent crashed, timed out, or skipped the protocol), a report without a prior prescribe (unauthorized action), duplicate reports, or cross-actor reports. This is the most operationally immediate signal — it fires whenever the protocol is broken.

**retry_loop** — the same intent retried multiple times within a window, typically after failures. Indicates an agent stuck in a retry cycle. Fires when the same intent digest appears 3+ times in 30 minutes with prior failures.

**blast_radius** — a destroy operation affecting more than 5 resources. Indicates a potentially high-impact deletion that warrants review.

Additional signals (`artifact_drift`, `new_scope`, `repair_loop`, `thrashing`, `risk_escalation`) contribute to scoring and mature as evidence accumulates. All eight are documented in the [Signal specification](docs/system-design/EVIDRA_SIGNAL_SPEC_V1.md).

Scoring details: [Scoring model](docs/system-design/EVIDRA_SCORING_MODEL_V1.md) · [Default profile rationale](docs/system-design/scoring/default.v1.1.0.md)

## Docs Map

Architecture and protocol:

- [V1 Architecture](docs/system-design/EVIDRA_ARCHITECTURE_V1.md)
- [Prescribe/Report Protocol](docs/system-design/EVIDRA_PROTOCOL_V1.md)
- [Core Data Model](docs/system-design/EVIDRA_CORE_DATA_MODEL_V1.md)
- [Canonicalization Contract](docs/system-design/EVIDRA_CANONICALIZATION_CONTRACT_V1.md)
- [Signal Specification](docs/system-design/EVIDRA_SIGNAL_SPEC_V1.md)
- [Scoring Rationale](docs/system-design/scoring/default.v1.1.0.md)

Integration and operations:

- [CLI Reference](docs/integrations/cli-reference.md)
- [MCP Setup Guide](docs/guides/mcp-setup.md)
- [Skill Setup Guide](docs/guides/skill-setup.md)
- [API Reference](docs/api-reference.md)
- [Supported Tools](docs/supported-tools.md)
- [Observability Quickstart](docs/guides/observability-quickstart.md)
- [Scanner SARIF Quickstart](docs/integrations/scanner-sarif-quickstart.md)
- [Self-Hosted Setup Guide](docs/guides/self-hosted-setup.md)

Developer references:

- [Architecture Overview](docs/ARCHITECTURE.md)
- [E2E Testing Map](docs/E2E_TESTING.md)
- [Tests Index](docs/tests-index.md)
- [Shared Artifact Fixtures](tests/artifacts/fixtures/README.md)

## Development

```bash
make build
make test
make e2e
make test-contracts
make test-mcp-inspector
make lint
make test-signals
```

## License

Licensed under the [Apache License 2.0](LICENSE).
