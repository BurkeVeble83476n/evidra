package assess

import (
	"context"
	"testing"

	"samebits.com/evidra/internal/canon"
)

func TestMatrixAssessor_MutateProduction(t *testing.T) {
	t.Parallel()
	inputs, err := MatrixAssessor{}.Assess(context.Background(), canon.CanonicalAction{
		OperationClass: "mutate",
		ScopeClass:     "production",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].RiskLevel != "high" {
		t.Errorf("expected high, got %v", inputs)
	}
}

func TestMatrixAssessor_ReadDevelopment(t *testing.T) {
	t.Parallel()
	inputs, err := MatrixAssessor{}.Assess(context.Background(), canon.CanonicalAction{
		OperationClass: "read",
		ScopeClass:     "development",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].RiskLevel != "low" {
		t.Errorf("expected low, got %v", inputs)
	}
}

func TestMatrixAssessor_DestroyProduction(t *testing.T) {
	t.Parallel()
	inputs, err := MatrixAssessor{}.Assess(context.Background(), canon.CanonicalAction{
		OperationClass: "destroy",
		ScopeClass:     "production",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].RiskLevel != "critical" {
		t.Errorf("expected critical, got %v", inputs)
	}
}
