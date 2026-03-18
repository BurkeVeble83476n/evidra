package argocd

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"samebits.com/evidra/internal/automationevent"
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/pkg/evidence"
)

var applicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

type ControllerConfig struct {
	ApplicationNamespace string
	TenantID             string
}

type Controller struct {
	client               dynamic.Interface
	emitter              *automationevent.Emitter
	applicationNamespace string
	tenantID             string
}

func NewController(client dynamic.Interface, store automationevent.EventStore, signer evidence.Signer, cfg ControllerConfig) *Controller {
	namespace := strings.TrimSpace(cfg.ApplicationNamespace)
	if namespace == "" {
		namespace = "argocd"
	}
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	return &Controller{
		client:               client,
		emitter:              automationevent.NewEmitter(store, signer),
		applicationNamespace: namespace,
		tenantID:             tenantID,
	}
}

func (c *Controller) SyncOnce(ctx context.Context) error {
	list, err := c.client.Resource(applicationGVR).Namespace(c.applicationNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	items := make([]unstructured.Unstructured, len(list.Items))
	copy(items, list.Items)
	sort.Slice(items, func(i, j int) bool {
		leftRank := lifecycleSortRank(ReduceApplication(&items[i], c.tenantID))
		rightRank := lifecycleSortRank(ReduceApplication(&items[j], c.tenantID))
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return items[i].GetName() < items[j].GetName()
	})

	for i := range items {
		if err := c.handleApplication(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) Run(ctx context.Context) error {
	if err := c.SyncOnce(ctx); err != nil {
		return err
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		watcher, err := c.client.Resource(applicationGVR).Namespace(c.applicationNamespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := c.consumeWatch(ctx, watcher); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

func (c *Controller) consumeWatch(ctx context.Context, watcher watch.Interface) error {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if err := c.handleApplication(ctx, obj); err != nil {
				return err
			}
		}
	}
}

func (c *Controller) handleApplication(ctx context.Context, obj *unstructured.Unstructured) error {
	event, ok := ReduceApplication(obj, c.tenantID)
	if !ok {
		return nil
	}

	claimPayload, err := json.Marshal(obj.Object)
	if err != nil {
		return err
	}
	tenantID := firstNonEmpty(event.Correlation.TenantID, c.tenantID)

	if event.Correlation.Mode == CorrelationModeExplicit {
		if event.Kind != EventKindSyncCompleted || strings.TrimSpace(event.Correlation.PrescriptionID) == "" {
			return nil
		}

		_, err := c.emitter.EmitExplicitReport(ctx, automationevent.ExplicitReportInput{
			TenantID:        tenantID,
			ClaimSource:     event.Source,
			ClaimKey:        event.Key,
			ClaimPayload:    claimPayload,
			Actor:           controllerActor(),
			SessionID:       event.Correlation.SessionID,
			TraceID:         event.Correlation.TraceID,
			PrescriptionID:  event.Correlation.PrescriptionID,
			ArtifactDigest:  event.ArtifactDigest,
			ScopeDimensions: event.ScopeDimensions,
			Verdict:         verdictForPhase(event.Phase),
			ExitCode:        exitCodePtr(exitCodeForPhase(event.Phase)),
			ExternalRefs:    externalRefsForEvent(event),
			Flavor:          automationevent.FlavorReconcile,
			EvidenceKind:    evidence.EvidenceKindTranslated,
			SourceSystem:    "argocd",
		})
		return err
	}

	sessionID := lifecycleSessionID(event)
	prescriptionID := mappedPrescriptionID(event)
	if event.Kind == EventKindSyncStarted {
		_, err := c.emitter.EmitMappedPrescribe(ctx, automationevent.MappedPrescribeInput{
			TenantID:        tenantID,
			ClaimSource:     event.Source,
			ClaimKey:        event.Key,
			ClaimPayload:    claimPayload,
			Actor:           controllerActor(),
			SessionID:       sessionID,
			OperationID:     lifecycleOperationID(event),
			PrescriptionID:  prescriptionID,
			Action:          mappedActionForEvent(event),
			ArtifactDigest:  event.ArtifactDigest,
			ScopeDimensions: event.ScopeDimensions,
			Flavor:          automationevent.FlavorReconcile,
			EvidenceKind:    evidence.EvidenceKindTranslated,
			SourceSystem:    "argocd",
		})
		return err
	}

	_, err = c.emitter.EmitMappedReport(ctx, automationevent.MappedReportInput{
		TenantID:        tenantID,
		ClaimSource:     event.Source,
		ClaimKey:        event.Key,
		ClaimPayload:    claimPayload,
		Actor:           controllerActor(),
		SessionID:       sessionID,
		OperationID:     lifecycleOperationID(event),
		PrescriptionID:  prescriptionID,
		ArtifactDigest:  event.ArtifactDigest,
		ScopeDimensions: event.ScopeDimensions,
		Verdict:         verdictForPhase(event.Phase),
		ExitCode:        exitCodePtr(exitCodeForPhase(event.Phase)),
		ExternalRefs:    externalRefsForEvent(event),
		Flavor:          automationevent.FlavorReconcile,
		EvidenceKind:    evidence.EvidenceKindTranslated,
		SourceSystem:    "argocd",
	})
	return err
}

func controllerActor() evidence.Actor {
	return evidence.Actor{
		Type:       "controller",
		ID:         "argocd-controller",
		Provenance: "mapped:argocd_controller",
	}
}

func lifecycleSessionID(event LifecycleEvent) string {
	if event.OperationID != "" {
		return event.OperationID
	}
	return strings.Join([]string{event.ApplicationUID, event.Revision}, ":")
}

func lifecycleOperationID(event LifecycleEvent) string {
	if event.OperationID != "" {
		return event.OperationID
	}
	return lifecycleSessionID(event)
}

func mappedPrescriptionID(event LifecycleEvent) string {
	parts := []string{
		SourceSystem,
		event.ApplicationUID,
		event.Application,
		event.ApplicationNamespace,
		event.Namespace,
		event.Cluster,
		firstNonEmpty(event.OperationID, event.Revision),
	}
	return "map-" + canon.SHA256Hex([]byte(strings.Join(parts, "|")))
}

func mappedActionForEvent(event LifecycleEvent) canon.CanonicalAction {
	scopeClass := canon.NormalizeScopeClass(firstNonEmpty(event.Environment, event.Namespace))
	shapeHash := canon.SHA256Hex([]byte(strings.Join([]string{
		event.ApplicationUID,
		event.Application,
		event.Namespace,
		event.Cluster,
		event.Revision,
	}, "|")))
	return canon.CanonicalAction{
		Tool:              "argocd",
		Operation:         "sync",
		OperationClass:    "mutate",
		ScopeClass:        scopeClass,
		ResourceCount:     1,
		ResourceShapeHash: shapeHash,
	}
}

func externalRefsForEvent(event LifecycleEvent) []evidence.ExternalRef {
	refs := make([]evidence.ExternalRef, 0, 4)
	if event.Application != "" {
		refs = append(refs, evidence.ExternalRef{
			Type: "argocd_application",
			ID:   firstNonEmpty(event.ApplicationNamespace, "argocd") + "/" + event.Application,
		})
	}
	if event.ApplicationUID != "" {
		refs = append(refs, evidence.ExternalRef{Type: "argocd_application_uid", ID: event.ApplicationUID})
	}
	if event.Revision != "" {
		refs = append(refs, evidence.ExternalRef{Type: "argocd_revision", ID: event.Revision})
	}
	if event.OperationID != "" {
		refs = append(refs, evidence.ExternalRef{Type: "argocd_operation", ID: event.OperationID})
	}
	return refs
}

func verdictForPhase(phase string) evidence.Verdict {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "succeeded":
		return evidence.VerdictSuccess
	case "error", "degraded":
		return evidence.VerdictError
	default:
		return evidence.VerdictFailure
	}
}

func exitCodeForPhase(phase string) int {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "succeeded":
		return 0
	case "error", "degraded":
		return -1
	default:
		return 1
	}
}

func exitCodePtr(v int) *int {
	return &v
}

func lifecycleSortRank(event LifecycleEvent, ok bool) int {
	if !ok {
		return 2
	}
	if event.Kind == EventKindSyncStarted {
		return 0
	}
	return 1
}
