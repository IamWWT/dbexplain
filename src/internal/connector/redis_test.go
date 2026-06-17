//go:build redis || full

package connector

import (
	"testing"
)

func TestParseRedisInfo(t *testing.T) {
	info := `# Server
redis_version:7.0.0
os:Linux

# Keyspace
db0:keys=100,expires=10,avg_ttl=123456
db1:keys=50,expires=5,avg_ttl=7890
`
	m := parseRedisInfo(info)
	tests := []struct {
		key   string
		want  string
	}{
		{"redis_version", "7.0.0"},
		{"os", "Linux"},
		{"db0", "keys=100,expires=10,avg_ttl=123456"},
		{"db1", "keys=50,expires=5,avg_ttl=7890"},
	}
	for _, tt := range tests {
		got, ok := m[tt.key]
		if !ok {
			t.Errorf("parseRedisInfo missing key %q", tt.key)
			continue
		}
		if got != tt.want {
			t.Errorf("parseRedisInfo[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestParseRedisInfo_Empty(t *testing.T) {
	m := parseRedisInfo("")
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestParseRedisInfo_OnlyComments(t *testing.T) {
	m := parseRedisInfo("# comment line\n# another")
	if len(m) != 0 {
		t.Errorf("expected empty map for comments-only input, got %d entries", len(m))
	}
}

func TestFormatRedisResult(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want []string
	}{
		{"string", "PONG", []string{"PONG"}},
		{"int64", int64(42), []string{"42"}},
		{"nil", nil, []string{"<nil>"}},
		{"slice", []interface{}{"a", "b"}, []string{"a", "b"}},
		{"empty slice", []interface{}{}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRedisResult(tt.val)
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
