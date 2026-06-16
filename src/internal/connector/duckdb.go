//go:build duckdb

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("duckdb", func() Connector { return duckdbConnector{} })
}

type duckdbConnector struct{}

func (duckdbConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapSQL,
		capabilities.CapRowCount,
		capabilities.CapSampling,
	}
}

// fileReadFuncs lists DuckDB functions that read external files.
// These are restricted when allowed_path is not configured.
var fileReadFuncs = []string{"read_parquet", "read_csv_auto", "read_csv", "read_json"}

// buildDuckDBConnStr builds a DuckDB connection string from parsed DSN.
// memory mode: duckdb://:memory: → ":memory:"
// file mode:   duckdb:///path/to/db → "/path/to/db"
func buildDuckDBConnStr(d *dsn.DSN) string {
	// Parse from raw DSN to properly handle absolute paths
	after := d.Raw
	if i := strings.Index(after, "://"); i >= 0 {
		after = after[i+3:]
	} else {
		return ":memory:"
	}

	// Strip query parameters
	if i := strings.Index(after, "?"); i >= 0 {
		after = after[:i]
	}

	if after == ":memory:" || after == "" || after == "/:memory:" {
		return ":memory:"
	}

	after, _ = url.PathUnescape(after)
	// Windows: duckdb:///C:/path → /C:/path → C:/path
	if runtime.GOOS == "windows" && len(after) >= 3 &&
		after[0] == '/' && after[2] == ':' {
		after = after[1:]
	}
	return after
}

func (duckdbConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildDuckDBConnStr(d)
	db, err := sql.Open("duckdb", connStr)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "duckdb", Label: d.Label}
	database := &schema.Database{Name: "main"}

	// Build table filter clause
	tfClause := ""
	var tfArgs []any
	if names := GetTableFilter(ctx); len(names) > 0 {
		phs := make([]string, len(names))
		for i, n := range names {
			phs[i] = "?"
			tfArgs = append(tfArgs, n)
		}
		tfClause = " AND table_name IN (" + strings.Join(phs, ",") + ")"
	}

	// Enumerate all user tables/views via information_schema
	// DuckDB has no multi-database concept, single "main" schema
	Logf(ctx, "[duckdb] [collect] %s", "SELECT table_name, table_type FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'pg_catalog', 'temp')"+tfClause+" ORDER BY table_name")
	rows, err := db.QueryContext(ctx, `
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema NOT IN ('information_schema', 'pg_catalog', 'temp')`+tfClause+`
		ORDER BY table_name`, tfArgs...)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "main", "", "list tables", err)
	}

	var tableNames []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n, new(string)); err == nil {
			if !strings.HasPrefix(n, "sqlite_") {
				tableNames = append(tableNames, n)
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("[duckdb] rows iteration: %v", err)
	}

	total := len(tableNames)
	for i, tn := range tableNames {
		Logf(ctx, "[duckdb] 采集表 %d/%d: %s", i+1, total, tn)
		t, err := collectDuckDBTable(ctx, db, tn, d.Redacted())
		if err != nil {
			Logf(ctx, "[duckdb] skip table %s: %v", tn, err)
			continue
		}
		database.Tables = append(database.Tables, t)
	}

	inst.Databases = append(inst.Databases, database)
	return inst, nil
}

