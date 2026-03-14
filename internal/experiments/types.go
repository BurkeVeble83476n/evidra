package experiments

import "time"

type ArtifactRunOptions struct {
	ModelID          string
	Provider         string
	PromptVersion    string
	PromptFile       string
	Temperature      *float64
	Mode             string
	Repeats          int
	TimeoutSeconds   int
	CaseFilter       string
	MaxCases         int
	CasesDir         string
	OutDir           string
	CleanOutDir      bool
	DelayBetweenRuns time.Duration
	Agent            string
	DryRun           bool
}

type ArtifactBaselineRunOptions struct {
	ModelIDs         []string
	Provider         string
	PromptVersion    string
	PromptFile       string
	Temperature      *float64
	Mode             string
	Repeats          int
	TimeoutSeconds   int
	CaseFilter       string
	MaxCases         int
	CasesDir         string
	OutDir           string
	CleanOutDir      bool
	DelayBetweenRuns time.Duration
	Agent            string
	DryRun           bool
}

type ArtifactCase struct {
	CaseID              string
	Category            string
	Difficulty          string
	GroundTruthPattern  string
	ExpectedRiskLevel   string
	ExpectedRiskDetails []string
	ArtifactPath        string
	ExpectedJSONPath    string
}

type ArtifactAgentRequest struct {
	Case            ArtifactCase
	ModelID         string
	Provider        string
	Prompt          PromptInfo
	Temperature     *float64
	TimeoutSeconds  int
	RunID           string
	RepeatIndex     int
	RawStreamOut    string
	AgentOutputPath string
}

type ArtifactAgentResult struct {
	Output    map[string]any
	StdoutLog string
	StderrLog string
	RawStream string
}

type RunCounters struct {
	Total    int
	Success  int
	Failure  int
	Timeout  int
	DryRun   int
	EvalPass int
	EvalFail int
}
