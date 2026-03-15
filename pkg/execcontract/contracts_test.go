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
	for _, field := range []string{"tool", "operation", "raw_artifact", "actor"} {
		if !contains(required, field) {
			t.Fatalf("required missing %q: %#v", field, required)
		}
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
