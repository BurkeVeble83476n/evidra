# Bench Service Integration Design

**Date:** 2026-03-25
**Status:** Approved

## Goal

Connect Evidra self-hosted to the private benchmark framework via a
REST service interface. Users trigger certification runs from Evidra
UI, the private bench service executes scenarios, and results flow
back to Evidra's existing bench and evidence pages.

## Architecture

```
Evidra UI          Evidra API              Bench Service (private)
─────────          ──────────              ───────────────────────

"Run" button  →  POST /v1/bench/trigger
                   → calls bench service  →  POST /v1/certify
                   → creates job            { model, scenarios,
                   → returns { id }           evidra_url, api_key }
                                                    ↓
SSE stream    ←  GET /v1/bench/trigger/{id}   Runs scenario 1...
  "1/5 ✓"    ←  ← POST .../progress          → POST /v1/evidence/forward
                                                → POST /v1/bench/runs
  "2/5 ✓"    ←  ← POST .../progress          Runs scenario 2...
                                                → POST /v1/evidence/forward
  "3/5 ..."  ←  ← POST .../progress            → POST /v1/bench/runs
                                                ...
  "5/5 ✓"    ←  ← POST .../complete          Done

redirect to    /bench/runs?model=X            All data already in Evidra
  results      /evidence
               /bench (leaderboard)
```

## New Evidra API Endpoints

### POST /v1/bench/trigger

Starts a remote benchmark run.

```json
Request:
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"]
}

Response:
{
  "id": "trigger-01KMH...",
  "status": "pending"
}
```

Internally: calls `EVIDRA_BENCH_SERVICE_URL/v1/certify` with
the request + Evidra's own URL and API key for callbacks.

### GET /v1/bench/trigger/{id}

SSE stream of progress events.

```
event: progress
data: {"scenario":"broken-deployment","status":"running","completed":0,"total":5}

event: progress
data: {"scenario":"broken-deployment","status":"passed","completed":1,"total":5,"run_id":"20260325-..."}

event: progress
data: {"scenario":"repair-loop-escalation","status":"running","completed":1,"total":5}

event: complete
data: {"completed":5,"total":5,"passed":4,"failed":1,"run_ids":["20260325-...","..."]}
```

### POST /v1/bench/trigger/{id}/progress (webhook)

Called by the bench service to update progress.

```json
Request:
{
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260325-broken-deployment-deepseek"
}
```

Pushes to the SSE stream for that trigger ID.

## Bench Service API (evidra-stand, private)

### POST /v1/certify

```json
Request:
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "callback_url": "https://evidra-api:8080/v1/bench/trigger/trigger-01KMH.../progress",
  "evidra_url": "https://evidra-api:8080",
  "evidra_api_key": "ev1_..."
}
```

The bench service:
1. For each scenario:
   - Seeds the cluster
   - Runs kagent with evidra-mcp configured to forward to `evidra_url`
   - Evidence entries flow to Evidra during execution
   - Verifies the outcome
   - Submits bench run to `evidra_url/v1/bench/runs`
   - Calls `callback_url` with progress
2. On completion, calls callback with `status: complete`

## Evidra UI Changes

### /bench page — add "Run" button

Small addition to BenchLeaderboard or BenchDashboard:
- "Run Certification" button
- Modal: select model, provider, scenarios (checkboxes)
- Submit → POST /v1/bench/trigger
- Progress overlay: scenario list with checkmarks
- On complete → redirect to /bench/runs filtered by run IDs

### No new results pages

All results displayed on existing pages:
- `/bench` — leaderboard updates with new results
- `/bench/runs` — new runs appear, filterable
- `/bench/runs/{id}` — full detail with scorecard, signals, timeline
- `/evidence` — evidence entries from the run

## Configuration

```
EVIDRA_BENCH_SERVICE_URL=https://bench.internal:8090
```

When not set, the "Run" button is hidden. Evidra works without the
bench service — results can still be submitted via API.

## What Stays Private

- Scenario YAML manifests (in evidra-stand)
- Verification check logic
- Cluster management (Kind/k3d)
- Agent execution orchestration
- Bench service deployment

## What's Public (in Evidra)

- Bench run results (submitted via API)
- Evidence entries (forwarded during execution)
- Scorecards and signals (computed by Evidra)
- Leaderboard and comparison views
- Trigger endpoint (thin proxy)

## Implementation Order

1. Bench service: `POST /v1/certify` endpoint in evidra-stand
2. Evidra API: `POST /v1/bench/trigger` + webhook + SSE
3. Evidra UI: "Run" button + progress overlay
4. Wire: EVIDRA_BENCH_SERVICE_URL config
5. Test end-to-end
