package main

import (
	"testing"

	"samebits.com/evidra-benchmark/internal/score"
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
