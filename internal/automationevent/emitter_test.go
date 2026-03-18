package automationevent

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/store"
	testutil "samebits.com/evidra/internal/testutil"
	"samebits.com/evidra/pkg/evidence"
)

func TestEmitterMappedLifecycleCreatesPrescribeAndReport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	eventStore := &fakeEventStore{lastHash: "sha256:previous"}
	emitter := NewEmitter(eventStore, testutil.TestSigner(t))

	action := canon.CanonicalAction{
		Tool:              "argocd",
		Operation:         "sync",
		OperationClass:    "mutate",
		ScopeClass:        "production",
		ResourceCount:     1,
		ResourceShapeHash: "sha256:shape",
	}
	scope := map[string]string{
		"source_kind":           "mapped",
		"source_system":         "argocd_controller",
		"integration_mode":      "zero_touch",
		"correlation_mode":      "best_effort",
		"environment":           "production",
		"cluster":               "prod-us-east",
		"namespace":             "payments",
		"application":           "payments-app",
		"application_namespace": "argocd",
		"project":               "default",
		"revision":              "abc123",
	}
	reportRefs := []evidence.ExternalRef{
		{Type: "argocd_application", ID: "argocd/payments-app"},
		{Type: "argocd_revision", ID: "abc123"},
		{Type: "argocd_operation", ID: "op-123"},
	}
	desiredDigest := canon.SHA256Hex([]byte("desired"))
	observedDigest := canon.SHA256Hex([]byte("observed"))

	prescribeResult, err := emitter.EmitMappedPrescribe(ctx, MappedPrescribeInput{
		TenantID:        "tenant-123",
		ClaimSource:     "argocd_controller_start",
		ClaimKey:        "payments-app:op-123:start",
		ClaimPayload:    json.RawMessage(`{"event":"sync_started"}`),
		Actor:           evidence.Actor{Type: "controller", ID: "argocd-controller", Provenance: "mapped:argocd"},
		SessionID:       "op-123",
		OperationID:     "op-123",
		PrescriptionID:  "map-prescription-123",
		Action:          action,
		ArtifactDigest:  desiredDigest,
		ScopeDimensions: scope,
		Flavor:          FlavorReconcile,
		EvidenceKind:    evidence.EvidenceKindTranslated,
		SourceSystem:    "argocd",
	})
	if err != nil {
		t.Fatalf("EmitMappedPrescribe: %v", err)
	}
	if prescribeResult.Duplicate {
		t.Fatal("EmitMappedPrescribe reported duplicate on first claim")
	}

	duplicateResult, err := emitter.EmitMappedPrescribe(ctx, MappedPrescribeInput{
		TenantID:        "tenant-123",
		ClaimSource:     "argocd_controller_start",
		ClaimKey:        "payments-app:op-123:start",
		ClaimPayload:    json.RawMessage(`{"event":"sync_started"}`),
		Actor:           evidence.Actor{Type: "controller", ID: "argocd-controller", Provenance: "mapped:argocd"},
		SessionID:       "op-123",
		OperationID:     "op-123",
		PrescriptionID:  "map-prescription-123",
		Action:          action,
		ArtifactDigest:  desiredDigest,
		ScopeDimensions: scope,
		Flavor:          FlavorReconcile,
		EvidenceKind:    evidence.EvidenceKindTranslated,
		SourceSystem:    "argocd",
	})
	if err != nil {
		t.Fatalf("EmitMappedPrescribe duplicate: %v", err)
	}
	if !duplicateResult.Duplicate {
		t.Fatal("EmitMappedPrescribe duplicate claim was not treated as duplicate")
	}
	if len(eventStore.savedRaw) != 1 {
		t.Fatalf("saved entries after duplicate = %d, want 1", len(eventStore.savedRaw))
	}

	reportResult, err := emitter.EmitMappedReport(ctx, MappedReportInput{
		TenantID:        "tenant-123",
		ClaimSource:     "argocd_controller_complete",
		ClaimKey:        "payments-app:op-123:complete",
		ClaimPayload:    json.RawMessage(`{"event":"sync_completed"}`),
		Actor:           evidence.Actor{Type: "controller", ID: "argocd-controller", Provenance: "mapped:argocd"},
		SessionID:       "op-123",
		OperationID:     "op-123",
		PrescriptionID:  "map-prescription-123",
		ArtifactDigest:  observedDigest,
		ScopeDimensions: scope,
		Verdict:         evidence.VerdictSuccess,
		ExitCode:        intPtr(0),
		ExternalRefs:    reportRefs,
		Flavor:          FlavorReconcile,
		EvidenceKind:    evidence.EvidenceKindTranslated,
		SourceSystem:    "argocd",
	})
	if err != nil {
		t.Fatalf("EmitMappedReport: %v", err)
	}
	if reportResult.Duplicate {
		t.Fatal("EmitMappedReport reported duplicate on first claim")
	}
	if len(eventStore.savedRaw) != 2 {
		t.Fatalf("saved entries = %d, want 2", len(eventStore.savedRaw))
	}

	prescribeEntry := decodeEntry(t, eventStore.savedRaw[0])
	reportEntry := decodeEntry(t, eventStore.savedRaw[1])

	if prescribeEntry.Type != evidence.EntryTypePrescribe {
		t.Fatalf("prescribe entry type = %q, want %q", prescribeEntry.Type, evidence.EntryTypePrescribe)
	}
	if reportEntry.Type != evidence.EntryTypeReport {
		t.Fatalf("report entry type = %q, want %q", reportEntry.Type, evidence.EntryTypeReport)
	}
	if !reflect.DeepEqual(prescribeEntry.ScopeDimensions, scope) {
		t.Fatalf("prescribe scope_dimensions = %#v, want %#v", prescribeEntry.ScopeDimensions, scope)
	}
	if !reflect.DeepEqual(reportEntry.ScopeDimensions, scope) {
		t.Fatalf("report scope_dimensions = %#v, want %#v", reportEntry.ScopeDimensions, scope)
	}

	var typedPrescribe evidence.PrescriptionPayload
	if err := json.Unmarshal(prescribeEntry.Payload, &typedPrescribe); err != nil {
		t.Fatalf("decode prescribe payload: %v", err)
	}
	if typedPrescribe.Flavor != evidence.FlavorReconcile {
		t.Fatalf("prescribe payload flavor = %q, want reconcile", typedPrescribe.Flavor)
	}
	if typedPrescribe.Evidence == nil || typedPrescribe.Evidence.Kind != evidence.EvidenceKindTranslated {
		t.Fatalf("prescribe payload evidence = %+v, want translated", typedPrescribe.Evidence)
	}
	if typedPrescribe.Source == nil || typedPrescribe.Source.System != "argocd" {
		t.Fatalf("prescribe payload source = %+v, want argocd", typedPrescribe.Source)
	}

	var typedReport evidence.ReportPayload
	if err := json.Unmarshal(reportEntry.Payload, &typedReport); err != nil {
		t.Fatalf("decode report payload: %v", err)
	}
	if typedReport.Flavor != evidence.FlavorReconcile {
		t.Fatalf("report payload flavor = %q, want reconcile", typedReport.Flavor)
	}
	if typedReport.Evidence == nil || typedReport.Evidence.Kind != evidence.EvidenceKindTranslated {
		t.Fatalf("report payload evidence = %+v, want translated", typedReport.Evidence)
	}
	if typedReport.Source == nil || typedReport.Source.System != "argocd" {
		t.Fatalf("report payload source = %+v, want argocd", typedReport.Source)
	}
	if typedReport.PrescriptionID != prescribeEntry.EntryID {
		t.Fatalf("report prescription_id = %q, want %q", typedReport.PrescriptionID, prescribeEntry.EntryID)
	}
	if !reflect.DeepEqual(typedReport.ExternalRefs, reportRefs) {
		t.Fatalf("report external_refs = %#v, want %#v", typedReport.ExternalRefs, reportRefs)
	}
}

