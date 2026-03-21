# SEP-0000: `ext-audit` — Operational Audit Trail Extension for MCP

- **Author:** Vitaliy Ryumshyn (@vitas)
- **Status:** Pre-draft (not yet submitted)
- **Type:** Standards Track
- **Created:** 2026-03-21

## Abstract

This SEP proposes `ext-audit`, an MCP extension that adds structured
operational audit events to the tool call lifecycle. When negotiated,
servers emit machine-readable audit records for every `tools/call`
request and response, enabling downstream systems to build evidence
chains, detect behavioral patterns, and compute reliability metrics
without modifying agent code.

The extension is strictly additive. Servers and clients that do not
negotiate `ext-audit` are unaffected.

## Motivation

The MCP 2026 roadmap identifies "audit trails and observability" as a
Priority #4 Enterprise Readiness area:

> "end-to-end visibility into what a client requested and what a server
> did, in a form enterprises can feed into their existing logging and
> compliance pipelines"

Today, achieving this requires one of:

1. **MCP proxy interception** — a wrapper process that decodes JSON-RPC
   stdio or HTTP traffic, extracts tool calls, and forwards evidence to
   an external system. Fragile, transport-dependent, and invisible to
   the protocol.

2. **Gateway OTLP telemetry** — an intermediary (AgentGateway, Envoy)
   that emits OpenTelemetry spans for each request. Rich but requires
   external infrastructure, and the span attributes are not standardized
   for MCP semantics.

3. **Agent-side instrumentation** — each agent framework (LangGraph,
   ADK, Claude Code) builds its own audit logging. Fragmented,
   inconsistent, and not interoperable.

None of these approaches produce a **protocol-native** audit record that
any MCP client or server can emit and any audit consumer can ingest
without custom integration.

### What's Missing

- No standard format for "what tool was called with what arguments and
  what happened"
- No capability negotiation — audit consumers can't discover whether a
  server supports audit events
- No session-level correlation — individual tool calls lack a standard
  grouping mechanism for behavioral analysis
- No risk or classification metadata — audit records carry no semantic
  context about the operation type (mutate/destroy/read) or scope

### Who Benefits

- **Platform teams** deploying AI agents in production need audit trails
  for compliance, incident response, and operational review
- **Agent framework authors** want a standard way to emit audit events
  without building custom integrations per observability tool
- **Gateway operators** (AgentGateway, Envoy, Kong) need a standard
  audit event to forward, not tool-specific OTLP attributes
- **Observability tools** want to consume structured MCP audit events
  without parsing raw JSON-RPC traffic

## Specification

### Extension Identifier

```
ext-audit
```

### Capability Negotiation

During `initialize`, the server declares `ext-audit` support:

```json
{
  "capabilities": {
    "experimental": {
      "ext-audit": {
        "version": "0.1.0",
        "events": ["tool_call"]
      }
    }
  }
}
```

The client may declare audit consumption preferences:

```json
{
  "capabilities": {
    "experimental": {
      "ext-audit": {
        "subscribe": true
      }
    }
  }
}
```

### Audit Event Format

When `ext-audit` is negotiated, the server emits audit events as
**notifications** on the MCP transport. The notification method is
`audit/event`.

#### Tool Call Audit Event

Emitted after a `tools/call` completes (success or failure):

```json
{
  "jsonrpc": "2.0",
  "method": "audit/event",
  "params": {
    "type": "tool_call",
    "timestamp": "2026-03-21T14:30:00.123Z",
    "session_id": "sess_01HXY...",
    "sequence": 7,

    "intent": {
      "tool": "kubectl",
      "operation": "apply",
      "resource": "deployment/web",
      "namespace": "production",
      "operation_class": "mutate",
      "scope_class": "production",
      "arguments_digest": "sha256:abc123..."
    },

    "outcome": {
      "verdict": "success",
      "exit_code": 0,
      "duration_ms": 1250,
      "error": null
    },

    "actor": {
      "type": "agent",
      "id": "claude-code",
      "version": "1.2.0"
    },

    "risk": {
      "level": "high",
      "sources": [
        {
          "source": "matrix",
          "level": "high"
        }
      ]
    },

    "trace": {
      "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
      "span_id": "00f067aa0ba902b7",
      "parent_span_id": "b3dc23a1e0c2f7d8"
    }
  }
}
```

