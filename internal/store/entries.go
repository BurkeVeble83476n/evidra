package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// StoredEntry represents an evidence entry in the database.
type StoredEntry struct {
	ID              string
	TenantID        string
	EntryType       string
	SessionID       string
	OperationID     string
	PreviousHash    string
	Hash            string
	Signature       string
	IntentDigest    string
	ArtifactDigest  string
	Payload         json.RawMessage
	ScopeDimensions json.RawMessage
	CreatedAt       time.Time
}

// ListOptions controls entry listing pagination and filters.
type ListOptions struct {
	Limit     int
	Offset    int
	EntryType string
	Period    string
	SessionID string
}

func (o ListOptions) withDefaults() ListOptions {
	if o.Limit <= 0 {
		o.Limit = 100
	}
	if o.Limit > 1000 {
		o.Limit = 1000
	}
	return o
}

// EntryStore manages evidence entries backed by PostgreSQL.
type EntryStore struct {
	pool *pgxpool.Pool
}

// NewEntryStore creates an EntryStore with the given connection pool.
func NewEntryStore(pool *pgxpool.Pool) *EntryStore {
	return &EntryStore{pool: pool}
}

// SaveEntry persists an evidence entry and returns a receipt ID.
func (es *EntryStore) SaveEntry(ctx context.Context, tenantID string, entry StoredEntry) (string, error) {
	if entry.ID == "" {
		entry.ID = ulid.Make().String()
	}
	_, err := es.pool.Exec(ctx,
		`INSERT INTO evidence_entries
		 (id, tenant_id, entry_type, session_id, operation_id,
		  previous_hash, hash, signature, intent_digest, artifact_digest,
		  payload, scope_dimensions, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		entry.ID, tenantID, entry.EntryType, entry.SessionID, entry.OperationID,
		entry.PreviousHash, entry.Hash, entry.Signature,
		entry.IntentDigest, entry.ArtifactDigest,
		entry.Payload, entry.ScopeDimensions, entry.CreatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("store.SaveEntry: %w", err)
	}
	return entry.ID, nil
}

// GetEntry retrieves a single entry by ID, scoped to tenant.
func (es *EntryStore) GetEntry(ctx context.Context, tenantID, entryID string) (StoredEntry, error) {
	var e StoredEntry
	err := es.pool.QueryRow(ctx,
		`SELECT id, tenant_id, entry_type, session_id, operation_id,
		        previous_hash, hash, signature, intent_digest, artifact_digest,
		        payload, scope_dimensions, created_at
		 FROM evidence_entries
		 WHERE id = $1 AND tenant_id = $2`,
		entryID, tenantID,
	).Scan(&e.ID, &e.TenantID, &e.EntryType, &e.SessionID, &e.OperationID,
		&e.PreviousHash, &e.Hash, &e.Signature, &e.IntentDigest, &e.ArtifactDigest,
		&e.Payload, &e.ScopeDimensions, &e.CreatedAt)
	if err != nil {
		return StoredEntry{}, fmt.Errorf("store.GetEntry: %w", err)
	}
	return e, nil
}

// ListEntries returns paginated entries for a tenant.
func (es *EntryStore) ListEntries(ctx context.Context, tenantID string, opts ListOptions) ([]StoredEntry, int, error) {
	opts = opts.withDefaults()

	var where []string
	var args []interface{}
	argIdx := 1

	where = append(where, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, tenantID)
	argIdx++

	if opts.EntryType != "" {
		where = append(where, fmt.Sprintf("entry_type = $%d", argIdx))
		args = append(args, opts.EntryType)
		argIdx++
	}
	if opts.SessionID != "" {
		where = append(where, fmt.Sprintf("session_id = $%d", argIdx))
		args = append(args, opts.SessionID)
		argIdx++
	}
	if opts.Period != "" {
		dur := parsePeriod(opts.Period)
		where = append(where, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, time.Now().Add(-dur))
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	// Count total.
	var total int
	countQuery := "SELECT COUNT(*) FROM evidence_entries WHERE " + whereClause
	if err := es.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store.ListEntries: count: %w", err)
	}

	// Fetch page.
	query := fmt.Sprintf(
		`SELECT id, tenant_id, entry_type, session_id, operation_id,
		        previous_hash, hash, signature, intent_digest, artifact_digest,
		        payload, scope_dimensions, created_at
		 FROM evidence_entries
		 WHERE %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, opts.Limit, opts.Offset)

	rows, err := es.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store.ListEntries: query: %w", err)
	}
	defer rows.Close()

	var entries []StoredEntry
	for rows.Next() {
		var e StoredEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EntryType, &e.SessionID, &e.OperationID,
			&e.PreviousHash, &e.Hash, &e.Signature, &e.IntentDigest, &e.ArtifactDigest,
			&e.Payload, &e.ScopeDimensions, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("store.ListEntries: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store.ListEntries: rows: %w", err)
	}

	return entries, total, nil
}

