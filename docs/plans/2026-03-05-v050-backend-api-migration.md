# v0.5.0 — Backend API, Database & Auth Migration

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate the REST API server, Postgres database layer, auth middleware, and key store from `evidra-mcp` into `evidra-benchmark`, creating `evidra-api` — a centralized evidence collection service with multi-tenancy, signed receipts, and server-side scorecards.

**Architecture:** Reuse proven infrastructure from `../evidra-mcp/` (database connection pool, auth middleware, key store, API scaffolding) but rewrite all handlers for benchmark semantics. The old repo's `POST /v1/validate` (OPA policy evaluation) is replaced by `POST /v1/evidence/forward` (evidence ingestion + signed receipts). The API server receives `EvidenceEntry` JSON from MCP servers and CLI clients, stores in Postgres, returns signed receipts, and serves scorecards. Local evidence always succeeds; forwarding is best-effort.

**Tech Stack:** Go 1.24, `github.com/jackc/pgx/v5` (Postgres), stdlib `net/http`, existing `pkg/evidence`, `internal/signal`, `internal/score`, `internal/pipeline` packages.

**Prerequisites:** v0.4.0 signing integration must be complete (entries signed before forwarding).

**Source repo:** `../evidra-mcp/` (at `/Users/vitas/git/evidra-mcp/`)

---

## Phase 1: Database Layer

### Task 1: Add pgx/v5 dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add pgx dependency**

```bash
go get github.com/jackc/pgx/v5
go mod tidy
```

**Step 2: Verify build**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add github.com/jackc/pgx/v5 for Postgres support"
```

---

### Task 2: Create database connection pool

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/migrations/001_tenants_and_keys.sql`
- Create: `internal/db/migrations/002_evidence_entries.sql`

**Step 1: Create `internal/db/db.go`**

Copy from `../evidra-mcp/internal/db/db.go` verbatim — the connection pool, migration runner, and `splitStatements` helper are reusable as-is. Change the import path only:

```go
// Package db provides Postgres connection pool initialisation and schema migration.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect creates a pgxpool, pings the server, and runs all pending migrations.
// Returns the pool ready for use. Caller must call pool.Close() when done.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db.Connect: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.Connect: ping: %w", err)
	}

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.Connect: migrate: %w", err)
	}

	return pool, nil
}

// runMigrations executes all *.sql files from the embedded migrations directory
// in lexicographic order. Statements are separated by semicolons and executed
// individually so partial failures are visible.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	for _, name := range names {
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		for _, stmt := range splitStatements(string(data)) {
			if _, err := conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		slog.Debug("migration applied", "file", name)
	}

	return nil
}

// splitStatements splits a SQL file on semicolons, skipping blank and comment-only lines.
func splitStatements(sql string) []string {
	var stmts []string
	for _, raw := range strings.Split(sql, ";") {
		var lines []string
		for _, line := range strings.Split(raw, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		s := strings.TrimSpace(strings.Join(lines, "\n"))
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}
```

**Step 2: Create migration 001 — tenants and API keys**

File: `internal/db/migrations/001_tenants_and_keys.sql`

Copy from `../evidra-mcp/internal/db/migrations/001_keys.sql` verbatim:

```sql
-- Phase 1: tenant and API key tables.
-- All statements are idempotent (IF NOT EXISTS) -- safe to re-run on restart.

CREATE TABLE IF NOT EXISTS tenants (
    id         TEXT        PRIMARY KEY,          -- ULID
    label      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT        PRIMARY KEY,         -- ULID
    tenant_id    TEXT        NOT NULL REFERENCES tenants(id),
    key_hash     BYTEA       NOT NULL,            -- SHA-256(plaintext), 32 bytes
    prefix       TEXT        NOT NULL,            -- "ev1_<first8>" for log correlation
    label        TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,                     -- NULL = active
    last_used_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash   ON api_keys (key_hash);
CREATE        INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys (tenant_id);
```

**Step 3: Create migration 002 — evidence entries**

File: `internal/db/migrations/002_evidence_entries.sql`

```sql
-- Evidence entries table for centralized storage.
-- Mirrors EvidenceEntry struct from pkg/evidence/entry.go.

CREATE TABLE IF NOT EXISTS evidence_entries (
    id              BIGSERIAL   PRIMARY KEY,
    entry_id        TEXT        NOT NULL UNIQUE,
    tenant_id       TEXT        NOT NULL DEFAULT '',
    type            TEXT        NOT NULL,
    trace_id        TEXT        NOT NULL,
    actor_type      TEXT        NOT NULL DEFAULT '',
    actor_id        TEXT        NOT NULL DEFAULT '',
    actor_provenance TEXT       NOT NULL DEFAULT '',
    timestamp       TIMESTAMPTZ NOT NULL,
    intent_digest   TEXT        NOT NULL DEFAULT '',
    artifact_digest TEXT        NOT NULL DEFAULT '',
    payload         JSONB       NOT NULL,
    previous_hash   TEXT        NOT NULL DEFAULT '',
    hash            TEXT        NOT NULL,
    signature       TEXT        NOT NULL DEFAULT '',
    spec_version    TEXT        NOT NULL DEFAULT '',
    canon_version   TEXT        NOT NULL DEFAULT '',
    adapter_version TEXT        NOT NULL DEFAULT '',
    scoring_version TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_entries_tenant_id  ON evidence_entries(tenant_id);
CREATE INDEX IF NOT EXISTS idx_entries_actor_id   ON evidence_entries(actor_id);
CREATE INDEX IF NOT EXISTS idx_entries_type       ON evidence_entries(type);
CREATE INDEX IF NOT EXISTS idx_entries_trace_id   ON evidence_entries(trace_id);
CREATE INDEX IF NOT EXISTS idx_entries_timestamp  ON evidence_entries(timestamp);
```

**Step 4: Write unit test for splitStatements**

File: `internal/db/db_test.go`

```go
package db

import "testing"

func TestSplitStatements_Basic(t *testing.T) {
	t.Parallel()
	sql := "CREATE TABLE foo (id INT);\nCREATE TABLE bar (id INT);"
	stmts := splitStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
}

func TestSplitStatements_SkipsComments(t *testing.T) {
	t.Parallel()
	sql := "-- this is a comment\nCREATE TABLE foo (id INT);"
	stmts := splitStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if stmts[0] != "CREATE TABLE foo (id INT)" {
		t.Errorf("unexpected statement: %q", stmts[0])
	}
}

func TestSplitStatements_Empty(t *testing.T) {
	t.Parallel()
	stmts := splitStatements("-- just comments\n;")
	if len(stmts) != 0 {
		t.Fatalf("expected 0 statements, got %d", len(stmts))
	}
}
```

**Step 5: Run tests, gofmt, commit**

```bash
gofmt -w internal/db/
go test ./internal/db/ -v -count=1
git add internal/db/
git commit -m "feat: add database layer with connection pool and migrations"
```

---

### Task 3: Create key store

**Files:**
- Create: `internal/store/keys.go`
- Create: `internal/store/keys_test.go`

**Step 1: Create `internal/store/keys.go`**

Copy from `../evidra-mcp/internal/store/keys.go` verbatim, changing only the module path in imports (no `samebits.com/evidra` imports needed — only pgx and ulid):