#### Field Definitions

**Required fields:**

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Event type. `"tool_call"` for tool invocations. |
| `timestamp` | string | ISO 8601 timestamp with millisecond precision. |
| `session_id` | string | Groups related tool calls into a session. |
| `sequence` | integer | Monotonically increasing counter within the session. |
| `intent.tool` | string | Tool name as registered in `tools/list`. |
| `outcome.verdict` | string | One of `"success"`, `"failure"`, `"error"`, `"declined"`. |
| `actor.type` | string | Actor classification: `"agent"`, `"human"`, `"system"`. |
| `actor.id` | string | Stable actor identifier. |

**Optional fields:**

| Field | Type | Description |
|-------|------|-------------|
| `intent.operation` | string | Operation verb (e.g., `"apply"`, `"delete"`). |
| `intent.resource` | string | Target resource identifier. |
| `intent.namespace` | string | Scope qualifier. |
| `intent.operation_class` | string | Classification: `"mutate"`, `"destroy"`, `"read"`, `"plan"`. |
| `intent.scope_class` | string | Environment: `"production"`, `"staging"`, `"development"`. |
| `intent.arguments_digest` | string | SHA-256 digest of the tool arguments (not the arguments themselves). |
| `outcome.exit_code` | integer | Numeric exit code when applicable. |
| `outcome.duration_ms` | integer | Execution duration in milliseconds. |
| `outcome.error` | string | Error message for failed calls. |
| `actor.version` | string | Actor software version. |
| `risk.level` | string | Aggregated risk: `"low"`, `"medium"`, `"high"`, `"critical"`. |
| `risk.sources` | array | Per-source risk assessments. |
| `trace.trace_id` | string | W3C Trace Context trace ID (hex). |
| `trace.span_id` | string | W3C Trace Context span ID (hex). |
| `trace.parent_span_id` | string | Parent span for distributed tracing. |

### Declined Operations

When an agent intentionally refuses to execute a tool call, the audit
event records the refusal as evidence:

```json
{
  "outcome": {
    "verdict": "declined",
    "decision_context": {
      "trigger": "risk_threshold_exceeded",
      "reason": "privileged container in production namespace"
    }
  }
}
```

Declined operations are first-class audit records, not silent gaps.

### Session Correlation

The `session_id` field groups tool calls from a single agent task or
CI pipeline run. The `sequence` field provides ordering within the
session. Together they enable behavioral pattern detection:

- Retry loops: same `intent.arguments_digest` repeated after failure
- Protocol violations: unexpected sequences
- Blast radius: multiple distinct resources mutated in one session
- Repair loops: failure followed by a different operation on the same
  resource

### Arguments Privacy

The extension transmits `arguments_digest` (a hash), not the raw
arguments. This allows duplicate detection and drift analysis without
exposing sensitive values (secrets, credentials, PII) in the audit
trail. Servers MAY include raw arguments in a separate `arguments`
field if their security policy permits.

### Transport Considerations

`audit/event` is a JSON-RPC notification (no `id`, no response
expected). It flows on the existing MCP transport:

- **Stdio:** written to stdout alongside other notifications
- **Streamable HTTP:** sent as SSE events on the open connection
- **Proxies and gateways:** MUST forward `audit/event` notifications
  transparently if both upstream and downstream negotiate `ext-audit`

Audit events MUST NOT block tool call responses. They are
informational side-channel data.

### OpenTelemetry Alignment

The `trace` fields align with W3C Trace Context and OpenTelemetry
conventions. Audit consumers that also ingest OTLP data can correlate
`ext-audit` events with gateway-level spans using shared trace IDs.

The `intent.operation_class` and `intent.scope_class` fields align
with the OpenTelemetry MCP semantic conventions (`gen_ai.tool.name`,
`mcp.method.name`) merged in January 2026.

## Rationale

### Why an Extension, Not Core

