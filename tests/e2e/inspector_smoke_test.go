//go:build e2e

package e2e_test

import (
	"os"
	"os/exec"
	"testing"
)

func TestE2E_InspectorSmoke(t *testing.T) {
	if os.Getenv("EVIDRA_RUN_INSPECTOR_SMOKE") != "1" {
		t.Skip("set EVIDRA_RUN_INSPECTOR_SMOKE=1 to run inspector smoke test")
	}

	cmd := exec.Command("make", "test-mcp-inspector-ci")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspector smoke failed: %v\n%s", err, string(out))
	}
}
