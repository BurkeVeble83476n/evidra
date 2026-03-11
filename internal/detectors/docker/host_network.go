package docker

import (
	"strings"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&HostNetwork{}) }

// HostNetwork detects compose services using host networking.
type HostNetwork struct{}

func (d *HostNetwork) Tag() string          { return "docker.host_network" }
func (d *HostNetwork) BaseSeverity() string { return "high" }
func (d *HostNetwork) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "docker",
		SourceKind:   "compose_yaml",
		Summary:      "Compose service sets network_mode=host",
	}
}
func (d *HostNetwork) Detect(_ canon.CanonicalAction, raw []byte) bool {
	cf := parseCompose(raw)
	for _, svc := range cf.Services {
		if mode, ok := svc["network_mode"].(string); ok && strings.EqualFold(strings.TrimSpace(mode), "host") {
			return true
		}
	}
	return false
}
