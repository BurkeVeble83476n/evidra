package assess

import (
	"context"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	_ "samebits.com/evidra/internal/detectors/all"
	"samebits.com/evidra/internal/risk"
	"samebits.com/evidra/pkg/evidence"
)

// DetectorAssessor runs the native tag detector registry against the raw artifact.
// It only produces output when raw artifact bytes are available.
type DetectorAssessor struct{}

// Name returns the assessor name.
func (DetectorAssessor) Name() string { return "detector" }

// Assess runs all registered detectors and returns a risk input elevated by
// the fired tags' base severities. Returns nil if no raw artifact is provided.
func (DetectorAssessor) Assess(_ context.Context, action canon.CanonicalAction, raw []byte) ([]evidence.RiskInput, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	tags := detectors.ProduceAll(action, raw)
	matrixLevel := risk.RiskLevel(action.OperationClass, action.ScopeClass)
	elevated := risk.ElevateRiskLevel(matrixLevel, tags)

	return []evidence.RiskInput{{
		Source:    "evidra/native",
		RiskLevel: elevated,
		RiskTags:  tags,
	}}, nil
}
