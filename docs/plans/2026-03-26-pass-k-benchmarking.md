# pass^k Benchmarking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add pass^k reliability metric to the leaderboard — measures consistency across multiple trials, not just single-attempt pass rate.

**Architecture:** Server-side computation in the leaderboard SQL query. Uses a CTE to compute per-scenario pass rates, then `POWER(pass_rate, k)` averaged across scenarios. k defaults to 3, configurable via `?k=N` query param. Only scenarios with >= k trials contribute to pass^k (insufficient-data guard). No schema changes — uses existing `bench_runs` data.

**Tech Stack:** Go (SQL aggregation), PostgreSQL, React/TypeScript

**Math:** For each (model, scenario) pair with >= k trials, compute `p = passed / total`. Then `pass^k = p^k`. Average across qualifying scenarios. Report as percentage.

**Example:** Model X, 3 scenarios, 3 trials each:
- Scenario A: 3/3 pass → p=1.0, p^3 = 1.0
- Scenario B: 2/3 pass → p=0.67, p^3 = 0.30
- Scenario C: 1/3 pass → p=0.33, p^3 = 0.036
- pass^3 = avg(1.0, 0.30, 0.036) = 44.5%

---

### Task 1: Add pass^k fields to LeaderboardEntry type

**Files:**
- Modify: `pkg/bench/types.go:10-18`

**Step 1: Add fields to LeaderboardEntry**

```go
type LeaderboardEntry struct {
	Model              string  `json:"model"`
	Scenarios          int     `json:"scenarios"`
	Runs               int     `json:"runs"`
	PassRate           float64 `json:"pass_rate"`
	AvgDuration        float64 `json:"avg_duration"`
	AvgCost            float64 `json:"avg_cost"`
	TotalCost          float64 `json:"total_cost"`
	PassK              float64 `json:"pass_k"`              // pass^k reliability (0-100)
	PassKTrials        int     `json:"pass_k_trials"`        // k value used
	SufficientScenarios int    `json:"sufficient_scenarios"` // scenarios with >= k trials
}
```

**Step 2: Write test**

```go
func TestLeaderboardEntryHasPassKFields(t *testing.T) {
	t.Parallel()
	e := LeaderboardEntry{
		PassK:              44.5,
		PassKTrials:        3,
		SufficientScenarios: 3,
	}
	if e.PassK != 44.5 {
		t.Fatalf("PassK = %v, want 44.5", e.PassK)
	}
	if e.PassKTrials != 3 {
		t.Fatalf("PassKTrials = %v, want 3", e.PassKTrials)
	}
}
```

**Step 3: `gofmt -w .` and test**

```bash
go test ./pkg/bench/ -count=1
```

**Step 4: Commit**

```bash
git add pkg/bench/types.go pkg/bench/types_test.go
git commit -s -m "feat(bench): add PassK reliability fields to LeaderboardEntry"
```

---

### Task 2: Compute pass^k in leaderboard SQL query

**Files:**
- Modify: `internal/benchsvc/leaderboard.go`
- Modify: `internal/benchsvc/service.go` (Leaderboard signature adds k param)
- Modify: `internal/benchsvc/handlers.go` (parse ?k= query param)

**Step 1: Update Leaderboard signature**

In `service.go`, Repository interface:
```go
Leaderboard(ctx context.Context, tenantID string, evidenceMode string, k int) ([]bench.LeaderboardEntry, error)
```

Service method:
```go
func (s *Service) Leaderboard(ctx context.Context, evidenceMode string, k int) ([]bench.LeaderboardEntry, error) {
	if s.cfg.PublicTenant == "" {
		return nil, ErrPublicTenantUnavailable
	}
	return s.repo.Leaderboard(ctx, s.cfg.PublicTenant, evidenceMode, k)
}
```

**Step 2: Rewrite leaderboard.go query with CTE**

```go
func (s *PgStore) Leaderboard(ctx context.Context, tenantID string, evidenceMode string, k int) ([]bench.LeaderboardEntry, error) {
	if k < 1 {
		k = 3
	}

	query := `
		WITH per_scenario AS (
			SELECT
				model,
				scenario_id,
				COUNT(*) AS trials,
				AVG(CASE WHEN passed THEN 1.0 ELSE 0.0 END) AS pass_rate,
				AVG(duration_seconds) AS avg_duration,
				AVG(estimated_cost_usd) AS avg_cost,
				SUM(estimated_cost_usd) AS total_cost
			FROM bench_runs
			WHERE tenant_id = $1 AND archived_at IS NULL`

	args := []any{tenantID}
	argN := 2

	if evidenceMode != "" {
		query += fmt.Sprintf(` AND evidence_mode = $%d`, argN)
		args = append(args, evidenceMode)
		argN++
	}

	query += fmt.Sprintf(`
			GROUP BY model, scenario_id
		)
		SELECT
			model,
			COUNT(*) AS scenarios,
			SUM(trials)::int AS runs,
			100.0 * AVG(pass_rate) AS pass_rate,
			AVG(avg_duration) AS avg_duration,
			AVG(avg_cost) AS avg_cost,
			SUM(total_cost) AS total_cost,
			COALESCE(100.0 * AVG(CASE WHEN trials >= $%d THEN POWER(pass_rate, $%d) END), 0) AS pass_k,
			$%d AS pass_k_trials,
			COUNT(CASE WHEN trials >= $%d THEN 1 END)::int AS sufficient_scenarios
		FROM per_scenario
		GROUP BY model
		ORDER BY pass_rate DESC, model`, argN, argN, argN, argN)
	args = append(args, k)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: %w", err)
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (bench.LeaderboardEntry, error) {
		var e bench.LeaderboardEntry
		err := row.Scan(&e.Model, &e.Scenarios, &e.Runs, &e.PassRate,
			&e.AvgDuration, &e.AvgCost, &e.TotalCost,
			&e.PassK, &e.PassKTrials, &e.SufficientScenarios)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: collect: %w", err)
	}
	return entries, nil
}
```

