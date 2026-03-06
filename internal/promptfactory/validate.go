package promptfactory

import (
	"fmt"
)

func ValidateBundle(bundle Bundle, expectedVersion string) error {
	if bundle.Contract.Version == "" {
		return fmt.Errorf("contract.contract_version is required")
	}
	if expectedVersion != "" && bundle.Contract.Version != expectedVersion {
		return fmt.Errorf("contract version mismatch: bundle=%s expected=%s", bundle.Contract.Version, expectedVersion)
	}
	if len(bundle.Contract.Invariants) == 0 {
		return fmt.Errorf("contract.invariants is required")
	}
	if len(bundle.Contract.MCP.Initialize.ProductSummary) == 0 {
		return fmt.Errorf("mcp.initialize.product_summary is required")
	}
	if bundle.Contract.MCP.Initialize.ProtocolIntro == "" {
		return fmt.Errorf("mcp.initialize.protocol_intro is required")
	}
	if len(bundle.Contract.MCP.Initialize.CriticalInvariants) == 0 {
		return fmt.Errorf("mcp.initialize.critical_invariants is required")
	}
	if len(bundle.Contract.MCP.Initialize.Rules) == 0 {
		return fmt.Errorf("mcp.initialize.rules is required")
	}
	if bundle.Contract.MCP.Prescribe.Intro == "" {
		return fmt.Errorf("mcp.prescribe.intro is required")
	}
	if len(bundle.Contract.MCP.Prescribe.RequiredInputs) == 0 {
		return fmt.Errorf("mcp.prescribe.required_inputs is required")
	}
	if len(bundle.Contract.MCP.Prescribe.PreCallChecklist) == 0 {
		return fmt.Errorf("mcp.prescribe.pre_call_checklist is required")
	}
	if len(bundle.Contract.MCP.Report.RequiredInputs) == 0 {
		return fmt.Errorf("mcp.report.required_inputs is required")
	}
	if len(bundle.Contract.MCP.Report.TerminalOutcomeRule) == 0 {
		return fmt.Errorf("mcp.report.terminal_outcome_rule is required")
	}
	if len(bundle.Contract.MCP.Report.Rules) == 0 {
		return fmt.Errorf("mcp.report.rules is required")
	}
	if bundle.Contract.MCP.GetEvent.Intro == "" {
		return fmt.Errorf("mcp.get_event.intro is required")
	}
	if len(bundle.Contract.MCP.GetEvent.Input) == 0 {
		return fmt.Errorf("mcp.get_event.input is required")
	}
	if len(bundle.Contract.MCP.GetEvent.Returns) == 0 {
		return fmt.Errorf("mcp.get_event.returns is required")
	}
	if len(bundle.Contract.LiteLLM.SystemIntro) == 0 {
		return fmt.Errorf("litellm.system_intro is required")
	}
	if len(bundle.Contract.LiteLLM.ExecutionModeRules) == 0 {
		return fmt.Errorf("litellm.execution_mode_rules is required")
	}
	if len(bundle.Contract.LiteLLM.AssessmentModeRequirements) == 0 {
		return fmt.Errorf("litellm.assessment_mode_requirements is required")
	}
	if len(bundle.Classification.MutateExamples) == 0 {
		return fmt.Errorf("classification.mutate_examples is required")
	}
	if len(bundle.Classification.ReadOnlyExamples) == 0 {
		return fmt.Errorf("classification.read_only_examples is required")
	}
	if bundle.Output.AssessmentJSON.LevelField == "" || bundle.Output.AssessmentJSON.DetailsField == "" {
		return fmt.Errorf("output_contracts.assessment_json fields are required")
	}
	if len(bundle.Output.AssessmentJSON.AllowedLevel) == 0 {
		return fmt.Errorf("output_contracts.assessment_json.allowed_levels is required")
	}
	if bundle.Contract.AgentContract.Title == "" {
		return fmt.Errorf("agent_contract.title is required")
	}
	if bundle.Contract.AgentContract.VersionPolicy == "" {
		return fmt.Errorf("agent_contract.version_policy is required")
	}
	if len(bundle.Contract.AgentContract.Changelog) == 0 {
		return fmt.Errorf("agent_contract.changelog is required")
	}
	if len(bundle.Contract.AgentContract.ExecutionRules) == 0 {
		return fmt.Errorf("agent_contract.execution_rules is required")
	}
	if len(bundle.Contract.AgentContract.OutputRules) == 0 {
		return fmt.Errorf("agent_contract.output_rules is required")
	}
	return nil
}
