# Evidra Architecture Overview

This is the non-normative public index for the live architecture set.
The canonical design and business logic live in the versioned docs under
`docs/system-design/`.

One-sentence model:

**Evidra records automation intent, execution, and outcome across agents, pipelines, and controllers, then computes reliability signals and scorecards from an append-only evidence chain.**

## Evidence Modes

All three modes feed the same evidence chain.

- Full Prescribe: MCP agent calls `prescribe_full` with `raw_artifact`, then `report`
- Smart Prescribe: MCP agent calls `prescribe_smart` with lightweight target context, then `report`
- Proxy Observed: evidra records mutations around an upstream MCP server without agent participation

## Hosted Mode

Hosted mode changes where evidence is collected and replayed, not what evidence means.

- CLI and MCP can keep evidence local in append-only JSONL or forward the same signed entries to `evidra-api`.
- Self-hosted also accepts raw `/v1/evidence/forward` and `/v1/evidence/batch` transport, plus typed `/v1/evidence/ingest/prescribe` and `/v1/evidence/ingest/report` lifecycle ingest for external adapters.
- Self-hosted also accepts webhook ingestion and controller-observed GitOps reconciliation evidence from systems such as ArgoCD, and maps those events into the same evidence model. Webhook routes are compatibility wrappers over the shared lifecycle ingest service.
- `evidra-api` stores tenant evidence in Postgres and runs tenant-wide `scorecard` / `explain` over that centralized evidence.
- Deliberate refusals remain first-class evidence: `report(verdict=declined, decision_context)` is analyzed through the same signal and scoring path as any other terminal report.
- The lifecycle pair stays `prescribe_full` or `prescribe_smart`, followed by `report`; the external ingest request contract uses `flavor`, `evidence.kind`, and `source.system` to describe execution shape and ingestion source without creating a second scoring lane. Persisted entries expose the same context as `payload.flavor`, `payload.evidence.kind`, and `payload.source.system`. `flavor` includes `imperative`, `reconcile`, and `workflow`; `evidence.kind` includes `declared`, `observed`, and `translated`.

```text
Full Prescribe -----\
Smart Prescribe ----+-----> local JSONL evidence --------> local scorecard / explain
Proxy Observed -----/                \
CLI ----------------------------------\ forward evidence
                                       v
                                evidra-api <----- webhook ingestion + GitOps controller evidence
                                     |
                                     v
                               Postgres evidence store --------> hosted scorecard / explain
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

**Public types:** `pkg/bench/` — RunRecord, RunFilters, BenchStore interface, timeline parser
**Private implementation:** `internal/benchsvc/` — pgx store, HTTP handlers, JSONL import
**Database:** `bench_runs`, `bench_artifacts`, `bench_scenarios` tables (migration 006)
**UI:** `ui/src/pages/bench/` — Leaderboard, Dashboard, Runs, RunDetail

The bench layer is self-contained — extractable to its own microservice via `cmd/bench-api/`.

Operational references:
- [CLI Reference](integrations/cli-reference.md)
- [End-to-End Example](system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md)
- [Self-Hosted Setup](guides/self-hosted-setup.md)
- [Signal Validation Guide](guides/signal-validation.md)
