# Evidra Clean Architecture Design

**Date:** 2026-03-21
**Status:** Approved

## Problem

The current codebase mixes recording and intelligence concerns.
Canonicalization, risk matrix lookup, detector execution, and SARIF
parsing are all baked into one monolithic function
(`buildPrescribeRiskState()` in `internal/lifecycle/service.go`). External
integrations (gateway ext-authz, OPA, scanners) have no clean entry point.
The ingest layer in `internal/ingest/service.go` duplicates parts of this
logic for the API path.

This must be clean before we can demo it, document it, or integrate with
gateways.

## Target Architecture

```
                        RECORDER
                        ────────

  Prescribe Request ──→ Assessment Pipeline ──→ Store (sign, chain, persist)
                             │
                        ┌────┴────────────────────────┐
                        │  Phase 1: Canonicalize       │
                        │    raw artifact OR smart     │
                        │    target → CanonicalAction  │
                        │                              │
                        │  Phase 2: Assess (pluggable) │
                        │    assessor 1: matrix        │ ← native
                        │    assessor 2: detectors     │ ← native
                        │    assessor 3: SARIF         │ ← external
                        │    assessor 4: ext-authz     │ ← future
                        │    assessor 5: OPA           │ ← future
                        │    → risk_inputs[]           │
                        │                              │
                        │  Phase 3: Aggregate          │
                        │    effective_risk = max(all)  │
                        └─────────────────────────────┘
                             │
                             ▼
  Response: { prescription_id, effective_risk, risk_inputs[] }


  Report Request ──→ Store (sign, chain, persist)

  Observed (bridge/proxy) ──→ Store (no assessment, passive)


                      INTELLIGENCE
                      ────────────

  Evidence Store ──→ Signal Detectors (8 behavioral)
                 ──→ Scoring (weighted penalty → 0-100)
                 ──→ Benchmarking (run comparison, leaderboards)
                 ──→ Analytics (scorecards, explain, trends)
```

## Core Interface

```go
// Assessor evaluates a canonical action and returns risk inputs.
type Assessor interface {
    Assess(ctx context.Context, action CanonicalAction, raw []byte) ([]RiskInput, error)
}
```

Every assessor contributes zero or more `RiskInput` entries. The pipeline
runs all assessors and merges results. `effective_risk` is the max severity
across all inputs.

## Built-in Assessors

### MatrixAssessor
Replaces the inline `risk.RiskLevel()` call. Static 2D lookup:
`operationClass × scopeClass → riskLevel`. Always runs.

### DetectorAssessor
Replaces the inline `detectors.ProduceAll()` call. Runs the detector
registry against the canonical action and raw artifact bytes. Returns
risk_input with `source: "evidra/native"` and fired tags.

### SARIFAssessor (optional)
Replaces the inline `buildSARIFRiskInput()`. Accepts external SARIF
findings and converts them to risk_inputs. Only runs when findings are
present in the request.

## Future Assessors (Not Implemented Now)

### ExtAuthzAssessor
Receives risk context from gateway ext-authz requests. The gateway calls
Evidra's assessment endpoint; this assessor translates the gateway's policy
decision into a risk_input.

### WebhookAssessor
Calls an external webhook (OPA, custom scanner) with the canonical action,
receives a risk_input back.

## Gateway Integration (Pattern C)

```
Agent → Gateway → ext-authz → Evidra Assessment Endpoint
                                  │
                                  ├─ runs assessment pipeline
                                  ├─ stores prescribe entry
                                  ├─ returns { prescription_id, effective_risk }
                                  │
                              Gateway gets risk level
                              Gateway forwards to MCP server
                                  │
                              OTLP traces → bridge → Evidra report
```

Evidra is an **assessment service** that gateways consult — not a
competing gateway. The gateway stays in control. Evidra assesses and
records.

This works with any gateway supporting ext-authz: AgentGateway (Envoy
gRPC + HTTP), Istio, Kong, any Envoy-based proxy.

## Three Observation Modes, One Pipeline

| Mode | How Evidra connects | Prescribe? | Assessment? |
|------|-------------------|-----------|------------|
| **MCP proxy** | Wraps MCP stdio | Yes — native | Full pipeline |
| **Ext-authz** | Gateway calls Evidra | Yes — via gateway | Full pipeline |
| **Bridge/OTLP** | Taps telemetry | No — observed only | Post-hoc only |

All three produce evidence entries in the same format. All three feed the
same intelligence layer (signals, scoring, benchmarking).

## What Changes

### New package: `internal/assess/`
- `Pipeline` — orchestrates canonicalization + assessors + aggregation
- `Assessor` interface
- `MatrixAssessor`, `DetectorAssessor`, `SARIFAssessor` implementations
- `Aggregate()` — computes effective_risk from risk_inputs[]

### Refactored: `internal/lifecycle/service.go`
- `buildPrescribeRiskState()` replaced by `assess.Pipeline.Run()`
- Prescribe calls the pipeline, gets back risk_inputs + effective_risk
- Clean separation: lifecycle handles evidence construction, pipeline
  handles assessment

### Refactored: `internal/ingest/service.go`
- Server-side prescribe delegates to the same `assess.Pipeline`
- Eliminates duplicated risk logic between CLI and API paths

### Unchanged
- `internal/risk/matrix.go` — extracted into MatrixAssessor, logic same
- `internal/detectors/` — extracted into DetectorAssessor, registry same
- `internal/canon/` — called by pipeline Phase 1, interface unchanged
- `internal/signal/` — post-ingest, untouched
- `internal/score/` — post-ingest, untouched
- `internal/analytics/` — post-ingest, untouched
- `pkg/evidence/` — evidence types unchanged
- `pkg/bench/` — benchmarking unchanged

## What Does NOT Change

- Evidence entry format (same fields, same payloads)
- `risk_inputs[]` contract (same structure)
- Prescribe/report MCP tools (same interface)
- API endpoints (same request/response shapes)
- Signal detectors (same post-hoc pipeline)
- Scoring (same weighted penalty model)

## Implementation Order

1. Create `internal/assess/` with Pipeline, Assessor interface, and
   three built-in assessors (Matrix, Detector, SARIF)
2. Wire Pipeline into `internal/lifecycle/service.go` replacing
   `buildPrescribeRiskState()`
3. Wire Pipeline into `internal/ingest/service.go` replacing
   duplicated risk logic
4. Verify all existing tests pass (same behavior, cleaner structure)
5. Update ARCHITECTURE.md with the clean split diagram
6. Then: rework demo to use the clean architecture
7. Then: add ext-authz endpoint for gateway integration

## Black Box Principle

Evidra is a recorder with pluggable pre-flight assessment. It never
blocks execution. The assessment pipeline enriches the evidence (intent +
risk), but the agent decides what to do with the risk level. Recording
happens regardless of assessment outcome.

Observed-only evidence (bridge/proxy tap) skips assessment entirely —
it's still valid evidence, just less rich than prescribed evidence.
Both feed the same scoring pipeline.
