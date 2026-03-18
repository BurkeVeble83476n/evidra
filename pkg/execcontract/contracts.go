package execcontract

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	promptdata "samebits.com/evidra/prompts"
)

const (
	PrescribeToolName      = "evidra_prescribe"
	PrescribeFullToolName  = "evidra_prescribe_full"
	PrescribeSmartToolName = "evidra_prescribe_smart"
	ReportToolName         = "evidra_report"

	VerdictSuccess  = "success"
	VerdictFailure  = "failure"
	VerdictError    = "error"
	VerdictDeclined = "declined"
)

//go:embed schemas/prescribe.schema.json
var prescribeSchemaBytes []byte

//go:embed schemas/prescribe_full.schema.json
var prescribeFullSchemaBytes []byte

//go:embed schemas/prescribe_smart.schema.json
var prescribeSmartSchemaBytes []byte

//go:embed schemas/report.schema.json
var reportSchemaBytes []byte

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Actor struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Origin       string `json:"origin"`
	InstanceID   string `json:"instance_id,omitempty"`
	Version      string `json:"version,omitempty"`
	SkillVersion string `json:"skill_version,omitempty"`
}

type ResourceID struct {
	APIVersion string `json:"api_version,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Actions    string `json:"actions,omitempty"`
}

type CanonicalAction struct {
	ResourceIdentity  []ResourceID `json:"resource_identity,omitempty"`
	ResourceCount     int          `json:"resource_count,omitempty"`
	OperationClass    string       `json:"operation_class,omitempty"`
	ScopeClass        string       `json:"scope_class,omitempty"`
	ResourceShapeHash string       `json:"resource_shape_hash,omitempty"`
}

type PrescribeInput struct {
	Tool            string            `json:"tool"`
	Operation       string            `json:"operation"`
	RawArtifact     string            `json:"raw_artifact"`
	Resource        string            `json:"resource,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	CanonicalAction *CanonicalAction  `json:"canonical_action,omitempty"`
	Actor           Actor             `json:"actor"`
	SessionID       string            `json:"session_id,omitempty"`
	OperationID     string            `json:"operation_id,omitempty"`
	Attempt         int               `json:"attempt,omitempty"`
	TraceID         string            `json:"trace_id,omitempty"`
	SpanID          string            `json:"span_id,omitempty"`
	ParentSpanID    string            `json:"parent_span_id,omitempty"`
	Environment     string            `json:"environment,omitempty"`
	ScopeDimensions map[string]string `json:"scope_dimensions,omitempty"`
}

type PrescribeFullInput struct {
	Tool            string            `json:"tool"`
	Operation       string            `json:"operation"`
	RawArtifact     string            `json:"raw_artifact"`
	CanonicalAction *CanonicalAction  `json:"canonical_action,omitempty"`
	Actor           Actor             `json:"actor"`
	SessionID       string            `json:"session_id,omitempty"`
	OperationID     string            `json:"operation_id,omitempty"`
	Attempt         int               `json:"attempt,omitempty"`
	TraceID         string            `json:"trace_id,omitempty"`
	SpanID          string            `json:"span_id,omitempty"`
	ParentSpanID    string            `json:"parent_span_id,omitempty"`
	Environment     string            `json:"environment,omitempty"`
	ScopeDimensions map[string]string `json:"scope_dimensions,omitempty"`
}

type PrescribeSmartInput struct {
	Tool            string            `json:"tool"`
	Operation       string            `json:"operation"`
	Resource        string            `json:"resource"`
	Namespace       string            `json:"namespace,omitempty"`
	Actor           Actor             `json:"actor"`
	SessionID       string            `json:"session_id,omitempty"`
	OperationID     string            `json:"operation_id,omitempty"`
	Attempt         int               `json:"attempt,omitempty"`
	TraceID         string            `json:"trace_id,omitempty"`
	SpanID          string            `json:"span_id,omitempty"`
	ParentSpanID    string            `json:"parent_span_id,omitempty"`
	Environment     string            `json:"environment,omitempty"`
	ScopeDimensions map[string]string `json:"scope_dimensions,omitempty"`
}

type DecisionContext struct {
	Trigger string `json:"trigger"`
	Reason  string `json:"reason"`
}

type ExternalRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type ReportInput struct {
	PrescriptionID  string           `json:"prescription_id"`
	Verdict         string           `json:"verdict"`
	ExitCode        *int             `json:"exit_code,omitempty"`
	DecisionContext *DecisionContext `json:"decision_context,omitempty"`
	ArtifactDigest  string           `json:"artifact_digest,omitempty"`
	Actor           Actor            `json:"actor,omitempty"`
	ExternalRefs    []ExternalRef    `json:"external_refs,omitempty"`
	SessionID       string           `json:"session_id,omitempty"`
	OperationID     string           `json:"operation_id,omitempty"`
	SpanID          string           `json:"span_id,omitempty"`
	ParentSpanID    string           `json:"parent_span_id,omitempty"`
}

func PrescribeToolDefinition() (ToolDefinition, error) {
	description, err := promptdata.Read(promptdata.MCPPrescribeDescriptionPath)
	if err != nil {
		return ToolDefinition{}, fmt.Errorf("read prescribe description: %w", err)
	}
	parameters, err := loadSchema(prescribeSchemaBytes, "prescribe")
	if err != nil {
		return ToolDefinition{}, err
	}
	return ToolDefinition{
		Name:        PrescribeToolName,
		Description: description,
		Parameters:  parameters,
	}, nil
}

