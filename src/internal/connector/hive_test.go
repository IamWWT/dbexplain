//go:build hive || full

package connector

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/beltran/gohive/v2"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/schema"
)

// ── Pure function tests ──

func TestBuildHiveConfig(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		check func(*testing.T, *dsn.DSN, gohive.Config)
	}{
		{
			name: "default NOSASL",
			dsn:  "hive://host:10000/default?label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.Auth != "NOSASL" {
					t.Errorf("Auth = %q, want %q", cfg.Auth, "NOSASL")
				}
				if cfg.Port != 10000 {
					t.Errorf("Port = %d, want 10000", cfg.Port)
				}
				if cfg.Database != "default" {
					t.Errorf("Database = %q, want %q", cfg.Database, "default")
				}
				if cfg.Username != "" {
					t.Errorf("Username = %q, want empty", cfg.Username)
				}
			},
		},
		{
			name: "LDAP auth with credentials",
			dsn:  "hive://user1:pass1@host:10000/mydb?label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.Auth != "NONE" {
					t.Errorf("Auth = %q, want %q (user set => NONE)", cfg.Auth, "NONE")
				}
				if cfg.Username != "user1" {
					t.Errorf("Username = %q, want %q", cfg.Username, "user1")
				}
				if cfg.Password != "pass1" {
					t.Errorf("Password = %q, want %q", cfg.Password, "pass1")
				}
				if cfg.Database != "mydb" {
					t.Errorf("Database = %q, want %q", cfg.Database, "mydb")
				}
			},
		},
		{
			name: "TLS enabled",
			dsn:  "hives://user:pass@host:10000/db?label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.SSLInsecureSkip {
					t.Error("SSLInsecureSkip should be false (secure default)")
				}
				if cfg.TLSConfig == nil {
					t.Fatal("TLSConfig should not be nil")
				}
				if cfg.TLSConfig != nil && cfg.TLSConfig.InsecureSkipVerify {
					t.Error("TLSConfig.InsecureSkipVerify should be false (secure default)")
				}
			},
		},
		{
			name: "KERBEROS auth",
			dsn:  "hive://host:10000/db?auth=KERBEROS&label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.Auth != "KERBEROS" {
					t.Errorf("Auth = %q, want %q", cfg.Auth, "KERBEROS")
				}
			},
		},
		{
			name: "custom transport and http_path",
			dsn:  "hive://host:10000/db?transport=http&http_path=/hive&label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.TransportMode != "http" {
					t.Errorf("TransportMode = %q, want %q", cfg.TransportMode, "http")
				}
				if cfg.HTTPPath != "/hive" {
					t.Errorf("HTTPPath = %q, want %q", cfg.HTTPPath, "/hive")
				}
			},
		},
		{
			name: "custom service name",
			dsn:  "hive://host:10000/db?service=HS2&label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.Service != "HS2" {
					t.Errorf("Service = %q, want %q", cfg.Service, "HS2")
				}
			},
		},
		{
			name: "default port and db",
			dsn:  "hive://host?label=test",
			check: func(t *testing.T, d *dsn.DSN, cfg gohive.Config) {
				if cfg.Host != "host" {
					t.Errorf("Host = %q, want %q", cfg.Host, "host")
				}
				if cfg.Port != 10000 {
					t.Errorf("Port = %d, want 10000", cfg.Port)
				}
				if cfg.Database != "default" {
					t.Errorf("Database = %q, want %q", cfg.Database, "default")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := dsn.ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("ParseDSN(%q) error: %v", tt.dsn, err)
			}
			cfg := buildHiveConfig(d)
			tt.check(t, d, cfg)
		})
	}
}

