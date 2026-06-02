package dsl

import (
	"fmt"
	"strings"

	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// ErrSyntax is returned when DSL parsing fails.
type ErrSyntax struct {
	Input string
	Err   error
}

func (e *ErrSyntax) Error() string {
	return fmt.Sprintf("DSL syntax error: %v", e.Err)
}

func (e *ErrSyntax) Unwrap() error {
	return e.Err
}

// Parse compiles DSL input into a DSLQuery.
//
// It performs two phases:
//  1. Pre-process: extract @label.table references and replace with placeholders
//  2. SQL parse: pass the preprocessed SQL through the standard sqlast parser
//
// Returns an error if:
//   - The input is empty
//   - Preprocessing fails
//   - The underlying SQL cannot be parsed by sqlast
func Parse(input string) (*DSLQuery, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, &ErrSyntax{Input: input, Err: fmt.Errorf("empty input")}
	}

	// Phase 1: Pre-process @label.table references
	cleaned, refs, err := preprocess(input)
	if err != nil {
		return nil, &ErrSyntax{Input: input, Err: err}
	}

	// Phase 2: Parse the preprocessed SQL
	stmt, err := sqlast.Parse(cleaned)
	if err != nil {
		return nil, &ErrSyntax{Input: input, Err: err}
	}

	return &DSLQuery{
		Raw:     input,
		SQL:     cleaned,
		Stmt:    stmt,
		Sources: sourceRefsMap(refs),
	}, nil
}

// HasSourceRefs returns true if the query contains any @label.table references.
func (q *DSLQuery) HasSourceRefs() bool {
	return len(q.Sources) > 0
}

// PlaceholderTable returns the table name in the preprocessed SQL for a
// given source reference. This is useful for matching AST table names back
// to source references.
func (q *DSLQuery) PlaceholderTable(ref SourceRef) string {
	return ref.Placeholder
}
