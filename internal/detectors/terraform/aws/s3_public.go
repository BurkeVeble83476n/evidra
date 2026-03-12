package aws

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	tdet "samebits.com/evidra/internal/detectors/terraform"
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
	if plan == nil {
		return false
	}

	buckets := tdet.ResourcesByType(plan, "aws_s3_bucket")
	if len(buckets) == 0 {
		return false
	}

	protected := make(map[string]struct{})
	for _, rc := range tdet.ResourcesByType(plan, "aws_s3_bucket_public_access_block") {
		if rc.Change == nil || !isCompletePublicAccessBlock(rc.Change.After) {
			continue
		}
		for _, key := range s3ResourceKeys(rc) {
			protected[key] = struct{}{}
		}
	}

	for _, rc := range buckets {
		if rc.Change == nil {
			continue
		}
		if !hasMatchingPublicAccessBlock(protected, rc) {
			return true
		}
	}
	return false
}

func hasMatchingPublicAccessBlock(protected map[string]struct{}, rc *tdet.ResourceChange) bool {
	for _, key := range s3ResourceKeys(rc) {
		if _, ok := protected[key]; ok {
			return true
		}
	}
	return false
}

func s3ResourceKeys(rc *tdet.ResourceChange) []string {
	if rc == nil {
		return nil
	}

	keys := make([]string, 0, 2)
	if rc.Name != "" {
		keys = append(keys, rc.Name)
	}
	if bucket := tdet.AfterString(rc, "bucket"); bucket != "" && bucket != rc.Name {
		keys = append(keys, bucket)
	}
	return keys
}
