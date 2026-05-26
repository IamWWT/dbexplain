// Package query provides types and interfaces for read-only SQL query execution.
// It is separate from the schema collection pipeline — QueryResult is a pure
// data table (columns + rows), not schema.Instance/Graph.
package query

import (
	"context"
	"fmt"
	"sync"

	"github.com/IamWWT/dbexplain/dsn"
)

// ColumnInfo describes a single result column.
type ColumnInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// QueryResult holds the result of an executed read-only query.
// This is a data-table format, deliberately distinct from schema.Instance
// to avoid confusion with the schema collection output.
type QueryResult struct {
	Columns       []ColumnInfo `json:"columns"`
	Rows          [][]*string  `json:"rows"`
	RowCount      int          `json:"row_count"`
	Truncated     bool         `json:"truncated"`
	ExecutionTime string       `json:"execution_time"`
}

// ExecuteOpts bundles all parameters for query execution.
type ExecuteOpts struct {
	DSN      *dsn.DSN
	SQL      string
	MaxRows  int
	Timeout  int // seconds
	Explain  bool
}

// Queryable is an optional interface that relational connectors can implement.
// It is separate from the Connector interface — non-relational connectors
// (Redis, MongoDB, ES, Qdrant) do not implement it.
type Queryable interface {
	ExecQuery(ctx context.Context, opts ExecuteOpts) (*QueryResult, error)
}

// ErrNotSupported is returned when a connector does not support query execution.
type ErrNotSupported struct {
	Kind string
}

func (e *ErrNotSupported) Error() string {
	return fmt.Sprintf("QUERY_NOT_SUPPORTED: %s does not support SQL query execution", e.Kind)
}

// ErrConcurrentLimit is returned when a query is already running for the same label.
type ErrConcurrentLimit struct {
	Label string
}

func (e *ErrConcurrentLimit) Error() string {
	return fmt.Sprintf("CONCURRENT_LIMIT: a query is already running for label %q", e.Label)
}

// QueryLock provides per-label mutual exclusion to prevent concurrent
// queries against the same database instance.
type QueryLock struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewQueryLock creates a new QueryLock.
func NewQueryLock() *QueryLock {
	return &QueryLock{locks: make(map[string]*sync.Mutex)}
}

// Lock acquires the lock for a given label. Returns false if already locked.
func (ql *QueryLock) Lock(label string) bool {
	ql.mu.Lock()
	mu, ok := ql.locks[label]
	if !ok {
		mu = &sync.Mutex{}
		ql.locks[label] = mu
	}
	ql.mu.Unlock()
	return mu.TryLock()
}

// Unlock releases the lock for a given label.
func (ql *QueryLock) Unlock(label string) {
	ql.mu.Lock()
	mu, ok := ql.locks[label]
	ql.mu.Unlock()
	if ok {
		mu.Unlock()
	}
}
