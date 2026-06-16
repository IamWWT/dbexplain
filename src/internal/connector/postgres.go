//go:build postgres || full

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("postgres", func() Connector { return postgresConnector{} })
	// gaussdb 使用独立的 gaussdbConnector（见 gaussdb.go），
	// 针对 Oracle 兼容模式 (DBCOMPATIBILITY='A'/'ORA') 做了适配。
}

type postgresConnector struct{}

func (postgresConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapForeignKey,
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapIndex,
		capabilities.CapSQL,
	}
}

func (postgresConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildPGDSN(d)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	kind := d.Kind
	if kind == "" {
		kind = "postgres"
	}
	inst := &schema.Instance{DSN: d.Redacted(), Kind: kind, Label: d.Label}

	var dbNames []string
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else {
		Logf(ctx, "[postgres] [collect] %s", `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
		rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
		if err != nil {
			// GaussDB（Oracle 兼容模式）可能没有 datistemplate 列，回退到简单查询
			Logf(ctx, "[postgres] datistemplate query failed, trying fallback: %v", err)
			Logf(ctx, "[postgres] [collect] %s", `SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname`)
			rows, err = db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname`)
			if err != nil {
				return nil, schema.NewDBError(d.Redacted(), "", "", "list databases", err)
			}
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				dbNames = append(dbNames, n)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[postgres] rows iteration: %v", err)
		}
	}

	for _, dbName := range dbNames {
		Logf(ctx, "[postgres] collecting database %s", dbName)
		database, err := collectPGDB(ctx, db, dbName, d.Redacted())
		if err != nil {
			Logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectPGDB(ctx context.Context, db *sql.DB, dbName, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}

	// 查询所有非系统 schema
	Logf(ctx, "[postgres] [collect] %s", `
		SELECT nspname FROM pg_namespace
		WHERE nspname NOT LIKE 'pg_%' AND nspname != 'information_schema'
		ORDER BY nspname`)
	schemaRows, err := db.QueryContext(ctx, `
		SELECT nspname FROM pg_namespace
		WHERE nspname NOT LIKE 'pg_%' AND nspname != 'information_schema'
		ORDER BY nspname`)
	if err != nil {
		return nil, schema.NewDBError(redactedDSN, dbName, "", "query schemas", err)
	}
	defer schemaRows.Close()

	var schemas []string
	for schemaRows.Next() {
		var s string
		if err := schemaRows.Scan(&s); err == nil {
			schemas = append(schemas, s)
		}
	}
	schemaRows.Close()
	if err := schemaRows.Err(); err != nil {
		log.Printf("[postgres] rows iteration: %v", err)
	}
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	var tables []*schema.Table
	for _, schemaName := range schemas {
		Logf(ctx, "[postgres] [collect] %s", `
			SELECT t.tablename,
			       COALESCE(s.n_live_tup, 0),
			       COALESCE(pg_total_relation_size(c.oid), 0),
			       COALESCE(obj_description(c.oid, 'pg_class'), ''),
			       COALESCE(s.seq_scan, 0),
			       COALESCE(s.idx_scan, 0),
			       COALESCE(s.n_tup_ins, 0),
			       COALESCE(s.n_tup_upd, 0),
			       COALESCE(s.n_tup_del, 0)
			FROM pg_tables t
			JOIN pg_class c ON c.relname = t.tablename
			JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.schemaname
			LEFT JOIN pg_stat_user_tables s
				ON s.schemaname = t.schemaname AND s.relname = t.tablename
			WHERE t.schemaname = $1
			ORDER BY t.tablename`)
		tRows, err := db.QueryContext(ctx, `
			SELECT t.tablename,
			       COALESCE(s.n_live_tup, 0),
			       COALESCE(pg_total_relation_size(c.oid), 0),
			       COALESCE(obj_description(c.oid, 'pg_class'), ''),
			       COALESCE(s.seq_scan, 0),
			       COALESCE(s.idx_scan, 0),
			       COALESCE(s.n_tup_ins, 0),
			       COALESCE(s.n_tup_upd, 0),
			       COALESCE(s.n_tup_del, 0)
			FROM pg_tables t
			JOIN pg_class c ON c.relname = t.tablename
			JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.schemaname
			LEFT JOIN pg_stat_user_tables s
				ON s.schemaname = t.schemaname AND s.relname = t.tablename
			WHERE t.schemaname = $1
			ORDER BY t.tablename`, schemaName)
		if err != nil {
			Logf(ctx, "[postgres] query tables in schema %s: %v", schemaName, err)
			continue
		}
		for tRows.Next() {
			t := &schema.Table{}
			var seqScan, idxScan, ntupIns, ntupUpd, ntupDel int64
			if err := tRows.Scan(&t.Name, &t.RowCount, &t.SizeBytes, &t.Comment,
				&seqScan, &idxScan, &ntupIns, &ntupUpd, &ntupDel); err != nil {
				continue
			}
			t.OpStats = &schema.OpStats{
				SeqScan: seqScan,
				IdxScan: idxScan,
				NtupIns: ntupIns,
				NtupUpd: ntupUpd,
				NtupDel: ntupDel,
			}
			// 非 public schema 的表名加上 schema 前缀，保证跨 schema 唯一
			if schemaName != "public" {
				t.Name = schemaName + "." + t.Name
			}
			tables = append(tables, t)
		}
		tRows.Close()
		if err := tRows.Err(); err != nil {
			log.Printf("[postgres] rows iteration: %v", err)
		}
	}

	total := len(tables)

	// --- Batch 1: columns (all schemas) ---
	// key: table full name (e.g., "public.users" or "schema.table")
	type pgColData struct {
		columns            []*schema.Column
		colsWithoutComment []*schema.Column
	}
	colMap := map[string]*pgColData{}

	for _, schemaName := range schemas {
		batchSQL := `
			SELECT c.relname,
			       a.attname,
			       pg_catalog.format_type(a.atttypid, a.atttypmod),
			       NOT a.attnotnull,
			       COALESCE(pg_get_expr(d.adbin, d.adrelid),''),
			       COALESCE(col_description(a.attrelid, a.attnum),''),
			       COALESCE((SELECT string_agg(contype::text,'')
			                 FROM pg_constraint c2
			                 WHERE a.attnum = ANY(c2.conkey) AND c2.conrelid = a.attrelid),'')
			FROM pg_attribute a
			LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
			JOIN pg_class c ON c.oid = a.attrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname=$1 AND a.attnum>0 AND NOT a.attisdropped
			ORDER BY c.relname, a.attnum`
		Logf(ctx, "[postgres] [collect] %s", batchSQL)
		bRows, err := db.QueryContext(ctx, batchSQL, schemaName)
		if err != nil {
			Logf(ctx, "[postgres] batch columns error for schema %s: %v", schemaName, err)
			continue
		}
		for bRows.Next() {
			var relname, attname, typ string
			var nullable bool
			var def, comment, constraints string
			if err := bRows.Scan(&relname, &attname, &typ, &nullable, &def, &comment, &constraints); err != nil {
				continue
			}
			tableKey := relname
			if schemaName != "public" {
				tableKey = schemaName + "." + relname
			}
			cd, ok := colMap[tableKey]
			if !ok {
				cd = &pgColData{}
				colMap[tableKey] = cd
			}
			c := &schema.Column{
				Name:      attname,
				Type:      typ,
				Nullable:  nullable,
				Default:   def,
				Comment:   comment,
				IsPrimary: strings.Contains(constraints, "p"),
				IsUnique:  strings.Contains(constraints, "u"),
			}
			cd.columns = append(cd.columns, c)
			if comment == "" {
				cd.colsWithoutComment = append(cd.colsWithoutComment, c)
			}
		}
		bRows.Close()
		if err := bRows.Err(); err != nil {
			log.Printf("[postgres] batch columns iteration: %v", err)
		}
	}

	// --- Batch 2: indexes (all schemas) ---
	idxMap := map[string][]*schema.Index{}
	for _, schemaName := range schemas {
		Logf(ctx, "[postgres] [collect] %s", `
			SELECT tablename, indexname, indexdef
			FROM pg_indexes WHERE schemaname=$1`)
		iRows, err := db.QueryContext(ctx, `
			SELECT tablename, indexname, indexdef
			FROM pg_indexes WHERE schemaname=$1`, schemaName)
		if err != nil {
			Logf(ctx, "[postgres] batch indexes error for schema %s: %v", schemaName, err)
			continue
		}
		for iRows.Next() {
			var tablename, indexname, indexdef string
			if err := iRows.Scan(&tablename, &indexname, &indexdef); err != nil {
				continue
			}
			tableKey := tablename
			if schemaName != "public" {
				tableKey = schemaName + "." + tablename
			}
			idx := &schema.Index{
				Name:    indexname,
				Unique:  strings.Contains(strings.ToUpper(indexdef), "UNIQUE"),
				Columns: extractIndexColumns(indexdef),
			}
			idxMap[tableKey] = append(idxMap[tableKey], idx)
		}
		iRows.Close()
		if err := iRows.Err(); err != nil {
			log.Printf("[postgres] batch indexes iteration: %v", err)
		}
	}

	// --- Batch 3: foreign keys (all schemas) ---
	fkMap := map[string][]*schema.ForeignKey{}
	for _, schemaName := range schemas {
		Logf(ctx, "[postgres] [collect] %s", `
			SELECT c1.relname AS tablename,
			       c.conname, a.attname, c2.relname, a2.attname,
			       c.confupdtype, c.confdeltype
			FROM pg_constraint c
			JOIN pg_class c1 ON c1.oid=c.conrelid
			JOIN pg_namespace n1 ON n1.oid=c1.relnamespace
			JOIN pg_class c2 ON c2.oid=c.confrelid
			JOIN pg_namespace n2 ON n2.oid=c2.relnamespace
			JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
			JOIN pg_attribute a2 ON a2.attrelid=c.confrelid AND a2.attnum=ANY(c.confkey)
			WHERE c.contype='f' AND n1.nspname=$1`)
		fRows, err := db.QueryContext(ctx, `
			SELECT c1.relname AS tablename,
			       c.conname, a.attname, c2.relname, a2.attname,
			       c.confupdtype, c.confdeltype
			FROM pg_constraint c
			JOIN pg_class c1 ON c1.oid=c.conrelid
			JOIN pg_namespace n1 ON n1.oid=c1.relnamespace
			JOIN pg_class c2 ON c2.oid=c.confrelid
			JOIN pg_namespace n2 ON n2.oid=c2.relnamespace
			JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
			JOIN pg_attribute a2 ON a2.attrelid=c.confrelid AND a2.attnum=ANY(c.confkey)
			WHERE c.contype='f' AND n1.nspname=$1`, schemaName)
		if err != nil {
			Logf(ctx, "[postgres] batch FK error for schema %s: %v", schemaName, err)
			continue
		}
		for fRows.Next() {
			var relname, name, col, refTable, refCol, onUpdateChar, onDeleteChar string
			if err := fRows.Scan(&relname, &name, &col, &refTable, &refCol, &onUpdateChar, &onDeleteChar); err != nil {
				continue
			}
			tableKey := relname
			if schemaName != "public" {
				tableKey = schemaName + "." + relname
			}
			// Group FK columns under the same constraint name
			var fk *schema.ForeignKey
			for _, existing := range fkMap[tableKey] {
				if existing.Name == name {
					fk = existing
					break
				}
			}
			if fk == nil {
				fk = &schema.ForeignKey{
					Name:     name,
					RefTable: refTable,
					OnUpdate: pgFKAction(onUpdateChar),
					OnDelete: pgFKAction(onDeleteChar),
				}
				fkMap[tableKey] = append(fkMap[tableKey], fk)
			}
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, refCol)
		}
		fRows.Close()
		if err := fRows.Err(); err != nil {
			log.Printf("[postgres] batch FK iteration: %v", err)
		}
	}

	// Assign batch results + sample rows
	for i, t := range tables {
		schemaName, baseName := parsePGTableName(t.Name)
		Logf(ctx, "[%s] 采集表 %d/%d: %s", dbName, i+1, total, t.Name)

		// Assign pre-fetched columns, indexes, FKs
		if cd := colMap[t.Name]; cd != nil {
			t.Columns = cd.columns
		}
		t.Indexes = idxMap[t.Name]
		t.ForeignKeys = fkMap[t.Name]

		// Sample row for comment inference (still per-table, controlled by --no-sample)
		if !IsNoSample(ctx) {
			if cd := colMap[t.Name]; cd != nil && len(cd.colsWithoutComment) > 0 && t.RowCount > 0 {
				sample, err := fetchPGSampleRow(ctx, db, schemaName, baseName)
				if err == nil {
					for _, c := range cd.colsWithoutComment {
						if val, ok := sample[c.Name]; ok {
							c.Comment = schema.InferComment(c.Name, c.Type, val)
						}
					}
				} else {
					Logf(ctx, "[postgres] sample row failed for %s: %v", t.Name, err)
				}
			}
		}
	}
	database.Tables = tables
	return database, nil
}

