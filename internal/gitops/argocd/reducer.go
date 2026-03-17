package argocd

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"samebits.com/evidra/internal/canon"
)

const (
	argocdOperationIDAnnotation = "argocd.argoproj.io/operation-id"
	environmentLabel            = "environment"
	environmentAnnotation       = "evidra.cc/environment"
)

func ReduceApplication(obj *unstructured.Unstructured, defaultTenantID string) (LifecycleEvent, bool) {
	phase := nestedString(obj.Object, "status", "operationState", "phase")
	kind, source, ok := reducePhase(phase)
	if !ok {
		return LifecycleEvent{}, false
	}

	correlation := ParseAnnotations(obj, defaultTenantID)
	application := obj.GetName()
	applicationNamespace := obj.GetNamespace()
	applicationUID := string(obj.GetUID())
	revision := firstNonEmpty(
		nestedString(obj.Object, "status", "operationState", "operation", "sync", "revision"),
		nestedString(obj.Object, "status", "sync", "revision"),
	)
	namespace := nestedString(obj.Object, "spec", "destination", "namespace")
	cluster := firstNonEmpty(
		nestedString(obj.Object, "spec", "destination", "name"),
		nestedString(obj.Object, "spec", "destination", "server"),
	)
	project := nestedString(obj.Object, "spec", "project")
	health := nestedString(obj.Object, "status", "health", "status")
	operationID := firstNonEmpty(
		strings.TrimSpace(obj.GetAnnotations()[argocdOperationIDAnnotation]),
		strings.TrimSpace(obj.GetAnnotations()["evidra.cc/operation-id"]),
	)
	environment := firstNonEmpty(
		strings.TrimSpace(obj.GetLabels()[environmentLabel]),
		strings.TrimSpace(obj.GetAnnotations()[environmentAnnotation]),
	)

	scopeDimensions := map[string]string{
		"source_system":         SourceSystem,
		"correlation_mode":      correlation.Mode,
		"cluster":               cluster,
		"namespace":             namespace,
		"application":           application,
		"application_namespace": applicationNamespace,
		"project":               project,
		"revision":              revision,
	}
	if environment != "" {
		scopeDimensions["environment"] = environment
	}
	if correlation.Mode == CorrelationModeExplicit {
		scopeDimensions["source_kind"] = "controller_observed"
		scopeDimensions["integration_mode"] = "explicit"
	} else {
		scopeDimensions["source_kind"] = "mapped"
		scopeDimensions["integration_mode"] = "zero_touch"
	}

	return LifecycleEvent{
		Key:                  lifecycleKey(applicationUID, operationID, revision, phase, kind),
		Source:               source,
		Kind:                 kind,
		Phase:                phase,
		Health:               health,
		Application:          application,
		ApplicationNamespace: applicationNamespace,
		ApplicationUID:       applicationUID,
		Namespace:            namespace,
		Cluster:              cluster,
		Project:              project,
		Environment:          environment,
		Revision:             revision,
		OperationID:          operationID,
		ArtifactDigest: canon.SHA256Hex([]byte(strings.Join([]string{
			kind,
			applicationUID,
			application,
			applicationNamespace,
			namespace,
			cluster,
			project,
			revision,
			phase,
			operationID,
		}, "|"))),
		Correlation:     correlation,
		ScopeDimensions: scopeDimensions,
	}, true
}

func reducePhase(phase string) (kind, source string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "running":
		return EventKindSyncStarted, SourceStart, true
	case "succeeded", "failed", "error", "degraded":
		return EventKindSyncCompleted, SourceComplete, true
	default:
		return "", "", false
	}
}

func lifecycleKey(applicationUID, operationID, revision, phase, kind string) string {
	if strings.TrimSpace(operationID) != "" {
		return strings.Join([]string{applicationUID, operationID, kind}, ":")
	}
	return strings.Join([]string{applicationUID, revision, strings.ToLower(strings.TrimSpace(phase)), kind}, ":")
}

func nestedString(object map[string]any, fields ...string) string {
	value, found, err := unstructured.NestedString(object, fields...)
	if err != nil || !found {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
