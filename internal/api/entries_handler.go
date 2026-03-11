package api

import (
	"errors"
	"net/http"
	"strconv"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/store"
)

func handleListEntries(es *store.EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()

		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))

		opts := store.ListOptions{
			Limit:     limit,
			Offset:    offset,
			EntryType: q.Get("type"),
			Period:    q.Get("period"),
			SessionID: q.Get("session_id"),
		}
		entries, total, err := es.ListEntries(r.Context(), tenantID, opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list entries failed")
			return
		}

		// Echo resolved limit (after defaults) so clients know the effective page size.
		resolvedLimit := opts.Resolved().Limit
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"entries": toEntryAPIResponses(entries),
			"total":   total,
			"limit":   resolvedLimit,
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
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "entry not found")
			} else {
				writeError(w, http.StatusInternalServerError, "failed to retrieve entry")
			}
			return
		}

		writeJSON(w, http.StatusOK, toEntryAPIResponse(entry))
	}
}
