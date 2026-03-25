package benchsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"samebits.com/evidra/internal/apiutil"
)

// handleTrigger returns a handler that starts a new bench trigger job.
// POST /v1/bench/trigger
func handleTrigger(svc *Service, store *TriggerStore, executor RunExecutor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if executor == nil {
			apiutil.WriteError(w, http.StatusNotImplemented, "bench trigger not configured: no executor")
			return
		}

		var req TriggerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Model == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "model is required")
			return
		}
		if len(req.Scenarios) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "scenarios is required")
			return
		}

		progress := make([]ScenarioProgress, len(req.Scenarios))
		for i, s := range req.Scenarios {
			progress[i] = ScenarioProgress{Scenario: s, Status: "pending"}
		}

		job := &TriggerJob{
			ID:        NewJobID(),
			Status:    "pending",
			Model:     req.Model,
			Provider:  req.Provider,
			Total:     len(req.Scenarios),
			Progress:  progress,
			CreatedAt: time.Now(),
		}
		store.Create(job)

		evidraURL := resolveEvidraURL(r)
		apiKey := r.Header.Get("Authorization")

		go func() {
			if err := executor.Start(context.Background(), job, evidraURL, apiKey); err != nil {
				log.Printf("[bench-trigger] executor failed for job %s: %v", job.ID, err)
				store.Update(ProgressUpdate{
					JobID:     job.ID,
					Scenario:  "",
					Status:    "error",
					Completed: job.Total,
					Total:     job.Total,
				})
			}
		}()

		apiutil.WriteJSON(w, http.StatusAccepted, map[string]any{
			"id":     job.ID,
			"status": job.Status,
		})
	}
}

// handleTriggerStatus returns a handler for GET /v1/bench/trigger/{id}.
// Supports SSE streaming when the client accepts it, otherwise returns a JSON snapshot.
func handleTriggerStatus(store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job := store.Get(id)
		if job == nil {
			apiutil.WriteError(w, http.StatusNotFound, "job not found")
			return
		}

		// Check if SSE is possible.
		flusher, ok := w.(http.Flusher)
		if !ok || r.Header.Get("Accept") != "text/event-stream" {
			apiutil.WriteJSON(w, http.StatusOK, job)
			return
		}

		// SSE mode.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send current state.
		writeSSE(w, "status", job)
		flusher.Flush()

		// If already terminal, close immediately.
		if job.Status == "completed" || job.Status == "failed" {
			writeSSE(w, "complete", job)
			flusher.Flush()
			return
		}

		ch := store.Subscribe(id)
		defer store.Unsubscribe(id, ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case update, open := <-ch:
				if !open {
					return
				}
				writeSSE(w, "progress", update)
				flusher.Flush()

				// Check if job is done.
				current := store.Get(id)
				if current != nil && (current.Status == "completed" || current.Status == "failed") {
					writeSSE(w, "complete", current)
					flusher.Flush()
					return
				}
			}
		}
	}
}

// handleTriggerProgress returns a handler for POST /v1/bench/trigger/{id}/progress.
// This is the webhook endpoint called by the bench service.
func handleTriggerProgress(store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var update ProgressUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if update.ContractVersion == "" {
			// Accept for backward compatibility, but future versions will require it.
		} else if update.ContractVersion != ExecutorContractVersion {
			apiutil.WriteError(w, http.StatusBadRequest, "unsupported contract version")
			return
		}
		update.JobID = id

		if !store.Update(update) {
			apiutil.WriteError(w, http.StatusNotFound, "job not found")
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// resolveEvidraURL determines the base URL for the Evidra API from the request.
// In container deployments, EVIDRA_SELF_URL overrides request-based resolution
// because external and internal URLs may differ.
func resolveEvidraURL(r *http.Request) string {
	if selfURL := os.Getenv("EVIDRA_SELF_URL"); selfURL != "" {
		return selfURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// writeSSE writes a server-sent event to the response writer.
func writeSSE(w http.ResponseWriter, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}
