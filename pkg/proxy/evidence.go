package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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

// PrescribeEntry is an auto-generated prescribe record.
type PrescribeEntry struct {
	Type           string    `json:"type"`
	PrescriptionID string    `json:"prescription_id"`
	Tool           string    `json:"tool"`
	Operation      string    `json:"operation"`
	Command        string    `json:"command"`
	Timestamp      time.Time `json:"timestamp"`
	Actor          Actor     `json:"actor"`
}

// ReportEntry is an auto-generated report record.
type ReportEntry struct {
	Type           string    `json:"type"`
	PrescriptionID string    `json:"prescription_id"`
	ExitCode       int       `json:"exit_code"`
	Verdict        string    `json:"verdict"`
	Timestamp      time.Time `json:"timestamp"`
}

// Actor identifies the proxy as the evidence source.
type Actor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Prescribe records a pre-execution entry and returns the prescription ID.
func (w *EvidenceWriter) Prescribe(command string) string {
	tool, operation := ParseCommand(command)
	id := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	entry := PrescribeEntry{
		Type:           "prescribe",
		PrescriptionID: id,
		Tool:           tool,
		Operation:      operation,
		Command:        command,
		Timestamp:      time.Now().UTC(),
		Actor:          Actor{Type: "proxy", ID: "evidra-proxy"},
	}

	w.write(entry)
	return id
}

// Report records a post-execution entry.
func (w *EvidenceWriter) Report(prescriptionID string, exitCode int) {
	verdict := "success"
	if exitCode != 0 {
		verdict = "failure"
	}

	entry := ReportEntry{
		Type:           "report",
		PrescriptionID: prescriptionID,
		ExitCode:       exitCode,
		Verdict:        verdict,
		Timestamp:      time.Now().UTC(),
	}

	w.write(entry)
}

func (w *EvidenceWriter) write(entry any) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.file.Write(append(data, '\n'))
}

// Dir returns the evidence session directory path.
func (w *EvidenceWriter) Dir() string {
	return w.dir
}
