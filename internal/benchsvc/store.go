package benchsvc

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgStore is a tenant-agnostic PostgreSQL repository for benchmark data.
// All query methods accept tenantID as a parameter rather than capturing
// it at construction time.
//
// The tenantID field is retained temporarily for backward compatibility
// with handlers that have not yet been migrated to the Service layer.
// It will be removed in a follow-up task.
type PgStore struct {
	db       *pgxpool.Pool
	tenantID string // Deprecated: use per-call tenantID parameters via Service.
}

// NewPgStore creates a new PgStore. The tenantID parameter is retained for
// backward compatibility with code that has not yet migrated to the Service
// layer. Pass "" when using PgStore exclusively through Service.
func NewPgStore(db *pgxpool.Pool, tenantID string) *PgStore {
	return &PgStore{db: db, tenantID: tenantID}
}
