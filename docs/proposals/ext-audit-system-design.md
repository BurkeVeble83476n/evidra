# ext-audit System Design

How `ext-audit` integrates with MCP infrastructure and replaces
current workarounds.

## Current State (Without ext-audit)

```
                    Three separate capture mechanisms
                    ─────────────────────────────────

  Agent ──→ MCP Server                    Agent ──→ Gateway ──→ MCP Server
       ↑                                                ↓
  Evidra MCP Proxy                              OTLP Collector
  (stdio interception)                          (span translation)
       ↓                                                ↓
  Prescribe + Report                            Bridge ──→ Evidra
       ↓
     Evidra


  Agent ──→ MCP Server                    Each mechanism:
       ↓                                  - different wire format
  Agent-side SDK                          - different integration
  (framework-specific)                    - different data shape
       ↓                                  - different blind spots
     Vendor logger
```

**Problems:**
- Proxy mode: transport-dependent, adds latency, extra process
- OTLP bridge: requires gateway + collector + bridge + translation
- Agent-side: fragmented per framework, not interoperable
- No standard: audit consumers must support all three

## Target State (With ext-audit)

```
                    One protocol-native mechanism
                    ────────────────────────────

  Agent ──→ MCP Server (with ext-audit capability)
                ↓
         audit/event notifications on the same transport
                ↓
     ┌──────────┼──────────┐
     ↓          ↓          ↓
  Evidra    Compliance   SIEM/OTLP
  (signals  (SOC2 log)  (Datadog,
   + score)              Splunk)
```

**Benefits:**
- One event format, any consumer
- No proxy, no bridge, no collector needed
- Capability negotiated at init — zero config for the agent
- Privacy by default (argument digests, not raw args)
- Behavioral analysis enabled by session + sequence

## Integration Patterns

### Pattern 1: Direct Consumption

MCP server emits `ext-audit` events. Evidra subscribes as a client
or receives forwarded notifications.

```
Agent ──→ MCP Server
              │
              ├─→ tool result (to agent)
              └─→ audit/event (to audit consumer)
                       │
                    Evidra API
                    POST /v1/evidence/ingest/audit
```

New Evidra endpoint: `POST /v1/evidence/ingest/audit` accepts
`ext-audit` event payloads directly. Maps to internal prescribe +
report entries:

| ext-audit field | Evidra evidence field |
|----------------|----------------------|
| `intent.tool` | `canonical_action.tool` |
| `intent.operation` | `canonical_action.operation` |
| `intent.operation_class` | `canonical_action.operation_class` |
| `intent.scope_class` | `canonical_action.scope_class` |
| `intent.arguments_digest` | `artifact_digest` |
| `outcome.verdict` | `report.verdict` |
| `outcome.exit_code` | `report.exit_code` |
| `session_id` | `session_id` |
| `sequence` | `operation_id` (derived) |
| `actor` | `actor` |
| `risk.level` | `effective_risk` |
| `trace.*` | `trace_id`, `span_id` |

One `ext-audit` event maps to one prescribe + one report entry in
Evidra's evidence chain. The signal detectors and scoring pipeline
work unchanged.

### Pattern 2: Gateway Forwarding

Gateway negotiates `ext-audit` with the upstream MCP server and
forwards audit events to Evidra.

```
Agent ──→ Gateway ──→ MCP Server
              │              │
              │         audit/event
              │              │
              ├──────────────┘
              │
              ↓
         Evidra API
```

This replaces the current OTLP bridge path entirely. The gateway
forwards the protocol-native audit event instead of translating
OTLP spans.

### Pattern 3: Proxy Injection

For MCP servers that don't support `ext-audit`, a proxy can generate
audit events from observed traffic:

```
Agent ──→ Evidra Proxy ──→ MCP Server
              │
              ├─→ synthesize audit/event from tools/call
              └─→ forward to Evidra
```

This is the current Evidra MCP proxy mode, but producing `ext-audit`
format events instead of the proprietary prescribe/report format.
The proxy becomes a compatibility shim, not the primary path.

### Pattern 4: SDK Emission

Agent frameworks (LangGraph, ADK, Claude Code) implement `ext-audit`
natively. Every MCP tool call automatically generates an audit event.

```
Agent (with ext-audit SDK) ──→ MCP Server
         │
    audit/event
         │
      Evidra
```

This is the long-term goal. Framework authors add one integration
(ext-audit), not per-tool audit code.