func PrescribeFullToolDefinition() (ToolDefinition, error) {
	description, err := promptdata.Read(promptdata.MCPPrescribeFullDescriptionPath)
	if err != nil {
		return ToolDefinition{}, fmt.Errorf("read prescribe_full description: %w", err)
	}
	parameters, err := loadSchema(prescribeFullSchemaBytes, "prescribe_full")
	if err != nil {
		return ToolDefinition{}, err
	}
	return ToolDefinition{
		Name:        PrescribeFullToolName,
		Description: description,
		Parameters:  parameters,
	}, nil
}

func PrescribeSmartToolDefinition() (ToolDefinition, error) {
	description, err := promptdata.Read(promptdata.MCPPrescribeSmartDescriptionPath)
	if err != nil {
		return ToolDefinition{}, fmt.Errorf("read prescribe_smart description: %w", err)
	}
	parameters, err := loadSchema(prescribeSmartSchemaBytes, "prescribe_smart")
	if err != nil {
		return ToolDefinition{}, err
	}
	return ToolDefinition{
		Name:        PrescribeSmartToolName,
		Description: description,
		Parameters:  parameters,
	}, nil
}

func ReportToolDefinition() (ToolDefinition, error) {
	description, err := promptdata.Read(promptdata.MCPReportDescriptionPath)
	if err != nil {
		return ToolDefinition{}, fmt.Errorf("read report description: %w", err)
	}
	parameters, err := loadSchema(reportSchemaBytes, "report")
	if err != nil {
		return ToolDefinition{}, err
	}
	return ToolDefinition{
		Name:        ReportToolName,
		Description: description,
		Parameters:  parameters,
	}, nil
}

func ValidatePrescribeInput(input PrescribeInput) error {
	if err := validateToolOperation(input.Tool, input.Operation); err != nil {
		return err
	}
	if strings.TrimSpace(input.RawArtifact) == "" &&
		strings.TrimSpace(input.Resource) == "" &&
		input.CanonicalAction == nil {
		return fmt.Errorf("one of raw_artifact, resource, or canonical_action is required")
	}
	return validateActor(input.Actor)
}

func ValidatePrescribeFullInput(input PrescribeFullInput) error {
	if err := validateToolOperation(input.Tool, input.Operation); err != nil {
		return err
	}
	if strings.TrimSpace(input.RawArtifact) == "" {
		return fmt.Errorf("raw_artifact is required")
	}
	return validateActor(input.Actor)
}

func ValidatePrescribeSmartInput(input PrescribeSmartInput) error {
	if err := validateToolOperation(input.Tool, input.Operation); err != nil {
		return err
	}
	if strings.TrimSpace(input.Resource) == "" {
		return fmt.Errorf("resource is required")
	}
	return validateActor(input.Actor)
}

func ValidateReportInput(input ReportInput) error {
	if strings.TrimSpace(input.PrescriptionID) == "" {
		return fmt.Errorf("prescription_id is required")
	}
	switch strings.TrimSpace(input.Verdict) {
	case VerdictSuccess, VerdictFailure, VerdictError:
		if input.ExitCode == nil {
			return fmt.Errorf("verdict %s requires exit_code", input.Verdict)
		}
		if input.DecisionContext != nil {
			return fmt.Errorf("decision_context is only valid for declined verdicts")
		}
	case VerdictDeclined:
		if input.ExitCode != nil {
			return fmt.Errorf("declined verdict must not include exit_code")
		}
		if input.DecisionContext == nil {
			return fmt.Errorf("declined verdict requires decision_context")
		}
		if strings.TrimSpace(input.DecisionContext.Trigger) == "" {
			return fmt.Errorf("declined decision_context.trigger is required")
		}
		if strings.TrimSpace(input.DecisionContext.Reason) == "" {
			return fmt.Errorf("declined decision_context.reason is required")
		}
	default:
		return fmt.Errorf("invalid verdict %q", input.Verdict)
	}
	if actorIsZero(input.Actor) {
		return nil
	}
	return validateActor(input.Actor)
}

func validateToolOperation(tool, operation string) error {
	if strings.TrimSpace(tool) == "" {
		return fmt.Errorf("tool is required")
	}
	if strings.TrimSpace(operation) == "" {
		return fmt.Errorf("operation is required")
	}
	return nil
}

func actorIsZero(actor Actor) bool {
	return strings.TrimSpace(actor.Type) == "" &&
		strings.TrimSpace(actor.ID) == "" &&
		strings.TrimSpace(actor.Origin) == ""
}

func validateActor(actor Actor) error {
	if strings.TrimSpace(actor.Type) == "" {
		return fmt.Errorf("actor.type is required")
	}
	if strings.TrimSpace(actor.ID) == "" {
		return fmt.Errorf("actor.id is required")
	}
	if strings.TrimSpace(actor.Origin) == "" {
		return fmt.Errorf("actor.origin is required")
	}
	return nil
}

func loadSchema(raw []byte, name string) (map[string]any, error) {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("parse %s schema: %w", name, err)
	}
	return normalizeSchema(schema).(map[string]any), nil
}

func normalizeSchema(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			out[key] = normalizeSchema(nested)
		}
		return out
	case []any:
		asStrings := make([]string, 0, len(typed))
		for _, nested := range typed {
			str, ok := nested.(string)
			if !ok {
				asStrings = nil
				break
			}
			asStrings = append(asStrings, str)
		}
		if asStrings != nil {
			return asStrings
		}
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, normalizeSchema(nested))
		}
		return out
	default:
		return value
	}
}
