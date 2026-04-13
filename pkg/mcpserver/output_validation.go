package mcpserver

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// loadedOutputSchema holds both the advertised schema (map[string]any for the
// MCP Tool definition) and the resolved schema (for output validation).
type loadedOutputSchema struct {
	advertised map[string]any
	resolved   *jsonschema.Resolved
}

// loadOutputSchema parses and resolves a JSON schema from embedded bytes.
func loadOutputSchema(raw []byte, name string) (loadedOutputSchema, error) {
	advertised, err := loadSchema(raw, name)
	if err != nil {
		return loadedOutputSchema{}, err
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return loadedOutputSchema{}, fmt.Errorf("decode JSON schema %s: %w", name, err)
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return loadedOutputSchema{}, fmt.Errorf("resolve JSON schema %s: %w", name, err)
	}

	return loadedOutputSchema{
		advertised: advertised,
		resolved:   resolved,
	}, nil
}

// structuredToolResultValidated marshals out, validates it against resolved
// (if non-nil), then returns a CallToolResult with both Content and
// StructuredContent populated.
func structuredToolResultValidated(out any, resolved *jsonschema.Resolved) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	if resolved != nil {
		value := map[string]any{}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("unmarshal tool output: %w", err)
		}
		if err := resolved.ApplyDefaults(&value); err != nil {
			return nil, fmt.Errorf("apply output schema defaults: %w", err)
		}
		if err := resolved.Validate(&value); err != nil {
			return nil, fmt.Errorf("validating tool output: %w", err)
		}
		raw, err = json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal validated tool output: %w", err)
		}
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: json.RawMessage(raw),
	}, nil
}
