# A2A Execution Mode Integration Design

**Date:** 2026-03-27

**Status:** Approved design

**Supersedes:** `docs/plans/2026-03-27-thread-adapter-through-trigger.md`

## Goal

Make A2A a first-class hosted execution mode across `evidra`, `evidra-bench`, and `evidra-kagent-bench` so an Evidra-triggered run can explicitly choose remote A2A agent execution end-to-end without exposing bench-internal adapter plumbing in the public API.

## Problem

Today the demo stack contains a real A2A runtime in `evidra-bench`, but Evidra's public trigger contract is still provider-centric. The current raw `adapter` threading proposal leaks bench internals into the hosted API and still misses key paths like the UI trigger flow and runner claim payloads.

The missing abstraction is not "adapter". It is "execution mode".

## Design Summary

Expose `execution_mode` as the public control-plane field in Evidra.

- Public trigger API/UI uses `execution_mode: provider | a2a`
- Evidra persists and reports `execution_mode` on jobs
- Evidra translates `execution_mode=a2a` to bench-cli's internal `config.adapter=a2a`
- Bench workers continue resolving the actual A2A endpoint from local config or environment
- The public API does not expose `a2a_agent_url`

This keeps Evidra responsible for orchestration intent and keeps bench-cli responsible for actual A2A execution.

## Architecture

### Public contract

`POST /v1/bench/trigger` accepts:

- `model`
- optional `provider`
- optional `runner_id`
- required `evidence_mode`
- optional `execution_mode`
- `scenarios`

`execution_mode` defaults to `provider` when omitted.

Allowed values:

- `provider`: bench-cli runs its built-in provider/tool-use loop
- `a2a`: bench-cli delegates the task to a remote A2A-compatible agent

### Boundary between repos

Evidra owns the public hosted contract.

- user-facing field: `execution_mode`
- persisted job field: `execution_mode`
- runner claim field: `execution_mode`

bench-cli owns internal execution mechanics.

- internal certify request field: `config.adapter`
- internal worker config: `A2AAgentURL`
- A2A RPC, discovery, and run metadata

Translation happens only at the Evidra → bench-cli boundary.

### Why not `provider=a2a`

`provider` answers "which model backend/tool loop is being used when bench owns execution".

`a2a` answers "who owns execution".

They are different axes. A remote A2A agent may itself use OpenAI, Anthropic, DeepSeek, or something else internally. Overloading `provider` would mix transport mode with model-provider identity and make the contract harder to reason about.

### Why not public `adapter`

Raw `adapter` is a bench implementation detail. Exposing it in Evidra would:

- leak internal bench concepts into the public API
- imply support for low-level modes that hosted Evidra does not directly model
- make future control-plane evolution harder

`execution_mode` is the correct public abstraction.

### Why not public `a2a_agent_url`

The A2A endpoint is deployment configuration, not user intent.

Keeping it worker-local:

- avoids coupling the hosted API to transport URLs
- avoids validation and security complexity in Evidra
- keeps runner mode and direct-executor mode symmetrical
- allows future multi-agent routing to use a stable logical field such as `execution_target` rather than raw URLs

## End-to-End Data Flow

### Direct executor path

1. UI or API sends `execution_mode`
2. Evidra validates and stores it on the trigger job
3. Remote executor maps:
   - `provider` mode → no adapter override
   - `a2a` mode → `config.adapter = "a2a"`
4. bench-cli builds run config from the certify request
5. bench-cli executes either provider mode or A2A mode
6. bench-cli reports progress and bench run results back to Evidra

### Poll-based runner path

1. UI or API sends `execution_mode`
2. Evidra persists `execution_mode` in queued job config
3. Runner claims a job and receives `execution_mode`
4. Runner-side bench translation maps:
   - `provider` mode → normal provider path
   - `a2a` mode → adapter `a2a`
5. bench-cli executes and reports completion through existing runner/job result flow

The same public field survives both execution paths unchanged.

## Repo-by-Repo Changes

### `evidra`

Add `execution_mode` to:

- trigger request model
- trigger job model
- queued runner job config
- trigger status payload
- runner claim payload
- OpenAPI and markdown API docs
- architecture and runner contract docs
- UI trigger modal

Validation rules:

- default missing value to `provider`
- reject unknown values with `400`

Executor translation:

- `execution_mode=a2a` sets `config.adapter=a2a` in the certify request
- `provider` mode leaves adapter unset

Legacy behavior:

- queued jobs without `execution_mode` default to `provider`

### `evidra-bench`

Keep A2A runtime ownership in bench-cli.

Changes:

- accept hosted `execution_mode` in the runner/control-plane integration path
- map `execution_mode=a2a` to existing internal adapter config
- keep A2A endpoint resolution local via config/env
- update hosted integration docs to reflect public `execution_mode` and internal `config.adapter`

The existing direct `/v1/certify` adapter support remains valid and does not need to be replaced.

### `evidra-kagent-bench`

Align the demo stack and tests with the new public contract.

Changes:

- keep the configured A2A endpoint in compose env
- update E2E trigger requests to use `execution_mode:"a2a"`
- update demo docs/UI expectations to describe execution mode selection
- verify resulting run metadata reflects A2A execution rather than provider mode

## Error Handling

- missing `execution_mode` means `provider`
- unknown `execution_mode` returns `400`
- Evidra does not validate A2A endpoint reachability
- worker misconfiguration remains a bench-side execution failure and should surface through existing job progress/failure paths

This preserves clear ownership:

- control-plane validation in Evidra
- runtime/A2A validation in bench-cli

## Testing Strategy

### `evidra`

- handler tests for trigger validation/defaulting
- remote executor tests for `execution_mode -> config.adapter`
- runner handler tests for claim payload threading
- runner storage tests for persistence/defaulting
- OpenAPI and markdown docs tests
- UI trigger request test if UI tests exist locally

### `evidra-bench`

- certify translation tests
- hosted runner/control-plane tests for `execution_mode`
- retain existing A2A harness tests

### `evidra-kagent-bench`

- full E2E trigger with `execution_mode:"a2a"`
- verify trigger completes
- verify resulting run is recorded as A2A-backed

## Non-Goals

- no public raw `adapter` field in Evidra
- no public per-run `a2a_agent_url`
- no Evidra-side implementation of A2A protocol logic
- no hardcoded kagent-specific branching in Evidra

## Migration Notes

This design is backward-compatible for existing trigger clients because `execution_mode` is optional and defaults to `provider`.

It also gives a clean upgrade path from the earlier raw-adapter design: the old plan should be treated as superseded rather than partially implemented.
