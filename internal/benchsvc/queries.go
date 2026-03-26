package benchsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra/pkg/bench"
)

// runRecordColumns is the SELECT column list for RunRecord scans.
const runRecordColumns = `id, tenant_id, scenario_id, model, provider, adapter, evidence_mode, tool_server,
	tool_server_version, scenario_version,
	passed, duration_seconds, exit_code, turns, memory_window,
	prompt_tokens, completion_tokens, estimated_cost_usd,
	checks_passed, checks_total, checks_json, metadata_json, created_at`

// EnabledModel is a model available to a tenant via platform default or tenant override.
type EnabledModel struct {
	ID                string  `json:"id"`
	DisplayName       string  `json:"display_name"`
	Provider          string  `json:"provider"`
	InputCostPerMtok  float64 `json:"input_cost_per_mtok"`
	OutputCostPerMtok float64 `json:"output_cost_per_mtok"`
}

// TenantProviderConfig holds mutable tenant-specific provider settings.
type TenantProviderConfig struct {
	APIKeyEnc     string  `json:"api_key"`
	APIBaseURL    string  `json:"api_base_url,omitempty"`
	RateLimit     int     `json:"rate_limit,omitempty"`
	MonthlyBudget float64 `json:"monthly_budget,omitempty"`
}

// scanRunRecord scans a row into a bench.RunRecord.
func scanRunRecord(row pgx.CollectableRow) (bench.RunRecord, error) {
	var r bench.RunRecord
	var checksJSON, metadataJSON *string
	err := row.Scan(
		&r.ID, &r.TenantID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.EvidenceMode, &r.ToolServer,
		&r.ToolServerVersion, &r.ScenarioVersion,
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
func (s *PgStore) ListRuns(ctx context.Context, tenantID string, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	where, args := buildWhere(tenantID, f)

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

// GetRun returns a single run by ID, scoped to the given tenant.
func (s *PgStore) GetRun(ctx context.Context, tenantID string, id string) (*bench.RunRecord, error) {
	query := "SELECT " + runRecordColumns + " FROM bench_runs WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL"
	rows, err := s.db.Query(ctx, query, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("bench.GetRun: %w", err)
	}
	defer rows.Close()

	r, err := pgx.CollectExactlyOneRow(rows, scanRunRecord)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("bench.GetRun: %w", err)
	}
	return &r, nil
}

// InsertRun inserts a single benchmark run record.
func (s *PgStore) InsertRun(ctx context.Context, tenantID string, r bench.RunRecord) error {
	query := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode, tool_server,
		tool_server_version, scenario_version,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	checksJSON := nullableJSONB(r.ChecksJSON)
	metadataJSON := nullableJSONB(r.MetadataJSON)
	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := s.db.Exec(ctx, query,
		r.ID, tenantID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.EvidenceMode, r.ToolServer,
		r.ToolServerVersion, r.ScenarioVersion,
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
func (s *PgStore) InsertRunBatch(ctx context.Context, tenantID string, runs []bench.RunRecord) (int, error) {
	if len(runs) == 0 {
		return 0, nil
	}

	query := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode, tool_server,
		tool_server_version, scenario_version,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
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
			r.ID, tenantID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.EvidenceMode, r.ToolServer,
			r.ToolServerVersion, r.ScenarioVersion,
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
func (s *PgStore) FilteredStats(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.StatsResult, error) {
	where, args := buildWhere(tenantID, f)

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

// Catalog returns distinct models and providers from bench_runs.
func (s *PgStore) Catalog(ctx context.Context, tenantID string) (*bench.RunCatalog, error) {
	var models, providers []string

	rows, err := s.db.Query(ctx,
		"SELECT DISTINCT model FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL ORDER BY model", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: models: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan model: %w", err)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: models rows: %w", err)
	}

	rows2, err := s.db.Query(ctx,
		"SELECT DISTINCT provider FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL AND provider != '' ORDER BY provider", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: providers: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var p string
		if err := rows2.Scan(&p); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan provider: %w", err)
		}
		providers = append(providers, p)
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: providers rows: %w", err)
	}

	return &bench.RunCatalog{Models: models, Providers: providers}, nil
}

// ListEnabledModels returns models available to a tenant.
// A model is available if it has a platform API key env var or
// the tenant has an enabled provider entry for that model.
func (s *PgStore) ListEnabledModels(ctx context.Context, tenantID string) ([]EnabledModel, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.id, m.display_name, m.provider,
		       m.input_cost_per_mtok, m.output_cost_per_mtok
		FROM bench_models m
		LEFT JOIN bench_tenant_providers tp
		  ON tp.model_id = m.id AND tp.tenant_id = $1 AND tp.enabled = true
		WHERE m.api_key_env != '' OR tp.tenant_id IS NOT NULL
		ORDER BY m.provider, m.display_name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.ListEnabledModels: %w", err)
	}
	defer rows.Close()

	var models []EnabledModel
	for rows.Next() {
		var model EnabledModel
		if err := rows.Scan(
			&model.ID,
			&model.DisplayName,
			&model.Provider,
			&model.InputCostPerMtok,
			&model.OutputCostPerMtok,
		); err != nil {
			return nil, fmt.Errorf("benchsvc.ListEnabledModels scan: %w", err)
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("benchsvc.ListEnabledModels rows: %w", err)
	}
	return models, nil
}

// UpsertTenantProvider inserts or updates a tenant provider override for a model.
func (s *PgStore) UpsertTenantProvider(ctx context.Context, tenantID, modelID string, cfg TenantProviderConfig) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO bench_tenant_providers (tenant_id, model_id, api_key_enc, api_base_url, rate_limit, monthly_budget, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, true)
		ON CONFLICT (tenant_id, model_id) DO UPDATE SET
			api_key_enc = CASE WHEN $3 != '' THEN $3 ELSE bench_tenant_providers.api_key_enc END,
			api_base_url = CASE WHEN $4 != '' THEN $4 ELSE bench_tenant_providers.api_base_url END,
			rate_limit = CASE WHEN $5 > 0 THEN $5 ELSE bench_tenant_providers.rate_limit END,
			monthly_budget = CASE WHEN $6 > 0 THEN $6 ELSE bench_tenant_providers.monthly_budget END,
			enabled = true,
			updated_at = NOW()
	`, tenantID, modelID, cfg.APIKeyEnc, cfg.APIBaseURL, cfg.RateLimit, cfg.MonthlyBudget)
	if err != nil {
		return fmt.Errorf("benchsvc.UpsertTenantProvider: %w", err)
	}
	return nil
}

// DeleteTenantProvider removes a tenant-specific provider override for a model.
func (s *PgStore) DeleteTenantProvider(ctx context.Context, tenantID, modelID string) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM bench_tenant_providers
		WHERE tenant_id = $1 AND model_id = $2
	`, tenantID, modelID)
	if err != nil {
		return fmt.Errorf("benchsvc.DeleteTenantProvider: %w", err)
	}
	return nil
}

// ListScenarios returns all scenarios from the global catalog.
func (s *PgStore) ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, category, title, tools, chaos, evidra_enabled
		 FROM bench_scenarios ORDER BY category, id`)
	if err != nil {
		return nil, fmt.Errorf("bench.ListScenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []bench.ScenarioSummary
	for rows.Next() {
		var sc bench.ScenarioSummary
		var tools []string
		if err := rows.Scan(&sc.ID, &sc.Category, &sc.Title, &tools, &sc.Chaos, &sc.Evidra); err != nil {
			return nil, fmt.Errorf("bench.ListScenarios: scan: %w", err)
		}
		sc.Tags = tools
		scenarios = append(scenarios, sc)
	}
	return scenarios, rows.Err()
}

// UpsertScenarios inserts or updates scenario metadata in bench_scenarios.
func (s *PgStore) UpsertScenarios(ctx context.Context, scenarios []bench.ScenarioSummary) (int, error) {
	upserted := 0
	for _, sc := range scenarios {
		tags := sc.Tags
		if tags == nil {
			tags = []string{}
		}
		_, err := s.db.Exec(ctx,
			`INSERT INTO bench_scenarios (id, category, title, description, tools, chaos, evidra_enabled, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			 ON CONFLICT (id) DO UPDATE SET
			   category = EXCLUDED.category,
			   title = EXCLUDED.title,
			   description = EXCLUDED.description,
			   tools = EXCLUDED.tools,
			   chaos = EXCLUDED.chaos,
			   evidra_enabled = EXCLUDED.evidra_enabled,
			   updated_at = NOW()`,
			sc.ID, sc.Category, sc.Title, sc.Description, tags, sc.Chaos, sc.Evidra)
		if err != nil {
			return upserted, fmt.Errorf("bench.UpsertScenarios(%s): %w", sc.ID, err)
		}
		upserted++
	}
	return upserted, nil
}

// StoreArtifact upserts an artifact for a given run.
// If the artifact already exists, the data is replaced.
func (s *PgStore) StoreArtifact(ctx context.Context, runID, artifactType, contentType string, data []byte) error {
	query := `INSERT INTO bench_artifacts (run_id, artifact_type, content_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id, artifact_type) DO UPDATE SET data = EXCLUDED.data, content_type = EXCLUDED.content_type`
	_, err := s.db.Exec(ctx, query, runID, artifactType, contentType, data)
	if err != nil {
		return fmt.Errorf("bench.StoreArtifact: %w", err)
	}
	return nil
}

// GetArtifact retrieves an artifact by run ID and type.
// It verifies the run belongs to the given tenant before returning data.
// Returns data, contentType, error.
func (s *PgStore) GetArtifact(ctx context.Context, tenantID string, runID, artifactType string) ([]byte, string, error) {
	query := `SELECT a.data, a.content_type FROM bench_artifacts a
		JOIN bench_runs r ON r.id = a.run_id
		WHERE r.tenant_id = $1 AND a.run_id = $2 AND a.artifact_type = $3`
	var data []byte
	var ct string
	err := s.db.QueryRow(ctx, query, tenantID, runID, artifactType).Scan(&data, &ct)
	if err != nil {
		return nil, "", fmt.Errorf("bench.GetArtifact: %w", err)
	}
	return data, ct, nil
}

// DeleteRun deletes a single run by ID, scoped to the given tenant.
// Artifacts are cascade-deleted via the foreign key constraint.
func (s *PgStore) DeleteRun(ctx context.Context, tenantID, runID string) error {
	query := `DELETE FROM bench_runs WHERE id = $1 AND tenant_id = $2`
	ct, err := s.db.Exec(ctx, query, runID, tenantID)
	if err != nil {
		return fmt.Errorf("bench.DeleteRun: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ArchiveRuns sets archived_at on runs matching the request filters.
// Returns the number of runs archived.
func (s *PgStore) ArchiveRuns(ctx context.Context, tenantID string, req ArchiveRequest) (int, error) {
	clauses := []string{"tenant_id = $1", "archived_at IS NULL"}
	args := []any{tenantID}

	if req.Before != nil {
		args = append(args, *req.Before)
		clauses = append(clauses, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(req.IDs) > 0 {
		args = append(args, req.IDs)
		clauses = append(clauses, fmt.Sprintf("id = ANY($%d)", len(args)))
	}
	if req.Model != "" {
		args = append(args, req.Model)
		clauses = append(clauses, fmt.Sprintf("model = $%d", len(args)))
	}

	query := "UPDATE bench_runs SET archived_at = NOW() WHERE " + strings.Join(clauses, " AND ")
	ct, err := s.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bench.ArchiveRuns: %w", err)
	}
	return int(ct.RowsAffected()), nil
}

// buildWhere constructs a WHERE clause with numbered PostgreSQL placeholders.
// The tenant_id filter is always applied.
func buildWhere(tenantID string, f bench.RunFilters) (string, []any) {
	clauses := []string{"tenant_id = $1", "archived_at IS NULL"}
	args := []any{tenantID}

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
	if f.Since != nil {
		args = append(args, *f.Since)
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", len(args)))
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

// CompareModels returns per-scenario stats for two models side-by-side.
// Single query with conditional aggregation.
func (s *PgStore) CompareModels(ctx context.Context, tenantID, modelA, modelB, evidenceMode string) ([]ScenarioModelComparison, error) {
	query := `
		SELECT scenario_id,
			COALESCE(100.0 * SUM(CASE WHEN model = $2 AND passed THEN 1 ELSE 0 END) /
				NULLIF(SUM(CASE WHEN model = $2 THEN 1 ELSE 0 END), 0), -1) AS a_pass_rate,
			COALESCE(100.0 * SUM(CASE WHEN model = $3 AND passed THEN 1 ELSE 0 END) /
				NULLIF(SUM(CASE WHEN model = $3 THEN 1 ELSE 0 END), 0), -1) AS b_pass_rate,
			COALESCE(AVG(CASE WHEN model = $2 THEN duration_seconds END), 0) AS a_duration,
			COALESCE(AVG(CASE WHEN model = $3 THEN duration_seconds END), 0) AS b_duration,
			COALESCE(AVG(CASE WHEN model = $2 THEN estimated_cost_usd END), 0) AS a_cost,
			COALESCE(AVG(CASE WHEN model = $3 THEN estimated_cost_usd END), 0) AS b_cost
		FROM bench_runs
		WHERE tenant_id = $1 AND archived_at IS NULL AND evidence_mode = $4
			AND model IN ($2, $3)
		GROUP BY scenario_id
		HAVING SUM(CASE WHEN model = $2 THEN 1 ELSE 0 END) > 0
			AND SUM(CASE WHEN model = $3 THEN 1 ELSE 0 END) > 0
		ORDER BY scenario_id`

	rows, err := s.db.Query(ctx, query, tenantID, modelA, modelB, evidenceMode)
	if err != nil {
		return nil, fmt.Errorf("bench.CompareModels: %w", err)
	}
	defer rows.Close()

	var results []ScenarioModelComparison
	for rows.Next() {
		var sc ScenarioModelComparison
		if err := rows.Scan(&sc.ScenarioID, &sc.APassRate, &sc.BPassRate,
			&sc.ADuration, &sc.BDuration, &sc.ACost, &sc.BCost); err != nil {
			return nil, fmt.Errorf("bench.CompareModels: scan: %w", err)
		}
		results = append(results, sc)
	}
	return results, rows.Err()
}
