package k8s

import (
	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
)

func init() { detectors.Register(&DangerousCapabilities{}) }

// DangerousCapabilities detects risky Linux capabilities.
type DangerousCapabilities struct{}

func (d *DangerousCapabilities) Tag() string          { return "k8s.dangerous_capabilities" }
func (d *DangerousCapabilities) BaseSeverity() string { return "high" }
func (d *DangerousCapabilities) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Container adds SYS_ADMIN/NET_ADMIN/NET_RAW/ALL capabilities",
	}
}
func (d *DangerousCapabilities) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, obj := range ParseK8sYAML(raw) {
		for _, c := range GetAllContainers(obj) {
			sc, ok := c["securityContext"].(map[string]interface{})
			if !ok {
				continue
			}
			caps, ok := sc["capabilities"].(map[string]interface{})
			if !ok {
				continue
			}
			addList, ok := caps["add"].([]interface{})
			if !ok {
				continue
			}
			if hasDangerousCapability(addList) {
				return true
			}
		}
	}
	return false
}
