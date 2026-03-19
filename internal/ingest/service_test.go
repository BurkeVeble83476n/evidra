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
				Type:       " controller ",
				ID:         " argocd ",
				Provenance: " argocd ",
			},
			SessionID:       "session-1",
			OperationID:     "operation-1",
			TraceID:         "trace-1",
			SpanID:          " span-1 ",
			ParentSpanID:    " parent-span-1 ",
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
	if entry.Actor.Type != "controller" || entry.Actor.ID != "argocd" || entry.Actor.Provenance != "argocd" {
		t.Fatalf("entry actor = %+v, want trimmed fields", entry.Actor)
	}
	if entry.SpanID != "span-1" || entry.ParentSpanID != "parent-span-1" {
		t.Fatalf("entry span fields = span_id=%q parent_span_id=%q, want trimmed values", entry.SpanID, entry.ParentSpanID)
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
	var actionPayload canon.CanonicalAction
	if err := json.Unmarshal(payload.CanonicalAction, &actionPayload); err != nil {
		t.Fatalf("unmarshal canonical action: %v", err)
	}
	if actionPayload.Tool != "kubectl" || actionPayload.Operation != "apply" {
		t.Fatalf("canonical action = %+v, want trimmed tool/operation", actionPayload)
	}
	if actionPayload.ScopeClass != "production" {
		t.Fatalf("canonical action scope_class = %q, want production", actionPayload.ScopeClass)
	}
}

func TestServicePrescribe_PayloadOverrideAlignsEntryIDAndPayload(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	svc := NewService(fakeStore, testutil.TestSigner(t))
	tenantID := "tenant-1"

	override := json.RawMessage(`{
		"prescription_id":"presc-override",
		"canonical_action":{
			"tool":" kubectl ",
			"operation":" apply ",
			"operation_class":" mutate ",
			"scope_class":" stage ",
			"resource_count":1,
			"resource_shape_hash":"sha256:` + strings.Repeat("a", 64) + `"
		},
		"ttl_ms":300000,
		"canon_source":"external",
		"risk_inputs":[{"source":"evidra/matrix","risk_level":"medium"}],
		"effective_risk":"medium"
	}`)

	out, err := svc.Prescribe(context.Background(), tenantID, PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       " controller ",
				ID:         " argocd ",
				Provenance: " argocd ",
			},
			SessionID:    "session-override",
			OperationID:  "operation-override",
			TraceID:      "trace-override",
			SpanID:       " span-override ",
			ParentSpanID: " parent-override ",
			Flavor:       evidence.FlavorWorkflow,
			Evidence:     &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:       &evidence.SourceMetadata{System: " argocd "},
		},
		PayloadOverride: &override,
	})
	if err != nil {
		t.Fatalf("Prescribe: %v", err)
	}
	if out.Entry.EntryID != "presc-override" {
		t.Fatalf("entry id = %q, want presc-override", out.Entry.EntryID)
	}
	if len(fakeStore.savedRaw) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(fakeStore.savedRaw))
	}

	entry := decodeStoredEntry(t, fakeStore.savedRaw[0])
	if entry.EntryID != "presc-override" {
		t.Fatalf("stored entry id = %q, want presc-override", entry.EntryID)
	}
	if entry.SpanID != "span-override" || entry.ParentSpanID != "parent-override" {
		t.Fatalf("entry span fields = span_id=%q parent_span_id=%q, want trimmed values", entry.SpanID, entry.ParentSpanID)
	}

	var payload evidence.PrescriptionPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("unmarshal prescribe payload: %v", err)
	}
	if payload.PrescriptionID != "presc-override" {
		t.Fatalf("payload prescription_id = %q, want presc-override", payload.PrescriptionID)
	}
	var actionPayload canon.CanonicalAction
	if err := json.Unmarshal(payload.CanonicalAction, &actionPayload); err != nil {
		t.Fatalf("unmarshal canonical action: %v", err)
	}
	if actionPayload.ScopeClass != "staging" {
		t.Fatalf("canonical action scope_class = %q, want staging", actionPayload.ScopeClass)
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
				Type:       " controller ",
				ID:         " argocd ",
				Provenance: " argocd ",
			},
			SessionID:       "session-1",
			OperationID:     "operation-1",
			TraceID:         "trace-1",
			SpanID:          " span-report ",
			ParentSpanID:    " parent-report ",
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
	if entry.Actor.Type != "controller" || entry.Actor.ID != "argocd" || entry.Actor.Provenance != "argocd" {
		t.Fatalf("entry actor = %+v, want trimmed fields", entry.Actor)
	}
	if entry.SpanID != "span-report" || entry.ParentSpanID != "parent-report" {
		t.Fatalf("entry span fields = span_id=%q parent_span_id=%q, want trimmed values", entry.SpanID, entry.ParentSpanID)
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

func TestServiceReport_RejectsSessionMismatch(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	fakeStore.entries["presc-mismatch"] = store.StoredEntry{
		ID:          "presc-mismatch",
		TenantID:    "tenant-1",
		EntryType:   string(evidence.EntryTypePrescribe),
		SessionID:   "session-prescription",
		OperationID: "operation-prescription",
	}
	svc := NewService(fakeStore, testutil.TestSigner(t))

	_, err := svc.Report(context.Background(), "tenant-1", ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-other",
			OperationID: "operation-other",
			TraceID:     "trace-other",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		PrescriptionID: "presc-mismatch",
		Verdict:        evidence.VerdictSuccess,
		ExitCode:       intPtr(0),
	})
	if err == nil {
		t.Fatal("expected session mismatch error")
	}
	if ErrorCode(err) != ErrCodeInvalidInput {
		t.Fatalf("error code = %q, want invalid_input", ErrorCode(err))
	}
	if !strings.Contains(err.Error(), "session_id") {
		t.Fatalf("error = %q, want session_id mismatch", err.Error())
	}
}

