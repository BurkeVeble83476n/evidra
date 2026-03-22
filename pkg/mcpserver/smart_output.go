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

	// JSON/YAML output requested explicitly — strip noisy fields, pass through.
	if hasOutputFlag(cmd, "json") {
		return stripJSONNoise(rawOutput)
	}
	if hasOutputFlag(cmd, "yaml") {
		return truncateString(rawOutput, maxFallbackLen)
	}

	if isGetDeployments(cmd) {
		return formatGetDeployments(rawOutput)
	}
	if isGetPods(cmd) {
		return formatGetPods(rawOutput)
	}
	if isDescribe(cmd) {
		return formatDescribe(rawOutput)
	}
	if isLogs(cmd) {
		return formatLogs(cmd, rawOutput)
	}

	return truncateString(rawOutput, maxFallbackLen)
}

// hasOutputFlag checks whether the command contains -o <format> or --output=<format>.
func hasOutputFlag(cmd, format string) bool {
	for _, pattern := range []string{
		"-o " + format,
		"-o=" + format,
		"--output " + format,
		"--output=" + format,
	} {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}
	return false
}

// --- command classifiers ---

func isGetDeployments(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "get" {
		return false
	}
	res := strings.ToLower(fields[2])
	return res == "deployment" || res == "deployments" ||
		res == "deploy" || res == "deployment.apps" || res == "deployments.apps"
}

func isGetPods(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "get" {
		return false
	}
	res := strings.ToLower(fields[2])
	return res == "pod" || res == "pods" || res == "po"
}

func isDescribe(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "describe" {
		return false
	}
	return true
}

func isLogs(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 3 || fields[0] != "kubectl" || fields[1] != "logs" {
		return false
	}
	return true
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

func formatLogs(cmd, rawOutput string) string {
	lines := strings.Split(strings.TrimSpace(rawOutput), "\n")
	total := len(lines)

	// Take the last maxLogLines lines.
	start := 0
	if total > maxLogLines {
		start = total - maxLogLines
	}
	kept := lines[start:]

	// Extract pod name from command.
	fields := strings.Fields(cmd)
	podName := ""
	if len(fields) >= 3 {
		podName = fields[2]
	}

	header := fmt.Sprintf("logs %s (last %d lines, %d total):", podName, len(kept), total)
	return header + "\n" + strings.Join(kept, "\n")
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
