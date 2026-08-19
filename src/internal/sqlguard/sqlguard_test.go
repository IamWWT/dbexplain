package sqlguard

import (
	"strings"
	"testing"
)

// ─── Validate ────────────────────────────────────────────────────────────────

func TestValidate_AllowedReadOps(t *testing.T) {
	tests := []struct {
		sql string
	}{
		{"SELECT 1"},
		{"SELECT * FROM users"},
		{"select id, name from orders"},
		{"EXPLAIN SELECT * FROM t"},
		{"explain analyze select 1"},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte"},
		{"SHOW TABLES"},
		{"show databases"},
		{"DESCRIBE users"},
		{"DESC users"},
		{"PRAGMA table_info('users')"},
		{"CHECK TABLE users"},
		{"SELECT * FROM t INTO @var"}, // MySQL variable assignment (read-only)
		{"SELECT id, name INTO @a, @b FROM t"},
		// Optimizer hint SQL — /*+ */ should pass through
		{"SELECT /*+ index(t idx) */ * FROM t"},
		{"SELECT /*+ FULL(t) */ * FROM t WHERE id=1"},
		{"SELECT /*+ hashjoin(t1 t2) */ * FROM t1 JOIN t2 ON t1.id = t2.id"},
		{"SELECT /*+ leading(t1 t2) */ * FROM t1, t2 WHERE t1.id = t2.id"},
		{"EXPLAIN SELECT /*+ hint */ * FROM t"},
		// EXPLAIN dialect forms — all read-only, must keep passing
		{"EXPLAIN (ANALYZE) SELECT 1"},
		{"EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) SELECT * FROM t"},
		{"EXPLAIN (ANALYZE, FORMAT TEXT) SELECT * FROM t"},
		{"EXPLAIN FORMAT=JSON SELECT * FROM t"},
		{"EXPLAIN ANALYZE SELECT * FROM t"},
		{"EXPLAIN QUERY PLAN SELECT * FROM t"},
		{"EXPLAIN PLAN FOR SELECT * FROM t"},
		{"EXPLAIN PLAN SELECT * FROM t"},
		{"EXPLAIN VERBOSE SELECT * FROM t"},
		{"EXPLAIN WITH cte AS (SELECT 1) SELECT * FROM cte"},
		{"  EXPLAIN   SELECT * FROM t"},
		{"EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) WITH cte AS (SELECT 1) SELECT * FROM cte"},
		// Comment / optimizer-hint forms between EXPLAIN and the inner statement
		{"EXPLAIN /*+ hint */ SELECT 1"},
		{"EXPLAIN /*+ qb_name(q1) */ SELECT * FROM t"},
		{"EXPLAIN (ANALYZE) /* c */ SELECT 1"},
		{"EXPLAIN -- comment\nSELECT 1"},
		// String literal containing a write verb must NOT be treated as a write
		{"EXPLAIN SELECT * FROM t WHERE note = 'INSERT'"},
		{"EXPLAIN SELECT * FROM t WHERE note = '-- not a comment'"},
		// ANALYZE + WITH inner
		{"EXPLAIN ANALYZE WITH cte AS (SELECT 1) SELECT * FROM cte"},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			if err := Validate(tt.sql); err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestValidate_RejectedWriteOps(t *testing.T) {
	tests := []struct {
		sql    string
		reason string
	}{
		{"INSERT INTO t VALUES(1)", "INSERT"},
		{"UPDATE t SET x=1", "UPDATE"},
		{"DELETE FROM t", "DELETE"},
		{"DROP TABLE t", "DROP"},
		{"DROP DATABASE foo", "DROP"},
		{"ALTER TABLE t ADD COLUMN x INT", "ALTER"},
		{"CREATE TABLE t (id INT)", "CREATE"},
		{"TRUNCATE TABLE t", "TRUNCATE"},
		{"TRUNCATE t", "TRUNCATE"},
		{"RENAME TABLE t TO t2", "RENAME"},
		{"REPLACE INTO t VALUES(1)", "REPLACE"},
		{"GRANT SELECT ON t TO user", "GRANT"},
		{"REVOKE SELECT ON t FROM user", "REVOKE"},
		{"KILL QUERY 123", "KILL"},
		{"SHUTDOWN", "SHUTDOWN"},
		{"FLUSH TABLES", "FLUSH"},
		{"SET GLOBAL var=1", "SET"},
		{"RESET QUERY CACHE", "RESET"},
		{"INSTALL PLUGSON", "INSTALL"},
		{"UNINSTALL PLUGSON", "UNINSTALL"},
		{"CALL procedure()", "CALL"},
		{"PURGE BINARY LOGS TO 'x'", "PURGE"},
		{"MERGE INTO t USING s ON t.id=s.id WHEN MATCHED THEN UPDATE SET x=1", "MERGE"},
		{"UPSERT INTO t VALUES(1)", "UPSERT"},
		{"LOAD DATA INFILE 'x.csv' INTO TABLE t", "LOAD"},
		{"IMPORT TABLE t FROM 'x.csv'", "IMPORT"},
		{"EXPORT TABLE t TO 'x.csv'", "EXPORT"},
		{"ANALYZE", "ANALYZE"},
		{"ANALYZE TABLE t", "ANALYZE"},
		{"REINDEX", "REINDEX"},
		// CTE with write operations
		{"WITH del AS (DELETE FROM orders WHERE id=1 RETURNING id) SELECT * FROM del", "WITH CTE"},
		{"WITH ins AS (INSERT INTO t VALUES(1) RETURNING id) SELECT * FROM ins", "WITH CTE"},
		{"WITH upd AS (UPDATE users SET status='banned' RETURNING id) SELECT * FROM upd", "WITH CTE"},
		{"WITH a AS (INSERT INTO t VALUES(1)), b AS (SELECT 1) SELECT * FROM b", "WITH CTE"},
		// CTE + main query write (WITH x AS (...) INSERT INTO y ...)
		{"WITH x AS (SELECT 1) INSERT INTO y VALUES (1)", "WITH CTE"},
		{"WITH x AS (SELECT 1), y AS (SELECT 2) DELETE FROM z", "WITH CTE"},
		// SELECT INTO (PostgreSQL DDL write)
		{"SELECT * INTO backup_users FROM users", "SELECT INTO"},
		{"SELECT id, name INTO new_table FROM old_table WHERE created > NOW()-7", "SELECT INTO"},
		// EXPLAIN-wrapped write statements — must be rejected via recursive validation
		{"EXPLAIN INSERT INTO t VALUES(1)", "INSERT"},
		{"EXPLAIN (ANALYZE, BUFFERS) INSERT INTO t VALUES(1)", "INSERT"},
		{"EXPLAIN (ANALYZE) INSERT INTO t VALUES (1)", "INSERT"},
		{"EXPLAIN ANALYZE UPDATE t SET a=1", "UPDATE"},
		{"explain analyze delete from t", "DELETE"},
		{"EXPLAIN DROP TABLE t", "DROP"},
		{"EXPLAIN TRUNCATE TABLE t", "TRUNCATE"},
		{"EXPLAIN FORMAT=JSON INSERT INTO t VALUES(1)", "INSERT"},
		{"EXPLAIN QUERY PLAN INSERT INTO t VALUES(1)", "INSERT"},
		{"EXPLAIN PLAN FOR INSERT INTO t VALUES(1)", "INSERT"},
		{"EXPLAIN WITH x AS (SELECT 1) INSERT INTO y VALUES (1)", "WITH CTE"},
		{"EXPLAIN WITH ins AS (INSERT INTO t VALUES(1) RETURNING id) SELECT * FROM ins", "WITH CTE"},
		{"EXPLAIN SELECT * INTO new_table FROM t", "SELECT INTO"},
		{"EXPLAIN", "inner statement"},
		// Comment / hint before the write verb still rejected
		{"EXPLAIN /*+ hint */ INSERT INTO t VALUES(1)", "INSERT"},
		{"EXPLAIN -- note\nDELETE FROM t", "DELETE"},
		// EXPLAIN + multi-statement
		{"EXPLAIN SELECT 1; DROP TABLE t", "multiple statements"},
		{"EXPLAIN SELECT 1; INSERT INTO t VALUES(1)", "multiple statements"},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			err := Validate(tt.sql)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tt.sql)
			}
			e, ok := err.(*ErrReadOnlyViolation)
			if !ok {
				t.Fatalf("expected *ErrReadOnlyViolation, got %T", err)
			}
			if !strings.Contains(e.Reason, tt.reason) {
				t.Errorf("reason %q should contain %q", e.Reason, tt.reason)
			}
		})
	}
}

