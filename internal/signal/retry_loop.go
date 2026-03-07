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

// DefaultVariantRetryThreshold is the minimum number of same-scope prescriptions
// within the retry window to fire the variant retry signal. Higher than the exact
// threshold to reduce false positives on legitimate investigative retries.
const DefaultVariantRetryThreshold = 5

// DetectRetryLoops finds retry loop patterns using both exact and variant detection:
//   - Exact: same (actor, intent_digest, shape_hash) repeated >= DefaultRetryThreshold times
//   - Variant: same (actor, tool, operation_class, scope_class) repeated >= DefaultVariantRetryThreshold
//     times regardless of artifact content — catches agents that mutate the artifact
//     between attempts without making real progress.
//
// Results are merged and deduplicated by event ID.
func DetectRetryLoops(entries []Entry) SignalResult {
	exact := DetectRetryLoopsWithConfig(entries, DefaultRetryThreshold, DefaultRetryWindow)
	variant := DetectVariantRetryLoopsWithConfig(entries, DefaultVariantRetryThreshold, DefaultRetryWindow)

	seen := make(map[string]bool, len(exact.EventIDs)+len(variant.EventIDs))
	var merged []string
	for _, id := range append(exact.EventIDs, variant.EventIDs...) {
		if !seen[id] {
			seen[id] = true
			merged = append(merged, id)
		}
	}
	return SignalResult{
		Name:     "retry_loop",
		Count:    len(merged),
		EventIDs: merged,
	}
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

// DetectVariantRetryLoopsWithConfig detects retry loops where the agent mutates
// the artifact between attempts (different intent_digest / shape_hash) but keeps
// operating in the same (actor, tool, operation_class, scope_class) space after a
// failure. This captures investigative-looking loops that escape exact-match detection.
//
// A higher threshold (DefaultVariantRetryThreshold = 5) reduces false positives from
// legitimate troubleshooting where the operator is genuinely making different attempts.
func DetectVariantRetryLoopsWithConfig(entries []Entry, threshold int, window time.Duration) SignalResult {
	type key struct{ actor, tool, opClass, scope string }

	reportExitCode := make(map[string]*int)
	for _, e := range entries {
		if e.IsReport && e.PrescriptionID != "" {
			reportExitCode[e.PrescriptionID] = e.ExitCode
		}
	}

	groups := make(map[key][]Entry)
	for _, e := range entries {
		if !e.IsPrescription {
			continue
		}
		k := key{e.ActorID, e.Tool, e.OperationClass, e.ScopeClass}
		groups[k] = append(groups[k], e)
	}

	var eventIDs []string
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Timestamp.Before(group[j].Timestamp)
		})

		var chain []Entry
		failSeen := false
		for _, p := range group {
			ec, hasReport := reportExitCode[p.EventID]
			if !failSeen {
				if hasReport && ec != nil && *ec != 0 {
					failSeen = true
					chain = append(chain, p)
				}
				continue
			}
			if p.Timestamp.Sub(chain[0].Timestamp) <= window {
				chain = append(chain, p)
			} else {
				if len(chain) >= threshold {
					for _, c := range chain {
						eventIDs = append(eventIDs, c.EventID)
					}
				}
				chain = nil
				failSeen = false
				if hasReport && ec != nil && *ec != 0 {
					failSeen = true
					chain = append(chain, p)
				}
			}
		}
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