func TestQuoteHive(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"simple", "mydb", "`mydb`"},
		{"with backtick", "my`db", "`my``db`"},
		{"with dot", "my.db", "`my.db`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteHive(tt.s)
			if got != tt.want {
				t.Errorf("quoteHive(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

// ── go-sqlmock tests ──

func TestCollectHiveSchema_Basic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	// SHOW DATABASES
	mock.ExpectQuery("SHOW DATABASES").
		WillReturnRows(sqlmock.NewRows([]string{"database_name"}).
			AddRow("mydb").
			AddRow("information_schema"). // should be filtered
			AddRow("default"))           // should be filtered

	// SHOW TABLES IN `mydb`
	mock.ExpectQuery("SHOW TABLES IN").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).
			AddRow("employees").
			AddRow("departments"))

	// DESCRIBE FORMATTED `mydb`.`employees`
	mock.ExpectQuery("DESCRIBE FORMATTED").
		WillReturnRows(sqlmock.NewRows([]string{"col_name", "data_type", "comment"}).
			AddRow("id", "int", "").
			AddRow("name", "string", "employee name").
			AddRow("# Detailed Table Information", "", "").
			AddRow("Database:", "mydb", ""))

	// Sample row for employees
	mock.ExpectQuery("LIMIT 1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Alice"))

	// DESCRIBE FORMATTED `mydb`.`departments`
	mock.ExpectQuery("DESCRIBE FORMATTED").
		WillReturnRows(sqlmock.NewRows([]string{"col_name", "data_type", "comment"}).
			AddRow("dept_id", "int", "").
			AddRow("dept_name", "string", "").
			AddRow("# Detailed Table Information", "", ""))

	// Sample row for departments
	mock.ExpectQuery("LIMIT 1").
		WillReturnRows(sqlmock.NewRows([]string{"dept_id", "dept_name"}).
			AddRow(10, "Engineering"))

	inst := &schema.Instance{DSN: "hive://redacted", Kind: "hive", Label: "test"}
	ctx := context.Background()

	result, err := collectHiveSchema(ctx, db, inst)
	if err != nil {
		t.Fatalf("collectHiveSchema error: %v", err)
	}

	if len(result.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(result.Databases))
	}
	db0 := result.Databases[0]
	if db0.Name != "mydb" {
		t.Errorf("Database name = %q, want %q", db0.Name, "mydb")
	}
	if len(db0.Tables) != 2 {
		t.Fatalf("Tables len = %d, want 2", len(db0.Tables))
	}

	// Check employees table
	emp := db0.Tables[0]
	if emp.Name != "employees" {
		t.Errorf("Table[0] name = %q, want %q", emp.Name, "employees")
	}
	if emp.RowCount != -1 {
		t.Errorf("Table RowCount = %d, want -1 (unknown)", emp.RowCount)
	}
	if len(emp.Columns) != 2 {
		t.Fatalf("employees Columns len = %d, want 2", len(emp.Columns))
	}
	if emp.Columns[0].Name != "id" {
		t.Errorf("emp col[0] = %q, want %q", emp.Columns[0].Name, "id")
	}
	if !emp.Columns[0].Nullable {
		t.Error("emp col[0] should be nullable (Hive default)")
	}
	if emp.Columns[1].Name != "name" {
		t.Errorf("emp col[1] = %q, want %q", emp.Columns[1].Name, "name")
	}
	if emp.Columns[1].Comment != "employee name" {
		t.Errorf("emp col[1] comment = %q, want %q", emp.Columns[1].Comment, "employee name")
	}

	// Check departments table
	dept := db0.Tables[1]
	if dept.Name != "departments" {
		t.Errorf("Table[1] name = %q, want %q", dept.Name, "departments")
	}
	if len(dept.Columns) != 2 {
		t.Fatalf("departments Columns len = %d, want 2", len(dept.Columns))
	}
	// dept_name has no comment → should be inferred from sample
	if dept.Columns[1].Comment == "" {
		t.Error("dept_name comment should be inferred from sample")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectHiveSchema_FilterSystemDBs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	// Only system databases returned — should filter all
	mock.ExpectQuery("SHOW DATABASES").
		WillReturnRows(sqlmock.NewRows([]string{"database_name"}).
			AddRow("information_schema").
			AddRow("sys").
			AddRow("default"))

	inst := &schema.Instance{DSN: "hive://redacted", Kind: "hive", Label: "test"}
	ctx := context.Background()

	result, err := collectHiveSchema(ctx, db, inst)
	if err != nil {
		t.Fatalf("collectHiveSchema error: %v", err)
	}

	if len(result.Databases) != 0 {
		t.Errorf("Databases len = %d, want 0 (all system DBs filtered)", len(result.Databases))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectHiveSchema_DescribeFormattedStops(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SHOW DATABASES").
		WillReturnRows(sqlmock.NewRows([]string{"database_name"}).
			AddRow("mydb"))

	mock.ExpectQuery("SHOW TABLES IN").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).
			AddRow("t1"))

	// DESCRIBE FORMATTED including # Storage Information and Partition Information
	mock.ExpectQuery("DESCRIBE FORMATTED").
		WillReturnRows(sqlmock.NewRows([]string{"col_name", "data_type", "comment"}).
			AddRow("id", "int", "").
			AddRow("name", "string", "").
			AddRow("# Detailed Table Information", "t1", "").
			AddRow("Owner:", "user", "").
			AddRow("# Storage Information", "", "").
			AddRow("Location:", "/user/hive/warehouse", "").
			AddRow("# Partition Information", "", "").
			AddRow("dt", "string", "").
			// Simulate a row that would otherwise be parsed as col_name if stop didn't work
			AddRow("extra_col", "string", ""))

	// No sample row needed (colsWithoutComment is empty since we stopped before processing)
	// Actually wait — with stopped=true, the code skips everything after # Detailed Table Information
	// So colsWithoutComment will contain id, name → we need sampling

	// Sample row
	mock.ExpectQuery("LIMIT 1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "test"))

	inst := &schema.Instance{DSN: "hive://redacted", Kind: "hive", Label: "test"}
	ctx := context.Background()

	result, err := collectHiveSchema(ctx, db, inst)
	if err != nil {
		t.Fatalf("collectHiveSchema error: %v", err)
	}

	if len(result.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(result.Databases))
	}
	if len(result.Databases[0].Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(result.Databases[0].Tables))
	}
	tbl := result.Databases[0].Tables[0]
	if len(tbl.Columns) != 2 {
		t.Fatalf("Columns len = %d, want 2 (id, name only; dt and extra_col should be skipped)", len(tbl.Columns))
	}
	if tbl.Columns[0].Name != "id" {
		t.Errorf("col[0] = %q, want %q", tbl.Columns[0].Name, "id")
	}
	if tbl.Columns[1].Name != "name" {
		t.Errorf("col[1] = %q, want %q", tbl.Columns[1].Name, "name")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectHiveSchema_EmptyDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SHOW DATABASES").
		WillReturnRows(sqlmock.NewRows([]string{"database_name"}).
			AddRow("emptydb"))

	// No tables
	mock.ExpectQuery("SHOW TABLES IN").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))

	inst := &schema.Instance{DSN: "hive://redacted", Kind: "hive", Label: "test"}
	ctx := context.Background()

	result, err := collectHiveSchema(ctx, db, inst)
	if err != nil {
		t.Fatalf("collectHiveSchema error: %v", err)
	}

	if len(result.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(result.Databases))
	}
	if len(result.Databases[0].Tables) != 0 {
		t.Errorf("Tables len = %d, want 0", len(result.Databases[0].Tables))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectHiveSchema_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SHOW DATABASES").
		WillReturnError(fmt.Errorf("connection refused"))

	inst := &schema.Instance{DSN: "hive://redacted", Kind: "hive", Label: "test"}
	ctx := context.Background()

	_, err = collectHiveSchema(ctx, db, inst)
	if err == nil {
		t.Fatal("expected error for failed SHOW DATABASES, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// Test that gohive.Config has the right types — compile-time check
func TestHiveConfigTLS(t *testing.T) {
	d, _ := dsn.ParseDSN("hives://host:10000/db?label=tlstest")
	cfg := buildHiveConfig(d)
	if cfg.SSLInsecureSkip {
		t.Error("SSLInsecureSkip should be false for hives:// without explicit skip-verify")
	}
	if cfg.TLSConfig != nil && cfg.TLSConfig.InsecureSkipVerify {
		t.Error("TLSConfig.InsecureSkipVerify should be false for hives:// without explicit skip-verify")
	}
	_ = cfg.TLSConfig // should be *tls.Config
}

func TestHiveConfigTLSWithSkipVerify(t *testing.T) {
	d, _ := dsn.ParseDSN("hives://host:10000/db?sslinsecureskipverify=true&label=tlstest-skip")
	cfg := buildHiveConfig(d)
	if !cfg.SSLInsecureSkip {
		t.Error("SSLInsecureSkip should be true when sslinsecureskipverify=true")
	}
	if cfg.TLSConfig == nil || !cfg.TLSConfig.InsecureSkipVerify {
		t.Error("TLSConfig.InsecureSkipVerify should be true when sslinsecureskipverify=true")
	}
}

func TestHiveConfigSSLDeps(t *testing.T) {
	d, _ := dsn.ParseDSN("hive://host:10000/db?sslinsecureskipverify=true&label=test")
	cfg := buildHiveConfig(d)
	if !cfg.SSLInsecureSkip {
		t.Error("SSLInsecureSkip should be true when sslinsecureskipverify=true")
	}
}

func TestHiveConfigSSLCertFiles(t *testing.T) {
	d, _ := dsn.ParseDSN("hive://host:10000/db?sslcert=/path/cert.pem&sslkey=/path/key.pem&sslca=/path/ca.pem&label=test")
	cfg := buildHiveConfig(d)
	if cfg.SSLCertFile != "/path/cert.pem" {
		t.Errorf("SSLCertFile = %q, want %q", cfg.SSLCertFile, "/path/cert.pem")
	}
	if cfg.SSLKeyFile != "/path/key.pem" {
		t.Errorf("SSLKeyFile = %q, want %q", cfg.SSLKeyFile, "/path/key.pem")
	}
	if cfg.SSLCAFile != "/path/ca.pem" {
		t.Errorf("SSLCAFile = %q, want %q", cfg.SSLCAFile, "/path/ca.pem")
	}
}

func TestHiveCollect_ColsWithoutCommentSkipSampleWhenCountUnknown(t *testing.T) {
	// When RowCount is -1 (unknown), sampling still happens because
	// the check is `if len(colsWithoutComment) > 0` only (no RowCount check in Hive)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SHOW DATABASES").
		WillReturnRows(sqlmock.NewRows([]string{"database_name"}).AddRow("mydb"))

	mock.ExpectQuery("SHOW TABLES IN").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("t1"))

	mock.ExpectQuery("DESCRIBE FORMATTED").
		WillReturnRows(sqlmock.NewRows([]string{"col_name", "data_type", "comment"}).
			AddRow("id", "int", "").
			AddRow("# Detailed Table Information", "", ""))

	mock.ExpectQuery("LIMIT 1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	inst := &schema.Instance{DSN: "hive://redacted", Kind: "hive", Label: "test"}
	ctx := context.Background()

	result, err := collectHiveSchema(ctx, db, inst)
	if err != nil {
		t.Fatalf("collectHiveSchema error: %v", err)
	}

	if len(result.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(result.Databases))
	}
	if len(result.Databases[0].Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(result.Databases[0].Tables))
	}
	tbl := result.Databases[0].Tables[0]
	if tbl.RowCount != -1 {
		t.Errorf("RowCount = %d, want -1", tbl.RowCount)
	}
	if len(tbl.Columns) != 1 {
		t.Fatalf("Columns len = %d, want 1", len(tbl.Columns))
	}
	// id has no comment → should be inferred from sample
	if tbl.Columns[0].Comment == "" {
		t.Error("id comment should be inferred from sample")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
