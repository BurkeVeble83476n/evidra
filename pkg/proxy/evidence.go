package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"samebits.com/evidra/pkg/execcontract"
)

// EvidenceWriter records prescribe/report entries to a local JSONL file.
type EvidenceWriter struct {
	mu   sync.Mutex
	dir  string
	file *os.File
}

// NewEvidenceWriter creates a writer that stores evidence in the given directory.
func NewEvidenceWriter(dir string) (*EvidenceWriter, error) {
	sessionDir := filepath.Join(dir, fmt.Sprintf("proxy-%d", time.Now().UnixMilli()))
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("proxy: create evidence dir: %w", err)
	}

	path := filepath.Join(sessionDir, "evidence.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("proxy: open evidence file: %w", err)
	}

	return &EvidenceWriter{dir: sessionDir, file: f}, nil
}

// Close closes the evidence file.
func (w *EvidenceWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// ProxyEntry is a unified evidence entry for proxy-generated records.
type ProxyEntry struct {
	Type           string             `json:"type"` // prescribe or report
	PrescriptionID string             `json:"prescription_id"`
	Tool           string             `json:"tool,omitempty"`
	Operation      string             `json:"operation,omitempty"`
	OperationClass string             `json:"operation_class,omitempty"`
	Command        string             `json:"command,omitempty"`
	ExitCode       *int               `json:"exit_code,omitempty"`
	Verdict        string             `json:"verdict,omitempty"`
	Timestamp      time.Time          `json:"timestamp"`
	Actor          execcontract.Actor `json:"actor"`
}

// Prescribe records a pre-execution entry and returns the prescription ID.
func (w *EvidenceWriter) Prescribe(command string) string {
	tool, operation, class := ClassifyCommand(command)
	if strings.TrimSpace(tool) == "" {
		tool = strings.TrimSpace(command)
	}
	id := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	entry := ProxyEntry{
		Type:           "prescribe",
		PrescriptionID: id,
		Tool:           tool,
		Operation:      operation,
		OperationClass: string(class),
		Command:        command,
		Timestamp:      time.Now().UTC(),
		Actor: execcontract.Actor{
			Type: "proxy",
			ID:   "evidra-proxy",
		},
	}

	w.write(entry)
	return id
}

// PrescribeObserved records a pre-execution entry for a generic MCP tool call
// when no raw shell command is available.
func (w *EvidenceWriter) PrescribeObserved(tool, operation string, class OperationClass) string {
	id := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	entry := ProxyEntry{
		Type:           "prescribe",
		PrescriptionID: id,
		Tool:           strings.TrimSpace(tool),
		Operation:      strings.TrimSpace(operation),
		OperationClass: string(class),
		Command:        strings.TrimSpace(tool),
		Timestamp:      time.Now().UTC(),
		Actor: execcontract.Actor{
			Type: "proxy",
			ID:   "evidra-proxy",
		},
	}

	w.write(entry)
	return id
}

// Report records a post-execution entry.
func (w *EvidenceWriter) Report(prescriptionID string, exitCode int) {
	verdict := execcontract.VerdictSuccess
	if exitCode != 0 {
		verdict = execcontract.VerdictFailure
	}

	entry := ProxyEntry{
		Type:           "report",
		PrescriptionID: prescriptionID,
		ExitCode:       &exitCode,
		Verdict:        verdict,
		Timestamp:      time.Now().UTC(),
		Actor: execcontract.Actor{
			Type: "proxy",
			ID:   "evidra-proxy",
		},
	}

	w.write(entry)
}

func (w *EvidenceWriter) write(entry any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := writeJSONLine(w.file, entry); err != nil {
		return
	}
}

func writeJSONLine(w io.Writer, entry any) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// Dir returns the evidence session directory path.
func (w *EvidenceWriter) Dir() string {
	return w.dir
}
