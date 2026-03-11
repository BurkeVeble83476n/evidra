package detectors_test

import (
	"regexp"
	"testing"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	_ "samebits.com/evidra/internal/detectors/all"
)

func TestDetectorContract(t *testing.T) {
	t.Parallel()

	ds := detectors.All()
	if len(ds) == 0 {
		t.Fatalf("no detectors registered")
	}

	tagRe := regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
	allowedSeverity := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	allowedStability := map[detectors.Stability]bool{detectors.Stable: true, detectors.Experimental: true, detectors.Deprecated: true}
	allowedLevel := map[detectors.VocabularyLevel]bool{detectors.ResourceRisk: true, detectors.OperationRisk: true}

	for _, d := range ds {
		d := d
		t.Run(d.Tag(), func(t *testing.T) {
			t.Parallel()
			meta := d.Metadata()
			if !tagRe.MatchString(d.Tag()) {
				t.Fatalf("invalid tag format: %q", d.Tag())
			}
			if !allowedSeverity[d.BaseSeverity()] {
				t.Fatalf("invalid base severity: %q", d.BaseSeverity())
			}
			if !allowedStability[meta.Stability] {
				t.Fatalf("invalid stability: %q", meta.Stability)
			}
			if !allowedLevel[meta.Level] {
				t.Fatalf("invalid level: %q", meta.Level)
			}
			if meta.Summary == "" {
				t.Fatalf("summary must not be empty")
			}
			if d.Tag() != meta.Tag {
				t.Fatalf("Tag()=%q metadata.tag=%q mismatch", d.Tag(), meta.Tag)
			}
			if d.BaseSeverity() != meta.BaseSeverity {
				t.Fatalf("BaseSeverity()=%q metadata.base_severity=%q mismatch", d.BaseSeverity(), meta.BaseSeverity)
			}
			a1 := d.Detect(canon.CanonicalAction{}, nil)
			a2 := d.Detect(canon.CanonicalAction{}, nil)
			if a1 != a2 {
				t.Fatalf("detect is not idempotent on empty input: %v vs %v", a1, a2)
			}
		})
	}
}
