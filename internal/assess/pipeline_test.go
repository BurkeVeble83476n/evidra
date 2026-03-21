package assess

import (
	"context"
	"testing"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/pkg/evidence"
)

func TestPipeline_EmptyAssessors(t *testing.T) {
	t.Parallel()
	p := NewPipeline()
	result, err := p.Run(context.Background(), canon.CanonicalAction{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.EffectiveRisk != "low" {
		t.Errorf("expected low (no inputs), got %s", result.EffectiveRisk)
	}
	if len(result.RiskInputs) != 0 {
		t.Errorf("expected 0 inputs, got %d", len(result.RiskInputs))
	}
}

func TestPipeline_MatrixOnly(t *testing.T) {
	t.Parallel()
	p := NewPipeline(MatrixAssessor{})
	result, err := p.Run(context.Background(), canon.CanonicalAction{
		OperationClass: "mutate",
		ScopeClass:     "production",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.EffectiveRisk != "high" {
		t.Errorf("expected high, got %s", result.EffectiveRisk)
	}
	if len(result.RiskInputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(result.RiskInputs))
	}
	if result.RiskInputs[0].Source != "evidra/matrix" {
		t.Errorf("expected source evidra/matrix, got %s", result.RiskInputs[0].Source)
	}
}

func TestPipeline_MatrixPlusDetector_NoArtifact(t *testing.T) {
	t.Parallel()
	p := NewPipeline(MatrixAssessor{}, DetectorAssessor{})
	result, err := p.Run(context.Background(), canon.CanonicalAction{
		OperationClass: "mutate",
		ScopeClass:     "staging",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Detector returns nil for empty artifact, so only matrix input
	if len(result.RiskInputs) != 1 {
		t.Errorf("expected 1 input (detector skipped), got %d", len(result.RiskInputs))
	}
	if result.EffectiveRisk != "medium" {
		t.Errorf("expected medium, got %s", result.EffectiveRisk)
	}
}

func TestPipeline_MultipleAssessors(t *testing.T) {
	t.Parallel()

	sarif := SARIFAssessor{Sources: []FindingsSource{{
		Source: "trivy/0.58.0",
		Findings: []evidence.FindingPayload{
			{Tool: "trivy", RuleID: "DS002", Severity: "critical"},
		},
	}}}

	p := NewPipeline(MatrixAssessor{}, sarif)
	result, err := p.Run(context.Background(), canon.CanonicalAction{
		OperationClass: "mutate",
		ScopeClass:     "development",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RiskInputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(result.RiskInputs))
	}
	// Matrix says low (mutate+dev), SARIF says critical -> effective = critical
	if result.EffectiveRisk != "critical" {
		t.Errorf("expected critical, got %s", result.EffectiveRisk)
	}
}

func TestPipeline_NativeTagsCollected(t *testing.T) {
	t.Parallel()

	// Use a spy assessor that returns native source
	spy := &spyAssessor{inputs: []evidence.RiskInput{{
		Source:    "evidra/native",
		RiskLevel: "high",
		RiskTags:  []string{"k8s.privileged_container", "ops.mass_delete"},
	}}}

	p := NewPipeline(MatrixAssessor{}, spy)
	result, err := p.Run(context.Background(), canon.CanonicalAction{
		OperationClass: "mutate",
		ScopeClass:     "production",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NativeTags) != 2 {
		t.Errorf("expected 2 native tags, got %d", len(result.NativeTags))
	}
}

func TestComputeEffectiveRisk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inputs []evidence.RiskInput
		want   string
	}{
		{"empty", nil, "low"},
		{"single low", []evidence.RiskInput{{RiskLevel: "low"}}, "low"},
		{"mixed", []evidence.RiskInput{{RiskLevel: "low"}, {RiskLevel: "critical"}}, "critical"},
		{"all medium", []evidence.RiskInput{{RiskLevel: "medium"}, {RiskLevel: "medium"}}, "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeEffectiveRisk(tt.inputs)
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

type spyAssessor struct {
	inputs []evidence.RiskInput
}

func (s *spyAssessor) Name() string { return "spy" }

func (s *spyAssessor) Assess(_ context.Context, _ canon.CanonicalAction, _ []byte) ([]evidence.RiskInput, error) {
	return s.inputs, nil
}
