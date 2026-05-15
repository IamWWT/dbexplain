package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"  

	_ "github.com/glebarez/go-sqlite"
	"dbexplain/dsn"
	"dbexplain/schema"
)

type sqliteConnector struct{}

func (sqliteConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	path := d.SQLitePath()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "sqlite", Label: d.Label}
	database := &schema.Database{Name: path}

	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
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

	for _, tn := range tableNames {
		t := &schema.Table{Name: tn}
		fillSQLiteTable(ctx, db, t)
		database.Tables = append(database.Tables, t)
	}
	inst.Databases = append(inst.Databases, database)
	return inst, nil
}

func fillSQLiteTable(ctx context.Context, db *sql.DB, t *schema.Table) {
	// columns
	colRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(t.Name, "'", "''")))
	if err != nil {
		return
	}
	defer colRows.Close()
	for colRows.Next() {
		var cid int
		var dflt sql.NullString
		c := &schema.Column{}
		var notnull, pk int
		if err := colRows.Scan(&cid, &c.Name, &c.Type, &notnull, &dflt, &pk); err != nil {
			continue
		}
		c.Nullable = notnull == 0
		c.IsPrimary = pk > 0
		if dflt.Valid {
			c.Default = dflt.String
		}
		t.Columns = append(t.Columns, c)
	}
	colRows.Close()

	// row count
	db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, strings.ReplaceAll(t.Name, `"`, `""`))).Scan(&t.RowCount)

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
			// get columns
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
				}
				fkMap[id] = fk
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}
			fk.Columns = append(fk.Columns, from)
			fk.RefColumns = append(fk.RefColumns, to)
		}
	}
}