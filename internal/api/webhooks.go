package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	iauth "samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/automationevent"
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/store"
	pkevidence "samebits.com/evidra/pkg/evidence"
)

type WebhookStore interface {
	automationevent.EventStore
}

type WebhookTenantResolver func(ctx context.Context, apiKey string) (string, error)

const webhookTenantAPIKeyHeader = "X-Evidra-API-Key"

type genericWebhookPayload struct {
	EventType      string             `json:"event_type"`
	Tool           string             `json:"tool"`
	Operation      string             `json:"operation"`
	OperationID    string             `json:"operation_id"`
	Environment    string             `json:"environment"`
	Actor          string             `json:"actor"`
	SessionID      string             `json:"session_id"`
	ExitCode       *int               `json:"exit_code,omitempty"`
	Verdict        pkevidence.Verdict `json:"verdict,omitempty"`
	IdempotencyKey string             `json:"idempotency_key,omitempty"`
}

type argoCDWebhookPayload struct {
	Event        string `json:"event"`
	AppName      string `json:"app_name"`
	AppNamespace string `json:"app_namespace"`
	Revision     string `json:"revision"`
	InitiatedBy  string `json:"initiated_by"`
	OperationID  string `json:"operation_id"`
	Phase        string `json:"phase"`
	Message      string `json:"message"`
}

func handleGenericWebhookWithTenantResolver(store WebhookStore, signer pkevidence.Signer, secret string, resolveTenant WebhookTenantResolver) http.HandlerFunc {
	emitter := automationevent.NewEmitter(store, signer)

	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := webhookRequestBody(w, r, secret, signer)
		if !ok {
			return
		}
		tenantID, ok := resolveWebhookTenant(w, r, resolveTenant)
		if !ok {
			return
		}

		var payload genericWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if strings.TrimSpace(payload.Tool) == "" || strings.TrimSpace(payload.Operation) == "" {
			writeError(w, http.StatusBadRequest, "tool and operation are required")
			return
		}
		operationID := strings.TrimSpace(payload.OperationID)
		if operationID == "" {
			writeError(w, http.StatusBadRequest, "operation_id is required")
			return
		}
		if payload.EventType != "operation_started" && payload.EventType != "operation_completed" {
			writeError(w, http.StatusBadRequest, "unsupported event_type")
			return
		}
		if payload.EventType == "operation_completed" && strings.TrimSpace(payload.IdempotencyKey) == "" {
			writeError(w, http.StatusBadRequest, "idempotency_key is required for operation_completed")
			return
		}

		idempotencyKey := strings.TrimSpace(payload.IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = "generic:" + operationID + ":start"
		}

		action := mappedCanonicalAction(payload.Tool, payload.Operation, payload.Environment)
		actor := mappedActor(payload.Actor, "generic")
		sessionID := strings.TrimSpace(payload.SessionID)
		if sessionID == "" {
			sessionID = operationID
		}
		prescriptionID := mappedPrescriptionID("generic", payload.Tool, payload.Operation, "", operationID, payload.Environment, "")
		scope := mappedScopeDimensions("generic", payload.Environment, map[string]string{})
		artifactDigest := canon.SHA256Hex(body)

		if payload.EventType == "operation_started" {
			result, err := emitter.EmitMappedPrescribe(r.Context(), automationevent.MappedPrescribeInput{
				TenantID:        tenantID,
				ClaimSource:     "generic",
				ClaimKey:        idempotencyKey,
				ClaimPayload:    body,
				Actor:           actor,
				SessionID:       sessionID,
				OperationID:     operationID,
				PrescriptionID:  prescriptionID,
				Action:          action,
				ArtifactDigest:  artifactDigest,
				ScopeDimensions: scope,
				Flavor:          automationevent.FlavorImperative,
				EvidenceKind:    pkevidence.EvidenceKindTranslated,
				SourceSystem:    "generic",
			})
			writeWebhookEmitResult(w, result, err)
			return
		}

		exitCode := payload.ExitCode
		if exitCode == nil {
			defaultCode := exitCodeForVerdict(payload.Verdict)
			exitCode = &defaultCode
		}
		result, err := emitter.EmitMappedReport(r.Context(), automationevent.MappedReportInput{
			TenantID:        tenantID,
			ClaimSource:     "generic",
			ClaimKey:        idempotencyKey,
			ClaimPayload:    body,
			Actor:           actor,
			SessionID:       sessionID,
			OperationID:     operationID,
			PrescriptionID:  prescriptionID,
			ArtifactDigest:  artifactDigest,
			ScopeDimensions: scope,
			Verdict:         payload.Verdict,
			ExitCode:        exitCode,
			Flavor:          automationevent.FlavorImperative,
			EvidenceKind:    pkevidence.EvidenceKindTranslated,
			SourceSystem:    "generic",
		})
		writeWebhookEmitResult(w, result, err)
	}
}

