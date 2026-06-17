//go:build mongodb || full

package connector

import (
	"testing"
)

func TestStringifyVal(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"nil", nil, "null"},
		{"bytes", []byte("hello"), "hello"},
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
		{"empty bytes", []byte{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringifyVal(tt.val)
			if got != tt.want {
				t.Errorf("stringifyVal(%#v) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}
