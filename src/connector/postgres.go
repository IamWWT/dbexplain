package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"dbexplain/capabilities"
	"dbexplain/dsn"
	"dbexplain/schema"
)

func init() {
	Register("postgres", func() Connector { return postgresConnector{} })
	Register("gaussdb", func() Connector { return postgresConnector{} })
}

type postgresConnector struct{}

func (postgresConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapForeignKey,
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapIndex,
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

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "postgres", Label: d.Label}

	var dbNames []string
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else {
		rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
		if err != nil {
			return nil, schema.NewDBError(d.Redacted(), "", "", "list databases", err)
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				dbNames = append(dbNames, n)
			}
		}
	}

	for _, dbName := range dbNames {
		logf(ctx, "[postgres] collecting database %s", dbName)
		database, err := collectPGDB(ctx, db, dbName, d.Redacted())
		if err != nil {
			logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectPGDB(ctx context.Context, db *sql.DB, dbName, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}

	// 查询所有非系统 schema
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
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	var tables []*schema.Table
	for _, schemaName := range schemas {
		tRows, err := db.QueryContext(ctx, `
			SELECT t.tablename,
			       COALESCE(s.n_live_tup, 0),
			       COALESCE(pg_total_relation_size(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename)), 0),
			       COALESCE(obj_description((quote_ident(t.schemaname) || '.' || quote_ident(t.tablename))::regclass, 'pg_class'), '')
			FROM pg_tables t
			LEFT JOIN pg_stat_user_tables s
				ON s.schemaname = t.schemaname AND s.relname = t.tablename
			WHERE t.schemaname = $1
			ORDER BY t.tablename`, schemaName)
		if err != nil {
			logf(ctx, "[postgres] query tables in schema %s: %v", schemaName, err)
			continue
		}
		for tRows.Next() {
			t := &schema.Table{}
			if err := tRows.Scan(&t.Name, &t.RowCount, &t.SizeBytes, &t.Comment); err != nil {
				continue
			}
			// 非 public schema 的表名加上 schema 前缀，保证跨 schema 唯一
			if schemaName != "public" {
				t.Name = schemaName + "." + t.Name
			}
			tables = append(tables, t)
		}
		tRows.Close()
	}

	total := len(tables)
	for i, t := range tables {
		schemaName, baseName := parsePGTableName(t.Name)
		logf(ctx, "[%s] 采集表 %d/%d: %s", dbName, i+1, total, t.Name)
		fillPGTable(ctx, db, schemaName, baseName, t, redactedDSN)
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

func fillPGTable(ctx context.Context, db *sql.DB, schemaName, baseName string, t *schema.Table, redactedDSN string) {
	// columns
	colRows, err := db.QueryContext(ctx, `
		SELECT a.attname,
		       pg_catalog.format_type(a.atttypid, a.atttypmod),
		       NOT a.attnotnull,
		       COALESCE(pg_get_expr(d.adbin, d.adrelid),''),
		       COALESCE(col_description(a.attrelid, a.attnum),''),
		       COALESCE((SELECT string_agg(contype::text,'')
		                 FROM pg_constraint c
		                 WHERE a.attnum = ANY(c.conkey) AND c.conrelid = a.attrelid),'')
		FROM pg_attribute a
		LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
		WHERE a.attrelid=$1::regclass AND a.attnum>0 AND NOT a.attisdropped
		ORDER BY a.attnum`, schemaName+"."+baseName)
	if err != nil {
		logf(ctx, "[postgres] columns error %s: %v", t.Name, err)
		return
	}
	defer colRows.Close()

	var colsWithoutComment []*schema.Column
	for colRows.Next() {
		c := &schema.Column{}
		var constraints string
		if err := colRows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.Comment, &constraints); err != nil {
			continue
		}
		c.IsPrimary = strings.Contains(constraints, "p")
		c.IsUnique = strings.Contains(constraints, "u")
		t.Columns = append(t.Columns, c)
		if c.Comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}
	colRows.Close()

	// 无注释推断
	if len(colsWithoutComment) > 0 {
		sample, err := fetchPGSampleRow(ctx, db, schemaName, baseName)
		if err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			logf(ctx, "[postgres] sample row failed for %s: %v", t.Name, err)
		}
	}

	// indexes
	idxRows, err := db.QueryContext(ctx, `
		SELECT indexname, indexdef FROM pg_indexes
		WHERE schemaname=$2 AND tablename=$1`, baseName, schemaName)
	if err == nil {
		defer idxRows.Close()
		for idxRows.Next() {
			idx := &schema.Index{}
			var def string
			if err := idxRows.Scan(&idx.Name, &def); err != nil {
				continue
			}
			idx.Unique = strings.Contains(strings.ToUpper(def), "UNIQUE")
			if i := strings.Index(def, "("); i >= 0 {
				inner := def[i+1 : strings.LastIndex(def, ")")]
				for _, col := range strings.Split(inner, ",") {
					idx.Columns = append(idx.Columns, strings.TrimSpace(col))
				}
			}
			t.Indexes = append(t.Indexes, idx)
		}
	} else {
		logf(ctx, "[postgres] index query failed for %s: %v", t.Name, err)
	}

	// foreign keys
	fkRows, err := db.QueryContext(ctx, `
		SELECT c.conname, a.attname, c2.relname, a2.attname
		FROM pg_constraint c
		JOIN pg_class c1 ON c1.oid=c.conrelid
		JOIN pg_class c2 ON c2.oid=c.confrelid
		JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
		JOIN pg_attribute a2 ON a2.attrelid=c.confrelid AND a2.attnum=ANY(c.confkey)
		WHERE c.contype='f' AND c1.relname=$1`, baseName)
	if err == nil {
		defer fkRows.Close()
		fkMap := map[string]*schema.ForeignKey{}
		for fkRows.Next() {
			var name, col, refTable, refCol string
			if err := fkRows.Scan(&name, &col, &refTable, &refCol); err != nil {
				continue
			}
			fk, ok := fkMap[name]
			if !ok {
				fk = &schema.ForeignKey{Name: name, RefTable: refTable}
				fkMap[name] = fk
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, refCol)
		}
	} else {
		logf(ctx, "[postgres] FK query failed for %s: %v", t.Name, err)
	}
}

func fetchPGSampleRow(ctx context.Context, db *sql.DB, schemaName, table string) (map[string]string, error) {
	query := fmt.Sprintf(`SELECT * FROM %s.%s LIMIT 1`, quotePGIdent(schemaName), quotePGIdent(table))
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