package aws

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	tdet "samebits.com/evidra/internal/detectors/terraform"
)

func init() { detectors.Register(&RDSPublic{}) }

// RDSPublic detects publicly accessible RDS instances.
type RDSPublic struct{}

func (d *RDSPublic) Tag() string          { return "aws.rds_public" }
func (d *RDSPublic) BaseSeverity() string { return "high" }
func (d *RDSPublic) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "aws",
		SourceKind:   "terraform_plan",
		Summary:      "RDS instance sets publicly_accessible=true",
	}
}
func (d *RDSPublic) Detect(_ canon.CanonicalAction, raw []byte) bool {
	plan := tdet.ParsePlan(raw)
	if plan == nil {
		return false
	}
	for _, rc := range tdet.ResourcesByType(plan, "aws_db_instance") {
		if tdet.AfterBool(rc, "publicly_accessible") {
			return true
		}
	}
	return false
}
