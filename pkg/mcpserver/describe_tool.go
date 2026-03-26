package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var describeToolSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "Deferred tool name. Leave empty to list available deferred tools."
    }
  }
}`)

func RegisterDescribeTool(server *mcp.Server, registry *toolRegistry) {
	server.AddTool(&mcp.Tool{
		Name:        "describe_tool",
		Description: "Get the full input schema for a deferred protocol tool. Call with empty name to list deferred tools.",
		InputSchema: describeToolSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			Name string `json:"name"`
		}
		if req.Params != nil && req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return nil, fmt.Errorf("decode arguments: %w", err)
			}
		}

		text := "Deferred tools:\n" + strings.Join(registry.list(), "\n")
		if strings.TrimSpace(input.Name) != "" {
			desc, err := registry.describe(input.Name)
			if err != nil {
				text = err.Error()
			} else {
				text = desc
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil
	})
}
