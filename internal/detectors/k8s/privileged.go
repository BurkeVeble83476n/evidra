package k8s

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&Privileged{}) }

// Privileged detects privileged containers.
type Privileged struct{}

func (d *Privileged) Tag() string          { return "k8s.privileged_container" }
func (d *Privileged) BaseSeverity() string { return "critical" }
func (d *Privileged) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Container has securityContext.privileged=true",
	}
}
func (d *Privileged) Detect(_ canon.CanonicalAction, raw []byte) bool {
	objects, _ := ParseK8sYAML(raw)
	for _, obj := range objects {
		for _, c := range GetAllContainers(obj) {
			sc, ok := c["securityContext"].(map[string]interface{})
			if !ok {
				continue
			}
			if priv, ok := sc["privileged"].(bool); ok && priv {
				return true
			}
		}
	}
	return false
}
