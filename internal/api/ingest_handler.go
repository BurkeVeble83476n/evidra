package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
		}
		prescriptionID, err := prescriptionIDFromEntry(result.Entry.Payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "marshal response")
			return
		}
		resp.PrescriptionID = prescriptionID

		status := http.StatusAccepted
		if result.Duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, resp)
	}
}

func prescriptionIDFromEntry(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("missing evidence entry payload")
	}
	var payload evidence.PrescriptionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	id := payload.PrescriptionID
	if id == "" {
		return "", fmt.Errorf("missing prescription_id")
	}
	return id, nil
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
