# API Reference

- Status: Reference
- Version: current
- Canonical for: public HTTP endpoints and response examples
- Audience: public

All endpoints are served by `evidra-api` (default `:8080`).

## Authentication

Authenticated endpoints require a Bearer token in the `Authorization` header:

```
Authorization: Bearer <api-key>
```

Keys are issued via `POST /v1/keys` (see below) or set statically with the `EVIDRA_API_KEY` environment variable.

---

## Public Endpoints

### `GET /healthz`

Liveness probe. Returns `200 OK` with JSON body:

```json
{"status":"ok"}
```

### `GET /readyz`

Readiness probe. Returns `200 OK` with JSON body `{"status":"ok"}` when the database connection is healthy, and `503` when it is not.

### `GET /v1/evidence/pubkey`

Returns the Ed25519 public key (PEM-encoded) when signing is configured.

---

## Key Management

### `POST /v1/keys`

Issue a new API key. Gated by an invite secret, not by standard Bearer auth.

**Headers:**

| Header | Required | Description |
|---|---|---|
| `X-Invite-Secret` | Yes | Must match the server's `EVIDRA_INVITE_SECRET` value |

**Request body:**

```json
{ "label": "my-ci-pipeline" }
```

- `label` — optional, max 128 characters.

**Response** (`201 Created`):

```json
{
  "key": "ev1_abc123...",
  "prefix": "ev1_abc1",
  "tenant_id": "tnt_...",
  "created_at": "2025-01-15T10:30:00Z"
}
```

**Rate limit:** 3 keys per hour per IP.

**Errors:**
- `400` — invalid JSON or label too long
- `403` — missing or invalid invite secret
- `429` — rate limit exceeded
- `501` — key management not available
- `503` — invite secret not configured on server

---

## Evidence Ingestion

All ingestion endpoints require Bearer auth.

### `POST /v1/evidence/forward`

Forward a single evidence entry (raw JSON).

**Request body:** Any valid JSON evidence entry.

**Response:**

```json
{ "receipt_id": "01JD...", "status": "accepted" }
```

### `POST /v1/evidence/batch`

Ingest multiple entries in one request.

**Request body:**

```json
{ "entries": [ { ... }, { ... } ] }
```

**Response:**

```json
{ "accepted": 5, "errors": [] }
```

### `POST /v1/evidence/ingest/prescribe`

Typed external lifecycle ingest for prescribe entries. Use this when an external adapter wants Evidra to build and sign the final prescribe entry from normalized request fields instead of forwarding a raw entry blob.

The request carries:
- `contract_version`
- actor and correlation fields
- request taxonomy: `flavor`, `evidence.kind`, `source.system`
- either `canonical_action` or `smart_target`
- optional top-level `prescription_id` and `artifact_digest`
- optional `payload_override` when the caller already has a shaped prescribe payload body

**Response** (`200 OK` or `202 Accepted`):

```json
{
  "entry_id": "01JD...",
  "prescription_id": "presc_...",
  "effective_risk": "medium",
  "duplicate": false
}
```

### `POST /v1/evidence/ingest/report`

Typed external lifecycle ingest for report entries. Use this when an external adapter wants Evidra to resolve a prescription, validate the lifecycle terminal state, and build the canonical report entry server-side.

The request carries:
- `contract_version`
- actor and correlation fields
- request taxonomy: `flavor`, `evidence.kind`, `source.system`
- `prescription_id`
- `verdict`
- `exit_code` for non-declined typed reports or `decision_context` for declined typed reports
- optional top-level `artifact_digest`
- optional `payload_override` for already-shaped report payload bodies

**Response** (`200 OK` or `202 Accepted`):

```json
{
  "entry_id": "01JD...",
  "effective_risk": "medium",
  "duplicate": false
}
```

### `POST /v1/evidence/findings`

Ingest SARIF findings as evidence entries.

---

## Evidence Queries

All query endpoints require Bearer auth.

### `GET /v1/evidence/entries`

List evidence entries with pagination and optional filters.

**Query parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | integer | `100` | Page size (max 1000) |
| `offset` | integer | `0` | Number of entries to skip |
| `type` | string | — | Filter by entry type (`prescribe`, `report`, `finding`, etc.) |
| `period` | string | — | Time window (`7d`, `30d`, `90d`) |
| `session_id` | string | — | Filter by session ID |

**Response:**

