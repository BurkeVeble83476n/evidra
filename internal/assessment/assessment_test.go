package assessment

import (
	"testing"

	"samebits.com/evidra/internal/score"
)

func TestBuildFromResults_PreviewWhenBelowThreshold(t *testing.T) {
	t.Parallel()

	got := BuildFromResults(nil, 1)
	if got.Basis.AssessmentMode != AssessmentModePreview {
		t.Fatalf("mode=%q want %q", got.Basis.AssessmentMode, AssessmentModePreview)
	}
	if got.Basis.Sufficient {
		t.Fatal("basis.sufficient=true want false")
	}
	if got.ScoreBand == "insufficient_data" {
		t.Fatalf("preview mode should return a scored preview band, got %q", got.ScoreBand)
	}
}

func TestBuildFromResults_SufficientAtThreshold(t *testing.T) {
	t.Parallel()

	got := BuildFromResults(nil, score.MinOperations)
	if got.Basis.AssessmentMode != AssessmentModeSufficient {
		t.Fatalf("mode=%q want %q", got.Basis.AssessmentMode, AssessmentModeSufficient)
	}
	if !got.Basis.Sufficient {
		t.Fatal("basis.sufficient=false want true")
	}
}
