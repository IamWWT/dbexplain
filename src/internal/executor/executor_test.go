package executor

import (
	"testing"
)

func TestWrapExplainHint(t *testing.T) {
	tests := []struct {
		kind string
		sql  string
		want string
	}{
		{
			kind: "gaussdb",
			sql:  "SELECT /*+ hashjoin(t1 t2) */ * FROM t1 JOIN t2 ON t1.id = t2.id",
			want: "EXPLAIN (ANALYZE, FORMAT TEXT) SELECT /*+ hashjoin(t1 t2) */ * FROM t1 JOIN t2 ON t1.id = t2.id",
		},
		{
			kind: "mysql",
			sql:  "SELECT /*+ BKA(t) */ * FROM t JOIN s",
			want: "EXPLAIN FORMAT=JSON SELECT /*+ BKA(t) */ * FROM t JOIN s",
		},
		{
			kind: "postgres",
			sql:  "SELECT /*+ SeqScan(t) */ * FROM t",
			want: "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) SELECT /*+ SeqScan(t) */ * FROM t",
		},
		{
			kind: "oracle",
			sql:  "SELECT /*+ FULL(t) */ * FROM t",
			want: "EXPLAIN PLAN FOR SELECT /*+ FULL(t) */ * FROM t",
		},
		{
			kind: "hive",
			sql:  "SELECT /*+ MAPJOIN(t) */ * FROM t",
			want: "EXPLAIN SELECT /*+ MAPJOIN(t) */ * FROM t",
		},
		{
			kind: "sqlite",
			sql:  "SELECT /*+ dummy */ * FROM t",
			want: "EXPLAIN QUERY PLAN SELECT /*+ dummy */ * FROM t",
		},
		{
			kind: "clickhouse",
			sql:  "SELECT /*+ dummy */ * FROM t",
			want: "EXPLAIN PLAN SELECT /*+ dummy */ * FROM t",
		},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := wrapExplain(tt.sql, tt.kind)
			if got != tt.want {
				t.Errorf("wrapExplain(%q, %q)\n  got:  %q\n  want: %q", tt.sql, tt.kind, got, tt.want)
			}
		})
	}
}

func TestWrapExplainNoHint(t *testing.T) {
	// Plain SQL without hints should still wrap correctly
	tests := []struct {
		kind string
		sql  string
		want string
	}{
		{"gaussdb", "SELECT * FROM t", "EXPLAIN (ANALYZE, FORMAT TEXT) SELECT * FROM t"},
		{"mysql", "SELECT * FROM t", "EXPLAIN FORMAT=JSON SELECT * FROM t"},
		{"postgres", "SELECT * FROM t", "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) SELECT * FROM t"},
		{"oracle", "SELECT * FROM t", "EXPLAIN PLAN FOR SELECT * FROM t"},
		{"sqlite", "SELECT * FROM t", "EXPLAIN QUERY PLAN SELECT * FROM t"},
		{"clickhouse", "SELECT * FROM t", "EXPLAIN PLAN SELECT * FROM t"},
		{"duckdb", "SELECT * FROM t", "EXPLAIN SELECT * FROM t"},
		{"hive", "SELECT * FROM t", "EXPLAIN SELECT * FROM t"},
		{"unknown", "SELECT * FROM t", "EXPLAIN SELECT * FROM t"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := wrapExplain(tt.sql, tt.kind)
			if got != tt.want {
				t.Errorf("wrapExplain(%q, %q)\n  got:  %q\n  want: %q", tt.sql, tt.kind, got, tt.want)
			}
		})
	}
}