func TestServicePrescribe_RejectsInvalidCanonicalActionScope(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	svc := NewService(fakeStore, testutil.TestSigner(t))

	_, err := svc.Prescribe(context.Background(), "tenant-1", PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-invalid-scope",
			OperationID: "operation-invalid-scope",
			TraceID:     "trace-invalid-scope",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		CanonicalAction: &canon.CanonicalAction{
			Tool:           "kubectl",
			Operation:      "apply",
			OperationClass: "mutate",
			ScopeClass:     "moon",
		},
	})
	if err == nil {
		t.Fatal("expected invalid scope error")
	}
	if ErrorCode(err) != ErrCodeInvalidInput {
		t.Fatalf("error code = %q, want invalid_input", ErrorCode(err))
	}
	if !strings.Contains(err.Error(), "scope_class") {
		t.Fatalf("error = %q, want scope_class violation", err.Error())
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
	if first.EntryID == "" || second.EntryID == "" {
		t.Fatal("expected stable entry ids on duplicate retry")
	}
	if first.EntryID != second.EntryID {
		t.Fatalf("entry ids differ: first=%q second=%q", first.EntryID, second.EntryID)
	}
	if first.EffectiveRisk == "" || second.EffectiveRisk == "" {
		t.Fatal("expected effective risk on duplicate retry")
	}
	if first.EffectiveRisk != second.EffectiveRisk {
		t.Fatalf("effective risk differs: first=%q second=%q", first.EffectiveRisk, second.EffectiveRisk)
	}
	if len(first.Entry.Payload) == 0 || len(second.Entry.Payload) == 0 {
		t.Fatal("expected duplicate retry to return stored entry payload")
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

func TestServiceReport_PayloadOverrideResolvesPrescriptionAndBuildsPayload(t *testing.T) {
	t.Parallel()

	fakeStore := newFakeIngestStore()
	fakeStore.lastHash = "sha256:previous"
	fakeStore.entries["presc-override"] = store.StoredEntry{
		ID:        "presc-override",
		TenantID:  "tenant-1",
		EntryType: string(evidence.EntryTypePrescribe),
		SessionID: "session-override",
	}
	svc := NewService(fakeStore, testutil.TestSigner(t))
	tenantID := "tenant-1"

	override := json.RawMessage(`{
		"prescription_id":"presc-override",
		"verdict":"success",
		"exit_code":0,
		"decision_context":null,
		"external_refs":[{"type":"upstream","id":"ref-1"}]
	}`)

	out, err := svc.Report(context.Background(), tenantID, ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-override",
			OperationID: "operation-override",
			TraceID:     "trace-override",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		PayloadOverride: &override,
	})
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
	if payload.PrescriptionID != "presc-override" {
		t.Fatalf("payload prescription_id = %q, want presc-override", payload.PrescriptionID)
	}
	if payload.Verdict != evidence.VerdictSuccess {
		t.Fatalf("payload verdict = %q, want success", payload.Verdict)
	}
	if payload.ExitCode == nil || *payload.ExitCode != 0 {
		t.Fatalf("payload exit_code = %v, want 0", payload.ExitCode)
	}
	if len(payload.ExternalRefs) != 1 || payload.ExternalRefs[0].ID != "ref-1" {
		t.Fatalf("payload external_refs = %+v, want ref-1", payload.ExternalRefs)
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
	claimResults  map[string]store.WebhookEventResult
	savedRaw      []json.RawMessage
	getEntryCalls int
}

func newFakeIngestStore() *fakeIngestStore {
	return &fakeIngestStore{
		entries:      make(map[string]store.StoredEntry),
		claimed:      make(map[string]json.RawMessage),
		claimResults: make(map[string]store.WebhookEventResult),
	}
}

func (f *fakeIngestStore) LastHash(context.Context, string) (string, error) {
	return f.lastHash, nil
}

func (f *fakeIngestStore) BeginIngestTx(context.Context) (store.IngestTx, error) {
	tx := &fakeIngestTx{
		parent:       f,
		lastHash:     f.lastHash,
		entries:      cloneStoredEntries(f.entries),
		claimed:      cloneClaimedEntries(f.claimed),
		claimResults: cloneClaimResults(f.claimResults),
	}
	return tx, nil
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
		Payload:     append(json.RawMessage(nil), raw...),
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

type fakeIngestTx struct {
	parent       *fakeIngestStore
	lastHash     string
	entries      map[string]store.StoredEntry
	claimed      map[string]json.RawMessage
	claimResults map[string]store.WebhookEventResult
	savedRaw     []json.RawMessage
}

func (t *fakeIngestTx) LastHash(context.Context, string) (string, error) {
	return t.lastHash, nil
}

func (t *fakeIngestTx) SaveRaw(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	t.savedRaw = append(t.savedRaw, append(json.RawMessage(nil), raw...))
	var entry evidence.EvidenceEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return "", err
	}
	if entry.EntryID == "" {
		return "", errors.New("missing entry_id")
	}
	t.entries[entry.EntryID] = store.StoredEntry{
		ID:          entry.EntryID,
		EntryType:   string(entry.Type),
		SessionID:   entry.SessionID,
		OperationID: entry.OperationID,
		Payload:     append(json.RawMessage(nil), raw...),
	}
	t.lastHash = entry.Hash
	return entry.EntryID, nil
}

func (t *fakeIngestTx) GetEntry(_ context.Context, _ string, entryID string) (store.StoredEntry, error) {
	entry, ok := t.entries[entryID]
	if !ok {
		return store.StoredEntry{}, store.ErrNotFound
	}
	return entry, nil
}

func (t *fakeIngestTx) ClaimWebhookEvent(_ context.Context, tenantID, source, key string, payload json.RawMessage) (bool, error) {
	claimKey := tenantID + "|" + source + "|" + key
	if _, ok := t.claimed[claimKey]; ok {
		return true, nil
	}
	t.claimed[claimKey] = append(json.RawMessage(nil), payload...)
	return false, nil
}

func (t *fakeIngestTx) GetWebhookEventResult(_ context.Context, tenantID, source, key string) (store.WebhookEventResult, error) {
	claimKey := tenantID + "|" + source + "|" + key
	result, ok := t.claimResults[claimKey]
	if !ok {
		return store.WebhookEventResult{}, store.ErrNotFound
	}
	return result, nil
}

func (t *fakeIngestTx) FinalizeWebhookEvent(_ context.Context, tenantID, source, key, entryID, effectiveRisk string) error {
	claimKey := tenantID + "|" + source + "|" + key
	if _, ok := t.claimed[claimKey]; !ok {
		return store.ErrNotFound
	}
	t.claimResults[claimKey] = store.WebhookEventResult{EntryID: entryID, EffectiveRisk: effectiveRisk}
	return nil
}

func (t *fakeIngestTx) Commit(context.Context) error {
	if t.parent == nil {
		return nil
	}
	if t.parent.entries == nil {
		t.parent.entries = make(map[string]store.StoredEntry)
	}
	if t.parent.claimed == nil {
		t.parent.claimed = make(map[string]json.RawMessage)
	}
	if t.parent.claimResults == nil {
		t.parent.claimResults = make(map[string]store.WebhookEventResult)
	}
	t.parent.lastHash = t.lastHash
	for id, entry := range t.entries {
		t.parent.entries[id] = entry
	}
	for key, payload := range t.claimed {
		t.parent.claimed[key] = append(json.RawMessage(nil), payload...)
	}
	for key, result := range t.claimResults {
		t.parent.claimResults[key] = result
	}
	t.parent.savedRaw = append(t.parent.savedRaw, t.savedRaw...)
	return nil
}

func (t *fakeIngestTx) Rollback(context.Context) error {
	return nil
}

func cloneStoredEntries(src map[string]store.StoredEntry) map[string]store.StoredEntry {
	dst := make(map[string]store.StoredEntry, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneClaimedEntries(src map[string]json.RawMessage) map[string]json.RawMessage {
	dst := make(map[string]json.RawMessage, len(src))
	for k, v := range src {
		dst[k] = append(json.RawMessage(nil), v...)
	}
	return dst
}

func cloneClaimResults(src map[string]store.WebhookEventResult) map[string]store.WebhookEventResult {
	dst := make(map[string]store.WebhookEventResult, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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
