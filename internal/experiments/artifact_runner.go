package experiments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RunArtifact(ctx context.Context, opts ArtifactRunOptions, stdout, stderr io.Writer) error {
	userProvidedOutDir := strings.TrimSpace(opts.OutDir) != ""
	opts = withArtifactDefaults(opts)
	if err := validateArtifactOptions(opts); err != nil {
		return err
	}

	promptInfo, err := resolvePromptInfo(opts.PromptFile, opts.PromptVersion)
	if err != nil {
		return err
	}

	if opts.CleanOutDir {
		cleanTarget := opts.OutDir
		if !userProvidedOutDir {
			cleanTarget = DefaultArtifactOutRoot
		}
		if err := ensureDirClean(cleanTarget); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return fmt.Errorf("mkdir out dir: %w", err)
	}
	summaryPath := filepath.Join(opts.OutDir, "summary.jsonl")
	if err := os.WriteFile(summaryPath, nil, 0o644); err != nil {
		return err
	}

	cases, err := loadArtifactCases(opts.CasesDir, opts.CaseFilter, opts.MaxCases)
	if err != nil {
		return err
	}

	agent, err := newArtifactAgent(opts.Agent)
	if err != nil {
		return err
	}
	useDryStatus := opts.DryRun || agent.Name() == "dry-run"

	fmt.Fprintf(stdout,
		"run-agent-experiments: selected_cases=%d repeats=%d out_dir=%s prompt_version=%s prompt_file=%s delay_between_runs=%s\n",
		len(cases), opts.Repeats, opts.OutDir, promptInfo.Version, promptInfo.File, opts.DelayBetweenRuns,
	)

	counters := RunCounters{}
	statusCounts := map[string]int{}
	runStamp := runStampNow()
	totalRuns := len(cases) * opts.Repeats
	runIndex := 0

	for _, c := range cases {
		for repeat := 1; repeat <= opts.Repeats; repeat++ {
			counters.Total++
			runIndex++
			runID := fmt.Sprintf("%s-%s-%s-r%d", runStamp, safeModelID(opts.ModelID), c.CaseID, repeat)
			oneStatus, evalPass, err := runArtifactCase(ctx, opts, promptInfo, agent, c, repeat, runID, summaryPath, useDryStatus)
			if err != nil {
				return err
			}
			statusCounts[oneStatus]++
			applyStatus(&counters, oneStatus)
			if evalPass {
				counters.EvalPass++
			} else {
				counters.EvalFail++
			}
			fmt.Fprintf(stdout, "run-agent-experiments: run_id=%s status=%s pass=%v result=%s\n", runID, oneStatus, evalPass, filepath.Join(opts.OutDir, runID, "result.json"))
			if runIndex < totalRuns && opts.DelayBetweenRuns > 0 {
				fmt.Fprintf(stdout, "run-agent-experiments: sleeping_between_runs=%s\n", opts.DelayBetweenRuns)
				if err := sleepWithContext(ctx, opts.DelayBetweenRuns); err != nil {
					return err
				}
			}
		}
	}

	printArtifactSummary(stdout, counters, summaryPath, statusCounts)
	return nil
}

func withArtifactDefaults(opts ArtifactRunOptions) ArtifactRunOptions {
	if opts.Provider == "" {
		opts.Provider = "unknown"
	}
	if opts.PromptFile == "" {
		opts.PromptFile = DefaultPromptFile
	}
	if opts.CasesDir == "" {
		opts.CasesDir = DefaultArtifactCasesDir
	}
	if opts.Mode == "" {
		opts.Mode = "custom"
	}
	if opts.Repeats <= 0 {
		opts.Repeats = 3
	}
	if opts.TimeoutSeconds <= 0 {
		opts.TimeoutSeconds = 300
	}
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join(DefaultArtifactOutRoot, runStampNow())
	}
	if opts.Agent == "" {
		if opts.DryRun {
			opts.Agent = "dry-run"
		}
	}
	return opts
}

func validateArtifactOptions(opts ArtifactRunOptions) error {
	if opts.ModelID == "" {
		return errors.New("--model-id is required")
	}
	if opts.Agent == "" {
		return errors.New("--agent is required")
	}
	if opts.Repeats < 1 {
		return errors.New("--repeats must be >= 1")
	}
	if opts.TimeoutSeconds < 1 {
		return errors.New("--timeout-seconds must be >= 1")
	}
	if opts.DelayBetweenRuns < 0 {
		return errors.New("--delay-between-runs must be >= 0")
	}
	info, err := os.Stat(opts.CasesDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("cases dir not found: %s", opts.CasesDir)
	}
	if _, err := os.Stat(opts.PromptFile); err != nil {
		return fmt.Errorf("prompt file not found: %s", opts.PromptFile)
	}
	return nil
}