## What Evidra Gains

### Eliminates
- OTLP bridge (`evidra-agentgateway-bridge` repo)
- OTel Collector as a protocol converter
- Transport-specific proxy interception logic
- Custom OTLP span attribute mapping

### Simplifies
- One ingest path for all MCP audit data
- One event format to validate and store
- One set of field names for signal detection

### Enables
- Any MCP server can emit evidence without Evidra-specific code
- Any audit consumer can ingest MCP audit events
- Interoperability between Evidra and other audit tools
- Standardized behavioral signal definitions

## What Evidra Keeps

The ext-audit extension standardizes **capture**. Evidra's value is
in what happens after capture:

| Layer | ext-audit provides | Evidra provides |
|-------|-------------------|-----------------|
| Capture | Structured audit events | — (consumes events) |
| Storage | — | Cryptographically chained evidence |
| Detection | — | 8 behavioral signal detectors |
| Scoring | — | Weighted reliability scoring |
| Benchmarking | — | Run comparison, leaderboards |
| Assessment | Risk level field (from server) | Pluggable assessment pipeline |

## Migration Path

### Phase 1: Evidra as Reference Consumer

- Implement `POST /v1/evidence/ingest/audit` endpoint
- Map ext-audit events to existing evidence format
- Keep bridge/proxy as fallback for non-ext-audit servers

### Phase 2: MCP Proxy Emits ext-audit

- Evidra's MCP proxy generates ext-audit notifications
- Downstream audit consumers can subscribe
- Proxy becomes a compatibility adapter

### Phase 3: Native Server Support

- Contribute ext-audit support to major MCP server SDKs
- Servers emit audit events natively
- Bridge and proxy become optional, not required

### Phase 4: Deprecate Bridge

- Once major servers support ext-audit, the bridge is obsolete
- Evidra consumes ext-audit events directly
- One fewer component, one fewer repo

## Event Flow Example

Agent calls `kubectl apply` through an MCP server with `ext-audit`:

```
1. Agent sends tools/call:
   {"method": "tools/call", "params": {"name": "apply_yaml", ...}}

2. MCP server executes the tool call.

3. MCP server emits audit/event notification:
   {
     "method": "audit/event",
     "params": {
       "type": "tool_call",
       "timestamp": "2026-03-21T14:30:00.123Z",
       "session_id": "sess_abc",
       "sequence": 3,
       "intent": {
         "tool": "apply_yaml",
         "operation": "apply",
         "resource": "deployment/web",
         "operation_class": "mutate",
         "scope_class": "production",
         "arguments_digest": "sha256:def456..."
       },
       "outcome": {
         "verdict": "success",
         "exit_code": 0,
         "duration_ms": 1200
       },
       "actor": {
         "type": "agent",
         "id": "claude-code"
       },
       "risk": {
         "level": "high"
       }
     }
   }

4. MCP server returns tool result to agent (normal flow).

5. Evidra receives the audit/event (via transport or forwarding):
   - Creates prescribe entry: tool=apply_yaml, op_class=mutate,
     scope=production, risk=high
   - Creates report entry: verdict=success, exit_code=0
   - Links them: same session_id + sequence

6. After the session, Evidra computes:
   - Signal detection: no retry_loop, no protocol_violation
   - Scorecard: score=95, band=excellent
```

## Alignment with MCP Roadmap

| Roadmap Item | ext-audit Addresses |
|-------------|-------------------|
| "Audit trails and observability" | Directly — this IS the audit trail format |
| "Gateway and proxy patterns" | Defines how audit events flow through intermediaries |
| "Configuration portability" | Audit events work across any MCP transport |
| OTel MCP semantic conventions | Trace fields align with W3C Trace Context |

## Open Questions

1. **Should audit events be opt-in per tool call, or always-on when
   negotiated?** Current proposal: always-on. Selective auditing adds
   complexity and creates gaps.

2. **Should the server compute `operation_class` and `risk.level`, or
   should that be the consumer's job?** Current proposal: server MAY
   include them; consumer computes if absent. Keeps the server simple.

3. **Should there be a separate `audit/subscribe` method, or use the
   existing notification channel?** Current proposal: existing
   notification channel. Simpler, no new lifecycle to manage.

4. **Should session_id be a new concept or reuse the MCP session ID?**
   Current proposal: new field. MCP session ID is a transport concern;
   audit session_id groups logical operations (one agent task may span
   multiple MCP sessions).
