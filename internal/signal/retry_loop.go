package signal

import (
	"sort"
	"time"
)

const (
	// DefaultRetryThreshold is the minimum number of same-intent prescriptions
	// within the retry window to fire the signal.
	DefaultRetryThreshold = 3

	// DefaultRetryWindow is the time window for retry loop detection.
	DefaultRetryWindow = 30 * time.Minute
)

// DetectRetryLoops finds cases where the same (actor, intent_digest, shape_hash)
// appears N or more times within a time window after a prior failed execution.
func DetectRetryLoops(entries []Entry) SignalResult {
	return DetectRetryLoopsWithConfig(entries, DefaultRetryThreshold, DefaultRetryWindow)
}

// DetectRetryLoopsWithConfig allows configurable threshold and window.
func DetectRetryLoopsWithConfig(entries []Entry, threshold int, window time.Duration) SignalResult {
	type key struct{ actor, intent, shape string }

	// Build report lookup: prescription_id → exit_code.
	reportExitCode := make(map[string]*int)
	for _, e := range entries {
		if e.IsReport && e.PrescriptionID != "" {
			reportExitCode[e.PrescriptionID] = e.ExitCode
		}
	}

	// Group prescriptions by (actor, intent, shape).
	groups := make(map[key][]Entry)
	for _, e := range entries {
		if !e.IsPrescription || e.IntentDigest == "" {
			continue
		}
		k := key{e.ActorID, e.IntentDigest, e.ShapeHash}
		groups[k] = append(groups[k], e)
	}

	var eventIDs []string
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Timestamp.Before(group[j].Timestamp)
		})

		// Walk through prescriptions; only count retries after a failed execution.
		var chain []Entry
		failSeen := false
		for _, p := range group {
			ec, hasReport := reportExitCode[p.EventID]
			if !failSeen {
				// Look for the first failure to start a chain.
				if hasReport && ec != nil && *ec != 0 {
					failSeen = true
					chain = append(chain, p)
				}
				continue
			}
			// After a failure, subsequent prescriptions within window are retries.
			if p.Timestamp.Sub(chain[0].Timestamp) <= window {
				chain = append(chain, p)
			} else {
				// Window expired; check if chain met threshold, then reset.
				if len(chain) >= threshold {
					for _, c := range chain {
						eventIDs = append(eventIDs, c.EventID)
					}
				}
				chain = nil
				failSeen = false
				// Re-check current entry as potential new failure start.
				if hasReport && ec != nil && *ec != 0 {
					failSeen = true
					chain = append(chain, p)
				}
			}
		}
		// Check remaining chain.
		if len(chain) >= threshold {
			for _, c := range chain {
				eventIDs = append(eventIDs, c.EventID)
			}
		}
	}

	return SignalResult{
		Name:     "retry_loop",
		Count:    len(eventIDs),
		EventIDs: eventIDs,
	}
}
