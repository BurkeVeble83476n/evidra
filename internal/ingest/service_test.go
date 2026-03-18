package ingest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/store"
	testutil "samebits.com/evidra/internal/testutil"
	"samebits.com/evidra/pkg/evidence"
)

func TestServicePrescribe_CreatesAndStoresSignedPrescribeEntry(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	signer := testutil.TestSigner(t)
	svc := NewService(fakeStore, signer)
	action := canonicalActionForTest()
	tenantID := "tenant-1"

	req := PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Claim: &Claim{
				Source:  "gateway",
				Key:     "claim-1",
				Payload: json.RawMessage(`{"kind":"claim"}`),
			},
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:       "session-1",
			OperationID:     "operation-1",
			TraceID:         "trace-1",
			ScopeDimensions: map[string]string{"cluster": "prod"},
			Flavor:          evidence.FlavorWorkflow,
			Evidence:        &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindTranslated},
			Source:          &evidence.SourceMetadata{System: "argocd"},
		},
		CanonicalAction: &action,
	}

	out, err := svc.Prescribe(context.Background(), tenantID, req)
	if err != nil {
		t.Fatalf("Prescribe: %v", err)
	}
	if out.Duplicate {
		t.Fatal("expected non-duplicate prescribe result")
	}
	if out.Entry.EntryID == "" {
		t.Fatal("expected entry id")
	}
	if len(fakeStore.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(fakeStore.savedRaw))
	}

	entry := decodeStoredEntry(t, fakeStore.savedRaw[0])
	if entry.Type != evidence.EntryTypePrescribe {
		t.Fatalf("entry type = %q, want %q", entry.Type, evidence.EntryTypePrescribe)
	}
	if entry.Signature == "" {
		t.Fatal("expected signed entry")
	}
	if !signer.Verify([]byte(entry.Hash), mustDecodeSignature(t, entry.Signature)) {
		t.Fatal("signature verification failed")
	}

	var payload evidence.PrescriptionPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("unmarshal prescribe payload: %v", err)
	}
	if payload.Flavor != evidence.FlavorWorkflow {
		t.Fatalf("payload flavor = %q, want workflow", payload.Flavor)
	}
	if payload.Evidence == nil || payload.Evidence.Kind != evidence.EvidenceKindTranslated {
		t.Fatalf("payload evidence = %+v, want translated", payload.Evidence)
	}
	if payload.Source == nil || payload.Source.System != "argocd" {
		t.Fatalf("payload source = %+v, want argocd", payload.Source)
	}
}

func TestServiceReport_CreatesAndStoresSignedReportEntry(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	fakeStore.entries["presc-1"] = store.StoredEntry{
		ID:        "presc-1",
		TenantID:  "tenant-1",
		EntryType: string(evidence.EntryTypePrescribe),
		SessionID: "session-1",
	}
	signer := testutil.TestSigner(t)
	svc := NewService(fakeStore, signer)
	tenantID := "tenant-1"

	req := ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:       "session-1",
			OperationID:     "operation-1",
			TraceID:         "trace-1",
			ScopeDimensions: map[string]string{"cluster": "prod"},
			Flavor:          evidence.FlavorWorkflow,
			Evidence:        &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindTranslated},
			Source:          &evidence.SourceMetadata{System: "argocd"},
		},
		PrescriptionID: "presc-1",
		Verdict:        evidence.VerdictSuccess,
		ExitCode:       intPtr(0),
	}

	out, err := svc.Report(context.Background(), tenantID, req)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if out.Duplicate {
		t.Fatal("expected non-duplicate report result")
	}
	if len(fakeStore.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(fakeStore.savedRaw))
	}
	if fakeStore.getEntryCalls != 1 {
		t.Fatalf("get entry calls = %d, want 1", fakeStore.getEntryCalls)
	}

	entry := decodeStoredEntry(t, fakeStore.savedRaw[0])
	if entry.Type != evidence.EntryTypeReport {
		t.Fatalf("entry type = %q, want %q", entry.Type, evidence.EntryTypeReport)
	}
	if entry.Signature == "" {
		t.Fatal("expected signed entry")
	}
	if !signer.Verify([]byte(entry.Hash), mustDecodeSignature(t, entry.Signature)) {
		t.Fatal("signature verification failed")
	}

	var payload evidence.ReportPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload.PrescriptionID != "presc-1" {
		t.Fatalf("payload prescription_id = %q, want presc-1", payload.PrescriptionID)
	}
	if payload.Flavor != evidence.FlavorWorkflow {
		t.Fatalf("payload flavor = %q, want workflow", payload.Flavor)
	}
	if payload.Evidence == nil || payload.Evidence.Kind != evidence.EvidenceKindTranslated {
		t.Fatalf("payload evidence = %+v, want translated", payload.Evidence)
	}
	if payload.Source == nil || payload.Source.System != "argocd" {
		t.Fatalf("payload source = %+v, want argocd", payload.Source)
	}
}

