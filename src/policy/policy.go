// Package policy provides fine-grained access control for dbexplain execute.
// It supports three levels of deny policies:
//   - Statement-level: block queries matching specific patterns
//   - Table-level: block queries referencing denied tables/collections
//   - Column-level: block queries selecting denied columns (SQL only)
//
// Policies apply to ALL database types:
//   - SQL databases (mysql, postgres, gaussdb, sqlite, clickhouse): all 3 levels
//   - Elasticsearch: all 3 levels (ES uses _sql endpoint)
//   - MongoDB/Qdrant: statement + table levels (extract collection from JSON)
//   - Redis: statement + key levels (extract key from read commands)
//
// Configuration via .env file:
//   Global:   DENY_TABLES=table1,table2
//             DENY_COLUMNS=schema.table.column,table.column
//             DENY_STATEMENTS=DROP TABLE,ALTER TABLE
//             MASK_COLUMNS=password_hash=***,card_number=****
//   Per-DSN:  DB1_DENY_TABLES=sensitive_table
//             DB2_DENY_COLUMNS=users.email
//             DB1_MASK_COLUMNS=email=REDACTED
package policy

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IamWWT/dbexplain/query"
)

// Config holds deny policies and column masks for a single DSN.
// Empty slices/maps mean no restriction at that level.
type Config struct {
	DenyTables     []string
	DenyColumns    []string
	DenyStatements []string
	MaskColumns    map[string]string // column → replacement (post-execution masking)
}

// Load reads policies and masks from environment variables, merging global and per-DSN.
// envKey is the DSN's environment key (e.g. "DB1", "DB2").
// If envKey is empty (DSN from -dsn flag or -config file), only global policies apply.
func Load(envKey string) *Config {
	cfg := &Config{
		DenyTables:     loadCSV("DENY_TABLES"),
		DenyColumns:    loadCSV("DENY_COLUMNS"),
		DenyStatements: loadCSV("DENY_STATEMENTS"),
		MaskColumns:    loadMask("MASK_COLUMNS"),
	}

	if envKey != "" {
		prefix := envKey + "_"
		cfg.DenyTables = append(cfg.DenyTables, loadCSV(prefix+"DENY_TABLES")...)
		cfg.DenyColumns = append(cfg.DenyColumns, loadCSV(prefix+"DENY_COLUMNS")...)
		cfg.DenyStatements = append(cfg.DenyStatements, loadCSV(prefix+"DENY_STATEMENTS")...)
		for k, v := range loadMask(prefix + "MASK_COLUMNS") {
			cfg.MaskColumns[k] = v
		}
	}

	return cfg
}

// CheckSQL validates a SQL query against all three levels of policy.
// Returns nil if the query passes all checks.
func (c *Config) CheckSQL(sql string) error {
	if c == nil {
		return nil
	}

	// 1. Statement-level (fastest)
	normalizedSQL := normalizeWhitespace(sql)
	for _, pattern := range c.DenyStatements {
		if strings.Contains(strings.ToUpper(normalizedSQL), strings.ToUpper(pattern)) {
			return &ErrDenied{Level: "statement", Target: pattern, SQL: sql}
		}
	}

	// 2. Table-level
	if len(c.DenyTables) > 0 {
		tables := extractTableNames(sql)
		for _, t := range tables {
			for _, denied := range c.DenyTables {
				if strings.EqualFold(t, denied) {
					return &ErrDenied{Level: "table", Target: denied, SQL: sql}
				}
			}
		}
	}

	// 3. Column-level
	if len(c.DenyColumns) > 0 {
		refs := extractColumnRefs(sql)
		for _, ref := range refs {
			for _, denied := range c.DenyColumns {
				if strings.EqualFold(ref, denied) {
					return &ErrDenied{Level: "column", Target: denied, SQL: sql}
				}
			}
		}
	}

	return nil
}

