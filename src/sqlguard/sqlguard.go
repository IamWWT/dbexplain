// Package sqlguard provides read-only SQL validation and safe guards
// for the -execute subcommand. It enforces that only read operations
// are allowed and applies automatic safety limits.
package sqlguard

import (
	"fmt"
	"strings"
)

// writeOps contains SQL verbs that are unconditionally rejected.
var writeOps = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE",
	"TRUNCATE", "RENAME", "REPLACE", "GRANT", "REVOKE",
	"MERGE", "UPSERT", "LOAD", "IMPORT", "EXPORT",
}

// readOps contains SQL verbs that are allowed.
var readOps = []string{
	"SELECT", "EXPLAIN", "WITH", "SHOW", "DESCRIBE", "DESC",
	"PRAGMA", "ANALYZE", "CHECK", "REINDEX",
}

// ErrReadOnlyViolation is returned when the SQL contains a write operation.
type ErrReadOnlyViolation struct {
	SQL    string
	Reason string
}

func (e *ErrReadOnlyViolation) Error() string {
	return fmt.Sprintf("READ_ONLY_VIOLATION: %s", e.Reason)
}

// Validate checks that the given SQL is read-only.
// It rejects:
//   - Multiple statements (semicolon-separated)
//   - Any write operation verb (INSERT, UPDATE, DELETE, DROP, etc.)
//
// Returns nil if the SQL is safe to execute.
func Validate(sql string) error {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return &ErrReadOnlyViolation{SQL: sql, Reason: "empty query"}
	}

	// Reject multiple statements (semicolons outside string literals may be tricky,
	// but a simple split and count is a safe first defense)
	statements := splitStatements(sql)
	if len(statements) > 1 {
		return &ErrReadOnlyViolation{
			SQL:    sql,
			Reason: fmt.Sprintf("multiple statements detected (%d)", len(statements)),
		}
	}

	// Normalize whitespace for token extraction
	normalized := strings.TrimSpace(statements[0])
	firstToken := strings.ToUpper(firstWord(normalized))

	// Check write ops first
	for _, op := range writeOps {
		if firstToken == op {
			return &ErrReadOnlyViolation{
				SQL:    sql,
				Reason: fmt.Sprintf("write operation %q is not allowed", op),
			}
		}
	}

	// Check allowed read ops
	for _, op := range readOps {
		if firstToken == op {
			return nil
		}
	}

	return &ErrReadOnlyViolation{
		SQL:    sql,
		Reason: fmt.Sprintf("unknown or unsupported SQL verb %q", firstToken),
	}
}

// AutoLimit appends "LIMIT n" to a SELECT/EXPLAIN query if no LIMIT clause
// is already present.
func AutoLimit(sql string, maxRows int) string {
	sql = strings.TrimSpace(sql)
	upper := strings.ToUpper(sql)

	// Only add LIMIT to SELECT / WITH / EXPLAIN queries
	firstToken := strings.ToUpper(firstWord(sql))
	if firstToken != "SELECT" && firstToken != "WITH" && firstToken != "EXPLAIN" {
		return sql
	}

	// Don't add LIMIT if one already exists
	if strings.Contains(upper, "LIMIT ") || strings.Contains(upper, "LIMIT\t") || strings.Contains(upper, "LIMIT\n") {
		return sql
	}

	// Don't add LIMIT for SHOW, DESCRIBE, PRAGMA etc.
	if firstToken != "SELECT" && firstToken != "WITH" && firstToken != "EXPLAIN" {
		return sql
	}

	// Strip trailing semicolon before appending LIMIT
	sql = strings.TrimSuffix(sql, ";")
	sql = strings.TrimSpace(sql)

	return fmt.Sprintf("%s LIMIT %d", sql, maxRows)
}

// splitStatements does a basic count of statements by splitting on semicolons.
// This is a conservative first-pass; it does not handle semicolons inside
// string literals or comments perfectly, but any false positive is safe
// (the query is rejected rather than executed).
func splitStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	var nonEmpty []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return nonEmpty
}

// firstWord returns the first whitespace-delimited word of s.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	// Skip leading '(' for CTEs like (WITH ...)
	if len(s) > 0 && s[0] == '(' {
		s = strings.TrimSpace(s[1:])
	}
	for i, c := range s {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return s[:i]
		}
	}
	return s
}
