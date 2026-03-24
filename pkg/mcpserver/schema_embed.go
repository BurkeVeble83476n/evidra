package mcpserver

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed schemas/get_event.schema.json
var getEventSchemaBytes []byte

//go:embed schemas/get_event.output.schema.json
var getEventOutputSchemaBytes []byte

//go:embed schemas/run_command.output.schema.json
var runCommandOutputSchemaBytes []byte

func loadSchema(raw []byte, name string) (map[string]any, error) {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse embedded MCP schema %s: %w", name, err)
	}
	return schema, nil
}
