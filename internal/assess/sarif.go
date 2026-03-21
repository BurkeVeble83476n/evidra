package assess

import (
	"context"
	"fmt"
	"strings"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/risk"
	"samebits.com/evidra/pkg/evidence"
)

// FindingsSource represents external security scanner findings.
type FindingsSource struct {
	Source   string
	Findings []evidence.FindingPayload
}

// SARIFAssessor converts external security findings into risk inputs.
type SARIFAssessor struct {
	Sources []FindingsSource
}

// Name returns the assessor name.
func (SARIFAssessor) Name() string { return "sarif" }

// Assess converts each findings source into a risk input. Returns nil if no
// sources are configured.
func (a SARIFAssessor) Assess(_ context.Context, _ canon.CanonicalAction, _ []byte) ([]evidence.RiskInput, error) {
	if len(a.Sources) == 0 {
		return nil, nil
	}

	var inputs []evidence.RiskInput
	for _, src := range a.Sources {
		inputs = append(inputs, buildFindingsRiskInput(src))
	}
	return inputs, nil
}

func buildFindingsRiskInput(src FindingsSource) evidence.RiskInput {
	var tags []string
	maxLevel := "low"
	seen := make(map[string]bool)
	counts := map[string]int{}

	for _, f := range src.Findings {
		severity := strings.ToLower(strings.TrimSpace(f.Severity))
		counts[severity]++

		tag := strings.ToLower(strings.TrimSpace(f.Tool)) + "." + strings.TrimSpace(f.RuleID)
		if tag != "." && !seen[tag] && (severity == "high" || severity == "critical") {
			seen[tag] = true
			tags = append(tags, tag)
		}

		if risk.SeverityHigherThan(severity, maxLevel) {
			maxLevel = severity
		}
	}

	source := strings.TrimSpace(src.Source)
	if source == "" && len(src.Findings) > 0 {
		tool := strings.ToLower(strings.TrimSpace(src.Findings[0].Tool))
		source = tool
		if version := strings.TrimSpace(src.Findings[0].ToolVersion); version != "" {
			source += "/" + version
		}
	}

	return evidence.RiskInput{
		Source:    source,
		RiskLevel: maxLevel,
		RiskTags:  tags,
		Detail:    buildFindingsSummary(len(src.Findings), counts),
	}
}

func buildFindingsSummary(total int, counts map[string]int) string {
	if total == 0 {
		return ""
	}

	var parts []string
	for _, severity := range []string{"critical", "high", "medium", "low"} {
		if counts[severity] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[severity], severity))
		}
	}

	return fmt.Sprintf("%d findings (%s)", total, strings.Join(parts, ", "))
}
