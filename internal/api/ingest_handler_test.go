package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/ingest"
	"samebits.com/evidra/internal/store"
	testutil "samebits.com/evidra/internal/testutil"
	"samebits.com/evidra/pkg/evidence"
)

type fakeIngestStore struct {
	lastHash      string
	entries       map[string]store.StoredEntry
	claimed       map[string]bool
	savedRaw      []json.RawMessage
	lastTenant    string
	getEntryCalls int
	claimCalls    int
	releaseCalls  int
}

func (f *fakeIngestStore) LastHash(context.Context, string) (string, error) {
	return f.lastHash, nil
}

func (f *fakeIngestStore) SaveRaw(_ context.Context, tenantID string, raw json.RawMessage) (string, error) {
	f.lastTenant = tenantID
	f.savedRaw = append(f.savedRaw, append(json.RawMessage(nil), raw...))
	return "receipt-1", nil
}

func (f *fakeIngestStore) GetEntry(_ context.Context, tenantID, entryID string) (store.StoredEntry, error) {
	f.lastTenant = tenantID
	f.getEntryCalls++
	if f.entries != nil {
		if entry, ok := f.entries[entryID]; ok {
			return entry, nil
		}
	}
	return store.StoredEntry{}, store.ErrNotFound
}

func (f *fakeIngestStore) ClaimWebhookEvent(_ context.Context, tenantID, source, key string, _ json.RawMessage) (bool, error) {
	f.lastTenant = tenantID
	f.claimCalls++
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	composite := source + ":" + key
	if f.claimed[composite] {
		return true, nil
	}
	f.claimed[composite] = true
	return false, nil
}

func (f *fakeIngestStore) ReleaseWebhookEvent(_ context.Context, tenantID, source, key string) error {
	f.lastTenant = tenantID
	f.releaseCalls++
	return nil
}

func TestIngestPrescribeHandler_AcceptsValidInput(t *testing.T) {
	t.Parallel()

	store := &fakeIngestStore{lastHash: "sha256:previous"}
	svc := ingest.NewService(store, testutil.TestSigner(t))
	handler := handleIngestPrescribe(svc)

	req := httptest.NewRequest("POST", "/v1/evidence/ingest/prescribe", bytes.NewReader(mustJSON(t, ingest.PrescribeRequest{
		Envelope: ingest.Envelope{
			ContractVersion: ingest.ContractVersionV1,
			Actor: evidence.Actor{
				Type:       " controller ",
				ID:         " ci-bot ",
				Provenance: " github-actions ",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: " argocd "},
		},
		CanonicalAction: &canon.CanonicalAction{
			Tool:           "kubectl",
			Operation:      "apply",
			OperationClass: "mutate",
			ScopeClass:     "production",
			ResourceCount:  1,
		},
	})))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithTenantID(req.Context(), "tenant-1"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Duplicate      bool   `json:"duplicate"`
		EntryID        string `json:"entry_id"`
		EffectiveRisk  string `json:"effective_risk"`
		PrescriptionID string `json:"prescription_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Duplicate {
		t.Fatal("expected non-duplicate response")
	}
	if resp.EntryID == "" {
		t.Fatal("expected entry_id")
	}
	if resp.EffectiveRisk == "" {
		t.Fatal("expected effective_risk")
	}
	if resp.PrescriptionID == "" {
		t.Fatal("expected prescription_id")
	}
	if store.lastTenant != "tenant-1" {
		t.Fatalf("tenant = %q, want tenant-1", store.lastTenant)
	}
	if len(store.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(store.savedRaw))
	}
}

func TestIngestReportHandler_AcceptsValidInput(t *testing.T) {
	t.Parallel()

	store := &fakeIngestStore{
		lastHash: "sha256:previous",
		entries: map[string]store.StoredEntry{
			"presc-1": {
				ID:        "presc-1",
				TenantID:  "tenant-1",
				EntryType: string(evidence.EntryTypePrescribe),
				SessionID: "session-1",
			},
		},
	}
	svc := ingest.NewService(store, testutil.TestSigner(t))
	handler := handleIngestReport(svc)

	req := httptest.NewRequest("POST", "/v1/evidence/ingest/report", bytes.NewReader(mustJSON(t, ingest.ReportRequest{
		Envelope: ingest.Envelope{
			ContractVersion: ingest.ContractVersionV1,
			Actor: evidence.Actor{
				Type:       " controller ",
				ID:         " ci-bot ",
				Provenance: " github-actions ",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: " argocd "},
		},
		PrescriptionID: "presc-1",
		Verdict:        evidence.VerdictSuccess,
		ExitCode:       intPtr(0),
	})))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithTenantID(req.Context(), "tenant-1"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Duplicate     bool   `json:"duplicate"`
		EntryID       string `json:"entry_id"`
		EffectiveRisk string `json:"effective_risk"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Duplicate {
		t.Fatal("expected non-duplicate response")
	}
	if resp.EntryID == "" {
		t.Fatal("expected entry_id")
	}
	if resp.EffectiveRisk != "" {
		t.Fatalf("effective_risk = %q, want empty string for report", resp.EffectiveRisk)
	}
	if store.lastTenant != "tenant-1" {
		t.Fatalf("tenant = %q, want tenant-1", store.lastTenant)
	}
	if len(store.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(store.savedRaw))
	}
	if store.getEntryCalls != 1 {
		t.Fatalf("get entry calls = %d, want 1", store.getEntryCalls)
	}
}

