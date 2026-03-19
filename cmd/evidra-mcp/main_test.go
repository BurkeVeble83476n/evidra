package main

import (
	"reflect"
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
