package aws

import (
	"strconv"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
	tdet "samebits.com/evidra/internal/detectors/terraform"
)

func init() { detectors.Register(&SecurityGroupOpen{}) }

// SecurityGroupOpen detects world-open risky ports.
type SecurityGroupOpen struct{}

func (d *SecurityGroupOpen) Tag() string          { return "aws.security_group_open" }
func (d *SecurityGroupOpen) BaseSeverity() string { return "high" }
func (d *SecurityGroupOpen) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "aws",
		SourceKind:   "terraform_plan",
		Summary:      "Security group ingress exposes admin/db ports to 0.0.0.0/0",
	}
}
func (d *SecurityGroupOpen) Detect(_ canon.CanonicalAction, raw []byte) bool {
	plan := tdet.ParsePlan(raw)
	if plan == nil {
		return false
	}
	for _, rc := range tdet.ResourcesByType(plan, "aws_security_group") {
		if rc.Change == nil {
			continue
		}
		ingress, _ := rc.Change.After["ingress"].([]interface{})
		for _, ruleRaw := range ingress {
			rule, ok := ruleRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if !containsCIDR(rule["cidr_blocks"], "0.0.0.0/0") {
				continue
			}
			from := intFromAny(rule["from_port"])
			to := intFromAny(rule["to_port"])
			if from == 0 && to == 0 {
				if stringFromAny(rule["protocol"]) == "-1" {
					return true
				}
				continue
			}
			for _, p := range []int{22, 3389, 3306, 5432} {
				if portInRange(p, from, to) {
					return true
				}
			}
		}
	}
	return false
}

func stringFromAny(v interface{}) string {
	s, _ := v.(string)
	return s
}

func containsCIDR(v interface{}, want string) bool {
	list, ok := v.([]interface{})
	if !ok {
		return false
	}
	for _, raw := range list {
		s, _ := raw.(string)
		if s == want {
			return true
		}
	}
	return false
}

func intFromAny(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(x)
		return i
	default:
		return 0
	}
}

func portInRange(port, from, to int) bool {
	if from > to {
		from, to = to, from
	}
	return port >= from && port <= to
}
