package ops

import (
	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	"samebits.com/evidra/internal/detectors/k8s"
	tdet "samebits.com/evidra/internal/detectors/terraform"
)

func init() { detectors.Register(&MassDelete{}) }

// MassDeleteThreshold is the default count above which mass delete is flagged.
const MassDeleteThreshold = 10

// MassDelete detects bulk destructive operations.
type MassDelete struct{}

func (d *MassDelete) Tag() string          { return "ops.mass_delete" }
func (d *MassDelete) BaseSeverity() string { return "critical" }
func (d *MassDelete) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.OperationRisk,
		Domain:       "ops",
		SourceKind:   "any",
		Summary:      "Operation deletes more than safe threshold of resources",
	}
}
func (d *MassDelete) Detect(_ canon.CanonicalAction, raw []byte) bool {
	if plan := tdet.ParsePlan(raw); plan != nil {
		deletes := 0
		for i := range plan.ResourceChanges {
			rc := &plan.ResourceChanges[i]
			if rc.Change == nil {
				continue
			}
			for _, a := range rc.Change.Actions {
				if a == "delete" {
					deletes++
					break
				}
			}
		}
		return deletes > MassDeleteThreshold
	}
	// Fallback: large multi-doc YAML.
	objects, _ := k8s.ParseK8sYAML(raw)
	return len(objects) > MassDeleteThreshold
}
