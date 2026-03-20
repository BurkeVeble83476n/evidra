package bench

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BenchStore defines the query interface for bench API handlers.
type BenchStore interface {
	ListRuns(ctx context.Context, f RunFilters) ([]RunRecord, int, error)
	GetRun(ctx context.Context, id string) (*RunRecord, error)
	InsertRun(ctx context.Context, r RunRecord) error
	InsertRunBatch(ctx context.Context, runs []RunRecord) (int, error)
	Catalog(ctx context.Context) (*RunCatalog, error)
	CompareRuns(ctx context.Context, idA, idB string) (*RunComparison, error)
	ModelMatrix(ctx context.Context, models, scenarios []string) (*ModelMatrix, error)
	FilteredStats(ctx context.Context, f RunFilters) (*StatsResult, error)
	ListScenarios(ctx context.Context) ([]ScenarioSummary, error)
	SignalSummary(ctx context.Context, f RunFilters) (*SignalAggregation, error)
	Regressions(ctx context.Context) ([]Regression, error)
	FailureAnalysis(ctx context.Context, scenarioID string) (*FailureInsights, error)
	Leaderboard(ctx context.Context, evidenceMode string) ([]LeaderboardEntry, error)
}

// PgStore implements BenchStore backed by PostgreSQL via pgx.
type PgStore struct {
	db       *pgxpool.Pool
	tenantID string
}

// NewPgStore creates a new PgStore scoped to the given tenant.
func NewPgStore(db *pgxpool.Pool, tenantID string) *PgStore {
	return &PgStore{db: db, tenantID: tenantID}
}

// Verify PgStore implements BenchStore at compile time.
var _ BenchStore = (*PgStore)(nil)
