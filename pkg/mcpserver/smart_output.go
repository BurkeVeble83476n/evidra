package mcpserver

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	maxFallbackLen = 2000
	maxErrorLen    = 500
	maxLogLines    = 50
)

// FormatSmartOutput takes a command, its raw output, and exit code,
// and returns a concise, token-efficient summary.
func FormatSmartOutput(command string, rawOutput string, exitCode int) string {
	if exitCode != 0 {
		return formatError(rawOutput, exitCode)
	}

	cmd := strings.TrimSpace(command)
	fields := commandFields(cmd)

	// JSON/YAML output requested explicitly — strip noisy fields, pass through.
	if hasOutputFlag(fields, "json") {
		return stripJSONNoise(rawOutput)
	}
	if hasOutputFlag(fields, "yaml") {
		return truncateString(rawOutput, maxFallbackLen)
	}

	if isGetDeployments(fields) {
		return formatGetDeployments(rawOutput)
	}
	if isGetPods(fields) {
		return formatGetPods(rawOutput)
	}
	if isDescribe(fields) {
		return formatDescribe(rawOutput)
	}
	if isLogs(fields) {
		return formatLogs(fields, rawOutput)
	}
	if isTerraformPlan(fields) {
		return formatTerraformPlan(rawOutput)
	}
	if isTerraformApply(fields) {
		return formatTerraformApply(rawOutput)
	}
	if isHelmStatus(fields) {
		return formatHelmStatus(rawOutput)
	}
	if isHelmList(fields) {
		return formatHelmList(rawOutput)
	}
	if isEKSUpdateKubeconfig(fields) {
		return formatEKSKubeconfig(rawOutput)
	}

	return truncateString(rawOutput, maxFallbackLen)
}

// hasOutputFlag checks whether the command contains -o <format> or --output=<format>.
func hasOutputFlag(fields []string, format string) bool {
	for i := range fields {
		switch {
		case fields[i] == "-o" || fields[i] == "--output":
			if i+1 < len(fields) && strings.EqualFold(fields[i+1], format) {
				return true
			}
		case strings.HasPrefix(fields[i], "-o="):
			if strings.EqualFold(strings.TrimPrefix(fields[i], "-o="), format) {
				return true
			}
		case strings.HasPrefix(fields[i], "--output="):
			if strings.EqualFold(strings.TrimPrefix(fields[i], "--output="), format) {
				return true
			}
		case strings.HasPrefix(fields[i], "-o") && len(fields[i]) > 2:
			if strings.EqualFold(strings.TrimPrefix(fields[i], "-o"), format) {
				return true
			}
		}
	}
	return false
}

// --- command classifiers ---

func isGetDeployments(fields []string) bool {
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "get" {
		return false
	}
	res := strings.ToLower(fields[2])
	return res == "deployment" || res == "deployments" ||
		res == "deploy" || res == "deployment.apps" || res == "deployments.apps"
}

func isGetPods(fields []string) bool {
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "get" {
		return false
	}
	res := strings.ToLower(fields[2])
	return res == "pod" || res == "pods" || res == "po"
}

func isDescribe(fields []string) bool {
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "describe" {
		return false
	}
	return true
}

func isLogs(fields []string) bool {
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "logs" {
		return false
	}
	return true
}

func isTerraformPlan(fields []string) bool {
	return len(fields) >= 2 && fields[0] == "terraform" && fields[1] == "plan"
}

func isTerraformApply(fields []string) bool {
	return len(fields) >= 2 && fields[0] == "terraform" && fields[1] == "apply"
}

func isHelmStatus(fields []string) bool {
	return len(fields) >= 2 && fields[0] == "helm" && fields[1] == "status"
}

func isHelmList(fields []string) bool {
	return len(fields) >= 2 && fields[0] == "helm" && fields[1] == "list"
}

func isEKSUpdateKubeconfig(fields []string) bool {
	return len(fields) >= 3 && fields[0] == "aws" && fields[1] == "eks" && fields[2] == "update-kubeconfig"
}

// --- formatters ---

func formatError(rawOutput string, exitCode int) string {
	output := strings.TrimSpace(rawOutput)
	if len(output) > maxErrorLen {
		output = output[:maxErrorLen] + "..."
	}
	return fmt.Sprintf("error (exit code %d):\n%s", exitCode, output)
}

