package docker

import (
	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
)

func init() { detectors.Register(&Privileged{}) }

// Privileged detects privileged compose services.
type Privileged struct{}

func (d *Privileged) Tag() string          { return "docker.privileged" }
func (d *Privileged) BaseSeverity() string { return "critical" }
func (d *Privileged) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "docker",
		SourceKind:   "compose_yaml",
		Summary:      "Compose service has privileged=true",
	}
}
func (d *Privileged) Detect(_ canon.CanonicalAction, raw []byte) bool {
	cf := parseCompose(raw)
	for _, svc := range cf.Services {
		if b, ok := svc["privileged"].(bool); ok && b {
			return true
		}
	}
	return false
}
