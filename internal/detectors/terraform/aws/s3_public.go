package aws

import (
	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
	tdet "samebits.com/evidra-benchmark/internal/detectors/terraform"
)

func init() { detectors.Register(&S3PublicAccess{}) }

// S3PublicAccess detects buckets without complete public access block.
type S3PublicAccess struct{}

func (d *S3PublicAccess) Tag() string          { return "terraform.s3_public_access" }
func (d *S3PublicAccess) BaseSeverity() string { return "high" }
func (d *S3PublicAccess) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "aws",
		SourceKind:   "terraform_plan",
		Summary:      "S3 bucket created without full public access block",
	}
}
func (d *S3PublicAccess) Detect(_ canon.CanonicalAction, raw []byte) bool {
	plan := tdet.ParsePlan(raw)
	if plan == nil || !tdet.HasResource(plan, "aws_s3_bucket") {
		return false
	}
	for _, rc := range tdet.ResourcesByType(plan, "aws_s3_bucket_public_access_block") {
		if rc.Change != nil && isCompletePublicAccessBlock(rc.Change.After) {
			return false
		}
	}
	return true
}
