package mcpserver

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type toolEntry struct {
	description string
	inputSchema map[string]any
}

type toolRegistry struct {
	entries map[string]toolEntry
}

func newToolRegistry() *toolRegistry {
	return &toolRegistry{entries: make(map[string]toolEntry)}
}

func (r *toolRegistry) register(name string, entry toolEntry) {
	r.entries[name] = entry
}

func (r *toolRegistry) list() []string {
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *toolRegistry) describe(name string) (string, error) {
	entry, ok := r.entries[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q; available: %s", name, strings.Join(r.list(), ", "))
	}
	raw, err := json.MarshalIndent(entry.inputSchema, "", "  ")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Tool: %s\nDescription: %s\nInput Schema:\n%s", name, entry.description, raw), nil
}
