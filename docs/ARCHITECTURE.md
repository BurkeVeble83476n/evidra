# Evidra Architecture Overview

This is the non-normative public index for the live architecture set.
The canonical design and business logic live in the versioned docs under
`docs/system-design/`.

One-sentence model:

**Evidra records automation intent, execution, and outcome across agents, pipelines, and controllers, then computes reliability signals and scorecards from an append-only evidence chain.**

## Evidence Modes

All three modes feed the same evidence chain.

- Direct full mode: MCP agent calls `prescribe_full` with `raw_artifact`, then `report`
- Direct smart mode: MCP agent calls `prescribe_smart` with lightweight target context, then `report`
- Proxy mode: evidra records mutations around an upstream MCP server without agent participation

## Hosted Mode

Hosted mode changes where evidence is collected and replayed, not what evidence means.

- CLI and MCP can keep evidence local in append-only JSONL or forward the same signed entries to `evidra-api`.
- Self-hosted also accepts webhook ingestion and controller-observed GitOps reconciliation evidence from systems such as ArgoCD, and maps those events into the same evidence model.
- `evidra-api` stores tenant evidence in Postgres and runs tenant-wide `scorecard` / `explain` over that centralized evidence.
- Deliberate refusals remain first-class evidence: `report(verdict=declined, decision_context)` is analyzed through the same signal and scoring path as any other terminal report.
- The lifecycle pair stays `prescribe_full` or `prescribe_smart`, followed by `report`; `payload.flavor` distinguishes imperative execution from reconciliation-style execution without creating a second scoring lane.

```text
Direct full MCP ----\
Direct smart MCP ----+-----> local JSONL evidence --------> local scorecard / explain
Proxy MCP ----------/                \
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

Operational references:
- [CLI Reference](integrations/cli-reference.md)
- [End-to-End Example](system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md)
- [Self-Hosted Setup](guides/self-hosted-setup.md)
- [Signal Validation Guide](guides/signal-validation.md)
