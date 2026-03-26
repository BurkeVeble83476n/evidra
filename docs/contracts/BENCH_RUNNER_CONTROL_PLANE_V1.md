# Evidra Bench Runner Control Plane Contract v1

Specification for poll-based benchmark runners that register with `evidra-api`,
claim persisted jobs, and report final completion.

## Scope

This contract covers the V2b runner control plane:

- runner registration in `bench_infra`
- persisted job queue in `bench_jobs`
- poll/claim/complete HTTP routes under `/v1/runners/*`

It does not replace the direct executor contract. The V1 executor flow is still
documented in [Executor Contract v1.0.0](EXECUTOR_CONTRACT_V1.md).

## Lifecycle

1. Runner registers capabilities with `POST /v1/runners/register`
2. Control plane enqueues work through `POST /v1/bench/trigger`
3. Runner polls `GET /v1/runners/jobs?runner_id=...`
4. Runner executes assigned scenarios
5. Runner reports final status through `POST /v1/runners/jobs/{id}/complete`
6. Optional trigger compatibility progress continues through `POST /v1/bench/trigger/{id}/progress`

## Trigger Enqueue

### POST /v1/bench/trigger

Request:

```json
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "runner_id": "01K...",
  "scenarios": ["broken-deployment", "repair-loop-escalation"]
}
```

Notes:

- `runner_id` is optional. When present, the job is pinned to that runner.
- If `runner_id` is present, the control plane rejects the request unless that
  runner is healthy and advertises the requested model.
- If no healthy runner is available and no direct executor is configured,
  Evidra returns `501 Not Implemented`.

Runner-mode response:

```json
{
  "id": "01K...",
  "status": "pending",
  "mode": "runner"
}
```

## Runner Registration

### POST /v1/runners/register

Request:

```json
{
  "name": "eu-west-runner-a",
  "models": ["deepseek-chat", "qwen-plus"],
  "provider": "bifrost",
  "region": "eu-west",
  "max_parallel": 2,
  "labels": {
    "cluster": "kind-a"
  }
}
```

Response:

```json
{
  "runner_id": "01K...",
  "poll_interval": 5
}
```

### GET /v1/runners

Response:

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
        "labels": {
          "cluster": "kind-a"
        }
      },
      "created_at": "2026-03-26T12:00:00Z",
      "updated_at": "2026-03-26T12:00:05Z"
    }
  ]
}
```

### DELETE /v1/runners/{id}

Response: `204 No Content`

## Poll and Claim

### GET /v1/runners/jobs?runner_id={runner_id}

Semantics:

- polling is also the runner heartbeat
- only healthy runners can poll successfully
- claim uses `SELECT ... FOR UPDATE SKIP LOCKED`
- pinned jobs can only be claimed by the pinned runner
- if no matching job is available, Evidra returns `204 No Content`

Response when a job is claimed:

```json
{
  "job_id": "01K...",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "timeout": 300
}
```

## Completion

### POST /v1/runners/jobs/{id}/complete

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

Rules:

- `status` must be `completed` or `failed`
- `runner_id` is required
- the completing runner must still own the job (`bench_jobs.infra_id`)

Response: `204 No Content`

## Progress and Staleness

- `POST /v1/bench/trigger/{id}/progress` updates in-memory trigger state for UI polling/SSE
- the same callback also updates `bench_jobs.last_progress_at`
- the janitor can re-queue claimed jobs whose `last_progress_at` or `started_at`
  exceeds the stale threshold
- runners marked unhealthy stop receiving new work and must re-register
