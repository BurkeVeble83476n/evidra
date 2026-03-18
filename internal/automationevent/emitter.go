package automationevent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/risk"
	"samebits.com/evidra/internal/store"
	"samebits.com/evidra/pkg/evidence"
	"samebits.com/evidra/pkg/version"
)

const (
	FlavorImperative = evidence.FlavorImperative
	FlavorReconcile  = evidence.FlavorReconcile
	FlavorWorkflow   = evidence.FlavorWorkflow
)

type EventStore interface {
	LastHash(ctx context.Context, tenantID string) (string, error)
	SaveRaw(ctx context.Context, tenantID string, raw json.RawMessage) (string, error)
	GetEntry(ctx context.Context, tenantID, entryID string) (store.StoredEntry, error)
	ClaimWebhookEvent(ctx context.Context, tenantID, source, key string, payload json.RawMessage) (bool, error)
	ReleaseWebhookEvent(ctx context.Context, tenantID, source, key string) error
}

type Emitter struct {
	store  EventStore
	signer evidence.Signer
}

type EmitResult struct {
	Duplicate bool
	Entry     evidence.EvidenceEntry
}

type MappedPrescribeInput struct {
	TenantID        string
	ClaimSource     string
	ClaimKey        string
	ClaimPayload    json.RawMessage
	Actor           evidence.Actor
	SessionID       string
	OperationID     string
	PrescriptionID  string
	Action          canon.CanonicalAction
	ArtifactDigest  string
	ScopeDimensions map[string]string
	Flavor          evidence.Flavor
	EvidenceKind    evidence.EvidenceKind
	SourceSystem    string
}

type MappedReportInput struct {
	TenantID        string
	ClaimSource     string
	ClaimKey        string
	ClaimPayload    json.RawMessage
	Actor           evidence.Actor
	SessionID       string
	OperationID     string
	PrescriptionID  string
	ArtifactDigest  string
	ScopeDimensions map[string]string
	Verdict         evidence.Verdict
	ExitCode        *int
	ExternalRefs    []evidence.ExternalRef
	Flavor          evidence.Flavor
	EvidenceKind    evidence.EvidenceKind
	SourceSystem    string
}

type ExplicitReportInput struct {
	TenantID        string
	ClaimSource     string
	ClaimKey        string
	ClaimPayload    json.RawMessage
	Actor           evidence.Actor
	SessionID       string
	OperationID     string
	TraceID         string
	PrescriptionID  string
	ArtifactDigest  string
	ScopeDimensions map[string]string
	Verdict         evidence.Verdict
	ExitCode        *int
	ExternalRefs    []evidence.ExternalRef
	Flavor          evidence.Flavor
	EvidenceKind    evidence.EvidenceKind
	SourceSystem    string
}

func NewEmitter(store EventStore, signer evidence.Signer) *Emitter {
	return &Emitter{store: store, signer: signer}
}

func (e *Emitter) EmitMappedPrescribe(ctx context.Context, in MappedPrescribeInput) (EmitResult, error) {
	return e.emit(ctx, in.TenantID, in.ClaimSource, in.ClaimKey, in.ClaimPayload, func(lastHash string) (evidence.EvidenceEntry, error) {
		return buildMappedPrescribeEntry(lastHash, e.signer, in)
	})
}

func (e *Emitter) EmitMappedReport(ctx context.Context, in MappedReportInput) (EmitResult, error) {
	return e.emit(ctx, in.TenantID, in.ClaimSource, in.ClaimKey, in.ClaimPayload, func(lastHash string) (evidence.EvidenceEntry, error) {
		return buildMappedReportEntry(lastHash, e.signer, in)
	})
}

