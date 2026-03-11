package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"samebits.com/evidra/internal/auth"
)

// RawEntryStore persists raw evidence entries.
type RawEntryStore interface {
	SaveRaw(ctx context.Context, tenantID string, raw json.RawMessage) (receiptID string, err error)
}

func handleForward(store RawEntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			writeError(w, http.StatusBadRequest, "empty or unreadable body")
			return
		}

		if !json.Valid(body) {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		receiptID, err := store.SaveRaw(r.Context(), tenantID, body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store entry failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"receipt_id": receiptID,
			"status":     "accepted",
		})
	}
}

func handleBatch(store RawEntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req struct {
			Entries []json.RawMessage `json:"entries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if len(req.Entries) == 0 {
			writeError(w, http.StatusBadRequest, "no entries provided")
			return
		}

		accepted := 0
		var errs []string
		for i, entry := range req.Entries {
			_, err := store.SaveRaw(r.Context(), tenantID, entry)
			if err != nil {
				errs = append(errs, fmt.Sprintf("entry %d: save failed", i))
				continue
			}
			accepted++
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"accepted": accepted,
			"errors":   errs,
		})
	}
}
