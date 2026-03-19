package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/pkg/evidence"
)

const ContractVersionV1 = "v1"

// Claim identifies the upstream event or artifact that the ingest request is
// claiming for idempotency and audit purposes.
type Claim struct {
	Source  string          `json:"source"`
	Key     string          `json:"key"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SmartTarget captures a lightweight intent form that can be normalized into
// a canonical action by the ingest layer.
type SmartTarget struct {
	Tool      string `json:"tool"`
	Operation string `json:"operation"`
	Resource  string `json:"resource"`
	Namespace string `json:"namespace,omitempty"`
}

// Envelope carries the common metadata required by both prescribe and report
// ingest requests.
type Envelope struct {
	ContractVersion string            `json:"contract_version"`
	Claim           *Claim            `json:"claim,omitempty"`
	Actor           evidence.Actor    `json:"actor"`
	SessionID       string            `json:"session_id"`
	OperationID     string            `json:"operation_id"`
	TraceID         string            `json:"trace_id"`
	SpanID          string            `json:"span_id,omitempty"`
	ParentSpanID    string            `json:"parent_span_id,omitempty"`
	ScopeDimensions map[string]string `json:"scope_dimensions,omitempty"`
	Flavor          evidence.Flavor   `json:"flavor"`
	Evidence        *EvidenceMetadata `json:"evidence,omitempty"`
	Source          *SourceMetadata   `json:"source,omitempty"`
}

// EvidenceMetadata mirrors the lifecycle taxonomy for how the evidence was
// obtained.
type EvidenceMetadata = evidence.EvidenceMetadata

// SourceMetadata mirrors the lifecycle taxonomy for the producing system.
type SourceMetadata = evidence.SourceMetadata

// PrescribeRequest is the external contract for server-side prescribe ingest.
type PrescribeRequest struct {
	Envelope
	PrescriptionID  string                 `json:"prescription_id,omitempty"`
	ArtifactDigest  string                 `json:"artifact_digest,omitempty"`
	CanonicalAction *canon.CanonicalAction `json:"canonical_action,omitempty"`
	SmartTarget     *SmartTarget           `json:"smart_target,omitempty"`
	PayloadOverride *json.RawMessage       `json:"payload_override,omitempty"`
}

// ReportRequest is the external contract for server-side report ingest.
type ReportRequest struct {
	Envelope
	PrescriptionID  string                    `json:"prescription_id,omitempty"`
	ArtifactDigest  string                    `json:"artifact_digest,omitempty"`
	Verdict         evidence.Verdict          `json:"verdict,omitempty"`
	ExitCode        *int                      `json:"exit_code,omitempty"`
	DecisionContext *evidence.DecisionContext `json:"decision_context,omitempty"`
	ExternalRefs    []evidence.ExternalRef    `json:"external_refs,omitempty"`
	PayloadOverride *json.RawMessage          `json:"payload_override,omitempty"`
}

// ValidationError captures one or more contract violations.
type ValidationError struct {
	Violations []string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("ingest contract validation failed: %s", strings.Join(e.Violations, "; "))
}

func (e *ValidationError) Add(violation string) {
	if e == nil {
		return
	}
	e.Violations = append(e.Violations, violation)
}

// ValidatePrescribeRequest validates the prescribe ingest contract.
func ValidatePrescribeRequest(in PrescribeRequest) error {
	var violations ValidationError

	validateEnvelope(&violations, in.Envelope)
	if in.PayloadOverride != nil {
		if !hasRawJSON(*in.PayloadOverride) {
			violations.Add("payload_override must not be empty")
		}
		if strings.TrimSpace(in.PrescriptionID) != "" {
			violations.Add("payload_override is mutually exclusive with prescription_id")
		}
		if strings.TrimSpace(in.ArtifactDigest) != "" {
			violations.Add("payload_override is mutually exclusive with artifact_digest")
		}
		if in.CanonicalAction != nil || in.SmartTarget != nil {
			violations.Add("payload_override is mutually exclusive with explicit prescribe fields")
		}
	} else {
		validatePrescribeIntent(&violations, in.CanonicalAction, in.SmartTarget)
	}

	return violationsOrNil(&violations)
}

// ValidateReportRequest validates the report ingest contract.
func ValidateReportRequest(in ReportRequest) error {
	var violations ValidationError

	validateEnvelope(&violations, in.Envelope)
	if in.PayloadOverride != nil {
		if !hasRawJSON(*in.PayloadOverride) {
			violations.Add("payload_override must not be empty")
		}
		if strings.TrimSpace(in.PrescriptionID) != "" || strings.TrimSpace(in.ArtifactDigest) != "" || strings.TrimSpace(string(in.Verdict)) != "" || in.ExitCode != nil || in.DecisionContext != nil || len(in.ExternalRefs) > 0 {
			violations.Add("payload_override is mutually exclusive with explicit report fields")
		}
	} else {
		if strings.TrimSpace(in.PrescriptionID) == "" {
			violations.Add("prescription_id is required")
		}
		if !in.Verdict.Valid() {
			violations.Add("verdict is required and must be one of success, failure, error, declined")
		} else {
			validateReportVerdict(&violations, in.Verdict, in.ExitCode, in.DecisionContext)
		}
	}

	return violationsOrNil(&violations)
}

func validateEnvelope(violations *ValidationError, env Envelope) {
	if strings.TrimSpace(env.ContractVersion) == "" {
		violations.Add("contract_version is required")
	} else if strings.TrimSpace(env.ContractVersion) != ContractVersionV1 {
		violations.Add("contract_version must be v1")
	}

	validateClaim(violations, env.Claim)
	validateActor(violations, env.Actor)

	if strings.TrimSpace(env.SessionID) == "" {
		violations.Add("session_id is required")
	}
	if strings.TrimSpace(env.OperationID) == "" {
		violations.Add("operation_id is required")
	}
	if strings.TrimSpace(env.TraceID) == "" {
		violations.Add("trace_id is required")
	}
	validateFlavor(violations, env.Flavor)
	if env.Evidence == nil {
		violations.Add("evidence.kind is required")
	} else {
		validateEvidenceKind(violations, env.Evidence.Kind)
	}
	if env.Source == nil || strings.TrimSpace(env.Source.System) == "" {
		violations.Add("source.system is required")
	}
}

func validateClaim(violations *ValidationError, claim *Claim) {
	if claim == nil {
		return
	}
	if strings.TrimSpace(claim.Source) == "" {
		violations.Add("claim.source is required")
	}
	if strings.TrimSpace(claim.Key) == "" {
		violations.Add("claim.key is required")
	}
}

func validatePrescribeIntent(violations *ValidationError, canonicalAction *canon.CanonicalAction, smartTarget *SmartTarget) {
	switch {
	case canonicalAction != nil && smartTarget != nil:
		violations.Add("canonical_action and smart_target are mutually exclusive")
	case canonicalAction != nil:
		validateCanonicalAction(violations, canonicalAction)
		return
	case smartTarget != nil:
		validateSmartTarget(violations, smartTarget)
	default:
		violations.Add("canonical_action or smart_target is required")
	}
}

func validateSmartTarget(violations *ValidationError, target *SmartTarget) {
	if target == nil {
		return
	}
	if strings.TrimSpace(target.Tool) == "" {
		violations.Add("smart_target.tool is required")
	}
	if strings.TrimSpace(target.Operation) == "" {
		violations.Add("smart_target.operation is required")
	}
	if strings.TrimSpace(target.Resource) == "" {
		violations.Add("smart_target.resource is required")
	}
}

func validateCanonicalAction(violations *ValidationError, action *canon.CanonicalAction) {
	if action == nil {
		violations.Add("canonical_action is required")
		return
	}
	if strings.TrimSpace(action.Tool) == "" {
		violations.Add("canonical_action.tool is required")
	}
	if strings.TrimSpace(action.Operation) == "" {
		violations.Add("canonical_action.operation is required")
	}
	if strings.TrimSpace(action.Tool) == "" &&
		strings.TrimSpace(action.Operation) == "" &&
		strings.TrimSpace(action.OperationClass) == "" &&
		strings.TrimSpace(action.ScopeClass) == "" &&
		action.ResourceCount == 0 &&
		strings.TrimSpace(action.ResourceShapeHash) == "" &&
		len(action.ResourceIdentity) == 0 {
		violations.Add("canonical_action must not be empty")
	}
}

func validateActor(violations *ValidationError, actor evidence.Actor) {
	if strings.TrimSpace(actor.Type) == "" {
		violations.Add("actor.type is required")
	}
	if strings.TrimSpace(actor.ID) == "" {
		violations.Add("actor.id is required")
	}
	if strings.TrimSpace(actor.Provenance) == "" {
		violations.Add("actor.provenance is required")
	}
}

func validateReportVerdict(violations *ValidationError, verdict evidence.Verdict, exitCode *int, decisionContext *evidence.DecisionContext) {
	switch verdict {
	case evidence.VerdictDeclined:
		if exitCode != nil {
			violations.Add("declined reports must not include exit_code")
		}
		if decisionContext == nil {
			violations.Add("decision_context is required for declined reports")
		} else {
			if strings.TrimSpace(decisionContext.Trigger) == "" {
				violations.Add("decision_context.trigger is required")
			}
			if strings.TrimSpace(decisionContext.Reason) == "" {
				violations.Add("decision_context.reason is required")
			} else if len(strings.TrimSpace(decisionContext.Reason)) > 512 {
				violations.Add("decision_context.reason exceeds 512 characters")
			}
		}
	default:
		if decisionContext != nil {
			violations.Add("decision_context is only valid for declined reports")
		}
		if exitCode == nil {
			violations.Add(fmt.Sprintf("report verdict %s requires exit_code", verdict))
			return
		}
		if inferred := evidence.VerdictFromExitCode(*exitCode); inferred != verdict {
			violations.Add(fmt.Sprintf("report verdict %s does not match exit_code %d", verdict, *exitCode))
		}
	}
}

func validateFlavor(violations *ValidationError, flavor evidence.Flavor) {
	switch flavor {
	case evidence.FlavorImperative, evidence.FlavorReconcile, evidence.FlavorWorkflow:
		return
	case "":
		violations.Add("flavor is required")
	default:
		violations.Add("flavor must be one of imperative, reconcile, workflow")
	}
}

func validateEvidenceKind(violations *ValidationError, kind evidence.EvidenceKind) {
	switch kind {
	case evidence.EvidenceKindDeclared, evidence.EvidenceKindObserved, evidence.EvidenceKindTranslated:
		return
	case "":
		violations.Add("evidence.kind is required")
	default:
		violations.Add("evidence.kind must be one of declared, observed, translated")
	}
}

func hasRawJSON(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0
}

func violationsOrNil(violations *ValidationError) error {
	if violations == nil || len(violations.Violations) == 0 {
		return nil
	}
	return violations
}