func TestValidate_EmptyQuery(t *testing.T) {
	tests := []string{"", "  ", "\t", "\n"}
	for _, sql := range tests {
		t.Run("empty_"+sql, func(t *testing.T) {
			err := Validate(sql)
			if err == nil {
				t.Fatal("expected error for empty query")
			}
			if !strings.Contains(err.Error(), "empty query") {
				t.Errorf("expected 'empty query', got %q", err.Error())
			}
		})
	}
}

func TestValidate_MultiStatement(t *testing.T) {
	tests := []struct {
		sql  string
		want int // expected statement count in error
	}{
		{"SELECT 1; SELECT 2", 2},
		{"SELECT 1; DROP TABLE x", 2},
		{"SELECT 1;DROP TABLE x;SELECT 2", 3},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			err := Validate(tt.sql)
			if err == nil {
				t.Fatalf("expected error for multi-statement %q", tt.sql)
			}
			if !strings.Contains(err.Error(), "multiple statements") {
				t.Errorf("expected 'multiple statements', got %q", err.Error())
			}
		})
	}
}

func TestValidate_UnknownVerb(t *testing.T) {
	err := Validate("FOOBAR baz")
	if err == nil {
		t.Fatal("expected error for unknown verb")
	}
	if !strings.Contains(err.Error(), "unknown or unsupported") {
		t.Errorf("expected 'unknown or unsupported', got %q", err.Error())
	}
}

