package ops

import (
	"strings"

	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
)

func init() { detectors.Register(&NamespaceDelete{}) }

// NamespaceDelete detects namespace deletion operations.
type NamespaceDelete struct{}

func (d *NamespaceDelete) Tag() string          { return "ops.namespace_delete" }
func (d *NamespaceDelete) BaseSeverity() string { return "high" }
func (d *NamespaceDelete) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.OperationRisk,
		Domain:       "ops",
		SourceKind:   "any",
		Summary:      "Operation deletes Kubernetes Namespace resource",
	}
}
func (d *NamespaceDelete) Detect(action canon.CanonicalAction, _ []byte) bool {
	if !strings.EqualFold(strings.TrimSpace(action.Operation), "delete") {
		return false
	}
	for _, r := range action.ResourceIdentity {
		if strings.EqualFold(strings.TrimSpace(r.Kind), "namespace") {
			return true
		}
	}
	return false
}
