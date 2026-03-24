package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WriteFileInput is the input for the write_file tool.
type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteFileOutput is the output for the write_file tool.
type WriteFileOutput struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type writeFileHandler struct{}

func (h *writeFileHandler) Handle(
	_ context.Context,
	_ *mcp.CallToolRequest,
	input WriteFileInput,
) (*mcp.CallToolResult, WriteFileOutput, error) {
	if input.Path == "" {
		return &mcp.CallToolResult{IsError: true},
			WriteFileOutput{OK: false, Error: "path is required"}, nil
	}

	// Create parent directories if needed.
	dir := filepath.Dir(input.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &mcp.CallToolResult{IsError: true},
			WriteFileOutput{OK: false, Error: fmt.Sprintf("create directory: %v", err)}, nil
	}

	if err := os.WriteFile(input.Path, []byte(input.Content), 0o644); err != nil {
		return &mcp.CallToolResult{IsError: true},
			WriteFileOutput{OK: false, Error: fmt.Sprintf("write file: %v", err)}, nil
	}

	msg := fmt.Sprintf("wrote %d bytes to %s", len(input.Content), input.Path)
	return &mcp.CallToolResult{},
		WriteFileOutput{OK: true, Message: msg}, nil
}

// RegisterWriteFile registers the write_file tool on the given MCP server.
func RegisterWriteFile(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "write_file",
		Title:       "Write File",
		Description: "Write content to a file. Creates parent directories if needed. Use for creating or updating configuration files (Terraform .tf, YAML manifests, scripts).",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Write File",
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(true),
		},
	}, (&writeFileHandler{}).Handle)
}
