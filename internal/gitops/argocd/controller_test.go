package argocd

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"samebits.com/evidra/internal/store"
	testutil "samebits.com/evidra/internal/testutil"
	"samebits.com/evidra/pkg/evidence"
)

func TestController_ClaimsStartEventOnce(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:       "Running",
		Health:      "Progressing",
		Revision:    "abc123",
		OperationID: "argo-op-123",
	})
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), app)
	store := &fakeControllerStore{}
	controller := NewController(client, store, testutil.TestSigner(t), ControllerConfig{
		ApplicationNamespace: "argocd",
		TenantID:             "tenant-123",
	})

	if err := controller.SyncOnce(context.Background()); err != nil {
		t.Fatalf("first SyncOnce: %v", err)
	}
	if err := controller.SyncOnce(context.Background()); err != nil {
		t.Fatalf("second SyncOnce: %v", err)
	}
	if len(store.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(store.savedRaw))
	}

	entry := decodeControllerEntry(t, store.savedRaw[0])
	if entry.Type != evidence.EntryTypePrescribe {
		t.Fatalf("entry.Type = %q, want %q", entry.Type, evidence.EntryTypePrescribe)
	}
	payload := decodeControllerPayload(t, entry.Payload)
	if got, _ := payload["flavor"].(string); got != "reconcile" {
		t.Fatalf("payload flavor = %q, want reconcile", got)
	}
}

func TestController_ClaimsCompletionEventOnce(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:       "Succeeded",
		Health:      "Healthy",
		Revision:    "abc123",
		OperationID: "argo-op-123",
	})
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), app)
	store := &fakeControllerStore{}
	controller := NewController(client, store, testutil.TestSigner(t), ControllerConfig{
		ApplicationNamespace: "argocd",
		TenantID:             "tenant-123",
	})

	if err := controller.SyncOnce(context.Background()); err != nil {
		t.Fatalf("first SyncOnce: %v", err)
	}
	if err := controller.SyncOnce(context.Background()); err != nil {
		t.Fatalf("second SyncOnce: %v", err)
	}
	if len(store.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(store.savedRaw))
	}

	entry := decodeControllerEntry(t, store.savedRaw[0])
	if entry.Type != evidence.EntryTypeReport {
		t.Fatalf("entry.Type = %q, want %q", entry.Type, evidence.EntryTypeReport)
	}
	payload := decodeControllerPayload(t, entry.Payload)
	if got, _ := payload["flavor"].(string); got != "reconcile" {
		t.Fatalf("payload flavor = %q, want reconcile", got)
	}
}

func TestController_UsesExplicitReportWhenPrescriptionAnnotationExists(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Annotations: map[string]string{
			"evidra.cc/prescription-id": "presc-123",
			"evidra.cc/session-id":      "sess-123",
		},
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:       "Succeeded",
		Health:      "Healthy",
		Revision:    "abc123",
		OperationID: "argo-op-123",
	})
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), app)
	store := &fakeControllerStore{
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
	controller := NewController(client, store, testutil.TestSigner(t), ControllerConfig{
		ApplicationNamespace: "argocd",
		TenantID:             "tenant-123",
	})

	if err := controller.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if len(store.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(store.savedRaw))
	}

	entry := decodeControllerEntry(t, store.savedRaw[0])
	if entry.Type != evidence.EntryTypeReport {
		t.Fatalf("entry.Type = %q, want %q", entry.Type, evidence.EntryTypeReport)
	}

	var payload evidence.ReportPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("decode report payload: %v", err)
	}
	if payload.PrescriptionID != "presc-123" {
		t.Fatalf("payload.PrescriptionID = %q, want presc-123", payload.PrescriptionID)
	}
	if entry.SessionID != "sess-123" {
		t.Fatalf("entry.SessionID = %q, want sess-123", entry.SessionID)
	}
	if entry.OperationID != "deploy-123" {
		t.Fatalf("entry.OperationID = %q, want deploy-123", entry.OperationID)
	}
}