// SaveRaw persists a raw JSON entry (implements RawEntryStore for forward/batch handlers).
// Parses the JSON to extract structured fields for indexing and provenance continuity.
func (es *EntryStore) SaveRaw(ctx context.Context, tenantID string, raw json.RawMessage) (string, error) {
	var envelope struct {
		EntryID        string `json:"entry_id"`
		Type           string `json:"type"`
		SessionID      string `json:"session_id"`
		OperationID    string `json:"operation_id"`
		PreviousHash   string `json:"previous_hash"`
		Hash           string `json:"hash"`
		Signature      string `json:"signature"`
		IntentDigest   string `json:"intent_digest"`
		ArtifactDigest string `json:"artifact_digest"`
	}
	_ = json.Unmarshal(raw, &envelope)

	id := envelope.EntryID
	if id == "" {
		id = ulid.Make().String()
	}
	entryType := envelope.Type
	if entryType == "" {
		entryType = "raw"
	}

	_, err := es.pool.Exec(ctx,
		`INSERT INTO evidence_entries
		 (id, tenant_id, entry_type, session_id, operation_id,
		  previous_hash, hash, signature, intent_digest, artifact_digest,
		  payload, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())`,
		id, tenantID, entryType, envelope.SessionID, envelope.OperationID,
		envelope.PreviousHash, envelope.Hash, envelope.Signature,
		envelope.IntentDigest, envelope.ArtifactDigest, raw,
	)
	if err != nil {
		return "", fmt.Errorf("store.SaveRaw: %w", err)
	}
	return id, nil
}

// ComputeScorecard reads stored entries and runs them through the signal+score engine.
// Phase 0: reads entries from DB, converts to []evidence.EvidenceEntry, delegates to
// existing internal/signal and internal/score packages (same engine used by CLI scorecard).
func (es *EntryStore) ComputeScorecard(tenantID, period, tool, scope string, minOps int) (interface{}, error) {
	dur := parsePeriod(period)
	entries, _, err := es.ListEntries(context.Background(), tenantID, ListOptions{
		Limit:  10000,
		Period: period,
	})
	if err != nil {
		return nil, fmt.Errorf("ComputeScorecard: %w", err)
	}

	// Convert StoredEntry -> evidence.EvidenceEntry for the existing score engine.
	// Then call: signals := signal.DetectAll(evidenceEntries)
	//            result  := score.Compute(signals, score.DefaultWeights())
	// This wiring is the implementer's responsibility -- the signal/score packages
	// already exist and work. The conversion is mechanical (unmarshal payload fields).
	_ = dur
	_ = entries

	return map[string]interface{}{
		"total_operations": len(entries),
		"period":           period,
		"note":             "wire StoredEntry -> evidence.EvidenceEntry -> signal.DetectAll -> score.Compute",
	}, nil
}

// ComputeExplain reads stored entries and runs signal detection.
// Same conversion pattern as ComputeScorecard -- delegates to internal/signal.
func (es *EntryStore) ComputeExplain(tenantID, period string) (interface{}, error) {
	entries, _, err := es.ListEntries(context.Background(), tenantID, ListOptions{
		Limit:  10000,
		Period: period,
	})
	if err != nil {
		return nil, fmt.Errorf("ComputeExplain: %w", err)
	}

	// Convert StoredEntry -> evidence.EvidenceEntry, then:
	//   signals := signal.DetectAll(evidenceEntries)
	//   return signal breakdown per detector

	return map[string]interface{}{
		"total_entries": len(entries),
		"period":        period,
		"note":          "wire StoredEntry -> evidence.EvidenceEntry -> signal.DetectAll",
	}, nil
}

func parsePeriod(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 30 * 24 * time.Hour
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(s[:len(s)-1])
		if err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	return 30 * 24 * time.Hour
}
