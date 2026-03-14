package experiments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type expectedFilePayload struct {
	CaseID             string   `json:"case_id"`
	Category           string   `json:"category"`
	Difficulty         string   `json:"difficulty"`
	GroundTruthPattern string   `json:"ground_truth_pattern"`
	ArtifactRef        string   `json:"artifact_ref"`
	RiskLevel          string   `json:"risk_level"`
	RiskDetails        []string `json:"risk_details_expected"`
}

func loadArtifactCases(casesDir string, caseFilter string, maxCases int) ([]ArtifactCase, error) {
	pattern, err := compileOptionalRegex(caseFilter)
	if err != nil {
		return nil, fmt.Errorf("compile case filter: %w", err)
	}

	expectedFiles, err := listFilesByName(casesDir, "expected.json")
	if err != nil {
		return nil, fmt.Errorf("list expected files: %w", err)
	}

	cases := make([]ArtifactCase, 0, len(expectedFiles))
	for _, expectedPath := range expectedFiles {
		c, ok := parseArtifactCase(expectedPath, pattern)
		if !ok {
			continue
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].CaseID < cases[j].CaseID })

	if maxCases > 0 && len(cases) > maxCases {
		cases = cases[:maxCases]
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no artifact cases selected")
	}
	return cases, nil
}

func parseArtifactCase(expectedPath string, filterPattern *regexp.Regexp) (ArtifactCase, bool) {
	var payload expectedFilePayload
	b, err := os.ReadFile(expectedPath)
	if err != nil {
		return ArtifactCase{}, false
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return ArtifactCase{}, false
	}
	if payload.CaseID == "" || payload.ArtifactRef == "" {
		return ArtifactCase{}, false
	}

	if filterPattern != nil && !filterPattern.MatchString(payload.CaseID) {
		return ArtifactCase{}, false
	}

	artifactPath := filepath.Join(filepath.Dir(expectedPath), payload.ArtifactRef)
	if _, err := os.Stat(artifactPath); err != nil {
		return ArtifactCase{}, false
	}

	return ArtifactCase{
		CaseID:              payload.CaseID,
		Category:            emptyTo(payload.Category, "unknown"),
		Difficulty:          emptyTo(payload.Difficulty, "unknown"),
		GroundTruthPattern:  payload.GroundTruthPattern,
		ExpectedRiskLevel:   payload.RiskLevel,
		ExpectedRiskDetails: normalizeTags(payload.RiskDetails),
		ArtifactPath:        artifactPath,
		ExpectedJSONPath:    expectedPath,
	}, true
}

func emptyTo(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
