package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	promptdata "samebits.com/evidra/prompts"
)

// registerPrompts adds MCP prompt resources for detailed protocol templates.
func registerPrompts(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "prescribe-smart",
		Title:       "Prescribe Smart Template",
		Description: "Full JSON template for prescribe_smart with all fields explained.",
	}, promptHandler(promptdata.MCPPromptPrescribeSmartPath, "Prescribe Smart reference"))

	server.AddPrompt(&mcp.Prompt{
		Name:        "prescribe-full",
		Title:       "Prescribe Full Template",
		Description: "Full JSON template for prescribe_full with all fields explained.",
	}, promptHandler(promptdata.MCPPromptPrescribeFullPath, "Prescribe Full reference"))

	server.AddPrompt(&mcp.Prompt{
		Name:        "diagnosis",
		Title:       "Infrastructure Diagnosis Flowchart",
		Description: "Step-by-step diagnosis protocol: events, describe, logs, fix, verify.",
	}, promptHandler(promptdata.MCPPromptDiagnosisPath, "Infrastructure Diagnosis Flowchart"))
}

// promptHandler returns a PromptHandler that reads embedded prompt content.
func promptHandler(path, description string) mcp.PromptHandler {
	return func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		content, err := promptdata.Read(path)
		if err != nil {
			return nil, err
		}
		return &mcp.GetPromptResult{
			Description: description,
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: content},
				},
			},
		}, nil
	}
}
