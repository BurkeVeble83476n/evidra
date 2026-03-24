package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProxyRelayRequests_TracksGenericMutationTool(t *testing.T) {
	writer, err := NewEvidenceWriter(t.TempDir())
	if err != nil {
		t.Fatalf("NewEvidenceWriter: %v", err)
	}
	defer func() { _ = writer.Close() }()

	p := &Proxy{
		Evidence: writer,
		pending:  make(map[string]string),
	}

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kubernetes_apply_manifest","arguments":{"manifest":"apiVersion: v1"}}}`
	var upstream bytes.Buffer
	p.relayRequests(strings.NewReader(req+"\n"), &upstream)

	if len(p.pending) != 1 {
		t.Fatalf("pending count = %d, want 1", len(p.pending))
	}
	if _, ok := p.pending["1"]; !ok {
		t.Fatalf("pending map = %#v, want key 1", p.pending)
	}
}

func TestProxyRelayResponses_UsesStructuredExitCode(t *testing.T) {
	writer, err := NewEvidenceWriter(t.TempDir())
	if err != nil {
		t.Fatalf("NewEvidenceWriter: %v", err)
	}
	defer func() { _ = writer.Close() }()

	p := &Proxy{
		Evidence: writer,
		pending: map[string]string{
			"1": "presc-1",
		},
	}

	resp := `{"jsonrpc":"2.0","id":1,"result":{"structuredContent":{"ok":false,"exit_code":2}}}`
	var client bytes.Buffer
	p.relayResponses(strings.NewReader(resp+"\n"), &client)

	entries := readProxyEntries(t, filepath.Join(writer.Dir(), "evidence.jsonl"))
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].Type != "report" {
		t.Fatalf("entry type = %q, want report", entries[0].Type)
	}
	if entries[0].ExitCode == nil || *entries[0].ExitCode != 2 {
		t.Fatalf("exit code = %v, want 2", entries[0].ExitCode)
	}
	if entries[0].Verdict != "failure" {
		t.Fatalf("verdict = %q, want failure", entries[0].Verdict)
	}
}

func readProxyEntries(t *testing.T, path string) []ProxyEntry {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open proxy evidence: %v", err)
	}
	defer func() { _ = file.Close() }()

	var entries []ProxyEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry ProxyEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("unmarshal proxy entry: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan proxy entries: %v", err)
	}
	return entries
}
