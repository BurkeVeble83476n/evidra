package docker

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&SocketMount{}) }

// SocketMount detects docker socket mounts in compose services.
type SocketMount struct{}

func (d *SocketMount) Tag() string          { return "docker.socket_mount" }
func (d *SocketMount) BaseSeverity() string { return "critical" }
func (d *SocketMount) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "docker",
		SourceKind:   "compose_yaml",
		Summary:      "Compose service mounts docker.sock",
	}
}
func (d *SocketMount) Detect(_ canon.CanonicalAction, raw []byte) bool {
	cf := parseCompose(raw)
	for _, svc := range cf.Services {
		vols, ok := svc["volumes"].([]interface{})
		if !ok {
			continue
		}
		for _, vol := range vols {
			if hasDockerSockMount(vol) {
				return true
			}
		}
	}
	return false
}
