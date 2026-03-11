package k8s

import (
	"strings"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&ClusterAdminBinding{}) }

// ClusterAdminBinding detects cluster-admin role bindings.
type ClusterAdminBinding struct{}

func (d *ClusterAdminBinding) Tag() string          { return "k8s.cluster_admin_binding" }
func (d *ClusterAdminBinding) BaseSeverity() string { return "critical" }
func (d *ClusterAdminBinding) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "ClusterRoleBinding grants cluster-admin role",
	}
}
func (d *ClusterAdminBinding) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, obj := range ParseK8sYAML(raw) {
		kind := strings.ToLower(strings.TrimSpace(getString(obj, "kind")))
		if kind != "clusterrolebinding" {
			continue
		}
		roleRef, ok := obj["roleRef"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := roleRef["name"].(string)
		if strings.EqualFold(strings.TrimSpace(name), "cluster-admin") {
			return true
		}
	}
	return false
}