func handleArgoCDWebhookWithTenantResolver(store WebhookStore, signer pkevidence.Signer, secret string, resolveTenant WebhookTenantResolver) http.HandlerFunc {
	emitter := automationevent.NewEmitter(store, signer)

	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := webhookRequestBody(w, r, secret, signer)
		if !ok {
			return
		}
		tenantID, ok := resolveWebhookTenant(w, r, resolveTenant)
		if !ok {
			return
		}

		var payload argoCDWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if strings.TrimSpace(payload.AppName) == "" || strings.TrimSpace(payload.OperationID) == "" {
			writeError(w, http.StatusBadRequest, "app_name and operation_id are required")
			return
		}

		sourceKey := mappedPrescriptionID("argocd", payload.AppName, "sync", payload.InitiatedBy, payload.OperationID, payload.AppNamespace, "")
		idempotencyKey := payload.AppName + ":" + payload.OperationID
		source := "argocd_start"
		if payload.Event != "sync_started" {
			source = "argocd_complete"
			idempotencyKey += ":complete"
		}

		action := mappedCanonicalAction("argocd", "sync", payload.AppNamespace)
		actor := mappedActor(payload.InitiatedBy, "argocd")
		scope := mappedScopeDimensions("argocd", payload.AppNamespace, map[string]string{
			"application": payload.AppName,
			"revision":    payload.Revision,
		})
		artifactDigest := canon.SHA256Hex(body)
		externalRefs := []pkevidence.ExternalRef{
			{Type: "argocd_application", ID: payload.AppNamespace + "/" + payload.AppName},
			{Type: "argocd_revision", ID: payload.Revision},
			{Type: "argocd_operation", ID: payload.OperationID},
		}

		switch payload.Event {
		case "sync_started":
			result, err := emitter.EmitMappedPrescribe(r.Context(), automationevent.MappedPrescribeInput{
				TenantID:        tenantID,
				ClaimSource:     source,
				ClaimKey:        idempotencyKey,
				ClaimPayload:    body,
				Actor:           actor,
				SessionID:       payload.OperationID,
				OperationID:     payload.OperationID,
				PrescriptionID:  sourceKey,
				Action:          action,
				ArtifactDigest:  artifactDigest,
				ScopeDimensions: scope,
				Flavor:          automationevent.FlavorReconcile,
				EvidenceKind:    pkevidence.EvidenceKindTranslated,
				SourceSystem:    "argocd",
			})
			writeWebhookEmitResult(w, result, err)
		case "sync_completed":
			verdict, exitCode, ok := argoCDVerdict(payload.Phase)
			if !ok {
				writeError(w, http.StatusBadRequest, "unsupported argocd phase")
				return
			}
			result, err := emitter.EmitMappedReport(r.Context(), automationevent.MappedReportInput{
				TenantID:        tenantID,
				ClaimSource:     source,
				ClaimKey:        idempotencyKey,
				ClaimPayload:    body,
				Actor:           actor,
				SessionID:       payload.OperationID,
				OperationID:     payload.OperationID,
				PrescriptionID:  sourceKey,
				ArtifactDigest:  artifactDigest,
				ScopeDimensions: scope,
				Verdict:         verdict,
				ExitCode:        &exitCode,
				ExternalRefs:    externalRefs,
				Flavor:          automationevent.FlavorReconcile,
				EvidenceKind:    pkevidence.EvidenceKindTranslated,
				SourceSystem:    "argocd",
			})
			writeWebhookEmitResult(w, result, err)
		default:
			writeError(w, http.StatusBadRequest, "unsupported argocd event")
		}
	}
}

