package utils

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"trimmed with ellipsis", "hello world", 8, "hello..."},
		{"limit of three keeps no ellipsis", "abcdef", 3, "abc"},
		// Regression: truncating by byte split multi-byte characters in half.
		{"accented characters", "héllo wörld", 8, "héllo..."},
		{"accented at boundary", "café au lait", 6, "caf..."},
		{"accented fits exactly", "café", 4, "café"},
		{"multi-byte script", "日本語のテキストです", 6, "日本語..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.in, tt.maxLen); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
			}
		})
	}
}
