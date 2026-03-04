package signal

// DetectNewScope flags prescriptions that introduce an (actor, tool, operation_class, scope_class)
// combination not seen in earlier entries. Entries must be sorted chronologically.
func DetectNewScope(entries []Entry) SignalResult {
	type scopeKey struct {
		actor   string
		tool    string
		opClass string
		scope   string
	}

	seen := make(map[scopeKey]bool)
	var eventIDs []string

	for _, e := range entries {
		if !e.IsPrescription {
			continue
		}
		k := scopeKey{e.ActorID, e.Tool, e.OperationClass, e.ScopeClass}
		if !seen[k] {
			seen[k] = true
			eventIDs = append(eventIDs, e.EventID)
		}
	}

	return SignalResult{
		Name:     "new_scope",
		Count:    len(eventIDs),
		EventIDs: eventIDs,
	}
}
