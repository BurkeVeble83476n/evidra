package api

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"samebits.com/evidra/internal/benchsvc"
	bench "samebits.com/evidra/pkg/bench"
)

func TestRouter_HealthzNoAuth(t *testing.T) {
	t.Parallel()
	cfg := RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "t1",
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouter_PubkeyNoAuth(t *testing.T) {
	t.Parallel()
	pub, _, _ := ed25519.GenerateKey(nil)
	cfg := RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "t1",
		PublicKey:     pub,
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/v1/evidence/pubkey", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouter_ForwardRequiresAuth(t *testing.T) {
	t.Parallel()
	cfg := RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "t1",
		RawStore:      &fakeEntryStore{},
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest("POST", "/v1/evidence/forward", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRouter_AdminBenchModelRoute_RequiresInviteSecret(t *testing.T) {
	t.Parallel()

	repo := &routerBenchRepo{}
	router := NewRouter(RouterConfig{
		BenchService: benchsvc.NewService(repo, benchsvc.ServiceConfig{}),
		InviteSecret: "invite-secret",
	})

	req := httptest.NewRequest("PUT", "/v1/admin/bench/models/gemini-2.5-flash", strings.NewReader(`{"api_key_env":"CUSTOM_KEY"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("PUT", "/v1/admin/bench/models/gemini-2.5-flash", strings.NewReader(`{"api_key_env":"CUSTOM_KEY"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Invite-Secret", "invite-secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if repo.updatedModelID != "gemini-2.5-flash" {
		t.Fatalf("updatedModelID = %q, want gemini-2.5-flash", repo.updatedModelID)
	}
	if repo.updatedCfg.APIKeyEnv != "CUSTOM_KEY" {
		t.Fatalf("api_key_env = %q, want CUSTOM_KEY", repo.updatedCfg.APIKeyEnv)
	}
}

type routerBenchRepo struct {
	updatedModelID string
	updatedCfg     benchsvc.GlobalModelConfig
}

func (r *routerBenchRepo) ListRuns(context.Context, string, bench.RunFilters) ([]bench.RunRecord, int, error) {
	return nil, 0, nil
}
func (r *routerBenchRepo) GetRun(context.Context, string, string) (*bench.RunRecord, error) {
	return nil, nil
}
func (r *routerBenchRepo) InsertRun(context.Context, string, bench.RunRecord) error { return nil }
func (r *routerBenchRepo) InsertRunBatch(context.Context, string, []bench.RunRecord) (int, error) {
	return 0, nil
}
func (r *routerBenchRepo) DeleteRun(context.Context, string, string) error { return nil }
func (r *routerBenchRepo) ArchiveRuns(context.Context, string, benchsvc.ArchiveRequest) (int, error) {
	return 0, nil
}
func (r *routerBenchRepo) FilteredStats(context.Context, string, bench.RunFilters) (*bench.StatsResult, error) {
	return nil, nil
}
func (r *routerBenchRepo) Catalog(context.Context, string) (*bench.RunCatalog, error) {
	return nil, nil
}
func (r *routerBenchRepo) ListEnabledModels(context.Context, string) ([]benchsvc.EnabledModel, error) {
	return nil, nil
}
func (r *routerBenchRepo) UpsertTenantProvider(context.Context, string, string, benchsvc.TenantProviderConfig) error {
	return nil
}
func (r *routerBenchRepo) DeleteTenantProvider(context.Context, string, string) error { return nil }
func (r *routerBenchRepo) UpdateGlobalModel(_ context.Context, modelID string, cfg benchsvc.GlobalModelConfig) error {
	r.updatedModelID = modelID
	r.updatedCfg = cfg
	return nil
}
func (r *routerBenchRepo) Leaderboard(context.Context, string, string, int) ([]bench.LeaderboardEntry, error) {
	return nil, nil
}
func (r *routerBenchRepo) ListScenarios(context.Context) ([]bench.ScenarioSummary, error) {
	return nil, nil
}
func (r *routerBenchRepo) StoreArtifact(context.Context, string, string, string, []byte) error {
	return nil
}
func (r *routerBenchRepo) GetArtifact(context.Context, string, string, string) ([]byte, string, error) {
	return nil, "", nil
}
func (r *routerBenchRepo) CompareModels(context.Context, string, string, string, string) ([]benchsvc.ScenarioModelComparison, error) {
	return nil, nil
}
func (r *routerBenchRepo) ModelMatrix(context.Context, string, []string, []string, string) (*bench.ModelMatrix, error) {
	return nil, nil
}
func (r *routerBenchRepo) SignalSummary(context.Context, string, bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, nil
}
func (r *routerBenchRepo) Regressions(context.Context, string) ([]bench.Regression, error) {
	return nil, nil
}
func (r *routerBenchRepo) FailureAnalysis(context.Context, string, string) (*bench.FailureInsights, error) {
	return nil, nil
}
func (r *routerBenchRepo) UpsertScenarios(context.Context, []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *routerBenchRepo) ResolveModelProvider(context.Context, string) (*benchsvc.ModelProviderInfo, error) {
	return nil, nil
}
func (r *routerBenchRepo) RegisterRunner(context.Context, string, benchsvc.RegisterRunnerRequest) (*benchsvc.Runner, error) {
	return nil, nil
}
func (r *routerBenchRepo) ListRunners(context.Context, string) ([]benchsvc.Runner, error) {
	return nil, nil
}
func (r *routerBenchRepo) DeleteRunner(context.Context, string, string) error { return nil }
func (r *routerBenchRepo) TouchRunner(context.Context, string, string) error  { return nil }
func (r *routerBenchRepo) EnqueueJob(context.Context, string, string, string, benchsvc.JobConfig) (*benchsvc.BenchJob, error) {
	return nil, nil
}
func (r *routerBenchRepo) ClaimJob(context.Context, string, string, []string) (*benchsvc.BenchJob, error) {
	return nil, nil
}
func (r *routerBenchRepo) CompleteJob(context.Context, string, string, string, string, int, int, string) error {
	return nil
}
func (r *routerBenchRepo) FindRunnerForModel(context.Context, string, string) (*benchsvc.Runner, error) {
	return nil, nil
}
func (r *routerBenchRepo) MarkUnhealthyRunners(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (r *routerBenchRepo) ResetStaleJobs(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (r *routerBenchRepo) UpdateJobProgress(_ context.Context, _ string, _, _, _ int) error {
	return nil
}
func (r *routerBenchRepo) BeginTx(context.Context) (pgx.Tx, error) { return nil, nil }