// parsePGTableName 拆解 "schema.table" 或 "table" 为 schema 和表名
func parsePGTableName(name string) (schema, table string) {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[:idx], name[idx+1:]
	}
	return "public", name
}

// quotePGIdent 为 PostgreSQL 标识符加上双引号转义
func quotePGIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func fetchPGSampleRow(ctx context.Context, db *sql.DB, schemaName, table string) (map[string]string, error) {
	query := fmt.Sprintf(`SELECT * FROM %s.%s LIMIT 1`, quotePGIdent(schemaName), quotePGIdent(table))
	Logf(ctx, "[postgres] [collect] %s", query)
	rows, err := db.QueryContext(ctx, query)
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

func buildPGDSN(d *dsn.DSN) string {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "5432"
	}
	dbname := d.DBName
	if dbname == "" {
		dbname = d.User
	}
	sslmode := d.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, d.User, d.Password, dbname, sslmode)
}

// ExecQuery implements query.Queryable for PostgreSQL and GaussDB.
func (postgresConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	connStr := buildPGDSN(opts.DSN)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Set statement timeout if specified
	if opts.Timeout > 0 {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("SET statement_timeout = '%ds'", opts.Timeout)); err != nil {
			Logf(ctx, "[postgres] set statement_timeout failed: %v (query will still run without timeout guard)", err)
		}
	}

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	result, err := executeSQLQuery(runCtx, db, opts.SQL, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("postgres query: %w", err)
	}
	return result, nil
}

