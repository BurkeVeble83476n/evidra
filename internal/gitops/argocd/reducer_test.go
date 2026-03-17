package argocd

import (
	"strings"
	"testing"
)

func TestReduceApplication_RunningSyncProducesStartEvent(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Annotations: map[string]string{
			"evidra.cc/prescription-id": "presc-123",
			"evidra.cc/session-id":      "sess-123",
		},
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:       "Running",
		Health:      "Progressing",
		Revision:    "abc123",
		OperationID: "argo-op-123",
	})

	event, ok := ReduceApplication(app, "tenant-default")
	if !ok {
		t.Fatal("ReduceApplication() did not emit a start event")
	}
	if event.Kind != EventKindSyncStarted {
		t.Fatalf("event.Kind = %q, want %q", event.Kind, EventKindSyncStarted)
	}
	if event.Source != SourceStart {
		t.Fatalf("event.Source = %q, want %q", event.Source, SourceStart)
	}
	if event.Key != "uid-123:argo-op-123:sync_started" {
		t.Fatalf("event.Key = %q, want uid-123:argo-op-123:sync_started", event.Key)
	}
	if event.Application != "payments-app" {
		t.Fatalf("event.Application = %q, want payments-app", event.Application)
	}
	if event.ApplicationNamespace != "argocd" {
		t.Fatalf("event.ApplicationNamespace = %q, want argocd", event.ApplicationNamespace)
	}
	if event.Namespace != "payments" {
		t.Fatalf("event.Namespace = %q, want payments", event.Namespace)
	}
	if event.Cluster != "prod-us-east" {
		t.Fatalf("event.Cluster = %q, want prod-us-east", event.Cluster)
	}
	if event.Project != "default" {
		t.Fatalf("event.Project = %q, want default", event.Project)
	}
	if event.Revision != "abc123" {
		t.Fatalf("event.Revision = %q, want abc123", event.Revision)
	}
	if event.Phase != "Running" {
		t.Fatalf("event.Phase = %q, want Running", event.Phase)
	}
	if event.Health != "Progressing" {
		t.Fatalf("event.Health = %q, want Progressing", event.Health)
	}
	if event.Correlation.Mode != CorrelationModeExplicit {
		t.Fatalf("event.Correlation.Mode = %q, want %q", event.Correlation.Mode, CorrelationModeExplicit)
	}
	wantScope := map[string]string{
		"source_kind":           "controller_observed",
		"source_system":         "argocd_controller",
		"integration_mode":      "explicit",
		"correlation_mode":      "explicit",
		"environment":           "production",
		"cluster":               "prod-us-east",
		"namespace":             "payments",
		"application":           "payments-app",
		"application_namespace": "argocd",
		"project":               "default",
		"revision":              "abc123",
	}
	if diff := compareScopeDimensions(event.ScopeDimensions, wantScope); diff != "" {
		t.Fatalf("scope_dimensions mismatch: %s", diff)
	}
	if !strings.HasPrefix(event.ArtifactDigest, "sha256:") {
		t.Fatalf("event.ArtifactDigest = %q, want sha256 digest", event.ArtifactDigest)
	}
}

func TestReduceApplication_CompletedSyncProducesCompletionEvent(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Labels: map[string]string{
			"environment": "production",
		},
		Phase:    "Succeeded",
		Health:   "Healthy",
		Revision: "abc123",
	})

	event, ok := ReduceApplication(app, "tenant-default")
	if !ok {
		t.Fatal("ReduceApplication() did not emit a completion event")
	}
	if event.Kind != EventKindSyncCompleted {
		t.Fatalf("event.Kind = %q, want %q", event.Kind, EventKindSyncCompleted)
	}
	if event.Source != SourceComplete {
		t.Fatalf("event.Source = %q, want %q", event.Source, SourceComplete)
	}
	if event.Key != "uid-123:abc123:succeeded:sync_completed" {
		t.Fatalf("event.Key = %q, want uid-123:abc123:succeeded:sync_completed", event.Key)
	}
	if event.Correlation.Mode != CorrelationModeBestEffort {
		t.Fatalf("event.Correlation.Mode = %q, want %q", event.Correlation.Mode, CorrelationModeBestEffort)
	}
	wantScope := map[string]string{
		"source_kind":           "mapped",
		"source_system":         "argocd_controller",
		"integration_mode":      "zero_touch",
		"correlation_mode":      "best_effort",
		"environment":           "production",
		"cluster":               "prod-us-east",
		"namespace":             "payments",
		"application":           "payments-app",
		"application_namespace": "argocd",
		"project":               "default",
		"revision":              "abc123",
	}
	if diff := compareScopeDimensions(event.ScopeDimensions, wantScope); diff != "" {
		t.Fatalf("scope_dimensions mismatch: %s", diff)
	}
}

func TestReduceApplication_IgnoresSteadyStateNoise(t *testing.T) {
	t.Parallel()

	app := newApplication(t, applicationFixture{
		Labels: map[string]string{
			"environment": "production",
		},
		Health:   "Healthy",
		Revision: "abc123",
	})

	if _, ok := ReduceApplication(app, "tenant-default"); ok {
		t.Fatal("ReduceApplication() emitted an event for steady state noise")
	}
}

func compareScopeDimensions(got, want map[string]string) string {
	if len(got) != len(want) {
		return "different sizes"
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			return key + "=" + got[key] + " want " + wantValue
		}
	}
	return ""
}
