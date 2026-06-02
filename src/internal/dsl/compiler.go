package dsl

import (
	"fmt"
	"strings"
)

// ErrCompile is returned when DSL compilation fails.
type ErrCompile struct {
	Reason string
}

func (e *ErrCompile) Error() string {
	return fmt.Sprintf("DSL compile error: %s", e.Reason)
}

// CompileToSQL compiles a bound DSLQuery into executable SQL for SQL databases.
// It replaces placeholder table names with the actual table names from the
// source references.
//
// Returns the compiled SQL string. If the query has no @ references, returns
// the original input unchanged.
//
// Returns an error if:
//   - Any source reference is not an SQL database
//   - The query or bound sources are nil
func CompileToSQL(query *DSLQuery, bound *BoundQuery) (string, error) {
	if query == nil {
		return "", &ErrCompile{"nil query"}
	}
	if bound == nil {
		return "", &ErrCompile{"nil bound query"}
	}

	if !query.HasSourceRefs() {
		// No @ references — return the raw input as-is
		return query.Raw, nil
	}

	sql := query.SQL

	for placeholder, bs := range bound.Sources {
		if bs.Kind != SourceSQL {
			return "", &ErrCompile{
				Reason: fmt.Sprintf("source @%s.%s is a %s backend, not an SQL database",
					bs.Ref.Label, bs.Ref.Table, sourceKindName(bs.Kind)),
			}
		}
		sql = strings.ReplaceAll(sql, placeholder, bs.Ref.Table)
	}

	return sql, nil
}

// SourceKinds returns the distinct source kinds referenced in a bound query.
func (bq *BoundQuery) SourceKinds() []SourceKind {
	seen := make(map[SourceKind]bool)
	var kinds []SourceKind
	for _, bs := range bq.Sources {
		if !seen[bs.Kind] {
			seen[bs.Kind] = true
			kinds = append(kinds, bs.Kind)
		}
	}
	return kinds
}

// PrimarySource returns the first source in the bound query, or nil if empty.
func (bq *BoundQuery) PrimarySource() *BoundSource {
	for _, bs := range bq.Sources {
		return &bs
	}
	return nil
}

func sourceKindName(kind SourceKind) string {
	switch kind {
	case SourceSQL:
		return "SQL"
	case SourceFile:
		return "file"
	case SourceNative:
		return "native"
	default:
		return "unknown"
	}
}
