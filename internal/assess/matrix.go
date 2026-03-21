package assess

import (
	"context"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/risk"
	"samebits.com/evidra/pkg/evidence"
)

// MatrixAssessor evaluates risk using the static operationClass x scopeClass matrix.
type MatrixAssessor struct{}

// Name returns the assessor name.
func (MatrixAssessor) Name() string { return "matrix" }

// Assess returns a single risk input from the matrix lookup.
func (MatrixAssessor) Assess(_ context.Context, action canon.CanonicalAction, _ []byte) ([]evidence.RiskInput, error) {
	level := risk.RiskLevel(action.OperationClass, action.ScopeClass)
	return []evidence.RiskInput{{
		Source:    "evidra/matrix",
		RiskLevel: level,
	}}, nil
}
