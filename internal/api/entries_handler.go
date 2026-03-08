package api

import (
	"net/http"
	"strconv"

	"samebits.com/evidra-benchmark/internal/auth"
	"samebits.com/evidra-benchmark/internal/store"
)

func handleListEntries(es *store.EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()

		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))

		entries, total, err := es.ListEntries(r.Context(), tenantID, store.ListOptions{
			Limit:     limit,
			Offset:    offset,
			EntryType: q.Get("type"),
			Period:    q.Get("period"),
			SessionID: q.Get("session_id"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list entries failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"entries": entries,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		})
	}
}

func handleGetEntry(es *store.EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		entryID := r.PathValue("id")
		if entryID == "" {
			writeError(w, http.StatusBadRequest, "missing entry id")
			return
		}

		entry, err := es.GetEntry(r.Context(), tenantID, entryID)
		if err != nil {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}

		writeJSON(w, http.StatusOK, entry)
	}
}
