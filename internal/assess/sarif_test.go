package assess

import (
	"context"
	"testing"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/pkg/evidence"
)

func TestSARIFAssessor_NoSources(t *testing.T) {
	t.Parallel()
	a := SARIFAssessor{}
	inputs, err := a.Assess(context.Background(), canon.CanonicalAction{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if inputs != nil {
		t.Errorf("expected nil, got %v", inputs)
	}
}

func TestSARIFAssessor_CriticalFinding(t *testing.T) {
	t.Parallel()
	a := SARIFAssessor{Sources: []FindingsSource{{
		Source: "trivy/0.58.0",
		Findings: []evidence.FindingPayload{
			{Tool: "trivy", RuleID: "DS002", Severity: "critical"},
			{Tool: "trivy", RuleID: "DS005", Severity: "high"},
		},
	}}}
	inputs, err := a.Assess(context.Background(), canon.CanonicalAction{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].RiskLevel != "critical" {
		t.Errorf("expected critical, got %s", inputs[0].RiskLevel)
	}
	if inputs[0].Source != "trivy/0.58.0" {
		t.Errorf("expected source trivy/0.58.0, got %s", inputs[0].Source)
	}
	if len(inputs[0].RiskTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(inputs[0].RiskTags))
	}
}