// CheckNative validates a native (non-SQL) query against policies.
// Supports statement-level (all native DBs) and table-level (MongoDB/Qdrant JSON + Redis keys).
// Column-level is skipped for native queries.
func (c *Config) CheckNative(query string, kind string) error {
	if c == nil {
		return nil
	}

	// 1. Statement-level (applies to ALL native DBs)
	normalizedQuery := normalizeWhitespace(query)
	for _, pattern := range c.DenyStatements {
		if strings.Contains(strings.ToUpper(normalizedQuery), strings.ToUpper(pattern)) {
			return &ErrDenied{Level: "statement", Target: pattern, SQL: query}
		}
	}

	// 2. Table/key-level
	if len(c.DenyTables) == 0 {
		return nil
	}

	switch kind {
	case "mongodb", "qdrant":
		collections := extractJSONCollectionNames(query)
		for _, col := range collections {
			for _, denied := range c.DenyTables {
				if strings.EqualFold(col, denied) {
					return &ErrDenied{Level: "table", Target: denied, SQL: query}
				}
			}
		}
	case "redis":
		keys := extractRedisKeys(query)
		for _, k := range keys {
			for _, denied := range c.DenyTables {
				matched, gErr := globMatch(denied, k)
			if gErr != nil {
				log.Printf("WARN: malformed DENY_TABLES glob pattern %q: %v", denied, gErr)
			}
			if matched || strings.EqualFold(k, denied) {
					return &ErrDenied{Level: "table", Target: denied, SQL: query}
				}
			}
		}
	}

	return nil
}

// ErrDenied is returned when a query violates a deny policy.
type ErrDenied struct {
	Level  string // "table", "column", or "statement"
	Target string // the denied table name, column ref, or statement pattern
	SQL    string // the original query
}

func (e *ErrDenied) Error() string {
	switch e.Level {
	case "table":
		return fmt.Sprintf("ACCESS_DENIED: table %q is not allowed for query", e.Target)
	case "column":
		return fmt.Sprintf("ACCESS_DENIED: column %q is not allowed for query", e.Target)
	default:
		return fmt.Sprintf("ACCESS_DENIED: query matches denied statement pattern %q", e.Target)
	}
}

// loadCSV reads an env var and splits by comma.
func loadCSV(key string) []string {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// loadMask reads an env var with format "col=replacement,col2=replacement2".
// Each entry is split on the first '='. Returns nil if env var is empty.
func loadMask(key string) map[string]string {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}
	masks := make(map[string]string)
	pairs := strings.Split(val, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eqIdx := strings.Index(pair, "=")
		if eqIdx < 0 {
			continue // malformed entry, skip
		}
		col := strings.TrimSpace(pair[:eqIdx])
		repl := strings.TrimSpace(pair[eqIdx+1:])
		if col == "" {
			continue
		}
		masks[col] = repl
	}
	if len(masks) == 0 {
		return nil
	}
	return masks
}

// matchColumn checks if a column name matches a mask pattern.
// The pattern may include a table prefix (e.g. "users.password_hash"),
// which is stripped before matching against the bare column name.
// Supports filepath.Match glob wildcards (*, ?) and is case-insensitive.
func matchColumn(colName, pattern string) bool {
	p := pattern
	if idx := strings.LastIndex(p, "."); idx >= 0 {
		p = p[idx+1:] // strip table prefix
	}
	if strings.EqualFold(colName, p) {
		return true
	}
	// Glob match: lowercase both to match case-insensitively
	if matched, mErr := filepath.Match(strings.ToLower(p), strings.ToLower(colName)); matched {
		return true
	} else if mErr != nil {
		log.Printf("WARN: malformed MASK_COLUMNS glob pattern %q: %v", pattern, mErr)
	}
	return false
}

// ApplyMask replaces values in matching columns with configured replacement text.
// This is a post-execution transformation on QueryResult — it runs after the
// query has executed but before output formatting. NULL cells are left untouched.
// Non-SQL database results (MongoDB, Redis, Qdrant) are masked identically
// since they all produce *QueryResult with Columns + Rows.
func (c *Config) ApplyMask(result *query.QueryResult) {
	if c == nil || len(c.MaskColumns) == 0 || result == nil || len(result.Columns) == 0 {
		return
	}
	for i, col := range result.Columns {
		for pattern, replacement := range c.MaskColumns {
			if matchColumn(col.Name, pattern) {
				for _, row := range result.Rows {
					if row[i] != nil {
						val := replacement
						row[i] = &val
					}
				}
				break // column matched, move to next
			}
		}
	}
}

