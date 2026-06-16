//go:build mysql || full

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
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
		Logf(ctx, "[mysql] [collect] %s", "SHOW DATABASES")
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
		if err := rows.Err(); err != nil {
			log.Printf("[mysql] rows iteration: %v", err)
		}
	}

	for _, dbName := range dbNames {
		Logf(ctx, "[mysql] collecting database %s", dbName)
		database, err := collectMySQLDB(ctx, db, dbName, d.Redacted())
		if err != nil {
			Logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectMySQLDB(ctx context.Context, db *sql.DB, dbName, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}

	// Build table filter clause
	tfClause := ""
	fkTfClause := ""
	opTfClause := ""
	var tfArgs []any
	if names := GetTableFilter(ctx); len(names) > 0 {
		phs := make([]string, len(names))
		for i, n := range names {
			phs[i] = "?"
			tfArgs = append(tfArgs, n)
		}
		phsStr := strings.Join(phs, ",")
		tfClause = " AND TABLE_NAME IN (" + phsStr + ")"
		fkTfClause = " AND k.TABLE_NAME IN (" + phsStr + ")"
		opTfClause = " AND object_name IN (" + phsStr + ")"
	}

	// 设置 max_execution_time 作为服务端超时兜底（MySQL 版 statement_timeout）。
	// 上限 30000ms（30s），防止单个 statement 占满整个 --timeout 预算。
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			ms := int(remaining.Milliseconds())
			if ms > 30000 {
				ms = 30000
			}
			if ms < 1000 {
				ms = 1000
			}
			if _, err := db.ExecContext(ctx, fmt.Sprintf("SET SESSION max_execution_time=%d", ms)); err != nil {
				Logf(ctx, "[mysql] set max_execution_time failed: %v (queries will run without server-side timeout guard)", err)
			}
		}
	}

	// Try Level 1: information_schema.TABLES (fastest)
	Logf(ctx, "[mysql] [collect] [Level 1] %s", `
		SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0), COALESCE(DATA_LENGTH+INDEX_LENGTH,0),
		       COALESCE(TABLE_COMMENT,''), COALESCE(ENGINE,'')
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'` + tfClause + `
		ORDER BY TABLE_NAME`)
	qArgs := append([]any{dbName}, tfArgs...)
	rows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0), COALESCE(DATA_LENGTH+INDEX_LENGTH,0),
		       COALESCE(TABLE_COMMENT,''), COALESCE(ENGINE,'')
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'`+tfClause+`
		ORDER BY TABLE_NAME`, qArgs...)

	var tables []*schema.Table
	if err != nil {
		if isPermissionErr(err) {
			// Level 2: SHOW TABLE STATUS (fallback when information_schema not accessible)
			Logf(ctx, "[mysql] [collect] [Level 2] information_schema.TABLES denied, trying SHOW TABLE STATUS")
			rows, err = db.QueryContext(ctx, "SHOW TABLE STATUS FROM `"+dbName+"`")
			if err != nil {
				return nil, schema.NewDBError(redactedDSN, dbName, "", "query tables (fallback)", err)
			}
			defer rows.Close()
			// SHOW TABLE STATUS columns: Name, Engine, Version, Row_format, Rows, Avg_row_length,
			// Data_length, Max_data_length, Index_length, Data_free, Auto_increment, Create_time,
			// Update_time, Check_time, Collation, Checksum, Create_options, Comment
			for rows.Next() {
				var t schema.Table
				var name, engine string
				var rowsCount, dataLen, idxLen int64
				var comment string
				var version, rowFmt, avgRowLen, maxDataLen, dataFree, autoInc sql.NullInt64
				var createTime, updateTime, checkTime, collation, checksum, createOpts sql.NullString
				if err := rows.Scan(&name, &engine, &version, &rowFmt, &rowsCount, &avgRowLen,
					&dataLen, &maxDataLen, &idxLen, &dataFree, &autoInc,
					&createTime, &updateTime, &checkTime, &collation, &checksum, &createOpts, &comment); err != nil {
					continue
				}
				t.Name = name
				t.Engine = engine
				t.RowCount = rowsCount
				t.SizeBytes = dataLen + idxLen
				t.Comment = comment
				tables = append(tables, &t)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				log.Printf("[mysql] rows iteration: %v", err)
			}
			// Apply table filter in Go (SHOW TABLE STATUS doesn't support parameterized WHERE)
			if len(tfArgs) > 0 {
				filter := make(map[string]bool, len(tfArgs))
				for _, n := range tfArgs {
					filter[n.(string)] = true
				}
				var filtered []*schema.Table
				for _, t := range tables {
					if filter[t.Name] {
						filtered = append(filtered, t)
					}
				}
				tables = filtered
			}
		} else {
			return nil, schema.NewDBError(redactedDSN, dbName, "", "query tables", err)
		}
	} else {
		defer rows.Close()
		for rows.Next() {
			t := &schema.Table{}
			if err := rows.Scan(&t.Name, &t.RowCount, &t.SizeBytes, &t.Comment, &t.Engine); err != nil {
				continue
			}
			tables = append(tables, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			log.Printf("[mysql] rows iteration: %v", err)
		}
	}

	total := len(tables)

	// --- Batch 1: columns ---
	type myColData struct {
		columns            []*schema.Column
		colsWithoutComment []*schema.Column
	}
	colMap := map[string]*myColData{}
	Logf(ctx, "[mysql] [collect] %s", `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
		       COALESCE(COLUMN_DEFAULT,''), EXTRA, COALESCE(COLUMN_COMMENT,''),
		       TABLE_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=?
		ORDER BY TABLE_NAME, ORDINAL_POSITION`)
	cRows, err := db.QueryContext(ctx, `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
		       COALESCE(COLUMN_DEFAULT,''), EXTRA, COALESCE(COLUMN_COMMENT,''),
		       TABLE_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=?`+tfClause+`
		ORDER BY TABLE_NAME, ORDINAL_POSITION`, append([]any{dbName}, tfArgs...)...)
	if err != nil {
		return nil, schema.NewDBError(redactedDSN, dbName, "", "batch query columns", err)
	}
	for cRows.Next() {
		var c schema.Column
		var nullable, key, extra, tbl string
		if err := cRows.Scan(&c.Name, &c.Type, &nullable, &key, &c.Default, &extra, &c.Comment, &tbl); err != nil {
			continue
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsUnique = key == "UNI"
		c.IsIndex = key == "MUL"
		cd, ok := colMap[tbl]
		if !ok {
			cd = &myColData{}
			colMap[tbl] = cd
		}
		cd.columns = append(cd.columns, &c)
		if c.Comment == "" {
			cd.colsWithoutComment = append(cd.colsWithoutComment, &c)
		}
	}
	cRows.Close()
	if err := cRows.Err(); err != nil {
		log.Printf("[mysql] batch columns iteration: %v", err)
	}

	// --- Batch 2: indexes (via information_schema.STATISTICS) ---
	idxMap := map[string][]*schema.Index{}
	Logf(ctx, "[mysql] [collect] %s", `
		SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA=?
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`)
	iRows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA=?`+tfClause+`
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, append([]any{dbName}, tfArgs...)...)
	if err == nil {
		// Map to group columns per index per table: tableKey -> [indexKey]*Index
		type idxKey struct{ tbl, name string }
		tmpIdx := map[idxKey]*schema.Index{}
		for iRows.Next() {
			var tbl, idxName, colName string
			var nonUnique, seqInIndex int
			if err := iRows.Scan(&tbl, &idxName, &colName, &nonUnique, &seqInIndex); err != nil {
				continue
			}
			k := idxKey{tbl, idxName}
			if existing, ok := tmpIdx[k]; ok {
				existing.Columns = append(existing.Columns, colName)
			} else {
				isPK := idxName == "PRIMARY"
				tmpIdx[k] = &schema.Index{
					Name:    idxName,
					Columns: []string{colName},
					Unique:  isPK || nonUnique == 0,
				}
			}
		}
		for k, idx := range tmpIdx {
			idxMap[k.tbl] = append(idxMap[k.tbl], idx)
		}
		if err := iRows.Err(); err != nil {
			log.Printf("[mysql] batch indexes iteration: %v", err)
		}
	} else {
		Logf(ctx, "[mysql] batch indexes query failed: %v", err)
	}
	iRows.Close()

	// --- Batch 3: foreign keys ---
	fkMap := map[string][]*schema.ForeignKey{}
	Logf(ctx, "[mysql] [collect] %s", `
		SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME,
		       k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
		       k.TABLE_NAME
		FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA=? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.TABLE_NAME, k.CONSTRAINT_NAME, k.ORDINAL_POSITION`)
	fRows, err := db.QueryContext(ctx, `
		SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME,
		       k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
		       k.TABLE_NAME
		FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA=? AND k.REFERENCED_TABLE_NAME IS NOT NULL`+fkTfClause+`
		ORDER BY k.TABLE_NAME, k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, append([]any{dbName}, tfArgs...)...)
	if err == nil {
		for fRows.Next() {
			var name, col, refDB, refTable, refCol, tbl string
			if err := fRows.Scan(&name, &col, &refDB, &refTable, &refCol, &tbl); err != nil {
				continue
			}
			// Find FK for this table+constraint
			var fk *schema.ForeignKey
			for _, existing := range fkMap[tbl] {
				if existing.Name == name {
					fk = existing
					break
				}
			}
			if fk == nil {
				fk = &schema.ForeignKey{
					Name:     name,
					RefDB:    refDB,
					RefTable: refTable,
				}
				fkMap[tbl] = append(fkMap[tbl], fk)
			}
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, refCol)
		}
		if err := fRows.Err(); err != nil {
			log.Printf("[mysql] batch FK iteration: %v", err)
		}
	} else {
		Logf(ctx, "[mysql] batch FK query failed: %v", err)
	}
	fRows.Close()

	// --- Batch 4: FK rules ---
	rulesMap := map[string]map[string]struct{ del, upd string }{}
	// Key: tableName -> constraintName -> {del, upd}
	Logf(ctx, "[mysql] [collect] %s", `
		SELECT CONSTRAINT_NAME, DELETE_RULE, UPDATE_RULE, TABLE_NAME
		FROM information_schema.REFERENTIAL_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA=?`)
	ruleRows, err := db.QueryContext(ctx, `
		SELECT CONSTRAINT_NAME, DELETE_RULE, UPDATE_RULE, TABLE_NAME
		FROM information_schema.REFERENTIAL_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA=?`+tfClause, append([]any{dbName}, tfArgs...)...)
	if err == nil {
		for ruleRows.Next() {
			var cName, delRule, updRule, tbl string
			if err := ruleRows.Scan(&cName, &delRule, &updRule, &tbl); err != nil {
				continue
			}
			if rulesMap[tbl] == nil {
				rulesMap[tbl] = map[string]struct{ del, upd string }{}
			}
			rulesMap[tbl][cName] = struct{ del, upd string }{del: delRule, upd: updRule}
		}
		if err := ruleRows.Err(); err != nil {
			log.Printf("[mysql] batch FK rules iteration: %v", err)
		}
	} else {
		Logf(ctx, "[mysql] batch FK rules query failed: %v", err)
	}
	ruleRows.Close()

	// --- Batch 5: op_stats (skip if --skip-opstats) ---
	opMap := map[string]*schema.OpStats{}
	if !IsSkipOpstats(ctx) {
		Logf(ctx, "[mysql] [collect] %s", `
			SELECT object_name,
			       COALESCE(SUM(count_read), 0),
			       COALESCE(SUM(count_write), 0)
			FROM performance_schema.table_io_waits_summary_by_table
			WHERE object_schema=?
			GROUP BY object_name`)
		oRows, err := db.QueryContext(ctx, `
			SELECT object_name,
			       COALESCE(SUM(count_read), 0),
			       COALESCE(SUM(count_write), 0)
			FROM performance_schema.table_io_waits_summary_by_table
			WHERE object_schema=?`+opTfClause+`
			GROUP BY object_name`, append([]any{dbName}, tfArgs...)...)
		if err == nil {
			for oRows.Next() {
				var tbl string
				var read, write int64
				if err := oRows.Scan(&tbl, &read, &write); err != nil {
					continue
				}
				opMap[tbl] = &schema.OpStats{
					SeqScan: read + write,
					NtupIns: write,
				}
			}
			if err := oRows.Err(); err != nil {
				log.Printf("[mysql] batch opstats iteration: %v", err)
			}
		} else {
			Logf(ctx, "[mysql] batch opstats unavailable: %v", err)
		}
		oRows.Close()
	}

	// Assign batch results + sample rows
	for i, t := range tables {
		Logf(ctx, "[%s] 采集表 %d/%d: %s", dbName, i+1, total, t.Name)

		// Assign pre-fetched columns
		if cd := colMap[t.Name]; cd != nil {
			t.Columns = cd.columns
		}
		t.Indexes = idxMap[t.Name]
		t.ForeignKeys = fkMap[t.Name]

		// Assign FK rules
		if tblRules, ok := rulesMap[t.Name]; ok {
			for _, fk := range t.ForeignKeys {
				if r, ok := tblRules[fk.Name]; ok {
					fk.OnDelete = r.del
					fk.OnUpdate = r.upd
				}
			}
		}

		// Assign op stats
		if os, ok := opMap[t.Name]; ok {
			t.OpStats = os
		}

		// Sample row for comment inference
		if IsSample(ctx) {
			if cd := colMap[t.Name]; cd != nil && len(cd.colsWithoutComment) > 0 && t.RowCount > 0 {
				if sample, err := fetchMySQLSampleRow(ctx, db, dbName, t.Name); err == nil {
					for _, c := range cd.colsWithoutComment {
						if val, ok := sample[c.Name]; ok {
							c.Comment = schema.InferComment(c.Name, c.Type, val)
						}
					}
				} else {
					Logf(ctx, "[mysql] sample row failed for %s.%s: %v", dbName, t.Name, err)
				}
			}
		}
	}

	database.Tables = tables
	return database, nil
}

