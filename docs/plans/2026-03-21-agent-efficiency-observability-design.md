# Agent Efficiency Observability Design

**Date:** 2026-03-21
**Status:** Approved — Phase 1 implemented in bridge

## Principle: Black Box

Evidra is a wire tap, not a protocol participant. It reads data from buses
that already exist. It never asks agents to change behavior or integrate a
new SDK.

```
Airplane                          Agent Infrastructure
─────────────────────────────────────────────────────
Flight data recorder              Evidra
  taps into avionics bus            taps into OTLP / MCP stdio
  no pilot interaction              no agent changes
  records passively                 records passively
  analyzed after the fact           scorecard + signals
```

## The Data Buses

| Bus | What flows | How Evidra taps | Status |
|-----|-----------|----------------|--------|
| MCP stdio | tool calls + results | Proxy (wraps wire) | Shipped |
| OTLP traces | spans with tool/session/status + gen_ai.usage.* | Bridge (translates) | Shipped |
| A2A messages | task lifecycle, turns, agent identity | Bridge (future) | Design |
| LLM API | prompts, completions, tokens, cost | Via OTLP (AgentGateway emits) | Phase 1 done |

## What AgentGateway Already Emits on OTLP

```
gen_ai.usage.prompt_tokens       ← tokens in
gen_ai.usage.completion_tokens   ← tokens out
gen_ai.usage.total_tokens        ← total
gen_ai.request.model             ← model name
gen_ai.response.model            ← actual model used
gen_ai.operation.name            ← "chat", etc.
mcp.session.id                   ← session boundary
mcp.method.name                  ← tools/call
gen_ai.tool.name / mcp.tool.name ← which tool
```

## Phase 1: Token Metrics (Implemented)

**Change: bridge only, zero Evidra changes.**

The bridge already reads `mcp.*` attributes from OTLP spans to produce
prescribe/report ingest calls. Phase 1 extends it to also read
`gen_ai.usage.*` attributes from the same spans and pass them as
`scope_dimensions` on the ingest requests.

```
OTLP span → bridge normalize → {action, outcome} with GenAIUsage
         → bridge processor → prescribe/report with scope_dimensions:
             gen_ai.model: "qwen-plus"
             gen_ai.prompt_tokens: "1500"
             gen_ai.completion_tokens: "350"
             gen_ai.total_tokens: "1850"
         → Evidra API → stored on evidence entries
```

Evidence entries now carry LLM efficiency data alongside reliability data.
Queryable via `GET /v1/evidence/entries`. Aggregatable in scorecards and
bench runs.

## Phase 2: Efficiency Signals (Future)

New signal detectors that fire on efficiency patterns:

| Signal | What it catches | How |
|--------|----------------|-----|
| `token_waste` | High token usage relative to operation complexity | Compare tokens per prescribe/report pair to operation_class baseline |
| `turn_inflation` | Too many turns for simple operations | Count prescribe entries per session vs. operation complexity |
| `cost_escalation` | Cost increasing across retries | Track cumulative tokens across retry_loop sequences |

These would plug into the existing signal detector pipeline
(`internal/signal/`) and contribute to the reliability scorecard.

## Phase 3: A2A Task Observability (Future)

If AgentGateway emits A2A task lifecycle as OTLP spans (task created →
working → completed/failed), the bridge reads those too — same tap, same
pattern.

```
Layer 1 (shipped):   mcp.method + mcp.tool       → prescribe/report
Layer 2 (phase 1):   gen_ai.usage.*               → token/cost metrics
Layer 3 (future):    A2A task spans (if AG emits)  → session boundaries + turn count
```

If AgentGateway does not emit A2A lifecycle as OTLP, the bridge could add
a lightweight A2A task subscriber that watches task updates via streaming
and produces equivalent events. The bridge remains the translation layer;
Evidra remains the recorder.

A2A protocol context:
- A2A and MCP are both under Linux Foundation's Agentic AI Foundation
- A2A spec has `metadata` on Task objects (token usage actively discussed)
- A2A Traceability Extension already collects latency, cost, token usage
- LangGraph supports A2A natively (langgraph-api>=0.4.21)
- AgentGateway already routes A2A traffic

## What Evidra Gets for Free

