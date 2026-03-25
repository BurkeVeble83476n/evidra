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

	target, err := resolveWriteTarget(abs)
	if err != nil {
		return "", fmt.Sprintf("resolve symlinks: %v", err)
	}

	// Reject ~/.ssh.
	if isUnderAnyDir(target, sshBlockedDirs()) {
		return "", fmt.Sprintf("writing to %s is not allowed: blocked directory", target)
	}

	// Allow temp directories and subdirectories.
	if isUnderAnyDir(target, allowedTempDirs()) {
		return target, ""
	}

	// Allow current working directory and subdirectories.
	allowedWorkDirs, err := allowedWorkDirs()
	if err != nil {
		return "", fmt.Sprintf("cannot determine working directory: %v", err)
	}
	if isUnderAnyDir(target, allowedWorkDirs) {
		return target, ""
	}

	// Reject blocklisted directory prefixes after explicit allowlists so
	// platform temp directories (for example /private/var/folders on macOS)
	// still work, while symlinks into blocked targets remain rejected because
	// target already points at the resolved destination.
	if isUnderAnyDir(target, blockedSystemDirs()) {
		return "", fmt.Sprintf("writing to %s is not allowed: blocked directory", target)
	}

	return "", fmt.Sprintf("path %s is outside allowed directories (cwd or /tmp)", target)
}

func isUnderAnyDir(path string, dirs []string) bool {
	for _, dir := range dirs {
		if isUnderDir(path, dir) {
			return true
		}
	}
	return false
}

func sshBlockedDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	dirs := []string{sshDir}
	if realSSHDir, err := filepath.EvalSymlinks(sshDir); err == nil {
		realSSHDir = filepath.Clean(realSSHDir)
		if realSSHDir != sshDir {
			dirs = append(dirs, realSSHDir)
		}
	}
	return dirs
}

func allowedTempDirs() []string {
	// On macOS /tmp -> /private/tmp and os.TempDir() returns paths under
	// /var/folders, so we must resolve symlinks and check all variants.
	dirs := []string{"/tmp"}
	if realTmp, err := filepath.EvalSymlinks("/tmp"); err == nil && realTmp != "/tmp" {
		dirs = append(dirs, filepath.Clean(realTmp))
	}

	osTmp := filepath.Clean(os.TempDir())
	if osTmp == "" || osTmp == "." {
		return dirs
	}
	dirs = append(dirs, osTmp)
	if realOSTmp, err := filepath.EvalSymlinks(osTmp); err == nil && realOSTmp != osTmp {
		dirs = append(dirs, filepath.Clean(realOSTmp))
	}
	return dirs
}

func allowedWorkDirs() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dirs := []string{filepath.Clean(cwd)}
	if realCwd, err := filepath.EvalSymlinks(cwd); err == nil {
		realCwd = filepath.Clean(realCwd)
		if realCwd != dirs[0] {
			dirs = append(dirs, realCwd)
		}
	}
	return dirs, nil
}

func blockedSystemDirs() []string {
	dirs := make([]string, 0, len(blockedPrefixes)*2)
	for _, prefix := range blockedPrefixes {
		dir := filepath.Clean(strings.TrimSuffix(prefix, "/"))
		dirs = append(dirs, dir)
		if realDir, err := filepath.EvalSymlinks(dir); err == nil {
			realDir = filepath.Clean(realDir)
			if realDir != dir {
				dirs = append(dirs, realDir)
			}
		}
	}
	return dirs
}

func resolveWriteTarget(abs string) (string, error) {
	pending := make([]string, 0, 4)
	cur := filepath.Clean(abs)

	for {
		info, err := os.Lstat(cur)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(cur)
				if err != nil {
					return "", err
				}
				cur = filepath.Clean(resolved)
				continue
			}
			if resolved, err := filepath.EvalSymlinks(cur); err == nil {
				cur = filepath.Clean(resolved)
			}
			for i := len(pending) - 1; i >= 0; i-- {
				cur = filepath.Join(cur, pending[i])
			}
			return cur, nil
		case os.IsNotExist(err):
			parent := filepath.Dir(cur)
			if parent == cur {
				for i := len(pending) - 1; i >= 0; i-- {
					cur = filepath.Join(cur, pending[i])
				}
				return cur, nil
			}
			pending = append(pending, filepath.Base(cur))
			cur = parent
		default:
			return "", err
		}
	}
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
