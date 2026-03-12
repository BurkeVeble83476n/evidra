package prompts

import "testing"

func TestParseContractVersionHeader(t *testing.T) {
	t.Parallel()

	contract, ok := parseContractVersionHeader("# contract: v1.0\nHello")
	if !ok {
		t.Fatalf("expected header parse success")
	}
	if contract != "v1.0" {
		t.Fatalf("contract=%q, want v1.0", contract)
	}
}

func TestSkillVersionFromContractVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "v1.0", want: "1.0.0"},
		{in: "1.1", want: "1.1.0"},
		{in: "v1.2.3", want: "1.2.3"},
		{in: "", want: DefaultContractSkillVersion},
		{in: "garbage", want: DefaultContractSkillVersion},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := skillVersionFromContractVersion(tc.in); got != tc.want {
				t.Fatalf("skillVersionFromContractVersion(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripContractHeader(t *testing.T) {
	t.Parallel()

	in := "# contract: v1.0\nline1\nline2\n"
	want := "line1\nline2"
	if got := stripContractHeader(in); got != want {
		t.Fatalf("stripContractHeader()=%q, want %q", got, want)
	}
}