func TestValidate_LeadingWhitespace(t *testing.T) {
	// Leading whitespace should be trimmed and the query accepted
	tests := []string{
		"  SELECT 1",
		"\tSELECT 1",
		"\nSELECT 1",
		"\r\nSELECT 1",
		"  \t\n  SELECT * FROM t",
	}
	for _, sql := range tests {
		t.Run("ws", func(t *testing.T) {
			if err := Validate(sql); err != nil {
				t.Errorf("expected nil for %q, got %v", sql, err)
			}
		})
	}
}

func TestValidate_CTEWithLeadingParen(t *testing.T) {
	// SQL WITH clause wrapped in parentheses (common in some query generators)
	if err := Validate("(WITH cte AS (SELECT 1) SELECT * FROM cte)"); err != nil {
		t.Errorf("expected nil for parenthesized CTE, got %v", err)
	}
}

// ─── stripExplainPrefix ────────────────────────────────────────────────────────

func TestStripExplainPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Plain generic form
		{"EXPLAIN SELECT * FROM t", "SELECT * FROM t"},
		// Whitespace tolerance
		{"  EXPLAIN   SELECT * FROM t", "SELECT * FROM t"},
		// Postgres / GaussDB option list
		{"EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) SELECT * FROM t", "SELECT * FROM t"},
		{"EXPLAIN (ANALYZE, FORMAT TEXT) SELECT * FROM t", "SELECT * FROM t"},
		// MySQL
		{"EXPLAIN FORMAT=JSON SELECT * FROM t", "SELECT * FROM t"},
		{"EXPLAIN FORMAT JSON SELECT * FROM t", "SELECT * FROM t"},
		// MySQL 8.0.18+ / DuckDB
		{"EXPLAIN ANALYZE SELECT * FROM t", "SELECT * FROM t"},
		// SQLite
		{"EXPLAIN QUERY PLAN SELECT * FROM t", "SELECT * FROM t"},
		// Oracle (two-step)
		{"EXPLAIN PLAN FOR SELECT * FROM t", "SELECT * FROM t"},
		// ClickHouse
		{"EXPLAIN PLAN SELECT * FROM t", "SELECT * FROM t"},
		// Postgres bare VERBOSE
		{"EXPLAIN VERBOSE SELECT * FROM t", "SELECT * FROM t"},
		// WITH inner
		{"EXPLAIN WITH cte AS (SELECT 1) SELECT * FROM cte", "WITH cte AS (SELECT 1) SELECT * FROM cte"},
		// Write inner — stripping must expose the write verb
		{"EXPLAIN INSERT INTO t VALUES(1)", "INSERT INTO t VALUES(1)"},
		{"EXPLAIN (ANALYZE, BUFFERS) INSERT INTO t VALUES(1)", "INSERT INTO t VALUES(1)"},
		{"EXPLAIN ANALYZE UPDATE t SET a=1", "UPDATE t SET a=1"},
		// Option list with a quoted string must not disturb the balance
		{"EXPLAIN (ANALYZE, 'x) y') SELECT 1", "SELECT 1"},
		// EXPLAIN alone — no inner statement
		{"EXPLAIN", ""},
		{"EXPLAIN   ", ""},
		// Unbalanced parenthesis — fail-closed
		{"EXPLAIN (ANALYZE SELECT 1", ""},
		// Not EXPLAIN (word boundary)
		{"EXPLAINABLE SELECT * FROM t", "EXPLAINABLE SELECT * FROM t"},
		// Lowercase
		{"explain analyze insert into t values(1)", "insert into t values(1)"},
		// Comments / hints between EXPLAIN and the inner statement
		{"EXPLAIN /*+ hint */ SELECT 1", "SELECT 1"},
		{"EXPLAIN /*+ qb_name(q1) */ SELECT * FROM t", "SELECT * FROM t"},
		{"EXPLAIN (ANALYZE) /* c */ SELECT 1", "SELECT 1"},
		{"EXPLAIN -- note\nSELECT 1", "SELECT 1"},
		{"EXPLAIN /* c */ ANALYZE /* c2 */ SELECT 1", "SELECT 1"},
		{"EXPLAIN /*+ hint */ INSERT INTO t VALUES(1)", "INSERT INTO t VALUES(1)"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripExplainPrefix(tt.input)
			if got != tt.want {
				t.Errorf("stripExplainPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── AutoLimit ────────────────────────────────────────────────────────────────

func TestAutoLimit_AddsLimit(t *testing.T) {
	tests := []struct {
		sql     string
		maxRows int
		want    string
	}{
		{"SELECT 1", 100, "SELECT 1 LIMIT 100"},
		{"SELECT * FROM users", 1000, "SELECT * FROM users LIMIT 1000"},
		{"select id from t", 50, "select id from t LIMIT 50"},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", 500, "WITH cte AS (SELECT 1) SELECT * FROM cte LIMIT 500"},
		{"EXPLAIN SELECT * FROM t", 200, "EXPLAIN SELECT * FROM t LIMIT 200"},
		// Optimizer hint SQL — should get LIMIT appended correctly
		{"SELECT /*+ hint */ * FROM t", 200, "SELECT /*+ hint */ * FROM t LIMIT 200"},
		{"SELECT /*+ FULL(t) */ * FROM t WHERE id=1", 100, "SELECT /*+ FULL(t) */ * FROM t WHERE id=1 LIMIT 100"},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := AutoLimit(tt.sql, tt.maxRows)
			if got != tt.want {
				t.Errorf("AutoLimit(%q, %d) = %q, want %q", tt.sql, tt.maxRows, got, tt.want)
			}
		})
	}
}

func TestAutoLimit_ExistingLimit(t *testing.T) {
	tests := []string{
		"SELECT * FROM t LIMIT 10",
		"SELECT * FROM t limit 100",
		"SELECT * FROM t LIMIT\t10",
		"SELECT * FROM t\nLIMIT 10",
		"SELECT * FROM t WHERE x=1 ORDER BY y LIMIT 50",
		"SELECT * FROM t LIMIT(3)",
		"SELECT * FROM t LIMIT(  10  )",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			got := AutoLimit(sql, 1000)
			if got != sql {
				t.Errorf("should not modify query with existing LIMIT: got %q", got)
			}
		})
	}
}

