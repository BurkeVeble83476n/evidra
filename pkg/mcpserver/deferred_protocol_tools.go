package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra/pkg/execcontract"
)

var minimalObjectSchema = json.RawMessage(`{"type":"object"}`)

func registerDeferredProtocolTools(server *mcp.Server, svc *MCPService, registry *toolRegistry) error {
	smartDef, err := execcontract.PrescribeSmartToolDefinition()
	if err != nil {
		return err
	}
	registry.register("prescribe_smart", toolEntry{
		description: smartDef.Description,
		inputSchema: smartDef.Parameters,
	})
	server.AddTool(&mcp.Tool{
		Name:        "prescribe_smart",
		Title:       "Record Smart Infrastructure Intent",
		Description: smartDef.Description + " Use describe_tool for the full schema if you need explicit control.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Prescribe Smart",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: minimalObjectSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input PrescribeSmartInput
		if err := decodeDeferredInput(req, &input); err != nil {
			return nil, err
		}
		out := svc.PrescribeCtx(ctx, input.toPrescribeInput())
		return structuredToolResult(out)
	})

	reportDef, err := execcontract.ReportToolDefinition()
	if err != nil {
		return err
	}
	registry.register("report", toolEntry{
		description: reportDef.Description,
		inputSchema: reportDef.Parameters,
	})
	server.AddTool(&mcp.Tool{
		Name:        "report",
		Title:       "Report Operation Result",
		Description: reportDef.Description + " Use describe_tool for the full schema if you need explicit control.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Report",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: minimalObjectSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input ReportInput
		if err := decodeDeferredInput(req, &input); err != nil {
			return nil, err
		}
		out := svc.ReportCtx(ctx, input)
		return structuredToolResult(out)
	})

	return nil
}

func decodeDeferredInput(req *mcp.CallToolRequest, out any) error {
	if req.Params == nil || req.Params.Arguments == nil {
		return fmt.Errorf("arguments are required")
	}
	return json.Unmarshal(req.Params.Arguments, out)
}

func structuredToolResult(out any) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: json.RawMessage(raw),
	}, nil
}
