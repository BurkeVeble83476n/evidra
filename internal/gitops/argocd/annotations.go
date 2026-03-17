package argocd

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	annotationTenantID       = "evidra.cc/tenant-id"
	annotationAgentID        = "evidra.cc/agent-id"
	annotationRunID          = "evidra.cc/run-id"
	annotationSessionID      = "evidra.cc/session-id"
	annotationTraceID        = "evidra.cc/trace-id"
	annotationPrescriptionID = "evidra.cc/prescription-id"
)

func ParseAnnotations(obj *unstructured.Unstructured, defaultTenantID string) Correlation {
	annotations := obj.GetAnnotations()
	correlation := Correlation{
		Mode:           CorrelationModeBestEffort,
		TenantID:       strings.TrimSpace(defaultTenantID),
		AgentID:        strings.TrimSpace(annotations[annotationAgentID]),
		RunID:          strings.TrimSpace(annotations[annotationRunID]),
		SessionID:      strings.TrimSpace(annotations[annotationSessionID]),
		TraceID:        strings.TrimSpace(annotations[annotationTraceID]),
		PrescriptionID: strings.TrimSpace(annotations[annotationPrescriptionID]),
	}
	if tenantID := strings.TrimSpace(annotations[annotationTenantID]); tenantID != "" {
		correlation.TenantID = tenantID
	}
	if correlation.PrescriptionID != "" || correlation.SessionID != "" {
		correlation.Mode = CorrelationModeExplicit
	}
	return correlation
}
