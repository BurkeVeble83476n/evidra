# Evidra Bench Executor Contract v1.0.0

Specification for benchmark executors that plug into Evidra's bench
trigger system.

## Overview

An executor runs benchmark scenarios against real infrastructure and
reports results back to Evidra. Evidra provides the analytics
(scorecards, signals, leaderboards). The executor provides the
execution (clusters, agents, verification).

Any service implementing this contract can be used with Evidra's
`POST /v1/bench/trigger` endpoint.

## Contract Version

All requests and callbacks include `contract_version: "v1.0.0"`.
Evidra validates the version and rejects unsupported versions.

New fields are additive (backward compatible within a major version).
Breaking changes increment the major version.

## Endpoints the Executor Must Implement

### POST /v1/certify

Start a benchmark run.

**Request:**

```json
{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "config": {
    "timeout_per_scenario": 300,
    "adapter": "kagent"
  },
  "callback": {
    "progress_url": "https://evidra:8080/v1/bench/trigger/trigger-01KMH.../progress",
    "evidra_url": "https://evidra:8080",
    "evidra_api_key": "ev1_..."
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `contract_version` | string | yes | Must be `"v1.0.0"` |
| `job_id` | string | yes | Unique job ID assigned by Evidra |
| `model` | string | yes | LLM model name |
| `provider` | string | no | LLM provider name |
| `scenarios` | string[] | yes | Scenario IDs to run |
| `config.timeout_per_scenario` | int | no | Timeout in seconds per scenario (default: 300) |
| `config.adapter` | string | no | Agent adapter type (default: executor's choice) |
| `callback.progress_url` | string | yes | Webhook URL for progress updates |
| `callback.evidra_url` | string | yes | Evidra API base URL for data delivery |
| `callback.evidra_api_key` | string | yes | Bearer token for Evidra API auth |

**Response:**

```json
{
  "job_id": "trigger-01KMH...",
  "status": "accepted"
}
```

Status `202 Accepted`. The executor begins execution asynchronously.

### GET /healthz

Health check. Returns `200 OK` with `{"status": "ok"}`.

## Callbacks the Executor Must Send

During execution, the executor calls back to Evidra with progress.

### Progress Update

Called after each scenario starts or completes.

```
POST {callback.progress_url}
Content-Type: application/json
```

```json
{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260325-broken-deployment-deepseek",
  "completed": 1,
  "total": 5
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `contract_version` | string | yes | `"v1.0.0"` |
| `job_id` | string | yes | Job ID from the original request |
| `scenario` | string | yes | Current scenario ID |
| `status` | string | yes | `running`, `passed`, `failed`, `error`, `skipped` |
| `run_id` | string | no | Bench run ID (set after scenario completes) |
| `completed` | int | yes | Number of scenarios completed so far |
| `total` | int | yes | Total number of scenarios |

Send `status: "running"` before starting a scenario.
Send `status: "passed"` or `"failed"` after it completes.

The final callback (where `completed == total`) signals job completion.

## Data Delivery

During and after execution, the executor pushes results to Evidra
using standard APIs. Authentication: `Bearer {callback.evidra_api_key}`.

### Evidence Entries

During scenario execution, the agent's MCP server (evidra-mcp)
forwards evidence entries to Evidra:

```
POST {callback.evidra_url}/v1/evidence/forward
Authorization: Bearer {callback.evidra_api_key}
Content-Type: application/json

<raw evidence entry JSON>
```

This happens automatically when evidra-mcp is configured with
`--url {evidra_url} --api-key {evidra_api_key}`.

### Bench Run Results

After each scenario completes, the executor submits the run result:

```
POST {callback.evidra_url}/v1/bench/runs
Authorization: Bearer {callback.evidra_api_key}
Content-Type: application/json

{
  "id": "20260325-broken-deployment-deepseek",
  "scenario_id": "broken-deployment",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "adapter": "kagent",
  "evidence_mode": "direct",
  "passed": true,
  "duration_seconds": 35.2,
  "exit_code": 0,
  "turns": 8,
  "checks_passed": 3,
  "checks_total": 3
}
```

The `id` field must be unique. Recommended format:
`{timestamp}-{scenario}-{model}` (e.g. `20260325-143022-broken-deployment-deepseek`).

### Scenario Metadata (Optional)

Before the first run, the executor may sync scenario metadata:

```
POST {callback.evidra_url}/v1/bench/scenarios/sync
Authorization: Bearer {callback.evidra_api_key}
Content-Type: application/json

{
  "scenarios": [
    {
      "id": "broken-deployment",
      "title": "Broken Deployment",
      "category": "kubernetes",
      "tags": ["image-pull", "deployment"]
    }
  ]
}
```

## Execution Requirements

### Per Scenario

The executor must:

1. Prepare the target environment (seed the failure scenario)
2. Launch the agent with the specified model
3. Configure evidra-mcp to forward evidence to `callback.evidra_url`
4. Wait for the agent to complete (or timeout)
5. Verify the outcome (run checks)
6. Submit the bench run result to Evidra
7. Send a progress callback

### Error Handling

- If a scenario fails to start: send `status: "error"` callback
- If a scenario times out: send `status: "failed"` callback
- If the executor crashes: Evidra's trigger will time out and mark
  the job as failed (no callback needed)

### Concurrency

The executor may run one job at a time. If a second `POST /v1/certify`
arrives while a job is running, return `409 Conflict`.

## Configuration

The executor is registered with Evidra via environment variable:

```
EVIDRA_BENCH_SERVICE_URL=https://bench-service.internal:8090
```

When set, Evidra's `POST /v1/bench/trigger` calls the executor.
When not set, the trigger endpoint returns `501 Not Implemented`.

## Reference Implementation

The reference executor is `infra-bench serve` in the evidra-stand
repository:

```bash
BENCH_SERVICE_ADDR=:8090 infra-bench serve
```

## Third-Party Implementations

Any service implementing `POST /v1/certify` and the callback
contract can be used as an executor:

- **kagent team** — benchmarks kagent against their own scenarios
- **Platform teams** — runs company-specific compliance scenarios
- **Security vendors** — adversarial agent testing
- **CI/CD integrations** — automated regression testing
