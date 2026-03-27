# Hosted A2A Execution Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add first-class hosted `execution_mode` support so Evidra-triggered runs can explicitly choose A2A execution end-to-end in the `evidra-kagent-bench` stack.

**Architecture:** Evidra owns the public control-plane field `execution_mode` and threads it through trigger, job persistence, status, and runner claim payloads. Evidra translates `execution_mode=a2a` into bench-cli's internal `config.adapter=a2a` at the direct executor boundary; the actual A2A endpoint remains worker-local configuration in the bench stack. No `evidra-bench` runtime code changes are expected unless implementation reveals a real gap.

**Tech Stack:** Go, React/Vite, OpenAPI YAML, Playwright

**Compatibility Note:** `execution_mode` stays optional in the public API and OpenAPI schema. Omitted requests default to `provider`; do not add it to the request body's `required` array.

---

### Task 1: Add `execution_mode` To Evidra Trigger Models And Validation

**Files:**
- Modify: `/Users/vitas/git/evidra/internal/benchsvc/executor.go`
- Modify: `/Users/vitas/git/evidra/internal/benchsvc/trigger_handler.go`
- Test: `/Users/vitas/git/evidra/internal/benchsvc/handlers_test.go`

**Step 1: Write the failing tests**

Add tests that prove the new public contract behavior. Extend the existing trigger test block in `handlers_test.go` and reuse the current local helpers there (`handlerRepo`, `spyExecutor`, `NewService`, `RegisterRoutes`) rather than creating parallel setup code elsewhere:

```go
func TestHandleTrigger_DefaultsExecutionModeToProvider(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	spy := &spyExecutor{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"smart","scenarios":["s1"]}`
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
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("execution_mode = %q, want provider", stored.ExecutionMode)
	}
}

func TestHandleTrigger_RejectsInvalidExecutionMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","execution_mode":"wat","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
```

Also extend the existing valid trigger tests to assert `spy.job.ExecutionMode`.

**Step 2: Run the tests to verify they fail**

Run:

```bash
go test ./internal/benchsvc -run 'TestHandleTrigger_(DefaultsExecutionModeToProvider|RejectsInvalidExecutionMode|ValidRequest_Returns202|ValidRequest_Returns202_WithEvidenceModeNone)$' -v
```

Expected: FAIL because `TriggerRequest` and `TriggerJob` do not yet have `execution_mode`, and the trigger handler does not default or validate it.

**Step 3: Write the minimal implementation**

Add the new field to the public request/job models:

```go
type TriggerRequest struct {
	Model         string   `json:"model"`
	Provider      string   `json:"provider,omitempty"`
	RunnerID      string   `json:"runner_id,omitempty"`
	EvidenceMode  string   `json:"evidence_mode"`
	ExecutionMode string   `json:"execution_mode,omitempty"`
	Scenarios     []string `json:"scenarios"`
}

