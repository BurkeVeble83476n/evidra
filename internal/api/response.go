// Package api provides HTTP handlers for the Evidra API server.
package api

import (
	"net/http"

	"samebits.com/evidra/internal/apiutil"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	apiutil.WriteJSON(w, status, v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	apiutil.WriteError(w, status, msg)
}
