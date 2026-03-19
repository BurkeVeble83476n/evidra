package proxy

import (
	"bytes"
	"errors"
	"testing"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestWriteLine_AppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := writeLine(&buf, []byte(`{"jsonrpc":"2.0"}`)); err != nil {
		t.Fatalf("writeLine: %v", err)
	}
	if got, want := buf.String(), "{\"jsonrpc\":\"2.0\"}\n"; got != want {
		t.Fatalf("output=%q, want %q", got, want)
	}
}

func TestWriteLine_PropagatesWriteErrors(t *testing.T) {
	want := errors.New("boom")
	if err := writeLine(failingWriter{err: want}, []byte("x")); !errors.Is(err, want) {
		t.Fatalf("err=%v, want %v", err, want)
	}
}

func TestWriteJSONLine_EncodesAndAppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	entry := map[string]string{"type": "report"}
	if err := writeJSONLine(&buf, entry); err != nil {
		t.Fatalf("writeJSONLine: %v", err)
	}
	if got, want := buf.String(), "{\"type\":\"report\"}\n"; got != want {
		t.Fatalf("output=%q, want %q", got, want)
	}
}

func TestWriteJSONLine_PropagatesWriterErrors(t *testing.T) {
	want := errors.New("boom")
	entry := map[string]string{"type": "report"}
	if err := writeJSONLine(failingWriter{err: want}, entry); !errors.Is(err, want) {
		t.Fatalf("err=%v, want %v", err, want)
	}
}
