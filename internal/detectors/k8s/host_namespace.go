package k8s

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&HostNamespace{}) }

// HostNamespace detects host namespace access.
type HostNamespace struct{}

func (d *HostNamespace) Tag() string          { return "k8s.host_namespace_escape" }
func (d *HostNamespace) BaseSeverity() string { return "high" }
func (d *HostNamespace) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Pod spec uses hostPID/hostIPC/hostNetwork",
	}
}
func (d *HostNamespace) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, obj := range ParseK8sYAML(raw) {
		spec := GetPodSpec(obj)
		if spec == nil {
			continue
		}
		if GetBool(spec, "hostPID") || GetBool(spec, "hostIPC") || GetBool(spec, "hostNetwork") {
			return true
		}
	}
	return false
}