// pgFKAction maps pg_constraint FK action codes to human-readable strings.
// 'a'=NO ACTION, 'r'=RESTRICT, 'c'=CASCADE, 'n'=SET NULL, 'd'=SET DEFAULT
func pgFKAction(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return code
	}
}

// extractIndexColumns extracts column names from a PostgreSQL index definition string.
// Handles:
//   - Simple columns:   CREATE INDEX i ON t (col)                    → [col]
//   - Multi-column:     CREATE INDEX i ON t (a, b)                  → [a, b]
//   - Function indexes: CREATE INDEX i ON t (lower(name))           → [lower(name)]
//   - INCLUDE:          CREATE INDEX i ON t (a) INCLUDE (b)         → [a]
func extractIndexColumns(def string) []string {
	start := strings.Index(def, "(")
	if start < 0 {
		return nil
	}
	// Find matching ')' at parenthesis depth 1 (the index column list end)
	depth := 0
	end := -1
	for i := start; i < len(def); i++ {
		switch def[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
				goto foundEnd
			}
		}
	}
foundEnd:
	if end < 0 {
		return nil
	}
	inner := def[start+1 : end]
	// Split by comma, respecting parenthesis nesting
	var cols []string
	depth = 0
	curStart := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				cols = append(cols, strings.TrimSpace(inner[curStart:i]))
				curStart = i + 1
			}
		}
	}
	if curStart < len(inner) {
		cols = append(cols, strings.TrimSpace(inner[curStart:]))
	}
	return cols
}