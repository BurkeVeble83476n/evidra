package aws

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	tdet "samebits.com/evidra/internal/detectors/terraform"
)

func init() { detectors.Register(&EBSUnencrypted{}) }

// EBSUnencrypted detects unencrypted EBS volumes.
type EBSUnencrypted struct{}

func (d *EBSUnencrypted) Tag() string          { return "aws.ebs_unencrypted" }
func (d *EBSUnencrypted) BaseSeverity() string { return "medium" }
func (d *EBSUnencrypted) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "aws",
		SourceKind:   "terraform_plan",
		Summary:      "EBS volume does not set encrypted=true",
	}
}
func (d *EBSUnencrypted) Detect(_ canon.CanonicalAction, raw []byte) bool {
	plan := tdet.ParsePlan(raw)
	if plan == nil {
		return false
	}
	for _, rc := range tdet.ResourcesByType(plan, "aws_ebs_volume") {
		if !tdet.AfterBool(rc, "encrypted") {
			return true
		}
	}
	return false
}