**Step 3: Parse ?k= in handler**

In `handleLeaderboard`:
```go
mode := r.URL.Query().Get("evidence_mode")
k := 3
if kStr := r.URL.Query().Get("k"); kStr != "" {
	if kVal, err := strconv.Atoi(kStr); err == nil && kVal >= 1 && kVal <= 10 {
		k = kVal
	}
}
entries, err := svc.Leaderboard(r.Context(), mode, k)
```

**Step 4: Update all mock repos**

Update Leaderboard method signature in:
- `handlerRepo` in `handlers_test.go`
- `fakeRepo` in `service_test.go`
- `routerBenchRepo` in `router_test.go`

**Step 5: `gofmt -w .` and test**

```bash
gofmt -w .
go test ./internal/benchsvc/ -count=1
go test ./internal/api/ -count=1
```

**Step 6: Commit**

```bash
git add internal/benchsvc/leaderboard.go internal/benchsvc/service.go \
  internal/benchsvc/handlers.go internal/benchsvc/handlers_test.go \
  internal/benchsvc/service_test.go internal/api/router_test.go
git commit -s -m "feat(bench): compute pass^k in leaderboard with configurable k and minimum-trials guard"
```

---

### Task 3: Add pass^k column to evidra leaderboard UI

**Files:**
- Modify: `ui/src/pages/bench/BenchLeaderboard.tsx`

**Step 1: Update TypeScript interface**

```typescript
interface LeaderboardEntry {
  model: string;
  scenarios: number;
  runs: number;
  pass_rate: number;
  avg_duration: number;
  avg_cost: number;
  total_cost: number;
  pass_k: number;
  pass_k_trials: number;
  sufficient_scenarios: number;
}
```

**Step 2: Add column to table**

After Pass Rate column header:
```tsx
<th>Reliability (pass^k)</th>
```

In the row:
```tsx
<td>
  <span className={`font-semibold ${
    m.pass_k >= 70 ? "text-green-400" :
    m.pass_k >= 40 ? "text-accent" :
    m.pass_k >= 20 ? "text-yellow-400" :
    "text-red-400"
  }`}>
    {m.pass_k.toFixed(1)}%
  </span>
  <span className="text-[0.65rem] text-fg-muted ml-1">
    k={m.pass_k_trials}, {m.sufficient_scenarios}/{m.scenarios} scenarios
  </span>
</td>
```

**Step 3: Build and verify**

```bash
cd ui && npm run build
```

**Step 4: Commit**

```bash
git add ui/src/pages/bench/BenchLeaderboard.tsx
git commit -s -m "feat(ui): add pass^k reliability column to leaderboard"
```

---

### Task 4: Add pass^k to evidra-bench leaderboard UI

**Files:**
- Modify: `/Users/vitas/git/evidra-bench/ui/src/pages/bench/Leaderboard.tsx`

Mirror the changes from Task 3 in the evidra-bench leaderboard component.

**Commit (in evidra-bench repo):**

```bash
git add ui/src/pages/bench/Leaderboard.tsx
git commit -s -m "feat(ui): add pass^k reliability column to bench leaderboard"
```

---

### Task 5: Update OpenAPI spec and docs

**Files:**
- Modify: `cmd/evidra-api/static/openapi.yaml`
- Copy: `cp cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml`
- Modify: `docs/api-reference.md`

**Step 1: Add pass_k fields to LeaderboardEntry schema in openapi.yaml**

```yaml
pass_k:
  type: number
  description: "pass^k reliability — probability all k trials pass per scenario, averaged across qualifying scenarios (0-100)"
pass_k_trials:
  type: integer
  description: "k value used for pass^k computation"
sufficient_scenarios:
  type: integer
  description: "Number of scenarios with >= k trials (sufficient data for pass^k)"
```

**Step 2: Add ?k= query param to leaderboard endpoint**

```yaml
- name: k
  in: query
  schema:
    type: integer
    minimum: 1
    maximum: 10
    default: 3
  description: "Number of trials for pass^k reliability computation"
```

**Step 3: Update api-reference.md response example**

Add pass_k, pass_k_trials, sufficient_scenarios to the leaderboard example.

**Step 4: Copy and commit**

```bash
cp cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml
git add cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml docs/api-reference.md
git commit -s -m "docs: add pass^k to leaderboard OpenAPI spec and API reference"
```

---

## Dependency Graph

```
Task 1 (types) → Task 2 (SQL + handler + mocks)
                    → Task 3 (evidra UI)
                    → Task 4 (bench UI)
                    → Task 5 (docs)
```

Tasks 3, 4, 5 can run in parallel after Task 2.
