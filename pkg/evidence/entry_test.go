package evidence

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvidenceEntry_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	payload := json.RawMessage(`{"tool":"kubectl","operation":"apply"}`)

	original := EvidenceEntry{
		EntryID:        "01JTEST000000000000000001",
		PreviousHash:   "sha256:abc123",
		Hash:           "sha256:def456",
		Signature:      "hmac-sha256:deadbeef",
		Type:           EntryTypePrescribe,
		TenantID:       "tenant-42",
		TraceID:        "trace-001",
		Actor:          Actor{Type: "agent", ID: "claude-code", Provenance: "mcp-stdio"},
		Timestamp:      ts,
		IntentDigest:   "sha256:intent111",
		ArtifactDigest: "sha256:artifact222",
		Payload:        payload,
		SpecVersion:    "0.3.0",
		CanonVersion:   "k8s-v1",
		AdapterVersion: "k8s-v1.0.0",
		ScoringVersion: "scoring-v1",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded EvidenceEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Verify all fields survive the round trip.
	if decoded.EntryID != original.EntryID {
		t.Errorf("EntryID: got %q, want %q", decoded.EntryID, original.EntryID)
	}
	if decoded.PreviousHash != original.PreviousHash {
		t.Errorf("PreviousHash: got %q, want %q", decoded.PreviousHash, original.PreviousHash)
	}
	if decoded.Hash != original.Hash {
		t.Errorf("Hash: got %q, want %q", decoded.Hash, original.Hash)
	}
	if decoded.Signature != original.Signature {
		t.Errorf("Signature: got %q, want %q", decoded.Signature, original.Signature)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.TenantID != original.TenantID {
		t.Errorf("TenantID: got %q, want %q", decoded.TenantID, original.TenantID)
	}
	if decoded.TraceID != original.TraceID {
		t.Errorf("TraceID: got %q, want %q", decoded.TraceID, original.TraceID)
	}
	if decoded.Actor.Type != original.Actor.Type {
		t.Errorf("Actor.Type: got %q, want %q", decoded.Actor.Type, original.Actor.Type)
	}
	if decoded.Actor.ID != original.Actor.ID {
		t.Errorf("Actor.ID: got %q, want %q", decoded.Actor.ID, original.Actor.ID)
	}
	if decoded.Actor.Provenance != original.Actor.Provenance {
		t.Errorf("Actor.Provenance: got %q, want %q", decoded.Actor.Provenance, original.Actor.Provenance)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
	if decoded.IntentDigest != original.IntentDigest {
		t.Errorf("IntentDigest: got %q, want %q", decoded.IntentDigest, original.IntentDigest)
	}
	if decoded.ArtifactDigest != original.ArtifactDigest {
		t.Errorf("ArtifactDigest: got %q, want %q", decoded.ArtifactDigest, original.ArtifactDigest)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Errorf("Payload: got %s, want %s", decoded.Payload, original.Payload)
	}
	if decoded.SpecVersion != original.SpecVersion {
		t.Errorf("SpecVersion: got %q, want %q", decoded.SpecVersion, original.SpecVersion)
	}
	if decoded.CanonVersion != original.CanonVersion {
		t.Errorf("CanonVersion: got %q, want %q", decoded.CanonVersion, original.CanonVersion)
	}
	if decoded.AdapterVersion != original.AdapterVersion {
		t.Errorf("AdapterVersion: got %q, want %q", decoded.AdapterVersion, original.AdapterVersion)
	}
	if decoded.ScoringVersion != original.ScoringVersion {
		t.Errorf("ScoringVersion: got %q, want %q", decoded.ScoringVersion, original.ScoringVersion)
	}

	// Verify omitempty fields produce correct JSON keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal raw map: %v", err)
	}
	if _, ok := raw["tenant_id"]; !ok {
		t.Error("expected tenant_id key in JSON when TenantID is set")
	}
	if _, ok := raw["intent_digest"]; !ok {
		t.Error("expected intent_digest key in JSON when IntentDigest is set")
	}

	// Verify omitempty fields are absent when empty.
	empty := EvidenceEntry{
		EntryID:      "01JTEST000000000000000002",
		Type:         EntryTypeReport,
		TraceID:      "trace-002",
		Timestamp:    ts,
		SpecVersion:  "0.3.0",
		CanonVersion: "generic-v1",
	}
	emptyData, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("json.Marshal empty: %v", err)
	}
	var emptyRaw map[string]json.RawMessage
	if err := json.Unmarshal(emptyData, &emptyRaw); err != nil {
		t.Fatalf("json.Unmarshal empty raw: %v", err)
	}
	if _, ok := emptyRaw["tenant_id"]; ok {
		t.Error("expected tenant_id to be omitted when empty")
	}
	if _, ok := emptyRaw["scoring_version"]; ok {
		t.Error("expected scoring_version to be omitted when empty")
	}
}

func TestEntryType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		et    EntryType
		valid bool
	}{
		{name: "prescribe", et: EntryTypePrescribe, valid: true},
		{name: "report", et: EntryTypeReport, valid: true},
		{name: "finding", et: EntryTypeFinding, valid: true},
		{name: "signal", et: EntryTypeSignal, valid: true},
		{name: "receipt", et: EntryTypeReceipt, valid: true},
		{name: "canonicalization_failure", et: EntryTypeCanonFailure, valid: true},
		{name: "session_start", et: EntryTypeSessionStart, valid: true},
		{name: "session_end", et: EntryTypeSessionEnd, valid: true},
		{name: "annotation", et: EntryTypeAnnotation, valid: true},
		{name: "empty string", et: EntryType(""), valid: false},
		{name: "unknown type", et: EntryType("unknown"), valid: false},
		{name: "uppercase", et: EntryType("PRESCRIBE"), valid: false},
		{name: "partial match", et: EntryType("prescrib"), valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.et.Valid()
			if got != tt.valid {
				t.Errorf("EntryType(%q).Valid() = %v, want %v", tt.et, got, tt.valid)
			}
		})
	}
}

func TestEntryType_NewTypesAreValid(t *testing.T) {
	t.Parallel()
	newTypes := []EntryType{
		EntryTypeSessionStart,
		EntryTypeSessionEnd,
		EntryTypeAnnotation,
	}
	for _, et := range newTypes {
		if !et.Valid() {
			t.Errorf("expected %q to be valid", et)
		}
	}
}