func TestServiceDuplicateClaim_ReturnsDuplicateWithoutStoringTwice(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	svc := NewService(fakeStore, testutil.TestSigner(t))
	action := canonicalActionForTest()
	tenantID := "tenant-1"

	req := PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Claim: &Claim{
				Source:  "gateway",
				Key:     "claim-dup",
				Payload: json.RawMessage(`{"kind":"claim"}`),
			},
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-dup",
			OperationID: "operation-dup",
			TraceID:     "trace-dup",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindTranslated},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		CanonicalAction: &action,
	}

	first, err := svc.Prescribe(context.Background(), tenantID, req)
	if err != nil {
		t.Fatalf("Prescribe(first): %v", err)
	}
	if first.Duplicate {
		t.Fatal("first claim should not be duplicate")
	}

	second, err := svc.Prescribe(context.Background(), tenantID, req)
	if err != nil {
		t.Fatalf("Prescribe(second): %v", err)
	}
	if !second.Duplicate {
		t.Fatal("duplicate claim should be reported as duplicate")
	}
	if len(fakeStore.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(fakeStore.savedRaw))
	}
}

func TestServiceReport_ResolvesReferencedPrescription(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	fakeStore.entries["presc-2"] = store.StoredEntry{
		ID:        "presc-2",
		TenantID:  "tenant-1",
		EntryType: string(evidence.EntryTypePrescribe),
		SessionID: "session-resolve",
	}
	svc := NewService(fakeStore, testutil.TestSigner(t))
	tenantID := "tenant-1"

	req := ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-resolve",
			OperationID: "operation-resolve",
			TraceID:     "trace-resolve",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindTranslated},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		PrescriptionID: "presc-2",
		Verdict:        evidence.VerdictFailure,
		ExitCode:       intPtr(1),
	}

	out, err := svc.Report(context.Background(), tenantID, req)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if out.Duplicate {
		t.Fatal("expected non-duplicate report result")
	}
	if fakeStore.getEntryCalls != 1 {
		t.Fatalf("get entry calls = %d, want 1", fakeStore.getEntryCalls)
	}
	if len(fakeStore.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(fakeStore.savedRaw))
	}

	entry := decodeStoredEntry(t, fakeStore.savedRaw[0])
	var payload evidence.ReportPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload.PrescriptionID != "presc-2" {
		t.Fatalf("payload prescription_id = %q, want presc-2", payload.PrescriptionID)
	}
}

