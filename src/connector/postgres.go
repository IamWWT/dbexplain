package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"     

	_ "github.com/lib/pq"
	"dbexplain/dsn"
	"dbexplain/schema"
)

type postgresConnector struct{}

func (postgresConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildPGDSN(d)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "postgres", Label: d.Label}

	var dbNames []string
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else {
		rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
		if err != nil {
			return nil, err
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
		database, err := collectPGDB(ctx, db, dbName)
		if err != nil {
			return nil, fmt.Errorf("db %s: %w", dbName, err)
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectPGDB(ctx context.Context, db *sql.DB, dbName string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}
	rows, err := db.QueryContext(ctx, `
		SELECT tablename,
		       pg_size_pretty(pg_total_relation_size(quote_ident(tablename))),
		       COALESCE(obj_description(quote_ident(tablename)::regclass,'pg_class'),'')
		FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []*schema.Table
	for rows.Next() {
		t := &schema.Table{}
		var size string
		if err := rows.Scan(&t.Name, &size, &t.Comment); err != nil {
			continue
		}
		tables = append(tables, t)
	}
	rows.Close()

	for _, t := range tables {
		fillPGTable(ctx, db, t)
	}
	database.Tables = tables
	return database, nil
}

func fillPGTable(ctx context.Context, db *sql.DB, t *schema.Table) {
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
		ORDER BY a.attnum`, "public."+t.Name)
	if err != nil {
		return
	}
	defer colRows.Close()
	for colRows.Next() {
		c := &schema.Column{}
		var constraints string
		if err := colRows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.Comment, &constraints); err != nil {
			continue
		}
		c.IsPrimary = strings.Contains(constraints, "p")
		c.IsUnique = strings.Contains(constraints, "u")
		t.Columns = append(t.Columns, c)
	}
	colRows.Close()

	// indexes
	idxRows, err := db.QueryContext(ctx, `
		SELECT indexname, indexdef FROM pg_indexes
		WHERE schemaname='public' AND tablename=$1`, t.Name)
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
	}

	// foreign keys
	fkRows, err := db.QueryContext(ctx, `
		SELECT c.conname, a.attname, c2.relname, a2.attname
		FROM pg_constraint c
		JOIN pg_class c1 ON c1.oid=c.conrelid
		JOIN pg_class c2 ON c2.oid=c.confrelid
		JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
		JOIN pg_attribute a2 ON a2.attrelid=c.confrelid AND a2.attnum=ANY(c.confkey)
		WHERE c.contype='f' AND c1.relname=$1`, t.Name)
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
	}
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
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
		host, port, d.User, d.Password, dbname)
}