package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/IamWWT/dbexplain/capabilities"
	"github.com/IamWWT/dbexplain/dsn"
	"github.com/IamWWT/dbexplain/query"
	"github.com/IamWWT/dbexplain/schema"
)

func init() {
	Register("mysql", func() Connector { return mysqlConnector{} })
}

type mysqlConnector struct{}

func (mysqlConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapForeignKey,
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapIndex,
		capabilities.CapSQL,
	}
}

func (mysqlConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildMySQLDSN(d)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "mysql", Label: d.Label}

	var dbNames []string
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else {
		rows, err := db.QueryContext(ctx, "SHOW DATABASES")
		if err != nil {
			return nil, schema.NewDBError(d.Redacted(), "", "", "list databases", err)
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				if !isMySQLSystemDB(n) {
					dbNames = append(dbNames, n)
				}
			}
		}
	}

	for _, dbName := range dbNames {
		logf(ctx, "[mysql] collecting database %s", dbName)
		database, err := collectMySQLDB(ctx, db, dbName, d.Redacted())
		if err != nil {
			logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectMySQLDB(ctx context.Context, db *sql.DB, dbName, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}
	rows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0), COALESCE(DATA_LENGTH+INDEX_LENGTH,0),
		       COALESCE(TABLE_COMMENT,''), COALESCE(ENGINE,'')
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'
		ORDER BY TABLE_NAME`, dbName)
	if err != nil {
		return nil, schema.NewDBError(redactedDSN, dbName, "", "query tables", err)
	}
	defer rows.Close()

	var tables []*schema.Table
	for rows.Next() {
		t := &schema.Table{}
		if err := rows.Scan(&t.Name, &t.RowCount, &t.SizeBytes, &t.Comment, &t.Engine); err != nil {
			continue
		}
		tables = append(tables, t)
	}
	rows.Close()

	total := len(tables)
	for i, t := range tables {
		logf(ctx, "[%s] 采集表 %d/%d: %s", dbName, i+1, total, t.Name)
		fillMySQLTable(ctx, db, dbName, t, redactedDSN)
	}
	database.Tables = tables
	return database, nil
}

func fillMySQLTable(ctx context.Context, db *sql.DB, dbName string, t *schema.Table, redactedDSN string) {
	// columns
	colRows, err := db.QueryContext(ctx, `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
		       COALESCE(COLUMN_DEFAULT,''), EXTRA, COALESCE(COLUMN_COMMENT,'')
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=? AND TABLE_NAME=?
		ORDER BY ORDINAL_POSITION`, dbName, t.Name)
	if err != nil {
		logf(ctx, "[mysql] columns error %s.%s: %v", dbName, t.Name, err)
		return
	}
	defer colRows.Close()

	var colsWithoutComment []*schema.Column
	for colRows.Next() {
		c := &schema.Column{}
		var nullable, key, extra string
		if err := colRows.Scan(&c.Name, &c.Type, &nullable, &key, &c.Default, &extra, &c.Comment); err != nil {
			continue
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsUnique = key == "UNI"
		c.IsIndex = key == "MUL"
		t.Columns = append(t.Columns, c)
		if c.Comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}
	colRows.Close()

	// 对无注释的列，取首行数据推断
	if len(colsWithoutComment) > 0 && t.RowCount > 0 {
		if sample, err := fetchMySQLSampleRow(ctx, db, dbName, t.Name); err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			logf(ctx, "[mysql] sample row failed for %s.%s: %v", dbName, t.Name, err)
		}
	}

	// indexes and primary key (single SHOW INDEX query)
	idxRows, err := db.QueryContext(ctx, "SHOW INDEX FROM "+quoteMySQL(t.Name))
	if err == nil {
		defer idxRows.Close()
		idxMap := map[string]*schema.Index{}
		for idxRows.Next() {
			var nonUnique int
			var keyName, colName string
			var tableName, seqInIndex, collation, cardinality, subPart, packed, null, indexType, comment, indexComment, visible interface{}
			if err := idxRows.Scan(&tableName, &nonUnique, &keyName, &seqInIndex, &colName, &collation, &cardinality,
				&subPart, &packed, &null, &indexType, &comment, &indexComment, &visible); err != nil {
				continue
			}
			if existing, ok := idxMap[keyName]; ok {
				existing.Columns = append(existing.Columns, colName)
			} else {
				isPK := keyName == "PRIMARY"
				idxMap[keyName] = &schema.Index{
					Name:    keyName,
					Columns: []string{colName},
					Unique:  isPK || nonUnique == 0,
				}
			}
		}
		for _, idx := range idxMap {
			t.Indexes = append(t.Indexes, idx)
		}
	} else {
		logf(ctx, "[mysql] index query failed for %s: %v", t.Name, err)
	}

	// foreign keys
	fkRows, err := db.QueryContext(ctx, `
		SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME,
		       k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA=? AND k.TABLE_NAME=? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, dbName, t.Name)
	if err == nil {
		defer fkRows.Close()
		fkMap := map[string]*schema.ForeignKey{}
		for fkRows.Next() {
			var name, col, refDB, refTable, refCol string
			if err := fkRows.Scan(&name, &col, &refDB, &refTable, &refCol); err != nil {
				continue
			}
			fk, ok := fkMap[name]
			if !ok {
				fk = &schema.ForeignKey{
					Name:     name,
					RefDB:    refDB,
					RefTable: refTable,
				}
				fkMap[name] = fk
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, refCol)
		}
	} else {
		logf(ctx, "[mysql] FK query failed for %s: %v", t.Name, err)
	}

	// fetch FK on_delete/on_update rules from REFERENTIAL_CONSTRAINTS
	if len(t.ForeignKeys) > 0 {
		ruleRows, err := db.QueryContext(ctx, `
			SELECT CONSTRAINT_NAME, DELETE_RULE, UPDATE_RULE
			FROM information_schema.REFERENTIAL_CONSTRAINTS
			WHERE CONSTRAINT_SCHEMA=? AND TABLE_NAME=?`, dbName, t.Name)
		if err == nil {
			defer ruleRows.Close()
			for ruleRows.Next() {
				var cName, delRule, updRule string
				if err := ruleRows.Scan(&cName, &delRule, &updRule); err != nil {
					continue
				}
				for _, fk := range t.ForeignKeys {
					if fk.Name == cName {
						fk.OnDelete = delRule
						fk.OnUpdate = updRule
					}
				}
			}
		}
	}

	// 操作语义采集 (Phase 3) — performance_schema 可能不可用，静默跳过
	collectMySQLOpStats(ctx, db, dbName, t)
}