// collectMySQLOpStats 从 performance_schema 采集表级 IO 统计。
// 如果 performance_schema 不可用（如 TDSQL 分布式版本或关闭了该功能），静默跳过。
func collectMySQLOpStats(ctx context.Context, db *sql.DB, dbName string, t *schema.Table) {
	Logf(ctx, "[mysql] [collect] %s", `
		SELECT COALESCE(SUM(count_read), 0),
		       COALESCE(SUM(count_write), 0)
		FROM performance_schema.table_io_waits_summary_by_table
		WHERE object_schema = ? AND object_name = ?`)
	row := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(count_read), 0),
		       COALESCE(SUM(count_write), 0)
		FROM performance_schema.table_io_waits_summary_by_table
		WHERE object_schema = ? AND object_name = ?`, dbName, t.Name)

	var countRead, countWrite int64
	if err := row.Scan(&countRead, &countWrite); err != nil {
		Logf(ctx, "[mysql] performance_schema unavailable for %s.%s, skipping op stats", dbName, t.Name)
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
		if _, err := db.ExecContext(ctx, fmt.Sprintf("SET SESSION max_execution_time=%d", opts.Timeout*1000)); err != nil {
			Logf(ctx, "[mysql] set max_execution_time failed: %v (query will still run without timeout guard)", err)
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