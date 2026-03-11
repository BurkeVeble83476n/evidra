package mcpserver

import (
	"sync"
	"time"
)

// RetryTracker tracks (intent_digest, shape_hash, timestamp) tuples to detect
// retry loops. It is in-memory, per-process.
type RetryTracker struct {
	mu      sync.Mutex
	entries map[string]*retryEntry
	ttl     time.Duration
	stop    chan struct{}
}

type retryEntry struct {
	FirstSeen time.Time
	LastSeen  time.Time
	Count     int
	ShapeHash string
}

// NewRetryTracker creates a tracker with the given TTL for entries.
// A background goroutine periodically removes expired entries.
func NewRetryTracker(ttl time.Duration) *RetryTracker {
	rt := &RetryTracker{
		entries: make(map[string]*retryEntry),
		ttl:     ttl,
		stop:    make(chan struct{}),
	}
	go rt.cleanupLoop()
	return rt
}

func (rt *RetryTracker) cleanupLoop() {
	ticker := time.NewTicker(rt.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rt.Cleanup()
		case <-rt.stop:
			return
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (rt *RetryTracker) Stop() {
	select {
	case <-rt.stop:
	default:
		close(rt.stop)
	}
}

// Record records a prescription for the given intent digest and shape hash.
// Returns the current retry count (1 = first time, 2+ = retries).
func (rt *RetryTracker) Record(intentDigest, shapeHash string) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := intentDigest + ":" + shapeHash
	now := time.Now()

	entry, ok := rt.entries[key]
	if !ok || now.Sub(entry.FirstSeen) > rt.ttl {
		rt.entries[key] = &retryEntry{
			FirstSeen: now,
			LastSeen:  now,
			Count:     1,
			ShapeHash: shapeHash,
		}
		return 1
	}

	entry.Count++
	entry.LastSeen = now
	return entry.Count
}

// RetryCount returns the current count for an intent+shape pair. Returns 0
// if not tracked or expired.
func (rt *RetryTracker) RetryCount(intentDigest, shapeHash string) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := intentDigest + ":" + shapeHash
	entry, ok := rt.entries[key]
	if !ok || time.Since(entry.FirstSeen) > rt.ttl {
		return 0
	}
	return entry.Count
}

// Cleanup removes all expired entries.
func (rt *RetryTracker) Cleanup() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()
	for k, e := range rt.entries {
		if now.Sub(e.FirstSeen) > rt.ttl {
			delete(rt.entries, k)
		}
	}
}