type TriggerJob struct {
	ID            string             `json:"id"`
	Status        string             `json:"status"`
	Model         string             `json:"model"`
	Provider      string             `json:"provider,omitempty"`
	EvidenceMode  string             `json:"evidence_mode,omitempty"`
	ExecutionMode string             `json:"execution_mode,omitempty"`
	Total         int                `json:"total"`
	// ...
}
```

Normalize and validate in `decodeTriggerRequest`:

```go
func normalizeTriggerExecutionMode(mode string) (string, bool) {
	switch mode {
	case "", "provider":
		return "provider", true
	case "a2a":
		return "a2a", true
	default:
		return "", false
	}
}
```

In `decodeTriggerRequest`, call that helper immediately after the existing `evidence_mode` validation block and before `return req, true`:

```go
req.ExecutionMode, ok = normalizeTriggerExecutionMode(req.ExecutionMode)
if !ok {
	apiutil.WriteError(w, http.StatusBadRequest, "execution_mode must be provider or a2a")
	return TriggerRequest{}, false
}
```

**Step 4: Re-run the tests**

Run:

```bash
go test ./internal/benchsvc -run 'TestHandleTrigger_(DefaultsExecutionModeToProvider|RejectsInvalidExecutionMode|ValidRequest_Returns202|ValidRequest_Returns202_WithEvidenceModeNone)$' -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add /Users/vitas/git/evidra/internal/benchsvc/executor.go /Users/vitas/git/evidra/internal/benchsvc/trigger_handler.go /Users/vitas/git/evidra/internal/benchsvc/handlers_test.go
git commit -m "feat(benchsvc): add trigger execution mode contract"
```

---

### Task 2: Translate `execution_mode` To Bench Certify Config

**Files:**
- Modify: `/Users/vitas/git/evidra/internal/benchsvc/remote_executor.go`
- Test: `/Users/vitas/git/evidra/internal/benchsvc/executor_test.go`

**Step 1: Write the failing tests**

Add direct executor translation tests:

```go
func TestRemoteExecutor_StartSendsA2AAdapterForA2AExecutionMode(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exec := NewRemoteExecutor(srv.URL)
	job := &TriggerJob{
		ID:            "job-123",
		Model:         "sonnet",
		Provider:      "bifrost",
		EvidenceMode:  "smart",
		ExecutionMode: "a2a",
		Total:         1,
		Progress:      []ScenarioProgress{{Scenario: "s1", Status: "pending"}},
	}

	if err := exec.Start(t.Context(), job, "https://evidra.example", "Bearer token"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cfg := payload["config"].(map[string]any)
	if got := cfg["adapter"]; got != "a2a" {
		t.Fatalf("adapter = %v, want a2a", got)
	}
}

func TestRemoteExecutor_StartOmitsAdapterForProviderExecutionMode(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exec := NewRemoteExecutor(srv.URL)
	job := &TriggerJob{
		ID:            "job-123",
		Model:         "sonnet",
		Provider:      "bifrost",
		EvidenceMode:  "smart",
		ExecutionMode: "provider",
		Total:         1,
		Progress:      []ScenarioProgress{{Scenario: "s1", Status: "pending"}},
	}

	if err := exec.Start(t.Context(), job, "https://evidra.example", "Bearer token"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cfg := payload["config"].(map[string]any)
	if _, ok := cfg["adapter"]; ok {
		t.Fatalf("adapter override should be absent, got %v", cfg["adapter"])
	}
}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
go test ./internal/benchsvc -run 'TestRemoteExecutor_Start(SendsA2AAdapterForA2AExecutionMode|OmitsAdapterForProviderExecutionMode)$' -v
```

Expected: FAIL because `remote_executor.go` does not yet branch on `ExecutionMode`.

**Step 3: Write the minimal implementation**

Centralize the translation in `remote_executor.go` so there is one obvious boundary:

```go
func buildCertifyConfig(job *TriggerJob) map[string]any {
	cfg := map[string]any{
		"timeout_per_scenario": 300,
		"evidence_mode":        job.EvidenceMode,
	}
	if job.ExecutionMode == "a2a" {
		cfg["adapter"] = "a2a"
	}
	return cfg
}
```

Then use:

```go
Config: buildCertifyConfig(job),
```

**Step 4: Re-run the tests**

Run:

```bash
go test ./internal/benchsvc -run 'TestRemoteExecutor_Start(SendsA2AAdapterForA2AExecutionMode|OmitsAdapterForProviderExecutionMode)$' -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add /Users/vitas/git/evidra/internal/benchsvc/remote_executor.go /Users/vitas/git/evidra/internal/benchsvc/executor_test.go
git commit -m "feat(benchsvc): map execution mode to certify adapter"
```

---

### Task 3: Thread `execution_mode` Through Queued Runner Jobs

**Files:**
- Modify: `/Users/vitas/git/evidra/internal/benchsvc/queries.go`
- Modify: `/Users/vitas/git/evidra/internal/benchsvc/trigger_handler.go`
- Modify: `/Users/vitas/git/evidra/internal/benchsvc/runner_handler.go`
- Test: `/Users/vitas/git/evidra/internal/benchsvc/handlers_test.go`
- Test: `/Users/vitas/git/evidra/internal/benchsvc/runner_integration_test.go`

**Fake Repo Note:** Adding `ExecutionMode` to `JobConfig` should not change any method signatures, but still run the package compile/tests early and update any affected Repository fakes if needed. Today the directly affected queue-related fakes are `handlerRepo` in `handlers_test.go` and `fakeRepo` in `service_test.go`; `compareModelsRepo` and `ingestRepo` embed `handlerRepo`.

**Step 1: Write the failing tests**

Add queue/claim tests:

```go
func TestHandleTrigger_WithRunner_QueuesExecutionMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}},
		},
		enqueuedJob: &BenchJob{ID: "job-q-1", Status: "queued", Model: "sonnet", Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","execution_mode":"a2a","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if repo.lastEnqueueCfg.ExecutionMode != "a2a" {
		t.Fatalf("execution_mode = %q, want a2a", repo.lastEnqueueCfg.ExecutionMode)
	}
}

