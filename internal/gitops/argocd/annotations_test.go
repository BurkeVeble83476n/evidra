package argocd

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseAnnotations_ExplicitMode(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Annotations: map[string]string{
			"evidra.cc/tenant-id":       "tenant-explicit",
			"evidra.cc/agent-id":        "gha-deploy",
			"evidra.cc/run-id":          "run-123",
			"evidra.cc/session-id":      "sess-123",
			"evidra.cc/trace-id":        "trace-123",
			"evidra.cc/prescription-id": "presc-123",
		},
	})

	got := ParseAnnotations(app, "tenant-default")
	want := Correlation{
		Mode:           CorrelationModeExplicit,
		TenantID:       "tenant-explicit",
		AgentID:        "gha-deploy",
		RunID:          "run-123",
		SessionID:      "sess-123",
		TraceID:        "trace-123",
		PrescriptionID: "presc-123",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAnnotations() = %#v, want %#v", got, want)
	}
}

func TestParseAnnotations_ZeroTouchMode(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{})

	got := ParseAnnotations(app, "tenant-default")
	want := Correlation{
		Mode:     CorrelationModeBestEffort,
		TenantID: "tenant-default",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAnnotations() = %#v, want %#v", got, want)
	}
}

type applicationFixture struct {
	Name          string
	Namespace     string
	UID           string
	Annotations   map[string]string
	Labels        map[string]string
	DestinationNS string
	Destination   string
	Project       string
	Phase         string
	Health        string
	Revision      string
	OperationID   string
}

func newApplication(t *testing.T, fixture applicationFixture) *unstructured.Unstructured {
	t.Helper()

	name := fixture.Name
	if name == "" {
		name = "payments-app"
	}
	namespace := fixture.Namespace
	if namespace == "" {
		namespace = "argocd"
	}
	uid := fixture.UID
	if uid == "" {
		uid = "uid-123"
	}
	project := fixture.Project
	if project == "" {
		project = "default"
	}
	destinationNS := fixture.DestinationNS
	if destinationNS == "" {
		destinationNS = "payments"
	}
	destination := fixture.Destination
	if destination == "" {
		destination = "prod-us-east"
	}

	annotations := map[string]any{}
	for key, value := range fixture.Annotations {
		annotations[key] = value
	}
	if fixture.OperationID != "" {
		annotations["argocd.argoproj.io/operation-id"] = fixture.OperationID
	}

	labels := map[string]any{}
	for key, value := range fixture.Labels {
		labels[key] = value
	}

	object := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":        name,
			"namespace":   namespace,
			"uid":         uid,
			"annotations": annotations,
			"labels":      labels,
		},
		"spec": map[string]any{
			"project": project,
			"destination": map[string]any{
				"name":      destination,
				"namespace": destinationNS,
			},
		},
		"status": map[string]any{
			"health": map[string]any{
				"status": fixture.Health,
			},
			"operationState": map[string]any{
				"phase": fixture.Phase,
				"operation": map[string]any{
					"sync": map[string]any{
						"revision": fixture.Revision,
					},
				},
			},
			"sync": map[string]any{
				"revision": fixture.Revision,
			},
		},
	}

	return &unstructured.Unstructured{Object: object}
}
