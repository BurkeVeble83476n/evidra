package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestHashKey_Deterministic(t *testing.T) {
	t.Parallel()
	h1 := hashKey("ev1_testkey123")
	h2 := hashKey("ev1_testkey123")
	if string(h1) != string(h2) {
		t.Fatal("hashKey should be deterministic")
	}
}

func TestHashKey_DifferentInputs(t *testing.T) {
	t.Parallel()
	h1 := hashKey("ev1_key_a")
	h2 := hashKey("ev1_key_b")
	if string(h1) == string(h2) {
		t.Fatal("different keys should produce different hashes")
	}
}

func TestGenerateKeyPlaintext(t *testing.T) {
	t.Parallel()
	key, err := generateKeyPlaintext()
	if err != nil {
		t.Fatalf("generateKeyPlaintext: %v", err)
	}
	if len(key) < 32 {
		t.Fatalf("key too short: %d", len(key))
	}
	if key[:4] != "ev1_" {
		t.Fatalf("key should start with ev1_, got: %s", key[:4])
	}
}

func TestCreateKey_CreatesTenantBeforeKeyInsert(t *testing.T) {
	t.Parallel()

	tx := &fakeKeyTx{}
	ks := &KeyStore{
		begin: func(context.Context) (keyTx, error) {
			return tx, nil
		},
	}

	plaintext, rec, err := ks.CreateKey(context.Background(), "tnt_test", "pipeline")
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if plaintext == "" {
		t.Fatal("expected plaintext key")
	}
	if rec.TenantID != "tnt_test" {
		t.Fatalf("tenant_id = %q, want tnt_test", rec.TenantID)
	}
	if !tx.committed {
		t.Fatal("expected transaction commit")
	}
	if len(tx.execs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(tx.execs))
	}
	if !strings.Contains(tx.execs[0].sql, "INSERT INTO tenants") {
		t.Fatalf("first exec should insert tenant, got %q", tx.execs[0].sql)
	}
	if got := tx.execs[0].args[0]; got != "tnt_test" {
		t.Fatalf("tenant insert arg = %v, want tnt_test", got)
	}
	if !strings.Contains(tx.execs[1].sql, "INSERT INTO api_keys") {
		t.Fatalf("second exec should insert api key, got %q", tx.execs[1].sql)
	}
	if got := tx.execs[1].args[1]; got != "tnt_test" {
		t.Fatalf("api key tenant arg = %v, want tnt_test", got)
	}
}

func TestLookupKey_TouchesLastUsedWithBoundedContext(t *testing.T) {
	t.Parallel()

	touched := false
	ks := &KeyStore{
		lookupFn: func(_ context.Context, hash []byte) (KeyRecord, error) {
			if len(hash) == 0 {
				t.Fatal("expected hashed lookup key")
			}
			return KeyRecord{ID: "key-1", TenantID: "tenant-1"}, nil
		},
		touchFn: func(ctx context.Context, keyID string) error {
			touched = true
			if keyID != "key-1" {
				t.Fatalf("touch key id = %q, want key-1", keyID)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("expected bounded touch context deadline")
			}
			return nil
		},
	}

	rec, err := ks.LookupKey(context.Background(), "ev1_lookup")
	if err != nil {
		t.Fatalf("LookupKey: %v", err)
	}
	if rec.ID != "key-1" {
		t.Fatalf("record id = %q, want key-1", rec.ID)
	}
	if !touched {
		t.Fatal("expected last_used_at touch")
	}
}

func TestLookupKey_IgnoresTouchErrors(t *testing.T) {
	t.Parallel()

	ks := &KeyStore{
		lookupFn: func(context.Context, []byte) (KeyRecord, error) {
			return KeyRecord{ID: "key-1", TenantID: "tenant-1"}, nil
		},
		touchFn: func(context.Context, string) error {
			return errors.New("touch failed")
		},
	}

	rec, err := ks.LookupKey(context.Background(), "ev1_lookup")
	if err != nil {
		t.Fatalf("LookupKey: %v", err)
	}
	if rec.ID != "key-1" {
		t.Fatalf("record id = %q, want key-1", rec.ID)
	}
}

type fakeKeyExec struct {
	sql  string
	args []interface{}
}

type fakeKeyTx struct {
	execs      []fakeKeyExec
	committed  bool
	rolledBack bool
}

func (f *fakeKeyTx) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	f.execs = append(f.execs, fakeKeyExec{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

func (f *fakeKeyTx) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeKeyTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}
