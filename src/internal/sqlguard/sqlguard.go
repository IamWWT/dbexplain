// Package sqlguard provides read-only SQL validation and safe guards
// for the -execute subcommand. It enforces that only read operations
// are allowed and applies automatic safety limits.
//
// Validation uses a hybrid approach:
//  1. Try AST-level parsing via sqlast.Parse. If successful, verify
//     the statement type (SelectStmt / UnionStmt are read-only).
//  2. If AST parsing fails (e.g. EXPLAIN, SHOW, dialect-specific SQL),
//     fall back to string‑based first‑word detection.
//  3. EXPLAIN statements get a dedicated recursive branch: the prefix and its
//     dialect options are stripped, then the inner statement is validated
//     (prevents EXPLAIN-wrapped write statements such as
//     "EXPLAIN ANALYZE INSERT ...", which PostgreSQL/MySQL execute for real).
package sqlguard

import (
	"fmt"
	"strings"

	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// writeOps contains SQL verbs that are unconditionally rejected.
var writeOps = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE",
	"TRUNCATE", "RENAME", "REPLACE", "GRANT", "REVOKE",
	"MERGE", "UPSERT", "LOAD", "IMPORT", "EXPORT",
	"ANALYZE", "REINDEX",
	"KILL", "SHUTDOWN", "FLUSH", "SET", "RESET",
	"INSTALL", "UNINSTALL", "CALL", "PURGE",
}

// readOps contains SQL verbs that are allowed.
// NOTE: EXPLAIN is intentionally NOT listed here. It is handled by a
// dedicated recursive branch in Validate (see stripExplainPrefix) so that the
// inner statement is validated too. Leaving it here would be dead code (the
// branch intercepts first) and would silently re-open the read-only bypass
// if that branch is ever removed.
var readOps = []string{
	"SELECT", "WITH", "SHOW", "DESCRIBE", "DESC",
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
//   - SELECT INTO (PostgreSQL DDL write)
//   - CTE bodies containing write operations
//   - EXPLAIN statements whose inner statement is a write operation
//     (the prefix and dialect options are stripped via stripExplainPrefix,
//     then the inner statement is recursively validated)
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

	// CTE (WITH) queries cannot be reliably validated by AST parsing because
	// sqlast.Parse may silently ignore the CTE prefix and return a SelectStmt
	// for the main query, bypassing CTE write detection. Handle WITH before AST.
	normalized := strings.TrimSpace(statements[0])
	firstToken := strings.ToUpper(firstWord(normalized))
	if firstToken == "WITH" {
		return validateCTE(normalized, sql)
	}

	// EXPLAIN cannot be parsed by sqlast, and a naive first-word check would
	// accept EXPLAIN-wrapped write statements ("EXPLAIN ANALYZE INSERT ...",
	// which PostgreSQL / GaussDB / MySQL 8.0.18+ / DuckDB execute for real).
	// Strip the EXPLAIN prefix and its dialect options, then recursively
	// validate the inner statement with the full pipeline below.
	if firstToken == "EXPLAIN" {
		inner := stripExplainPrefix(normalized)
		if inner == "" {
			return &ErrReadOnlyViolation{
				SQL:    sql,
				Reason: "EXPLAIN without an inner statement",
			}
		}
		return Validate(inner)
	}

	// Try AST-level validation first — this handles standard SELECT / UNION.
	stmt, err := sqlast.Parse(sql)
	if err == nil {
		switch stmt.(type) {
		case *sqlast.SelectStmt:
			// Even if AST parsed successfully, check for SELECT INTO
			// (the parser does not recognise INTO and silently ignores the
			// clause, treating "SELECT * INTO t FROM u" as a bare SELECT).
			if isSelectInto(sql) {
				return &ErrReadOnlyViolation{
					SQL:    sql,
					Reason: `SELECT INTO creates a new table and is not allowed`,
				}
			}
			return nil
		case *sqlast.UnionStmt:
			return nil // clean read-only UNION
		default:
			return &ErrReadOnlyViolation{
				SQL:    sql,
				Reason: fmt.Sprintf("unsupported statement type %T", stmt),
			}
		}
	}

	// AST parsing failed — fall back to string-based first‑word detection
	// for non‑standard read ops (EXPLAIN, SHOW, DESCRIBE, PRAGMA, etc.).

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
			// Special: SELECT ... INTO is a write in PostgreSQL
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
// inside CTE bodies or in the main query after the CTE definitions.
// PostgreSQL allows data-modifying CTEs; we reject them unconditionally.
func containsCTEWrite(normalized string) bool {
	upper := strings.ToUpper(normalized)
	asIdx := strings.Index(upper, " AS ")
	if asIdx < 0 {
		return false
	}
	bodyStart := strings.IndexByte(upper[asIdx+4:], '(')
	if bodyStart < 0 {
		return false
	}
	bodyStart += asIdx + 4
	depth := 0
	writeVerbs := []string{"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "TRUNCATE "}
	var lastParenEnd int
cteLoop:
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
				return false
			}
			if depth == 0 {
				lastParenEnd = i + 1
				rest := upper[i+1:]
				trimmed := strings.TrimSpace(rest)
				if strings.HasPrefix(trimmed, ",") {
					nextAS := strings.Index(trimmed, " AS ")
					if nextAS < 0 {
						return false
					}
					nextBody := strings.IndexByte(trimmed[nextAS+4:], '(')
					if nextBody < 0 {
						return false
					}
					i = i + 1 + nextAS + 4 + nextBody - 1
					depth = 1
				} else {
					break cteLoop
				}
			}
		default:
			if depth == 1 {
				for _, verb := range writeVerbs {
					if i+len(verb) <= len(upper) && upper[i:i+len(verb)] == verb {
						return true
					}
				}
			}
		}
	}
	// After all CTE bodies are consumed, check the main query body
	// for write operations (e.g., WITH x AS (...) INSERT INTO ...).
	if lastParenEnd > 0 && lastParenEnd < len(upper) {
		mainQuery := strings.TrimSpace(upper[lastParenEnd:])
		firstToken := firstWord(mainQuery)
		for _, op := range writeOps {
			if firstToken == op {
				return true
			}
		}
	}
	return false
}

