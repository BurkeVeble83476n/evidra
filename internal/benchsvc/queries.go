package benchsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra/pkg/bench"
)

// runRecordColumns is the SELECT column list for RunRecord scans.
const runRecordColumns = `id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
	passed, duration_seconds, exit_code, turns, memory_window,
	prompt_tokens, completion_tokens, estimated_cost_usd,
	checks_passed, checks_total, checks_json, metadata_json, created_at`

// scanRunRecord scans a row into a bench.RunRecord.
func scanRunRecord(row pgx.CollectableRow) (bench.RunRecord, error) {
	var r bench.RunRecord
	var checksJSON, metadataJSON *string
	err := row.Scan(
		&r.ID, &r.TenantID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.EvidenceMode,
		&r.Passed, &r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow,
		&r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost,
		&r.ChecksPassed, &r.ChecksTotal, &checksJSON, &metadataJSON, &r.CreatedAt,
	)
	if err != nil {
		return r, err
	}
	if checksJSON != nil {
		r.ChecksJSON = *checksJSON
	}
	if metadataJSON != nil {
		r.MetadataJSON = *metadataJSON
	}
	return r, nil
}

// ListRuns returns runs matching filters with pagination (total count + page).
func (s *PgStore) ListRuns(ctx context.Context, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	where, args := s.buildWhere(f)

	// Count total.
	var total int
	countQ := "SELECT COUNT(*) FROM bench_runs" + where
	if err := s.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("bench.ListRuns: count: %w", err)
	}

	// Fetch page.
	orderCol := "created_at"
	validSortColumns := map[string]bool{
		"created_at": true, "duration_seconds": true, "estimated_cost_usd": true,
		"scenario_id": true, "model": true, "provider": true,
		"checks_passed": true, "turns": true, "passed": true,
	}
	if f.SortBy != "" && validSortColumns[f.SortBy] {
		orderCol = f.SortBy
	}
	orderDir := "DESC"
	if f.SortOrder == "asc" {
		orderDir = "ASC"
	}

	query := "SELECT " + runRecordColumns + " FROM bench_runs" + where +
		fmt.Sprintf(" ORDER BY %s %s", orderCol, orderDir)
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if f.Offset > 0 {
		args = append(args, f.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("bench.ListRuns: %w", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, scanRunRecord)
	if err != nil {
		return nil, 0, fmt.Errorf("bench.ListRuns: collect: %w", err)
	}
	return records, total, nil
}

// GetRun returns a single run by ID, scoped to the store's tenant.
func (s *PgStore) GetRun(ctx context.Context, id string) (*bench.RunRecord, error) {
	query := "SELECT " + runRecordColumns + " FROM bench_runs WHERE tenant_id = $1 AND id = $2"
	rows, err := s.db.Query(ctx, query, s.tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("bench.GetRun: %w", err)
	}
	defer rows.Close()

	r, err := pgx.CollectExactlyOneRow(rows, scanRunRecord)
	if err != nil {
		return nil, fmt.Errorf("bench.GetRun: %w", err)
	}
	return &r, nil
}

// InsertRun inserts a single benchmark run record.
func (s *PgStore) InsertRun(ctx context.Context, r bench.RunRecord) error {
	query := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`

	checksJSON := nullableJSONB(r.ChecksJSON)
	metadataJSON := nullableJSONB(r.MetadataJSON)
	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := s.db.Exec(ctx, query,
		r.ID, s.tenantID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.EvidenceMode,
		r.Passed, r.Duration, r.ExitCode, r.Turns, r.MemoryWindow,
		r.PromptTokens, r.CompletionTokens, r.EstimatedCost,
		r.ChecksPassed, r.ChecksTotal, checksJSON, metadataJSON, createdAt,
	)
	if err != nil {
		return fmt.Errorf("bench.InsertRun: %w", err)
	}
	return nil
}

// InsertRunBatch inserts multiple runs, skipping duplicates. Returns the number inserted.
func (s *PgStore) InsertRunBatch(ctx context.Context, runs []bench.RunRecord) (int, error) {
	if len(runs) == 0 {
		return 0, nil
	}

	query := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	ON CONFLICT (id) DO NOTHING`

	inserted := 0
	batch := &pgx.Batch{}
	for _, r := range runs {
		checksJSON := nullableJSONB(r.ChecksJSON)
		metadataJSON := nullableJSONB(r.MetadataJSON)
		createdAt := r.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		batch.Queue(query,
			r.ID, s.tenantID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.EvidenceMode,
			r.Passed, r.Duration, r.ExitCode, r.Turns, r.MemoryWindow,
			r.PromptTokens, r.CompletionTokens, r.EstimatedCost,
			r.ChecksPassed, r.ChecksTotal, checksJSON, metadataJSON, createdAt,
		)
	}

	br := s.db.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	for range runs {
		ct, err := br.Exec()
		if err != nil {
			return inserted, fmt.Errorf("bench.InsertRunBatch: %w", err)
		}
		inserted += int(ct.RowsAffected())
	}
	return inserted, nil
}

// FilteredStats returns aggregate statistics matching the given filters.
func (s *PgStore) FilteredStats(ctx context.Context, f bench.RunFilters) (*bench.StatsResult, error) {
	where, args := s.buildWhere(f)

	var st bench.StatsResult
	err := s.db.QueryRow(ctx,
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN passed THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN NOT passed THEN 1 ELSE 0 END),0) FROM bench_runs"+where,
		args...,
	).Scan(&st.TotalRuns, &st.PassCount, &st.FailCount)
	if err != nil {
		return nil, fmt.Errorf("bench.FilteredStats: %w", err)
	}

	rows, err := s.db.Query(ctx,
		"SELECT scenario_id, COUNT(*), SUM(CASE WHEN passed THEN 1 ELSE 0 END) FROM bench_runs"+where+" GROUP BY scenario_id ORDER BY scenario_id",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("bench.FilteredStats: by scenario: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ss bench.ScenarioStat
		if err := rows.Scan(&ss.ScenarioID, &ss.Runs, &ss.Passed); err != nil {
			return nil, fmt.Errorf("bench.FilteredStats: scan: %w", err)
		}
		st.ByScenario = append(st.ByScenario, ss)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bench.FilteredStats: rows: %w", err)
	}
	return &st, nil
}

