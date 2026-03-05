package config

import "testing"

func TestResolveSigningMode_DefaultStrict(t *testing.T) {
	t.Setenv("EVIDRA_SIGNING_MODE", "")
	mode, err := ResolveSigningMode("")
	if err != nil {
		t.Fatalf("ResolveSigningMode: %v", err)
	}
	if mode != SigningModeStrict {
		t.Fatalf("mode = %q, want %q", mode, SigningModeStrict)
	}
}

func TestResolveSigningMode_Explicit(t *testing.T) {
	t.Setenv("EVIDRA_SIGNING_MODE", "")
	mode, err := ResolveSigningMode("optional")
	if err != nil {
		t.Fatalf("ResolveSigningMode: %v", err)
	}
	if mode != SigningModeOptional {
		t.Fatalf("mode = %q, want %q", mode, SigningModeOptional)
	}
}

func TestResolveSigningMode_Invalid(t *testing.T) {
	t.Setenv("EVIDRA_SIGNING_MODE", "")
	if _, err := ResolveSigningMode("invalid"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}
