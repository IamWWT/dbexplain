// Package executor provides the query execution pipeline for all database types.
// It handles: capability routing → policy → sqlguard (if SQL) → AutoLimit (if SQL) →
// concurrent lock → Queryable check → execution → masking.
package executor

import (
	"context"
	"fmt"
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

	// SQL-specific validation and transformation
	if opts.IsSQL {
		if err := sqlguard.Validate(sqlArg); err != nil {
			return nil, err
		}
		if err := opts.Policies.CheckSQL(sqlArg); err != nil {
			return nil, err
		}
		if opts.Explain {
			sqlArg = wrapExplain(sqlArg, opts.Parsed.Kind)
		} else {
			sqlArg = sqlguard.AutoLimit(sqlArg, opts.Limit)
		}
	} else {
		if err := opts.Policies.CheckNative(sqlArg, opts.Parsed.Kind); err != nil {
			return nil, err
		}
	}

	// Concurrent control by label
	if !opts.Lock.Lock(opts.Parsed.Label) {
		return nil, fmt.Errorf("CONCURRENT_LIMIT: a query is already running for label %q", opts.Parsed.Label)
	}
	defer opts.Lock.Unlock(opts.Parsed.Label)

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

	result, err := q.ExecQuery(ctx, execOpts)
	if err != nil {
		return nil, fmt.Errorf("QUERY_ERROR: %w", err)
	}

	// Apply post-execution column masking
	opts.Policies.ApplyMask(result)

	return result, nil
}

// wrapExplain wraps SQL with the database-appropriate EXPLAIN prefix.
func wrapExplain(sql string, kind string) string {
	switch kind {
	case "mysql":
		return "EXPLAIN FORMAT=JSON " + sql
	case "postgres", "gaussdb":
		return "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) " + sql
	case "sqlite":
		return "EXPLAIN QUERY PLAN " + sql
	case "clickhouse":
		return "EXPLAIN PLAN " + sql
	default:
		return "EXPLAIN " + sql
	}
}
