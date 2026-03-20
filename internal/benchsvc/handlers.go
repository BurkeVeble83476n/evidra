package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra/pkg/bench"
)

// RegisterRoutes adds bench intelligence routes to the given mux.
// Public routes (leaderboard, scenarios) are registered directly.
// Authenticated routes (ingest) go through authMw.
func RegisterRoutes(mux *http.ServeMux, s *PgStore, authMw func(http.Handler) http.Handler) {
	// Public — no auth.
	mux.HandleFunc("GET /v1/bench/leaderboard", handleLeaderboard(s))
	mux.HandleFunc("GET /v1/bench/scenarios", handleListScenarios(s))

	// Authenticated — ingest.
	mux.Handle("POST /v1/bench/runs", authMw(http.HandlerFunc(handleIngestRun(s))))
	mux.Handle("POST /v1/bench/runs/batch", authMw(http.HandlerFunc(handleIngestBatch(s))))

	// Authenticated — queries.
	mux.Handle("GET /v1/bench/runs", authMw(http.HandlerFunc(handleListRuns(s))))
	mux.Handle("GET /v1/bench/runs/{id}", authMw(http.HandlerFunc(handleGetRun(s))))
	mux.Handle("GET /v1/bench/runs/{id}/transcript", authMw(http.HandlerFunc(handleGetTranscript(s))))
	mux.Handle("GET /v1/bench/runs/{id}/tool-calls", authMw(http.HandlerFunc(handleGetToolCalls(s))))
	mux.Handle("GET /v1/bench/runs/{id}/timeline", authMw(http.HandlerFunc(handleGetTimeline(s))))
	mux.Handle("GET /v1/bench/stats", authMw(http.HandlerFunc(handleStats(s))))
	mux.Handle("GET /v1/bench/catalog", authMw(http.HandlerFunc(handleCatalog(s))))
	mux.Handle("GET /v1/bench/signals", authMw(http.HandlerFunc(handleSignals(s))))
	mux.Handle("GET /v1/bench/runs/{id}/scorecard", authMw(http.HandlerFunc(handleGetScorecard(s))))
}

func handleLeaderboard(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("evidence_mode")
		if mode == "" {
			mode = "proxy"
		}
		entries, err := s.Leaderboard(r.Context(), mode)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"models":        entries,
			"evidence_mode": mode,
		})
	}
}

// ingestRunRequest extends RunRecord with optional artifact fields.
type ingestRunRequest struct {
	bench.RunRecord
	Transcript string          `json:"transcript,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
}

func handleIngestRun(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ingestRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.ID == "" || req.ScenarioID == "" || req.Model == "" {
			respondError(w, http.StatusBadRequest, "id, scenario_id, and model are required")
			return
		}
		req.TenantID = s.tenantID
		if err := s.InsertRun(r.Context(), req.RunRecord); err != nil {
			respondError(w, http.StatusInternalServerError, "insert: "+err.Error())
			return
		}
		if err := storeIngestArtifacts(r.Context(), s, req); err != nil {
			respondError(w, http.StatusInternalServerError, "artifacts: "+err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": req.ID})
	}
}

func handleIngestBatch(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Runs []ingestRunRequest `json:"runs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Runs) == 0 {
			respondError(w, http.StatusBadRequest, "runs array is empty")
			return
		}
		records := make([]bench.RunRecord, len(req.Runs))
		for i := range req.Runs {
			req.Runs[i].TenantID = s.tenantID
			records[i] = req.Runs[i].RunRecord
		}
		count, err := s.InsertRunBatch(r.Context(), records)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "batch insert: "+err.Error())
			return
		}
		// Store artifacts for each run.
		for _, run := range req.Runs {
			if err := storeIngestArtifacts(r.Context(), s, run); err != nil {
				respondError(w, http.StatusInternalServerError, "artifacts: "+err.Error())
				return
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"imported": count,
			"total":    len(req.Runs),
		})
	}
}

func handleListRuns(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 {
			limit = 50
		}
		offset, _ := strconv.Atoi(q.Get("offset"))

		f := bench.RunFilters{
			ScenarioID:   q.Get("scenario"),
			Model:        q.Get("model"),
			Provider:     q.Get("provider"),
			EvidenceMode: q.Get("evidence_mode"),
			Since:        q.Get("since"),
			Limit:        limit,
			Offset:       offset,
			SortBy:       q.Get("sort_by"),
			SortOrder:    q.Get("sort_order"),
		}
		if q.Get("passed") == "true" {
			f.PassedOnly = true
		}
		if q.Get("passed") == "false" {
			f.FailedOnly = true
		}

		runs, total, err := s.ListRuns(r.Context(), f)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runs == nil {
			runs = []bench.RunRecord{}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items":  runs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func handleGetRun(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		run, err := s.GetRun(r.Context(), id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, http.StatusNotFound, "run not found")
			} else {
				respondError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		respondJSON(w, http.StatusOK, run)
	}
}

func handleStats(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := bench.RunFilters{
			ScenarioID:   q.Get("scenario"),
			Model:        q.Get("model"),
			Provider:     q.Get("provider"),
			EvidenceMode: q.Get("evidence_mode"),
			Since:        q.Get("since"),
		}
		st, err := s.FilteredStats(r.Context(), f)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, st)
	}
}

func handleGetTranscript(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		data, contentType, err := s.GetArtifact(r.Context(), id, "transcript")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, http.StatusNotFound, "transcript not found")
				return
			}
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleGetToolCalls(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		data, contentType, err := s.GetArtifact(r.Context(), id, "tool_calls")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, http.StatusNotFound, "tool calls not found")
				return
			}
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleGetTimeline(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		data, _, err := s.GetArtifact(r.Context(), id, "tool_calls")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, http.StatusNotFound, "tool calls not found (needed for timeline)")
				return
			}
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var calls []bench.ToolCall
		if err := json.Unmarshal(data, &calls); err != nil {
			respondError(w, http.StatusInternalServerError, "parse tool calls: "+err.Error())
			return
		}

		tl := bench.Parse(calls)
		respondJSON(w, http.StatusOK, tl)
	}
}

func handleCatalog(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cat, err := s.Catalog(r.Context())
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]any{"models": []string{}, "providers": []string{}})
			return
		}
		respondJSON(w, http.StatusOK, cat)
	}
}

func handleSignals(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"total_runs":          0,
			"runs_with_scorecard": 0,
			"signals":             map[string]any{},
			"avg_score":           0,
		})
	}
}

func handleGetScorecard(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		data, contentType, err := s.GetArtifact(r.Context(), id, "scorecard")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, http.StatusNotFound, "scorecard not found")
				return
			}
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

// storeIngestArtifacts stores transcript and tool_calls artifacts from an ingest request.
func storeIngestArtifacts(ctx context.Context, s *PgStore, req ingestRunRequest) error {
	if req.Transcript != "" {
		if err := s.StoreArtifact(ctx, req.ID, "transcript", "text/plain", []byte(req.Transcript)); err != nil {
			return fmt.Errorf("transcript: %w", err)
		}
	}
	if len(req.ToolCalls) > 0 {
		if err := s.StoreArtifact(ctx, req.ID, "tool_calls", "application/json", req.ToolCalls); err != nil {
			return fmt.Errorf("tool_calls: %w", err)
		}
	}
	return nil
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintf(w, `{"error":"encode: %s"}`, err)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func handleListScenarios(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarios, err := s.ListScenarios(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if scenarios == nil {
			scenarios = []bench.ScenarioSummary{}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"scenarios": scenarios,
		})
	}
}
