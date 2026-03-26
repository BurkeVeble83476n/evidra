package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDescribeTool_ListAndDescribe(t *testing.T) {
	t.Parallel()

	reg := newToolRegistry()
	reg.register("prescribe_smart", toolEntry{
		description: "Pre-flight risk assessment",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool": map[string]any{"type": "string"},
			},
		},
	})

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	RegisterDescribeTool(server, reg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Wait() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	listResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "describe_tool"})
	if err != nil {
		t.Fatalf("describe_tool list: %v", err)
	}
	listText := listResult.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(listText, "prescribe_smart") {
		t.Fatalf("list output missing tool name: %q", listText)
	}

	describeResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "describe_tool",
		Arguments: map[string]any{
			"name": "prescribe_smart",
		},
	})
	if err != nil {
		t.Fatalf("describe_tool describe: %v", err)
	}
	describeText := describeResult.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(describeText, "Tool: prescribe_smart") {
		t.Fatalf("describe output missing tool header: %q", describeText)
	}
	if !strings.Contains(describeText, `"properties"`) {
		t.Fatalf("describe output missing schema body: %q", describeText)
	}
}
