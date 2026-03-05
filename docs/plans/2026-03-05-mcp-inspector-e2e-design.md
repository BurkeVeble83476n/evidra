# MCP Inspector E2E Design (Local + Hosted/REST Modes)

Date: 2026-03-05
Status: Approved

## 1. Goal

Add deterministic MCP inspector-based e2e tests for Evidra benchmark using the old `evidra-mcp` repository as baseline, while aligning to the current `prescribe/report` protocol.

## 2. Final Decisions

- Build baseline from old `evidra-mcp/tests/inspector` runner and structure.
- Use hybrid approach:
  - reuse runner shell/testing architecture and special-case pattern,
  - rewrite scenario contract for current `prescribe/report/get_event` tools.
- Modes to support:
  - `local-mcp` (default),
  - `local-rest` (opt-in),
  - `hosted-mcp` (opt-in, disabled by default),
  - `hosted-rest` (opt-in, disabled by default).
- Hosted and REST network paths must be skip-by-default.
- `local-rest` is also opt-in via flag/env.
- REST contract for test paths is new inspector model:
  - `POST /v1/prescribe`,
  - `POST /v1/report`,
  - optional evidence fetch endpoint for chain checks.
- Test data policy: curated subset from old repo if useful, not full corpus dump.

## 3. Architecture

Single runner script controls all modes:

1. Resolve mode and required env.
2. Execute special checks (`list_tools`, `schema_error`, `get_event_chain`) when applicable.
3. Execute curated scenario cases from `tests/inspector/cases`.
4. Print pass/fail/skip summary and exit non-zero on failures only.

Transport mapping:

- `local-mcp`: MCP Inspector CLI over stdio to local `evidra-mcp`.
- `hosted-mcp`: MCP Inspector CLI (or equivalent JSON-RPC path) to hosted MCP endpoint.
- `local-rest`: curl/jq against local backend REST inspector endpoints.
- `hosted-rest`: curl/jq against hosted backend REST inspector endpoints.

All non-local network modes are explicit opt-in and should skip cleanly when env prerequisites are absent.

## 4. Test Scenarios (Curated Baseline)

Initial required scenario set:

1. `lifecycle_ok`
   - `prescribe` valid manifest -> `report` success -> optional event/chain assertions.
2. `schema_error_prescribe`
   - invalid input shape -> structured error asserted.
3. `parse_error_artifact`
   - malformed artifact -> parse error asserted.
4. `risk_tag_privileged`
   - privileged k8s input -> expected risk tag present.
5. `report_unknown_prescription`
   - `report` with unknown `prescription_id` -> expected rejection path.
6. `cross_actor_report`
   - actor mismatch between prescribe/report -> protocol violation semantics asserted.

## 5. Test Data Strategy

Import only useful fixtures from old repository:

- `tests/e2e/fixtures/safe-nginx-deployment.yaml`
- `tests/e2e/fixtures/privileged-pod.yaml`

Optionally import additional corpus files only when directly mapped to current `prescribe/report` contract.

Do not migrate old full `validate` corpus blindly.

## 6. Repository Layout

Planned files:

- `tests/inspector/run_inspector_tests.sh`
- `tests/inspector/README.md`
- `tests/inspector/mcp-config.json`
- `tests/inspector/cases/*.json`
- `tests/inspector/fixtures/*`
- `tests/inspector/special/t_*.sh`

## 7. Execution Controls

Environment variables:

- `EVIDRA_TEST_MODE=local-mcp|local-rest|hosted-mcp|hosted-rest`
- `EVIDRA_LOCAL_API_URL` for local REST mode
- `EVIDRA_MCP_URL` for hosted MCP mode
- `EVIDRA_API_URL` and `EVIDRA_API_KEY` for REST modes
- `EVIDRA_ENABLE_NETWORK_TESTS=1` guard for hosted modes

Default behavior:

- no env flags -> run `local-mcp` only
- hosted/rest without required env -> skip with reason (not fail)

## 8. Makefile Targets

Add targets:

- `test-mcp-inspector` -> default local MCP run
- `test-mcp-inspector-local-rest` -> opt-in local REST
- `test-mcp-inspector-hosted` -> hosted MCP
- `test-mcp-inspector-hosted-rest` -> hosted REST

## 9. Non-Goals for This Iteration

- Full historical corpus migration from old repo.
- LLM-driven end-to-end tests.
- Docker-only orchestration for inspector tests.

This layer is intended to be deterministic, transport-aware integration coverage between unit/integration tests and any future full agent E2E layer.
