//go:build integration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"samebits.com/evidra/internal/db"
)

func TestEntryStoreClaimWebhookEvent_EmptyPayloadUsesEmptyObject(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	defer pool.Close()

	es := NewEntryStore(pool)
	ctx := context.Background()
	tenantID := "tenant-empty-claim-payload"
	source := "agentgateway"
	key := "claim-empty-payload-" + ulid.Make().String()

	_, err = pool.Exec(ctx,
		`INSERT INTO tenants (id, label) VALUES ($1, $2)
		 ON CONFLICT (id) DO NOTHING`,
		tenantID, "Integration Tenant",
	)
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	duplicate, err := es.ClaimWebhookEvent(ctx, tenantID, source, key, nil)
	if err != nil {
		t.Fatalf("ClaimWebhookEvent: %v", err)
	}
	if duplicate {
		t.Fatal("expected first claim to be non-duplicate")
	}

	var payload string
	err = pool.QueryRow(ctx,
		`SELECT payload::text
		 FROM webhook_events
		 WHERE tenant_id = $1 AND source = $2 AND idempotency_key = $3`,
		tenantID, source, key,
	).Scan(&payload)
	if err != nil {
		t.Fatalf("select payload: %v", err)
	}
	if payload != "{}" {
		t.Fatalf("payload = %q, want %q", payload, "{}")
	}
}
