package aws

import (
	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
)

func init() { detectors.Register(&TerraformIAMWildcard{}) }

// TerraformIAMWildcard detects any wildcard in allow policy.
type TerraformIAMWildcard struct{}

func (d *TerraformIAMWildcard) Tag() string          { return "terraform.iam_wildcard_policy" }
func (d *TerraformIAMWildcard) BaseSeverity() string { return "high" }
func (d *TerraformIAMWildcard) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "aws",
		SourceKind:   "terraform_plan",
		Summary:      "IAM allow policy uses wildcard action or resource",
	}
}
func (d *TerraformIAMWildcard) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, s := range extractIAMStatements(raw) {
		if s.Effect == "Allow" && (s.Action == "*" || s.Resource == "*") {
			return true
		}
	}
	return false
}
