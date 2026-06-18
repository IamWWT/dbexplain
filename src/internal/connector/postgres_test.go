//go:build postgres || full

package connector

import (
	"net/url"
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
				u, err := url.Parse(s)
				if err != nil {
					t.Fatalf("buildPGDSN returned invalid URL: %s", s)
				}
				if u.Scheme != "postgres" {
					t.Errorf("scheme = %q, want postgres", u.Scheme)
				}
				user := u.User.Username()
				pass, _ := u.User.Password()
				if user != "user1" || pass != "pass1" {
					t.Errorf("userinfo = (%q, %q), want (user1, pass1)", user, pass)
				}
				if u.Host != "myhost:5433" {
					t.Errorf("host = %q, want myhost:5433", u.Host)
				}
				if strings.TrimPrefix(u.Path, "/") != "mydb" {
					t.Errorf("path = %q, want /mydb", u.Path)
				}
				if u.Query().Get("sslmode") != "require" {
					t.Errorf("sslmode = %q, want require", u.Query().Get("sslmode"))
				}
				if u.Query().Get("connect_timeout") != "5" {
					t.Errorf("connect_timeout = %q, want 5", u.Query().Get("connect_timeout"))
				}
			},
		},
		{
			name: "defaults",
			raw:  "postgres://user1@/",
			check: func(t *testing.T, s string) {
				u, err := url.Parse(s)
				if err != nil {
					t.Fatalf("buildPGDSN returned invalid URL: %s", s)
				}
				if !strings.Contains(u.Host, "127.0.0.1") {
					t.Errorf("host should contain 127.0.0.1: %s", u.Host)
				}
				if u.User.Username() != "user1" {
					t.Errorf("username = %q, want user1", u.User.Username())
				}
				if u.Query().Get("sslmode") != "disable" {
					t.Errorf("sslmode = %q, want disable", u.Query().Get("sslmode"))
				}
				if u.Query().Get("connect_timeout") != "5" {
					t.Errorf("connect_timeout = %q, want 5", u.Query().Get("connect_timeout"))
				}
			},
		},
		{
			name: "password with special chars",
			raw:  "postgres://user1:p@ss'word@host:5432/db",
			check: func(t *testing.T, s string) {
				u, err := url.Parse(s)
				if err != nil {
					t.Fatalf("buildPGDSN returned invalid URL: %s", s)
				}
				user := u.User.Username()
				pass, _ := u.User.Password()
				if user != "user1" || pass != "p@ss'word" {
					t.Errorf("userinfo = (%q, %q), want (user1, p@ss'word)", user, pass)
				}
			},
		},
		{
			name: "connect_timeout present",
			raw:  "postgres://user1:pass@host:5432/db",
			check: func(t *testing.T, s string) {
				u, err := url.Parse(s)
				if err != nil {
					t.Fatalf("buildPGDSN returned invalid URL: %s", s)
				}
				if u.Query().Get("connect_timeout") != "5" {
					t.Errorf("connect_timeout = %q, want 5", u.Query().Get("connect_timeout"))
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
			t.Logf("buildPGDSN output: %s", connStr)
			tt.check(t, connStr)
		})
	}
}

func TestBuildGaussDBDSN(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		check func(*testing.T, string)
	}{
		{
			name: "gaussdb scheme",
			raw:  "gaussdb://user1:pass1@gauss-host:25308/mydb?label=test",
			check: func(t *testing.T, s string) {
				u, err := url.Parse(s)
				if err != nil {
					t.Fatalf("buildGaussDBDSN returned invalid URL: %s", s)
				}
				if u.Scheme != "gaussdb" {
					t.Errorf("scheme = %q, want gaussdb", u.Scheme)
				}
				if u.Host != "gauss-host:25308" {
					t.Errorf("host = %q, want gauss-host:25308", u.Host)
				}
				user := u.User.Username()
				pass, _ := u.User.Password()
				if user != "user1" || pass != "pass1" {
					t.Errorf("userinfo = (%q, %q), want (user1, pass1)", user, pass)
				}
				if strings.TrimPrefix(u.Path, "/") != "mydb" {
					t.Errorf("path = %q, want /mydb", u.Path)
				}
			},
		},
		{
			name: "defaults",
			raw:  "gaussdb://user1@/",
			check: func(t *testing.T, s string) {
				u, err := url.Parse(s)
				if err != nil {
					t.Fatalf("buildGaussDBDSN returned invalid URL: %s", s)
				}
				if u.Scheme != "gaussdb" {
					t.Errorf("scheme = %q, want gaussdb", u.Scheme)
				}
				if u.User.Username() != "user1" {
					t.Errorf("username = %q, want user1", u.User.Username())
				}
				if u.Query().Get("connect_timeout") != "5" {
					t.Errorf("connect_timeout = %q, want 5", u.Query().Get("connect_timeout"))
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
			connStr := buildGaussDBDSN(d)
			t.Logf("buildGaussDBDSN output: %s", connStr)
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
