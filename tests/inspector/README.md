# MCP Inspector E2E Tests

Deterministic integration tests for `evidra-mcp` using MCP Inspector CLI and REST transports.

## Modes

- `local-mcp` (default): Inspector CLI -> local `evidra-mcp` stdio
- `local-rest` (opt-in): curl -> local backend REST (`/v1/prescribe`, `/v1/report`)
- `hosted-mcp` (opt-in): JSON-RPC/curl -> hosted MCP endpoint
- `hosted-rest` (opt-in): curl -> hosted backend REST

`hosted-*` modes are disabled by default and require `EVIDRA_ENABLE_NETWORK_TESTS=1`.

## Run

```bash
make test-mcp-inspector
```

By default local mode uses `EVIDRA_SIGNING_MODE=optional` in the runner to avoid requiring persistent signing keys for smoke tests.

Optional modes:

```bash
EVIDRA_SIGNING_MODE=optional EVIDRA_TEST_MODE=local-rest EVIDRA_LOCAL_API_URL=http://127.0.0.1:8080 bash tests/inspector/run_inspector_tests.sh
EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-mcp EVIDRA_MCP_URL=https://example.com/mcp bash tests/inspector/run_inspector_tests.sh
EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-rest EVIDRA_API_URL=https://example.com EVIDRA_API_KEY=... bash tests/inspector/run_inspector_tests.sh
```

## Layout

- `run_inspector_tests.sh`: main runner
- `special/t_*.sh`: mode-aware special checks (`list_tools`, `schema_error`, `get_event` chain)
- `cases/*.json`: curated scenario set for `prescribe/report`
- `fixtures/*`: imported useful baseline fixtures from old `evidra-mcp`
