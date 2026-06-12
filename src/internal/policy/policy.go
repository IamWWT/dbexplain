// Package policy provides fine-grained access control for dbexplain execute.
// It supports three levels of deny policies:
//   - Statement-level: block queries matching specific patterns
//   - Table-level: block queries referencing denied tables/collections/metrics
//   - Column-level: block queries selecting denied columns/metric labels
//
// Policies apply to ALL database types:
//   - SQL databases (mysql, postgres, gaussdb, sqlite, clickhouse): all 3 levels
//   - Elasticsearch: all 3 levels (ES uses _sql endpoint)
//   - Prometheus: statement + table (metric) + column (label) levels via PromQL extraction
//   - MongoDB/Qdrant: statement + table levels (extract collection from JSON)
//   - Redis: statement + key levels (extract key from read commands)
//
// Configuration via .env file:
//   Global:   DENY_TABLES=table1,metric1
//             DENY_COLUMNS=schema.table.column,table.column,label_name
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

	"github.com/IamWWT/dbexplain/internal/sqlast"
	"github.com/IamWWT/dbexplain/internal/query"
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
		if perDSNMask := loadMask(prefix + "MASK_COLUMNS"); len(perDSNMask) > 0 {
			if cfg.MaskColumns == nil {
				cfg.MaskColumns = make(map[string]string, len(perDSNMask))
			}
			for k, v := range perDSNMask {
				cfg.MaskColumns[k] = v
			}
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

	// 3. Column-level — explicit column references only.
	// SELECT * + DENY_COLUMNS is handled post-execution via StripDeniedColumns
	// (see below), allowing the query to proceed while removing denied columns
	// from the result before output.
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
// Column-level is also checked when applicable — Prometheus label names are validated
// against DenyColumns via PromQL label extraction.
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

		// Column-level: MongoDB/Qdrant 原生查询默认返回所有字段
		// 相当于 SQL 的 SELECT *。如果 DENY_COLUMNS=collection.field 匹配
		// 且查询没有投影(projection)来排除该字段，则拦截
		if len(c.DenyColumns) > 0 {
			for _, col := range collections {
				for _, denied := range c.DenyColumns {
					deniedCol, field, hasField := strings.Cut(denied, ".")
					if !hasField {
						continue
					}
					if !strings.EqualFold(col, deniedCol) {
						continue
					}
					// 检查是否有投影排除该字段
					if hasProjection(query) && fieldIsProjectedOut(query, field) {
						continue
					}
					return &ErrDenied{Level: "column", Target: denied, SQL: query}
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
	case "prometheus":
		// Extract metric name from compiled PromQL and check against DenyTables
		if metricName := extractPromQLMetricName(query); metricName != "" {
			for _, denied := range c.DenyTables {
				if strings.EqualFold(metricName, denied) {
					return &ErrDenied{Level: "table", Target: denied, SQL: query}
				}
			}
		}
		// Extract label names from PromQL and check against DenyColumns
		if len(c.DenyColumns) > 0 {
			for _, label := range extractPromQLLabels(query) {
				for _, denied := range c.DenyColumns {
					if strings.EqualFold(label, denied) {
						return &ErrDenied{Level: "column", Target: denied, SQL: query}
					}
				}
			}
		}
	case "elasticsearch":
		// ES JSON native queries: index is specified via DSN URL path, not query body.
		// Statement-level checks already applied above. Table-level DenyTables checks
		// are not applicable to _search query bodies (no index name present).
		// Column-level checks handled post-execution via ApplyMask.
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

// StripDeniedColumns removes denied columns from the query result.
// This is a post-execution transformation on QueryResult — it runs after the
// query has executed but before output formatting.
//
// Unlike CheckSQL which blocks at the column-reference level (for explicit
// SELECT col queries), StripDeniedColumns handles the SELECT * case where
// denied columns are selected implicitly. It strips entire columns from the
// result so the user never sees the data, without blocking the query.
//
// Stripped column names are recorded in result.StrippedColumns for JSON
// output visibility, and a warning is printed to stderr.
//
// Works for ALL data sources (SQL, file, NoSQL) since all produce *QueryResult.
func (c *Config) StripDeniedColumns(result *query.QueryResult) {
	if c == nil || len(c.DenyColumns) == 0 || result == nil || len(result.Columns) == 0 {
		return
	}
	// Find column indices to strip
	type stripCol struct {
		idx  int
		name string
	}
	var toStrip []stripCol
	for i, col := range result.Columns {
		for _, denied := range c.DenyColumns {
			if matchColumn(col.Name, denied) {
				toStrip = append(toStrip, stripCol{idx: i, name: col.Name})
				break
			}
		}
	}
	if len(toStrip) == 0 {
		return
	}

	// Build set of indices for O(1) lookup
	stripIdx := make(map[int]bool)
	strippedNames := make([]string, 0, len(toStrip))
	for _, s := range toStrip {
		stripIdx[s.idx] = true
		strippedNames = append(strippedNames, s.name)
	}

	// Record stripped columns in result
	result.StrippedColumns = strippedNames

	// Print warning to stderr
	fmt.Fprintf(os.Stderr, "WARNING: columns %v are denied by policy and have been stripped from the result\n", strippedNames)

	// Filter columns
	newCols := make([]query.ColumnInfo, 0, len(result.Columns)-len(stripIdx))
	for i, col := range result.Columns {
		if !stripIdx[i] {
			newCols = append(newCols, col)
		}
	}

	// Filter each row's values
	for ri, row := range result.Rows {
		newRow := make([]*string, 0, len(row)-len(stripIdx))
		for ci, val := range row {
			if !stripIdx[ci] {
				newRow = append(newRow, val)
			}
		}
		result.Rows[ri] = newRow
	}

	result.Columns = newCols
}

// extractTableNames finds table names in SQL.
// Tries AST parsing first for precise extraction; falls back to regex for
// non‑standard SQL (SHOW, EXPLAIN, dialect‑specific syntax, etc.).
func extractTableNames(sql string) []string {
	// Strip comments first so AST parser sees clean SQL
	clean := stripSQLComments(sql)

	// Try AST-level extraction first
	if stmt, err := sqlast.Parse(clean); err == nil {
		return extractTablesFromAST(stmt)
	}

	// Fall back to regex-based extraction
	sql = normalizeWhitespace(clean)
	sql = normalizeIdentifiers(sql)
	re := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|UPDATE|INTO|TABLE)\s+(\w+(?:\.\w+)?)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	seen := make(map[string]bool)
	var tables []string
	for _, m := range matches {
		full := m[1]
		if !seen[full] {
			seen[full] = true
			tables = append(tables, full)
		}
		if schema, table, ok := strings.Cut(full, "."); ok {
			if !seen[table] {
				seen[table] = true
				tables = append(tables, table)
			}
			if !seen[schema] {
				seen[schema] = true
				tables = append(tables, schema)
			}
		}
	}
	return tables
}

// extractTablesFromAST collects table names from a parsed AST statement.
func extractTablesFromAST(stmt sqlast.Stmt) []string {
	seen := make(map[string]bool)
	var tables []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			tables = append(tables, name)
		}
		// Also add individual parts of schema-qualified names (schema.table → schema + table)
		if schema, table, ok := strings.Cut(name, "."); ok {
			if !seen[table] {
				seen[table] = true
				tables = append(tables, table)
			}
			if !seen[schema] {
				seen[schema] = true
				tables = append(tables, schema)
			}
		}
	}

	switch s := stmt.(type) {
	case *sqlast.SelectStmt:
		add(s.From)
		for _, j := range s.Joins {
			add(j.Table)
		}
	case *sqlast.UnionStmt:
		if s.Left != nil {
			tables = append(tables, extractTablesFromAST(s.Left)...)
		}
		if s.Right != nil {
			for _, t := range extractTablesFromAST(s.Right) {
				add(t)
			}
		}
	}
	return tables
}

// extractColumnRefs finds table-qualified column references in SQL.
// Tries AST parsing first for precise extraction; falls back to regex for
// non‑standard SQL.
func extractColumnRefs(sql string) []string {
	// Try AST-level extraction first
	if stmt, err := sqlast.Parse(sql); err == nil {
		return extractColRefsFromAST(stmt)
	}

	// Fall back to regex-based extraction
	sql = stripSQLComments(sql)
	sql = normalizeWhitespace(sql)
	sql = normalizeIdentifiers(sql)
	re := regexp.MustCompile(`\w+(?:\.\w+)+`)
	matches := re.FindAllString(sql, -1)
	seen := make(map[string]bool)
	var refs []string
	for _, m := range matches {
		parts := strings.Split(m, ".")
		if isSQLKeyword(parts[0]) || isNumeric(parts[len(parts)-1]) {
			continue
		}
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

// extractColRefsFromAST collects column references from a parsed AST statement.
func extractColRefsFromAST(stmt sqlast.Stmt) []string {
	var refs []string
	seen := make(map[string]bool)
	add := func(table, col string) {
		key := col
		if table != "" {
			key = table + "." + col
		}
		if !seen[key] {
			seen[key] = true
			refs = append(refs, key)
		}
	}

	walkExpr := func(expr sqlast.Expr) {
		if expr == nil {
			return
		}
		collectColRefs(expr, add)
	}

	switch s := stmt.(type) {
	case *sqlast.SelectStmt:
		// SELECT list
		for _, col := range s.Columns {
			walkExpr(col.Expr)
		}
		// WHERE
		walkExpr(s.Where)
		// JOIN ON conditions
		for _, j := range s.Joins {
			walkExpr(j.On)
		}
		// GROUP BY columns
		for _, gb := range s.GroupBy {
			add(gb.Table, gb.Col)
		}
		// ORDER BY columns
		for _, ob := range s.OrderBy {
			add(ob.Expr.Table, ob.Expr.Col)
		}
		// HAVING
		walkExpr(s.Having)
	case *sqlast.UnionStmt:
		if s.Left != nil {
			refs = append(refs, extractColRefsFromAST(s.Left)...)
		}
		if s.Right != nil {
			for _, r := range extractColRefsFromAST(s.Right) {
				if !seen[r] {
					seen[r] = true
					refs = append(refs, r)
				}
			}
		}
	}
	return refs
}

// collectColRefs traverses an expression tree and records all ColumnRef nodes.
func collectColRefs(expr sqlast.Expr, add func(table, col string)) {
	switch e := expr.(type) {
	case *sqlast.ColumnRef:
		add(e.Table, e.Col)
	case *sqlast.BinaryExpr:
		collectColRefs(e.Left, add)
		collectColRefs(e.Right, add)
	case *sqlast.UnaryExpr:
		collectColRefs(e.Right, add)
	case *sqlast.FuncCall:
		for _, arg := range e.Args {
			collectColRefs(arg, add)
		}
	case *sqlast.BetweenExpr:
		collectColRefs(e.Expr, add)
		collectColRefs(e.Low, add)
		collectColRefs(e.High, add)
	case *sqlast.SubqueryExpr:
		if e.Stmt != nil {
			subRefs := extractColRefsFromAST(e.Stmt)
			for _, r := range subRefs {
				add("", r) // record without splitting
			}
		}
	}
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

// hasProjection checks if a MongoDB JSON query includes a "projection" field.
func hasProjection(query string) bool {
	return regexp.MustCompile(`"projection"\s*:`).MatchString(query)
}

// fieldIsProjectedOut checks if a field is explicitly excluded (set to 0 or false)
// in a MongoDB projection. Returns false if the field is included (set to 1 or true)
// or if projection state is ambiguous.
func fieldIsProjectedOut(query string, field string) bool {
	// Match "field":0 or "field":false patterns in projection
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(field) + `"\s*:\s*(0|false)`)
	return re.MatchString(query)
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
			// Skip past the newline so text on the next line directly follows
			// the previous content. Prevents bypass where "FROM testdb.--
			// comment\niplist" leaves a newline that breaks regex-based
			// table/column extraction (\w doesn't match \n).
			if j < len(sql) {
				j++ // skip \n
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

// ── PromQL helpers ──────────────────────────────────────────────────────────

// extractPromQLMetricName extracts the metric name from a compiled PromQL query.
// Handles formats:
//
//	metric                      →  metric
//	metric{label="val"}         →  metric
//	rate(metric[5m])            →  metric
//	count(metric)               →  metric
//	sum by(job) (metric)        →  metric
//	metric > 0                  →  metric
func extractPromQLMetricName(query string) string {
	if query == "" {
		return ""
	}

	// Remove whitespace for easier matching
	normalized := strings.TrimSpace(query)

	// Pattern: function_name(metric_name[...]) or function_name(metric_name{...})
	// Walk backwards from the first '{' or end-of-string to find the metric name.
	braceIdx := strings.Index(normalized, "{")
	parenIdx := strings.Index(normalized, "(")

	// No braces or parens — the entire query may be just the metric name,
	// or a binary expression like "metric > 0"
	if braceIdx < 0 && parenIdx < 0 {
		// Take the first word
		fields := strings.Fields(normalized)
		if len(fields) > 0 {
			name := fields[0]
			if !isPromQLFunc(name) {
				return name
			}
		}
		return ""
	}

	// Has label matchers: find the identifier just before '{'
	if braceIdx >= 0 {
		preBrace := strings.TrimSpace(normalized[:braceIdx])
		// Handle nested function: count(metric{...})
		if lastParen := strings.LastIndex(preBrace, "("); lastParen >= 0 {
			preBrace = strings.TrimSpace(preBrace[lastParen+1:])
		}
		fields := strings.Fields(preBrace)
		if len(fields) > 0 {
			name := fields[len(fields)-1]
			if !isPromQLFunc(name) {
				return name
			}
		}
		return ""
	}

	// Has parentheses but no braces: function(metric) or function by(x) (metric)
	// Check if the first word before '(' is a PromQL function
	// (handles modifiers like by/without/on: "sum by(job) (up)")
	preFields := strings.Fields(normalized[:parenIdx])
	if len(preFields) > 0 && isPromQLFunc(preFields[0]) {
		// Find the LAST '(' which contains the actual vector expression
		// Handles: count(up), rate(up[5m]), sum by(job) (up)
		lastParen := strings.LastIndex(normalized, "(")
		if lastParen < 0 {
			return ""
		}
		inner := normalized[lastParen+1:]
		if closeIdx := strings.LastIndex(inner, ")"); closeIdx >= 0 {
			inner = inner[:closeIdx]
		}
		// Remove range vector [5m] if present
		if rangeIdx := strings.Index(inner, "["); rangeIdx >= 0 {
			inner = strings.TrimSpace(inner[:rangeIdx])
		}
		inner = strings.TrimSpace(inner)
		if !isPromQLFunc(inner) {
			return inner
		}
	}

	return ""
}

// extractPromQLLabels extracts label names from PromQL matcher {key="val"} blocks.
func extractPromQLLabels(query string) []string {
	braceIdx := strings.Index(query, "{")
	closeBrace := strings.LastIndex(query, "}")
	if braceIdx < 0 || closeBrace <= braceIdx {
		return nil
	}
	inner := query[braceIdx+1 : closeBrace]
	// Split by commas, but not inside quotes
	var labels []string
	seen := make(map[string]bool)
	i := 0
	for i < len(inner) {
		// Skip whitespace
		for i < len(inner) && inner[i] == ' ' {
			i++
		}
		// Read label name (word characters before =, !=, =~, !~)
		start := i
		for i < len(inner) && inner[i] != '=' && inner[i] != '!' && inner[i] != '~' {
			i++
		}
		name := strings.TrimSpace(inner[start:i])
		if name != "" && name != "__name__" && !seen[name] {
			seen[name] = true
			labels = append(labels, name)
		}
		// Skip to comma or end
		for i < len(inner) && inner[i] != ',' {
			i++
		}
		i++ // skip comma
	}
	return labels
}

// isPromQLFunc returns true if name is a PromQL built-in function.
func isPromQLFunc(name string) bool {
	switch strings.ToUpper(name) {
	case "ABS", "ABSENT", "AVG", "CEIL", "CHANGES",
		"CLAMP", "CLAMP_MAX", "CLAMP_MIN", "COUNT", "COUNT_VALUES",
		"DAYS_IN_MONTH", "DAY_OF_MONTH", "DAY_OF_WEEK", "DAY_OF_YEAR",
		"DELTA", "DERIV", "DROP_COMMON_LABELS",
		"EXP", "FLOOR", "HISTOGRAM_QUANTILE", "HOLT_WINTERS",
		"HOUR", "IDELTA", "INCREASE", "IRATE", "LABEL_JOIN",
		"LABEL_REPLACE", "LAST_OVER_TIME", "LN", "LOG10", "LOG2",
		"MAX", "MIN", "MINUTE", "MONTH", "PREDICT_LINEAR",
		"QUANTILE", "RATE", "RESETS", "ROUND", "SCALAR",
		"SGN", "SIN", "SORT", "SORT_DESC", "SQRT", "SUM",
		"TIME", "TIMESTAMP", "VECTOR", "YEAR",
		"AVG_OVER_TIME", "COUNT_OVER_TIME", "MAX_OVER_TIME", "MIN_OVER_TIME",
		"SUM_OVER_TIME", "STDDEV_OVER_TIME", "STDVAR_OVER_TIME",
		"PRESENT_OVER_TIME", "QUANTILE_OVER_TIME":
		return true
	}
	return false
}