func TestHandlePollJob_ReturnsExecutionMode(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runners: []Runner{{ID: "runner-1", Status: "healthy", Config: RunnerConfig{Models: []string{"sonnet"}}}},
		claimedJob: &BenchJob{
			ID:         "job-q-2",
			TenantID:   "pub",
			Model:      "sonnet",
			Provider:   "bifrost",
			Status:     "queued",
			ConfigJSON: json.RawMessage(`{"scenarios":["s1"],"execution_mode":"a2a","evidence_mode":"smart"}`),
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["execution_mode"] != "a2a" {
		t.Fatalf("execution_mode = %v, want a2a", resp["execution_mode"])
	}
}

func TestHandlePollJob_DefaultsExecutionModeForLegacyJobs(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runners: []Runner{{ID: "runner-1", Status: "healthy", Config: RunnerConfig{Models: []string{"sonnet"}}}},
		claimedJob: &BenchJob{
			ID:         "job-legacy",
			TenantID:   "pub",
			Model:      "sonnet",
			Provider:   "bifrost",
			Status:     "queued",
			ConfigJSON: json.RawMessage(`{"scenarios":["s1"],"evidence_mode":"smart"}`),
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["execution_mode"] != "provider" {
		t.Fatalf("execution_mode = %v, want provider", resp["execution_mode"])
	}
}

func TestPgStore_EnqueueAndClaimJob_PreservesExecutionMode(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	_, err := store.EnqueueJob(context.Background(), tenantID, "deepseek-chat", "bifrost", JobConfig{
		Scenarios:     []string{"broken-deployment"},
		EvidenceMode:  "smart",
		ExecutionMode: "a2a",
	})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	claimed, err := store.ClaimJob(context.Background(), tenantID, "runner-1", []string{"deepseek-chat"})
	if err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}

	var cfg JobConfig
	if err := json.Unmarshal(claimed.ConfigJSON, &cfg); err != nil {
		t.Fatalf("unmarshal config_json: %v", err)
	}
	if cfg.ExecutionMode != "a2a" {
		t.Fatalf("execution_mode = %q, want a2a", cfg.ExecutionMode)
	}
}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
go test ./internal/benchsvc -run 'TestHandleTrigger_WithRunner_QueuesExecutionMode|TestHandlePollJob_ReturnsExecutionMode|TestHandlePollJob_DefaultsExecutionModeForLegacyJobs|TestPgStore_EnqueueAndClaimJob_PreservesExecutionMode' -v
```

Expected: FAIL because `JobConfig` and runner responses do not yet carry `execution_mode`.

**Step 3: Write the minimal implementation**

Add the persisted field:

```go
type JobConfig struct {
	Scenarios     []string `json:"scenarios"`
	Timeout       int      `json:"timeout,omitempty"`
	RunnerID      string   `json:"runner_id,omitempty"`
	EvidenceMode  string   `json:"evidence_mode,omitempty"`
	ExecutionMode string   `json:"execution_mode,omitempty"`
}
```

Thread it through:

```go
cfg := JobConfig{
	Scenarios:     req.Scenarios,
	RunnerID:      req.RunnerID,
	EvidenceMode:  req.EvidenceMode,
	ExecutionMode: req.ExecutionMode,
}
```

Default legacy claimed jobs in `runner_handler.go`:

```go
if cfg.ExecutionMode == "" {
	cfg.ExecutionMode = "provider"
}
```

Return it from `GET /v1/runners/jobs`:

```go
"execution_mode": cfg.ExecutionMode,
```

**Step 4: Re-run the tests**

Run:

```bash
go test ./internal/benchsvc -run 'TestHandleTrigger_WithRunner_QueuesExecutionMode|TestHandlePollJob_ReturnsExecutionMode|TestHandlePollJob_DefaultsExecutionModeForLegacyJobs|TestPgStore_EnqueueAndClaimJob_PreservesExecutionMode' -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add /Users/vitas/git/evidra/internal/benchsvc/queries.go /Users/vitas/git/evidra/internal/benchsvc/trigger_handler.go /Users/vitas/git/evidra/internal/benchsvc/runner_handler.go /Users/vitas/git/evidra/internal/benchsvc/handlers_test.go /Users/vitas/git/evidra/internal/benchsvc/runner_integration_test.go
git commit -m "feat(benchsvc): persist execution mode for runner jobs"
```

---

### Task 4: Update Evidra OpenAPI, Docs Tests, And Trigger UI

**Files:**
- Modify: `/Users/vitas/git/evidra/cmd/evidra-api/static/openapi.yaml`
- Modify: `/Users/vitas/git/evidra/ui/public/openapi.yaml`
- Modify: `/Users/vitas/git/evidra/internal/api/openapi_bench_docs_test.go`
- Modify: `/Users/vitas/git/evidra/internal/api/api_reference_docs_test.go`

**Step 1: Write the failing docs tests**

Extend the OpenAPI/docs assertions:

```go
func TestOpenAPIBenchRoutesDocumentSupportedSurface(t *testing.T) {
	t.Parallel()

	spec := loadOpenAPISpec(t)

	assertRequestBodyRequiredFields(t, spec, "/v1/bench/trigger", "post", []string{"model", "scenarios", "evidence_mode"})
	assertRequestBodyPropertyEnumValues(t, spec, "/v1/bench/trigger", "post", "execution_mode", []string{"provider", "a2a"})
	assertResponseSchemaHasProperty(t, spec, "/v1/runners/jobs", "get", "200", "execution_mode")
}
```

and:

```go
func TestMarkdownAPIReference_CoversLiveExternalIngestSurface(t *testing.T) {
	t.Parallel()

	doc := loadMarkdownAPIReference(t)
	requiredSnippets := []string{
		"`execution_mode`",
		"claimed job payload includes `evidence_mode` and `execution_mode`",
	}
	// ...
}
```

**Step 2: Run the docs tests to verify they fail**

Run:

```bash
go test ./internal/api -run 'Test(OpenAPIBenchRoutesDocumentSupportedSurface|MarkdownAPIReference_CoversLiveExternalIngestSurface)' -v
```

Expected: FAIL because the specs and markdown docs do not yet mention `execution_mode`.

**Step 3: Update OpenAPI and markdown/docs**

Add the new property to `cmd/evidra-api/static/openapi.yaml`:

```yaml
execution_mode:
  type: string
  description: Optional execution path. `provider` uses bench-cli's built-in tool-use loop; `a2a` delegates to the configured remote A2A agent.
  enum: [provider, a2a]
```

Keep the `required` array unchanged at `[model, scenarios, evidence_mode]`.

Then copy the updated spec:

```bash
cp /Users/vitas/git/evidra/cmd/evidra-api/static/openapi.yaml /Users/vitas/git/evidra/ui/public/openapi.yaml
```

Validate the YAML:

```bash
python3 -c "import yaml; yaml.safe_load(open('/Users/vitas/git/evidra/cmd/evidra-api/static/openapi.yaml'))"
```

**Step 4: Re-run the docs tests**

Run:

```bash
go test ./internal/api -run 'Test(OpenAPIBenchRoutesDocumentSupportedSurface|MarkdownAPIReference_CoversLiveExternalIngestSurface)' -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add /Users/vitas/git/evidra/cmd/evidra-api/static/openapi.yaml /Users/vitas/git/evidra/ui/public/openapi.yaml /Users/vitas/git/evidra/internal/api/openapi_bench_docs_test.go /Users/vitas/git/evidra/internal/api/api_reference_docs_test.go
git commit -m "docs(api): add execution mode to hosted bench spec"
```

---

### Task 5: Update The Trigger UI

**Files:**
- Modify: `/Users/vitas/git/evidra/ui/src/pages/bench/BenchDashboard.tsx`

**Step 1: Add execution mode state and request wiring**

Add a new UI state and request field in `BenchDashboard.tsx`:

```tsx
type TriggerExecutionMode = "provider" | "a2a";

const [triggerExecutionMode, setTriggerExecutionMode] =
  useState<TriggerExecutionMode>("provider");
```

Send it with the trigger request:

```tsx
body: JSON.stringify({
  model: triggerModel,
  provider: triggerProvider,
  execution_mode: triggerExecutionMode,
  evidence_mode: triggerEvidenceMode,
  scenarios,
}),
```

Add a fieldset with `Provider` and `A2A` options next to the existing evidence-mode control.

**Step 2: Build the UI**

Run:

```bash
cd /Users/vitas/git/evidra/ui && npm run build
```

Expected: PASS

**Step 3: Commit**

```bash
git add /Users/vitas/git/evidra/ui/src/pages/bench/BenchDashboard.tsx
git commit -m "feat(ui): add hosted execution mode selection"
```

---

### Task 6: Update Markdown Docs, Architecture Docs, And Changelog

**Files:**
- Modify: `/Users/vitas/git/evidra/docs/api-reference.md`
- Modify: `/Users/vitas/git/evidra/docs/ARCHITECTURE.md`
- Modify: `/Users/vitas/git/evidra/docs/contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md`
- Modify: `/Users/vitas/git/evidra/CHANGELOG.md`

**Step 1: Update the markdown API and architecture docs**

Update trigger and runner examples so they show the public hosted field:

```json
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "execution_mode": "a2a",
  "evidence_mode": "smart",
  "runner_id": "01K...",
  "scenarios": ["broken-deployment"]
}
```

Update the runner claim docs to show:

```json
{
  "job_id": "01K...",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "execution_mode": "a2a",
  "evidence_mode": "smart",
  "scenarios": ["broken-deployment"],
  "timeout": 300
}
```

Add a short note that `execution_mode` is optional and defaults to `provider`.

**Step 2: Update `CHANGELOG.md`**

Add an entry under `## Unreleased`:

