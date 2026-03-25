package mcpserver

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// safeK8sName matches valid Kubernetes resource names and workload references like "deployment/web".
var safeK8sName = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)

// CollectDiagnosticsInput is the input for the collect_diagnostics tool.
type CollectDiagnosticsInput struct {
	Namespace   string `json:"namespace"`
	Workload    string `json:"workload"`
	IncludeLogs bool   `json:"include_logs,omitempty"`
}

// CollectDiagnosticsFinding captures one machine-readable diagnostic hint.
type CollectDiagnosticsFinding struct {
	Source  string `json:"source"`
	Summary string `json:"summary"`
}

// CollectDiagnosticsOutput is the output for the collect_diagnostics tool.
type CollectDiagnosticsOutput struct {
	OK       bool                        `json:"ok"`
	Summary  string                      `json:"summary"`
	Findings []CollectDiagnosticsFinding `json:"findings,omitempty"`
	Commands []string                    `json:"commands,omitempty"`
	Error    string                      `json:"error,omitempty"`
}

type diagnosticsRunFunc func(context.Context, string) RunCommandOutput

type collectDiagnosticsHandler struct {
	run diagnosticsRunFunc
}

var collectDiagnosticsInputSchema = map[string]any{
	"type":     "object",
	"required": []any{"namespace", "workload"},
	"properties": map[string]any{
		"namespace": map[string]any{
			"type":        "string",
			"description": "Kubernetes namespace to inspect",
		},
		"workload": map[string]any{
			"type":        "string",
			"description": "Workload to inspect, for example deployment/web",
		},
		"include_logs": map[string]any{
			"type":        "boolean",
			"description": "Always collect recent logs when a matching failing pod is found",
		},
	},
}

func (h *collectDiagnosticsHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input CollectDiagnosticsInput,
) (*mcp.CallToolResult, CollectDiagnosticsOutput, error) {
	namespace := strings.TrimSpace(input.Namespace)
	workload := strings.TrimSpace(input.Workload)
	if namespace == "" {
		return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{OK: false, Error: "namespace is required"}, nil
	}
	if workload == "" {
		return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{OK: false, Error: "workload is required"}, nil
	}
	if !safeK8sName.MatchString(namespace) {
		return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{OK: false, Error: "namespace contains invalid characters"}, nil
	}
	if !safeK8sName.MatchString(workload) {
		return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{OK: false, Error: "workload contains invalid characters"}, nil
	}
	if h.run == nil {
		return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{OK: false, Error: "diagnostics runner is not configured"}, nil
	}

	commands := []string{
		fmt.Sprintf("kubectl get pods -n %s", namespace),
		fmt.Sprintf("kubectl describe %s -n %s", workload, namespace),
		fmt.Sprintf("kubectl get events -n %s --sort-by=.lastTimestamp", namespace),
	}
	executed := make([]string, 0, len(commands)+1)
	outputs := make(map[string]RunCommandOutput, len(commands)+1)

	for _, command := range commands {
		out := h.run(ctx, command)
		executed = append(executed, command)
		outputs[command] = out
		if !out.OK {
			return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{
				OK:       false,
				Commands: executed,
				Error:    formatDiagnosticsCommandError(command, out),
			}, nil
		}
	}

	podsCommand := commands[0]
	describeCommand := commands[1]
	eventsCommand := commands[2]

	findings := make([]CollectDiagnosticsFinding, 0, 8)
	findings = append(findings, collectDiagnosticsFindings("pods", outputs[podsCommand].Output)...)
	findings = append(findings, collectDiagnosticsFindings("describe", outputs[describeCommand].Output)...)
	findings = append(findings, collectDiagnosticsFindings("events", outputs[eventsCommand].Output)...)

	if input.IncludeLogs || shouldCollectLogs(outputs[podsCommand].Output, outputs[describeCommand].Output, outputs[eventsCommand].Output) {
		if pod := selectDiagnosticPod(workload, outputs[podsCommand].Output); pod != "" {
			logsCommand := fmt.Sprintf("kubectl logs %s -n %s --tail=50", pod, namespace)
			out := h.run(ctx, logsCommand)
			executed = append(executed, logsCommand)
			outputs[logsCommand] = out
			if !out.OK {
				return &mcp.CallToolResult{IsError: true}, CollectDiagnosticsOutput{
					OK:       false,
					Commands: executed,
					Error:    formatDiagnosticsCommandError(logsCommand, out),
				}, nil
			}
			findings = append(findings, collectDiagnosticsFindings("logs", out.Output)...)
		}
	}

	findings = dedupeDiagnosticFindings(findings)
	summary := renderDiagnosticsSummary(namespace, workload, findings)

	return &mcp.CallToolResult{}, CollectDiagnosticsOutput{
		OK:       true,
		Summary:  summary,
		Findings: findings,
		Commands: executed,
	}, nil
}

