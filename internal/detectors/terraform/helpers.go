package terraform

import "encoding/json"

// Plan is a minimal Terraform plan representation used by detectors.
type Plan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

// ResourceChange mirrors Terraform resource_changes entries.
type ResourceChange struct {
	Type   string  `json:"type"`
	Name   string  `json:"name"`
	Change *Change `json:"change"`
}

// Change mirrors Terraform "change" payload.
type Change struct {
	Actions []string               `json:"actions"`
	After   map[string]interface{} `json:"after"`
}

// ParsePlan returns a plan or nil if payload is not Terraform plan JSON.
func ParsePlan(raw []byte) *Plan {
	var plan Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil
	}
	if len(plan.ResourceChanges) == 0 {
		return nil
	}
	return &plan
}

// ResourcesByType filters changes by resource type.
func ResourcesByType(plan *Plan, resourceType string) []*ResourceChange {
	if plan == nil {
		return nil
	}
	out := make([]*ResourceChange, 0)
	for i := range plan.ResourceChanges {
		rc := &plan.ResourceChanges[i]
		if rc.Type == resourceType {
			out = append(out, rc)
		}
	}
	return out
}

// AfterValue returns value from change.after.
func AfterValue(rc *ResourceChange, key string) (interface{}, bool) {
	if rc == nil || rc.Change == nil || rc.Change.After == nil {
		return nil, false
	}
	v, ok := rc.Change.After[key]
	return v, ok
}

// AfterBool returns a bool from change.after.
func AfterBool(rc *ResourceChange, key string) bool {
	v, ok := AfterValue(rc, key)
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// AfterString returns a string from change.after.
func AfterString(rc *ResourceChange, key string) string {
	v, ok := AfterValue(rc, key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
