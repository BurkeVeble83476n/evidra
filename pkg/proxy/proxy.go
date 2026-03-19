package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Proxy is a transparent MCP stdio proxy that intercepts tool calls
// and auto-records evidence for infrastructure mutations.
type Proxy struct {
	UpstreamCmd  string
	UpstreamArgs []string
	Evidence     *EvidenceWriter
	Verbose      bool
	DryRun       bool

	mu      sync.Mutex
	pending map[string]string // JSON-RPC id → prescription_id
}

// Run starts the proxy, relaying stdio between the MCP client and upstream server.
func (p *Proxy) Run(ctx context.Context) error {
	p.pending = make(map[string]string)

	cmd := exec.CommandContext(ctx, p.UpstreamCmd, p.UpstreamArgs...)
	cmd.Stderr = os.Stderr

	upstreamIn, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("proxy: stdin pipe: %w", err)
	}
	upstreamOut, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("proxy: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("proxy: start upstream %q: %w", p.UpstreamCmd, err)
	}

	if p.Verbose {
		log.Printf("[proxy] started upstream: %s %s", p.UpstreamCmd, strings.Join(p.UpstreamArgs, " "))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Client → Upstream (intercept requests)
	go func() {
		defer wg.Done()
		defer func() {
			if closeErr := upstreamIn.Close(); closeErr != nil && p.Verbose {
				log.Printf("[proxy] close upstream stdin: %v", closeErr)
			}
		}()
		p.relayRequests(os.Stdin, upstreamIn)
	}()

	// Upstream → Client (intercept responses)
	go func() {
		defer wg.Done()
		p.relayResponses(upstreamOut, os.Stdout)
	}()

	wg.Wait()
	return cmd.Wait()
}

// relayRequests reads JSON-RPC messages from client, intercepts tools/call mutations.
func (p *Proxy) relayRequests(client io.Reader, server io.Writer) {
	scanner := bufio.NewScanner(client)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max message

	for scanner.Scan() {
		line := scanner.Bytes()

		if req, ok := p.parseToolCallRequest(line); ok {
			command := req.extractCommand()
			if command != "" && IsMutation(command) {
				if p.Verbose {
					log.Printf("[proxy] mutation detected: %s", truncateLog(command, 80))
				}
				if p.Evidence != nil && !p.DryRun {
					prescriptionID := p.Evidence.Prescribe(command)
					p.mu.Lock()
					p.pending[string(req.ID)] = prescriptionID
					p.mu.Unlock()
				}
			}
		}

		// Always forward to upstream
		if err := writeLine(server, line); err != nil {
			if p.Verbose {
				log.Printf("[proxy] forward request: %v", err)
			}
			return
		}
	}
}

// relayResponses reads JSON-RPC messages from upstream, matches responses to prescriptions.
func (p *Proxy) relayResponses(server io.Reader, client io.Writer) {
	scanner := bufio.NewScanner(server)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		if resp, ok := p.parseToolCallResponse(line); ok {
			p.mu.Lock()
			prescriptionID, exists := p.pending[string(resp.ID)]
			if exists {
				delete(p.pending, string(resp.ID))
			}
			p.mu.Unlock()

			if exists && p.Evidence != nil {
				exitCode := resp.extractExitCode()
				p.Evidence.Report(prescriptionID, exitCode)
				if p.Verbose {
					verdict := "success"
					if exitCode != 0 {
						verdict = "failure"
					}
					log.Printf("[proxy] reported: %s exit=%d verdict=%s", prescriptionID, exitCode, verdict)
				}
			}
		}

		// Always forward to client
		if err := writeLine(client, line); err != nil {
			if p.Verbose {
				log.Printf("[proxy] forward response: %v", err)
			}
			return
		}
	}
}

func writeLine(w io.Writer, line []byte) error {
	if _, err := w.Write(line); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}

// jsonRPCRequest represents a parsed tools/call request.
type jsonRPCRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"params"`
}

func (r *jsonRPCRequest) extractCommand() string {
	if r.Params.Name != "run_command" {
		return ""
	}
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(r.Params.Arguments, &args) != nil {
		return ""
	}
	return args.Command
}

// jsonRPCResponse represents a parsed response.
type jsonRPCResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (r *jsonRPCResponse) extractExitCode() int {
	if len(r.Error) > 0 && string(r.Error) != "null" {
		return 1
	}
	// Try to extract exit code from result content
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(r.Result, &result) == nil {
		for _, c := range result.Content {
			if strings.Contains(c.Text, "Exit code:") {
				// Parse "Exit code: N" from the output
				parts := strings.Split(c.Text, "Exit code:")
				if len(parts) >= 2 {
					code := strings.TrimSpace(parts[len(parts)-1])
					if len(code) > 0 && code[0] != '0' {
						return 1
					}
				}
			}
		}
	}
	return 0
}

func (p *Proxy) parseToolCallRequest(data []byte) (*jsonRPCRequest, bool) {
	var msg jsonRPCRequest
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false
	}
	if msg.Method != "tools/call" {
		return nil, false
	}
	return &msg, true
}

func (p *Proxy) parseToolCallResponse(data []byte) (*jsonRPCResponse, bool) {
	var msg jsonRPCResponse
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false
	}
	// Only match responses (have id + result/error, no method)
	if len(msg.ID) == 0 || string(msg.ID) == "null" {
		return nil, false
	}
	return &msg, true
}

func truncateLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
