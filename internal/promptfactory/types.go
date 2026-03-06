package promptfactory

type Bundle struct {
	Contract       Contract        `yaml:"contract"`
	Classification Classification  `yaml:"classification"`
	Output         OutputContracts `yaml:"output_contracts"`
}

type Contract struct {
	Version    string   `yaml:"contract_version"`
	Invariants []string `yaml:"invariants"`

	MCP MCPContract `yaml:"mcp"`

	LiteLLM LiteLLMContract `yaml:"litellm"`

	AgentContract AgentContract `yaml:"agent_contract"`
}

type MCPContract struct {
	Initialize InitializeContract `yaml:"initialize"`
	Prescribe  PrescribeContract  `yaml:"prescribe"`
	Report     ReportContract     `yaml:"report"`
	GetEvent   GetEventContract   `yaml:"get_event"`
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

type LiteLLMContract struct {
	SystemIntro                []string `yaml:"system_intro"`
	ExecutionModeRules         []string `yaml:"execution_mode_rules"`
	AssessmentModeRequirements []string `yaml:"assessment_mode_requirements"`
}

type AgentContract struct {
	Title          string           `yaml:"title"`
	VersionPolicy  string           `yaml:"version_policy"`
	Changelog      []ChangelogEntry `yaml:"changelog"`
	Purpose        []string         `yaml:"purpose"`
	ExecutionRules []string         `yaml:"execution_rules"`
	OutputRules    []string         `yaml:"output_rules"`
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

type OutputContracts struct {
	AssessmentJSON AssessmentJSONContract `yaml:"assessment_json"`
}

type AssessmentJSONContract struct {
	LevelField   string   `yaml:"level_field"`
	DetailsField string   `yaml:"details_field"`
	AllowedLevel []string `yaml:"allowed_levels"`
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
