package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bench "samebits.com/evidra/pkg/bench"
)

// ErrPublicTenantUnavailable is returned when a public endpoint is called
// but no PublicTenant has been configured.
var ErrPublicTenantUnavailable = errors.New("benchsvc: public tenant not configured")

// ServiceConfig holds configuration for the bench service.
type ServiceConfig struct {
	PublicTenant string // tenant for unauthenticated leaderboard/scenarios
}

// Service provides request-scoped bench operations over a tenant-agnostic repository.
type Service struct {
	repo *PgStore
	cfg  ServiceConfig
}

// NewService creates a new Service backed by the given repository.
func NewService(repo *PgStore, cfg ServiceConfig) *Service {
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
	tx, err := s.repo.db.Begin(ctx)
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

// IngestRunBatch inserts multiple runs, skipping duplicates. Returns the count inserted.
func (s *Service) IngestRunBatch(ctx context.Context, tenantID string, runs []IngestRunRequest) (int, error) {
	records := make([]bench.RunRecord, len(runs))
	for i := range runs {
		records[i] = runs[i].RunRecord
	}
	count, err := s.repo.InsertRunBatch(ctx, tenantID, records)
	if err != nil {
		return 0, err
	}
	// Store artifacts for each run (best-effort after batch insert).
	for _, run := range runs {
		if run.Transcript != "" {
			if err := s.repo.StoreArtifact(ctx, run.ID, "transcript", "text/plain", []byte(run.Transcript)); err != nil {
				return count, fmt.Errorf("benchsvc.IngestRunBatch: transcript for %s: %w", run.ID, err)
			}
		}
		if len(run.ToolCalls) > 0 {
			if err := s.repo.StoreArtifact(ctx, run.ID, "tool_calls", "application/json", []byte(run.ToolCalls)); err != nil {
				return count, fmt.Errorf("benchsvc.IngestRunBatch: tool_calls for %s: %w", run.ID, err)
			}
		}
	}
	return count, nil
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
