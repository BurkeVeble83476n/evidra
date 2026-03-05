package canon

import "testing"

func TestScopeVocab_ExplicitAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env  string
		want string
	}{
		{env: "prod", want: "production"},
		{env: "production", want: "production"},
		{env: "staging", want: "staging"},
		{env: "dev", want: "development"},
		{env: "test", want: "development"},
		{env: "sandbox", want: "development"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.env, func(t *testing.T) {
			t.Parallel()
			got := ResolveScopeClass(tt.env, nil)
			if got != tt.want {
				t.Fatalf("ResolveScopeClass(%q,nil)=%q, want %q", tt.env, got, tt.want)
			}
		})
	}
}

func TestScopeVocab_NamespaceHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ns   string
		want string
	}{
		{ns: "prod-api", want: "production"},
		{ns: "staging-eu", want: "staging"},
		{ns: "dev-team", want: "development"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.ns, func(t *testing.T) {
			t.Parallel()
			got := ResolveScopeClass("", []ResourceID{{Namespace: tt.ns}})
			if got != tt.want {
				t.Fatalf("ResolveScopeClass(\"\",ns=%q)=%q, want %q", tt.ns, got, tt.want)
			}
		})
	}
}