func RegisterCollectDiagnostics(server *mcp.Server, svc *MCPService, kubeconfigPath string) {
	runCommand := &runCommandHandler{
		service:         svc,
		kubeconfigPath:  kubeconfigPath,
		allowedPrefixes: defaultAllowedPrefixes,
		blockedSubs:     defaultBlockedSubcommands,
	}
	handler := &collectDiagnosticsHandler{
		run: func(ctx context.Context, command string) RunCommandOutput {
			return runCommand.execute(ctx, RunCommandInput{Command: command})
		},
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "collect_diagnostics",
		Title:       "Collect Kubernetes Diagnostics",
		Description: "Run a fixed Kubernetes diagnosis sequence for one workload: get pods, describe the workload, inspect recent events, and fetch logs when a failing pod needs more context.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Collect Diagnostics",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(true),
		},
		InputSchema: collectDiagnosticsInputSchema,
	}, handler.Handle)
}

func formatDiagnosticsCommandError(command string, output RunCommandOutput) string {
	msg := strings.TrimSpace(output.Error)
	if msg == "" {
		msg = strings.TrimSpace(output.Output)
	}
	if msg == "" {
		msg = fmt.Sprintf("exit_code=%d", output.ExitCode)
	}
	return fmt.Sprintf("%s failed: %s", command, msg)
}

func shouldCollectLogs(outputs ...string) bool {
	joined := strings.ToLower(strings.Join(outputs, "\n"))
	for _, marker := range []string{
		"crashloopbackoff",
		"imagepullbackoff",
		"errimagepull",
		"failedpull",
		"back-off",
		"oomkilled",
		"error",
		"panic",
		"unhealthy",
		"failed",
	} {
		if strings.Contains(joined, marker) {
			return true
		}
	}
	return false
}

func selectDiagnosticPod(workload, podsOutput string) string {
	workloadName := workloadResourceName(workload)
	lines := strings.Split(strings.TrimSpace(podsOutput), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, ":") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		name := strings.TrimSpace(parts[0])
		status := strings.ToLower(strings.TrimSpace(parts[1]))
		if workloadName != "" && !strings.HasPrefix(name, workloadName) {
			continue
		}
		if status == "" || strings.Contains(status, "running") || strings.Contains(status, "completed") {
			continue
		}
		return name
	}
	for idx, line := range lines {
		if idx == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		status := strings.ToLower(fields[2])
		if workloadName != "" && !strings.HasPrefix(name, workloadName) {
			continue
		}
		if status == "running" || status == "completed" {
			continue
		}
		return name
	}
	return ""
}

func workloadResourceName(workload string) string {
	trimmed := strings.TrimSpace(workload)
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 && idx+1 < len(trimmed) {
		return trimmed[idx+1:]
	}
	return trimmed
}

func collectDiagnosticsFindings(source, output string) []CollectDiagnosticsFinding {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	out := make([]CollectDiagnosticsFinding, 0, 3)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !diagnosticSignalLine(trimmed) {
			continue
		}
		out = append(out, CollectDiagnosticsFinding{Source: source, Summary: trimmed})
		if len(out) == 3 {
			break
		}
	}
	return out
}

func diagnosticSignalLine(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{
		"crashloopbackoff",
		"imagepullbackoff",
		"errimagepull",
		"failedpull",
		"back-off",
		"oomkilled",
		"panic",
		"error",
		"failed",
		"warning",
		"unhealthy",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func dedupeDiagnosticFindings(findings []CollectDiagnosticsFinding) []CollectDiagnosticsFinding {
	if len(findings) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(findings))
	out := make([]CollectDiagnosticsFinding, 0, len(findings))
	for _, finding := range findings {
		key := finding.Source + "\n" + finding.Summary
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

func renderDiagnosticsSummary(namespace, workload string, findings []CollectDiagnosticsFinding) string {
	lines := []string{fmt.Sprintf("diagnostics for %s in %s", workload, namespace)}
	if len(findings) == 0 {
		lines = append(lines, "- no obvious issues found from get/describe/events")
	} else {
		limit := len(findings)
		if limit > 5 {
			limit = 5
		}
		for _, finding := range findings[:limit] {
			lines = append(lines, fmt.Sprintf("- %s: %s", finding.Source, finding.Summary))
		}
	}
	lines = append(lines, "next checks: make one targeted fix, then verify with kubectl rollout status or kubectl get pods")
	return strings.Join(lines, "\n")
}