func TestIngestPrescribeHandler_InvalidPayloadReturns400(t *testing.T) {
	t.Parallel()

	svc := ingest.NewService(&fakeIngestStore{}, testutil.TestSigner(t))
	handler := handleIngestPrescribe(svc)

	req := httptest.NewRequest("POST", "/v1/evidence/ingest/prescribe", strings.NewReader(`{
		"contract_version":"v1",
		"actor":{"type":"controller","id":"ci-bot","provenance":"github-actions"},
		"session_id":"session-1",
		"operation_id":"operation-1",
		"trace_id":"trace-1",
		"flavor":"workflow",
		"evidence":{"kind":"observed"},
		"source":{"system":"argocd"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithTenantID(req.Context(), "tenant-1"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngestPrescribeHandler_DuplicateClaimResponseStable(t *testing.T) {
	t.Parallel()

	store := &fakeIngestStore{
		lastHash: "sha256:previous",
		claimed: map[string]bool{
			"gateway:claim-dup": true,
		},
	}
	svc := ingest.NewService(store, testutil.TestSigner(t))
	handler := handleIngestPrescribe(svc)

	body := mustJSON(t, ingest.PrescribeRequest{
		Envelope: ingest.Envelope{
			ContractVersion: ingest.ContractVersionV1,
			Claim: &ingest.Claim{
				Source: "gateway",
				Key:    "claim-dup",
			},
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "ci-bot",
				Provenance: "github-actions",
			},
			SessionID:   "session-dup",
			OperationID: "operation-dup",
			TraceID:     "trace-dup",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		CanonicalAction: &canon.CanonicalAction{
			Tool:           "kubectl",
			Operation:      "apply",
			OperationClass: "mutate",
			ScopeClass:     "production",
			ResourceCount:  1,
		},
	})

	first := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/v1/evidence/ingest/prescribe", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(auth.WithTenantID(req1.Context(), "tenant-1"))
	handler.ServeHTTP(first, req1)

	second := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/v1/evidence/ingest/prescribe", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(auth.WithTenantID(req2.Context(), "tenant-1"))
	handler.ServeHTTP(second, req2)

	if first.Code != http.StatusOK {
		t.Fatalf("first expected 200, got %d body=%s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusOK {
		t.Fatalf("second expected 200, got %d body=%s", second.Code, second.Body.String())
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("duplicate responses differ:\nfirst:  %s\nsecond: %s", first.Body.String(), second.Body.String())
	}
	if len(store.savedRaw) != 0 {
		t.Fatalf("saved entries = %d, want 0 for duplicate claim", len(store.savedRaw))
	}
}

func TestIngestRouter_RequiresAuth(t *testing.T) {
	t.Parallel()

	router := NewRouter(RouterConfig{
		APIKey:        "test-key",
		DefaultTenant: "tenant-1",
		EntryStore:    &store.EntryStore{},
		WebhookSigner: testutil.TestSigner(t),
	})

	for _, path := range []string{
		"/v1/evidence/ingest/prescribe",
		"/v1/evidence/ingest/report",
	} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s expected 401, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func intPtr(v int) *int {
	return &v
}
