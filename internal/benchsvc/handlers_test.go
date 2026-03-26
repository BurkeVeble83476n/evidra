package benchsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"samebits.com/evidra/internal/auth"
	bench "samebits.com/evidra/pkg/bench"
)

// handlerRepo is an in-memory fake implementing Repository for handler tests.
// Each field holds canned return values; tests set them before making requests.
type handlerRepo struct {
	runs       []bench.RunRecord
	runsTotal  int
	runsErr    error
	run        *bench.RunRecord
	runErr     error
	stats      *bench.StatsResult
	statsErr   error
	catalog    *bench.RunCatalog
	catalogErr error
	enabledModels    []EnabledModel
	enabledModelsErr error
	leaders    []bench.LeaderboardEntry
	leadersErr error
	scenarios  []bench.ScenarioSummary
	scenErr    error
	artifact   []byte
	artCT      string
	artErr     error

	// delete / archive
	deleteErr    error
	archiveCount int
	archiveErr   error

	// analytics
	signals     *bench.SignalAggregation
	signalsErr  error
	regressions []bench.Regression
	regressErr  error
	insights    *bench.FailureInsights
	insightsErr error
	matrix      *bench.ModelMatrix
	matrixErr   error

	// capture
	lastTenant string
	lastFilter bench.RunFilters
	lastMode   string
}

func (r *handlerRepo) ListRuns(_ context.Context, tenant string, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	r.lastTenant = tenant
	r.lastFilter = f
	return r.runs, r.runsTotal, r.runsErr
}
func (r *handlerRepo) GetRun(_ context.Context, tenant, id string) (*bench.RunRecord, error) {
	r.lastTenant = tenant
	return r.run, r.runErr
}
func (r *handlerRepo) InsertRun(_ context.Context, _ string, _ bench.RunRecord) error { return nil }
func (r *handlerRepo) DeleteRun(_ context.Context, tenant, id string) error {
	r.lastTenant = tenant
	return r.deleteErr
}
func (r *handlerRepo) ArchiveRuns(_ context.Context, tenant string, _ ArchiveRequest) (int, error) {
	r.lastTenant = tenant
	return r.archiveCount, r.archiveErr
}
func (r *handlerRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return 0, nil
}
func (r *handlerRepo) FilteredStats(_ context.Context, tenant string, f bench.RunFilters) (*bench.StatsResult, error) {
	r.lastTenant = tenant
	r.lastFilter = f
	return r.stats, r.statsErr
}
func (r *handlerRepo) Catalog(_ context.Context, tenant string) (*bench.RunCatalog, error) {
	r.lastTenant = tenant
	return r.catalog, r.catalogErr
}
func (r *handlerRepo) ListEnabledModels(_ context.Context, tenant string) ([]EnabledModel, error) {
	r.lastTenant = tenant
	return r.enabledModels, r.enabledModelsErr
}
func (r *handlerRepo) UpsertTenantProvider(_ context.Context, tenantID, modelID string, _ TenantProviderConfig) error {
	r.lastTenant = tenantID
	return nil
}
func (r *handlerRepo) DeleteTenantProvider(_ context.Context, tenantID, modelID string) error {
	r.lastTenant = tenantID
	return nil
}
func (r *handlerRepo) UpdateGlobalModel(_ context.Context, _ string, _ GlobalModelConfig) error {
	return nil
}
func (r *handlerRepo) Leaderboard(_ context.Context, tenant, mode string) ([]bench.LeaderboardEntry, error) {
	r.lastTenant = tenant
	r.lastMode = mode
	return r.leaders, r.leadersErr
}
func (r *handlerRepo) ListScenarios(_ context.Context) ([]bench.ScenarioSummary, error) {
	return r.scenarios, r.scenErr
}
func (r *handlerRepo) StoreArtifact(_ context.Context, _, _, _ string, _ []byte) error { return nil }
func (r *handlerRepo) GetArtifact(_ context.Context, tenant, runID, artType string) ([]byte, string, error) {
	r.lastTenant = tenant
	return r.artifact, r.artCT, r.artErr
}
func (r *handlerRepo) CompareModels(_ context.Context, _, _, _, _ string) ([]ScenarioModelComparison, error) {
	return nil, nil
}
func (r *handlerRepo) ModelMatrix(_ context.Context, _ string, _, _ []string) (*bench.ModelMatrix, error) {
	return r.matrix, r.matrixErr
}
func (r *handlerRepo) SignalSummary(_ context.Context, tenant string, f bench.RunFilters) (*bench.SignalAggregation, error) {
	r.lastTenant = tenant
	r.lastFilter = f
	return r.signals, r.signalsErr
}
func (r *handlerRepo) Regressions(_ context.Context, tenant string) ([]bench.Regression, error) {
	r.lastTenant = tenant
	return r.regressions, r.regressErr
}
func (r *handlerRepo) FailureAnalysis(_ context.Context, tenant string, _ string) (*bench.FailureInsights, error) {
	r.lastTenant = tenant
	return r.insights, r.insightsErr
}
func (r *handlerRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *handlerRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	return nil, fmt.Errorf("handlerRepo: no real tx")
}

