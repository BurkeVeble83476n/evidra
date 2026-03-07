package signal

import "sort"

// DetectRepairLoop finds sequences where a failed intent later succeeds
// with a different artifact digest for the same actor+intent.
func DetectRepairLoop(entries []Entry) SignalResult {
	type key struct{ actor, intent string }

	reportExit := make(map[string]*int)
	for _, e := range entries {
		if e.IsReport && e.PrescriptionID != "" {
			reportExit[e.PrescriptionID] = e.ExitCode
		}
	}

	groups := make(map[key][]Entry)
	for _, e := range entries {
		if !e.IsPrescription || e.IntentDigest == "" {
			continue
		}
		k := key{actor: e.ActorID, intent: e.IntentDigest}
		groups[k] = append(groups[k], e)
	}

	var eventIDs []string
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].Timestamp.Before(group[j].Timestamp)
		})

		sawFailure := false
		failDigest := ""
		for _, p := range group {
			ec := reportExit[p.EventID]
			if ec != nil && *ec != 0 {
				sawFailure = true
				failDigest = p.ArtifactDigest
				continue
			}
			if sawFailure && ec != nil && *ec == 0 && p.ArtifactDigest != failDigest {
				eventIDs = append(eventIDs, p.EventID)
				sawFailure = false
				failDigest = ""
			}
		}
	}

	return SignalResult{
		Name:     "repair_loop",
		Count:    len(eventIDs),
		EventIDs: eventIDs,
	}
}