```go
// Package store provides Postgres-backed storage for API keys and tenants.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// ErrKeyNotFound is returned when the key does not exist or has been revoked.
var ErrKeyNotFound = errors.New("key not found")

// KeyRecord holds the stored metadata for an API key (never the plaintext).
type KeyRecord struct {
	ID         string
	TenantID   string
	Prefix     string
	Label      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// KeyStore manages API key lifecycle backed by Postgres.
type KeyStore struct {
	pool *pgxpool.Pool
}

// New creates a KeyStore using the given connection pool.
func New(pool *pgxpool.Pool) *KeyStore {
	return &KeyStore{pool: pool}
}

// CreateKey generates a new API key, persists the hash, and returns the
// plaintext exactly once. The plaintext is never stored.
func (s *KeyStore) CreateKey(ctx context.Context, label string) (plaintext string, rec KeyRecord, err error) {
	tenantID := ulid.Make().String()
	keyID := ulid.Make().String()

	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: rand: %w", err)
	}

	plaintext = "ev1_" + base62Encode(raw)
	prefix := plaintext[:12]
	hash := sha256.Sum256([]byte(plaintext))
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx,
		`INSERT INTO tenants (id, label, created_at) VALUES ($1, $2, $3)`,
		tenantID, label, now,
	)
	if err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: insert tenant: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO api_keys (id, tenant_id, key_hash, prefix, label, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		keyID, tenantID, hash[:], prefix, label, now,
	)
	if err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: insert key: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: commit: %w", err)
	}

	rec = KeyRecord{
		ID:        keyID,
		TenantID:  tenantID,
		Prefix:    prefix,
		Label:     label,
		CreatedAt: now,
	}
	return plaintext, rec, nil
}

// LookupKey hashes the plaintext and looks up the corresponding active key record.
func (s *KeyStore) LookupKey(ctx context.Context, plaintext string) (tenantID, keyID, prefix string, err error) {
	hash := sha256.Sum256([]byte(plaintext))

	err = s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, prefix
		 FROM api_keys
		 WHERE key_hash = $1 AND revoked_at IS NULL`,
		hash[:],
	).Scan(&keyID, &tenantID, &prefix)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", ErrKeyNotFound
	}
	if err != nil {
		return "", "", "", fmt.Errorf("store.LookupKey: %w", err)
	}
	return tenantID, keyID, prefix, nil
}

// TouchKey updates last_used_at for the given key ID.
// Called asynchronously after successful auth; errors are logged, not returned.
func (s *KeyStore) TouchKey(keyID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := s.pool.Exec(ctx,
			`UPDATE api_keys SET last_used_at = now() WHERE id = $1`,
			keyID,
		); err != nil {
			slog.Warn("store.TouchKey: update failed", "key_id", keyID, "error", err)
		}
	}()
}

// base62Encode encodes bytes using the base62 alphabet [0-9A-Za-z].
func base62Encode(b []byte) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 44)
	n := make([]byte, len(b))
	copy(n, b)
	for {
		if allZero(n) {
			break
		}
		remainder := 0
		for i := range n {
			val := remainder*256 + int(n[i])
			n[i] = byte(val / 62)
			remainder = val % 62
		}
		result = append(result, alphabet[remainder])
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
```

**Step 2: Write unit test for base62Encode**

File: `internal/store/keys_test.go`

```go
package store

import (
	"strings"
	"testing"
)

func TestBase62Encode_Length(t *testing.T) {
	t.Parallel()
	b := make([]byte, 32)
	for i := range b {
		b[i] = 0xFF
	}
	encoded := base62Encode(b)
	if len(encoded) == 0 {
		t.Fatal("base62 encoding of non-zero bytes should not be empty")
	}
	// Base62 of 32 bytes should be roughly 43 chars
	if len(encoded) < 40 || len(encoded) > 50 {
		t.Errorf("unexpected length %d for base62 of 32 bytes", len(encoded))
	}
}

func TestBase62Encode_Alphabet(t *testing.T) {
	t.Parallel()
	b := []byte{0x42, 0xAB, 0xFF}
	encoded := base62Encode(b)
	for _, c := range encoded {
		if !strings.ContainsRune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", c) {
			t.Errorf("invalid base62 character: %c", c)
		}
	}
}

func TestBase62Encode_AllZero(t *testing.T) {
	t.Parallel()
	b := make([]byte, 32)
	encoded := base62Encode(b)
	if encoded != "" {
		t.Errorf("all-zero bytes should encode to empty string, got %q", encoded)
	}
}
```

**Step 3: Run tests, gofmt, commit**

```bash
gofmt -w internal/store/
go test ./internal/store/ -v -count=1
git add internal/store/
git commit -m "feat: add Postgres-backed API key store"
```

---

### Task 4: Create evidence entry repository

**Files:**
- Create: `internal/store/entries.go`
- Create: `internal/store/entries_test.go`

**Step 1: Create `internal/store/entries.go`**

This is new code — no equivalent in evidra-mcp. It stores and queries `EvidenceEntry` structs in Postgres.

```go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"samebits.com/evidra-benchmark/pkg/evidence"
)

// EntryStore manages evidence entries in Postgres.
type EntryStore struct {
	pool *pgxpool.Pool
}

// NewEntryStore creates an EntryStore using the given connection pool.
func NewEntryStore(pool *pgxpool.Pool) *EntryStore {
	return &EntryStore{pool: pool}
}

// InsertEntry stores a single evidence entry. Returns the database sequence ID.
func (s *EntryStore) InsertEntry(ctx context.Context, e evidence.EvidenceEntry) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO evidence_entries (
			entry_id, tenant_id, type, trace_id,
			actor_type, actor_id, actor_provenance,
			timestamp, intent_digest, artifact_digest,
			payload, previous_hash, hash, signature,
			spec_version, canon_version, adapter_version, scoring_version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id`,
		e.EntryID, e.TenantID, string(e.Type), e.TraceID,
		e.Actor.Type, e.Actor.ID, e.Actor.Provenance,
		e.Timestamp, e.IntentDigest, e.ArtifactDigest,
		e.Payload, e.PreviousHash, e.Hash, e.Signature,
		e.SpecVersion, e.CanonVersion, e.AdapterVersion, e.ScoringVersion,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store.InsertEntry: %w", err)
	}
	return id, nil
}

// EntryFilter defines query parameters for listing entries.
type EntryFilter struct {
	TenantID string
	ActorID  string
	Type     string
	TraceID  string
	Tool     string // filters by payload->'canonical_action'->>'tool'
	Since    *time.Time
	Until    *time.Time
	Limit    int
	Offset   int
}

// ListEntries returns entries matching the given filter, ordered by timestamp desc.
func (s *EntryStore) ListEntries(ctx context.Context, f EntryFilter) ([]evidence.EvidenceEntry, error) {
	query := `SELECT entry_id, tenant_id, type, trace_id,
		actor_type, actor_id, actor_provenance,
		timestamp, intent_digest, artifact_digest,
		payload, previous_hash, hash, signature,
		spec_version, canon_version, adapter_version, scoring_version
		FROM evidence_entries WHERE 1=1`
	args := []any{}
	argN := 1

	if f.TenantID != "" {
		query += fmt.Sprintf(" AND tenant_id = $%d", argN)
		args = append(args, f.TenantID)
		argN++
	}
	if f.ActorID != "" {
		query += fmt.Sprintf(" AND actor_id = $%d", argN)
		args = append(args, f.ActorID)
		argN++
	}
	if f.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argN)
		args = append(args, f.Type)
		argN++
	}
	if f.TraceID != "" {
		query += fmt.Sprintf(" AND trace_id = $%d", argN)
		args = append(args, f.TraceID)
		argN++
	}
	if f.Since != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argN)
		args = append(args, *f.Since)
		argN++
	}
	if f.Until != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argN)
		args = append(args, *f.Until)
		argN++
	}

	query += " ORDER BY timestamp DESC"

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit)
	argN++

	if f.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argN)
		args = append(args, f.Offset)
		argN++
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.ListEntries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// GetEntry returns a single entry by entry_id, scoped to tenant.
func (s *EntryStore) GetEntry(ctx context.Context, tenantID, entryID string) (evidence.EvidenceEntry, error) {
	query := `SELECT entry_id, tenant_id, type, trace_id,
		actor_type, actor_id, actor_provenance,
		timestamp, intent_digest, artifact_digest,
		payload, previous_hash, hash, signature,
		spec_version, canon_version, adapter_version, scoring_version
		FROM evidence_entries WHERE entry_id = $1`
	args := []any{entryID}

	if tenantID != "" {
		query += " AND tenant_id = $2"
		args = append(args, tenantID)
	}

	row := s.pool.QueryRow(ctx, query, args...)
	var e evidence.EvidenceEntry
	var entryType string
	var payload json.RawMessage
	err := row.Scan(
		&e.EntryID, &e.TenantID, &entryType, &e.TraceID,
		&e.Actor.Type, &e.Actor.ID, &e.Actor.Provenance,
		&e.Timestamp, &e.IntentDigest, &e.ArtifactDigest,
		&payload, &e.PreviousHash, &e.Hash, &e.Signature,
		&e.SpecVersion, &e.CanonVersion, &e.AdapterVersion, &e.ScoringVersion,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return evidence.EvidenceEntry{}, fmt.Errorf("store.GetEntry: entry %q not found", entryID)
		}
		return evidence.EvidenceEntry{}, fmt.Errorf("store.GetEntry: %w", err)
	}
	e.Type = evidence.EntryType(entryType)
	e.Payload = payload
	return e, nil
}

// LastHash returns the hash of the most recent entry for a tenant,
// or empty string if no entries exist.
func (s *EntryStore) LastHash(ctx context.Context, tenantID string) (string, error) {
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT hash FROM evidence_entries
		 WHERE tenant_id = $1
		 ORDER BY id DESC LIMIT 1`,
		tenantID,
	).Scan(&hash)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store.LastHash: %w", err)
	}
	return hash, nil
}

