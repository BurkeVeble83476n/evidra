//go:build integration

package benchsvc

import (
	"context"
	"testing"
)

func TestPgStore_RegisterAndListRunners(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	runner, err := store.RegisterRunner(context.Background(), tenantID, RegisterRunnerRequest{
		Name:        "test-runner",
		Models:      []string{"deepseek-chat", "qwen-plus"},
		Provider:    "bifrost",
		Region:      "eu-west",
		MaxParallel: 2,
	})
	if err != nil {
		t.Fatalf("RegisterRunner: %v", err)
	}
	if runner.ID == "" {
		t.Fatal("runner ID is empty")
	}
	if runner.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", runner.Status)
	}

	runners, err := store.ListRunners(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListRunners: %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("len = %d, want 1", len(runners))
	}
	if len(runners[0].Config.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(runners[0].Config.Models))
	}
}

func TestPgStore_DeleteRunner(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	runner, _ := store.RegisterRunner(context.Background(), tenantID, RegisterRunnerRequest{
		Name:   "delete-me",
		Models: []string{"test-model"},
	})

	if err := store.DeleteRunner(context.Background(), tenantID, runner.ID); err != nil {
		t.Fatalf("DeleteRunner: %v", err)
	}

	runners, _ := store.ListRunners(context.Background(), tenantID)
	if len(runners) != 0 {
		t.Fatalf("len = %d, want 0", len(runners))
	}
}