// collectMySQLOpStats 从 performance_schema 采集表级 IO 统计。
// 如果 performance_schema 不可用（如 TDSQL 分布式版本或关闭了该功能），静默跳过。
func collectMySQLOpStats(ctx context.Context, db *sql.DB, dbName string, t *schema.Table) {
	row := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(count_read), 0),
		       COALESCE(SUM(count_write), 0)
		FROM performance_schema.table_io_waits_summary_by_table
		WHERE object_schema = ? AND object_name = ?`, dbName, t.Name)

	var countRead, countWrite int64
	if err := row.Scan(&countRead, &countWrite); err != nil {
		logf(ctx, "[mysql] performance_schema unavailable for %s.%s, skipping op stats", dbName, t.Name)
		return
	}
	t.OpStats = &schema.OpStats{
		SeqScan: countRead + countWrite, // MySQL 不区分 seq/index scan
		NtupIns: countWrite,
	}
}

// fetchMySQLSampleRow 获取表的第一行数据，返回 map[column]value
func fetchMySQLSampleRow(ctx context.Context, db *sql.DB, dbName, table string) (map[string]string, error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 1", quoteMySQL(dbName), quoteMySQL(table))
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

func buildMySQLDSN(d *dsn.DSN) string {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "3306"
	}
	auth := ""
	if d.User != "" {
		auth = d.User
		if d.Password != "" {
			auth += ":" + d.Password
		}
		auth += "@"
	}
	db := ""
	if d.DBName != "" {
		db = "/" + d.DBName
	}
	return fmt.Sprintf("%stcp(%s:%s)%s?charset=utf8mb4&parseTime=true&timeout=5s", auth, host, port, db)
}

func quoteMySQL(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// ExecQuery implements query.Queryable for MySQL.
func (mysqlConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	connStr := buildMySQLDSN(opts.DSN)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Set max execution time if timeout specified
	if opts.Timeout > 0 {
		db.ExecContext(ctx, fmt.Sprintf("SET SESSION max_execution_time=%d", opts.Timeout*1000))
	}

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	result, err := executeSQLQuery(runCtx, db, opts.SQL, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("mysql query: %w", err)
	}
	return result, nil
}

func isMySQLSystemDB(name string) bool {
	sys := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}
	return sys[strings.ToLower(name)]
}