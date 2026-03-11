package k8s

import (
	"strings"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&HostPath{}) }

// HostPath detects hostPath volume mounts.
type HostPath struct{}

func (d *HostPath) Tag() string          { return "k8s.hostpath_mount" }
func (d *HostPath) BaseSeverity() string { return "high" }
func (d *HostPath) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Pod spec includes hostPath volume mount",
	}
}
func (d *HostPath) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, obj := range ParseK8sYAML(raw) {
		spec := GetPodSpec(obj)
		if spec == nil {
			continue
		}
		volumes, ok := spec["volumes"].([]interface{})
		if !ok {
			continue
		}
		for _, v := range volumes {
			vol, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			if hostPathVal, ok := vol["hostPath"]; ok {
				if _, ok := hostPathVal.(map[string]interface{}); ok {
					return true
				}
				if s, ok := hostPathVal.(string); ok && strings.TrimSpace(s) != "" {
					return true
				}
			}
		}
	}
	return false
}
