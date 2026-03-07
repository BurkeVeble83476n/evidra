package experiments

import "testing"

func TestEvaluateArtifactPassExactMatch(t *testing.T) {
	eval := evaluateArtifact(
		"high",
		"high",
		[]string{"k8s.hostpath_mount", "k8s.privileged_container"},
		[]string{"k8s.privileged_container", "k8s.hostpath_mount"},
	)
	if !eval.Pass {
		t.Fatalf("expected pass=true, got false: %+v", eval)
	}
}

func TestEvaluateArtifactPassFalseOnTagMismatch(t *testing.T) {
	eval := evaluateArtifact(
		"high",
		"high",
		[]string{"k8s.hostpath_mount"},
		[]string{"k8s.privileged_container"},
	)
	if eval.Pass {
		t.Fatalf("expected pass=false, got true: %+v", eval)
	}
}

func TestEvaluateArtifactPassFalseOnLevelMismatch(t *testing.T) {
	eval := evaluateArtifact(
		"high",
		"medium",
		[]string{"k8s.hostpath_mount"},
		[]string{"k8s.hostpath_mount"},
	)
	if eval.Pass {
		t.Fatalf("expected pass=false, got true: %+v", eval)
	}
}
