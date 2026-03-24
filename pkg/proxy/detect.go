// Package proxy provides an MCP stdio proxy that auto-records evidence for infrastructure mutations.
package proxy

import "strings"

// OperationClass classifies a command's operation type.
// Mirrors internal/canon/types.go classification but works on raw command strings.
type OperationClass string

const (
	OpMutate  OperationClass = "mutate"
	OpDestroy OperationClass = "destroy"
	OpRead    OperationClass = "read"
	OpPlan    OperationClass = "plan"
	OpUnknown OperationClass = "unknown"
)

// toolOps maps tool names to their subcommand classifications.
// Kept in sync with internal/canon/types.go (k8sOperationClass, terraformOperationClass).
var toolOps = map[string]map[string]OperationClass{
	"kubectl": {
		"apply": OpMutate, "create": OpMutate, "patch": OpMutate, "replace": OpMutate,
		"set": OpMutate, "annotate": OpMutate, "label": OpMutate,
		"rollout": OpMutate, "scale": OpMutate, "autoscale": OpMutate,
		"taint": OpMutate, "cordon": OpMutate, "uncordon": OpMutate,
		"delete": OpDestroy, "drain": OpDestroy,
		"get": OpRead, "describe": OpRead, "logs": OpRead, "top": OpRead, "diff": OpRead,
		"explain": OpRead, "events": OpRead, "auth": OpRead,
	},
	"helm": {
		"install": OpMutate, "upgrade": OpMutate,
		"uninstall": OpDestroy, "rollback": OpMutate,
		"list": OpRead, "status": OpRead, "template": OpRead, "show": OpRead,
	},
	"terraform": {
		"apply": OpMutate, "import": OpMutate,
		"destroy": OpDestroy,
		"plan":    OpPlan, "validate": OpPlan, "refresh": OpPlan, "show": OpRead, "state": OpRead, "output": OpRead,
	},
	"argocd": {
		"app sync": OpMutate, "app delete": OpDestroy, "app set": OpMutate,
		"app get": OpRead, "app list": OpRead,
	},
	"docker": {
		"run": OpMutate, "rm": OpDestroy, "stop": OpMutate, "kill": OpDestroy,
		"ps": OpRead, "images": OpRead, "logs": OpRead, "inspect": OpRead,
	},
}

// ClassifyCommand returns the tool, operation, and operation class for a command.
func ClassifyCommand(command string) (tool, operation string, class OperationClass) {
	words := strings.Fields(strings.TrimSpace(command))
	if len(words) == 0 {
		return "", "", OpUnknown
	}

	tool = words[0]
	ops, ok := toolOps[tool]
	if !ok {
		return tool, "", OpUnknown
	}

	if len(words) < 2 {
		return tool, "", OpUnknown
	}
	operation = words[1]

	// Check two-word subcommands first (e.g. "app sync")
	if len(words) >= 3 {
		twoWord := words[1] + " " + words[2]
		if class, ok := ops[twoWord]; ok {
			return tool, twoWord, class
		}
	}

	if class, ok := ops[operation]; ok {
		return tool, operation, class
	}

	return tool, operation, OpUnknown
}

// IsMutation returns true if the command is an infrastructure mutation.
func IsMutation(command string) bool {
	_, _, class := ClassifyCommand(command)
	return class == OpMutate || class == OpDestroy
}

// ClassifyToolName infers mutation intent from a generic MCP tool name when
// no raw shell command is available.
func ClassifyToolName(name string) (tool, operation string, class OperationClass) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return "", "", OpUnknown
	}

	replacer := strings.NewReplacer(".", " ", "_", " ", "-", " ", "/", " ")
	tokens := strings.Fields(replacer.Replace(normalized))
	if len(tokens) == 0 {
		return normalized, "", OpUnknown
	}

	for _, token := range tokens {
		switch token {
		case "delete", "destroy", "uninstall", "remove", "rm", "down":
			return normalized, token, OpDestroy
		case "apply", "create", "patch", "update", "install", "upgrade", "sync", "restart",
			"rollout", "scale", "annotate", "label", "set", "import", "deploy":
			return normalized, token, OpMutate
		case "get", "list", "describe", "logs", "read", "show", "status", "top", "diff":
			return normalized, token, OpRead
		}
	}

	return normalized, "", OpUnknown
}
