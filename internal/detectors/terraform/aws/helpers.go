package aws

import (
	"encoding/json"

	tdet "samebits.com/evidra-benchmark/internal/detectors/terraform"
)

type iamStatement struct {
	Effect   string `json:"Effect"`
	Action   string `json:"Action"`
	Resource string `json:"Resource"`
}

type iamPolicyDoc struct {
	Statement []iamStatement `json:"Statement"`
}

func extractIAMStatements(raw []byte) []iamStatement {
	plan := tdet.ParsePlan(raw)
	if plan == nil {
		return nil
	}
	var out []iamStatement
	for i := range plan.ResourceChanges {
		rc := &plan.ResourceChanges[i]
		if !isIAMResourceType(rc.Type) || rc.Change == nil {
			continue
		}
		policyRaw, ok := rc.Change.After["policy"].(string)
		if !ok || policyRaw == "" {
			continue
		}
		var doc iamPolicyDoc
		if err := json.Unmarshal([]byte(policyRaw), &doc); err != nil {
			continue
		}
		out = append(out, doc.Statement...)
	}
	return out
}

func isIAMResourceType(t string) bool {
	switch t {
	case "aws_iam_policy", "aws_iam_role_policy", "aws_iam_user_policy", "aws_iam_group_policy":
		return true
	default:
		return false
	}
}

func isCompletePublicAccessBlock(after map[string]interface{}) bool {
	for _, key := range []string{
		"block_public_acls",
		"ignore_public_acls",
		"block_public_policy",
		"restrict_public_buckets",
	} {
		b, ok := after[key].(bool)
		if !ok || !b {
			return false
		}
	}
	return true
}
