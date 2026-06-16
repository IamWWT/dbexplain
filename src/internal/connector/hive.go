//go:build hive || full

package connector

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/beltran/gohive/v2"
	_ "github.com/beltran/gohive/v2"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("hive", func() Connector { return hiveConnector{} })
}

type hiveConnector struct{}

func (hiveConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapSQL,
		capabilities.CapRowCount,
		capabilities.CapSampling,
	}
}

func (hiveConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	cfg := buildHiveConfig(d)
	db := sql.OpenDB(gohive.NewConnector(cfg))
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "hive", Label: d.Label}

	return collectHiveSchema(ctx, db, inst)
}

// collectHiveSchema performs Hive schema collection using an existing database handle.
// Extracted for testability with go-sqlmock.
func collectHiveSchema(ctx context.Context, db *sql.DB, inst *schema.Instance) (*schema.Instance, error) {

	// Discover databases
	Logf(ctx, "[hive] [collect] %s", "SHOW DATABASES")
	dbRows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, schema.NewDBError(inst.DSN, "", "", "list databases", err)
	}
	defer dbRows.Close()

	var dbNames []string
	for dbRows.Next() {
		var name string
		if err := dbRows.Scan(&name); err == nil {
			name = strings.ToLower(name)
			if name != "information_schema" && name != "sys" && name != "default" {
				dbNames = append(dbNames, name)
			}
		}
	}
	dbRows.Close()
	if err := dbRows.Err(); err != nil {
		log.Printf("[hive] rows iteration: %v", err)
	}

	for _, dbName := range dbNames {
		Logf(ctx, "[hive] collecting database %s", dbName)
		database, err := collectHiveDB(ctx, db, dbName, inst.DSN)
		if err != nil {
			Logf(ctx, "[hive] error in database %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectHiveDB(ctx context.Context, db *sql.DB, dbName, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}

	// List tables
	Logf(ctx, "[hive] [collect] %s", "SHOW TABLES IN %s")
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SHOW TABLES IN %s", quoteHive(dbName)))
	if err != nil {
		return nil, schema.NewDBError(redactedDSN, dbName, "", "list tables", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tableNames = append(tableNames, name)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("[hive] rows iteration: %v", err)
	}

	total := len(tableNames)
	for i, tName := range tableNames {
		Logf(ctx, "[hive] collecting table %d/%d: %s.%s", i+1, total, dbName, tName)
		t := &schema.Table{Name: tName, RowCount: -1}
		fillHiveTable(ctx, db, dbName, t, redactedDSN)
		database.Tables = append(database.Tables, t)
	}
	return database, nil
}

func fillHiveTable(ctx context.Context, db *sql.DB, dbName string, t *schema.Table, redactedDSN string) {
	// DESCRIBE FORMATTED for column metadata
	descSQL := fmt.Sprintf("DESCRIBE FORMATTED %s.%s", quoteHive(dbName), quoteHive(t.Name))
	Logf(ctx, "[hive] [collect] %s", "DESCRIBE FORMATTED %s.%s")
	descRows, err := db.QueryContext(ctx, descSQL)
	if err != nil {
		Logf(ctx, "[hive] DESCRIBE FORMATTED failed for %s.%s: %v", dbName, t.Name, err)
		return
	}
	defer descRows.Close()

	var colsWithoutComment []*schema.Column
	stopped := false
	for descRows.Next() {
		var colName, dataType, comment string
		if err := descRows.Scan(&colName, &dataType, &comment); err != nil {
			continue
		}
		// Skip section header lines
		if strings.HasPrefix(colName, "#") {
			if colName == "# Detailed Table Information" || colName == "# Storage Information" {
				stopped = true
			}
			continue
		}
		if stopped {
			continue
		}
		// Skip separator lines and empty col_name
		colName = strings.TrimSpace(colName)
		if colName == "" {
			continue
		}
		c := &schema.Column{
			Name:     colName,
			Type:     strings.TrimSpace(dataType),
			Nullable: true, // Hive columns are nullable by default
			Comment:  strings.TrimSpace(comment),
		}
		t.Columns = append(t.Columns, c)
		if c.Comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}
	descRows.Close()
	if err := descRows.Err(); err != nil {
		log.Printf("[hive] rows iteration: %v", err)
	}

	// Sampling for comment inference
	if len(colsWithoutComment) > 0 {
		if sample, err := fetchHiveSampleRow(ctx, db, dbName, t.Name); err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			Logf(ctx, "[hive] sample row failed for %s.%s: %v", dbName, t.Name, err)
		}
	}
}

func fetchHiveSampleRow(ctx context.Context, db *sql.DB, dbName, table string) (map[string]string, error) {
	q := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 1", quoteHive(dbName), quoteHive(table))
	Logf(ctx, "[hive] [collect] %s", "SELECT * FROM %s.%s LIMIT 1")
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

// buildHiveConfig builds a gohive.Config from the DSN for use with sql.OpenDB.
func buildHiveConfig(d *dsn.DSN) gohive.Config {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := 10000
	if d.Port != "" {
		fmt.Sscanf(d.Port, "%d", &port)
	}
	dbName := d.DBName
	if dbName == "" {
		dbName = "default"
	}

	// Determine auth method
	auth := d.DSNParam("auth")
	if auth == "" {
		if d.User != "" {
			auth = "NONE"
		} else {
			auth = "NOSASL"
		}
	}

	cfg := gohive.Config{
		Host:     host,
		Port:     port,
		Auth:     auth,
		Username: d.User,
		Password: d.Password,
		Database: dbName,
		Service:  "hive",
	}

	// Optional DSN params
	if v := d.DSNParam("transport"); v != "" {
		cfg.TransportMode = v
	}
	if v := d.DSNParam("http_path"); v != "" {
		cfg.HTTPPath = v
	}
	if v := d.DSNParam("service"); v != "" {
		cfg.Service = v
	}
	if v := d.DSNParam("sslcert"); v != "" {
		cfg.SSLCertFile = v
	}
	if v := d.DSNParam("sslkey"); v != "" {
		cfg.SSLKeyFile = v
	}
	if v := d.DSNParam("sslca"); v != "" {
		cfg.SSLCAFile = v
	}
	if d.TLS {
		cfg.SSLInsecureSkip = d.TLSSkipVerify
		cfg.TLSConfig = &tls.Config{InsecureSkipVerify: d.TLSSkipVerify}
	}
	if d.DSNParam("sslinsecureskipverify") == "true" {
		cfg.SSLInsecureSkip = true
		if cfg.TLSConfig == nil {
			cfg.TLSConfig = &tls.Config{}
		}
		cfg.TLSConfig.InsecureSkipVerify = true
	}

	return cfg
}

func quoteHive(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// ExecQuery implements query.Queryable for Hive.
func (hiveConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	cfg := buildHiveConfig(opts.DSN)
	db := sql.OpenDB(gohive.NewConnector(cfg))
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
		return nil, fmt.Errorf("hive query: %w", err)
	}
	return result, nil
}
