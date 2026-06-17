//go:build prometheus || full

package connector

import (
	"testing"

	"github.com/IamWWT/dbexplain/internal/dsn"
)

func TestPromBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"default host and port", "prometheus://?label=test", "http://127.0.0.1:9090"},
		{"custom host", "prometheus://myhost:9090?label=test", "http://myhost:9090"},
		{"HTTPS", "prometheus://host:443?label=test&tls=true", "https://host:443"},
		{"with auth", "prometheus://user:pass@host:9090?label=test", "http://host:9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := dsn.ParseDSN(tt.raw)
			if err != nil {
				t.Fatalf("ParseDSN(%q) error: %v", tt.raw, err)
			}
			got := promBaseURL(d)
			if got != tt.want {
				t.Errorf("promBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestPromTimeout(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"default timeout", "prometheus://host:9090?label=test", 10},
		{"custom timeout", "prometheus://host:9090?label=test&timeout=30", 30},
		{"invalid timeout (fallback)", "prometheus://host:9090?label=test&timeout=abc", 10},
		{"zero timeout (fallback)", "prometheus://host:9090?label=test&timeout=0", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := dsn.ParseDSN(tt.raw)
			if err != nil {
				t.Fatalf("ParseDSN(%q) error: %v", tt.raw, err)
			}
			got := promTimeout(d)
			if got != tt.want {
				t.Errorf("promTimeout(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBuildPromColumnsFromKeys(t *testing.T) {
	got := buildColumnsFromKeys([]string{"__name__", "job", "instance"})
	// Input 3 keys + always-appended timestamp + value = 5 total
	if len(got) != 5 {
		t.Fatalf("expected 5 columns (3 keys + timestamp + value), got %d", len(got))
	}
	if got[0].Name != "__name__" {
		t.Errorf("column[0].Name = %q, want %q", got[0].Name, "__name__")
	}
	if got[1].Type != "string" {
		t.Errorf("column[1].Type = %q, want string", got[1].Type)
	}
	// Last two should be timestamp and value
	if got[3].Name != "timestamp" {
		t.Errorf("column[3].Name = %q, want timestamp", got[3].Name)
	}
	if got[4].Name != "value" {
		t.Errorf("column[4].Name = %q, want value", got[4].Name)
	}
}

func TestLabelsToSampleRows(t *testing.T) {
	got := labelsToSampleRows([]string{"up", "node_cpu", "go_memstats_alloc"})
	if len(got) != 3 {
		t.Fatalf("expected 3 sample rows, got %d", len(got))
	}
	if got[0]["name"] != "up" {
		t.Errorf("sample[0][name] = %q, want %q", got[0]["name"], "up")
	}
	if got[1]["name"] != "node_cpu" {
		t.Errorf("sample[1][name] = %q, want %q", got[1]["name"], "node_cpu")
	}
}

func TestBuildPromRowFromLabels(t *testing.T) {
	row := buildRowFromLabels([]string{"__name__", "job"}, map[string]string{"__name__": "up", "job": "prometheus"})
	if len(row) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(row))
	}
	if *row[0] != "up" {
		t.Errorf("row[0] = %q, want %q", *row[0], "up")
	}
	if *row[1] != "prometheus" {
		t.Errorf("row[1] = %q, want %q", *row[1], "prometheus")
	}
}
