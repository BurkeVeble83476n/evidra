package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra/pkg/bench"
)

// fakeRepo is an in-memory fake implementing Repository for unit tests.
type fakeRepo struct {
	leaderboardTenant string
	leaderboardMode   string
	beginTxErr        error
	tx                pgx.Tx
}

func (f *fakeRepo) ListRuns(_ context.Context, _ string, _ bench.RunFilters) ([]bench.RunRecord, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) GetRun(_ context.Context, _ string, _ string) (*bench.RunRecord, error) {
	return nil, nil
}
func (f *fakeRepo) InsertRun(_ context.Context, _ string, _ bench.RunRecord) error { return nil }
func (f *fakeRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return 0, nil
}
func (f *fakeRepo) FilteredStats(_ context.Context, _ string, _ bench.RunFilters) (*bench.StatsResult, error) {
	return nil, nil
}
func (f *fakeRepo) Catalog(_ context.Context, _ string) (*bench.RunCatalog, error) { return nil, nil }
func (f *fakeRepo) Leaderboard(_ context.Context, tenantID string, evidenceMode string) ([]bench.LeaderboardEntry, error) {
	f.leaderboardTenant = tenantID
	f.leaderboardMode = evidenceMode
	return nil, nil
}
func (f *fakeRepo) ListScenarios(_ context.Context) ([]bench.ScenarioSummary, error) {
	return nil, nil
}
func (f *fakeRepo) StoreArtifact(_ context.Context, _, _, _ string, _ []byte) error { return nil }
func (f *fakeRepo) GetArtifact(_ context.Context, _, _, _ string) ([]byte, string, error) {
	return nil, "", nil
}
func (f *fakeRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	if f.beginTxErr != nil {
		return nil, f.beginTxErr
	}
	if f.tx != nil {
		return f.tx, nil
	}
	return nil, fmt.Errorf("fakeRepo: no real tx available")
}

func TestServiceListRuns_UsesProvidedTenant(t *testing.T) {
	t.Parallel()

	// Verify that buildWhere passes tenantID correctly.
	where, args := buildWhere("tenant-b", bench.RunFilters{})
	if len(args) == 0 || args[0] != "tenant-b" {
		t.Fatalf("buildWhere args[0] = %v, want tenant-b", args)
	}
	if where == "" {
		t.Fatal("buildWhere returned empty WHERE clause")
	}

	// Verify Service construction and that it stores the config.
	svc := NewService(&fakeRepo{}, ServiceConfig{PublicTenant: "bench-public"})
	if svc.cfg.PublicTenant != "bench-public" {
		t.Fatalf("PublicTenant = %q, want bench-public", svc.cfg.PublicTenant)
	}
}

func TestServiceLeaderboard_UsesPublicTenant(t *testing.T) {
	t.Parallel()

	// When PublicTenant is empty, Leaderboard must return ErrPublicTenantUnavailable.
	svc := NewService(&fakeRepo{}, ServiceConfig{})
	_, err := svc.Leaderboard(context.Background(), "proxy")
	if !errors.Is(err, ErrPublicTenantUnavailable) {
		t.Fatalf("Leaderboard err = %v, want ErrPublicTenantUnavailable", err)
	}

	// When PublicTenant is set, the repo's Leaderboard should be called
	// with the configured public tenant.
	repo := &fakeRepo{}
	svc2 := NewService(repo, ServiceConfig{PublicTenant: "bench-public"})
	_, _ = svc2.Leaderboard(context.Background(), "proxy")
	if repo.leaderboardTenant != "bench-public" {
		t.Fatalf("leaderboardTenant = %q, want bench-public", repo.leaderboardTenant)
	}
}

func TestServiceIngestRun_RequiresTransaction(t *testing.T) {
	t.Parallel()

	// IngestRun must call BeginTx. With fakeRepo it returns an error.
	svc := NewService(&fakeRepo{}, ServiceConfig{})

	err := svc.IngestRun(context.Background(), "tenant-a", IngestRunRequest{
		RunRecord:  bench.RunRecord{ID: "run-1", ScenarioID: "s1", Model: "m1"},
		Transcript: "hello",
	})
	if err == nil {
		t.Fatal("expected error from fakeRepo BeginTx, got nil")
	}
}

func TestBuildWhere_TenantAlwaysFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tenant   string
		filters  bench.RunFilters
		wantArgs int
	}{
		{
			name:     "tenant only",
			tenant:   "t1",
			filters:  bench.RunFilters{},
			wantArgs: 1,
		},
		{
			name:   "tenant plus scenario",
			tenant: "t2",
			filters: bench.RunFilters{
				ScenarioID: "broken-deployment",
			},
			wantArgs: 2,
		},
		{
			name:   "tenant plus model and provider",
			tenant: "t3",
			filters: bench.RunFilters{
				Model:    "sonnet",
				Provider: "anthropic",
			},
			wantArgs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			where, args := buildWhere(tt.tenant, tt.filters)
			if args[0] != tt.tenant {
				t.Errorf("first arg = %v, want %v", args[0], tt.tenant)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("args count = %d, want %d", len(args), tt.wantArgs)
			}
			if where == "" {
				t.Error("WHERE clause is empty")
			}
		})
	}
}

func TestIngestRunRequest_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	req := IngestRunRequest{
		RunRecord:  bench.RunRecord{ID: "r1", ScenarioID: "s1", Model: "m1"},
		Transcript: "step 1\nstep 2",
		ToolCalls:  json.RawMessage(`[{"tool":"kubectl","args":["get","pods"]}]`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded IngestRunRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != "r1" {
		t.Errorf("ID = %q, want r1", decoded.ID)
	}
	if decoded.Transcript != req.Transcript {
		t.Errorf("Transcript = %q, want %q", decoded.Transcript, req.Transcript)
	}
	if string(decoded.ToolCalls) != string(req.ToolCalls) {
		t.Errorf("ToolCalls = %s, want %s", decoded.ToolCalls, req.ToolCalls)
	}
}

func TestServiceConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := ServiceConfig{}
	if cfg.PublicTenant != "" {
		t.Errorf("default PublicTenant = %q, want empty", cfg.PublicTenant)
	}
}
