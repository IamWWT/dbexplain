package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/IamWWT/dbexplain/capabilities"
	"github.com/IamWWT/dbexplain/dsn"
	"github.com/IamWWT/dbexplain/query"
	"github.com/IamWWT/dbexplain/schema"
)

func init() {
	Register("sqlite", func() Connector { return sqliteConnector{} })
}

type sqliteConnector struct{}

func (sqliteConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapForeignKey,
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapIndex,
	}
}

func (sqliteConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	path := d.SQLitePath()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "sqlite", Label: d.Label}
	database := &schema.Database{Name: path}

	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "list tables", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			tableNames = append(tableNames, n)
		}
	}
	rows.Close()

	total := len(tableNames)
	for i, tn := range tableNames {
		logf(ctx, "[sqlite] 采集表 %d/%d: %s", i+1, total, tn)
		t := &schema.Table{Name: tn}
		fillSQLiteTable(ctx, db, t, d.Redacted())
		database.Tables = append(database.Tables, t)
	}
	inst.Databases = append(inst.Databases, database)
	return inst, nil
}

func fillSQLiteTable(ctx context.Context, db *sql.DB, t *schema.Table, redactedDSN string) {
	// columns
	colRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(t.Name, "'", "''")))
	if err != nil {
		logf(ctx, "[sqlite] columns error %s: %v", t.Name, err)
		return
	}
	defer colRows.Close()

	var colsWithoutComment []*schema.Column
	for colRows.Next() {
		var cid int
		var dflt sql.NullString
		c := &schema.Column{}
		var notnull, pk int
		if err := colRows.Scan(&cid, &c.Name, &c.Type, &notnull, &dflt, &pk); err != nil {
			continue
		}
		c.Nullable = notnull == 0 && pk == 0
		c.IsPrimary = pk > 0
		if dflt.Valid {
			c.Default = dflt.String
		}
		t.Columns = append(t.Columns, c)
		if c.Comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}
	colRows.Close()

	// row count
	db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, strings.ReplaceAll(t.Name, `"`, `""`))).Scan(&t.RowCount)

	// 推断注释
	if len(colsWithoutComment) > 0 && t.RowCount > 0 {
		if sample, err := fetchSQLiteSampleRow(ctx, db, t.Name); err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			logf(ctx, "[sqlite] sample row failed for %s: %v", t.Name, err)
		}
	}

	// indexes
	idxRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_list('%s')", strings.ReplaceAll(t.Name, "'", "''")))
	if err == nil {
		defer idxRows.Close()
		for idxRows.Next() {
			var seq, partial int
			idx := &schema.Index{}
			var unique int
			if err := idxRows.Scan(&seq, &idx.Name, &unique, &idx.Type, &partial); err != nil {
				continue
			}
			idx.Unique = unique == 1
			icols, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_info('%s')", strings.ReplaceAll(idx.Name, "'", "''")))
			if err == nil {
				for icols.Next() {
					var seqno, cid int
					var name string
					if err := icols.Scan(&seqno, &cid, &name); err == nil {
						idx.Columns = append(idx.Columns, name)
					}
				}
				icols.Close()
			}
			t.Indexes = append(t.Indexes, idx)
		}
	}

	// foreign keys
	fkRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", strings.ReplaceAll(t.Name, "'", "''")))
	if err == nil {
		defer fkRows.Close()
		fkMap := map[int]*schema.ForeignKey{}
		for fkRows.Next() {
			var id, seq int
			var table, from, to, onUpdate, onDelete, match string
			if err := fkRows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				continue
			}
			fk, ok := fkMap[id]
			if !ok {
				fk = &schema.ForeignKey{
					Name:     strings.Join([]string{t.Name, from, table}, "_"),
					RefTable: table,
					OnDelete: onDelete,
					OnUpdate: onUpdate,
				}
				fkMap[id] = fk
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}
			fk.Columns = append(fk.Columns, from)
			fk.RefColumns = append(fk.RefColumns, to)
		}
	}
}

// ExecQuery implements query.Queryable for SQLite.
func (sqliteConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	path := opts.DSN.SQLitePath()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	defer db.Close()

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	result, err := executeSQLQuery(runCtx, db, opts.SQL, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("sqlite query: %w", err)
	}
	return result, nil
}

func fetchSQLiteSampleRow(ctx context.Context, db *sql.DB, table string) (map[string]string, error) {
	query := fmt.Sprintf(`SELECT * FROM "%s" LIMIT 1`, table)
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
		} else {
			result[col] = fmt.Sprintf("%v", val)
		}
	}
	return result, nil
}