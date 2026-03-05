# LangChain-First Integration Design (General Ingestion Sidecar)

Date: 2026-03-05
Status: Approved

## 1. Context

Integration priority:

1. LangChain
2. LangGraph
3. MCP (already exists)
4. AutoGen
5. CrewAI

Current decision is to implement v1 for LangChain first, while creating a reusable ingestion layer for future integrations.

## 2. Final Decisions

- Language for LangChain integration v1: Python.
- Integration mechanism: LangChain `CallbackHandler`.
- Capture scope: only infrastructure mutate operations.
- Filtering strategy: explicit allowlist only (`tool_name -> operation mapping`), no heuristics.
- Transport model: local Go sidecar `evidra-agent`.
- Backend transport: new REST ingestion endpoint (to be restored/added from previous backend lineage).
- Resilience model: store-and-forward (local durable spool when backend is unavailable).
- Delivery semantics: at-least-once; backend deduplicates by `event_id`.
- `evidra-agent` scope: general ingestion sidecar for all future framework integrations.

## 3. Architecture

High-level flow:

1. LangChain tool event occurs.
2. `EvidraCallbackHandler` checks allowlist.
3. If allowlisted mutate operation:
   - emit `prescribe` at start
   - emit `report` at completion/failure
4. Events are sent to local `evidra-agent` over localhost REST.
5. `evidra-agent` appends to local durable spool (JSONL) and asynchronously ships to backend ingestion API.
6. Backend acknowledges `accepted` or `duplicate` (idempotent behavior via `event_id`).

Responsibilities:

- Python SDK: event capture, allowlist filter, correlation (`run_id -> prescription_id`), sidecar calls.
- Go sidecar: durability, retries, backoff, redelivery, graceful recovery after restart.
- Backend endpoint: schema validation, idempotent ingest, status ack.

## 4. Data Contract (Minimum v1)

Unified envelope for all integrations (LangChain now, LangGraph/AutoGen/CrewAI later):

- `event_id` (UUID/ULID, unique per emission)
- `event_type` (`prescribe` | `report`)
- `timestamp` (RFC3339)
- `integration` (`langchain`)
- `actor` (`type`, `id`)
- `operation` (`tool`, `operation`, optional `scope_class`)
- `correlation` (`trace_id`, `prescription_id` for report)
- `payload` (event-specific details)

Backend endpoint (working name):

- `POST /v1/events/ingest`
- Response status semantics:
  - `accepted`: stored successfully
  - `duplicate`: already processed (idempotent success)
  - `rejected`: invalid payload/schema

## 5. Reliability and Error Handling

Store-and-forward behavior:

- If backend is down, sidecar continues accepting local SDK writes.
- Events remain in local spool until acknowledged by backend.
- On sidecar restart, replay resumes from last unacked offset.
- Duplicate sends are expected and safe under at-least-once model.

Non-blocking principle:

- SDK should not block agent workflows for long backend-side failures.
- SDK communicates only with localhost sidecar and uses short timeouts.
- If sidecar is unavailable, SDK logs integration error and proceeds without interrupting agent execution.

## 6. Testing Strategy (v1)

Python SDK unit tests:

- allowlist filtering behavior
- mapping `run_id -> prescription_id`
- `prescribe/report` payload composition

Go sidecar unit tests:

- spool append/read
- retry/backoff policy
- duplicate ack handling
- recovery after restart

Integration tests:

- `SDK -> sidecar -> mock backend`
- backend outage -> local buffering -> recovery flush
- duplicate delivery idempotency by `event_id`

Example/template (required in v1):

- ready-to-run LangChain example
- includes one allowlisted mutate tool and one non-mutate tool
- verifies non-mutate is not ingested

## 7. Scope Boundaries

In scope (v1):

- LangChain callback integration
- allowlist-only mutate capture
- common sidecar ingestion contract
- durable local buffering and async backend delivery

Out of scope (v1):

- exact-once delivery
- heuristic operation classification
- direct backend calls from SDK
- full multi-framework SDKs (LangGraph/AutoGen/CrewAI implementation)

## 8. Roadmap Continuity

This design intentionally makes `evidra-agent` framework-agnostic so that:

- LangGraph can reuse same local API and envelope.
- AutoGen can integrate without duplicating retry/spool logic.
- CrewAI can plug into the same ingestion contract.

Only framework-specific adapters/handlers should differ; reliability path remains shared.