// EntryCount returns the total number of entries for a tenant.
func (s *EntryStore) EntryCount(ctx context.Context, tenantID string) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM evidence_entries WHERE tenant_id = $1`,
		tenantID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store.EntryCount: %w", err)
	}
	return count, nil
}

func scanEntries(rows pgx.Rows) ([]evidence.EvidenceEntry, error) {
	var entries []evidence.EvidenceEntry
	for rows.Next() {
		var e evidence.EvidenceEntry
		var entryType string
		var payload json.RawMessage
		err := rows.Scan(
			&e.EntryID, &e.TenantID, &entryType, &e.TraceID,
			&e.Actor.Type, &e.Actor.ID, &e.Actor.Provenance,
			&e.Timestamp, &e.IntentDigest, &e.ArtifactDigest,
			&payload, &e.PreviousHash, &e.Hash, &e.Signature,
			&e.SpecVersion, &e.CanonVersion, &e.AdapterVersion, &e.ScoringVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("scanEntries: %w", err)
		}
		e.Type = evidence.EntryType(entryType)
		e.Payload = payload
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
```

**Step 2: Write unit test (compile-only, no DB)**

File: `internal/store/entries_test.go`

```go
package store

import (
	"testing"

	"samebits.com/evidra-benchmark/pkg/evidence"
)

func TestEntryFilter_Defaults(t *testing.T) {
	t.Parallel()
	f := EntryFilter{}
	if f.Limit != 0 {
		t.Errorf("default limit should be zero (applied in query), got %d", f.Limit)
	}
}

func TestEntryTypeRoundTrip(t *testing.T) {
	t.Parallel()
	et := evidence.EntryType("prescribe")
	if string(et) != "prescribe" {
		t.Errorf("entry type string conversion failed")
	}
}
```

**Step 3: Run tests, gofmt, commit**

```bash
gofmt -w internal/store/entries.go internal/store/entries_test.go
go test ./internal/store/ -v -count=1
git add internal/store/
git commit -m "feat: add Postgres evidence entry repository"
```

---

## Phase 2: Auth Middleware

### Task 5: Create auth middleware

**Files:**
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/context.go`
- Create: `internal/auth/middleware_test.go`

**Step 1: Create `internal/auth/context.go`**

Copy from `../evidra-mcp/internal/auth/context.go` verbatim:

```go
package auth

import "context"

type contextKey struct{}

// WithTenantID returns a new context with the tenant ID set.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, tenantID)
}

// TenantID extracts the tenant ID from the context.
// Returns empty string if not set.
func TenantID(ctx context.Context) string {
	v, _ := ctx.Value(contextKey{}).(string)
	return v
}
```

**Step 2: Create `internal/auth/middleware.go`**

Copy from `../evidra-mcp/internal/auth/middleware.go` verbatim, changing only the help URL:

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	staticTenantID = "static"
	jitterMinMS    = 50
	jitterMaxMS    = 100
)

// KeyLookup is the interface satisfied by *store.KeyStore.
type KeyLookup interface {
	LookupKey(ctx context.Context, plaintext string) (tenantID, keyID, prefix string, err error)
	TouchKey(keyID string)
}

// ErrKeyNotFound must be returned by KeyLookup.LookupKey when the key is absent or revoked.
var ErrKeyNotFound = errors.New("key not found")

// StaticKeyMiddleware authenticates requests using constant-time comparison
// against a static API key. Sets tenant_id "static" on success.
func StaticKeyMiddleware(apiKey string) func(http.Handler) http.Handler {
	keyBytes := []byte(apiKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				authFail(w, r)
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
				authFail(w, r)
				return
			}

			ctx := WithTenantID(r.Context(), staticTenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// KeyStoreMiddleware authenticates requests by looking up the Bearer token
// in the key store.
func KeyStoreMiddleware(store KeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				authFail(w, r)
				return
			}

			tenantID, keyID, prefix, err := store.LookupKey(r.Context(), token)
			if err != nil {
				if !errors.Is(err, ErrKeyNotFound) {
					slog.Error("auth: key lookup failed", "error", err)
				}
				authFail(w, r)
				return
			}

			store.TouchKey(keyID)

			slog.Debug("auth: key accepted",
				"prefix", prefix,
				"tenant_id", tenantID,
			)

			ctx := WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return h[len("Bearer "):]
}

func authFail(w http.ResponseWriter, r *http.Request) {
	jitterSleep()

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	slog.Warn("auth failure",
		"method", r.Method,
		"path", r.URL.Path,
		"client_ip", clientIP,
	)

	w.Header().Set("WWW-Authenticate", `Bearer realm="evidra"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":"unauthorized"}`)
}

// jitterSleep sleeps for a random duration between jitterMinMS and jitterMaxMS
// using crypto/rand for the random value.
func jitterSleep() {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(jitterMaxMS-jitterMinMS+1)))
	if err != nil {
		time.Sleep(time.Duration(jitterMinMS) * time.Millisecond)
		return
	}
	ms := jitterMinMS + int(n.Int64())
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
```

**Step 3: Write tests**

File: `internal/auth/middleware_test.go`

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStaticKeyMiddleware_ValidKey(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key-12345678901234567890ab")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid := TenantID(r.Context())
			if tid != "static" {
				t.Errorf("expected tenant_id 'static', got %q", tid)
			}
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-key-12345678901234567890ab")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestStaticKeyMiddleware_InvalidKey(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key-12345678901234567890ab")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestStaticKeyMiddleware_MissingToken(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key-12345678901234567890ab")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		}),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestExtractBearerToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid", "Bearer abc123", "abc123"},
		{"empty", "", ""},
		{"no_bearer", "Basic abc123", ""},
		{"lowercase", "bearer abc123", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestTenantID_EmptyContext(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/", nil)
	tid := TenantID(req.Context())
	if tid != "" {
		t.Errorf("expected empty tenant_id, got %q", tid)
	}
}
```

**Step 4: Run tests, gofmt, commit**

```bash
gofmt -w internal/auth/
go test ./internal/auth/ -v -count=1
git add internal/auth/
git commit -m "feat: add auth middleware with static key and key store support"
```

---

## Phase 3: API Server

### Task 6: Create API scaffolding (middleware, response helpers, health)

