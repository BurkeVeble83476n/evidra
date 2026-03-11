package score

import (
	"testing"

	"samebits.com/evidra/internal/signal"
)

func TestCompute_SignalProfiles(t *testing.T) {
	t.Parallel()

	results := []signal.SignalResult{
		{Name: "protocol_violation", Count: 0}, // none
		{Name: "retry_loop", Count: 1},         // low at 1%
		{Name: "artifact_drift", Count: 5},     // medium at 5%
		{Name: "thrashing", Count: 15},         // high at 15%
	}
	sc := Compute(results, 100, 0.0)

	if sc.SignalProfiles["protocol_violation"].Level != "none" {
		t.Fatalf("protocol_violation level=%q want none", sc.SignalProfiles["protocol_violation"].Level)
	}
	if sc.SignalProfiles["retry_loop"].Level != "low" {
		t.Fatalf("retry_loop level=%q want low", sc.SignalProfiles["retry_loop"].Level)
	}
	if sc.SignalProfiles["artifact_drift"].Level != "medium" {
		t.Fatalf("artifact_drift level=%q want medium", sc.SignalProfiles["artifact_drift"].Level)
	}
	if sc.SignalProfiles["thrashing"].Level != "high" {
		t.Fatalf("thrashing level=%q want high", sc.SignalProfiles["thrashing"].Level)
	}
}

func TestCompute_RepairLoopBonus(t *testing.T) {
	t.Parallel()

	base := Compute([]signal.SignalResult{
		{Name: "retry_loop", Count: 10},
	}, 100, 0.0)

	withRepair := Compute([]signal.SignalResult{
		{Name: "retry_loop", Count: 10},
		{Name: "repair_loop", Count: 10},
	}, 100, 0.0)

	if withRepair.Score <= base.Score {
		t.Fatalf("expected repair_loop bonus to increase score: base=%.2f with_repair=%.2f", base.Score, withRepair.Score)
	}
}