The MCP roadmap explicitly states audit trails "will likely land as
extensions rather than core specification changes." An extension
allows:

- Incremental adoption without breaking existing deployments
- Different audit consumers to coexist (compliance, observability,
  benchmarking)
- Experimentation with the event format before standardization

### Why Notifications, Not Resources

Audit events are ephemeral and high-frequency. Modeling them as MCP
resources would require polling and state management. Notifications
are fire-and-forget, matching the audit trail's append-only semantics.

### Why Not Just OTLP

OTLP spans carry rich telemetry but require external infrastructure
(collectors, backends) and use a different wire format. `ext-audit`
embeds audit events in the MCP transport itself — no additional
infrastructure needed. For teams that also use OTLP, the `trace`
fields enable correlation.

### Why Operation Classification

The `operation_class` (mutate/destroy/read/plan) and `scope_class`
(production/staging/development) fields enable risk assessment without
parsing tool-specific arguments. A gateway or audit consumer can
apply policies ("alert on destroy in production") without
understanding kubectl, terraform, or helm argument formats.

### Design Decisions

| Decision | Alternative Considered | Why This Way |
|----------|----------------------|-------------|
| Notification, not request | Request with acknowledgment | Audit must not block tool execution |
| Digest, not raw arguments | Include full arguments | Privacy by default; raw arguments opt-in |
| Server-side emission | Client-side emission | Server has ground truth about what was called and what happened |
| Session + sequence | Just timestamp | Enables behavioral pattern detection across related calls |
| Single event per call | Separate intent/outcome events | Simpler; one event captures the complete lifecycle |

## Backward Compatibility

Fully backward compatible. `ext-audit` is strictly additive:

- Servers that do not support it omit the capability
- Clients that do not understand it ignore `audit/event` notifications
- Proxies that do not understand it forward notifications transparently
  (JSON-RPC notifications require no response)

No existing MCP messages, methods, or behaviors are modified.

## Reference Implementation

A reference implementation exists in the
[Evidra](https://github.com/vitas/evidra) project:

- **MCP proxy mode:** intercepts `tools/call` on stdio and generates
  prescribe/report evidence pairs (equivalent to `ext-audit` events)
- **OTLP bridge mode:** translates AgentGateway OTLP traces into
  structured evidence
- **Signal detection:** 8 behavioral detectors operate on the audit
  event stream (retry_loop, blast_radius, protocol_violation, etc.)
- **Reliability scoring:** weighted penalty model produces 0-100
  reliability scores from audit events

The reference implementation demonstrates:
- Feasibility of the proposed event format
- Behavioral pattern detection on the event stream
- Integration with existing OTLP infrastructure via trace correlation
- Real agent benchmarking (evidra-kagent-bench)

Adapting Evidra to consume native `ext-audit` events (instead of
proxy interception or OTLP translation) would eliminate the bridge
and proxy components — a significant simplification.

## Security Implications

### Privacy

- `arguments_digest` hashes arguments by default, avoiding secret
  exposure in audit trails
- Servers SHOULD NOT include raw arguments unless explicitly
  configured and the audit transport is secured
- Audit events may contain resource names and namespaces — operators
  should consider whether these are sensitive in their environment

### Integrity

- The `session_id` + `sequence` pair enables gap detection (missing
  events indicate tampering or transport loss)
- Consumers that require tamper evidence should chain event hashes
  (not specified by this extension, but enabled by the format)

### Access Control

- `audit/event` notifications flow on the existing MCP transport and
  inherit its authentication context
- Proxies and gateways that inspect audit events should respect the
  same access controls as other MCP traffic

## Future Extensions

This SEP intentionally scopes to `tool_call` events. Future work may
add:

- `resource_read` events for read operations
- `session_start` / `session_end` events for session boundaries
- `task_lifecycle` events for A2A task state transitions
- `risk_assessment` events for pre-flight risk evaluation (separate
  from the tool call outcome)
- Standardized behavioral signal definitions (retry_loop,
  blast_radius, etc.)

These would be proposed as incremental additions to the `ext-audit`
extension, not separate extensions.
