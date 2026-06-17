//go:build clickhouse || full

package connector

import (
	"testing"
)

func TestEscCH(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"it's", "it''s"},
		{"no'escape'", "no''escape''"},
		{"", ""},
		{"nochange", "nochange"},
	}
	for _, tt := range tests {
		got := escCH(tt.input)
		if got != tt.want {
			t.Errorf("escCH(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