func TestEmitterExplicitReportLinksExistingPrescriptionID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	eventStore := &fakeEventStore{
		lastHash: "sha256:previous",
		entries: map[string]store.StoredEntry{
			"presc-123": {
				ID:          "presc-123",
				TenantID:    "tenant-123",
				EntryType:   string(evidence.EntryTypePrescribe),
				SessionID:   "sess-123",
				OperationID: "deploy-123",
			},
		},
	}
	emitter := NewEmitter(eventStore, testutil.TestSigner(t))

	scope := map[string]string{
		"source_kind":           "controller_observed",
		"source_system":         "argocd_controller",
		"integration_mode":      "explicit",
		"correlation_mode":      "explicit",
		"environment":           "production",
		"cluster":               "prod-us-east",
		"namespace":             "payments",
		"application":           "payments-app",
		"application_namespace": "argocd",
		"project":               "default",
		"revision":              "abc123",
	}
	reportRefs := []evidence.ExternalRef{
		{Type: "argocd_application", ID: "argocd/payments-app"},
		{Type: "argocd_application_uid", ID: "uid-123"},
		{Type: "argocd_revision", ID: "abc123"},
		{Type: "argocd_operation", ID: "argo-op-123"},
	}
	observedDigest := canon.SHA256Hex([]byte("observed"))

	result, err := emitter.EmitExplicitReport(ctx, ExplicitReportInput{
		TenantID:        "tenant-123",
		ClaimSource:     "argocd_controller_complete",
		ClaimKey:        "payments-app:argo-op-123:complete",
		ClaimPayload:    json.RawMessage(`{"event":"sync_completed"}`),
		Actor:           evidence.Actor{Type: "controller", ID: "argocd-controller", Provenance: "mapped:argocd"},
		PrescriptionID:  "presc-123",
		ArtifactDigest:  observedDigest,
		ScopeDimensions: scope,
		Verdict:         evidence.VerdictSuccess,
		ExitCode:        intPtr(0),
		ExternalRefs:    reportRefs,
		Flavor:          FlavorReconcile,
		EvidenceKind:    evidence.EvidenceKindTranslated,
		SourceSystem:    "argocd",
	})
	if err != nil {
		t.Fatalf("EmitExplicitReport: %v", err)
	}
	if result.Duplicate {
		t.Fatal("EmitExplicitReport reported duplicate on first claim")
	}
	if eventStore.getEntryCalls != 1 {
		t.Fatalf("GetEntry calls = %d, want 1", eventStore.getEntryCalls)
	}
	if len(eventStore.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(eventStore.savedRaw))
	}

	reportEntry := decodeEntry(t, eventStore.savedRaw[0])
	if reportEntry.Type != evidence.EntryTypeReport {
		t.Fatalf("report entry type = %q, want %q", reportEntry.Type, evidence.EntryTypeReport)
	}
	if reportEntry.SessionID != "sess-123" {
		t.Fatalf("report session_id = %q, want sess-123", reportEntry.SessionID)
	}
	if reportEntry.OperationID != "deploy-123" {
		t.Fatalf("report operation_id = %q, want deploy-123", reportEntry.OperationID)
	}
	if !reflect.DeepEqual(reportEntry.ScopeDimensions, scope) {
		t.Fatalf("report scope_dimensions = %#v, want %#v", reportEntry.ScopeDimensions, scope)
	}

	var typedReport evidence.ReportPayload
	if err := json.Unmarshal(reportEntry.Payload, &typedReport); err != nil {
		t.Fatalf("decode report payload: %v", err)
	}
	if typedReport.Flavor != evidence.FlavorReconcile {
		t.Fatalf("report payload flavor = %q, want reconcile", typedReport.Flavor)
	}
	if typedReport.Evidence == nil || typedReport.Evidence.Kind != evidence.EvidenceKindTranslated {
		t.Fatalf("report payload evidence = %+v, want translated", typedReport.Evidence)
	}
	if typedReport.Source == nil || typedReport.Source.System != "argocd" {
		t.Fatalf("report payload source = %+v, want argocd", typedReport.Source)
	}
	if typedReport.PrescriptionID != "presc-123" {
		t.Fatalf("report prescription_id = %q, want presc-123", typedReport.PrescriptionID)
	}
	if !reflect.DeepEqual(typedReport.ExternalRefs, reportRefs) {
		t.Fatalf("report external_refs = %#v, want %#v", typedReport.ExternalRefs, reportRefs)
	}
}