// passthroughAuth sets the given tenant on the request context without checking tokens.
func passthroughAuth(tenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// setupMux creates a mux with RegisterRoutes using the given repo and config.
func setupMux(repo *handlerRepo, cfg ServiceConfig, tenantID string) *http.ServeMux {
	svc := NewService(repo, cfg)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth(tenantID))
	return mux
}

// ---------- Leaderboard ----------

func TestHandleLeaderboard_ReturnsModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		leaders: []bench.LeaderboardEntry{
			{Model: "sonnet", Runs: 10, PassRate: 0.9},
			{Model: "opus", Runs: 5, PassRate: 1.0},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["models"]; !ok {
		t.Fatal("response missing 'models' key")
	}
	var models []bench.LeaderboardEntry
	if err := json.Unmarshal(body["models"], &models); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}

func TestHandleLeaderboard_DefaultsToProxy(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.lastMode != "" {
		t.Fatalf("evidence_mode = %q, want empty (all)", repo.lastMode)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["evidence_mode"] != "" {
		t.Fatalf("response evidence_mode = %q, want empty", body["evidence_mode"])
	}
}

func TestHandleLeaderboard_503WhenNoPublicTenant(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: ""}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// ---------- List Runs ----------

func TestHandleListRuns_ReturnsItems(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "s1", Model: "sonnet"},
			{ID: "r2", ScenarioID: "s2", Model: "opus"},
		},
		runsTotal: 2,
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Items  []bench.RunRecord `json:"items"`
		Total  int               `json:"total"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(body.Items))
	}
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}
	if body.Limit != 50 {
		t.Fatalf("limit = %d, want 50 (default)", body.Limit)
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}
}

func TestHandleListRuns_ParsesFilters(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{runsTotal: 0}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs?model=sonnet&scenario=broken-deployment&evidence_mode=direct&limit=10&offset=5", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	f := repo.lastFilter
	if f.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", f.Model)
	}
	if f.ScenarioID != "broken-deployment" {
		t.Errorf("ScenarioID = %q, want broken-deployment", f.ScenarioID)
	}
	if f.EvidenceMode != "direct" {
		t.Errorf("EvidenceMode = %q, want direct", f.EvidenceMode)
	}
	if f.Limit != 10 {
		t.Errorf("Limit = %d, want 10", f.Limit)
	}
	if f.Offset != 5 {
		t.Errorf("Offset = %d, want 5", f.Offset)
	}
}

// ---------- Get Run ----------

func TestHandleGetRun_ReturnsRecord(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "run-42", ScenarioID: "s1", Model: "sonnet", Passed: true},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/run-42", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var run bench.RunRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if run.ID != "run-42" {
		t.Fatalf("ID = %q, want run-42", run.ID)
	}
}

func TestHandleGetRun_404ForMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{runErr: pgx.ErrNoRows}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/nonexistent", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------- Ingest ----------

func TestHandleIngestRun_ValidPayload(t *testing.T) {
	t.Parallel()

	// IngestRun calls BeginTx which our handlerRepo doesn't support,
	// so we use a dedicated repo that returns a fakeTx.
	repo := &ingestRepo{}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"id":"r1","scenario_id":"s1","model":"sonnet"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

func TestHandleIngestRun_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	tests := []struct {
		name    string
		payload string
	}{
		{"missing id", `{"scenario_id":"s1","model":"m1"}`},
		{"missing scenario_id", `{"id":"r1","model":"m1"}`},
		{"missing model", `{"id":"r1","scenario_id":"s1"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHandleIngestBatch_ImportsRuns(t *testing.T) {
	t.Parallel()

	repo := &ingestRepo{batchCount: 3}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"runs":[
		{"id":"r1","scenario_id":"s1","model":"m1"},
		{"id":"r2","scenario_id":"s1","model":"m1"},
		{"id":"r3","scenario_id":"s2","model":"m2"}
	]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/batch", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	if int(body["imported"].(float64)) != 3 {
		t.Fatalf("imported = %v, want 3", body["imported"])
	}
}