```md
### Bench Trigger
- Added hosted `execution_mode` support for bench trigger jobs and runner claim payloads, including A2A execution selection for bench-cli backed runs.
```

**Step 3: Commit**

```bash
git add /Users/vitas/git/evidra/docs/api-reference.md /Users/vitas/git/evidra/docs/ARCHITECTURE.md /Users/vitas/git/evidra/docs/contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md /Users/vitas/git/evidra/CHANGELOG.md
git commit -m "docs: document hosted execution mode"
```

---

### Task 7: Update `evidra-kagent-bench` E2E Coverage And Demo Docs

**Mode:** manual cross-repo integration task after local `evidra` changes are built into images

**Files:**
- Modify: `/Users/vitas/git/evidra-kagent-bench/tests/e2e/full.spec.ts`
- Modify: `/Users/vitas/git/evidra-kagent-bench/README.md`
- Modify: `/Users/vitas/git/evidra-kagent-bench/docs/guides/demo-compose.md`

**Step 1: Update the full E2E test to assert A2A behavior**

Change the trigger request so it uses the new public contract:

```ts
const triggerRes = await apiRequest("/v1/bench/trigger", {
  method: "POST",
  body: JSON.stringify({
    model: process.env.KAGENT_MODEL || "deepseek-chat",
    execution_mode: "a2a",
    evidence_mode: "smart",
    scenarios: ["broken-deployment"],
  }),
});
```

