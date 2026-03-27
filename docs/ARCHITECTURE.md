# Evidra Architecture Overview

This is the non-normative public index for the live architecture set.
The canonical design and business logic live in the versioned docs under
`docs/system-design/` and `docs/contracts/`.

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
- Canonicalize: adapter translates raw artifact into CanonicalAction
- Assessment pipeline: pluggable assessors → risk_inputs[] → effective_risk
- Sign with Ed25519, chain via previous_hash, store

**Intelligence** (read path, post-hoc):
- Signal detection: 8 behavioral detectors across evidence sequences
- Scoring: weighted penalty model → 0-100 reliability metric
- Benchmarking: run comparison, leaderboards, model evaluation
- Analytics: scorecards, explain, trends

## Observation Modes

All modes produce the same evidence entries and feed the same intelligence
pipeline.
All three modes feed the same evidence chain.
At the MCP layer, Full Prescribe, Smart Prescribe, and Proxy Observed are different entry modes.

| Mode | How Evidra connects | Prescribe | Assessment |
|------|-------------------|-----------|------------|
| **MCP direct** | Agent usually calls `run_command`; explicit `prescribe_*` + `report` remain available | Yes — direct path or explicit protocol | Full pipeline |
| **MCP proxy** | `evidra-mcp --proxy` wraps upstream MCP server, classifies `run_command` and mutation-style `tools/call` requests heuristically | Implicit | Observed only |
| **OTLP bridge** | Reads AgentGateway OTLP traces, translates to prescribe/report | Implicit | Observed only |
| **Webhooks** | ArgoCD/generic webhook → mapped prescribe/report | Translated | Full pipeline |
| **Ext-authz** (future) | Gateway calls Evidra assessment endpoint before forwarding | Yes — via gateway | Full pipeline |

MCP direct gives the richest evidence. In the default direct surface, `run_command` remains the primary cheap-model path. `prescribe_smart` and `report` stay available, but their full schemas are loaded on demand via `describe_tool` so the default tool surface stays smaller. Proxy and bridge are passive taps. Proxy observation is heuristic: it records `run_command` and generic mutation-style MCP tool names it can classify, but it does not build a full upstream tool catalog.
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

## Bench Execution

Evidra supports two benchmark execution paths:

| Mode | When | How |
|------|------|-----|
| **Direct executor** | Default local flow, or `EVIDRA_BENCH_SERVICE_URL` is set | `POST /v1/bench/trigger` starts a `RunExecutor` immediately |
| **Poll-based runners** | Runner dispatcher enabled and at least one healthy runner is registered | `POST /v1/bench/trigger` enqueues a persisted job; runners claim it via `/v1/runners/jobs` |

### Direct Executor Flow

| Executor | When | How |
|----------|------|-----|
| **LocalExecutor** | Default (OSS) | Basic scenario execution via kubectl |
| **RemoteExecutor** | `EVIDRA_BENCH_SERVICE_URL` set | Delegates to external REST service |

LocalExecutor provides basic trigger flow. Full scenario orchestration (seed, agent, verify) requires RemoteExecutor with an external bench service.

```text
POST /v1/bench/trigger { model, provider?, execution_mode?, evidence_mode, scenarios[] }
        ↓
  RunExecutor.Start()
        ↓
  Evidence → POST /v1/evidence/forward
  Bench runs → POST /v1/bench/runs
  Progress → POST /v1/bench/trigger/{id}/progress
        ↓
  UI polls GET /v1/bench/trigger/{id}
```

The direct-executor contract (v1.0.0) is an open specification. Third-party
executors can implement it to plug into Evidra's analytics.
See [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md).

### Poll-Based Runner Flow

The runner control plane persists execution in PostgreSQL:

- `bench_infra` stores registered runners and their advertised capabilities
- `bench_jobs` stores queued/running/completed jobs and liveness timestamps
- `last_progress_at` allows stale claimed jobs to be re-queued if a runner stops reporting

```text
POST /v1/bench/trigger { model, provider?, runner_id?, execution_mode?, evidence_mode, scenarios[] }
        ↓
  bench_jobs row inserted with status=queued
        ↓
  Runner registers capabilities → POST /v1/runners/register
        ↓
  Runner heartbeat + poll → GET /v1/runners/jobs?runner_id=...
        ↓
  Claim next matching job via SELECT ... FOR UPDATE SKIP LOCKED
        ↓
  Optional trigger compatibility progress → POST /v1/bench/trigger/{id}/progress
        ↓
  Final ownership-checked completion → POST /v1/runners/jobs/{id}/complete
        ↓
  UI polls GET /v1/bench/trigger/{id}
```

Control-plane invariants:

- `evidence_mode` on `POST /v1/bench/trigger` is required and limited to `none|smart`
- only healthy runners can poll and claim work
- `runner_id` on `POST /v1/bench/trigger` pins a job to one specific runner
- runner poll payloads include the requested `evidence_mode`
- a runner can only complete a job it currently owns
- the janitor marks silent runners unhealthy and re-queues stale claimed jobs

Public dashboard filters use the coarse `All | Baseline | Evidra` aliases
(`all|none|evidra`). Exact-match stored subtypes such as `proxy`, `direct`,
and `mcp` stay internal until the advanced filter story is documented.

See [Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md).

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
                       ├──▸ canonicalize ──▸ assess.Pipeline ──▸ sign ──▸ store ──▸ signals    │
  OTLP bridge ─────────┤                     (assess risk,     chain      │     scoring     │
  Webhooks ────────────┤                      aggregate)                  │     benchmarks  │
  Ext-authz (future) ──┘                                                  │     analytics   │
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
- [Canonicalization Contract](contracts/EVIDRA_CANONICALIZATION_CONTRACT_V1.md)
- [Signal Spec](system-design/EVIDRA_SIGNAL_SPEC_V1.md)
- [Executor Contract](contracts/EXECUTOR_CONTRACT_V1.md)
- [Bench Runner Control Plane Contract](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)

System design and implementation mapping:
- [Architecture](system-design/EVIDRA_ARCHITECTURE_V1.md)
- [Record/Import Contract](contracts/EVIDRA_RUN_RECORD_CONTRACT_V1.md)
- [Default Scoring Profile](system-design/scoring/default.v1.1.0.md)

### Bench Intelligence Layer

Infrastructure agent benchmark results and analytics.

**Public types:** `pkg/bench/` — RunRecord, RunFilters, timeline parser
**Private implementation:** `internal/benchsvc/` — request-scoped `Service`, internal `Repository` contract, `PgStore`, HTTP handlers, JSONL import
**Database:** `bench_runs`, `bench_artifacts`, `bench_scenarios`, `bench_jobs`, `bench_infra`
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

Trigger-originated jobs use two compatibility layers:

- persisted queue state in `bench_jobs`
- in-memory `TriggerStore` state for `/v1/bench/trigger/{id}` polling and SSE

This keeps the existing bench dashboard contract stable while allowing direct
executor mode and poll-based runner mode to share one trigger surface.

The supported HTTP contract for the bench surface is `/v1/bench/*`. The checked-in
OpenAPI specs in `cmd/evidra-api/static/openapi.yaml` and `ui/public/openapi.yaml`
must describe the same live bench routes so fallback builds and UI builds expose
the same API contract.

Operational references:
- [CLI Reference](integrations/cli-reference.md)
- [End-to-End Example](system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md)
- [Self-Hosted Setup](guides/self-hosted-setup.md)
- [Signal Validation Guide](guides/signal-validation.md)
