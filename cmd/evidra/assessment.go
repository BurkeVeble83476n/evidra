package main

import (
	"fmt"

	"samebits.com/evidra-benchmark/internal/pipeline"
	"samebits.com/evidra-benchmark/internal/score"
	"samebits.com/evidra-benchmark/internal/signal"
	"samebits.com/evidra-benchmark/pkg/evidence"
)

const (
	assessmentModePreview    = "preview"
	assessmentModeSufficient = "sufficient"
	previewMinOperations     = 1
)

type assessmentBasis struct {
	AssessmentMode       string `json:"assessment_mode"`
	Sufficient           bool   `json:"sufficient"`
	TotalOperations      int    `json:"total_operations"`
	SufficientThreshold  int    `json:"sufficient_threshold"`
	PreviewMinOperations int    `json:"preview_min_operations"`
}

type operationAssessment struct {
	RiskClassification string           `json:"risk_classification"`
	RiskLevel          string           `json:"risk_level"`
	Score              float64          `json:"score"`
	ScoreBand          string           `json:"score_band"`
	SignalSummary      map[string]int   `json:"signal_summary"`
	Confidence         score.Confidence `json:"confidence"`
	Basis              assessmentBasis  `json:"basis"`
}

func buildOperationAssessment(evidencePath, sessionID, riskLevel string) (operationAssessment, error) {
	entries, err := evidence.ReadAllEntriesAtPath(evidencePath)
	if err != nil {
		return operationAssessment{}, fmt.Errorf("read evidence for assessment: %w", err)
	}

	filtered := filterEntries(entries, "", "", sessionID)
	signalEntries, err := pipeline.EvidenceToSignalEntries(filtered)
	if err != nil {
		return operationAssessment{}, fmt.Errorf("convert evidence for assessment: %w", err)
	}

	results := signal.AllSignals(signalEntries, signal.DefaultTTL)
	totalOps := countPrescriptions(signalEntries)
	return buildAssessment(results, totalOps, riskLevel), nil
}

func buildAssessment(results []signal.SignalResult, totalOps int, riskLevel string) operationAssessment {
	strict := score.Compute(results, totalOps, 0.0)
	preview := score.ComputeWithMinOperations(results, totalOps, 0.0, previewMinOperations)

	selected := strict
	mode := assessmentModeSufficient
	if !strict.Sufficient {
		selected = preview
		mode = assessmentModePreview
	}

	return operationAssessment{
		RiskClassification: selected.Band,
		RiskLevel:          riskLevel,
		Score:              selected.Score,
		ScoreBand:          selected.Band,
		SignalSummary:      selected.Signals,
		Confidence:         selected.Confidence,
		Basis: assessmentBasis{
			AssessmentMode:       mode,
			Sufficient:           strict.Sufficient,
			TotalOperations:      totalOps,
			SufficientThreshold:  score.MinOperations,
			PreviewMinOperations: previewMinOperations,
		},
	}
}
