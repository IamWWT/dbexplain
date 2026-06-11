package policy

import (
	"os"
	"testing"

	"github.com/IamWWT/dbexplain/internal/query"
)

func TestLoad_EmptyEnv(t *testing.T) {
	cfg := Load("")
	if len(cfg.DenyTables) != 0 || len(cfg.DenyColumns) != 0 || len(cfg.DenyStatements) != 0 {
		t.Error("expected empty config when no env vars set")
	}
}

func TestLoad_GlobalOnly(t *testing.T) {
	os.Setenv("DENY_TABLES", "users,orders")
	os.Setenv("DENY_COLUMNS", "users.password")
	os.Setenv("DENY_STATEMENTS", "DROP TABLE")
	defer os.Unsetenv("DENY_TABLES")
	defer os.Unsetenv("DENY_COLUMNS")
	defer os.Unsetenv("DENY_STATEMENTS")

	cfg := Load("")
	if len(cfg.DenyTables) != 2 || cfg.DenyTables[0] != "users" || cfg.DenyTables[1] != "orders" {
		t.Errorf("DENY_TABLES parse failed: %v", cfg.DenyTables)
	}
	if len(cfg.DenyColumns) != 1 || cfg.DenyColumns[0] != "users.password" {
		t.Errorf("DENY_COLUMNS parse failed: %v", cfg.DenyColumns)
	}
	if len(cfg.DenyStatements) != 1 || cfg.DenyStatements[0] != "DROP TABLE" {
		t.Errorf("DENY_STATEMENTS parse failed: %v", cfg.DenyStatements)
	}
}

func TestLoad_PerDSN(t *testing.T) {
	os.Setenv("DENY_TABLES", "global_table")
	os.Setenv("DB1_DENY_TABLES", "secret_table")
	defer os.Unsetenv("DENY_TABLES")
	defer os.Unsetenv("DB1_DENY_TABLES")

	cfg := Load("DB1")
	found := false
	for _, t := range cfg.DenyTables {
		if t == "secret_table" {
			found = true
		}
	}
	if !found {
		t.Errorf("per-DSN DENY_TABLES not merged: %v", cfg.DenyTables)
	}
	if len(cfg.DenyTables) != 2 {
		t.Errorf("expected 2 tables (global + per-DSN), got %d: %v", len(cfg.DenyTables), cfg.DenyTables)
	}
}