func (e *Emitter) EmitExplicitReport(ctx context.Context, in ExplicitReportInput) (EmitResult, error) {
	return e.emit(ctx, in.TenantID, in.ClaimSource, in.ClaimKey, in.ClaimPayload, func(lastHash string) (evidence.EvidenceEntry, error) {
		prescriptionID := strings.TrimSpace(in.PrescriptionID)
		if prescriptionID == "" {
			return evidence.EvidenceEntry{}, fmt.Errorf("prescription_id is required")
		}

		prescription, err := e.store.GetEntry(ctx, in.TenantID, prescriptionID)
		if err != nil {
			return evidence.EvidenceEntry{}, err
		}
		if prescription.EntryType != string(evidence.EntryTypePrescribe) {
			return evidence.EvidenceEntry{}, fmt.Errorf("entry %q is not a prescribe entry", prescriptionID)
		}

		sessionID := strings.TrimSpace(in.SessionID)
		if sessionID == "" {
			sessionID = strings.TrimSpace(prescription.SessionID)
		}
		operationID := strings.TrimSpace(in.OperationID)
		if operationID == "" {
			operationID = strings.TrimSpace(prescription.OperationID)
		}
		traceID := strings.TrimSpace(in.TraceID)
		if traceID == "" {
			traceID = sessionID
		}
		if traceID == "" {
			traceID = prescriptionID
		}

		return buildReportEntry(lastHash, e.signer, reportBuildInput{
			SessionID:       sessionID,
			OperationID:     operationID,
			TraceID:         traceID,
			Actor:           in.Actor,
			PrescriptionID:  prescriptionID,
			ArtifactDigest:  in.ArtifactDigest,
			ScopeDimensions: in.ScopeDimensions,
			Verdict:         in.Verdict,
			ExitCode:        in.ExitCode,
			ExternalRefs:    in.ExternalRefs,
			Flavor:          in.Flavor,
			EvidenceKind:    in.EvidenceKind,
			SourceSystem:    in.SourceSystem,
			CanonVersion:    "mapped/v1",
		})
	})
}

func (e *Emitter) emit(
	ctx context.Context,
	tenantID, source, key string,
	payload json.RawMessage,
	build func(lastHash string) (evidence.EvidenceEntry, error),
) (EmitResult, error) {
	release := func() {}
	if strings.TrimSpace(source) != "" && strings.TrimSpace(key) != "" {
		duplicate, claimRelease, err := claimEvent(ctx, e.store, tenantID, source, key, payload)
		if err != nil {
			return EmitResult{}, err
		}
		release = claimRelease
		if duplicate {
			return EmitResult{Duplicate: true}, nil
		}
	}

	success := false
	defer func() {
		if !success {
			release()
		}
	}()

	lastHash, err := e.store.LastHash(ctx, tenantID)
	if err != nil {
		return EmitResult{}, err
	}

	entry, err := build(lastHash)
	if err != nil {
		return EmitResult{}, err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return EmitResult{}, err
	}
	if _, err := e.store.SaveRaw(ctx, tenantID, raw); err != nil {
		return EmitResult{}, err
	}

	success = true
	return EmitResult{Entry: entry}, nil
}

type reportBuildInput struct {
	SessionID       string
	OperationID     string
	TraceID         string
	Actor           evidence.Actor
	PrescriptionID  string
	ArtifactDigest  string
	ScopeDimensions map[string]string
	Verdict         evidence.Verdict
	ExitCode        *int
	ExternalRefs    []evidence.ExternalRef
	Flavor          evidence.Flavor
	EvidenceKind    evidence.EvidenceKind
	SourceSystem    string
	CanonVersion    string
}

func buildMappedPrescribeEntry(lastHash string, signer evidence.Signer, in MappedPrescribeInput) (evidence.EvidenceEntry, error) {
	rawAction, err := json.Marshal(in.Action)
	if err != nil {
		return evidence.EvidenceEntry{}, err
	}
	riskLevel := risk.ElevateRiskLevel(risk.RiskLevel(in.Action.OperationClass, in.Action.ScopeClass), nil)
	payload, err := json.Marshal(evidence.PrescriptionPayload{
		PrescriptionID:  in.PrescriptionID,
		CanonicalAction: rawAction,
		RiskInputs: []evidence.RiskInput{
			{
				Source:    "evidra/matrix",
				RiskLevel: riskLevel,
			},
		},
		EffectiveRisk: riskLevel,
		RiskLevel:     riskLevel,
		TTLMs:         evidence.DefaultTTLMs,
		CanonSource:   "mapped",
		Flavor:        in.Flavor,
		Evidence:      payloadEvidenceMetadata(in.EvidenceKind),
		Source:        payloadSourceMetadata(in.SourceSystem),
	})
	if err != nil {
		return evidence.EvidenceEntry{}, err
	}

	traceID := strings.TrimSpace(in.SessionID)
	if traceID == "" {
		traceID = strings.TrimSpace(in.PrescriptionID)
	}

	return evidence.BuildEntry(evidence.EntryBuildParams{
		EntryID:         in.PrescriptionID,
		Type:            evidence.EntryTypePrescribe,
		SessionID:       in.SessionID,
		OperationID:     in.OperationID,
		TraceID:         traceID,
		Actor:           in.Actor,
		IntentDigest:    canon.ComputeIntentDigest(in.Action),
		ArtifactDigest:  in.ArtifactDigest,
		Payload:         payload,
		PreviousHash:    lastHash,
		ScopeDimensions: in.ScopeDimensions,
		SpecVersion:     version.SpecVersion,
		CanonVersion:    "mapped/v1",
		AdapterVersion:  version.Version,
		ScoringVersion:  version.ScoringVersion,
		Signer:          signer,
	})
}

