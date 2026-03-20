package benchsvc

import (
	"github.com/jackc/pgx/v5/pgxpool"

	bench "samebits.com/evidra/pkg/bench"
)

// PgStore implements bench.BenchStore backed by PostgreSQL via pgx.
type PgStore struct {
	db       *pgxpool.Pool
	tenantID string
}

// NewPgStore creates a new PgStore scoped to the given tenant.
func NewPgStore(db *pgxpool.Pool, tenantID string) *PgStore {
	return &PgStore{db: db, tenantID: tenantID}
}

// Verify PgStore implements bench.BenchStore at compile time.
var _ bench.BenchStore = (*PgStore)(nil)
