package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/risk"
	"samebits.com/evidra/internal/store"
	"samebits.com/evidra/pkg/evidence"
	"samebits.com/evidra/pkg/version"
)

// Store is the minimal persistence dependency required by the ingest service.
type Store interface {
	LastHash(ctx context.Context, tenantID string) (string, error)
	SaveRaw(ctx context.Context, tenantID string, raw json.RawMessage) (string, error)
	GetEntry(ctx context.Context, tenantID, entryID string) (store.StoredEntry, error)
	ClaimWebhookEvent(ctx context.Context, tenantID, source, key string, payload json.RawMessage) (bool, error)
	ReleaseWebhookEvent(ctx context.Context, tenantID, source, key string) error
}

// Result captures the stable ingest response shape.
type Result struct {
	Duplicate     bool
	EntryID       string
	EffectiveRisk string
	Entry         evidence.EvidenceEntry
}

// Service owns server-side external lifecycle ingest.
type Service struct {
	store  Store
	signer evidence.Signer
}

// NewService creates an ingest service.
func NewService(store Store, signer evidence.Signer) *Service {
	return &Service{store: store, signer: signer}
}

// Prescribe builds, signs, and persists a prescribe entry.
func (s *Service) Prescribe(ctx context.Context, tenantID string, in PrescribeRequest) (Result, error) {
	if err := requiredSigner(s.signer); err != nil {
		return Result{}, err
	}
	if err := ValidatePrescribeRequest(in); err != nil {
		return Result{}, wrapError(ErrCodeInvalidInput, err.Error(), err)
	}

	release, duplicate, err := s.claimIfNeeded(ctx, tenantID, in.Claim)
	if err != nil {
		return Result{}, err
	}
	if duplicate {
		return Result{Duplicate: true}, nil
	}

	success := false
	defer func() {
		if !success {
			release()
		}
	}()

	var (
		effectiveRisk string
		entry         evidence.EvidenceEntry
	)
	entry, err = s.saveEntry(ctx, tenantID, func(lastHash string) (evidence.EvidenceEntry, error) {
		var buildErr error
		entry, effectiveRisk, buildErr = buildPrescribeEntry(lastHash, s.signer, in)
		return entry, buildErr
	})
	if err != nil {
		return Result{}, err
	}

	success = true
	return Result{
		EntryID:       entry.EntryID,
		EffectiveRisk: effectiveRisk,
		Entry:         entry,
	}, nil
}

// Report builds, signs, and persists a report entry after resolving the prescription.
func (s *Service) Report(ctx context.Context, tenantID string, in ReportRequest) (Result, error) {
	if err := requiredSigner(s.signer); err != nil {
		return Result{}, err
	}
	if err := ValidateReportRequest(in); err != nil {
		return Result{}, wrapError(ErrCodeInvalidInput, err.Error(), err)
	}

	release, duplicate, err := s.claimIfNeeded(ctx, tenantID, in.Claim)
	if err != nil {
		return Result{}, err
	}
	if duplicate {
		return Result{Duplicate: true}, nil
	}

	success := false
	defer func() {
		if !success {
			release()
		}
	}()

	reportPayload, prescriptionID, err := buildReportPayloadState(in)
	if err != nil {
		return Result{}, err
	}

	prescription, err := s.loadPrescription(ctx, tenantID, prescriptionID)
	if err != nil {
		return Result{}, err
	}

	entry, err := s.saveEntry(ctx, tenantID, func(lastHash string) (evidence.EvidenceEntry, error) {
		return buildReportEntry(lastHash, s.signer, in, reportPayload, prescription)
	})
	if err != nil {
		return Result{}, err
	}

	success = true
	return Result{
		EntryID: entry.EntryID,
		Entry:   entry,
	}, nil
}

func (s *Service) loadPrescription(ctx context.Context, tenantID, prescriptionID string) (store.StoredEntry, error) {
	entry, err := s.store.GetEntry(ctx, tenantID, prescriptionID)
	if err != nil {
		if errorsIsNotFound(err) {
			return store.StoredEntry{}, wrapError(ErrCodeNotFound, "prescription_id not found", err)
		}
		return store.StoredEntry{}, wrapError(ErrCodeInternal, "failed to read prescription entry", err)
	}
	if entry.EntryType != string(evidence.EntryTypePrescribe) {
		return store.StoredEntry{}, wrapError(ErrCodeInvalidInput, fmt.Sprintf("entry %q is not a prescribe entry", prescriptionID), nil)
	}
	return entry, nil
}

