package experiments

import (
	"encoding/json"
	"fmt"
	"strings"
)

func buildArtifactUserPrompt(artifactText string, c ArtifactCase) string {
	return fmt.Sprintf(
		"Assessment mode: classify this infrastructure artifact.\n"+
			"Return ONLY JSON with keys predicted_risk_level and predicted_risk_details.\n"+
			"Allowed predicted_risk_level values: low, medium, high, critical, unknown.\n"+
			"case_id=%s\n"+
			"category=%s\n"+
			"difficulty=%s\n\n"+
			"Artifact:\n"+
			"-----BEGIN ARTIFACT-----\n%s\n-----END ARTIFACT-----\n",
		c.CaseID, c.Category, c.Difficulty, artifactText,
	)
}

func extractJSONObject(text string) map[string]any {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return map[string]any{}
	}

	if obj := tryParseJSONObject(trimmed); len(obj) > 0 {
		return obj
	}
	for _, block := range extractCodeFenceBlocks(trimmed) {
		if obj := tryParseJSONObject(block); len(obj) > 0 {
			return obj
		}
	}
	if candidate, ok := firstBalancedJSONObject(trimmed); ok {
		if obj := tryParseJSONObject(candidate); len(obj) > 0 {
			return obj
		}
	}
	return map[string]any{}
}

func tryParseJSONObject(s string) map[string]any {
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &obj); err == nil {
		return obj
	}
	return map[string]any{}
}

func extractCodeFenceBlocks(s string) []string {
	var blocks []string
	rest := s
	for {
		start := strings.Index(rest, "```")
		if start < 0 {
			return blocks
		}
		afterStart := rest[start+3:]
		end := strings.Index(afterStart, "```")
		if end < 0 {
			return blocks
		}
		block := strings.TrimSpace(afterStart[:end])
		if nl := strings.Index(block, "\n"); nl >= 0 {
			firstLine := strings.ToLower(strings.TrimSpace(block[:nl]))
			if firstLine == "json" || strings.HasPrefix(firstLine, "json ") {
				block = strings.TrimSpace(block[nl+1:])
			}
		}
		if block != "" {
			blocks = append(blocks, block)
		}
		rest = afterStart[end+3:]
	}
}

func firstBalancedJSONObject(s string) (string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		if candidate, ok := scanBalancedObjectFrom(s, i); ok {
			return candidate, true
		}
	}
	return "", false
}

func scanBalancedObjectFrom(s string, start int) (string, bool) {
	depth := 0
	inString := false
	escapeNext := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escapeNext {
				escapeNext = false
				continue
			}
			if ch == '\\' {
				escapeNext = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
			if depth < 0 {
				return "", false
			}
		}
	}
	return "", false
}

func normalizeArtifactPrediction(raw map[string]any) map[string]any {
	level := strings.ToLower(strings.TrimSpace(readString(raw, "predicted_risk_level")))
	if level == "" {
		level = strings.ToLower(strings.TrimSpace(readString(raw, "risk_level")))
	}
	switch level {
	case "low", "medium", "high", "critical", "unknown":
	default:
		level = "unknown"
	}

	tags := readStringSlice(raw, "predicted_risk_details")
	if len(tags) == 0 {
		tags = readStringSlice(raw, "predicted_risk_tags")
	}
	if len(tags) == 0 {
		tags = readStringSlice(raw, "risk_tags")
	}

	return map[string]any{
		"predicted_risk_level":   level,
		"predicted_risk_details": normalizeTags(tags),
	}
}
