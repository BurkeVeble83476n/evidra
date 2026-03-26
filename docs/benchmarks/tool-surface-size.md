# MCP Tool Surface Size

- Status: Reference
- Version: current
- Canonical for: measured default `tools/list` payload size
- Audience: maintainers

## Why Measure This

Smart tool loading is intended to protect the cheap-model default path:

- most agents should use `run_command`
- explicit `prescribe_smart` / `report` control is still available
- the default `tools/list` response should carry less protocol overhead

This note records the actual byte size of the default `tools/list` payload before
and after deferring the full `prescribe_smart` / `report` schemas.

## Measurement Method

Measured with a temporary local Go program that:

1. starts `mcpserver.NewServer(...)` with `HidePrescribeFull: true`
2. connects an in-memory MCP client
3. calls `tools/list`
4. marshals the full response and each individual tool to JSON
5. counts bytes

## Before Deferred Protocol Schemas

Baseline: `main` at `460ec1e`

```text
total_bytes=15603
collect_diagnostics=1198
get_event=1852
prescribe_smart=4775
report=5050
run_command=2053
write_file=658
```

## After Deferred Protocol Schemas

Branch: `smart-tool-loading`

```text
total_bytes=9745
collect_diagnostics=1198
describe_tool=295
get_event=1852
prescribe_smart=1798
report=1873
run_command=2053
write_file=658
```

## Net Change

- total payload: `15603 -> 9745` bytes
- reduction: `5858` bytes
- relative reduction: about `37.5%`

## Interpretation

This change is still useful, but the result is smaller than the original rough
estimate.

What improved:

- `prescribe_smart` dropped from `4775` bytes to `1798`
- `report` dropped from `5050` bytes to `1873`
- the default surface now encourages `run_command` first and explicit protocol
  control only when the agent asks for it

What still dominates the payload:

- tool descriptions remain large
- `run_command`, `collect_diagnostics`, and `get_event` still carry full schema
  and description payloads
- `describe_tool` adds a small fixed cost

## Product Takeaway

The main win is architectural, not just byte-count reduction:

- cheap models stay on the `run_command` + auto-evidence path
- stronger models can still opt into explicit protocol control via
  `describe_tool`
- `prescribe_full` remains opt-in

If we want a bigger reduction later, the next place to look is tool description
size, not just schema size.
