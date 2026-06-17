//go:build postgres || full

package connector

import (
	"strings"
	"testing"

	"github.com/IamWWT/dbexplain/internal/dsn"
)

func TestParsePGTableName(t *testing.T) {
	tests := []struct {
		input      string
		wantSchema string
		wantTable  string
	}{
		{"users", "public", "users"},
		{"public.users", "public", "users"},
		{"my_schema.my_table", "my_schema", "my_table"},
		{"a.b.c", "a.b", "c"},
		{"", "public", ""},
	}
	for _, tt := range tests {
		schema, table := parsePGTableName(tt.input)
		if schema != tt.wantSchema || table != tt.wantTable {
			t.Errorf("parsePGTableName(%q) = (%q, %q), want (%q, %q)",
				tt.input, schema, table, tt.wantSchema, tt.wantTable)
		}
	}
}

func TestQuotePGIdent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", `"users"`},
		{`ta"ble`, `"ta""ble"`},
		{"", `""`},
		{"employees", `"employees"`},
		{"ORDER BY 1", `"ORDER BY 1"`},
	}
	for _, tt := range tests {
		got := quotePGIdent(tt.input)
		if got != tt.want {
			t.Errorf("quotePGIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPgFKAction(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"a", "NO ACTION"},
		{"r", "RESTRICT"},
		{"c", "CASCADE"},
		{"n", "SET NULL"},
		{"d", "SET DEFAULT"},
		{"x", "x"}, // unknown → passthrough
		{"", ""},   // empty → passthrough
		{"C", "C"}, // case-sensitive
	}
	for _, tt := range tests {
		got := pgFKAction(tt.code)
		if got != tt.want {
			t.Errorf("pgFKAction(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestBuildPGDSN(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		check func(*testing.T, string)
	}{
		{
			name: "full DSN",
			raw:  "postgres://user1:pass1@myhost:5433/mydb?label=test&sslmode=require",
			check: func(t *testing.T, s string) {
				for _, want := range []string{"host=myhost", "port=5433", "user=user1", "dbname=mydb", "sslmode=require"} {
					if !strings.Contains(s, want) {
						t.Errorf("buildPGDSN missing %q in: %s", want, s)
					}
				}
			},
		},
		{
			name: "defaults",
			raw:  "postgres://user1@/",
			check: func(t *testing.T, s string) {
				for _, want := range []string{"host=127.0.0.1", "port=5432", "user=user1", "sslmode=disable"} {
					if !strings.Contains(s, want) {
						t.Errorf("buildPGDSN missing default %q in: %s", want, s)
					}
				}
			},
		},
		{
			name: "password with special chars",
			raw:  "postgres://user1:p@ss'word@host:5432/db",
			check: func(t *testing.T, s string) {
				if !strings.Contains(s, "password=") {
					t.Errorf("buildPGDSN missing password field: %s", s)
				}
			},
		},
		{
			name: "connect_timeout present",
			raw:  "postgres://user1:pass@host:5432/db",
			check: func(t *testing.T, s string) {
				if !strings.Contains(s, "connect_timeout=5") {
					t.Errorf("buildPGDSN missing connect_timeout=5: %s", s)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := dsn.ParseDSN(tt.raw)
			if err != nil {
				t.Fatalf("ParseDSN(%q) error: %v", tt.raw, err)
			}
			connStr := buildPGDSN(d)
			tt.check(t, connStr)
		})
	}
}

func TestFormatGaussDBType(t *testing.T) {
	tests := []struct {
		typname   string
		atttypmod int32
		want      string
	}{
		{"int4", -1, "integer"},
		{"int2", -1, "smallint"},
		{"int8", -1, "bigint"},
		{"float4", -1, "real"},
		{"float8", -1, "double precision"},
		{"bool", -1, "boolean"},
		{"text", -1, "text"},
		{"varchar", -1, "character varying"},
		{"varchar", 104, "character varying(100)"},    // 104-4 = 100
		{"bpchar", -1, "character"},
		{"bpchar", 10, "character(6)"},                // 10-4 = 6
		{"numeric", -1, "numeric"},
		{"numeric", 65540, "numeric(1)"},              // raw=65536, precision=1, scale=0
		{"numeric", 655368, "numeric(10,2)"},          // raw=655364, precision=10, scale=2
		{"timestamptz", -1, "timestamp with time zone"},
		{"timetz", -1, "time with time zone"},
		{"unknown_type", -1, "unknown_type"},           // passthrough
	}
	for _, tt := range tests {
		got := formatGaussDBType(tt.typname, tt.atttypmod)
		if got != tt.want {
			t.Errorf("formatGaussDBType(%q, %d) = %q, want %q",
				tt.typname, tt.atttypmod, got, tt.want)
		}
	}
}
