# MCP Inspector E2E Tests

Deterministic integration tests for `evidra-mcp` using MCP Inspector CLI and MCP transports.

## Modes

- `local-mcp` (default): Inspector CLI -> local `evidra-mcp` stdio
- `hosted-mcp` (opt-in): JSON-RPC/curl -> hosted MCP endpoint

`hosted-mcp` is disabled by default and requires `EVIDRA_ENABLE_NETWORK_TESTS=1`.

## Run

```bash
make test-mcp-inspector
```

By default local mode uses `EVIDRA_SIGNING_MODE=optional` in the runner to avoid requiring persistent signing keys for smoke tests.
If npm registry DNS/network is unavailable, local-mcp preflight skips the suite with a single clear reason.
Set `EVIDRA_INSPECTOR_STRICT_NETWORK=1` to fail fast instead of skip.

Optional hosted mode:

```bash
EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-mcp EVIDRA_MCP_URL=https://example.com/mcp bash tests/inspector/run_inspector_tests.sh
```

## Layout

- `run_inspector_tests.sh`: main runner
- `special/t_*.sh`: mode-aware special checks (`list_tools`, `schema_error`, `get_event` chain). `list_tools` also verifies the `run_command` metadata surface and the `collect_diagnostics` schema.
- `cases/*.json`: curated scenario set for `prescribe_full` / `prescribe_smart` / `report`
- `fixtures/*`: imported useful baseline fixtures from old `evidra-mcp`
