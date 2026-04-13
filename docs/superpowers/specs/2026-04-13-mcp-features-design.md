# MCP Features Design

**Date:** 2026-04-13
**Status:** Approved
**Scope:** MCP spec 2025-11-25 / go-sdk v1.5.0 feature adoption

## Context

Evidra currently runs go-sdk v1.4.1. PR #20 bumps to v1.5.0. The MCP spec has shipped several features since Evidra's MCP server was first built. This design covers the subset worth adopting now.

## Scope

Three features in one PR, plus one deferred:

- **PR 1 — Foundation:** structured output on all tools, resource links in `report` output, scorecard resource, registry description field
- **PR 2 — Async (deferred):** Tasks support for `collect_diagnostics` — blocked on go-sdk implementing `execution.taskSupport` on `Tool`. The `MemoryEventStore` in v1.5.0 is for SSE transport resumability only, not async tool execution. Revisit when the SDK ships Tasks.

## PR 1: Foundation

### 1. Structured output on all tools

**Goal:** Every Evidra tool declares an `outputSchema`. Agents receive validated, schema-declared JSON in `structuredContent`.

**Current state:**
- `get_event` — uses `mcp.AddTool[In, Out]` with `OutputSchema` JSON file. Correct.
- `prescribe_smart`, `report` — use raw `server.AddTool` + manual `structuredToolResult()` helper. No `outputSchema` declared.
- `prescribe_full`, `run_command`, `collect_diagnostics` — use `mcp.AddTool[In, Out]`; `run_command` has `OutputSchema` wired; `prescribe_full` and `collect_diagnostics` are missing output schema JSON files.

**Changes:**
1. `prescribe_smart` and `report` intentionally use a minimal `{"type":"object"}` input schema (the "deferred protocol" pattern — agents call `describe_tool` first). This input schema must not be replaced by auto-generated schema. Therefore these tools stay on raw `server.AddTool` but gain an explicit `OutputSchema` field, populated from new JSON files embedded via `schema_embed.go`.
2. Add `schemas/prescribe.output.schema.json`, `schemas/report.output.schema.json`, `schemas/prescribe_full.output.schema.json`, and `schemas/collect_diagnostics.output.schema.json` — hand-authored or generated from the existing output structs, embedded via `schema_embed.go`.
3. Update `structuredToolResult()` to accept and attach an `outputSchema` parameter, so callers can pass the loaded schema. `decodeDeferredInput()` is unchanged.
4. `prescribe_full` and `collect_diagnostics` use `mcp.AddTool[In, Out]` — wire their output schema JSON files into the existing `OutputSchema` field on the tool definition.

**Files:** `pkg/mcpserver/deferred_protocol_tools.go`, `pkg/mcpserver/prescribe_full.go`, `pkg/mcpserver/collect_diagnostics.go`, `pkg/mcpserver/schema_embed.go`, `pkg/mcpserver/schemas/`

### 2. Resource links in `report` output

**Goal:** After a successful report, include a `resource_link` content item so agents can follow directly to the evidence entry without a separate `get_event` call.

**Change:** In the `report` tool's Handle function (after migration to typed `AddTool[In, Out]`), when `ReportOutput.OK == true`, append a `mcp.ResourceLinkContent` item to the returned `*mcp.CallToolResult.Content` slice:

```
evidra://event/{report_id}
```

`ReportOutput` struct is unchanged. This is presentation-layer only.

**Files:** `pkg/mcpserver/deferred_protocol_tools.go`

### 3. Scorecard resource

**Goal:** Expose the current assessment snapshot as a readable MCP resource so agents can check Evidra's scoring of their session at any time without a tool call.

**New resources registered in `NewServerWithCleanup`:**

| URI | Type | Handler |
|-----|------|---------|
| `evidra://scorecard/current` | static resource | `s.readResourceScorecard("")` |
| `evidra://scorecard/{session_id}` | resource template | `s.readResourceScorecard(sessionID)` |

**Handler:** Calls `s.sessionSnapshot(sessionID)` (already exists on `MCPService`), marshals the `assessment.Snapshot` as indented JSON, returns it as `application/json` content.

Error cases: if evidence path is not configured or snapshot fails, return `mcp.ResourceNotFoundError`.

**Files:** `pkg/mcpserver/server.go`

### 4. Registry description field

**Change:** One line in `NewServerWithCleanup`:

```go
&mcp.Implementation{
    Name:        opts.Name,
    Version:     opts.Version,
    Description: "Flight recorder for AI infrastructure agents",
}
```

**Files:** `pkg/mcpserver/server.go`

---

## PR 2: Tasks for `collect_diagnostics` (deferred)

**Status: blocked.** The MCP Tasks spec (2025-11-25) defines `execution.taskSupport` on tools and `tasks/get` / `tasks/result` protocol methods, but go-sdk v1.5.0 does not implement them. The `MemoryEventStore` in `ServerOptions` is for HTTP Streamable transport resumability only, not async tool execution. Design and implement this PR once the SDK ships Tasks support.

---

## What is not in scope

- **Elicitation** — conflicts with "never block agents" principle. Revisit if the principle is relaxed.
- **Resource subscribe** — no server-side push events today.
- **OAuth / HTTP transport changes** — Evidra uses stdio + API key.

## Dependencies

- Merge PR #20 (`go-sdk` 1.4.1 → 1.5.0) before implementing. PR 1 uses the stable v1.5.0 API.
