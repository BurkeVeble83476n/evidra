package k8s

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&WritableRootFS{}) }

// WritableRootFS detects containers with writable root filesystem.
type WritableRootFS struct{}

func (d *WritableRootFS) Tag() string          { return "k8s.writable_rootfs" }
func (d *WritableRootFS) BaseSeverity() string { return "low" }
func (d *WritableRootFS) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Container does not set readOnlyRootFilesystem=true",
	}
}
func (d *WritableRootFS) Detect(_ canon.CanonicalAction, raw []byte) bool {
	objects, _ := ParseK8sYAML(raw)
	for _, obj := range objects {
		for _, c := range GetAllContainers(obj) {
			sc, _ := c["securityContext"].(map[string]interface{})
			if sc == nil {
				return true
			}
			ro, ok := sc["readOnlyRootFilesystem"].(bool)
			if !ok || !ro {
				return true
			}
		}
	}
	return false
}
