package store

import (
	"testing"
)

func TestHashKey_Deterministic(t *testing.T) {
	t.Parallel()
	h1 := hashKey("ev1_testkey123")
	h2 := hashKey("ev1_testkey123")
	if string(h1) != string(h2) {
		t.Fatal("hashKey should be deterministic")
	}
}

func TestHashKey_DifferentInputs(t *testing.T) {
	t.Parallel()
	h1 := hashKey("ev1_key_a")
	h2 := hashKey("ev1_key_b")
	if string(h1) == string(h2) {
		t.Fatal("different keys should produce different hashes")
	}
}

func TestGenerateKeyPlaintext(t *testing.T) {
	t.Parallel()
	key, err := generateKeyPlaintext()
	if err != nil {
		t.Fatalf("generateKeyPlaintext: %v", err)
	}
	if len(key) < 32 {
		t.Fatalf("key too short: %d", len(key))
	}
	if key[:4] != "ev1_" {
		t.Fatalf("key should start with ev1_, got: %s", key[:4])
	}
}