func runArtifactCase(
	parent context.Context,
	opts ArtifactRunOptions,
	prompt PromptInfo,
	agent ArtifactAgent,
	c ArtifactCase,
	repeat int,
	runID string,
	summaryPath string,
	forceDry bool,
) (string, bool, error) {
	runDir, stdoutPath, stderrPath, outputPath, rawPath, resultPath := runPaths(opts.OutDir, runID)
	start := time.Now().UTC()
	result, status, agentExitCode := executeArtifactAgent(parent, opts, prompt, agent, c, repeat, runID, rawPath, outputPath, forceDry)
	output := ensureObjectOutput(result.Output, map[string]any{
		"predicted_risk_level":   "",
		"predicted_risk_details": []string{},
		"status":                 status,
		"exit_code":              agentExitCode,
	})

	if err := writeRunArtifacts(runDir, stdoutPath, stderrPath, outputPath, rawPath, result.StdoutLog, result.StderrLog, output, result.RawStream); err != nil {
		return "", false, err
	}

	predictedLevel, predictedTags := extractRiskPrediction(output)
	eval := evaluateArtifact(c.ExpectedRiskLevel, predictedLevel, c.ExpectedRiskDetails, predictedTags)

	end := time.Now().UTC()
	resultObj := buildArtifactResult(resultPath, runID, opts, prompt, c, runDir, stdoutPath, stderrPath, outputPath, rawPath, status, agentExitCode, repeat, start, end, eval)
	if err := writeJSONFile(resultPath, resultObj); err != nil {
		return "", false, err
	}

	summaryRow := map[string]any{
		"run_id":      runID,
		"case_id":     c.CaseID,
		"status":      status,
		"pass":        eval.Pass,
		"result_json": resultPath,
	}
	if err := writeJSONL(summaryPath, summaryRow); err != nil {
		return "", false, err
	}
	return status, eval.Pass, nil
}

func executeArtifactAgent(
	parent context.Context,
	opts ArtifactRunOptions,
	prompt PromptInfo,
	agent ArtifactAgent,
	c ArtifactCase,
	repeat int,
	runID string,
	rawPath string,
	outputPath string,
	forceDry bool,
) (ArtifactAgentResult, string, int) {
	if forceDry {
		res, _ := (&dryRunAgent{}).RunArtifact(parent, ArtifactAgentRequest{})
		return res, "dry_run", 0
	}

	ctx, cancel := context.WithTimeout(parent, time.Duration(opts.TimeoutSeconds)*time.Second)
	defer cancel()
	req := ArtifactAgentRequest{
		Case:            c,
		ModelID:         opts.ModelID,
		Provider:        opts.Provider,
		Prompt:          prompt,
		Temperature:     opts.Temperature,
		TimeoutSeconds:  opts.TimeoutSeconds,
		RunID:           runID,
		RepeatIndex:     repeat,
		RawStreamOut:    rawPath,
		AgentOutputPath: outputPath,
	}
	res, err := agent.RunArtifact(ctx, req)
	if err == nil {
		return res, "success", 0
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return res, "timeout", 124
	}
	if res.StderrLog == "" {
		res.StderrLog = err.Error() + "\n"
	}
	return res, "failure", 1
}

func buildArtifactResult(
	resultPath, runID string,
	opts ArtifactRunOptions,
	prompt PromptInfo,
	c ArtifactCase,
	runDir, stdoutPath, stderrPath, outputPath, rawPath, status string,
	agentExitCode, repeat int,
	start, end time.Time,
	eval ArtifactEvaluation,
) map[string]any {
	return map[string]any{
		"schema_version":   ResultSchemaVersion,
		"run_id":           runID,
		"started_at":       start.Format(time.RFC3339),
		"finished_at":      end.Format(time.RFC3339),
		"duration_seconds": int(end.Sub(start).Seconds()),
		"mode":             opts.Mode,
		"model": map[string]any{
			"id":                      opts.ModelID,
			"provider":                opts.Provider,
			"prompt_version":          prompt.Version,
			"prompt_file":             prompt.File,
			"prompt_contract_version": prompt.ContractVersion,
			"temperature":             opts.Temperature,
			"repeat_index":            repeat,
		},
		"case": map[string]any{
			"id":                    c.CaseID,
			"category":              c.Category,
			"difficulty":            c.Difficulty,
			"ground_truth_pattern":  c.GroundTruthPattern,
			"expected_risk_level":   c.ExpectedRiskLevel,
			"expected_risk_details": c.ExpectedRiskDetails,
			"artifact_path":         c.ArtifactPath,
			"expected_json_path":    c.ExpectedJSONPath,
		},
		"execution": map[string]any{
			"agent_cmd":       "agent:" + opts.Agent,
			"timeout_seconds": opts.TimeoutSeconds,
			"status":          status,
			"exit_code":       agentExitCode,
		},
		"artifacts": map[string]any{
			"run_dir":          runDir,
			"stdout_log":       stdoutPath,
			"stderr_log":       stderrPath,
			"agent_output":     outputPath,
			"agent_raw_stream": rawPath,
			"result_json":      resultPath,
		},
		"evaluation": eval,
	}
}

func applyStatus(counters *RunCounters, status string) {
	switch status {
	case "success":
		counters.Success++
	case "failure":
		counters.Failure++
	case "timeout":
		counters.Timeout++
	case "dry_run":
		counters.DryRun++
	}
}

func printArtifactSummary(stdout io.Writer, counters RunCounters, summaryPath string, statusCounts map[string]int) {
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "run-agent-experiments: summary")
	fmt.Fprintf(stdout, "  total:    %d\n", counters.Total)
	fmt.Fprintf(stdout, "  success:  %d\n", counters.Success)
	fmt.Fprintf(stdout, "  failure:  %d\n", counters.Failure)
	fmt.Fprintf(stdout, "  timeout:  %d\n", counters.Timeout)
	fmt.Fprintf(stdout, "  dry_run:  %d\n", counters.DryRun)
	fmt.Fprintf(stdout, "  eval_pass:%d\n", counters.EvalPass)
	fmt.Fprintf(stdout, "  eval_fail:%d\n", counters.EvalFail)
	fmt.Fprintf(stdout, "  index:    %s\n", summaryPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "run-agent-experiments: top statuses from summary")
	writeStatusCounts(stdout, statusCounts)
}

func writeStatusCounts(stdout io.Writer, counts map[string]int) {
	type row struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	rows := make([]row, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, row{Status: k, Count: v})
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		fmt.Fprintln(stdout, "[]")
		return
	}
	fmt.Fprintln(stdout, string(b))
}
