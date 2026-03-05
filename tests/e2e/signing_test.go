//go:build e2e

package e2e_test

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-benchmark/pkg/evidence"
)

// evidraBinary builds the evidra CLI binary and returns its path.
func evidraBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(os.TempDir(), "evidra-e2e-test")
	repoRoot := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evidra")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build evidra binary: %v\n%s", err, out)
	}
	t.Cleanup(func() { os.Remove(bin) })
	return bin
}

// generateKeyPair creates an Ed25519 key pair and writes PEM files to dir.
func generateKeyPair(t *testing.T, dir string) (privPath, pubPath string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	// Write private key PEM.
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	privPath = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	// Write public key PEM.
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	pubPath = filepath.Join(dir, "pub.pem")
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatalf("write public key: %v", err)
	}
	return privPath, pubPath
}

// runEvidra executes the evidra binary with the given arguments.
func runEvidra(t *testing.T, bin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("run evidra: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestE2E_SigningEndToEnd(t *testing.T) {
	bin := evidraBinary(t)
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")
	privPath, pubPath := generateKeyPair(t, tmpDir)
	artifactPath := filepath.Join("..", "..", "tests", "e2e", "fixtures", "k8s_deployment.yaml")

	// Step 1: Prescribe with signing key.
	stdout, stderr, exitCode := runEvidra(t, bin,
		"prescribe",
		"--tool", "kubectl",
		"--operation", "apply",
		"--artifact", artifactPath,
		"--session-id", "e2e-signing-001",
		"--signing-key-path", privPath,
		"--evidence-dir", evidenceDir,
	)
	if exitCode != 0 {
		t.Fatalf("prescribe exit code = %d, stderr = %s", exitCode, stderr)
	}

	// Parse JSON output to get prescription_id.
	var prescribeResult map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &prescribeResult); err != nil {
		t.Fatalf("decode prescribe output: %v\nstdout: %s", err, stdout)
	}
	if prescribeResult["ok"] != true {
		t.Fatalf("prescribe result not ok: %v", prescribeResult)
	}
	prescriptionID, ok := prescribeResult["prescription_id"].(string)
	if !ok || prescriptionID == "" {
		t.Fatalf("prescription_id missing or empty in output: %v", prescribeResult)
	}

	// Step 2: Report with signing key.
	stdout, stderr, exitCode = runEvidra(t, bin,
		"report",
		"--prescription", prescriptionID,
		"--exit-code", "0",
		"--session-id", "e2e-signing-001",
		"--signing-key-path", privPath,
		"--evidence-dir", evidenceDir,
	)
	if exitCode != 0 {
		t.Fatalf("report exit code = %d, stderr = %s", exitCode, stderr)
	}

	// Step 3: Validate with public key — expect success.
	stdout, stderr, exitCode = runEvidra(t, bin,
		"validate",
		"--public-key", pubPath,
		"--evidence-dir", evidenceDir,
	)
	if exitCode != 0 {
		t.Fatalf("validate exit code = %d, stderr = %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "signatures verified") {
		t.Fatalf("validate output missing 'signatures verified': %s", stdout)
	}

	// Step 4: Verify all entries have non-empty Signature and correct SessionID.
	entries, err := evidence.ReadAllEntriesAtPath(evidenceDir)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no evidence entries found")
	}
	for i, e := range entries {
		if e.Signature == "" {
			t.Errorf("entry %d (%s) has empty Signature", i, e.Type)
		}
		if e.SessionID != "e2e-signing-001" {
			t.Errorf("entry %d (%s) SessionID = %q, want %q", i, e.Type, e.SessionID, "e2e-signing-001")
		}
	}

	// Step 5: Tamper detection — modify 1 byte in the segment file, then validate should fail.
	segmentFiles, err := filepath.Glob(filepath.Join(evidenceDir, "segments", "*.jsonl"))
	if err != nil {
		t.Fatalf("glob segment files: %v", err)
	}
	if len(segmentFiles) == 0 {
		t.Fatal("no segment files found for tamper test")
	}

	segData, err := os.ReadFile(segmentFiles[0])
	if err != nil {
		t.Fatalf("read segment file: %v", err)
	}
	// Flip one byte in the middle of the file.
	mid := len(segData) / 2
	segData[mid] ^= 0xFF
	if err := os.WriteFile(segmentFiles[0], segData, 0o644); err != nil {
		t.Fatalf("write tampered segment file: %v", err)
	}

	_, stderr, exitCode = runEvidra(t, bin,
		"validate",
		"--public-key", pubPath,
		"--evidence-dir", evidenceDir,
	)
	if exitCode == 0 {
		t.Fatal("validate should have failed after tampering, but exit code was 0")
	}
}
