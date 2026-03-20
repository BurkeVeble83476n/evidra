package bench

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// RegisterRoutes adds bench intelligence routes to the given mux.
// Public routes (leaderboard, scenarios) are registered directly.
// Authenticated routes (ingest) go through authMw.
func RegisterRoutes(mux *http.ServeMux, s *PgStore, authMw func(http.Handler) http.Handler) {
	// Public — no auth.
	mux.HandleFunc("GET /v1/bench/leaderboard", handleLeaderboard(s))

	// Authenticated — ingest.
	mux.Handle("POST /v1/bench/runs", authMw(http.HandlerFunc(handleIngestRun(s))))
	mux.Handle("POST /v1/bench/runs/batch", authMw(http.HandlerFunc(handleIngestBatch(s))))

	// Authenticated — queries.
	mux.Handle("GET /v1/bench/runs", authMw(http.HandlerFunc(handleListRuns(s))))
	mux.Handle("GET /v1/bench/runs/{id}", authMw(http.HandlerFunc(handleGetRun(s))))
	mux.Handle("GET /v1/bench/stats", authMw(http.HandlerFunc(handleStats(s))))
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

func handleIngestRun(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rec RunRecord
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if rec.ID == "" || rec.ScenarioID == "" || rec.Model == "" {
			respondError(w, http.StatusBadRequest, "id, scenario_id, and model are required")
			return
		}
		rec.TenantID = s.tenantID
		if err := s.InsertRun(r.Context(), rec); err != nil {
			respondError(w, http.StatusInternalServerError, "insert: "+err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": rec.ID})
	}
}

func handleIngestBatch(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Runs []RunRecord `json:"runs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Runs) == 0 {
			respondError(w, http.StatusBadRequest, "runs array is empty")
			return
		}
		for i := range req.Runs {
			req.Runs[i].TenantID = s.tenantID
		}
		count, err := s.InsertRunBatch(r.Context(), req.Runs)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "batch insert: "+err.Error())
			return
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

		f := RunFilters{
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
			runs = []RunRecord{}
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
			respondError(w, http.StatusNotFound, "run not found")
			return
		}
		respondJSON(w, http.StatusOK, run)
	}
}

func handleStats(s *PgStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := RunFilters{
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