func buildMappedReportEntry(lastHash string, signer evidence.Signer, in MappedReportInput) (evidence.EvidenceEntry, error) {
	traceID := strings.TrimSpace(in.SessionID)
	if traceID == "" {
		traceID = strings.TrimSpace(in.PrescriptionID)
	}
	return buildReportEntry(lastHash, signer, reportBuildInput{
		SessionID:       in.SessionID,
		OperationID:     in.OperationID,
		TraceID:         traceID,
		Actor:           in.Actor,
		PrescriptionID:  in.PrescriptionID,
		ArtifactDigest:  in.ArtifactDigest,
		ScopeDimensions: in.ScopeDimensions,
		Verdict:         in.Verdict,
		ExitCode:        in.ExitCode,
		ExternalRefs:    in.ExternalRefs,
		Flavor:          in.Flavor,
		EvidenceKind:    in.EvidenceKind,
		SourceSystem:    in.SourceSystem,
		CanonVersion:    "mapped/v1",
	})
}

func buildReportEntry(lastHash string, signer evidence.Signer, in reportBuildInput) (evidence.EvidenceEntry, error) {
	if !in.Verdict.Valid() {
		return evidence.EvidenceEntry{}, fmt.Errorf("invalid verdict %q", in.Verdict)
	}

	payload, err := json.Marshal(evidence.ReportPayload{
		ReportID:       ulid.Make().String(),
		PrescriptionID: in.PrescriptionID,
		ExitCode:       in.ExitCode,
		Verdict:        in.Verdict,
		ExternalRefs:   in.ExternalRefs,
		Flavor:         in.Flavor,
		Evidence:       payloadEvidenceMetadata(in.EvidenceKind),
		Source:         payloadSourceMetadata(in.SourceSystem),
	})
	if err != nil {
		return evidence.EvidenceEntry{}, err
	}

	return evidence.BuildEntry(evidence.EntryBuildParams{
		Type:            evidence.EntryTypeReport,
		SessionID:       in.SessionID,
		OperationID:     in.OperationID,
		TraceID:         in.TraceID,
		Actor:           in.Actor,
		ArtifactDigest:  in.ArtifactDigest,
		Payload:         payload,
		PreviousHash:    lastHash,
		ScopeDimensions: in.ScopeDimensions,
		SpecVersion:     version.SpecVersion,
		CanonVersion:    in.CanonVersion,
		AdapterVersion:  version.Version,
		ScoringVersion:  version.ScoringVersion,
		Signer:          signer,
	})
}

func claimEvent(ctx context.Context, store EventStore, tenantID, source, key string, payload json.RawMessage) (bool, func(), error) {
	duplicate, err := store.ClaimWebhookEvent(ctx, tenantID, source, key, payload)
	if err != nil {
		return false, nil, err
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		_ = store.ReleaseWebhookEvent(ctx, tenantID, source, key)
	}
	if duplicate {
		released = true
		return true, release, nil
	}
	return false, release, nil
}

func payloadEvidenceMetadata(kind evidence.EvidenceKind) *evidence.EvidenceMetadata {
	if strings.TrimSpace(string(kind)) == "" {
		return nil
	}
	return &evidence.EvidenceMetadata{Kind: kind}
}

func payloadSourceMetadata(system string) *evidence.SourceMetadata {
	system = strings.TrimSpace(system)
	if system == "" {
		return nil
	}
	return &evidence.SourceMetadata{System: system}
}
