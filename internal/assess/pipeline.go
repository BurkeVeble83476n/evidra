package assess

import (
	"context"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/risk"
	"samebits.com/evidra/pkg/evidence"
)

// Result holds the output of a pipeline run.
type Result struct {
	RiskInputs    []evidence.RiskInput
	EffectiveRisk string
	NativeTags    []string // tags from the detector assessor, if any
}

// Pipeline orchestrates assessment by running all registered assessors
// and aggregating their risk inputs.
type Pipeline struct {
	assessors []Assessor
}

// NewPipeline creates a pipeline with the given assessors.
// Assessors are evaluated in order; all results are merged.
func NewPipeline(assessors ...Assessor) *Pipeline {
	return &Pipeline{assessors: assessors}
}

// Assessors returns the current assessor list. Used to build derived pipelines.
func (p *Pipeline) Assessors() []Assessor {
	return p.assessors
}

// Run executes all assessors against the canonical action and raw artifact,
// then aggregates risk inputs into an effective risk level.
func (p *Pipeline) Run(ctx context.Context, action canon.CanonicalAction, raw []byte) (Result, error) {
	var allInputs []evidence.RiskInput
	var nativeTags []string

	for _, a := range p.assessors {
		inputs, err := a.Assess(ctx, action, raw)
		if err != nil {
			return Result{}, err
		}
		for _, ri := range inputs {
			allInputs = append(allInputs, ri)
			if ri.Source == "evidra/native" {
				nativeTags = append(nativeTags, ri.RiskTags...)
			}
		}
	}

	return Result{
		RiskInputs:    allInputs,
		EffectiveRisk: computeEffectiveRisk(allInputs),
		NativeTags:    nativeTags,
	}, nil
}

func computeEffectiveRisk(inputs []evidence.RiskInput) string {
	best := "low"
	for _, ri := range inputs {
		if risk.SeverityHigherThan(ri.RiskLevel, best) {
			best = ri.RiskLevel
		}
	}
	return best
}