func resolveWebhookTenant(w http.ResponseWriter, r *http.Request, resolveTenant WebhookTenantResolver) (string, bool) {
	apiKey := strings.TrimSpace(r.Header.Get(webhookTenantAPIKeyHeader))
	if apiKey == "" {
		writeError(w, http.StatusUnauthorized, "missing tenant api key")
		return "", false
	}
	tenantID, err := resolveTenant(r.Context(), apiKey)
	if err != nil || strings.TrimSpace(tenantID) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return tenantID, true
}

func tenantResolverFromKeyStore(ks interface {
	LookupKey(ctx context.Context, plaintext string) (store.KeyRecord, error)
}) WebhookTenantResolver {
	return func(ctx context.Context, apiKey string) (string, error) {
		rec, err := ks.LookupKey(ctx, apiKey)
		if err != nil {
			return "", err
		}
		return rec.TenantID, nil
	}
}

func webhookRequestBody(w http.ResponseWriter, r *http.Request, secret string, signer pkevidence.Signer) (json.RawMessage, bool) {
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(iauth.ParseBearerToken(r.Header.Get("Authorization")))), []byte(secret)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	if signer == nil {
		writeError(w, http.StatusServiceUnavailable, "webhook ingestion requires server signing")
		return nil, false
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		writeError(w, http.StatusBadRequest, "content-type must be application/json")
		return nil, false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		writeError(w, http.StatusBadRequest, "empty or unreadable body")
		return nil, false
	}
	return body, true
}

func writeWebhookEmitResult(w http.ResponseWriter, result automationevent.EmitResult, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build mapped evidence failed")
		return
	}
	if result.Duplicate {
		writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func mappedActor(actorID, source string) pkevidence.Actor {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		actorID = source + "-controller"
	}
	return pkevidence.Actor{
		Type:       "controller",
		ID:         actorID,
		Provenance: "mapped:" + source,
	}
}

func mappedCanonicalAction(tool, operation, environment string) canon.CanonicalAction {
	scope := canon.NormalizeScopeClass(environment)
	return canon.CanonicalAction{
		Tool:              strings.TrimSpace(tool),
		Operation:         strings.TrimSpace(operation),
		OperationClass:    mappedOperationClass(operation),
		ScopeClass:        scope,
		ResourceCount:     1,
		ResourceShapeHash: canon.SHA256Hex([]byte(tool + "|" + operation + "|" + scope)),
	}
}

func mappedOperationClass(operation string) string {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "delete", "destroy", "remove", "uninstall":
		return "destroy"
	case "get", "read", "describe", "list":
		return "read"
	case "plan":
		return "plan"
	default:
		return "mutate"
	}
}

func mappedScopeDimensions(source, environment string, extra map[string]string) map[string]string {
	scope := map[string]string{
		"source_kind":   "mapped",
		"source_system": source,
	}
	if environment != "" {
		scope["environment"] = environment
	}
	for k, v := range extra {
		if strings.TrimSpace(v) == "" {
			continue
		}
		scope[k] = v
	}
	return scope
}

func mappedPrescriptionID(source, tool, operation, actor, sessionID, environment, suffix string) string {
	parts := []string{source, tool, operation, actor, sessionID, environment, suffix}
	return "map-" + canon.SHA256Hex([]byte(strings.Join(parts, "|")))
}

func argoCDVerdict(phase string) (pkevidence.Verdict, int, bool) {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "succeeded":
		return pkevidence.VerdictSuccess, 0, true
	case "failed":
		return pkevidence.VerdictFailure, 1, true
	case "error", "degraded":
		return pkevidence.VerdictError, -1, true
	default:
		return "", 0, false
	}
}

func exitCodeForVerdict(verdict pkevidence.Verdict) int {
	switch verdict {
	case pkevidence.VerdictSuccess:
		return 0
	case pkevidence.VerdictError:
		return -1
	default:
		return 1
	}
}