func TestAutoLimit_NonApplicable(t *testing.T) {
	tests := []string{
		"SHOW TABLES",
		"DESCRIBE users",
		"DESC users",
		"PRAGMA table_info('x')",
		"ANALYZE TABLE t",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			got := AutoLimit(sql, 1000)
			if got != sql {
				t.Errorf("should not add LIMIT to %q: got %q", sql, got)
			}
		})
	}
}

func TestAutoLimit_TrailingSemicolon(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{"SELECT 1;", "SELECT 1 LIMIT 100"},
		{"SELECT * FROM t;", "SELECT * FROM t LIMIT 100"},
		{"SELECT 1 ;", "SELECT 1 LIMIT 100"}, // space before semicolon
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := AutoLimit(tt.sql, 100)
			if got != tt.want {
				t.Errorf("AutoLimit(%q, 100) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

func TestAutoLimit_CaseInsensitiveLimit(t *testing.T) {
	// LIMIT keyword in mixed case should be detected
	tests := []string{
		"SELECT * FROM t LiMiT 10",
		"SELECT * FROM t limit 5",
		"SELECT * FROM t Limit 20",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			got := AutoLimit(sql, 1000)
			if got != sql {
				t.Errorf("should not add LIMIT when already present: got %q", got)
			}
		})
	}
}

// ─── firstWord ────────────────────────────────────────────────────────────────

func TestFirstWord(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SELECT 1", "SELECT"},
		{"  select id", "select"},
		{"\t\tSELECT\nx", "SELECT"},
		{"WITH", "WITH"},
		{"(WITH cte", "WITH"},
		{"  (  SELECT 1", "SELECT"}, // strips '(' then trims, first word is SELECT
		{"singleword", "singleword"},
		{"", ""},
		// Optimizer hint comment should not interfere with first-word detection
		{"SELECT /*+ hint */ * FROM t", "SELECT"},
		{"  SELECT /*+ hint */ * FROM t", "SELECT"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := firstWord(tt.input)
			if got != tt.want {
				t.Errorf("firstWord(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── splitStatements ──────────────────────────────────────────────────────────

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		sql  string
		want int
	}{
		{"SELECT 1", 1},
		{"SELECT 1; SELECT 2", 2},
		{"SELECT 1;DROP TABLE x", 2},
		{"SELECT 1; DROP TABLE x; SELECT 3", 3},
		{"", 0},
		{"  ", 0},
		{";", 0},         // only empty parts
		{"SELECT 1;", 1}, // trailing semicolon
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			got := splitStatements(tt.sql)
			if len(got) != tt.want {
				t.Errorf("splitStatements(%q) = %d statements, want %d", tt.sql, len(got), tt.want)
			}
		})
	}
}

// ─── ErrReadOnlyViolation ─────────────────────────────────────────────────────

func TestErrReadOnlyViolation_Error(t *testing.T) {
	e := &ErrReadOnlyViolation{SQL: "DROP TABLE x", Reason: `write operation "DROP" is not allowed`}
	got := e.Error()
	if !strings.Contains(got, "READ_ONLY_VIOLATION") {
		t.Errorf("error string should contain READ_ONLY_VIOLATION: %q", got)
	}
	if !strings.Contains(got, "DROP") {
		t.Errorf("error string should contain DROP: %q", got)
	}
}
