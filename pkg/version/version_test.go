package version

import "testing"

func TestContractVersions_UseSemverStyleValues(t *testing.T) {
	t.Parallel()

	if SpecVersion != "v1.1.0" {
		t.Fatalf("SpecVersion = %q, want %q", SpecVersion, "v1.1.0")
	}
	if ScoringVersion != "v1.1.0" {
		t.Fatalf("ScoringVersion = %q, want %q", ScoringVersion, "v1.1.0")
	}
}