After polling the job to completion, assert the resulting run is A2A-backed:

```ts
expect(job.run_ids?.length).toBeGreaterThan(0);
const runId = job.run_ids[0];

const runRes = await apiRequest(`/v1/bench/runs/${runId}`);
expect(runRes.status).toBe(200);
const run = await runRes.json();
expect(run.adapter).toBe("a2a");
expect(run.evidence_mode).toBe("smart");
```

**Step 2: Build local images after Tasks 1-6 are complete**

Build the updated images locally:

```bash
docker build -t evidra-api:a2a-dev -f /Users/vitas/git/evidra/Dockerfile.api /Users/vitas/git/evidra
docker build -t bench-cli:a2a-dev -f /Users/vitas/git/evidra-bench/Dockerfile.bench /Users/vitas/git/evidra-bench
```

Restart the relevant services with local image overrides:

```bash
cd /Users/vitas/git/evidra-kagent-bench
EVIDRA_API_IMAGE=evidra-api:a2a-dev BENCH_IMAGE=bench-cli:a2a-dev docker compose up -d --force-recreate evidra-api bench-cli
```

Run:

```bash
cd /Users/vitas/git/evidra-kagent-bench/tests/e2e
npm run test:full
```

Expected: PASS, with the completed run recorded as `adapter: "a2a"`.

