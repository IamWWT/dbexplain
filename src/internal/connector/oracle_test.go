//go:build oracle || full

package connector

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IamWWT/dbexplain/internal/dsn"
)

// ── Pure function tests (no mock needed) ──

func TestBuildOracleDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string // input DSN URL
		want string // expected oracle:// connection string
	}{
		{
			name: "basic",
			dsn:  "oracle://user:pass@host:1521/XE?label=test",
			want: "oracle://user:pass@host:1521/XE?connectionTimeout=5",
		},
		{
			name: "tls",
			dsn:  "oracles://user:pass@host:1521/XE?label=test",
			want: "oracle://user:pass@host:1521/XE?connectionTimeout=5&ssl=true",
		},
		{
			name: "default port and service",
			dsn:  "oracle://user:pass@host?label=test",
			want: "oracle://user:pass@host:1521/XE?connectionTimeout=5",
		},
		{
			name: "custom service",
			dsn:  "oracle://user:pass@host:1521/ORCL?label=test",
			want: "oracle://user:pass@host:1521/ORCL?connectionTimeout=5",
		},
		{
			name: "url-encoded credentials",
			dsn:  "oracle://user%40domain:p%40ss@host:1521/XE?label=test",
			want: "oracle://user%40domain:p%40ss@host:1521/XE?connectionTimeout=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := dsn.ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("ParseDSN(%q) error: %v", tt.dsn, err)
			}
			got := buildOracleDSN(d)
			if got != tt.want {
				t.Errorf("buildOracleDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestIsOracleSystemSchema(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"SYS is system", "SYS", true},
		{"SYSTEM is system", "SYSTEM", true},
		{"DBSNMP is system", "DBSNMP", true},
		{"XDB is system", "XDB", true},
		{"DVSYS is system", "DVSYS", true},
		{"AUDSYS is system", "AUDSYS", true},
		{"HR is user", "HR", false},
		{"SCOTT is user", "SCOTT", false},
		{"APP is user", "APP", false},
		{"lowercase sys is system", "sys", true},
		{"mixed case Sys is system", "Sys", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOracleSystemSchema(tt.s)
			if got != tt.want {
				t.Errorf("isOracleSystemSchema(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestQuoteOracle(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"simple", "HR", `"HR"`},
		{"with quote", `ta"ble`, `"ta""ble"`},
		{"lowercase", "employees", `"employees"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteOracle(tt.s)
			if got != tt.want {
				t.Errorf("quoteOracle(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

// ── go-sqlmock based tests ──
//
// Note: regex patterns use `(?s)` flag so `.` matches newlines in multi-line SQL.
// `\s+` handles flexible whitespace including tabs/newlines.

func TestCollectOracleSchema_Basic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	// Step 1: Owner discovery
	mock.ExpectQuery("SELECT DISTINCT owner FROM all_tables ORDER BY owner").
		WillReturnRows(sqlmock.NewRows([]string{"owner"}).
			AddRow("HR").
			AddRow("SYS"))

	// Step 2: Table query for HR (multi-line SQL, use (?s) for dot-all)
	mock.ExpectQuery(`(?s)SELECT t\.table_name, COALESCE\(t\.num_rows.*FROM all_tables t`).
		WithArgs("HR").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "row_count", "comment"}).
			AddRow("EMPLOYEES", 100, "Employee records"))

	// Step 3: Columns for HR.EMPLOYEES
	mock.ExpectQuery(`(?s)SELECT c\.column_name, c\.data_type.*FROM all_tab_columns c`).
		WithArgs("HR", "EMPLOYEES").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "nullable", "data_default", "comments", "column_id"}).
			AddRow("ID", "NUMBER", "N", "", "Primary key", 1).
			AddRow("NAME", "VARCHAR2", "Y", "", "", 2))

	// Step 4: Constraints (PK)
	mock.ExpectQuery(`(?s)constraint_type IN\s*\(\s*'P',\s*'U'\s*\)`).
		WithArgs("HR", "EMPLOYEES").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "constraint_type", "column_name"}).
			AddRow("PK_EMP", "P", "ID"))

	// Step 5: Indexes
	mock.ExpectQuery(`(?s)FROM all_ind_columns ic\s+JOIN all_indexes i`).
		WithArgs("HR", "EMPLOYEES").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "column_name", "uniqueness"}).
			AddRow("PK_EMP", "ID", "UNIQUE"))

	// Step 6: Foreign keys (empty)
	mock.ExpectQuery(`(?s)constraint_type = 'R'\s+ORDER BY a\.constraint_name`).
		WithArgs("HR", "EMPLOYEES").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "r_owner", "r_constraint_name", "position"}))

	// Step 7: Sample row for comment inference
	mock.ExpectQuery(`(?s)SELECT \* FROM "HR"\."EMPLOYEES" FETCH FIRST 1 ROWS ONLY`).
		WillReturnRows(sqlmock.NewRows([]string{"ID", "NAME"}).
			AddRow(1, "Alice"))

	d, _ := dsn.ParseDSN("oracle://user:pass@host?label=test")
	ctx := context.Background()

	inst, err := collectOracleSchema(ctx, db, d)
	if err != nil {
		t.Fatalf("collectOracleSchema error: %v", err)
	}

	if inst.Kind != "oracle" {
		t.Errorf("Kind = %q, want %q", inst.Kind, "oracle")
	}
	if inst.Label != "test" {
		t.Errorf("Label = %q, want %q", inst.Label, "test")
	}
	if len(inst.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(inst.Databases))
	}
	db0 := inst.Databases[0]
	if db0.Name != "HR" {
		t.Errorf("Database name = %q, want %q", db0.Name, "HR")
	}
	if len(db0.Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(db0.Tables))
	}
	tbl := db0.Tables[0]
	if tbl.Name != "EMPLOYEES" {
		t.Errorf("Table name = %q, want %q", tbl.Name, "EMPLOYEES")
	}
	if tbl.RowCount != 100 {
		t.Errorf("RowCount = %d, want 100", tbl.RowCount)
	}
	if len(tbl.Columns) != 2 {
		t.Fatalf("Columns len = %d, want 2", len(tbl.Columns))
	}
	if !tbl.Columns[0].IsPrimary {
		t.Error("ID should be primary key")
	}
	if tbl.Columns[0].Nullable {
		t.Error("ID should not be nullable")
	}
	if tbl.Columns[0].Comment != "Primary key" {
		t.Errorf("ID comment = %q, want %q", tbl.Columns[0].Comment, "Primary key")
	}
	if tbl.Columns[1].Name != "NAME" {
		t.Errorf("Col[1] name = %q, want %q", tbl.Columns[1].Name, "NAME")
	}
	if !tbl.Columns[1].Nullable {
		t.Error("NAME should be nullable")
	}
	// NAME has no explicit comment — should be inferred from sample row
	if tbl.Columns[1].Comment == "" {
		t.Error("NAME comment should be inferred from sample")
	}
	if len(tbl.Indexes) != 1 {
		t.Errorf("Indexes len = %d, want 1", len(tbl.Indexes))
	}
	if len(tbl.ForeignKeys) != 0 {
		t.Errorf("ForeignKeys len = %d, want 0", len(tbl.ForeignKeys))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectOracleSchema_SpecificDBName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	// When DBName is set, no owner discovery — uses DBName directly.
	// Table query for the specified owner
	mock.ExpectQuery(`(?s)SELECT t\.table_name, COALESCE\(t\.num_rows.*FROM all_tables t`).
		WithArgs("APP").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "row_count", "comment"}).
			AddRow("CONFIG", 50, ""))

	// Columns
	mock.ExpectQuery(`(?s)SELECT c\.column_name, c\.data_type.*FROM all_tab_columns c`).
		WithArgs("APP", "CONFIG").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "nullable", "data_default", "comments", "column_id"}).
			AddRow("KEY", "VARCHAR2", "N", "", "", 1).
			AddRow("VAL", "VARCHAR2", "Y", "", "", 2))

	// Constraints (none)
	mock.ExpectQuery(`(?s)constraint_type IN\s*\(\s*'P',\s*'U'\s*\)`).
		WithArgs("APP", "CONFIG").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "constraint_type", "column_name"}))

	// Indexes (none)
	mock.ExpectQuery(`(?s)FROM all_ind_columns ic\s+JOIN all_indexes i`).
		WithArgs("APP", "CONFIG").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "column_name", "uniqueness"}))

	// Foreign keys (none)
	mock.ExpectQuery(`(?s)constraint_type = 'R'\s+ORDER BY a\.constraint_name`).
		WithArgs("APP", "CONFIG").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "r_owner", "r_constraint_name", "position"}))

	// Sample row
	mock.ExpectQuery(`(?s)SELECT \* FROM "APP"\."CONFIG" FETCH FIRST 1 ROWS ONLY`).
		WillReturnRows(sqlmock.NewRows([]string{"KEY", "VAL"}).
			AddRow("timeout", "30"))

	d, _ := dsn.ParseDSN("oracle://user:pass@host:1521/APP?label=app-test")
	ctx := context.Background()

	inst, err := collectOracleSchema(ctx, db, d)
	if err != nil {
		t.Fatalf("collectOracleSchema error: %v", err)
	}

	if len(inst.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(inst.Databases))
	}
	if inst.Databases[0].Name != "APP" {
		t.Errorf("Database name = %q, want %q", inst.Databases[0].Name, "APP")
	}
	if len(inst.Databases[0].Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(inst.Databases[0].Tables))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectOracleSchema_SystemSchemaSkip(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	// Only system schemas returned — should be filtered out
	mock.ExpectQuery("SELECT DISTINCT owner FROM all_tables ORDER BY owner").
		WillReturnRows(sqlmock.NewRows([]string{"owner"}).
			AddRow("SYS").
			AddRow("SYSTEM").
			AddRow("XDB"))

	d, _ := dsn.ParseDSN("oracle://user:pass@host?label=test")
	ctx := context.Background()

	inst, err := collectOracleSchema(ctx, db, d)
	if err != nil {
		t.Fatalf("collectOracleSchema error: %v", err)
	}

	if len(inst.Databases) != 0 {
		t.Errorf("Databases len = %d, want 0 (all system schemas filtered)", len(inst.Databases))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectOracleSchema_OwnerQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT DISTINCT owner FROM all_tables ORDER BY owner").
		WillReturnError(fmt.Errorf("connection refused"))

	d, _ := dsn.ParseDSN("oracle://user:pass@host?label=test")
	ctx := context.Background()

	_, err = collectOracleSchema(ctx, db, d)
	if err == nil {
		t.Fatal("expected error for failed owner query, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectOracleSchema_NoTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT DISTINCT owner FROM all_tables ORDER BY owner").
		WillReturnRows(sqlmock.NewRows([]string{"owner"}).AddRow("HR"))

	// HR has no tables
	mock.ExpectQuery(`(?s)SELECT t\.table_name, COALESCE\(t\.num_rows.*FROM all_tables t`).
		WithArgs("HR").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "row_count", "comment"}))

	d, _ := dsn.ParseDSN("oracle://user:pass@host?label=test")
	ctx := context.Background()

	inst, err := collectOracleSchema(ctx, db, d)
	if err != nil {
		t.Fatalf("collectOracleSchema error: %v", err)
	}

	if len(inst.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(inst.Databases))
	}
	if len(inst.Databases[0].Tables) != 0 {
		t.Errorf("Tables len = %d, want 0", len(inst.Databases[0].Tables))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

func TestCollectOracleSchema_ForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT DISTINCT owner FROM all_tables ORDER BY owner").
		WillReturnRows(sqlmock.NewRows([]string{"owner"}).AddRow("HR"))

	// Tables
	mock.ExpectQuery(`(?s)SELECT t\.table_name, COALESCE\(t\.num_rows.*FROM all_tables t`).
		WithArgs("HR").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "row_count", "comment"}).
			AddRow("EMP", 10, ""))

	// Columns
	mock.ExpectQuery(`(?s)SELECT c\.column_name, c\.data_type.*FROM all_tab_columns c`).
		WithArgs("HR", "EMP").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "nullable", "data_default", "comments", "column_id"}).
			AddRow("ID", "NUMBER", "N", "", "", 1).
			AddRow("DEPT_ID", "NUMBER", "Y", "", "", 2).
			AddRow("NAME", "VARCHAR2", "Y", "", "", 3))

	// Constraints (PK)
	mock.ExpectQuery(`(?s)constraint_type IN\s*\(\s*'P',\s*'U'\s*\)`).
		WithArgs("HR", "EMP").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "constraint_type", "column_name"}).
			AddRow("PK_EMP", "P", "ID"))

	// Indexes
	mock.ExpectQuery(`(?s)FROM all_ind_columns ic\s+JOIN all_indexes i`).
		WithArgs("HR", "EMP").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "column_name", "uniqueness"}).
			AddRow("PK_EMP", "ID", "UNIQUE"))

	// Foreign keys
	mock.ExpectQuery(`(?s)constraint_type = 'R'\s+ORDER BY a\.constraint_name`).
		WithArgs("HR", "EMP").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "r_owner", "r_constraint_name", "position"}).
			AddRow("FK_DEPT", "DEPT_ID", "HR", "PK_DEPT", 1))

	// FK resolution: get referenced table name
	mock.ExpectQuery(`(?s)COALESCE\(delete_rule, 'NO ACTION'\)`).
		WithArgs("HR", "PK_DEPT").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "delete_rule"}).
			AddRow("DEPT", "NO ACTION"))

	// FK resolution: get referenced columns
	mock.ExpectQuery(`(?s)SELECT column_name FROM all_cons_columns`).
		WithArgs("HR", "PK_DEPT").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).
			AddRow("ID"))

	// Sample row
	mock.ExpectQuery(`(?s)SELECT \* FROM "HR"\."EMP" FETCH FIRST 1 ROWS ONLY`).
		WillReturnRows(sqlmock.NewRows([]string{"ID", "DEPT_ID", "NAME"}).
			AddRow(1, 10, "Alice"))

	d, _ := dsn.ParseDSN("oracle://user:pass@host?label=test")
	ctx := context.Background()

	inst, err := collectOracleSchema(ctx, db, d)
	if err != nil {
		t.Fatalf("collectOracleSchema error: %v", err)
	}

	if len(inst.Databases) != 1 {
		t.Fatalf("Databases len = %d, want 1", len(inst.Databases))
	}
	if len(inst.Databases[0].Tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(inst.Databases[0].Tables))
	}
	tbl := inst.Databases[0].Tables[0]

	if len(tbl.ForeignKeys) != 1 {
		t.Fatalf("ForeignKeys len = %d, want 1", len(tbl.ForeignKeys))
	}
	fk := tbl.ForeignKeys[0]
	if fk.Name != "FK_DEPT" {
		t.Errorf("FK name = %q, want %q", fk.Name, "FK_DEPT")
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "DEPT_ID" {
		t.Errorf("FK columns = %v, want [DEPT_ID]", fk.Columns)
	}
	if fk.RefTable != "DEPT" {
		t.Errorf("FK RefTable = %q, want %q", fk.RefTable, "DEPT")
	}
	if fk.RefDB != "HR" {
		t.Errorf("FK RefDB = %q, want %q", fk.RefDB, "HR")
	}
	if len(fk.RefColumns) != 1 || fk.RefColumns[0] != "ID" {
		t.Errorf("FK RefColumns = %v, want [ID]", fk.RefColumns)
	}
	if fk.OnDelete != "NO ACTION" {
		t.Errorf("FK OnDelete = %q, want %q", fk.OnDelete, "NO ACTION")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
