package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"samebits.com/evidra/internal/config"
)

func TestResolveSigner_OptionalWithoutKey(t *testing.T) {
	t.Setenv("EVIDRA_SIGNING_KEY", "")
	t.Setenv("EVIDRA_SIGNING_KEY_PATH", "")
	s, err := resolveSigner("optional")
	if err != nil {
		t.Fatalf("resolveSigner(optional): %v", err)
	}
	if s == nil {
		t.Fatal("expected signer in optional mode")
	}
}

func TestResolveSigner_StrictWithoutKeyFails(t *testing.T) {
	t.Setenv("EVIDRA_SIGNING_KEY", "")
	t.Setenv("EVIDRA_SIGNING_KEY_PATH", "")
	if _, err := resolveSigner("strict"); err == nil {
		t.Fatal("expected strict mode error when no key configured")
	}
}

func TestResolveSigner_InvalidModeFails(t *testing.T) {
	t.Setenv("EVIDRA_SIGNING_KEY", "")
	t.Setenv("EVIDRA_SIGNING_KEY_PATH", "")
	if _, err := resolveSigner("bad"); err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestResolveEvidenceWriteMode_FromEnv(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_WRITE_MODE", "best_effort")
	mode, err := config.ResolveEvidenceWriteMode("")
	if err != nil {
		t.Fatalf("ResolveEvidenceWriteMode: %v", err)
	}
	if mode != config.EvidenceWriteModeBestEffort {
		t.Fatalf("mode=%q, want %q", mode, config.EvidenceWriteModeBestEffort)
	}
}

func TestNormalizeProxyArgs_StripsLeadingSeparator(t *testing.T) {
	got, err := normalizeProxyArgs([]string{"--", "upstream", "--flag"})
	if err != nil {
		t.Fatalf("normalizeProxyArgs: %v", err)
	}
	want := []string{"upstream", "--flag"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%v, want %v", got, want)
	}
}

func TestNormalizeProxyArgs_RejectsMissingCommand(t *testing.T) {
	cases := [][]string{
		nil,
		{},
		{"--"},
	}
	for _, tc := range cases {
		if _, err := normalizeProxyArgs(tc); err == nil {
			t.Fatalf("normalizeProxyArgs(%v): expected error", tc)
		}
	}
}

func TestPrintHelp_DescribesDefaultToolSurfaceAndOptionalFullPrescribe(t *testing.T) {
	var out bytes.Buffer
	printHelp(&out)
	help := out.String()

	for _, needle := range []string{
		"--full-prescribe",
		"write_file",
		"describe_tool",
		"prescribe_smart",
		"run_command",
		"collect_diagnostics",
		"auto-evidence",
	} {
		if !strings.Contains(help, needle) {
			t.Fatalf("help missing %q: %s", needle, help)
		}
	}
	if strings.Contains(help, "Agent calls prescribe_full/prescribe_smart/report tools explicitly") {
		t.Fatalf("help should not claim prescribe_full is part of the default direct surface: %s", help)
	}
	if !strings.Contains(help, "Use run_command for the normal workflow") {
		t.Fatalf("help should describe run_command as the default workflow: %s", help)
	}
}
