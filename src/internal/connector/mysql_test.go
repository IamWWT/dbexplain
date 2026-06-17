//go:build mysql || full

package connector

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestQuoteMySQL(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"simple", "users", "`users`"},
		{"with backtick", "us`ers", "`us``ers`"},
		{"double backtick", "a``b", "`a````b`"},
		{"empty", "", "``"},
		{"unicode", "测 试", "`测 试`"},
		{"numbers", "table_123", "`table_123`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteMySQL(tt.s)
			if got != tt.want {
				t.Errorf("quoteMySQL(%q) = %q, want %q", tt.s, got, tt.want)
			}
			// Verify roundtrip
			if back := unquoteMySQL(got); back != tt.s {
				t.Errorf("unquoteMySQL(quoteMySQL(%q)) = %q, want %q", tt.s, back, tt.s)
			}
		})
	}
}

func FuzzQuoteMySQL(f *testing.F) {
	// Seed corpus
	seeds := []string{
		"users",
		"us`ers",
		"a``b",
		"",
		"test_table",
		"ORDER BY 1",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	// Fuzz with random inputs
	f.Fuzz(func(t *testing.T, s string) {
		// Must not panic for any input
		quoted := quoteMySQL(s)
		// Must start and end with backtick
		if len(quoted) < 2 || quoted[0] != '`' || quoted[len(quoted)-1] != '`' {
			t.Errorf("quoteMySQL(%q) = %q, missing backtick delimiters", s, quoted)
		}
		// Every backtick run inside must be even-length (all are escaped pairs)
		inner := quoted[1 : len(quoted)-1]
		if hasOddDelimRun(inner, '`') {
			t.Errorf("quoteMySQL(%q) = %q, odd-length backtick run inside", s, quoted)
		}
		// Roundtrip must recover original
		if back := unquoteMySQL(quoted); back != s {
			t.Errorf("roundtrip: quoteMySQL(%q) = %q, unquote -> %q", s, quoted, back)
		}
	})
}

// unquoteMySQL strips MySQL backtick quoting: `foo``bar` → foo`bar
func unquoteMySQL(s string) string {
	if len(s) < 2 || s[0] != '`' || s[len(s)-1] != '`' {
		return s
	}
	inner := s[1 : len(s)-1]
	if !strings.Contains(inner, "`") {
		return inner
	}
	return strings.ReplaceAll(inner, "``", "`")
}

// TestQuoteMySQL_InvalidUTF8 verifies quoting functions handle invalid UTF-8 without panic.
func TestQuoteMySQL_InvalidUTF8(t *testing.T) {
	inputs := []string{
		string([]byte{0xff, 0xfe, 0x00}),
		string([]byte{0x80, 0x81, 0x82}),
	}
	for _, s := range inputs {
		quoted := quoteMySQL(s)
		if !utf8.ValidString(quoted) {
			t.Logf("quoteMySQL produced invalid UTF-8 for input %x, which is acceptable but noted", []byte(s))
		}
		_ = quoted
	}
}


