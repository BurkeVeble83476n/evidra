package ops

import (
	"testing"

	"samebits.com/evidra-benchmark/internal/canon"
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
