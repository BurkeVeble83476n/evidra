package ingest

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/pkg/evidence"
)

func TestValidatePrescribeRequestValidCanonicalAction(t *testing.T) {
	t.Parallel()

	req := PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:       "session-1",
			OperationID:     "operation-1",
			TraceID:         "trace-1",
			Flavor:          evidence.FlavorWorkflow,
			Evidence:        &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:          &evidence.SourceMetadata{System: "argocd"},
			ScopeDimensions: map[string]string{"cluster": "prod"},
		},
		CanonicalAction: &canon.CanonicalAction{
			Tool:              "kubectl",
			Operation:         "apply",
			OperationClass:    "mutate",
			ScopeClass:        "production",
			ResourceCount:     1,
			ResourceShapeHash: "sha256:" + strings.Repeat("a", 64),
		},
	}

	if err := ValidatePrescribeRequest(req); err != nil {
		t.Fatalf("ValidatePrescribeRequest: %v", err)
	}
}

func TestValidatePrescribeRequestValidSmartTarget(t *testing.T) {
	t.Parallel()

	req := PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-2",
			OperationID: "operation-2",
			TraceID:     "trace-2",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		SmartTarget: &SmartTarget{
			Tool:      "kubectl",
			Operation: "apply",
			Resource:  "deployment/nginx",
			Namespace: "default",
		},
	}

	if err := ValidatePrescribeRequest(req); err != nil {
		t.Fatalf("ValidatePrescribeRequest: %v", err)
	}
}

func TestValidateReportRequestValidPrescriptionIDAndVerdict(t *testing.T) {
	t.Parallel()

	exitCode := 0
	req := ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		PrescriptionID: "rx-1",
		Verdict:        evidence.VerdictSuccess,
		ExitCode:       &exitCode,
	}

	if err := ValidateReportRequest(req); err != nil {
		t.Fatalf("ValidateReportRequest: %v", err)
	}
}

func TestValidatePrescribeRequestRequiresSmartTargetWhenCanonicalActionMissing(t *testing.T) {
	t.Parallel()

	req := PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
	}

	err := ValidatePrescribeRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "smart_target") {
		t.Fatalf("error = %q, want smart_target violation", err.Error())
	}
}

func TestValidateContractVersionRequired(t *testing.T) {
	t.Parallel()

	req := PrescribeRequest{
		Envelope: Envelope{
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		CanonicalAction: &canon.CanonicalAction{Tool: "kubectl", Operation: "apply"},
	}

	err := ValidatePrescribeRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if !strings.Contains(err.Error(), "contract_version") {
		t.Fatalf("error = %q, want contract_version violation", err.Error())
	}
}

func TestValidatePrescribeRequestRequiresTaxonomyFields(t *testing.T) {
	t.Parallel()

	req := PrescribeRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
		},
		CanonicalAction: &canon.CanonicalAction{Tool: "kubectl", Operation: "apply"},
	}

	err := ValidatePrescribeRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{"flavor", "evidence.kind", "source.system"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want violation for %q", err.Error(), want)
		}
	}
}

func TestValidateReportRequestDeclinedRules(t *testing.T) {
	t.Parallel()

	t.Run("missing decision context", func(t *testing.T) {
		req := declinedReportRequest()
		err := ValidateReportRequest(req)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "decision_context") {
			t.Fatalf("error = %q, want decision_context violation", err.Error())
		}
	})

	t.Run("rejects exit code", func(t *testing.T) {
		req := declinedReportRequest()
		exitCode := 1
		req.ExitCode = &exitCode
		req.DecisionContext = &evidence.DecisionContext{
			Trigger: "risk_threshold_exceeded",
			Reason:  "risk too high",
		}
		err := ValidateReportRequest(req)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "exit_code") {
			t.Fatalf("error = %q, want exit_code violation", err.Error())
		}
	})
}

func TestValidateReportRequestRequiresExitCodeForNonDeclined(t *testing.T) {
	t.Parallel()

	req := ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		PrescriptionID: "rx-1",
		Verdict:        evidence.VerdictSuccess,
	}

	err := ValidateReportRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "exit_code") {
		t.Fatalf("error = %q, want exit_code violation", err.Error())
	}
}

