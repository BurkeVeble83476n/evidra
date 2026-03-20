package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"samebits.com/evidra/internal/apiutil"
	"samebits.com/evidra/internal/auth"
	bench "samebits.com/evidra/pkg/bench"
)

// RegisterRoutes adds bench intelligence routes to the given mux.
// Public routes (leaderboard, scenarios) are registered directly.
// Authenticated routes go through authMw and extract tenant from context.
func RegisterRoutes(mux *http.ServeMux, svc *Service, authMw func(http.Handler) http.Handler) {
	// Public — no auth.
	mux.HandleFunc("GET /v1/bench/leaderboard", handleLeaderboard(svc))
	mux.HandleFunc("GET /v1/bench/scenarios", handleListScenarios(svc))

	// Authenticated — ingest.
	mux.Handle("POST /v1/bench/runs", authMw(http.HandlerFunc(handleIngestRun(svc))))
	mux.Handle("POST /v1/bench/runs/batch", authMw(http.HandlerFunc(handleIngestBatch(svc))))

	// Authenticated — queries.
	mux.Handle("GET /v1/bench/runs", authMw(http.HandlerFunc(handleListRuns(svc))))
	mux.Handle("GET /v1/bench/runs/{id}", authMw(http.HandlerFunc(handleGetRun(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/transcript", authMw(http.HandlerFunc(handleGetTranscript(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/tool-calls", authMw(http.HandlerFunc(handleGetToolCalls(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/timeline", authMw(http.HandlerFunc(handleGetTimeline(svc))))
	mux.Handle("GET /v1/bench/stats", authMw(http.HandlerFunc(handleStats(svc))))
	mux.Handle("GET /v1/bench/catalog", authMw(http.HandlerFunc(handleCatalog(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scorecard", authMw(http.HandlerFunc(handleGetScorecard(svc))))
	mux.Handle("GET /v1/bench/compare/runs", authMw(http.HandlerFunc(handleCompareRuns(svc))))
	mux.Handle("GET /v1/bench/compare/models", authMw(http.HandlerFunc(handleCompareModels(svc))))
}

// parseSince parses a "since" query parameter as RFC3339 or date string.
// Returns nil if the string is empty or unparseable.
func parseSince(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
	}
	if err != nil {
		return nil
	}
	return &t
}

func handleLeaderboard(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("evidence_mode")
		if mode == "" {
			mode = "proxy"
		}
		entries, err := svc.Leaderboard(r.Context(), mode)
		if err != nil {
			if errors.Is(err, ErrPublicTenantUnavailable) {
				apiutil.WriteError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"models":        entries,
			"evidence_mode": mode,
		})
	}
}

func handleIngestRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req IngestRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.ID == "" || req.ScenarioID == "" || req.Model == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "id, scenario_id, and model are required")
			return
		}
		req.TenantID = tenantID
		if err := svc.IngestRun(r.Context(), tenantID, req); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "insert: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": req.ID})
	}
}

func handleIngestBatch(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req struct {
			Runs []IngestRunRequest `json:"runs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Runs) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "runs array is empty")
			return
		}
		for i := range req.Runs {
			req.Runs[i].TenantID = tenantID
		}
		count, err := svc.IngestRunBatch(r.Context(), tenantID, req.Runs)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "batch insert: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"imported": count,
			"total":    len(req.Runs),
		})
	}
}

func handleListRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
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
			Since:        parseSince(q.Get("since")),
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

		runs, total, err := svc.ListRuns(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runs == nil {
			runs = []bench.RunRecord{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"items":  runs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func handleGetRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		run, err := svc.GetRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, run)
	}
}

func handleStats(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		f := bench.RunFilters{
			ScenarioID:   q.Get("scenario"),
			Model:        q.Get("model"),
			Provider:     q.Get("provider"),
			EvidenceMode: q.Get("evidence_mode"),
			Since:        parseSince(q.Get("since")),
		}
		st, err := svc.FilteredStats(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, st)
	}
}

func handleGetTranscript(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, "transcript")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				apiutil.WriteError(w, http.StatusNotFound, "transcript not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleGetToolCalls(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, "tool_calls")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				apiutil.WriteError(w, http.StatusNotFound, "tool calls not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleGetTimeline(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, _, err := svc.GetArtifact(r.Context(), tenantID, id, "tool_calls")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				apiutil.WriteError(w, http.StatusNotFound, "tool calls not found (needed for timeline)")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var calls []bench.ToolCall
		if err := json.Unmarshal(data, &calls); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "parse tool calls: "+err.Error())
			return
		}

		tl := bench.Parse(calls)
		apiutil.WriteJSON(w, http.StatusOK, tl)
	}
}

func handleCatalog(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		cat, err := svc.Catalog(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteJSON(w, http.StatusOK, map[string]any{"models": []string{}, "providers": []string{}})
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, cat)
	}
}

func handleGetScorecard(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, "scorecard")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				apiutil.WriteError(w, http.StatusNotFound, "scorecard not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleListScenarios(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarios, err := svc.ListScenarios(r.Context())
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if scenarios == nil {
			scenarios = []bench.ScenarioSummary{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"scenarios": scenarios,
		})
	}
}

func handleCompareRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		a := r.URL.Query().Get("a")
		b := r.URL.Query().Get("b")
		if a == "" || b == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'a' and 'b' (run IDs) are required")
			return
		}
		result, err := svc.CompareRuns(r.Context(), tenantID, a, b)
		if err != nil {
			apiutil.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}

func handleCompareModels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		modelA := r.URL.Query().Get("a")
		modelB := r.URL.Query().Get("b")
		if modelA == "" || modelB == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'a' and 'b' (model names) are required")
			return
		}
		mode := r.URL.Query().Get("evidence_mode")
		result, err := svc.CompareModels(r.Context(), tenantID, modelA, modelB, mode)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}
