// Package dsl provides a unified DSL layer for dbexplain execute.
//
// The DSL extends standard SQL with @label.table syntax for referencing
// data sources by their DSN label. The package compiles DSL input into
// an intermediate representation that can be routed to the appropriate
// backend (SQL database, file, or native connector).
//
// DSL syntax example:
//
//	SELECT u.name, o.total
//	FROM @mydb.users u
//	JOIN @analytics.orders o ON u.id = o.user_id
//
// The @label.table references are resolved against the configured DSN
// entries by the Binder.
package dsl

import (
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// SourceRef represents a @label.table reference to a data source.
type SourceRef struct {
	Label       string // DSN label (e.g., "mydb" in @mydb.users)
	Table       string // table name (e.g., "users" in @mydb.users)
	Placeholder string // replacement token in the preprocessed SQL (e.g., "__dsl_0__")
	IsRawPromQL bool   // true = Table is a raw PromQL expression (from promql(...) syntax)
}

// DSLQuery is the result of parsing DSL input.
// It contains the standard sqlast statement alongside the source
// references extracted from @label.table syntax.
type DSLQuery struct {
	Raw     string              // original DSL input
	SQL     string              // preprocessed SQL (with placeholders)
	Stmt    sqlast.Stmt         // parsed SQL statement
	Sources map[string]SourceRef // placeholder → SourceRef
}

// SourceKind categorises the type of data source a bound reference points to.
type SourceKind int

const (
	SourceSQL    SourceKind = iota // SQL database (mysql, postgres, sqlite, etc.)
	SourceFile                     // File-based (csv, xlsx)
	SourceNative                   // Native connector (redis, mongo, qdrant, etc.)
)

// BoundSource holds the resolved information for a single @label.table reference.
type BoundSource struct {
	Ref    SourceRef  // the original reference
	DSN    *dsn.DSN   // parsed DSN matching the label
	Kind   SourceKind  // determined from connector capabilities
	Vendor Vendor      // query language vendor (SQL, PromQL, file)
}

// BoundQuery is the result of binding a DSLQuery against available DSN entries.
// Each source placeholder in the query is resolved to a concrete data source.
type BoundQuery struct {
	Query   *DSLQuery
	Sources map[string]BoundSource // placeholder → resolved source info
}
