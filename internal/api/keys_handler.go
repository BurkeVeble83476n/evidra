package api

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
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

		// Periodic cleanup: prune stale entries when map grows large.
		if len(history) > 10000 {
			for k, timestamps := range history {
				var fresh []time.Time
				for _, t := range timestamps {
					if t.After(cutoff) {
						fresh = append(fresh, t)
					}
				}
				if len(fresh) == 0 {
					delete(history, k)
				} else {
					history[k] = fresh
				}
			}
		}

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

		// Create tenant + key with a unique tenant ID.
		tenantID := "tnt_" + ulid.Make().String()
		plaintext, rec, err := ks.CreateKey(r.Context(), tenantID, req.Label)
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
