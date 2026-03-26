//go:build integration

package benchsvc

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"samebits.com/evidra/internal/db"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	resetModelConfigTables(t, pool)
	return pool
}

func resetModelConfigTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(context.Background(), `DELETE FROM bench_tenant_providers`); err != nil {
		t.Fatalf("clear bench_tenant_providers: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM bench_models`); err != nil {
		t.Fatalf("clear bench_models: %v", err)
	}
}

func testID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToLower(ulid.Make().String()))
}

func seedTenant(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, label) VALUES ($1, $2)
		 ON CONFLICT (id) DO NOTHING`,
		tenantID, "Benchsvc Integration Tenant")
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
}

func seedModel(t *testing.T, pool *pgxpool.Pool, modelID string) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO bench_models (id, display_name, provider, input_cost_per_mtok, output_cost_per_mtok)
		 VALUES ($1, $2, $3, $4, $5)`,
		modelID, "Test Model", "custom", 0.5, 1.0)
	if err != nil {
		t.Fatalf("insert model: %v", err)
	}
}

func seedModelWithEnv(t *testing.T, pool *pgxpool.Pool, modelID, apiKeyEnv string) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO bench_models (id, display_name, provider, api_key_env, input_cost_per_mtok, output_cost_per_mtok)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		modelID, "Platform Model", "google", apiKeyEnv, 0.15, 0.60)
	if err != nil {
		t.Fatalf("insert model with env: %v", err)
	}
}

func TestPgStore_ListEnabledModels(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	tenantID := testID("tnt")
	platformModelID := testID("gemini")
	customModelID := testID("custom")

	seedTenant(t, pool, tenantID)
	seedModelWithEnv(t, pool, platformModelID, "GEMINI_API_KEY")
	seedModel(t, pool, customModelID)

	_, err := pool.Exec(context.Background(),
		`INSERT INTO bench_tenant_providers (tenant_id, model_id, api_key_enc, enabled)
		 VALUES ($1, $2, $3, true)`,
		tenantID, customModelID, "sk-secret")
	if err != nil {
		t.Fatalf("insert tenant provider: %v", err)
	}

	models, err := store.ListEnabledModels(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListEnabledModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}