**Files:**
- Create: `internal/api/middleware.go`
- Create: `internal/api/response.go`
- Create: `internal/api/health_handler.go`

**Step 1: Create `internal/api/middleware.go`**

Copy from `../evidra-mcp/internal/api/middleware.go` verbatim (no import changes — only stdlib):

```go
package api

import (
	"log/slog"
	"net/http"
	"time"
)

const maxBodyBytes = 1 << 20 // 1MB

func bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				slog.Error("panic recovered",
					"error", rv,
					"method", r.Method,
					"path", r.URL.Path,
				)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}
```

**Step 2: Create `internal/api/response.go`**

Copy from `../evidra-mcp/internal/api/response.go` verbatim:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
```

**Step 3: Create `internal/api/health_handler.go`**

Copy from `../evidra-mcp/internal/api/health_handler.go` verbatim:

```go
package api

import (
	"context"
	"net/http"
)

func handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Pinger can verify database connectivity.
type Pinger interface {
	Ping(ctx context.Context) error
}

func handleReadyz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"error":  "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
```

**Step 4: Run tests (compile check), gofmt, commit**

```bash
gofmt -w internal/api/
go build ./internal/api/
git add internal/api/
git commit -m "feat: add API scaffolding (middleware, response helpers, health)"
```

---

### Task 7: Create public key handler

**Files:**
- Create: `internal/api/pubkey_handler.go`

**Step 1: Create `internal/api/pubkey_handler.go`**

```go
package api

import (
	"log/slog"
	"net/http"

	"samebits.com/evidra-benchmark/internal/evidence"
)

func handlePubkey(signer *evidence.Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if signer == nil {
			writeError(w, http.StatusNotImplemented, "signing not configured")
			return
		}
		pemData, err := signer.PublicKeyPEM()
		if err != nil {
			slog.Error("pubkey handler", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pemData)
	}
}
```

**Step 2: gofmt, commit**

```bash
gofmt -w internal/api/pubkey_handler.go
go build ./internal/api/
git add internal/api/pubkey_handler.go
git commit -m "feat: add public key endpoint handler"
```

---

### Task 8: Create key issuance handler

**Files:**
- Create: `internal/api/keys_handler.go`

**Step 1: Create `internal/api/keys_handler.go`**

Adapted from `../evidra-mcp/internal/api/keys_handler.go`, using `internal/store` from this repo:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"samebits.com/evidra-benchmark/internal/store"
)

const (
	rateLimit   = 3
	rateWindow  = time.Hour
	maxLabelLen = 128
)

type rateLimiter struct {
	mu    sync.Mutex
	store map[string][]time.Time
}

var keyIssuanceRL = &rateLimiter{store: make(map[string][]time.Time)}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateWindow)

	ts := rl.store[ip]
	fresh := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) >= rateLimit {
		rl.store[ip] = fresh
		return false
	}
	rl.store[ip] = append(fresh, now)
	return true
}

func handleKeys(ks *store.KeyStore, inviteSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ks == nil {
			writeError(w, http.StatusNotImplemented,
				"key self-service requires a database; contact the admin to obtain an API key")
			return
		}

		if inviteSecret != "" && r.Header.Get("X-Invite-Token") != inviteSecret {
			writeError(w, http.StatusForbidden, "invalid invite token")
			return
		}

		ip := clientIP(r)
		if !keyIssuanceRL.allow(ip) {
			w.Header().Set("Retry-After", "3600")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded: 3 keys per hour per IP")
			return
		}

		var label string
		if ct := r.Header.Get("Content-Type"); strings.Contains(ct, "application/json") {
			var body struct {
				Label string `json:"label"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				label = body.Label
			}
		}
		if len(label) > maxLabelLen {
			writeError(w, http.StatusBadRequest, "label exceeds 128 characters")
			return
		}

		plaintext, rec, err := ks.CreateKey(r.Context(), label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create key")
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusCreated, map[string]any{
			"key":        plaintext,
			"prefix":     rec.Prefix,
			"tenant_id":  rec.TenantID,
			"created_at": rec.CreatedAt.Format(time.RFC3339),
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}
```

**Step 2: gofmt, commit**

```bash
gofmt -w internal/api/keys_handler.go
go build ./internal/api/
git add internal/api/keys_handler.go
git commit -m "feat: add key issuance endpoint with rate limiting"
```

---

### Task 9: Create evidence forward handler

**Files:**
- Create: `internal/api/forward_handler.go`
- Create: `internal/api/forward_handler_test.go`

This is new code — the core of v0.5.0. It replaces the old `POST /v1/validate`.

**Step 1: Create `internal/api/forward_handler.go`**

```go
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"samebits.com/evidra-benchmark/internal/auth"
	"samebits.com/evidra-benchmark/internal/evidence"
	"samebits.com/evidra-benchmark/internal/store"
	evidencePkg "samebits.com/evidra-benchmark/pkg/evidence"
)

// ReceiptPayload is the payload for receipt entries returned to callers.
type ReceiptPayload struct {
	ReceivedEntryID string `json:"received_entry_id"`
	ServerTimestamp  string `json:"server_timestamp"`
	Sequence        int64  `json:"sequence"`
}

func handleForward(entries *store.EntryStore, signer *evidence.Signer, tenantMode bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var entry evidencePkg.EvidenceEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		// Validate entry type
		if !entry.Type.Valid() {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid entry type %q", entry.Type))
			return
		}

		// Verify hash integrity: recompute hash and compare
		if err := verifyEntryHash(entry); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("hash verification failed: %v", err))
			return
		}

		// Enforce tenant isolation
		if tenantMode && entry.TenantID != "" && entry.TenantID != tenantID {
			writeError(w, http.StatusForbidden, "tenant_id mismatch")
			return
		}
		// Stamp server-side tenant_id
		entry.TenantID = tenantID

		// Store entry
		seq, err := entries.InsertEntry(r.Context(), entry)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store entry")
			return
		}

		// Build receipt
		receiptPayload, _ := json.Marshal(ReceiptPayload{
			ReceivedEntryID: entry.EntryID,
			ServerTimestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			Sequence:        seq,
		})

		receipt, err := evidencePkg.BuildEntry(evidencePkg.EntryBuildParams{
			Type:         evidencePkg.EntryTypeReceipt,
			TenantID:     tenantID,
			TraceID:      entry.TraceID,
			Actor:        evidencePkg.Actor{Type: "system", ID: "evidra-api", Provenance: "server"},
			Payload:      receiptPayload,
			PreviousHash: entry.Hash,
			SpecVersion:  "0.5.0",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to build receipt")
			return
		}

		// Sign receipt if signer available
		if signer != nil {
			sig := signer.Sign([]byte(receipt.Hash))
			receipt.Signature = "ed25519:" + encodeBase64(sig)
		}

		// Store receipt
		if _, err := entries.InsertEntry(r.Context(), receipt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store receipt")
			return
		}

		writeJSON(w, http.StatusOK, receipt)
	}
}