**Step 3: Update the demo docs**

Update README and `docs/guides/demo-compose.md` to say:

- the dashboard/API now expose `execution_mode`
- choose `A2A` when running against kagent
- the actual A2A endpoint remains stack config (`INFRA_BENCH_A2A_AGENT_URL`)

**Step 4: Commit**

```bash
git -C /Users/vitas/git/evidra-kagent-bench add /Users/vitas/git/evidra-kagent-bench/tests/e2e/full.spec.ts /Users/vitas/git/evidra-kagent-bench/README.md /Users/vitas/git/evidra-kagent-bench/docs/guides/demo-compose.md
git -C /Users/vitas/git/evidra-kagent-bench commit -m "test: verify hosted a2a trigger flow"
```

---

### Task 8: Run Cross-Repo Verification And Capture Any Unexpected Bench Gap

**Files:**
- Verify: `/Users/vitas/git/evidra/internal/benchsvc/...`
- Verify: `/Users/vitas/git/evidra/internal/api/...`
- Verify: `/Users/vitas/git/evidra/ui/...`
- Verify: `/Users/vitas/git/evidra-kagent-bench/tests/e2e/full.spec.ts`
- Inspect only if needed: `/Users/vitas/git/evidra-bench/cmd/bench-cli/serve.go`
- Inspect only if needed: `/Users/vitas/git/evidra-bench/cmd/bench-cli/serve_test.go`

**Step 1: Run Evidra package tests**

Run:

```bash
cd /Users/vitas/git/evidra
go test ./internal/benchsvc -v
go test ./internal/api -v
```

Expected: PASS

**Step 2: Run Evidra frontend build**

Run:

```bash
cd /Users/vitas/git/evidra/ui
npm run build
```

Expected: PASS

**Step 3: Run Evidra lint**

Run:

```bash
cd /Users/vitas/git/evidra
make lint
```

Expected: PASS

**Step 4: Run the `evidra-kagent-bench` full E2E again as the final proof**

Run:

```bash
cd /Users/vitas/git/evidra-kagent-bench/tests/e2e
npm run test:full
```

Expected: PASS

**Step 5: Only if the E2E test reveals a real bench-side gap, patch `evidra-bench` minimally**

The current expectation is that no runtime change is required in `/Users/vitas/git/evidra-bench` because `bench-cli` already supports `config.adapter=a2a`. If the full E2E run shows a real gap, constrain the fix to the smallest possible direct-executor translation or reporting issue, then run:

```bash
cd /Users/vitas/git/evidra-bench
go test ./cmd/bench-cli/... -v
make lint
```

**Step 6: Commit the final Evidra verification if any cleanup was required**

```bash
cd /Users/vitas/git/evidra
git status --short
```

Expected: clean working tree in `evidra`; any additional repo commits should already be created in their own repositories.
