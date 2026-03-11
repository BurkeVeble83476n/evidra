package k8s

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&RunAsRoot{}) }

// RunAsRoot detects root-running container configs.
type RunAsRoot struct{}

func (d *RunAsRoot) Tag() string          { return "k8s.run_as_root" }
func (d *RunAsRoot) BaseSeverity() string { return "medium" }
func (d *RunAsRoot) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Container runs as UID 0 or does not enforce runAsNonRoot",
	}
}
func (d *RunAsRoot) Detect(_ canon.CanonicalAction, raw []byte) bool {
	objects, _ := ParseK8sYAML(raw)
	for _, obj := range objects {
		for _, c := range GetAllContainers(obj) {
			sc, _ := c["securityContext"].(map[string]interface{})
			if sc == nil {
				return true
			}
			if uid, ok := sc["runAsUser"].(int); ok && uid == 0 {
				return true
			}
			if uidf, ok := sc["runAsUser"].(float64); ok && uidf == 0 {
				return true
			}
			runAsNonRoot, ok := sc["runAsNonRoot"].(bool)
			if !ok || !runAsNonRoot {
				return true
			}
		}
	}
	return false
}
