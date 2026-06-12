//go:build oracle || full

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	_ "github.com/sijms/go-ora/v2"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("oracle", func() Connector { return oracleConnector{} })
}

type oracleConnector struct{}

func (oracleConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapForeignKey,
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapIndex,
		capabilities.CapSQL,
	}
}

func (oracleConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildOracleDSN(d)
	db, err := sql.Open("oracle", connStr)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	return collectOracleSchema(ctx, db, d)
}

// collectOracleSchema performs Oracle schema collection using an existing database handle.
// Extracted for testability with go-sqlmock.
func collectOracleSchema(ctx context.Context, db *sql.DB, d *dsn.DSN) (*schema.Instance, error) {
	inst := &schema.Instance{DSN: d.Redacted(), Kind: "oracle", Label: d.Label}

	var owners []string
	if d.DBName != "" {
		owners = []string{strings.ToUpper(d.DBName)}
	} else {
		rows, err := db.QueryContext(ctx, "SELECT DISTINCT owner FROM all_tables ORDER BY owner")
		if err != nil {
			return nil, schema.NewDBError(d.Redacted(), "", "", "list owners", err)
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				if !isOracleSystemSchema(n) {
					owners = append(owners, n)
				}
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[oracle] rows iteration: %v", err)
		}
	}

	for _, owner := range owners {
		logf(ctx, "[oracle] collecting schema %s", owner)
		database, err := collectOracleDB(ctx, db, owner, d.Redacted())
		if err != nil {
			logf(ctx, "error in schema %s: %v", owner, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectOracleDB(ctx context.Context, db *sql.DB, owner, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: owner}
	rows, err := db.QueryContext(ctx, `
		SELECT t.table_name, COALESCE(t.num_rows, 0), COALESCE(c.comments, '')
		FROM all_tables t
		LEFT JOIN all_tab_comments c ON c.owner = t.owner AND c.table_name = t.table_name AND c.table_type = 'TABLE'
		WHERE t.owner = :1
		ORDER BY t.table_name`, owner)
	if err != nil {
		return nil, schema.NewDBError(redactedDSN, owner, "", "query tables", err)
	}
	defer rows.Close()

	var tables []*schema.Table
	for rows.Next() {
		t := &schema.Table{}
		if err := rows.Scan(&t.Name, &t.RowCount, &t.Comment); err != nil {
			continue
		}
		tables = append(tables, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("[oracle] rows iteration: %v", err)
	}

	total := len(tables)
	for i, t := range tables {
		logf(ctx, "[oracle] collecting table %d/%d: %s.%s", i+1, total, owner, t.Name)
		fillOracleTable(ctx, db, owner, t, redactedDSN)
	}
	database.Tables = tables
	return database, nil
}

func fillOracleTable(ctx context.Context, db *sql.DB, owner string, t *schema.Table, redactedDSN string) {
	// columns
	colRows, err := db.QueryContext(ctx, `
		SELECT c.column_name, c.data_type,
		       c.nullable, COALESCE(c.data_default, ''),
		       COALESCE(cc.comments, ''), c.column_id
		FROM all_tab_columns c
		LEFT JOIN all_col_comments cc ON cc.owner = c.owner AND cc.table_name = c.table_name AND cc.column_name = c.column_name
		WHERE c.owner = :1 AND c.table_name = :2
		ORDER BY c.column_id`, owner, t.Name)
	if err != nil {
		logf(ctx, "[oracle] columns error %s.%s: %v", owner, t.Name, err)
		return
	}
	defer colRows.Close()

	var colsWithoutComment []*schema.Column
	for colRows.Next() {
		c := &schema.Column{}
		var nullable, defaultVal, comment string
		var colID int
		if err := colRows.Scan(&c.Name, &c.Type, &nullable, &defaultVal, &comment, &colID); err != nil {
			continue
		}
		c.Nullable = nullable == "Y"
		c.Default = defaultVal
		c.Comment = comment
		t.Columns = append(t.Columns, c)
		if comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}
	colRows.Close()
	if err := colRows.Err(); err != nil {
		log.Printf("[oracle] rows iteration: %v", err)
	}

	// constraints (PK/UK) — set IsPrimary and IsUnique
	cRows, err := db.QueryContext(ctx, `
		SELECT cc.constraint_name, c.constraint_type, cc.column_name
		FROM all_cons_columns cc
		JOIN all_constraints c ON c.constraint_name = cc.constraint_name AND c.owner = cc.owner
		WHERE c.owner = :1 AND c.table_name = :2 AND c.constraint_type IN ('P', 'U')
		ORDER BY c.constraint_name, cc.position`, owner, t.Name)
	if err == nil {
		defer cRows.Close()
		pkCols := make(map[string]bool)
		ukCols := make(map[string]bool)
		for cRows.Next() {
			var conName, conType, colName string
			if err := cRows.Scan(&conName, &conType, &colName); err != nil {
				continue
			}
			if conType == "P" {
				pkCols[colName] = true
			} else if conType == "U" {
				ukCols[colName] = true
			}
		}
		cRows.Close()
		if err := cRows.Err(); err != nil {
			log.Printf("[oracle] rows iteration: %v", err)
		}
		for _, col := range t.Columns {
			if pkCols[col.Name] {
				col.IsPrimary = true
			}
			if ukCols[col.Name] {
				col.IsUnique = true
			}
		}
	} else {
		logf(ctx, "[oracle] constraint query failed for %s: %v", t.Name, err)
	}

	// indexes
	idxRows, err := db.QueryContext(ctx, `
		SELECT ic.index_name, ic.column_name, i.uniqueness
		FROM all_ind_columns ic
		JOIN all_indexes i ON i.index_name = ic.index_name AND i.owner = ic.index_owner
			AND i.table_owner = ic.table_owner AND i.table_name = ic.table_name
		WHERE ic.table_owner = :1 AND ic.table_name = :2
		ORDER BY ic.index_name, ic.column_position`, owner, t.Name)
	if err == nil {
		defer idxRows.Close()
		idxMap := map[string]*schema.Index{}
		for idxRows.Next() {
			var idxName, colName, uniqueness string
			if err := idxRows.Scan(&idxName, &colName, &uniqueness); err != nil {
				continue
			}
			if existing, ok := idxMap[idxName]; ok {
				existing.Columns = append(existing.Columns, colName)
			} else {
				idxMap[idxName] = &schema.Index{
					Name:    idxName,
					Columns: []string{colName},
					Unique:  uniqueness == "UNIQUE",
				}
			}
		}
		if err := idxRows.Err(); err != nil {
			log.Printf("[oracle] rows iteration: %v", err)
		}
		for _, idx := range idxMap {
			t.Indexes = append(t.Indexes, idx)
		}
	} else {
		logf(ctx, "[oracle] index query failed for %s: %v", t.Name, err)
	}

	// foreign keys — 4-table JOIN with position alignment
	fkRows, err := db.QueryContext(ctx, `
		SELECT a.constraint_name, a.column_name,
		       c.r_owner, c.r_constraint_name, a.position
		FROM all_cons_columns a
		JOIN all_constraints c ON c.constraint_name = a.constraint_name AND c.owner = a.owner
		WHERE c.owner = :1 AND c.table_name = :2 AND c.constraint_type = 'R'
		ORDER BY a.constraint_name, a.position`, owner, t.Name)
	if err == nil {
		defer fkRows.Close()
		fkMap := map[string]*schema.ForeignKey{}
		type fkRef struct {
			rOwner       string
			rConstraint  string
			positions    []int
		}
		fkRefs := map[string]*fkRef{}
		for fkRows.Next() {
			var name, col, rOwner, rConstraint string
			var pos int
			if err := fkRows.Scan(&name, &col, &rOwner, &rConstraint, &pos); err != nil {
				continue
			}
			fk, ok := fkMap[name]
			if !ok {
				fk = &schema.ForeignKey{Name: name}
				fkMap[name] = fk
				fkRefs[name] = &fkRef{rOwner: rOwner, rConstraint: rConstraint}
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}
			fk.Columns = append(fk.Columns, col)
			fkRefs[name].positions = append(fkRefs[name].positions, pos)
		}
		fkRows.Close()
		if err := fkRows.Err(); err != nil {
			log.Printf("[oracle] rows iteration: %v", err)
		}

		// resolve referenced table and columns for each FK
		for name, fk := range fkMap {
			ref := fkRefs[name]
			// get referenced table name
			var refTable, deleteRule string
			err := db.QueryRowContext(ctx, `
				SELECT table_name, COALESCE(delete_rule, 'NO ACTION')
				FROM all_constraints
				WHERE owner = :1 AND constraint_name = :2 AND constraint_type = 'P'`,
				ref.rOwner, ref.rConstraint).Scan(&refTable, &deleteRule)
			if err != nil {
				logf(ctx, "[oracle] FK resolve table failed for %s: %v", name, err)
				continue
			}
			fk.RefDB = ref.rOwner
			fk.RefTable = refTable
			fk.OnDelete = deleteRule

			// get referenced columns (position-aligned)
			refColRows, err := db.QueryContext(ctx, `
				SELECT column_name FROM all_cons_columns
				WHERE owner = :1 AND constraint_name = :2
				ORDER BY position`, ref.rOwner, ref.rConstraint)
			if err != nil {
				logf(ctx, "[oracle] FK resolve columns failed for %s: %v", name, err)
				continue
			}
			for refColRows.Next() {
				var refCol string
				if err := refColRows.Scan(&refCol); err != nil {
					continue
				}
				fk.RefColumns = append(fk.RefColumns, refCol)
			}
			refColRows.Close()
			if err := refColRows.Err(); err != nil {
				log.Printf("[oracle] rows iteration: %v", err)
			}
		}
	} else {
		logf(ctx, "[oracle] FK query failed for %s: %v", t.Name, err)
	}

	// sampling for comment inference
	if len(colsWithoutComment) > 0 && t.RowCount > 0 {
		if sample, err := fetchOracleSampleRow(ctx, db, owner, t.Name); err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			logf(ctx, "[oracle] sample row failed for %s.%s: %v", owner, t.Name, err)
		}
	}
}

// fetchOracleSampleRow gets the first row of a table using FETCH FIRST 1 ROWS ONLY (12c+).
func fetchOracleSampleRow(ctx context.Context, db *sql.DB, owner, table string) (map[string]string, error) {
	q := fmt.Sprintf("SELECT * FROM %s.%s FETCH FIRST 1 ROWS ONLY",
		quoteOracle(owner), quoteOracle(table))
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

func buildOracleDSN(d *dsn.DSN) string {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "1521"
	}
	service := d.DBName
	if service == "" {
		service = "XE"
	}
	connStr := fmt.Sprintf("oracle://%s:%s@%s:%s/%s?connectionTimeout=5",
		url.QueryEscape(d.User), url.QueryEscape(d.Password), host, port, service)
	if d.TLS {
		connStr += "&ssl=true"
	}
	return connStr
}

func quoteOracle(name string) string {
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

func isOracleSystemSchema(name string) bool {
	sys := map[string]bool{
		"SYS":                true,
		"SYSTEM":             true,
		"DBSNMP":             true,
		"XDB":                true,
		"DVSYS":              true,
		"AUDSYS":             true,
		"GSMADMIN_INTERNAL":  true,
		"OJVMSYS":            true,
		"LBACSYS":            true,
		"OUTLN":              true,
		"APPQOSSYS":          true,
		"CTXSYS":             true,
		"MDSYS":              true,
		"ORDSYS":             true,
		"ORDDATA":            true,
		"ORDPLUGINS":         true,
		"SI_INFORMTN_SCHEMA": true,
		"DMSYS":              true,
		"OLAPSYS":            true,
		"EXFSYS":             true,
		"WMSYS":              true,
		"PERFSTAT":           true,
		"STDBYPERF":          true,
	}
	return sys[strings.ToUpper(name)]
}

// ExecQuery implements query.Queryable for Oracle.
func (oracleConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	connStr := buildOracleDSN(opts.DSN)
	db, err := sql.Open("oracle", connStr)
	if err != nil {
		return nil, fmt.Errorf("oracle open: %w", err)
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

	sqlStr := opts.SQL

	// Detect EXPLAIN PLAN FOR and use two-step with pinned connection
	if strings.HasPrefix(strings.TrimSpace(sqlStr), "EXPLAIN PLAN FOR") {
		return oracleExplain(runCtx, db, sqlStr)
	}

	// AutoLimit adaptation: replace trailing LIMIT N with FETCH FIRST N ROWS ONLY (Oracle 12c+)
	re := regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)\s*$`)
	sqlStr = re.ReplaceAllString(sqlStr, " FETCH FIRST $1 ROWS ONLY")

	result, err := executeSQLQuery(runCtx, db, sqlStr, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("oracle query: %w", err)
	}
	return result, nil
}

// oracleExplain handles Oracle's two-step EXPLAIN:
// Step 1: EXPLAIN PLAN FOR sql — writes plan to PLAN_TABLE
// Step 2: SELECT * FROM TABLE(DBMS_XPLAN.DISPLAY()) — reads the plan
// Uses db.Conn() to pin both steps to the same session.
func oracleExplain(ctx context.Context, db *sql.DB, explainSQL string) (*query.QueryResult, error) {
	start := time.Now()

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("oracle explain get conn: %w", err)
	}
	defer conn.Close()

	// Step 1: Execute EXPLAIN PLAN FOR (no result set)
	_, err = conn.ExecContext(ctx, explainSQL)
	if err != nil {
		return nil, fmt.Errorf("oracle explain plan: %w", err)
	}

	// Step 2: Read plan from DBMS_XPLAN.DISPLAY()
	planRows, err := conn.QueryContext(ctx, "SELECT * FROM TABLE(DBMS_XPLAN.DISPLAY())")
	if err != nil {
		return nil, fmt.Errorf("oracle explain display: %w", err)
	}
	defer planRows.Close()

	result := &query.QueryResult{}

	// Get column info
	colTypes, err := planRows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("oracle explain columns: %w", err)
	}
	colNames, err := planRows.Columns()
	if err != nil {
		return nil, fmt.Errorf("oracle explain col names: %w", err)
	}
	for i, ct := range colTypes {
		result.Columns = append(result.Columns, query.ColumnInfo{
			Name: colNames[i],
			Type: ct.DatabaseTypeName(),
		})
	}

	// Scan plan output rows
	for planRows.Next() {
		scanArgs := make([]interface{}, len(colNames))
		for i := range scanArgs {
			scanArgs[i] = new(interface{})
		}
		if err := planRows.Scan(scanArgs...); err != nil {
			continue
		}
		row := make([]*string, len(colNames))
		for i, arg := range scanArgs {
			val := arg.(*interface{})
			if *val == nil {
				row[i] = nil
			} else {
				s := fmt.Sprintf("%v", *val)
				if b, ok := (*val).([]byte); ok {
					s = string(b)
				}
				row[i] = &s
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if err := planRows.Err(); err != nil {
		log.Printf("[oracle] rows iteration: %v", err)
	}

	result.RowCount = len(result.Rows)
	result.ExecutionTime = time.Since(start).String()
	return result, nil
}
