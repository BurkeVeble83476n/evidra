package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra/pkg/bench"
)

// ErrPublicTenantUnavailable is returned when a public endpoint is called
// but no PublicTenant has been configured.
var ErrPublicTenantUnavailable = errors.New("benchsvc: public tenant not configured")

// Repository defines the data-access contract the Service depends on.
// PgStore satisfies this interface; test fakes can implement it too.
type Repository interface {
	ListRuns(ctx context.Context, tenantID string, f bench.RunFilters) ([]bench.RunRecord, int, error)
	GetRun(ctx context.Context, tenantID string, id string) (*bench.RunRecord, error)
	InsertRun(ctx context.Context, tenantID string, r bench.RunRecord) error
	InsertRunBatch(ctx context.Context, tenantID string, runs []bench.RunRecord) (int, error)
	FilteredStats(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.StatsResult, error)
	Catalog(ctx context.Context, tenantID string) (*bench.RunCatalog, error)
	Leaderboard(ctx context.Context, tenantID string, evidenceMode string) ([]bench.LeaderboardEntry, error)
	ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error)
	StoreArtifact(ctx context.Context, runID, artifactType, contentType string, data []byte) error
	GetArtifact(ctx context.Context, tenantID string, runID, artifactType string) ([]byte, string, error)
	CompareModels(ctx context.Context, tenantID, modelA, modelB, evidenceMode string) ([]ScenarioModelComparison, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// ServiceConfig holds configuration for the bench service.
type ServiceConfig struct {
	PublicTenant string // tenant for unauthenticated leaderboard/scenarios
}

// Service provides request-scoped bench operations over a tenant-agnostic repository.
type Service struct {
	repo Repository
	cfg  ServiceConfig
}

// NewService creates a new Service backed by the given repository.
func NewService(repo Repository, cfg ServiceConfig) *Service {
	return &Service{repo: repo, cfg: cfg}
}

// IngestRunRequest wraps RunRecord with optional artifact payloads
// for atomic run+artifact ingestion.
type IngestRunRequest struct {
	bench.RunRecord
	Transcript string          `json:"transcript,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
}

// --- Authenticated methods (tenant from caller) ---

// ListRuns returns runs matching filters, scoped to the given tenant.
func (s *Service) ListRuns(ctx context.Context, tenantID string, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	return s.repo.ListRuns(ctx, tenantID, f)
}

// GetRun returns a single run by ID, scoped to the given tenant.
func (s *Service) GetRun(ctx context.Context, tenantID string, id string) (*bench.RunRecord, error) {
	return s.repo.GetRun(ctx, tenantID, id)
}

// IngestRun atomically inserts a run and its artifacts using a database transaction.
// If any step fails, the entire operation is rolled back.
func (s *Service) IngestRun(ctx context.Context, tenantID string, req IngestRunRequest) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("benchsvc.IngestRun: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Build the insert query inline using the transaction.
	insertQ := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`

	checksJSON := nullableJSONB(req.ChecksJSON)
	metadataJSON := nullableJSONB(req.MetadataJSON)
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err = tx.Exec(ctx, insertQ,
		req.ID, tenantID, req.ScenarioID, req.Model, req.Provider, req.Adapter, req.EvidenceMode,
		req.Passed, req.Duration, req.ExitCode, req.Turns, req.MemoryWindow,
		req.PromptTokens, req.CompletionTokens, req.EstimatedCost,
		req.ChecksPassed, req.ChecksTotal, checksJSON, metadataJSON, createdAt,
	)
	if err != nil {
		return fmt.Errorf("benchsvc.IngestRun: insert run: %w", err)
	}

	// Store artifacts within the same transaction.
	artifactQ := `INSERT INTO bench_artifacts (run_id, artifact_type, content_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id, artifact_type) DO UPDATE SET data = EXCLUDED.data, content_type = EXCLUDED.content_type`

	if req.Transcript != "" {
		_, err = tx.Exec(ctx, artifactQ, req.ID, "transcript", "text/plain", []byte(req.Transcript))
		if err != nil {
			return fmt.Errorf("benchsvc.IngestRun: store transcript: %w", err)
		}
	}
	if len(req.ToolCalls) > 0 {
		_, err = tx.Exec(ctx, artifactQ, req.ID, "tool_calls", "application/json", []byte(req.ToolCalls))
		if err != nil {
			return fmt.Errorf("benchsvc.IngestRun: store tool_calls: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("benchsvc.IngestRun: commit: %w", err)
	}
	return nil
}

// IngestRunBatch atomically inserts multiple runs and their artifacts.
// If any artifact fails, all runs in the batch are rolled back.
// Returns the count of runs inserted.
func (s *Service) IngestRunBatch(ctx context.Context, tenantID string, runs []IngestRunRequest) (int, error) {
	if len(runs) == 0 {
		return 0, nil
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("benchsvc.IngestRunBatch: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	insertQ := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	ON CONFLICT (id) DO NOTHING`

	artifactQ := `INSERT INTO bench_artifacts (run_id, artifact_type, content_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id, artifact_type) DO UPDATE SET data = EXCLUDED.data, content_type = EXCLUDED.content_type`

	inserted := 0
	for _, run := range runs {
		checksJSON := nullableJSONB(run.ChecksJSON)
		metadataJSON := nullableJSONB(run.MetadataJSON)
		createdAt := run.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		ct, err := tx.Exec(ctx, insertQ,
			run.ID, tenantID, run.ScenarioID, run.Model, run.Provider, run.Adapter, run.EvidenceMode,
			run.Passed, run.Duration, run.ExitCode, run.Turns, run.MemoryWindow,
			run.PromptTokens, run.CompletionTokens, run.EstimatedCost,
			run.ChecksPassed, run.ChecksTotal, checksJSON, metadataJSON, createdAt,
		)
		if err != nil {
			return 0, fmt.Errorf("benchsvc.IngestRunBatch: insert run %s: %w", run.ID, err)
		}
		if ct.RowsAffected() == 0 {
			continue
		}
		inserted += int(ct.RowsAffected())

		if run.Transcript != "" {
			if _, err := tx.Exec(ctx, artifactQ, run.ID, "transcript", "text/plain", []byte(run.Transcript)); err != nil {
				return 0, fmt.Errorf("benchsvc.IngestRunBatch: transcript for %s: %w", run.ID, err)
			}
		}
		if len(run.ToolCalls) > 0 {
			if _, err := tx.Exec(ctx, artifactQ, run.ID, "tool_calls", "application/json", []byte(run.ToolCalls)); err != nil {
				return 0, fmt.Errorf("benchsvc.IngestRunBatch: tool_calls for %s: %w", run.ID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("benchsvc.IngestRunBatch: commit: %w", err)
	}
	return inserted, nil
}

// FilteredStats returns aggregate statistics matching the given filters.
func (s *Service) FilteredStats(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.StatsResult, error) {
	return s.repo.FilteredStats(ctx, tenantID, f)
}

// Catalog returns distinct models and providers for the given tenant.
func (s *Service) Catalog(ctx context.Context, tenantID string) (*bench.RunCatalog, error) {
	return s.repo.Catalog(ctx, tenantID)
}

// GetArtifact retrieves an artifact for a run, scoped to the given tenant.
func (s *Service) GetArtifact(ctx context.Context, tenantID string, runID, artifactType string) ([]byte, string, error) {
	return s.repo.GetArtifact(ctx, tenantID, runID, artifactType)
}

// StoreArtifact stores an artifact for a run (no tenant scoping on writes).
func (s *Service) StoreArtifact(ctx context.Context, runID, artifactType, contentType string, data []byte) error {
	return s.repo.StoreArtifact(ctx, runID, artifactType, contentType, data)
}

// --- Public methods (use configured PublicTenant) ---

// Leaderboard returns the public leaderboard using the configured public tenant.
func (s *Service) Leaderboard(ctx context.Context, evidenceMode string) ([]bench.LeaderboardEntry, error) {
	if s.cfg.PublicTenant == "" {
		return nil, ErrPublicTenantUnavailable
	}
	return s.repo.Leaderboard(ctx, s.cfg.PublicTenant, evidenceMode)
}

// ListScenarios returns the global scenario catalog.
func (s *Service) ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error) {
	return s.repo.ListScenarios(ctx)
}
