package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/ingest"
	"samebits.com/evidra/pkg/evidence"
)

type IngestPort interface {
	Prescribe(ctx context.Context, tenantID string, in ingest.PrescribeRequest) (ingest.Result, error)
	Report(ctx context.Context, tenantID string, in ingest.ReportRequest) (ingest.Result, error)
}

type ingestResponse struct {
	Duplicate     bool   `json:"duplicate"`
	EntryID       string `json:"entry_id"`
	EffectiveRisk string `json:"effective_risk"`
}

type ingestPrescribeResponse struct {
	ingestResponse
	PrescriptionID string `json:"prescription_id"`
}

func handleIngestPrescribe(svc IngestPort) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeError(w, http.StatusServiceUnavailable, "ingest not configured")
			return
		}

		tenantID := auth.TenantID(r.Context())
		if tenantID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req ingest.PrescribeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := ingest.ValidatePrescribeRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid ingest request")
			return
		}

		result, err := svc.Prescribe(r.Context(), tenantID, req)
		if err != nil {
			writeIngestServiceError(w, err)
			return
		}

		resp := ingestPrescribeResponse{
			ingestResponse: ingestResponse{
				Duplicate:     result.Duplicate,
				EntryID:       result.EntryID,
				EffectiveRisk: result.EffectiveRisk,
			},
			PrescriptionID: prescribeResponsePrescriptionID(req, result),
		}

		status := http.StatusAccepted
		if result.Duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, resp)
	}
}

func prescribeResponsePrescriptionID(req ingest.PrescribeRequest, result ingest.Result) string {
	if id := requestPrescriptionID(req); result.Duplicate && id != "" {
		return id
	}
	if id, ok := entryPrescriptionID(result.Entry.Payload); ok {
		return id
	}
	if id := requestPrescriptionID(req); id != "" {
		return id
	}
	return result.EntryID
}

func requestPrescriptionID(req ingest.PrescribeRequest) string {
	if req.PayloadOverride == nil {
		return ""
	}
	var payload evidence.PrescriptionPayload
	if err := json.Unmarshal(*req.PayloadOverride, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.PrescriptionID)
}

func entryPrescriptionID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var payload evidence.PrescriptionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", false
	}
	id := strings.TrimSpace(payload.PrescriptionID)
	if id == "" {
		return "", false
	}
	return id, true
}

func handleIngestReport(svc IngestPort) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeError(w, http.StatusServiceUnavailable, "ingest not configured")
			return
		}

		tenantID := auth.TenantID(r.Context())
		if tenantID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req ingest.ReportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := ingest.ValidateReportRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid ingest request")
			return
		}

		result, err := svc.Report(r.Context(), tenantID, req)
		if err != nil {
			writeIngestServiceError(w, err)
			return
		}

		status := http.StatusAccepted
		if result.Duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, ingestResponse{
			Duplicate:     result.Duplicate,
			EntryID:       result.EntryID,
			EffectiveRisk: result.EffectiveRisk,
		})
	}
}

func writeIngestServiceError(w http.ResponseWriter, err error) {
	switch ingest.ErrorCode(err) {
	case ingest.ErrCodeInvalidInput:
		writeError(w, http.StatusBadRequest, "invalid ingest request")
	case ingest.ErrCodeNotFound:
		writeError(w, http.StatusNotFound, "prescription not found")
	case ingest.ErrCodeNoSignerConfigured:
		writeError(w, http.StatusServiceUnavailable, "ingest not configured")
	default:
		writeError(w, http.StatusInternalServerError, "ingest failed")
	}
}
