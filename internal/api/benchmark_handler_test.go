package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"samebits.com/evidra/internal/store"
)

type benchmarkStoreStub struct {
	saveRun func(ctx context.Context, run store.BenchmarkRun, results []store.BenchmarkResult) (string, error)
}

func (s benchmarkStoreStub) SaveRun(ctx context.Context, run store.BenchmarkRun, results []store.BenchmarkResult) (string, error) {
	return s.saveRun(ctx, run, results)
}

func (benchmarkStoreStub) ListRuns(context.Context, string, int, int) ([]store.BenchmarkRun, error) {
	return nil, nil
}

func (benchmarkStoreStub) GetRunWithResults(context.Context, string, string) (store.BenchmarkRun, []store.BenchmarkResult, error) {
	return store.BenchmarkRun{}, nil, nil
}

func TestHandleBenchmarkRun_PreservesMetadata(t *testing.T) {
	t.Parallel()

	var savedRun store.BenchmarkRun
	var savedResults []store.BenchmarkResult
	handler := handleBenchmarkRun(benchmarkStoreStub{
		saveRun: func(_ context.Context, run store.BenchmarkRun, results []store.BenchmarkResult) (string, error) {
			savedRun = run
			savedResults = append([]store.BenchmarkResult(nil), results...)
			return "run-123", nil
		},
	})

	body := bytes.NewBufferString(`{
		"suite":"infra-bench",
		"score":0.9,
		"band":"excellent",
		"metadata":{"contract_version":"v1.0.1","skill_version":"1.0.1"},
		"results":[{"case_id":"kubernetes/fix-deployment","expected_signal":"safe_success","actual_signal":"safe_success","passed":true}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/benchmark/run", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if string(savedRun.Metadata) != `{"contract_version":"v1.0.1","skill_version":"1.0.1"}` {
		t.Fatalf("metadata = %s", savedRun.Metadata)
	}
	if len(savedResults) != 1 || savedResults[0].CaseID != "kubernetes/fix-deployment" {
		t.Fatalf("results = %#v", savedResults)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["run_id"] != "run-123" {
		t.Fatalf("run_id = %q", resp["run_id"])
	}
}
