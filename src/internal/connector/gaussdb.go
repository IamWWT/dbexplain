//go:build postgres || full

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("gaussdb", func() Connector { return gaussdbConnector{} })
}

type gaussdbConnector struct{}

func (gaussdbConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapForeignKey,
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapIndex,
		capabilities.CapSQL,
	}
}

func (gaussdbConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	connStr := buildPGDSN(d)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer func() { go db.Close() }()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	Debugf(ctx, "[DEBUG] gaussdb ping start: %s", d.Redacted())
	pingStart := time.Now()
	pingErr := db.PingContext(pingCtx)
	Debugf(ctx, "[DEBUG] gaussdb ping end: %s elapsed=%v err=%v", d.Redacted(), time.Since(pingStart), pingErr)
	if pingErr != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", pingErr)
	}

	// 设置 statement_timeout 保护数据库列表查询（collectPGDB 内也会设置）
	setPGStatementTimeout(ctx, db)

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "gaussdb", Label: d.Label}

	var dbNames []string
	// oracleCompatible=true 时跳过 datistemplate 查询（Oracle 兼容模式无此列）
	isOracleCompat := d.DSNParam("oracleCompatible") == "true"
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else if isOracleCompat {
		Logf(ctx, "[gaussdb] [collect] %s", `SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname (oracleCompatible)`)
		rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname`)
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
		if err := rows.Err(); err != nil {
			log.Printf("[gaussdb] rows iteration: %v", err)
		}
	} else {
		Logf(ctx, "[gaussdb] [collect] %s", `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
		rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
		if err != nil {
			// GaussDB Oracle 兼容模式可能没有 datistemplate 列，回退到简单查询
			Logf(ctx, "[gaussdb] datistemplate query failed, trying fallback: %v", err)
			Logf(ctx, "[gaussdb] [collect] %s", `SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname`)
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
			log.Printf("[gaussdb] rows iteration: %v", err)
		}
	}

	for _, dbName := range dbNames {
		Logf(ctx, "[gaussdb] collecting database %s", dbName)
		// GaussDB Oracle 兼容模式使用 information_schema.COLUMNS 替代 PG 专用函数
		collectCtx := WithGaussDBCompat(ctx)
		database, err := collectPGDB(collectCtx, db, dbName, d.Redacted())
		if err != nil {
			Logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

// ExecQuery implements query.Queryable for GaussDB.
// GaussDB Oracle-compatible mode uses PostgreSQL wire protocol via lib/pq.
func (gaussdbConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	connStr := buildPGDSN(opts.DSN)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("gaussdb open: %w", err)
	}
	defer func() { go db.Close() }()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// GaussDB Oracle 兼容模式可能不识别 statement_timeout GUC，
	// SET 失败时仅记录日志，查询继续运行（应用层超时兜底）
	if opts.Timeout > 0 {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("SET statement_timeout = '%ds'", opts.Timeout)); err != nil {
			Logf(ctx, "[gaussdb] set statement_timeout failed: %v (query will still run without timeout guard)", err)
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
		return nil, fmt.Errorf("gaussdb query: %w", err)
	}
	return result, nil
}