func TestValidatePayloadOverrideCombinations(t *testing.T) {
	t.Parallel()

	t.Run("prescribe rejects empty override", func(t *testing.T) {
		emptyOverride := json.RawMessage("")
		req := PrescribeRequest{
			Envelope: Envelope{
				ContractVersion: ContractVersionV1,
				Actor: evidence.Actor{
					Type:       "controller",
					ID:         "argocd",
					Provenance: "argocd",
				},
				SessionID:   "session-1",
				OperationID: "operation-1",
				TraceID:     "trace-1",
				Flavor:      evidence.FlavorWorkflow,
				Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
				Source:      &evidence.SourceMetadata{System: "argocd"},
			},
			PayloadOverride: &emptyOverride,
		}

		err := ValidatePrescribeRequest(req)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "payload_override") {
			t.Fatalf("error = %q, want payload_override violation", err.Error())
		}
	})

	t.Run("prescribe rejects smart target plus override", func(t *testing.T) {
		override := json.RawMessage(`{"canonical_action":{"tool":"kubectl"}}`)
		req := PrescribeRequest{
			Envelope: Envelope{
				ContractVersion: ContractVersionV1,
				Actor: evidence.Actor{
					Type:       "controller",
					ID:         "argocd",
					Provenance: "argocd",
				},
				SessionID:   "session-1",
				OperationID: "operation-1",
				TraceID:     "trace-1",
				Flavor:      evidence.FlavorWorkflow,
				Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
				Source:      &evidence.SourceMetadata{System: "argocd"},
			},
			SmartTarget: &SmartTarget{
				Tool:      "kubectl",
				Operation: "apply",
				Resource:  "deployment/nginx",
			},
			PayloadOverride: &override,
		}

		err := ValidatePrescribeRequest(req)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "payload_override") {
			t.Fatalf("error = %q, want payload_override violation", err.Error())
		}
	})

	t.Run("report rejects explicit fields plus override", func(t *testing.T) {
		exitCode := 0
		override := json.RawMessage(`{"verdict":"success"}`)
		req := ReportRequest{
			Envelope: Envelope{
				ContractVersion: ContractVersionV1,
				Actor: evidence.Actor{
					Type:       "controller",
					ID:         "argocd",
					Provenance: "argocd",
				},
				SessionID:   "session-1",
				OperationID: "operation-1",
				TraceID:     "trace-1",
				Flavor:      evidence.FlavorWorkflow,
				Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
				Source:      &evidence.SourceMetadata{System: "argocd"},
			},
			PrescriptionID:  "rx-1",
			Verdict:         evidence.VerdictSuccess,
			ExitCode:        &exitCode,
			PayloadOverride: &override,
		}

		err := ValidateReportRequest(req)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "payload_override") {
			t.Fatalf("error = %q, want payload_override violation", err.Error())
		}
	})

	t.Run("report rejects invalid verdict plus override", func(t *testing.T) {
		override := json.RawMessage(`{"verdict":"success"}`)
		req := ReportRequest{
			Envelope: Envelope{
				ContractVersion: ContractVersionV1,
				Actor: evidence.Actor{
					Type:       "controller",
					ID:         "argocd",
					Provenance: "argocd",
				},
				SessionID:   "session-1",
				OperationID: "operation-1",
				TraceID:     "trace-1",
				Flavor:      evidence.FlavorWorkflow,
				Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
				Source:      &evidence.SourceMetadata{System: "argocd"},
			},
			Verdict:         evidence.Verdict("bogus"),
			PayloadOverride: &override,
		}

		err := ValidateReportRequest(req)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "payload_override") {
			t.Fatalf("error = %q, want payload_override violation", err.Error())
		}
	})
}

func declinedReportRequest() ReportRequest {
	return ReportRequest{
		Envelope: Envelope{
			ContractVersion: ContractVersionV1,
			Actor: evidence.Actor{
				Type:       "controller",
				ID:         "argocd",
				Provenance: "argocd",
			},
			SessionID:   "session-1",
			OperationID: "operation-1",
			TraceID:     "trace-1",
			Flavor:      evidence.FlavorWorkflow,
			Evidence:    &evidence.EvidenceMetadata{Kind: evidence.EvidenceKindObserved},
			Source:      &evidence.SourceMetadata{System: "argocd"},
		},
		PrescriptionID: "rx-1",
		Verdict:        evidence.VerdictDeclined,
	}
}
