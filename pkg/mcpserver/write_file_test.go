package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWriteFile_ValidPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "hello.txt")

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    dest,
		Content: "hello world",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", out.Error)
	}
	if !out.OK {
		t.Fatalf("expected OK=true, got false: %s", out.Error)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", string(data), "hello world")
	}
}

func TestWriteFile_DirectoryAutoCreation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "sub", "dir", "file.yaml")

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    dest,
		Content: "key: value",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", out.Error)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != "key: value" {
		t.Errorf("content = %q, want %q", string(data), "key: value")
	}
}

func TestWriteFile_EmptyPath(t *testing.T) {
	t.Parallel()

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    "",
		Content: "data",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for empty path")
	}
	if out.OK {
		t.Fatal("expected OK=false for empty path")
	}
	if !strings.Contains(out.Error, "path is required") {
		t.Errorf("error = %q, want it to contain 'path is required'", out.Error)
	}
}

func TestWriteFile_PathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{"relative_etc_passwd", "../../etc/passwd"},
		{"relative_root", "../../../root/.bashrc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &writeFileHandler{}
			result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
				Path:    tc.path,
				Content: "malicious",
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected IsError=true for path traversal")
			}
			if out.OK {
				t.Fatal("expected OK=false for path traversal")
			}
		})
	}
}

func TestWriteFile_BlocklistedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{"etc_cron", "/etc/cron.d/evil"},
		{"root_bashrc", "/root/.bashrc"},
		{"var_spool", "/var/spool/cron/evil"},
		{"usr_local_bin", "/usr/local/bin/evil"},
		{"bin_sh", "/bin/sh"},
		{"sbin_init", "/sbin/init"},
		{"boot_grub", "/boot/grub/evil.cfg"},
		{"proc_self", "/proc/self/environ"},
		{"sys_module", "/sys/module/evil"},
		{"dev_null_sub", "/dev/shm/evil"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &writeFileHandler{}
			result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
				Path:    tc.path,
				Content: "malicious",
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError=true for blocked path %s", tc.path)
			}
			if out.OK {
				t.Fatalf("expected OK=false for blocked path %s", tc.path)
			}
			if !strings.Contains(out.Error, "not allowed") {
				t.Errorf("error = %q, want it to contain 'not allowed'", out.Error)
			}
		})
	}
}

func TestWriteFile_SSHBlocked(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	sshPath := filepath.Join(home, ".ssh", "authorized_keys")

	h := &writeFileHandler{}
	result, out, herr := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    sshPath,
		Content: "ssh-rsa AAAA evil",
	})

	if herr != nil {
		t.Fatalf("unexpected error: %v", herr)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for ~/.ssh path")
	}
	if !strings.Contains(out.Error, "not allowed") {
		t.Errorf("error = %q, want it to contain 'not allowed'", out.Error)
	}
}

func TestWriteFile_OutsideAllowedDirs(t *testing.T) {
	t.Parallel()

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    "/opt/evil/payload.sh",
		Content: "#!/bin/sh\nrm -rf /",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for path outside allowed dirs")
	}
	if !strings.Contains(out.Error, "outside allowed directories") {
		t.Errorf("error = %q, want it to contain 'outside allowed directories'", out.Error)
	}
}

func TestWriteFile_SymlinkedParentResolvesToBlockedDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}

	linkPath := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(sshDir, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    filepath.Join(linkPath, "payload.txt"),
		Content: "malicious",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for symlinked blocked parent")
	}
	if out.OK {
		t.Fatal("expected OK=false for symlinked blocked parent")
	}
	if !strings.Contains(out.Error, "not allowed") {
		t.Fatalf("error = %q, want blocked-directory message", out.Error)
	}
	if _, err := os.Stat(filepath.Join(sshDir, "payload.txt")); !os.IsNotExist(err) {
		t.Fatalf("blocked target file should not be created, stat err=%v", err)
	}
}

func TestWriteFile_SymlinkedFileResolvesToBlockedTarget(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}

	blockedTarget := filepath.Join(sshDir, "authorized_keys")
	if err := os.WriteFile(blockedTarget, []byte("original"), 0o600); err != nil {
		t.Fatalf("write blocked target: %v", err)
	}

	linkPath := filepath.Join(t.TempDir(), "payload-link.txt")
	if err := os.Symlink(blockedTarget, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    linkPath,
		Content: "malicious",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for symlinked blocked file")
	}
	if out.OK {
		t.Fatal("expected OK=false for symlinked blocked file")
	}
	if !strings.Contains(out.Error, "not allowed") {
		t.Fatalf("error = %q, want blocked-directory message", out.Error)
	}
	data, err := os.ReadFile(blockedTarget)
	if err != nil {
		t.Fatalf("read blocked target: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("blocked target was modified: %q", string(data))
	}
}

func TestWriteFile_AllowsFilenameContainingDoubleDots(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "config..backup.yaml")

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    dest,
		Content: "key: value",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", out.Error)
	}
	if !out.OK {
		t.Fatalf("expected OK=true, got false: %s", out.Error)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != "key: value" {
		t.Errorf("content = %q, want %q", string(data), "key: value")
	}
}

func TestValidateWritePath_Table(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantOK  bool
		wantErr string
	}{
		{"tmp_file", "/tmp/test.txt", true, ""},
		{"tmp_nested", "/tmp/a/b/c.txt", true, ""},
		{"cwd_file", filepath.Join(cwd, "test.txt"), true, ""},
		{"cwd_nested", filepath.Join(cwd, "sub", "dir", "f.txt"), true, ""},
		{"etc_passwd", "/etc/passwd", false, "not allowed"},
		{"proc", "/proc/1/maps", false, "not allowed"},
		{"sys", "/sys/class/net", false, "not allowed"},
		{"random_abs", "/opt/something", false, "outside allowed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			abs, errMsg := validateWritePath(tc.path)
			if tc.wantOK {
				if errMsg != "" {
					t.Fatalf("expected success for %s, got error: %s", tc.path, errMsg)
				}
				if abs == "" {
					t.Fatal("expected non-empty absolute path")
				}
			} else {
				if errMsg == "" {
					t.Fatalf("expected error for %s, got success: %s", tc.path, abs)
				}
				if !strings.Contains(errMsg, tc.wantErr) {
					t.Errorf("error = %q, want it to contain %q", errMsg, tc.wantErr)
				}
			}
		})
	}
}

// TestWriteFile_OverwriteExisting verifies that writing to an existing file overwrites it.
func TestWriteFile_OverwriteExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "overwrite.txt")

	if err := os.WriteFile(dest, []byte("original"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	h := &writeFileHandler{}
	result, out, err := h.Handle(context.Background(), nil, WriteFileInput{
		Path:    dest,
		Content: "replaced",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", out.Error)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "replaced" {
		t.Errorf("content = %q, want %q", string(data), "replaced")
	}
}

// Ensure the unused import of mcp is referenced.
var _ = (*mcp.CallToolRequest)(nil)