// extractTableNames finds table names in SQL: after FROM, JOIN, UPDATE, INTO, TABLE.
// Handles quoted identifiers (backtick, double-quote, bracket) and SQL comments
// by normalizing the SQL before extraction.
func extractTableNames(sql string) []string {
	sql = stripSQLComments(sql)
	sql = normalizeIdentifiers(sql)
	re := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|UPDATE|INTO|TABLE)\s+(?:\w+\.)?(\w+)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	seen := make(map[string]bool)
	var tables []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			tables = append(tables, name)
		}
	}
	return tables
}

// extractColumnRefs finds table.column and schema.table.column references in SQL.
// For multi-level dotted names (e.g. schema.table.column), it returns both the
// full path and the last 2 parts so DENY_COLUMNS=table.column also blocks
// schema.table.column references.
// Handles quoted identifiers and SQL comments by normalizing the SQL first.
func extractColumnRefs(sql string) []string {
	sql = stripSQLComments(sql)
	sql = normalizeIdentifiers(sql)
	// Match any contiguous word-dot-word sequence (any depth)
	re := regexp.MustCompile(`\w+(?:\.\w+)+`)
	matches := re.FindAllString(sql, -1)
	seen := make(map[string]bool)
	var refs []string
	for _, m := range matches {
		parts := strings.Split(m, ".")
		// Skip if first part is SQL keyword or last part is numeric
		if isSQLKeyword(parts[0]) || isNumeric(parts[len(parts)-1]) {
			continue
		}
		// For multi-level (schema.table.column), also add the last 2-part
		// so DENY_COLUMNS=table.column matches schema.table.column in SQL
		if len(parts) >= 3 {
			twoPart := parts[len(parts)-2] + "." + parts[len(parts)-1]
			if !seen[twoPart] {
				seen[twoPart] = true
				refs = append(refs, twoPart)
			}
		}
		if !seen[m] {
			seen[m] = true
			refs = append(refs, m)
		}
	}
	return refs
}