// validateCTE checks a WITH query for write operations both inside CTE bodies
// and in the main query body. This is called before AST parsing because
// sqlast.Parse may silently consume the CTE prefix.
func validateCTE(normalized, originalSQL string) error {
	// Check if the main query after CTE is a write operation
	if containsCTEWrite(normalized) {
		return &ErrReadOnlyViolation{
			SQL:    originalSQL,
			Reason: "WITH CTE contains write operation",
		}
	}
	return nil
}

// stripExplainPrefix removes the EXPLAIN keyword and its dialect-specific
// option tokens from the head of an EXPLAIN statement, returning the inner
// statement. Supported forms:
//
//	EXPLAIN <inner>                          (generic / duckdb / hive / starrocks)
//	EXPLAIN (OPT, OPT, ...) <inner>          (postgres / gaussdb option list)
//	EXPLAIN FORMAT=JSON <inner>              (mysql)
//	EXPLAIN ANALYZE <inner>                  (mysql 8.0.18+, duckdb)
//	EXPLAIN QUERY PLAN <inner>               (sqlite)
//	EXPLAIN PLAN <inner>                     (clickhouse)
//	EXPLAIN PLAN FOR <inner>                 (oracle)
//	EXPLAIN VERBOSE <inner>                  (postgres)
//
// The returned string is strictly shorter than the input (at least the
// EXPLAIN token is consumed), which bounds any recursive validation.
// Malformed input (e.g. unbalanced parentheses) yields an empty string so
// callers reject it (fail-closed).
func stripExplainPrefix(sql string) string {
	s := strings.TrimSpace(sql)
	upper := strings.ToUpper(s)

	// Consume the EXPLAIN keyword, requiring a word boundary so identifiers
	// like "EXPLAINABLE" are not matched.
	if !strings.HasPrefix(upper, "EXPLAIN") {
		return s
	}
	i := len("EXPLAIN")
	if i < len(upper) && !isSpace(upper[i]) {
		return s
	}
	i = skipSpaces(upper, i)
	// Skip comments between EXPLAIN and its options/inner statement
	// ("EXPLAIN /*+ hint */ SELECT ...", "EXPLAIN -- note\nSELECT ...").
	i = skipSQLComments(upper, i)

	// Skip an optional parenthesized option list (postgres / gaussdb),
	// tracking quotes so string literals inside options don't disturb the
	// parenthesis balance.
	if i < len(upper) && upper[i] == '(' {
		depth := 0
		j := i
		var quote byte
		balanced := false
	scanParens:
		for ; j < len(upper); j++ {
			c := upper[j]
			if quote != 0 {
				if c == quote {
					quote = 0
				}
				continue
			}
			if c == '/' && j+1 < len(upper) && upper[j+1] == '*' {
				end := strings.Index(upper[j+2:], "*/")
				if end < 0 {
					return "" // unterminated block comment — fail-closed
				}
				j = j + 2 + end + 1 // +1 so the loop's j++ lands after "*/"
				continue
			}
			switch c {
			case '\'', '"':
				quote = c
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					j++
					balanced = true
					break scanParens
				}
			}
		}
		if !balanced {
			return ""
		}
		i = j
	}
	i = skipSpaces(upper, i)
	i = skipSQLComments(upper, i)

	// Consume known EXPLAIN option tokens. The inner statement's first word
	// (SELECT / WITH / SHOW / DESCRIBE / DESC / PRAGMA / CHECK) is never in
	// this set, so the loop stops exactly at the inner statement.
	modifiers := map[string]bool{
		"ANALYZE": true, "VERBOSE": true, "FORMAT": true,
		"QUERY": true, "PLAN": true, "FOR": true,
	}
	for {
		i = skipSQLComments(upper, i)
		tok, rest := nextToken(upper, i)
		if tok == "" || !modifiers[tok] {
			break
		}
		i = rest
		if tok == "FORMAT" {
			// Consume an optional "=" and the format value (JSON/TEXT/YAML/VERBOSE).
			i = skipSpaces(upper, i)
			if i < len(upper) && upper[i] == '=' {
				i++
				i = skipSpaces(upper, i)
			}
			if _, after := nextToken(upper, i); after > i {
				i = after
			}
		}
		i = skipSpaces(upper, i)
	}
	return strings.TrimSpace(s[i:])
}