```json
{
  "entries": [
    {
      "id": "01JD1A2B3C",
      "type": "prescribe",
      "tool": "kubectl",
      "operation": "apply",
      "scope": "namespace",
      "risk_level": "medium",
      "actor": "alice",
      "resource": "deployment/web (demo)",
      "created_at": "2025-01-15T10:30:00Z"
    },
    {
      "id": "01JD1A2B3D",
      "type": "report",
      "tool": "kubectl",
      "operation": "apply",
      "scope": "namespace",
      "risk_level": "medium",
      "actor": "alice",
      "resource": "deployment/web (demo)",
      "verdict": "success",
      "exit_code": 0,
      "created_at": "2025-01-15T10:30:05Z"
    }
  ],
  "total": 47,
  "limit": 20,
  "offset": 0
}
```

Compatibility note: this query surface still returns a flat `risk_level` field.
For prescribe entries, it is derived from the stored `effective_risk` when present,
with fallback to legacy payloads.

**Pagination example:**

```bash
# First page (20 entries)
curl -H "Authorization: Bearer $KEY" \
  "http://localhost:8080/v1/evidence/entries?limit=20&offset=0"

# Second page
curl -H "Authorization: Bearer $KEY" \
  "http://localhost:8080/v1/evidence/entries?limit=20&offset=20"

# Filter by type and period
curl -H "Authorization: Bearer $KEY" \
  "http://localhost:8080/v1/evidence/entries?type=prescribe&period=7d&limit=50"
```

### `GET /v1/evidence/entries/{id}`

Retrieve a single entry by ID.

**Response:** Same shape as a single entry in the list response above.

---

## Analytics

All analytics endpoints require Bearer auth.

### `GET /v1/evidence/scorecard`

Compute a reliability scorecard from stored evidence.

**Query parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `period` | string | `30d` | Time window (`7d`, `24h`, `30d`, `90d`) |
| `actor` | string | — | Filter by actor ID |
| `tool` | string | — | Filter by tool name |
| `scope` | string | — | Filter by scope |
| `session_id` | string | — | Filter by session |
| `min_operations` | integer | — | Minimum operation count threshold |

**Response:**

```json
{
  "score": 96.5,
  "band": "good",
  "basis": "sufficient",
  "confidence": "high",
  "total_entries": 47,
  "signal_summary": {
    "protocol_violation": { "detected": false, "weight": 0.30, "count": 0 },
    "artifact_drift": { "detected": true, "weight": 0.25, "count": 2 },
    "retry_loop": { "detected": true, "weight": 0.15, "count": 1 },
    "thrashing": { "detected": false, "weight": 0.10, "count": 0 },
    "blast_radius": { "detected": false, "weight": 0.10, "count": 0 },
    "risk_escalation": { "detected": false, "weight": 0.10, "count": 0 },
    "new_scope": { "detected": true, "weight": 0.05, "count": 3 },
    "repair_loop": { "detected": false, "weight": -0.05, "count": 0 }
  },
  "period": "30d",
  "scoring_version": "v1.1.0",
  "generated_at": "2025-01-15T10:30:00Z"
}
```

### `GET /v1/evidence/explain`

Signal-level breakdown of detected behavioral patterns.

**Query parameters:** Same as `/v1/evidence/scorecard`.

---

## Webhooks

### `POST /v1/hooks/argocd`

ArgoCD webhook receiver. Requires:
- `Authorization: Bearer <EVIDRA_WEBHOOK_SECRET_ARGOCD>`
- `X-Evidra-API-Key: <tenant-api-key>`

Maps `sync_started` / `sync_completed` events to prescribe/report entries.

This route remains supported, but it is not the full Argo CD product story.
In self-hosted mode, controller-observed reconciliation is the primary GitOps
path; webhook mode is the adjacent push path.

### `POST /v1/hooks/generic`

Generic webhook receiver. Requires:
- `Authorization: Bearer <EVIDRA_WEBHOOK_SECRET_GENERIC>`
- `X-Evidra-API-Key: <tenant-api-key>`

Maps `operation_started` / `operation_completed` events.

Contract:
- `operation_id` is required on both start and completion events and is the stable lifecycle identity used to correlate prescribe/report entries.
- `idempotency_key` remains required on `operation_completed`, but only for duplicate suppression.

---

## Auth Check

### `GET /auth/check`

### `HEAD /auth/check`

Validate a Bearer token. `GET` returns `200` with tenant metadata if valid, `401` if not. `HEAD` returns the same auth decision without a body. Both are useful as forward-auth targets for reverse proxies.