func buildPrescribeEntry(lastHash string, signer evidence.Signer, in PrescribeRequest) (evidence.EvidenceEntry, string, error) {
	entryID := ulid.Make().String()
	payload, action, canonVersion, effectiveRisk, err := buildPrescribePayload(entryID, in)
	if err != nil {
		return evidence.EvidenceEntry{}, "", err
	}

	intentDigest := canon.ComputeIntentDigest(action)
	entry, err := evidence.BuildEntry(evidence.EntryBuildParams{
		EntryID:         entryID,
		Type:            evidence.EntryTypePrescribe,
		SessionID:       strings.TrimSpace(in.SessionID),
		OperationID:     strings.TrimSpace(in.OperationID),
		TraceID:         strings.TrimSpace(in.TraceID),
		Actor:           in.Actor,
		IntentDigest:    intentDigest,
		Payload:         payload,
		PreviousHash:    lastHash,
		ScopeDimensions: in.ScopeDimensions,
		SpecVersion:     version.SpecVersion,
		CanonVersion:    canonVersion,
		AdapterVersion:  version.Version,
		ScoringVersion:  version.ScoringVersion,
		Signer:          signer,
	})
	if err != nil {
		return evidence.EvidenceEntry{}, "", wrapError(ErrCodeInternal, err.Error(), err)
	}
	return entry, effectiveRisk, nil
}

func (s *Service) saveEntry(ctx context.Context, tenantID string, build func(lastHash string) (evidence.EvidenceEntry, error)) (evidence.EvidenceEntry, error) {
	lastHash, err := s.store.LastHash(ctx, tenantID)
	if err != nil {
		return evidence.EvidenceEntry{}, wrapError(ErrCodeInternal, "failed to read last hash", err)
	}

	entry, err := build(lastHash)
	if err != nil {
		return evidence.EvidenceEntry{}, err
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return evidence.EvidenceEntry{}, wrapError(ErrCodeInternal, "failed to marshal evidence entry", err)
	}
	if _, err := s.store.SaveRaw(ctx, tenantID, raw); err != nil {
		return evidence.EvidenceEntry{}, wrapError(ErrCodeInternal, "failed to store evidence entry", err)
	}
	return entry, nil
}

func buildReportEntry(lastHash string, signer evidence.Signer, in ReportRequest, payload evidence.ReportPayload, prescription store.StoredEntry) (evidence.EvidenceEntry, error) {
	entryID := ulid.Make().String()
	payload.ReportID = entryID

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
		traceID = prescription.ID
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return evidence.EvidenceEntry{}, wrapError(ErrCodeInternal, "failed to marshal report payload", err)
	}

	entry, err := evidence.BuildEntry(evidence.EntryBuildParams{
		EntryID:         entryID,
		Type:            evidence.EntryTypeReport,
		SessionID:       sessionID,
		OperationID:     operationID,
		TraceID:         traceID,
		Actor:           in.Actor,
		ArtifactDigest:  strings.TrimSpace(in.ArtifactDigest),
		Payload:         rawPayload,
		PreviousHash:    lastHash,
		ScopeDimensions: in.ScopeDimensions,
		SpecVersion:     version.SpecVersion,
		CanonVersion:    "external/v1",
		AdapterVersion:  version.Version,
		ScoringVersion:  version.ScoringVersion,
		Signer:          signer,
	})
	if err != nil {
		return evidence.EvidenceEntry{}, wrapError(ErrCodeInternal, err.Error(), err)
	}
	return entry, nil
}

