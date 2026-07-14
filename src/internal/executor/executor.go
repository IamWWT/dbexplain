// Package executor provides the query execution pipeline for all database types.
// It handles: capability routing → policy → sqlguard (if SQL) → AutoLimit (if SQL) →
// concurrent lock → Queryable check → execution → masking.
package executor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/policy"
	"github.com/IamWWT/dbexplain/internal/sqlguard"
)

// ExecOptions holds all parameters for the execution pipeline.
type ExecOptions struct {
	Conn       connector.Connector
	Parsed     *dsn.DSN
	SQL        string
	Limit      int
	Explain    bool
	TimeoutSec int
	Policies   *policy.Config
	Lock       *query.QueryLock
	IsSQL      bool
	Context    context.Context // optional caller context; derived from Background if nil
}

// ExecQuery runs the full execution pipeline for SQL and native connectors.
// It handles validation, policy, concurrent control, execution, and masking.
// Returns the query result ready for output rendering.
func ExecQuery(opts *ExecOptions) (*query.QueryResult, error) {
	sqlArg := opts.SQL

	if opts.Parsed == nil {
		return nil, fmt.Errorf("ExecOptions.Parsed is nil")
	}

	// Log original SQL (truncated to MaxSQLLogLen chars for safety)
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	logSQL := connector.TruncateSQL(sqlArg)
	connector.Logf(ctx, "[%s] [execute] %s", opts.Parsed.Kind, logSQL)

	// SQL-specific validation and transformation
	if opts.IsSQL {
		if err := sqlguard.Validate(sqlArg); err != nil {
			return nil, err
		}
		if err := opts.Policies.CheckSQL(sqlArg); err != nil {
			return nil, err
		}
		if opts.Explain {
			if hasExplainPrefix(sqlArg) {
				fmt.Fprintf(os.Stderr, "WARNING: SQL already starts with EXPLAIN — --explain flag is redundant, skipping double-wrap\n")
			} else {
				sqlArg = wrapExplain(sqlArg, opts.Parsed.Kind)
			}
		} else {
			sqlArg = sqlguard.AutoLimit(sqlArg, opts.Limit)
		}
		// Log wrapped SQL if different from original (after EXPLAIN/AutoLimit transformation)
		if sqlArg != logSQL && len(sqlArg) > 0 {
			wrappedLog := connector.TruncateSQL(sqlArg)
			connector.Logf(ctx, "[%s] [execute] (wrapped) %s", opts.Parsed.Kind, wrappedLog)
		}
	} else {
		if err := opts.Policies.CheckNative(sqlArg, opts.Parsed.Kind); err != nil {
			return nil, err
		}
	}

	// Concurrent control by label (skip if Lock not set)
	if opts.Lock != nil {
		if !opts.Lock.Lock(opts.Parsed.Label) {
			return nil, fmt.Errorf("CONCURRENT_LIMIT: a query is already running for label %q", opts.Parsed.Label)
		}
		defer opts.Lock.Unlock(opts.Parsed.Label)
	}

	// Check if connector supports Queryable
	q, ok := opts.Conn.(query.Queryable)
	if !ok {
		return nil, fmt.Errorf("QUERY_NOT_SUPPORTED: %s does not support query execution", opts.Parsed.Kind)
	}

	// Execute — use caller context if provided, fall back to Background
	baseCtx := context.Background()
	if opts.Context != nil {
		baseCtx = opts.Context
	}
	ctx, cancel := context.WithTimeout(baseCtx, time.Duration(opts.TimeoutSec+5)*time.Second)
	defer cancel()

	execOpts := query.ExecuteOpts{
		DSN:     opts.Parsed,
		SQL:     sqlArg,
		MaxRows: opts.Limit,
		Timeout: opts.TimeoutSec,
		Explain: opts.Explain,
	}

	// Run query in sub-goroutine with timeout guard.
	// pgx/v5 context cancellation is reliable, but the
	// select+channel pattern remains as a defense-in-depth timeout guard.
	type execResult struct {
		result *query.QueryResult
		err    error
	}
	execCh := make(chan execResult, 1)
	go func() {
		res, execErr := q.ExecQuery(ctx, execOpts)
		execCh <- execResult{res, execErr}
	}()

	var (
		result *query.QueryResult
		err    error
	)
	// Use time.NewTimer for reliable timeout (see handler.go for rationale).
	execTimer := time.NewTimer(time.Duration(opts.TimeoutSec+5) * time.Second)
	select {
	case r := <-execCh:
		result, err = r.result, r.err
		execTimer.Stop()
	case <-execTimer.C:
		return nil, fmt.Errorf("QUERY_TIMEOUT: query execution exceeded %d seconds", opts.TimeoutSec)
	}
	if err != nil {
		return nil, fmt.Errorf("QUERY_ERROR: %w", err)
	}

	// Apply post-execution column masking and stripping
	opts.Policies.ApplyMask(result)
	opts.Policies.StripDeniedColumns(result)

	return result, nil
}

// wrapExplain wraps SQL with the database-appropriate EXPLAIN prefix.
func wrapExplain(sql string, kind string) string {
	switch kind {
	case "mysql":
		return "EXPLAIN FORMAT=JSON " + sql
	case "postgres":
		return "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) " + sql
	case "gaussdb":
		// GaussDB Oracle 兼容模式不支持 BUFFERS 选项
		return "EXPLAIN (ANALYZE, FORMAT TEXT) " + sql
	case "sqlite":
		return "EXPLAIN QUERY PLAN " + sql
	case "clickhouse":
		return "EXPLAIN PLAN " + sql
	case "duckdb":
		return "EXPLAIN " + sql
	case "oracle":
		return "EXPLAIN PLAN FOR " + sql
	case "hive":
		return "EXPLAIN " + sql
	case "starrocks":
		return "EXPLAIN " + sql
	default:
		return "EXPLAIN " + sql
	}
}

// hasExplainPrefix reports whether sql starts with the EXPLAIN keyword (case-insensitive),
// ignoring leading whitespace. Used to prevent double-wrapping when --explain flag is used
// on a query that already includes EXPLAIN.
func hasExplainPrefix(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)
	return strings.HasPrefix(upper, "EXPLAIN")
}
