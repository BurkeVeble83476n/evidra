package ops

import (
	"strings"

	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
)

func init() { detectors.Register(&KubeSystem{}) }

// KubeSystem detects mutate/destroy operations targeting kube-system.
type KubeSystem struct{}

func (d *KubeSystem) Tag() string          { return "ops.kube_system" }
func (d *KubeSystem) BaseSeverity() string { return "high" }
func (d *KubeSystem) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.OperationRisk,
		Domain:       "ops",
		SourceKind:   "any",
		Summary:      "Mutate/destroy operation targets kube-system namespace",
	}
}
func (d *KubeSystem) Detect(action canon.CanonicalAction, _ []byte) bool {
	if action.OperationClass != "mutate" && action.OperationClass != "destroy" {
		return false
	}
	for _, r := range action.ResourceIdentity {
		if strings.EqualFold(strings.TrimSpace(r.Namespace), "kube-system") {
			return true
		}
	}
	return false
}
