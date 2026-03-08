package api

import (
	"encoding/json"
	"io"
	"net/http"

	"samebits.com/evidra-benchmark/internal/auth"
)

func handleFindings(store RawEntryStore) http.HandlerFunc {
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
			writeError(w, http.StatusInternalServerError, "store finding failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"receipt_id": receiptID,
			"status":     "accepted",
		})
	}
}