// isSpace reports whether c is a whitespace byte.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// skipSpaces advances i past any whitespace bytes.
func skipSpaces(s string, i int) int {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}

// skipSQLComments advances i past any leading SQL comments — block comments
// (/* ... */, including optimizer hints /*+ ... */) and -- line comments.
// Returns the index of the first non-comment character, or len(s) when the
// rest of the input is comments (an unterminated block comment included).
func skipSQLComments(s string, i int) int {
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return len(s)
			}
			i = i + 2 + end + 2
			continue
		}
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			nl := strings.IndexByte(s[i:], '\n')
			if nl < 0 {
				return len(s)
			}
			i = i + nl + 1
			continue
		}
		return i
	}
	return i
}

// nextToken returns the next token starting at i, delimited by whitespace or
// '=' (so FORMAT=JSON tokenizes as FORMAT + JSON, letting the caller consume
// the '=' separately). Returns ("", i) when no token remains.
func nextToken(s string, i int) (string, int) {
	i = skipSpaces(s, i)
	start := i
	for i < len(s) && !isSpace(s[i]) && s[i] != '=' {
		i++
	}
	if i == start {
		return "", start
	}
	return s[start:i], i
}

// isSelectInto checks if a SELECT has an INTO clause targeting a table
// (not a MySQL variable like INTO @var).
func isSelectInto(normalized string) bool {
	upper := strings.ToUpper(normalized)
	for i := 0; i+4 <= len(upper); i++ {
		if upper[i:i+4] != "INTO" {
			continue
		}
		if i > 0 && !isWordBoundaryBefore(upper, i) {
			continue
		}
		if i+4 < len(upper) && !isWordBoundaryAfter(upper, i+4) {
			continue
		}
		after := strings.TrimSpace(upper[i+4:])
		if strings.HasPrefix(after, "@") || strings.HasPrefix(after, ":") {
			return false
		}
		return true
	}
	return false
}

func isWordBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	return s[i-1] == ' ' || s[i-1] == '\t' || s[i-1] == '\n' || s[i-1] == '\r'
}

func isWordBoundaryAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	return s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r'
}

// AutoLimit appends "LIMIT n" to a SELECT query if no LIMIT clause
// is already present.
func AutoLimit(sql string, maxRows int) string {
	sql = strings.TrimSpace(sql)

	// Try AST parse first — if it succeeds with a bound SelectStmt that has
	// no LIMIT, we know it's safe to append one.
	if stmt, err := sqlast.Parse(sql); err == nil {
		switch s := stmt.(type) {
		case *sqlast.SelectStmt:
			if s.Limit > 0 {
				return sql // already has LIMIT
			}
			// Check LIMIT( compact syntax that AST parser doesn't recognize
			if hasOuterLimit(strings.ToUpper(sql)) {
				return sql
			}
		case *sqlast.UnionStmt:
			if s.Left != nil && s.Left.Limit > 0 {
				return sql
			}
		default:
			return sql // not a SELECT — don't add LIMIT
		}
		// Strip trailing semicolon before appending LIMIT
		clean := strings.TrimRight(sql, "; \t\r\n")
		return fmt.Sprintf("%s LIMIT %d", clean, maxRows)
	}

	// AST parse failed — fall back to string-based detection
	// for non-standard SQL (EXPLAIN, WITH, etc.).
	upper := strings.ToUpper(sql)
	firstToken := strings.ToUpper(firstWord(sql))

	if firstToken != "SELECT" && firstToken != "WITH" && firstToken != "EXPLAIN" {
		return sql
	}

	if hasOuterLimit(upper) {
		return sql
	}

	sql = strings.TrimSuffix(sql, ";")
	sql = strings.TrimSpace(sql)

	return fmt.Sprintf("%s LIMIT %d", sql, maxRows)
}

// hasOuterLimit checks if a LIMIT clause exists at the top level of a query,
// ignoring LIMITs inside parenthesized subqueries.
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
	// Standard LIMIT followed by space/tab/newline
	if strings.Contains(outer, "LIMIT ") ||
		strings.Contains(outer, "LIMIT\t") ||
		strings.Contains(outer, "LIMIT\n") ||
		strings.Contains(outer, "LIMIT\r") {
		return true
	}
	// LIMIT( compact syntax: the '(' increments depth so LIMIT(
	// is not preserved in stripped output — check if stripped ends with LIMIT
	trimmed := strings.TrimSpace(outer)
	if strings.HasSuffix(trimmed, "LIMIT") {
		return true
	}
	return false
}

// splitStatements does a basic count of statements by splitting on semicolons.
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
