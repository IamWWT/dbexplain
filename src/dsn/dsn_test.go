package dsn

import "testing"

func TestParseDSN_Schemes(t *testing.T) {
	tests := []struct {
		raw         string
		wantKind    string
		wantHost    string
		wantPort    string
		wantUser    string
		wantPass    string
		wantDBName  string
		wantLabel   string
		wantSSLMode string
		wantTLS     bool
		wantCluster bool
		wantErr     bool
	}{
		// MySQL variants
		{raw: "mysql://root:pass@localhost:3306/testdb", wantKind: "mysql", wantHost: "localhost", wantPort: "3306", wantUser: "root", wantPass: "pass", wantDBName: "testdb"},
		{raw: "mariadb://user@host:3307/db", wantKind: "mysql", wantHost: "host", wantPort: "3307", wantUser: "user", wantPass: "", wantDBName: "db"},

		// PostgreSQL variants
		{raw: "postgres://user:pass@host:5432/db", wantKind: "postgres", wantHost: "host", wantPort: "5432", wantUser: "user", wantPass: "pass", wantDBName: "db"},
		{raw: "postgresql://user@host/db", wantKind: "postgres"},
		{raw: "pg://user@host/db", wantKind: "postgres"},

		// GaussDB variants
		{raw: "gaussdb://user:pass@host:5432/db", wantKind: "gaussdb", wantHost: "host", wantPort: "5432"},
		{raw: "opengauss://user@host/db", wantKind: "gaussdb"},

		// SQLite variants
		{raw: "sqlite://./test.db", wantKind: "sqlite"},
		{raw: "sqlite3:///absolute/path.db", wantKind: "sqlite"},

		// ClickHouse variants
		{raw: "clickhouse://user:pass@host:9000/db", wantKind: "clickhouse", wantHost: "host", wantPort: "9000"},
		{raw: "ch://user@host/db", wantKind: "clickhouse"},

		// Redis variants
		{raw: "redis://:pass@host:6379/0", wantKind: "redis", wantPass: "pass", wantHost: "host", wantPort: "6379"},
		{raw: "rediss://:pass@host:6380/0", wantKind: "redis", wantTLS: true},

		// MongoDB
		{raw: "mongodb://user:pass@host:27017/db", wantKind: "mongodb", wantHost: "host", wantPort: "27017"},

		// Qdrant
		{raw: "qdrant://host:6333", wantKind: "qdrant", wantHost: "host", wantPort: "6333"},

		// Elasticsearch variants
		{raw: "elasticsearch://host:9200", wantKind: "elasticsearch", wantHost: "host", wantPort: "9200"},
		{raw: "es://user:pass@host:9200", wantKind: "elasticsearch"},
		{raw: "elasticsearchs://host:9200", wantKind: "elasticsearch", wantTLS: true},

		// Error cases
		{raw: "unsupported://host/db", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			d, err := ParseDSN(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d.Kind != tt.wantKind {
				t.Errorf("Kind: got %q, want %q", d.Kind, tt.wantKind)
			}
			if tt.wantHost != "" && d.Host != tt.wantHost {
				t.Errorf("Host: got %q, want %q", d.Host, tt.wantHost)
			}
			if tt.wantPort != "" && d.Port != tt.wantPort {
				t.Errorf("Port: got %q, want %q", d.Port, tt.wantPort)
			}
			if tt.wantUser != "" && d.User != tt.wantUser {
				t.Errorf("User: got %q, want %q", d.User, tt.wantUser)
			}
			if tt.wantPass != "" && d.Password != tt.wantPass {
				t.Errorf("Password: got %q, want %q", d.Password, tt.wantPass)
			}
			if tt.wantDBName != "" && d.DBName != tt.wantDBName {
				t.Errorf("DBName: got %q, want %q", d.DBName, tt.wantDBName)
			}
			if tt.wantTLS != d.TLS {
				t.Errorf("TLS: got %v, want %v", d.TLS, tt.wantTLS)
			}
			if tt.wantCluster != d.Cluster {
				t.Errorf("Cluster: got %v, want %v", d.Cluster, tt.wantCluster)
			}
		})
	}
}

func TestParseDSN_QueryParams(t *testing.T) {
	tests := []struct {
		raw         string
		wantLabel   string
		wantSSLMode string
		wantTLS     bool
		wantCluster bool
	}{
		{raw: "mysql://root:pass@host:3306/db?label=myapp", wantLabel: "myapp"},
		{raw: "postgres://user@host/db?sslmode=require", wantSSLMode: "require"},
		{raw: "postgres://user@host/db?sslmode=verify-full", wantSSLMode: "verify-full"},
		{raw: "redis://:pass@host:6379/0?cluster=true", wantCluster: true},
		{raw: "redis://:pass@host:6379/0?cluster=1", wantCluster: true},
		{raw: "elasticsearch://host:9200?tls=true", wantTLS: true},
		{raw: "redis://host:6379?tls=true", wantTLS: true},
		{raw: "mysql://host/db?label=测试&tls=false", wantLabel: "测试", wantTLS: false},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			d, err := ParseDSN(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantLabel != "" && d.Label != tt.wantLabel {
				t.Errorf("Label: got %q, want %q", d.Label, tt.wantLabel)
			}
			if tt.wantSSLMode != "" && d.SSLMode != tt.wantSSLMode {
				t.Errorf("SSLMode: got %q, want %q", d.SSLMode, tt.wantSSLMode)
			}
			if tt.wantTLS != d.TLS {
				t.Errorf("TLS: got %v, want %v", d.TLS, tt.wantTLS)
			}
			if tt.wantCluster != d.Cluster {
				t.Errorf("Cluster: got %v, want %v", d.Cluster, tt.wantCluster)
			}
		})
	}
}

func TestParseDSN_AutoLabel(t *testing.T) {
	d, err := ParseDSN("mysql://root:pass@myhost:3306/mydb")
	if err != nil {
		t.Fatal(err)
	}
	expected := "myhost:3306/mydb"
	if d.Label != expected {
		t.Errorf("Label: got %q, want %q", d.Label, expected)
	}
}

func TestRedacted(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "mysql://user:secret@host/db", want: "mysql://user:***@host/db"},
		{raw: "postgres://admin:p@ss@host:5432/db", want: "postgres://admin:***@host:5432/db"},
		{raw: "redis://:mypwd@host:6379/0", want: "redis://:***@host:6379/0"},
		{raw: "clickhouse://host/db", want: "clickhouse://host/db"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			d, err := ParseDSN(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			got := d.Redacted()
			if got != tt.want {
				t.Errorf("Redacted: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDSN_EdgeCases(t *testing.T) {
	// Empty DSN
	_, err := ParseDSN("")
	if err == nil {
		t.Error("expected error for empty DSN")
	}

	// No user, default port
	d, err := ParseDSN("mysql://localhost:3306/db")
	if err != nil {
		t.Fatal(err)
	}
	if d.User != "" {
		t.Errorf("User: got %q, want empty", d.User)
	}
	if d.Password != "" {
		t.Errorf("Password: got %q, want empty", d.Password)
	}
}