---

## Bench Endpoints

Infrastructure agent benchmark results and analytics.

Top-level bench filters use the public labels All | Baseline | Evidra.
The API alias values are `all|none|evidra`.
TODO: exact-match stored subtype filtering for internal modes like `proxy`,
`direct`, and `mcp` stays behind the advanced query surface.

### Public Endpoints (No Auth)

#### GET /v1/bench/leaderboard

Model ranking by pass rate with pass^k reliability metric.

Query params:
- `evidence_mode` (`""` = all, `none` = baseline only, `evidra` = non-`none`, other non-empty values match stored modes exactly)
- `k` (integer, 1-10, default 3) — number of trials for pass^k reliability metric

Response:
```json
{
  "models": [
    {
      "model": "claude-sonnet-4",
      "scenarios": 33,
      "runs": 40,
      "pass_rate": 97.5,
      "avg_duration": 72.0,
      "avg_cost": 0.24,
      "total_cost": 8.07,
      "pass_k": 85.2,
      "pass_k_trials": 3,
      "sufficient_scenarios": 28
    }
  ],
  "evidence_mode": ""
}
```

`pass_k` is the pass^k reliability score (0-100): for each scenario with >= k trials, compute POWER(pass_rate, k), then average across qualifying scenarios. `sufficient_scenarios` shows how many scenarios had enough trials.

#### GET /v1/bench/scenarios

Scenario catalog.

### Authenticated Endpoints

#### POST /v1/bench/runs

Submit a single benchmark run.

#### POST /v1/bench/runs/batch

Batch submit runs. Body: `{"runs": [...]}`. Idempotent (ON CONFLICT DO NOTHING).

#### GET /v1/bench/runs

List runs with filters: `model`, `scenario`, `evidence_mode` (all|none|evidra), `since`, `passed`, `limit`, `offset`, `sort_by`, `sort_order`.

`evidence_mode` follows the bench contract:
- empty means all runs
- `none` returns baseline runs only
- `evidra` returns all non-`none` runs
- any other non-empty value is an exact-match filter against stored modes

#### GET /v1/bench/runs/{id}

Get single run detail.

#### GET /v1/bench/runs/{id}/transcript

Run transcript (text/plain).

#### GET /v1/bench/runs/{id}/tool-calls

Tool call log (JSON array).

#### GET /v1/bench/runs/{id}/timeline

Decision timeline — phases: discover, diagnose, decide, act, verify.

#### GET /v1/bench/runs/{id}/scorecard

Scorecard data (JSON).

#### GET /v1/bench/stats

Aggregate statistics. Same filters as runs list, including the `evidence_mode` contract above.

#### GET /v1/bench/catalog

Distinct models and providers.

#### GET /v1/bench/models

List models available to the authenticated tenant.

A model is returned when either:
- the platform has configured a global `api_key_env` for it, or
- the tenant has an enabled override in `bench_tenant_providers`.

Response:
```json
{
  "models": [
    {
      "id": "gemini-2.5-flash",
      "display_name": "Gemini 2.5 Flash",
      "provider": "google",
      "api_base_url": "https://generativelanguage.googleapis.com/v1beta/openai",
      "available": true,
      "input_cost_per_mtok": 0.15,
      "output_cost_per_mtok": 0.6
    }
  ]
}
```

The `available` field indicates whether the server has an API key configured for the model
(checked via `os.Getenv(api_key_env)` at request time). The UI uses this to show only
usable models.

#### PUT /v1/bench/models/{model_id}/provider

> **Not yet enabled.** Per-tenant API key storage requires AES-256-GCM encryption
> (tracked as a prerequisite for SaaS). Currently all tenants share platform-level
> credentials configured via environment variables.

Create or update a tenant-specific provider override for a model.

Request body:
```json
{
  "api_key": "sk-secret",
  "api_base_url": "https://gateway.example.com/v1",
  "rate_limit": 10,
  "monthly_budget": 100
}
```

Response: `204 No Content`

Errors:
- `400` — invalid JSON
- `500` — provider update failed

#### DELETE /v1/bench/models/{model_id}/provider

> **Not yet enabled.** See PUT above.

Delete the tenant-specific provider override for a model.

Response: `204 No Content`

#### PUT /v1/admin/bench/models/{model_id}

Invite-gated administrative route for updating platform-level model defaults.
This route uses `X-Invite-Secret`, not Bearer auth.