// verifyEntryHash recomputes the entry hash and compares it to the stored hash.
func verifyEntryHash(e evidencePkg.EvidenceEntry) error {
	// Recompute hash using the same method as BuildEntry
	type hashableEntry struct {
		EntryID        string                  `json:"entry_id"`
		PreviousHash   string                  `json:"previous_hash"`
		Type           evidencePkg.EntryType   `json:"type"`
		TenantID       string                  `json:"tenant_id,omitempty"`
		TraceID        string                  `json:"trace_id"`
		Actor          evidencePkg.Actor       `json:"actor"`
		Timestamp      time.Time               `json:"timestamp"`
		IntentDigest   string                  `json:"intent_digest,omitempty"`
		ArtifactDigest string                  `json:"artifact_digest,omitempty"`
		Payload        json.RawMessage         `json:"payload"`
		SpecVersion    string                  `json:"spec_version"`
		CanonVersion   string                  `json:"canonical_version"`
		AdapterVersion string                  `json:"adapter_version"`
		ScoringVersion string                  `json:"scoring_version,omitempty"`
	}

	h := hashableEntry{
		EntryID:        e.EntryID,
		PreviousHash:   e.PreviousHash,
		Type:           e.Type,
		TenantID:       e.TenantID,
		TraceID:        e.TraceID,
		Actor:          e.Actor,
		Timestamp:      e.Timestamp,
		IntentDigest:   e.IntentDigest,
		ArtifactDigest: e.ArtifactDigest,
		Payload:        e.Payload,
		SpecVersion:    e.SpecVersion,
		CanonVersion:   e.CanonVersion,
		AdapterVersion: e.AdapterVersion,
		ScoringVersion: e.ScoringVersion,
	}

	data, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	sum := sha256.Sum256(data)
	computed := "sha256:" + hex.EncodeToString(sum[:])

	if computed != e.Hash {
		return fmt.Errorf("expected %s, got %s", e.Hash, computed)
	}
	return nil
}

func encodeBase64(b []byte) string {
	import_encoding := "encoding/base64"
	_ = import_encoding // handled by import block
	return ""
}
```

**IMPORTANT:** The `encodeBase64` helper above is a placeholder. Replace with a proper import of `encoding/base64` and use `base64.StdEncoding.EncodeToString(sig)` inline. The actual implementation should be:

```go
package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"samebits.com/evidra-benchmark/internal/auth"
	"samebits.com/evidra-benchmark/internal/evidence"
	"samebits.com/evidra-benchmark/internal/store"
	evidencePkg "samebits.com/evidra-benchmark/pkg/evidence"
)

// ReceiptPayload is the payload for receipt entries returned to callers.
type ReceiptPayload struct {
	ReceivedEntryID string `json:"received_entry_id"`
	ServerTimestamp  string `json:"server_timestamp"`
	Sequence        int64  `json:"sequence"`
}

func handleForward(entries *store.EntryStore, signer *evidence.Signer, tenantMode bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var entry evidencePkg.EvidenceEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if !entry.Type.Valid() {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid entry type %q", entry.Type))
			return
		}

		if err := verifyEntryHash(entry); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("hash verification failed: %v", err))
			return
		}

		if tenantMode && entry.TenantID != "" && entry.TenantID != tenantID {
			writeError(w, http.StatusForbidden, "tenant_id mismatch")
			return
		}
		entry.TenantID = tenantID

		seq, err := entries.InsertEntry(r.Context(), entry)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store entry")
			return
		}

		receiptPayload, _ := json.Marshal(ReceiptPayload{
			ReceivedEntryID: entry.EntryID,
			ServerTimestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			Sequence:        seq,
		})

		receipt, err := evidencePkg.BuildEntry(evidencePkg.EntryBuildParams{
			Type:         evidencePkg.EntryTypeReceipt,
			TenantID:     tenantID,
			TraceID:      entry.TraceID,
			Actor:        evidencePkg.Actor{Type: "system", ID: "evidra-api", Provenance: "server"},
			Payload:      receiptPayload,
			PreviousHash: entry.Hash,
			SpecVersion:  "0.5.0",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to build receipt")
			return
		}

		if signer != nil {
			sig := signer.Sign([]byte(receipt.Hash))
			receipt.Signature = "ed25519:" + base64.StdEncoding.EncodeToString(sig)
		}

		if _, err := entries.InsertEntry(r.Context(), receipt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store receipt")
			return
		}

		writeJSON(w, http.StatusOK, receipt)
	}
}

func verifyEntryHash(e evidencePkg.EvidenceEntry) error {
	type hashableEntry struct {
		EntryID        string                `json:"entry_id"`
		PreviousHash   string                `json:"previous_hash"`
		Type           evidencePkg.EntryType `json:"type"`
		TenantID       string                `json:"tenant_id,omitempty"`
		TraceID        string                `json:"trace_id"`
		Actor          evidencePkg.Actor     `json:"actor"`
		Timestamp      time.Time             `json:"timestamp"`
		IntentDigest   string                `json:"intent_digest,omitempty"`
		ArtifactDigest string                `json:"artifact_digest,omitempty"`
		Payload        json.RawMessage       `json:"payload"`
		SpecVersion    string                `json:"spec_version"`
		CanonVersion   string                `json:"canonical_version"`
		AdapterVersion string                `json:"adapter_version"`
		ScoringVersion string                `json:"scoring_version,omitempty"`
	}

	h := hashableEntry{
		EntryID:        e.EntryID,
		PreviousHash:   e.PreviousHash,
		Type:           e.Type,
		TenantID:       e.TenantID,
		TraceID:        e.TraceID,
		Actor:          e.Actor,
		Timestamp:      e.Timestamp,
		IntentDigest:   e.IntentDigest,
		ArtifactDigest: e.ArtifactDigest,
		Payload:        e.Payload,
		SpecVersion:    e.SpecVersion,
		CanonVersion:   e.CanonVersion,
		AdapterVersion: e.AdapterVersion,
		ScoringVersion: e.ScoringVersion,
	}

	data, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	sum := sha256.Sum256(data)
	computed := "sha256:" + hex.EncodeToString(sum[:])

	if computed != e.Hash {
		return fmt.Errorf("expected %s, got %s", e.Hash, computed)
	}
	return nil
}
```

**Step 2: Write test for verifyEntryHash**

File: `internal/api/forward_handler_test.go`

```go
package api

import (
	"encoding/json"
	"testing"
	"time"

	evidencePkg "samebits.com/evidra-benchmark/pkg/evidence"
)