// Catalog returns distinct models and providers (not implemented).
func (s *PgStore) Catalog(_ context.Context) (*bench.RunCatalog, error) {
	return nil, fmt.Errorf("bench.Catalog: not implemented")
}

// CompareRuns compares two runs (not implemented).
func (s *PgStore) CompareRuns(_ context.Context, _, _ string) (*bench.RunComparison, error) {
	return nil, fmt.Errorf("bench.CompareRuns: not implemented")
}

// ModelMatrix builds a model/scenario comparison grid (not implemented).
func (s *PgStore) ModelMatrix(_ context.Context, _, _ []string) (*bench.ModelMatrix, error) {
	return nil, fmt.Errorf("bench.ModelMatrix: not implemented")
}

// ListScenarios returns distinct scenarios (not implemented).
func (s *PgStore) ListScenarios(_ context.Context) ([]bench.ScenarioSummary, error) {
	return nil, fmt.Errorf("bench.ListScenarios: not implemented")
}

// SignalSummary aggregates signal counts (not implemented).
func (s *PgStore) SignalSummary(_ context.Context, _ bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, fmt.Errorf("bench.SignalSummary: not implemented")
}

// Regressions finds scenario/model regressions (not implemented).
func (s *PgStore) Regressions(_ context.Context) ([]bench.Regression, error) {
	return nil, fmt.Errorf("bench.Regressions: not implemented")
}

// FailureAnalysis computes failure patterns (not implemented).
func (s *PgStore) FailureAnalysis(_ context.Context, _ string) (*bench.FailureInsights, error) {
	return nil, fmt.Errorf("bench.FailureAnalysis: not implemented")
}

// buildWhere constructs a WHERE clause with numbered PostgreSQL placeholders.
// The tenant_id filter is always applied.
func (s *PgStore) buildWhere(f bench.RunFilters) (string, []any) {
	clauses := []string{"tenant_id = $1"}
	args := []any{s.tenantID}

	if f.ScenarioID != "" {
		args = append(args, f.ScenarioID)
		clauses = append(clauses, fmt.Sprintf("scenario_id = $%d", len(args)))
	}
	if f.Model != "" {
		args = append(args, f.Model)
		clauses = append(clauses, fmt.Sprintf("model = $%d", len(args)))
	}
	if f.Provider != "" {
		args = append(args, f.Provider)
		clauses = append(clauses, fmt.Sprintf("provider = $%d", len(args)))
	}
	if f.EvidenceMode != "" {
		args = append(args, f.EvidenceMode)
		clauses = append(clauses, fmt.Sprintf("evidence_mode = $%d", len(args)))
	}
	if f.PassedOnly {
		clauses = append(clauses, "passed = TRUE")
	}
	if f.FailedOnly {
		clauses = append(clauses, "passed = FALSE")
	}
	if f.Since != "" {
		t, err := time.Parse(time.RFC3339, f.Since)
		if err != nil {
			t, err = time.Parse("2006-01-02", f.Since)
		}
		if err == nil {
			args = append(args, t)
			clauses = append(clauses, fmt.Sprintf("created_at >= $%d", len(args)))
		}
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

// nullableJSONB returns nil for empty strings (maps to SQL NULL for JSONB columns),
// or the string pointer for non-empty JSON.
func nullableJSONB(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