| Metric | How | Available |
|--------|-----|-----------|
| Token counts per operation | `scope_dimensions` on evidence entries | Now (Phase 1) |
| Model name per operation | `scope_dimensions` on evidence entries | Now (Phase 1) |
| Turn count per session | Count prescribe entries per session_id | Now (derived) |
| Session duration | First/last entry timestamps | Now (derived) |
| Cost per operation | tokens × model pricing lookup | Future |
| Cost per session | Sum of operation costs | Future |

## Key Constraint

No new Evidra protocols. No agent changes. No SDK. No new ingest
endpoints. Everything flows through existing `POST /v1/evidence/ingest/*`
using `scope_dimensions` — a free-form `map[string]string` that already
exists on every evidence entry.

The bridge is the only component that changes. Evidra stores what it
receives. The airplane black box doesn't need new wiring — it just decodes
more channels from the same data bus.

## MCP Roadmap Alignment (Attention Required)

The MCP protocol roadmap (updated 2026-03-05) explicitly lists as
Priority #4 "Enterprise Readiness":

- **Audit trails and observability** — "end-to-end visibility into what a
  client requested and what a server did, in a form enterprises can feed
  into existing logging and compliance pipelines"
- **Gateway and proxy patterns** — "well-defined behavior when routing
  through an intermediary"

An Enterprise WG is forming to own this. Output will likely land as
extensions rather than core spec changes.

Additionally, OpenTelemetry MCP semantic conventions merged in January
2026, standardizing attribute names for MCP tool invocations:
`mcp.method.name`, `mcp.session.id`, `gen_ai.tool.name`,
`gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`.

**What this means for Evidra:**

MCP is heading toward Evidra's territory. This is validation — the
ecosystem agrees that audit trails and observability for agent tool calls
matter. But it's also a signal: once gateways add "audit logging"
checkboxes, the bar moves up.

**Evidra's moat is not data capture — it's judgment.** Gateways will log
what happened. Compliance platforms will store it for auditors. Evidra is
the only thing that chains tool calls into cryptographically linked
evidence, runs behavioral signal detectors (retry loops, blast radius,
repair escalation, protocol violations), computes a reliability score, and
compares runs.

The standard audit format will be table stakes. The signal detection +
scoring + comparison is the differentiator.

**Action items:**

1. Track the MCP Enterprise WG formation closely. Evidra should be
   compatible with whatever audit format they standardize — ideally Evidra
   can ingest it natively.
2. Align the bridge's OTLP attribute reading with the official OTel MCP
   semantic conventions (`mcp.method.name`, `gen_ai.tool.name`, etc.)
   rather than ad-hoc AgentGateway attribute variations. The bridge
   already reads most of these but supports alternative attribute names
   as fallbacks.
3. Consider contributing to the Enterprise WG. Evidra's prescribe/report
   lifecycle is a concrete implementation of "what a client requested and
   what a server did" — the exact problem they're scoping.

## Competitive Landscape

| Category | Products | What they do | Evidra's edge |
|----------|----------|-------------|--------------|
| **Gateways** | AgentGateway, Portkey, Cloudflare MCP Portals | Route traffic, emit telemetry | They're the wire. Evidra reads the wire and adds judgment. |
| **Security** | Lasso Security | Pre-execution threat detection, tool reputation scoring | Pre-execution only. No post-execution evidence chain or behavioral signals. |
| **Compliance** | MintMCP, Lunar.dev MCPX | SOC2/HIPAA audit logs, RBAC | Store what happened for auditors. No behavioral analysis or scoring. |
| **Observability** | Langfuse, Datadog (via OTel) | Traces, spans, dashboards | Show telemetry. Don't chain evidence, detect signals, or score reliability. |
| **Evidra** | — | Evidence chain + signal detection + scoring + comparison | The only platform that answers: "should you trust this agent in production?" |

Nobody else combines cryptographically linked evidence chains with
behavioral signal detection and reliability scoring. The closest is Lasso
Security with tool reputation scoring, but that's pre-execution risk
assessment — not post-execution evidence and behavioral analysis.

**Risk:** If the MCP Enterprise WG standardizes an audit format and
gateways implement it, "audit logging" becomes commoditized. Evidra must
stay ahead on the judgment layer: signals, scores, and comparisons that
turn raw audit data into actionable trust decisions.
