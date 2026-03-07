package detectors

import (
	"encoding/json"

	"samebits.com/evidra-benchmark/internal/canon"
)

// SARIFProducer extracts tags from SARIF result payloads.
type SARIFProducer struct {
	RuleMapping map[string]string
}

func (p *SARIFProducer) Name() string { return "sarif" }

func (p *SARIFProducer) ProduceTags(_ canon.CanonicalAction, raw []byte) []string {
	var doc struct {
		Runs []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var tags []string
	for _, run := range doc.Runs {
		for _, result := range run.Results {
			tag, ok := p.RuleMapping[result.RuleID]
			if !ok || tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}
