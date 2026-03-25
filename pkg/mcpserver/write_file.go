package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// blockedPrefixes lists directory prefixes that are never writable.
var blockedPrefixes = []string{
	"/etc/",
	"/root/",
	"/var/",
	"/usr/",
	"/bin/",
	"/sbin/",
	"/boot/",
	"/proc/",
	"/sys/",
	"/dev/",
}

// isUnderDir reports whether path is equal to or nested under dir.
func isUnderDir(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+"/")
}

// validateWritePath resolves the path and checks it against safety rules.
// It returns the cleaned absolute path or an error string.
func validateWritePath(raw string) (string, string) {
	cleaned := filepath.Clean(raw)

	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Sprintf("resolve path: %v", err)
	}

	// Reject any remaining ".." components after resolution.
	if strings.Contains(abs, "..") {
		return "", fmt.Sprintf("path traversal not allowed: %s", raw)
	}

	// Reject ~/.ssh.
	home, err := os.UserHomeDir()
	if err == nil {
		sshDir := filepath.Join(home, ".ssh")
		if isUnderDir(abs, sshDir) {
			return "", fmt.Sprintf("writing to %s is not allowed: blocked directory", abs)
		}
	}

	// Build the list of allowed temp directory prefixes.
	// On macOS /tmp -> /private/tmp and os.TempDir() returns paths under
	// /var/folders, so we must resolve symlinks and check all variants.
	// All paths are cleaned to remove trailing slashes.
	allowedTmpDirs := []string{"/tmp"}
	if real, err := filepath.EvalSymlinks("/tmp"); err == nil && real != "/tmp" {
		allowedTmpDirs = append(allowedTmpDirs, filepath.Clean(real))
	}
	osTmp := filepath.Clean(os.TempDir())
	if osTmp != "" && osTmp != "." {
		allowedTmpDirs = append(allowedTmpDirs, osTmp)
		if real, err := filepath.EvalSymlinks(osTmp); err == nil && real != osTmp {
			allowedTmpDirs = append(allowedTmpDirs, filepath.Clean(real))
		}
	}

	// Allow temp directories and subdirectories.
	for _, td := range allowedTmpDirs {
		if isUnderDir(abs, td) {
			return abs, ""
		}
	}

	// Allow current working directory and subdirectories.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Sprintf("cannot determine working directory: %v", err)
	}
	if isUnderDir(abs, cwd) {
		return abs, ""
	}

	// Reject blocklisted directory prefixes.
	for _, prefix := range blockedPrefixes {
		dir := strings.TrimSuffix(prefix, "/")
		if isUnderDir(abs, dir) {
			return "", fmt.Sprintf("writing to %s is not allowed: blocked directory", abs)
		}
	}

	return "", fmt.Sprintf("path %s is outside allowed directories (cwd or /tmp)", abs)
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

	absPath, errMsg := validateWritePath(input.Path)
	if errMsg != "" {
		return &mcp.CallToolResult{IsError: true},
			WriteFileOutput{OK: false, Error: errMsg}, nil
	}

	// Create parent directories if needed.
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &mcp.CallToolResult{IsError: true},
			WriteFileOutput{OK: false, Error: fmt.Sprintf("create directory: %v", err)}, nil
	}

	if err := os.WriteFile(absPath, []byte(input.Content), 0o644); err != nil {
		return &mcp.CallToolResult{IsError: true},
			WriteFileOutput{OK: false, Error: fmt.Sprintf("write file: %v", err)}, nil
	}

	msg := fmt.Sprintf("wrote %d bytes to %s", len(input.Content), absPath)
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
