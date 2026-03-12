package main

import (
	"testing"

	"samebits.com/evidra/internal/assessment"
	"samebits.com/evidra/internal/score"
	"samebits.com/evidra/internal/signal"
)

func TestAssessmentBelowThresholdMarkedPreview(t *testing.T) {
	t.Parallel()

	assessment := buildAssessment(nil, score.MinOperations-1, "medium")
	if assessment.Basis.AssessmentMode != assessmentModePreview {
		t.Fatalf("assessment mode=%q want %q", assessment.Basis.AssessmentMode, assessmentModePreview)
	}
	if assessment.Basis.Sufficient {
		t.Fatal("basis.sufficient=true want false")
	}
}

func TestAssessmentAtThresholdMarkedSufficient(t *testing.T) {
	t.Parallel()

	assessment := buildAssessment(nil, score.MinOperations, "medium")
	if assessment.Basis.AssessmentMode != assessmentModeSufficient {
		t.Fatalf("assessment mode=%q want %q", assessment.Basis.AssessmentMode, assessmentModeSufficient)
	}
	if !assessment.Basis.Sufficient {
		t.Fatal("basis.sufficient=false want true")
	}
}

func buildAssessment(results []signal.SignalResult, totalOps int, riskLevel string) operationAssessment {
	snapshot := assessment.BuildFromResults(results, totalOps)
	return assessmentFromSnapshot(snapshot, riskLevel)
}
