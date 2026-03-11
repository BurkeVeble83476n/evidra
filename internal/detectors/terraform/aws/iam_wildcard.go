package aws

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&IAMWildcard{}) }

// IAMWildcard detects allow *:* policies.
type IAMWildcard struct{}

func (d *IAMWildcard) Tag() string          { return "aws_iam.wildcard_policy" }
func (d *IAMWildcard) BaseSeverity() string { return "critical" }
func (d *IAMWildcard) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "aws",
		SourceKind:   "terraform_plan",
		Summary:      "IAM allow policy grants Action:* and Resource:*",
	}
}
func (d *IAMWildcard) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, s := range extractIAMStatements(raw) {
		if s.Effect == "Allow" && s.Action == "*" && s.Resource == "*" {
			return true
		}
	}
	return false
}
