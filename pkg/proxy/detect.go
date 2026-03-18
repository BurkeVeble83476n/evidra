// Package proxy provides an MCP stdio proxy that auto-records evidence for infrastructure mutations.
package proxy

import "strings"

// mutationRules maps tool prefixes to their mutation subcommands.
var mutationRules = map[string][]string{
	"kubectl":   {"apply", "patch", "delete", "create", "replace", "scale", "rollout", "set", "annotate", "label", "taint", "cordon", "drain", "uncordon"},
	"helm":      {"install", "upgrade", "uninstall", "rollback"},
	"terraform": {"apply", "destroy", "import"},
	"argocd":    {"app sync", "app delete", "app set"},
	"docker":    {"run", "rm", "stop", "kill"},
}

// IsMutation returns true if the command is an infrastructure mutation.
func IsMutation(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	words := strings.Fields(command)
	if len(words) < 2 {
		return false
	}

	tool := words[0]
	subcommands, ok := mutationRules[tool]
	if !ok {
		return false
	}

	sub := words[1]
	for _, m := range subcommands {
		// Handle two-word subcommands like "app sync"
		if strings.Contains(m, " ") {
			if len(words) >= 3 && words[1]+" "+words[2] == m {
				return true
			}
			continue
		}
		if sub == m {
			return true
		}
	}
	return false
}

// ParseCommand extracts tool and operation from a command string.
func ParseCommand(command string) (tool, operation string) {
	words := strings.Fields(strings.TrimSpace(command))
	if len(words) == 0 {
		return "", ""
	}
	tool = words[0]
	if len(words) >= 2 {
		operation = words[1]
	}
	return
}
