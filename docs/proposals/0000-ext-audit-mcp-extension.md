# SEP-0000: `ext-audit` — Tool Call Audit Events for MCP

- **Author:** Vitaliy Ryumshyn (@vitas)
- **Status:** Pre-draft (not yet submitted)
- **Type:** Standards Track
- **Created:** 2026-03-21

## Abstract

This SEP proposes `ext-audit`, an MCP extension that emits structured
audit events for tool calls. When negotiated, servers send a
notification after each `tools/call` with the tool name, arguments
digest, result status, timing, session context, and actor identity.

The extension is strictly additive. No existing behavior changes.

## Motivation

The MCP 2026 roadmap identifies "audit trails and observability" as an
Enterprise Readiness priority:

> "end-to-end visibility into what a client requested and what a server
> did, in a form enterprises can feed into their existing logging and
> compliance pipelines"

Today there is no protocol-native way to answer: "what tools did this
agent call, and what happened?" Teams resort to:

- Proxy interception (fragile, transport-dependent)
- Gateway OTLP telemetry (requires external infrastructure, non-standard attributes)
- Agent-side logging (fragmented per framework)

Each produces a different format. Audit consumers must support all
three. A protocol-native event solves this at the source.

## Specification

### Extension Identifier

```
ext-audit
```

### Capability Negotiation

Server declares support during `initialize`:

```json
{
  "capabilities": {
    "experimental": {
      "ext-audit": {
        "version": "0.1.0"
      }
    }
  }
}
```

### Audit Event

After each `tools/call` completes, the server emits a JSON-RPC
notification:

```json
{
  "jsonrpc": "2.0",
  "method": "audit/toolCall",
  "params": {
    "timestamp": "2026-03-21T14:30:00.123Z",
    "sessionId": "sess_01HXY",
    "sequence": 7,

    "tool": "apply_yaml",
    "argumentsDigest": "sha256:abc123def456",
    "status": "success",
    "durationMs": 1250,
    "error": null,

    "actor": {
      "type": "agent",
      "id": "claude-code"
    },

    "trace": {
      "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
      "spanId": "00f067aa0ba902b7"
    },

    "metadata": {}
  }
}
```

### Fields

**Required:**

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | ISO 8601 with millisecond precision |
| `sessionId` | string | Groups related tool calls |
| `sequence` | integer | Ordering within session (monotonic) |
| `tool` | string | Tool name from `tools/list` |
| `status` | string | `"success"`, `"failure"`, or `"error"` |
| `actor.type` | string | `"agent"`, `"human"`, or `"system"` |
| `actor.id` | string | Stable identifier |

**Optional:**

| Field | Type | Description |
|-------|------|-------------|
| `argumentsDigest` | string | SHA-256 of the arguments JSON (privacy by default) |
| `durationMs` | integer | Execution time in milliseconds |
| `error` | string | Error message when `status` is not `"success"` |
| `actor.version` | string | Software version |
| `trace.traceId` | string | W3C Trace Context trace ID |
| `trace.spanId` | string | W3C Trace Context span ID |
| `metadata` | object | Arbitrary key-value pairs for consumer-specific data |

### Design Choices

**Arguments are hashed, not included.** `argumentsDigest` is a SHA-256
of the raw arguments JSON. This enables duplicate detection and drift
analysis without exposing secrets, credentials, or PII in the audit
trail. Servers MAY include raw arguments in `metadata` if their policy
permits.

**One event per call, emitted after completion.** The server knows the
full picture — what was called and what happened. Splitting into
before/after events adds complexity. Consumers that need pre-execution
signals should use `metadata` or a separate extension.

**Session grouping is explicit.** `sessionId` groups related calls
(one agent task, one CI run). It is distinct from the MCP transport
session — one logical session may span reconnects or multiple
transport sessions.

**Sequence is monotonic within session.** Enables gap detection
(missing events) without wall-clock ordering assumptions.

**`metadata` is the extension point.** Consumers that need additional
fields (risk levels, operation classification, resource identifiers)
put them in `metadata`. The core event stays small and neutral.

### Transport

`audit/toolCall` is a JSON-RPC notification (no response expected):

- **Stdio:** written to stdout alongside other notifications
- **Streamable HTTP:** sent as SSE event
- **Proxies:** MUST forward transparently if both sides negotiate

Audit events MUST NOT block tool call responses.

## Rationale

### Why Minimal

The extension captures **facts**: what tool, what happened, how long,
who did it. It does not classify, assess, or score. Different consumers
need different intelligence:

- Compliance tools need immutable logs
- Observability tools need metrics and traces
- Reliability tools need behavioral pattern detection
- Security tools need threat detection

A minimal event serves all of them. A rich event serves only one.

### Why Not OTLP

OTLP requires external infrastructure (collectors, backends) and uses
a different wire format. `ext-audit` embeds events in the MCP
transport — zero additional infrastructure. The `trace` fields enable
correlation with OTLP for teams that use both.

### Why Not Resources

Audit events are high-frequency and append-only. Resources are
stateful and polled. Notifications match the audit model.

### Why `metadata` Instead of Defined Fields

Different audit consumers have different needs. A compliance logger
wants nothing extra. A reliability tool wants operation classification.
A cost tracker wants token counts. Defining all possible fields in the
core event would bloat the spec and create perpetual versioning
pressure. `metadata` lets consumers negotiate with servers out-of-band.

Example: a reliability tool might expect:

```json
{
  "metadata": {
    "operationClass": "mutate",
    "scopeClass": "production",
    "riskLevel": "high"
  }
}
```

While a cost tracker might expect:

```json
{
  "metadata": {
    "promptTokens": 1500,
    "completionTokens": 350,
    "estimatedCostUsd": 0.002
  }
}
```

Neither is in the core spec. Both work.

## Backward Compatibility

Fully backward compatible:

- Servers without `ext-audit` omit the capability — no change
- Clients that don't understand `audit/toolCall` ignore it — per
  JSON-RPC notification semantics
- Proxies forward notifications transparently

No existing messages, methods, or behaviors are modified.

## Reference Implementation

The [Evidra](https://github.com/vitas/evidra) project implements the
audit trail concept today using proxy interception and OTLP bridge
translation. Adapting to consume `ext-audit` events natively would
eliminate both workarounds.

A prototype `ext-audit` emitter can be built as:
- A middleware wrapper for any MCP server SDK
- An addition to existing MCP proxy implementations
- A gateway plugin (AgentGateway, Envoy)

## Security Implications

**Privacy:** Arguments are hashed by default. Tool names and session
IDs are transmitted — operators should consider whether these are
sensitive.

**Integrity:** `sessionId` + `sequence` enables gap detection. For
tamper evidence, consumers can chain event hashes (not in scope for
this extension).

**Access:** Audit events inherit the MCP transport's authentication.
No new auth mechanism introduced.