type fakeEventStore struct {
	lastHash      string
	savedRaw      []json.RawMessage
	claimed       map[string]json.RawMessage
	entries       map[string]store.StoredEntry
	getEntryCalls int
}

func (f *fakeEventStore) LastHash(context.Context, string) (string, error) {
	return f.lastHash, nil
}

func (f *fakeEventStore) SaveRaw(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	f.savedRaw = append(f.savedRaw, append(json.RawMessage(nil), raw...))
	return "receipt-1", nil
}

func (f *fakeEventStore) GetEntry(_ context.Context, _ string, entryID string) (store.StoredEntry, error) {
	f.getEntryCalls++
	if entry, ok := f.entries[entryID]; ok {
		return entry, nil
	}
	return store.StoredEntry{}, store.ErrNotFound
}

func (f *fakeEventStore) ClaimWebhookEvent(_ context.Context, tenantID, source, key string, payload json.RawMessage) (bool, error) {
	if f.claimed == nil {
		f.claimed = make(map[string]json.RawMessage)
	}
	claimKey := tenantID + "|" + source + "|" + key
	if _, ok := f.claimed[claimKey]; ok {
		return true, nil
	}
	f.claimed[claimKey] = append(json.RawMessage(nil), payload...)
	return false, nil
}

func (f *fakeEventStore) ReleaseWebhookEvent(_ context.Context, tenantID, source, key string) error {
	if f.claimed == nil {
		return nil
	}
	delete(f.claimed, tenantID+"|"+source+"|"+key)
	return nil
}

func decodeEntry(t *testing.T, raw json.RawMessage) evidence.EvidenceEntry {
	t.Helper()

	var entry evidence.EvidenceEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("decode evidence entry: %v", err)
	}
	return entry
}

func intPtr(v int) *int {
	return &v
}

var _ EventStore = (*fakeEventStore)(nil)
