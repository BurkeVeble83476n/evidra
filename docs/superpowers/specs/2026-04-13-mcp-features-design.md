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
1. `prescribe_smart` and `report` remain on raw `server.AddTool` (not migrated to typed `mcp.AddTool[In, Out]`).
2. A new file `pkg/mcpserver/output_validation.go` provides `loadOutputSchema` and `structuredToolResultValidated` helpers. These resolve the JSON schema with `github.com/google/jsonschema-go/jsonschema`, validate the marshaled output, and build `StructuredContent`. Used by deferred tools.
3. One shared `schemas/prescribe.output.schema.json` covers both `prescribe_full` and `prescribe_smart`.
4. Schema files use conditional `allOf[if/then/else]` so both success payloads and tool-error payloads validate.
5. `structuredToolResult()` (the old helper) is replaced by `structuredToolResultValidated()` for deferred tools; typed tools keep using `mcp.AddTool[In, Out]` with `OutputSchema` field.

**Files:** `pkg/mcpserver/deferred_protocol_tools.go`, `pkg/mcpserver/prescribe_full.go`, `pkg/mcpserver/collect_diagnostics.go`, `pkg/mcpserver/schema_embed.go`, `pkg/mcpserver/schemas/`

### 2. Resource links in `report` output

**Goal:** After a successful report, include a `resource_link` content item so agents can follow directly to the evidence entry without a separate `get_event` call.

**Change:** The `report` tool stays on raw `server.AddTool`. On success, append `*mcp.ResourceLink` to `CallToolResult.Content` after calling `structuredToolResultValidated`:

```
evidra://event/{report_id}
```

`ReportOutput` struct is unchanged. This is presentation-layer only.

**Files:** `pkg/mcpserver/deferred_protocol_tools.go`

### 3. Scorecard resource

**Goal:** Expose the current assessment snapshot as a readable MCP resource so agents can check Evidra's scoring of their session at any time without a tool call.

**New resources registered in `NewServerWithCleanup`:**

| URI | Type | Meaning |
|---|---|---|
| `evidra://scorecard/aggregate` | static resource | aggregate snapshot across the whole evidence path (`sessionSnapshot("")`) |
| `evidra://scorecard/session/{session_id}` | resource template | snapshot filtered to one session (`sessionSnapshot(sessionID)`) |

**Handler:** Calls `s.sessionSnapshot(sessionID)` (already exists on `MCPService`), marshals the `assessment.Snapshot` as indented JSON, returns it as `application/json` content.

Error cases: if evidence path is not configured or snapshot fails, return `mcp.ResourceNotFoundError`.

Note: This feature does NOT use a `current` alias.

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
