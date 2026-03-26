package promptfactory

import (
	"fmt"
)

func ValidateBundle(bundle Bundle, expectedVersion string) error {
	if err := validateContract(bundle.Contract, expectedVersion); err != nil {
		return err
	}
	if err := validateDevOpsContract(bundle.Contract.DevOps); err != nil {
		return err
	}
	if err := validateMCPContract(bundle.Contract.MCP); err != nil {
		return err
	}
	if err := validateClassification(bundle.Classification); err != nil {
		return err
	}
	if err := validateAgentContract(bundle.Contract.AgentContract); err != nil {
		return err
	}
	return nil
}

func validateDevOpsContract(contract DevOpsContract) error {
	if contract.RunCommand.Intro == "" &&
		contract.RunCommand.SmartOutput == "" &&
		len(contract.RunCommand.DiagnosisProtocol) == 0 &&
		len(contract.RunCommand.SafetyRules) == 0 &&
		contract.RunCommand.EvidenceIntro == "" &&
		len(contract.RunCommand.WhenToPrescribe) == 0 {
		return nil
	}
	if contract.RunCommand.Intro == "" {
		return fmt.Errorf("devops.run_command.intro is required")
	}
	if contract.RunCommand.SmartOutput == "" {
		return fmt.Errorf("devops.run_command.smart_output is required")
	}
	if len(contract.RunCommand.DiagnosisProtocol) == 0 {
		return fmt.Errorf("devops.run_command.diagnosis_protocol is required")
	}
	if len(contract.RunCommand.SafetyRules) == 0 {
		return fmt.Errorf("devops.run_command.safety_rules is required")
	}
	if contract.RunCommand.EvidenceIntro == "" {
		return fmt.Errorf("devops.run_command.evidence_intro is required")
	}
	if len(contract.RunCommand.WhenToPrescribe) == 0 {
		return fmt.Errorf("devops.run_command.when_to_prescribe is required")
	}
	return nil
}

func validateContract(contract Contract, expectedVersion string) error {
	if contract.Version == "" {
		return fmt.Errorf("contract.contract_version is required")
	}
	if expectedVersion != "" && contract.Version != expectedVersion {
		return fmt.Errorf("contract version mismatch: bundle=%s expected=%s", contract.Version, expectedVersion)
	}
	if len(contract.Invariants) == 0 {
		return fmt.Errorf("contract.invariants is required")
	}
	return nil
}

func validateMCPContract(mcp MCPContract) error {
	if len(mcp.Initialize.ProductSummary) == 0 {
		return fmt.Errorf("mcp.initialize.product_summary is required")
	}
	if mcp.Initialize.ProtocolIntro == "" {
		return fmt.Errorf("mcp.initialize.protocol_intro is required")
	}
	if len(mcp.Initialize.CriticalInvariants) == 0 {
		return fmt.Errorf("mcp.initialize.critical_invariants is required")
	}
	if len(mcp.Initialize.Rules) == 0 {
		return fmt.Errorf("mcp.initialize.rules is required")
	}
	if err := validatePrescribeContracts(mcp); err != nil {
		return err
	}
	if len(mcp.Report.RequiredInputs) == 0 {
		return fmt.Errorf("mcp.report.required_inputs is required")
	}
	if len(mcp.Report.TerminalOutcomeRule) == 0 {
		return fmt.Errorf("mcp.report.terminal_outcome_rule is required")
	}
	if len(mcp.Report.Rules) == 0 {
		return fmt.Errorf("mcp.report.rules is required")
	}
	if mcp.GetEvent.Intro == "" {
		return fmt.Errorf("mcp.get_event.intro is required")
	}
	if len(mcp.GetEvent.Input) == 0 {
		return fmt.Errorf("mcp.get_event.input is required")
	}
	if len(mcp.GetEvent.Returns) == 0 {
		return fmt.Errorf("mcp.get_event.returns is required")
	}
	return nil
}

func validatePrescribeContracts(mcp MCPContract) error {
	if mcp.Prescribe.Intro != "" {
		if len(mcp.Prescribe.RequiredInputs) == 0 {
			return fmt.Errorf("mcp.prescribe.required_inputs is required")
		}
		if len(mcp.Prescribe.PreCallChecklist) == 0 {
			return fmt.Errorf("mcp.prescribe.pre_call_checklist is required")
		}
		return nil
	}
	if mcp.PrescribeFull.Intro == "" {
		return fmt.Errorf("mcp.prescribe_full.intro is required")
	}
	if len(mcp.PrescribeFull.RequiredInputs) == 0 {
		return fmt.Errorf("mcp.prescribe_full.required_inputs is required")
	}
	if len(mcp.PrescribeFull.PreCallChecklist) == 0 {
		return fmt.Errorf("mcp.prescribe_full.pre_call_checklist is required")
	}
	if mcp.PrescribeSmart.Intro == "" {
		return fmt.Errorf("mcp.prescribe_smart.intro is required")
	}
	if len(mcp.PrescribeSmart.RequiredInputs) == 0 {
		return fmt.Errorf("mcp.prescribe_smart.required_inputs is required")
	}
	if len(mcp.PrescribeSmart.PreCallChecklist) == 0 {
		return fmt.Errorf("mcp.prescribe_smart.pre_call_checklist is required")
	}
	return nil
}

func validateClassification(classification Classification) error {
	if len(classification.MutateExamples) == 0 {
		return fmt.Errorf("classification.mutate_examples is required")
	}
	if len(classification.ReadOnlyExamples) == 0 {
		return fmt.Errorf("classification.read_only_examples is required")
	}
	return nil
}

func validateAgentContract(contract AgentContract) error {
	if contract.Title == "" {
		return fmt.Errorf("agent_contract.title is required")
	}
	if contract.VersionPolicy == "" {
		return fmt.Errorf("agent_contract.version_policy is required")
	}
	if len(contract.Changelog) == 0 {
		return fmt.Errorf("agent_contract.changelog is required")
	}
	if len(contract.ExecutionRules) == 0 {
		return fmt.Errorf("agent_contract.execution_rules is required")
	}
	return nil
}
