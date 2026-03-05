package config

import (
	"fmt"
	"os"
	"strings"
)

const signingModeEnv = "EVIDRA_SIGNING_MODE"

// SigningMode controls whether a key is required for writing evidence.
type SigningMode string

const (
	SigningModeStrict   SigningMode = "strict"
	SigningModeOptional SigningMode = "optional"
)

// ResolveSigningMode returns signing mode from explicit flag, then env, then default.
// Default is strict.
func ResolveSigningMode(explicit string) (SigningMode, error) {
	raw := strings.TrimSpace(explicit)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(signingModeEnv))
	}
	if raw == "" {
		return SigningModeStrict, nil
	}
	switch strings.ToLower(raw) {
	case string(SigningModeStrict):
		return SigningModeStrict, nil
	case string(SigningModeOptional):
		return SigningModeOptional, nil
	default:
		return "", fmt.Errorf("invalid signing mode %q (expected strict|optional)", raw)
	}
}
