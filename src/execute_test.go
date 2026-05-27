package main

import (
	"strings"
	"testing"

	"github.com/IamWWT/dbexplain/query"
)

// ── sanitizeCell ──────────────────────────────────────────────────────────

func TestSanitizeCell_PreservesNormalText(t *testing.T) {
	in := "hello world 123 !@#$%"
	got := sanitizeCell(in)
	if got != in {
		t.Errorf("expected %q, got %q", in, got)
	}
}

func TestSanitizeCell_StripsANSI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"red foreground", "\x1b[31mred", "red"},
		{"bold", "\x1b[1mbold", "bold"},
		{"complex CSI", "\x1b[38;5;196mcolor", "color"},
		{"cursor movement", "\x1b[2Jclear", "clear"},
		{"mixed with text", "a\x1b[31mb\x1b[0mc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCell(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeCell(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeCell_StripsControlChars(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"null byte", "a\x00b", "ab"},
		{"bell", "a\x07b", "ab"},
		{"escape without bracket", "a\x1bb", "ab"},
		{"delete", "a\x7fb", "ab"},
		{"mix of controls", "\x00\x01\x02\x03text\x7f", "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCell(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeCell(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeCell_PreservesTabNewlineCR(t *testing.T) {
	in := "a\tb\nc\rd"
	got := sanitizeCell(in)
	if got != in {
		t.Errorf("sanitizeCell(%q) = %q, want %q", in, got, in)
	}
}

// ── formatHuman ───────────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }

func TestFormatHuman_EmptyResult(t *testing.T) {
	r := &query.QueryResult{}
	got := formatHuman(r)
	if !strings.Contains(got, "(empty result)") {
		t.Errorf("expected empty result message, got %q", got)
	}
}

func TestFormatHuman_NormalResult(t *testing.T) {
	r := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("Alice")},
			{strPtr("2"), strPtr("Bob")},
		},
		RowCount:      2,
		ExecutionTime: "1.23ms",
	}
	got := formatHuman(r)

	// Must contain headers
	if !strings.Contains(got, "id") || !strings.Contains(got, "name") {
		t.Errorf("output missing headers: %q", got)
	}
	// Must contain data
	if !strings.Contains(got, "Alice") || !strings.Contains(got, "Bob") {
		t.Errorf("output missing data: %q", got)
	}
	// Must contain row count + execution time
	if !strings.Contains(got, "2 row(s) in set") || !strings.Contains(got, "1.23ms") {
		t.Errorf("output missing footer: %q", got)
	}
	// Must contain table border chars
	if !strings.HasPrefix(got, "+") {
		t.Errorf("output should start with table border '+': %q", got)
	}
}

func TestFormatHuman_NULLValues(t *testing.T) {
	r := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INT"},
			{Name: "val", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("1"), nil},
			{strPtr("2"), strPtr("hello")},
		},
		RowCount:      2,
		ExecutionTime: "0.5ms",
	}
	got := formatHuman(r)
	if !strings.Contains(got, "NULL") {
		t.Errorf("expected NULL in output, got %q", got)
	}
}

func TestFormatHuman_TruncatedFlag(t *testing.T) {
	r := &query.QueryResult{
		Columns:       []query.ColumnInfo{{Name: "c", Type: "TEXT"}},
		Rows:          [][]*string{{strPtr("a")}},
		RowCount:      1,
		Truncated:     true,
		ExecutionTime: "0.1ms",
	}
	got := formatHuman(r)
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncated message, got %q", got)
	}
}

func TestFormatHuman_ColumnWidthCap(t *testing.T) {
	// Create a cell value longer than maxColWidth (256)
	longVal := strings.Repeat("x", 300)
	r := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "short", Type: "TEXT"},
			{Name: "long", Type: "TEXT"},
		},
		Rows: [][]*string{
			{strPtr("ok"), strPtr(longVal)},
		},
		RowCount:      1,
		ExecutionTime: "0.1ms",
	}
	got := formatHuman(r)

	// The output should NOT contain a line with 300 consecutive 'x' chars
	if strings.Contains(got, strings.Repeat("x", 300)) {
		t.Errorf("output contains 300-char unbroken sequence, column width cap not applied")
	}
	// Verify no data line exceeds reasonable width (header width ~270 + padding)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 300 {
			t.Errorf("table line too long (%d chars)", len(line))
		}
	}
}

func TestFormatHuman_MultiRowMultiCol(t *testing.T) {
	r := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "a", Type: "INT"},
			{Name: "b", Type: "INT"},
			{Name: "c", Type: "INT"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("2"), strPtr("3")},
			{strPtr("4"), strPtr("5"), strPtr("6")},
		},
		RowCount:      2,
		ExecutionTime: "2ms",
	}
	got := formatHuman(r)

	// Count data rows (lines between separators that start with '|')
	lines := strings.Split(got, "\n")
	dataRows := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			dataRows++
		}
	}
	// header + 2 data rows = 3 '|' lines
	if dataRows != 3 {
		t.Errorf("expected 3 data/header lines, got %d", dataRows)
	}
}

func TestFormatHuman_NoColumns(t *testing.T) {
	r := &query.QueryResult{
		Columns:       []query.ColumnInfo{},
		Rows:          [][]*string{},
		RowCount:      0,
		ExecutionTime: "0ms",
	}
	got := formatHuman(r)
	if !strings.Contains(got, "(empty result)") {
		t.Errorf("expected empty result for no columns, got %q", got)
	}
}
