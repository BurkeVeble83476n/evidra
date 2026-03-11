package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/store"
)

func handleBenchmarkRun(bs *store.BenchmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bs == nil {
			writeError(w, http.StatusNotImplemented, "benchmark storage not available")
			return
		}

		tenantID := auth.TenantID(r.Context())

		var req struct {
			Suite   string                  `json:"suite"`
			Score   *float64                `json:"score,omitempty"`
			Band    string                  `json:"band"`
			Results []store.BenchmarkResult `json:"results"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		run := store.BenchmarkRun{
			TenantID:  tenantID,
			Suite:     req.Suite,
			Score:     req.Score,
			Band:      req.Band,
			StartedAt: time.Now().UTC(),
		}

		runID, err := bs.SaveRun(r.Context(), run, req.Results)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "save benchmark run failed")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"run_id": runID,
			"status": "accepted",
		})
	}
}

func handleBenchmarkRuns(bs *store.BenchmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bs == nil {
			writeError(w, http.StatusNotImplemented, "benchmark storage not available")
			return
		}

		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))

		runs, err := bs.ListRuns(r.Context(), tenantID, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list benchmark runs failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"runs": runs,
		})
	}
}

func handleBenchmarkCompare(bs *store.BenchmarkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bs == nil {
			writeError(w, http.StatusNotImplemented, "benchmark storage not available")
			return
		}

		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		runA := q.Get("run_a")
		runB := q.Get("run_b")
		if runA == "" || runB == "" {
			writeError(w, http.StatusBadRequest, "run_a and run_b query params required")
			return
		}

		a, aResults, err := bs.GetRunWithResults(r.Context(), tenantID, runA)
		if err != nil {
			writeError(w, http.StatusNotFound, "run_a not found")
			return
		}
		b, bResults, err := bs.GetRunWithResults(r.Context(), tenantID, runB)
		if err != nil {
			writeError(w, http.StatusNotFound, "run_b not found")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"run_a":     a,
			"run_b":     b,
			"results_a": aResults,
			"results_b": bResults,
		})
	}
}
