package benchsvc

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra/pkg/bench"
)

// Leaderboard returns aggregate stats per model, optionally filtered by evidence mode.
func (s *PgStore) Leaderboard(ctx context.Context, tenantID string, evidenceMode string) ([]bench.LeaderboardEntry, error) {
	query := `
		SELECT model,
			COUNT(DISTINCT scenario_id) AS scenarios,
			COUNT(*) AS runs,
			100.0 * SUM(CASE WHEN passed THEN 1 ELSE 0 END) / COUNT(*) AS pass_rate,
			AVG(duration_seconds) AS avg_duration,
			AVG(estimated_cost_usd) AS avg_cost,
			SUM(estimated_cost_usd) AS total_cost
		FROM bench_runs
		WHERE tenant_id = $1`

	args := []any{tenantID}

	if evidenceMode != "" {
		query += ` AND evidence_mode = $2`
		args = append(args, evidenceMode)
	}

	query += `
		GROUP BY model
		ORDER BY pass_rate DESC, model`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: %w", err)
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (bench.LeaderboardEntry, error) {
		var e bench.LeaderboardEntry
		err := row.Scan(&e.Model, &e.Scenarios, &e.Runs, &e.PassRate,
			&e.AvgDuration, &e.AvgCost, &e.TotalCost)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: collect: %w", err)
	}
	return entries, nil
}
