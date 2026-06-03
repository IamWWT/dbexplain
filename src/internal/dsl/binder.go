package dsl

import (
	"fmt"

	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
)

// ErrUnresolved is returned when a @label.table reference cannot be resolved.
type ErrUnresolved struct {
	Label string
	Table string
}

func (e *ErrUnresolved) Error() string {
	return fmt.Sprintf("unresolved data source: @%s.%s — no DSN with matching label found", e.Label, e.Table)
}

// Bind resolves all @label.table references in a DSLQuery against the
// available DSN entries. It returns a BoundQuery where each placeholder
// is mapped to a concrete data source.
//
// The binder iterates all provided DSN entries, finds entries whose label
// matches each reference, and categorises the source kind based on the
// connector type (SQL, file, or native).
func Bind(query *DSLQuery, entries []config.DSNEntry) (*BoundQuery, error) {
	if query == nil {
		return nil, fmt.Errorf("nil query")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no DSN entries available for binding")
	}

	// Build a lookup: label → DSN (first match wins)
	labelDSN := make(map[string]*dsn.DSN)
	for _, entry := range entries {
		parsed, err := dsn.ParseDSN(entry.Raw)
		if err != nil {
			continue
		}
		if parsed.Label == "" {
			continue
		}
		if _, exists := labelDSN[parsed.Label]; !exists {
			labelDSN[parsed.Label] = parsed
		}
	}

	// Resolve each source reference
	sources := make(map[string]BoundSource, len(query.Sources))
	for placeholder, ref := range query.Sources {
		resolved, ok := labelDSN[ref.Label]
		if !ok {
			return nil, &ErrUnresolved{Label: ref.Label, Table: ref.Table}
		}

		kind := classifySource(resolved.Kind)

		sources[placeholder] = BoundSource{
			Ref:  ref,
			DSN:  resolved,
			Kind: kind,
		}
	}

	return &BoundQuery{
		Query:   query,
		Sources: sources,
	}, nil
}

// classifySource maps a connector kind string to a SourceKind.
func classifySource(kind string) SourceKind {
	switch kind {
	case "csv", "xlsx", "tsv":
		return SourceFile
	case "mysql", "postgres", "sqlite", "clickhouse", "gaussdb", "duckdb":
		return SourceSQL
	default:
		// Redis, MongoDB, Qdrant, Elasticsearch → native
		return SourceNative
	}
}

// String returns a human-readable summary of the bound source.
func (bs BoundSource) String() string {
	kindName := "sql"
	switch bs.Kind {
	case SourceFile:
		kindName = "file"
	case SourceNative:
		kindName = "native"
	}
	return fmt.Sprintf("@%s.%s → %s (%s)", bs.Ref.Label, bs.Ref.Table, bs.DSN.Redacted(), kindName)
}