// extractJSONCollectionNames extracts collection/table names from
// MongoDB/Qdrant JSON query formats: "find":"name", "aggregate":"name",
// "scroll":"name", "count":"name".
func extractJSONCollectionNames(query string) []string {
	re := regexp.MustCompile(`"(?:find|aggregate|scroll|count)"\s*:\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(query, -1)
	seen := make(map[string]bool)
	var names []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// extractRedisKeys extracts the first key argument from Redis read commands.
// Most Redis read commands have the key as the first argument after the command name.
// Commands like SCAN, PING, ECHO have no key argument and return empty.
func extractRedisKeys(query string) []string {
	parts := strings.Fields(query)
	if len(parts) < 2 {
		return nil
	}
	cmd := strings.ToUpper(parts[0])

	// Commands that do NOT use a key as the first argument
	switch cmd {
	case "PING", "ECHO", "SCAN", "HSCAN", "SSCAN", "ZSCAN":
		return nil
	}

	key := parts[1]
	if key == "" {
		return nil
	}
	return []string{key}
}

// isNumeric checks if a string is all digits.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// normalizeWhitespace collapses all whitespace sequences (spaces, tabs, newlines, CR)
// to single spaces. Used for statement-level pattern matching to prevent bypass via
// whitespace variation (e.g. "DROP  TABLE" matching "DROP TABLE").
func normalizeWhitespace(s string) string {
	var result strings.Builder
	inSpace := false
	for _, ch := range s {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if !inSpace {
				result.WriteByte(' ')
				inSpace = true
			}
		} else {
			result.WriteRune(ch)
			inSpace = false
		}
	}
	return strings.TrimSpace(result.String())
}

// globMatch checks if name matches a glob pattern, supporting * and ? wildcards.
// Unlike filepath.Match, it does NOT treat any character as a path separator.
// This is needed for Redis key matching where keys may contain '/'.
func globMatch(pattern, name string) (bool, error) {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '?':
			if len(name) == 0 {
				return false, nil
			}
			name = name[1:]
			pattern = pattern[1:]
		case '*':
			pattern = pattern[1:]
			// * matches any sequence, including empty; try all positions
			for i := 0; i <= len(name); i++ {
				if matched, _ := globMatch(pattern, name[i:]); matched {
					return true, nil
				}
			}
			return false, nil
		case '\\':
			if len(pattern) < 2 {
				return false, nil // trailing backslash is literal
			}
			if len(name) == 0 || name[0] != pattern[1] {
				return false, nil
			}
			name = name[1:]
			pattern = pattern[2:]
		default:
			if len(name) == 0 || name[0] != pattern[0] {
				return false, nil
			}
			name = name[1:]
			pattern = pattern[1:]
		}
	}
	return len(name) == 0, nil
}

// stripSQLComments removes SQL comments (-- single-line and /* */ multi-line)
// from a SQL string to prevent comment-based bypass of table/column extraction.
func stripSQLComments(sql string) string {
	var result strings.Builder
	i := 0
	for i < len(sql) {
		// Single-line comment: --
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			j := i + 2
			for j < len(sql) && sql[j] != '\n' {
				j++
			}
			i = j
			continue
		}
		// Multi-line comment: /* */
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			j := i + 2
			for j+1 < len(sql) && !(sql[j] == '*' && sql[j+1] == '/') {
				j++
			}
			if j+1 < len(sql) {
				j += 2 // skip past */
			} else {
				j = len(sql)
			}
			i = j
			continue
		}
		result.WriteByte(sql[i])
		i++
	}
	return result.String()
}

// normalizeIdentifiers replaces quoted SQL identifiers with their unquoted content.
// Handles three quoting styles:
//   - Backtick: `identifier` (MySQL)
//   - Double-quote: "identifier" (PostgreSQL, SQLite)
//   - Bracket: [identifier] (SQL Server/T-SQL)
//
// This prevents bypass of table/column extraction via quoted identifiers.
func normalizeIdentifiers(sql string) string {
	var result strings.Builder
	i := 0
	for i < len(sql) {
		switch sql[i] {
		case '`':
			// Backtick-quoted identifier
			j := i + 1
			for j < len(sql) && sql[j] != '`' {
				j++
			}
			if j < len(sql) {
				result.WriteString(sql[i+1 : j])
				i = j + 1
			} else {
				result.WriteByte(sql[i])
				i++
			}
		case '"':
			// Double-quote-quoted identifier (or string literal in MySQL mode).
			// Conservative: treat all double-quoted tokens as identifiers for extraction.
			j := i + 1
			for j < len(sql) && sql[j] != '"' {
				j++
			}
			if j < len(sql) {
				result.WriteString(sql[i+1 : j])
				i = j + 1
			} else {
				result.WriteByte(sql[i])
				i++
			}
		case '[':
			// Bracket-quoted identifier (SQL Server)
			j := i + 1
			for j < len(sql) && sql[j] != ']' {
				j++
			}
			if j < len(sql) {
				result.WriteString(sql[i+1 : j])
				i = j + 1
			} else {
				result.WriteByte(sql[i])
				i++
			}
		default:
			result.WriteByte(sql[i])
			i++
		}
	}
	return result.String()
}

// isSQLKeyword returns true for common SQL keywords to filter false positives.
func isSQLKeyword(s string) bool {
	upper := strings.ToUpper(s)
	switch upper {
	case "SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "AS", "ON",
		"JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS",
		"ORDER", "GROUP", "BY", "HAVING", "LIMIT", "OFFSET",
		"INSERT", "UPDATE", "DELETE", "INTO", "VALUES", "SET",
		"CREATE", "ALTER", "DROP", "TABLE", "INDEX", "VIEW",
		"DISTINCT", "ALL", "UNION", "EXCEPT", "INTERSECT",
		"CASE", "WHEN", "THEN", "ELSE", "END",
		"TRUE", "FALSE", "NULL", "IS", "LIKE", "BETWEEN",
		"COUNT", "SUM", "AVG", "MIN", "MAX",
		"EXISTS", "ANY", "SOME", "CAST", "COALESCE", "NULLIF":
		return true
	}
	return false
}