func buildPrescribePayload(entryID string, in PrescribeRequest) (json.RawMessage, canon.CanonicalAction, string, string, error) {
	var (
		payload      evidence.PrescriptionPayload
		action       canon.CanonicalAction
		canonVersion string
	)

	switch {
	case in.PayloadOverride != nil:
		if err := json.Unmarshal(*in.PayloadOverride, &payload); err != nil {
			return nil, canon.CanonicalAction{}, "", "", wrapError(ErrCodeInvalidInput, "payload_override must be valid prescribe payload JSON", err)
		}
		if len(payload.CanonicalAction) == 0 {
			return nil, canon.CanonicalAction{}, "", "", wrapError(ErrCodeInvalidInput, "payload_override must include canonical_action", nil)
		}
		if err := json.Unmarshal(payload.CanonicalAction, &action); err != nil {
			return nil, canon.CanonicalAction{}, "", "", wrapError(ErrCodeInvalidInput, "payload_override canonical_action is invalid", err)
		}
		canonVersion = "external/v1"
	case in.CanonicalAction != nil:
		action = normalizeExplicitCanonicalAction(*in.CanonicalAction)
		canonVersion = "external/v1"
	case in.SmartTarget != nil:
		action = buildSmartTargetCanonicalAction(*in.SmartTarget)
		canonVersion = "external/v1"
	default:
		return nil, canon.CanonicalAction{}, "", "", wrapError(ErrCodeInvalidInput, "canonical_action or smart_target is required", nil)
	}

	riskInputs, effectiveRiskValue := buildPrescribeRiskInputs(action)
	if payload.PrescriptionID == "" {
		payload.PrescriptionID = entryID
	}
	if len(payload.CanonicalAction) == 0 {
		rawAction, err := json.Marshal(action)
		if err != nil {
			return nil, canon.CanonicalAction{}, "", "", wrapError(ErrCodeInternal, "failed to marshal canonical action", err)
		}
		payload.CanonicalAction = rawAction
	}
	if len(payload.RiskInputs) == 0 {
		payload.RiskInputs = riskInputs
	}
	if payload.EffectiveRisk == "" {
		payload.EffectiveRisk = effectiveRiskValue
	}
	if payload.TTLMs == 0 {
		payload.TTLMs = evidence.DefaultTTLMs
	}
	if payload.CanonSource == "" {
		payload.CanonSource = "external"
	}
	payload.Flavor = in.Flavor
	payload.Evidence = payloadEvidenceMetadata(in.Evidence)
	payload.Source = payloadSourceMetadata(in.Source)

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, canon.CanonicalAction{}, "", "", wrapError(ErrCodeInternal, "failed to marshal prescribe payload", err)
	}
	return rawPayload, action, canonVersion, payload.EffectiveRisk, nil
}

func buildReportPayloadState(in ReportRequest) (evidence.ReportPayload, string, error) {
	if in.PayloadOverride != nil {
		return parseReportPayloadOverride(*in.PayloadOverride, in.Envelope)
	}

	payload := evidence.ReportPayload{
		PrescriptionID:  strings.TrimSpace(in.PrescriptionID),
		ExitCode:        in.ExitCode,
		Verdict:         in.Verdict,
		DecisionContext: in.DecisionContext,
		ExternalRefs:    in.ExternalRefs,
		Flavor:          in.Flavor,
		Evidence:        payloadEvidenceMetadata(in.Evidence),
		Source:          payloadSourceMetadata(in.Source),
	}
	return payload, payload.PrescriptionID, nil
}

func parseReportPayloadOverride(raw json.RawMessage, env Envelope) (evidence.ReportPayload, string, error) {
	var payload evidence.ReportPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return evidence.ReportPayload{}, "", wrapError(ErrCodeInvalidInput, "payload_override must be valid report payload JSON", err)
	}
	if err := validateReportPayloadBody(payload); err != nil {
		return evidence.ReportPayload{}, "", err
	}

	payload.Flavor = env.Flavor
	payload.Evidence = payloadEvidenceMetadata(env.Evidence)
	payload.Source = payloadSourceMetadata(env.Source)

	return payload, strings.TrimSpace(payload.PrescriptionID), nil
}

func validateReportPayloadBody(payload evidence.ReportPayload) error {
	if strings.TrimSpace(payload.PrescriptionID) == "" {
		return wrapError(ErrCodeInvalidInput, "payload_override prescription_id is required", nil)
	}
	if !payload.Verdict.Valid() {
		return wrapError(ErrCodeInvalidInput, "payload_override verdict is required and must be one of success, failure, error, declined", nil)
	}
	if payload.Verdict == evidence.VerdictDeclined {
		if payload.ExitCode != nil {
			return wrapError(ErrCodeInvalidInput, "declined reports must not include exit_code", nil)
		}
		if payload.DecisionContext == nil {
			return wrapError(ErrCodeInvalidInput, "decision_context is required for declined reports", nil)
		}
		if strings.TrimSpace(payload.DecisionContext.Trigger) == "" {
			return wrapError(ErrCodeInvalidInput, "decision_context.trigger is required", nil)
		}
		if strings.TrimSpace(payload.DecisionContext.Reason) == "" {
			return wrapError(ErrCodeInvalidInput, "decision_context.reason is required", nil)
		}
		if len(strings.TrimSpace(payload.DecisionContext.Reason)) > 512 {
			return wrapError(ErrCodeInvalidInput, "decision_context.reason exceeds 512 characters", nil)
		}
		return nil
	}
	if payload.DecisionContext != nil {
		return wrapError(ErrCodeInvalidInput, "decision_context is only valid for declined reports", nil)
	}
	if payload.ExitCode == nil {
		return wrapError(ErrCodeInvalidInput, fmt.Sprintf("report verdict %s requires exit_code", payload.Verdict), nil)
	}
	if inferred := evidence.VerdictFromExitCode(*payload.ExitCode); inferred != payload.Verdict {
		return wrapError(ErrCodeInvalidInput, fmt.Sprintf("report verdict %s does not match exit_code %d", payload.Verdict, *payload.ExitCode), nil)
	}
	return nil
}

