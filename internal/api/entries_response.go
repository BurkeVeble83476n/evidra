package api

import (
	"encoding/json"
	"time"

	"samebits.com/evidra/internal/store"
)

type entryAPIResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Tool      string    `json:"tool,omitempty"`
	Operation string    `json:"operation,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	RiskLevel string    `json:"risk_level,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	Verdict   string    `json:"verdict,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// payloadFields captures evidence fields that may appear at different
// nesting levels depending on how the entry was stored.
type payloadFields struct {
	Tool            string `json:"tool"`
	Operation       string `json:"operation"`
	EffectiveRisk   string `json:"effective_risk"`
	RiskLevel       string `json:"risk_level"`
	CanonicalAction struct {
		Tool           string `json:"tool"`
		Operation      string `json:"operation"`
		OperationClass string `json:"operation_class"`
		ScopeClass     string `json:"scope_class"`
	} `json:"canonical_action"`
	Verdict  string `json:"verdict"`
	ExitCode *int   `json:"exit_code"`
	Scope    string `json:"scope"`
}

func toEntryAPIResponse(e store.StoredEntry) entryAPIResponse {
	resp := entryAPIResponse{
		ID:        e.ID,
		Type:      e.EntryType,
		CreatedAt: e.CreatedAt,
	}

	// The stored JSONB has two possible shapes:
	// 1. Full envelope (forward/batch): {actor, payload: {...}, scope_dimensions}
	// 2. Direct payload (lifecycle/MCP): {canonical_action, effective_risk, ...}
	// Parse both and merge.
	var envelope struct {
		Actor struct {
			ID string `json:"id"`
		} `json:"actor"`
		Payload         payloadFields     `json:"payload"`
		ScopeDimensions map[string]string `json:"scope_dimensions"`
	}
	_ = json.Unmarshal(e.Payload, &envelope)

	var flat payloadFields
	_ = json.Unmarshal(e.Payload, &flat)

	// Merge: prefer envelope.Payload (nested), fall back to flat (direct).
	p := envelope.Payload
	if p.EffectiveRisk == "" {
		p.EffectiveRisk = flat.EffectiveRisk
	}
	if p.RiskLevel == "" {
		p.RiskLevel = flat.RiskLevel
	}
	if p.Verdict == "" {
		p.Verdict = flat.Verdict
	}
	if p.ExitCode == nil {
		p.ExitCode = flat.ExitCode
	}
	if p.CanonicalAction.Tool == "" {
		p.CanonicalAction = flat.CanonicalAction
	}
	if p.Tool == "" {
		p.Tool = flat.Tool
	}
	if p.Operation == "" {
		p.Operation = flat.Operation
	}
	if p.Scope == "" {
		p.Scope = flat.Scope
	}

	resp.Actor = envelope.Actor.ID
	resp.Verdict = p.Verdict
	resp.ExitCode = p.ExitCode
	resp.RiskLevel = p.EffectiveRisk
	if resp.RiskLevel == "" {
		resp.RiskLevel = p.RiskLevel
	}

	resp.Tool = p.CanonicalAction.Tool
	if resp.Tool == "" {
		resp.Tool = p.Tool
	}
	resp.Operation = p.CanonicalAction.Operation
	if resp.Operation == "" {
		resp.Operation = p.Operation
	}
	resp.Scope = p.CanonicalAction.ScopeClass
	if resp.Scope == "" {
		resp.Scope = p.Scope
	}

	if resp.Tool == "" {
		resp.Tool = envelope.ScopeDimensions["tool"]
	}

	return resp
}

func toEntryAPIResponses(entries []store.StoredEntry) []entryAPIResponse {
	out := make([]entryAPIResponse, len(entries))
	for i, e := range entries {
		out[i] = toEntryAPIResponse(e)
	}
	return out
}
