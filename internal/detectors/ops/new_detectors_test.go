package ops

import (
	"strings"
	"testing"

	"samebits.com/evidra/internal/canon"
)

func TestKubeSystem(t *testing.T) {
	t.Parallel()
	d := &KubeSystem{}
	if !d.Detect(canon.CanonicalAction{
		OperationClass: "mutate",
		ResourceIdentity: []canon.ResourceID{
			{Kind: "deployment", Namespace: "kube-system"},
		},
	}, nil) {
		t.Fatalf("expected kube_system detection")
	}
	if d.Detect(canon.CanonicalAction{
		OperationClass: "read",
		ResourceIdentity: []canon.ResourceID{
			{Kind: "deployment", Namespace: "kube-system"},
		},
	}, nil) {
		t.Fatalf("did not expect kube_system detection")
	}
}

func TestNamespaceDelete(t *testing.T) {
	t.Parallel()
	d := &NamespaceDelete{}
	if !d.Detect(canon.CanonicalAction{
		Operation: "delete",
		ResourceIdentity: []canon.ResourceID{
			{Kind: "namespace", Name: "prod"},
		},
	}, nil) {
		t.Fatalf("expected namespace_delete detection")
	}
	if d.Detect(canon.CanonicalAction{
		Operation: "apply",
		ResourceIdentity: []canon.ResourceID{
			{Kind: "namespace", Name: "prod"},
		},
	}, nil) {
		t.Fatalf("did not expect namespace_delete detection")
	}
}

func TestMassDelete_K8sFallbackRequiresDestroyOperation(t *testing.T) {
	t.Parallel()

	d := &MassDelete{}
	raw := []byte(strings.Repeat(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
`, 11))

	if d.Detect(canon.CanonicalAction{OperationClass: "mutate"}, raw) {
		t.Fatal("did not expect mass_delete detection for mutate fallback input")
	}
	if !d.Detect(canon.CanonicalAction{OperationClass: "destroy"}, raw) {
		t.Fatal("expected mass_delete detection for destroy fallback input")
	}
}
