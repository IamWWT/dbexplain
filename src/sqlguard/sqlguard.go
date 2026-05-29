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
	"ANALYZE", "REINDEX",
}

// readOps contains SQL verbs that are allowed.
var readOps = []string{
	"SELECT", "EXPLAIN", "WITH", "SHOW", "DESCRIBE", "DESC",
	"PRAGMA", "CHECK",
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
			// Special: WITH must not contain INSERT/UPDATE/DELETE inside CTE bodies
			if op == "WITH" {
				if containsCTEWrite(normalized) {
					return &ErrReadOnlyViolation{
						SQL:    sql,
						Reason: "WITH CTE contains write operation",
					}
				}
			}
			// Special: SELECT ... INTO is a write in PostgreSQL (CREATE TABLE AS SELECT)
			if op == "SELECT" {
				if isSelectInto(normalized) {
					return &ErrReadOnlyViolation{
						SQL:    sql,
						Reason: `SELECT INTO creates a new table and is not allowed`,
					}
				}
			}
			return nil
		}
	}

	return &ErrReadOnlyViolation{
		SQL:    sql,
		Reason: fmt.Sprintf("unknown or unsupported SQL verb %q", firstToken),
	}
}

// containsCTEWrite checks if a WITH query contains write operations
// (INSERT/UPDATE/DELETE/DROP/ALTER/TRUNCATE) inside CTE bodies.
// PostgreSQL allows data-modifying CTEs: WITH cte AS (DELETE FROM t RETURNING *) SELECT * FROM cte
func containsCTEWrite(normalized string) bool {
	upper := strings.ToUpper(normalized)
	// Find the first AS keyword to locate the CTE body
	asIdx := strings.Index(upper, " AS ")
	if asIdx < 0 {
		return false
	}
	// The CTE body starts at the first '(' after AS
	bodyStart := strings.IndexByte(upper[asIdx+4:], '(')
	if bodyStart < 0 {
		return false
	}
	bodyStart += asIdx + 4
	// Track parenthesis depth to find CTE body boundaries
	depth := 0
	writeVerbs := []string{"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "TRUNCATE "}
	for i := bodyStart; i < len(upper); i++ {
		switch upper[i] {
		case '(':
			if depth == 0 {
				depth = 1
			} else {
				depth++
			}
		case ')':
			depth--
			if depth < 0 {
				return false // malformed — unbalanced parens
			}
			if depth == 0 {
				// End of this CTE body — check if there are more CTEs
				// CTEs are comma-separated: WITH a AS (...), b AS (...) SELECT ...
				rest := upper[i+1:]
				trimmed := strings.TrimSpace(rest)
				if strings.HasPrefix(trimmed, ",") {
					// More CTEs follow — scan the next one
					nextAS := strings.Index(trimmed, " AS ")
					if nextAS < 0 {
						return false
					}
					nextBody := strings.IndexByte(trimmed[nextAS+4:], '(')
					if nextBody < 0 {
						return false
					}
					i = i + 1 + nextAS + 4 + nextBody - 1 // -1 because loop will increment
					depth = 1
					_ = strings.TrimSpace // suppress unused warning
				} else {
					// No more CTEs — we're past all CTE bodies
					break
				}
			}
		default:
			if depth == 1 {
				// Inside the outermost CTE body — check for write verbs
				for _, verb := range writeVerbs {
					if i+len(verb) <= len(upper) && upper[i:i+len(verb)] == verb {
						return true
					}
				}
			}
		}
	}
	return false
}

// isSelectInto checks if a SELECT has an INTO clause targeting a table
// (not a MySQL variable like INTO @var).
func isSelectInto(normalized string) bool {
	upper := strings.ToUpper(normalized)
	// Find "INTO" at word boundaries (preceded and followed by whitespace
	// or start/end of string).
	for i := 0; i+4 <= len(upper); i++ {
		if upper[i:i+4] != "INTO" {
			continue
		}
		// Check word boundary before INTO
		if i > 0 && !isWordBoundaryBefore(upper, i) {
			continue
		}
		// Check word boundary after INTO
		if i+4 < len(upper) && !isWordBoundaryAfter(upper, i+4) {
			continue
		}
		// Skip MySQL variables: INTO @var (read-only, variable assignment)
		after := strings.TrimSpace(upper[i+4:])
		if strings.HasPrefix(after, "@") || strings.HasPrefix(after, ":") {
			return false // MySQL INTO @var or PG INTO :variable
		}
		// INTO followed by a table name — this is a write
		return true
	}
	return false
}

// isWordBoundaryBefore checks if position i is at a word boundary
// (preceded by space, start of string, or non-alphanumeric).
func isWordBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	return s[i-1] == ' ' || s[i-1] == '\t' || s[i-1] == '\n' || s[i-1] == '\r'
}

// isWordBoundaryAfter checks if position i is a word boundary
// (followed by space, end of string, or non-alphanumeric).
func isWordBoundaryAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	return s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r'
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

	// Don't add LIMIT if one already exists at the outer level
	// (ignore LIMIT inside subqueries to prevent bypass via SELECT * FROM (SELECT ... LIMIT 99999))
	if hasOuterLimit(upper) {
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

// hasOuterLimit checks if a LIMIT clause exists at the top level of a query,
// ignoring LIMITs inside parenthesized subqueries. This prevents the auto-limit
// from being bypassed by placing a large LIMIT inside a subquery.
func hasOuterLimit(upper string) bool {
	depth := 0
	var stripped strings.Builder
	for _, ch := range upper {
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				stripped.WriteRune(ch)
			}
		}
	}
	outer := stripped.String()
	return strings.Contains(outer, "LIMIT ") ||
		strings.Contains(outer, "LIMIT\t") ||
		strings.Contains(outer, "LIMIT\n") ||
		strings.Contains(outer, "LIMIT\r")
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
