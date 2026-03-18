package execcontract

import "testing"

func TestPrescribeToolDefinition_UsesSharedSchema(t *testing.T) {
	t.Parallel()

	def, err := PrescribeToolDefinition()
	if err != nil {
		t.Fatalf("PrescribeToolDefinition: %v", err)
	}
	if def.Name != PrescribeToolName {
		t.Fatalf("name = %q, want %q", def.Name, PrescribeToolName)
	}

	required, ok := def.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required = %#v, want []string", def.Parameters["required"])
	}
	for _, field := range []string{"tool", "operation", "actor"} {
		if !contains(required, field) {
			t.Fatalf("required missing %q: %#v", field, required)
		}
	}
	if contains(required, "raw_artifact") {
		t.Fatalf("required unexpectedly contains raw_artifact: %#v", required)
	}

	properties, ok := def.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want map[string]any", def.Parameters["properties"])
	}
	for _, field := range []string{"resource", "namespace"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("properties missing %q: %#v", field, properties)
		}
	}
}

func TestValidatePrescribeInput_AllowsSmartMode(t *testing.T) {
	t.Parallel()

	err := ValidatePrescribeInput(PrescribeInput{
		Tool:      "kubectl",
		Operation: "apply",
		Resource:  "deployment/web",
		Actor: Actor{
			Type:   "agent",
			ID:     "bench",
			Origin: "mcp-stdio",
		},
	})
	if err != nil {
		t.Fatalf("ValidatePrescribeInput: %v", err)
	}
}

func TestValidatePrescribeInput_RequiresArtifactOrSmartTarget(t *testing.T) {
	t.Parallel()

	err := ValidatePrescribeInput(PrescribeInput{
		Tool:      "kubectl",
		Operation: "apply",
		Actor: Actor{
			Type:   "agent",
			ID:     "bench",
			Origin: "mcp-stdio",
		},
	})
	if err == nil {
		t.Fatal("expected validation error when both raw_artifact and smart target are missing")
	}
}

func TestValidateReportInput_DeclinedRules(t *testing.T) {
	t.Parallel()

	exitCode := 0
	err := ValidateReportInput(ReportInput{
		PrescriptionID: "presc-1",
		Verdict:        VerdictDeclined,
		ExitCode:       &exitCode,
		DecisionContext: &DecisionContext{
			Trigger: "risk_threshold_exceeded",
			Reason:  "blast radius too large",
		},
	})
	if err == nil {
		t.Fatal("expected declined validation error when exit_code is present")
	}

	err = ValidateReportInput(ReportInput{
		PrescriptionID: "presc-1",
		Verdict:        VerdictDeclined,
		DecisionContext: &DecisionContext{
			Trigger: "risk_threshold_exceeded",
			Reason:  "blast radius too large",
		},
	})
	if err != nil {
		t.Fatalf("declined validation failed: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