func formatGetDeployments(rawOutput string) string {
	lines := strings.Split(strings.TrimSpace(rawOutput), "\n")
	if len(lines) < 2 {
		return rawOutput
	}

	headers := parseTableHeaders(lines[0])
	nameIdx := findColumn(headers, "NAME")
	readyIdx := findColumn(headers, "READY")
	imageIdx := findColumn(headers, "CONTAINERS", "IMAGE") // images column sometimes named differently
	nsIdx := findColumn(headers, "NAMESPACE")

	var result []string
	for _, line := range lines[1:] {
		cols := splitTableRow(line, len(headers))
		name := safeCol(cols, nameIdx)
		ready := safeCol(cols, readyIdx)
		ns := safeCol(cols, nsIdx)

		loc := name
		if ns != "" {
			loc = fmt.Sprintf("%s (%s)", name, ns)
		}

		// Try to find image from the raw output table if present
		image := safeCol(cols, imageIdx)
		entry := fmt.Sprintf("deployment/%s: %s ready", loc, ready)
		if image != "" {
			entry += " | image: " + image
		}
		result = append(result, entry)
	}
	return strings.Join(result, "\n")
}

func formatGetPods(rawOutput string) string {
	lines := strings.Split(strings.TrimSpace(rawOutput), "\n")
	if len(lines) < 2 {
		return rawOutput
	}

	headers := parseTableHeaders(lines[0])
	nameIdx := findColumn(headers, "NAME")
	statusIdx := findColumn(headers, "STATUS")
	readyIdx := findColumn(headers, "READY")
	nsIdx := findColumn(headers, "NAMESPACE")

	// Count statuses.
	statusCounts := map[string]int{}
	type podInfo struct {
		name   string
		status string
		ready  string
	}
	var pods []podInfo

	for _, line := range lines[1:] {
		cols := splitTableRow(line, len(headers))
		name := safeCol(cols, nameIdx)
		status := safeCol(cols, statusIdx)
		ready := safeCol(cols, readyIdx)
		_ = safeCol(cols, nsIdx)

		statusCounts[status]++
		pods = append(pods, podInfo{name: name, status: status, ready: ready})
	}

	total := len(pods)
	var summary []string
	summary = append(summary, fmt.Sprintf("pods: %d total", total))
	for status, count := range statusCounts {
		summary = append(summary, fmt.Sprintf("%d %s", count, strings.ToLower(status)))
	}
	header := strings.Join(summary, ", ")

	var details []string
	for _, p := range pods {
		readySuffix := ""
		if p.ready != "" {
			readySuffix = fmt.Sprintf(" (%s ready)", p.ready)
		}
		details = append(details, fmt.Sprintf("  %s: %s%s", p.name, p.status, readySuffix))
	}

	return header + "\n" + strings.Join(details, "\n")
}

func formatDescribe(rawOutput string) string {
	lines := strings.Split(rawOutput, "\n")
	var result []string
	inEvents := false
	eventCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case isDescribeKeyField(trimmed, inEvents):
			result = append(result, trimmed)
		case trimmed == "Conditions:":
			result = append(result, "conditions:")
		case trimmed == "Events:":
			inEvents = true
			eventCount = 0
			result = append(result, "events (last 5):")
		case inEvents:
			inEvents, eventCount, result = handleEventLine(trimmed, eventCount, result)
		case isConditionRow(trimmed):
			result = append(result, "  "+trimmed)
		}
	}

	if len(result) == 0 {
		return truncateString(rawOutput, maxFallbackLen)
	}
	return strings.Join(result, "\n")
}

func handleEventLine(trimmed string, eventCount int, result []string) (bool, int, []string) {
	if trimmed == "" {
		return false, eventCount, result
	}
	if isDescribeHeaderRow(trimmed) {
		return true, eventCount, result
	}
	eventCount++
	if eventCount <= 5 {
		result = append(result, "  "+trimmed)
	}
	return true, eventCount, result
}

func isDescribeKeyField(line string, inEvents bool) bool {
	prefixes := []string{"Name:", "Namespace:", "Replicas:", "Image:", "Status:", "Ready:", "Restart Count:"}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return strings.HasPrefix(line, "Type:") && inEvents
}

func isDescribeHeaderRow(line string) bool {
	return strings.HasPrefix(line, "Type") && strings.Contains(line, "Reason")
}

var conditionPrefixes = []string{"Available", "Progressing", "Ready", "PodScheduled", "Initialized", "ContainersReady"}

func isConditionRow(line string) bool {
	for _, p := range conditionPrefixes {
		if strings.HasPrefix(line, p) && (strings.Contains(line, "True") || strings.Contains(line, "False")) {
			return true
		}
	}
	return false
}

func formatLogs(fields []string, rawOutput string) string {
	lines := strings.Split(strings.TrimSpace(rawOutput), "\n")
	total := len(lines)

	// Take the last maxLogLines lines.
	start := 0
	if total > maxLogLines {
		start = total - maxLogLines
	}
	kept := lines[start:]

	// Extract pod name from command.
	podName := ""
	if len(fields) >= 3 {
		podName = fields[2]
	}

	header := fmt.Sprintf("logs %s (last %d lines, %d total):", podName, len(kept), total)
	return header + "\n" + strings.Join(kept, "\n")
}