func collectDuckDBTable(ctx context.Context, db *sql.DB, tableName, redactedDSN string) (*schema.Table, error) {
	t := &schema.Table{Name: tableName}

	// Step 1: Column info via pragma_table_info (most reliable for DuckDB)
	Logf(ctx, "[duckdb] [collect] %s", `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('%s')`)
	colRows, err := db.QueryContext(ctx, fmt.Sprintf(
		`SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('%s')`,
		strings.ReplaceAll(tableName, "'", "''")))
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var colsWithoutComment []*schema.Column
	for colRows.Next() {
		var name, colType string
		var notnullBool, pkBool bool
		var dflt sql.NullString
		if err := colRows.Scan(&name, &colType, &notnullBool, &dflt, &pkBool); err != nil {
			continue
		}
		c := &schema.Column{
			Name:      name,
			Type:      colType,
			Nullable:  !notnullBool && !pkBool,
			IsPrimary: pkBool,
		}
		if dflt.Valid {
			c.Default = dflt.String
		}
		t.Columns = append(t.Columns, c)
		if c.Comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}
	colRows.Close()
	if err := colRows.Err(); err != nil {
		log.Printf("[duckdb] rows iteration: %v", err)
	}

	if len(t.Columns) == 0 {
		return nil, fmt.Errorf("no columns found for %s", tableName)
	}

	// Step 2: Row count
	Logf(ctx, "[duckdb] [collect] %s", `SELECT COUNT(*) FROM "%s"`)
	if err := db.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT COUNT(*) FROM "%s"`,
		strings.ReplaceAll(tableName, `"`, `""`)),
	).Scan(&t.RowCount); err != nil {
		log.Printf("[duckdb] row count for %s: %v", tableName, err)
	}

	// Step 3: Sample row for comment inference
	if len(colsWithoutComment) > 0 && t.RowCount > 0 {
		if sample, err := fetchDuckDBSampleRow(ctx, db, tableName); err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			Logf(ctx, "[duckdb] sample row failed for %s: %v", tableName, err)
		}
	}

	// Step 4: Indexes via duckdb_constraints()
	Logf(ctx, "[duckdb] [collect] %s", "SELECT constraint_type, constraint_text FROM duckdb_constraints() WHERE table_name = '%s'")
	constraintRows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT constraint_type, constraint_text
		FROM duckdb_constraints()
		WHERE table_name = '%s'`,
		strings.ReplaceAll(tableName, "'", "''")))
	if err == nil {
		defer constraintRows.Close()
		for constraintRows.Next() {
			var constraintType, constraintText string
			if err := constraintRows.Scan(&constraintType, &constraintText); err != nil {
				continue
			}
			// Extract column names from constraint_text: "PRIMARY KEY(id)" → ["id"]
			// constraint_text format: "PRIMARY KEY(col1, col2)" or "UNIQUE(col1)" or "FOREIGN KEY(col1) REFERENCES ref(col2)"
			cols := extractConstraintColumns(constraintText)
			switch {
			case strings.HasPrefix(constraintType, "PRIMARY KEY"):
				for _, c := range cols {
					for _, col := range t.Columns {
						if col.Name == c {
							col.IsPrimary = true
							col.Nullable = false
						}
					}
				}
			case strings.HasPrefix(constraintType, "UNIQUE"):
				t.Indexes = append(t.Indexes, &schema.Index{
					Name:    constraintType + "_" + strings.Join(cols, "_"),
					Columns: cols,
					Unique:  true,
				})
			case strings.HasPrefix(constraintType, "FOREIGN KEY"):
				refPart := extractFKRef(constraintText)
				t.ForeignKeys = append(t.ForeignKeys, &schema.ForeignKey{
					Name:     fmt.Sprintf("%s_%s_fk", tableName, strings.Join(cols, "_")),
					Columns:  cols,
					RefTable: refPart,
				})
			}
		}
		if err := constraintRows.Err(); err != nil {
			log.Printf("[duckdb] rows iteration: %v", err)
		}
	} else {
		Logf(ctx, "[duckdb] constraints query failed for %s: %v", tableName, err)
	}

	return t, nil
}

// extractConstraintColumns extracts column names from constraint text.
// "PRIMARY KEY(id, name)" → ["id", "name"]
// "UNIQUE(score)" → ["score"]
// "FOREIGN KEY(dept_id) REFERENCES departments(id)" → ["dept_id"]
func extractConstraintColumns(text string) []string {
	start := strings.Index(text, "(")
	if start < 0 {
		return nil
	}
	end := strings.Index(text[start:], ")")
	if end < 0 {
		return nil
	}
	inner := text[start+1 : start+end]
	parts := strings.Split(inner, ",")
	var cols []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cols = append(cols, p)
		}
	}
	return cols
}

// extractFKRef extracts the referenced table from a FOREIGN KEY constraint text.
// "FOREIGN KEY(dept_id) REFERENCES departments(id)" → "departments"
func extractFKRef(text string) string {
	refIdx := strings.Index(text, "REFERENCES")
	if refIdx < 0 {
		return ""
	}
	afterRef := strings.TrimSpace(text[refIdx+len("REFERENCES"):])
	end := strings.Index(afterRef, "(")
	if end < 0 {
		return afterRef
	}
	return strings.TrimSpace(afterRef[:end])
}

func fetchDuckDBSampleRow(ctx context.Context, db *sql.DB, table string) (map[string]string, error) {
	q := fmt.Sprintf(`SELECT * FROM "%s" LIMIT 1`, strings.ReplaceAll(table, `"`, `""`))
	Logf(ctx, "[duckdb] [collect] %s", `SELECT * FROM "%s" LIMIT 1`)
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("no rows")
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}
	if err := rows.Scan(values...); err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for i, col := range columns {
		val := *(values[i].(*interface{}))
		if val == nil {
			result[col] = "NULL"
		} else if b, ok := val.([]byte); ok {
			result[col] = string(b)
		} else {
			result[col] = fmt.Sprintf("%v", val)
		}
	}
	return result, nil
}