func TestVerifyEntryHash_ValidEntry(t *testing.T) {
	t.Parallel()
	entry, err := evidencePkg.BuildEntry(evidencePkg.EntryBuildParams{
		Type:        evidencePkg.EntryTypePrescribe,
		TraceID:     "trace-1",
		Actor:       evidencePkg.Actor{Type: "agent", ID: "test"},
		Payload:     json.RawMessage(`{"prescription_id":"rx-1"}`),
		SpecVersion: "0.5.0",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := verifyEntryHash(entry); err != nil {
		t.Fatalf("valid entry should pass hash verification: %v", err)
	}
}

func TestVerifyEntryHash_TamperedPayload(t *testing.T) {
	t.Parallel()
	entry, err := evidencePkg.BuildEntry(evidencePkg.EntryBuildParams{
		Type:        evidencePkg.EntryTypePrescribe,
		TraceID:     "trace-1",
		Actor:       evidencePkg.Actor{Type: "agent", ID: "test"},
		Payload:     json.RawMessage(`{"prescription_id":"rx-1"}`),
		SpecVersion: "0.5.0",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with payload
	entry.Payload = json.RawMessage(`{"prescription_id":"rx-TAMPERED"}`)

	if err := verifyEntryHash(entry); err == nil {
		t.Fatal("tampered entry should fail hash verification")
	}
}

func TestVerifyEntryHash_TamperedTimestamp(t *testing.T) {
	t.Parallel()
	entry, err := evidencePkg.BuildEntry(evidencePkg.EntryBuildParams{
		Type:        evidencePkg.EntryTypeReport,
		TraceID:     "trace-2",
		Actor:       evidencePkg.Actor{Type: "agent", ID: "test"},
		Payload:     json.RawMessage(`{"report_id":"rpt-1"}`),
		SpecVersion: "0.5.0",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with timestamp
	entry.Timestamp = entry.Timestamp.Add(1 * time.Hour)

	if err := verifyEntryHash(entry); err == nil {
		t.Fatal("tampered timestamp should fail hash verification")
	}
}
```

**Step 3: Run tests, gofmt, commit**

```bash
gofmt -w internal/api/forward_handler.go internal/api/forward_handler_test.go
go test ./internal/api/ -v -count=1
git add internal/api/forward_handler.go internal/api/forward_handler_test.go
git commit -m "feat: add evidence forward handler with hash verification and signed receipts"
```

---

### Task 10: Create server-side scorecard handler

**Files:**
- Create: `internal/api/scorecard_handler.go`

**Step 1: Create `internal/api/scorecard_handler.go`**

```go
package api

import (
	"net/http"
	"strconv"
	"time"

	"samebits.com/evidra-benchmark/internal/auth"
	"samebits.com/evidra-benchmark/internal/pipeline"
	"samebits.com/evidra-benchmark/internal/score"
	"samebits.com/evidra-benchmark/internal/signal"
	"samebits.com/evidra-benchmark/internal/store"
)

func handleScorecard(entries *store.EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		q := r.URL.Query()
		actorID := q.Get("actor")
		tool := q.Get("tool")
		scope := q.Get("scope")
		periodStr := q.Get("period")

		var since *time.Time
		if periodStr != "" {
			d, err := parsePeriod(periodStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid period: use format like 30d, 7d, 24h")
				return
			}
			t := time.Now().UTC().Add(-d)
			since = &t
		}

		limitStr := q.Get("limit")
		limit := 10000
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
				limit = n
			}
		}

		evidenceEntries, err := entries.ListEntries(r.Context(), store.EntryFilter{
			TenantID: tenantID,
			ActorID:  actorID,
			Tool:     tool,
			Since:    since,
			Limit:    limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to query entries")
			return
		}

		signalEntries := pipeline.EvidenceToSignalEntries(evidenceEntries)

		// Apply scope filter if specified
		if scope != "" {
			var filtered []signal.Entry
			for _, e := range signalEntries {
				if e.ScopeClass == scope {
					filtered = append(filtered, e)
				}
			}
			signalEntries = filtered
		}

		totalOps := 0
		for _, e := range signalEntries {
			if e.IsPrescription {
				totalOps++
			}
		}

		results := signal.AllSignals(signalEntries)
		sc := score.Compute(results, totalOps)

		writeJSON(w, http.StatusOK, sc)
	}
}

func parsePeriod(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, &time.ParseError{}
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}
	switch unit {
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	default:
		return time.ParseDuration(s)
	}
}
```

**Step 2: gofmt, commit**

```bash
gofmt -w internal/api/scorecard_handler.go
go build ./internal/api/
git add internal/api/scorecard_handler.go
git commit -m "feat: add server-side scorecard endpoint"
```

---

### Task 11: Create API router and wire everything together

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/router_test.go`

**Step 1: Create `internal/api/router.go`**

```go
package api

import (
	"net/http"

	"samebits.com/evidra-benchmark/internal/auth"
	"samebits.com/evidra-benchmark/internal/evidence"
	"samebits.com/evidra-benchmark/internal/store"
)

// RouterConfig holds the dependencies for building the API router.
type RouterConfig struct {
	Signer       *evidence.Signer
	APIKey       string          // Phase 0: static key; empty when Store is set.
	Store        *store.KeyStore // Phase 1: nil in Phase 0.
	EntryStore   *store.EntryStore
	DB           Pinger // Phase 1: database pool for readyz; nil in Phase 0.
	InviteSecret string // Phase 1: optional invite gate for POST /v1/keys.
	TenantMode   bool   // Require tenant_id isolation.
}

// NewRouter builds the HTTP handler with all routes and middleware.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Choose auth middleware: DB-backed (Phase 1) or static key (Phase 0).
	var authMW func(http.Handler) http.Handler
	if cfg.Store != nil {
		authMW = auth.KeyStoreMiddleware(cfg.Store)
	} else {
		authMW = auth.StaticKeyMiddleware(cfg.APIKey)
	}

	// Public endpoints (no auth).
	mux.Handle("GET /healthz", handleHealthz())
	mux.Handle("GET /v1/evidence/pubkey", handlePubkey(cfg.Signer))
	mux.Handle("POST /v1/keys", handleKeys(cfg.Store, cfg.InviteSecret))

	// Phase 1: readyz requires a database.
	if cfg.DB != nil {
		mux.Handle("GET /readyz", handleReadyz(cfg.DB))
	}

	// Auth check endpoint (forwardAuth target for Traefik/reverse proxies).
	authCheckHandler := authMW(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := auth.TenantID(r.Context())
			w.Header().Set("X-Evidra-Tenant", tenantID)
			w.WriteHeader(http.StatusOK)
		}),
	)
	mux.Handle("GET /auth/check", authCheckHandler)
	mux.Handle("HEAD /auth/check", authCheckHandler)

	// Authenticated endpoints.
	mux.Handle("POST /v1/evidence/forward", authMW(
		handleForward(cfg.EntryStore, cfg.Signer, cfg.TenantMode),
	))
	mux.Handle("GET /v1/evidence/entries", authMW(
		handleListEntries(cfg.EntryStore),
	))
	mux.Handle("GET /v1/evidence/entries/{id}", authMW(
		handleGetEntry(cfg.EntryStore),
	))
	mux.Handle("GET /v1/evidence/scorecard", authMW(
		handleScorecard(cfg.EntryStore),
	))

	// Middleware stack: recovery -> logging -> body limit -> router.
	var handler http.Handler = mux
	handler = bodyLimitMiddleware(handler)
	handler = requestLogMiddleware(handler)
	handler = recoveryMiddleware(handler)

	return handler
}
```

**Step 2: Create entry list/get handlers**

File: `internal/api/entries_handler.go`

```go
package api

import (
	"net/http"
	"strconv"
	"time"

	"samebits.com/evidra-benchmark/internal/auth"
	"samebits.com/evidra-benchmark/internal/store"
)

func handleListEntries(entries *store.EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()

		var since *time.Time
		if s := q.Get("since"); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid since: use RFC3339 format")
				return
			}
			since = &t
		}

		limit := 100
		if s := q.Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		offset := 0
		if s := q.Get("offset"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				offset = n
			}
		}

		results, err := entries.ListEntries(r.Context(), store.EntryFilter{
			TenantID: tenantID,
			ActorID:  q.Get("actor"),
			Type:     q.Get("type"),
			TraceID:  q.Get("trace_id"),
			Since:    since,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to query entries")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"entries": results,
			"count":   len(results),
		})
	}
}

func handleGetEntry(entries *store.EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		entryID := r.PathValue("id")

		entry, err := entries.GetEntry(r.Context(), tenantID, entryID)
		if err != nil {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}

		writeJSON(w, http.StatusOK, entry)
	}
}
```

**Step 3: Write router tests**

File: `internal/api/router_test.go`

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	t.Parallel()
	cfg := RouterConfig{
		APIKey: "test-key-12345678901234567890ab",
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz: expected 200, got %d", w.Code)
	}
}

func TestForward_RequiresAuth(t *testing.T) {
	t.Parallel()
	cfg := RouterConfig{
		APIKey: "test-key-12345678901234567890ab",
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("POST", "/v1/evidence/forward", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("forward without auth: expected 401, got %d", w.Code)
	}
}

func TestPubkey_NoSigner(t *testing.T) {
	t.Parallel()
	cfg := RouterConfig{
		APIKey: "test-key-12345678901234567890ab",
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/v1/evidence/pubkey", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("pubkey without signer: expected 501, got %d", w.Code)
	}
}

func TestKeys_NotImplemented_Phase0(t *testing.T) {
	t.Parallel()
	cfg := RouterConfig{
		APIKey: "test-key-12345678901234567890ab",
	}
	handler := NewRouter(cfg)

	req := httptest.NewRequest("POST", "/v1/keys", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("keys in Phase 0: expected 501, got %d", w.Code)
	}
}
```

**Step 4: Run tests, gofmt, commit**

```bash
gofmt -w internal/api/
go test ./internal/api/ -v -count=1
git add internal/api/
git commit -m "feat: add API router with all endpoints wired"
```

---

### Task 12: Create cmd/evidra-api binary

**Files:**
- Create: `cmd/evidra-api/main.go`

**Step 1: Create `cmd/evidra-api/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"samebits.com/evidra-benchmark/internal/api"
	"samebits.com/evidra-benchmark/internal/db"
	"samebits.com/evidra-benchmark/internal/evidence"
	"samebits.com/evidra-benchmark/internal/store"
	"samebits.com/evidra-benchmark/pkg/version"
)

func main() {
	// Version flag
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.String())
		os.Exit(0)
	}

	// Configuration from environment
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	databaseURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("EVIDRA_API_KEY")
	signingKey := os.Getenv("EVIDRA_SIGNING_KEY")
	signingKeyPath := os.Getenv("EVIDRA_SIGNING_KEY_PATH")
	environment := os.Getenv("EVIDRA_ENVIRONMENT")
	inviteSecret := os.Getenv("EVIDRA_INVITE_SECRET")
	tenantMode := os.Getenv("EVIDRA_TENANT_MODE") == "true"

	// Log level
	logLevel := slog.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	isDev := strings.EqualFold(environment, "development")

	// Signer (optional)
	var signer *evidence.Signer
	signerCfg := evidence.SignerConfig{
		KeyBase64: signingKey,
		KeyPath:   signingKeyPath,
		DevMode:   isDev,
	}
	if signingKey != "" || signingKeyPath != "" || isDev {
		s, err := evidence.NewSigner(signerCfg)
		if err != nil {
			slog.Error("signing disabled", "error", err)
		} else {
			signer = s
			slog.Info("signing enabled")
		}
	}

	// Auth validation
	if apiKey == "" && databaseURL == "" {
		fmt.Fprintln(os.Stderr, "fatal: set EVIDRA_API_KEY (Phase 0) or DATABASE_URL (Phase 1)")
		os.Exit(1)
	}
	if apiKey != "" && len(apiKey) < 32 {
		fmt.Fprintln(os.Stderr, "fatal: EVIDRA_API_KEY must be at least 32 characters")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	routerCfg := api.RouterConfig{
		Signer:       signer,
		APIKey:       apiKey,
		InviteSecret: inviteSecret,
		TenantMode:   tenantMode,
	}

	// Database (Phase 1)
	if databaseURL != "" {
		pool, err := db.Connect(ctx, databaseURL)
		if err != nil {
			slog.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer pool.Close()

		ks := store.New(pool)
		es := store.NewEntryStore(pool)

		routerCfg.Store = ks
		routerCfg.EntryStore = es
		routerCfg.DB = pool
		slog.Info("database connected (Phase 1)")
	} else {
		slog.Info("running without database (Phase 0)")
	}

	handler := api.NewRouter(routerCfg)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("evidra-api starting",
			"addr", listenAddr,
			"version", version.String(),
			"phase", phase(databaseURL),
		)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func phase(databaseURL string) string {
	if databaseURL != "" {
		return "1"
	}
	return "0"
}
```

**Step 2: Build, gofmt, commit**

```bash
gofmt -w cmd/evidra-api/main.go
go build ./cmd/evidra-api/
git add cmd/evidra-api/
git commit -m "feat: bootstrap evidra-api server binary"
```

---

## Phase 4: Wire Forwarding into Clients

### Task 13: Wire evidence forwarding into MCP server

**Files:**
- Modify: `pkg/mcpserver/server.go` — add ForwardURL to Options, POST entries after local write

**Step 1: Add ForwardURL and API key to Options**

Add to the `Options` struct:

```go
ForwardURL string // e.g. "https://api.evidra.io" — empty disables forwarding
ForwardKey string // Bearer token for API authentication
```

Add to `BenchmarkService`:

```go
forwardURL string
forwardKey string
```

**Step 2: Add forwardEntry method**

```go
func (s *BenchmarkService) forwardEntry(entry evidencePkg.EvidenceEntry) {
	if s.forwardURL == "" {
		return
	}
	go func() {
		body, err := json.Marshal(entry)
		if err != nil {
			slog.Warn("forward: marshal error", "error", err)
			return
		}
		req, err := http.NewRequest("POST", s.forwardURL+"/v1/evidence/forward", bytes.NewReader(body))
		if err != nil {
			slog.Warn("forward: request error", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if s.forwardKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.forwardKey)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("forward: request failed", "error", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var receipt evidencePkg.EvidenceEntry
			if json.NewDecoder(resp.Body).Decode(&receipt) == nil {
				_ = evidencePkg.AppendEntryAtPath(s.evidencePath, receipt)
			}
		} else {
			slog.Warn("forward: unexpected status", "status", resp.StatusCode)
		}
	}()
}
```

**Step 3: Call forwardEntry after AppendEntryAtPath in Prescribe() and Report()**

After each `evidencePkg.AppendEntryAtPath(s.evidencePath, entry)` call, add:

```go
s.forwardEntry(entry)
```

**Step 4: Wire env vars in `cmd/evidra-mcp/main.go`**

```go
forwardURL := os.Getenv("EVIDRA_API_URL")
forwardKey := os.Getenv("EVIDRA_API_KEY")
```

Pass to Options:

```go
opts := mcpserver.Options{
    // ... existing fields ...
    ForwardURL: forwardURL,
    ForwardKey: forwardKey,
}
```

**Step 5: Run tests, gofmt, commit**

```bash
gofmt -w pkg/mcpserver/server.go cmd/evidra-mcp/main.go
go build ./cmd/evidra-mcp/
go test ./pkg/mcpserver/ -v -count=1
git add pkg/mcpserver/server.go cmd/evidra-mcp/main.go
git commit -m "feat: wire evidence forwarding to remote API in MCP server"
```

---

### Task 14: Wire evidence forwarding into CLI

**Files:**
- Modify: `cmd/evidra/main.go` — add `--api-url` and `--api-key` flags to prescribe and report

**Step 1: Add flags to cmdPrescribe and cmdReport**

```go
apiURLFlag := fs.String("api-url", "", "Evidra API URL for evidence forwarding (or set EVIDRA_API_URL)")
apiKeyFlag := fs.String("api-key", "", "API key for evidence forwarding (or set EVIDRA_API_KEY)")
```

**Step 2: Add forwardEntry helper**

In `cmd/evidra/main.go`, add a standalone function:

```go
func forwardEntry(entry evidencePkg.EvidenceEntry, apiURL, apiKey, evidencePath string) {
	if apiURL == "" {
		return
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", apiURL+"/v1/evidence/forward", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var receipt evidencePkg.EvidenceEntry
		if json.NewDecoder(resp.Body).Decode(&receipt) == nil {
			_ = evidencePkg.AppendEntryAtPath(evidencePath, receipt)
		}
	}
}
```

**Step 3: Call after local write in prescribe and report**

After `AppendEntryAtPath`, resolve apiURL:

```go
apiURL := *apiURLFlag
if apiURL == "" {
    apiURL = os.Getenv("EVIDRA_API_URL")
}
apiKey := *apiKeyFlag
if apiKey == "" {
    apiKey = os.Getenv("EVIDRA_API_KEY")
}
forwardEntry(entry, apiURL, apiKey, evidencePath)
```

**Step 4: Build, gofmt, commit**

```bash
gofmt -w cmd/evidra/main.go
go build ./cmd/evidra/
git add cmd/evidra/main.go
git commit -m "feat: add --api-url flag for evidence forwarding in CLI"
```

---

## Phase 5: Dockerfile & Makefile

### Task 15: Create API Dockerfile and update Makefile

**Files:**
- Create: `Dockerfile.api`
- Modify: `Makefile` — add `docker-api` target

**Step 1: Create `Dockerfile.api`**

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /evidra-api ./cmd/evidra-api/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /evidra-api /evidra-api
ENTRYPOINT ["/evidra-api"]
```

**Step 2: Add Makefile target**

Add to Makefile:

```makefile
docker-api:
	docker build -f Dockerfile.api -t evidra-api:latest .
```

**Step 3: Update `docker-compose.yml` with API service**

Add to existing docker-compose.yml:

```yaml
  evidra-api:
    build:
      context: .
      dockerfile: Dockerfile.api
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://evidra:evidra@postgres:5432/evidra?sslmode=disable
      - EVIDRA_API_KEY=${EVIDRA_API_KEY:-dev-key-12345678901234567890abcdef}
      - EVIDRA_ENVIRONMENT=development
      - LOG_LEVEL=debug
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: evidra
      POSTGRES_PASSWORD: evidra
      POSTGRES_DB: evidra
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U evidra"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

**Step 4: Build, commit**

```bash
go build ./cmd/evidra-api/
git add Dockerfile.api Makefile docker-compose.yml
git commit -m "feat: add API Dockerfile and docker-compose with Postgres"
```

---

## Phase 6: Integration Tests

### Task 16: Write integration tests for API + DB

**Files:**
- Create: `cmd/evidra-api/integration_test.go`

**Build tag:** `//go:build integration`

**Step 1: Create integration test**

```go
//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"samebits.com/evidra-benchmark/internal/api"
	"samebits.com/evidra-benchmark/internal/db"
	"samebits.com/evidra-benchmark/internal/evidence"
	"samebits.com/evidra-benchmark/internal/store"
	evidencePkg "samebits.com/evidra-benchmark/pkg/evidence"
)

func startTestServer(t *testing.T) (*httptest.Server, *evidence.Signer) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://evidra:evidra@localhost:5432/evidra?sslmode=disable"
	}

	pool, err := db.Connect(t.Context(), dbURL)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)

	signer, err := evidence.NewSigner(evidence.SignerConfig{DevMode: true})
	if err != nil {
		t.Fatal(err)
	}

	ks := store.New(pool)
	es := store.NewEntryStore(pool)

	handler := api.NewRouter(api.RouterConfig{
		Signer:     signer,
		APIKey:     "test-key-12345678901234567890ab",
		Store:      ks,
		EntryStore: es,
		DB:         pool,
	})

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, signer
}

func TestIntegration_Healthz(t *testing.T) {
	ts, _ := startTestServer(t)
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("healthz: expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_Readyz(t *testing.T) {
	ts, _ := startTestServer(t)
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("readyz: expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_ForwardAndRetrieve(t *testing.T) {
	ts, _ := startTestServer(t)
	apiKey := "test-key-12345678901234567890ab"

	// Build a test entry
	entry, err := evidencePkg.BuildEntry(evidencePkg.EntryBuildParams{
		Type:        evidencePkg.EntryTypePrescribe,
		TraceID:     "trace-integ-1",
		Actor:       evidencePkg.Actor{Type: "agent", ID: "test-agent", Provenance: "integration-test"},
		Payload:     json.RawMessage(`{"prescription_id":"rx-integ-1","canonical_action":{},"risk_level":"low","risk_tags":[],"ttl_ms":300000,"canon_source":"adapter"}`),
		SpecVersion: "0.5.0",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Forward entry
	body, _ := json.Marshal(entry)
	req, _ := http.NewRequest("POST", ts.URL+"/v1/evidence/forward", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("forward: expected 200, got %d", resp.StatusCode)
	}

	// Verify receipt
	var receipt evidencePkg.EvidenceEntry
	if err := json.NewDecoder(resp.Body).Decode(&receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	resp.Body.Close()

	if receipt.Type != evidencePkg.EntryTypeReceipt {
		t.Errorf("receipt type: expected 'receipt', got %q", receipt.Type)
	}
	if receipt.Signature == "" {
		t.Error("receipt should be signed")
	}

	// Retrieve entry by ID
	req, _ = http.NewRequest("GET", ts.URL+"/v1/evidence/entries/"+entry.EntryID, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("get entry: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestIntegration_AuthRequired(t *testing.T) {
	ts, _ := startTestServer(t)

	req, _ := http.NewRequest("POST", ts.URL+"/v1/evidence/forward", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("forward without auth: expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_Pubkey(t *testing.T) {
	ts, _ := startTestServer(t)
	resp, err := http.Get(ts.URL + "/v1/evidence/pubkey")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("pubkey: expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-pem-file" {
		t.Errorf("pubkey content-type: expected application/x-pem-file, got %q", ct)
	}
}
```

**Step 2: Run integration test (requires Postgres)**

```bash
# Start Postgres with docker-compose first:
docker compose up -d postgres
sleep 3

# Run integration tests:
go test -tags integration ./cmd/evidra-api/ -v -count=1

# Or skip if no Postgres:
echo "Integration tests require: docker compose up -d postgres"
```

**Step 3: gofmt, commit**

```bash
gofmt -w cmd/evidra-api/integration_test.go
git add cmd/evidra-api/integration_test.go
git commit -m "test: add integration tests for evidra-api with Postgres"
```

---

## Phase 7: Final Verification

### Task 17: Full build and test verification

**Step 1: Build all binaries**

```bash
go build ./cmd/evidra/ ./cmd/evidra-mcp/ ./cmd/evidra-api/
```

**Step 2: All unit tests pass**

```bash
go test ./... -v -count=1
```

**Step 3: Race detector**

```bash
go test -race ./...
```

**Step 4: Lint**

```bash
gofmt -l .
make lint
```

**Step 5: Integration tests (with Postgres)**

```bash
docker compose up -d postgres
go test -tags integration ./cmd/evidra-api/ -v -count=1
```

**Step 6: Commit and tag**

```bash
git tag v0.5.0-dev
```

Target: 230+ tests passing, 3 binaries building, integration tests green.

---

## Reuse Map (from evidra-mcp)

| Source File | Target File | Adaptation |
|---|---|---|
| `internal/db/db.go` | `internal/db/db.go` | Verbatim (import path only) |
| `internal/db/migrations/001_keys.sql` | `internal/db/migrations/001_tenants_and_keys.sql` | Verbatim |
| — | `internal/db/migrations/002_evidence_entries.sql` | New (matches EvidenceEntry struct) |
| `internal/store/keys.go` | `internal/store/keys.go` | Verbatim (no evidra imports) |
| — | `internal/store/entries.go` | New (evidence entry CRUD) |
| `internal/auth/middleware.go` | `internal/auth/middleware.go` | Verbatim (remove help URL) |
| `internal/auth/context.go` | `internal/auth/context.go` | Verbatim |
| `internal/api/middleware.go` | `internal/api/middleware.go` | Verbatim |
| `internal/api/response.go` | `internal/api/response.go` | Verbatim |
| `internal/api/health_handler.go` | `internal/api/health_handler.go` | Verbatim |
| `internal/api/pubkey_handler.go` | `internal/api/pubkey_handler.go` | Adapted (nil signer check) |
| `internal/api/keys_handler.go` | `internal/api/keys_handler.go` | Adapted (internal/store import) |
| `internal/api/router.go` | `internal/api/router.go` | Rewritten (benchmark endpoints) |
| `internal/api/validate_handler.go` | — | Not copied (replaced by forward_handler) |
| `internal/api/ui_handler.go` | — | Not copied (no UI in benchmark) |
| `cmd/evidra-api/main.go` | `cmd/evidra-api/main.go` | Rewritten (no OPA, benchmark service) |

## Do NOT Reuse

| Module | Reason |
|---|---|
| `pkg/policy/*` | OPA policy engine — not benchmark semantics |
| `pkg/runtime/*` | OPA runtime — not needed |
| `internal/engine/*` | OPA evaluation adapter — replaced by canon/risk |
| `pkg/bundlesource/*` | OPA bundle loading — not needed |
| `ui/` | React dashboard — replaced by CLI scorecard |