func formatTerraformPlan(rawOutput string) string {
	for _, line := range strings.Split(strings.TrimSpace(rawOutput), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Plan:") {
			return "terraform plan: " + strings.TrimPrefix(trimmed, "Plan: ")
		}
		if trimmed == "No changes. Your infrastructure matches the configuration." {
			return "terraform plan: no changes"
		}
	}
	return truncateString(rawOutput, maxFallbackLen)
}

func formatTerraformApply(rawOutput string) string {
	for _, line := range strings.Split(strings.TrimSpace(rawOutput), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Apply complete! Resources:") {
			return "terraform apply complete: " + strings.TrimPrefix(trimmed, "Apply complete! Resources: ")
		}
		if trimmed == "No changes. Your infrastructure matches the configuration." {
			return "terraform apply: no changes"
		}
	}
	return truncateString(rawOutput, maxFallbackLen)
}

func formatHelmStatus(rawOutput string) string {
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(rawOutput), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		values[key] = strings.TrimSpace(parts[1])
	}
	name := values["NAME"]
	status := values["STATUS"]
	if name == "" || status == "" {
		return truncateString(rawOutput, maxFallbackLen)
	}
	lines := []string{fmt.Sprintf("helm release %s: %s", name, strings.ToLower(status))}
	if ns := values["NAMESPACE"]; ns != "" {
		lines = append(lines, "namespace: "+ns)
	}
	if rev := values["REVISION"]; rev != "" {
		lines = append(lines, "revision: "+rev)
	}
	if deployed := values["LAST DEPLOYED"]; deployed != "" {
		lines = append(lines, "last deployed: "+deployed)
	}
	return strings.Join(lines, "\n")
}

func formatHelmList(rawOutput string) string {
	lines := strings.Split(strings.TrimSpace(rawOutput), "\n")
	if len(lines) < 2 {
		return truncateString(rawOutput, maxFallbackLen)
	}

	var result []string
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		name := fields[0]
		namespace := fields[1]
		revision := fields[2]
		status := fields[len(fields)-3]
		chart := fields[len(fields)-2]

		entry := fmt.Sprintf("release/%s (%s): %s | rev %s", name, namespace, strings.ToLower(status), revision)
		if chart != "" {
			entry += " | chart " + chart
		}
		result = append(result, entry)
	}
	if len(result) == 0 {
		return truncateString(rawOutput, maxFallbackLen)
	}
	return strings.Join(result, "\n")
}

func formatEKSKubeconfig(rawOutput string) string {
	output := strings.TrimSpace(rawOutput)
	for _, prefix := range []string{"Added new context ", "Updated context "} {
		if !strings.HasPrefix(output, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(output, prefix)
		for _, separator := range []string{" to ", " in "} {
			if idx := strings.Index(remainder, separator); idx >= 0 {
				return "kubeconfig updated: " + remainder[:idx]
			}
		}
		return "kubeconfig updated: " + remainder
	}
	return truncateString(rawOutput, maxFallbackLen)
}

// --- JSON noise stripping ---

// stripJSONNoise removes verbose metadata fields from kubectl JSON output.
func stripJSONNoise(rawOutput string) string {
	var data any
	if err := json.Unmarshal([]byte(rawOutput), &data); err != nil {
		return truncateString(rawOutput, maxFallbackLen)
	}

	stripNoiseFields(data)

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return truncateString(rawOutput, maxFallbackLen)
	}
	return string(out)
}

var noiseMetadataKeys = map[string]bool{
	"managedFields":     true,
	"uid":               true,
	"resourceVersion":   true,
	"generation":        true,
	"creationTimestamp": true,
}

const lastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

func stripNoiseFields(v any) {
	switch val := v.(type) {
	case map[string]any:
		// Remove noisy metadata keys.
		if meta, ok := val["metadata"].(map[string]any); ok {
			for key := range noiseMetadataKeys {
				delete(meta, key)
			}
			if annotations, ok := meta["annotations"].(map[string]any); ok {
				delete(annotations, lastAppliedConfigAnnotation)
				if len(annotations) == 0 {
					delete(meta, "annotations")
				}
			}
		}
		// Also strip managedFields at top level (some outputs).
		delete(val, "managedFields")

		for _, child := range val {
			stripNoiseFields(child)
		}
	case []any:
		for _, item := range val {
			stripNoiseFields(item)
		}
	}
}

// --- table parsing helpers ---

func parseTableHeaders(headerLine string) []string {
	// kubectl tables use whitespace-delimited columns.
	return strings.Fields(headerLine)
}

func findColumn(headers []string, names ...string) int {
	for i, h := range headers {
		for _, name := range names {
			if strings.EqualFold(h, name) {
				return i
			}
		}
	}
	return -1
}

func splitTableRow(line string, numCols int) []string {
	fields := strings.Fields(line)
	if len(fields) >= numCols {
		return fields
	}
	return fields
}

func safeCol(cols []string, idx int) string {
	if idx < 0 || idx >= len(cols) {
		return ""
	}
	return cols[idx]
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
