package store

import (
	"testing"
	"time"
)

func TestListOptions_Defaults(t *testing.T) {
	t.Parallel()
	opts := ListOptions{}
	opts = opts.withDefaults()
	if opts.Limit != 100 {
		t.Fatalf("expected default limit=100, got %d", opts.Limit)
	}
}

func TestListOptions_MaxLimit(t *testing.T) {
	t.Parallel()
	opts := ListOptions{Limit: 5000}
	opts = opts.withDefaults()
	if opts.Limit != 1000 {
		t.Fatalf("expected max limit=1000, got %d", opts.Limit)
	}
}

func TestParsePeriod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"", 30 * 24 * time.Hour},
	}
	for _, tt := range tests {
		got := parsePeriod(tt.input)
		if got != tt.want {
			t.Errorf("parsePeriod(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
