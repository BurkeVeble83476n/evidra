// Package assess provides a pluggable assessment pipeline for prescribe risk evaluation.
//
// The pipeline runs a set of Assessor implementations against a canonical action
// and aggregates their risk inputs into an effective risk level.
package assess

import (
	"context"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/pkg/evidence"
)

// Assessor evaluates a canonical action and returns risk inputs.
// Each assessor represents one source of risk assessment (matrix lookup,
// native detectors, SARIF findings, external policy, etc.).
type Assessor interface {
	// Name returns a human-readable identifier for this assessor.
	Name() string

	// Assess evaluates the action and raw artifact bytes, returning zero or
	// more risk inputs. Assessors that don't apply (e.g., SARIF with no
	// findings) should return nil, nil.
	Assess(ctx context.Context, action canon.CanonicalAction, raw []byte) ([]evidence.RiskInput, error)
}