func TestServiceTaxonomyFields_PropagateIntoTypedPayloads(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	action := canonicalActionForTest()
	fakeStore.entries["presc-3"] = store.StoredEntry{
		ID:        "presc-3",
		TenantID:  "tenant-1",
		EntryType: string(evidence.EntryTypePrescribe),
		SessionID: "session-taxonomy",
	}
	svc := NewService(fakeStore, testutil.TestSigner(t))
	tenantID := "tenant-1"

	prescOut, err := svc.Prescribe(context.Background(), tenantID, PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:       "session-taxonomy",
			OperationID:     "operation-taxonomy",
			TraceID:         "trace-taxonomy",
			Flavor:          evidence.FlavorReconcile,
			Evidence:        &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindDeclared},
			Source:          &evidence.SourceMetadata{System: "argocd"},
			ScopeDimensions: map[string]string{"cluster": "prod"},
		},
		CanonicalAction: &action,
	})
	if err != nil {
		t.Fatalf("Prescribe: %v", err)
	}

	reportOut, err := svc.Report(context.Background(), tenantID, ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:       "session-taxonomy",
			OperationID:     "operation-taxonomy",
			TraceID:         "trace-taxonomy",
			Flavor:          evidence.FlavorReconcile,
			Evidence:        &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindDeclared},
			Source:          &evidence.SourceMetadata{System: "argocd"},
			ScopeDimensions: map[string]string{"cluster": "prod"},
		},
		PrescriptionID: prescOut.Entry.EntryID,
		Verdict:        evidence.VerdictSuccess,
		ExitCode:       intPtr(0),
	})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if reportOut.Entry.EntryID == "" {
		t.Fatal("expected report entry id")
	}

	prescEntry := decodeStoredEntry(t, fakeStore.savedRaw[0])
	reportEntry := decodeStoredEntry(t, fakeStore.savedRaw[1])

	var prescPayload evidence.PrescriptionPayload
	if err := json.Unmarshal(prescEntry.Payload, &prescPayload); err != nil {
		t.Fatalf("unmarshal prescribe payload: %v", err)
	}
	var reportPayload evidence.ReportPayload
	if err := json.Unmarshal(reportEntry.Payload, &reportPayload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}

	for name, got := range map[string]*evidence.EvidenceMetadata{
		"prescribe": prescPayload.Evidence,
		"report":    reportPayload.Evidence,
	} {
		if got == nil || got.Kind != evidence.EvidenceKindDeclared {
			t.Fatalf("%s evidence = %+v, want declared", name, got)
		}
	}
	for name, got := range map[string]*evidence.SourceMetadata{
		"prescribe": prescPayload.Source,
		"report":    reportPayload.Source,
	} {
		if got == nil || got.System != "argocd" {
			t.Fatalf("%s source = %+v, want argocd", name, got)
		}
	}
}

type fakeIngestStore struct {
	lastHash      string
	entries       map[string]store.StoredEntry
	claimed       map[string]json.RawMessage
	savedRaw      []json.RawMessage
	getEntryCalls int
}

func newFakeIngestStore() *fakeIngestStore {
	return &fakeIngestStore{
		entries: make(map[string]store.StoredEntry),
		claimed: make(map[string]json.RawMessage),
	}
}

func (f *fakeIngestStore) LastHash(context.Context, string) (string, error) {
	return f.lastHash, nil
}

func (f *fakeIngestStore) SaveRaw(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	f.savedRaw = append(f.savedRaw, append(json.RawMessage(nil), raw...))
	var entry evidence.EvidenceEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return "", err
	}
	if entry.EntryID == "" {
		return "", errors.New("missing entry_id")
	}
	f.entries[entry.EntryID] = store.StoredEntry{
		ID:          entry.EntryID,
		EntryType:   string(entry.Type),
		SessionID:   entry.SessionID,
		OperationID: entry.OperationID,
		Payload:     append(json.RawMessage(nil), entry.Payload...),
	}
	return entry.EntryID, nil
}

func (f *fakeIngestStore) GetEntry(_ context.Context, _ string, entryID string) (store.StoredEntry, error) {
	f.getEntryCalls++
	entry, ok := f.entries[entryID]
	if !ok {
		return store.StoredEntry{}, store.ErrNotFound
	}
	return entry, nil
}

func (f *fakeIngestStore) ClaimWebhookEvent(_ context.Context, tenantID, source, key string, payload json.RawMessage) (bool, error) {
	claimKey := tenantID + "|" + source + "|" + key
	if _, ok := f.claimed[claimKey]; ok {
		return true, nil
	}
	f.claimed[claimKey] = append(json.RawMessage(nil), payload...)
	return false, nil
}

func (f *fakeIngestStore) ReleaseWebhookEvent(context.Context, string, string, string) error {
	return nil
}

func canonicalActionForTest() canon.CanonicalAction {
	return canon.CanonicalAction{
		Tool:              "kubectl",
		Operation:         "apply",
		OperationClass:    "mutate",
		ScopeClass:        "production",
		ResourceCount:     1,
		ResourceShapeHash: "sha256:" + strings.Repeat("a", 64),
	}
}

func decodeStoredEntry(t *testing.T, raw json.RawMessage) evidence.EvidenceEntry {
	t.Helper()

	var entry evidence.EvidenceEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("unmarshal stored entry: %v", err)
	}
	return entry
}

func mustDecodeSignature(t *testing.T, signature string) []byte {
	t.Helper()

	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	return sig
}

func intPtr(v int) *int {
	return &v
}
