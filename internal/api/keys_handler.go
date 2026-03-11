package api

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"samebits.com/evidra/internal/store"
)

func handleKeys(ks *store.KeyStore, inviteSecret string) http.HandlerFunc {
	var (
		mu      sync.Mutex
		history = make(map[string][]time.Time) // IP -> timestamps
	)

	return func(w http.ResponseWriter, r *http.Request) {
		if ks == nil {
			writeError(w, http.StatusNotImplemented, "key management not available")
			return
		}

		// Rate limit: 3 keys per hour per IP.
		ip := clientIP(r)
		mu.Lock()
		now := time.Now()
		cutoff := now.Add(-1 * time.Hour)
		var recent []time.Time
		for _, t := range history[ip] {
			if t.After(cutoff) {
				recent = append(recent, t)
			}
		}
		if len(recent) >= 3 {
			mu.Unlock()
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		history[ip] = append(recent, now)
		mu.Unlock()

		// Invite gate is required for key issuance.
		if inviteSecret == "" {
			writeError(w, http.StatusServiceUnavailable, "key issuance disabled: invite secret not configured")
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Invite-Secret")), []byte(inviteSecret)) != 1 {
			writeError(w, http.StatusForbidden, "invite required")
			return
		}

		var req struct {
			Label string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if len(req.Label) > 128 {
			writeError(w, http.StatusBadRequest, "label too long (max 128)")
			return
		}

		// Create tenant + key.
		plaintext, rec, err := ks.CreateKey(r.Context(), "default", req.Label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "key creation failed")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"key":        plaintext,
			"prefix":     rec.Prefix,
			"tenant_id":  rec.TenantID,
			"created_at": rec.CreatedAt,
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use only the leftmost (client) IP.
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
