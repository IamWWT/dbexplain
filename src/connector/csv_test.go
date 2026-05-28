package connector

import (
	"testing"
)

func TestInferColumnType(t *testing.T) {
	tests := []struct {
		name     string
		samples  []string
		expected string
	}{
		{"all integers", []string{"1", "2", "3"}, "INTEGER"},
		{"mixed int and float", []string{"1", "2.5", "3"}, "FLOAT"},
		{"all floats", []string{"1.5", "2.5", "3.14"}, "FLOAT"},
		{"dates ISO", []string{"2024-01-01", "2024-02-15"}, "DATE"},
		{"dates with time", []string{"2024-01-01 12:00:00", "2024-02-15 08:30:00"}, "DATE"},
		{"text", []string{"hello", "world", "foo"}, "TEXT"},
		{"empty samples", []string{}, "TEXT"},
		{"null values", []string{"", "NULL", "1"}, "INTEGER"},
		{"mixed types", []string{"hello", "42", "2024-01-01"}, "TEXT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferColumnType(tt.samples)
			if got != tt.expected {
				t.Errorf("inferColumnType(%v) = %q, want %q", tt.samples, got, tt.expected)
			}
		})
	}
}

func TestTableName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/tmp/data.csv", "data"},
		{"/tmp/data.tsv", "data"},
		{"/path/to/my-file.csv", "my-file"},
		{"data.CSV", "data"},
		{"noext", "noext"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := tableName(tt.path)
			if got != tt.expected {
				t.Errorf("tableName(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestParseSelectStar(t *testing.T) {
	tests := []struct {
		sql       string
		wantLimit int
		wantOff   int
		wantOK    bool
	}{
		{"SELECT *", 0, 0, true},
		{"SELECT * LIMIT 10", 10, 0, true},
		{"SELECT * LIMIT 10 OFFSET 5", 10, 5, true},
		{"select * from table limit 100", 100, 0, true},
		{"SELECT * FROM my_table LIMIT 50 OFFSET 10", 50, 10, true},
		{"SELECT col1, col2 LIMIT 10", 0, 0, false},
		{"DELETE FROM table", 0, 0, false},
		{"DROP TABLE users", 0, 0, false},
		{"", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			limit, offset := parseSelectStar(tt.sql)
			if tt.wantOK {
				if limit < 0 {
					t.Errorf("parseSelectStar(%q) = (%d, %d), want non-negative", tt.sql, limit, offset)
				}
				if limit != tt.wantLimit {
					t.Errorf("parseSelectStar(%q) limit = %d, want %d", tt.sql, limit, tt.wantLimit)
				}
				if offset != tt.wantOff {
					t.Errorf("parseSelectStar(%q) offset = %d, want %d", tt.sql, offset, tt.wantOff)
				}
			} else {
				if limit >= 0 {
					t.Errorf("parseSelectStar(%q) = (%d, %d), want negative limit", tt.sql, limit, offset)
				}
			}
		})
	}
}

func TestIsDateLike(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"2024-01-01", true},
		{"2024/01/01", true},
		{"2024-01-01 12:00:00", true},
		{"01/02/2024", true},
		{"not-a-date", false},
		{"42", false},
		{"", false},
		{"January 02, 2024", true},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isDateLike(tt.s)
			if got != tt.want {
				t.Errorf("isDateLike(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestHasGlobMeta(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/tmp/data.csv", false},
		{"/tmp/*.csv", true},
		{"/tmp/data?.csv", true},
		{"/tmp/data[0-9].csv", true},
		{"/tmp/dir/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := hasGlobMeta(tt.path)
			if got != tt.want {
				t.Errorf("hasGlobMeta(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
