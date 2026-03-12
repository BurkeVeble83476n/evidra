package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"samebits.com/evidra/internal/store"
)

func TestClientIP_UsesForwardedForOnlyFromTrustedProxy(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/v1/keys", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.20, 10.0.0.5")

	if got := clientIP(req); got != "198.51.100.20" {
		t.Fatalf("clientIP = %q, want 198.51.100.20", got)
	}
}

func TestClientIP_IgnoresForwardedForFromUntrustedPeer(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/v1/keys", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.20")

	if got := clientIP(req); got != "203.0.113.10" {
		t.Fatalf("clientIP = %q, want 203.0.113.10", got)
	}
}

func TestHandleKeys_InvalidInviteDoesNotConsumeRateLimit(t *testing.T) {
	t.Parallel()

	handler := handleKeys(&store.KeyStore{}, "invite-secret")

	for attempt := 0; attempt < keyIssueRateLimitPerIP+1; attempt++ {
		req := httptest.NewRequest("POST", "/v1/keys", strings.NewReader(`{"label":"ci"}`))
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Invite-Secret", "wrong-secret")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("attempt %d: expected 403, got %d", attempt+1, rec.Code)
		}
	}
}

func TestHandleKeys_RateLimitAppliesAfterInviteValidation(t *testing.T) {
	t.Parallel()

	handler := handleKeys(&store.KeyStore{}, "invite-secret")

	for attempt := 0; attempt < keyIssueRateLimitPerIP; attempt++ {
		req := httptest.NewRequest("POST", "/v1/keys", strings.NewReader(`{`))
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Invite-Secret", "invite-secret")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400, got %d", attempt+1, rec.Code)
		}
	}

	req := httptest.NewRequest("POST", "/v1/keys", strings.NewReader(`{`))
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Invite-Secret", "invite-secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on rate-limited attempt, got %d", rec.Code)
	}
}
