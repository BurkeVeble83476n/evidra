package promptfactory

type Bundle struct {
	Contract       Contract        `yaml:"contract"`
	Classification Classification  `yaml:"classification"`
}

type Contract struct {
	Version    string   `yaml:"contract_version"`
	Invariants []string `yaml:"invariants"`

	DevOps DevOpsContract `yaml:"devops"`

	MCP MCPContract `yaml:"mcp"`

	AgentContract AgentContract `yaml:"agent_contract"`
}

type DevOpsContract struct {
	RunCommand         RunCommandContract         `yaml:"run_command"`
	DiagnosisFlowchart DiagnosisFlowchartContract `yaml:"diagnosis_flowchart"`
	RiskTags           []RiskTagEntry             `yaml:"risk_tags"`
	BehavioralSignals  []string                   `yaml:"behavioral_signals"`
}

type RunCommandContract struct {
	Intro             string   `yaml:"intro"`
	SmartOutput       string   `yaml:"smart_output"`
	DiagnosisProtocol []string `yaml:"diagnosis_protocol"`
	SafetyRules       []string `yaml:"safety_rules"`
	EvidenceIntro     string   `yaml:"evidence_intro"`
	WhenToPrescribe   []string `yaml:"when_to_prescribe"`
}

type DiagnosisFlowchartContract struct {
	Step1Gather    DiagnosisStep          `yaml:"step_1_gather"`
	Step2Logs      DiagnosisStep          `yaml:"step_2_logs"`
	Step3RootCause DiagnosisRootCauseStep `yaml:"step_3_root_cause"`
	Step4Fix       DiagnosisFixStep       `yaml:"step_4_fix"`
	Step5Verify    DiagnosisStep          `yaml:"step_5_verify"`
	Rules          []string               `yaml:"rules"`
}

type DiagnosisStep struct {
	Title    string   `yaml:"title"`
	Commands []string `yaml:"commands"`
	LookFor  string   `yaml:"look_for,omitempty"`
}

type DiagnosisRootCauseStep struct {
	Title    string   `yaml:"title"`
	Patterns []string `yaml:"patterns"`
}

type DiagnosisFixStep struct {
	Title string `yaml:"title"`
	Rule  string `yaml:"rule"`
	Flow  string `yaml:"flow"`
}

type RiskTagEntry struct {
	Tag      string `yaml:"tag"`
	Severity string `yaml:"severity"`
}

type MCPContract struct {
	Initialize     InitializeContract `yaml:"initialize"`
	Prescribe      PrescribeContract  `yaml:"prescribe"`
	PrescribeFull  PrescribeContract  `yaml:"prescribe_full"`
	PrescribeSmart PrescribeContract  `yaml:"prescribe_smart"`
	Report         ReportContract     `yaml:"report"`
	GetEvent       GetEventContract   `yaml:"get_event"`
	Prompts        MCPPrompts         `yaml:"prompts"`
}

type MCPPrompts struct {
	PrescribeSmart MCPPromptPrescribeSmart `yaml:"prescribe_smart"`
	PrescribeFull  MCPPromptPrescribeFull  `yaml:"prescribe_full"`
	Diagnosis      MCPPromptDiagnosis      `yaml:"diagnosis"`
}

type MCPPromptField struct {
	Field       string `yaml:"field"`
	Description string `yaml:"description"`
}

type MCPPromptPrescribeSmart struct {
	Title          string           `yaml:"title"`
	Intro          string           `yaml:"intro"`
	RequiredFields []MCPPromptField `yaml:"required_fields"`
	OptionalFields []MCPPromptField `yaml:"optional_fields"`
	Response       string           `yaml:"response"`
	WhenToUse      string           `yaml:"when_to_use"`
}

type MCPPromptPrescribeFull struct {
	Title              string           `yaml:"title"`
	Intro              string           `yaml:"intro"`
	RequiredFields     []MCPPromptField `yaml:"required_fields"`
	Response           string           `yaml:"response"`
	WhenToUseOverSmart []string         `yaml:"when_to_use_over_smart"`
}

type MCPPromptDiagnosis struct {
	Title string `yaml:"title"`
	Intro string `yaml:"intro"`
}

type InitializeContract struct {
	ProductSummary     []string `yaml:"product_summary"`
	ProtocolIntro      string   `yaml:"protocol_intro"`
	CriticalInvariants []string `yaml:"critical_invariants"`
	Rules              []string `yaml:"rules"`
}

type PrescribeContract struct {
	Intro                     string   `yaml:"intro"`
	RequiredInputs            []string `yaml:"required_inputs"`
	RecommendedActorFields    []string `yaml:"recommended_actor_fields"`
	OptionalCorrelationInputs []string `yaml:"optional_correlation_inputs"`
	PreCallChecklist          []string `yaml:"pre_call_checklist"`
	Returns                   []string `yaml:"returns"`
	FailureRule               string   `yaml:"failure_rule"`
}

type ReportContract struct {
	Intro               string   `yaml:"intro"`
	RequiredInputs      []string `yaml:"required_inputs"`
	OptionalInputs      []string `yaml:"optional_inputs"`
	TerminalOutcomeRule []string `yaml:"terminal_outcome_rule"`
	Rules               []string `yaml:"rules"`
	Returns             []string `yaml:"returns"`
}

type GetEventContract struct {
	Intro    string   `yaml:"intro"`
	Input    []string `yaml:"input"`
	UseCases []string `yaml:"use_cases"`
	Returns  []string `yaml:"returns"`
}

type AgentContract struct {
	Title          string           `yaml:"title"`
	VersionPolicy  string           `yaml:"version_policy"`
	Changelog      []ChangelogEntry `yaml:"changelog"`
	Purpose        []string         `yaml:"purpose"`
	ExecutionRules []string         `yaml:"execution_rules"`
}

type ChangelogEntry struct {
	Version string `yaml:"version"`
	Date    string `yaml:"date"`
	Text    string `yaml:"text"`
}

type Classification struct {
	MutateExamples   []string `yaml:"mutate_examples"`
	ReadOnlyExamples []string `yaml:"read_only_examples"`
}

type RenderedFile struct {
	ID          string
	TemplateRel string
	OutputRel   string
	ActiveRel   string
	Content     string
}

type Manifest struct {
	ContractVersion string            `json:"contract_version"`
	GeneratedAt     string            `json:"generated_at"`
	Files           map[string]string `json:"files"`
}
