# Evidra Architecture Overview

This is the non-normative public index for the live architecture set.
The canonical design and business logic live in the versioned docs under
`docs/system-design/`.

One-sentence model:

**Evidra is a flight recorder for infrastructure automation — it observes
and measures AI agent and CI pipeline reliability without blocking
operations.**

## Black Box Principle

Evidra is a wire tap, not a gatekeeper. It reads data from buses that
already exist (MCP stdio, OTLP traces, HTTP webhooks) and never asks
agents to change behavior. Agents, like pilots, need to do their work —
evidence exists to improve, not to prevent.

Prescribe is a pre-flight check, not a gate. When available it enriches
the evidence with intent and risk assessment, but it never blocks
execution. Passive recording (bridge/proxy mode) works without it.

## Two-Layer Architecture

**Recorder** (write path, real-time):
- Ingest evidence from any source
- Assessment pipeline: canonicalize → assess risk → aggregate
- Sign with Ed25519, chain via previous_hash, store

**Intelligence** (read path, post-hoc):
- Signal detection: 8 behavioral detectors across evidence sequences
- Scoring: weighted penalty model → 0-100 reliability metric
- Benchmarking: run comparison, leaderboards, model evaluation
- Analytics: scorecards, explain, trends

## Observation Modes

All modes produce the same evidence entries and feed the same intelligence
pipeline.

| Mode | How Evidra connects | Prescribe | Assessment |
|------|-------------------|-----------|------------|
| **MCP direct** | Agent calls `prescribe_full`/`prescribe_smart` + `report` | Yes — full intent + artifact | Full pipeline |
| **MCP proxy** | `evidra-mcp --proxy` wraps upstream MCP server, intercepts `tools/call` | Implicit | Observed only |
| **OTLP bridge** | Reads AgentGateway OTLP traces, translates to prescribe/report | Implicit | Observed only |
| **Webhooks** | ArgoCD/generic webhook → mapped prescribe/report | Translated | Full pipeline |
| **Ext-authz** (future) | Gateway calls Evidra assessment endpoint before forwarding | Yes — via gateway | Full pipeline |

MCP direct gives the richest evidence. Proxy and bridge are passive taps.
Ext-authz combines both: the gateway consults Evidra for risk assessment,
the agent never changes.

## Assessment Pipeline

At prescribe time, risk assessment runs through the pluggable `internal/assess/` pipeline. Both `lifecycle` (CLI/MCP) and `ingest` (API) prescribe paths call `assess.Pipeline.Run()`:

1. Pipeline receives a `CanonicalAction` and raw artifact bytes
2. Registered `Assessor` implementations run in order:
   - `MatrixAssessor` — static risk matrix lookup (`operationClass x scopeClass`)
   - `DetectorAssessor` — native tag detectors (privileged containers, wildcard RBAC, etc.)
   - `SARIFAssessor` — external scanner findings from SARIF reports
3. Each assessor returns `[]RiskInput` with source, risk level, and tags
4. Pipeline aggregates via max-severity into `effective_risk`

The pipeline replaces the former monolithic risk computation that was duplicated across lifecycle and ingest services.

## Hosted Mode

Hosted mode changes where evidence is collected and replayed, not what evidence means.

- CLI and MCP can keep evidence local in append-only JSONL or forward the same signed entries to `evidra-api`.
- Self-hosted also accepts raw `/v1/evidence/forward` and `/v1/evidence/batch` transport, plus typed `/v1/evidence/ingest/prescribe` and `/v1/evidence/ingest/report` lifecycle ingest for external adapters.
- Self-hosted also accepts webhook ingestion and controller-observed GitOps reconciliation evidence from systems such as ArgoCD, and maps those events into the same evidence model. Webhook routes are compatibility wrappers over the shared lifecycle ingest service.
- `evidra-api` stores tenant evidence in Postgres and runs tenant-wide `scorecard` / `explain` over that centralized evidence.
- Deliberate refusals remain first-class evidence: `report(verdict=declined, decision_context)` is analyzed through the same signal and scoring path as any other terminal report.
- The lifecycle pair stays `prescribe_full` or `prescribe_smart`, followed by `report`; the external ingest request contract uses `flavor`, `evidence.kind`, and `source.system` to describe execution shape and ingestion source without creating a second scoring lane. Persisted entries expose the same context as `payload.flavor`, `payload.evidence.kind`, and `payload.source.system`. `flavor` includes `imperative`, `reconcile`, and `workflow`; `evidence.kind` includes `declared`, `observed`, and `translated`.

```text
                          RECORDER                          INTELLIGENCE
                          ────────                          ────────────

  MCP direct ──────────┐
  MCP proxy ───────────┤
  CLI record/import ───┤                                   ┌──────────────────┐
                       ├──▸ assess.Pipeline ──▸ sign ──▸ store ──▸ signals    │
  OTLP bridge ─────────┤    (canonicalize,       chain      │     scoring     │
  Webhooks ────────────┤     assess risk,                   │     benchmarks  │
  Ext-authz (future) ──┘     aggregate)                     │     analytics   │
                                                            └──────────────────┘
                                                                    │
  Storage:                                                          ▼
    local ──▸ JSONL evidence chain                          scorecard / explain
    hosted ──▸ Postgres (evidra-api)                        bench comparison
                                                            leaderboards
```

## Where To Find Details

Normative contracts:
- [Protocol](system-design/EVIDRA_PROTOCOL_V1.md)
- [Core Data Model](system-design/EVIDRA_CORE_DATA_MODEL_V1.md)
- [Canonicalization Contract](system-design/EVIDRA_CANONICALIZATION_CONTRACT_V1.md)
- [Signal Spec](system-design/EVIDRA_SIGNAL_SPEC_V1.md)

System design and implementation mapping:
- [Architecture](system-design/EVIDRA_ARCHITECTURE_V1.md)
- [Record/Import Contract](system-design/EVIDRA_RUN_RECORD_CONTRACT_V1.md)
- [Default Scoring Profile](system-design/scoring/default.v1.1.0.md)

### Bench Intelligence Layer

Infrastructure agent benchmark results and analytics.

**Public types:** `pkg/bench/` — RunRecord, RunFilters, timeline parser
**Private implementation:** `internal/benchsvc/` — request-scoped `Service`, internal `Repository` contract, `PgStore`, HTTP handlers, JSONL import
**Database:** `bench_runs`, `bench_artifacts`, `bench_scenarios` tables (migration 006)
**UI:** `ui/src/pages/bench/` — Leaderboard, Dashboard, Runs, RunDetail

The bench layer uses an internal `Service -> Repository` seam: `benchsvc.Service`
accepts a tenant ID per call, and the underlying `PgStore` repository handles
persistence without tenant assumptions. The repository contract is intentionally
kept inside `internal/benchsvc/`; `pkg/bench/` only carries data types shared by
the API, UI, and import/export paths.

Run ingestion is transactional at the service boundary. `POST /v1/bench/runs`
commits the run and any attached artifacts atomically, and
`POST /v1/bench/runs/batch` is idempotent by `run.id`: duplicate IDs are treated
as no-ops and do not mutate artifacts for existing runs.

The supported HTTP contract for the bench surface is `/v1/bench/*`. The checked-in
OpenAPI specs in `cmd/evidra-api/static/openapi.yaml` and `ui/public/openapi.yaml`
must describe the same live bench routes so fallback builds and UI builds expose
the same API contract.

Operational references:
- [CLI Reference](integrations/cli-reference.md)
- [End-to-End Example](system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md)
- [Self-Hosted Setup](guides/self-hosted-setup.md)
- [Signal Validation Guide](guides/signal-validation.md)