func buildPrescribeRiskInputs(action canon.CanonicalAction) ([]evidence.RiskInput, string) {
	matrixLevel := risk.RiskLevel(action.OperationClass, action.ScopeClass)
	riskInputs := []evidence.RiskInput{
		{
			Source:    "evidra/matrix",
			RiskLevel: matrixLevel,
		},
	}
	return riskInputs, risk.ElevateRiskLevel(matrixLevel, nil)
}

func buildSmartTargetCanonicalAction(target SmartTarget) canon.CanonicalAction {
	resource := strings.TrimSpace(target.Resource)
	identity := []canon.ResourceID{{
		Name:      resource,
		Namespace: strings.TrimSpace(target.Namespace),
	}}

	scopeClass := canon.ResolveScopeClass(strings.TrimSpace(target.Namespace), identity)
	return canon.CanonicalAction{
		Tool:              strings.TrimSpace(target.Tool),
		Operation:         strings.TrimSpace(target.Operation),
		OperationClass:    inferOperationClass(target.Operation),
		ResourceIdentity:  identity,
		ScopeClass:        scopeClass,
		ResourceCount:     1,
		ResourceShapeHash: canon.SHA256Hex([]byte(strings.Join([]string{target.Tool, target.Operation, target.Resource, target.Namespace}, "|"))),
	}
}

func normalizeExplicitCanonicalAction(action canon.CanonicalAction) canon.CanonicalAction {
	action.Tool = strings.TrimSpace(action.Tool)
	action.Operation = strings.TrimSpace(action.Operation)
	action.OperationClass = strings.TrimSpace(action.OperationClass)
	action.ScopeClass = strings.TrimSpace(action.ScopeClass)
	action.ResourceShapeHash = strings.TrimSpace(action.ResourceShapeHash)
	return action
}

func inferOperationClass(operation string) string {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "delete", "destroy", "remove", "rm", "uninstall":
		return "destroy"
	case "plan", "validate", "diff", "show", "get", "describe", "logs", "top":
		return "read"
	default:
		return "mutate"
	}
}

func payloadEvidenceMetadata(meta *EvidenceMetadata) *evidence.EvidenceMetadata {
	if meta == nil || strings.TrimSpace(string(meta.Kind)) == "" {
		return nil
	}
	return &evidence.EvidenceMetadata{Kind: meta.Kind}
}

func payloadSourceMetadata(meta *SourceMetadata) *evidence.SourceMetadata {
	if meta == nil {
		return nil
	}
	system := strings.TrimSpace(meta.System)
	if system == "" {
		return nil
	}
	return &evidence.SourceMetadata{System: system}
}

func claimEvent(ctx context.Context, store Store, tenantID, source, key string, payload json.RawMessage) (bool, func(), error) {
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

func (s *Service) claimIfNeeded(ctx context.Context, tenantID string, claim *Claim) (func(), bool, error) {
	if !hasMeaningfulClaim(claim) {
		return func() {}, false, nil
	}

	duplicate, release, err := claimEvent(ctx, s.store, tenantID, claim.Source, claim.Key, claim.Payload)
	if err != nil {
		return nil, false, wrapError(ErrCodeInternal, "failed to claim ingest event", err)
	}
	if duplicate {
		release()
		return func() {}, true, nil
	}
	return release, false, nil
}

func hasMeaningfulClaim(claim *Claim) bool {
	return claim != nil && (strings.TrimSpace(claim.Source) != "" || strings.TrimSpace(claim.Key) != "" || len(claim.Payload) > 0)
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound)
}
