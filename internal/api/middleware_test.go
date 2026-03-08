package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimitMiddleware(t *testing.T) {
	t.Parallel()
	handler := bodyLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		w.WriteHeader(200)
	}))

	bigBody := strings.NewReader(strings.Repeat("x", 2048))
	req := httptest.NewRequest("POST", "/", bigBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Parallel()
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, map[string]string{"ok": "true"})

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}