**Headers:**

| Header | Required | Description |
|---|---|---|
| `X-Invite-Secret` | Yes | Must match the server invite secret |

Request body:
```json
{
  "api_base_url": "https://gateway.example.com/v1",
  "api_key_env": "CUSTOM_API_KEY"
}
```

Response: `204 No Content`

Errors:
- `400` — invalid JSON
- `403` — missing or invalid invite secret
- `500` — update failed
- `503` — invite secret not configured

#### GET /v1/bench/compare/runs

Compare two runs side-by-side with computed delta.

Query params: `a` (run ID), `b` (run ID) — both required.

Response:
```json
{
  "run_a": { "id": "...", "model": "sonnet", "passed": true, ... },
  "run_b": { "id": "...", "model": "gpt-5.2", "passed": true, ... },
  "delta": {
    "passed_changed": false,
    "duration_diff_seconds": -12.5,
    "turns_diff": -3,
    "cost_diff_usd": -0.15,
    "tokens_diff": -2400,
    "checks_passed_diff": 0
  }
}
```

#### GET /v1/bench/compare/models

Compare models across scenarios. Two modes:

**Pairwise:** `?a=claude-sonnet-4&b=gpt-5.2` — compare two models.

**Matrix:** `?models=claude-sonnet-4,gpt-5.2,gemini-2.5-flash&scenarios=broken-deployment,crashloop-backoff` — multi-model comparison grid. `scenarios` is optional (all if omitted).

Both pairwise and matrix modes honor `evidence_mode` with the same contract as leaderboard/runs/stats.

Matrix response:
```json
{
  "models": ["claude-sonnet-4", "gpt-5.2"],
  "scenarios": ["broken-deployment", "crashloop-backoff"],
  "cells": {
    "broken-deployment": {
      "claude-sonnet-4": { "runs": 5, "passed": 5, "pass_rate": 100, "avg_cost": 0.03, "avg_tokens": 1200, "avg_duration": 45.2 },
      "gpt-5.2": { "runs": 3, "passed": 3, "pass_rate": 100, "avg_cost": 0.02, "avg_tokens": 900, "avg_duration": 32.1 }
    }
  }
}
```

#### GET /v1/bench/signals

Aggregated signal counts across runs. Parses scorecard artifacts.

Query params: `evidence_mode` (`""` = all, `none` = baseline only, `evidra` = non-`none`, other non-empty values match stored modes exactly), `since` (RFC3339).
Query params: `evidence_mode` (`""` = all, `none` = baseline only, `evidra` = non-`none`, other non-empty values match stored modes exactly), `since` (RFC3339).

Response:
```json
{
  "total_runs": 825,
  "runs_with_scorecard": 340,
  "signals": {
    "protocol_violation": { "total": 12, "run_count": 8 },
    "retry_loop": { "total": 5, "run_count": 3 }
  },
  "avg_score": 87.5
}
```

#### GET /v1/bench/regressions

Detects scenario/model pairs where the latest run failed but previous runs passed.

Response:
```json
[
  {
    "scenario_id": "crashloop-backoff",
    "model": "gpt-4o",
    "latest_run_id": "20260323-...",
    "latest_passed": false,
    "prev_passed": 8,
    "prev_total": 10,
    "prev_rate": 80.0,
    "severity": "critical"
  }
]
```

#### GET /v1/bench/insights

Failure analysis for a specific scenario. Requires `?scenario=<id>`.

Response:
```json
{
  "scenario_id": "network-policy-fix",
  "total_runs": 20,
  "failed_runs": 8,
  "passed_runs": 12,
  "check_failures": [
    { "check_name": "service-reachable", "check_type": "command-succeeds", "fail_count": 8, "fail_rate": 100 }
  ],
  "model_breakdown": [
    { "model": "gpt-4o", "runs": 5, "passed": 2, "failed": 3, "rate": 40.0 }
  ],
  "behavior_metrics": {
    "pass_avg_turns": 8.2,
    "fail_avg_turns": 14.5,
    "pass_avg_duration": 35.1,
    "fail_avg_duration": 62.3
  }
}
```

### Bench Trigger

