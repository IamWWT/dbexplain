package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"dbexplain/dsn"
	"dbexplain/schema"
)

type mysqlConnector struct{}

func (mysqlConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildMySQLDSN(d)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	defer db.Close()

	// 使用 context 控制 Ping 超时
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "mysql", Label: d.Label}
	var dbNames []string
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else {
		rows, err := db.QueryContext(ctx, "SHOW DATABASES")
		if err != nil {
			return nil, err
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
		database, err := collectMySQLDB(ctx, db, dbName)
		if err != nil {
			return nil, fmt.Errorf("db %s: %w", dbName, err)
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectMySQLDB(ctx context.Context, db *sql.DB, dbName string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}
	rows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0), COALESCE(DATA_LENGTH+INDEX_LENGTH,0),
		       COALESCE(TABLE_COMMENT,''), COALESCE(ENGINE,'')
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'
		ORDER BY TABLE_NAME`, dbName)
	if err != nil {
		return nil, err
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

	for _, t := range tables {
		if err := fillMySQLTable(ctx, db, dbName, t); err != nil {
			return nil, err
		}
	}
	database.Tables = tables
	return database, nil
}

func fillMySQLTable(ctx context.Context, db *sql.DB, dbName string, t *schema.Table) error {
	// columns
	colRows, err := db.QueryContext(ctx, `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
		       COALESCE(COLUMN_DEFAULT,''), EXTRA, COALESCE(COLUMN_COMMENT,'')
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=? AND TABLE_NAME=?
		ORDER BY ORDINAL_POSITION`, dbName, t.Name)
	if err != nil {
		return fmt.Errorf("columns: %w", err)
	}
	defer colRows.Close()
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
	}
	colRows.Close()

	// indexes
	idxQuery := "SHOW INDEX FROM " + quoteMySQL(t.Name) + " WHERE Key_name != 'PRIMARY'"
	idxRows, err := db.QueryContext(ctx, idxQuery)
	if err == nil {
		defer idxRows.Close()
		idxMap := map[string]*schema.Index{}
		for idxRows.Next() {
			var nonUnique, keyName, colName string
			var tableName, seqInIndex, collation, cardinality, subPart, packed, null, indexType, comment, indexComment, visible interface{}
			if err := idxRows.Scan(&tableName, &nonUnique, &keyName, &seqInIndex, &colName, &collation, &cardinality,
				&subPart, &packed, &null, &indexType, &comment, &indexComment, &visible); err != nil {
				continue
			}
			if existing, ok := idxMap[keyName]; ok {
				existing.Columns = append(existing.Columns, colName)
			} else {
				idxMap[keyName] = &schema.Index{
					Name:    keyName,
					Columns: []string{colName},
					Unique:  nonUnique == "0",
				}
			}
		}
		for _, idx := range idxMap {
			t.Indexes = append(t.Indexes, idx)
		}
	}

	// primary key
	pkRows, err := db.QueryContext(ctx, "SHOW INDEX FROM "+quoteMySQL(t.Name)+" WHERE Key_name = 'PRIMARY'")
	if err == nil {
		defer pkRows.Close()
		pk := &schema.Index{Name: "PRIMARY", Unique: true, Columns: []string{}}
		for pkRows.Next() {
			var colName string
			var ignore [12]interface{}
			if err := pkRows.Scan(&ignore[0], &ignore[1], &ignore[2], &ignore[3], &colName, &ignore[4], &ignore[5],
				&ignore[6], &ignore[7], &ignore[8], &ignore[9], &ignore[10], &ignore[11]); err != nil {
				continue
			}
			pk.Columns = append(pk.Columns, colName)
		}
		if len(pk.Columns) > 0 {
			t.Indexes = append(t.Indexes, pk)
		}
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
	}
	return nil
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

func isMySQLSystemDB(name string) bool {
	sys := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}
	return sys[strings.ToLower(name)]
}