func TestController_UsesMappedLifecycleInZeroTouchMode(t *testing.T) {
	t.Parallel()

	startApp := newApplication(t, applicationFixture{
		Name: "payments-start",
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:       "Running",
		Health:      "Progressing",
		Revision:    "abc123",
		OperationID: "argo-op-start",
	})
	completeApp := newApplication(t, applicationFixture{
		Name: "payments-complete",
		UID:  "uid-456",
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:       "Succeeded",
		Health:      "Healthy",
		Revision:    "def456",
		OperationID: "argo-op-complete",
	})
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), startApp, completeApp)
	store := &fakeControllerStore{}
	controller := NewController(client, store, testutil.TestSigner(t), ControllerConfig{
		ApplicationNamespace: "argocd",
		TenantID:             "tenant-123",
	})

	if err := controller.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if len(store.savedRaw) != 2 {
		t.Fatalf("saved entries = %d, want 2", len(store.savedRaw))
	}

	startEntry := decodeControllerEntry(t, store.savedRaw[0])
	completeEntry := decodeControllerEntry(t, store.savedRaw[1])
	if startEntry.Type != evidence.EntryTypePrescribe {
		t.Fatalf("start entry type = %q, want %q", startEntry.Type, evidence.EntryTypePrescribe)
	}
	if completeEntry.Type != evidence.EntryTypeReport {
		t.Fatalf("complete entry type = %q, want %q", completeEntry.Type, evidence.EntryTypeReport)
	}
	if startEntry.ScopeDimensions["source_kind"] != "mapped" || completeEntry.ScopeDimensions["source_kind"] != "mapped" {
		t.Fatalf("source_kind values = %#v / %#v, want mapped", startEntry.ScopeDimensions, completeEntry.ScopeDimensions)
	}

	var reportPayload evidence.ReportPayload
	if err := json.Unmarshal(completeEntry.Payload, &reportPayload); err != nil {
		t.Fatalf("decode report payload: %v", err)
	}
	wantRefs := []evidence.ExternalRef{
		{Type: "argocd_application", ID: "argocd/payments-complete"},
		{Type: "argocd_application_uid", ID: "uid-456"},
		{Type: "argocd_revision", ID: "def456"},
		{Type: "argocd_operation", ID: "argo-op-complete"},
	}
	if !reflect.DeepEqual(reportPayload.ExternalRefs, wantRefs) {
		t.Fatalf("report external_refs = %#v, want %#v", reportPayload.ExternalRefs, wantRefs)
	}
}

type fakeControllerStore struct {
	lastHash string
	savedRaw []json.RawMessage
	claimed  map[string]json.RawMessage
	entries  map[string]store.StoredEntry
}

func (f *fakeControllerStore) LastHash(context.Context, string) (string, error) {
	return f.lastHash, nil
}

func (f *fakeControllerStore) SaveRaw(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	f.savedRaw = append(f.savedRaw, append(json.RawMessage(nil), raw...))
	return "receipt-1", nil
}

func (f *fakeControllerStore) GetEntry(_ context.Context, _ string, entryID string) (store.StoredEntry, error) {
	if entry, ok := f.entries[entryID]; ok {
		return entry, nil
	}
	return store.StoredEntry{}, store.ErrNotFound
}

func (f *fakeControllerStore) ClaimWebhookEvent(_ context.Context, tenantID, source, key string, payload json.RawMessage) (bool, error) {
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

func (f *fakeControllerStore) ReleaseWebhookEvent(_ context.Context, tenantID, source, key string) error {
	if f.claimed == nil {
		return nil
	}
	delete(f.claimed, tenantID+"|"+source+"|"+key)
	return nil
}

func decodeControllerEntry(t *testing.T, raw json.RawMessage) evidence.EvidenceEntry {
	t.Helper()

	var entry evidence.EvidenceEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("decode evidence entry: %v", err)
	}
	return entry
}

func decodeControllerPayload(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}