func TestCheckSQL_StatementLevel(t *testing.T) {
	cfg := &Config{DenyStatements: []string{"DROP TABLE", "ALTER TABLE"}}

	tests := []struct {
		sql     string
		wantErr bool
	}{
		{"SELECT * FROM users", false},
		{"DROP TABLE users", true},
		{"select * from users; drop table users", true},
		{"ALTER TABLE users ADD COLUMN x INT", true},
		{"SHOW TABLES", false},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			err := cfg.CheckSQL(tt.sql)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestCheckSQL_TableLevel(t *testing.T) {
	cfg := &Config{DenyTables: []string{"user_credentials", "payment_log"}}

	tests := []struct {
		sql     string
		wantErr bool
		want    string // expected table name in error
	}{
		{"SELECT * FROM users", false, ""},
		{"SELECT * FROM user_credentials", true, "user_credentials"},
		{"SELECT * FROM payment_log WHERE id=1", true, "payment_log"},
		{"SELECT u.name FROM user_credentials u", true, "user_credentials"},
		{"SELECT * FROM users JOIN payment_log ON users.id=payment_log.user_id", true, "payment_log"},
		{"SHOW TABLES", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			err := cfg.CheckSQL(tt.sql)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				e, ok := err.(*ErrDenied)
				if !ok {
					t.Fatalf("expected *ErrDenied, got %T", err)
				}
				if e.Level != "table" {
					t.Errorf("expected level=table, got %s", e.Level)
				}
				if e.Target != tt.want {
					t.Errorf("expected target=%q, got %q", tt.want, e.Target)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestCheckSQL_ColumnLevel(t *testing.T) {
	cfg := &Config{DenyColumns: []string{"users.password_hash", "orders.card_number"}}

	tests := []struct {
		sql     string
		wantErr bool
	}{
		{"SELECT id, name FROM users", false},
		{"SELECT password_hash FROM users", false}, // no table. prefix = no match
		{"SELECT users.password_hash FROM users", true},
		{"SELECT users.name, users.password_hash FROM users", true},
		{"SELECT orders.card_number, orders.total FROM orders", true},
		{"SELECT users.password_hash FROM users WHERE users.id=1", true},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			err := cfg.CheckSQL(tt.sql)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestCheckSQL_CaseInsensitive(t *testing.T) {
	cfg := &Config{DenyTables: []string{"SENSITIVE_DATA"}}
	err := cfg.CheckSQL("SELECT * FROM sensitive_data")
	if err == nil {
		t.Error("expected case-insensitive match")
	}
}

func TestCheckNative_StatementLevel(t *testing.T) {
	cfg := &Config{DenyStatements: []string{"FLUSHALL", "CONFIG"}}

	tests := []struct {
		query   string
		kind    string
		wantErr bool
	}{
		{"GET user:1001", "redis", false},
		{"FLUSHALL", "redis", true},
		{"CONFIG SET requirepass newpass", "redis", true},
		{"PING", "redis", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			err := cfg.CheckNative(tt.query, tt.kind)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestCheckNative_MongoTableLevel(t *testing.T) {
	cfg := &Config{DenyTables: []string{"user_credentials"}}

	tests := []struct {
		query   string
		wantErr bool
	}{
		{`{"find":"users","filter":{}}`, false},
		{`{"find":"user_credentials","filter":{"age":{"$gt":18}}}`, true},
		{`{"aggregate":"user_credentials","pipeline":[]}`, true},
		{`{"count":"user_credentials"}`, true},
		{`{"find":"orders","filter":{"status":"active"}}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			err := cfg.CheckNative(tt.query, "mongodb")
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestCheckNative_QdrantTableLevel(t *testing.T) {
	cfg := &Config{DenyTables: []string{"internal_docs"}}

	tests := []struct {
		query   string
		wantErr bool
	}{
		{`{"scroll":"documents","limit":100}`, false},
		{`{"scroll":"internal_docs","limit":10}`, true},
		{`{"count":"internal_docs"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			err := cfg.CheckNative(tt.query, "qdrant")
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestCheckNative_RedisSkipTableLevel(t *testing.T) {
	cfg := &Config{DenyTables: []string{"users"}}
	// Redis doesn't extract table names, so table-level deny should not apply
	err := cfg.CheckNative("GET users:1001", "redis")
	if err != nil {
		t.Errorf("Redis should skip table-level check, got: %v", err)
	}
}

func TestExtractTableNames(t *testing.T) {
	tests := []struct {
		sql    string
		expect []string
	}{
		{"SELECT * FROM users", []string{"users"}},
		{"SELECT u.name FROM users u JOIN orders o ON u.id=o.user_id", []string{"users", "orders"}},
		{"SELECT * FROM user_credentials WHERE id=1", []string{"user_credentials"}},
		{"SHOW TABLES", nil},
		{"PRAGMA table_info(users)", nil},
		// Comment-based bypass: -- comment with newline should still extract table
		// AST-based extraction returns schema-qualified name + individual parts
		{"SELECT * FROM testdb.-- comment\niplist", []string{"testdb.iplist", "iplist", "testdb"}},
		{"SELECT * FROM iplist -- comment\nWHERE id=1", []string{"iplist"}},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := extractTableNames(tt.sql)
			if len(got) != len(tt.expect) {
				t.Errorf("got %v, want %v", got, tt.expect)
				return
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("got %v, want %v", got, tt.expect)
				}
			}
		})
	}
}

func TestExtractJSONCollectionNames(t *testing.T) {
	tests := []struct {
		query  string
		expect []string
	}{
		{`{"find":"users","filter":{}}`, []string{"users"}},
		{`{"aggregate":"orders","pipeline":[]}`, []string{"orders"}},
		{`{"scroll":"documents","limit":100}`, []string{"documents"}},
		{`{"count":"items"}`, []string{"items"}},
		{`GET users:1001`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := extractJSONCollectionNames(tt.query)
			if len(got) != len(tt.expect) {
				t.Errorf("got %v, want %v", got, tt.expect)
				return
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("got %v, want %v", got, tt.expect)
				}
			}
		})
	}
}

func TestCheckNative_RedisKeyLevel(t *testing.T) {
	cfg := &Config{DenyTables: []string{"CONVERSATION:*", "secret_key", "user:admin:*"}}

	tests := []struct {
		query   string
		wantErr bool
	}{
		{"GET CONVERSATION:abc123", true},           // wildcard match
		{"GET secret_key", true},                      // exact match
		{"GET user:admin:1001", true},                 // wildcard match
		{"GET user:1001", false},                      // no match
		{"HGETALL CONVERSATION:xyz", true},            // wildcard
		{"TYPE CONVERSATION:test", true},              // wildcard
		{"PING", false},                                // no key command
		{"SCAN 0 MATCH CONVERSATION:* COUNT 10", false}, // SCAN has no key
		{"LRANGE mylist 0 -1", false},                 // not denied
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			err := cfg.CheckNative(tt.query, "redis")
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestExtractRedisKeys(t *testing.T) {
	tests := []struct {
		query  string
		expect []string
	}{
		{"GET user:1001", []string{"user:1001"}},
		{"HGETALL CONVERSATION:abc", []string{"CONVERSATION:abc"}},
		{"SCAN 0 MATCH user:*", nil},
		{"PING", nil},
		{"TYPE mykey", []string{"mykey"}},
		{"LRANGE mylist 0 -1", []string{"mylist"}},
		{"", nil},
		{"GET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := extractRedisKeys(tt.query)
			if len(got) != len(tt.expect) {
				t.Errorf("got %v, want %v", got, tt.expect)
				return
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("got %v, want %v", got, tt.expect)
				}
			}
		})
	}
}

func TestErrDenied_Format(t *testing.T) {
	e := &ErrDenied{Level: "table", Target: "users", SQL: "SELECT * FROM users"}
	msg := e.Error()
	if msg != `ACCESS_DENIED: table "users" is not allowed for query` {
		t.Errorf("unexpected error message: %s", msg)
	}
}

func TestNilConfig(t *testing.T) {
	var cfg *Config
	if err := cfg.CheckSQL("SELECT 1"); err != nil {
		t.Error("nil config should not error")
	}
	if err := cfg.CheckNative("PING", "redis"); err != nil {
		t.Error("nil config should not error")
	}
}

// --- MASK_COLUMNS tests ---

func TestLoadMask_Empty(t *testing.T) {
	m := loadMask("MASK_COLUMNS_NONEXISTENT")
	if m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}

func TestLoadMask_Normal(t *testing.T) {
	os.Setenv("TEST_MASK", "password_hash=***,email=REDACTED")
	defer os.Unsetenv("TEST_MASK")

	m := loadMask("TEST_MASK")
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m["password_hash"] != "***" {
		t.Errorf("password_hash: expected ***, got %q", m["password_hash"])
	}
	if m["email"] != "REDACTED" {
		t.Errorf("email: expected REDACTED, got %q", m["email"])
	}
}

func TestLoadMask_WithEqualInReplacement(t *testing.T) {
	os.Setenv("TEST_MASK", "token=Bearer ****")
	defer os.Unsetenv("TEST_MASK")

	m := loadMask("TEST_MASK")
	if len(m) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m))
	}
	if m["token"] != "Bearer ****" {
		t.Errorf("expected 'Bearer ****', got %q", m["token"])
	}
}

func TestLoadMask_MalformedEntry(t *testing.T) {
	os.Setenv("TEST_MASK", "justacolumn,email=REDACTED")
	defer os.Unsetenv("TEST_MASK")

	m := loadMask("TEST_MASK")
	if len(m) != 1 {
		t.Fatalf("expected 1 entry (skip malformed), got %d", len(m))
	}
	if m["email"] != "REDACTED" {
		t.Errorf("email: expected REDACTED, got %q", m["email"])
	}
}

func TestMatchColumn_Exact(t *testing.T) {
	if !matchColumn("password_hash", "password_hash") {
		t.Error("exact match should succeed")
	}
	if matchColumn("password_hash", "email") {
		t.Error("different column should not match")
	}
}

func TestMatchColumn_CaseInsensitive(t *testing.T) {
	if !matchColumn("PASSWORD_HASH", "password_hash") {
		t.Error("case-insensitive match should succeed")
	}
}

func TestMatchColumn_TablePrefix(t *testing.T) {
	if !matchColumn("password_hash", "users.password_hash") {
		t.Error("table.pattern should match bare column name")
	}
	if !matchColumn("email", "users.email") {
		t.Error("table.pattern should match bare column name")
	}
}

func TestMatchColumn_Glob(t *testing.T) {
	tests := []struct {
		col     string
		pattern string
		want    bool
	}{
		{"password_hash", "pass*", true},
		{"pass_token", "pass*", true},
		{"email", "pass*", false},
		{"credit_card_1", "credit_card_?", true},
		{"credit_card_12", "credit_card_?", false},
		{"card_number", "card_*", true},
	}
	for _, tt := range tests {
		t.Run(tt.col+"/"+tt.pattern, func(t *testing.T) {
			got := matchColumn(tt.col, tt.pattern)
			if got != tt.want {
				t.Errorf("matchColumn(%q, %q) = %v, want %v", tt.col, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestApplyMask_Basic(t *testing.T) {
	cfg := &Config{MaskColumns: map[string]string{"password_hash": "***"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INT"},
			{Name: "password_hash", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("secret123")},
			{strPtr("2"), strPtr("secret456")},
		},
	}

	cfg.ApplyMask(result)

	if *result.Rows[0][0] != "1" {
		t.Error("non-masked column should be unchanged")
	}
	if *result.Rows[0][1] != "***" {
		t.Errorf("masked column: expected '***', got %q", *result.Rows[0][1])
	}
	if *result.Rows[1][1] != "***" {
		t.Errorf("masked column row2: expected '***', got %q", *result.Rows[1][1])
	}
}

func TestApplyMask_NullPreserved(t *testing.T) {
	cfg := &Config{MaskColumns: map[string]string{"secret": "HIDDEN"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INT"},
			{Name: "secret", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("1"), nil}, // NULL in secret column
			{strPtr("2"), strPtr("s3kr1t")},
		},
	}

	cfg.ApplyMask(result)

	if result.Rows[0][1] != nil {
		t.Error("NULL cell should remain nil after masking")
	}
	if *result.Rows[1][1] != "HIDDEN" {
		t.Errorf("non-null cell: expected 'HIDDEN', got %q", *result.Rows[1][1])
	}
}

func TestApplyMask_NoMatch(t *testing.T) {
	cfg := &Config{MaskColumns: map[string]string{"other_col": "***"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "name", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("Alice")},
		},
	}

	cfg.ApplyMask(result)

	if *result.Rows[0][0] != "Alice" {
		t.Error("non-matching column should be unchanged")
	}
}

func TestApplyMask_MultipleColumns(t *testing.T) {
	cfg := &Config{MaskColumns: map[string]string{
		"password": "***",
		"email":    "HIDDEN",
	}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INT"},
			{Name: "password", Type: "VARCHAR"},
			{Name: "email", Type: "VARCHAR"},
			{Name: "name", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("p@ss"), strPtr("a@b.com"), strPtr("Alice")},
		},
	}

	cfg.ApplyMask(result)

	if *result.Rows[0][1] != "***" {
		t.Error("password should be masked")
	}
	if *result.Rows[0][2] != "HIDDEN" {
		t.Error("email should be masked")
	}
	if *result.Rows[0][3] != "Alice" {
		t.Error("name should be unchanged")
	}
}

func TestApplyMask_GlobPattern(t *testing.T) {
	cfg := &Config{MaskColumns: map[string]string{"pass*": "***"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "password", Type: "VARCHAR"},
			{Name: "pass_token", Type: "VARCHAR"},
			{Name: "name", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("secret1"), strPtr("token123"), strPtr("Alice")},
		},
	}

	cfg.ApplyMask(result)

	if *result.Rows[0][0] != "***" {
		t.Error("password should be masked via glob")
	}
	if *result.Rows[0][1] != "***" {
		t.Error("pass_token should be masked via glob")
	}
	if *result.Rows[0][2] != "Alice" {
		t.Error("name should be unchanged")
	}
}

func TestApplyMask_TablePrefixedPattern(t *testing.T) {
	cfg := &Config{MaskColumns: map[string]string{"users.password_hash": "***"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "password_hash", Type: "VARCHAR"},
		},
		Rows: [][]*string{
			{strPtr("s3kr1t")},
		},
	}

	cfg.ApplyMask(result)

	if *result.Rows[0][0] != "***" {
		t.Errorf("table-prefixed pattern should match bare column: got %q", *result.Rows[0][0])
	}
}

func TestApplyMask_NilConfig(t *testing.T) {
	var cfg *Config
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{{Name: "c", Type: "T"}},
		Rows:    [][]*string{{strPtr("val")}},
	}
	cfg.ApplyMask(result) // should not panic
	if *result.Rows[0][0] != "val" {
		t.Error("nil config should not change result")
	}
}

func TestApplyMask_EmptyMask(t *testing.T) {
	cfg := &Config{}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{{Name: "c", Type: "T"}},
		Rows:    [][]*string{{strPtr("val")}},
	}
	cfg.ApplyMask(result)
	if *result.Rows[0][0] != "val" {
		t.Error("empty mask should not change result")
	}
}

func TestApplyMask_NativeQueryResult(t *testing.T) {
	// Simulate a MongoDB result with a sensitive field
	cfg := &Config{MaskColumns: map[string]string{"ssn": "***"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "_id", Type: "string"},
			{Name: "ssn", Type: "string"},
			{Name: "name", Type: "string"},
		},
		Rows: [][]*string{
			{strPtr("abc123"), strPtr("123-45-6789"), strPtr("Alice")},
		},
	}

	cfg.ApplyMask(result)

	if *result.Rows[0][1] != "***" {
		t.Errorf("MongoDB ssn should be masked: got %q", *result.Rows[0][1])
	}
	if *result.Rows[0][2] != "Alice" {
		t.Error("MongoDB name should be unchanged")
	}
}

func TestLoadMask_PerDSN(t *testing.T) {
	os.Setenv("MASK_COLUMNS", "password=***")
	os.Setenv("DB1_MASK_COLUMNS", "email=REDACTED")
	defer os.Unsetenv("MASK_COLUMNS")
	defer os.Unsetenv("DB1_MASK_COLUMNS")

	cfg := Load("DB1")
	if len(cfg.MaskColumns) != 2 {
		t.Fatalf("expected 2 masks (global + per-DSN), got %d", len(cfg.MaskColumns))
	}
	if cfg.MaskColumns["password"] != "***" {
		t.Errorf("global mask: expected ***, got %q", cfg.MaskColumns["password"])
	}
	if cfg.MaskColumns["email"] != "REDACTED" {
		t.Errorf("per-DSN mask: expected REDACTED, got %q", cfg.MaskColumns["email"])
	}
}

// strPtr is a helper to create *string literals.
func strPtr(s string) *string {
	return &s
}

// ── PromQL extraction tests ──

func TestExtractPromQLMetricName_Basic(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"up", "up"},
		{"up{job=\"node\"}", "up"},
		{"node_cpu_seconds_total", "node_cpu_seconds_total"},
		{"count(up)", "up"},
		{"rate(node_cpu_seconds_total[5m])", "node_cpu_seconds_total"},
		{"sum by(job) (up)", "up"},
		{"up > 0", "up"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractPromQLMetricName(tt.query)
		if got != tt.want {
			t.Errorf("extractPromQLMetricName(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestCheckNative_Prometheus_DenyTables(t *testing.T) {
	cfg := &Config{
		DenyTables: []string{"up", "sensitive_metric"},
	}

	// Denied
	if err := cfg.CheckNative("up", "prometheus"); err == nil {
		t.Error("expected error for denied metric 'up'")
	}
	if err := cfg.CheckNative("up{job=\"node\"}", "prometheus"); err == nil {
		t.Error("expected error for denied metric 'up' with labels")
	}

	// Allowed
	if err := cfg.CheckNative("node_cpu_seconds_total", "prometheus"); err != nil {
		t.Errorf("unexpected error for allowed metric: %v", err)
	}
}

func TestCheckNative_Prometheus_DenyStatements(t *testing.T) {
	cfg := &Config{
		DenyStatements: []string{"DROP"},
	}

	// Allowed (PromQL doesn't have DROP)
	if err := cfg.CheckNative("up", "prometheus"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── StripDeniedColumns tests ──

func TestStripDeniedColumns_Basic(t *testing.T) {
	cfg := &Config{DenyColumns: []string{"password", "ssn"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INTEGER"},
			{Name: "password", Type: "TEXT"},
			{Name: "name", Type: "TEXT"},
			{Name: "ssn", Type: "TEXT"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("secret123"), strPtr("Alice"), strPtr("123-45-6789")},
			{strPtr("2"), strPtr("pass456"), strPtr("Bob"), strPtr("987-65-4321")},
		},
		RowCount: 2,
	}

	cfg.StripDeniedColumns(result)

	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns after strip, got %d", len(result.Columns))
	}
	if result.Columns[0].Name != "id" {
		t.Errorf("expected first column 'id', got %q", result.Columns[0].Name)
	}
	if result.Columns[1].Name != "name" {
		t.Errorf("expected second column 'name', got %q", result.Columns[1].Name)
	}
	if len(result.Rows[0]) != 2 {
		t.Fatalf("expected 2 values per row after strip, got %d", len(result.Rows[0]))
	}
	if *result.Rows[0][0] != "1" {
		t.Errorf("expected row[0][0]='1', got %q", *result.Rows[0][0])
	}
	if *result.Rows[1][1] != "Bob" {
		t.Errorf("expected row[1][1]='Bob', got %q", *result.Rows[1][1])
	}
}

func TestStripDeniedColumns_TablePrefixed(t *testing.T) {
	// DENY_COLUMNS=users.password should match bare column "password"
	cfg := &Config{DenyColumns: []string{"users.password"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INTEGER"},
			{Name: "password", Type: "TEXT"},
			{Name: "name", Type: "TEXT"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("secret"), strPtr("Alice")},
		},
	}

	cfg.StripDeniedColumns(result)

	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0].Name != "id" {
		t.Errorf("expected 'id', got %q", result.Columns[0].Name)
	}
	if result.Columns[1].Name != "name" {
		t.Errorf("expected 'name', got %q", result.Columns[1].Name)
	}
}

func TestStripDeniedColumns_NoMatch(t *testing.T) {
	cfg := &Config{DenyColumns: []string{"other_col"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INTEGER"},
			{Name: "name", Type: "TEXT"},
		},
		Rows: [][]*string{
			{strPtr("1"), strPtr("Alice")},
		},
		RowCount: 1,
	}

	cfg.StripDeniedColumns(result)

	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns unchanged, got %d", len(result.Columns))
	}
	if *result.Rows[0][0] != "1" {
		t.Errorf("expected row[0][0]='1', got %q", *result.Rows[0][0])
	}
}

func TestStripDeniedColumns_NilConfig(t *testing.T) {
	var cfg *Config
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{{Name: "c", Type: "T"}},
		Rows:    [][]*string{{strPtr("val")}},
	}
	cfg.StripDeniedColumns(result) // should not panic
	if *result.Rows[0][0] != "val" {
		t.Error("nil config should not change result")
	}
}

func TestStripDeniedColumns_EmptyDenyColumns(t *testing.T) {
	cfg := &Config{}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{{Name: "c", Type: "T"}},
		Rows:    [][]*string{{strPtr("val")}},
	}
	cfg.StripDeniedColumns(result)
	if *result.Rows[0][0] != "val" {
		t.Error("empty deny columns should not change result")
	}
}

func TestStripDeniedColumns_NullPreserved(t *testing.T) {
	cfg := &Config{DenyColumns: []string{"secret"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "id", Type: "INTEGER"},
			{Name: "secret", Type: "TEXT"},
			{Name: "name", Type: "TEXT"},
		},
		Rows: [][]*string{
			{strPtr("1"), nil, strPtr("Alice")},
		},
	}
	cfg.StripDeniedColumns(result)
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0].Name != "id" {
		t.Errorf("expected 'id', got %q", result.Columns[0].Name)
	}
	if result.Columns[1].Name != "name" {
		t.Errorf("expected 'name', got %q", result.Columns[1].Name)
	}
	if *result.Rows[0][1] != "Alice" {
		t.Errorf("expected row[0][1]='Alice', got %q", *result.Rows[0][1])
	}
}

func TestStripDeniedColumns_NativeQueryResult(t *testing.T) {
	// Simulate MongoDB result with sensitive field
	cfg := &Config{DenyColumns: []string{"ssn"}}
	result := &query.QueryResult{
		Columns: []query.ColumnInfo{
			{Name: "_id", Type: "string"},
			{Name: "ssn", Type: "string"},
			{Name: "name", Type: "string"},
		},
		Rows: [][]*string{
			{strPtr("abc123"), strPtr("123-45-6789"), strPtr("Alice")},
		},
	}
	cfg.StripDeniedColumns(result)
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0].Name != "_id" {
		t.Errorf("expected '_id', got %q", result.Columns[0].Name)
	}
	if result.Columns[1].Name != "name" {
		t.Errorf("expected 'name', got %q", result.Columns[1].Name)
	}
}