Start and monitor benchmark runs via either a direct executor or the persisted
runner queue.
See [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md) for
implementing custom executors, and
[Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
for the poll-based runner surface.

#### POST /v1/bench/trigger

Start a benchmark run. Requires `model`, `scenarios`, and `evidence_mode` in the request body. `evidence_mode` accepts only `none` or `smart`. Returns a job ID for progress tracking. When a healthy runner is available, Evidra may enqueue the job instead of starting a direct executor immediately.

Request:
```json
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "evidence_mode": "smart",
  "runner_id": "01K...",
  "scenarios": ["broken-deployment", "repair-loop-escalation"]
}
```

Response (`202 Accepted`, direct executor):
```json
{ "id": "job_01JD...", "status": "pending" }
```

Response (`202 Accepted`, queued for poll-based runner):
```json
{ "id": "job_01JD...", "status": "pending", "mode": "runner" }
```

Errors: `400` (missing model/scenarios, or unavailable pinned runner), `401` (unauthorized), `501` (no executor configured and no eligible runner).

#### GET /v1/bench/trigger/{id}

Get current job state. Supports SSE streaming when `Accept: text/event-stream` is set.

Response (`200 OK`):
```json
{
  "id": "job_01JD...",
  "status": "running",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "completed": 2,
  "passed": 1,
  "failed": 1,
  "total": 5,
  "current_scenario": "repair-loop-escalation",
  "run_ids": ["run_01..."],
  "progress": [
    { "scenario": "broken-deployment", "status": "passed", "run_id": "run_01..." },
    { "scenario": "repair-loop-escalation", "status": "running" }
  ],
  "created_at": "2026-03-26T12:00:00Z"
}
```

#### POST /v1/bench/trigger/{id}/progress

Webhook called by the bench executor or runner bridge to report scenario completion.

Request:
```json
{
  "contract_version": "v1.0.0",
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "run_01...",
  "completed": 1,
  "total": 5
}
```

Response: `200 OK`.

### Runner Control Plane

Poll-based runners register capabilities, claim queued jobs, and complete them
through `/v1/runners/*`.

#### POST /v1/runners/register

Register a runner and advertise its supported models.

Request:
```json
{
  "name": "eu-west-runner-a",
  "models": ["deepseek-chat", "qwen-plus"],
  "provider": "bifrost",
  "region": "eu-west",
  "max_parallel": 2,
  "labels": { "cluster": "kind-a" }
}
```

Response (`201 Created`):
```json
{ "runner_id": "01K...", "poll_interval": 5 }
```

#### GET /v1/runners

List registered runners for the authenticated tenant.

Response (`200 OK`):
```json
{
  "runners": [
    {
      "id": "01K...",
      "tenant_id": "default",
      "name": "eu-west-runner-a",
      "region": "eu-west",
      "status": "healthy",
      "config": {
        "models": ["deepseek-chat", "qwen-plus"],
        "provider": "bifrost",
        "max_parallel": 2,
        "poll_interval": 5,
        "labels": { "cluster": "kind-a" }
      },
      "created_at": "2026-03-26T12:00:00Z",
      "updated_at": "2026-03-26T12:00:05Z"
    }
  ]
}
```

#### DELETE /v1/runners/{id}

Delete a runner registration. Response: `204 No Content`.

#### GET /v1/runners/jobs

Runner poll endpoint. Requires `runner_id` as a query parameter. Polling also
acts as the runner heartbeat. claimed job payload includes `evidence_mode`.

Response when a job is available (`200 OK`):
```json
{
  "job_id": "01K...",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "evidence_mode": "smart",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "timeout": 300
}
```

Response when no job is available: `204 No Content`.

Errors: `400` (missing `runner_id`), `404` (runner not found or not healthy).

#### POST /v1/runners/jobs/{id}/complete

Mark a claimed job as complete.

Request:
```json
{
  "runner_id": "01K...",
  "status": "completed",
  "passed": 2,
  "failed": 0,
  "message": ""
}
```

Response: `204 No Content`.

#### POST /v1/bench/scenarios/sync

Upsert scenario metadata. Used by `infra-bench scenario push`.

Request:
```json
{
  "scenarios": [
    { "id": "broken-deployment", "title": "Fix a broken deployment", "category": "kubernetes", "tags": ["deployment", "image"], "chaos": false, "evidra": true }
  ]
}
```

Response:
```json
{ "ok": true, "upserted": 62, "total": 62 }
```

---

## Error Format

All errors return JSON:

```json
{ "error": "human-readable message" }
```

Common status codes:
- `400` — bad request (invalid params or body)
- `401` — missing or invalid auth token
- `403` — forbidden (invalid invite secret)
- `404` — resource not found
- `429` — rate limit exceeded
- `500` — internal server error
