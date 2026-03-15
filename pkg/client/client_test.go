package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_DefaultTimeout(t *testing.T) {
	t.Parallel()
	c := New(Config{URL: "http://localhost", APIKey: "key"})
	if c.URL() != "http://localhost" {
		t.Fatalf("expected URL=http://localhost, got %s", c.URL())
	}
}

func TestForward_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing auth header")
		}
		if r.URL.Path != "/v1/evidence/forward" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"receipt_id": "r1",
			"status":     "accepted",
		})
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, APIKey: "test-key"})
	resp, err := c.Forward(context.Background(), json.RawMessage(`{"type":"prescribe"}`))
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if resp.ReceiptID != "r1" {
		t.Fatalf("expected receipt_id=r1, got %s", resp.ReceiptID)
	}
}

func TestForward_Unauthorized(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, APIKey: "bad"})
	_, err := c.Forward(context.Background(), json.RawMessage(`{}`))
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestPing_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, APIKey: "key"})
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestIsReachabilityError(t *testing.T) {
	t.Parallel()
	if !IsReachabilityError(ErrUnreachable) {
		t.Error("ErrUnreachable should be reachability error")
	}
	if !IsReachabilityError(ErrServerError) {
		t.Error("ErrServerError should be reachability error")
	}
	if IsReachabilityError(ErrUnauthorized) {
		t.Error("ErrUnauthorized should NOT be reachability error")
	}
}

func TestSubmitBenchmarkRun_Success(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/benchmark/run" {
			t.Fatalf("path = %q, want /v1/benchmark/run", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}

		var req BenchmarkRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Suite != "infra-bench" {
			t.Fatalf("suite = %q", req.Suite)
		}
		if string(req.Metadata) != `{"contract_version":"v1.0.1"}` {
			t.Fatalf("metadata = %s", req.Metadata)
		}
		if len(req.Results) != 1 || req.Results[0].CaseID != "kubernetes/fix-deployment" {
			t.Fatalf("results = %#v", req.Results)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(BenchmarkRunResponse{
			RunID:  "run-123",
			Status: "accepted",
		})
	}))
	defer ts.Close()

	score := 0.91
	c := New(Config{URL: ts.URL, APIKey: "test-key"})
	resp, err := c.SubmitBenchmarkRun(context.Background(), BenchmarkRunRequest{
		Suite:    "infra-bench",
		Score:    &score,
		Band:     "excellent",
		Metadata: json.RawMessage(`{"contract_version":"v1.0.1"}`),
		Results: []BenchmarkResult{
			{
				CaseID:         "kubernetes/fix-deployment",
				ExpectedSignal: "safe_success",
				ActualSignal:   "safe_success",
				Passed:         true,
				Details:        json.RawMessage(`{"duration_ms":1234}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitBenchmarkRun: %v", err)
	}
	if resp.RunID != "run-123" {
		t.Fatalf("run_id = %q", resp.RunID)
	}
}

func TestSubmitBenchmarkRun_ServerError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, APIKey: "test-key"})
	_, err := c.SubmitBenchmarkRun(context.Background(), BenchmarkRunRequest{
		Suite: "infra-bench",
		Band:  "unknown",
	})
	if err == nil {
		t.Fatal("expected server error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("error = %v", err)
	}
}