// ---------- Artifacts ----------

func TestHandleGetTranscript_ReturnsText(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		artifact: []byte("step 1\nstep 2\nstep 3"),
		artCT:    "text/plain",
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/transcript", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
	if rec.Body.String() != "step 1\nstep 2\nstep 3" {
		t.Fatalf("body = %q, want transcript text", rec.Body.String())
	}
}

func TestHandleGetTranscript_404WhenMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{artErr: pgx.ErrNoRows}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/transcript", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleGetTimeline_ComputesPhases(t *testing.T) {
	t.Parallel()

	toolCalls := []bench.ToolCall{
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl get pods -n default"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl describe pod/web -n default"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl apply -f fix.yaml -n default"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl get pods -n default"}`)},
	}
	data, _ := json.Marshal(toolCalls)

	repo := &handlerRepo{
		artifact: data,
		artCT:    "application/json",
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/timeline", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var tl bench.Timeline
	if err := json.Unmarshal(rec.Body.Bytes(), &tl); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}
	if tl.TotalSteps != 4 {
		t.Fatalf("TotalSteps = %d, want 4", tl.TotalSteps)
	}
	if tl.MutationCount != 1 {
		t.Fatalf("MutationCount = %d, want 1", tl.MutationCount)
	}
	// First call is discover, second is diagnose, third is act, fourth is verify.
	wantPhases := []bench.Phase{bench.PhaseDiscover, bench.PhaseDiagnose, bench.PhaseAct, bench.PhaseVerify}
	for i, want := range wantPhases {
		if tl.Steps[i].Phase != want {
			t.Errorf("step[%d].Phase = %q, want %q", i, tl.Steps[i].Phase, want)
		}
	}
}

func TestHandleGetTimeline_404WhenNoToolCalls(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{artErr: pgx.ErrNoRows}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/timeline", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------- Stats / Catalog / Scenarios ----------

func TestHandleStats_ReturnsAggregates(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		stats: &bench.StatsResult{
			TotalRuns: 42,
			PassCount: 38,
			FailCount: 4,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/stats", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body bench.StatsResult
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalRuns != 42 {
		t.Fatalf("TotalRuns = %d, want 42", body.TotalRuns)
	}
}

func TestHandleCatalog_ReturnsModelsAndProviders(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		catalog: &bench.RunCatalog{
			Models:    []string{"sonnet", "opus"},
			Providers: []string{"anthropic", "bifrost"},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/catalog", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body bench.RunCatalog
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 2 {
		t.Fatalf("len(Models) = %d, want 2", len(body.Models))
	}
	if len(body.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(body.Providers))
	}
}

func TestHandleListScenarios_ReturnsArray(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "broken-deployment", Title: "Broken Deployment", Category: "kubectl"},
			{ID: "helm-rollback", Title: "Helm Rollback", Category: "helm"},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/scenarios", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Scenarios []bench.ScenarioSummary `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Scenarios) != 2 {
		t.Fatalf("len(scenarios) = %d, want 2", len(body.Scenarios))
	}
}

// ---------- Ingest support: fake that supports transactions ----------

// ingestRepo wraps handlerRepo with a fake transaction that accepts Exec and Commit.
type ingestRepo struct {
	handlerRepo
	batchCount int
}

func (r *ingestRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *ingestRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	// Reuse fakeTx from service_batch_test.go (same package).
	// Supply enough "INSERT 0 1" tags so IngestRunBatch counts rows as inserted.
	tags := make([]pgconn.CommandTag, 20)
	for i := range tags {
		tags[i] = pgconn.NewCommandTag("INSERT 0 1")
	}
	return &fakeTx{execTags: tags}, nil
}

func (r *ingestRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return r.batchCount, nil
}

// ---------- Delete ----------

