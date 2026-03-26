package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"

	"samebits.com/evidra/internal/apiutil"
	"samebits.com/evidra/internal/auth"
)

func handleRegisterRunner(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		var req RegisterRunnerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Models) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "models is required")
			return
		}
		runner, err := svc.RegisterRunner(r.Context(), tenantID, req)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
			"runner_id":     runner.ID,
			"poll_interval": runner.Config.PollInterval,
		})
	}
}

func handleListRunners(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		runners, err := svc.ListRunners(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runners == nil {
			runners = []Runner{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"runners": runners})
	}
}

func handleDeleteRunner(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		runnerID := r.PathValue("id")
		if err := svc.DeleteRunner(r.Context(), tenantID, runnerID); err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "runner not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