// ExecQuery implements query.Queryable for DuckDB.
func (duckdbConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	// Validate file access if allowed_path is configured
	if err := validateFileAccess(opts); err != nil {
		return nil, err
	}

	connStr := buildDuckDBConnStr(opts.DSN)
	db, err := sql.Open("duckdb", connStr)
	if err != nil {
		return nil, fmt.Errorf("duckdb open: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	result, err := executeSQLQuery(runCtx, db, opts.SQL, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("duckdb query: %w", err)
	}
	return result, nil
}

// validateFileAccess checks if the SQL references file-reading functions
// and whether those file paths are within allowed_path boundaries.
func validateFileAccess(opts query.ExecuteOpts) error {
	allowedPath := opts.DSN.DSNParam("allowed_path")
	sqlUpper := strings.ToUpper(opts.SQL)

	// Check if SQL contains any file read function calls
	hasFileRead := false
	for _, fn := range fileReadFuncs {
		if strings.Contains(sqlUpper, strings.ToUpper(fn)) {
			hasFileRead = true
			break
		}
	}
	if !hasFileRead {
		return nil
	}

	// File read functions require allowed_path
	if allowedPath == "" {
		return fmt.Errorf("FILE_ACCESS_DENIED: %s requires allowed_path DSN parameter (e.g. ?allowed_path=/data/)",
			"read_parquet/read_csv_auto/read_json")
	}

	// Extract and validate file paths from SQL arguments
	// Simple heuristic: extract single-quoted strings after file read function calls
	allowedPaths := strings.Split(allowedPath, ",")
	for i := 0; i < len(allowedPaths); i++ {
		allowedPaths[i] = filepath.Clean(allowedPaths[i])
	}

	for _, fn := range fileReadFuncs {
		remaining := opts.SQL
		for {
			idx := strings.Index(strings.ToUpper(remaining), strings.ToUpper(fn))
			if idx < 0 {
				break
			}
			remaining = remaining[idx+len(fn):]

			// Find the opening parenthesis
			parenIdx := strings.Index(remaining, "(")
			if parenIdx < 0 {
				continue
			}
			argStr := remaining[parenIdx+1:]

			// Find the first quoted string argument
			// Check both single and double quotes
			for _, quote := range []byte{'\'', '"'} {
				quoteIdx := strings.IndexByte(argStr, quote)
				if quoteIdx < 0 {
					continue
				}
				endQuote := strings.IndexByte(argStr[quoteIdx+1:], quote)
				if endQuote < 0 {
					continue
				}
				filePath := argStr[quoteIdx+1 : quoteIdx+1+endQuote]
				cleanPath := filepath.Clean(filePath)

				// Check if path is within any allowed path
				allowed := false
				for _, ap := range allowedPaths {
					if strings.HasPrefix(cleanPath, ap) &&
						(len(cleanPath) == len(ap) || cleanPath[len(ap)] == filepath.Separator) {
						allowed = true
						break
					}
				}
				if !allowed {
					return fmt.Errorf("FILE_ACCESS_DENIED: path %q is not within allowed_path %q",
						filePath, allowedPath)
				}
			}
		}
	}

	return nil
}