func TestHandleDeleteRun_Returns204(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/runs/run-42", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}
}

func TestHandleDeleteRun_404ForMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{deleteErr: pgx.ErrNoRows}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/runs/nonexistent", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------- Archive ----------

func TestHandleArchiveRuns_ReturnsCount(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{archiveCount: 5}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{"model":"sonnet"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(body["archived"].(float64)) != 5 {
		t.Fatalf("archived = %v, want 5", body["archived"])
	}
}

func TestHandleArchiveRuns_RejectsEmptyFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleArchiveRuns_AcceptsBeforeFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{archiveCount: 10}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{"before":"2026-03-21T00:00:00Z"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandleArchiveRuns_AcceptsIDsFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{archiveCount: 2}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{"ids":["run-1","run-2"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// ---------- Compare ----------

func TestHandleCompareRuns_ReturnsDelta(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:               "run-1",
			ScenarioID:       "s1",
			Model:            "sonnet",
			Passed:           true,
			Duration:         30.0,
			Turns:            5,
			EstimatedCost:    0.10,
			PromptTokens:     1000,
			CompletionTokens: 500,
			ChecksPassed:     3,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/runs?a=run-1&b=run-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var cmp RunComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &cmp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cmp.RunA.ID != "run-1" {
		t.Fatalf("RunA.ID = %q, want run-1", cmp.RunA.ID)
	}
	if cmp.Delta.PassedChanged {
		t.Fatal("PassedChanged = true, want false (same run)")
	}
	if cmp.Delta.DurationDiff != 0 {
		t.Fatalf("DurationDiff = %f, want 0", cmp.Delta.DurationDiff)
	}
}

func TestHandleCompareModels_ReturnsComparison(t *testing.T) {
	t.Parallel()

	repo := &compareModelsRepo{
		scenarios: []ScenarioModelComparison{
			{ScenarioID: "broken-deployment", APassRate: 100, BPassRate: 80, ACost: 0.10, BCost: 0.20},
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?a=sonnet&b=opus", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var cmp ModelComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &cmp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cmp.ModelA != "sonnet" {
		t.Fatalf("ModelA = %q, want sonnet", cmp.ModelA)
	}
	if cmp.ModelB != "opus" {
		t.Fatalf("ModelB = %q, want opus", cmp.ModelB)
	}
	if len(cmp.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(cmp.Scenarios))
	}
	if cmp.Summary.SharedScenarios != 1 {
		t.Fatalf("SharedScenarios = %d, want 1", cmp.Summary.SharedScenarios)
	}
}

// compareModelsRepo is a fake that returns canned CompareModels data.
type compareModelsRepo struct {
	handlerRepo
	scenarios []ScenarioModelComparison
}

func (r *compareModelsRepo) CompareModels(_ context.Context, _, _, _, _ string) ([]ScenarioModelComparison, error) {
	return r.scenarios, nil
}
func (r *compareModelsRepo) ModelMatrix(_ context.Context, _ string, _, _ []string) (*bench.ModelMatrix, error) {
	return nil, nil
}
func (r *compareModelsRepo) SignalSummary(_ context.Context, _ string, _ bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, nil
}
func (r *compareModelsRepo) Regressions(_ context.Context, _ string) ([]bench.Regression, error) {
	return nil, nil
}
func (r *compareModelsRepo) FailureAnalysis(_ context.Context, _ string, _ string) (*bench.FailureInsights, error) {
	return nil, nil
}
func (r *compareModelsRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}

// ---------- Signals ----------

func TestHandleSignals_ReturnsAggregation(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		signals: &bench.SignalAggregation{
			TotalRuns:         10,
			RunsWithScorecard: 8,
			AvgScore:          75.5,
			Signals: map[string]bench.SignalCount{
				"artifact_drift": {Total: 5, RunCount: 3},
			},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/signals?model=sonnet", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body bench.SignalAggregation
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalRuns != 10 {
		t.Fatalf("TotalRuns = %d, want 10", body.TotalRuns)
	}
	if body.RunsWithScorecard != 8 {
		t.Fatalf("RunsWithScorecard = %d, want 8", body.RunsWithScorecard)
	}
	if repo.lastFilter.Model != "sonnet" {
		t.Fatalf("filter.Model = %q, want sonnet", repo.lastFilter.Model)
	}
}

// ---------- Regressions ----------

func TestHandleRegressions_ReturnsArray(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		regressions: []bench.Regression{
			{ScenarioID: "broken-deployment", Model: "sonnet", LatestRunID: "r1", Severity: "critical", PrevRate: 90},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/regressions", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []bench.Regression
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("len(regressions) = %d, want 1", len(body))
	}
	if body[0].Severity != "critical" {
		t.Fatalf("severity = %q, want critical", body[0].Severity)
	}
}

func TestHandleRegressions_EmptyReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/regressions", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "[]" && rec.Body.String() != "[]\n" {
		t.Fatalf("body = %q, want empty array", rec.Body.String())
	}
}

// ---------- Failure Analysis ----------

func TestHandleFailureAnalysis_ReturnsInsights(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		insights: &bench.FailureInsights{
			ScenarioID: "broken-deployment",
			TotalRuns:  20,
			FailedRuns: 8,
			PassedRuns: 12,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/insights?scenario=broken-deployment", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body bench.FailureInsights
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ScenarioID != "broken-deployment" {
		t.Fatalf("ScenarioID = %q, want broken-deployment", body.ScenarioID)
	}
	if body.TotalRuns != 20 {
		t.Fatalf("TotalRuns = %d, want 20", body.TotalRuns)
	}
}

func TestHandleFailureAnalysis_RequiresScenario(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/insights", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// ---------- Compare Models (multi-model) ----------

func TestHandleCompareModels_AcceptsModelsParam(t *testing.T) {
	t.Parallel()

	matrixRepo := &matrixRepo{
		matrix: &bench.ModelMatrix{
			Models:    []string{"opus", "sonnet"},
			Scenarios: []string{"broken-deployment"},
			Cells: map[string]map[string]bench.ModelMatrixCell{
				"broken-deployment": {
					"sonnet": {Runs: 5, Passed: 4, PassRate: 80},
					"opus":   {Runs: 3, Passed: 3, PassRate: 100},
				},
			},
		},
	}
	svc := NewService(matrixRepo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?models=sonnet,opus&scenarios=broken-deployment", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body bench.ModelMatrix
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 2 {
		t.Fatalf("len(Models) = %d, want 2", len(body.Models))
	}
	if len(body.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(body.Scenarios))
	}
}

func TestHandleCompareModels_RejectsNoParams(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// matrixRepo is a fake that returns canned ModelMatrix data.
type matrixRepo struct {
	handlerRepo
	matrix *bench.ModelMatrix
}

func (r *matrixRepo) ModelMatrix(_ context.Context, _ string, _, _ []string) (*bench.ModelMatrix, error) {
	return r.matrix, nil
}
func (r *matrixRepo) CompareModels(_ context.Context, _, _, _, _ string) ([]ScenarioModelComparison, error) {
	return nil, nil
}
func (r *matrixRepo) SignalSummary(_ context.Context, _ string, _ bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, nil
}
func (r *matrixRepo) Regressions(_ context.Context, _ string) ([]bench.Regression, error) {
	return nil, nil
}
func (r *matrixRepo) FailureAnalysis(_ context.Context, _ string, _ string) (*bench.FailureInsights, error) {
	return nil, nil
}
func (r *matrixRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}

// ---------- Trigger ----------

// spyExecutor records whether Start was called.
type spyExecutor struct {
	started bool
}

func (e *spyExecutor) Start(_ context.Context, _ *TriggerJob, _, _ string) error {
	e.started = true
	return nil
}

func TestHandleTrigger_NoExecutor_Returns501(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     nil, // no executor
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
}

func TestHandleTrigger_ValidRequest_Returns202(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	spy := &spyExecutor{}
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["id"]; !ok {
		t.Fatal("response missing 'id' key")
	}
}

func TestHandleTriggerProgress_UnknownJob_Returns404(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"scenario":"s1","status":"passed","completed":1,"total":1}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/nonexistent/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleTriggerProgress_InvalidVersion_Returns400(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	// Create a job first so the 404 check passes.
	job := &TriggerJob{
		ID:     "job-1",
		Status: "running",
		Total:  1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "running"},
		},
	}
	store.Create(job)

	rec := httptest.NewRecorder()
	body := `{"contract_version":"v2.0.0","scenario":"s1","status":"passed","completed":1,"total":1}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/job-1/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
