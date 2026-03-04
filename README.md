# Evidra Benchmark

**Flight recorder and reliability benchmark for infrastructure automation**

Evidence and reliability metrics for AI agents, CI pipelines, and IaC workflows.

Works with:

- AI DevOps agents (via MCP server)
- CI pipelines
- Terraform workflows
- Kubernetes manifests

---

## How It Works

1. **Prescribe** — before any infrastructure operation, record intent
2. **Execute** — run kubectl apply, terraform apply, helm upgrade, etc.
3. **Report** — after execution, record the outcome

Every prescribe/report pair generates an evidence record. Evidra never blocks operations — it records and measures.

---

## Signals

Evidra computes five behavioral signals from the evidence chain:

| Signal | What it detects |
|---|---|
| Protocol Violation | Missing prescriptions or reports, duplicate reports, cross-actor reports |
| Artifact Drift | Artifact changed between prescribe and execution |
| Retry Loop | Same operation repeated many times in a short window |
| Blast Radius | Destructive operations affecting many resources |
| New Scope | First-time tool/operation combination |

Signals are aggregated into a weighted reliability scorecard: `score = 100 × (1 - penalty)`.

---

## Quick Start

```bash
# Build
make build

# Prescribe before an operation
evidra prescribe --tool terraform --operation apply --artifact plan.json

# Report after execution
evidra report --prescription <id> --exit-code 0

# Generate scorecard
evidra scorecard --actor agent-1 --period 30d
```

### MCP Server (for AI agents)

```bash
# Run as MCP server on stdio
evidra-mcp --evidence-dir ~/.evidra/evidence

# Or via Docker
docker build -t evidra-mcp:dev -f Dockerfile .
```

---

## Architecture

```
raw artifact → adapter (k8s/tf/generic) → canonical action → risk detectors → prescription
                                                                                    ↓
exit code + prescription_id ────────────────────────────────────────────────→ report
                                                                                    ↓
                                                              evidence chain → signals → scorecard
```

Three binaries:

| Binary | Purpose |
|---|---|
| `evidra` | CLI: scorecard, compare, prescribe, report |
| `evidra-mcp` | MCP server for AI agents (stdio transport) |

---

## Documentation

### Architecture & Design

- [Architecture Overview](docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md) — system diagram, component map, data flow
- [Inspector Model Architecture](docs/system-design/EVIDRA_INSPECTOR_MODEL_ARCHITECTURE.md) — why Evidra observes instead of enforcing
- [Architecture Review](docs/system-design/EVIDRA_ARCHITECTURE_REVIEW.md) — gap analysis and trade-offs
- [Architecture Recommendation](docs/system-design/EVIDRA_ARCHITECTURE_RECOMMENTATION_V1.md) — v1 architecture decisions

### Specifications

- [Agent Reliability Benchmark](docs/system-design/EVIDRA_AGENT_RELIABILITY_BENCHMARK.md) — protocol, signals, scoring formula, Prometheus metrics
- [Signal Spec](docs/system-design/EVIDRA_SIGNAL_SPEC.md) — formal definitions of all five signals
- [Canonicalization Contract v1](docs/system-design/CANONICALIZATION_CONTRACT_V1.md) — adapter interface, digest model, compatibility rules
- [Canonicalization Test Strategy](docs/system-design/EVIDRA_CANONICALIZATION_TEST_STRATEGY.md) — golden corpus, determinism testing
- [End-to-End Example](docs/system-design/EVIDRA_END_TO_END_EXAMPLE_v2.md) — full prescribe/report walkthrough

### Product & Strategy

- [Product Positioning](docs/product/EVIDRA_PRODUCT_POSITIONING.md) — market position and value proposition
- [Roadmap](docs/product/EVIDRA_ROADMAP.md) — release plan and milestones
- [Strategic Direction](docs/product/EVIDRA_STRATEGIC_DIRECTION.md) — long-term vision
- [Strategic Moat & Standardization](docs/system-design/EVIDRA_STRATEGIC_MOAT_AND_STANDARDIZATION.md) — competitive positioning
- [Integration Roadmap](docs/system-design/EVIDRA_INTEGRATION_ROADMAP.md) — tool integration plan

### Migration & History

- [Migration Map](docs/system-design/done/EVIDRA_MIGRATION_MAP.md) — how evidra-benchmark was bootstrapped from evidra v0.2.0
- [Codebase Review](docs/system-design/done/EVIDRA_CODEBASE_REVIEW.md) — pre-migration code analysis
- [Current State Baseline](docs/system-design/done/EVIDRA_CURRENT_STATE_BASELINE.md) — state at migration start

### Backlog

- [Threat Model](docs/system-design/backlog/EVIDRA_THREAT_MODEL.md) — security considerations
- [Anti-Goodhart Addendum](docs/system-design/backlog/ANTI_GOODHART_BACKLOG_ADDENDUM.md) — gaming resistance

---

## License

Proprietary.
